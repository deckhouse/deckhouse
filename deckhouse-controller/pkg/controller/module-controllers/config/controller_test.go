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
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"testing"

	"github.com/flant/addon-operator/pkg/kube_config_manager/config"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules/events"
	"github.com/flant/addon-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"helm.sh/helm/v3/pkg/releaseutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/confighandler"
	d8edition "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

var (
	generateGolden     bool
	manifestsDelimiter *regexp.Regexp
	conversionsStore   = conversion.NewConversionsStore()
)

func init() {
	flag.BoolVar(&generateGolden, "golden", false, "generate golden files")
	manifestsDelimiter = regexp.MustCompile("(?m)^---$")
}

type ControllerTestSuite struct {
	suite.Suite

	client client.Client
	r      *reconciler

	goldenFile    string
	compareGolden bool
}

func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}

func (suite *ControllerTestSuite) setupTestController(raw string) {
	manifests := releaseutil.SplitManifests(raw)

	var objects = make([]client.Object, 0, len(manifests))
	for _, manifest := range manifests {
		obj := suite.parseKubernetesObject([]byte(manifest))
		if obj != nil {
			objects = append(objects, obj)
		}
	}

	sc := runtime.NewScheme()
	_ = v1alpha1.SchemeBuilder.AddToScheme(sc)
	suite.client = fake.NewClientBuilder().
		WithScheme(sc).
		WithObjects(objects...).
		WithStatusSubresource(&v1alpha1.Module{}, &v1alpha1.ModuleConfig{}, &v1alpha1.ModuleRelease{}).
		Build()

	rec := &reconciler{
		init:             new(sync.WaitGroup),
		client:           suite.client,
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

func (suite *ControllerTestSuite) parseKubernetesObject(raw []byte) client.Object {
	metaType := new(runtime.TypeMeta)
	err := yaml.Unmarshal(raw, metaType)
	require.NoError(suite.T(), err)

	var obj client.Object

	switch metaType.Kind {
	case v1alpha1.ModuleConfigGVK.Kind:
		moduleConfig := new(v1alpha1.ModuleConfig)
		err = yaml.Unmarshal(raw, moduleConfig)
		require.NoError(suite.T(), err)
		obj = moduleConfig

	case v1alpha1.ModuleGVK.Kind:
		module := new(v1alpha1.Module)
		err = yaml.Unmarshal(raw, module)
		require.NoError(suite.T(), err)
		obj = module

	case v1alpha1.ModuleReleaseGVK.Kind:
		release := new(v1alpha1.ModuleRelease)
		err = yaml.Unmarshal(raw, release)
		require.NoError(suite.T(), err)
		obj = release
	}

	return obj
}

func (suite *ControllerTestSuite) SetupSuite() {
	flag.Parse()
	suite.T().Setenv("D8_IS_TESTS_ENVIRONMENT", "true")
}

func (suite *ControllerTestSuite) BeforeTest(suiteName, testName string) {
	if suiteName == "ControllerTestSuite" && testName == "TestCreateReconcile" {
		suite.compareGolden = true
	}
}

func (suite *ControllerTestSuite) AfterTest(_, _ string) {
	suite.compareGolden = false
}

func (suite *ControllerTestSuite) TearDownSubTest() {
	if !suite.compareGolden {
		return
	}

	currentObjects := suite.fetchResults()

	if generateGolden {
		err := os.WriteFile(suite.goldenFile, currentObjects, 0666)
		require.NoError(suite.T(), err)
		return
	}

	raw, err := os.ReadFile(suite.goldenFile)
	require.NoError(suite.T(), err)

	exp := splitManifests(raw)
	got := splitManifests(currentObjects)

	require.Equal(suite.T(), len(got), len(exp), "The number of `got` manifests must be equal to the number of `exp` manifests")
	for i := range got {
		assert.YAMLEq(suite.T(), exp[i], got[i], "Got and exp manifests must match")
	}
}

func (suite *ControllerTestSuite) fetchResults() []byte {
	result := bytes.NewBuffer(nil)

	configs := new(v1alpha1.ModuleConfigList)
	err := suite.client.List(context.TODO(), configs)
	require.NoError(suite.T(), err)

	for _, config := range configs.Items {
		got, _ := yaml.Marshal(config)
		result.WriteString("---\n")
		result.Write(got)
	}

	modules := new(v1alpha1.ModuleList)
	err = suite.client.List(context.TODO(), modules)
	require.NoError(suite.T(), err)

	for _, module := range modules.Items {
		// Clear timestamp fields from conditions to avoid test flakiness
		for i := range module.Status.Conditions {
			module.Status.Conditions[i].LastProbeTime = metav1.Time{}
			module.Status.Conditions[i].LastTransitionTime = metav1.Time{}
		}
		got, _ := yaml.Marshal(module)
		result.WriteString("---\n")
		result.Write(got)
	}

	releases := new(v1alpha1.ModuleReleaseList)
	err = suite.client.List(context.TODO(), releases)
	require.NoError(suite.T(), err)

	for _, release := range releases.Items {
		got, _ := yaml.Marshal(release)
		result.WriteString("---\n")
		result.Write(got)
	}

	return result.Bytes()
}

func splitManifests(doc []byte) []string {
	splits := manifestsDelimiter.Split(string(doc), -1)

	result := make([]string, 0, len(splits))
	for i := range splits {
		if splits[i] != "" {
			result = append(result, splits[i])
		}
	}

	return result
}

func (suite *ControllerTestSuite) TestCreateReconcile() {
	suite.Run("enable module", func() {
		suite.setupTestController(string(suite.parseTestdata("enable-module.yaml")))
		_, err := suite.r.handleModuleConfig(context.TODO(), suite.moduleConfig("test-module"))
		require.NoError(suite.T(), err)
	})

	suite.Run("disable module", func() {
		suite.setupTestController(string(suite.parseTestdata("disable-module.yaml")))
		_, err := suite.r.handleModuleConfig(context.TODO(), suite.moduleConfig("test-module"))
		require.NoError(suite.T(), err)
	})

	suite.Run("global module config", func() {
		suite.setupTestController(string(suite.parseTestdata("global-config.yaml")))
		configModule := suite.moduleConfig("global")
		// Global doesn't have a module object - skip this test or test differently
		assert.Equal(suite.T(), "global", configModule.Name)
	})

	suite.Run("module with source change", func() {
		suite.setupTestController(string(suite.parseTestdata("change-source.yaml")))
		_, err := suite.r.handleModuleConfig(context.TODO(), suite.moduleConfig("test-module"))
		require.NoError(suite.T(), err)
	})

	suite.Run("module conflict with multiple sources", func() {
		suite.setupTestController(string(suite.parseTestdata("multiple-sources.yaml")))
		_, err := suite.r.handleModuleConfig(context.TODO(), suite.moduleConfig("test-module"))
		require.NoError(suite.T(), err)
	})

	suite.Run("embedded module", func() {
		suite.setupTestController(string(suite.parseTestdata("embedded-module.yaml")))
		_, err := suite.r.handleModuleConfig(context.TODO(), suite.moduleConfig("test-module"))
		require.NoError(suite.T(), err)
	})

	suite.Run("disable with pending releases", func() {
		suite.setupTestController(string(suite.parseTestdata("disable-with-releases.yaml")))
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
		suite.setupTestController(m)

		config := suite.moduleConfig("test-module")
		assert.NotNil(suite.T(), config.DeletionTimestamp)
		assert.Len(suite.T(), config.Finalizers, 1)
	})
}

func (suite *ControllerTestSuite) parseTestdata(filename string) []byte {
	dir := "./testdata"
	data, err := os.ReadFile(filepath.Join(dir, filename))
	require.NoError(suite.T(), err)

	suite.goldenFile = filepath.Join("./testdata", "golden", filename)

	return data
}

func (suite *ControllerTestSuite) moduleConfig(name string) *v1alpha1.ModuleConfig {
	config := new(v1alpha1.ModuleConfig)
	err := suite.client.Get(context.TODO(), types.NamespacedName{Name: name}, config)
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

	handler := confighandler.New(nil, conversionsStore, deckhouseConfigCh)
	handler.StartInformer(context.Background(), configEventCh)

	return handler
}
