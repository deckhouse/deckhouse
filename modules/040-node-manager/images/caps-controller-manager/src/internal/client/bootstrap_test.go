/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util/conditions"

	deckhousev1 "caps-controller-manager/api/deckhouse.io/v1alpha2"
	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
)

func pendingStaticInstance() *deckhousev1.StaticInstance {
	return &deckhousev1.StaticInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "static-instance"},
		Status: deckhousev1.StaticInstanceStatus{
			CurrentStatus: &deckhousev1.StaticInstanceStatusCurrentStatus{
				Phase:          deckhousev1.StaticInstanceStatusCurrentStatusPhasePending,
				LastUpdateTime: metav1.NewTime(time.Now().Add(-time.Hour).UTC()),
			},
		},
	}
}

func staticMachine() *infrav1.StaticMachine {
	return &infrav1.StaticMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "static-machine",
			Namespace: "d8-cloud-instance-manager",
			UID:       types.UID("a2c0f1ba-0000-4000-8000-000000000001"),
		},
	}
}

// Reserving an instance that this very StaticMachine already holds must not touch the
// status again: SetPhase returns early on an unchanged phase, so a repeated attempt
// produces no diff and therefore no write to etcd.
func TestReserveIsIdempotentForTheSameMachine(t *testing.T) {
	c := &Client{}
	instance := pendingStaticInstance()
	machine := staticMachine()

	require.NoError(t, c.reserveStaticInstance(instance, machine))
	require.Equal(t, deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping, instance.GetPhase())
	require.NotNil(t, instance.Status.MachineRef)

	settled := instance.DeepCopy()

	require.NoError(t, c.reserveStaticInstance(instance, machine))

	require.Equal(t, settled.Status, instance.Status)
}

// An instance already reserved by another StaticMachine must be refused rather than stolen.
func TestReserveRefusesInstanceHeldByAnotherMachine(t *testing.T) {
	c := &Client{}
	instance := pendingStaticInstance()

	owner := staticMachine()
	require.NoError(t, c.reserveStaticInstance(instance, owner))

	reserved := instance.DeepCopy()

	other := staticMachine()
	other.UID = types.UID("a2c0f1ba-0000-4000-8000-000000000002")

	require.Error(t, c.reserveStaticInstance(instance, other))
	require.Equal(t, reserved.Status, instance.Status)
}

// A host that keeps refusing ssh must not rewrite the StaticInstance on every attempt. This
// walks the status through what a repeated failed reconcile does to it — re-reserving the
// instance the machine already holds and re-reporting the same failure — and requires the
// result to be byte-identical, because any diff here is a write to etcd, a watch event and
// an immediate re-reconcile, which is the loop this fix is about. Warning events are separate
// objects and are still emitted per attempt; this only bounds writes to the instance itself.
func TestRepeatedFailedAttemptProducesNoStatusDiff(t *testing.T) {
	c := &Client{}
	instance := pendingStaticInstance()
	machine := staticMachine()

	failure := func(instance *deckhousev1.StaticInstance) {
		require.NoError(t, c.reserveStaticInstance(instance, machine))

		conditions.Set(instance, metav1.Condition{
			Type:               infrav1.StaticInstanceCheckSSHCondition,
			Status:             metav1.ConditionFalse,
			Reason:             infrav1.StaticInstanceCheckFailedReason,
			Message:            "failed to connect via ssh with address 192.168.0.1:22: handshake failed",
			LastTransitionTime: metav1.Now(),
		})
	}

	failure(instance)
	settled := instance.DeepCopy()

	failure(instance)

	require.Equal(t, settled.Status, instance.Status)
}

// Only a successful check may reset the backoff. If Forget is called on every attempt, the
// delay stays at the base value and the rate limiter never limits anything.
func TestSSHCheckRateLimiterBacksOffUntilSuccess(t *testing.T) {
	c := NewClient(nil, nil)
	defer c.taskManagerCancel()

	const address = "192.168.0.1:22"

	first := c.sshCheckRateLimiter.When(address)
	second := c.sshCheckRateLimiter.When(address)
	require.Greater(t, second, first)

	c.sshCheckRateLimiter.Forget(address)

	require.Equal(t, first, c.sshCheckRateLimiter.When(address))
}

func TestRequestedNodeNameCommandLeavesTheNameWhereBootstrapLooksForIt(t *testing.T) {
	if got := requestedNodeNameCommand(""); got != "" {
		t.Fatalf("an unset name should add nothing to the bootstrap command, got %q", got)
	}

	got := requestedNodeNameCommand("worker-rack3-07")
	want := " && echo 'worker-rack3-07' > /var/lib/bashible/node-name"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// The CRD pattern keeps anything but an RFC 1123 subdomain out of spec.nodeName,
// so this only ever runs on a caller that got past it. What the quoting has to
// guarantee is that whatever the value holds stays one shell word: the fragment
// is interpolated into a command run over SSH as root.
func TestRequestedNodeNameCommandKeepsTheNameOneShellWord(t *testing.T) {
	// One directory for the whole table, and files named by index: a per-subtest
	// TempDir would carry the case into the path, and the cases are chosen to be
	// exactly the strings a shell would rather not be handed.
	dir := t.TempDir()

	names := []string{
		"plain-name",
		"a'; touch /tmp/pwned; echo 'b",
		"quotes'\"and$backticks`",
		"$(id -u)",
		"two\nlines",
	}

	for i, name := range names {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			target := filepath.Join(dir, strconv.Itoa(i))

			script := strings.TrimPrefix(requestedNodeNameCommand(name), " && ")
			script = strings.Replace(script, "/var/lib/bashible/node-name", target, 1)

			if out, err := exec.Command("sh", "-c", script).CombinedOutput(); err != nil {
				t.Fatalf("run %q: %v (%s)", script, err, out)
			}

			written, err := os.ReadFile(target)
			if err != nil {
				t.Fatalf("read back: %v", err)
			}
			if got := strings.TrimSuffix(string(written), "\n"); got != name {
				t.Fatalf("the shell saw %q, not the name %q", got, name)
			}
		})
	}
}
