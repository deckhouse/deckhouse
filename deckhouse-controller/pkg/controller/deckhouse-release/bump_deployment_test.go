// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package deckhouse_release

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/pkg/app"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// TestBumpDeckhouseDeployment_FieldOwner verifies that bumping the deckhouse
// Deployment image is performed via server-side apply and that this controller
// registers as the "deckhouse-release-controller" field manager owning only the
// release container image — never the whole Deployment.
func TestBumpDeckhouseDeployment_FieldOwner(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))

	depl := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.DeploymentName,
			Namespace: app.NamespaceDeckhouse,
		},
		Spec: appsv1.DeploymentSpec{
			// replicas is owned by another actor and must stay untouched by the bump.
			Replicas: ptr.To(int32(3)),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "deckhouse", Image: "my.registry.com/deckhouse:v1.0.0"}},
				},
			},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(depl).
		// Ask the fake client to keep managedFields on read so we can assert ownership.
		WithReturnManagedFields().
		Build()

	r := &deckhouseReleaseReconciler{
		client:         cl,
		logger:         log.NewNop(),
		registrySecret: &utils.DeckhouseRegistrySecret{ImageRegistry: "my.registry.com/deckhouse"},
	}

	dr := &v1alpha1.DeckhouseRelease{
		ObjectMeta: metav1.ObjectMeta{Name: "v1.2.3"},
		Spec:       v1alpha1.DeckhouseReleaseSpec{Version: "v1.2.3"},
	}

	require.NoError(t, r.bumpDeckhouseDeployment(context.Background(), dr))

	got := &appsv1.Deployment{}
	require.NoError(t, cl.Get(context.Background(), client.ObjectKey{
		Namespace: app.NamespaceDeckhouse,
		Name:      app.DeploymentName,
	}, got))

	// The image was bumped to the release version.
	require.Len(t, got.Spec.Template.Spec.Containers, 1)
	require.Equal(t, "my.registry.com/deckhouse:v1.2.3", got.Spec.Template.Spec.Containers[0].Image)
	// Fields we do not manage stay intact.
	require.Equal(t, int32(3), *got.Spec.Replicas)

	// Our field manager exists, applied via SSA, and owns exactly the container image.
	entry := findManagedFields(got.ManagedFields, fieldOwner)
	require.NotNil(t, entry, "expected a managedFields entry for %q, got %+v", fieldOwner, got.ManagedFields)
	require.Equal(t, metav1.ManagedFieldsOperationApply, entry.Operation)

	require.NotNil(t, entry.FieldsV1)
	ownedFields := string(entry.FieldsV1.Raw)
	require.Contains(t, ownedFields, `"f:image"`)
	// The field manager must NOT claim ownership of replicas.
	require.NotContains(t, ownedFields, `"f:replicas"`)
}

func findManagedFields(entries []metav1.ManagedFieldsEntry, manager string) *metav1.ManagedFieldsEntry {
	for i := range entries {
		if entries[i].Manager == manager {
			return &entries[i]
		}
	}

	return nil
}
