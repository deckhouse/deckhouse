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
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	addonmodules "github.com/flant/addon-operator/pkg/module_manager/models/modules"
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
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var (
	golden     bool
	mDelimiter *regexp.Regexp
)

func init() {
	flag.BoolVar(&golden, "golden", false, "generate golden files")
	mDelimiter = regexp.MustCompile("(?m)^---$")
}

func TestReleaseControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ReleaseControllerTestSuite))
}

type ReleaseControllerTestSuite struct {
	suite.Suite

	kubeClient client.Client
	ctr        *moduleReleaseReconciler

	testDataFileName string
	testMRName       string

	tmpDir string
}

func (suite *ReleaseControllerTestSuite) SetupSuite() {
	flag.Parse()
	suite.T().Setenv("D8_IS_TESTS_ENVIRONMENT", "true")
	suite.tmpDir = suite.T().TempDir()
	suite.T().Setenv("EXTERNAL_MODULES_DIR", suite.tmpDir)
	_ = os.MkdirAll(filepath.Join(suite.tmpDir, "modules"), 0777)
}

func (suite *ReleaseControllerTestSuite) TearDownSubTest() {
	goldenFile := filepath.Join("./testdata/releaseController", "golden", suite.testDataFileName)
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

func (suite *ReleaseControllerTestSuite) TestCreateReconcile() {
	entries, err := os.ReadDir("./testdata/releaseController")
	require.NoError(suite.T(), err)

	suite.Run("testdata cases", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{LayersStub: func() ([]v1.Layer, error) {
			return []v1.Layer{&utils.FakeLayer{}, &utils.FakeLayer{FilesContent: map[string]string{"openapi/values.yaml": "{}}"}}}, nil
		}}, nil)

		for _, en := range entries {
			if en.IsDir() {
				continue
			}

			suite.Run(en.Name(), func() {
				suite.setupReleaseController(string(suite.fetchTestFileData(en.Name())))
				mr := suite.getModuleRelease(suite.testMRName)
				_, err := suite.ctr.createOrUpdateReconcile(context.TODO(), mr)
				require.NoError(suite.T(), err)
			})
		}
	})
}

func (suite *ReleaseControllerTestSuite) setupReleaseController(yamlDoc string) {
	manifests := releaseutil.SplitManifests(yamlDoc)

	manifests["deckhouse-discovery"] = `
---
apiVersion: v1
data:
  bundle: RGVmYXVsdA==
  releaseChannel: VW5rbm93bg==
  updateSettings.json: eyJkaXNydXB0aW9uQXBwcm92YWxNb2RlIjoiQXV0byIsIm1vZGUiOiJNYW51YWwifQ==
kind: Secret
metadata:
  annotations:
    meta.helm.sh/release-name: deckhouse
    meta.helm.sh/release-namespace: d8-system
  creationTimestamp: "2024-01-18T14:29:03Z"
  labels:
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: deckhouse
  name: deckhouse-discovery
  namespace: d8-system
  resourceVersion: "134952280"
  uid: 7016bec6-b17c-4e90-bd35-16456d0df532
type: Opaque
`

	var initObjects = make([]client.Object, 0, len(manifests))

	for _, manifest := range manifests {
		obj := suite.assembleInitObject(manifest)
		initObjects = append(initObjects, obj)
	}

	sc := runtime.NewScheme()
	_ = v1alpha1.SchemeBuilder.AddToScheme(sc)
	_ = corev1.AddToScheme(sc)
	cl := fake.NewClientBuilder().WithScheme(sc).WithObjects(initObjects...).WithStatusSubresource(&v1alpha1.ModuleSource{}, &v1alpha1.ModuleRelease{}).Build()

	rec := &moduleReleaseReconciler{
		client:             cl,
		externalModulesDir: os.Getenv("EXTERNAL_MODULES_DIR"),
		dc:                 dependency.NewDependencyContainer(),
		logger:             log.New(),
		symlinksDir:        filepath.Join(os.Getenv("EXTERNAL_MODULES_DIR"), "modules"),
		moduleManager:      stubModulesManager{},
		delayTimer:         time.NewTimer(3 * time.Second),

		deckhouseEmbeddedPolicy: &v1alpha1.ModuleUpdatePolicySpec{
			Update: v1alpha1.ModuleUpdatePolicySpecUpdate{
				Mode: "Auto",
			},
			ReleaseChannel: "Stable",
		},
	}

	suite.ctr = rec
	suite.kubeClient = cl
}

func (suite *ReleaseControllerTestSuite) assembleInitObject(obj string) client.Object {
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

	case "ModuleRelease":
		var mr v1alpha1.ModuleRelease
		err = yaml.Unmarshal([]byte(obj), &mr)
		require.NoError(suite.T(), err)
		res = &mr
		suite.testMRName = mr.Name

	case "ModuleUpdatePolicy":
		var mup v1alpha1.ModuleUpdatePolicy
		err = yaml.Unmarshal([]byte(obj), &mup)
		require.NoError(suite.T(), err)
		res = &mup

	case "Secret":
		var sec corev1.Secret
		err = yaml.Unmarshal([]byte(obj), &sec)
		require.NoError(suite.T(), err)
		res = &sec
	}

	return res
}

func (suite *ReleaseControllerTestSuite) fetchTestFileData(filename string) []byte {
	dir := "./testdata/releaseController"
	data, err := os.ReadFile(filepath.Join(dir, filename))
	require.NoError(suite.T(), err)

	suite.testDataFileName = filename

	return data
}

func (suite *ReleaseControllerTestSuite) getModuleRelease(name string) *v1alpha1.ModuleRelease {
	var mr v1alpha1.ModuleRelease
	err := suite.kubeClient.Get(context.TODO(), types.NamespacedName{Name: name}, &mr)
	require.NoError(suite.T(), err)

	return &mr
}

func (suite *ReleaseControllerTestSuite) fetchResults() []byte {
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

type stubModulesManager struct{}

func (s stubModulesManager) DisableModuleHooks(_ string) {

}

func (s stubModulesManager) GetModule(_ string) *addonmodules.BasicModule {
	return nil
}

func (s stubModulesManager) GetEnabledModuleNames() []string {
	return nil
}

func (s stubModulesManager) RunModuleWithNewOpenAPISchema(_, _, _ string) error {
	return nil
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
