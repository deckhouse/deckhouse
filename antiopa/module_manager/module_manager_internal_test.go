package module_manager

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/magiconair/properties/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/antiopa/helm"
	"github.com/deckhouse/deckhouse/antiopa/kube_config_manager"
	"github.com/deckhouse/deckhouse/antiopa/utils"
)

func runInitModulesIndex(t *testing.T, mm *MainModuleManager, subPath string) {
	initTempAndWorkingDirectories(t, filepath.Join("init_modules_index", subPath))

	if err := mm.initModulesIndex(); err != nil {
		t.Fatal(err)
	}
}

func runInitGlobalHooks(t *testing.T, mm *MainModuleManager, subPath string) {
	initTempAndWorkingDirectories(t, filepath.Join("init_global_hooks", subPath))

	if err := mm.initGlobalHooks(); err != nil {
		t.Fatal(err)
	}
}

func initTempAndWorkingDirectories(t *testing.T, subPath string) {
	_, testFile, _, _ := runtime.Caller(0)
	testDirectory := filepath.Dir(testFile)
	WorkingDir = filepath.Join(testDirectory, "testdata", subPath)

	var err error
	TempDir, err = ioutil.TempDir("", "antiopa-")
	if err != nil {
		t.Fatal(err)
	}
}

func TestMainModuleManager_globalStaticValues(t *testing.T) {
	mm := NewMainModuleManager(nil, nil)

	runInitModulesIndex(t, mm, "test_global_static_values")

	expectedValues := utils.Values{
		"global": map[string]interface{}{
			"a": 1.0,
			"b": 2.0,
			"c": 3.0,
			"d": []interface{}{"a", "b", "c"},
		},
	}

	if !reflect.DeepEqual(mm.globalStaticValues, expectedValues) {
		t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectedValues, mm.globalStaticValues)
	}
}

func TestMainModuleManager_modulesStaticValues(t *testing.T) {
	mm := NewMainModuleManager(nil, nil)

	runInitModulesIndex(t, mm, "test_modules_static_values")

	var expectations = []struct {
		moduleName string
		values     utils.Values
	}{
		{
			moduleName: "with-values-1",
			values: utils.Values{
				"withValues1": map[string]interface{}{
					"a": 1.0, "b": 2.0, "c": 3.0,
				},
			},
		},
		{
			moduleName: "with-values-2",
			values: utils.Values{
				"withValues2": map[string]interface{}{
					"a": []interface{}{1.0, 2.0, map[string]interface{}{"b": 3.0}},
				},
			},
		},
	}

	for _, expectation := range expectations {
		t.Run(expectation.moduleName, func(t *testing.T) {
			if !reflect.DeepEqual(mm.modulesStaticValues[expectation.moduleName], expectation.values) {
				t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectation.values, mm.modulesStaticValues[expectation.moduleName])
			}
		})
	}
}

func TestMainModuleManager_GetModule2(t *testing.T) {
	mm := NewMainModuleManager(nil, nil)

	runInitModulesIndex(t, mm, "test_get_module")

	var expectations = []*Module{
		{
			Name:          "module",
			Path:          filepath.Join(WorkingDir, "modules/000-module"),
			DirectoryName: "000-module",
			moduleManager: mm,
		},
	}

	for _, expectation := range expectations {
		t.Run(expectation.Name, func(t *testing.T) {
			module, err := mm.GetModule(expectation.Name)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(module, expectation) {
				t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectation, module)
			}
		})
	}
}

func TestMainModuleManager_EnabledModules(t *testing.T) {
	mm := NewMainModuleManager(&MockHelmClient{}, nil)

	runInitModulesIndex(t, mm, "test_get_module_names_in_order")

	expectedModules := []string{
		"module-c",
		"module-a",
		"module-b",
	}

	modulesState, err := mm.DiscoverModulesState()
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(expectedModules, modulesState.EnabledModules) {
		t.Errorf("\n[EXPECTED]: %s\n[GOT]: %s", expectedModules, modulesState.EnabledModules)
	}
}

func TestMainModuleManager_GetModuleHook2(t *testing.T) {
	mm := NewMainModuleManager(nil, nil)

	runInitModulesIndex(t, mm, "test_get_module_hook")

	createModuleHook := func(moduleName, name string, bindings []BindingType, orderByBindings map[BindingType]interface{}, schedule []ScheduleConfig, onKubernetesEvent []OnKubernetesEventConfig) *ModuleHook {
		config := &ModuleHookConfig{
			HookConfig{
				orderByBindings[OnStartup],
				schedule,
				onKubernetesEvent,
			},
			orderByBindings[BeforeHelm],
			orderByBindings[AfterHelm],
			orderByBindings[AfterDeleteHelm],
		}

		moduleHook := mm.newModuleHook(name, filepath.Join(WorkingDir, "modules", name), config)

		var err error
		if moduleHook.Module, err = mm.GetModule(moduleName); err != nil {
			t.Fatal(err)
		}

		moduleHook.Bindings = bindings
		for k, v := range orderByBindings {
			moduleHook.OrderByBinding[k] = v.(float64)
		}

		return moduleHook
	}

	expectations := []struct {
		moduleName        string
		name              string
		bindings          []BindingType
		orderByBinding    map[BindingType]interface{}
		schedule          []ScheduleConfig
		onKubernetesEvent []OnKubernetesEventConfig
	}{
		{
			"all-bindings",
			"000-all-bindings/hooks/all",
			[]BindingType{BeforeHelm, AfterHelm, AfterDeleteHelm, OnStartup, Schedule, KubeEvents},
			map[BindingType]interface{}{
				BeforeHelm:      1.0,
				AfterHelm:       1.0,
				AfterDeleteHelm: 1.0,
				OnStartup:       1.0,
			},
			[]ScheduleConfig{
				{
					Crontab:      "* * * * *",
					AllowFailure: true,
				},
			},
			[]OnKubernetesEventConfig{
				{
					EventTypes: []OnKubernetesEventType{KubernetesEventOnAdd},
					Kind:       "configmap",
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"component": "component1",
						},
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "tier",
								Operator: "In",
								Values:   []string{"cache"},
							},
						},
					},
					NamespaceSelector: &KubeNamespaceSelector{
						MatchNames: []string{"namespace1"},
						Any:        false,
					},
					JqFilter:     ".items[] | del(.metadata, .field1)",
					AllowFailure: true,
				},
				{
					EventTypes: []OnKubernetesEventType{KubernetesEventOnAdd, KubernetesEventOnUpdate, KubernetesEventOnDelete},
					Kind:       "namespace",
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"component": "component2",
						},
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "tier",
								Operator: "In",
								Values:   []string{"cache"},
							},
						},
					},
					NamespaceSelector: &KubeNamespaceSelector{
						MatchNames: []string{"namespace2"},
						Any:        false,
					},
					JqFilter:     ".items[] | del(.metadata, .field2)",
					AllowFailure: true,
				},
				{
					EventTypes: []OnKubernetesEventType{KubernetesEventOnAdd, KubernetesEventOnUpdate, KubernetesEventOnDelete},
					Kind:       "pod",
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"component": "component3",
						},
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "tier",
								Operator: "In",
								Values:   []string{"cache"},
							},
						},
					},
					NamespaceSelector: &KubeNamespaceSelector{
						MatchNames: nil,
						Any:        true,
					},
					JqFilter:     ".items[] | del(.metadata, .field3)",
					AllowFailure: true,
				},
			},
		},
		{
			"nested-hooks",
			"100-nested-hooks/hooks/sub/sub/nested-before-helm",
			[]BindingType{BeforeHelm},
			map[BindingType]interface{}{
				BeforeHelm: 1.0,
			},
			nil,
			nil,
		},
	}

	for _, expectation := range expectations {
		t.Run(expectation.moduleName, func(t *testing.T) {
			expectedModuleHook := createModuleHook(expectation.moduleName, expectation.name, expectation.bindings, expectation.orderByBinding, expectation.schedule, expectation.onKubernetesEvent)

			moduleHook, err := mm.GetModuleHook(expectedModuleHook.Name)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(expectedModuleHook, moduleHook) {
				t.Errorf("\n[EXPECTED]: \n%#v\n[GOT]: \n%#v", expectedModuleHook, moduleHook)
			}
		})
	}
}

func TestMainModuleManager_GetModuleHooksInOrder2(t *testing.T) {
	mm := NewMainModuleManager(nil, nil)

	runInitModulesIndex(t, mm, "test_get_module_hooks_in_order")

	var expectations = []struct {
		moduleName  string
		bindingType BindingType
		hooksOrder  []string
	}{
		{
			moduleName:  "after-helm-binding-hooks",
			bindingType: AfterHelm,
			hooksOrder: []string{
				"107-after-helm-binding-hooks/hooks/b",
				"107-after-helm-binding-hooks/hooks/c",
				"107-after-helm-binding-hooks/hooks/a",
			},
		},
	}

	for _, expectation := range expectations {
		t.Run(fmt.Sprintf("%s, %s", expectation.moduleName, expectation.bindingType), func(t *testing.T) {
			moduleHooks, err := mm.GetModuleHooksInOrder(expectation.moduleName, expectation.bindingType)

			if err != nil {
				t.Error(err)
			}

			if !reflect.DeepEqual(expectation.hooksOrder, moduleHooks) {
				t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectation.hooksOrder, moduleHooks)
			}
		})
	}
}

type MockHelmClient struct {
	helm.HelmClient
	DeleteSingleFailedRevisionExecuted bool
	UpgradeReleaseExecuted             bool
	DeleteReleaseExecuted              bool
}

func (h *MockHelmClient) DeleteOldFailedRevisions(releaseName string) error {
	return nil
}

func (h *MockHelmClient) ListReleases(_ map[string]string) ([]string, error) {
	return []string{}, nil
}

func (h *MockHelmClient) ListReleasesNames(_ map[string]string) ([]string, error) {
	return []string{}, nil
}

func (h *MockHelmClient) CommandEnv() []string {
	return []string{}
}

func (h *MockHelmClient) TillerNamespace() string {
	return "antiopa"
}

func (h *MockHelmClient) DeleteSingleFailedRevision(_ string) error {
	h.DeleteSingleFailedRevisionExecuted = true
	return nil
}

func (h *MockHelmClient) LastReleaseStatus(_ string) (string, string, error) {
	return "", "", nil
}

func (h *MockHelmClient) IsReleaseExists(_ string) (bool, error) {
	return true, nil
}

func (h *MockHelmClient) GetReleaseValues(_ string) (utils.Values, error) {
	return make(utils.Values), nil
}

func (h *MockHelmClient) UpgradeRelease(_, _ string, _ []string, _ []string, _ string) error {
	h.UpgradeReleaseExecuted = true
	return nil
}

func (h *MockHelmClient) DeleteRelease(_ string) error {
	h.DeleteReleaseExecuted = true
	return nil
}

type MockKubeConfigManager struct {
	kube_config_manager.KubeConfigManager
}

func (kcm MockKubeConfigManager) SetKubeGlobalValues(values utils.Values) error {
	return nil
}

func (kcm MockKubeConfigManager) SetKubeModuleValues(moduleName string, values utils.Values) error {
	return nil
}

func TestMainModuleManager_RunModule(t *testing.T) {
	hc := &MockHelmClient{}

	mm := NewMainModuleManager(hc, MockKubeConfigManager{})

	runInitModulesIndex(t, mm, "test_run_module")

	moduleName := "module"
	expectedModuleValues := utils.Values{
		"global": map[string]interface{}{
			"enabledModules": []string{},
		},
		"module": map[string]interface{}{
			"afterHelm":    "override-value",
			"beforeHelm":   "override-value",
			"replicaCount": 1.0,
			"image": map[string]interface{}{
				"repository": "nginx",
				"tag":        "stable",
				"pullPolicy": "IfNotPresent",
			},
		},
	}

	err := mm.RunModule(moduleName)
	if err != nil {
		t.Fatal(err)
	}

	module, err := mm.GetModule(moduleName)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(expectedModuleValues, module.values()) {
		t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectedModuleValues, module.values())
	}

	assert.Equal(t, hc.DeleteSingleFailedRevisionExecuted, true, "helm.DeleteSingleFailedRevision must be executed!")
	assert.Equal(t, hc.UpgradeReleaseExecuted, true, "helm.UpgradeReleaseExecuted must be executed!")
}

func TestMainModuleManager_DeleteModule(t *testing.T) {
	hc := &MockHelmClient{}

	mm := NewMainModuleManager(hc, MockKubeConfigManager{})

	runInitModulesIndex(t, mm, "test_delete_module")

	moduleName := "module"
	expectedModuleValues := utils.Values{
		"global": map[string]interface{}{
			"enabledModules": []string{},
		},
		"module": map[string]interface{}{
			"afterDeleteHelm": "override-value",
			"replicaCount":    1.0,
			"image": map[string]interface{}{
				"repository": "nginx",
				"tag":        "stable",
				"pullPolicy": "IfNotPresent",
			},
		},
	}

	err := mm.DeleteModule(moduleName)
	if err != nil {
		t.Fatal(err)
	}

	module, err := mm.GetModule(moduleName)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(expectedModuleValues, module.values()) {
		t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectedModuleValues, module.values())
	}

	assert.Equal(t, hc.DeleteReleaseExecuted, true, "helm.DeleteRelease must be executed!")
}

func TestMainModuleManager_RunModuleHook(t *testing.T) {
	mm := NewMainModuleManager(&MockHelmClient{}, MockKubeConfigManager{})

	runInitModulesIndex(t, mm, "test_run_module_hook")

	expectations := []struct {
		testName                   string
		moduleName                 string
		hookName                   string
		kubeModuleConfigValues     utils.Values
		moduleDynamicValuesPatches []utils.ValuesPatch
		expectedModuleConfigValues utils.Values
		expectedModuleValues       utils.Values
	}{
		{
			"merge_and_patch_kube_module_config_values",
			"update-kube-module-config",
			"000-update-kube-module-config/hooks/merge_and_patch_values",
			utils.Values{
				"updateKubeModuleConfig": map[string]interface{}{
					"b": "should-be-deleted",
				},
			},
			[]utils.ValuesPatch{},
			utils.Values{
				"global": map[string]interface{}{},
				"updateKubeModuleConfig": map[string]interface{}{
					"a": 2.0, "c": []interface{}{3.0},
				},
			},
			utils.Values{
				"global": map[string]interface{}{
					"enabledModules": []string{},
				},
				"updateKubeModuleConfig": map[string]interface{}{
					"a": 2.0, "c": []interface{}{3.0},
				},
			},
		},
		{
			"merge_and_patch_module_dynamic_values",
			"update-module-dynamic",
			"100-update-module-dynamic/hooks/merge_and_patch_values",
			utils.Values{},
			[]utils.ValuesPatch{},
			utils.Values{
				"global":              map[string]interface{}{},
				"updateModuleDynamic": map[string]interface{}{},
			},
			utils.Values{
				"global": map[string]interface{}{
					"enabledModules": []string{},
				},
				"updateModuleDynamic": map[string]interface{}{
					"a": 9.0, "c": "10",
				},
			},
		},
		{
			"merge_and_patch_over_existing_kube_module_config_values",
			"update-kube-module-config",
			"000-update-kube-module-config/hooks/merge_and_patch_values",
			utils.Values{
				"updateKubeModuleConfig": map[string]interface{}{
					"a": 1.0, "b": 2.0, "x": "123",
				},
			},
			[]utils.ValuesPatch{},
			utils.Values{
				"global": map[string]interface{}{},
				"updateKubeModuleConfig": map[string]interface{}{
					"a": 2.0, "c": []interface{}{3.0}, "x": "123",
				},
			},
			utils.Values{
				"global": map[string]interface{}{
					"enabledModules": []string{},
				},
				"updateKubeModuleConfig": map[string]interface{}{
					"a": 2.0, "c": []interface{}{3.0}, "x": "123",
				},
			},
		},
		{
			"merge_and_patch_over_existing_module_dynamic_values",
			"update-module-dynamic",
			"100-update-module-dynamic/hooks/merge_and_patch_values",
			utils.Values{},
			[]utils.ValuesPatch{
				*utils.MustValuesPatch(utils.ValuesPatchFromBytes([]byte(`[
{"op": "add", "path": "/updateModuleDynamic/a", "value": 123},
{"op": "add", "path": "/updateModuleDynamic/x", "value": 10}
				]`))),
			},
			utils.Values{
				"global":              map[string]interface{}{},
				"updateModuleDynamic": map[string]interface{}{},
			},
			utils.Values{
				"global": map[string]interface{}{
					"enabledModules": []string{},
				},
				"updateModuleDynamic": map[string]interface{}{
					"a": 9.0, "c": "10", "x": 10.0,
				},
			},
		},
	}

	mm.kubeModulesConfigValues = make(map[string]utils.Values)
	for _, expectation := range expectations {
		t.Run(expectation.testName, func(t *testing.T) {
			mm.kubeModulesConfigValues[expectation.moduleName] = expectation.kubeModuleConfigValues
			mm.modulesDynamicValuesPatches[expectation.moduleName] = expectation.moduleDynamicValuesPatches

			if err := mm.RunModuleHook(expectation.hookName, BeforeHelm); err != nil {
				t.Fatal(err)
			}

			module, err := mm.GetModule(expectation.moduleName)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(expectation.expectedModuleConfigValues, module.configValues()) {
				t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectation.expectedModuleConfigValues, module.configValues())
			}

			if !reflect.DeepEqual(expectation.expectedModuleValues, module.values()) {
				t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectation.expectedModuleValues, module.values())
			}
		})
	}
}

func TestMainModuleManager_GetGlobalHook2(t *testing.T) {
	mm := NewMainModuleManager(nil, nil)

	runInitGlobalHooks(t, mm, "test_get_global_hook")

	createGlobalHook := func(name string, bindings []BindingType, orderByBindings map[BindingType]interface{}, schedule []ScheduleConfig, onKubernetesEvent []OnKubernetesEventConfig) *GlobalHook {
		config := &GlobalHookConfig{
			HookConfig{
				orderByBindings[OnStartup],
				schedule,
				onKubernetesEvent,
			},
			orderByBindings[BeforeAll],
			orderByBindings[AfterAll],
		}

		globalHook := mm.newGlobalHook(name, filepath.Join(WorkingDir, name), config)
		globalHook.Bindings = bindings

		for k, v := range orderByBindings {
			globalHook.OrderByBinding[k] = v.(float64)
		}

		return globalHook
	}

	expectations := []struct {
		name              string
		bindings          []BindingType
		orderByBinding    map[BindingType]interface{}
		schedule          []ScheduleConfig
		onKubernetesEvent []OnKubernetesEventConfig
	}{
		{
			"global-hooks/000-all-bindings/all",
			[]BindingType{BeforeAll, AfterAll, OnStartup, Schedule, KubeEvents},
			map[BindingType]interface{}{
				BeforeAll: 1.0,
				AfterAll:  1.0,
				OnStartup: 1.0,
			},
			[]ScheduleConfig{
				{
					Crontab:      "* * * * *",
					AllowFailure: true,
				},
			},
			[]OnKubernetesEventConfig{
				{
					EventTypes: []OnKubernetesEventType{KubernetesEventOnAdd},
					Kind:       "configmap",
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"component": "component1",
						},
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "tier",
								Operator: "In",
								Values:   []string{"cache"},
							},
						},
					},
					NamespaceSelector: &KubeNamespaceSelector{
						MatchNames: []string{"namespace1"},
						Any:        false,
					},
					JqFilter:     ".items[] | del(.metadata, .field1)",
					AllowFailure: true,
				},
				{
					EventTypes: []OnKubernetesEventType{KubernetesEventOnAdd, KubernetesEventOnUpdate, KubernetesEventOnDelete},
					Kind:       "namespace",
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"component": "component2",
						},
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "tier",
								Operator: "In",
								Values:   []string{"cache"},
							},
						},
					},
					NamespaceSelector: &KubeNamespaceSelector{
						MatchNames: []string{"namespace2"},
						Any:        false,
					},
					JqFilter:     ".items[] | del(.metadata, .field2)",
					AllowFailure: true,
				},
				{
					EventTypes: []OnKubernetesEventType{KubernetesEventOnAdd, KubernetesEventOnUpdate, KubernetesEventOnDelete},
					Kind:       "pod",
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"component": "component3",
						},
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "tier",
								Operator: "In",
								Values:   []string{"cache"},
							},
						},
					},
					NamespaceSelector: &KubeNamespaceSelector{
						MatchNames: nil,
						Any:        true,
					},
					JqFilter:     ".items[] | del(.metadata, .field3)",
					AllowFailure: true,
				},
			},
		},
		{
			"global-hooks/100-nested-hook/sub/sub/nested-before-all",
			[]BindingType{BeforeAll},
			map[BindingType]interface{}{
				BeforeAll: 1.0,
			},
			nil,
			nil,
		},
	}

	for _, exp := range expectations {
		t.Run(exp.name, func(t *testing.T) {
			expectedGlobalHook := createGlobalHook(exp.name, exp.bindings, exp.orderByBinding, exp.schedule, exp.onKubernetesEvent)

			globalHook, err := mm.GetGlobalHook(expectedGlobalHook.Name)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(expectedGlobalHook, globalHook) {
				t.Errorf("\n[EXPECTED]: \n%#v\n[GOT]: \n%#v", expectedGlobalHook, globalHook)
			}
		})
	}
}

func TestMainModuleManager_GetGlobalHooksInOrder2(t *testing.T) {
	mm := NewMainModuleManager(nil, nil)

	runInitGlobalHooks(t, mm, "test_get_global_hooks_in_order")

	var expectations = []struct {
		testName    string
		bindingType BindingType
		hooksOrder  []string
	}{
		{
			testName:    "hooks",
			bindingType: AfterAll,
			hooksOrder: []string{
				"global-hooks/000-before-all-binding-hooks/b",
				"global-hooks/000-before-all-binding-hooks/c",
				"global-hooks/000-before-all-binding-hooks/a",
			},
		},
		{
			testName:    "non-supported-binding-type",
			bindingType: BeforeHelm,
			hooksOrder:  []string{},
		},
	}

	for _, expectation := range expectations {
		t.Run(expectation.testName, func(t *testing.T) {
			globalHooks := mm.GetGlobalHooksInOrder(expectation.bindingType)

			if !reflect.DeepEqual(expectation.hooksOrder, globalHooks) {
				t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectation.hooksOrder, globalHooks)
			}
		})
	}
}

func TestMainModuleManager_RunGlobalHook(t *testing.T) {
	mm := NewMainModuleManager(&MockHelmClient{}, MockKubeConfigManager{})

	runInitGlobalHooks(t, mm, "test_run_global_hook")

	expectations := []struct {
		testName                   string
		hookName                   string
		kubeGlobalConfigValues     utils.Values
		globalDynamicValuesPatches []utils.ValuesPatch
		expectedConfigValues       utils.Values
		expectedValues             utils.Values
	}{
		{
			"merge_and_patch_kube_config_values",
			"global-hooks/000-update-kube-config/merge_and_patch_values",
			utils.Values{
				"global": map[string]interface{}{
					"b": "should-be-deleted",
				},
			},
			[]utils.ValuesPatch{},
			utils.Values{
				"global": map[string]interface{}{
					"a": 2.0, "c": []interface{}{3.0},
				},
			},
			utils.Values{
				"global": map[string]interface{}{
					"a": 2.0, "c": []interface{}{3.0},
				},
			},
		},
		{
			"merge_and_patch_dynamic_values",
			"global-hooks/100-update-dynamic/merge_and_patch_values",
			utils.Values{},
			[]utils.ValuesPatch{},
			utils.Values{
				"global": map[string]interface{}{},
			},
			utils.Values{
				"global": map[string]interface{}{
					"a": 9.0, "c": "10",
				},
			},
		},
		{
			"merge_and_patch_over_existing_kube_config_values",
			"global-hooks/000-update-kube-config/merge_and_patch_values",
			utils.Values{
				"global": map[string]interface{}{
					"a": 1.0, "b": 2.0, "x": "123",
				},
			},
			[]utils.ValuesPatch{},
			utils.Values{
				"global": map[string]interface{}{
					"a": 2.0, "c": []interface{}{3.0}, "x": "123",
				},
			},
			utils.Values{
				"global": map[string]interface{}{
					"a": 2.0, "c": []interface{}{3.0}, "x": "123",
				},
			},
		},
		{
			"merge_and_patch_over_existing_dynamic_values",
			"global-hooks/100-update-dynamic/merge_and_patch_values",
			utils.Values{},
			[]utils.ValuesPatch{
				*utils.MustValuesPatch(utils.ValuesPatchFromBytes([]byte(`[
{"op": "add", "path": "/global/a", "value": 123},
{"op": "add", "path": "/global/x", "value": 10.0}
				]`))),
			},
			utils.Values{
				"global": map[string]interface{}{},
			},
			utils.Values{
				"global": map[string]interface{}{
					"a": 9.0, "c": "10", "x": 10.0,
				},
			},
		},
	}

	for _, expectation := range expectations {
		t.Run(expectation.testName, func(t *testing.T) {
			mm.kubeGlobalConfigValues = expectation.kubeGlobalConfigValues
			mm.globalDynamicValuesPatches = expectation.globalDynamicValuesPatches

			if err := mm.RunGlobalHook(expectation.hookName, BeforeHelm); err != nil {
				t.Fatal(err)
			}

			hook, err := mm.GetGlobalHook(expectation.hookName)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(expectation.expectedConfigValues, hook.configValues()) {
				t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectation.expectedConfigValues, hook.configValues())
			}

			if !reflect.DeepEqual(expectation.expectedValues, hook.values()) {
				t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectation.expectedValues, hook.values())
			}
		})
	}
}
