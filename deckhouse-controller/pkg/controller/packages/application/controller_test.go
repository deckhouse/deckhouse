/*
Copyright 2025 Flant JSC

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

package application

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"

	packageoperator "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime"
	packagestatus "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/releaseupdater"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	"github.com/deckhouse/deckhouse/go_lib/libapi"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/testing/controller/reconcilertest"
)

func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}

type ControllerTestSuite struct {
	reconcilertest.Suite

	ctr     *reconciler
	runtime *operatorStub
}

func (suite *ControllerTestSuite) SetupSuite() {
	suite.Init(reconcilertest.Config{
		StatusSubresources: []client.Object{
			&v1alpha1.Application{},
			&v1alpha1.ApplicationPackage{},
			&v1alpha1.ApplicationPackageVersion{},
		},
		SnapshotKinds: []schema.GroupVersionKind{
			v1alpha1.SchemeGroupVersion.WithKind("Application"),
			v1alpha1.SchemeGroupVersion.WithKind("ApplicationPackage"),
			v1alpha1.SchemeGroupVersion.WithKind("ApplicationPackageVersion"),
			v1alpha1.SchemeGroupVersion.WithKind("PackageRepository"),
		},
		GoldenMode:    reconcilertest.PerDocument,
		SeedViaCreate: true,
	})
}

func (suite *ControllerTestSuite) SetupSubTest() {
	reconcilertest.RespondHTTPOK()
}

type reconcilerOption func(*reconciler)

func withDependencyContainer(dc dependency.Container) reconcilerOption {
	return func(r *reconciler) {
		r.dc = dc
	}
}

// updateSettings are the deckhouse and the global ModuleConfig settings, as JSON, that the
// update settings of an application fall back to. Empty ones leave the deckhouse defaults and
// no global applications settings.
type updateSettings struct {
	deckhouse string
	global    string
}

// withUpdateSettings fills the settings containers the way the deckhouse and the global
// ModuleConfigs fill them in a running controller.
func withUpdateSettings(t *testing.T, settings updateSettings) reconcilerOption {
	return func(r *reconciler) {
		deckhouse := helpers.DefaultDeckhouseSettings()
		if settings.deckhouse != "" {
			require.NoError(t, json.Unmarshal([]byte(settings.deckhouse), deckhouse))
		}

		applications := helpers.NewApplicationsSettingsContainer()
		if settings.global != "" {
			global := make(addonutils.Values)
			require.NoError(t, json.Unmarshal([]byte(settings.global), &global))
			require.NoError(t, applications.Set(global))
		}

		r.deckhouseSettings = helpers.NewDeckhouseSettingsContainer(deckhouse, nil)
		r.applicationsSettings = applications
	}
}

// setupController seeds the cluster from a fixture and wires the reconciler under
// test to the fake client. No ModuleConfig sets anything unless an option says so, so the
// update settings fall back to the deckhouse defaults.
func (suite *ControllerTestSuite) setupController(filename string, options ...reconcilerOption) {
	suite.Seed(filename)

	suite.runtime = &operatorStub{}
	suite.ctr = &reconciler{
		init:                 new(sync.WaitGroup),
		client:               suite.Client(),
		logger:               log.NewNop(),
		runtime:              suite.runtime,
		moduleManager:        &moduleManagerStub{},
		dc:                   dependency.NewMockedContainer(),
		deckhouseSettings:    helpers.NewDeckhouseSettingsContainer(helpers.DefaultDeckhouseSettings(), nil),
		applicationsSettings: helpers.NewApplicationsSettingsContainer(),
	}

	for _, opt := range options {
		opt(suite.ctr)
	}
}

func (suite *ControllerTestSuite) TestReconcile() {
	ctx := context.Background()

	dependency.TestDC.CRClient.ImageMock.Return(reconcilertest.Image(nil), nil)

	suite.Run("resource not found", func() {
		suite.setupController("resource-not-found.yaml")
		_, err := suite.ctr.Reconcile(ctx, suite.Request("non-existent-app", ""))
		require.NoError(suite.T(), err)
	})

	suite.Run("successful reconcile with golden file", func() {
		requirements.RegisterCheck("k8s", func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
			v, _ := getter.Get("global.discovery.kubernetesVersion")
			if v != requirementValue {
				return false, errors.New("min k8s version failed")
			}

			return true, nil
		})
		requirements.SaveValue("global.discovery.kubernetesVersion", "1.19.0")

		dc := dependency.NewMockedContainer()

		suite.setupController("successful-reconcile.yaml", withDependencyContainer(dc))

		app := suite.getApplication("test-app", "foobar")
		_, err := suite.ctr.Reconcile(ctx, suite.Request(app.Name, app.Namespace))
		require.NoError(suite.T(), err)
	})

	suite.Run("version not found", func() {
		suite.setupController("version-not-found.yaml")
		app := suite.getApplication("test-app", "foobar")
		_, err := suite.ctr.Reconcile(ctx, suite.Request(app.Name, app.Namespace))
		require.NoError(suite.T(), err)
	})

	suite.Run("version is draft", func() {
		suite.setupController("version-is-draft.yaml")
		app := suite.getApplication("test-app", "foobar")
		_, err := suite.ctr.Reconcile(ctx, suite.Request(app.Name, app.Namespace))
		require.NoError(suite.T(), err)
	})

	suite.Run("successful reconcile with some falses", func() {
		requirements.RegisterCheck("k8s", func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
			v, _ := getter.Get("global.discovery.kubernetesVersion")
			if v != requirementValue {
				return false, errors.New("min k8s version failed")
			}

			return true, nil
		})
		requirements.SaveValue("global.discovery.kubernetesVersion", "1.19.0")

		dc := dependency.NewMockedContainer()

		suite.setupController("successful-reconcile-some-falses.yaml", withDependencyContainer(dc))

		app := suite.getApplication("test-app", "foobar")
		_, err := suite.ctr.Reconcile(ctx, suite.Request(app.Name, app.Namespace))
		require.NoError(suite.T(), err)
	})

	suite.Run("successful reconcile with all falses", func() {
		requirements.RegisterCheck("k8s", func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
			v, _ := getter.Get("global.discovery.kubernetesVersion")
			if v != requirementValue {
				return false, errors.New("min k8s version failed")
			}

			return true, nil
		})
		requirements.SaveValue("global.discovery.kubernetesVersion", "1.19.0")

		dc := dependency.NewMockedContainer()

		suite.setupController("successful-reconcile-all-falses.yaml", withDependencyContainer(dc))

		app := suite.getApplication("test-app", "foobar")
		_, err := suite.ctr.Reconcile(ctx, suite.Request(app.Name, app.Namespace))
		require.NoError(suite.T(), err)
	})

	suite.Run("version update cleans up old APV", func() {
		requirements.RegisterCheck("k8s", func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
			v, _ := getter.Get("global.discovery.kubernetesVersion")
			if v != requirementValue {
				return false, errors.New("min k8s version failed")
			}

			return true, nil
		})
		requirements.SaveValue("global.discovery.kubernetesVersion", "1.19.0")

		dc := dependency.NewMockedContainer()

		suite.setupController("version-update.yaml", withDependencyContainer(dc))

		app := suite.getApplication("test-app", "foobar")
		_, err := suite.ctr.Reconcile(ctx, suite.Request(app.Name, app.Namespace))
		require.NoError(suite.T(), err)

		// Verify old APV no longer has the app in usedBy
		oldAPV := new(v1alpha1.ApplicationPackageVersion)
		err = suite.Client().Get(ctx, client.ObjectKey{Name: "deckhouse-test-v1.0.1"}, oldAPV)
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), 0, oldAPV.Status.UsedByCount, "Old APV should have usedByCount=0")
		assert.Empty(suite.T(), oldAPV.Status.UsedBy, "Old APV should have empty usedBy list")

		// Verify new APV has the app in usedBy
		newAPV := new(v1alpha1.ApplicationPackageVersion)
		err = suite.Client().Get(ctx, client.ObjectKey{Name: "deckhouse-test-v1.0.2"}, newAPV)
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), 1, newAPV.Status.UsedByCount, "New APV should have usedByCount=1")
		assert.Len(suite.T(), newAPV.Status.UsedBy, 1, "New APV should have 1 app in usedBy")
		assert.Equal(suite.T(), "test-app", newAPV.Status.UsedBy[0].Name)
		assert.Equal(suite.T(), "foobar", newAPV.Status.UsedBy[0].Namespace)

		// Verify app owner references updated
		updatedApp := suite.getApplication("test-app", "foobar")
		ownerRefs := updatedApp.GetOwnerReferences()
		apvRefCount := 0
		var apvRefName string
		for _, ref := range ownerRefs {
			if ref.Kind == v1alpha1.ApplicationPackageVersionKind {
				apvRefCount++
				apvRefName = ref.Name
			}
		}
		assert.Equal(suite.T(), 1, apvRefCount, "App should have exactly 1 APV owner reference")
		assert.Equal(suite.T(), "deckhouse-test-v1.0.2", apvRefName, "App should reference new APV")

		// Verify ApplicationPackage usedBy version is updated
		ap := new(v1alpha1.ApplicationPackage)
		err = suite.Client().Get(ctx, client.ObjectKey{Name: "test"}, ap)
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), 1, ap.Status.UsedByCount, "AP should have usedByCount=1")
		require.Len(suite.T(), ap.Status.UsedBy, 1, "AP should have 1 app in usedBy")
		assert.Equal(suite.T(), "test-app", ap.Status.UsedBy[0].Name)
		assert.Equal(suite.T(), "foobar", ap.Status.UsedBy[0].Namespace)
		assert.Equal(suite.T(), "v1.0.2", ap.Status.UsedBy[0].Version, "AP usedBy version should be updated to new version")
	})

	suite.Run("update settings default to the deckhouse defaults", func() {
		suite.setupController("update-settings.yaml")

		for _, name := range []string{"inherit-app", "own-app"} {
			_, err := suite.ctr.Reconcile(ctx, suite.Request(name, "foobar"))
			require.NoError(suite.T(), err)
		}

		require.Len(suite.T(), suite.runtime.updated, 2)

		// No ModuleConfig sets anything, so these are the deckhouse defaults.
		assert.Equal(suite.T(), releaseupdater.Settings{
			Mode:          v1alpha2.UpdateModeAutoPatch,
			BlockOnAlerts: releaseupdater.BlockOnAlerts{Severity: 4},
		}, suite.runtime.updated[0].UpdateSettings,
			"without any settings an application must take the deckhouse defaults")

		assert.Equal(suite.T(), releaseupdater.Settings{
			Mode:          v1alpha2.UpdateModeManual,
			Windows:       update.Windows{{From: "20:00", To: "23:30", Days: []string{"Sun"}}},
			BlockOnAlerts: releaseupdater.BlockOnAlerts{Enabled: true, Severity: 0},
		}, suite.runtime.updated[1].UpdateSettings,
			"an application with its own settings must keep them over the deckhouse defaults")
	})

	suite.Run("update settings are taken from the global applications settings", func() {
		suite.setupController("update-settings.yaml", withUpdateSettings(suite.T(), updateSettings{
			deckhouse: `{"update": {
				"mode": "Manual",
				"windows": [{"from": "1:00", "to": "2:00"}],
				"notification": {"webhook": "https://deckhouse.example.com/webhook"},
				"blockOnAlerts": {"enabled": true, "severity": 1}
			}}`,
			global: `{"applications": {"update": {
				"mode": "Auto",
				"windows": [{"from": "8:00", "to": "15:00", "days": ["Tue", "Sat"]}],
				"notification": {"webhook": "https://applications.example.com/webhook", "minimalNotificationTime": "6h", "releaseType": "All"},
				"blockOnAlerts": {"enabled": true}
			}}}`,
		}))

		for _, name := range []string{"inherit-app", "own-app"} {
			_, err := suite.ctr.Reconcile(ctx, suite.Request(name, "foobar"))
			require.NoError(suite.T(), err)
		}

		require.Len(suite.T(), suite.runtime.updated, 2)

		notification := releaseupdater.NotificationConfig{
			WebhookURL:              "https://applications.example.com/webhook",
			MinimalNotificationTime: libapi.Duration{Duration: 6 * time.Hour},
			ReleaseType:             releaseupdater.ReleaseTypeAll,
		}

		assert.Equal(suite.T(), releaseupdater.Settings{
			Mode:               v1alpha2.UpdateModeAuto,
			Windows:            update.Windows{{From: "8:00", To: "15:00", Days: []string{"Tue", "Sat"}}},
			NotificationConfig: notification,
			BlockOnAlerts:      releaseupdater.BlockOnAlerts{Enabled: true, Severity: 4},
		}, suite.runtime.updated[0].UpdateSettings,
			"an application without its own settings must take the global applications settings as a whole")

		assert.Equal(suite.T(), releaseupdater.Settings{
			Mode:               v1alpha2.UpdateModeManual,
			Windows:            update.Windows{{From: "20:00", To: "23:30", Days: []string{"Sun"}}},
			NotificationConfig: notification,
			BlockOnAlerts:      releaseupdater.BlockOnAlerts{Enabled: true, Severity: 0},
		}, suite.runtime.updated[1].UpdateSettings,
			"an application with its own settings must take only the notification settings from the global applications settings")
	})

	suite.Run("update settings fall back to the deckhouse settings", func() {
		suite.setupController("update-settings.yaml", withUpdateSettings(suite.T(), updateSettings{
			deckhouse: `{"releaseChannel": "Stable", "update": {
				"mode": "Manual",
				"windows": [{"from": "1:00", "to": "2:00"}],
				"notification": {"webhook": "https://deckhouse.example.com/webhook", "tlsSkipVerify": true},
				"blockOnAlerts": {"enabled": true}
			}}`,
			global: `{"applications": {"ingressClass": "nginx"}}`,
		}))

		for _, name := range []string{"inherit-app", "own-app"} {
			_, err := suite.ctr.Reconcile(ctx, suite.Request(name, "foobar"))
			require.NoError(suite.T(), err)
		}

		require.Len(suite.T(), suite.runtime.updated, 2)

		notification := releaseupdater.NotificationConfig{
			WebhookURL:    "https://deckhouse.example.com/webhook",
			SkipTLSVerify: true,
		}

		assert.Equal(suite.T(), releaseupdater.Settings{
			Mode:               v1alpha2.UpdateModeManual,
			Windows:            update.Windows{{From: "1:00", To: "2:00"}},
			NotificationConfig: notification,
			BlockOnAlerts:      releaseupdater.BlockOnAlerts{Enabled: true, Severity: 4},
		}, suite.runtime.updated[0].UpdateSettings,
			"without the global applications settings an application must take the deckhouse settings as a whole")

		assert.Equal(suite.T(), releaseupdater.Settings{
			Mode:               v1alpha2.UpdateModeManual,
			Windows:            update.Windows{{From: "20:00", To: "23:30", Days: []string{"Sun"}}},
			NotificationConfig: notification,
			BlockOnAlerts:      releaseupdater.BlockOnAlerts{Enabled: true, Severity: 0},
		}, suite.runtime.updated[1].UpdateSettings,
			"without the global applications settings an application with its own settings must take the deckhouse notification settings")
	})
}

// nolint:unparam
func (suite *ControllerTestSuite) getApplication(name string, namespace string) *v1alpha1.Application {
	var app v1alpha1.Application
	err := suite.Client().Get(context.TODO(), client.ObjectKey{Name: name, Namespace: namespace}, &app)
	require.NoError(suite.T(), err)
	return &app
}

type moduleManagerStub struct{}

func (m *moduleManagerStub) AreModulesInited() bool {
	return true
}

// operatorStub records the applications handed to the runtime.
type operatorStub struct {
	updated []packageoperator.App
}

func (o *operatorStub) UpdateApp(_ registry.Remote, app packageoperator.App) {
	o.updated = append(o.updated, app)
}

// RemoveApp reports the teardown as finished: the stub tears nothing down.
func (o *operatorStub) RemoveApp(_, _ string) bool {
	return true
}

func (o *operatorStub) GetStatus(name string) packagestatus.Status {
	return packagestatus.NewService().GetStatus(name)
}

func (o *operatorStub) GetStatusQueue() workqueue.TypedRateLimitingInterface[string] {
	return packagestatus.NewService().Queue()
}

func (o *operatorStub) Cleanup(_ context.Context, _ []packageoperator.PreservePackage) {}
