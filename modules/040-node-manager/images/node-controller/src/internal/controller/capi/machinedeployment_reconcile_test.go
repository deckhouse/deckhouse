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

package capi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/cloudprovider"
	"github.com/deckhouse/node-controller/internal/register"
)

func mdReconciler(t *testing.T, ng *deckhousev1.NodeGroup) (*MachineDeploymentReconciler, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, deckhousev1.AddToScheme(scheme))

	registration := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cloudprovider.SecretNamespace,
			Name:      cloudprovider.SecretNamePrefix,
			Labels:    map[string]string{cloudprovider.SecretLabel: ""},
		},
		Data: map[string][]byte{"type": []byte("yandex")},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(ng, registration,
			clusterConfigSecret("clusterType: Cloud\ncloud:\n  provider: Yandex\n")).
		Build()

	return &MachineDeploymentReconciler{BaseWithReader: BaseWithReader{
		Base:      register.Base{Client: c},
		APIReader: c,
	}}, c
}

func staticNodeGroupDeclaring(providerType string) *deckhousev1.NodeGroup {
	return &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType:     deckhousev1.NodeTypeStatic,
			ProviderType: providerType,
		},
	}
}

// A Static NodeGroup renders no cloud objects, so nothing downstream ever looked at its declared
// provider. The gate in Reconcile is what closes that: a provider that does not resolve stops the
// pass instead of letting it render on a NodeGroup nobody can place.
func TestReconcile_UnresolvedProviderStopsTheRender(t *testing.T) {
	for _, tc := range []struct {
		name     string
		declared string
		wantErr  string
	}{
		{name: "no declaration"},
		{
			// A Static group runs in no cloud, so naming one is wrong even where the cluster has it.
			name: "a provider on Static", declared: "yandex",
			wantErr: "The nodes of this group run in no cloud",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := mdReconciler(t, staticNodeGroupDeclaring(tc.declared))

			_, err := r.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "worker"},
			})

			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

// The finalizer is added before the gate: a NodeGroup that fails it must still be cleaned up when
// it is deleted.
func TestReconcile_FinalizerIsAddedBeforeTheProviderGate(t *testing.T) {
	r, c := mdReconciler(t, staticNodeGroupDeclaring("yandex"))

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker"},
	})
	require.Error(t, err)

	got := &deckhousev1.NodeGroup{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "worker"}, got))
	assert.Contains(t, got.Finalizers, mdCleanupFinalizer)
}
