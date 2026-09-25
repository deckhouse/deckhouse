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
)

// A pass that cannot read the fleet knows nothing about it, and zero counts are
// not "nothing": publishing them turned "50 nodes refused this sysext" into a
// clean Ready with the reason erased, on one unlucky listing.
func TestNERStatusIsNotPublishedFromAFleetThatCouldNotBeRead(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))
	require.NoError(t, deckhousev1alpha1.AddToScheme(scheme))
	require.NoError(t, internalv1alpha1.AddToScheme(scheme))
	// The pass lists Nodes for the denominator: without this it would stay green
	// because the scheme refused the listing, not because the fleet read failed.
	require.NoError(t, corev1.AddToScheme(scheme))

	ner := &deckhousev1alpha1.NodeExtensionRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "bob-request"},
		Spec: deckhousev1alpha1.NodeExtensionRequestSpec{
			Sysext: deckhousev1alpha1.Sysext{
				Name:   "bob",
				Digest: "sha256:1111111111111111111111111111111111111111111111111111111111111111",
			},
		},
		Status: deckhousev1alpha1.NodeExtensionRequestStatus{
			Phase:          phaseDegraded,
			FailedNodes:    50,
			FailureMessage: "Required key not available",
			Conditions: []metav1.Condition{{
				Type:               readyConditionType,
				Status:             metav1.ConditionFalse,
				Reason:             reasonRefusedByNodes,
				Message:            "50 node(s) refused the sysext, 0 applied it: Required key not available",
				LastTransitionTime: metav1.Now(),
			}},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ner).
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

	r := &Reconciler{}
	r.Client = cl

	require.Error(t, r.reconcileNERStatuses(t.Context(), logr.Discard()),
		"the pass has to be retried, not counted as a successful report")

	fresh := &deckhousev1alpha1.NodeExtensionRequest{}
	require.NoError(t, cl.Get(t.Context(), types.NamespacedName{Name: ner.Name}, fresh))
	require.Equal(t, phaseDegraded, fresh.Status.Phase)
	require.Equal(t, int32(50), fresh.Status.FailedNodes)
	require.Equal(t, "Required key not available", fresh.Status.FailureMessage)
}

// A NER never reaches a bashible node, so its denominator is the nodes of the
// Immutable groups it matches and nothing else.
func TestNERStatusCountsTheNodesOfTheImmutableGroupsItMatches(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))
	require.NoError(t, deckhousev1alpha1.AddToScheme(scheme))
	require.NoError(t, internalv1alpha1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	ner := &deckhousev1alpha1.NodeExtensionRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "bob-request"},
		Spec: deckhousev1alpha1.NodeExtensionRequestSpec{
			Sysext: deckhousev1alpha1.Sysext{
				Name:   "bob",
				Digest: "sha256:1111111111111111111111111111111111111111111111111111111111111111",
			},
		},
	}
	engineNode := nodeInGroup("worker-0", "worker")
	bashibleNode := nodeInGroup("mutable-0", "mutable-workers")

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			ner,
			immutableGroup("worker"),
			&v1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "mutable-workers"},
				Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral, SystemType: v1.SystemTypeMutable},
			},
			&engineNode, &bashibleNode,
		).
		WithStatusSubresource(&deckhousev1alpha1.NodeExtensionRequest{}).
		Build()

	r := &Reconciler{}
	r.Client = cl
	require.NoError(t, r.reconcileNERStatuses(t.Context(), logr.Discard()))

	fresh := &deckhousev1alpha1.NodeExtensionRequest{}
	require.NoError(t, cl.Get(t.Context(), types.NamespacedName{Name: ner.Name}, fresh))
	require.Equal(t, []string{"worker"}, fresh.Status.MatchedNodeGroups)
	require.Equal(t, int32(1), fresh.Status.MatchedNodes, "a bashible node is not a node a sysext reaches")
	require.Equal(t, int32(1), fresh.Status.PendingNodes)
	require.Equal(t, int32(0), fresh.Status.AppliedNodes)
}

// A request this controller refused was handed to no node, so nobody is late
// with an answer: "Degraded, 2 nodes have not answered" sends an operator
// looking at nodes that were never asked.
func TestNERStatusRefusedHereLeavesNothingPending(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))
	require.NoError(t, deckhousev1alpha1.AddToScheme(scheme))
	require.NoError(t, internalv1alpha1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	ner := &deckhousev1alpha1.NodeExtensionRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "bob-request"},
		Spec: deckhousev1alpha1.NodeExtensionRequestSpec{
			Sysext: deckhousev1alpha1.Sysext{Name: "bob"},
		},
	}
	first := nodeInGroup("worker-0", "worker")
	second := nodeInGroup("worker-1", "worker")

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ner, immutableGroup("worker"), &first, &second).
		WithStatusSubresource(&deckhousev1alpha1.NodeExtensionRequest{}).
		Build()

	r := &Reconciler{}
	r.Client = cl
	require.NoError(t, r.reconcileNERStatuses(t.Context(), logr.Discard()))

	fresh := &deckhousev1alpha1.NodeExtensionRequest{}
	require.NoError(t, cl.Get(t.Context(), types.NamespacedName{Name: ner.Name}, fresh))
	require.Equal(t, reasonInvalidSysext,
		meta.FindStatusCondition(fresh.Status.Conditions, readyConditionType).Reason)
	require.Equal(t, int32(0), fresh.Status.PendingNodes, "nothing was handed to a node")
	require.Equal(t, int32(2), fresh.Status.MatchedNodes, "how many nodes the selector covers is still the answer")
}

// The nodes did get this one, so the arithmetic stands: zeroing pending on every
// Degraded request would lose the node that has not reported yet.
func TestNERStatusRefusedByTheNodesKeepsItsArithmetic(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))
	require.NoError(t, deckhousev1alpha1.AddToScheme(scheme))
	require.NoError(t, internalv1alpha1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	ner := &deckhousev1alpha1.NodeExtensionRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "bob-request"},
		Spec: deckhousev1alpha1.NodeExtensionRequestSpec{
			Sysext: deckhousev1alpha1.Sysext{
				Name:   "bob",
				Digest: "sha256:1111111111111111111111111111111111111111111111111111111111111111",
			},
		},
	}
	first := nodeInGroup("worker-0", "worker")
	second := nodeInGroup("worker-1", "worker")
	silent := nodeInGroup("worker-2", "worker")

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			ner, immutableGroup("worker"), &first, &second, &silent,
			nodeConfigWith("worker-0",
				[]internalv1alpha1.Extension{{Name: "bob", RequestedBy: nerRequestedByPrefix + "bob-request"}},
				[]internalv1alpha1.ExtensionStatus{{Name: "bob", State: "Ready"}}),
			nodeConfigWith("worker-1",
				[]internalv1alpha1.Extension{{Name: "bob", RequestedBy: nerRequestedByPrefix + "bob-request"}},
				[]internalv1alpha1.ExtensionStatus{{
					Name: "bob", State: "Failed", Message: "Required key not available",
				}}),
		).
		WithStatusSubresource(&deckhousev1alpha1.NodeExtensionRequest{}).
		Build()

	r := &Reconciler{}
	r.Client = cl
	require.NoError(t, r.reconcileNERStatuses(t.Context(), logr.Discard()))

	fresh := &deckhousev1alpha1.NodeExtensionRequest{}
	require.NoError(t, cl.Get(t.Context(), types.NamespacedName{Name: ner.Name}, fresh))
	require.Equal(t, reasonRefusedByNodes,
		meta.FindStatusCondition(fresh.Status.Conditions, readyConditionType).Reason)
	require.Equal(t, int32(1), fresh.Status.PendingNodes, "the third node still owes an answer")
}

// The denominator is the nodes the render gives the sysext to, so a request
// narrowed by node labels does not wait for the nodes it never reached.
func TestNERStatusCountsOnlyTheNodesItsLabelsSelect(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))
	require.NoError(t, deckhousev1alpha1.AddToScheme(scheme))
	require.NoError(t, internalv1alpha1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	ner := &deckhousev1alpha1.NodeExtensionRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "bob-request"},
		Spec: deckhousev1alpha1.NodeExtensionRequestSpec{
			Sysext: deckhousev1alpha1.Sysext{
				Name:   "bob",
				Digest: "sha256:1111111111111111111111111111111111111111111111111111111111111111",
			},
			NodeSelector: deckhousev1alpha1.NodeSelector{MatchLabels: map[string]string{"gpu": "true"}},
		},
	}
	selected := nodeInGroup("worker-0", "worker")
	selected.Labels["gpu"] = "true"
	other := nodeInGroup("worker-1", "worker")

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ner, immutableGroup("worker"), &selected, &other).
		WithStatusSubresource(&deckhousev1alpha1.NodeExtensionRequest{}).
		Build()

	r := &Reconciler{}
	r.Client = cl
	require.NoError(t, r.reconcileNERStatuses(t.Context(), logr.Discard()))

	fresh := &deckhousev1alpha1.NodeExtensionRequest{}
	require.NoError(t, cl.Get(t.Context(), types.NamespacedName{Name: ner.Name}, fresh))
	require.Equal(t, int32(1), fresh.Status.MatchedNodes)
	require.Equal(t, int32(1), fresh.Status.PendingNodes)
}
