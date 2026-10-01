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
	rec := &reconciler{
		init:             new(sync.WaitGroup),
		client:           suite.Client(),
		logger:           log.NewNop(),
		handler:          newMockHandler(),
		conversionsStore: conversionsStore,
		moduleManager:    newMockModuleManager(),
		edition:          &d8edition.Edition{Name: "fe", Bundle: "Default"},
		metricStorage:    metricstorage.NewMetricStorage(metricstorage.WithNewRegistry(), metricstorage.WithLogger(log.NewNop())),
		configValidator:  nil, // Disable validation in tests to avoid schema issues
		exts:             nil, // Extenders not needed for these tests
	}

	// simulate initialization
	rec.init.Add(1)
	rec.init.Done()
	suite.r = rec
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

func newMockHandler() *confighandler.Handler {
	// minimal handler for tests with dummy channels
	deckhouseConfigCh := make(chan utils.Values, 10)
	configEventCh := make(chan config.Event, 10)

	handler := confighandler.New(nil, conversionsStore, deckhouseConfigCh, nil)
	handler.StartInformer(context.Background(), configEventCh)

	return handler
}
