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
