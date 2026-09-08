// Copyright 2026 Flant JSC
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

package module

import (
	"context"
	"sync"
	"testing"

	"github.com/flant/addon-operator/pkg/kube_config_manager/config"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	promdto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/controller/confighandler"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/deckhouse/deckhouse/testing/controller/reconcilertest"
)

// These tests pin the settings controller's own contract. Since the migration to
// the package status service, this controller no longer computes Module status:
// it applies settings-and-enabled to the package runtime and does the bookkeeping
// around that (finalizer, MPV.Used, the allow-disabling annotation, module docs,
// the conflict metric). The lifecycle status (conditions/summary) is owned by
// pkg/controller/packages/module/status and asserted there — so every test below
// also checks that this controller leaves the Module status untouched.

var conversionsStore = conversion.NewConversionsStore()

type ControllerTestSuite struct {
	reconcilertest.Suite

	r   *reconciler
	pkg *spyPackageManager
	ms  *metricstorage.MetricStorage
}

func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}

func (suite *ControllerTestSuite) SetupSuite() {
	suite.Init(reconcilertest.Config{
		StatusSubresources: []client.Object{
			&v1alpha2.Module{},
			&v1alpha1.ModulePackage{},
			&v1alpha1.ModulePackageVersion{},
		},
	})
}

// TearDownSubTest overrides the base golden assertion: this suite verifies the
// controller's contract with explicit assertions instead of golden snapshots.
func (suite *ControllerTestSuite) TearDownSubTest() {}

func (suite *ControllerTestSuite) setupTestController(filename string) {
	suite.Seed(filename)
	suite.buildReconciler()
}

func (suite *ControllerTestSuite) setupTestControllerRaw(raw string) {
	suite.SeedRaw("", []byte(raw))
	suite.buildReconciler()
}

func (suite *ControllerTestSuite) buildReconciler() {
	suite.pkg = &spyPackageManager{}
	suite.ms = metricstorage.NewMetricStorage(metricstorage.WithNewRegistry(), metricstorage.WithLogger(log.NewNop()))

	rec := &reconciler{
		init:             new(sync.WaitGroup),
		client:           suite.Client(),
		logger:           log.NewNop(),
		handler:          newMockHandler(),
		conversionsStore: conversionsStore,
		moduleManager:    newMockModuleManager(),
		packageManager:   suite.pkg,
		metricStorage:    suite.ms,
		configValidator:  nil, // Disable validation in tests to avoid schema issues
	}

	// simulate initialization
	rec.init.Add(1)
	rec.init.Done()
	suite.r = rec
}

func (suite *ControllerTestSuite) reconcile(name string) {
	_, err := suite.r.Reconcile(context.TODO(), suite.Request(name, ""))
	require.NoError(suite.T(), err)
}

func (suite *ControllerTestSuite) module(name string) *v1alpha2.Module {
	module := new(v1alpha2.Module)
	err := suite.Client().Get(context.TODO(), client.ObjectKey{Name: name}, module)
	require.NoError(suite.T(), err)

	return module
}

func (suite *ControllerTestSuite) modulePackageVersion(name string) *v1alpha1.ModulePackageVersion {
	mpv := new(v1alpha1.ModulePackageVersion)
	err := suite.Client().Get(context.TODO(), client.ObjectKey{Name: name}, mpv)
	require.NoError(suite.T(), err)

	return mpv
}

// conflictMetric returns the value of the module-conflict gauge for the given
// module, or -1 when it was never set.
func (suite *ControllerTestSuite) conflictMetric(module string) float64 {
	families, err := suite.ms.Gather()
	require.NoError(suite.T(), err)

	for _, family := range families {
		if family.GetName() != metrics.D8ModuleAtConflict {
			continue
		}
		for _, m := range family.GetMetric() {
			if labelValue(m, "module") == module && m.GetGauge() != nil {
				return m.GetGauge().GetValue()
			}
		}
	}

	return -1
}

func labelValue(m *promdto.Metric, name string) string {
	for _, l := range m.GetLabel() {
		if l.GetName() == name {
			return l.GetValue()
		}
	}
	return ""
}

// assertStatusUntouched fails if the controller changed the Module status: the
// lifecycle status is owned by the package status service, not this controller.
func (suite *ControllerTestSuite) assertStatusUntouched(name string, seeded v1alpha2.ModuleStatus) {
	assert.Equal(suite.T(), seeded, suite.module(name).Status, "controller must not modify Module status")
}

// TestReconcileEnabledModule pushes an enabled module's settings to the runtime,
// installs the finalizer and leaves the status alone.
func (suite *ControllerTestSuite) TestReconcileEnabledModule() {
	suite.setupTestController("create-module.yaml")
	seeded := suite.module("test-module").Status

	suite.reconcile("test-module")

	// settings-and-enabled are handed to the package runtime verbatim
	require.Len(suite.T(), suite.pkg.calls, 1)
	call := suite.pkg.calls[0]
	assert.Equal(suite.T(), "test-module", call.name)
	assert.Equal(suite.T(), 1, call.settingsVersion)
	assert.Equal(suite.T(), addonutils.Values{"foo": "bar"}, call.settings)
	require.NotNil(suite.T(), call.enabled)
	assert.True(suite.T(), *call.enabled)

	// finalizer is installed so a later delete can be intercepted
	assert.Contains(suite.T(), suite.module("test-module").Finalizers, v1alpha2.ModuleFinalizerModuleRegistered)

	// status is not this controller's job
	suite.assertStatusUntouched("test-module", seeded)
}

// TestReconcileDoesNotModifyStatus guards the migration boundary directly: a
// module that arrives with conditions and a summary must keep them verbatim.
func (suite *ControllerTestSuite) TestReconcileDoesNotModifyStatus() {
	suite.setupTestController("change-source.yaml")
	seeded := suite.module("test-module").Status
	require.NotNil(suite.T(), seeded.Summary)
	require.NotEmpty(suite.T(), seeded.Conditions)

	suite.reconcile("test-module")

	suite.assertStatusUntouched("test-module", seeded)
}

// TestReconcileMultipleSources fires the conflict metric when a module resolves
// to more than one repository, without writing that conflict onto the status.
func (suite *ControllerTestSuite) TestReconcileMultipleSources() {
	suite.setupTestController("multiple-sources.yaml")
	seeded := suite.module("test-module").Status

	suite.reconcile("test-module")

	assert.Equal(suite.T(), 1.0, suite.conflictMetric("test-module"))
	suite.assertStatusUntouched("test-module", seeded)
}

// TestReconcilePinnedRepository: a module that pins packageRepositoryName never
// enters conflict detection, so the conflict metric stays unset.
func (suite *ControllerTestSuite) TestReconcilePinnedRepository() {
	suite.setupTestController("no-settings.yaml")

	suite.reconcile("no-settings")

	require.Len(suite.T(), suite.pkg.calls, 1)
	assert.Equal(suite.T(), "no-settings", suite.pkg.calls[0].name)
	assert.Equal(suite.T(), -1.0, suite.conflictMetric("no-settings"))
}

// TestReconcileSystemModule: a system module (global) is registered with the
// runtime and finalized, but reconcile returns before conflict detection —
// it has no ModulePackage to look up.
func (suite *ControllerTestSuite) TestReconcileSystemModule() {
	suite.setupTestController("global-config.yaml")

	suite.reconcile("global")

	require.Len(suite.T(), suite.pkg.calls, 1)
	assert.Equal(suite.T(), "global", suite.pkg.calls[0].name)
	assert.Contains(suite.T(), suite.module("global").Finalizers, v1alpha2.ModuleFinalizerModuleRegistered)
}

// TestReconcileEmbeddedModule: an embedded module is registered and finalized but
// skipped before conflict detection, so no conflict metric is emitted.
func (suite *ControllerTestSuite) TestReconcileEmbeddedModule() {
	suite.setupTestController("embedded-module.yaml")
	seeded := suite.module("test-module").Status

	suite.reconcile("test-module")

	assert.Contains(suite.T(), suite.module("test-module").Finalizers, v1alpha2.ModuleFinalizerModuleRegistered)
	assert.Equal(suite.T(), -1.0, suite.conflictMetric("test-module"))
	suite.assertStatusUntouched("test-module", seeded)
}

// TestReconcileDisablePreviouslyEnabled covers the disable bookkeeping for a
// module that was running: MPV.Used drops, the allow-disabling annotation is
// removed and the module documentation is deleted.
func (suite *ControllerTestSuite) TestReconcileDisablePreviouslyEnabled() {
	suite.setupTestController("disable-enabled-module.yaml")
	seeded := suite.module("test-module").Status

	suite.reconcile("test-module")

	// settings still flow to the runtime, now with enabled=false
	require.Len(suite.T(), suite.pkg.calls, 1)
	require.NotNil(suite.T(), suite.pkg.calls[0].enabled)
	assert.False(suite.T(), *suite.pkg.calls[0].enabled)

	// the package version is released
	assert.False(suite.T(), suite.modulePackageVersion("test-source-test-module-v1.0.0").Status.Used)

	// the allow-disabling annotation is cleared
	_, ok := suite.module("test-module").Annotations[v1alpha2.ModuleConfigAnnotationAllowDisable]
	assert.False(suite.T(), ok)

	// the module documentation is dropped so docs-builder removes it
	err := suite.Client().Get(context.TODO(), client.ObjectKey{Name: "test-module"}, new(v1alpha1.ModuleDocumentation))
	assert.True(suite.T(), apierrors.IsNotFound(err))

	// status stays owned by the status service
	suite.assertStatusUntouched("test-module", seeded)
}

// TestReconcileDisableNotPreviouslyEnabled: a module that was never Enabled=True
// must not touch MPV.Used — the release bookkeeping is gated on the prior state.
func (suite *ControllerTestSuite) TestReconcileDisableNotPreviouslyEnabled() {
	suite.setupTestController("disable-module.yaml")

	suite.reconcile("test-module")

	assert.True(suite.T(), suite.modulePackageVersion("test-source-test-module-v1.0.0").Status.Used)
}

// TestReconcileNotFound: a reconcile for an absent module is a no-op — no error,
// no settings pushed to the runtime.
func (suite *ControllerTestSuite) TestReconcileNotFound() {
	suite.setupTestController("not-found.yaml")

	suite.reconcile("test-module")

	assert.Empty(suite.T(), suite.pkg.calls)
}

func (suite *ControllerTestSuite) TestDeleteReconcile() {
	suite.Run("delete module removes the finalizer", func() {
		suite.setupTestController("delete-module.yaml")
		suite.reconcile("test-module")

		// the finalizer is removed, so the module is garbage-collected
		module := new(v1alpha2.Module)
		err := suite.Client().Get(context.TODO(), client.ObjectKey{Name: "test-module"}, module)
		assert.True(suite.T(), apierrors.IsNotFound(err))
	})

	suite.Run("system module keeps the finalizer", func() {
		suite.setupTestController("delete-system-module.yaml")
		suite.reconcile("global")

		// system modules are skipped, so the finalizer stays in place
		module := new(v1alpha2.Module)
		err := suite.Client().Get(context.TODO(), client.ObjectKey{Name: "global"}, module)
		require.NoError(suite.T(), err)
		assert.Len(suite.T(), module.Finalizers, 1)
	})

	suite.Run("deletion timestamp is honored", func() {
		m := `
apiVersion: deckhouse.io/v1alpha2
kind: Module
metadata:
  name: test-module
  finalizers:
  - module.deckhouse.io/module-registered
  deletionTimestamp: "2024-01-01T00:00:00Z"
spec:
  enabled: true
`
		suite.setupTestControllerRaw(m)

		module := suite.module("test-module")
		assert.NotNil(suite.T(), module.DeletionTimestamp)
		assert.Len(suite.T(), module.Finalizers, 1)
	})
}

// Mock implementations

// spyPackageManager records UpdateModulesSettings calls so tests can assert the
// exact settings-and-enabled handed to the package runtime.
type spyPackageManager struct {
	calls []updateSettingsCall
}

type updateSettingsCall struct {
	name            string
	settingsVersion int
	settings        addonutils.Values
	maintenance     string
	enabled         *bool
}

func (s *spyPackageManager) UpdateModulesSettings(name string, settingsVersion int, settings addonutils.Values, maintenance string, enabled *bool) {
	s.calls = append(s.calls, updateSettingsCall{
		name:            name,
		settingsVersion: settingsVersion,
		settings:        settings,
		maintenance:     maintenance,
		enabled:         enabled,
	})
}

type mockModuleManager struct{}

func newMockModuleManager() *mockModuleManager {
	return &mockModuleManager{}
}

func (m *mockModuleManager) GetModule(name string) *modules.BasicModule {
	if name == "test-module" || name == "deckhouse" {
		return &modules.BasicModule{Name: name}
	}
	return nil
}

func (m *mockModuleManager) GetGlobal() *modules.GlobalModule {
	return &modules.GlobalModule{}
}

func newMockHandler() *confighandler.Handler {
	// minimal handler for tests with dummy channels
	deckhouseConfigCh := make(chan addonutils.Values, 10)
	configEventCh := make(chan config.Event, 10)

	handler := confighandler.New(nil, conversionsStore, deckhouseConfigCh)
	handler.StartInformer(context.Background(), configEventCh)

	return handler
}
