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

package release

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// newPackageVersionReconciler builds a reconciler with only the fields
// ensureModulePackageVersion touches.
func newPackageVersionReconciler(t *testing.T, objs ...client.Object) *reconciler {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	return &reconciler{
		client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build(),
		log:    log.NewNop(),
	}
}

func newTestModuleRelease(name, moduleName, version string, labels map[string]string) *v1alpha1.ModuleRelease {
	return &v1alpha1.ModuleRelease{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Spec:       v1alpha1.ModuleReleaseSpec{ModuleName: moduleName, Version: version},
	}
}

// TestEnsureModulePackageVersionCreatesDraft pins the cross-controller
// contract: the promotion pipeline keys off the draft and legacy labels, and
// the module controller resolves versions by the
// <repository>-<package>-<version> name. The release version is normalized to
// "v" + canonical semver on the way.
func TestEnsureModulePackageVersionCreatesDraft(t *testing.T) {
	release := newTestModuleRelease("echo-v1.2.3", "echo", "1.2.3", map[string]string{"source": "test-repo"})
	r := newPackageVersionReconciler(t, release)

	require.NoError(t, r.ensureModulePackageVersion(context.Background(), release))

	mpv := new(v1alpha1.ModulePackageVersion)
	require.NoError(t, r.client.Get(context.Background(), client.ObjectKey{Name: "test-repo-echo-v1.2.3"}, mpv))

	require.Equal(t, map[string]string{
		"heritage": "deckhouse",
		v1alpha1.ModulePackageVersionLabelRepository: "test-repo",
		v1alpha1.ModulePackageVersionLabelPackage:    "echo",
		v1alpha1.ModulePackageVersionLabelDraft:      "true",
		v1alpha1.ModulePackageVersionLabelLegacy:     "true",
	}, mpv.Labels)
	require.Equal(t, v1alpha1.ModulePackageVersionSpec{
		PackageName:           "echo",
		PackageVersion:        "v1.2.3",
		PackageRepositoryName: "test-repo",
	}, mpv.Spec)
}

// TestEnsureModulePackageVersionLeavesExisting verifies idempotency: a version
// that already exists — promoted, without the draft label — is not recreated
// and not flipped back to draft.
func TestEnsureModulePackageVersionLeavesExisting(t *testing.T) {
	release := newTestModuleRelease("echo-v1.0.0", "echo", "v1.0.0", map[string]string{"source": "test-repo"})
	existing := &v1alpha1.ModulePackageVersion{
		ObjectMeta: metav1.ObjectMeta{Name: "test-repo-echo-v1.0.0"},
		Spec: v1alpha1.ModulePackageVersionSpec{
			PackageName:           "echo",
			PackageVersion:        "v1.0.0",
			PackageRepositoryName: "test-repo",
		},
	}
	r := newPackageVersionReconciler(t, release, existing)

	require.NoError(t, r.ensureModulePackageVersion(context.Background(), release))

	mpv := new(v1alpha1.ModulePackageVersion)
	require.NoError(t, r.client.Get(context.Background(), client.ObjectKey{Name: "test-repo-echo-v1.0.0"}, mpv))
	require.Empty(t, mpv.Labels)
}

// TestEnsureModulePackageVersionSkipsUnbridgeable verifies that a release the
// bridge cannot map — no module source, or a non-semver version — creates
// nothing and returns no error, so it is not retried.
func TestEnsureModulePackageVersionSkipsUnbridgeable(t *testing.T) {
	tests := []struct {
		name    string
		release *v1alpha1.ModuleRelease
	}{
		{
			name:    "no module source",
			release: newTestModuleRelease("echo-v1.0.0", "echo", "v1.0.0", nil),
		},
		{
			name:    "non-semver version",
			release: newTestModuleRelease("echo-broken", "echo", "not-a-version", map[string]string{"source": "test-repo"}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newPackageVersionReconciler(t, tt.release)

			require.NoError(t, r.ensureModulePackageVersion(context.Background(), tt.release))

			list := new(v1alpha1.ModulePackageVersionList)
			require.NoError(t, r.client.List(context.Background(), list))
			require.Empty(t, list.Items)
		})
	}
}
