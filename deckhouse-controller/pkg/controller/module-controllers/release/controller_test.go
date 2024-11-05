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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"helm.sh/helm/v3/pkg/releaseutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/models"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	d8env "github.com/deckhouse/deckhouse/go_lib/deckhouse-config/env"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	"github.com/deckhouse/deckhouse/pkg/log"
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
	suite.T().Setenv(d8env.DownloadedModulesDir, suite.tmpDir)
	_ = os.MkdirAll(filepath.Join(suite.tmpDir, "modules"), 0o777)
}

func (suite *ReleaseControllerTestSuite) TearDownSubTest() {
	if suite.T().Skipped() {
		return
	}

	goldenFile := filepath.Join("./testdata/releaseController", "golden", suite.testDataFileName)
	gotB := suite.fetchResults()

	if golden {
		err := os.WriteFile(goldenFile, gotB, 0o666)
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
	err := os.Setenv("TEST_EXTENDER_DECKHOUSE_VERSION", "v1.0.0")
	require.NoError(suite.T(), err)
	err = os.Setenv("TEST_EXTENDER_KUBERNETES_VERSION", "1.28.0")
	require.NoError(suite.T(), err)
	suite.Run("testdata cases", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{LayersStub: func() ([]v1.Layer, error) {
			return []v1.Layer{&utils.FakeLayer{}}, nil
		}}, nil)

		suite.Run("simple", func() {
			suite.setupReleaseController(string(suite.fetchTestFileData("simple.yaml")))
			mr := suite.getModuleRelease(suite.testMRName)
			_, err := suite.ctr.createOrUpdateReconcile(context.TODO(), mr)
			require.NoError(suite.T(), err)
		})

		suite.Run("with annotation", func() {
			suite.setupReleaseController(string(suite.fetchTestFileData("with-annotation.yaml")))
			mr := suite.getModuleRelease(suite.testMRName)
			_, err := suite.ctr.createOrUpdateReconcile(context.TODO(), mr)
			require.NoError(suite.T(), err)
		})

		suite.Run("deckhouse suitable version", func() {
			suite.setupReleaseController(string(suite.fetchTestFileData("dVersion-suitable.yaml")))
			mr := suite.getModuleRelease(suite.testMRName)
			_, err := suite.ctr.createOrUpdateReconcile(context.TODO(), mr)
			require.NoError(suite.T(), err)
		})

		suite.Run("deckhouse unsuitable version", func() {
			suite.setupReleaseController(string(suite.fetchTestFileData("dVersion-suitable.yaml")))
			mr := suite.getModuleRelease(suite.testMRName)
			_, err := suite.ctr.createOrUpdateReconcile(context.TODO(), mr)
			require.NoError(suite.T(), err)
		})

		suite.Run("kubernetes suitable version", func() {
			suite.setupReleaseController(string(suite.fetchTestFileData("kVersion-suitable.yaml")))
			mr := suite.getModuleRelease(suite.testMRName)
			_, err := suite.ctr.createOrUpdateReconcile(context.TODO(), mr)
			require.NoError(suite.T(), err)
		})

		suite.Run("kubernetes unsuitable version", func() {
			suite.setupReleaseController(string(suite.fetchTestFileData("kVersion-suitable.yaml")))
			mr := suite.getModuleRelease(suite.testMRName)
			_, err := suite.ctr.createOrUpdateReconcile(context.TODO(), mr)
			require.NoError(suite.T(), err)
		})

		suite.Run("deploy with outdated module releases", func() {
			dependency.TestDC.CRClient.ListTagsMock.Return([]string{}, nil)
			suite.setupReleaseController(string(suite.fetchTestFileData("clean-up-outdated-module-releases-when-deploy.yaml")))
			err := suite.updateModuleReleasesStatuses()
			require.NoError(suite.T(), err)
			mr := suite.getModuleRelease("echo-v0.4.54")
			_, err = suite.ctr.reconcilePendingRelease(context.TODO(), mr)
			require.NoError(suite.T(), err)
		})

		suite.Run("clean up for a deployed module release with outdated module releases", func() {
			dependency.TestDC.CRClient.ListTagsMock.Return([]string{}, nil)
			suite.setupReleaseController(string(suite.fetchTestFileData("clean-up-outdated-module-releases-for-deployed.yaml")))
			err := suite.updateModuleReleasesStatuses()
			require.NoError(suite.T(), err)
			mr := suite.getModuleRelease("echo-v0.4.54")
			_, err = suite.ctr.reconcileDeployedRelease(context.TODO(), mr)
			require.NoError(suite.T(), err)
		})

		suite.Run("loop until deploy: canary", func() {
			dc := dependency.NewMockedContainer()
			dc.CRClient.ImageMock.Return(&crfake.FakeImage{LayersStub: func() ([]v1.Layer, error) {
				return []v1.Layer{&utils.FakeLayer{}}, nil
			}}, nil)

			mup := &v1alpha1.ModuleUpdatePolicySpec{
				Update: v1alpha1.ModuleUpdatePolicySpecUpdate{
					Mode:    "Auto",
					Windows: update.Windows{{From: "00:00", To: "24:00", Days: []string{"tue"}}},
				},
				ReleaseChannel: "Stable",
			}

			testData := suite.fetchTestFileData("loop-canary.yaml")
			suite.setupReleaseController(string(testData), withModuleUpdatePolicy(mup), withDependencyContainer(dc))
			suite.loopUntilDeploy(dc, suite.testMRName)
		})

		suite.Run("install new module in manual mode with deckhouse release approval annotation", func() {
			suite.setupReleaseController(string(suite.fetchTestFileData("new-module-manual-mode.yaml")))
			mr := suite.getModuleRelease(suite.testMRName)
			ctx := context.Background()
			_, err := suite.ctr.createOrUpdateReconcile(ctx, mr)
			require.NoError(suite.T(), err)
		})
	})
}

func (suite *ReleaseControllerTestSuite) loopUntilDeploy(dc *dependency.MockedContainer, releaseName string) {
	const maxIterations = 3
	suite.T().Skip("TODO: requeue all releases after got deckhouse module config update")

	var (
		result = ctrl.Result{Requeue: true}
		err    error
		i      int
	)

	// Setting restartReason field in real code causes the process to reboot.
	// And at the next startup, Reconcile will be called for existing objects.
	// Therefore, this condition emulates the behavior in real code.
	for result.Requeue || result.RequeueAfter > 0 || suite.ctr.restartReason != "" {
		suite.ctr.restartReason = ""
		dc.GetFakeClock().Advance(result.RequeueAfter)

		dr := suite.getModuleRelease(releaseName)
		if dr.Status.Phase == v1alpha1.PhaseDeployed {
			return
		}

		result, err = suite.ctr.createOrUpdateReconcile(context.TODO(), dr)
		require.NoError(suite.T(), err)

		i++
		if i > maxIterations {
			suite.T().Fatal("Too many iterations")
		}
		suite.ctr.logger.Infof("Iteration %d result: %+v\n", i, result)
	}

	suite.T().Fatal("Loop was broken")
}

func (suite *ReleaseControllerTestSuite) updateModuleReleasesStatuses() error {
	var releases v1alpha1.ModuleReleaseList
	err := suite.kubeClient.List(context.TODO(), &releases)
	if err != nil {
		return err
	}

	caser := cases.Title(language.English)
	for _, release := range releases.Items {
		release.Status.Phase = caser.String(release.ObjectMeta.Labels["status"])
		err = suite.kubeClient.Status().Update(context.TODO(), &release)
		if err != nil {
			return err
		}
	}

	return nil
}

type reconcilerOption func(*moduleReleaseReconciler)

func withModuleUpdatePolicy(mup *v1alpha1.ModuleUpdatePolicySpec) reconcilerOption {
	return func(r *moduleReleaseReconciler) {
		r.deckhouseEmbeddedPolicy = helpers.NewModuleUpdatePolicySpecContainer(mup)
	}
}

func withDependencyContainer(dc dependency.Container) reconcilerOption {
	return func(r *moduleReleaseReconciler) {
		r.dc = dc
	}
}

func (suite *ReleaseControllerTestSuite) setupReleaseController(yamlDoc string, options ...reconcilerOption) {
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

	initObjects := make([]client.Object, 0, len(manifests))

	for _, manifest := range manifests {
		obj := suite.assembleInitObject(manifest)
		initObjects = append(initObjects, obj)
	}

	sc := runtime.NewScheme()
	_ = v1alpha1.SchemeBuilder.AddToScheme(sc)
	_ = corev1.AddToScheme(sc)
	cl := fake.NewClientBuilder().WithScheme(sc).WithObjects(initObjects...).WithStatusSubresource(&v1alpha1.ModuleSource{}, &v1alpha1.ModuleRelease{}).Build()

	rec := &moduleReleaseReconciler{
		client:               cl,
		downloadedModulesDir: d8env.GetDownloadedModulesDir(),
		dc:                   dependency.NewDependencyContainer(),
		logger:               log.NewNop(),
		symlinksDir:          filepath.Join(d8env.GetDownloadedModulesDir(), "modules"),
		moduleManager:        stubModulesManager{},
		delayTimer:           time.NewTimer(3 * time.Second),

		deckhouseEmbeddedPolicy: helpers.NewModuleUpdatePolicySpecContainer(&v1alpha1.ModuleUpdatePolicySpec{
			Update: v1alpha1.ModuleUpdatePolicySpecUpdate{
				Mode: "Auto",
			},
			ReleaseChannel: "Stable",
		}),
	}

	for _, option := range options {
		option(rec)
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

func (s stubModulesManager) GetModule(name string) *addonmodules.BasicModule {
	bm, _ := addonmodules.NewBasicModule(name, "", 900, nil, []byte{}, []byte{}, log.NewNop())
	return bm
}

func (s stubModulesManager) GetEnabledModuleNames() []string {
	return nil
}

func (s stubModulesManager) IsModuleEnabled(_ string) bool {
	return true
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

func Test_validateModule(t *testing.T) {
	check := func(name string, failed bool) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("./testdata", name)
			err := validateModule(
				models.DeckhouseModuleDefinition{
					Name:   name,
					Weight: 900,
					Path:   path,
				},
				nil,
				log.NewNop(),
			)

			if !failed {
				require.NoError(t, err, "%s: unexpected error: %v", name, err)
			}

			if failed {
				require.Error(t, err, "%s: got nil error", name)
			}
		})
	}

	check("module", false)
	check("module-not-valid", true)
	check("module-failed", true)
	check("module-values-failed", true)
	check("virtualization", false)
}
