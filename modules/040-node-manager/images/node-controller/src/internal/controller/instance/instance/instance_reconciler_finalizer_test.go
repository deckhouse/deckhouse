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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	instancecommon "github.com/deckhouse/node-controller/internal/controller/instance/common"
	"github.com/deckhouse/node-controller/internal/controller/instance/common/machine"
)

func TestFinalizeAfterMachineDeletion(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)

	t.Run("no finalizer returns without patch", func(t *testing.T) {
		t.Parallel()

		instance := &deckhousev1alpha2.Instance{ObjectMeta: metav1.ObjectMeta{Name: "no-finalizer"}}
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(instance.DeepCopy()).Build()
		svc := &InstanceService{client: c, machineFactory: machine.NewMachineFactory()}

		require.NoError(t, svc.finalizeAfterMachineDeletion(context.Background(), instance, true))
	})

	t.Run("machine still exists keeps finalizer", func(t *testing.T) {
		t.Parallel()

		instance := &deckhousev1alpha2.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "wait-machine-delete",
				Finalizers: []string{instancecommon.InstanceControllerFinalizer},
			},
		}
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(instance.DeepCopy()).Build()
		svc := &InstanceService{client: c, machineFactory: machine.NewMachineFactory()}

		require.NoError(t, svc.finalizeAfterMachineDeletion(context.Background(), instance, false))

		persisted := &deckhousev1alpha2.Instance{}
		err := c.Get(context.Background(), types.NamespacedName{Name: instance.Name}, persisted)
		require.NoError(t, err)
		require.Contains(t, persisted.Finalizers, instancecommon.InstanceControllerFinalizer)
	})

	t.Run("machine gone removes finalizer", func(t *testing.T) {
		t.Parallel()

		instance := &deckhousev1alpha2.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "remove-finalizer",
				Finalizers: []string{instancecommon.InstanceControllerFinalizer},
			},
		}
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(instance.DeepCopy()).Build()
		svc := &InstanceService{client: c, machineFactory: machine.NewMachineFactory()}

		require.NoError(t, svc.finalizeAfterMachineDeletion(context.Background(), instance, true))
		require.NotContains(t, instance.Finalizers, instancecommon.InstanceControllerFinalizer)

		persisted := &deckhousev1alpha2.Instance{}
		err := c.Get(context.Background(), types.NamespacedName{Name: instance.Name}, persisted)
		require.NoError(t, err)
		require.NotContains(t, persisted.Finalizers, instancecommon.InstanceControllerFinalizer)
	})
}

func TestReconcileLinkedMachineDeletion(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)

	t.Run("missing machine ref is treated as machine gone", func(t *testing.T) {
		t.Parallel()

		instance := &deckhousev1alpha2.Instance{ObjectMeta: metav1.ObjectMeta{Name: "no-machine-ref"}}
		c := fake.NewClientBuilder().WithScheme(scheme).Build()
		svc := &InstanceService{client: c, machineFactory: machine.NewMachineFactory()}

		machineGone, err := svc.reconcileLinkedMachineDeletion(context.Background(), instance)
		require.NoError(t, err)
		require.True(t, machineGone)
	})

	t.Run("not found machine is treated as machine gone", func(t *testing.T) {
		t.Parallel()

		instance := &deckhousev1alpha2.Instance{
			ObjectMeta: metav1.ObjectMeta{Name: "missing-machine"},
			Spec: deckhousev1alpha2.InstanceSpec{
				MachineRef: &deckhousev1alpha2.MachineRef{
					Kind:       "Machine",
					APIVersion: "cluster.x-k8s.io/v1beta2",
					Name:       "missing-machine",
					Namespace:  machine.MachineNamespace,
				},
			},
		}
		c := fake.NewClientBuilder().WithScheme(scheme).Build()
		svc := &InstanceService{client: c, machineFactory: &fakeMachineFactory{err: apierrors.NewNotFound(schema.GroupResource{Resource: "machines"}, "missing-machine")}}

		machineGone, err := svc.reconcileLinkedMachineDeletion(context.Background(), instance)
		require.NoError(t, err)
		require.True(t, machineGone)
	})
}
