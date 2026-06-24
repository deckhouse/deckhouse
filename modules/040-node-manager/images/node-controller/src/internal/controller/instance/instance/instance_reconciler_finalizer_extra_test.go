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

package instance

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	capi "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	instancecommon "github.com/deckhouse/node-controller/internal/controller/instance/common"
	"github.com/deckhouse/node-controller/internal/controller/instance/common/machine"
)

func TestNewInstanceService(t *testing.T) {
	t.Parallel()

	c := fake.NewClientBuilder().WithScheme(newTestScheme(t)).Build()
	svc := NewInstanceService(c)
	require.NotNil(t, svc)
	require.Equal(t, c, svc.client)
	require.NotNil(t, svc.machineFactory)
}

func TestEnsureInstanceFinalizer(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)

	t.Run("adds finalizer when missing", func(t *testing.T) {
		t.Parallel()

		instance := &deckhousev1alpha2.Instance{ObjectMeta: metav1.ObjectMeta{Name: "add-finalizer"}}
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(instance.DeepCopy()).Build()
		svc := &InstanceService{client: c, machineFactory: machine.NewMachineFactory()}

		require.NoError(t, svc.EnsureInstanceFinalizer(context.Background(), instance))
		require.Contains(t, instance.Finalizers, instancecommon.InstanceControllerFinalizer)

		persisted := &deckhousev1alpha2.Instance{}
		require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: instance.Name}, persisted))
		require.Contains(t, persisted.Finalizers, instancecommon.InstanceControllerFinalizer)
	})

	t.Run("no-op when finalizer already present", func(t *testing.T) {
		t.Parallel()

		instance := &deckhousev1alpha2.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "has-finalizer",
				Finalizers: []string{instancecommon.InstanceControllerFinalizer},
			},
		}
		patchCalled := false
		c := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(instance.DeepCopy()).
			WithInterceptorFuncs(interceptor.Funcs{
				Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
					patchCalled = true
					return c.Patch(ctx, obj, patch, opts...)
				},
			}).
			Build()
		svc := &InstanceService{client: c, machineFactory: machine.NewMachineFactory()}

		require.NoError(t, svc.EnsureInstanceFinalizer(context.Background(), instance))
		require.False(t, patchCalled)
	})
}

func TestReconcileLinkedMachineDeletionDeletesLiveMachine(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	capiMachineObj := &capi.Machine{ObjectMeta: metav1.ObjectMeta{Name: "live-machine", Namespace: machine.MachineNamespace}}
	instance := &deckhousev1alpha2.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "live-machine"},
		Spec: deckhousev1alpha2.InstanceSpec{
			MachineRef: &deckhousev1alpha2.MachineRef{
				Kind:       "Machine",
				APIVersion: capi.GroupVersion.String(),
				Name:       "live-machine",
				Namespace:  machine.MachineNamespace,
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(instance.DeepCopy(), capiMachineObj).Build()
	svc := &InstanceService{client: c, machineFactory: machine.NewMachineFactory()}

	result, err := svc.reconcileLinkedMachineDeletion(context.Background(), instance)
	require.NoError(t, err)
	require.False(t, result.MachineGone, "delete just issued, machine not yet gone")
}

func TestReconcileFinalization(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)

	t.Run("no machine ref removes finalizer and reports machine gone", func(t *testing.T) {
		t.Parallel()

		instance := &deckhousev1alpha2.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "finalize-no-ref",
				Finalizers: []string{instancecommon.InstanceControllerFinalizer},
			},
		}
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(instance.DeepCopy()).Build()
		svc := &InstanceService{client: c, machineFactory: machine.NewMachineFactory()}

		result, err := svc.ReconcileFinalization(context.Background(), instance)
		require.NoError(t, err)
		require.True(t, result.MachineGone)
		require.NotContains(t, instance.Finalizers, instancecommon.InstanceControllerFinalizer)
	})

	t.Run("machine still present keeps finalizer", func(t *testing.T) {
		t.Parallel()

		capiMachineObj := &capi.Machine{ObjectMeta: metav1.ObjectMeta{Name: "alive", Namespace: machine.MachineNamespace}}
		instance := &deckhousev1alpha2.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "finalize-keep",
				Finalizers: []string{instancecommon.InstanceControllerFinalizer},
			},
			Spec: deckhousev1alpha2.InstanceSpec{
				MachineRef: &deckhousev1alpha2.MachineRef{
					Kind:       "Machine",
					APIVersion: capi.GroupVersion.String(),
					Name:       "alive",
					Namespace:  machine.MachineNamespace,
				},
			},
		}
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(instance.DeepCopy(), capiMachineObj).Build()
		svc := &InstanceService{client: c, machineFactory: machine.NewMachineFactory()}

		result, err := svc.ReconcileFinalization(context.Background(), instance)
		require.NoError(t, err)
		require.False(t, result.MachineGone)
		require.Contains(t, instance.Finalizers, instancecommon.InstanceControllerFinalizer)
	})
}
