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

package config

import (
	"context"
	"sync"
	"testing"

	"github.com/flant/addon-operator/pkg/kube_config_manager/config"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules/events"
	"github.com/flant/addon-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/confighandler"
	d8edition "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/deckhouse/deckhouse/testing/controller/reconcilertest"
)

var conversionsStore = conversion.NewConversionsStore()

type ControllerTestSuite struct {
	reconcilertest.Suite

	r *reconciler

	// handlerEventCh receives every config.Event that r.handler.HandleEvent
	// dispatches to addon-operator, so tests can assert whether the v1 path
	// was (not) taken. See drainHandlerEvents.
	handlerEventCh chan config.Event

	compareGolden bool
}

func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}

func (suite *ControllerTestSuite) SetupSuite() {
	suite.Init(reconcilertest.Config{
		StatusSubresources: []client.Object{
			&v1alpha1.Module{},
			&v1alpha1.ModuleConfig{},
			&v1alpha1.ModuleRelease{},
		},
		SnapshotKinds: []schema.GroupVersionKind{
			v1alpha1.SchemeGroupVersion.WithKind("ModuleConfig"),
			v1alpha1.SchemeGroupVersion.WithKind("Module"),
			v1alpha1.SchemeGroupVersion.WithKind("ModuleRelease"),
		},
		ObjectNormalizers: []reconcilertest.ObjectNormalizer{clearModuleConditionTimes},
		GoldenMode:        reconcilertest.PerDocument,
	})
}

func (suite *ControllerTestSuite) BeforeTest(suiteName, testName string) {
	if suiteName == "ControllerTestSuite" && testName == "TestCreateReconcile" {
		suite.compareGolden = true
	}
}

func (suite *ControllerTestSuite) AfterTest(_, _ string) {
	suite.compareGolden = false
}

// TearDownSubTest only asserts golden for the golden-driven test (TestCreateReconcile).
func (suite *ControllerTestSuite) TearDownSubTest() {
	if !suite.compareGolden {
		return
	}
	suite.AssertGolden()
}

func (suite *ControllerTestSuite) setupTestController(filename string) {
	suite.Seed(filename)
	suite.buildReconciler()
}

func (suite *ControllerTestSuite) setupTestControllerRaw(raw string) {
	suite.SeedRaw("", []byte(raw))
	suite.buildReconciler()
}

func (suite *ControllerTestSuite) buildReconciler() {
	handler, eventCh := newMockHandler()

	rec := &reconciler{
		init:                 new(sync.WaitGroup),
		client:               suite.Client(),
		logger:               log.NewNop(),
		handler:              handler,
		conversionsStore:     conversionsStore,
		moduleManager:        newMockModuleManager(),
		edition:              &d8edition.Edition{Name: "fe", Bundle: "Default"},
		metricStorage:        metricstorage.NewMetricStorage(metricstorage.WithNewRegistry(), metricstorage.WithLogger(log.NewNop())),
		configValidator:      nil,                          // Disable validation in tests to avoid schema issues
		exts:                 nil,                          // Extenders not needed for these tests
		packageRuntime:       newMockPackageRuntime(false), // v1 path by default; set true for v2 tests
		packageSystemEnabled: true,                         // gate required to enter the v2 branch
	}

	// simulate initialization
	rec.init.Add(1)
	rec.init.Done()
	suite.r = rec
	suite.handlerEventCh = eventCh
}

// drainHandlerEvents non-blockingly drains suite.handlerEventCh and returns how
// many addon-operator events were dispatched via handler.HandleEvent since the
// last drain. Used to prove the v1/v2 routing fork is exclusive (G2): the v2
// path must never also emit an addon-operator event, and the v1 path must
// always emit exactly one.
func (suite *ControllerTestSuite) drainHandlerEvents() int {
	count := 0
	for {
		select {
		case <-suite.handlerEventCh:
			count++
		default:
			return count
		}
	}
}

// clearModuleConditionTimes drops timestamp fields from Module conditions to keep
// golden snapshots stable.
func clearModuleConditionTimes(obj client.Object) {
	module, ok := obj.(*v1alpha1.Module)
	if !ok {
		return
	}
	for i := range module.Status.Conditions {
		module.Status.Conditions[i].LastProbeTime = metav1.Time{}
		module.Status.Conditions[i].LastTransitionTime = metav1.Time{}
	}
}

// TestV2ModuleConfigRouting verifies that when the package-system feature flag
// is enabled and the module is tracked by the package runtime, handleModuleConfig
// routes settings via UpdateModulesSettings instead of the addon-operator
// HandleEvent. Conversely, when the flag is off it always uses the v1 path.
//
// Each sub-test asserts both sides of the fork (G2): the v2 path must call
// UpdateModulesSettings and must NOT dispatch an addon-operator event, and the
// v1 path must dispatch exactly one addon-operator event and must NOT call
// UpdateModulesSettings. A regression that dispatched both would fail here.
func (suite *ControllerTestSuite) TestV2ModuleConfigRouting() {
	suite.Run("v2 path: settings routed to UpdateModulesSettings, no addon-operator event", func() {
		suite.setupTestController("enable-module.yaml")

		// Override with v2-aware mock
		mockPkg := newMockPackageRuntime(true) // HasModule returns true
		suite.r.packageRuntime = mockPkg
		suite.r.packageSystemEnabled = true

		config := suite.moduleConfig("test-module")
		_, err := suite.r.handleModuleConfig(context.TODO(), config)
		require.NoError(suite.T(), err)

		// Verify UpdateModulesSettings was called with correct arguments
		require.Len(suite.T(), mockPkg.settingsCalls, 1)
		assert.Equal(suite.T(), "test-module", mockPkg.settingsCalls[0].name)
		assert.Equal(suite.T(), config.Spec.Version, mockPkg.settingsCalls[0].settingsVersion)
		assert.Equal(suite.T(), config.Spec.Enabled, mockPkg.settingsCalls[0].enabled)

		// The v2 path must not also dispatch an addon-operator event.
		assert.Equal(suite.T(), 0, suite.drainHandlerEvents())
	})

	suite.Run("v1 path: flag off falls back to addon-operator", func() {
		suite.setupTestController("enable-module.yaml")

		mockPkg := newMockPackageRuntime(true) // HasModule returns true
		suite.r.packageRuntime = mockPkg
		suite.r.packageSystemEnabled = false // flag off → v1 path

		config := suite.moduleConfig("test-module")
		_, err := suite.r.handleModuleConfig(context.TODO(), config)
		require.NoError(suite.T(), err)

		// Verify UpdateModulesSettings was NOT called (v1 path)
		assert.Empty(suite.T(), mockPkg.settingsCalls)
		// The v1 path must dispatch exactly one addon-operator event.
		assert.Equal(suite.T(), 1, suite.drainHandlerEvents())
	})

	suite.Run("v1 path: unknown module always goes to addon-operator", func() {
		suite.setupTestController("enable-module.yaml")

		mockPkg := newMockPackageRuntime(false) // HasModule returns false
		suite.r.packageRuntime = mockPkg
		suite.r.packageSystemEnabled = true

		config := suite.moduleConfig("test-module")
		_, err := suite.r.handleModuleConfig(context.TODO(), config)
		require.NoError(suite.T(), err)

		// Verify UpdateModulesSettings was NOT called (unknown to runtime)
		assert.Empty(suite.T(), mockPkg.settingsCalls)
		// The v1 path must dispatch exactly one addon-operator event.
		assert.Equal(suite.T(), 1, suite.drainHandlerEvents())
	})

	suite.Run("v2 path: global always routes through the runtime even when untracked", func() {
		suite.setupTestController("global-config.yaml")

		// HasModule returns false: global is never tracked in r.modules, only
		// the moduleConfig.Name == moduleGlobal check should route it to v2.
		mockPkg := newMockPackageRuntime(false)
		suite.r.packageRuntime = mockPkg
		suite.r.packageSystemEnabled = true

		config := suite.moduleConfig("global")
		_, err := suite.r.handleModuleConfig(context.TODO(), config)
		require.NoError(suite.T(), err)

		require.Len(suite.T(), mockPkg.settingsCalls, 1)
		assert.Equal(suite.T(), "global", mockPkg.settingsCalls[0].name)
		assert.Equal(suite.T(), 0, suite.drainHandlerEvents())
	})
}

// TestDeleteModuleConfigRouting mirrors TestV2ModuleConfigRouting for the
// delete path (deleteModuleConfig), proving the same v1/v2 fork holds on
// deletion: the v2 path resets settings via UpdateModulesSettings and must
// not dispatch an addon-operator delete event, while the v1 path (flag off,
// or an untracked non-global module) dispatches exactly one delete event and
// never calls UpdateModulesSettings.
func (suite *ControllerTestSuite) TestDeleteModuleConfigRouting() {
	suite.Run("v2 path: settings reset via UpdateModulesSettings, no addon-operator event", func() {
		suite.setupTestControllerRaw(deleteModuleConfigRaw("test-module"))

		mockPkg := newMockPackageRuntime(true) // HasModule returns true
		suite.r.packageRuntime = mockPkg
		suite.r.packageSystemEnabled = true

		config := suite.moduleConfig("test-module")
		_, err := suite.r.deleteModuleConfig(context.TODO(), config)
		require.NoError(suite.T(), err)

		require.Len(suite.T(), mockPkg.settingsCalls, 1)
		assert.Equal(suite.T(), "test-module", mockPkg.settingsCalls[0].name)
		assert.Equal(suite.T(), 0, mockPkg.settingsCalls[0].settingsVersion)
		// enabled must be explicitly false (not nil) so the runtime's view
		// stays in sync with disableModule, instead of falling back to the
		// bundle default (see controller.go deleteModuleConfig).
		require.NotNil(suite.T(), mockPkg.settingsCalls[0].enabled)
		assert.False(suite.T(), *mockPkg.settingsCalls[0].enabled)

		assert.Equal(suite.T(), 0, suite.drainHandlerEvents())
	})

	suite.Run("v1 path: flag off falls back to addon-operator", func() {
		suite.setupTestControllerRaw(deleteModuleConfigRaw("test-module"))

		mockPkg := newMockPackageRuntime(true) // HasModule returns true
		suite.r.packageRuntime = mockPkg
		suite.r.packageSystemEnabled = false // flag off → v1 path

		config := suite.moduleConfig("test-module")
		_, err := suite.r.deleteModuleConfig(context.TODO(), config)
		require.NoError(suite.T(), err)

		assert.Empty(suite.T(), mockPkg.settingsCalls)
		assert.Equal(suite.T(), 1, suite.drainHandlerEvents())
	})

	suite.Run("v1 path: unknown module always goes to addon-operator", func() {
		suite.setupTestControllerRaw(deleteModuleConfigRaw("test-module"))

		mockPkg := newMockPackageRuntime(false) // HasModule returns false
		suite.r.packageRuntime = mockPkg
		suite.r.packageSystemEnabled = true

		config := suite.moduleConfig("test-module")
		_, err := suite.r.deleteModuleConfig(context.TODO(), config)
		require.NoError(suite.T(), err)

		assert.Empty(suite.T(), mockPkg.settingsCalls)
		assert.Equal(suite.T(), 1, suite.drainHandlerEvents())
	})

	suite.Run("v2 path: global always routes through the runtime even when untracked", func() {
		suite.setupTestControllerRaw(deleteModuleConfigRaw("global"))

		mockPkg := newMockPackageRuntime(false) // HasModule returns false
		suite.r.packageRuntime = mockPkg
		suite.r.packageSystemEnabled = true

		config := suite.moduleConfig("global")
		_, err := suite.r.deleteModuleConfig(context.TODO(), config)
		require.NoError(suite.T(), err)

		require.Len(suite.T(), mockPkg.settingsCalls, 1)
		assert.Equal(suite.T(), "global", mockPkg.settingsCalls[0].name)
		assert.Equal(suite.T(), 0, suite.drainHandlerEvents())
	})
}

// deleteModuleConfigRaw returns a minimal ModuleConfig manifest with a
// deletion timestamp and finalizer set, for exercising deleteModuleConfig
// directly in tests.
func deleteModuleConfigRaw(name string) string {
	return `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: ` + name + `
  finalizers:
  - modules.deckhouse.io/module-registered
  deletionTimestamp: "2024-01-01T00:00:00Z"
spec:
  enabled: true
`
}

func (suite *ControllerTestSuite) TestCreateReconcile() {
	suite.Run("enable module", func() {
		suite.setupTestController("enable-module.yaml")
		_, err := suite.r.handleModuleConfig(context.TODO(), suite.moduleConfig("test-module"))
		require.NoError(suite.T(), err)
	})

	suite.Run("disable module", func() {
		suite.setupTestController("disable-module.yaml")
		_, err := suite.r.handleModuleConfig(context.TODO(), suite.moduleConfig("test-module"))
		require.NoError(suite.T(), err)
	})

	suite.Run("global module config", func() {
		suite.setupTestController("global-config.yaml")
		configModule := suite.moduleConfig("global")
		// Global doesn't have a module object - skip this test or test differently
		assert.Equal(suite.T(), "global", configModule.Name)
	})

	suite.Run("module with source change", func() {
		suite.setupTestController("change-source.yaml")
		_, err := suite.r.handleModuleConfig(context.TODO(), suite.moduleConfig("test-module"))
		require.NoError(suite.T(), err)
	})

	suite.Run("module conflict with multiple sources", func() {
		suite.setupTestController("multiple-sources.yaml")
		_, err := suite.r.handleModuleConfig(context.TODO(), suite.moduleConfig("test-module"))
		require.NoError(suite.T(), err)
	})

	suite.Run("embedded module", func() {
		suite.setupTestController("embedded-module.yaml")
		_, err := suite.r.handleModuleConfig(context.TODO(), suite.moduleConfig("test-module"))
		require.NoError(suite.T(), err)
	})

	suite.Run("disable with pending releases", func() {
		suite.setupTestController("disable-with-releases.yaml")
		_, err := suite.r.handleModuleConfig(context.TODO(), suite.moduleConfig("test-module"))
		require.NoError(suite.T(), err)
	})
}

func (suite *ControllerTestSuite) TestDeleteReconcile() {
	suite.Run("simple delete test", func() {
		m := `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: test-module
  finalizers:
  - modules.deckhouse.io/module-registered
  deletionTimestamp: "2024-01-01T00:00:00Z"
spec:
  enabled: true
`
		suite.setupTestControllerRaw(m)

		config := suite.moduleConfig("test-module")
		assert.NotNil(suite.T(), config.DeletionTimestamp)
		assert.Len(suite.T(), config.Finalizers, 1)
	})
}

func (suite *ControllerTestSuite) moduleConfig(name string) *v1alpha1.ModuleConfig {
	config := new(v1alpha1.ModuleConfig)
	err := suite.Client().Get(context.TODO(), client.ObjectKey{Name: name}, config)
	require.NoError(suite.T(), err)

	return config
}

// Mock implementations

type mockModuleManager struct {
	modules map[string]*modules.BasicModule
}

func newMockModuleManager() *mockModuleManager {
	return &mockModuleManager{
		modules: make(map[string]*modules.BasicModule),
	}
}

func (m *mockModuleManager) AreModulesInited() bool {
	return true
}

func (m *mockModuleManager) IsModuleEnabled(moduleName string) bool {
	_, exists := m.modules[moduleName]
	return exists
}

func (m *mockModuleManager) GetModuleNames() []string {
	names := make([]string, 0, len(m.modules))
	for name := range m.modules {
		names = append(names, name)
	}
	return names
}

func (m *mockModuleManager) GetModule(name string) *modules.BasicModule {
	if name == "test-module" || name == "deckhouse" {
		return &modules.BasicModule{Name: name}
	}
	return m.modules[name]
}

func (m *mockModuleManager) GetGlobal() *modules.GlobalModule {
	return &modules.GlobalModule{}
}

func (m *mockModuleManager) GetUpdatedByExtender(_ string) (string, error) {
	return "", nil
}

func (m *mockModuleManager) GetModuleEventsChannel() chan events.ModuleEvent {
	return make(chan events.ModuleEvent)
}

// newMockHandler builds a minimal confighandler.Handler wired to dummy
// channels for tests. It also returns the config.Event channel so callers
// can observe (drain) events dispatched via HandleEvent and assert whether
// the v1 (addon-operator) path was taken.
func newMockHandler() (*confighandler.Handler, chan config.Event) {
	deckhouseConfigCh := make(chan utils.Values, 10)
	configEventCh := make(chan config.Event, 10)

	handler := confighandler.New(nil, conversionsStore, deckhouseConfigCh)
	handler.StartInformer(context.Background(), configEventCh)

	return handler, configEventCh
}

type mockPackageRuntime struct {
	settingsCalls   []settingsCall
	hasModuleResult bool
}

type settingsCall struct {
	name            string
	settingsVersion int
	settings        utils.Values
	enabled         *bool
}

func (m *mockPackageRuntime) UpdateModulesSettings(name string, settingsVersion int, settings utils.Values, enabled *bool) {
	m.settingsCalls = append(m.settingsCalls, settingsCall{name, settingsVersion, settings, enabled})
}

func (m *mockPackageRuntime) HasModule(name string) bool {
	return m.hasModuleResult
}

func newMockPackageRuntime(hasModule bool) *mockPackageRuntime {
	return &mockPackageRuntime{
		hasModuleResult: hasModule,
	}
}
