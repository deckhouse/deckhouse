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
	"errors"
	"fmt"
	"testing"

	capi "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/controller/instance/common/machine"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

type fakeMachineFactory struct {
	err error
}

var _ machine.MachineFactory = (*fakeMachineFactory)(nil)

func (f *fakeMachineFactory) NewMachine(client.Object) (machine.Machine, error) {
	return nil, fmt.Errorf("unexpected NewMachine call")
}

func (f *fakeMachineFactory) NewMachineFromRef(
	context.Context,
	client.Client,
	*deckhousev1alpha2.MachineRef,
) (machine.Machine, error) {
	return nil, f.err
}

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, deckhousev1alpha2.AddToScheme(scheme))
	require.NoError(t, capi.AddToScheme(scheme))
	require.NoError(t, mcmv1alpha1.AddToScheme(scheme))

	return scheme
}

func TestReconcileLinkedSourceExistence(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)

	testCases := []struct {
		name           string
		instance       *deckhousev1alpha2.Instance
		initialObjects []client.Object
	}{
		{
			name: "node source with existing node",
			instance: &deckhousev1alpha2.Instance{
				ObjectMeta: v1.ObjectMeta{Name: "node-source-existing"},
				Spec: deckhousev1alpha2.InstanceSpec{
					NodeRef: deckhousev1alpha2.NodeRef{Name: "static-node"},
				},
			},
			initialObjects: []client.Object{
				&corev1.Node{ObjectMeta: v1.ObjectMeta{Name: "static-node"}},
			},
		},
		{
			name: "machine source with live machine",
			instance: &deckhousev1alpha2.Instance{
				ObjectMeta: v1.ObjectMeta{Name: "machine-source-live"},
				Spec: deckhousev1alpha2.InstanceSpec{
					MachineRef: &deckhousev1alpha2.MachineRef{
						Kind:       "Machine",
						APIVersion: capi.GroupVersion.String(),
						Name:       "live-machine",
						Namespace:  machine.MachineNamespace,
					},
				},
			},
			initialObjects: []client.Object{
				&capi.Machine{ObjectMeta: v1.ObjectMeta{Name: "live-machine", Namespace: machine.MachineNamespace}},
			},
		},
		{
			name: "source none instance",
			instance: &deckhousev1alpha2.Instance{
				ObjectMeta: v1.ObjectMeta{Name: "source-none"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			objects := append([]client.Object{tc.instance.DeepCopy()}, tc.initialObjects...)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
			svc := &InstanceService{client: fakeClient, machineFactory: machine.NewMachineFactory()}

			result, err := svc.ReconcileSourceExistence(context.Background(), tc.instance)
			require.NoError(t, err)
			require.False(t, result.InstanceDeleted)

			persisted := &deckhousev1alpha2.Instance{}
			err = fakeClient.Get(context.Background(), types.NamespacedName{Name: tc.instance.Name}, persisted)
			require.False(t, apierrors.IsNotFound(err), "instance %q should not be deleted on skip path", tc.instance.Name)
			require.NoError(t, err)
		})
	}
}

func TestReconcileLinkedSourceExistenceDeletes(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)

	testCases := []struct {
		name           string
		instance       *deckhousev1alpha2.Instance
		initialObjects []client.Object
	}{
		{
			name: "node source with missing node",
			instance: &deckhousev1alpha2.Instance{
				ObjectMeta: v1.ObjectMeta{Name: "node-source-missing"},
				Spec: deckhousev1alpha2.InstanceSpec{
					NodeRef: deckhousev1alpha2.NodeRef{Name: "ghost-node"},
				},
			},
		},
		{
			name: "machine source with missing machine and no node ref",
			instance: &deckhousev1alpha2.Instance{
				ObjectMeta: v1.ObjectMeta{Name: "machine-source-missing"},
				Spec: deckhousev1alpha2.InstanceSpec{
					MachineRef: &deckhousev1alpha2.MachineRef{
						Kind:       "Machine",
						APIVersion: capi.GroupVersion.String(),
						Name:       "missing-machine",
						Namespace:  machine.MachineNamespace,
					},
				},
			},
		},
		{
			name: "machine source with missing machine and missing node",
			instance: &deckhousev1alpha2.Instance{
				ObjectMeta: v1.ObjectMeta{Name: "machine-and-node-both-missing"},
				Spec: deckhousev1alpha2.InstanceSpec{
					MachineRef: &deckhousev1alpha2.MachineRef{
						Kind:       "Machine",
						APIVersion: capi.GroupVersion.String(),
						Name:       "missing-machine",
						Namespace:  machine.MachineNamespace,
					},
					NodeRef: deckhousev1alpha2.NodeRef{Name: "missing-node"},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			objects := append([]client.Object{tc.instance.DeepCopy()}, tc.initialObjects...)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
			svc := &InstanceService{client: fakeClient, machineFactory: machine.NewMachineFactory()}

			result, err := svc.ReconcileSourceExistence(context.Background(), tc.instance)
			require.NoError(t, err)
			require.True(t, result.InstanceDeleted)

			persisted := &deckhousev1alpha2.Instance{}
			err = fakeClient.Get(context.Background(), types.NamespacedName{Name: tc.instance.Name}, persisted)
			require.True(t, apierrors.IsNotFound(err), "instance %q should be deleted", tc.instance.Name)
		})
	}
}

func TestReconcileLinkedSourceExistenceErrors(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	errBoom := errors.New("boom")

	testCases := []struct {
		name           string
		instance       *deckhousev1alpha2.Instance
		initialObjects []client.Object
		clientBuilder  func() *fake.ClientBuilder
		machineFactory machine.MachineFactory
		errorContains  string
	}{
		{
			name: "machine factory error",
			instance: &deckhousev1alpha2.Instance{
				ObjectMeta: v1.ObjectMeta{Name: "machine-source-error"},
				Spec: deckhousev1alpha2.InstanceSpec{
					MachineRef: &deckhousev1alpha2.MachineRef{
						Kind:       "Machine",
						APIVersion: capi.GroupVersion.String(),
						Name:       "broken-machine",
						Namespace:  machine.MachineNamespace,
					},
				},
			},
			clientBuilder: func() *fake.ClientBuilder {
				return fake.NewClientBuilder().WithScheme(scheme)
			},
			machineFactory: &fakeMachineFactory{err: errBoom},
			errorContains:  "get machine \"broken-machine\": boom",
		},
		{
			name: "node get error",
			instance: &deckhousev1alpha2.Instance{
				ObjectMeta: v1.ObjectMeta{Name: "node-source-error"},
				Spec: deckhousev1alpha2.InstanceSpec{
					NodeRef: deckhousev1alpha2.NodeRef{Name: "broken-node"},
				},
			},
			clientBuilder: func() *fake.ClientBuilder {
				return fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
					Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						if _, ok := obj.(*corev1.Node); ok {
							return errBoom
						}
						return c.Get(ctx, key, obj, opts...)
					},
				})
			},
			machineFactory: machine.NewMachineFactory(),
			errorContains:  "get node \"broken-node\": boom",
		},
		{
			name: "delete error after missing source confirmed",
			instance: &deckhousev1alpha2.Instance{
				ObjectMeta: v1.ObjectMeta{Name: "delete-error"},
				Spec: deckhousev1alpha2.InstanceSpec{
					NodeRef: deckhousev1alpha2.NodeRef{Name: "missing-node"},
				},
			},
			initialObjects: []client.Object{
				&deckhousev1alpha2.Instance{
					ObjectMeta: v1.ObjectMeta{Name: "delete-error"},
					Spec: deckhousev1alpha2.InstanceSpec{
						NodeRef: deckhousev1alpha2.NodeRef{Name: "missing-node"},
					},
				},
			},
			clientBuilder: func() *fake.ClientBuilder {
				return fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
					Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
						if _, ok := obj.(*deckhousev1alpha2.Instance); ok {
							return errBoom
						}
						return c.Delete(ctx, obj, opts...)
					},
				})
			},
			machineFactory: machine.NewMachineFactory(),
			errorContains:  "delete instance \"delete-error\" with missing source: boom",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			builder := tc.clientBuilder()
			if len(tc.initialObjects) > 0 {
				builder = builder.WithObjects(tc.initialObjects...)
			}
			fakeClient := builder.Build()

			svc := &InstanceService{client: fakeClient, machineFactory: tc.machineFactory}

			result, err := svc.ReconcileSourceExistence(context.Background(), tc.instance)
			require.Error(t, err)
			require.ErrorContains(t, err, tc.errorContains)
			require.False(t, result.InstanceDeleted)
		})
	}

	t.Run("delete error keeps instance", func(t *testing.T) {
		t.Parallel()

		instance := &deckhousev1alpha2.Instance{
			ObjectMeta: v1.ObjectMeta{Name: "delete-error-persisted"},
			Spec: deckhousev1alpha2.InstanceSpec{
				NodeRef: deckhousev1alpha2.NodeRef{Name: "missing-node"},
			},
		}
		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(instance.DeepCopy()).
			WithInterceptorFuncs(interceptor.Funcs{
				Delete: func(context.Context, client.WithWatch, client.Object, ...client.DeleteOption) error {
					return errBoom
				},
			}).
			Build()

		svc := &InstanceService{client: fakeClient, machineFactory: machine.NewMachineFactory()}

		result, err := svc.ReconcileSourceExistence(context.Background(), instance)
		require.Error(t, err)
		require.False(t, result.InstanceDeleted)

		persisted := &deckhousev1alpha2.Instance{}
		err = fakeClient.Get(context.Background(), types.NamespacedName{Name: instance.Name}, persisted)
		require.NoError(t, err)
	})
}

func TestMachineRefName(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		ref  *deckhousev1alpha2.MachineRef
		want string
	}{
		{
			name: "nil ref",
			ref:  nil,
			want: "",
		},
		{
			name: "named ref",
			ref:  &deckhousev1alpha2.MachineRef{Name: "x"},
			want: "x",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, machineRefName(tc.ref))
		})
	}
}

func TestReconcileLinkedSourceExistenceWithBothRefsUsesMachinePriority(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	instance := &deckhousev1alpha2.Instance{
		ObjectMeta: v1.ObjectMeta{Name: "dual-ref-machine-priority"},
		Spec: deckhousev1alpha2.InstanceSpec{
			MachineRef: &deckhousev1alpha2.MachineRef{
				Kind:       "Machine",
				APIVersion: capi.GroupVersion.String(),
				Name:       "missing-machine",
				Namespace:  machine.MachineNamespace,
			},
			NodeRef: deckhousev1alpha2.NodeRef{Name: "existing-node"},
		},
	}
	objects := []client.Object{
		instance.DeepCopy(),
		&corev1.Node{ObjectMeta: v1.ObjectMeta{Name: "existing-node"}},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
	svc := &InstanceService{client: fakeClient, machineFactory: machine.NewMachineFactory()}

	result, err := svc.ReconcileSourceExistence(context.Background(), instance)
	require.NoError(t, err)
	require.True(t, result.InstanceDeleted)

	persisted := &deckhousev1alpha2.Instance{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: instance.Name}, persisted)
	require.True(t, apierrors.IsNotFound(err), "instance %q should be deleted even if NodeRef exists", instance.Name)
}
