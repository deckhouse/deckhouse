/*
Copyright 2025 Flant JSC

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

package packagerepository

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/project"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// TestSyncRegistrySettingsChecksum pins when the repository checksum may be committed. Committing it
// retires the fan-out: the next reconcile compares it, finds a match and returns before annotating
// anything. So the checksum must not be written while the change it stands for has reached nobody.
func TestSyncRegistrySettingsChecksum(t *testing.T) {
	const repoName = "deckhouse"

	scheme, err := project.Scheme()
	require.NoError(t, err)

	newRepo := func() *v1alpha1.PackageRepository {
		return &v1alpha1.PackageRepository{
			ObjectMeta: metav1.ObjectMeta{
				Name: repoName,
				// a stale checksum, so the fan-out is due
				Annotations: map[string]string{v1alpha1.PackageRepositoryAnnotationRegistryChecksum: "stale"},
			},
			Spec: v1alpha1.PackageRepositorySpec{
				Registry: v1alpha1.PackageRepositorySpecRegistry{Repo: "registry.example.com/packages", Scheme: "HTTPS"},
			},
		}
	}

	newApp := func(name string) *v1alpha1.Application {
		return &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec:       v1alpha1.ApplicationSpec{PackageName: name, PackageRepositoryName: repoName},
		}
	}

	checksumOf := func(t *testing.T, cl client.Client) string {
		t.Helper()

		repo := new(v1alpha1.PackageRepository)
		require.NoError(t, cl.Get(context.Background(), client.ObjectKey{Name: repoName}, repo))

		return repo.GetAnnotations()[v1alpha1.PackageRepositoryAnnotationRegistryChecksum]
	}

	// failAll refuses to annotate any application, leaving the change with nowhere to land.
	failAll := interceptor.Funcs{
		Patch: func(ctx context.Context, cl client.WithWatch, obj client.Object, p client.Patch, opts ...client.PatchOption) error {
			if _, isApp := obj.(*v1alpha1.Application); isApp {
				return errors.New("boom")
			}

			return cl.Patch(ctx, obj, p, opts...)
		},
	}

	t.Run("not committed when no application could be annotated", func(t *testing.T) {
		repo := newRepo()

		cl := fake.NewClientBuilder().WithScheme(scheme).
			WithObjects(repo, newApp("console"), newApp("commander")).
			WithInterceptorFuncs(failAll).Build()

		r := &reconciler{client: cl, dc: dependency.NewMockedContainer(), logger: log.NewNop()}

		require.Error(t, r.syncRegistrySettings(context.Background(), repo))
		assert.Equal(t, "stale", checksumOf(t, cl),
			"committing here would make the next reconcile skip the fan-out and strand every application on the old registry")
	})

	t.Run("committed when some applications were annotated", func(t *testing.T) {
		repo := newRepo()

		cl := fake.NewClientBuilder().WithScheme(scheme).
			WithObjects(repo, newApp("console"), newApp("commander")).
			WithInterceptorFuncs(interceptor.Funcs{
				Patch: func(ctx context.Context, cl client.WithWatch, obj client.Object, p client.Patch, opts ...client.PatchOption) error {
					if obj.GetName() == "commander" {
						return errors.New("boom")
					}

					return cl.Patch(ctx, obj, p, opts...)
				},
			}).Build()

		r := &reconciler{client: cl, dc: dependency.NewMockedContainer(), logger: log.NewNop()}

		require.NoError(t, r.syncRegistrySettings(context.Background(), repo),
			"one unreachable application must not fail the sync")
		assert.NotEqual(t, "stale", checksumOf(t, cl),
			"replaying the fan-out would re-annotate the applications that did succeed")
	})

	t.Run("committed when the repository has no applications", func(t *testing.T) {
		repo := newRepo()

		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(repo).
			WithInterceptorFuncs(failAll).Build()

		r := &reconciler{client: cl, dc: dependency.NewMockedContainer(), logger: log.NewNop()}

		require.NoError(t, r.syncRegistrySettings(context.Background(), repo))
		assert.NotEqual(t, "stale", checksumOf(t, cl),
			"there is nothing to annotate, so the change is fully applied")
	})
}
