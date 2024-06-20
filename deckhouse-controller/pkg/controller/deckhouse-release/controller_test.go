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
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/tidwall/sjson"
	"helm.sh/helm/v3/pkg/releaseutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	"github.com/deckhouse/deckhouse/go_lib/updater"
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
		mup := embeddedMUP.DeepCopy()
		mup.Update.Windows = update.Windows{{From: "8:00", To: "10:00"}}

		suite.setupController("update-out-of-window.yaml", initValues, mup)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("No update windows configured", func() {
		values, err := sjson.SetRaw(initValues, "deckhouse.releaseChannel", `"Alpha"`)
		require.NoError(suite.T(), err)

		suite.setupController("no-update-windows-configured.yaml", values, embeddedMUP)
		_, err = suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Update out of day window", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Windows = update.Windows{{From: "8:00", To: "23:00", Days: []string{"Mon", "Tue"}}}

		suite.setupController("update-out-of-day-window.yaml", initValues, mup)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Update in day window", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Windows = update.Windows{{From: "8:00", To: "23:00", Days: []string{"Fri", "Sun"}}}

		suite.setupController("update-in-day-window.yaml", initValues, mup)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Shutdown and evicted pods", func() {
		suite.setupController("shutdown-and-evicted-pods.yaml", initValues, embeddedMUP)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Patch out of update window", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Windows = update.Windows{{From: "8:00", To: "8:01"}}

		suite.setupController("patch-out-of-update-window.yaml", initValues, mup)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Deckhouse previous release is not ready", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Windows = update.Windows{{From: "00:00", To: "23:59"}}

		dependency.TestDC.HTTPClient.DoMock.
			Expect(&http.Request{}).
			Return(&http.Response{
				StatusCode: http.StatusInternalServerError,
			}, errors.New("some internal error"))

		suite.setupController("deckhouse-previous-release-is-not-ready.yaml", initValues, mup)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Manual approval mode is set", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Mode = "Manual"

		suite.setupController("manual-approval-mode-is-set.yaml", initValues, mup)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("After setting manual approve", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Mode = "Manual"

		suite.setupController("after-setting-manual-approve.yaml", initValues, mup)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Auto deploy Patch release in Manual mode", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Mode = "Manual"

		suite.setupController("auto-deploy-patch-release-in-manual-mode.yaml", initValues, mup)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Manual approval mode with canary process", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Mode = "Manual"

		suite.setupController("manual-approval-mode-with-canary-process.yaml", initValues, mup)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("After setting manual approve with canary process", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Mode = "Manual"

		suite.setupController("after-setting-manual-approve-with-canary-process.yaml", initValues, mup)
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

	suite.Run("Manual mode", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Mode = "Manual"

		suite.setupController("manual-mode.yaml", initValues, mup)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Second run of the hook in a Manual mode should not change state", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Mode = "Manual"

		suite.setupController("second-run-of-the-hook-in-a-manual-mode-should-not-change-state.yaml", initValues, mup)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)

		_, err = suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Single First Release", func() {
		suite.setupController("single-first-release.yaml", initValues, embeddedMUP)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("First Release with manual mode", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Mode = "Manual"

		values, err := sjson.Delete(initValues, "global.clusterIsBootstrapped")
		require.NoError(suite.T(), err)

		suite.setupController("first-release-with-manual-mode.yaml", values, mup)
		_, err = suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Few patch releases", func() {
		suite.setupController("few-patch-releases.yaml", initValues, embeddedMUP)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Pending Manual release on cluster bootstrap", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Mode = "Manual"

		values, err := sjson.Delete(initValues, "global.clusterIsBootstrapped")
		require.NoError(suite.T(), err)

		suite.setupController("pending-manual-release-on-cluster-bootstrap.yaml", values, mup)
		_, err = suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Forced release", func() {
		suite.setupController("forced-release.yaml", initValues, embeddedMUP)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Postponed release", func() {
		suite.setupController("postponed-release.yaml", initValues, embeddedMUP)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Release applyAfter time passed", func() {
		suite.setupController("release-apply-after-time-passed.yaml", initValues, embeddedMUP)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Suspend release", func() {
		suite.setupController("suspend-release.yaml", initValues, embeddedMUP)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Release with not met requirements", func() {
		requirements.RegisterCheck("k8s", func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
			v, _ := getter.Get("global.discovery.kubernetesVersion")
			if v != requirementValue {
				return false, errors.New("min k8s version failed")
			}

			return true, nil
		})
		requirements.SaveValue("global.discovery.kubernetesVersion", "1.16.0")

		suite.setupController("release-with-not-met-requirements.yaml", initValues, embeddedMUP)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Release requirements passed", func() {
		requirements.RegisterCheck("k8s", func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
			v, _ := getter.Get("global.discovery.kubernetesVersion")
			if v != requirementValue {
				return false, errors.New("min k8s version failed")
			}

			return true, nil
		})
		requirements.SaveValue("global.discovery.kubernetesVersion", "1.19.0")

		suite.setupController("release-requirements-passed.yaml", initValues, embeddedMUP)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Disruption release", func() {
		values, err := sjson.SetRaw(initValues, "deckhouse.update.disruptionApprovalMode", `"Manual"`)
		require.NoError(suite.T(), err)

		var df requirements.DisruptionFunc = func(_ requirements.ValueGetter) (bool, string) {
			return true, "some test reason"
		}
		requirements.RegisterDisruption("testme", df)

		suite.setupController("disruption-release.yaml", values, embeddedMUP)
		_, err = suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Disruption release approved", func() {
		values, err := sjson.SetRaw(initValues, "deckhouse.update.disruptionApprovalMode", `"Manual"`)
		require.NoError(suite.T(), err)

		suite.setupController("disruption-release-approved.yaml", values, embeddedMUP)
		_, err = suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Notification: release with notification settings", func() {
		var httpBody string
		svr := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			data, _ := io.ReadAll(r.Body)
			httpBody = string(data)
		}))
		defer svr.Close()

		values, err := sjson.SetRaw(initValues, "deckhouse.update.notification.webhook", strconv.Quote(svr.URL))
		require.NoError(suite.T(), err)

		values, err = sjson.SetRaw(values, "deckhouse.update.notification.minimalNotificationTime", `"1h"`)
		require.NoError(suite.T(), err)

		suite.setupController("release-with-notification-settings.yaml", values, embeddedMUP)
		_, err = suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)

		require.Contains(suite.T(), httpBody, "New Deckhouse Release 1.26 is available. Release will be applied at: Friday, 01-Jan-21 14:30:00 UTC")
		require.Contains(suite.T(), httpBody, `"version":"1.26"`)
	})

	suite.Run("Notification: after met conditions", func() {
		suite.setupController("notification-after-met-conditions.yaml", initValues, embeddedMUP)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Update: Release is deployed", func() {
		suite.setupController("update-release-is-deployed.yaml", initValues, embeddedMUP)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
		//TODO default annotations in release

		//TODO: check
		//Expect(cm.Field("data.isUpdating").Bool()).To(BeFalse())
		//Expect(cm.Field("data.notified").Bool()).To(BeFalse())
	})

	suite.Run("Notification: release applyAfter time is after notification period", func() {
		var httpBody string
		svr := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			data, _ := io.ReadAll(r.Body)
			httpBody = string(data)
		}))
		defer svr.Close()

		values, err := sjson.SetRaw(initValues, "deckhouse.update.notification.webhook", strconv.Quote(svr.URL))
		require.NoError(suite.T(), err)

		values, err = sjson.SetRaw(values, "deckhouse.update.notification.minimalNotificationTime", `"4h10m"`)
		require.NoError(suite.T(), err)

		suite.setupController("notification-release-apply-after-time-is-after-notification-period.yaml", values, embeddedMUP)
		_, err = suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)

		require.Contains(suite.T(), httpBody, "New Deckhouse Release 1.36 is available. Release will be applied at: Monday, 11-Nov-22 23:23:23 UTC")
		require.Contains(suite.T(), httpBody, `"version":"1.36"`)
	})

	suite.Run("Notification: basic auth", func() {
		var (
			username string
			password string
		)
		svr := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			username, password, _ = r.BasicAuth()
		}))
		defer svr.Close()

		values, err := sjson.SetRaw(initValues, "deckhouse.update.notification.webhook", strconv.Quote(svr.URL))
		require.NoError(suite.T(), err)

		values, err = sjson.Set(values, "deckhouse.update.notification.auth", updater.Auth{Basic: &updater.BasicAuth{Username: "user", Password: "pass"}})
		require.NoError(suite.T(), err)

		suite.setupController("notification-basic-auth.yaml", values, embeddedMUP)
		_, err = suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)

		require.Equal(suite.T(), username, "user")
		require.Equal(suite.T(), password, "pass")
	})

	suite.Run("Notification: bearer token auth", func() {
		var (
			headerValue string
		)
		svr := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			headerValue = r.Header.Get("Authorization")
		}))
		defer svr.Close()

		values, err := sjson.SetRaw(initValues, "deckhouse.update.notification.webhook", strconv.Quote(svr.URL))
		require.NoError(suite.T(), err)

		values, err = sjson.Set(values, "deckhouse.update.notification.auth", updater.Auth{Token: pointer.String("the_token")})
		require.NoError(suite.T(), err)

		suite.setupController("notification-bearer-token-auth.yaml", values, embeddedMUP)
		_, err = suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)

		require.Equal(suite.T(), headerValue, "Bearer the_token")
	})

	suite.Run("Update minimal notification time without configuring notification webhook", func() {
		svr := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
		defer svr.Close()

		values, err := sjson.SetRaw(initValues, "deckhouse.update.notification.webhook", strconv.Quote(svr.URL))
		require.NoError(suite.T(), err)

		values, err = sjson.Set(values, "deckhouse.update.notification.minimalNotificationTime", []byte("2h"))
		require.NoError(suite.T(), err)

		suite.setupController("update-minimal-notification-time-without-configuring-notification-webhook.yaml", values, embeddedMUP)
		_, err = suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Release with apply-now annotation out of window", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Windows = update.Windows{{From: "8:00", To: "10:00"}}

		suite.setupController("release-with-apply-now-annotation-out-of-window.yaml", initValues, mup)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("ApplyNow: Deckhouse previous release is not ready", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Windows = update.Windows{{From: "8:00", To: "23:59"}}

		dependency.TestDC.HTTPClient.DoMock.
			Expect(&http.Request{}).
			Return(&http.Response{
				StatusCode: http.StatusInternalServerError,
			}, errors.New("some internal error"))

		suite.setupController("apply-now-deckhouse-previous-release-is-not-ready.yaml", initValues, mup)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("ApplyNow: Manual approval mode is set", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Mode = "Manual"

		dependency.TestDC.HTTPClient.DoMock.
			Expect(&http.Request{}).
			Return(&http.Response{
				StatusCode: http.StatusOK,
			}, nil)

		suite.setupController("apply-now-manual-approval-mode-is-set.yaml", initValues, mup)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Applied now postponed release", func() {
		suite.setupController("applied-now-postponed-release.yaml", initValues, embeddedMUP)
		_, err := suite.ctr.createOrUpdateReconcile(ctx)
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
		cl,
		dc,
		log.New(),
		stubModulesManager{},
		v1alpha1.NewModuleUpdatePolicySpecContainer(mup),
		new(container[string]),
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
	case "ConfigMap":
		res = unmarshal[corev1.ConfigMap](obj, suite)

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

	var cmList corev1.ConfigMapList
	err = suite.kubeClient.List(context.TODO(), &cmList)
	require.NoError(suite.T(), err)

	for _, item := range cmList.Items {
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

	deckhouseDiscovery := `---
apiVersion: v1
kind: Secret
metadata:
  name: deckhouse-discovery
  namespace: d8-system
type: Opaque
data:
  bundle: {{ b64enc .Values.deckhouse.bundle }}
  releaseChannel: {{ .Values.deckhouse.releaseChannel | default "Unknown" | b64enc }}
{{- if .Values.deckhouse.update }}
  updateSettings.json: {{ .Values.deckhouse.update | toJson | b64enc }}
{{- end }}
  clusterIsBootstrapped: {{ .Values.global.clusterIsBootstrapped | quote | b64enc }}
  imagesRegistry: {{ b64enc .Values.global.modulesImages.registry.base }}
{{- if $.Values.global.discovery.clusterUUID }}
  clusterUUID: {{ $.Values.global.discovery.clusterUUID | b64enc }}
{{- end }}
`

	tmpl, err := template.New("manifest").
		Funcs(sprig.TxtFuncMap()).
		Parse(string(data) + deckhouseDiscovery)
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

type stubModulesManager struct{}

func (s stubModulesManager) GetEnabledModuleNames() []string {
	return []string{"cert-manager", "prometheus"}
}

func (s stubModulesManager) AreModulesInited() bool {
	return true
}
