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
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/tidwall/sjson"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	updater "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/releaseupdater"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	"github.com/deckhouse/deckhouse/go_lib/libapi"
)

var (
	golden     bool
	mDelimiter *regexp.Regexp
)

func init() {
	flag.BoolVar(&golden, "golden", false, "generate golden files")
	mDelimiter = regexp.MustCompile("(?m)^---$")
}

var embeddedMUP = &v1alpha2.ModuleUpdatePolicySpec{
	Update: v1alpha2.ModuleUpdatePolicySpecUpdate{
		Mode: v1alpha2.UpdateModeAuto.String(),
	},
	ReleaseChannel: "Stable",
}

var initValues = `{
	"global": {
		"clusterIsBootstrapped": true,
		"clusterConfiguration": {
			"kubernetesVersion": "1.29"
		},
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
	backupTZ         *time.Location
}

func (suite *ControllerTestSuite) SetupSuite() {
	flag.Parse()
	suite.T().Setenv("D8_IS_TESTS_ENVIRONMENT", "true")
	suite.backupTZ = time.Local
	time.Local = dependency.TestTimeZone
}

func (suite *ControllerTestSuite) SetupSubTest() {
	dependency.TestDC.CRClient = cr.NewClientMock(suite.T())
	dependency.TestDC.HTTPClient.DoMock.
		Expect(&http.Request{}).
		Return(&http.Response{
			StatusCode: http.StatusOK,
		}, nil)
}

func (suite *ControllerTestSuite) TearDownSuite() {
	time.Local = suite.backupTZ
}

func (suite *ControllerTestSuite) TearDownSubTest() {
	if suite.T().Skipped() {
		return
	}

	goldenFile := filepath.Join("./testdata", "golden", suite.testDataFileName)
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

func (suite *ControllerTestSuite) setupController(
	filename string,
	initValues string,
	mup *v1alpha2.ModuleUpdatePolicySpec,
	options ...reconcilerOption,
) {
	suite.testDataFileName = filename
	suite.ctr, suite.kubeClient = setupFakeController(suite.T(), filename, initValues, mup, options...)
}

func (suite *ControllerTestSuite) setupControllerSettings(
	filename string,
	initValues string,
	ds *helpers.DeckhouseSettings,
) {
	suite.testDataFileName = filename
	suite.ctr, suite.kubeClient = setupControllerSettings(suite.T(), filename, initValues, ds)
}

func (suite *ControllerTestSuite) TestCreateReconcile() {
	ctx := context.Background()

	suite.Run("Set initial state", func() {
		suite.setupController("set-initial-state.yaml", initValues, embeddedMUP)
		dr := suite.getDeckhouseRelease("v1.26.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Update out of window", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Windows = update.Windows{{From: "8:00", To: "10:00"}}

		suite.setupController("update-out-of-window.yaml", initValues, mup)
		dr := suite.getDeckhouseRelease("v1.26.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("No update windows configured", func() {
		values, err := sjson.SetRaw(initValues, "deckhouse.releaseChannel", `"Alpha"`)
		require.NoError(suite.T(), err)

		suite.setupController("no-update-windows-configured.yaml", values, embeddedMUP)
		dr := suite.getDeckhouseRelease("v1.26.0")
		_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Update out of day window", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Windows = update.Windows{{From: "8:00", To: "23:00", Days: []string{"Mon", "Tue"}}}

		suite.setupController("update-out-of-day-window.yaml", initValues, mup)
		dr := suite.getDeckhouseRelease("v1.26.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Loop until deploy: canary", func() {
		dc := newDependencyContainer(suite.T())

		mup := embeddedMUP.DeepCopy()
		mup.Update.Windows = update.Windows{{From: "8:00", To: "23:00", Days: []string{"Mon", "Tue"}}}

		suite.setupController("loop-until-deploy-canary.yaml", initValues, mup, withDependencyContainer(dc))
		suite.loopUntilDeploy(dc, "v1.26.0")
	})

	suite.Run("Update in day window", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Windows = update.Windows{{From: "8:00", To: "23:00", Days: []string{"Fri", "Sun", "Thu"}}}

		suite.setupController("update-in-day-window.yaml", initValues, mup)
		dr := suite.getDeckhouseRelease("v1.26.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Shutdown and evicted pods", func() {
		suite.setupController("shutdown-and-evicted-pods.yaml", initValues, embeddedMUP)
		dr := suite.getDeckhouseRelease("v1.26.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Patch awaits update window", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Windows = update.Windows{{From: "8:00", To: "8:01"}}

		suite.setupController("patch-awaits-update-window.yaml", initValues, mup)
		dr := suite.getDeckhouseRelease("v1.26.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
		dr = suite.getDeckhouseRelease("v1.25.1")
		_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
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
		dr := suite.getDeckhouseRelease("v1.26.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Manual approval mode is set", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Mode = v1alpha2.UpdateModeManual.String()

		suite.setupController("manual-approval-mode-is-set.yaml", initValues, mup)
		dr := suite.getDeckhouseRelease("v1.26.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("After setting manual approve", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Mode = v1alpha2.UpdateModeManual.String()

		suite.setupController("after-setting-manual-approve.yaml", initValues, mup)
		dr := suite.getDeckhouseRelease("v1.26.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Auto deploy Patch release in Manual mode", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Mode = v1alpha2.UpdateModeManual.String()

		suite.setupController("auto-deploy-patch-release-in-manual-mode.yaml", initValues, mup)
		dr := suite.getDeckhouseRelease("v1.26.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
		dr = suite.getDeckhouseRelease("v1.25.1")
		_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Manual approval mode with canary process", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Mode = v1alpha2.UpdateModeManual.String()
		suite.setupController("manual-approval-mode-with-canary-process.yaml", initValues, mup)
		dr := suite.getDeckhouseRelease("v1.36.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("After setting manual approve with canary process", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Mode = v1alpha2.UpdateModeManual.String()

		suite.setupController("after-setting-manual-approve-with-canary-process.yaml", initValues, mup)
		dr := suite.getDeckhouseRelease("v1.36.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Manual mode", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Mode = v1alpha2.UpdateModeManual.String()

		suite.setupController("manual-mode.yaml", initValues, mup)
		dr := suite.getDeckhouseRelease("v1.27.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Second run of the hook in a Manual mode should not change state", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Mode = v1alpha2.UpdateModeManual.String()

		suite.setupController("second-run-of-the-hook-in-a-manual-mode-should-not-change-state.yaml", initValues, mup)
		dr := suite.getDeckhouseRelease("v1.27.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)

		dr = suite.getDeckhouseRelease("v1.27.0")
		_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Single First Release", func() {
		suite.setupController("single-first-release.yaml", initValues, embeddedMUP)
		dr := suite.getDeckhouseRelease("v1.25.1")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Few patch releases", func() {
		dependency.TestDC.HTTPClient.DoMock.
			Expect(&http.Request{}).
			Return(&http.Response{
				StatusCode: http.StatusInternalServerError,
			}, errors.New("some internal error"))

		suite.setupController("few-patch-releases.yaml", initValues, embeddedMUP)
		dr := suite.getDeckhouseRelease("v1.31.1")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
		dr = suite.getDeckhouseRelease("v1.31.2")
		_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
		dr = suite.getDeckhouseRelease("v1.31.3")
		_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
		dr = suite.getDeckhouseRelease("v1.32.0")
		_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("few minor releases", func() {
		dependency.TestDC.HTTPClient.DoMock.
			Expect(&http.Request{}).
			Return(&http.Response{
				StatusCode: http.StatusOK,
			}, nil)

		suite.setupController("few-minor-releases.yaml", initValues, embeddedMUP)
		dr := suite.getDeckhouseRelease("v1.31.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
		dr = suite.getDeckhouseRelease("v1.32.0")
		_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)

		dependency.TestDC.HTTPClient.DoMock.
			Expect(&http.Request{}).
			Return(&http.Response{
				StatusCode: http.StatusInternalServerError,
			}, errors.New("some internal error"))

		dr = suite.getDeckhouseRelease("v1.33.0")
		_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
		dr = suite.getDeckhouseRelease("v1.34.0")
		_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
		dr = suite.getDeckhouseRelease("v1.35.0")
		_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("few minor releases with version more than one from deployed", func() {
		dependency.TestDC.HTTPClient.DoMock.
			Expect(&http.Request{}).
			Return(&http.Response{
				StatusCode: http.StatusOK,
			}, nil)

		suite.setupController("few-minor-releases-version-more-than-one.yaml", initValues, embeddedMUP)
		dr := suite.getDeckhouseRelease("v1.33.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
		dr = suite.getDeckhouseRelease("v1.34.0")
		_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)

		dependency.TestDC.HTTPClient.DoMock.
			Expect(&http.Request{}).
			Return(&http.Response{
				StatusCode: http.StatusInternalServerError,
			}, errors.New("some internal error"))

		dr = suite.getDeckhouseRelease("v1.33.0")
		_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
		dr = suite.getDeckhouseRelease("v1.34.0")
		_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
		dr = suite.getDeckhouseRelease("v1.35.0")
		_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("minor release and patch release", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Mode = v1alpha2.UpdateModeAuto.String()
		mup.Update.Windows = update.Windows{{From: "8:00", To: "10:00"}}

		dependency.TestDC.HTTPClient.DoMock.
			Expect(&http.Request{}).
			Return(&http.Response{
				StatusCode: http.StatusOK,
			}, nil)

		suite.setupController("minor-release-and-patch-release.yaml", initValues, mup)
		dr := suite.getDeckhouseRelease("v1.31.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
		dr = suite.getDeckhouseRelease("v1.32.0")
		_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
		dr = suite.getDeckhouseRelease("v1.32.1")
		_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("forced through few minor releases", func() {
		dependency.TestDC.HTTPClient.DoMock.
			Expect(&http.Request{}).
			Return(&http.Response{
				StatusCode: http.StatusInternalServerError,
			}, errors.New("some internal error"))

		suite.setupController("forced-few-minor-releases.yaml", initValues, embeddedMUP)
		dr := suite.getDeckhouseRelease("v1.31.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
		dr = suite.getDeckhouseRelease("v1.32.0")
		_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
		dr = suite.getDeckhouseRelease("v1.33.0")
		_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
		dr = suite.getDeckhouseRelease("v1.34.0")
		_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
		dr = suite.getDeckhouseRelease("v1.35.0")
		_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Pending Manual release on cluster bootstrap", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Mode = v1alpha2.UpdateModeManual.String()

		values, err := sjson.Delete(initValues, "global.clusterIsBootstrapped")
		require.NoError(suite.T(), err)

		suite.setupController("pending-manual-release-on-cluster-bootstrap.yaml", values, mup)
		dr := suite.getDeckhouseRelease("v1.46.0")
		_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Forced release", func() {
		suite.setupController("forced-release.yaml", initValues, embeddedMUP)
		dr := suite.getDeckhouseRelease("v1.31.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
		dr = suite.getDeckhouseRelease("v1.31.1")
		_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Postponed release", func() {
		suite.setupController("postponed-release.yaml", initValues, embeddedMUP)
		dr := suite.getDeckhouseRelease("v1.25.1")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Release applyAfter time passed", func() {
		suite.setupController("release-apply-after-time-passed.yaml", initValues, embeddedMUP)
		dr := suite.getDeckhouseRelease("v1.25.1")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Suspend release", func() {
		suite.setupController("suspend-release.yaml", initValues, embeddedMUP)
		dr := suite.getDeckhouseRelease("v1.25.1")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.Error(suite.T(), err)
		require.Contains(suite.T(), err.Error(), "release phase is not pending")
		dr = suite.getDeckhouseRelease("v1.25.2")
		_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
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
		dr := suite.getDeckhouseRelease("v1.30.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
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
		dr := suite.getDeckhouseRelease("v1.30.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Disruption release", func() {
		var df requirements.DisruptionFunc = func(_ requirements.ValueGetter) (bool, string) {
			return true, "some test reason"
		}
		requirements.RegisterDisruption("testme", df)

		ds := &helpers.DeckhouseSettings{
			ReleaseChannel: embeddedMUP.ReleaseChannel,
		}
		ds.Update.Mode = embeddedMUP.Update.Mode
		ds.Update.Windows = embeddedMUP.Update.Windows
		ds.Update.DisruptionApprovalMode = v1alpha2.UpdateModeManual.String()

		suite.setupControllerSettings("disruption-release.yaml", initValues, ds)
		dr := suite.getDeckhouseRelease("v1.36.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Disruption release approved", func() {
		ds := &helpers.DeckhouseSettings{
			ReleaseChannel: embeddedMUP.ReleaseChannel,
		}
		ds.Update.Mode = embeddedMUP.Update.Mode
		ds.Update.Windows = embeddedMUP.Update.Windows
		ds.Update.DisruptionApprovalMode = v1alpha2.UpdateModeManual.String()

		suite.setupControllerSettings("disruption-release-approved.yaml", initValues, ds)
		dr := suite.getDeckhouseRelease("v1.36.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Notification: release with notification settings", func() {
		var httpBody string
		svr := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			data, _ := io.ReadAll(r.Body)
			httpBody = string(data)
		}))
		defer svr.Close()

		ds := &helpers.DeckhouseSettings{
			ReleaseChannel: embeddedMUP.ReleaseChannel,
		}
		ds.Update.Mode = embeddedMUP.Update.Mode
		ds.Update.Windows = embeddedMUP.Update.Windows
		ds.Update.NotificationConfig.WebhookURL = svr.URL
		ds.Update.NotificationConfig.MinimalNotificationTime = libapi.Duration{Duration: time.Hour}

		suite.setupControllerSettings("release-with-notification-settings.yaml", initValues, ds)
		dr := suite.getDeckhouseRelease("v1.26.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)

		require.Contains(suite.T(), httpBody, "New Deckhouse Release 1.26.0 is available. Release will be applied at: Thursday, 17-Oct-19 16:33:00 UTC")
		require.Contains(suite.T(), httpBody, `"version":"1.26.0"`)
		require.Contains(suite.T(), httpBody, `"subject":"Deckhouse"`)
	})

	suite.Run("Notification: after met conditions", func() {
		suite.setupController("notification-after-met-conditions.yaml", initValues, embeddedMUP)
		dr := suite.getDeckhouseRelease("v1.26.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Update: Release is deployed", func() {
		suite.setupController("update-release-is-deployed.yaml", initValues, embeddedMUP)
		dr := suite.getDeckhouseRelease("v1.26.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Notification: release applyAfter time is after notification period", func() {
		var httpBody string
		svr := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			data, _ := io.ReadAll(r.Body)
			httpBody = string(data)
		}))
		defer svr.Close()

		ds := &helpers.DeckhouseSettings{
			ReleaseChannel: embeddedMUP.ReleaseChannel,
		}
		ds.Update.Mode = embeddedMUP.Update.Mode
		ds.Update.Windows = embeddedMUP.Update.Windows
		ds.Update.NotificationConfig.WebhookURL = svr.URL
		ds.Update.NotificationConfig.MinimalNotificationTime = libapi.Duration{Duration: 4*time.Hour + 10*time.Minute}

		suite.setupControllerSettings("notification-release-apply-after-time-is-after-notification-period.yaml", initValues, ds)
		dr := suite.getDeckhouseRelease("v1.36.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)

		require.Contains(suite.T(), httpBody, "New Deckhouse Release 1.36.0 is available. Release will be applied at: Monday, 11-Nov-22 23:23:23 UTC")
		require.Contains(suite.T(), httpBody, `"version":"1.36.0"`)
		require.Contains(suite.T(), httpBody, `"subject":"Deckhouse"`)
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

		ds := &helpers.DeckhouseSettings{
			ReleaseChannel: embeddedMUP.ReleaseChannel,
		}
		ds.Update.Mode = embeddedMUP.Update.Mode
		ds.Update.Windows = embeddedMUP.Update.Windows
		ds.Update.NotificationConfig.WebhookURL = svr.URL
		ds.Update.NotificationConfig.Auth = &updater.Auth{Basic: &updater.BasicAuth{Username: "user", Password: "pass"}}

		suite.setupControllerSettings("notification-basic-auth.yaml", initValues, ds)
		dr := suite.getDeckhouseRelease("v1.36.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)

		require.Equal(suite.T(), username, "user")
		require.Equal(suite.T(), password, "pass")
	})

	suite.Run("Notification: bearer token auth", func() {
		var headerValue string
		svr := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			headerValue = r.Header.Get("Authorization")
		}))
		defer svr.Close()

		ds := &helpers.DeckhouseSettings{
			ReleaseChannel: embeddedMUP.ReleaseChannel,
		}
		ds.Update.Mode = embeddedMUP.Update.Mode
		ds.Update.Windows = embeddedMUP.Update.Windows
		ds.Update.NotificationConfig.WebhookURL = svr.URL
		ds.Update.NotificationConfig.Auth = &updater.Auth{Token: ptr.To("the_token")}

		suite.setupControllerSettings("notification-bearer-token-auth.yaml", initValues, ds)
		dr := suite.getDeckhouseRelease("v1.36.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)

		require.Equal(suite.T(), headerValue, "Bearer the_token")
	})

	suite.Run("Update minimal notification time without configuring notification webhook", func() {
		svr := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
		defer svr.Close()

		ds := &helpers.DeckhouseSettings{
			ReleaseChannel: embeddedMUP.ReleaseChannel,
		}
		ds.Update.Mode = embeddedMUP.Update.Mode
		ds.Update.Windows = embeddedMUP.Update.Windows
		ds.Update.NotificationConfig.WebhookURL = svr.URL
		ds.Update.NotificationConfig.MinimalNotificationTime = libapi.Duration{Duration: 2 * time.Hour}

		suite.setupControllerSettings("update-minimal-notification-time-without-configuring-notification-webhook.yaml", initValues, ds)
		dr := suite.getDeckhouseRelease("v1.26.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Patch release notification", func() {
		var httpBody string
		svr := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			data, _ := io.ReadAll(r.Body)
			httpBody = string(data)
		}))
		defer svr.Close()

		ds := &helpers.DeckhouseSettings{
			ReleaseChannel: embeddedMUP.ReleaseChannel,
		}
		ds.Update.Mode = embeddedMUP.Update.Mode
		ds.Update.Windows = embeddedMUP.Update.Windows
		ds.Update.NotificationConfig.WebhookURL = svr.URL
		ds.Update.NotificationConfig.MinimalNotificationTime = libapi.Duration{Duration: 2 * time.Hour}
		ds.Update.NotificationConfig.ReleaseType = updater.ReleaseTypeAll

		suite.setupControllerSettings("patch-release-notification.yaml", initValues, ds)
		dr := suite.getDeckhouseRelease("v1.26.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)

		require.Contains(suite.T(), httpBody, "New Deckhouse Release 1.25.1 is available. Release will be applied at: Thursday, 17-Oct-19 17:33:00 UTC")
		require.Contains(suite.T(), httpBody, `"version":"1.25.1"`)
		require.Contains(suite.T(), httpBody, `"subject":"Deckhouse"`)
	})

	suite.Run("apply now annotation", func() {
		suite.Run("Minor update out of window", func() {
			mup := embeddedMUP.DeepCopy()
			mup.Update.Mode = v1alpha2.UpdateModeAuto.String()
			mup.Update.Windows = update.Windows{{From: "8:00", To: "10:00"}}

			suite.setupController("release-with-apply-now-annotation-out-of-window.yaml", initValues, mup)
			dr := suite.getDeckhouseRelease("v1.26.0")
			_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
			require.NoError(suite.T(), err)
		})

		suite.Run("Minor update respect requirements", func() {
			mup := embeddedMUP.DeepCopy()
			mup.Update.Windows = update.Windows{{From: "8:00", To: "10:00"}}

			requirements.RegisterCheck("k8s", func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
				v, _ := getter.Get("global.discovery.kubernetesVersion")
				if v != requirementValue {
					return false, errors.New("min k8s version failed")
				}

				return true, nil
			})
			requirements.SaveValue("global.discovery.kubernetesVersion", "1.16.0")

			suite.setupController("release-with-apply-now-annotation-requirements.yaml", initValues, mup)
			dr := suite.getDeckhouseRelease("v1.26.0")
			_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
			require.NoError(suite.T(), err)
		})

		suite.Run("Minor update respect disruption", func() {
			mup := embeddedMUP.DeepCopy()
			mup.Update.Windows = update.Windows{{From: "8:00", To: "10:00"}}

			var df requirements.DisruptionFunc = func(_ requirements.ValueGetter) (bool, string) {
				return true, "some test reason"
			}
			requirements.RegisterDisruption("testme", df)

			suite.setupController("release-with-apply-now-annotation-disruption.yaml", initValues, mup)
			dr := suite.getDeckhouseRelease("v1.26.0")
			_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
			require.NoError(suite.T(), err)
		})

		suite.Run("Patch update out of window", func() {
			mup := embeddedMUP.DeepCopy()
			mup.Update.Mode = v1alpha2.UpdateModeAutoPatch.String()
			mup.Update.Windows = update.Windows{{From: "8:00", To: "10:00"}}

			suite.setupController("patch-release-with-apply-now-annotation-out-of-window.yaml", initValues, mup)
			dr := suite.getDeckhouseRelease("v1.25.1")
			_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
			require.NoError(suite.T(), err)
		})

		suite.Run("Deckhouse previous release is not ready", func() {
			mup := embeddedMUP.DeepCopy()
			mup.Update.Windows = update.Windows{{From: "8:00", To: "23:59"}}

			dependency.TestDC.HTTPClient.DoMock.
				Expect(&http.Request{}).
				Return(&http.Response{
					StatusCode: http.StatusInternalServerError,
				}, errors.New("some internal error"))

			suite.setupController("apply-now-deckhouse-previous-release-is-not-ready.yaml", initValues, mup)
			dr := suite.getDeckhouseRelease("v1.26.0")
			_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
			require.NoError(suite.T(), err)
		})

		suite.Run("Manual approval mode is set", func() {
			mup := embeddedMUP.DeepCopy()
			mup.Update.Mode = v1alpha2.UpdateModeManual.String()

			dependency.TestDC.HTTPClient.DoMock.
				Expect(&http.Request{}).
				Return(&http.Response{
					StatusCode: http.StatusOK,
				}, nil)

			suite.setupController("apply-now-manual-approval-mode-is-set.yaml", initValues, mup)
			dr := suite.getDeckhouseRelease("v1.26.0")
			_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
			require.NoError(suite.T(), err)
		})

		suite.Run("Postponed release", func() {
			suite.setupController("applied-now-postponed-release.yaml", initValues, embeddedMUP)
			dr := suite.getDeckhouseRelease("v1.25.1")
			_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
			require.NoError(suite.T(), err)
		})
	})

	suite.Run("Test auto-mode for postponed release", func() {
		suite.setupController("auto-mode.yaml", initValues, embeddedMUP)
		dr := suite.getDeckhouseRelease("v1.27.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Test auto-mode for postponed release with previous suspend", func() {
		suite.setupController("auto-mode-with-previous-suspend.yaml", initValues, embeddedMUP)
		dr := suite.getDeckhouseRelease("v1.70.17")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
		dr = suite.getDeckhouseRelease("v1.72.10")
		_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Test autoPatch-mode for postponed patch release", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Mode = v1alpha2.UpdateModeAutoPatch.String()

		dependency.TestDC.HTTPClient.DoMock.
			Expect(&http.Request{}).
			Return(&http.Response{
				StatusCode: http.StatusOK,
			}, nil)

		suite.setupController("auto-patch-mode.yaml", initValues, mup)
		dr := suite.getDeckhouseRelease("v1.26.2")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
		dr = suite.getDeckhouseRelease("v1.26.3")
		_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Test autoPatch-mode for postponed minor release", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Mode = v1alpha2.UpdateModeAutoPatch.String()

		dependency.TestDC.HTTPClient.DoMock.
			Expect(&http.Request{}).
			Return(&http.Response{
				StatusCode: http.StatusOK,
			}, nil)

		suite.setupController("auto-patch-mode-minor-release.yaml", initValues, mup)
		dr := suite.getDeckhouseRelease("v1.27.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Test autoPatch-mode for approved minor release", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Mode = v1alpha2.UpdateModeAutoPatch.String()

		dependency.TestDC.HTTPClient.DoMock.
			Expect(&http.Request{}).
			Return(&http.Response{
				StatusCode: http.StatusOK,
			}, nil)

		suite.setupController("auto-patch-mode-minor-release-approved.yaml", initValues, mup)
		dr := suite.getDeckhouseRelease("v1.27.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Test unknown-mode for postponed patch release", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Mode = "unknown"

		dependency.TestDC.HTTPClient.DoMock.
			Expect(&http.Request{}).
			Return(&http.Response{
				StatusCode: http.StatusOK,
			}, nil)

		suite.setupController("unknown-mode.yaml", initValues, mup)
		dr := suite.getDeckhouseRelease("v1.26.3")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Test unknown-mode for postponed minor release", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Mode = "unknown"

		dependency.TestDC.HTTPClient.DoMock.
			Expect(&http.Request{}).
			Return(&http.Response{
				StatusCode: http.StatusOK,
			}, nil)

		suite.setupController("unknown-mode-minor-release.yaml", initValues, mup)
		dr := suite.getDeckhouseRelease("v1.27.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("Test manual-mode for approved minor release", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Mode = v1alpha2.UpdateModeManual.String()

		dependency.TestDC.HTTPClient.DoMock.
			Expect(&http.Request{}).
			Return(&http.Response{
				StatusCode: http.StatusOK,
			}, nil)

		suite.setupController("manual-mode-with-approved.yaml", initValues, mup)
		dr := suite.getDeckhouseRelease("v1.27.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("ApplyNow: AutoPatch mode is set", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Mode = v1alpha2.UpdateModeAutoPatch.String()

		dependency.TestDC.HTTPClient.DoMock.
			Expect(&http.Request{}).
			Return(&http.Response{
				StatusCode: http.StatusOK,
			}, nil)

		suite.setupController("apply-now-autopatch-mode-is-set.yaml", initValues, mup)
		dr := suite.getDeckhouseRelease("v1.26.0")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)
	})

	suite.Run("AutoPatch", func() {
		suite.Run("patch update respect window", func() {
			mup := &v1alpha2.ModuleUpdatePolicySpec{
				Update: v1alpha2.ModuleUpdatePolicySpecUpdate{
					Mode:    "AutoPatch",
					Windows: update.Windows{{From: "10:00", To: "11:00"}},
				},
				ReleaseChannel: "Stable",
			}

			suite.setupController("auto-patch-patch-update.yaml", initValues, mup)
			dr := suite.getDeckhouseRelease("v1.26.3")
			_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
			require.NoError(suite.T(), err)
		})

		suite.Run("minor update don't respect window", func() {
			mup := &v1alpha2.ModuleUpdatePolicySpec{
				Update: v1alpha2.ModuleUpdatePolicySpecUpdate{
					Mode:    "AutoPatch",
					Windows: update.Windows{{From: "10:00", To: "11:00"}},
				},
				ReleaseChannel: "Stable",
			}

			suite.setupController("auto-patch-minor-update.yaml", initValues, mup)
			dr := suite.getDeckhouseRelease("v1.27.0")
			_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
			require.NoError(suite.T(), err)
		})
	})
	suite.Run("LTS Release channel", func() {
		suite.Run("auto", func() {
			mup := &v1alpha2.ModuleUpdatePolicySpec{
				Update: v1alpha2.ModuleUpdatePolicySpecUpdate{
					Mode: "Auto",
				},
				ReleaseChannel: "LTS",
			}

			suite.setupController("lts-release-channel-update.yaml", initValues, mup)
			// first run - change status to pending
			dr := suite.getDeckhouseRelease("v1.37.0")
			_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
			require.NoError(suite.T(), err)
			// second run - process pending release
			dr = suite.getDeckhouseRelease("v1.37.0")
			_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
			require.NoError(suite.T(), err)
		})
		suite.Run("several releases", func() {
			mup := &v1alpha2.ModuleUpdatePolicySpec{
				Update: v1alpha2.ModuleUpdatePolicySpecUpdate{
					Mode: "Auto",
				},
				ReleaseChannel: "LTS",
			}

			suite.setupController("lts-release-channel-update-several-versions.yaml", initValues, mup)
			dr := suite.getDeckhouseRelease("v1.65.6")
			_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
			require.NoError(suite.T(), err)
			dr = suite.getDeckhouseRelease("v1.70.7")
			_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
			require.NoError(suite.T(), err)
		})
		suite.Run("cannot upgrade", func() {
			mup := &v1alpha2.ModuleUpdatePolicySpec{
				Update: v1alpha2.ModuleUpdatePolicySpecUpdate{
					Mode: "Auto",
				},
				ReleaseChannel: "LTS",
			}

			suite.setupController("lts-release-channel-cannot-upgrade.yaml", initValues, mup)
			dr := suite.getDeckhouseRelease("v1.65.6")
			_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
			require.NoError(suite.T(), err)
			dr = suite.getDeckhouseRelease("v1.76.7")
			_, err = suite.ctr.createOrUpdateReconcile(ctx, dr)
			require.NoError(suite.T(), err)
		})
		suite.Run("clear data after deploy", func() {
			mup := embeddedMUP.DeepCopy()
			mup.Update.Mode = v1alpha2.UpdateModeManual.String()

			dependency.TestDC.HTTPClient.DoMock.
				Expect(&http.Request{}).
				Return(&http.Response{
					StatusCode: http.StatusNotFound,
				}, nil)
			suite.setupController("clear-data-after-deploy.yaml", initValues, mup)
			dr := suite.getDeckhouseRelease("v1.31.0")
			_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
			require.NoError(suite.T(), err)
			require.Empty(suite.T(), dr.Status.Message)
		})
	})

	suite.Run("Migrated Modules", func() {
		suite.Run("No migrated modules", func() {
			suite.setupController("migrated-modules-no-migrated-modules.yaml", initValues, embeddedMUP)
			dr := suite.getDeckhouseRelease("v1.50.0")
			_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
			require.NoError(suite.T(), err)

			// Check that v1.49.0 is superseded and v1.50.0 is deployed
			oldRelease := suite.getDeckhouseRelease("v1.49.0")
			require.Equal(suite.T(), "Superseded", oldRelease.Status.Phase)

			newRelease := suite.getDeckhouseRelease("v1.50.0")
			require.Equal(suite.T(), "Deployed", newRelease.Status.Phase)
		})

		suite.Run("Empty migrated modules", func() {
			suite.setupController("migrated-modules-empty-migrated-modules.yaml", initValues, embeddedMUP)
			dr := suite.getDeckhouseRelease("v1.50.0")
			_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
			require.NoError(suite.T(), err)

			// Check that v1.49.0 is superseded and v1.50.0 is deployed
			oldRelease := suite.getDeckhouseRelease("v1.49.0")
			require.Equal(suite.T(), "Superseded", oldRelease.Status.Phase)

			newRelease := suite.getDeckhouseRelease("v1.50.0")
			require.Equal(suite.T(), "Deployed", newRelease.Status.Phase)
		})

		suite.Run("Modules available", func() {
			suite.setupController("migrated-modules-modules-available.yaml", initValues, embeddedMUP)
			dr := suite.getDeckhouseRelease("v1.50.0")
			_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
			require.NoError(suite.T(), err)

			// Check that v1.49.0 is superseded and v1.50.0 is deployed
			oldRelease := suite.getDeckhouseRelease("v1.49.0")
			require.Equal(suite.T(), "Superseded", oldRelease.Status.Phase)

			newRelease := suite.getDeckhouseRelease("v1.50.0")
			require.Equal(suite.T(), "Deployed", newRelease.Status.Phase)
		})

		suite.Run("Module missing", func() {
			suite.setupController("migrated-modules-module-missing.yaml", initValues, embeddedMUP)
			dr := suite.getDeckhouseRelease("v1.50.0")
			_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
			require.NoError(suite.T(), err)
		})

		suite.Run("Module pull error", func() {
			suite.setupController("migrated-modules-module-pull-error.yaml", initValues, embeddedMUP)
			dr := suite.getDeckhouseRelease("v1.50.0")
			_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
			require.NoError(suite.T(), err)
		})
		suite.Run("MC disabled not in source", func() {
			suite.setupController("migrated-modules-mc-disabled-not-in-source.yaml", initValues, embeddedMUP)
			dr := suite.getDeckhouseRelease("v1.50.0")
			_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
			require.NoError(suite.T(), err)
			oldRelease := suite.getDeckhouseRelease("v1.49.0")
			require.Equal(suite.T(), "Superseded", oldRelease.Status.Phase)

			newRelease := suite.getDeckhouseRelease("v1.50.0")
			require.Equal(suite.T(), "Deployed", newRelease.Status.Phase)
		})

		suite.Run("MC enabled not in source", func() {
			suite.setupController("migrated-modules-mc-enabled-not-in-source.yaml", initValues, embeddedMUP)
			dr := suite.getDeckhouseRelease("v1.50.0")
			_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
			require.NoError(suite.T(), err)

			oldRelease := suite.getDeckhouseRelease("v1.49.0")
			require.Equal(suite.T(), "Deployed", oldRelease.Status.Phase)

			newRelease := suite.getDeckhouseRelease("v1.50.0")
			require.Equal(suite.T(), "Pending", newRelease.Status.Phase)
			require.Contains(suite.T(), newRelease.Status.Message, "not found in any ModuleSource registry")
		})
		suite.Run("MC enabled in source", func() {
			suite.setupController("migrated-modules-mc-enabled-in-source.yaml", initValues, embeddedMUP)
			dr := suite.getDeckhouseRelease("v1.50.0")
			_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
			require.NoError(suite.T(), err)

			// Check that v1.49.0 is superseded and v1.50.0 is deployed
			oldRelease := suite.getDeckhouseRelease("v1.49.0")
			require.Equal(suite.T(), "Superseded", oldRelease.Status.Phase)

			newRelease := suite.getDeckhouseRelease("v1.50.0")
			require.Equal(suite.T(), "Deployed", newRelease.Status.Phase)
		})
	})
}

func newDependencyContainer(t *testing.T) *dependency.MockedContainer {
	t.Helper()

	dc := dependency.NewMockedContainer()
	dc.CRClient = cr.NewClientMock(t)
	dc.HTTPClient.DoMock.
		Expect(&http.Request{}).
		Return(&http.Response{
			StatusCode: http.StatusOK,
		}, nil)

	return dc
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

func singleDocToManifests(doc []byte) []string {
	split := mDelimiter.Split(string(doc), -1)

	result := make([]string, 0, len(split))
	for i := range split {
		if split[i] != "" {
			result = append(result, split[i])
		}
	}

	return result
}

func (suite *ControllerTestSuite) getDeckhouseRelease(name string) *v1alpha1.DeckhouseRelease {
	var release v1alpha1.DeckhouseRelease
	err := suite.kubeClient.Get(context.TODO(), types.NamespacedName{Name: name}, &release)
	require.NoError(suite.T(), err)

	return &release
}

func (suite *ControllerTestSuite) loopUntilDeploy(dc *dependency.MockedContainer, releaseName string) {
	const maxIterations = 3
	suite.T().Skip("TODO: requeue all releases after got deckhouse module config update")

	var (
		result = ctrl.Result{Requeue: true}
		err    error
		i      int
	)

	for result.Requeue || result.RequeueAfter > 0 {
		dc.GetFakeClock().Advance(result.RequeueAfter)

		dr := suite.getDeckhouseRelease(releaseName)
		if dr.Status.Phase == v1alpha1.ModuleReleasePhaseDeployed {
			suite.T().Log("Deployed")
			return
		}

		result, err = suite.ctr.createOrUpdateReconcile(context.TODO(), dr)
		require.NoError(suite.T(), err)

		i++
		if i > maxIterations {
			suite.T().Fatal("Too many iterations")
		}
	}

	suite.T().Fatal("Loop was broken")
}

type stubModulesManager struct{}

func (s stubModulesManager) GetEnabledModuleNames() []string {
	return []string{
		"cert-manager",
		"prometheus",
		"test-module-1",
		"test-module-2",
		"test-module-missing",
		"enabled-module-not-found",
		"enabled-module",
	}
}

func (s stubModulesManager) IsModuleEnabled(_ string) bool {
	return true
}
