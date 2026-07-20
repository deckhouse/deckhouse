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

package modulepackageversion

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/project"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/testing/controller/reconcilertest"
)

func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}

type ControllerTestSuite struct {
	reconcilertest.Suite

	ctr *reconciler
}

func (suite *ControllerTestSuite) SetupSuite() {
	suite.Init(reconcilertest.Config{
		StatusSubresources: []client.Object{&v1alpha1.ModulePackageVersion{}},
		SnapshotKinds: []schema.GroupVersionKind{
			v1alpha1.SchemeGroupVersion.WithKind("ModulePackageVersion"),
			v1alpha1.SchemeGroupVersion.WithKind("PackageRepository"),
		},
		GoldenMode:    reconcilertest.PerDocument,
		SeedViaCreate: true,
	})
}

func (suite *ControllerTestSuite) SetupSubTest() {
	reconcilertest.RespondHTTPOK()
}

type reconcilerOption func(*reconciler)

func withDependencyContainer(dc dependency.Container) reconcilerOption {
	return func(r *reconciler) {
		r.dc = dc
		r.registry = registry.NewService(dc, log.NewNop())
	}
}

func newReconciler(cl client.Client, dc dependency.Container) *reconciler {
	return &reconciler{
		client:   cl,
		logger:   log.NewNop(),
		dc:       dc,
		registry: registry.NewService(dc, log.NewNop()),
	}
}

func (suite *ControllerTestSuite) setupController(filename string, options ...reconcilerOption) {
	suite.Seed(filename)
	suite.ctr = newReconciler(suite.Client(), dependency.NewDependencyContainer())

	for _, opt := range options {
		opt(suite.ctr)
	}
}

// setupFakeController builds a reconciler and a seeded client for standalone
// (non-suite) tests, reusing the framework's fixture decoding.
func setupFakeController(t *testing.T, filename string) (*reconciler, client.Client) {
	t.Helper()

	sc, err := project.Scheme()
	require.NoError(t, err)

	raw, err := reconcilertest.LoadFixture("./testdata", filename)
	require.NoError(t, err)

	objs, err := reconcilertest.Decode(sc, raw)
	require.NoError(t, err)

	cl := fake.NewClientBuilder().
		WithScheme(sc).
		WithStatusSubresource(&v1alpha1.ModulePackageVersion{}).
		Build()
	for _, obj := range objs {
		require.NoError(t, cl.Create(context.TODO(), obj))
	}

	return newReconciler(cl, dependency.NewDependencyContainer()), cl
}

const testModuleV2YAML = `name: test-module
descriptions:
  en: Test module
disable:
  confirmation: true
  messages:
    ru: "RU disable message"
    en: "EN disable message"
stage: GA
type: Module
version: "1.0.0"
`

func (suite *ControllerTestSuite) TestReconcile() {
	ctx := context.Background()

	dependency.TestDC.CRClient.ImageMock.Return(reconcilertest.Image(nil), nil)

	suite.Run("resource not found", func() {
		suite.setupController("resource-not-found.yaml")
		_, err := suite.ctr.Reconcile(ctx, suite.Request("non-existent-mpv", ""))
		require.NoError(suite.T(), err)
	})

	suite.Run("successful reconcile with v2 package metadata", func() {
		dc := dependency.NewMockedContainer()
		dc.CRClient.ImageMock.Return(reconcilertest.Image(map[string]string{
			"package.yaml":   testModuleV2YAML,
			"version.json":   `{"version": "1.0.0"}`,
			"changelog.yaml": "features:\n- Added new feature\nfixes:\n- Fixed a bug\n",
		}), nil)
		dc.CRClient.DigestMock.Return("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", nil)

		suite.setupController("successful-reconcile-v2.yaml", withDependencyContainer(dc))

		mpv := suite.getModulePackageVersion("deckhouse-test-module-v1.0.0")
		_, err := suite.ctr.Reconcile(ctx, suite.Request(mpv.Name, ""))
		require.NoError(suite.T(), err)
	})

	suite.Run("successful reconcile with legacy module metadata", func() {
		dc := dependency.NewMockedContainer()
		dc.CRClient.ImageMock.Return(reconcilertest.Image(map[string]string{
			"module.yaml": `name: test-module
descriptions:
  en: Legacy test module
stage: Sandbox
requirements:
  deckhouse: ">= 1.60"
  kubernetes: ">= 1.27"
`,
			"version.json":   `{"version": "1.0.0"}`,
			"changelog.yaml": "features:\n- Legacy feature\n",
		}), nil)
		dc.CRClient.DigestMock.Return("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", nil)

		suite.setupController("successful-reconcile-legacy.yaml", withDependencyContainer(dc))

		mpv := suite.getModulePackageVersion("deckhouse-test-module-v1.0.0")
		_, err := suite.ctr.Reconcile(ctx, suite.Request(mpv.Name, ""))
		require.NoError(suite.T(), err)
	})

	suite.Run("legacy module from old registry", func() {
		dc := dependency.NewMockedContainer()
		dc.CRClient.ImageMock.Return(reconcilertest.Image(map[string]string{
			"module.yaml": `name: test-module
descriptions:
  en: Legacy module from old registry
stage: Sandbox
`,
			"version.json": `{"version": "1.0.0"}`,
		}), nil)
		dc.CRClient.DigestMock.Return("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", nil)

		suite.setupController("release-path-segment.yaml", withDependencyContainer(dc))

		mpv := suite.getModulePackageVersion("deckhouse-test-module-v1.0.0")
		_, err := suite.ctr.Reconcile(ctx, suite.Request(mpv.Name, ""))
		require.NoError(suite.T(), err)
	})

	suite.Run("registry error reconcile", func() {
		dc := dependency.NewMockedContainer()
		dc.CRClient.ImageMock.Return(nil, fmt.Errorf("registry error"))

		suite.setupController("registry-error-reconcile.yaml", withDependencyContainer(dc))

		mpv := suite.getModulePackageVersion("deckhouse-test-module-v1.0.0")
		_, err := suite.ctr.Reconcile(ctx, suite.Request(mpv.Name, ""))
		require.Error(suite.T(), err)
	})

	suite.Run("non-draft resource skip", func() {
		suite.setupController("non-draft-resource.yaml")
		mpv := suite.getModulePackageVersion("deckhouse-test-module-v1.0.0")
		_, err := suite.ctr.Reconcile(ctx, suite.Request(mpv.Name, ""))
		require.NoError(suite.T(), err)
	})
}

func (suite *ControllerTestSuite) getModulePackageVersion(name string) *v1alpha1.ModulePackageVersion {
	var mpv v1alpha1.ModulePackageVersion
	err := suite.Client().Get(context.TODO(), client.ObjectKey{Name: name}, &mpv)
	require.NoError(suite.T(), err)
	return &mpv
}

func TestDeleteBlockedByUsedByCount(t *testing.T) {
	ctx := context.Background()
	ctr, kubeClient := setupFakeController(t, "delete-with-used-by.yaml")

	mpvName := "deckhouse-test-module-v1.0.0"

	// First reconcile adds the finalizer
	_, err := ctr.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Name: mpvName}})
	require.NoError(t, err)

	// Trigger deletion (fake client sets DeletionTimestamp because finalizer exists)
	var mpv v1alpha1.ModulePackageVersion
	require.NoError(t, kubeClient.Get(ctx, client.ObjectKey{Name: mpvName}, &mpv))
	require.NoError(t, kubeClient.Delete(ctx, &mpv))

	// Reconcile should block deletion because UsedByCount > 0
	result, err := ctr.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Name: mpvName}})
	require.NoError(t, err)
	assert.Equal(t, 15*time.Second, result.RequeueAfter, "should requeue when UsedByCount > 0")

	// Verify finalizer is still present (object not deleted)
	var updated v1alpha1.ModulePackageVersion
	require.NoError(t, kubeClient.Get(ctx, client.ObjectKey{Name: mpvName}, &updated))
	assert.Contains(t, updated.Finalizers, v1alpha1.ModulePackageVersionFinalizer)
}

func TestDeleteSucceedsWhenUnused(t *testing.T) {
	ctx := context.Background()
	ctr, kubeClient := setupFakeController(t, "delete-unused.yaml")

	mpvName := "deckhouse-test-module-v1.0.0"

	// First reconcile adds the finalizer
	_, err := ctr.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Name: mpvName}})
	require.NoError(t, err)

	// Trigger deletion
	var mpv v1alpha1.ModulePackageVersion
	require.NoError(t, kubeClient.Get(ctx, client.ObjectKey{Name: mpvName}, &mpv))
	require.NoError(t, kubeClient.Delete(ctx, &mpv))

	// Reconcile should remove finalizer because UsedByCount == 0
	result, err := ctr.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Name: mpvName}})
	require.NoError(t, err)
	assert.Zero(t, result.RequeueAfter, "should not requeue when unused")

	// Object should be fully deleted (finalizer removed, fake client garbage-collected it)
	var updated v1alpha1.ModulePackageVersion
	err = kubeClient.Get(ctx, client.ObjectKey{Name: mpvName}, &updated)
	assert.True(t, apierrors.IsNotFound(err), "object should be deleted after finalizer removal")
}

func TestSetFromModuleDefinitionMapsLegacyAccessibilityToLicensing(t *testing.T) {
	mpv := new(v1alpha1.ModulePackageVersion)
	def := &moduletypes.Definition{
		Accessibility: &moduletypes.ModuleAccessibility{
			Editions: map[string]moduletypes.ModuleEdition{
				"_default": {
					Available:        true,
					EnabledInBundles: []string{"Default"},
				},
				"ee": {
					Available:        false,
					EnabledInBundles: []string{"Minimal", "Managed"},
				},
			},
		},
	}

	setFromModuleDefinition(mpv, def)

	require.NotNil(t, mpv.Status.PackageMetadata)
	require.NotNil(t, mpv.Status.PackageMetadata.Licensing)
	assert.True(t, mpv.Status.PackageMetadata.Licensing.Editions["_default"].Available)
	assert.Equal(t, []string{"Default"}, mpv.Status.PackageMetadata.Licensing.Editions["_default"].EnabledInBundles)
	assert.False(t, mpv.Status.PackageMetadata.Licensing.Editions["ee"].Available)
	assert.Equal(t, []string{"Minimal", "Managed"}, mpv.Status.PackageMetadata.Licensing.Editions["ee"].EnabledInBundles)
}
