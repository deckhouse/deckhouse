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
	"errors"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"testing"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	crfake "github.com/google/go-containerregistry/pkg/v1/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/go_lib/d8env"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/pkg/log"
)

var (
	golden       bool
	mDelimiter   *regexp.Regexp
	ManifestStub = func() (*v1.Manifest, error) {
		return &v1.Manifest{
			Layers: []v1.Descriptor{},
		}, nil
	}
)

func init() {
	flag.BoolVar(&golden, "golden", false, "generate golden files")
	mDelimiter = regexp.MustCompile("(?m)^---$")
}

func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}

type ControllerTestSuite struct {
	suite.Suite

	kubeClient client.Client
	ctr        *reconciler

	testDataFileName string
	testMSName       string
	compareGolden    bool
}

func (suite *ControllerTestSuite) SetupSuite() {
	flag.Parse()
	suite.T().Setenv("D8_IS_TESTS_ENVIRONMENT", "true")
}

func (suite *ControllerTestSuite) setupTestController(yamlDoc string) {
	manifests := releaseutil.SplitManifests(yamlDoc)

	var initObjects = make([]client.Object, 0, len(manifests))
	for _, manifest := range manifests {
		obj := suite.parseObject(manifest)
		initObjects = append(initObjects, obj)
	}

	sc := runtime.NewScheme()
	_ = v1alpha1.SchemeBuilder.AddToScheme(sc)
	_ = v1alpha2.SchemeBuilder.AddToScheme(sc)
	cl := fake.NewClientBuilder().
		WithScheme(sc).
		WithObjects(initObjects...).
		WithStatusSubresource(&v1alpha1.Module{}, &v1alpha1.ModuleSource{}, &v1alpha1.ModuleRelease{}).
		Build()

	rec := &reconciler{
		init:                 new(sync.WaitGroup),
		client:               cl,
		downloadedModulesDir: d8env.GetDownloadedModulesDir(),
		dependencyContainer:  dependency.NewDependencyContainer(),
		log:                  log.NewNop(),

		embeddedPolicy: helpers.NewModuleUpdatePolicySpecContainer(&v1alpha2.ModuleUpdatePolicySpec{
			Update: v1alpha2.ModuleUpdatePolicySpecUpdate{
				Mode: "Auto",
			},
			ReleaseChannel: "Stable",
		}),
	}

	suite.ctr = rec
	suite.kubeClient = cl
}

func (suite *ControllerTestSuite) parseObject(obj string) client.Object {
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
		var mup v1alpha2.ModuleUpdatePolicy
		err = yaml.Unmarshal([]byte(obj), &mup)
		require.NoError(suite.T(), err)
		res = &mup

	case "Module":
		var m v1alpha1.Module
		err = yaml.Unmarshal([]byte(obj), &m)
		require.NoError(suite.T(), err)
		res = &m
	}

	return res
}

func (suite *ControllerTestSuite) BeforeTest(suiteName, testName string) {
	if suiteName == "ControllerTestSuite" && testName == "TestCreateReconcile" {
		suite.compareGolden = true
	}
}

func (suite *ControllerTestSuite) AfterTest(_, _ string) {
	suite.compareGolden = false
}

func (suite *ControllerTestSuite) SetupSubTest() {
	dependency.TestDC.CRClient = cr.NewClientMock(suite.T())
}

func (suite *ControllerTestSuite) TearDownSubTest() {
	if !suite.compareGolden {
		return
	}

	goldenFile := filepath.Join("./testdata", "golden", suite.testDataFileName)
	gotB := suite.fetchResults()

	if golden {
		err := os.WriteFile(goldenFile, gotB, 0666)
		require.NoError(suite.T(), err)
	} else {
		got := singleDocToManifests(gotB)
		expB, err := os.ReadFile(goldenFile)
		require.NoError(suite.T(), err)
		exp := singleDocToManifests(expB)
		assert.Equal(suite.T(), len(got), len(exp), "The number of `got` manifests must be equal to the number of `exp` manifests")
		for i := range got {
			assert.YAMLEq(suite.T(), exp[i], got[i], "Got and exp manifests must match")
		}
	}
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

func singleDocToManifests(doc []byte) (result []string) {
	split := mDelimiter.Split(string(doc), -1)

	for i := range split {
		if split[i] != "" {
			result = append(result, split[i])
		}
	}
	return
}

func (suite *ControllerTestSuite) TestCreateReconcile() {
	suite.Run("empty source", func() {
		dependency.TestDC.CRClient.ListTagsMock.Return([]string{"foo", "bar"}, nil)
		suite.setupTestController(string(suite.parseTestData("empty.yaml")))
		ms := suite.getModuleSource(suite.testMSName)
		_, err := suite.ctr.handleModuleSource(context.TODO(), ms)
		require.NoError(suite.T(), err)
	})

	suite.Run("source with modules", func() {
		dependency.TestDC.CRClient.ListTagsMock.Return([]string{"enabledmodule", "disabledmodule", "withpolicymodule", "notthissourcemodule"}, nil)
		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{
			ManifestStub: ManifestStub,
			LayersStub: func() ([]v1.Layer, error) {
				return []v1.Layer{&utils.FakeLayer{}, &utils.FakeLayer{FilesContent: map[string]string{"version.json": `{"version": "v1.2.3"}`}}}, nil
			},
			DigestStub: func() (v1.Hash, error) {
				return v1.Hash{Algorithm: "sha256"}, nil
			},
		}, nil)

		suite.setupTestController(string(suite.parseTestData("withmodules.yaml")))
		ms := suite.getModuleSource(suite.testMSName)
		_, err := suite.ctr.handleModuleSource(context.TODO(), ms)
		require.NoError(suite.T(), err)
	})

	suite.Run("source with module with pull error", func() {
		dependency.TestDC.CRClient.ListTagsMock.Return([]string{"enabledmodule", "errormodule"}, nil)
		dependency.TestDC.CRClient.ImageMock.Set(func(tag string) (i1 v1.Image, err error) {
			if tag == "alpha" {
				return nil, errors.New("GET https://registry.deckhouse.io/v2/deckhouse/ee/modules/errormodule/release/manifests/alpha:\n      MANIFEST_UNKNOWN: manifest unknown; map[Tag:alpha]")
			}

			return &crfake.FakeImage{
				ManifestStub: ManifestStub,
				LayersStub: func() ([]v1.Layer, error) {
					return []v1.Layer{&utils.FakeLayer{}, &utils.FakeLayer{FilesContent: map[string]string{"version.json": `{"version": "v1.2.3"}`}}}, nil
				},
				DigestStub: func() (v1.Hash, error) {
					return v1.Hash{Algorithm: "sha256"}, nil
				},
			}, nil
		})

		suite.setupTestController(string(suite.parseTestData("withmodulepullerror.yaml")))
		ms := suite.getModuleSource(suite.testMSName)
		_, err := suite.ctr.handleModuleSource(context.TODO(), ms)
		require.NoError(suite.T(), err)
	})
}

func (suite *ControllerTestSuite) parseTestData(filename string) []byte {
	dir := "./testdata"
	data, err := os.ReadFile(filepath.Join(dir, filename))
	require.NoError(suite.T(), err)

	suite.testDataFileName = filename

	return data
}

func (suite *ControllerTestSuite) TestDeleteReconcile() {
	suite.Run("source with finalizer and empty releases", func() {
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
		suite.setupTestController(m)

		ms := suite.getModuleSource("test-source")

		result, err := suite.ctr.deleteModuleSource(context.TODO(), ms)
		require.NoError(suite.T(), err)
		assert.False(suite.T(), result.Requeue)
		assert.Empty(suite.T(), result.RequeueAfter)

		ms = suite.getModuleSource("test-source")
		require.NoError(suite.T(), err)
		assert.Len(suite.T(), ms.Finalizers, 0)
	})

	suite.Run("source with finalizer and release", func() {
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
		suite.setupTestController(m)

		ms := suite.getModuleSource("test-source-2")
		result, err := suite.ctr.deleteModuleSource(context.TODO(), ms)
		require.NoError(suite.T(), err)
		assert.False(suite.T(), result.Requeue)
		assert.Equal(suite.T(), 5*time.Second, result.RequeueAfter)

		ms = suite.getModuleSource("test-source-2")
		require.NoError(suite.T(), err)
		assert.Len(suite.T(), ms.Finalizers, 1)
		assert.Equal(suite.T(), ms.Status.Message, "The source contains at least 1 deployed release and cannot be deleted. Please delete target ModuleReleases manually to continue")
	})

	suite.Run("source with finalizer, annotation and release", func() {
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

		suite.setupTestController(m)

		ms := suite.getModuleSource("test-source-3")

		result, err := suite.ctr.deleteModuleSource(context.TODO(), ms)
		require.NoError(suite.T(), err)
		assert.False(suite.T(), result.Requeue)
		assert.Empty(suite.T(), result.RequeueAfter)

		ms = suite.getModuleSource("test-source-3")
		assert.Len(suite.T(), ms.Finalizers, 0)
	})
}

func (suite *ControllerTestSuite) TestInvalidRegistry() {
	suite.T().Setenv("D8_IS_TESTS_ENVIRONMENT", "false")
	invalidSource := `
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

	suite.setupTestController(invalidSource)

	ms := suite.getModuleSource("test-source")

	_, err := suite.ctr.handleModuleSource(context.Background(), ms)
	require.NoError(suite.T(), err)

	ms = suite.getModuleSource(ms.Name)
	assert.Contains(suite.T(), ms.Status.Message, "credentials not found in the dockerCfg")
	assert.Len(suite.T(), ms.Status.AvailableModules, 0)
}

func (suite *ControllerTestSuite) getModuleSource(name string) *v1alpha1.ModuleSource {
	source := new(v1alpha1.ModuleSource)
	err := suite.kubeClient.Get(context.TODO(), types.NamespacedName{Name: name}, source)
	require.NoError(suite.T(), err)

	return source
}
