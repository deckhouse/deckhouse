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
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
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
	testMPOName      string

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
		WithStatusSubresource(&v1alpha1.ModuleSource{}, &v1alpha2.ModulePullOverride{}).Build()

	suite.loader = &Loader{
		client:               suite.client,
		downloadedModulesDir: d8env.GetDownloadedModulesDir(),
		dependencyContainer:  dependency.NewDependencyContainer(),
		log:                  log.NewNop(),
		symlinksDir:          filepath.Join(d8env.GetDownloadedModulesDir(), "modules"),
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
	moduleName := "echo"
	moduleDir := filepath.Join(suite.tmpDir, moduleName, downloader.DefaultDevVersion)
	moduleValues := filepath.Join(moduleDir, "openapi", "values.yaml")
	ManifestStub := func() (*v1.Manifest, error) {
		return &v1.Manifest{
			Layers: []v1.Descriptor{},
		}, nil
	}

	type testCase struct {
		name           string
		filename       string
		modulePath     string
		layersStab     func() ([]v1.Layer, error)
		weight         int
		weightMessage  string
		symlinkChanged bool
		valuesChanged  bool
	}

	testCases := []testCase{
		{
			name:          "Ok",
			filename:      "up-to-date.yaml",
			modulePath:    fmt.Sprintf("910-%s", moduleName),
			weight:        910,
			weightMessage: "Module's weight mustn't be modified",
		},
		{
			name:       "No weight, no module.yaml",
			filename:   "up-to-date-no-weight.yaml",
			modulePath: fmt.Sprintf("900-%s", moduleName),
			layersStab: func() ([]v1.Layer, error) {
				return []v1.Layer{&utils.FakeLayer{}}, nil
			},
			weight:        900,
			weightMessage: "Module's weight must be set to 900",
		},
		{
			name:       "No weight",
			filename:   "up-to-date-no-weight.yaml",
			modulePath: fmt.Sprintf("915-%s", moduleName),
			layersStab: func() ([]v1.Layer, error) {
				return []v1.Layer{&utils.FakeLayer{}, &utils.FakeLayer{FilesContent: map[string]string{"module.yaml": "weight: 915"}}}, nil
			},
			weight:        915,
			weightMessage: "Module's weight must be set to 915",
		},
		{
			name:          "Stale module values",
			filename:      "up-to-date.yaml",
			modulePath:    fmt.Sprintf("910-%s", moduleName),
			weight:        910,
			weightMessage: "Module's weight must be set to 910",
		},
		{
			name:       "Old deployed-on annotation",
			filename:   "old-deckhouse-node-name-annotation.yaml",
			modulePath: fmt.Sprintf("900-%s", moduleName),
			layersStab: func() ([]v1.Layer, error) {
				return []v1.Layer{&utils.FakeLayer{}}, nil
			},
			weight:         900,
			weightMessage:  "Module's weight must be set to 900",
			symlinkChanged: true,
			valuesChanged:  true,
		},
		{
			name:       "No deployed-on annotation",
			filename:   "no-deckhouse-node-name-annotation.yaml",
			modulePath: fmt.Sprintf("900-%s", moduleName),
			layersStab: func() ([]v1.Layer, error) {
				return []v1.Layer{&utils.FakeLayer{}}, nil
			},
			weight:         900,
			weightMessage:  "Module's weight must be set to 900",
			symlinkChanged: true,
			valuesChanged:  true,
		},
		{
			name:       "No symlink",
			filename:   "up-to-date-no-weight.yaml",
			modulePath: fmt.Sprintf("900-%s", moduleName),
			layersStab: func() ([]v1.Layer, error) {
				return []v1.Layer{&utils.FakeLayer{}}, nil
			},
			weight:        900,
			weightMessage: "Module's weight must be set to 900",
		},
		{
			name:       "No module dir",
			filename:   "up-to-date-no-weight.yaml",
			modulePath: fmt.Sprintf("900-%s", moduleName),
			layersStab: func() ([]v1.Layer, error) {
				return []v1.Layer{&utils.FakeLayer{}}, nil
			},
			weight:        900,
			weightMessage: "Module's weight must be set to 900",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			if tc.layersStab != nil {
				dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{
					ManifestStub: ManifestStub,
					LayersStub:   tc.layersStab,
				}, nil)
			}

			desc := moduleDirDescriptor{
				dir:     moduleDir,
				values:  values,
				symlink: filepath.Join(suite.tmpDir, "modules", tc.modulePath),
			}
			err := desc.prepareModuleDir()
			require.NoError(suite.T(), err)

			stat, err := os.Stat(moduleValues)
			require.NoError(suite.T(), err)

			sstat, err := os.Lstat(desc.symlink)
			require.NoError(suite.T(), err)

			time.Sleep(50 * time.Millisecond)

			suite.setupModuleLoader(string(suite.parseTestdata(tc.filename)))
			err = suite.loader.restoreAbsentModulesFromOverrides(context.TODO())
			require.NoError(suite.T(), err)

			newstat, err := os.Stat(moduleValues)
			require.NoError(suite.T(), err)
			if tc.valuesChanged {
				assert.False(suite.T(), stat.ModTime().Equal(newstat.ModTime()), "Module's values.yaml must be modified")
			} else {
				assert.True(suite.T(), stat.ModTime().Equal(newstat.ModTime()), "Module's values.yaml mustn't be modified")
			}

			newsstat, err := os.Lstat(desc.symlink)
			require.NoError(suite.T(), err)
			if tc.symlinkChanged {
				assert.False(suite.T(), sstat.ModTime().Equal(newsstat.ModTime()), "Module's symlink must be modified")
			} else {
				assert.True(suite.T(), sstat.ModTime().Equal(newsstat.ModTime()), "Module's symlink mustn't be modified")
			}

			mpo := suite.modulePullOverride(moduleName)
			assert.Equalf(suite.T(), mpo.Annotations[v1alpha1.ModulePullOverrideAnnotationDeployedOn], "dev-master-0", "%s must be set to dev-master-0", v1alpha1.ModulePullOverrideAnnotationDeployedOn)
			assert.Equal(suite.T(), mpo.Status.Weight, uint32(tc.weight), tc.weightMessage)

			require.NoError(suite.T(), cleanupPaths(desc.dir, desc.symlink))
		})
	}

	suite.Run("Extra symlinks", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{
			ManifestStub: ManifestStub,
			LayersStub: func() ([]v1.Layer, error) {
				return []v1.Layer{&utils.FakeLayer{}}, nil
			},
		}, nil)
		symlink := filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("900-%s", moduleName))
		symlink1 := filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("901-%s", moduleName))
		symlink2 := filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("902-%s", moduleName))
		symlink3 := filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("903-%s", moduleName))
		desc := moduleDirDescriptor{
			dir:    moduleDir,
			values: values,
		}
		err := desc.prepareModuleDir()

		// extra symlinks
		require.NoError(suite.T(), err)
		err = os.Symlink(desc.dir, symlink1)
		require.NoError(suite.T(), err)
		err = os.Symlink(desc.dir, symlink2)
		require.NoError(suite.T(), err)
		err = os.Symlink(desc.dir, symlink3)
		require.NoError(suite.T(), err)

		stat, err := os.Stat(desc.dir)
		require.NoError(suite.T(), err)

		_, err = os.Lstat(symlink)
		assert.True(suite.T(), os.IsNotExist(err), "Module's symlink mustn't exist")

		time.Sleep(50 * time.Millisecond)

		suite.setupModuleLoader(string(suite.parseTestdata("up-to-date-no-weight.yaml")))
		err = suite.loader.restoreAbsentModulesFromOverrides(context.TODO())
		require.NoError(suite.T(), err)

		newstat, err := os.Stat(desc.dir)
		require.NoError(suite.T(), err)
		assert.True(suite.T(), stat.ModTime().Equal(newstat.ModTime()), "Module's dir mustn't be modified")

		_, err = os.Lstat(symlink)
		assert.Equal(suite.T(), err, nil, "Module's symlink must be created")

		_, err = os.Lstat(symlink1)
		assert.True(suite.T(), os.IsNotExist(err), "Extra symlink mustn't exist")
		_, err = os.Lstat(symlink2)
		assert.True(suite.T(), os.IsNotExist(err), "Extra symlink mustn't exist")
		_, err = os.Lstat(symlink3)
		assert.True(suite.T(), os.IsNotExist(err), "Extra symlink mustn't exist")

		mpo := suite.modulePullOverride(moduleName)
		assert.Equalf(suite.T(), mpo.Annotations[v1alpha1.ModulePullOverrideAnnotationDeployedOn], "dev-master-0", "%s must be set to dev-master-0", v1alpha1.ModulePullOverrideAnnotationDeployedOn)
		assert.Equalf(suite.T(), mpo.Status.Weight, uint32(900), "Module's weight must be set to %d", 900)

		require.NoError(suite.T(), cleanupPaths(desc.dir, symlink, symlink1, symlink2, symlink3))
	})

	suite.Run("Wrong symlink", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{
			ManifestStub: ManifestStub,
			LayersStub: func() ([]v1.Layer, error) {
				return []v1.Layer{&utils.FakeLayer{}}, nil
			},
		}, nil)

		desc := moduleDirDescriptor{
			dir:    moduleDir,
			values: values,
		}
		err := desc.prepareModuleDir()
		require.NoError(suite.T(), err)

		symlink := filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("900-%s", moduleName))
		err = os.Symlink("../notEcho/fakeVersion", symlink)
		require.NoError(suite.T(), err)

		stat, err := os.Stat(moduleDir)
		require.NoError(suite.T(), err)

		sstat, err := os.Lstat(symlink)
		require.NoError(suite.T(), err)

		time.Sleep(50 * time.Millisecond)

		suite.setupModuleLoader(string(suite.parseTestdata("up-to-date-no-weight.yaml")))
		err = suite.loader.restoreAbsentModulesFromOverrides(context.TODO())
		require.NoError(suite.T(), err)

		newstat, err := os.Stat(moduleDir)
		require.NoError(suite.T(), err)
		assert.True(suite.T(), stat.ModTime().Equal(newstat.ModTime()), "Module's dir mustn't be modified")

		newsstat, err := os.Lstat(symlink)
		require.NoError(suite.T(), err)
		assert.False(suite.T(), sstat.ModTime().Equal(newsstat.ModTime()), "Module's symlink must be modified")

		mpo := suite.modulePullOverride(moduleName)
		assert.Equalf(suite.T(), mpo.Annotations[v1alpha1.ModulePullOverrideAnnotationDeployedOn], "dev-master-0", "%s must be set to dev-master-0", v1alpha1.ModulePullOverrideAnnotationDeployedOn)
		assert.Equalf(suite.T(), mpo.Status.Weight, uint32(900), "Module's weight must be set to %d", 900)

		require.NoError(suite.T(), cleanupPaths(desc.dir, symlink))
	})

	suite.Run("Module is absent and deletionTimestamp is set", func() {
		suite.setupModuleLoader(string(suite.parseTestdata("deleted.yaml")))
		err := suite.loader.restoreAbsentModulesFromOverrides(context.TODO())
		require.NoError(suite.T(), err)

		_, err = os.Stat(moduleDir)
		assert.True(suite.T(), os.IsNotExist(err), "Module's dir mustn't exist")

		_, err = os.Lstat(filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("910-%s", moduleName)))
		assert.True(suite.T(), os.IsNotExist(err), "Module's symlink mustn't exist")
	})
}

func (suite *ModuleLoaderTestSuite) modulePullOverride(name string) *v1alpha2.ModulePullOverride {
	mpo := new(v1alpha2.ModulePullOverride)
	err := suite.client.Get(context.TODO(), client.ObjectKey{Name: name}, mpo)
	require.NoError(suite.T(), err)

	return mpo
}

func (suite *ModuleLoaderTestSuite) fetchResults() []byte {
	result := bytes.NewBuffer(nil)

	sources := new(v1alpha1.ModuleSourceList)
	err := suite.client.List(context.TODO(), sources)
	require.NoError(suite.T(), err)

	for _, item := range sources.Items {
		got, _ := yaml.Marshal(item)
		result.WriteString("---\n")
		result.Write(got)
	}

	mpos := new(v1alpha2.ModulePullOverrideList)
	err = suite.client.List(context.TODO(), mpos)
	require.NoError(suite.T(), err)

	for _, item := range mpos.Items {
		got, _ := yaml.Marshal(item)
		result.WriteString("---\n")
		result.Write(got)
	}

	return result.Bytes()
}

func (suite *ModuleLoaderTestSuite) parseTestdata(filename string) []byte {
	dir := "./testdata/override"
	data, err := os.ReadFile(filepath.Join(dir, filename))
	require.NoError(suite.T(), err)

	suite.testDataFileName = filename

	return data
}

type moduleDirDescriptor struct {
	dir     string
	values  string
	symlink string
}

func (d *moduleDirDescriptor) prepareModuleDir() error {
	if d.dir != "" {
		if err := os.MkdirAll(d.dir, 0750); err != nil {
			return err
		}
		if d.values != "" {
			openAPIDir := filepath.Join(d.dir, "openapi")
			if err := os.MkdirAll(openAPIDir, 0750); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(openAPIDir, "values.yaml"), []byte(d.values), 0644); err != nil {
				return err
			}
		}
	}

	if d.symlink != "" {
		if err := os.Symlink(d.dir, d.symlink); err != nil {
			return err
		}
	}

	return nil
}

func cleanupPaths(paths ...string) error {
	for _, p := range paths {
		err := os.RemoveAll(p)
		if err != nil {
			return err
		}
	}

	return nil
}
