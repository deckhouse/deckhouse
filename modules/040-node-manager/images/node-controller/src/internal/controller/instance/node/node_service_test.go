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

package node

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	instancecommon "github.com/deckhouse/node-controller/internal/controller/instance/common"
	"github.com/deckhouse/node-controller/internal/controller/instance/common/machine"
)

func TestDeleteNodeBasedInstanceIfExistsSkipsMachineBackedInstance(t *testing.T) {
	t.Parallel()

	c := newNodeTestClient(t, &deckhousev1alpha2.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "node-a",
			Finalizers: []string{instancecommon.InstanceControllerFinalizer},
		},
		Spec: deckhousev1alpha2.InstanceSpec{
			NodeRef: deckhousev1alpha2.NodeRef{Name: "node-a"},
			MachineRef: &deckhousev1alpha2.MachineRef{
				Kind:       "Machine",
				APIVersion: "cluster.x-k8s.io/v1beta2",
				Name:       "node-a",
				Namespace:  machine.MachineNamespace,
			},
		},
	})

	result, err := deleteNodeBasedInstanceIfExists(context.Background(), c, "node-a")
	require.NoError(t, err)
	require.False(t, result.InstanceDeleted)

	persisted := &deckhousev1alpha2.Instance{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "node-a"}, persisted)
	require.NoError(t, err)
	require.Contains(t, persisted.Finalizers, instancecommon.InstanceControllerFinalizer)
}

func TestDeleteNodeBasedInstanceIfExistsSkipsNodeMismatch(t *testing.T) {
	t.Parallel()

	c := newNodeTestClient(t, &deckhousev1alpha2.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "node-a",
			Finalizers: []string{instancecommon.InstanceControllerFinalizer},
		},
		Spec: deckhousev1alpha2.InstanceSpec{
			NodeRef: deckhousev1alpha2.NodeRef{Name: "other-node"},
		},
	})

	result, err := deleteNodeBasedInstanceIfExists(context.Background(), c, "node-a")
	require.NoError(t, err)
	require.False(t, result.InstanceDeleted)

	persisted := &deckhousev1alpha2.Instance{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "node-a"}, persisted)
	require.NoError(t, err)
	require.Contains(t, persisted.Finalizers, instancecommon.InstanceControllerFinalizer)
}

func TestDeleteNodeBasedInstanceIfExistsRemovesFinalizerBeforeDelete(t *testing.T) {
	t.Parallel()

	scheme := newNodeTestScheme(t)
	instance := &deckhousev1alpha2.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "node-a",
			Finalizers: []string{instancecommon.InstanceControllerFinalizer},
		},
		Spec: deckhousev1alpha2.InstanceSpec{
			NodeRef: deckhousev1alpha2.NodeRef{Name: "node-a"},
		},
	}

	deleteSawFinalizer := false
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(instance.DeepCopy()).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
				instanceObj, ok := obj.(*deckhousev1alpha2.Instance)
				if ok {
					deleteSawFinalizer = len(instanceObj.Finalizers) > 0
				}
				return c.Delete(ctx, obj, opts...)
			},
		}).
		Build()

	result, err := deleteNodeBasedInstanceIfExists(context.Background(), c, "node-a")
	require.NoError(t, err)
	require.True(t, result.InstanceDeleted)
	require.False(t, deleteSawFinalizer)

	persisted := &deckhousev1alpha2.Instance{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "node-a"}, persisted)
	require.True(t, apierrors.IsNotFound(err))
}

func newNodeTestClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()

	return fake.NewClientBuilder().
		WithScheme(newNodeTestScheme(t)).
		WithObjects(objects...).
		Build()
}

func newNodeTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, deckhousev1alpha2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	return scheme
}
