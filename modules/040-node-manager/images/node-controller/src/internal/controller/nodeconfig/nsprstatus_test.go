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
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

func nsprStatusScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))
	require.NoError(t, deckhousev1alpha1.AddToScheme(scheme))
	require.NoError(t, internalv1alpha1.AddToScheme(scheme))
	// The pass lists Nodes for the denominator of the counts it publishes.
	require.NoError(t, corev1.AddToScheme(scheme))
	return scheme
}

func immutableGroup(name string) *v1.NodeGroup {
	return &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral, SystemType: v1.SystemTypeImmutable},
	}
}

// The status is the only thing an operator reads about an object that reaches
// nodes they cannot log in to.
func TestNSPRStatusReportsResolutionAndCounts(t *testing.T) {
	object := nspr("registry-agent", deckhousev1alpha1.NodeStaticPodRequestSpec{})
	cl := fake.NewClientBuilder().
		WithScheme(nsprStatusScheme(t)).
		WithObjects(
			&object,
			immutableGroup("worker"),
			nodeConfigWithPod("worker-0",
				[]internalv1alpha1.StaticPod{{Name: "registry-agent", Manifest: podManifest("registry-agent")}},
				[]internalv1alpha1.StaticPodStatus{{Name: "registry-agent", State: "Written"}}),
		).
		WithStatusSubresource(&deckhousev1alpha1.NodeStaticPodRequest{}).
		Build()

	r := &Reconciler{}
	r.Client = cl
	require.NoError(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard()))

	fresh := &deckhousev1alpha1.NodeStaticPodRequest{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "registry-agent"}, fresh))
	require.Equal(t, phaseReady, fresh.Status.Phase)
	require.Equal(t, int32(1), fresh.Status.AppliedNodes)
	require.Equal(t, int32(0), fresh.Status.FailedNodes)
	require.Equal(t, []string{"worker"}, fresh.Status.MatchedNodeGroups)
	require.True(t, meta.IsStatusConditionTrue(fresh.Status.Conditions, readyConditionType))
	require.Equal(t, reasonResolved, meta.FindStatusCondition(fresh.Status.Conditions, readyConditionType).Reason)
}

// A manifest this controller refused never reaches a node, so nothing but its
// own status can say why.
func TestNSPRStatusReportsTheRefusal(t *testing.T) {
	broken := nspr("broken", deckhousev1alpha1.NodeStaticPodRequestSpec{Manifest: "apiVersion: apps/v1\nkind: Deployment\n"})
	cl := fake.NewClientBuilder().
		WithScheme(nsprStatusScheme(t)).
		WithObjects(&broken, immutableGroup("worker")).
		WithStatusSubresource(&deckhousev1alpha1.NodeStaticPodRequest{}).
		Build()

	r := &Reconciler{}
	r.Client = cl
	require.NoError(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard()))

	fresh := &deckhousev1alpha1.NodeStaticPodRequest{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "broken"}, fresh))
	require.Equal(t, phaseDegraded, fresh.Status.Phase)
	condition := meta.FindStatusCondition(fresh.Status.Conditions, readyConditionType)
	require.Equal(t, metav1.ConditionFalse, condition.Status)
	require.Equal(t, reasonInvalidManifest, condition.Reason)
}

// It resolved here and the nodes refused it: reporting Ready on the strength of
// the resolution alone is what made a broken static pod indistinguishable from a
// working one.
func TestNSPRStatusReportsWhatTheNodesRefused(t *testing.T) {
	object := nspr("registry-agent", deckhousev1alpha1.NodeStaticPodRequestSpec{})
	cl := fake.NewClientBuilder().
		WithScheme(nsprStatusScheme(t)).
		WithObjects(
			&object,
			immutableGroup("worker"),
			nodeConfigWithPod("worker-0",
				[]internalv1alpha1.StaticPod{{Name: "registry-agent", Manifest: podManifest("registry-agent")}},
				[]internalv1alpha1.StaticPodStatus{{
					Name: "registry-agent", State: "Failed",
					Reason: "WriteFailed", Message: "read-only file system",
				}}),
		).
		WithStatusSubresource(&deckhousev1alpha1.NodeStaticPodRequest{}).
		Build()

	r := &Reconciler{}
	r.Client = cl
	require.NoError(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard()))

	fresh := &deckhousev1alpha1.NodeStaticPodRequest{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "registry-agent"}, fresh))
	require.Equal(t, phaseDegraded, fresh.Status.Phase)
	require.Equal(t, int32(1), fresh.Status.FailedNodes)
	require.Equal(t, reasonRefusedByNodes,
		meta.FindStatusCondition(fresh.Status.Conditions, readyConditionType).Reason)
	// Reason first: which of the three refusals it was decides whether the
	// operator edits this object or goes and looks at the node.
	require.Contains(t, fresh.Status.FailureMessage, "WriteFailed: read-only file system")
}

// A pass that cannot read the fleet knows nothing about it, and zero counts are
// not "nothing": publishing them turns "every node refused this pod" into a
// clean Ready with the reason erased, on one unlucky listing.
func TestNSPRStatusIsNotPublishedFromAFleetThatCouldNotBeRead(t *testing.T) {
	object := nspr("registry-agent", deckhousev1alpha1.NodeStaticPodRequestSpec{})
	object.Status = deckhousev1alpha1.NodeStaticPodRequestStatus{
		Phase:          phaseDegraded,
		FailedNodes:    50,
		FailureMessage: "read-only file system",
	}
	cl := fake.NewClientBuilder().
		WithScheme(nsprStatusScheme(t)).
		WithObjects(&object, immutableGroup("worker")).
		WithStatusSubresource(&deckhousev1alpha1.NodeStaticPodRequest{}).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				if _, fleet := list.(*internalv1alpha1.NodeConfigList); fleet {
					return apierrors.NewServiceUnavailable("etcd leader changed")
				}
				return c.List(ctx, list, opts...)
			},
		}).
		Build()

	r := &Reconciler{}
	r.Client = cl
	require.Error(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard()),
		"the pass has to be retried, not counted as a successful report")

	fresh := &deckhousev1alpha1.NodeStaticPodRequest{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "registry-agent"}, fresh))
	require.Equal(t, int32(50), fresh.Status.FailedNodes)
	require.Equal(t, "read-only file system", fresh.Status.FailureMessage)
}

// Nobody has reported yet is Ready with no applied nodes, not Degraded: painting
// every object red for a minute teaches an operator to ignore the colour. And an
// identical status is not rewritten: each write wakes every watcher of the kind.
func TestNSPRStatusSettlesAndIsNotRewritten(t *testing.T) {
	object := nspr("registry-agent", deckhousev1alpha1.NodeStaticPodRequestSpec{})
	cl := fake.NewClientBuilder().
		WithScheme(nsprStatusScheme(t)).
		WithObjects(&object, immutableGroup("worker")).
		WithStatusSubresource(&deckhousev1alpha1.NodeStaticPodRequest{}).
		Build()

	r := &Reconciler{}
	r.Client = cl
	require.NoError(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard()))

	written := &deckhousev1alpha1.NodeStaticPodRequest{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "registry-agent"}, written))
	require.Equal(t, written.Generation, written.Status.ObservedGeneration)
	require.Equal(t, phaseReady, written.Status.Phase)
	require.Equal(t, int32(0), written.Status.AppliedNodes)
	require.Equal(t, reasonResolved, meta.FindStatusCondition(written.Status.Conditions, readyConditionType).Reason)

	require.NoError(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard()))
	again := &deckhousev1alpha1.NodeStaticPodRequest{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "registry-agent"}, again))
	require.Equal(t, written.ResourceVersion, again.ResourceVersion, "an unchanged status must not be patched")
}

// A count with no denominator answers nothing: 3 applied out of how many?
func TestTheStatusSaysHowManyNodesWereAskedAndHowManyHaveNotAnswered(t *testing.T) {
	nodes := []corev1.Node{
		nodeInGroup("worker-0", "worker"), nodeInGroup("worker-1", "worker"),
		nodeInGroup("worker-2", "worker"), nodeInGroup("master-0", "master"),
	}
	if got := matchedNodeCount(nodes, []string{"worker"}); got != 3 {
		t.Fatalf("matchedNodeCount = %d, want 3", got)
	}
	if got := pendingNodeCount(3, 1, 1); got != 1 {
		t.Fatalf("pendingNodeCount = %d, want 1", got)
	}
	// A node reports before the node list catches up: never a negative count.
	if got := pendingNodeCount(1, 2, 0); got != 0 {
		t.Fatalf("pendingNodeCount = %d, want 0", got)
	}
}

func nodeInGroup(name, group string) corev1.Node {
	return corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name,
		Labels: map[string]string{nodecommon.NodeGroupLabel: group}}}
}

// A static pod reaches bashible groups too, and after task 10 their nodes report
// through a NodeConfig like every other: the denominator counts them, and a node
// yet to answer is pending rather than missing.
func TestNSPRStatusCountsTheNodesOfEveryMatchedGroup(t *testing.T) {
	object := nspr("registry-agent", deckhousev1alpha1.NodeStaticPodRequestSpec{
		NodeGroupSelector: deckhousev1alpha1.NodeGroupSelector{MatchNames: []string{"mutable-workers"}},
	})
	mutableNode := nodeInGroup("mutable-0", "mutable-workers")
	pendingNode := nodeInGroup("mutable-1", "mutable-workers")
	otherNode := nodeInGroup("worker-0", "worker")
	cl := fake.NewClientBuilder().
		WithScheme(nsprStatusScheme(t)).
		WithObjects(
			&object,
			immutableGroup("worker"),
			&v1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "mutable-workers"},
				Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral, SystemType: v1.SystemTypeMutable},
			},
			&mutableNode, &pendingNode, &otherNode,
			nodeConfigWithPod("mutable-0",
				[]internalv1alpha1.StaticPod{{Name: "registry-agent", Manifest: podManifest("registry-agent")}},
				[]internalv1alpha1.StaticPodStatus{{Name: "registry-agent", State: "Written"}}),
		).
		WithStatusSubresource(&deckhousev1alpha1.NodeStaticPodRequest{}).
		Build()

	r := &Reconciler{}
	r.Client = cl
	require.NoError(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard()))

	fresh := &deckhousev1alpha1.NodeStaticPodRequest{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "registry-agent"}, fresh))
	require.Equal(t, []string{"mutable-workers"}, fresh.Status.MatchedNodeGroups)
	require.Equal(t, int32(2), fresh.Status.MatchedNodes, "the node of the group it does not select is not a denominator")
	require.Equal(t, int32(1), fresh.Status.AppliedNodes)
	require.Equal(t, int32(1), fresh.Status.PendingNodes)
	require.Contains(t, meta.FindStatusCondition(fresh.Status.Conditions, readyConditionType).Message, "1 of 2 node(s)")
}

// An object this controller refused was handed to no node, so nobody is late
// with an answer: "Degraded, 2 nodes have not answered" sends an operator
// looking at nodes that were never asked.
func TestNSPRStatusRefusedHereLeavesNothingPending(t *testing.T) {
	broken := nspr("broken", deckhousev1alpha1.NodeStaticPodRequestSpec{Manifest: "apiVersion: apps/v1\nkind: Deployment\n"})
	first := nodeInGroup("worker-0", "worker")
	second := nodeInGroup("worker-1", "worker")
	cl := fake.NewClientBuilder().
		WithScheme(nsprStatusScheme(t)).
		WithObjects(&broken, immutableGroup("worker"), &first, &second).
		WithStatusSubresource(&deckhousev1alpha1.NodeStaticPodRequest{}).
		Build()

	r := &Reconciler{}
	r.Client = cl
	require.NoError(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard()))

	fresh := &deckhousev1alpha1.NodeStaticPodRequest{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "broken"}, fresh))
	require.Equal(t, int32(0), fresh.Status.PendingNodes, "nothing was handed to a node")
	require.Equal(t, int32(2), fresh.Status.MatchedNodes, "how many nodes the selector covers is still the answer")
}

// The nodes did get this one, so the arithmetic stands: zeroing pending on every
// Degraded object would lose the node that has not reported yet.
func TestNSPRStatusRefusedByTheNodesKeepsItsArithmetic(t *testing.T) {
	object := nspr("registry-agent", deckhousev1alpha1.NodeStaticPodRequestSpec{})
	first := nodeInGroup("worker-0", "worker")
	second := nodeInGroup("worker-1", "worker")
	silent := nodeInGroup("worker-2", "worker")
	cl := fake.NewClientBuilder().
		WithScheme(nsprStatusScheme(t)).
		WithObjects(
			&object, immutableGroup("worker"), &first, &second, &silent,
			nodeConfigWithPod("worker-0",
				[]internalv1alpha1.StaticPod{{Name: "registry-agent", Manifest: podManifest("registry-agent")}},
				[]internalv1alpha1.StaticPodStatus{{Name: "registry-agent", State: "Written"}}),
			nodeConfigWithPod("worker-1",
				[]internalv1alpha1.StaticPod{{Name: "registry-agent", Manifest: podManifest("registry-agent")}},
				[]internalv1alpha1.StaticPodStatus{{
					Name: "registry-agent", State: "Failed",
					Reason: "WriteFailed", Message: "read-only file system",
				}}),
		).
		WithStatusSubresource(&deckhousev1alpha1.NodeStaticPodRequest{}).
		Build()

	r := &Reconciler{}
	r.Client = cl
	require.NoError(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard()))

	fresh := &deckhousev1alpha1.NodeStaticPodRequest{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "registry-agent"}, fresh))
	require.Equal(t, reasonRefusedByNodes,
		meta.FindStatusCondition(fresh.Status.Conditions, readyConditionType).Reason)
	require.Equal(t, int32(1), fresh.Status.PendingNodes, "the third node still owes an answer")
}
