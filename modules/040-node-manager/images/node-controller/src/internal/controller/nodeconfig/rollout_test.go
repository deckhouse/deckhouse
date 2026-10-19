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
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
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

// The NodeConfig watch enqueues the whole fleet, so it must fire on what a pass
// reads and on nothing else.
func TestNodeConfigRolloutInputsChanged(t *testing.T) {
	config := func(mutate func(*internalv1alpha1.NodeConfig)) *internalv1alpha1.NodeConfig {
		nc := &internalv1alpha1.NodeConfig{ObjectMeta: metav1.ObjectMeta{Name: "worker-0", Generation: 3}}
		nc.Status.AppliedGeneration = 3
		nc.Status.Extensions = []internalv1alpha1.ExtensionStatus{
			{Name: containerdExtension, Digest: "sha256:old", State: "Ready"},
		}
		nc.Status.StaticPods = []internalv1alpha1.StaticPodStatus{
			{Name: "registry-agent", State: "Failed", Reason: "WriteFailed", Message: "read-only file system"},
		}
		nc.Status.LastReconcileTime = metav1.NewTime(time.Unix(1700000000, 0))
		if mutate != nil {
			mutate(nc)
		}
		return nc
	}

	tests := []struct {
		name       string
		after      *internalv1alpha1.NodeConfig
		expChanged bool
	}{
		{
			// nerOutcomes counts Ready only at the digest the spec asks for.
			name: "only the status digest changed",
			after: config(func(nc *internalv1alpha1.NodeConfig) {
				nc.Status.Extensions[0].Digest = "sha256:new"
			}),
			expChanged: true,
		},
		{
			name: "an extension started failing",
			after: config(func(nc *internalv1alpha1.NodeConfig) {
				nc.Status.Extensions[0].State = "Failed"
			}),
			expChanged: true,
		},
		{
			name: "the node finished applying",
			after: config(func(nc *internalv1alpha1.NodeConfig) {
				nc.Status.AppliedGeneration = 4
			}),
			expChanged: true,
		},
		{
			// status.staticPods is the only source of a NodeStaticPodRequest's
			// counts, and a retry that wrote the manifest moves nothing else.
			name: "a static pod was written on a retry",
			after: config(func(nc *internalv1alpha1.NodeConfig) {
				nc.Status.StaticPods[0] = internalv1alpha1.StaticPodStatus{
					Name: "registry-agent", State: "Written",
				}
			}),
			expChanged: true,
		},
		{
			// nsprOutcomes counts Written only at the digest of the request's manifest.
			name: "only the manifest digest changed",
			after: config(func(nc *internalv1alpha1.NodeConfig) {
				nc.Status.StaticPods[0].ManifestDigest = "sha256:new"
			}),
			expChanged: true,
		},
		{
			name: "only the heartbeat moved",
			after: config(func(nc *internalv1alpha1.NodeConfig) {
				nc.Status.LastReconcileTime = metav1.NewTime(time.Unix(1700000600, 0))
			}),
			expChanged: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expChanged, nodeConfigRolloutInputsChanged(config(nil), tt.after))
		})
	}
}

// maxConcurrent passes the NodeGroup schema as any integer, and a budget of
// zero hands out no slot ever: the group is frozen for good, with no error and
// no event. The floor is this controller's, the shared helper has none.
func TestGroupConcurrencyNeverFreezesAGroup(t *testing.T) {
	tests := []struct {
		name          string
		maxConcurrent *intstr.IntOrString
		nodes         int
		want          int
	}{
		{name: "a group that says nothing updates one node at a time", nodes: 5, want: 1},
		{
			name:          "zero would freeze the group forever",
			maxConcurrent: ptr.To(intstr.FromInt32(0)),
			nodes:         5,
			want:          1,
		},
		{
			name:          "a negative number, which the schema takes too",
			maxConcurrent: ptr.To(intstr.FromInt32(-3)),
			nodes:         5,
			want:          1,
		},
		{
			name:          "a percentage rounding down to nothing",
			maxConcurrent: ptr.To(intstr.FromString("10%")),
			nodes:         5,
			want:          1,
		},
		{
			name:          "what the operator asked for",
			maxConcurrent: ptr.To(intstr.FromInt32(3)),
			nodes:         5,
			want:          3,
		},
		{
			name:          "a percentage of the group",
			maxConcurrent: ptr.To(intstr.FromString("40%")),
			nodes:         5,
			want:          2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ng := &v1.NodeGroup{Spec: v1.NodeGroupSpec{Update: &v1.UpdateSpec{MaxConcurrent: tt.maxConcurrent}}}

			require.Equal(t, tt.want, groupConcurrency(ng, tt.nodes))
		})
	}
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
		Type: internalv1alpha1.NodeConfigConditionTypeConfigurationApplied, Status: metav1.ConditionTrue,
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
	}, false)

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

	if updating := membersOfUpdating(configs, map[string]internalv1alpha1.NodeSpec{}, false); len(updating) != 0 {
		t.Fatalf("membersOfUpdating() = %v, want empty: an orphan holds no slot", updating)
	}
}

// The bootstrap-only fields the render cannot reproduce must not read as drift:
// a node keeping the status token it minted is not stuck on an older spec.
func TestBudgetIgnoresBootstrapOnlyFields(t *testing.T) {
	desired := internalv1alpha1.NodeSpec{NodeName: "n1", OSImage: internalv1alpha1.OSImage{Digest: testOSImageDigest}}
	bootstrapped := desired
	bootstrapped.StatusToken = "1f8b0800000000000003"

	updating := membersOfUpdating(
		[]internalv1alpha1.NodeConfig{notApplied("n1", bootstrapped)},
		map[string]internalv1alpha1.NodeSpec{"n1": desired},
		false,
	)

	if _, ok := updating["n1"]; !ok {
		t.Fatal("a node carrying the current spec plus its own status token must hold its slot")
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
		Type:               internalv1alpha1.NodeConfigConditionTypeConfigurationApplied,
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
					Type:               internalv1alpha1.NodeConfigConditionTypeDisruptionApproved,
					Status:             metav1.ConditionFalse,
					Reason:             "ApprovalPending",
					ObservedGeneration: tt.nc.Generation,
				})
			}
			if got := applied(tt.nc); got != tt.want {
				t.Fatalf("applied() = %v, want %v", got, tt.want)
			}
		})
	}
}

// The inputs of a bashible node are the same for every group, so a pass reads
// them once, failure included, however many such nodes it renders.
func TestPassReadsTheInputsOfABashibleNodeOnce(t *testing.T) {
	cluster := dnsCluster(t) // empty: the first read already fails
	r := &Reconciler{}
	r.sources = sourceReaderOver(cluster)
	p := newPass()

	_, first := r.mutableInputs(t.Context(), p)
	require.Error(t, first)
	require.NotNil(t, p.mutable, "the attempt has to be remembered, failure and all")

	_, second := r.mutableInputs(t.Context(), p)
	require.Equal(t, first, second)
}

// mutableReconciler is a reconciler over cl and a pass that has already read the inputs of a
// bashible node as in.
func mutableReconciler(cl client.Client, in clusterInputs) (*Reconciler, *pass) {
	r := &Reconciler{}
	r.Client = cl
	r.sources = sourceReaderOver(cl)
	p := newPass()
	p.mutable = &clusterInputsResult{inputs: in}
	return r, p
}

// A request selects a bashible node by its labels, so the budget renders each node with
// them. Rendered by name alone, a node that took the request reads as one left behind,
// and the slot it holds goes to a second node.
func TestTheBudgetOfABashibleGroupRendersEachNodeWithItsLabels(t *testing.T) {
	ng := mutableGroup("worker", v1.SystemTypeMutable)
	node := nodeInGroup("worker-0", "worker")
	node.Labels["storage"] = "drbd"
	var in clusterInputs
	in.NodeExtensions, in.NodeExtensionConflicts = settleNERs([]deckhousev1alpha1.NodeExtensionRequest{
		ner("sds-utils", deckhousev1alpha1.NodeExtensionRequestSpec{
			Extension:    deckhousev1alpha1.NodeExtension{Name: "sds-utils", Digest: drbdDigest, Kind: deckhousev1alpha1.ExtensionKindPackage},
			NodeSelector: deckhousev1alpha1.NodeSelector{MatchLabels: map[string]string{"storage": "drbd"}},
		}),
	}, noPlatform)
	config := notApplied("worker-0", renderMutableSpec(&node, "worker", in))
	config.Labels = map[string]string{nodecommon.NodeGroupLabel: "worker"}
	cl := fake.NewClientBuilder().WithScheme(nsprStatusScheme(t)).WithObjects(ng, &node, &config).Build()
	r, p := mutableReconciler(cl, in)

	budget, err := r.rolloutBudget(t.Context(), ng, p)
	require.NoError(t, err)
	require.Contains(t, budget.updating, "worker-0", "it carries the published spec, unproven")
}

// bashible installs the agent of a node it manages later, under the group's own update
// approval, and the document is written before that. One node waiting for its agent held
// the only slot, and no other node of the group got its document.
func TestABashibleNodeWithoutAnAgentHoldsNoSlot(t *testing.T) {
	ng := mutableGroup("worker", v1.SystemTypeMutable)
	current := nodeInGroup("worker-0", "worker")
	fresh := nodeInGroup("worker-1", "worker")
	stale := nodeInGroup("worker-2", "worker")
	silentConfig := func(node *corev1.Node, in clusterInputs) *internalv1alpha1.NodeConfig {
		return &internalv1alpha1.NodeConfig{
			ObjectMeta: metav1.ObjectMeta{Name: node.Name, Generation: 1,
				Labels: map[string]string{nodecommon.NodeGroupLabel: ng.Name}},
			Spec: renderMutableSpec(node, ng.Name, in),
		}
	}
	// The stale document is created the way a writer creates one, so its fields have an
	// owner for the apply to take over; an object nobody owns a field of keeps it.
	cl := fake.NewClientBuilder().WithScheme(nsprStatusScheme(t)).WithReturnManagedFields().
		WithObjects(ng, &current, &fresh, &stale, silentConfig(&current, clusterInputs{})).Build()
	require.NoError(t, cl.Create(t.Context(), silentConfig(&stale, clusterInputs{RegistryPackagesProxyToken: []byte("previous-token")}),
		client.FieldOwner("node-controller-before-apply")))
	r, p := mutableReconciler(cl, clusterInputs{})

	budget, err := r.rolloutBudget(t.Context(), ng, p)
	require.NoError(t, err)
	require.Empty(t, budget.updating, "a document no agent has reported on holds no slot")

	for _, node := range []*corev1.Node{&fresh, &stale} {
		_, err = r.apply(t.Context(), ng, node, newNodeConfig(ng, node, renderMutableSpec(node, ng.Name, clusterInputs{})), logr.Discard(), p)
		require.NoError(t, err)
		require.Empty(t, budget.updating, "handing %s a document no agent is there to take spends no slot", node.Name)
	}
	rewritten := &internalv1alpha1.NodeConfig{}
	require.NoError(t, cl.Get(t.Context(), types.NamespacedName{Name: stale.Name}, rewritten))
	require.Empty(t, rewritten.Spec.RegistryPackagesProxy.AccessToken, "the stale document is rewritten")
}

// A node registers its NodeConfig by name alone. With no slot free its labels and owner
// are written anyway, or the gate never sees it and nothing removes it with its node;
// that write hands the node nothing, so it spends no slot.
func TestAHeldNodeConfigIsLabelledWithoutSpendingASlot(t *testing.T) {
	ng := mutableGroup("worker", v1.SystemTypeMutable)
	holder := nodeInGroup("worker-0", "worker")
	joiner := nodeInGroup("worker-1", "worker")
	joiner.UID = "worker-1-uid"
	updating := &internalv1alpha1.NodeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: holder.Name, Generation: 1,
			Labels: map[string]string{nodecommon.NodeGroupLabel: ng.Name}},
		Spec: renderMutableSpec(&holder, ng.Name, clusterInputs{}),
	}
	updating.Status.ObservedGeneration = 1
	registered := &internalv1alpha1.NodeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: joiner.Name},
		Spec:       renderMutableSpec(&joiner, ng.Name, clusterInputs{RegistryPackagesProxyToken: []byte("seed-token")}),
	}
	cl := fake.NewClientBuilder().WithScheme(nsprStatusScheme(t)).WithObjects(ng, &holder, &joiner, updating, registered).Build()
	r, p := mutableReconciler(cl, clusterInputs{})
	desired := newNodeConfig(ng, &joiner, renderMutableSpec(&joiner, ng.Name, clusterInputs{}))

	current, err := r.apply(t.Context(), ng, &joiner, desired, logr.Discard(), p)
	require.NoError(t, err)
	require.Equal(t, []string{holder.Name}, p.rollouts[ng.Name].holders(), "the held node took a slot")

	stored := &internalv1alpha1.NodeConfig{}
	require.NoError(t, cl.Get(t.Context(), types.NamespacedName{Name: joiner.Name}, stored))
	require.Equal(t, desired.Labels, stored.Labels)
	require.Equal(t, desired.OwnerReferences, stored.OwnerReferences)
	require.Equal(t, registered.Spec, stored.Spec, "the spec was written without a slot")
	require.Equal(t, stored.ResourceVersion, current.ResourceVersion, "the object is returned as it now stands")

	_, next := mutableReconciler(cl, clusterInputs{})
	budget, err := r.rolloutBudget(t.Context(), ng, next)
	require.NoError(t, err)
	require.Equal(t, []string{holder.Name}, budget.holders(), "the held node, now listed, is not updating")
}

// A held node that reported on an older spec is not mid-rollout: labelled once the pass has
// read its group's budget, it is not counted there, and the only slot stays the holder's.
func TestAHeldNodeOnAnOlderSpecIsNotCountedWhenLabelled(t *testing.T) {
	ng := mutableGroup("worker", v1.SystemTypeMutable)
	holder := nodeInGroup("worker-0", "worker")
	joiner := nodeInGroup("worker-1", "worker")
	joiner.UID = "worker-1-uid"
	updating := notApplied(holder.Name, renderMutableSpec(&holder, ng.Name, clusterInputs{}))
	updating.Labels = map[string]string{nodecommon.NodeGroupLabel: ng.Name}
	older := notApplied(joiner.Name, renderMutableSpec(&joiner, ng.Name, clusterInputs{RegistryPackagesProxyToken: []byte("previous-token")}))
	cl := fake.NewClientBuilder().WithScheme(nsprStatusScheme(t)).WithObjects(ng, &holder, &joiner, &updating, &older).Build()
	r, p := mutableReconciler(cl, clusterInputs{})
	budget, err := r.rolloutBudget(t.Context(), ng, p)
	require.NoError(t, err)

	desired := newNodeConfig(ng, &joiner, renderMutableSpec(&joiner, ng.Name, clusterInputs{}))
	_, err = r.apply(t.Context(), ng, &joiner, desired, logr.Discard(), p)
	require.NoError(t, err)

	stored := &internalv1alpha1.NodeConfig{}
	require.NoError(t, cl.Get(t.Context(), types.NamespacedName{Name: joiner.Name}, stored))
	require.Equal(t, desired.Labels, stored.Labels)
	require.Equal(t, []string{holder.Name}, budget.holders())
}

// A node running its current spec whose labels or owner drifted is handed nothing new: writing
// them through the spec patch spent a free slot for the rest of the pass. A node still applying
// that spec holds its slot, though the budget was read before the node was labelled.
func TestAMetadataOnlyChangeSpendsNoFreeSlot(t *testing.T) {
	for _, tc := range []struct {
		name        string
		applied     bool
		wantHolders []string
	}{
		{name: "running its spec", applied: true},
		{name: "still applying its spec", wantHolders: []string{"worker-0"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ng := mutableGroup("worker", v1.SystemTypeMutable)
			node := nodeInGroup("worker-0", "worker")
			node.UID = "worker-0-uid"
			spec := renderMutableSpec(&node, ng.Name, clusterInputs{})
			unlabelled := &internalv1alpha1.NodeConfig{
				ObjectMeta: metav1.ObjectMeta{Name: node.Name, Generation: 1},
				Spec:       spec,
			}
			unlabelled.Status.ObservedGeneration = 1
			if tc.applied {
				unlabelled.Status.AppliedGeneration = 1
				unlabelled.Status.Conditions = []metav1.Condition{{
					Type: internalv1alpha1.NodeConfigConditionTypeConfigurationApplied, Status: metav1.ConditionTrue, ObservedGeneration: 1, Reason: "Applied",
				}}
			}
			cl := fake.NewClientBuilder().WithScheme(nsprStatusScheme(t)).WithObjects(ng, &node, unlabelled).Build()
			r, p := mutableReconciler(cl, clusterInputs{})
			budget, err := r.rolloutBudget(t.Context(), ng, p)
			require.NoError(t, err)
			require.Empty(t, budget.holders(), "an unlabelled NodeConfig is not listed")

			desired := newNodeConfig(ng, &node, spec)
			_, err = r.apply(t.Context(), ng, &node, desired, logr.Discard(), p)
			require.NoError(t, err)

			stored := &internalv1alpha1.NodeConfig{}
			require.NoError(t, cl.Get(t.Context(), types.NamespacedName{Name: node.Name}, stored))
			require.Equal(t, desired.Labels, stored.Labels)
			require.Equal(t, desired.OwnerReferences, stored.OwnerReferences)
			require.Equal(t, tc.wantHolders, budget.holders())
			require.Equal(t, len(tc.wantHolders) == 0, budget.hasSlot("worker-1"), "another node of the group")
		})
	}
}

// The exception is bashible's: an Engine node is installed with its agent, so its silence
// is a fault and still holds the rollout.
func TestAnEngineNodeWithoutAnAgentHoldsItsSlot(t *testing.T) {
	desired := internalv1alpha1.NodeSpec{NodeName: "n1", OSImage: internalv1alpha1.OSImage{Digest: testOSImageDigest}}
	silent := internalv1alpha1.NodeConfig{ObjectMeta: metav1.ObjectMeta{Name: "n1", Generation: 1}, Spec: desired}

	updating := membersOfUpdating([]internalv1alpha1.NodeConfig{silent}, map[string]internalv1alpha1.NodeSpec{"n1": desired}, false)

	require.Contains(t, updating, "n1")
}

// The exception ends where the agent speaks: whichever part of the status it reported
// first, a bashible node whose agent answered is mid-rollout and holds its slot.
func TestABashibleNodeWhoseAgentReportedHoldsItsSlot(t *testing.T) {
	desired := internalv1alpha1.NodeSpec{NodeName: "n1", SystemType: internalv1alpha1.SystemTypeMutable}
	observedOlder := internalv1alpha1.NodeConfig{ObjectMeta: metav1.ObjectMeta{Name: "n1", Generation: 2}, Spec: desired}
	observedOlder.Status.ObservedGeneration = 1
	conditionsOnly := internalv1alpha1.NodeConfig{ObjectMeta: metav1.ObjectMeta{Name: "n1", Generation: 2}, Spec: desired}
	meta.SetStatusCondition(&conditionsOnly.Status.Conditions, metav1.Condition{
		Type: internalv1alpha1.NodeConfigConditionTypeConfigurationApplied, Status: metav1.ConditionFalse, Reason: "Applying",
	})

	for name, nc := range map[string]internalv1alpha1.NodeConfig{
		"observed an older generation": observedOlder,
		"reported conditions only":     conditionsOnly,
	} {
		t.Run(name, func(t *testing.T) {
			updating := membersOfUpdating([]internalv1alpha1.NodeConfig{nc}, map[string]internalv1alpha1.NodeSpec{"n1": desired}, true)

			require.Contains(t, updating, "n1")
		})
	}
}

// The request statuses name a NodeOperation as waited for or failed, and its phase is all
// they follow: an operation that fails before the node is let go moves nothing on the node.
func TestOperationPhaseChanged(t *testing.T) {
	op := &deckhousev1alpha1.NodeOperation{Status: deckhousev1alpha1.NodeOperationStatus{Phase: deckhousev1alpha1.NodeOperationPhaseInProgress}}
	failed := op.DeepCopy()
	failed.Status.Phase = deckhousev1alpha1.NodeOperationPhaseFailed
	reworded := op.DeepCopy()
	reworded.Status.Message = "the eviction of node worker-0 is failing"

	require.True(t, operationPhaseChanged(op, failed))
	require.False(t, operationPhaseChanged(op, reworded))
}
