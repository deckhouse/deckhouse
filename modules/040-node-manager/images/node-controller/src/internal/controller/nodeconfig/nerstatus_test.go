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
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// imagesConfigMap is the release's images_digests.json.
func imagesConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: cloudInstanceManagerNS, Name: imagesDigestsConfigMapName},
		Data:       map[string]string{imagesDigestsKey: testenv.ImagesDigestsJSON(testContainerdDigest)},
	}
}

// nerStatusCluster holds the requests, the groups and the nodes a status pass reads, and
// serves NodeOperations by node name the way the API server does.
func nerStatusCluster(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	return fake.NewClientBuilder().
		WithScheme(nsprStatusScheme(t)).
		WithObjects(append(objects, imagesConfigMap(), testenv.PlatformTemplatesConfigMap())...).
		WithStatusSubresource(&deckhousev1alpha1.NodeExtensionRequest{}, &deckhousev1alpha1.NodeStaticPodRequest{}).
		WithIndex(&deckhousev1alpha1.NodeOperation{}, deckhousev1alpha1.NodeOperationNodeNameField, func(obj client.Object) []string {
			return []string{obj.(*deckhousev1alpha1.NodeOperation).Spec.NodeName}
		}).
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

func acceptedOf(t *testing.T, status deckhousev1alpha1.NodeExtensionRequestStatus) metav1.Condition {
	t.Helper()
	accepted := meta.FindStatusCondition(status.Conditions, acceptedConditionType)
	require.NotNil(t, accepted, "no Accepted condition")
	return *accepted
}

// A pass that cannot read the fleet knows nothing about it, and zero counts are
// not "nothing": publishing them turned "50 nodes refused this extension" into a
// clean Ready with the reason erased, on one unlucky listing.
func TestNERStatusIsNotPublishedFromAFleetThatCouldNotBeRead(t *testing.T) {
	request := kindedRequest("bob-request", drbdDigest, deckhousev1alpha1.ExtensionKindSystemExtension)
	request.Status.Phase = phaseDegraded
	request.Status.FailedNodes = 50
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

	require.Error(t, r.reconcileNERStatuses(t.Context(), logr.Discard(), newPass()),
		"the pass has to be retried, not counted as a successful report")

	status := nerStatusOf(t, cl, request.Name)
	require.Equal(t, phaseDegraded, status.Phase)
	require.Equal(t, int32(50), status.FailedNodes)
	require.Contains(t, readyOf(t, status).Message, "Required key not available")
}

// A system extension reaches the Engine nodes it matches and no bashible node, and a
// request that names no group mentions no group of the other type.
func TestASystemExtensionCountsTheNodesOfEngineGroups(t *testing.T) {
	request := kindedRequest("bob-request", drbdDigest, deckhousev1alpha1.ExtensionKindSystemExtension)
	engineNode := nodeInGroup("worker-0", "worker")
	bashibleNode := nodeInGroup("mutable-0", "mutable-workers")
	cl := nerStatusCluster(t, request, immutableGroup("worker"),
		mutableGroup("mutable-workers", v1.SystemTypeMutable), &engineNode, &bashibleNode)

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard(), newPass()))

	status := nerStatusOf(t, cl, request.Name)
	require.Equal(t, []string{"worker"}, status.MatchedNodeGroups)
	require.Equal(t, int32(1), status.MatchedNodes, "a bashible node installs no system extension")
	require.Equal(t, int32(1), status.PendingNodes)
	require.Equal(t, metav1.ConditionTrue, acceptedOf(t, status).Status)
	require.NotContains(t, acceptedOf(t, status).Message, "mutable-workers")
	require.Equal(t, reasonRolloutInProgress, readyOf(t, status).Reason)
	require.Equal(t, phasePending, status.Phase)
}

// A registry package reaches the nodes bashible manages.
func TestARegistryPackageCountsTheNodesOfBashibleGroups(t *testing.T) {
	engineNode := nodeInGroup("engine-0", "engine")
	bashibleNode := nodeInGroup("worker-0", "worker")
	cl := nerStatusCluster(t, kindedRequest("sds-utils", drbdDigest, deckhousev1alpha1.ExtensionKindPackage),
		immutableGroup("engine"), mutableGroup("worker", v1.SystemTypeMutable), &engineNode, &bashibleNode)

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard(), newPass()))

	status := nerStatusOf(t, cl, "sds-utils")
	require.Equal(t, []string{"worker"}, status.MatchedNodeGroups)
	require.Equal(t, int32(1), status.MatchedNodes)
	require.Equal(t, int32(1), status.PendingNodes)
	require.Equal(t, metav1.ConditionTrue, acceptedOf(t, status).Status)
	require.Equal(t, metav1.ConditionFalse, readyOf(t, status).Status)
}

// A release without the agent's package is broken, not an edition without the agent:
// the pass fails on it by name and publishes no status that blames the edition.
func TestNERStatusFailsOnAReleaseWithoutTheAgentPackage(t *testing.T) {
	bashibleNode := nodeInGroup("worker-0", "worker")
	digests := mutableDigests()
	delete(digests, "nodelet")
	cl := mutableCluster(t, digestsJSON(t, digests), kindedRequest("sds-utils", drbdDigest, deckhousev1alpha1.ExtensionKindPackage),
		mutableGroup("worker", v1.SystemTypeMutable), &bashibleNode)

	err := nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard(), newPass())

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

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard(), newPass()))

	status := nerStatusOf(t, cl, loser.Name)
	require.Equal(t, reasonConflict, acceptedOf(t, status).Reason)
	require.Equal(t, reasonNotAccepted, readyOf(t, status).Reason)
	require.Equal(t, phaseDegraded, status.Phase)
	require.Equal(t, int32(0), status.PendingNodes, "nothing was handed to a node")
	require.Empty(t, status.Nodes, "no node owes an answer")
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
			[]internalv1alpha1.Extension{{Name: "bob", Digest: drbdDigest, RequestedBy: nerOwner("bob-request")}},
			[]internalv1alpha1.ExtensionStatus{{Name: "bob", Digest: drbdDigest, State: "Ready"}}),
		nodeConfigWith("worker-1",
			[]internalv1alpha1.Extension{{Name: "bob", Digest: drbdDigest, RequestedBy: nerOwner("bob-request")}},
			[]internalv1alpha1.ExtensionStatus{{Name: "bob", Digest: drbdDigest, State: "Failed", Message: "Required key not available"}}),
	)

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard(), newPass()))

	status := nerStatusOf(t, cl, request.Name)
	require.Equal(t, reasonRefusedByNodes, readyOf(t, status).Reason)
	require.Contains(t, readyOf(t, status).Message, "Required key not available")
	require.Equal(t, phaseDegraded, status.Phase)
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

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard(), newPass()))

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

			require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard(), newPass()))

			status := nerStatusOf(t, cl, "sds-utils")
			require.Equal(t, []string{tc.target}, status.MatchedNodeGroups)
			require.Equal(t, int32(1), status.MatchedNodes)
			require.Equal(t, int64(2), status.ObservedGeneration)
			require.Equal(t, metav1.ConditionTrue, acceptedOf(t, status).Status)
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
		require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard(), newPass()))
		return nerStatusOf(t, cl, "sds-utils")
	}

	t.Run("one group of each type", func(t *testing.T) {
		accepted := acceptedOf(t, statusFor(t, []string{"engine", "worker"}))
		require.Equal(t, metav1.ConditionTrue, accepted.Status)
		require.Contains(t, accepted.Message, "NodeGroup(s) engine run Deckhouse Engine, which installs no registry package")
	})

	t.Run("only the other type", func(t *testing.T) {
		status := statusFor(t, []string{"engine"})
		require.Equal(t, phaseDegraded, status.Phase)
		require.Equal(t, reasonNotDeliverable, acceptedOf(t, status).Reason)
		require.Contains(t, acceptedOf(t, status).Message, "NodeGroup(s) engine")
	})

	t.Run("no names", func(t *testing.T) {
		accepted := acceptedOf(t, statusFor(t, nil))
		require.Equal(t, metav1.ConditionTrue, accepted.Status)
		require.NotContains(t, accepted.Message, "NodeGroup(s)")
	})
}

// Kernel modules load from a system extension only: a registry package that asks for
// them reaches no node.
func TestARegistryPackageWithKernelModulesIsNotDeliverable(t *testing.T) {
	request := kindedRequest("sds-utils", drbdDigest, deckhousev1alpha1.ExtensionKindPackage)
	request.Spec.Extension.KernelModules = []deckhousev1alpha1.KernelModule{{Name: "drbd"}}
	node := nodeInGroup("worker-0", "worker")
	cl := nerStatusCluster(t, request, mutableGroup("worker", v1.SystemTypeMutable), &node)

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard(), newPass()))

	status := nerStatusOf(t, cl, "sds-utils")
	require.Equal(t, reasonNotDeliverable, acceptedOf(t, status).Reason)
	require.Contains(t, acceptedOf(t, status).Message, "kernel modules")
	require.Equal(t, int32(0), status.MatchedNodes)
}

// 0 is an answer the printer columns show; a counter left out shows nothing.
func TestNERStatusWritesItsZeroCounters(t *testing.T) {
	request := kindedRequest("sds-utils", drbdDigest, deckhousev1alpha1.ExtensionKindSystemExtension)
	request.Status.FailedNodes = 3
	cl := nerStatusCluster(t, request, immutableGroup("worker"))

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard(), newPass()))

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

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard(), newPass()))

	accepted := acceptedOf(t, nerStatusOf(t, cl, "jq"))
	require.Equal(t, reasonConflict, accepted.Reason)
	require.Equal(t, `name "jq" is taken by a platform package. Choose another name`, accepted.Message)
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

	err := nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard(), newPass())

	require.ErrorContains(t, err, "platform node document")
	require.Equal(t, metav1.ConditionTrue, acceptedOf(t, nerStatusOf(t, cl, sysext.Name)).Status)
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

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard(), newPass()))
	require.Equal(t, metav1.ConditionTrue, acceptedOf(t, nerStatusOf(t, cl, sysext.Name)).Status)
}

// Ready means the fleet runs this request's digest: a node still on the digest of an
// older generation is pending, and the phase stays Pending until every node answers.
func TestNERIsReadyOnlyWhenTheNodesRunItsDigest(t *testing.T) {
	request := kindedRequest("bob-request", drbdDigest, deckhousev1alpha1.ExtensionKindSystemExtension)
	request.Spec.Extension.Name = "bob"
	updated := nodeInGroup("worker-0", "worker")
	behind := nodeInGroup("worker-1", "worker")
	runs := func(node, digest string) *internalv1alpha1.NodeConfig {
		return nodeConfigWith(node,
			[]internalv1alpha1.Extension{{Name: "bob", Digest: digest, RequestedBy: nerOwner("bob-request")}},
			[]internalv1alpha1.ExtensionStatus{{Name: "bob", Digest: digest, State: "Ready"}})
	}
	cl := nerStatusCluster(t, request, immutableGroup("worker"), &updated, &behind,
		runs("worker-0", drbdDigest), runs("worker-1", otherDigest))

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard(), newPass()))

	status := nerStatusOf(t, cl, request.Name)
	require.Equal(t, int32(1), status.AppliedNodes)
	require.Equal(t, int32(1), status.PendingNodes)
	require.Equal(t, phasePending, status.Phase)
	require.Equal(t, metav1.Condition{
		Type: readyConditionType, Status: metav1.ConditionFalse, Reason: reasonRolloutInProgress,
		Message: "1 of 2 node(s) run " + drbdDigest + "; 1 pending: node worker-1: its NodeConfig does not include the request yet",
	}, withoutTime(readyOf(t, status)))

	behindConfig := runs("worker-1", drbdDigest)
	stored := &internalv1alpha1.NodeConfig{}
	require.NoError(t, cl.Get(t.Context(), types.NamespacedName{Name: "worker-1"}, stored))
	stored.Spec, stored.Status = behindConfig.Spec, behindConfig.Status
	require.NoError(t, cl.Update(t.Context(), stored))
	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard(), newPass()))

	status = nerStatusOf(t, cl, request.Name)
	require.Equal(t, phaseReady, status.Phase)
	require.Equal(t, reasonAllNodesApplied, readyOf(t, status).Reason)
	require.Equal(t, "2 of 2 node(s) run "+drbdDigest, readyOf(t, status).Message)
}

// A valid request that selects no node has nothing to wait for: it is accepted and
// Ready, and kubectl wait --for=condition=Ready returns.
func TestANERThatSelectsNoNodeIsReady(t *testing.T) {
	request := kindedRequest("bob-request", drbdDigest, deckhousev1alpha1.ExtensionKindSystemExtension)
	request.Spec.NodeGroupSelector.MatchNames = []string{"absent"}
	cl := nerStatusCluster(t, request, immutableGroup("worker"))

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard(), newPass()))

	status := nerStatusOf(t, cl, request.Name)
	require.Equal(t, metav1.ConditionTrue, acceptedOf(t, status).Status)
	require.Equal(t, reasonNoNodesSelected, readyOf(t, status).Reason)
	require.Equal(t, metav1.ConditionTrue, readyOf(t, status).Status)
	require.Equal(t, phaseReady, status.Phase)
}

// A reserved name is the controller's refusal now that no webhook stands in front:
// the request is not accepted and reaches no node.
func TestANERForAPlatformExtensionNameIsNotAccepted(t *testing.T) {
	request := kindedRequest("kubelet", drbdDigest, deckhousev1alpha1.ExtensionKindSystemExtension)
	node := nodeInGroup("worker-0", "worker")
	cl := nerStatusCluster(t, request, immutableGroup("worker"), &node)

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard(), newPass()))

	status := nerStatusOf(t, cl, request.Name)
	require.Equal(t, reasonReservedName, acceptedOf(t, status).Reason)
	require.Equal(t, metav1.ConditionFalse, acceptedOf(t, status).Status)
	require.Equal(t, phaseDegraded, status.Phase)
}

// A node's refusal can carry a whole script output; the status write must not fail on
// its size, so the message is cut and says so.
func TestANodesRefusalIsTruncatedInTheCondition(t *testing.T) {
	request := kindedRequest("bob-request", drbdDigest, deckhousev1alpha1.ExtensionKindSystemExtension)
	request.Spec.Extension.Name = "bob"
	node := nodeInGroup("worker-0", "worker")
	cl := nerStatusCluster(t, request, immutableGroup("worker"), &node,
		nodeConfigWith("worker-0",
			[]internalv1alpha1.Extension{{Name: "bob", Digest: drbdDigest, RequestedBy: nerOwner("bob-request")}},
			[]internalv1alpha1.ExtensionStatus{{Name: "bob", Digest: drbdDigest, State: "Failed", Message: strings.Repeat("failure ", 2000)}}))

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard(), newPass()))

	status := nerStatusOf(t, cl, request.Name)
	require.Len(t, status.Nodes, 1)
	for _, message := range []string{readyOf(t, status).Message, status.Nodes[0].Message} {
		require.LessOrEqual(t, len(message), maxConditionMessage)
		require.True(t, strings.HasSuffix(message, "… (truncated)"), message[len(message)-40:])
		require.True(t, utf8.ValidString(message))
	}
}

func withoutTime(cond metav1.Condition) metav1.Condition {
	cond.LastTransitionTime = metav1.Time{}
	cond.ObservedGeneration = 0
	return cond
}

// waitingForApproval makes a node's config ask to be interrupted for the revision it holds.
func waitingForApproval(config *internalv1alpha1.NodeConfig, uid string, generation int64) {
	config.UID = types.UID(uid)
	config.Generation = generation
	config.Status.Conditions = append(config.Status.Conditions, metav1.Condition{
		Type: internalv1alpha1.NodeConfigConditionTypeDisruptionApproved, Status: metav1.ConditionFalse, Reason: internalv1alpha1.NodeConfigReasonApprovalPending,
		ObservedGeneration: generation, LastTransitionTime: metav1.Now(),
	})
}

// approvalOf is a Disruption of one node's revision, in the given phase.
func approvalOf(node, name, uid string, generation int64, phase deckhousev1alpha1.NodeOperationPhase, message string) *deckhousev1alpha1.NodeOperation {
	op := approval(name, uid, generation, true, phase)
	op.Spec.NodeName = node
	op.Status.Message = message
	if phase == deckhousev1alpha1.NodeOperationPhaseFailed {
		op.Status.FinishedAt = ptr.To(metav1.Now())
	}
	return op
}

// A node that will not run the revision carrying a request has refused the request: it refused
// or rolled the revision back, or node-manager gave up interrupting it for that revision. Every
// such node is named with its cause, or the operator learns that nodes failed and not which.
func TestNERStatusCountsTheNodesThatFailedTheRevisionCarryingIt(t *testing.T) {
	request := kindedRequest("bob-request", drbdDigest, deckhousev1alpha1.ExtensionKindSystemExtension)
	request.Spec.Extension.Name = "bob"
	carrying := []internalv1alpha1.Extension{{Name: "bob", Digest: drbdDigest, RequestedBy: nerOwner("bob-request")}}
	runs := nodeConfigWith("worker-0", carrying, []internalv1alpha1.ExtensionStatus{{Name: "bob", Digest: drbdDigest, State: "Ready"}})
	rejected := nodeConfigWith("worker-1", carrying, nil)
	rejected.Generation, rejected.Status.AppliedGeneration = 4, 3
	rejected.Status.Conditions = []metav1.Condition{{
		Type: internalv1alpha1.NodeConfigConditionTypeConfigurationApplied, Status: metav1.ConditionFalse, Reason: "Rejected",
		ObservedGeneration: 4, Message: "bob: the image is not signed", LastTransitionTime: metav1.Now(),
	}}
	givenUp := nodeConfigWith("worker-2", carrying, nil)
	waitingForApproval(givenUp, "uid-2", 4)
	objects := []client.Object{request, immutableGroup("worker"), runs, rejected, givenUp}
	for _, name := range []string{"worker-0", "worker-1", "worker-2"} {
		node := nodeInGroup(name, "worker")
		objects = append(objects, &node)
	}
	for i := range nodecommon.MaxDisruptionAttempts {
		op := approvalOf("worker-2", fmt.Sprintf("disruption-worker-2-%d", i), "uid-2", 4,
			deckhousev1alpha1.NodeOperationPhaseFailed, "the eviction of node worker-2 did not finish in time")
		op.Status.FinishedAt = ptr.To(metav1.NewTime(time.Now().Add(time.Duration(i-nodecommon.MaxDisruptionAttempts) * time.Hour)))
		objects = append(objects, op)
	}
	cl := nerStatusCluster(t, objects...)

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard(), newPass()))

	status := nerStatusOf(t, cl, request.Name)
	require.Equal(t, int32(1), status.AppliedNodes)
	require.Equal(t, int32(2), status.FailedNodes)
	require.Equal(t, int32(0), status.PendingNodes)
	require.Equal(t, phaseDegraded, status.Phase)
	ready := readyOf(t, status)
	require.Equal(t, reasonRefusedByNodes, ready.Reason)
	require.Equal(t, "2 of 3 node(s) refused the system extension "+drbdDigest+", 1 run it: "+
		"node worker-1: Rejected: bob: the image is not signed; "+
		"node worker-2: NodeOperation disruption-worker-2-2 failed: the eviction of node worker-2 did not finish in time. "+
		"node-manager creates no new NodeOperation after 3 failed attempts. Fix the cause and change the configuration to retry",
		ready.Message)
}

// Once node-manager has given up, the node says so with DisruptionApproved=False/AttemptsExhausted
// instead of ApprovalPending. The request still names it as refusing.
func TestNERStatusCountsANodeNodeManagerGaveUpOn(t *testing.T) {
	request := kindedRequest("bob-request", drbdDigest, deckhousev1alpha1.ExtensionKindSystemExtension)
	request.Spec.Extension.Name = "bob"
	config := nodeConfigWith("worker-0",
		[]internalv1alpha1.Extension{{Name: "bob", Digest: drbdDigest, RequestedBy: nerOwner("bob-request")}}, nil)
	waitingForApproval(config, "uid-0", 4)
	config.Status.Conditions[0].Reason = "AttemptsExhausted"
	node := nodeInGroup("worker-0", "worker")
	objects := []client.Object{request, immutableGroup("worker"), &node, config}
	for i := range nodecommon.MaxDisruptionAttempts {
		op := approvalOf("worker-0", fmt.Sprintf("disruption-worker-0-%d", i), "uid-0", 4,
			deckhousev1alpha1.NodeOperationPhaseFailed, "the node did not answer")
		op.Status.FinishedAt = ptr.To(metav1.NewTime(time.Now().Add(time.Duration(i-nodecommon.MaxDisruptionAttempts) * time.Hour)))
		objects = append(objects, op)
	}
	cl := nerStatusCluster(t, objects...)

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard(), newPass()))

	status := nerStatusOf(t, cl, request.Name)
	require.Equal(t, int32(1), status.FailedNodes)
	require.Equal(t, int32(0), status.PendingNodes)
	require.Equal(t, "1 of 1 node(s) refused the system extension "+drbdDigest+", 0 run it: "+
		"node worker-0: NodeOperation disruption-worker-0-2 failed: the node did not answer. "+
		"node-manager creates no new NodeOperation after 3 failed attempts. Fix the cause and change the configuration to retry",
		readyOf(t, status).Message)
}

// In Manual mode the retry after a failed attempt waits for an operator, which can take days:
// the failure stays a refusal until the retry is approved, or the request reads as rolling out.
func TestNERStatusCountsAFailedAttemptWhileTheRetryWaitsForAnOperator(t *testing.T) {
	request := kindedRequest("bob-request", drbdDigest, deckhousev1alpha1.ExtensionKindSystemExtension)
	request.Spec.Extension.Name = "bob"
	config := nodeConfigWith("worker-0",
		[]internalv1alpha1.Extension{{Name: "bob", Digest: drbdDigest, RequestedBy: nerOwner("bob-request")}}, nil)
	waitingForApproval(config, "uid-0", 4)
	node := nodeInGroup("worker-0", "worker")
	cl := nerStatusCluster(t, request, immutableGroup("worker"), &node, config,
		approvalOf("worker-0", "disruption-worker-0-0", "uid-0", 4, deckhousev1alpha1.NodeOperationPhaseFailed,
			"module example: no example.ko found"),
		approval("disruption-worker-0-1", "uid-0", 4, false, deckhousev1alpha1.NodeOperationPhasePending))

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard(), newPass()))

	status := nerStatusOf(t, cl, request.Name)
	require.Equal(t, int32(1), status.FailedNodes)
	require.Equal(t, int32(0), status.PendingNodes)
	require.Equal(t, phaseDegraded, status.Phase)
	ready := readyOf(t, status)
	require.Equal(t, reasonRefusedByNodes, ready.Reason)
	require.Equal(t, "1 of 1 node(s) refused the system extension "+drbdDigest+", 0 run it: "+
		"node worker-0: NodeOperation disruption-worker-0-0 failed: module example: no example.ko found. "+
		"Attempt 2 of 3, NodeOperation disruption-worker-0-1, waits for an operator to set spec.approved to true",
		ready.Message)
}

// A pending node is named with what it waits for, so an operator knows where to look without
// reading every NodeConfig: no NodeConfig yet, a silent agent, the rollout slot of its group,
// an approval NodeOperation (a retry in flight is a wait, not a refusal), the update window.
func TestNERStatusNamesThePendingNodesAndWhatTheyWaitFor(t *testing.T) {
	request := kindedRequest("bob-request", drbdDigest, deckhousev1alpha1.ExtensionKindSystemExtension)
	request.Spec.Extension.Name = "bob"
	carrying := []internalv1alpha1.Extension{{Name: "bob", Digest: drbdDigest, RequestedBy: nerOwner("bob-request")}}

	silent := nodeConfigWith("worker-b", carrying, nil)
	silent.Status.LastReconcileTime = metav1.NewTime(time.Date(2026, time.January, 1, 8, 0, 0, 0, time.UTC))
	queued := nodeConfigWith("worker-c", nil, nil)
	queued.Labels = map[string]string{nodecommon.NodeGroupLabel: "worker"}
	approving := nodeConfigWith("worker-d", carrying, nil)
	waitingForApproval(approving, "uid-d", 5)
	windowed := nodeConfigWith("worker-e", carrying, []internalv1alpha1.ExtensionStatus{{Name: "bob", Digest: drbdDigest, State: "Pending"}})
	windowed.Status.Conditions = []metav1.Condition{{
		Type: internalv1alpha1.NodeConfigConditionTypeConfigurationApplied, Status: metav1.ConditionFalse, Reason: "Pending",
		Message: "waiting for the update window for bob", LastTransitionTime: metav1.Now(),
	}}
	objects := []client.Object{request, immutableGroup("worker"), silent, queued, approving, windowed,
		approvalOf("worker-d", "disruption-worker-d-0", "uid-d", 5, deckhousev1alpha1.NodeOperationPhaseFailed, "the node did not answer"),
		approvalOf("worker-d", "disruption-worker-d-1", "uid-d", 5, deckhousev1alpha1.NodeOperationPhasePending, ""),
	}
	for _, name := range []string{"worker-a", "worker-b", "worker-c", "worker-d", "worker-e"} {
		node := nodeInGroup(name, "worker")
		objects = append(objects, &node)
	}
	cl := nerStatusCluster(t, objects...)
	p := newPass()
	p.rollouts["worker"] = &rolloutBudget{concurrency: 1, updating: map[string]struct{}{"worker-z": {}}}

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard(), p))

	status := nerStatusOf(t, cl, request.Name)
	require.Equal(t, int32(0), status.FailedNodes, "the failed attempt has a retry in flight")
	require.Equal(t, int32(5), status.PendingNodes)
	ready := readyOf(t, status)
	require.Equal(t, reasonRolloutInProgress, ready.Reason)
	require.Equal(t, "0 of 5 node(s) run "+drbdDigest+"; 5 pending: "+
		"node worker-a: the node has no NodeConfig yet; "+
		"node worker-b: no report since 2026-01-01T08:00:00Z; "+
		"node worker-c: waiting for a rollout slot held by node(s) worker-z; "+
		"node worker-d: waiting for approval by NodeOperation disruption-worker-d-1 in phase Pending; "+
		"node worker-e: the extension is Pending on the node: waiting for the update window for bob",
		ready.Message)
}

// The console reads the request and not the NodeConfigs behind it. The status lists each node
// that has not applied the request with the text the Ready message gives it. Refusals come first
// whatever the names, and a node that runs the request is not listed.
func TestNERStatusListsTheNodesThatHaveNotAppliedIt(t *testing.T) {
	request := kindedRequest("bob-request", drbdDigest, deckhousev1alpha1.ExtensionKindSystemExtension)
	request.Spec.Extension.Name = "bob"
	carrying := []internalv1alpha1.Extension{{Name: "bob", Digest: drbdDigest, RequestedBy: nerOwner("bob-request")}}
	runs := nodeConfigWith("worker-0", carrying, []internalv1alpha1.ExtensionStatus{{Name: "bob", Digest: drbdDigest, State: "Ready"}})
	refused := nodeConfigWith("worker-2", carrying,
		[]internalv1alpha1.ExtensionStatus{{Name: "bob", Digest: drbdDigest, State: "Failed", Message: "Required key not available"}})
	queued := nodeConfigWith("worker-3", nil, nil)
	rejected := nodeConfigWith("worker-4", carrying, nil)
	rejected.Generation, rejected.Status.AppliedGeneration = 4, 3
	rejected.Status.Conditions = []metav1.Condition{{
		Type: internalv1alpha1.NodeConfigConditionTypeConfigurationApplied, Status: metav1.ConditionFalse, Reason: "Rejected",
		ObservedGeneration: 4, Message: "bob: the image is not signed", LastTransitionTime: metav1.Now(),
	}}
	objects := []client.Object{request, immutableGroup("worker"), runs, refused, queued, rejected}
	for i := range 5 {
		node := nodeInGroup(fmt.Sprintf("worker-%d", i), "worker")
		objects = append(objects, &node)
	}
	cl := nerStatusCluster(t, objects...)

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard(), newPass()))

	status := nerStatusOf(t, cl, request.Name)
	require.Equal(t, []deckhousev1alpha1.RequestNodeStatus{
		{Name: "worker-2", State: deckhousev1alpha1.RequestNodeStateFailed, Message: "Required key not available"},
		{Name: "worker-4", State: deckhousev1alpha1.RequestNodeStateFailed, Message: "Rejected: bob: the image is not signed"},
		{Name: "worker-1", State: deckhousev1alpha1.RequestNodeStatePending, Message: "the node has no NodeConfig yet"},
		{Name: "worker-3", State: deckhousev1alpha1.RequestNodeStatePending, Message: "its NodeConfig does not include the request yet"},
	}, status.Nodes)
	require.Equal(t, "2 of 5 node(s) refused the system extension "+drbdDigest+", 1 run it: "+
		"node worker-2: Required key not available; node worker-4: Rejected: bob: the image is not signed",
		readyOf(t, status).Message)
}

var workerNodeName = regexp.MustCompile(`worker-\d+`)

// The list and the Ready message come from the same data in the same place, so every node the
// message names is listed: when the nodes are pending, and when some refused and the list is
// cut at MaxStatusNodes. The counters still give every node.
func TestTheReadyMessageNamesOnlyListedNodes(t *testing.T) {
	const nodeCount = 40
	for _, refusing := range []int{0, 3} {
		t.Run(fmt.Sprintf("%d nodes refuse", refusing), func(t *testing.T) {
			request := kindedRequest("bob-request", drbdDigest, deckhousev1alpha1.ExtensionKindSystemExtension)
			request.Spec.Extension.Name = "bob"
			staticPod := nspr("static-pod", deckhousev1alpha1.NodeStaticPodRequestSpec{})
			objects := []client.Object{request, &staticPod, immutableGroup("worker")}
			var listed []string
			for i := range nodeCount {
				name := fmt.Sprintf("worker-%02d", i)
				node := nodeInGroup(name, "worker")
				objects = append(objects, &node)
				if i < nodeCount-refusing {
					continue
				}
				config := nodeConfigWith(name,
					[]internalv1alpha1.Extension{{Name: "bob", Digest: drbdDigest, RequestedBy: nerOwner("bob-request")}},
					[]internalv1alpha1.ExtensionStatus{{Name: "bob", Digest: drbdDigest, State: "Failed", Message: "Required key not available"}})
				config.Spec.StaticPods = []internalv1alpha1.StaticPod{{Name: "static-pod", Manifest: podManifest("static-pod")}}
				config.Status.StaticPods = []internalv1alpha1.StaticPodStatus{{Name: "static-pod", State: "Failed", Reason: "WriteFailed", Message: "read-only file system"}}
				objects = append(objects, config)
				listed = append(listed, name)
			}
			for i := range deckhousev1alpha1.MaxStatusNodes - refusing {
				listed = append(listed, fmt.Sprintf("worker-%02d", i))
			}
			cl := nerStatusCluster(t, objects...)
			r := nerStatusReconciler(cl)

			require.NoError(t, r.reconcileNERStatuses(t.Context(), logr.Discard(), newPass()))
			require.NoError(t, r.reconcileNSPRStatuses(t.Context(), logr.Discard(), newPass()))

			ner := nerStatusOf(t, cl, request.Name)
			requireListedNodes(t, listed, readyOf(t, ner).Message, ner.Nodes)
			require.Equal(t, int32(refusing), ner.FailedNodes)
			require.Equal(t, int32(nodeCount-refusing), ner.PendingNodes)
			staticPodStatus := nsprStatusOf(t, cl, staticPod.Name)
			requireListedNodes(t, listed, meta.FindStatusCondition(staticPodStatus.Conditions, readyConditionType).Message, staticPodStatus.Nodes)
			require.Equal(t, int32(refusing), staticPodStatus.FailedNodes)
			require.Equal(t, int32(nodeCount-refusing), staticPodStatus.PendingNodes)
		})
	}
}

// requireListedNodes checks status.nodes lists the nodes named by listed, in that order, and holds
// every node the Ready message names.
func requireListedNodes(t *testing.T, listed []string, message string, nodes []deckhousev1alpha1.RequestNodeStatus) {
	t.Helper()
	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		names = append(names, node.Name)
	}
	require.Equal(t, listed, names)
	named := workerNodeName.FindAllString(message, -1)
	require.NotEmpty(t, named, message)
	require.Subset(t, names, named, message)
}

// A request that reaches no node says why, group by group: the names no NodeGroup carries,
// the groups without nodes, and the groups whose nodes the nodeSelector leaves out.
func TestANERThatReachesNoNodeNamesTheGroups(t *testing.T) {
	request := kindedRequest("bob-request", drbdDigest, deckhousev1alpha1.ExtensionKindSystemExtension)
	request.Spec.NodeGroupSelector.MatchNames = []string{"ghost", "empty", "worker"}
	request.Spec.NodeSelector = deckhousev1alpha1.NodeSelector{MatchLabels: map[string]string{"gpu": "true"}}
	node := nodeInGroup("worker-0", "worker")
	cl := nerStatusCluster(t, request, immutableGroup("worker"), immutableGroup("empty"), &node)

	require.NoError(t, nerStatusReconciler(cl).reconcileNERStatuses(t.Context(), logr.Discard(), newPass()))

	status := nerStatusOf(t, cl, request.Name)
	require.Equal(t, phaseReady, status.Phase)
	ready := readyOf(t, status)
	require.Equal(t, reasonNoNodesSelected, ready.Reason)
	require.Equal(t, "the request selects no node: NodeGroup(s) ghost do not exist; NodeGroup(s) empty have no nodes; "+
		"no node of NodeGroup(s) worker has the labels from nodeSelector", ready.Message)
}
