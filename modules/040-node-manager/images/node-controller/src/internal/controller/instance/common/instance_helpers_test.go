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

package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
)

func TestEnsureInstanceExistsLeavesExistingUnchanged(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	existingMachineRef := &deckhousev1alpha2.MachineRef{
		Kind:       "Machine",
		APIVersion: "machine.sapcloud.io/v1alpha1",
		Name:       "worker-a",
		Namespace:  "d8-cloud-instance-manager",
	}
	existing := &deckhousev1alpha2.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-a"},
		Spec: deckhousev1alpha2.InstanceSpec{
			NodeRef:    deckhousev1alpha2.NodeRef{Name: "worker-a"},
			MachineRef: existingMachineRef,
		},
	}

	c := newFakeInstanceClient(t, existing)

	require.NoError(t, EnsureInstanceExists(ctx, c, "worker-a", deckhousev1alpha2.InstanceSpec{}))

	persisted := &deckhousev1alpha2.Instance{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: "worker-a"}, persisted))
	require.Equal(t, existingMachineRef, persisted.Spec.MachineRef)
	require.Equal(t, deckhousev1alpha2.NodeRef{Name: "worker-a"}, persisted.Spec.NodeRef)
}

func TestEnsureInstanceExistsCreatesWhenMissing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	c := newFakeInstanceClient(t)

	spec := deckhousev1alpha2.InstanceSpec{NodeRef: deckhousev1alpha2.NodeRef{Name: "worker-a"}}
	require.NoError(t, EnsureInstanceExists(ctx, c, "worker-a", spec))

	persisted := &deckhousev1alpha2.Instance{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: "worker-a"}, persisted))
	require.Equal(t, "worker-a", persisted.Spec.NodeRef.Name)
}

func newFakeInstanceClient(t *testing.T, objects ...runtime.Object) client.Client {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, deckhousev1alpha2.AddToScheme(scheme))

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objects...).
		Build()
}
