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
	"github.com/flant/addon-operator/pkg/module_manager/models/modules/events"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/controller/confighandler"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
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
			&v1alpha2.Module{},
			&v1alpha1.ModulePackage{},
			&v1alpha1.ModulePackageVersion{},
		},
		SnapshotKinds: []schema.GroupVersionKind{
			v1alpha2.SchemeGroupVersion.WithKind("Module"),
			v1alpha1.SchemeGroupVersion.WithKind("ModulePackage"),
			v1alpha1.SchemeGroupVersion.WithKind("ModulePackageVersion"),
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
		packageManager:   &stubPackageManager{},
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
	module, ok := obj.(*v1alpha2.Module)
	if !ok {
		return
	}
	for i := range module.Status.Conditions {
		module.Status.Conditions[i].LastTransitionTime = metav1.Time{}
	}
}

func (suite *ControllerTestSuite) reconcile(name string) {
	_, err := suite.r.Reconcile(context.TODO(), suite.Request(name, ""))
	require.NoError(suite.T(), err)
}

func (suite *ControllerTestSuite) TestCreateReconcile() {
	suite.Run("enable module", func() {
		suite.setupTestController("create-module.yaml")
		suite.reconcile("test-module")
	})

	suite.Run("enable module without settings", func() {
		suite.setupTestController("no-settings.yaml")
		suite.reconcile("no-settings")
	})

	suite.Run("ready module", func() {
		suite.setupTestController("ready-module.yaml")
		// the scheduler runs the module and it has fully converged (phase Ready),
		// which is the only case that yields Summary.State=Ready.
		suite.r.moduleManager.(*mockModuleManager).addReadyModule("test-module")
		suite.reconcile("test-module")
	})

	suite.Run("disable module", func() {
		suite.setupTestController("disable-module.yaml")
		suite.reconcile("test-module")
	})

	suite.Run("global module config", func() {
		suite.setupTestController("global-config.yaml")
		suite.reconcile("global")
	})

	suite.Run("module with source change", func() {
		suite.setupTestController("change-source.yaml")
		suite.reconcile("test-module")
	})

	suite.Run("module conflict with multiple sources", func() {
		suite.setupTestController("multiple-sources.yaml")
		suite.reconcile("test-module")
	})

	suite.Run("embedded module", func() {
		suite.setupTestController("embedded-module.yaml")
		suite.reconcile("test-module")
	})

	suite.Run("disable with draft MPV", func() {
		suite.setupTestController("disable-with-draft-mpv.yaml")
		suite.reconcile("test-module")
	})

	suite.Run("module not found", func() {
		suite.setupTestController("not-found.yaml")
		suite.reconcile("test-module")
	})
}

func (suite *ControllerTestSuite) TestDeleteReconcile() {
	suite.Run("delete module", func() {
		suite.setupTestController("delete-module.yaml")
		suite.reconcile("test-module")

		// the finalizer is removed, so the module is garbage-collected
		module := new(v1alpha2.Module)
		err := suite.Client().Get(context.TODO(), client.ObjectKey{Name: "test-module"}, module)
		assert.True(suite.T(), apierrors.IsNotFound(err))
	})

	suite.Run("delete system module", func() {
		suite.setupTestController("delete-system-module.yaml")
		suite.reconcile("global")

		// system modules are skipped, so the finalizer stays in place
		module := new(v1alpha2.Module)
		err := suite.Client().Get(context.TODO(), client.ObjectKey{Name: "global"}, module)
		require.NoError(suite.T(), err)
		assert.Len(suite.T(), module.Finalizers, 1)
	})

	suite.Run("simple delete test", func() {
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

func (suite *ControllerTestSuite) module(name string) *v1alpha2.Module {
	module := new(v1alpha2.Module)
	err := suite.Client().Get(context.TODO(), client.ObjectKey{Name: name}, module)
	require.NoError(suite.T(), err)

	return module
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
	if bm, ok := m.modules[name]; ok {
		return bm
	}
	if name == "test-module" || name == "deckhouse" {
		return &modules.BasicModule{Name: name}
	}
	return nil
}

// addReadyModule registers a module the scheduler runs and that has fully
// converged: IsModuleEnabled reports it on, and GetModule returns a module in
// the Ready run phase with no hook or module errors.
func (m *mockModuleManager) addReadyModule(name string) {
	bm, err := modules.NewBasicModule(name, "", 0, nil, nil, nil)
	if err != nil {
		panic(err)
	}
	bm.SetPhase(modules.Ready)
	m.modules[name] = bm
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
	deckhouseConfigCh := make(chan addonutils.Values, 10)
	configEventCh := make(chan config.Event, 10)

	handler := confighandler.New(nil, conversionsStore, deckhouseConfigCh)
	handler.StartInformer(context.Background(), configEventCh)

	return handler
}

type stubPackageManager struct{}

func (s *stubPackageManager) UpdateModulesSettings(_ string, _ int, _ addonutils.Values, _ string, _ *bool) {
	// no-op
}
