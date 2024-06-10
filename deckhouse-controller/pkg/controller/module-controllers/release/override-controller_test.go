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

package release

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
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"helm.sh/helm/v3/pkg/releaseutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/downloader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
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

func TestPullOverrideControllerTestSuite(t *testing.T) {
	suite.Run(t, new(PullOverrideControllerTestSuite))
}

type PullOverrideControllerTestSuite struct {
	suite.Suite

	kubeClient client.Client
	ctr        *modulePullOverrideReconciler

	testDataFileName string
	testMPOName      string

	tmpDir string
}

func (suite *PullOverrideControllerTestSuite) SetupSuite() {
	flag.Parse()
	suite.T().Setenv("D8_IS_TESTS_ENVIRONMENT", "true")
	suite.T().Setenv("DECKHOUSE_NODE_NAME", "dev-master-0")
	suite.tmpDir = suite.T().TempDir()
	suite.T().Setenv("EXTERNAL_MODULES_DIR", suite.tmpDir)
	_ = os.MkdirAll(filepath.Join(suite.tmpDir, "modules"), 0777)
}

type moduleDirDescriptor struct {
	dir     string
	values  string
	symlink string
}

func prepareModuleDir(d moduleDirDescriptor) error {
	if d.dir != "" {
		err := os.MkdirAll(d.dir, 0750)
		if err != nil {
			return err
		}
		if d.values != "" {
			openAPIDir := filepath.Join(d.dir, "openapi")
			err := os.MkdirAll(openAPIDir, 0750)
			if err != nil {
				return err
			}

			valuesFile := filepath.Join(openAPIDir, "values.yaml")
			err = os.WriteFile(valuesFile, []byte(d.values), 0644)
			if err != nil {
				return err
			}
		}
	}

	if d.symlink != "" {
		dir := d.dir
		if dir == "" {
			dir = filepath.Join("../", "some-module", downloader.DefaultDevVersion)
		}
		err := os.Symlink(dir, d.symlink)
		if err != nil {
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

func (suite *PullOverrideControllerTestSuite) TestRestoreAbsentModulesFromOverrides() {
	moduleName := "echo"
	moduleDir := filepath.Join(suite.tmpDir, moduleName, downloader.DefaultDevVersion)
	moduleValues := filepath.Join(moduleDir, "openapi", "values.yaml")

	suite.Run("Ok", func() {
		d := moduleDirDescriptor{
			dir:     moduleDir,
			values:  values,
			symlink: filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("910-%s", moduleName)),
		}
		err := prepareModuleDir(d)
		require.NoError(suite.T(), err)

		stat, err := os.Stat(moduleValues)
		require.NoError(suite.T(), err)

		sstat, err := os.Lstat(d.symlink)
		require.NoError(suite.T(), err)

		time.Sleep(50 * time.Millisecond)

		suite.setupPullOverrideController(string(suite.fetchTestFileData("up-to-date.yaml")))
		err = suite.ctr.restoreAbsentModulesFromOverrides(context.TODO())
		require.NoError(suite.T(), err)

		newstat, err := os.Stat(moduleValues)
		require.NoError(suite.T(), err)
		assert.True(suite.T(), stat.ModTime().Equal(newstat.ModTime()), "Module's values.yaml mustn't be modified")

		newsstat, err := os.Lstat(d.symlink)
		require.NoError(suite.T(), err)
		assert.True(suite.T(), sstat.ModTime().Equal(newsstat.ModTime()), "Module's symlink mustn't be modified")

		mpo := suite.getModulePullOverride(moduleName)
		assert.Equalf(suite.T(), mpo.Annotations[deckhouseNodeNameAnnotation], "dev-master-0", "%s must be set to dev-master-0", deckhouseNodeNameAnnotation)
		assert.Equal(suite.T(), mpo.Status.Weight, uint32(910), "Module's weight mustn't be modified")

		require.NoError(suite.T(), cleanupPaths(d.dir, d.symlink))
	})

	suite.Run("No weight, no module.yaml", func() {
		d := moduleDirDescriptor{
			dir:     moduleDir,
			values:  values,
			symlink: filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("900-%s", moduleName)),
		}
		err := prepareModuleDir(d)
		require.NoError(suite.T(), err)

		stat, err := os.Stat(moduleValues)
		require.NoError(suite.T(), err)

		sstat, err := os.Lstat(d.symlink)
		require.NoError(suite.T(), err)

		time.Sleep(50 * time.Millisecond)

		suite.setupPullOverrideController(string(suite.fetchTestFileData("up-to-date-no-weight.yaml")))
		err = suite.ctr.restoreAbsentModulesFromOverrides(context.TODO())
		require.NoError(suite.T(), err)

		newstat, err := os.Stat(moduleValues)
		require.NoError(suite.T(), err)
		assert.True(suite.T(), stat.ModTime().Equal(newstat.ModTime()), "Module's values.yaml mustn't be modified")

		newsstat, err := os.Lstat(d.symlink)
		require.NoError(suite.T(), err)
		assert.True(suite.T(), sstat.ModTime().Equal(newsstat.ModTime()), "Module's symlink mustn't be modified")

		mpo := suite.getModulePullOverride(moduleName)
		assert.Equalf(suite.T(), mpo.Annotations[deckhouseNodeNameAnnotation], "dev-master-0", "%s must be set to dev-master-0", deckhouseNodeNameAnnotation)
		assert.Equal(suite.T(), mpo.Status.Weight, uint32(900), "dev-master-0", "Module's weight must be set to 900")

		require.NoError(suite.T(), cleanupPaths(d.dir, d.symlink))
	})

	suite.Run("No weight", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{LayersStub: func() ([]v1.Layer, error) {
			return []v1.Layer{&utils.FakeLayer{}, &utils.FakeLayer{FilesContent: map[string]string{"module.yaml": "weight: 915"}}}, nil
		}}, nil)

		d := moduleDirDescriptor{
			dir:     moduleDir,
			values:  values,
			symlink: filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("915-%s", moduleName)),
		}
		err := prepareModuleDir(d)
		require.NoError(suite.T(), err)

		stat, err := os.Stat(moduleValues)
		require.NoError(suite.T(), err)

		sstat, err := os.Lstat(d.symlink)
		require.NoError(suite.T(), err)

		time.Sleep(50 * time.Millisecond)

		suite.setupPullOverrideController(string(suite.fetchTestFileData("up-to-date-no-weight.yaml")))
		err = suite.ctr.restoreAbsentModulesFromOverrides(context.TODO())
		require.NoError(suite.T(), err)

		newstat, err := os.Stat(moduleValues)
		require.NoError(suite.T(), err)
		assert.True(suite.T(), stat.ModTime().Equal(newstat.ModTime()), "Module's values.yaml mustn't be modified")

		newsstat, err := os.Lstat(d.symlink)
		require.NoError(suite.T(), err)
		assert.True(suite.T(), sstat.ModTime().Equal(newsstat.ModTime()), "Module's symlink mustn't be modified")

		mpo := suite.getModulePullOverride(moduleName)
		assert.Equalf(suite.T(), mpo.Annotations[deckhouseNodeNameAnnotation], "dev-master-0", "%s must be set to dev-master-0", deckhouseNodeNameAnnotation)
		assert.Equal(suite.T(), mpo.Status.Weight, uint32(915), "dev-master-0", "Module's weight must be set to 915")

		require.NoError(suite.T(), cleanupPaths(d.dir, d.symlink))
	})

	suite.Run("Stale module values", func() {
		d := moduleDirDescriptor{
			dir:     moduleDir,
			values:  "someKey: value",
			symlink: filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("910-%s", moduleName)),
		}
		err := prepareModuleDir(d)
		require.NoError(suite.T(), err)

		stat, err := os.Stat(moduleValues)
		require.NoError(suite.T(), err)

		sstat, err := os.Lstat(d.symlink)
		require.NoError(suite.T(), err)

		time.Sleep(50 * time.Millisecond)

		suite.setupPullOverrideController(string(suite.fetchTestFileData("up-to-date.yaml")))
		err = suite.ctr.restoreAbsentModulesFromOverrides(context.TODO())
		require.NoError(suite.T(), err)

		newstat, err := os.Stat(moduleValues)
		require.NoError(suite.T(), err)
		assert.False(suite.T(), stat.ModTime().Equal(newstat.ModTime()), "Module's values.yaml must be modified")

		newsstat, err := os.Lstat(d.symlink)
		require.NoError(suite.T(), err)
		assert.True(suite.T(), sstat.ModTime().Equal(newsstat.ModTime()), "Module's symlink mustn't be modified")

		mpo := suite.getModulePullOverride(moduleName)
		assert.Equalf(suite.T(), mpo.Annotations[deckhouseNodeNameAnnotation], "dev-master-0", "%s must be set to dev-master-0", deckhouseNodeNameAnnotation)
		assert.Equal(suite.T(), mpo.Status.Weight, uint32(910), "dev-master-0", "Module's weight must be set to 910")

		require.NoError(suite.T(), cleanupPaths(d.dir, d.symlink))
	})

	suite.Run("Old deployed-on annotation", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{LayersStub: func() ([]v1.Layer, error) {
			return []v1.Layer{&utils.FakeLayer{}, &utils.FakeLayer{FilesContent: map[string]string{"openapi/values.yaml": "{}"}}}, nil
		}}, nil)
		d := moduleDirDescriptor{
			dir:     moduleDir,
			values:  values,
			symlink: filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("900-%s", moduleName)),
		}
		err := prepareModuleDir(d)
		require.NoError(suite.T(), err)

		stat, err := os.Stat(d.dir)
		require.NoError(suite.T(), err)

		sstat, err := os.Lstat(d.symlink)
		require.NoError(suite.T(), err)

		time.Sleep(50 * time.Millisecond)

		suite.setupPullOverrideController(string(suite.fetchTestFileData("old-deckhouse-node-name-annotation.yaml")))
		err = suite.ctr.restoreAbsentModulesFromOverrides(context.TODO())
		require.NoError(suite.T(), err)

		newstat, err := os.Stat(d.dir)
		require.NoError(suite.T(), err)
		assert.False(suite.T(), stat.ModTime().Equal(newstat.ModTime()), "Module's dir must be modified")

		newsstat, err := os.Lstat(d.symlink)
		require.NoError(suite.T(), err)
		assert.False(suite.T(), sstat.ModTime().Equal(newsstat.ModTime()), "Module's symlink must be modified")

		mpo := suite.getModulePullOverride(moduleName)
		assert.Equalf(suite.T(), mpo.Annotations[deckhouseNodeNameAnnotation], "dev-master-0", "%s must be set to dev-master-0", deckhouseNodeNameAnnotation)
		assert.Equalf(suite.T(), mpo.Status.Weight, uint32(900), "dev-master-0", "Module's weight must be set to %d", 900)

		require.NoError(suite.T(), cleanupPaths(d.dir, d.symlink))
	})

	suite.Run("No deployed-on annotation", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{LayersStub: func() ([]v1.Layer, error) {
			return []v1.Layer{&utils.FakeLayer{}, &utils.FakeLayer{FilesContent: map[string]string{"openapi/values.yaml": "{}"}}}, nil
		}}, nil)
		d := moduleDirDescriptor{
			dir:     moduleDir,
			values:  values,
			symlink: filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("900-%s", moduleName)),
		}
		err := prepareModuleDir(d)
		require.NoError(suite.T(), err)

		stat, err := os.Stat(d.dir)
		require.NoError(suite.T(), err)

		sstat, err := os.Lstat(d.symlink)
		require.NoError(suite.T(), err)

		time.Sleep(50 * time.Millisecond)

		suite.setupPullOverrideController(string(suite.fetchTestFileData("no-deckhouse-node-name-annotation.yaml")))
		err = suite.ctr.restoreAbsentModulesFromOverrides(context.TODO())
		require.NoError(suite.T(), err)

		newstat, err := os.Stat(d.dir)
		require.NoError(suite.T(), err)
		assert.False(suite.T(), stat.ModTime().Equal(newstat.ModTime()), "Module's dir must be modified")

		newsstat, err := os.Lstat(d.symlink)
		require.NoError(suite.T(), err)
		assert.False(suite.T(), sstat.ModTime().Equal(newsstat.ModTime()), "Module's symlink must be modified")

		mpo := suite.getModulePullOverride(moduleName)
		assert.Equalf(suite.T(), mpo.Annotations[deckhouseNodeNameAnnotation], "dev-master-0", "%s must be set to dev-master-0", deckhouseNodeNameAnnotation)
		assert.Equalf(suite.T(), mpo.Status.Weight, uint32(900), "dev-master-0", "Module's weight must be set to %d", 900)

		require.NoError(suite.T(), cleanupPaths(d.dir, d.symlink))
	})

	suite.Run("No symlink", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{LayersStub: func() ([]v1.Layer, error) {
			return []v1.Layer{&utils.FakeLayer{}, &utils.FakeLayer{FilesContent: map[string]string{"openapi/values.yaml": "{}"}}}, nil
		}}, nil)
		symlink := filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("900-%s", moduleName))
		d := moduleDirDescriptor{
			dir:    moduleDir,
			values: values,
		}
		err := prepareModuleDir(d)
		require.NoError(suite.T(), err)

		stat, err := os.Stat(d.dir)
		require.NoError(suite.T(), err)

		_, err = os.Lstat(symlink)
		assert.True(suite.T(), os.IsNotExist(err), "Module's symlink mustn't exist")

		time.Sleep(50 * time.Millisecond)

		suite.setupPullOverrideController(string(suite.fetchTestFileData("up-to-date-no-weight.yaml")))
		err = suite.ctr.restoreAbsentModulesFromOverrides(context.TODO())
		require.NoError(suite.T(), err)

		newstat, err := os.Stat(d.dir)
		require.NoError(suite.T(), err)
		assert.True(suite.T(), stat.ModTime().Equal(newstat.ModTime()), "Module's dir mustn't be modified")

		_, err = os.Lstat(symlink)
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), err, nil, "Module's symlink must be created")

		mpo := suite.getModulePullOverride(moduleName)
		assert.Equalf(suite.T(), mpo.Annotations[deckhouseNodeNameAnnotation], "dev-master-0", "%s must be set to dev-master-0", deckhouseNodeNameAnnotation)
		assert.Equalf(suite.T(), mpo.Status.Weight, uint32(900), "dev-master-0", "Module's weight must be set to %d", 900)

		require.NoError(suite.T(), cleanupPaths(d.dir, symlink))
	})

	suite.Run("Extra symlinks", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{LayersStub: func() ([]v1.Layer, error) {
			return []v1.Layer{&utils.FakeLayer{}, &utils.FakeLayer{FilesContent: map[string]string{"openapi/values.yaml": "{}"}}}, nil
		}}, nil)
		symlink := filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("900-%s", moduleName))
		symlink1 := filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("901-%s", moduleName))
		symlink2 := filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("902-%s", moduleName))
		symlink3 := filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("903-%s", moduleName))
		d := moduleDirDescriptor{
			dir:    moduleDir,
			values: values,
		}
		err := prepareModuleDir(d)

		// extra symlinks
		require.NoError(suite.T(), err)
		err = os.Symlink(d.dir, symlink1)
		require.NoError(suite.T(), err)
		err = os.Symlink(d.dir, symlink2)
		require.NoError(suite.T(), err)
		err = os.Symlink(d.dir, symlink3)
		require.NoError(suite.T(), err)

		stat, err := os.Stat(d.dir)
		require.NoError(suite.T(), err)

		_, err = os.Lstat(symlink)
		assert.True(suite.T(), os.IsNotExist(err), "Module's symlink mustn't exist")

		time.Sleep(50 * time.Millisecond)

		suite.setupPullOverrideController(string(suite.fetchTestFileData("up-to-date-no-weight.yaml")))
		err = suite.ctr.restoreAbsentModulesFromOverrides(context.TODO())
		require.NoError(suite.T(), err)

		newstat, err := os.Stat(d.dir)
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

		mpo := suite.getModulePullOverride(moduleName)
		assert.Equalf(suite.T(), mpo.Annotations[deckhouseNodeNameAnnotation], "dev-master-0", "%s must be set to dev-master-0", deckhouseNodeNameAnnotation)
		assert.Equalf(suite.T(), mpo.Status.Weight, uint32(900), "dev-master-0", "Module's weight must be set to %d", 900)

		require.NoError(suite.T(), cleanupPaths(d.dir, symlink, symlink1, symlink2, symlink3))
	})

	suite.Run("No module dir", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{LayersStub: func() ([]v1.Layer, error) {
			return []v1.Layer{&utils.FakeLayer{}, &utils.FakeLayer{FilesContent: map[string]string{"openapi/values.yaml": "{}"}}}, nil
		}}, nil)

		d := moduleDirDescriptor{
			values:  values,
			symlink: filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("900-%s", moduleName)),
		}
		err := prepareModuleDir(d)
		require.NoError(suite.T(), err)

		_, err = os.Stat(moduleDir)
		assert.True(suite.T(), os.IsNotExist(err), "Modules's dir mustn't exist")

		sstat, err := os.Lstat(d.symlink)
		require.NoError(suite.T(), err)

		time.Sleep(50 * time.Millisecond)

		suite.setupPullOverrideController(string(suite.fetchTestFileData("up-to-date-no-weight.yaml")))
		err = suite.ctr.restoreAbsentModulesFromOverrides(context.TODO())
		require.NoError(suite.T(), err)

		_, err = os.Stat(moduleDir)
		require.NoError(suite.T(), err)

		newsstat, err := os.Lstat(d.symlink)
		require.NoError(suite.T(), err)
		assert.False(suite.T(), sstat.ModTime().Equal(newsstat.ModTime()), "Module's symlink mustn't be modified")

		mpo := suite.getModulePullOverride(moduleName)
		assert.Equalf(suite.T(), mpo.Annotations[deckhouseNodeNameAnnotation], "dev-master-0", "%s must be set to dev-master-0", deckhouseNodeNameAnnotation)
		assert.Equalf(suite.T(), mpo.Status.Weight, uint32(900), "dev-master-0", "Module's weight must be set to %d", 900)

		require.NoError(suite.T(), cleanupPaths(moduleDir, d.symlink))
	})

	suite.Run("Wrong symlink", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{LayersStub: func() ([]v1.Layer, error) {
			return []v1.Layer{&utils.FakeLayer{}, &utils.FakeLayer{FilesContent: map[string]string{"openapi/values.yaml": "{}"}}}, nil
		}}, nil)

		symlink := filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("900-%s", moduleName))
		d := moduleDirDescriptor{
			dir:    moduleDir,
			values: values,
		}
		err := prepareModuleDir(d)
		require.NoError(suite.T(), err)

		err = os.Symlink("../notEcho/fakeVersion", symlink)
		require.NoError(suite.T(), err)

		stat, err := os.Stat(moduleDir)
		require.NoError(suite.T(), err)

		sstat, err := os.Lstat(symlink)
		require.NoError(suite.T(), err)

		time.Sleep(50 * time.Millisecond)

		suite.setupPullOverrideController(string(suite.fetchTestFileData("up-to-date-no-weight.yaml")))
		err = suite.ctr.restoreAbsentModulesFromOverrides(context.TODO())
		require.NoError(suite.T(), err)

		newstat, err := os.Stat(moduleDir)
		require.NoError(suite.T(), err)
		assert.True(suite.T(), stat.ModTime().Equal(newstat.ModTime()), "Module's dir mustn't be modified")

		newsstat, err := os.Lstat(symlink)
		require.NoError(suite.T(), err)
		assert.False(suite.T(), sstat.ModTime().Equal(newsstat.ModTime()), "Module's symlink must be modified")

		mpo := suite.getModulePullOverride(moduleName)
		assert.Equalf(suite.T(), mpo.Annotations[deckhouseNodeNameAnnotation], "dev-master-0", "%s must be set to dev-master-0", deckhouseNodeNameAnnotation)
		assert.Equalf(suite.T(), mpo.Status.Weight, uint32(900), "dev-master-0", "Module's weight must be set to %d", 900)

		require.NoError(suite.T(), cleanupPaths(d.dir, symlink))
	})

	suite.Run("Module is absent and deletionTimestamp is set", func() {
		suite.setupPullOverrideController(string(suite.fetchTestFileData("deleted.yaml")))
		err := suite.ctr.restoreAbsentModulesFromOverrides(context.TODO())
		require.NoError(suite.T(), err)

		_, err = os.Stat(moduleDir)
		assert.True(suite.T(), os.IsNotExist(err), "Module's dir mustn't exist")

		_, err = os.Lstat(filepath.Join(suite.tmpDir, "modules", fmt.Sprintf("910-%s", moduleName)))
		assert.True(suite.T(), os.IsNotExist(err), "Module's symlink mustn't exist")
	})
}

func (suite *PullOverrideControllerTestSuite) setupPullOverrideController(yamlDoc string) {
	manifests := releaseutil.SplitManifests(yamlDoc)

	var initObjects = make([]client.Object, 0, len(manifests))

	for _, manifest := range manifests {
		obj := suite.assembleInitObject(manifest)
		initObjects = append(initObjects, obj)
	}

	sc := runtime.NewScheme()
	_ = v1alpha1.SchemeBuilder.AddToScheme(sc)
	_ = corev1.AddToScheme(sc)
	cl := fake.NewClientBuilder().WithScheme(sc).WithObjects(initObjects...).WithStatusSubresource(&v1alpha1.ModuleSource{}, &v1alpha1.ModulePullOverride{}).Build()

	rec := &modulePullOverrideReconciler{
		client:             cl,
		externalModulesDir: os.Getenv("EXTERNAL_MODULES_DIR"),
		dc:                 dependency.NewDependencyContainer(),
		logger:             log.New(),
		symlinksDir:        filepath.Join(os.Getenv("EXTERNAL_MODULES_DIR"), "modules"),
		moduleManager:      stubModulesManager{},
	}

	suite.ctr = rec
	suite.kubeClient = cl
}

func (suite *PullOverrideControllerTestSuite) assembleInitObject(obj string) client.Object {
	var res client.Object

	var typ runtime.TypeMeta

	err := yaml.Unmarshal([]byte(obj), &typ)
	require.NoError(suite.T(), err)

	switch typ.Kind {
	case "ModuleSource":
		var ms v1alpha1.ModuleSource
		err = yaml.Unmarshal([]byte(obj), &ms)
		require.NoError(suite.T(), err)
		res = &ms

	case "ModulePullOverride":
		var mpo v1alpha1.ModulePullOverride
		err = yaml.Unmarshal([]byte(obj), &mpo)
		require.NoError(suite.T(), err)
		res = &mpo
		suite.testMPOName = mpo.Name
	}

	return res
}

func (suite *PullOverrideControllerTestSuite) getModulePullOverride(name string) *v1alpha1.ModulePullOverride {
	var mpo v1alpha1.ModulePullOverride
	err := suite.kubeClient.Get(context.TODO(), types.NamespacedName{Name: name}, &mpo)
	require.NoError(suite.T(), err)

	return &mpo
}

func (suite *PullOverrideControllerTestSuite) fetchResults() []byte {
	result := bytes.NewBuffer(nil)

	var mslist v1alpha1.ModuleSourceList
	err := suite.kubeClient.List(context.TODO(), &mslist)
	require.NoError(suite.T(), err)

	for _, item := range mslist.Items {
		got, _ := yaml.Marshal(item)
		result.WriteString("---\n")
		result.Write(got)
	}

	var mpolist v1alpha1.ModulePullOverrideList
	err = suite.kubeClient.List(context.TODO(), &mpolist)
	require.NoError(suite.T(), err)

	for _, item := range mpolist.Items {
		got, _ := yaml.Marshal(item)
		result.WriteString("---\n")
		result.Write(got)
	}

	return result.Bytes()
}

func (suite *PullOverrideControllerTestSuite) fetchTestFileData(filename string) []byte {
	dir := "./testdata/pulloverrideController"
	data, err := os.ReadFile(filepath.Join(dir, filename))
	require.NoError(suite.T(), err)

	suite.testDataFileName = filename

	return data
}
