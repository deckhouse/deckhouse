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

package nodeoperation

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
)

// hardDeadline is the whole timeout model, so these are scenarios rather than
// branches: an operation is followed through the states it really passes and the
// deadline is asked for at each one.
func TestHardDeadline(t *testing.T) {
	created := time.Now().Add(-2 * time.Hour)
	op := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.Time{Time: created}},
	}

	t.Run("a queued operation is bounded from its creation", func(t *testing.T) {
		require.Equal(t, created.Add(operationTimeout), hardDeadline(op))
		require.True(t, time.Now().After(hardDeadline(op)), "two hours queued is past the bound")
	})

	t.Run("asking for an eviction extends it to that eviction's own bound", func(t *testing.T) {
		drainEnds := time.Now().Add(time.Hour)
		op.Status.DrainDeadline = ptr.To(metav1.NewTime(drainEnds))

		require.Equal(t, drainEnds.Add(operationTimeout), hardDeadline(op))
		require.False(t, time.Now().After(hardDeadline(op)),
			"an operation whose drain is still running must not be past the bound")
	})

	t.Run("handing the work to the node restarts the clock at that moment", func(t *testing.T) {
		startedAt := time.Now().Add(-time.Minute)
		op.Status.StartedAt = ptr.To(metav1.NewTime(startedAt))

		require.Equal(t, startedAt.Add(operationTimeout), hardDeadline(op))
	})
}

// nodeDrainTimeoutSecond is a plain integer. Past roughly 9.2e9 seconds the
// multiplication into a Duration wraps negative, which would put the deadline
// before the drain even started and fail the operation instantly.
func TestDrainTimeoutOfNeverGoesBackwards(t *testing.T) {
	tests := []struct {
		name    string
		seconds *int
	}{
		{name: "unset"},
		{name: "zero", seconds: ptr.To(0)},
		{name: "negative", seconds: ptr.To(-1)},
		{name: "ordinary", seconds: ptr.To(3600)},
		{name: "past the cap", seconds: ptr.To(100000)},
		{name: "overflowing a duration", seconds: ptr.To(9223372037)},
		{name: "the largest int", seconds: ptr.To(int(^uint(0) >> 1))},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := drainTimeoutOf(&v1.NodeGroup{
				Spec: v1.NodeGroupSpec{NodeDrainTimeoutSecond: tc.seconds},
			})
			require.Positive(t, got, "a drain bound must never be zero or negative")
			require.LessOrEqual(t, got, maxDrainTimeout)
		})
	}
}

// The two controllers agree on what "no nodeDrainTimeoutSecond" means via a
// copied constant (the draining controller's is unexported). This reads its
// declaration and compares the two source expressions.
func TestDefaultDrainTimeoutMatchesTheDrainingController(t *testing.T) {
	ours := declaredDefaultDrainTimeout(t, "constants.go")
	theirs := declaredDefaultDrainTimeout(t, "../draining/controller.go")

	require.Equal(t, theirs, ours,
		"defaultDrainTimeout must stay equal to the draining controller's own default; "+
			"an operation that allows less than the drain it waits for aborts a healthy drain")
}

func declaredDefaultDrainTimeout(t *testing.T, path string) string {
	t.Helper()

	source, err := os.ReadFile(path)
	require.NoError(t, err)

	match := regexp.MustCompile(`defaultDrainTimeout\s*=\s*([^\n/]+)`).FindSubmatch(source)
	require.NotNil(t, match, "no defaultDrainTimeout declaration in %s", path)
	return strings.TrimSpace(string(match[1]))
}

func TestTimedOut(t *testing.T) {
	deadline := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		op         v1alpha1.NodeOperation
		expReason  string
		expMessage string
	}{
		{
			name: "waiting for the eviction names the node holding the group",
			op: v1alpha1.NodeOperation{
				Spec: v1alpha1.NodeOperationSpec{NodeName: "worker-1"},
			},
			expReason:  "PreparationTimedOut",
			expMessage: "node worker-1 was not prepared by 2026-08-04T12:00:00Z: its workload has not finished leaving",
		},
		{
			name: "waiting for the node names the node that went quiet",
			op: v1alpha1.NodeOperation{
				Spec:   v1alpha1.NodeOperationSpec{NodeName: "worker-2"},
				Status: v1alpha1.NodeOperationStatus{StartedAt: &metav1.Time{Time: time.Now()}},
			},
			expReason:  "NodeTimedOut",
			expMessage: "node worker-2 did not report back by 2026-08-04T12:00:00Z",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reason, message := timedOut(&tc.op, deadline)
			require.Equal(t, tc.expReason, reason)
			require.Equal(t, tc.expMessage, message)
		})
	}
}
