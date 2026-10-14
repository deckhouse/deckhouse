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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// imagesConfigMap is the release's images_digests.json.
func imagesConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: cloudInstanceManagerNS, Name: imagesDigestsConfigMapName},
		Data:       map[string]string{imagesDigestsKey: testenv.ImagesDigestsJSON(testContainerdDigest)},
	}
}

// nerStatusCluster holds the requests, the groups and the nodes a status pass reads.
func nerStatusCluster(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	return fake.NewClientBuilder().
		WithScheme(nsprStatusScheme(t)).
		WithObjects(append(objects, imagesConfigMap(), testenv.PlatformTemplatesConfigMap())...).
		WithStatusSubresource(&deckhousev1alpha1.NodeExtensionRequest{}).
		Build()
}

// nerStatusReconciler is a reconciler over the cluster and nothing else: no proxy.
func nerStatusReconciler(cl client.Client) *Reconciler {
	r := &Reconciler{}
	r.Client = cl
	r.sources = sourceReaderOver(cl)
	return r
}

// kindedRequest is a request for an image of the kind its requester declared.
func kindedRequest(name, digest string, kind deckhousev1alpha1.ExtensionKind) *deckhousev1alpha1.NodeExtensionRequest {
	return &deckhousev1alpha1.NodeExtensionRequest{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: deckhousev1alpha1.NodeExtensionRequestSpec{
			Extension: deckhousev1alpha1.NodeExtension{Name: name, Digest: digest, Kind: kind},
		},
	}
}

func nerStatusOf(t *testing.T, cl client.Client, name string) deckhousev1alpha1.NodeExtensionRequestStatus {
	t.Helper()
	fresh := &deckhousev1alpha1.NodeExtensionRequest{}
	require.NoError(t, cl.Get(t.Context(), types.NamespacedName{Name: name}, fresh))
	return fresh.Status
}

func readyOf(t *testing.T, status deckhousev1alpha1.NodeExtensionRequestStatus) metav1.Condition {
	t.Helper()
	ready := meta.FindStatusCondition(status.Conditions, readyConditionType)
	require.NotNil(t, ready, "no Ready condition")
	return *ready
}

// A pass that cannot read the fleet knows nothing about it, and zero counts are
// not "nothing": publishing them turned "50 nodes refused this extension" into a
// clean Ready with the reason erased, on one unlucky listing.
func TestNERStatusIsNotPublishedFromAFleetThatCouldNotBeRead(t *testing.T) {
	request := kindedRequest("bob-request", drbdDigest, deckhousev1alpha1.ExtensionKindSystemExtension)
	request.Status.Phase = phaseDegraded
	request.Status.FailedNodes = 50
	request.Status.FailureMessage = "Required key not available"
	request.Status.Conditions = []metav1.Condition{{
		Type:               readyConditionType,
		Status:             metav1.ConditionFalse,
		Reason:             reasonRefusedByNodes,
		Message:            "50 node(s) refused the system extension, 0 applied it: Required key not available",
		LastTransitionTime: metav1.Now(),
	}}
	cl := fake.NewClientBuilder().
		WithScheme(nsprStatusScheme(t)).
		WithObjects(request, imagesConfigMap(), testenv.PlatformTemplatesConfigMap()).
		WithStatusSubresource(&deckhousev1alpha1.NodeExtensionRequest{}).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				if _, fleet := list.(*internalv1alpha1.NodeConfigList); fleet {
					return apierrors.NewServiceUnavailable("etcd leader changed")
				}
				return c.List(ctx, list, opts...)
			},
		}).
		Build()
	r := nerStatusReconciler(cl)

	require.Error(t, r.reconcileNERStatuses(t.Context(), logr.Discard()),
		"the pass has to be retried, not counted as a successful report")

	status := nerStatusOf(t, cl, request.Name)
	require.Equal(t, phaseDegraded, status.Phase)
	require.Equal(t, int32(50), status.FailedNodes)
	require.Equal(t, "Required key not available", status.FailureMessage)
}

// A system extension reaches the Engine nodes it matches and no bashible node, and a
// request that names no group mentions no group of the other type.
func TestASystemExtensionCountsTheNodesOfEngineGroups(t *testing.T) {
	request := kindedRequest("bob-request", drbdDigest, deckhousev1alpha1.ExtensionKindSystemExtension)
	engineNode := nodeInGroup("worker-0", "worker")
	bashibleNode := nodeInGroup("mutable-0", "mutable-workers")
	cl := nerStatusCluster(t, request, immutableGroup("worker"),
		mutableGroup("mutable-workers", v1.SystemTypeMutable), &engineNode, &bashibleNode)

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard()))

	status := nerStatusOf(t, cl, request.Name)
	require.Equal(t, []string{"worker"}, status.MatchedNodeGroups)
	require.Equal(t, int32(1), status.MatchedNodes, "a bashible node installs no system extension")
	require.Equal(t, int32(1), status.PendingNodes)
	require.Equal(t, metav1.ConditionTrue, readyOf(t, status).Status)
	require.NotContains(t, readyOf(t, status).Message, "mutable-workers")
}

// A registry package reaches the nodes bashible manages.
func TestARegistryPackageCountsTheNodesOfBashibleGroups(t *testing.T) {
	engineNode := nodeInGroup("engine-0", "engine")
	bashibleNode := nodeInGroup("worker-0", "worker")
	cl := nerStatusCluster(t, kindedRequest("sds-utils", drbdDigest, deckhousev1alpha1.ExtensionKindPackage),
		immutableGroup("engine"), mutableGroup("worker", v1.SystemTypeMutable), &engineNode, &bashibleNode)

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard()))

	status := nerStatusOf(t, cl, "sds-utils")
	require.Equal(t, []string{"worker"}, status.MatchedNodeGroups)
	require.Equal(t, int32(1), status.MatchedNodes)
	require.Equal(t, int32(1), status.PendingNodes)
	require.Equal(t, metav1.ConditionTrue, readyOf(t, status).Status)
}

// A release without the agent's package is broken, not an edition without the agent:
// the pass fails on it by name and publishes no status that blames the edition.
func TestNERStatusFailsOnAReleaseWithoutTheAgentPackage(t *testing.T) {
	bashibleNode := nodeInGroup("worker-0", "worker")
	digests := mutableDigests()
	delete(digests, "nodelet")
	cl := mutableCluster(t, digestsJSON(t, digests), kindedRequest("sds-utils", drbdDigest, deckhousev1alpha1.ExtensionKindPackage),
		mutableGroup("worker", v1.SystemTypeMutable), &bashibleNode)

	err := nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard())

	require.ErrorContains(t, err, `"nodelet"`)
	require.Empty(t, nerStatusOf(t, cl, "sds-utils").Conditions)
}

// A request this controller refused was handed to no node, so nobody is late with an
// answer: "Degraded, 2 nodes have not answered" sends an operator looking at nodes that
// were never asked.
func TestNERStatusRefusedHereLeavesNothingPending(t *testing.T) {
	winner := kindedRequest("bob-copy", otherDigest, deckhousev1alpha1.ExtensionKindSystemExtension)
	winner.Spec.Extension.Name = "bob"
	loser := kindedRequest("bob-request", drbdDigest, deckhousev1alpha1.ExtensionKindSystemExtension)
	loser.Spec.Extension.Name = "bob"
	first := nodeInGroup("worker-0", "worker")
	second := nodeInGroup("worker-1", "worker")
	cl := nerStatusCluster(t, winner, loser, immutableGroup("worker"), &first, &second)

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard()))

	status := nerStatusOf(t, cl, loser.Name)
	require.Equal(t, reasonConflict, readyOf(t, status).Reason)
	require.Equal(t, int32(0), status.PendingNodes, "nothing was handed to a node")
	require.Equal(t, int32(2), status.MatchedNodes, "how many nodes the selector covers is still the answer")
}

// The nodes did get this one, so the arithmetic stands: zeroing pending on every
// Degraded request would lose the node that has not reported yet.
func TestNERStatusRefusedByTheNodesKeepsItsArithmetic(t *testing.T) {
	request := kindedRequest("bob-request", drbdDigest, deckhousev1alpha1.ExtensionKindSystemExtension)
	request.Spec.Extension.Name = "bob"
	first := nodeInGroup("worker-0", "worker")
	second := nodeInGroup("worker-1", "worker")
	silent := nodeInGroup("worker-2", "worker")
	cl := nerStatusCluster(t,
		request, immutableGroup("worker"), &first, &second, &silent,
		nodeConfigWith("worker-0",
			[]internalv1alpha1.Extension{{Name: "bob", RequestedBy: nerRequestedByPrefix + "bob-request"}},
			[]internalv1alpha1.ExtensionStatus{{Name: "bob", State: "Ready"}}),
		nodeConfigWith("worker-1",
			[]internalv1alpha1.Extension{{Name: "bob", RequestedBy: nerRequestedByPrefix + "bob-request"}},
			[]internalv1alpha1.ExtensionStatus{{Name: "bob", State: "Failed", Message: "Required key not available"}}),
	)

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard()))

	status := nerStatusOf(t, cl, request.Name)
	require.Equal(t, reasonRefusedByNodes, readyOf(t, status).Reason)
	require.Equal(t, int32(1), status.PendingNodes, "the third node still owes an answer")
}

// The denominator is the nodes the render gives the extension to, so a request
// narrowed by node labels does not wait for the nodes it never reached.
func TestNERStatusCountsOnlyTheNodesItsLabelsSelect(t *testing.T) {
	request := kindedRequest("bob-request", drbdDigest, deckhousev1alpha1.ExtensionKindSystemExtension)
	request.Spec.NodeSelector = deckhousev1alpha1.NodeSelector{MatchLabels: map[string]string{"gpu": "true"}}
	selected := nodeInGroup("worker-0", "worker")
	selected.Labels["gpu"] = "true"
	other := nodeInGroup("worker-1", "worker")
	cl := nerStatusCluster(t, request, immutableGroup("worker"), &selected, &other)

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard()))

	status := nerStatusOf(t, cl, request.Name)
	require.Equal(t, int32(1), status.MatchedNodes)
	require.Equal(t, int32(1), status.PendingNodes)
}

// The kind is the spec's from the first pass: a request nobody has seen yet reaches its
// groups and records this generation, with no proxy to ask.
func TestNERStatusTakesTheKindFromTheSpec(t *testing.T) {
	for _, tc := range []struct {
		kind   deckhousev1alpha1.ExtensionKind
		target string
	}{
		{kind: deckhousev1alpha1.ExtensionKindSystemExtension, target: "engine"},
		{kind: deckhousev1alpha1.ExtensionKindPackage, target: "worker"},
	} {
		t.Run(string(tc.kind), func(t *testing.T) {
			request := kindedRequest("sds-utils", drbdDigest, tc.kind)
			request.Generation = 2
			engineNode := nodeInGroup("engine-0", "engine")
			bashibleNode := nodeInGroup("worker-0", "worker")
			cl := nerStatusCluster(t, request, immutableGroup("engine"), mutableGroup("worker", v1.SystemTypeMutable),
				&engineNode, &bashibleNode)

			require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard()))

			status := nerStatusOf(t, cl, "sds-utils")
			require.Equal(t, []string{tc.target}, status.MatchedNodeGroups)
			require.Equal(t, int32(1), status.MatchedNodes)
			require.Equal(t, int64(2), status.ObservedGeneration)
			require.Equal(t, metav1.ConditionTrue, readyOf(t, status).Status)
		})
	}
}

// A group named in matchNames that cannot install this kind of artifact is named in
// the message; only such groups make the request NotDeliverable; an empty selector
// means "wherever it works" and names no group.
func TestARequestNamingGroupsOfTheOtherTypeSaysSo(t *testing.T) {
	statusFor := func(t *testing.T, matchNames []string) deckhousev1alpha1.NodeExtensionRequestStatus {
		request := kindedRequest("sds-utils", drbdDigest, deckhousev1alpha1.ExtensionKindPackage)
		request.Spec.NodeGroupSelector.MatchNames = matchNames
		engineNode := nodeInGroup("engine-0", "engine")
		bashibleNode := nodeInGroup("worker-0", "worker")
		cl := nerStatusCluster(t, request, immutableGroup("engine"), mutableGroup("worker", v1.SystemTypeMutable),
			&engineNode, &bashibleNode)
		require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard()))
		return nerStatusOf(t, cl, "sds-utils")
	}

	t.Run("one group of each type", func(t *testing.T) {
		ready := readyOf(t, statusFor(t, []string{"engine", "worker"}))
		require.Equal(t, metav1.ConditionTrue, ready.Status)
		require.Contains(t, ready.Message, "NodeGroup(s) engine run Deckhouse Engine, which installs no registry package")
	})

	t.Run("only the other type", func(t *testing.T) {
		status := statusFor(t, []string{"engine"})
		require.Equal(t, phaseDegraded, status.Phase)
		require.Equal(t, reasonNotDeliverable, readyOf(t, status).Reason)
		require.Contains(t, readyOf(t, status).Message, "NodeGroup(s) engine")
	})

	t.Run("no names", func(t *testing.T) {
		ready := readyOf(t, statusFor(t, nil))
		require.Equal(t, metav1.ConditionTrue, ready.Status)
		require.NotContains(t, ready.Message, "NodeGroup(s)")
	})
}

// Kernel modules load from a system extension only: a registry package that asks for
// them reaches no node.
func TestARegistryPackageWithKernelModulesIsNotDeliverable(t *testing.T) {
	request := kindedRequest("sds-utils", drbdDigest, deckhousev1alpha1.ExtensionKindPackage)
	request.Spec.KernelModules = []deckhousev1alpha1.KernelModule{{Name: "drbd"}}
	node := nodeInGroup("worker-0", "worker")
	cl := nerStatusCluster(t, request, mutableGroup("worker", v1.SystemTypeMutable), &node)

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard()))

	status := nerStatusOf(t, cl, "sds-utils")
	require.Equal(t, reasonNotDeliverable, readyOf(t, status).Reason)
	require.Contains(t, readyOf(t, status).Message, "kernel modules")
	require.Equal(t, int32(0), status.MatchedNodes)
}

// 0 is an answer the printer columns show; a counter left out shows nothing.
func TestNERStatusWritesItsZeroCounters(t *testing.T) {
	request := kindedRequest("sds-utils", drbdDigest, deckhousev1alpha1.ExtensionKindSystemExtension)
	request.Status.FailedNodes = 3
	cl := nerStatusCluster(t, request, immutableGroup("worker"))

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard()))

	stored := &unstructured.Unstructured{}
	stored.SetGroupVersionKind(deckhousev1alpha1.GroupVersion.WithKind("NodeExtensionRequest"))
	require.NoError(t, cl.Get(t.Context(), types.NamespacedName{Name: request.Name}, stored))
	for _, counter := range []string{"matchedNodes", "appliedNodes", "failedNodes", "pendingNodes"} {
		value, found, err := unstructured.NestedInt64(stored.Object, "status", counter)
		require.NoError(t, err)
		require.True(t, found, "%s is left out of the stored status", counter)
		require.Zero(t, value, counter)
	}
}

// The status names the platform package a request yields to, in the contract's words.
func TestNERStatusNamesThePlatformPackageARequestYieldsTo(t *testing.T) {
	node := nodeInGroup("worker-0", "worker")
	cl := nerStatusCluster(t, kindedRequest("jq", drbdDigest, deckhousev1alpha1.ExtensionKindPackage),
		mutableGroup("worker", v1.SystemTypeMutable), &node)

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard()))

	ready := readyOf(t, nerStatusOf(t, cl, "jq"))
	require.Equal(t, reasonConflict, ready.Reason)
	require.Equal(t, `name "jq" is taken by a platform package; the name is only a label, pick another`, ready.Message)
}

// Only a registry package can lose to a platform package, so a release that cannot be
// read holds back their status alone; the pass still fails and is retried.
func TestAnUnreadablePlatformHoldsBackOnlyTheRegistryPackages(t *testing.T) {
	sysext := kindedRequest("bob-request", drbdDigest, deckhousev1alpha1.ExtensionKindSystemExtension)
	registryPackage := kindedRequest("sds-utils", otherDigest, deckhousev1alpha1.ExtensionKindPackage)
	engineNode := nodeInGroup("engine-0", "engine")
	bashibleNode := nodeInGroup("worker-0", "worker")
	cl := fake.NewClientBuilder().
		WithScheme(nsprStatusScheme(t)).
		WithObjects(sysext, registryPackage, immutableGroup("engine"), mutableGroup("worker", v1.SystemTypeMutable),
			&engineNode, &bashibleNode, imagesConfigMap()).
		WithStatusSubresource(&deckhousev1alpha1.NodeExtensionRequest{}).
		Build()

	err := nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard())

	require.ErrorContains(t, err, "platform node document")
	require.Equal(t, metav1.ConditionTrue, readyOf(t, nerStatusOf(t, cl, sysext.Name)).Status)
	require.Empty(t, nerStatusOf(t, cl, registryPackage.Name).Conditions)
}

// System extensions alone contend with no platform package: the release is not read.
func TestSystemExtensionsAloneReadNoRelease(t *testing.T) {
	sysext := kindedRequest("bob-request", drbdDigest, deckhousev1alpha1.ExtensionKindSystemExtension)
	engineNode := nodeInGroup("engine-0", "engine")
	cl := fake.NewClientBuilder().
		WithScheme(nsprStatusScheme(t)).
		WithObjects(sysext, immutableGroup("engine"), &engineNode).
		WithStatusSubresource(&deckhousev1alpha1.NodeExtensionRequest{}).
		Build()

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard()))
	require.Equal(t, metav1.ConditionTrue, readyOf(t, nerStatusOf(t, cl, sysext.Name)).Status)
}
