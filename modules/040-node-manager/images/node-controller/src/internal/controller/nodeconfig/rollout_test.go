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

package nodeconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/derived_status"
	"github.com/deckhouse/node-controller/internal/register"
)

// The gate above is only correct with one worker, and this is where that is
// declared. It has to be declared through the interface the manager setup looks
// for, or the declaration is a method nobody calls: renaming it, or dropping it
// out of the interface, has to fail here rather than silently restore the
// multi-worker race. (What the setup then does with the number is
// register.TestEffectiveMaxConcurrentReconciles; register cannot import this
// package to do both in one place.)
func TestReconcilerCapsItselfAtOneWorker(t *testing.T) {
	require.Implements(t, (*register.NeedsMaxConcurrentReconciles)(nil), &Reconciler{})
	require.Equal(t, 1, (&Reconciler{}).MaxConcurrentReconciles())
}

// A pass remembers that the cluster's own state could not be read, not only that
// it could. These inputs are cluster-wide by construction — one absent DNS
// service, one unreadable secret, fails identically for every node — so a pass
// that forgot the failure repeated the uncached reads behind it once per node,
// turning a single missing object into thousands of live reads on a large fleet.
func TestPassRemembersThatTheClusterCouldNotBeRead(t *testing.T) {
	cluster := dnsCluster(t) // empty: the first read already fails
	r := &Reconciler{}
	r.Client = cluster
	r.sources = sourceReaderOver(cluster)
	r.derived = &derived_status.Service{Client: cluster}

	ng := &v1.NodeGroup{}
	ng.Name = "worker"
	p := newPass()

	_, first := r.clusterInputs(t.Context(), ng, p)
	require.Error(t, first)
	require.Len(t, p.inputs, 1, "the attempt has to be remembered, failure and all")

	// Every node after the first in the same pass is answered from the pass,
	// with the same error, instead of reading the cluster again.
	_, second := r.clusterInputs(t.Context(), ng, p)
	require.Equal(t, first, second)
}

// The budget is read once per group per pass and spent as the pass hands out
// slots. Counting its own writes is what makes one reading as accurate as one
// reading per node: without it, every node of the group would be judged against
// a group where nothing is updating, and the whole group would take the change
// at once.
func TestRolloutBudget(t *testing.T) {
	t.Run("one node at a time", func(t *testing.T) {
		budget := &rolloutBudget{concurrency: 1, updating: map[string]struct{}{}}

		require.True(t, budget.hasSlot("node-a"))
		budget.spend("node-a")

		require.False(t, budget.hasSlot("node-b"), "the group is already updating a node")
		// The node that has the slot keeps it: it is mid-update either way, and
		// withholding the next change would strand it on a config nobody is
		// going to finish.
		require.True(t, budget.hasSlot("node-a"))
	})

	t.Run("a node that was already updating when the group was read", func(t *testing.T) {
		budget := &rolloutBudget{concurrency: 1, updating: map[string]struct{}{"node-a": {}}}

		require.True(t, budget.hasSlot("node-a"))
		require.False(t, budget.hasSlot("node-b"))
	})

	t.Run("a group with more silent nodes than it allows updates", func(t *testing.T) {
		// What an agent that is absent, too old to publish the condition, or
		// broken looks like from here. Grandfathering them unconditionally let
		// every one of them take every change, and they disrupted together the
		// moment their agents came back — the gate opening widest exactly when
		// the group was least healthy.
		budget := &rolloutBudget{concurrency: 1, updating: map[string]struct{}{
			"node-a": {}, "node-b": {}, "node-c": {},
		}}

		require.False(t, budget.hasSlot("node-a"))
		require.False(t, budget.hasSlot("node-b"))
		require.False(t, budget.hasSlot("node-d"), "a node that has applied has no slot either")
	})

	t.Run("a group exactly at its budget still finishes the node it started", func(t *testing.T) {
		budget := &rolloutBudget{concurrency: 2, updating: map[string]struct{}{"node-a": {}, "node-b": {}}}

		require.True(t, budget.hasSlot("node-a"))
		require.True(t, budget.hasSlot("node-b"))
		require.False(t, budget.hasSlot("node-c"))
	})

	t.Run("a group allowed more at once", func(t *testing.T) {
		budget := &rolloutBudget{concurrency: 3, updating: map[string]struct{}{}}

		for _, name := range []string{"node-a", "node-b", "node-c"} {
			require.True(t, budget.hasSlot(name))
			budget.spend(name)
		}
		require.False(t, budget.hasSlot("node-d"))
	})
}

func nodeConfigAt(generation, observed, applied int64, phase string) *internalv1alpha1.NodeConfig {
	nc := &internalv1alpha1.NodeConfig{}
	nc.Generation = generation
	nc.Status.ObservedGeneration = observed
	nc.Status.AppliedGeneration = applied
	nc.Status.Phase = phase
	return nc
}

// withConfigApplied stamps the condition the agent publishes on every pass,
// carrying the generation it describes.
func withConfigApplied(nc *internalv1alpha1.NodeConfig, status metav1.ConditionStatus, generation int64) *internalv1alpha1.NodeConfig {
	meta.SetStatusCondition(&nc.Status.Conditions, metav1.Condition{
		Type:               configurationAppliedCondition,
		Status:             status,
		Reason:             "ReconcileSucceeded",
		ObservedGeneration: generation,
	})
	return nc
}

// applied() is the rollout's "this node converged" test. It must key on the
// generation the node is RUNNING (appliedGeneration), not the one it has merely
// SEEN (observedGeneration): a held node has observed the current generation but
// is still running the previous one, and counting it as done would walk the
// change through the whole group while every node waits.
//
// It must also key on ConfigurationApplied rather than on the phase. The two
// answer different questions, and a node that is Degraded for a reason the
// rollout neither caused nor can fix must not hold the group's slot forever.
func TestApplied(t *testing.T) {
	tests := []struct {
		name       string
		nc         *internalv1alpha1.NodeConfig
		disruption bool
		want       bool
	}{
		{
			name: "running the current generation, Ready",
			nc:   withConfigApplied(nodeConfigAt(4, 4, 4, phaseReady), metav1.ConditionTrue, 4),
			want: true,
		},
		{
			name: "has seen gen 4 but is still running gen 3",
			nc:   withConfigApplied(nodeConfigAt(4, 4, 3, phaseReady), metav1.ConditionFalse, 4),
			want: false,
		},
		{
			name: "observedGeneration says 4 but appliedGeneration is 3 — not done",
			// The exact overstating agent the two-number split guards against.
			nc:   withConfigApplied(nodeConfigAt(4, 4, 3, phaseReady), metav1.ConditionTrue, 4),
			want: false,
		},
		{
			name: "rolled the config back: running gen 4's number but not its config",
			nc:   withConfigApplied(nodeConfigAt(4, 4, 4, "Degraded"), metav1.ConditionFalse, 4),
			want: false,
		},
		{
			// What the phase test got wrong. The node took the spec and is running
			// it; something else on it is broken. Holding the slot here froze every
			// other node in the group behind a fault the rollout cannot repair.
			name: "running the published config, but degraded for an unrelated reason",
			nc:   withConfigApplied(nodeConfigAt(4, 4, 4, "Degraded"), metav1.ConditionTrue, 4),
			want: true,
		},
		{
			name: "the condition is a generation stale",
			nc:   withConfigApplied(nodeConfigAt(4, 4, 4, phaseReady), metav1.ConditionTrue, 3),
			want: false,
		},
		{
			name:       "applied and Ready, but still asking to be interrupted",
			nc:         withConfigApplied(nodeConfigAt(4, 4, 4, phaseReady), metav1.ConditionTrue, 4),
			disruption: true,
			want:       false,
		},
		{
			name: "never reported: appliedGeneration 0",
			nc:   nodeConfigAt(4, 0, 0, ""),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.disruption {
				meta.SetStatusCondition(&tt.nc.Status.Conditions, metav1.Condition{
					Type:               disruptionRequiredCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "DisruptionPending",
					ObservedGeneration: tt.nc.Generation,
				})
			}
			if got := applied(tt.nc); got != tt.want {
				t.Fatalf("applied() = %v, want %v", got, tt.want)
			}
		})
	}
}
