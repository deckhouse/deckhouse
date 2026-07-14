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

package instance_controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/controller/instance/common/machine"
	"github.com/deckhouse/node-controller/internal/register"
)

func newInterceptedController(t *testing.T, funcs interceptor.Funcs, objects ...client.Object) (*InstanceController, client.Client) {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, deckhousev1alpha2.AddToScheme(scheme))
	require.NoError(t, capiv1beta2.AddToScheme(scheme))
	require.NoError(t, mcmv1alpha1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&deckhousev1alpha2.Instance{}, &capiv1beta2.Machine{}, &mcmv1alpha1.Machine{}).
		WithObjects(objects...).
		WithInterceptorFuncs(funcs).
		Build()

	controller := &InstanceController{Base: register.Base{Client: k8sClient}}
	require.NoError(t, controller.Setup(nil))

	return controller, k8sClient
}

func capiMachineRef(name string) *deckhousev1alpha2.MachineRef {
	return &deckhousev1alpha2.MachineRef{
		Kind:       "Machine",
		APIVersion: capiv1beta2.GroupVersion.String(),
		Name:       name,
		Namespace:  machine.MachineNamespace,
	}
}

func reconcileInstance(ctx context.Context, c *InstanceController, name string) (ctrl.Result, error) {
	return c.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: name}})
}

// TestReconcileNodeRefSelfHealNoMachineRef covers the early-return decision branch of
// reconcileNodeRef: when the instance has neither a NodeRef nor a MachineRef, the spec is
// left untouched and reconcile requeues normally.
func TestReconcileNodeRefSelfHealNoMachineRef(t *testing.T) {
	t.Parallel()

	instance := existingInstanceWithFinalizer("noderef-no-machineref", deckhousev1alpha2.InstanceSpec{}, deckhousev1alpha2.InstancePhaseRunning)

	ctx := ctrl.LoggerInto(context.Background(), ctrl.Log.WithName("test"))
	controller, k8sClient := newInterceptedController(t, interceptor.Funcs{}, instance)

	result, err := reconcileInstance(ctx, controller, instance.Name)
	require.NoError(t, err)
	require.Equal(t, ctrl.Result{RequeueAfter: instanceRequeueInterval}, result)

	persisted := &deckhousev1alpha2.Instance{}
	require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: instance.Name}, persisted))
	require.Empty(t, persisted.Spec.NodeRef.Name)
	require.Nil(t, persisted.Spec.MachineRef)
}

// TestReconcileMachineStatusMachineNotFound covers the NotFound decision branch of
// reconcileMachineStatus: a machine that exists during source-existence but disappears by the
// time the status step resolves it is treated as a no-op, leaving MachineStatus empty.
func TestReconcileMachineStatusMachineNotFound(t *testing.T) {
	t.Parallel()

	instance := existingInstanceWithFinalizer("machine-status-notfound", deckhousev1alpha2.InstanceSpec{
		MachineRef: capiMachineRef("machine-status-notfound"),
		NodeRef:    deckhousev1alpha2.NodeRef{Name: "machine-status-notfound"},
	}, deckhousev1alpha2.InstancePhaseRunning)

	machineObj := capiMachineWithStatus("machine-status-notfound", capiv1beta2.MachineStatus{
		Phase: string(capiv1beta2.MachinePhaseRunning),
	})

	capiGets := 0
	ctx := ctrl.LoggerInto(context.Background(), ctrl.Log.WithName("test"))
	controller, k8sClient := newInterceptedController(t, interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*capiv1beta2.Machine); ok {
				capiGets++
				if capiGets >= 2 {
					return apierrors.NewNotFound(schema.GroupResource{Group: capiv1beta2.GroupVersion.Group, Resource: "machines"}, key.Name)
				}
			}
			return c.Get(ctx, key, obj, opts...)
		},
	}, instance, machineObj)

	result, err := reconcileInstance(ctx, controller, instance.Name)
	require.NoError(t, err)
	require.Equal(t, ctrl.Result{RequeueAfter: instanceRequeueInterval}, result)

	persisted := &deckhousev1alpha2.Instance{}
	require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: instance.Name}, persisted))
	require.Empty(t, persisted.Status.MachineStatus)
}
