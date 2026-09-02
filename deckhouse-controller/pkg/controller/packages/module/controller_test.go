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

package module

import (
	"context"
	"sync"
	"testing"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	packageruntime "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime"
	packagestatus "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/ctrlutils"
	"github.com/deckhouse/deckhouse/go_lib/project"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// stubManager records what the reconciler hands to the package runtime.
type stubManager struct {
	removed []string
	updated []string
}

func (s *stubManager) UpdateModulesSettings(string, int, addonutils.Values, string, *bool) {}

func (s *stubManager) UpdateModule(_ registry.Remote, module packageruntime.Module, _ bool) {
	s.updated = append(s.updated, module.Name)
}

func (s *stubManager) GetModuleDigest(context.Context, registry.Remote, string, string) (string, error) {
	return "", nil
}

func (s *stubManager) UpdateEmbeddedModule(module packageruntime.Module) {
	s.updated = append(s.updated, module.Name)
}

func (s *stubManager) RemoveModule(name string) bool {
	s.removed = append(s.removed, name)

	return true
}

func (s *stubManager) RemoveEmbeddedModule(name string) bool {
	s.removed = append(s.removed, name)

	return true
}

func (s *stubManager) GetStatus(string) packagestatus.Status {
	var status packagestatus.Status

	return status
}

func (s *stubManager) GetModuleStatusQueue() workqueue.TypedRateLimitingInterface[string] {
	return nil
}

func newTestReconciler(t *testing.T, objects ...client.Object) (*reconciler, *stubManager, client.Client) {
	t.Helper()

	sc, err := project.Scheme()
	require.NoError(t, err)

	cli := fake.NewClientBuilder().
		WithScheme(sc).
		WithStatusSubresource(&v1alpha1.ModulePackageVersion{}, &v1alpha2.Module{}).
		WithObjects(objects...).
		Build()

	manager := new(stubManager)

	r := &reconciler{
		init:    new(sync.WaitGroup),
		client:  cli,
		manager: manager,
		logger:  log.NewNop(),
	}

	return r, manager, cli
}

func reconcileModule(t *testing.T, r *reconciler, name string) ctrl.Result {
	t.Helper()

	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: name}})
	require.NoError(t, err)

	return res
}

func getModule(t *testing.T, cli client.Client, name string) *v1alpha2.Module {
	t.Helper()

	module := new(v1alpha2.Module)
	require.NoError(t, cli.Get(context.Background(), client.ObjectKey{Name: name}, module))

	return module
}

func TestReconcileNotInstalledModuleIsLeftAlone(t *testing.T) {
	r, manager, cli := newTestReconciler(t, &v1alpha2.Module{
		ObjectMeta: metav1.ObjectMeta{Name: "offered"},
		Spec:       v1alpha2.ModuleSpec{PackageRepositoryName: "deckhouse-modules", ReleaseChannel: "Stable"},
	})

	res := reconcileModule(t, r, "offered")

	assert.Zero(t, res.RequeueAfter, "a module nothing installed needs no retry")
	assert.Empty(t, manager.updated, "the runtime never sees a module nothing installed")
	assert.Equal(t, []string{"offered"}, manager.removed, "a module the runtime ran before its uninstall is torn down")

	module := getModule(t, cli, "offered")
	assert.False(t, controllerutil.ContainsFinalizer(module, v1alpha2.ModuleFinalizerStatisticRegistered))
	assert.Empty(t, module.OwnerReferences)
	assert.Equal(t, "deckhouse-modules", module.Spec.PackageRepositoryName)
	assert.Equal(t, "Stable", module.Spec.ReleaseChannel)
}

func TestReconcileNotInstalledModuleReleasesItsVersion(t *testing.T) {
	versionName := v1alpha1.MakeModulePackageVersionName("deckhouse-modules", "gone", "v1.0.0")

	mpv := &v1alpha1.ModulePackageVersion{
		ObjectMeta: metav1.ObjectMeta{Name: versionName, UID: "mpv-uid"},
		Spec: v1alpha1.ModulePackageVersionSpec{
			PackageName:           "gone",
			PackageRepositoryName: "deckhouse-modules",
			PackageVersion:        "v1.0.0",
		},
		Status: v1alpha1.ModulePackageVersionStatus{Used: true},
	}
	pkg := &v1alpha1.ModulePackage{ObjectMeta: metav1.ObjectMeta{Name: "gone", UID: "pkg-uid"}}

	// the object of a module whose package was uninstalled: the sync cleared the version,
	// the rest is what the controller wrote while the module ran
	module := &v1alpha2.Module{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "gone",
			Finalizers:  []string{v1alpha2.ModuleFinalizerStatisticRegistered},
			Annotations: map[string]string{v1alpha2.ModuleAnnotationHash: "sha256:abc"},
			OwnerReferences: []metav1.OwnerReference{
				ctrlutils.OwnerReference(v1alpha1.ModulePackageVersionGVK, mpv.Name, mpv.UID),
				ctrlutils.OwnerReference(v1alpha1.ModulePackageGVK, pkg.Name, pkg.UID),
			},
		},
		Spec: v1alpha2.ModuleSpec{PackageRepositoryName: "deckhouse-modules"},
	}

	r, _, cli := newTestReconciler(t, module, mpv, pkg)

	res := reconcileModule(t, r, "gone")
	assert.Zero(t, res.RequeueAfter)

	module = getModule(t, cli, "gone")
	assert.False(t, controllerutil.ContainsFinalizer(module, v1alpha2.ModuleFinalizerStatisticRegistered))
	assert.Empty(t, module.OwnerReferences)
	assert.NotContains(t, module.Annotations, v1alpha2.ModuleAnnotationHash)

	mpv = new(v1alpha1.ModulePackageVersion)
	require.NoError(t, cli.Get(context.Background(), client.ObjectKey{Name: versionName}, mpv))
	assert.False(t, mpv.Status.Used, "the version of a module nothing runs is free to go")
}

func TestReconcileInstalledModuleReachesTheRuntime(t *testing.T) {
	versionName := v1alpha1.MakeModulePackageVersionName("deckhouse-modules", "installed", "v1.0.0")

	r, manager, cli := newTestReconciler(t,
		&v1alpha2.Module{
			ObjectMeta: metav1.ObjectMeta{Name: "installed"},
			Spec:       v1alpha2.ModuleSpec{PackageRepositoryName: "deckhouse-modules", PackageVersion: "v1.0.0"},
		},
		&v1alpha1.ModulePackage{ObjectMeta: metav1.ObjectMeta{Name: "installed", UID: "pkg-uid"}},
		&v1alpha1.ModulePackageVersion{
			ObjectMeta: metav1.ObjectMeta{Name: versionName, UID: "mpv-uid"},
			Spec: v1alpha1.ModulePackageVersionSpec{
				PackageName:           "installed",
				PackageRepositoryName: "deckhouse-modules",
				PackageVersion:        "v1.0.0",
			},
		},
		&v1alpha1.PackageRepository{
			ObjectMeta: metav1.ObjectMeta{Name: "deckhouse-modules"},
			Spec:       v1alpha1.PackageRepositorySpec{Registry: v1alpha1.PackageRepositorySpecRegistry{Repo: "registry.example.com/modules"}},
		},
	)

	res := reconcileModule(t, r, "installed")
	assert.Zero(t, res.RequeueAfter)

	assert.Equal(t, []string{"installed"}, manager.updated)
	assert.Empty(t, manager.removed)

	module := getModule(t, cli, "installed")
	assert.True(t, controllerutil.ContainsFinalizer(module, v1alpha2.ModuleFinalizerStatisticRegistered))
	assert.Equal(t, versionName, ctrlutils.OwnerRefName(module, v1alpha1.ModulePackageVersionKind))
	assert.Equal(t, "installed", ctrlutils.OwnerRefName(module, v1alpha1.ModulePackageKind))

	mpv := new(v1alpha1.ModulePackageVersion)
	require.NoError(t, cli.Get(context.Background(), client.ObjectKey{Name: versionName}, mpv))
	assert.True(t, mpv.Status.Used)
}
