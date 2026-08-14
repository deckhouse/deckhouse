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

// TestReconcilerCapsItselfAtOneWorker: the rollout gate is only correct with
// one worker, declared through the interface the manager setup looks for —
// dropping out of that interface must fail here, not restore the race silently.
func TestReconcilerCapsItselfAtOneWorker(t *testing.T) {
	require.Implements(t, (*register.NeedsMaxConcurrentReconciles)(nil), &Reconciler{})
	require.Equal(t, 1, (&Reconciler{}).MaxConcurrentReconciles())
}

// A pass memoises read failures too: cluster-wide inputs fail identically for
// every node, and forgetting the failure turned one missing object into
// thousands of live reads on a large fleet.
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

// The budget is read once per group per pass and spent as slots are handed out;
// without counting its own writes every node would be judged against a group
// where nothing is updating, and the whole group would change at once.
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
		// What absent, too-old or broken agents look like from here.
		// Grandfathering them unconditionally let all of them take every change
		// and disrupt together the moment their agents came back.
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

// The held node's log line names who it is waiting for, so the order has to be
// the group's and not the map's — an operator comparing two lines must not see
// the same group reshuffled.
func TestBudgetHoldersAreSorted(t *testing.T) {
	budget := &rolloutBudget{concurrency: 1, updating: map[string]struct{}{
		"node-c": {}, "node-a": {}, "node-b": {},
	}}

	require.Equal(t, []string{"node-a", "node-b", "node-c"}, budget.holders())
	require.Empty(t, (&rolloutBudget{updating: map[string]struct{}{}}).holders())
}

// testOldOSImageDigest is a spec nobody publishes any more: what a node left
// behind by a bad release is still carrying.
const testOldOSImageDigest = "sha256:1111111111111111111111111111111111111111111111111111111111111111"

func notApplied(name string, spec internalv1alpha1.NodeSpec) internalv1alpha1.NodeConfig {
	nc := internalv1alpha1.NodeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name, Generation: 2},
		Spec:       spec,
	}
	nc.Status.AppliedGeneration = 1
	return nc
}

func appliedConfig(name string, spec internalv1alpha1.NodeSpec) internalv1alpha1.NodeConfig {
	nc := internalv1alpha1.NodeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name, Generation: 2},
		Spec:       spec,
	}
	nc.Status.AppliedGeneration = 2
	meta.SetStatusCondition(&nc.Status.Conditions, metav1.Condition{
		Type: configurationAppliedCondition, Status: metav1.ConditionTrue,
		Reason: "ReconcileSucceeded", Message: "applied", ObservedGeneration: nc.Generation,
	})
	return nc
}

// The gate must hold "no more than N nodes carry an unproven current spec", not
// "no more than N nodes are unhealthy" — the latter it cannot control, and
// trying to hold it locks the cure out of a group that is already broken.
func TestBudgetIgnoresNodesStuckOnAnOlderSpec(t *testing.T) {
	desired := internalv1alpha1.NodeSpec{NodeName: "n1", OSImage: internalv1alpha1.OSImage{Digest: testOSImageDigest}}
	stale := internalv1alpha1.NodeSpec{NodeName: "n1", OSImage: internalv1alpha1.OSImage{Digest: testOldOSImageDigest}}

	configs := []internalv1alpha1.NodeConfig{
		notApplied("n1", stale),   // broken on a spec nobody publishes any more
		notApplied("n2", desired), // genuinely mid-rollout
		appliedConfig("n3", desired),
	}

	updating := membersOfUpdating(configs, map[string]internalv1alpha1.NodeSpec{
		"n1": desired, "n2": desired, "n3": desired,
	})

	if _, ok := updating["n1"]; ok {
		t.Fatal("a node stuck on an older spec occupies a rollout slot it does not use")
	}
	if _, ok := updating["n2"]; !ok {
		t.Fatal("a node carrying the current spec without proving it must hold its slot")
	}
	if _, ok := updating["n3"]; ok {
		t.Fatal("a converged node holds a slot")
	}
}

// A NodeConfig whose Node is gone is about to be deleted; it must not hold the
// group hostage in the meantime.
func TestBudgetIgnoresOrphans(t *testing.T) {
	desired := internalv1alpha1.NodeSpec{NodeName: "gone", OSImage: internalv1alpha1.OSImage{Digest: testOSImageDigest}}
	configs := []internalv1alpha1.NodeConfig{notApplied("gone", desired)}

	if updating := membersOfUpdating(configs, map[string]internalv1alpha1.NodeSpec{}); len(updating) != 0 {
		t.Fatalf("membersOfUpdating() = %v, want empty: an orphan holds no slot", updating)
	}
}

// The bootstrap-only fields the render cannot reproduce must not read as drift:
// a node keeping the address it booted with is not stuck on an older spec.
func TestBudgetIgnoresBootstrapOnlyFields(t *testing.T) {
	desired := internalv1alpha1.NodeSpec{NodeName: "n1", OSImage: internalv1alpha1.OSImage{Digest: testOSImageDigest}}
	bootstrapped := desired
	bootstrapped.Kubelet.NodeIP = "10.0.0.5"

	updating := membersOfUpdating(
		[]internalv1alpha1.NodeConfig{notApplied("n1", bootstrapped)},
		map[string]internalv1alpha1.NodeSpec{"n1": desired},
	)

	if _, ok := updating["n1"]; !ok {
		t.Fatal("a node carrying the current spec plus its bootstrapped address must hold its slot")
	}
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

// applied() must key on the generation the node is RUNNING (appliedGeneration),
// not the one it has SEEN (observedGeneration), and on ConfigurationApplied
// rather than phase — an unrelated Degraded must not hold the slot forever.
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
