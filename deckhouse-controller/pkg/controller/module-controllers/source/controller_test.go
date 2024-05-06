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

package source

import (
	"bytes"
	"context"
	"flag"
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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var golden bool

func init() {
	flag.BoolVar(&golden, "golden", false, "generate golden files")
}

func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}

type ControllerTestSuite struct {
	suite.Suite

	kubeClient client.Client
	ctr        *moduleSourceReconciler

	testDataFileName string
	testMSName       string
	compareGolden    bool
}

func (suite *ControllerTestSuite) SetupSuite() {
	flag.Parse()
	suite.T().Setenv("D8_IS_TESTS_ENVIRONMENT", "true")
}

func (suite *ControllerTestSuite) BeforeTest(suiteName, testName string) {
	if suiteName == "ControllerTestSuite" && testName == "TestCreateReconcile" {
		suite.compareGolden = true
	}
}

func (suite *ControllerTestSuite) AfterTest(_, _ string) {
	suite.compareGolden = false
}

func (suite *ControllerTestSuite) TearDownSubTest() {
	if !suite.compareGolden {
		return
	}

	goldenFile := filepath.Join("./testdata", "golden", suite.testDataFileName)
	got := suite.fetchResults()

	if golden {
		err := os.WriteFile(goldenFile, got, 0666)
		require.NoError(suite.T(), err)
	} else {
		exp, err := os.ReadFile(goldenFile)
		require.NoError(suite.T(), err)
		assert.YAMLEq(suite.T(), string(exp), string(got))
	}
}

func (suite *ControllerTestSuite) TestCreateReconcile() {
	entries, err := os.ReadDir("./testdata")
	require.NoError(suite.T(), err)

	suite.Run("testdata cases", func() {
		dependency.TestDC.CRClient.ListTagsMock.Return([]string{"foo", "bar"}, nil)
		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{LayersStub: func() ([]v1.Layer, error) {
			return []v1.Layer{&utils.FakeLayer{}, &utils.FakeLayer{FilesContent: map[string]string{"version.json": `{"version": "v1.2.3"}`}}}, nil
		}}, nil)

		for _, en := range entries {
			if en.IsDir() {
				continue
			}

			suite.Run(en.Name(), func() {
				suite.setupController(string(suite.fetchTestFileData(en.Name())))
				ms := suite.getModuleSource(suite.testMSName)
				_, err := suite.ctr.createOrUpdateReconcile(context.TODO(), ms)
				require.NoError(suite.T(), err)
			})
		}
	})
}

func (suite *ControllerTestSuite) fetchTestFileData(filename string) []byte {
	dir := "./testdata"
	data, err := os.ReadFile(filepath.Join(dir, filename))
	require.NoError(suite.T(), err)

	suite.testDataFileName = filename

	return data
}

func (suite *ControllerTestSuite) TestDeleteReconcile() {
	suite.Run("not found ModuleSource, must truncate the checksum map", func() {
		suite.setupController("")
		suite.ctr.moduleSourcesChecksum["not-found"] = map[string]string{"foo": "bar"}
		_, err := suite.ctr.Reconcile(context.TODO(), controllerruntime.Request{NamespacedName: types.NamespacedName{Name: "not-found"}})
		require.NoError(suite.T(), err)

		assert.Len(suite.T(), suite.ctr.moduleSourcesChecksum["not-found"], 0)
	})

	suite.Run("ModuleSource with finalizer and empty releases", func() {
		m := `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: test-source
  finalizers:
  - modules.deckhouse.io/release-exists
spec:
  registry:
    dockerCfg: YXNiCg==
    repo: dev-registry.deckhouse.io/deckhouse/modules
    scheme: HTTPS
`
		suite.setupController(m)

		ms := suite.getModuleSource("test-source")

		result, err := suite.ctr.deleteReconcile(context.TODO(), ms)
		require.NoError(suite.T(), err)
		assert.False(suite.T(), result.Requeue)
		assert.Empty(suite.T(), result.RequeueAfter)

		ms = suite.getModuleSource("test-source")
		require.NoError(suite.T(), err)
		assert.Len(suite.T(), ms.Finalizers, 0)
	})

	suite.Run("ModuleSource with finalizer and release", func() {
		m := `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: test-source-2
  finalizers:
  - modules.deckhouse.io/release-exists
spec:
  registry:
    dockerCfg: YXNiCg==
    repo: dev-registry.deckhouse.io/deckhouse/modules
    scheme: HTTPS
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleRelease
metadata:
  labels:
    module: some-module
    release-checksum: ed8ed428a470a76e30ed4f50dd7cf570
    source: test-source-2
    status: deployed
  name: some-module-v0.0.1
  ownerReferences:
  - apiVersion: deckhouse.io/v1alpha1
    controller: true
    kind: ModuleSource
    name: test-source-2
    uid: ec6c2028-39bd-4068-bbda-84587e63e4c4
spec:
  moduleName: some-module
  version: 0.0.1
  weight: 900
status:
  approved: false
  message: ""
  phase: Deployed
`
		suite.setupController(m)

		ms := suite.getModuleSource("test-source-2")
		result, err := suite.ctr.deleteReconcile(context.TODO(), ms)
		require.NoError(suite.T(), err)
		assert.False(suite.T(), result.Requeue)
		assert.Equal(suite.T(), 5*time.Second, result.RequeueAfter)

		ms = suite.getModuleSource("test-source-2")
		require.NoError(suite.T(), err)
		assert.Len(suite.T(), ms.Finalizers, 1)
		assert.Equal(suite.T(), ms.Status.Msg, "ModuleSource contains at least 1 Deployed release and cannot be deleted. Please delete target ModuleReleases manually to continue")
	})

	suite.Run("ModuleSource with finalizer,annotation and release", func() {
		m := `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: test-source-3
  annotations:
    modules.deckhouse.io/force-delete: "true"
  finalizers:
  - modules.deckhouse.io/release-exists
spec:
  registry:
    dockerCfg: YXNiCg==
    repo: dev-registry.deckhouse.io/deckhouse/modules
    scheme: HTTPS
  releaseChannel: alpha
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleRelease
metadata:
  labels:
    module: some-module-2
    release-checksum: ed8ed428a470a76e30ed4f50dd7cf570
    source: test-source-3
    status: deployed
  name: some-module-2-v0.0.1
  ownerReferences:
  - apiVersion: deckhouse.io/v1alpha1
    controller: true
    kind: ModuleSource
    name: test-source-3
    uid: ec6c2028-39bd-4068-bbda-84587e63e4c4
spec:
  moduleName: some-module-2
  version: 0.0.1
  weight: 900
status:
  approved: false
  message: ""
  phase: Deployed
`

		suite.setupController(m)

		ms := suite.getModuleSource("test-source-3")

		result, err := suite.ctr.deleteReconcile(context.TODO(), ms)
		require.NoError(suite.T(), err)
		assert.False(suite.T(), result.Requeue)
		assert.Empty(suite.T(), result.RequeueAfter)

		ms = suite.getModuleSource("test-source-3")
		require.NoError(suite.T(), err)
		assert.Len(suite.T(), ms.Finalizers, 0)
	})
}

func (suite *ControllerTestSuite) assembleInitObject(obj string) client.Object {
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
		suite.testMSName = ms.Name

	case "ModuleRelease":
		var mr v1alpha1.ModuleRelease
		err = yaml.Unmarshal([]byte(obj), &mr)
		require.NoError(suite.T(), err)
		res = &mr

	case "ModuleUpdatePolicy":
		var mup v1alpha1.ModuleUpdatePolicy
		err = yaml.Unmarshal([]byte(obj), &mup)
		require.NoError(suite.T(), err)
		res = &mup
	}

	return res
}

func (suite *ControllerTestSuite) fetchResults() []byte {
	result := bytes.NewBuffer(nil)

	var mslist v1alpha1.ModuleSourceList
	err := suite.kubeClient.List(context.TODO(), &mslist)
	require.NoError(suite.T(), err)

	for _, item := range mslist.Items {
		got, _ := yaml.Marshal(item)
		result.WriteString("---\n")
		result.Write(got)
	}

	var mrlist v1alpha1.ModuleReleaseList
	err = suite.kubeClient.List(context.TODO(), &mrlist)
	require.NoError(suite.T(), err)

	for _, item := range mrlist.Items {
		got, _ := yaml.Marshal(item)
		result.WriteString("---\n")
		result.Write(got)
	}

	return result.Bytes()
}

func (suite *ControllerTestSuite) createFakeModuleSource(yamlObj string) *v1alpha1.ModuleSource {
	var ms v1alpha1.ModuleSource
	err := yaml.Unmarshal([]byte(yamlObj), &ms)
	require.NoError(suite.T(), err)

	err = suite.kubeClient.Create(context.TODO(), &ms)
	require.NoError(suite.T(), err)

	return &ms
}

func (suite *ControllerTestSuite) getModuleSource(name string) *v1alpha1.ModuleSource {
	var ms v1alpha1.ModuleSource
	err := suite.kubeClient.Get(context.TODO(), types.NamespacedName{Name: name}, &ms)
	require.NoError(suite.T(), err)

	return &ms
}

func (suite *ControllerTestSuite) TestInvalidRegistry() {
	suite.T().Setenv("D8_IS_TESTS_ENVIRONMENT", "false")
	invalidMS := `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: test-source
spec:
  registry:
    dockerCfg: YXNiCg==
    repo: dev-registry.deckhouse.io/deckhouse/modules
    scheme: HTTPS
`
	ms := suite.createFakeModuleSource(invalidMS)
	_, err := suite.ctr.Reconcile(context.TODO(), controllerruntime.Request{NamespacedName: types.NamespacedName{Name: ms.Name}})
	require.NoError(suite.T(), err)

	ms = suite.getModuleSource(ms.Name)
	assert.Contains(suite.T(), ms.Status.Msg, "credentials not found in the dockerCfg")
	assert.Len(suite.T(), ms.Status.AvailableModules, 0)
}

func (suite *ControllerTestSuite) setupController(yamlDoc string) {
	manifests := releaseutil.SplitManifests(yamlDoc)

	var initObjects = make([]client.Object, 0, len(manifests))
	for _, manifest := range manifests {
		obj := suite.assembleInitObject(manifest)
		initObjects = append(initObjects, obj)
	}

	sc := runtime.NewScheme()
	_ = v1alpha1.SchemeBuilder.AddToScheme(sc)
	cl := fake.NewClientBuilder().WithScheme(sc).WithObjects(initObjects...).WithStatusSubresource(&v1alpha1.ModuleSource{}).Build()

	rec := &moduleSourceReconciler{
		client:             cl,
		externalModulesDir: os.Getenv("EXTERNAL_MODULES_DIR"),
		dc:                 dependency.NewDependencyContainer(),
		logger:             log.New(),

		deckhouseEmbeddedPolicy: &v1alpha1.ModuleUpdatePolicySpec{
			Update: v1alpha1.ModuleUpdatePolicySpecUpdate{
				Mode: "Auto",
			},
			ReleaseChannel: "Stable",
		},
		moduleSourcesChecksum: make(sourceChecksum),
	}

	suite.ctr = rec
	suite.kubeClient = cl
}
