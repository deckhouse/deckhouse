// Copyright 2024 Flant JSC
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

package override

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	addonmodules "github.com/flant/addon-operator/pkg/module_manager/models/modules"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"helm.sh/helm/v3/pkg/releaseutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	installermock "github.com/deckhouse/deckhouse/deckhouse-controller/internal/module/installer/mock"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	d8edition "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/deckhouse/deckhouse/go_lib/d8env"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/testing/controller/controllersuite"
	"github.com/deckhouse/deckhouse/testing/controller/reconcilertest"
)

const (
	repeatCount = 5
	testDigest  = "sha256:cafe"
)

// TestIsRenewRequested pins the renew annotation semantics the controller relies on:
// only the exact value "true" requests a forced redeploy.
func TestIsRenewRequested(t *testing.T) {
	cases := []struct {
		name        string
		annotations map[string]string
		want        bool
	}{
		{"no annotations", nil, false},
		{"renew=true", map[string]string{v1alpha2.ModulePullOverrideAnnotationRenew: "true"}, true},
		{"renew=false", map[string]string{v1alpha2.ModulePullOverrideAnnotationRenew: "false"}, false},
		{"renew=1", map[string]string{v1alpha2.ModulePullOverrideAnnotationRenew: "1"}, false},
		{"renew empty", map[string]string{v1alpha2.ModulePullOverrideAnnotationRenew: ""}, false},
		{"unrelated annotation", map[string]string{"foo": "true"}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mpo := &v1alpha2.ModulePullOverride{ObjectMeta: metav1.ObjectMeta{Annotations: tc.annotations}}
			assert.Equal(t, tc.want, mpo.IsRenewRequested())
		})
	}
}

func TestOverrideControllerTestSuite(t *testing.T) {
	suite.Run(t, new(OverrideControllerTestSuite))
}

type OverrideControllerTestSuite struct {
	controllersuite.Suite

	client client.Client
	ctr    *reconciler

	testDataFileName string
	testMPOName      string
}

func (suite *OverrideControllerTestSuite) SetupSubTest() {
	suite.Suite.SetupSubTest()

	suite.T().Setenv(d8env.DownloadedModulesDir, suite.TmpDir())
	moduleDir := filepath.Join(suite.TmpDir(), "modules")
	err := os.MkdirAll(moduleDir, 0o777)
	if errors.Is(err, os.ErrExist) {
		err = nil
	}
	suite.Check(err)
}

func (suite *OverrideControllerTestSuite) TearDownSubTest() {
	defer suite.Suite.TearDownSubTest()

	if suite.T().Skipped() || suite.T().Failed() {
		return
	}

	if suite.testDataFileName == "" {
		return
	}

	goldenFile := filepath.Join("./testdata/overrides", "golden", suite.testDataFileName)
	reconcilertest.CompareOrUpdate(suite.T(), goldenFile, suite.fetchResults(), reconcilertest.PerDocument)
}

func (suite *OverrideControllerTestSuite) TestHandleModuleOverride() {
	// The module has no Module object yet - the override waits and reports it.
	suite.Run("module not found", func() {
		suite.setupController(suite.fetchTestFileData("module-not-found.yaml"))

		repeatTest(func() {
			_, err := suite.ctr.handleModuleOverride(context.TODO(), suite.getMPO(suite.testMPOName))
			require.NoError(suite.T(), err)
		})
	})

	// An embedded module is served from the image, so the override is a no-op.
	suite.Run("embedded module", func() {
		suite.setupController(suite.fetchTestFileData("embedded-module.yaml"))

		repeatTest(func() {
			_, err := suite.ctr.handleModuleOverride(context.TODO(), suite.getMPO(suite.testMPOName))
			require.NoError(suite.T(), err)
		})
	})

	// A disabled module must not stay pinned to a stale digest: the controller clears
	// status.imageDigest so it is re-downloaded once the module is enabled again.
	suite.Run("disabled module", func() {
		suite.setupController(suite.fetchTestFileData("disabled-module.yaml"))

		repeatTest(func() {
			_, err := suite.ctr.handleModuleOverride(context.TODO(), suite.getMPO(suite.testMPOName))
			require.NoError(suite.T(), err)
		})
	})

	// The module is enabled but has no active source to pull from.
	suite.Run("module without source", func() {
		suite.setupController(suite.fetchTestFileData("no-source.yaml"))

		repeatTest(func() {
			_, err := suite.ctr.handleModuleOverride(context.TODO(), suite.getMPO(suite.testMPOName))
			require.NoError(suite.T(), err)
		})
	})

	// The module points at a source that does not exist.
	suite.Run("source not found", func() {
		suite.setupController(suite.fetchTestFileData("source-not-found.yaml"))

		repeatTest(func() {
			_, err := suite.ctr.handleModuleOverride(context.TODO(), suite.getMPO(suite.testMPOName))
			require.NoError(suite.T(), err)
		})
	})

	// When the registry digest matches status.imageDigest, the module is left as is:
	// nothing is downloaded and no ModuleDocumentation is (re)created.
	suite.Run("up-to-date module", func() {
		suite.setupController(suite.fetchTestFileData("up-to-date.yaml"), withInstaller(&installermock.Installer{
			GetImageDigestFunc: func(context.Context, *v1alpha1.ModuleSource, string, string) (string, error) {
				return testDigest, nil
			},
			DownloadFunc: func(context.Context, *v1alpha1.ModuleSource, string, string) (string, error) {
				return "", errors.New("must not download an up-to-date module")
			},
		}))

		repeatTest(func() {
			_, err := suite.ctr.handleModuleOverride(context.TODO(), suite.getMPO(suite.testMPOName))
			require.NoError(suite.T(), err)
		})
	})

	// The renew annotation forces a redeploy even though the digest is unchanged: the
	// module is downloaded and installed, ModuleDocumentation is (re)created, and the
	// one-shot annotation is dropped so it triggers only once.
	suite.Run("renew forces redeploy", func() {
		moduleDir := filepath.Join(suite.TmpDir(), "downloaded")

		var installed, restarts int
		suite.setupController(suite.fetchTestFileData("renew-forces-redeploy.yaml"), withInstaller(&installermock.Installer{
			// Same digest as status: only the renew annotation can trigger a redeploy.
			GetImageDigestFunc: func(context.Context, *v1alpha1.ModuleSource, string, string) (string, error) {
				return testDigest, nil
			},
			DownloadFunc: func(context.Context, *v1alpha1.ModuleSource, string, string) (string, error) {
				// deployModule removes the returned path, so recreate it per download.
				writeValidModule(suite.T(), moduleDir)
				return moduleDir, nil
			},
			InstallFunc: func(context.Context, string, string, string) error {
				installed++
				return nil
			},
		}), withShutdown(func() error { restarts++; return nil }))

		repeatTest(func() {
			_, err := suite.ctr.handleModuleOverride(context.TODO(), suite.getMPO(suite.testMPOName))
			require.NoError(suite.T(), err)
		})

		assert.Equal(suite.T(), 1, installed, "the module must be installed exactly once")
		assert.Equal(suite.T(), 1, restarts, "Deckhouse must be restarted exactly once")
	})

	// Any renew value other than "true" is ignored, so the annotation is kept and the
	// module is not redeployed.
	suite.Run("renew ignored unless true", func() {
		suite.setupController(suite.fetchTestFileData("renew-ignored.yaml"), withInstaller(&installermock.Installer{
			GetImageDigestFunc: func(context.Context, *v1alpha1.ModuleSource, string, string) (string, error) {
				return testDigest, nil
			},
			DownloadFunc: func(context.Context, *v1alpha1.ModuleSource, string, string) (string, error) {
				return "", errors.New(`must not download: renew value is not "true"`)
			},
		}))

		repeatTest(func() {
			_, err := suite.ctr.handleModuleOverride(context.TODO(), suite.getMPO(suite.testMPOName))
			require.NoError(suite.T(), err)
		})
	})

	// A failed deploy must keep the renew request armed and must not restart Deckhouse,
	// so the re-pull is retried instead of being consumed by the failed attempt.
	suite.Run("renew survives a failed deploy", func() {
		var restarts int
		suite.setupController(suite.fetchTestFileData("renew-deploy-fails.yaml"), withInstaller(&installermock.Installer{
			GetImageDigestFunc: func(context.Context, *v1alpha1.ModuleSource, string, string) (string, error) {
				return testDigest, nil
			},
			DownloadFunc: func(context.Context, *v1alpha1.ModuleSource, string, string) (string, error) {
				return "", errors.New("registry is unavailable")
			},
		}), withShutdown(func() error { restarts++; return nil }))

		repeatTest(func() {
			_, err := suite.ctr.handleModuleOverride(context.TODO(), suite.getMPO(suite.testMPOName))
			require.Error(suite.T(), err)
		})

		assert.True(suite.T(), suite.getMPO(suite.testMPOName).IsRenewRequested(), "the renew annotation must survive")
		assert.Zero(suite.T(), restarts, "Deckhouse must not be restarted when the deploy failed")
	})
}

// setupController seeds the fake cluster from a YAML document and builds a reconciler
// wired with test doubles: a mock installer and a no-op restart hook, so the deploy
// path never touches a registry or signals PID 1.
func (suite *OverrideControllerTestSuite) setupController(yamlDoc string, options ...reconcilerOption) {
	manifests := releaseutil.SplitManifests(yamlDoc)

	initObjects := make([]client.Object, 0, len(manifests))
	for _, manifest := range manifests {
		if obj := suite.assembleInitObject(manifest); obj != nil {
			initObjects = append(initObjects, obj)
		}
	}

	require.NoError(suite.T(), suite.Suite.SetupNoLock(initObjects))

	rec := &reconciler{
		client:              suite.Suite.Client(),
		installer:           &installermock.Installer{},
		log:                 log.NewNop(),
		dependencyContainer: dependency.NewDependencyContainer(),
		moduleManager:       stubModuleManager{},
		edition:             new(d8edition.Edition),
		shutdownFunc:        func() error { return nil },
	}

	for _, option := range options {
		option(rec)
	}

	suite.ctr = rec
	suite.client = suite.Suite.Client()
}

func (suite *OverrideControllerTestSuite) assembleInitObject(manifest string) client.Object {
	raw := []byte(manifest)

	metaType := new(runtime.TypeMeta)
	require.NoError(suite.T(), yaml.Unmarshal(raw, metaType))

	var obj client.Object

	switch metaType.Kind {
	case v1alpha2.ModulePullOverrideGVK.Kind:
		mpo := new(v1alpha2.ModulePullOverride)
		require.NoError(suite.T(), yaml.Unmarshal(raw, mpo))
		suite.testMPOName = mpo.Name
		obj = mpo

	case v1alpha1.ModuleGVK.Kind:
		module := new(v1alpha1.Module)
		require.NoError(suite.T(), yaml.Unmarshal(raw, module))
		obj = module

	case v1alpha1.ModuleSourceGVK.Kind:
		source := new(v1alpha1.ModuleSource)
		require.NoError(suite.T(), yaml.Unmarshal(raw, source))
		obj = source

	case v1alpha1.ModuleDocumentationGVK.Kind:
		documentation := new(v1alpha1.ModuleDocumentation)
		require.NoError(suite.T(), yaml.Unmarshal(raw, documentation))
		obj = documentation
	}

	return obj
}

func (suite *OverrideControllerTestSuite) fetchTestFileData(filename string) string {
	data, err := os.ReadFile(filepath.Join("./testdata/overrides", filename))
	require.NoError(suite.T(), err)

	suite.testDataFileName = filename

	return string(data)
}

func (suite *OverrideControllerTestSuite) getMPO(name string) *v1alpha2.ModulePullOverride {
	mpo := new(v1alpha2.ModulePullOverride)
	require.NoError(suite.T(), suite.client.Get(context.TODO(), client.ObjectKey{Name: name}, mpo))
	return mpo
}

// fetchResults snapshots the objects the override controller touches. The clock-driven
// timestamps it writes (status.updatedAt and the module conditions) are zeroed so the
// golden files stay stable across runs.
func (suite *OverrideControllerTestSuite) fetchResults() []byte {
	got, err := reconcilertest.Snapshot(suite.Context(), suite.client, suite.client.Scheme(), reconcilertest.SnapshotSpec{
		Kinds: []schema.GroupVersionKind{
			v1alpha2.ModulePullOverrideGVK,
			v1alpha1.ModuleGVK,
			v1alpha1.ModuleDocumentationGVK,
		},
		ObjectNormalizers: []reconcilertest.ObjectNormalizer{
			stripDeletionTimestamp,
			stripModuleOverrideUpdatedAt,
			stripModuleConditionTimestamps,
		},
	})
	require.NoError(suite.T(), err)

	return got
}

// --- normalizers ------------------------------------------------------------

func stripDeletionTimestamp(obj client.Object) {
	obj.SetDeletionTimestamp(nil)
}

func stripModuleOverrideUpdatedAt(obj client.Object) {
	if mpo, ok := obj.(*v1alpha2.ModulePullOverride); ok {
		mpo.Status.UpdatedAt = metav1.Time{}
	}
}

func stripModuleConditionTimestamps(obj client.Object) {
	module, ok := obj.(*v1alpha1.Module)
	if !ok {
		return
	}

	for i := range module.Status.Conditions {
		module.Status.Conditions[i].LastProbeTime = metav1.Time{}
		module.Status.Conditions[i].LastTransitionTime = metav1.Time{}
	}
}

// --- options ----------------------------------------------------------------

type reconcilerOption func(*reconciler)

func withInstaller(installer Installer) reconcilerOption {
	return func(r *reconciler) { r.installer = installer }
}

func withShutdown(shutdown func() error) reconcilerOption {
	return func(r *reconciler) { r.shutdownFunc = shutdown }
}

// writeValidModule materializes the minimal on-disk layout Definition.Validate needs
// (an openapi dir with empty schemas), so the deploy path can validate a downloaded
// module without a real registry.
func writeValidModule(t *testing.T, dir string) {
	t.Helper()

	openAPIDir := filepath.Join(dir, "openapi")
	require.NoError(t, os.MkdirAll(openAPIDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(openAPIDir, "config-values.yaml"), []byte("type: object\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(openAPIDir, "values.yaml"), []byte("type: object\n"), 0o644))
}

// stubModuleManager is a no-op moduleManager; the override deploy path only calls
// GetModule (to read config values) after a successful download.
type stubModuleManager struct{}

func (stubModuleManager) DisableModuleHooks(string) {}

func (stubModuleManager) GetModule(string) *addonmodules.BasicModule { return nil }

func (stubModuleManager) RunModuleWithNewOpenAPISchema(string, string, string) error { return nil }

func (stubModuleManager) AreModulesInited() bool { return true }

func repeatTest(fn func()) {
	for range repeatCount {
		fn()
	}
}
