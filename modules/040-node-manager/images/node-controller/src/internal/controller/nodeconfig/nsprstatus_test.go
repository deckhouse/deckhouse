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
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
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
	node := nodeInGroup("worker-0", "worker")
	cl := fake.NewClientBuilder().
		WithScheme(nsprStatusScheme(t)).
		WithObjects(
			&object,
			immutableGroup("worker"),
			&node,
			nodeConfigWithPod("worker-0",
				[]internalv1alpha1.StaticPod{{Name: "registry-agent", Manifest: podManifest("registry-agent")}},
				[]internalv1alpha1.StaticPodStatus{writtenPod("registry-agent", podManifest("registry-agent"))}),
		).
		WithStatusSubresource(&deckhousev1alpha1.NodeStaticPodRequest{}).
		Build()

	r := &Reconciler{}
	r.Client = cl
	require.NoError(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard(), newPass()))

	fresh := &deckhousev1alpha1.NodeStaticPodRequest{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "registry-agent"}, fresh))
	require.Equal(t, phaseReady, fresh.Status.Phase)
	require.Equal(t, int32(1), fresh.Status.AppliedNodes)
	require.Equal(t, int32(0), fresh.Status.FailedNodes)
	require.Equal(t, []string{"worker"}, fresh.Status.MatchedNodeGroups)
	require.True(t, meta.IsStatusConditionTrue(fresh.Status.Conditions, acceptedConditionType))
	require.True(t, meta.IsStatusConditionTrue(fresh.Status.Conditions, readyConditionType))
	ready := meta.FindStatusCondition(fresh.Status.Conditions, readyConditionType)
	require.Equal(t, reasonAllNodesApplied, ready.Reason)
	require.Equal(t, "1 of 1 node(s) wrote the current manifest", ready.Message)
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
	require.NoError(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard(), newPass()))

	fresh := &deckhousev1alpha1.NodeStaticPodRequest{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "broken"}, fresh))
	require.Equal(t, phaseDegraded, fresh.Status.Phase)
	condition := meta.FindStatusCondition(fresh.Status.Conditions, acceptedConditionType)
	require.Equal(t, metav1.ConditionFalse, condition.Status)
	require.Equal(t, reasonInvalidManifest, condition.Reason)
	require.Equal(t, reasonNotAccepted, meta.FindStatusCondition(fresh.Status.Conditions, readyConditionType).Reason)
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
	require.NoError(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard(), newPass()))

	fresh := &deckhousev1alpha1.NodeStaticPodRequest{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "registry-agent"}, fresh))
	require.Equal(t, phaseDegraded, fresh.Status.Phase)
	require.Equal(t, int32(1), fresh.Status.FailedNodes)
	ready := meta.FindStatusCondition(fresh.Status.Conditions, readyConditionType)
	require.Equal(t, reasonRefusedByNodes, ready.Reason)
	// Reason first: which of the three refusals it was decides whether the
	// operator edits this object or goes and looks at the node.
	require.Contains(t, ready.Message, "WriteFailed: read-only file system")
}

// A pass that cannot read the fleet knows nothing about it, and zero counts are
// not "nothing": publishing them turns "every node refused this pod" into a
// clean Ready with the reason erased, on one unlucky listing.
func TestNSPRStatusIsNotPublishedFromAFleetThatCouldNotBeRead(t *testing.T) {
	object := nspr("registry-agent", deckhousev1alpha1.NodeStaticPodRequestSpec{})
	object.Status = deckhousev1alpha1.NodeStaticPodRequestStatus{
		Phase:       phaseDegraded,
		FailedNodes: 50,
		Conditions: []metav1.Condition{{
			Type: readyConditionType, Status: metav1.ConditionFalse, Reason: reasonRefusedByNodes,
			Message: "50 of 50 node(s) refused the static pod, 0 wrote it: read-only file system", LastTransitionTime: metav1.Now(),
		}},
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
	require.Error(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard(), newPass()),
		"the pass has to be retried, not counted as a successful report")

	fresh := &deckhousev1alpha1.NodeStaticPodRequest{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "registry-agent"}, fresh))
	require.Equal(t, int32(50), fresh.Status.FailedNodes)
	require.Contains(t, meta.FindStatusCondition(fresh.Status.Conditions, readyConditionType).Message, "read-only file system")
}

// Nobody has reported yet is Pending, not Degraded: painting every object red for a
// minute teaches an operator to ignore the colour. And an identical status is not
// rewritten: each write wakes every watcher of the kind.
func TestNSPRStatusSettlesAndIsNotRewritten(t *testing.T) {
	object := nspr("registry-agent", deckhousev1alpha1.NodeStaticPodRequestSpec{})
	node := nodeInGroup("worker-0", "worker")
	cl := fake.NewClientBuilder().
		WithScheme(nsprStatusScheme(t)).
		WithObjects(&object, immutableGroup("worker"), &node).
		WithStatusSubresource(&deckhousev1alpha1.NodeStaticPodRequest{}).
		Build()

	r := &Reconciler{}
	r.Client = cl
	require.NoError(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard(), newPass()))

	written := &deckhousev1alpha1.NodeStaticPodRequest{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "registry-agent"}, written))
	require.Equal(t, written.Generation, written.Status.ObservedGeneration)
	require.Equal(t, phasePending, written.Status.Phase)
	require.Equal(t, int32(0), written.Status.AppliedNodes)
	require.Equal(t, reasonRolloutInProgress, meta.FindStatusCondition(written.Status.Conditions, readyConditionType).Reason)

	require.NoError(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard(), newPass()))
	again := &deckhousev1alpha1.NodeStaticPodRequest{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "registry-agent"}, again))
	require.Equal(t, written.ResourceVersion, again.ResourceVersion, "an unchanged status must not be patched")
}

// A merge patch carries only what differs from its base, and a counter that was
// zero and stays zero never differs: without all four in the write, a request's
// first status says nothing in the columns where it should say 0.
func TestTheStatusWriteCarriesEveryCounter(t *testing.T) {
	object := nspr("static-pod", deckhousev1alpha1.NodeStaticPodRequestSpec{})
	var patches []string
	cl := fake.NewClientBuilder().
		WithScheme(nsprStatusScheme(t)).
		WithObjects(&object, immutableGroup("worker")).
		WithStatusSubresource(&deckhousev1alpha1.NodeStaticPodRequest{}).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourcePatch: func(ctx context.Context, c client.Client, subResource string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
				data, err := patch.Data(obj)
				require.NoError(t, err)
				patches = append(patches, string(data))
				return c.SubResource(subResource).Patch(ctx, obj, patch, opts...)
			},
		}).
		Build()

	r := &Reconciler{}
	r.Client = cl
	require.NoError(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard(), newPass()))

	require.Len(t, patches, 1)
	for _, counter := range []string{"matchedNodes", "appliedNodes", "failedNodes", "pendingNodes"} {
		require.Contains(t, patches[0], `"`+counter+`":0`)
	}
}

// The status is written whole, so a field that went empty is gone: a group the
// selector no longer matches must not stay listed.
func TestTheStatusWriteClearsWhatWentEmpty(t *testing.T) {
	object := nspr("static-pod", deckhousev1alpha1.NodeStaticPodRequestSpec{})
	object.Status = deckhousev1alpha1.NodeStaticPodRequestStatus{
		Phase: phaseDegraded, FailedNodes: 1, MatchedNodeGroups: []string{"gone"},
	}
	node := nodeInGroup("worker-0", "worker")
	cl := fake.NewClientBuilder().
		WithScheme(nsprStatusScheme(t)).
		WithObjects(&object, immutableGroup("worker"), &node,
			nodeConfigWithPod("worker-0",
				[]internalv1alpha1.StaticPod{{Name: "static-pod", Manifest: podManifest("static-pod")}},
				[]internalv1alpha1.StaticPodStatus{writtenPod("static-pod", podManifest("static-pod"))})).
		WithStatusSubresource(&deckhousev1alpha1.NodeStaticPodRequest{}).
		Build()

	r := &Reconciler{}
	r.Client = cl
	require.NoError(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard(), newPass()))

	fresh := &deckhousev1alpha1.NodeStaticPodRequest{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "static-pod"}, fresh))
	require.Equal(t, []string{"worker"}, fresh.Status.MatchedNodeGroups)
	require.Equal(t, int32(0), fresh.Status.FailedNodes)
}

// A count with no denominator answers nothing: 3 applied out of how many?
func TestTheStatusSaysHowManyNodesWereAskedAndHowManyHaveNotAnswered(t *testing.T) {
	nodes := []corev1.Node{
		nodeInGroup("worker-0", "worker"), nodeInGroup("worker-1", "worker"),
		nodeInGroup("worker-2", "worker"), nodeInGroup("master-0", "master"),
	}
	if got := matchedNodes(nodes, []string{"worker"}); len(got) != 3 {
		t.Fatalf("matchedNodes = %v, want the 3 workers", got)
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

// Bashible nodes run the agent too, so an empty selector reaches every group,
// and a group that names no type is a bashible one.
func TestNSPRStatusCountsTheNodesOfEveryGroup(t *testing.T) {
	object := nspr("static-pod", deckhousev1alpha1.NodeStaticPodRequestSpec{})
	engineNode := nodeInGroup("engine-0", "engine")
	pendingNode := nodeInGroup("engine-1", "engine")
	bashibleNode := nodeInGroup("worker-0", "worker")
	legacyNode := nodeInGroup("legacy-0", "legacy")
	written := []internalv1alpha1.StaticPodStatus{writtenPod("static-pod", podManifest("static-pod"))}
	pods := []internalv1alpha1.StaticPod{{Name: "static-pod", Manifest: podManifest("static-pod")}}
	cl := fake.NewClientBuilder().
		WithScheme(nsprStatusScheme(t)).
		WithObjects(
			&object,
			immutableGroup("engine"),
			mutableGroup("worker", v1.SystemTypeMutable),
			mutableGroup("legacy", ""),
			&engineNode, &pendingNode, &bashibleNode, &legacyNode,
			nodeConfigWithPod("engine-0", pods, written),
			nodeConfigWithPod("worker-0", pods, written),
		).
		WithStatusSubresource(&deckhousev1alpha1.NodeStaticPodRequest{}).
		Build()

	r := &Reconciler{}
	r.Client = cl
	require.NoError(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard(), newPass()))

	fresh := &deckhousev1alpha1.NodeStaticPodRequest{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "static-pod"}, fresh))
	require.Equal(t, []string{"engine", "legacy", "worker"}, fresh.Status.MatchedNodeGroups)
	require.Equal(t, int32(4), fresh.Status.MatchedNodes)
	require.Equal(t, int32(2), fresh.Status.AppliedNodes)
	require.Equal(t, int32(2), fresh.Status.PendingNodes)
}

// registry-agent is bashible's on the nodes it manages (step 053), whether an
// object asks for it by file name or by pod: their groups match the selector,
// but nothing is asked of their nodes, so those are neither counted nor waited for.
func TestNSPRStatusLeavesTheRegistryAgentToBashible(t *testing.T) {
	byName := nspr("registry-agent", deckhousev1alpha1.NodeStaticPodRequestSpec{})
	byPod := nspr("agent-copy", deckhousev1alpha1.NodeStaticPodRequestSpec{Manifest: kubeSystemPodManifest("registry-agent")})
	engineNode := nodeInGroup("engine-0", "engine")
	bashibleNode := nodeInGroup("worker-0", "worker")
	cl := fake.NewClientBuilder().
		WithScheme(nsprStatusScheme(t)).
		WithObjects(
			&byName, &byPod,
			immutableGroup("engine"),
			mutableGroup("worker", v1.SystemTypeMutable),
			&engineNode, &bashibleNode,
		).
		WithStatusSubresource(&deckhousev1alpha1.NodeStaticPodRequest{}).
		Build()

	r := &Reconciler{}
	r.Client = cl
	require.NoError(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard(), newPass()))

	for _, name := range []string{"registry-agent", "agent-copy"} {
		fresh := &deckhousev1alpha1.NodeStaticPodRequest{}
		require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: name}, fresh))
		require.Equal(t, []string{"engine", "worker"}, fresh.Status.MatchedNodeGroups, name)
		require.Equal(t, int32(1), fresh.Status.MatchedNodes, name)
		require.Equal(t, int32(1), fresh.Status.PendingNodes, name)
	}
}

// The cap counts what a node is given. A bashible node is not given registry-agent,
// so it must not push the youngest of sixteen others out of that group's configs.
func TestTheCapOnABashibleGroupLeavesTheRegistryAgentOut(t *testing.T) {
	objects := []client.Object{mutableGroup("worker", v1.SystemTypeMutable)}
	for i := range maxStaticPods {
		object := nspr(fmt.Sprintf("pod-%02d", i), deckhousev1alpha1.NodeStaticPodRequestSpec{})
		objects = append(objects, &object)
	}
	agent := nspr("registry-agent", deckhousev1alpha1.NodeStaticPodRequestSpec{})
	objects = append(objects, &agent)
	cl := fake.NewClientBuilder().
		WithScheme(nsprStatusScheme(t)).
		WithObjects(objects...).
		WithStatusSubresource(&deckhousev1alpha1.NodeStaticPodRequest{}).
		Build()

	r := &Reconciler{}
	r.Client = cl
	require.NoError(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard(), newPass()))

	for _, name := range []string{fmt.Sprintf("pod-%02d", maxStaticPods-1), "registry-agent"} {
		fresh := &deckhousev1alpha1.NodeStaticPodRequest{}
		require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: name}, fresh))
		condition := meta.FindStatusCondition(fresh.Status.Conditions, readyConditionType)
		require.NotEqual(t, reasonLimitExceeded, condition.Reason, name)
	}
}

func mutableGroup(name string, systemType v1.SystemType) *v1.NodeGroup {
	return &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral, SystemType: systemType},
	}
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
	require.NoError(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard(), newPass()))

	fresh := &deckhousev1alpha1.NodeStaticPodRequest{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "broken"}, fresh))
	require.Equal(t, int32(0), fresh.Status.PendingNodes, "nothing was handed to a node")
	require.Empty(t, fresh.Status.Nodes, "no node owes an answer")
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
				[]internalv1alpha1.StaticPodStatus{writtenPod("registry-agent", podManifest("registry-agent"))}),
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
	require.NoError(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard(), newPass()))

	fresh := &deckhousev1alpha1.NodeStaticPodRequest{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "registry-agent"}, fresh))
	require.Equal(t, reasonRefusedByNodes,
		meta.FindStatusCondition(fresh.Status.Conditions, readyConditionType).Reason)
	require.Equal(t, int32(1), fresh.Status.PendingNodes, "the third node still owes an answer")
}

// Every node of a classic cluster now goes through readMutableInputs, which
// fails whole on one bad read of the kubernetes endpoints or the proxy-token
// Secret. Returning on the first such node froze every request's status in the
// cluster, and the statuses read their own inputs anyway.
func TestARenderFailureDoesNotFreezeTheRequestStatuses(t *testing.T) {
	object := nspr("registry-agent", deckhousev1alpha1.NodeStaticPodRequestSpec{})
	node := nodeInGroup("worker-0", "worker")
	cl := fake.NewClientBuilder().
		WithScheme(nsprStatusScheme(t)).
		WithObjects(&object, immutableGroup("worker"), &node).
		WithStatusSubresource(&deckhousev1alpha1.NodeStaticPodRequest{}).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				if _, group := obj.(*v1.NodeGroup); group {
					return apierrors.NewServiceUnavailable("etcd leader changed")
				}
				return c.Get(ctx, key, obj, opts...)
			},
		}).
		Build()

	r := &Reconciler{}
	r.Client = cl
	_, err := r.reconcileAllNodes(context.Background(), logr.Discard())
	require.ErrorContains(t, err, "render the NodeConfig", "the render failure is still the pass's to retry")

	fresh := &deckhousev1alpha1.NodeStaticPodRequest{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "registry-agent"}, fresh))
	require.Equal(t, phasePending, fresh.Status.Phase, "the status reads its own inputs and they were readable")
	require.Equal(t, int32(1), fresh.Status.MatchedNodes)
}

// An object the per-group cap left out never reaches a node, so it must not
// read Ready with "0 of N written" and the reason buried in a log line.
func TestNSPRStatusReportsTheObjectTheCapLeftOut(t *testing.T) {
	objects := []client.Object{immutableGroup("engine")}
	for i := range maxStaticPods + 1 {
		object := nspr(fmt.Sprintf("pod-%02d", i), deckhousev1alpha1.NodeStaticPodRequestSpec{})
		objects = append(objects, &object)
	}
	cl := fake.NewClientBuilder().
		WithScheme(nsprStatusScheme(t)).
		WithObjects(objects...).
		WithStatusSubresource(&deckhousev1alpha1.NodeStaticPodRequest{}).
		Build()

	r := &Reconciler{}
	r.Client = cl
	require.NoError(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard(), newPass()))

	dropped := &deckhousev1alpha1.NodeStaticPodRequest{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: fmt.Sprintf("pod-%02d", maxStaticPods)}, dropped))
	require.Equal(t, phaseDegraded, dropped.Status.Phase)
	condition := meta.FindStatusCondition(dropped.Status.Conditions, acceptedConditionType)
	require.Equal(t, metav1.ConditionFalse, condition.Status)
	require.Equal(t, reasonLimitExceeded, condition.Reason)
	require.Contains(t, condition.Message, "engine")

	kept := &deckhousev1alpha1.NodeStaticPodRequest{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "pod-00"}, kept))
	require.True(t, meta.IsStatusConditionTrue(kept.Status.Conditions, acceptedConditionType))
}

// A request that reaches no node is not broken, so it stays Ready for
// `kubectl wait`, and the reason says whether bashible serves it or what keeps it
// from every node. One with nodes that have not answered yet is Pending.
func TestNSPRStatusSaysWhyARequestReachesNoNode(t *testing.T) {
	engineNode := nodeInGroup("engine-0", "engine")
	workerNode := nodeInGroup("worker-0", "worker")
	selecting := func(name string, groups ...string) *deckhousev1alpha1.NodeStaticPodRequest {
		object := nspr(name, deckhousev1alpha1.NodeStaticPodRequestSpec{
			NodeGroupSelector: deckhousev1alpha1.NodeGroupSelector{MatchNames: groups},
		})
		return &object
	}
	for _, tc := range []struct {
		name          string
		request       *deckhousev1alpha1.NodeStaticPodRequest
		reason        string
		phase         string
		matchedGroups []string
		matchedNodes  int32
		mentions      []string
	}{
		{
			name: "a request with nodes waits for them", request: selecting("static-pod", "engine"),
			reason: reasonRolloutInProgress, phase: phasePending, matchedGroups: []string{"engine"}, matchedNodes: 1,
		},
		{
			name: "a request on a bashible group with nodes waits for them", request: selecting("static-pod", "worker"),
			reason: reasonRolloutInProgress, phase: phasePending, matchedGroups: []string{"worker"}, matchedNodes: 1,
		},
		{
			name: "bashible serves registry-agent on its own groups", request: selecting("registry-agent", "worker"),
			reason: "ServedByBashible", phase: phaseReady, matchedGroups: []string{"worker"}, mentions: []string{"worker"},
		},
		{
			name: "the selector names no existing group", request: selecting("static-pod", "ghost"),
			reason: "NoNodesSelected", phase: phaseReady, mentions: []string{"ghost"},
		},
		{
			name: "the selected group has no nodes", request: selecting("static-pod", "empty"),
			reason: "NoNodesSelected", phase: phaseReady, matchedGroups: []string{"empty"}, mentions: []string{"empty", "no nodes"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cl := fake.NewClientBuilder().
				WithScheme(nsprStatusScheme(t)).
				WithObjects(tc.request, immutableGroup("engine"), immutableGroup("empty"),
					mutableGroup("worker", v1.SystemTypeMutable), &engineNode, &workerNode).
				WithStatusSubresource(&deckhousev1alpha1.NodeStaticPodRequest{}).
				Build()

			r := &Reconciler{}
			r.Client = cl
			require.NoError(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard(), newPass()))

			fresh := &deckhousev1alpha1.NodeStaticPodRequest{}
			require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: tc.request.Name}, fresh))
			condition := meta.FindStatusCondition(fresh.Status.Conditions, readyConditionType)
			require.Equal(t, tc.phase == phaseReady, condition.Status == metav1.ConditionTrue, "Ready follows the phase")
			require.Equal(t, tc.phase, fresh.Status.Phase)
			require.Equal(t, tc.reason, condition.Reason)
			require.Equal(t, tc.matchedGroups, fresh.Status.MatchedNodeGroups)
			require.Equal(t, tc.matchedNodes, fresh.Status.MatchedNodes)
			for _, mention := range tc.mentions {
				require.Contains(t, condition.Message, mention)
			}
		})
	}
}

// The statuses read no release input: every bashible group runs the agent. A cluster
// with no images, apiserver endpoints or proxy token still publishes every request's status.
func TestNSPRStatusDoesNotWaitForTheRenderInputsOfABashibleNode(t *testing.T) {
	engineNode := nodeInGroup("engine-0", "engine")
	workerNode := nodeInGroup("worker-0", "worker")
	enginePod := nspr("engine-pod", deckhousev1alpha1.NodeStaticPodRequestSpec{
		NodeGroupSelector: deckhousev1alpha1.NodeGroupSelector{MatchNames: []string{"engine"}},
	})
	workerPod := nspr("worker-pod", deckhousev1alpha1.NodeStaticPodRequestSpec{
		NodeGroupSelector: deckhousev1alpha1.NodeGroupSelector{MatchNames: []string{"worker"}},
	})
	cl := fake.NewClientBuilder().
		WithScheme(nsprStatusScheme(t)).
		WithObjects(&enginePod, &workerPod, immutableGroup("engine"),
			mutableGroup("worker", v1.SystemTypeMutable), &engineNode, &workerNode).
		WithStatusSubresource(&deckhousev1alpha1.NodeStaticPodRequest{}).
		Build()

	r := &Reconciler{}
	r.Client = cl
	r.sources = sourceReaderOver(cl)
	require.NoError(t, r.reconcileNSPRStatuses(context.Background(), logr.Discard(), newPass()))

	for name, group := range map[string]string{"engine-pod": "engine", "worker-pod": "worker"} {
		fresh := &deckhousev1alpha1.NodeStaticPodRequest{}
		require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: name}, fresh))
		require.Equal(t, []string{group}, fresh.Status.MatchedNodeGroups, name)
		require.Equal(t, int32(1), fresh.Status.MatchedNodes, name)
		require.True(t, meta.IsStatusConditionTrue(fresh.Status.Conditions, acceptedConditionType), name)
	}
}

func nsprStatusOf(t *testing.T, cl client.Client, name string) deckhousev1alpha1.NodeStaticPodRequestStatus {
	t.Helper()
	fresh := &deckhousev1alpha1.NodeStaticPodRequest{}
	require.NoError(t, cl.Get(t.Context(), types.NamespacedName{Name: name}, fresh))
	return fresh.Status
}

// A node that will not run the revision carrying the manifest has refused it, whether it
// rolled the revision back or node-manager's approval to interrupt it failed; the message
// names each such node, so the operator knows which nodes to look at.
func TestNSPRStatusCountsTheNodesThatFailedTheRevisionCarryingIt(t *testing.T) {
	object := nspr("static-pod", deckhousev1alpha1.NodeStaticPodRequestSpec{})
	carrying := []internalv1alpha1.StaticPod{{Name: "static-pod", Manifest: podManifest("static-pod")}}
	runs := nodeConfigWithPod("worker-0", carrying, []internalv1alpha1.StaticPodStatus{writtenPod("static-pod", podManifest("static-pod"))})
	rolledBack := nodeConfigWithPod("worker-1", carrying, nil)
	rolledBack.Generation, rolledBack.Status.AppliedGeneration = 4, 3
	rolledBack.Status.Conditions = []metav1.Condition{{
		Type: internalv1alpha1.NodeConfigConditionTypeConfigurationApplied, Status: metav1.ConditionFalse, Reason: "RolledBackToLastKnownGood",
		ObservedGeneration: 4, Message: "kubelet: the unit did not start", LastTransitionTime: metav1.Now(),
	}}
	drainFailed := nodeConfigWithPod("worker-2", carrying, nil)
	waitingForApproval(drainFailed, "uid-2", 4)
	failed := approvalOf("worker-2", "disruption-worker-2-0", "uid-2", 4, deckhousev1alpha1.NodeOperationPhaseFailed,
		"the eviction of node worker-2 did not finish in time")
	failed.Status.FinishedAt = ptr.To(metav1.NewTime(time.Date(2026, time.January, 1, 8, 0, 0, 0, time.UTC)))
	objects := []client.Object{&object, immutableGroup("worker"), runs, rolledBack, drainFailed, failed}
	for _, name := range []string{"worker-0", "worker-1", "worker-2"} {
		node := nodeInGroup(name, "worker")
		objects = append(objects, &node)
	}
	cl := nerStatusCluster(t, objects...)

	require.NoError(t, nerStatusReconciler(cl).reconcileNSPRStatuses(t.Context(), logr.Discard(), newPass()))

	status := nsprStatusOf(t, cl, "static-pod")
	require.Equal(t, int32(1), status.AppliedNodes)
	require.Equal(t, int32(2), status.FailedNodes)
	require.Equal(t, phaseDegraded, status.Phase)
	ready := meta.FindStatusCondition(status.Conditions, readyConditionType)
	require.Equal(t, reasonRefusedByNodes, ready.Reason)
	require.Equal(t, "2 of 3 node(s) refused the static pod, 1 wrote it: "+
		"node worker-1: RolledBackToLastKnownGood: kubelet: the unit did not start; "+
		"node worker-2: NodeOperation disruption-worker-2-0 failed: the eviction of node worker-2 did not finish in time. "+
		"Attempt 2 of 3 is requested at 2026-01-01T08:01:00Z",
		ready.Message)
	require.Equal(t, []deckhousev1alpha1.RequestNodeStatus{
		{Name: "worker-1", State: deckhousev1alpha1.RequestNodeStateFailed, Message: "RolledBackToLastKnownGood: kubelet: the unit did not start"},
		{Name: "worker-2", State: deckhousev1alpha1.RequestNodeStateFailed,
			Message: "NodeOperation disruption-worker-2-0 failed: the eviction of node worker-2 did not finish in time. " +
				"Attempt 2 of 3 is requested at 2026-01-01T08:01:00Z"},
	}, status.Nodes)
}

// The retry after a failed attempt waiting for an operator leaves the failure a refusal, as for
// a NodeExtensionRequest: both read the node's refusal from nodeRefusal.
func TestNSPRStatusCountsAFailedAttemptWhileTheRetryWaitsForAnOperator(t *testing.T) {
	object := nspr("static-pod", deckhousev1alpha1.NodeStaticPodRequestSpec{})
	config := nodeConfigWithPod("worker-0", []internalv1alpha1.StaticPod{{Name: "static-pod", Manifest: podManifest("static-pod")}}, nil)
	waitingForApproval(config, "uid-0", 4)
	node := nodeInGroup("worker-0", "worker")
	cl := nerStatusCluster(t, &object, immutableGroup("worker"), &node, config,
		approvalOf("worker-0", "disruption-worker-0-0", "uid-0", 4, deckhousev1alpha1.NodeOperationPhaseFailed,
			"module example: no example.ko found"),
		approval("disruption-worker-0-1", "uid-0", 4, false, deckhousev1alpha1.NodeOperationPhasePending))

	require.NoError(t, nerStatusReconciler(cl).reconcileNSPRStatuses(t.Context(), logr.Discard(), newPass()))

	status := nsprStatusOf(t, cl, "static-pod")
	require.Equal(t, int32(1), status.FailedNodes)
	require.Equal(t, phaseDegraded, status.Phase)
	ready := meta.FindStatusCondition(status.Conditions, readyConditionType)
	require.Equal(t, reasonRefusedByNodes, ready.Reason)
	require.Equal(t, "1 of 1 node(s) refused the static pod, 0 wrote it: "+
		"node worker-0: NodeOperation disruption-worker-0-0 failed: module example: no example.ko found. "+
		"Attempt 2 of 3, NodeOperation disruption-worker-0-1, waits for an operator to set spec.approved to true",
		ready.Message)
}

// The pending list names five nodes and counts the rest, so a large group still gets a
// message under the size of a condition; a node whose agent does not know a field of its
// configuration says so.
func TestNSPRStatusNamesFivePendingNodesAndCountsTheRest(t *testing.T) {
	object := nspr("static-pod", deckhousev1alpha1.NodeStaticPodRequestSpec{})
	oldAgent := nodeConfigWithPod("worker-0", []internalv1alpha1.StaticPod{{Name: "static-pod", Manifest: podManifest("static-pod")}}, nil)
	oldAgent.Status.Conditions = []metav1.Condition{{
		Type: internalv1alpha1.NodeConfigConditionTypeConfigurationApplied, Status: metav1.ConditionTrue, Reason: "UnknownFields",
		Message: "the node does not apply spec.staticPods", LastTransitionTime: metav1.Now(),
	}}
	objects := []client.Object{&object, immutableGroup("worker"), oldAgent}
	for i := range 7 {
		node := nodeInGroup(fmt.Sprintf("worker-%d", i), "worker")
		objects = append(objects, &node)
	}
	cl := nerStatusCluster(t, objects...)

	require.NoError(t, nerStatusReconciler(cl).reconcileNSPRStatuses(t.Context(), logr.Discard(), newPass()))

	ready := meta.FindStatusCondition(nsprStatusOf(t, cl, "static-pod").Conditions, readyConditionType)
	require.Equal(t, reasonRolloutInProgress, ready.Reason)
	require.Equal(t, "0 of 7 node(s) wrote the current manifest; 7 pending: "+
		"node worker-0: the node agent does not support some fields of its NodeConfig: the node does not apply spec.staticPods; "+
		"node worker-1: the node has no NodeConfig yet; node worker-2: the node has no NodeConfig yet; "+
		"node worker-3: the node has no NodeConfig yet; node worker-4: the node has no NodeConfig yet; and 2 more",
		ready.Message)
}

// A node held on an older generation — for an approval of an unrelated change, or behind the
// rollout slot — still runs a manifest the request did not change, and is counted applied by
// the digest of the manifest it wrote: a held revision must not turn every request pending.
func TestNSPRStatusCountsAHeldNodeThatRunsTheManifest(t *testing.T) {
	object := nspr("static-pod", deckhousev1alpha1.NodeStaticPodRequestSpec{})
	held := nodeConfigWithPod("worker-0",
		[]internalv1alpha1.StaticPod{{Name: "static-pod", Manifest: podManifest("static-pod")}},
		[]internalv1alpha1.StaticPodStatus{writtenPod("static-pod", podManifest("static-pod"))})
	held.Status.AppliedGeneration = 2
	waitingForApproval(held, "uid-0", 3)
	node := nodeInGroup("worker-0", "worker")
	cl := nerStatusCluster(t, &object, immutableGroup("worker"), held, &node,
		approvalOf("worker-0", "disruption-worker-0-0", "uid-0", 3, deckhousev1alpha1.NodeOperationPhasePending, ""))

	require.NoError(t, nerStatusReconciler(cl).reconcileNSPRStatuses(t.Context(), logr.Discard(), newPass()))

	status := nsprStatusOf(t, cl, "static-pod")
	require.Equal(t, int32(1), status.AppliedNodes)
	require.Equal(t, phaseReady, status.Phase)
	require.Equal(t, reasonAllNodesApplied, meta.FindStatusCondition(status.Conditions, readyConditionType).Reason)
}
