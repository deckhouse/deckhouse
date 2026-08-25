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

package nodegroup

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/register"
)

// The provider registrations are read once per reconcile, before any status is computed. That
// ordering is the whole guard: a failed read used to be indistinguishable from "no cloud provider",
// and a NodeGroup with no provider has no zones — which makes Min and Max zero, and a Min of zero
// makes conditionscalc report the NodeGroup Ready (readySchedulableNodes >= minPerAllZone) no
// matter how many nodes are actually up. A green NodeGroup covering an unreachable API is worse
// than a failed reconcile, which simply retries.
//
// The interceptor is the last-resort tool the node-controller-testing skill allows here: producing
// a Forbidden needs RBAC that envtest does not run.
func newDeniedRegistrationsReconciler(t *testing.T, objs ...runtime.Object) *Status {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		WithStatusSubresource(&v1.NodeGroup{}).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				if _, ok := list.(*corev1.SecretList); ok {
					return apierrors.NewForbidden(
						schema.GroupResource{Resource: "secrets"}, "", context.Canceled)
				}
				return c.List(ctx, list, opts...)
			},
		}).
		Build()

	return &Status{Base: register.Base{Client: cl, Recorder: record.NewFakeRecorder(10)}}
}

func TestReconcile_UnreadableRegistrationsAbortInsteadOfPublishing(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudEphemeral,
			CloudInstances: &v1.CloudInstancesSpec{
				ClassReference: v1.ClassReference{Kind: "YandexInstanceClass", Name: "worker"},
				MinPerZone:     2,
				MaxPerZone:     4,
			},
		},
	}
	r := newDeniedRegistrationsReconciler(t, ng)

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker"},
	})

	require.Error(t, err, "an unreadable registration must fail the reconcile, not yield an empty provider")

	// Nothing was published: had the read been swallowed, Min would be 0 and the group would read
	// Ready with no nodes at all.
	got := &v1.NodeGroup{}
	require.NoError(t, r.Client.Get(context.Background(), types.NamespacedName{Name: "worker"}, got))
	require.Zero(t, got.Status.Min)
	require.Empty(t, got.Status.Conditions)
}
