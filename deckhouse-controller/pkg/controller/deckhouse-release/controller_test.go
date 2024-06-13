/*
Copyright 2024 Flant JSC

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

package deckhouse_release

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/tidwall/sjson"

	"github.com/deckhouse/deckhouse/go_lib/hooks/update"

	"github.com/Masterminds/sprig/v3"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"helm.sh/helm/v3/pkg/releaseutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
)

var golden bool

func init() {
	flag.BoolVar(&golden, "golden", false, "generate golden files")
}

var embeddedMUP = &v1alpha1.ModuleUpdatePolicySpec{
	Update: v1alpha1.ModuleUpdatePolicySpecUpdate{
		Mode: "Auto",
	},
	ReleaseChannel: "Stable",
}

var initValues = `{
	"global": {
		"clusterIsBootstrapped": true,
        "modulesImages": {
			"registry": {
				"base": "my.registry.com/deckhouse"
			}
		}
	},
	"deckhouse": {
		"bundle": "Default",
		"internal": {},
		"releaseChannel": "Stable",
		"update": {
			"mode": "Auto",
			"windows": [{"from": "00:00", "to": "23:00"}]
		}
	}
}`

func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}

type ControllerTestSuite struct {
	suite.Suite

	kubeClient client.Client
	ctr        *deckhouseReleaseReconciler

	testDataFileName string
}

func (suite *ControllerTestSuite) SetupSuite() {
	flag.Parse()
	suite.T().Setenv("D8_IS_TESTS_ENVIRONMENT", "true")
}

func (suite *ControllerTestSuite) SetupSubTest() {
	dependency.TestDC.CRClient = cr.NewClientMock(suite.T())
	dependency.TestDC.HTTPClient.DoMock.
		Expect(&http.Request{}).
		Return(&http.Response{
			StatusCode: http.StatusOK,
		}, nil)
}

func (suite *ControllerTestSuite) TearDownSubTest() {
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
	ctx := context.Background()

	suite.Run("Update out of window", func() {
		mpu := embeddedMUP.DeepCopy()
		mpu.Update.Windows = update.Windows{{From: "8:00", To: "10:00"}}

		suite.setupController("update-out-of-window.yaml", initValues, mpu)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("No update windows configured", func() {
		values, err := sjson.Delete(initValues, "deckhouse.update.windows")
		require.NoError(suite.T(), err)
		values, err = sjson.SetRaw(values, "deckhouse.releaseChannel", `"Alpha"`)
		require.NoError(suite.T(), err)

		suite.setupController("no-update-windows-configured.yaml", values, embeddedMUP)
		_, err = suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Update out of day window", func() {
		mpu := embeddedMUP.DeepCopy()
		mpu.Update.Windows = update.Windows{{From: "8:00", To: "23:00", Days: []string{"Mon", "Tue"}}}

		suite.setupController("update-out-of-day-window.yaml", initValues, mpu)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Update in day window", func() {
		mpu := embeddedMUP.DeepCopy()
		mpu.Update.Windows = update.Windows{{From: "8:00", To: "23:00", Days: []string{"Fri", "Sun"}}}

		suite.setupController("update-in-day-window.yaml", initValues, mpu)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Shutdown and evicted pods", func() {
		suite.setupController("shutdown-and-evicted-pods.yaml", initValues, embeddedMUP)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Patch out of update window", func() {
		mpu := embeddedMUP.DeepCopy()
		mpu.Update.Windows = update.Windows{{From: "8:00", To: "8:01"}}

		suite.setupController("patch-out-of-update-window.yaml", initValues, mpu)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Deckhouse previous release is not ready", func() {
		mpu := embeddedMUP.DeepCopy()
		mpu.Update.Windows = update.Windows{{From: "00:00", To: "23:59"}}

		dependency.TestDC.HTTPClient.DoMock.
			Expect(&http.Request{}).
			Return(&http.Response{
				StatusCode: http.StatusInternalServerError,
			}, errors.New("some internal error"))

		suite.setupController("deckhouse-previous-release-is-not-ready.yaml", initValues, mpu)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Manual approval mode is set", func() {
		mpu := embeddedMUP.DeepCopy()
		mpu.Update.Mode = "Manual"

		suite.setupController("manual-approval-mode-is-set.yaml", initValues, mpu)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("After setting manual approve", func() {
		mpu := embeddedMUP.DeepCopy()
		mpu.Update.Mode = "Manual"

		suite.setupController("after-setting-manual-approve.yaml", initValues, mpu)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Auto deploy Patch release in Manual mode", func() {
		mpu := embeddedMUP.DeepCopy()
		mpu.Update.Mode = "Manual"

		suite.setupController("auto-deploy-patch-release-in-manual-mode.yaml", initValues, mpu)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Manual approval mode with canary process", func() {
		mpu := embeddedMUP.DeepCopy()
		mpu.Update.Mode = "Manual"

		suite.setupController("manual-approval-mode-with-canary-process.yaml", initValues, mpu)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("After setting manual approve with canary process", func() {
		mpu := embeddedMUP.DeepCopy()
		mpu.Update.Mode = "Manual"

		suite.setupController("after-setting-manual-approve-with-canary-process.yaml", initValues, mpu)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("DEV: No new deckhouse image", func() {
		dependency.TestDC.CRClient.DigestMock.Set(func(_ string) (s1 string, err error) {
			return "sha256:d57f01a88e54f863ff5365c989cb4e2654398fa274d46389e0af749090b862d1", nil
		})

		suite.setupController("dev-no-new-deckhouse-image.yaml", initValues, embeddedMUP)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("DEV: Have new deckhouse image", func() {
		dependency.TestDC.CRClient.DigestMock.Set(func(_ string) (s1 string, err error) {
			return "sha256:123456", nil
		})

		values, err := sjson.Delete(initValues, "deckhouse.releaseChannel")
		require.NoError(suite.T(), err)

		suite.setupController("dev-have-new-deckhouse-image.yaml", values, embeddedMUP)
		_, err = suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})
}

func (suite *ControllerTestSuite) setupController(filename, values string, mup *v1alpha1.ModuleUpdatePolicySpec) {
	yamlDoc := suite.fetchTestFileData(filename, values)
	manifests := releaseutil.SplitManifests(yamlDoc)

	var initObjects = make([]client.Object, 0, len(manifests))
	for _, manifest := range manifests {
		obj := suite.assembleInitObject(manifest)
		initObjects = append(initObjects, obj)
	}

	sc := runtime.NewScheme()
	_ = v1alpha1.SchemeBuilder.AddToScheme(sc)
	_ = appsv1.AddToScheme(sc)
	_ = corev1.AddToScheme(sc)
	cl := fake.NewClientBuilder().
		WithScheme(sc).
		WithObjects(initObjects...).
		WithStatusSubresource(&v1alpha1.DeckhouseRelease{}).
		Build()
	dc := dependency.NewDependencyContainer()
	rec := &deckhouseReleaseReconciler{
		client:       cl,
		dc:           dc,
		logger:       log.New(),
		updatePolicy: v1alpha1.NewModuleUpdatePolicySpecContainer(mup),
	}

	suite.ctr = rec
	suite.kubeClient = cl
}

func (suite *ControllerTestSuite) assembleInitObject(obj string) client.Object {
	var res client.Object
	var typ runtime.TypeMeta

	err := yaml.Unmarshal([]byte(obj), &typ)
	require.NoError(suite.T(), err)

	switch typ.Kind {
	case "Secret":
		res = unmarshal[corev1.Secret](obj, suite)
	case "Pod":
		res = unmarshal[corev1.Pod](obj, suite)
	case "Deployment":
		res = unmarshal[appsv1.Deployment](obj, suite)
	case "DeckhouseRelease":
		res = unmarshal[v1alpha1.DeckhouseRelease](obj, suite)

	default:
		require.Fail(suite.T(), "unknown Kind:"+typ.Kind)
	}

	return res
}

func (suite *ControllerTestSuite) fetchResults() []byte {
	result := bytes.NewBuffer(nil)

	var releaseList v1alpha1.DeckhouseReleaseList
	err := suite.kubeClient.List(context.TODO(), &releaseList)
	require.NoError(suite.T(), err)

	for _, item := range releaseList.Items {
		got, _ := yaml.Marshal(item)
		result.WriteString("---\n")
		result.Write(got)
	}

	var podsList corev1.PodList
	err = suite.kubeClient.List(context.TODO(), &podsList)
	require.NoError(suite.T(), err)

	for _, item := range podsList.Items {
		got, _ := yaml.Marshal(item)
		result.WriteString("---\n")
		result.Write(got)
	}

	var deploymentList appsv1.DeploymentList
	err = suite.kubeClient.List(context.TODO(), &deploymentList)
	require.NoError(suite.T(), err)

	for _, item := range deploymentList.Items {
		got, _ := yaml.Marshal(item)
		result.WriteString("---\n")
		result.Write(got)
	}

	return result.Bytes()
}

func (suite *ControllerTestSuite) fetchTestFileData(filename, valuesJSON string) string {
	dir := "./testdata"
	data, err := os.ReadFile(filepath.Join(dir, filename))
	require.NoError(suite.T(), err)

	tmpl, err := template.New("manifest").
		Funcs(sprig.TxtFuncMap()).
		Parse(string(data))
	require.NoError(suite.T(), err)

	var values any
	err = json.Unmarshal([]byte(valuesJSON), &values)
	require.NoError(suite.T(), err)

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]any{"Values": values})
	require.NoError(suite.T(), err)

	suite.testDataFileName = filename

	return buf.String()
}

func (suite *ControllerTestSuite) getDeckhouseRelease(name string) *v1alpha1.DeckhouseRelease {
	var release v1alpha1.DeckhouseRelease
	err := suite.kubeClient.Get(context.TODO(), types.NamespacedName{Name: name}, &release)
	require.NoError(suite.T(), err)

	return &release
}

func unmarshal[T any](manifest string, suite *ControllerTestSuite) *T {
	var obj T
	err := yaml.Unmarshal([]byte(manifest), &obj)
	require.NoError(suite.T(), err)
	return &obj
}
