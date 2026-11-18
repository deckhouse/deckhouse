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
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util/conditions"

	deckhousev1 "caps-controller-manager/api/deckhouse.io/v1alpha2"
	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
)

// Exit code 2 from the bootstrap wrapper means bashible took over and the node reboots, so
// it is a success for both ssh clients. Everything else, including exit code 1 from a stale
// bootstrap-token, must stay an error.
func TestBootstrapExitedWithStatus2(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		output string
		want   bool
	}{
		{
			name: "gossh reports the status in the error",
			err:  errors.New("Process exited with status 2"),
			want: true,
		},
		{
			name: "gossh error wrapped by the caller",
			err:  fmt.Errorf("wrapped: %w", errors.New("Process exited with status 2")),
			want: true,
		},
		{
			name:   "clissh reports the status in stderr",
			err:    errors.New("failed to run ssh command: exit status 2"),
			output: "debug1: Sending command\ndebug1: Exit status 2\n",
			want:   true,
		},
		{
			name:   "clissh status found in combined stdout and stderr",
			err:    errors.New("failed to run ssh command: exit status 2"),
			output: "bashible progress\ndebug1: Exit status 2",
			want:   true,
		},
		{
			name:   "exit code 1 of a stale bootstrap-token",
			err:    errors.New("Process exited with status 1"),
			output: "The node already have bootstrap-token and under bashible",
			want:   false,
		},
		{
			name:   "clissh exit status 1",
			err:    errors.New("failed to run ssh command: exit status 1"),
			output: "debug1: Exit status 1",
			want:   false,
		},
		{
			name: "gossh exit status 22 is not status 2",
			err:  errors.New("Process exited with status 22"),
			want: false,
		},
		{
			name: "gossh exit status 255 is a connection failure, not status 2",
			err:  errors.New("Process exited with status 255"),
			want: false,
		},
		{
			name:   "clissh exit status 255 is not status 2",
			err:    errors.New("failed to run ssh command: exit status 255"),
			output: "debug1: Exit status 255",
			want:   false,
		},
		{
			name: "dropped connection without output",
			err:  errors.New("wait: remote command exited without exit status or exit signal"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, hasExitedWithStatus2(tt.err, tt.output))
		})
	}
}

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
