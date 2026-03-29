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
)

func TestReconcileLinkedSourceExistence(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, deckhousev1alpha2.AddToScheme(scheme))
	require.NoError(t, capi.AddToScheme(scheme))
	require.NoError(t, mcmv1alpha1.AddToScheme(scheme))

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
			name: "node source with missing node",
			instance: &deckhousev1alpha2.Instance{
				ObjectMeta: v1.ObjectMeta{Name: "node-source-missing"},
				Spec: deckhousev1alpha2.InstanceSpec{
					NodeRef: deckhousev1alpha2.NodeRef{Name: "ghost-node"},
				},
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
			name: "machine source missing machine and empty node name",
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
			name: "source none instance",
			instance: &deckhousev1alpha2.Instance{
				ObjectMeta: v1.ObjectMeta{Name: "source-none"},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			objects := append([]client.Object{tc.instance.DeepCopy()}, tc.initialObjects...)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
			svc := &InstanceService{client: fakeClient, machineFactory: machine.NewMachineFactory()}

			deleted, err := svc.reconcileLinkedSourceExistence(context.Background(), tc.instance)
			require.NoError(t, err)
			require.False(t, deleted)

			persisted := &deckhousev1alpha2.Instance{}
			err = fakeClient.Get(context.Background(), types.NamespacedName{Name: tc.instance.Name}, persisted)
			require.False(t, apierrors.IsNotFound(err), "instance %q should not be deleted on skip path", tc.instance.Name)
			require.NoError(t, err)
		})
	}
}
