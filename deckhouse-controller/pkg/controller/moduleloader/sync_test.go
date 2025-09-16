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

package moduleloader

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	addonmodules "github.com/flant/addon-operator/pkg/module_manager/models/modules"
	crv1 "github.com/google/go-containerregistry/pkg/v1"
	crfake "github.com/google/go-containerregistry/pkg/v1/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"helm.sh/helm/v3/pkg/releaseutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/downloader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/go_lib/d8env"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	values = `
type: object
x-extend:
  schema: config-values.yaml
properties:
  registry:
    type: object
    default: {}
    properties:
      base:
        type: string
        default: dev-registry.deckhouse.io/deckhouse/losev/external-modules
      dockercfg:
        type: string
        default: YXNiCg==
      scheme:
        type: string
        default: HTTP
      ca:
        type: string
        default:
  internal:
    default: {}
    properties:
      pythonVersions:
        default: []
        items:
          type: string
        type: array
    type: object`
)

type ModuleLoaderTestSuite struct {
	suite.Suite

	client client.Client
	loader *Loader

	testDataFileName string

	tmpDir string
}

func TestModuleLoaderTestSuite(t *testing.T) {
	suite.Run(t, new(ModuleLoaderTestSuite))
}

func (suite *ModuleLoaderTestSuite) setupModuleLoader(raw string) {
	manifests := releaseutil.SplitManifests(raw)

	var objects = make([]client.Object, 0, len(manifests))
	for _, manifest := range manifests {
		obj := suite.parseKubernetesObject([]byte(manifest))
		objects = append(objects, obj)
	}

	sc := runtime.NewScheme()
	_ = v1alpha1.SchemeBuilder.AddToScheme(sc)
	_ = v1alpha2.SchemeBuilder.AddToScheme(sc)
	_ = corev1.AddToScheme(sc)
	suite.client = fake.NewClientBuilder().
		WithScheme(sc).
		WithObjects(objects...).
		WithStatusSubresource(&v1alpha1.ModuleRelease{}, &v1alpha1.ModuleSource{}, &v1alpha2.ModulePullOverride{}).
		Build()

	suite.loader = &Loader{
		client:               suite.client,
		logger:               log.NewNop(),
		downloadedModulesDir: d8env.GetDownloadedModulesDir(),
		symlinksDir:          filepath.Join(d8env.GetDownloadedModulesDir(), "modules"),
		dependencyContainer:  dependency.NewDependencyContainer(),
		registries:           make(map[string]*addonmodules.Registry),
	}
}

func (suite *ModuleLoaderTestSuite) parseKubernetesObject(raw []byte) client.Object {
	metaType := new(runtime.TypeMeta)
	err := yaml.Unmarshal(raw, metaType)
	require.NoError(suite.T(), err)

	var obj client.Object

	switch metaType.Kind {
	case v1alpha1.ModuleSourceGVK.Kind:
		source := new(v1alpha1.ModuleSource)
		err = yaml.Unmarshal(raw, source)
		require.NoError(suite.T(), err)
		obj = source

	case v1alpha1.ModuleReleaseGVK.Kind:
		release := new(v1alpha1.ModuleRelease)
		err = yaml.Unmarshal(raw, release)
		require.NoError(suite.T(), err)
		obj = release

	case v1alpha2.ModuleUpdatePolicyGVK.Kind:
		policy := new(v1alpha2.ModuleUpdatePolicy)
		err = yaml.Unmarshal(raw, policy)
		require.NoError(suite.T(), err)
		obj = policy

	case v1alpha2.ModulePullOverrideGVK.Kind:
		mpo := new(v1alpha2.ModulePullOverride)
		err = yaml.Unmarshal(raw, mpo)
		require.NoError(suite.T(), err)
		obj = mpo

	case v1alpha1.ModuleGVK.Kind:
		module := new(v1alpha1.Module)
		err = yaml.Unmarshal(raw, module)
		require.NoError(suite.T(), err)
		obj = module
	}

	return obj
}

func (suite *ModuleLoaderTestSuite) SetupSuite() {
	flag.Parse()
	suite.T().Setenv("D8_IS_TESTS_ENVIRONMENT", "true")
	suite.T().Setenv("DECKHOUSE_NODE_NAME", "dev-master-0")
	suite.tmpDir = suite.T().TempDir()
	suite.T().Setenv(d8env.DownloadedModulesDir, suite.tmpDir)
	_ = os.MkdirAll(filepath.Join(suite.tmpDir, "modules"), 0777)
}

func (suite *ModuleLoaderTestSuite) TestRestoreAbsentModulesFromOverrides() {
	module := moduleSuite{
		name:          "echo",
		version:       downloader.DefaultDevVersion,
		weight:        900,
		downloadedDir: suite.tmpDir,
	}

	manifestStub := func() (*crv1.Manifest, error) {
		return &crv1.Manifest{
			Layers: []crv1.Descriptor{},
		}, nil
	}

	type testCase struct {
		name           string
		filename       string
		layersStab     func() ([]crv1.Layer, error)
		symlinkChanged bool
		valuesChanged  bool
		checkValues    bool
	}

	testCases := []testCase{
		{
			// should not do anything
			name:           "Ok",
			filename:       "mpo.yaml",
			symlinkChanged: false,
			valuesChanged:  false,
			checkValues:    true,
		},
		{
			// should set default weight for module
			name:     "NoWeightNoDefinition",
			filename: "mpo-without-weight.yaml",
			layersStab: func() ([]crv1.Layer, error) {
				return []crv1.Layer{&utils.FakeLayer{
					FilesContent: map[string]string{"version.json": `{"version": "v1.16.0"}`}}}, nil
			},
			symlinkChanged: false,
			valuesChanged:  false,
			checkValues:    true,
		},
		{
			// should set mpo`s the weight from module.yaml
			name:     "NoWeightWithDefinition",
			filename: "mpo-without-weight.yaml",
			layersStab: func() ([]crv1.Layer, error) {
				return []crv1.Layer{&utils.FakeLayer{
					FilesContent: map[string]string{"version.json": `{"version": "v1.16.0"}`}},
					&utils.FakeLayer{FilesContent: map[string]string{"module.yaml": "weight: 900"}}}, nil
			},
			symlinkChanged: false,
			valuesChanged:  false,
			checkValues:    true,
		},
		{
			// should update deployed-on annotation
			name:     "WrongDeployedOnAnnotation",
			filename: "mpo-with-old-deployed-on.yaml",
			layersStab: func() ([]crv1.Layer, error) {
				return []crv1.Layer{&utils.FakeLayer{
					FilesContent: map[string]string{"version.json": `{"version": "v1.16.0"}`}},
					&utils.FakeLayer{FilesContent: map[string]string{"module.yaml": "weight: 900"}}}, nil
			},
			symlinkChanged: true,
			valuesChanged:  true,
			checkValues:    false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			if tc.layersStab != nil {
				dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{
					ManifestStub: manifestStub,
					LayersStub:   tc.layersStab,
				}, nil)
			}

			require.NoError(suite.T(), module.prepare(true, true))

			statValues, err := os.Stat(module.valuesPath)
			require.NoError(suite.T(), err)

			statSymlink, err := os.Lstat(module.symlinkPath)
			require.NoError(suite.T(), err)

			time.Sleep(50 * time.Millisecond)

			suite.setupModuleLoader(string(suite.parseTestdata("overrides", tc.filename)))
			require.NoError(suite.T(), suite.loader.restoreAbsentModulesFromOverrides(context.TODO()))

			if tc.checkValues {
				newStatValues, err := os.Stat(module.valuesPath)
				require.NoError(suite.T(), err)

				if tc.valuesChanged {
					assert.False(suite.T(), statValues.ModTime().Equal(newStatValues.ModTime()), "values.yaml must be modified")
				} else {
					assert.True(suite.T(), statValues.ModTime().Equal(newStatValues.ModTime()), "values.yaml mustn't be modified")
				}
			}

			newStatSymlink, err := os.Lstat(module.symlinkPath)
			require.NoError(suite.T(), err)

			if tc.symlinkChanged {
				assert.False(suite.T(), statSymlink.ModTime().Equal(newStatSymlink.ModTime()), "Module's symlink must be modified")
			} else {
				assert.True(suite.T(), statSymlink.ModTime().Equal(newStatSymlink.ModTime()), "Module's symlink mustn't be modified")
			}

			mpo := suite.modulePullOverride(module.name)
			assert.Equal(suite.T(), mpo.Annotations[v1alpha1.ModulePullOverrideAnnotationDeployedOn], "dev-master-0", "deployedOn must be set to dev-master-0")
			assert.Equal(suite.T(), mpo.Status.Weight, uint32(module.weight), "ModulePullOverride weight must equal to module's weight")

			suite.cleanupPaths([]string{module.downloadedPath, module.symlinkPath})
		})
	}

	// should ensure symlink
	suite.Run("NoSymlink", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{
			ManifestStub: manifestStub,
			LayersStub: func() ([]crv1.Layer, error) {
				return []crv1.Layer{&utils.FakeLayer{}}, nil
			},
		}, nil)

		require.NoError(suite.T(), module.prepare(true, false))

		statValues, err := os.Stat(module.valuesPath)
		require.NoError(suite.T(), err)

		time.Sleep(50 * time.Millisecond)

		suite.setupModuleLoader(string(suite.parseTestdata("overrides", "mpo.yaml")))
		require.NoError(suite.T(), suite.loader.restoreAbsentModulesFromOverrides(context.TODO()))

		newStatValues, err := os.Stat(module.valuesPath)
		require.NoError(suite.T(), err)

		assert.True(suite.T(), statValues.ModTime().Equal(newStatValues.ModTime()), "values.yaml mustn't be modified")

		_, err = os.Lstat(module.symlinkPath)
		require.NoError(suite.T(), err)

		mpo := suite.modulePullOverride(module.name)
		assert.Equal(suite.T(), mpo.Annotations[v1alpha1.ModulePullOverrideAnnotationDeployedOn], "dev-master-0", "deployedOn must be set to dev-master-0")
		assert.Equal(suite.T(), mpo.Status.Weight, uint32(module.weight), "ModulePullOverride weight must equal to module's weight")

		suite.cleanupPaths([]string{module.downloadedPath, module.symlinkPath})
	})

	// should ensure downloaded module`s dir
	suite.Run("NoDownloadedModule", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{
			ManifestStub: manifestStub,
			LayersStub: func() ([]crv1.Layer, error) {
				return []crv1.Layer{&utils.FakeLayer{}}, nil
			},
		}, nil)

		require.NoError(suite.T(), module.prepare(false, false))

		time.Sleep(50 * time.Millisecond)

		suite.setupModuleLoader(string(suite.parseTestdata("overrides", "mpo.yaml")))
		require.NoError(suite.T(), suite.loader.restoreAbsentModulesFromOverrides(context.TODO()))

		_, err := os.Lstat(module.symlinkPath)
		require.NoError(suite.T(), err)

		mpo := suite.modulePullOverride(module.name)
		assert.Equal(suite.T(), mpo.Annotations[v1alpha1.ModulePullOverrideAnnotationDeployedOn], "dev-master-0", "deployedOn must be set to dev-master-0")
		assert.Equal(suite.T(), mpo.Status.Weight, uint32(module.weight), "ModulePullOverride weight must equal to module's weight")

		suite.cleanupPaths([]string{module.downloadedPath, module.symlinkPath})
	})

	// should remove extra symlink
	suite.Run("ExtraSymlinks", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{
			ManifestStub: manifestStub,
			LayersStub: func() ([]crv1.Layer, error) {
				return []crv1.Layer{&utils.FakeLayer{}}, nil
			},
		}, nil)

		require.NoError(suite.T(), module.prepare(true, false))

		statValues, err := os.Stat(module.valuesPath)
		require.NoError(suite.T(), err)

		_, err = os.Lstat(module.symlinkPath)
		assert.True(suite.T(), os.IsNotExist(err), "Module's symlink mustn't exist")

		symlink1 := filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("901-%s", module.name))
		symlink2 := filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("902-%s", module.name))
		symlink3 := filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("903-%s", module.name))

		// extra symlinks
		require.NoError(suite.T(), os.Symlink(module.downloadedDir, symlink1))
		require.NoError(suite.T(), os.Symlink(module.downloadedDir, symlink2))
		require.NoError(suite.T(), os.Symlink(module.downloadedDir, symlink3))

		time.Sleep(50 * time.Millisecond)

		suite.setupModuleLoader(string(suite.parseTestdata("overrides", "mpo.yaml")))
		require.NoError(suite.T(), suite.loader.restoreAbsentModulesFromOverrides(context.TODO()))

		newStatValues, err := os.Stat(module.valuesPath)
		require.NoError(suite.T(), err)
		assert.True(suite.T(), statValues.ModTime().Equal(newStatValues.ModTime()), "values.yaml mustn't be modified")

		_, err = os.Lstat(module.symlinkPath)
		assert.Equal(suite.T(), err, nil, "Module's symlink must be created")

		_, err = os.Lstat(symlink1)
		assert.True(suite.T(), os.IsNotExist(err), "Extra symlink mustn't exist")
		_, err = os.Lstat(symlink2)
		assert.True(suite.T(), os.IsNotExist(err), "Extra symlink mustn't exist")
		_, err = os.Lstat(symlink3)
		assert.True(suite.T(), os.IsNotExist(err), "Extra symlink mustn't exist")

		mpo := suite.modulePullOverride(module.name)
		assert.Equal(suite.T(), mpo.Annotations[v1alpha1.ModulePullOverrideAnnotationDeployedOn], "dev-master-0", "deployedOn must be set to dev-master-0")
		assert.Equal(suite.T(), mpo.Status.Weight, uint32(module.weight), "ModulePullOverride weight must equal to module's weight")

		suite.cleanupPaths([]string{module.downloadedPath, module.symlinkPath})
	})

	// should remove wrong symlink and ensure new
	suite.Run("WrongSymlink", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{
			ManifestStub: manifestStub,
			LayersStub: func() ([]crv1.Layer, error) {
				return []crv1.Layer{&utils.FakeLayer{}}, nil
			},
		}, nil)

		require.NoError(suite.T(), module.prepare(true, false))

		require.NoError(suite.T(), os.MkdirAll(filepath.Join(suite.tmpDir, "echo", "fakeVersion"), 0750))

		symlink := filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("900-%s", module.name))
		require.NoError(suite.T(), os.Symlink(filepath.Join(suite.tmpDir, "echo", "fakeVersion"), symlink))

		statValues, err := os.Stat(module.valuesPath)
		require.NoError(suite.T(), err)

		statSymlink, err := os.Lstat(symlink)
		require.NoError(suite.T(), err)

		time.Sleep(50 * time.Millisecond)

		suite.setupModuleLoader(string(suite.parseTestdata("overrides", "mpo.yaml")))
		require.NoError(suite.T(), suite.loader.restoreAbsentModulesFromOverrides(context.TODO()))

		newStatValues, err := os.Stat(module.valuesPath)
		require.NoError(suite.T(), err)
		assert.True(suite.T(), statValues.ModTime().Equal(newStatValues.ModTime()), "values.yaml mustn't be modified")

		newStatSymlink, err := os.Lstat(symlink)
		require.NoError(suite.T(), err)
		assert.False(suite.T(), statSymlink.ModTime().Equal(newStatSymlink.ModTime()), "Module's symlink must be modified")

		mpo := suite.modulePullOverride(module.name)
		assert.Equal(suite.T(), mpo.Annotations[v1alpha1.ModulePullOverrideAnnotationDeployedOn], "dev-master-0", "deployedOn must be set to dev-master-0")
		assert.Equal(suite.T(), mpo.Status.Weight, uint32(module.weight), "ModulePullOverride weight must equal to module's weight")

		suite.cleanupPaths([]string{symlink, module.downloadedPath, module.symlinkPath})
	})
}

func (suite *ModuleLoaderTestSuite) TestRestoreAbsentModulesFromOverridesWithMultipleReleases() {
	require.NoError(suite.T(), os.Setenv("DECKHOUSE_NODE_NAME", "dev-master-0"))
	defer os.Unsetenv("DECKHOUSE_NODE_NAME")

	module := moduleSuite{
		name:          "test-module",
		weight:        900,
		downloadedDir: suite.tmpDir,
		version:       downloader.DefaultDevVersion,
	}

	manifestStub := func() (*crv1.Manifest, error) {
		return &crv1.Manifest{
			Layers: []crv1.Descriptor{},
		}, nil
	}

	// Test case 1: Multiple releases, all in Deployed status
	// Expected: MPO should set module version but not change release statuses
	// This test verifies that restoreAbsentModulesFromOverrides correctly handles MPO
	// without affecting the existing ModuleRelease statuses
	suite.Run("MultipleReleasesAllDeployed", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{
			ManifestStub: manifestStub,
			LayersStub: func() ([]crv1.Layer, error) {
				return []crv1.Layer{&utils.FakeLayer{}}, nil
			},
		}, nil)

		require.NoError(suite.T(), module.prepare(true, true))

		suite.setupModuleLoader(string(suite.parseTestdata("overrides", "multiple-releases-all-deployed.yaml")))
		require.NoError(suite.T(), suite.loader.restoreAbsentModulesFromOverrides(context.TODO()))

		// Check that the module symlink was created
		_, err := os.Lstat(module.symlinkPath)
		require.NoError(suite.T(), err, "Module symlink should exist")

		// Check that the module files exist
		_, err = os.Stat(module.valuesPath)
		require.NoError(suite.T(), err, "Module values should exist")

		// Verify that the module version in the Module resource is set to v1.0.2 (from MPO)
		moduleObj := new(v1alpha1.Module)
		err = suite.client.Get(context.TODO(), client.ObjectKey{Name: "test-module"}, moduleObj)
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), "v1.0.2", moduleObj.Properties.Version, "Module version should be set from MPO")

		// Verify the ModuleRelease statuses remain unchanged (MPO should not affect release statuses)
		releases := new(v1alpha1.ModuleReleaseList)
		err = suite.client.List(context.TODO(), releases, client.MatchingLabels{"module": "test-module"})
		require.NoError(suite.T(), err)
		require.Len(suite.T(), releases.Items, 3, "Should have 3 releases")

		// Check specific statuses for each release to ensure MPO doesn't change them
		// MPO should only affect module version, not release statuses
		var deployedCount int
		for _, release := range releases.Items {
			switch release.GetModuleVersion() {
			case "v1.0.0":
				assert.Equal(suite.T(), v1alpha1.ModuleReleasePhaseDeployed, release.Status.Phase, "v1.0.0 should remain Deployed")
				deployedCount++
			case "v1.0.1":
				assert.Equal(suite.T(), v1alpha1.ModuleReleasePhaseDeployed, release.Status.Phase, "v1.0.1 should remain Deployed")
				deployedCount++
			case "v1.0.2":
				assert.Equal(suite.T(), v1alpha1.ModuleReleasePhaseDeployed, release.Status.Phase, "v1.0.2 should remain Deployed")
				deployedCount++
			default:
				suite.T().Fatalf("Unexpected release version: %s", release.GetModuleVersion())
			}
		}
		assert.Equal(suite.T(), 3, deployedCount, "Should have 3 deployed releases")

		suite.cleanupPaths([]string{module.downloadedPath, module.symlinkPath})
	})

	// Test case 2: Multiple releases, all Superseded except last in Deployed
	// Expected: MPO should set module version but not change release statuses
	// This test verifies that restoreAbsentModulesFromOverrides correctly handles MPO
	// with mixed release statuses without affecting them
	suite.Run("MultipleReleasesSupersededExceptLast", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{
			ManifestStub: manifestStub,
			LayersStub: func() ([]crv1.Layer, error) {
				return []crv1.Layer{&utils.FakeLayer{}}, nil
			},
		}, nil)

		require.NoError(suite.T(), module.prepare(true, true))

		suite.setupModuleLoader(string(suite.parseTestdata("overrides", "multiple-releases-superseded-except-last.yaml")))
		require.NoError(suite.T(), suite.loader.restoreAbsentModulesFromOverrides(context.TODO()))

		// Check that the module symlink was created
		_, err := os.Lstat(module.symlinkPath)
		require.NoError(suite.T(), err, "Module symlink should exist")

		// Check that the module files exist
		_, err = os.Stat(module.valuesPath)
		require.NoError(suite.T(), err, "Module values should exist")

		// Verify that the module version in the Module resource is set to v1.0.3 (from MPO)
		moduleObj := new(v1alpha1.Module)
		err = suite.client.Get(context.TODO(), client.ObjectKey{Name: "test-module"}, moduleObj)
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), "v1.0.3", moduleObj.Properties.Version, "Module version should be set from MPO")

		// Verify the ModuleRelease statuses remain unchanged (MPO should not affect release statuses)
		releases := new(v1alpha1.ModuleReleaseList)
		err = suite.client.List(context.TODO(), releases, client.MatchingLabels{"module": "test-module"})
		require.NoError(suite.T(), err)
		require.Len(suite.T(), releases.Items, 4, "Should have 4 releases")

		// Check specific statuses for each release to ensure MPO doesn't change them
		var supersededCount, deployedCount int
		for _, release := range releases.Items {
			switch release.GetModuleVersion() {
			case "v1.0.0":
				assert.Equal(suite.T(), v1alpha1.ModuleReleasePhaseSuperseded, release.Status.Phase, "v1.0.0 should remain Superseded")
				supersededCount++
			case "v1.0.1":
				assert.Equal(suite.T(), v1alpha1.ModuleReleasePhaseSuperseded, release.Status.Phase, "v1.0.1 should remain Superseded")
				supersededCount++
			case "v1.0.2":
				assert.Equal(suite.T(), v1alpha1.ModuleReleasePhaseSuperseded, release.Status.Phase, "v1.0.2 should remain Superseded")
				supersededCount++
			case "v1.0.3":
				assert.Equal(suite.T(), v1alpha1.ModuleReleasePhaseDeployed, release.Status.Phase, "v1.0.3 should remain Deployed")
				deployedCount++
			default:
				suite.T().Fatalf("Unexpected release version: %s", release.GetModuleVersion())
			}
		}
		assert.Equal(suite.T(), 3, supersededCount, "Should have 3 superseded releases")
		assert.Equal(suite.T(), 1, deployedCount, "Should have 1 deployed release")

		suite.cleanupPaths([]string{module.downloadedPath, module.symlinkPath})
	})

	// Test case 3: Multiple releases, first Superseded, several Deployed
	// Expected: MPO should set module version but not change release statuses
	// This test verifies that restoreAbsentModulesFromOverrides correctly handles MPO
	// with complex release status patterns without affecting them
	suite.Run("MultipleReleasesMixedStatus", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{
			ManifestStub: manifestStub,
			LayersStub: func() ([]crv1.Layer, error) {
				return []crv1.Layer{&utils.FakeLayer{}}, nil
			},
		}, nil)

		require.NoError(suite.T(), module.prepare(true, true))

		suite.setupModuleLoader(string(suite.parseTestdata("overrides", "multiple-releases-mixed-status.yaml")))
		require.NoError(suite.T(), suite.loader.restoreAbsentModulesFromOverrides(context.TODO()))

		// Check that the module symlink was created
		_, err := os.Lstat(module.symlinkPath)
		require.NoError(suite.T(), err, "Module symlink should exist")

		// Check that the module files exist
		_, err = os.Stat(module.valuesPath)
		require.NoError(suite.T(), err, "Module values should exist")

		// Verify that the module version in the Module resource is set to v1.0.2 (from MPO)
		moduleObj := new(v1alpha1.Module)
		err = suite.client.Get(context.TODO(), client.ObjectKey{Name: "test-module"}, moduleObj)
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), "v1.0.2", moduleObj.Properties.Version, "Module version should be set from MPO")

		// Verify the ModuleRelease statuses remain unchanged (MPO should not affect release statuses)
		releases := new(v1alpha1.ModuleReleaseList)
		err = suite.client.List(context.TODO(), releases, client.MatchingLabels{"module": "test-module"})
		require.NoError(suite.T(), err)
		require.Len(suite.T(), releases.Items, 4, "Should have 4 releases")

		// Check specific statuses for each release to ensure MPO doesn't change them
		var supersededCount, deployedCount int
		for _, release := range releases.Items {
			switch release.GetModuleVersion() {
			case "v1.0.0":
				assert.Equal(suite.T(), v1alpha1.ModuleReleasePhaseSuperseded, release.Status.Phase, "v1.0.0 should remain Superseded")
				supersededCount++
			case "v1.0.1":
				assert.Equal(suite.T(), v1alpha1.ModuleReleasePhaseDeployed, release.Status.Phase, "v1.0.1 should remain Deployed")
				deployedCount++
			case "v1.0.2":
				assert.Equal(suite.T(), v1alpha1.ModuleReleasePhaseDeployed, release.Status.Phase, "v1.0.2 should remain Deployed")
				deployedCount++
			case "v1.0.3":
				assert.Equal(suite.T(), v1alpha1.ModuleReleasePhaseDeployed, release.Status.Phase, "v1.0.3 should remain Deployed")
				deployedCount++
			default:
				suite.T().Fatalf("Unexpected release version: %s", release.GetModuleVersion())
			}
		}
		assert.Equal(suite.T(), 1, supersededCount, "Should have 1 superseded release")
		assert.Equal(suite.T(), 3, deployedCount, "Should have 3 deployed releases")

		suite.cleanupPaths([]string{module.downloadedPath, module.symlinkPath})
	})

	// Test with different release statuses and ensure correct version selection
	suite.Run("MultipleReleasesWithDifferentVersionPrecedence", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{
			ManifestStub: manifestStub,
			LayersStub: func() ([]crv1.Layer, error) {
				return []crv1.Layer{&utils.FakeLayer{}}, nil
			},
		}, nil)

		// Delete the module first to start fresh
		suite.cleanupPaths([]string{module.downloadedPath, module.symlinkPath})
		require.NoError(suite.T(), module.prepare(true, true))

		// Test with multiple deployed releases - MPO should override all
		suite.setupModuleLoader(string(suite.parseTestdata("overrides", "multiple-releases-all-deployed.yaml")))
		require.NoError(suite.T(), suite.loader.restoreAbsentModulesFromOverrides(context.TODO()))

		// Check symlink exists
		_, err := os.Lstat(module.symlinkPath)
		require.NoError(suite.T(), err, "Module symlink should exist")

		// MPO version should take precedence
		moduleObj := new(v1alpha1.Module)
		err = suite.client.Get(context.TODO(), client.ObjectKey{Name: "test-module"}, moduleObj)
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), "v1.0.2", moduleObj.Properties.Version, "MPO version should override any deployed release version")

		suite.cleanupPaths([]string{module.downloadedPath, module.symlinkPath})
	})
}

func (suite *ModuleLoaderTestSuite) TestRestoreAbsentModulesFromReleases() {
	module := moduleSuite{
		name:          "echo",
		weight:        900,
		downloadedDir: suite.tmpDir,
		version:       "v1.0.0",
	}

	manifestStub := func() (*crv1.Manifest, error) {
		return &crv1.Manifest{
			Layers: []crv1.Descriptor{},
		}, nil
	}

	// should ensure symlink
	suite.Run("NoSymlink", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{
			ManifestStub: manifestStub,
			LayersStub: func() ([]crv1.Layer, error) {
				return []crv1.Layer{&utils.FakeLayer{}}, nil
			},
		}, nil)

		require.NoError(suite.T(), module.prepare(true, false))

		statValues, err := os.Stat(module.valuesPath)
		require.NoError(suite.T(), err)

		time.Sleep(50 * time.Millisecond)

		suite.setupModuleLoader(string(suite.parseTestdata("releases", "release.yaml")))
		require.NoError(suite.T(), suite.loader.restoreAbsentModulesFromReleases(context.TODO()))

		newStatValues, err := os.Stat(module.valuesPath)
		require.NoError(suite.T(), err)

		assert.True(suite.T(), statValues.ModTime().Equal(newStatValues.ModTime()), "values.yaml mustn't be modified")

		_, err = os.Lstat(module.symlinkPath)
		require.NoError(suite.T(), err)

		suite.cleanupPaths([]string{module.downloadedPath, module.symlinkPath})
	})

	// should ensure downloaded module`s dir
	suite.Run("NoDownloadedModule", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{
			ManifestStub: manifestStub,
			LayersStub: func() ([]crv1.Layer, error) {
				return []crv1.Layer{&utils.FakeLayer{}}, nil
			},
		}, nil)

		require.NoError(suite.T(), module.prepare(false, false))

		time.Sleep(50 * time.Millisecond)

		suite.setupModuleLoader(string(suite.parseTestdata("releases", "release.yaml")))
		require.NoError(suite.T(), suite.loader.restoreAbsentModulesFromReleases(context.TODO()))

		_, err := os.Lstat(module.symlinkPath)
		require.NoError(suite.T(), err)

		suite.cleanupPaths([]string{module.downloadedPath, module.symlinkPath})
	})

	// should remove extra symlink
	suite.Run("ExtraSymlinks", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{
			ManifestStub: manifestStub,
			LayersStub: func() ([]crv1.Layer, error) {
				return []crv1.Layer{&utils.FakeLayer{}}, nil
			},
		}, nil)

		require.NoError(suite.T(), module.prepare(true, false))

		statValues, err := os.Stat(module.valuesPath)
		require.NoError(suite.T(), err)

		_, err = os.Lstat(module.symlinkPath)
		assert.True(suite.T(), os.IsNotExist(err), "Module's symlink mustn't exist")

		symlink1 := filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("901-%s", module.name))
		symlink2 := filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("902-%s", module.name))
		symlink3 := filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("903-%s", module.name))

		// extra symlinks
		require.NoError(suite.T(), os.Symlink(module.downloadedDir, symlink1))
		require.NoError(suite.T(), os.Symlink(module.downloadedDir, symlink2))
		require.NoError(suite.T(), os.Symlink(module.downloadedDir, symlink3))

		time.Sleep(50 * time.Millisecond)

		suite.setupModuleLoader(string(suite.parseTestdata("releases", "release.yaml")))
		require.NoError(suite.T(), suite.loader.restoreAbsentModulesFromReleases(context.TODO()))

		newStatValues, err := os.Stat(module.valuesPath)
		require.NoError(suite.T(), err)
		assert.True(suite.T(), statValues.ModTime().Equal(newStatValues.ModTime()), "values.yaml mustn't be modified")

		_, err = os.Lstat(module.symlinkPath)
		assert.Equal(suite.T(), err, nil, "Module's symlink must be created")

		_, err = os.Lstat(symlink1)
		assert.True(suite.T(), os.IsNotExist(err), "Extra symlink mustn't exist")
		_, err = os.Lstat(symlink2)
		assert.True(suite.T(), os.IsNotExist(err), "Extra symlink mustn't exist")
		_, err = os.Lstat(symlink3)
		assert.True(suite.T(), os.IsNotExist(err), "Extra symlink mustn't exist")

		suite.cleanupPaths([]string{module.downloadedPath, module.symlinkPath})
	})

	// HA deckhouse installations could have previous version symlink on the standby masters
	// have to delete it and add an actual one
	suite.Run("Old version symlink", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{
			ManifestStub: manifestStub,
			LayersStub: func() ([]crv1.Layer, error) {
				return []crv1.Layer{&utils.FakeLayer{}}, nil
			},
		}, nil)

		require.NoError(suite.T(), module.prepare(true, false))

		require.NoError(suite.T(), os.MkdirAll(filepath.Join(suite.tmpDir, "echo", "v0.9.0"), 0750))

		symlink := filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("900-%s", module.name))
		require.NoError(suite.T(), os.Symlink(filepath.Join(suite.tmpDir, "echo", "v0.9.0"), symlink))

		time.Sleep(50 * time.Millisecond)

		suite.setupModuleLoader(string(suite.parseTestdata("releases", "release.yaml")))
		require.NoError(suite.T(), suite.loader.restoreAbsentModulesFromReleases(context.TODO()))

		symlinkTarget, err := filepath.EvalSymlinks(symlink)
		require.NoError(suite.T(), err)

		assert.True(suite.T(), strings.HasSuffix(symlinkTarget, "echo/v1.0.0"), "module have to be restored to the v1.0.0 version")

		suite.cleanupPaths([]string{symlink, module.downloadedPath, module.symlinkPath})
	})

	suite.Run("WrongSymlink", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{
			ManifestStub: manifestStub,
			LayersStub: func() ([]crv1.Layer, error) {
				return []crv1.Layer{&utils.FakeLayer{}}, nil
			},
		}, nil)

		require.NoError(suite.T(), module.prepare(true, false))

		require.NoError(suite.T(), os.MkdirAll(filepath.Join(suite.tmpDir, "echo", "fakeVersion"), 0750))

		symlink := filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("900-%s", module.name))
		require.NoError(suite.T(), os.Symlink(filepath.Join(suite.tmpDir, "echo", "fakeVersion"), symlink))

		statValues, err := os.Stat(module.valuesPath)
		require.NoError(suite.T(), err)

		statSymlink, err := os.Lstat(symlink)
		require.NoError(suite.T(), err)

		time.Sleep(50 * time.Millisecond)

		suite.setupModuleLoader(string(suite.parseTestdata("releases", "release.yaml")))
		require.NoError(suite.T(), suite.loader.restoreAbsentModulesFromReleases(context.TODO()))

		newStatValues, err := os.Stat(module.valuesPath)
		require.NoError(suite.T(), err)
		assert.True(suite.T(), statValues.ModTime().Equal(newStatValues.ModTime()), "values.yaml mustn't be modified")

		newStatSymlink, err := os.Lstat(symlink)
		require.NoError(suite.T(), err)
		assert.False(suite.T(), statSymlink.ModTime().Equal(newStatSymlink.ModTime()), "Module's symlink must be modified")

		suite.cleanupPaths([]string{symlink, module.downloadedPath, module.symlinkPath})
	})

	// Test case 1: Multiple releases, all in Deployed status
	// Expected: only the latest version should remain deployed, older versions should become superseded
	// This test verifies that restoreAbsentModulesFromReleases correctly handles multiple deployed releases
	// by keeping only the latest version deployed and marking older versions as superseded
	suite.Run("MultipleReleasesAllDeployed", func() {
		testModule := moduleSuite{
			name:          "test-module",
			weight:        900,
			downloadedDir: suite.tmpDir,
			version:       "v1.0.2", // latest version
		}

		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{
			ManifestStub: manifestStub,
			LayersStub: func() ([]crv1.Layer, error) {
				return []crv1.Layer{&utils.FakeLayer{}}, nil
			},
		}, nil)

		require.NoError(suite.T(), testModule.prepare(true, false))

		suite.setupModuleLoader(string(suite.parseTestdata("releases", "multiple-releases-all-deployed.yaml")))

		// Verify initial state - all releases should be Deployed
		initialReleases := new(v1alpha1.ModuleReleaseList)
		initialErr := suite.client.List(context.TODO(), initialReleases, client.MatchingLabels{"module": "test-module"})
		require.NoError(suite.T(), initialErr)
		for _, release := range initialReleases.Items {
			assert.Equal(suite.T(), v1alpha1.ModuleReleasePhaseDeployed, release.Status.Phase,
				"Initial state: %s should be Deployed", release.GetModuleVersion())
		}

		require.NoError(suite.T(), suite.loader.restoreAbsentModulesFromReleases(context.TODO()))

		// Check that the module symlink was created
		_, err := os.Lstat(testModule.symlinkPath)
		require.NoError(suite.T(), err, "Module symlink should exist")

		// Check that the module files exist
		_, err = os.Stat(testModule.valuesPath)
		require.NoError(suite.T(), err, "Module values should exist")

		// Verify that the module version is set to the latest deployed release
		moduleObj := new(v1alpha1.Module)
		err = suite.client.Get(context.TODO(), client.ObjectKey{Name: "test-module"}, moduleObj)
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), "v1.0.2", moduleObj.Properties.Version, "Module version should be set to latest deployed release")

		// Verify the ModuleRelease statuses were updated correctly
		releases := new(v1alpha1.ModuleReleaseList)
		err = suite.client.List(context.TODO(), releases, client.MatchingLabels{"module": "test-module"})
		require.NoError(suite.T(), err)
		require.Len(suite.T(), releases.Items, 3, "Should have 3 releases")

		// Check specific statuses for each release to ensure the function correctly
		// changed the statuses according to its logic (only latest version remains deployed)
		var deployedCount, supersededCount int
		for _, release := range releases.Items {
			switch release.GetModuleVersion() {
			case "v1.0.0":
				assert.Equal(suite.T(), v1alpha1.ModuleReleasePhaseSuperseded, release.Status.Phase, "v1.0.0 should be superseded")
				supersededCount++
			case "v1.0.1":
				assert.Equal(suite.T(), v1alpha1.ModuleReleasePhaseSuperseded, release.Status.Phase, "v1.0.1 should be superseded")
				supersededCount++
			case "v1.0.2":
				assert.Equal(suite.T(), v1alpha1.ModuleReleasePhaseDeployed, release.Status.Phase, "v1.0.2 should be deployed")
				deployedCount++
			default:
				suite.T().Fatalf("Unexpected release version: %s", release.GetModuleVersion())
			}
		}
		assert.Equal(suite.T(), 1, deployedCount, "Should have 1 deployed release")
		assert.Equal(suite.T(), 2, supersededCount, "Should have 2 superseded releases")

		suite.cleanupPaths([]string{testModule.downloadedPath, testModule.symlinkPath})
	})

	// Test case 2: Multiple releases, all Superseded except last in Deployed
	// Expected: only the deployed release should be processed
	suite.Run("MultipleReleasesSupersededExceptLast", func() {
		testModule := moduleSuite{
			name:          "test-module",
			weight:        900,
			downloadedDir: suite.tmpDir,
			version:       "v1.0.3",
		}

		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{
			ManifestStub: manifestStub,
			LayersStub: func() ([]crv1.Layer, error) {
				return []crv1.Layer{&utils.FakeLayer{}}, nil
			},
		}, nil)

		require.NoError(suite.T(), testModule.prepare(true, false))

		suite.setupModuleLoader(string(suite.parseTestdata("releases", "multiple-releases-superseded-except-last.yaml")))
		require.NoError(suite.T(), suite.loader.restoreAbsentModulesFromReleases(context.TODO()))

		// Check that the module symlink was created
		_, err := os.Lstat(testModule.symlinkPath)
		require.NoError(suite.T(), err, "Module symlink should exist")

		// Check that the module files exist
		_, err = os.Stat(testModule.valuesPath)
		require.NoError(suite.T(), err, "Module values should exist")

		// Verify that the module version is set to v1.0.3 (the only deployed release)
		moduleObj := new(v1alpha1.Module)
		err = suite.client.Get(context.TODO(), client.ObjectKey{Name: "test-module"}, moduleObj)
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), "v1.0.3", moduleObj.Properties.Version, "Module version should be set to the deployed release")

		// Verify the ModuleRelease statuses remain unchanged
		releases := new(v1alpha1.ModuleReleaseList)
		err = suite.client.List(context.TODO(), releases, client.MatchingLabels{"module": "test-module"})
		require.NoError(suite.T(), err)
		require.Len(suite.T(), releases.Items, 4, "Should have 4 releases")

		// Check specific statuses for each release
		var supersededCount, deployedCount int
		for _, release := range releases.Items {
			switch release.GetModuleVersion() {
			case "v1.0.0":
				assert.Equal(suite.T(), v1alpha1.ModuleReleasePhaseSuperseded, release.Status.Phase, "v1.0.0 should be superseded")
				supersededCount++
			case "v1.0.1":
				assert.Equal(suite.T(), v1alpha1.ModuleReleasePhaseSuperseded, release.Status.Phase, "v1.0.1 should be superseded")
				supersededCount++
			case "v1.0.2":
				assert.Equal(suite.T(), v1alpha1.ModuleReleasePhaseSuperseded, release.Status.Phase, "v1.0.2 should be superseded")
				supersededCount++
			case "v1.0.3":
				assert.Equal(suite.T(), v1alpha1.ModuleReleasePhaseDeployed, release.Status.Phase, "v1.0.3 should be deployed")
				deployedCount++
			default:
				suite.T().Fatalf("Unexpected release version: %s", release.GetModuleVersion())
			}
		}
		assert.Equal(suite.T(), 3, supersededCount, "Should have 3 superseded releases")
		assert.Equal(suite.T(), 1, deployedCount, "Should have 1 deployed release")

		suite.cleanupPaths([]string{testModule.downloadedPath, testModule.symlinkPath})
	})

	// Test case 3: Multiple releases, first Superseded, several Deployed
	// Expected: only the latest deployed version should remain deployed
	suite.Run("MultipleReleasesMixedStatus", func() {
		testModule := moduleSuite{
			name:          "test-module",
			weight:        900,
			downloadedDir: suite.tmpDir,
			version:       "v1.0.3", // latest deployed version
		}

		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{
			ManifestStub: manifestStub,
			LayersStub: func() ([]crv1.Layer, error) {
				return []crv1.Layer{&utils.FakeLayer{}}, nil
			},
		}, nil)

		require.NoError(suite.T(), testModule.prepare(true, false))

		suite.setupModuleLoader(string(suite.parseTestdata("releases", "multiple-releases-mixed-status.yaml")))
		require.NoError(suite.T(), suite.loader.restoreAbsentModulesFromReleases(context.TODO()))

		// Check that the module symlink was created
		_, err := os.Lstat(testModule.symlinkPath)
		require.NoError(suite.T(), err, "Module symlink should exist")

		// Check that the module files exist
		_, err = os.Stat(testModule.valuesPath)
		require.NoError(suite.T(), err, "Module values should exist")

		// Verify that the module version is set to the latest deployed version (v1.0.3)
		moduleObj := new(v1alpha1.Module)
		err = suite.client.Get(context.TODO(), client.ObjectKey{Name: "test-module"}, moduleObj)
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), "v1.0.3", moduleObj.Properties.Version, "Module version should be set to latest deployed release")

		// Verify the ModuleRelease statuses were updated correctly
		releases := new(v1alpha1.ModuleReleaseList)
		err = suite.client.List(context.TODO(), releases, client.MatchingLabels{"module": "test-module"})
		require.NoError(suite.T(), err)
		require.Len(suite.T(), releases.Items, 4, "Should have 4 releases")

		// Check specific statuses for each release
		var supersededCount, deployedCount int
		for _, release := range releases.Items {
			switch release.GetModuleVersion() {
			case "v1.0.0":
				assert.Equal(suite.T(), v1alpha1.ModuleReleasePhaseSuperseded, release.Status.Phase, "v1.0.0 should be superseded")
				supersededCount++
			case "v1.0.1":
				assert.Equal(suite.T(), v1alpha1.ModuleReleasePhaseSuperseded, release.Status.Phase, "v1.0.1 should be superseded")
				supersededCount++
			case "v1.0.2":
				assert.Equal(suite.T(), v1alpha1.ModuleReleasePhaseSuperseded, release.Status.Phase, "v1.0.2 should be superseded")
				supersededCount++
			case "v1.0.3":
				assert.Equal(suite.T(), v1alpha1.ModuleReleasePhaseDeployed, release.Status.Phase, "v1.0.3 should be deployed")
				deployedCount++
			default:
				suite.T().Fatalf("Unexpected release version: %s", release.GetModuleVersion())
			}
		}
		assert.Equal(suite.T(), 3, supersededCount, "Should have 3 superseded releases")
		assert.Equal(suite.T(), 1, deployedCount, "Should have 1 deployed release")

		suite.cleanupPaths([]string{testModule.downloadedPath, testModule.symlinkPath})
	})
}

func (suite *ModuleLoaderTestSuite) modulePullOverride(name string) *v1alpha2.ModulePullOverride {
	mpo := new(v1alpha2.ModulePullOverride)
	err := suite.client.Get(context.TODO(), client.ObjectKey{Name: name}, mpo)
	require.NoError(suite.T(), err)

	return mpo
}

func (suite *ModuleLoaderTestSuite) parseTestdata(scope, filename string) []byte {
	data, err := os.ReadFile(filepath.Join("./testdata", scope, filename))
	require.NoError(suite.T(), err)

	suite.testDataFileName = filename

	return data
}

func (suite *ModuleLoaderTestSuite) cleanupPaths(paths []string) {
	for _, path := range paths {
		require.NoError(suite.T(), os.RemoveAll(path))
	}
}

type moduleSuite struct {
	name           string
	version        string
	weight         int
	valuesPath     string
	symlinkPath    string
	downloadedPath string
	downloadedDir  string
}

func (suite *moduleSuite) prepare(ensureDownloaded, ensureSymlink bool) error {
	suite.downloadedPath = filepath.Join(suite.downloadedDir, suite.name, suite.version)
	suite.symlinkPath = filepath.Join(suite.downloadedDir, "modules", fmt.Sprintf("%d-%s", suite.weight, suite.name))
	suite.valuesPath = filepath.Join(suite.downloadedPath, "openapi", "values.yaml")

	if ensureDownloaded {
		if err := os.MkdirAll(filepath.Join(suite.downloadedPath, "openapi"), 0750); err != nil {
			return err
		}

		if err := os.WriteFile(suite.valuesPath, []byte(values), 0750); err != nil {
			return err
		}
	}

	if ensureSymlink {
		if err := os.Symlink(suite.downloadedPath, suite.symlinkPath); err != nil {
			return err
		}
	}

	return nil
}
