package module_manager

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/deckhouse/deckhouse/antiopa/helm"
	"github.com/deckhouse/deckhouse/antiopa/utils"
)

type MockHelmClient struct {
	helm.HelmClient
}

func (h MockHelmClient) CommandEnv() []string {
	return []string{}
}

func beforeTest(t *testing.T) {
	_, testFile, _, _ := runtime.Caller(0)
	testDirectory := filepath.Dir(testFile)
	WorkingDir = filepath.Join(testDirectory, "testdata")

	var err error
	TempDir, err = ioutil.TempDir("", "antiopa-")
	if err != nil {
		t.Fatal(err)
	}
}

func TestInit_initModulesIndex(t *testing.T) {
	beforeTest(t)

	mm := &MainModuleManager{}
	mm.helm = MockHelmClient{}

	if err := mm.initModulesIndex(); err != nil {
		t.Fatal(err)
	}

	t.Run("globalConfigValues", globalConfigValues(mm))
	t.Run("globalModulesConfigValues", testGlobalModulesConfigValues(mm))
	t.Run("GetModule", testGetModule(mm))
	t.Run("GetModuleNamesInOrder", testGetModuleNamesInOrder(mm))
	t.Run("GetModuleHook", testGetModuleHook(mm))
	t.Run("GetModuleHooksInOrder", testGetModuleHooksInOrder(mm))
	t.Run("RunModule", testRunModule(mm))
	t.Run("RunModuleHook", testRunModuleHook(mm))
}

func globalConfigValues(mm *MainModuleManager) func(t *testing.T) {
	return func(t *testing.T) {
		expectedValues := utils.Values{
			"a": 1.0,
			"b": 2.0,
			"c": 3.0,
			"d": []interface{}{"a", "b", "c"},
		}

		if !reflect.DeepEqual(mm.globalConfigValues, expectedValues) {
			t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectedValues, mm.globalConfigValues)
		}
	}
}

func testGlobalModulesConfigValues(mm *MainModuleManager) func(t *testing.T) {
	return func(t *testing.T) {
		var expectations = []struct {
			moduleName string
			values     utils.Values
		}{
			{
				moduleName: "with-values-1",
				values:     utils.Values{"a": 1.0, "b": 2.0, "c": 3.0},
			},
			{
				moduleName: "with-values-2",
				values:     utils.Values{"a": []interface{}{1.0, 2.0, map[string]interface{}{"b": 3.0}}},
			},
		}

		for _, expectation := range expectations {
			t.Run(expectation.moduleName, func(t *testing.T) {
				if !reflect.DeepEqual(mm.globalModulesConfigValues[expectation.moduleName], expectation.values) {
					t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectation.values, mm.globalModulesConfigValues[expectation.moduleName])
				}
			})
		}
	}
}

func testGetModule(mm *MainModuleManager) func(t *testing.T) {
	return func(t *testing.T) {
		var expectations = []*Module{
			{
				Name:          "module",
				Path:          filepath.Join(WorkingDir, "modules/000-module"),
				DirectoryName: "000-module",
				moduleManager: mm,
			},
		}

		for _, expectedModule := range expectations {
			t.Run(fmt.Sprintf("%s", expectedModule.Name), func(t *testing.T) {
				module, err := mm.GetModule(expectedModule.Name)
				if err != nil {
					t.Fatal(err)
				}

				if !reflect.DeepEqual(module, expectedModule) {
					t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectedModule, module)
				}
			})
		}
	}
}

func testGetModuleNamesInOrder(mm *MainModuleManager) func(t *testing.T) {
	return func(t *testing.T) {
		expectedModules := []string{
			"module",
			"after-helm-binding-hooks",
			"all-bindings",
			"nested-hooks",
			"with-values-1",
			"with-values-2",
			"update-kube-module-config",
			"update-module-dynamic",
		}

		modulesInOrder := mm.GetModuleNamesInOrder()
		if !reflect.DeepEqual(expectedModules, modulesInOrder) {
			t.Errorf("\n[EXPECTED]: %s\n[GOT]: %s", expectedModules, modulesInOrder)
		}
	}
}

func testGetModuleHook(mm *MainModuleManager) func(t *testing.T) {
	return func(t *testing.T) {
		createModuleHook := func(moduleName, name string, bindings []BindingType, orderByBindings map[BindingType]float64, schedules []ScheduleConfig) *ModuleHook {
			moduleHook := mm.newModuleHook()
			moduleHook.Name = name

			var err error
			if moduleHook.Module, err = mm.GetModule(moduleName); err != nil {
				t.Fatal(err)
			}

			moduleHook.Path = filepath.Join(WorkingDir, "modules", name)
			moduleHook.Schedules = schedules
			moduleHook.Bindings = bindings
			moduleHook.OrderByBinding = orderByBindings

			return moduleHook
		}

		expectations := []struct {
			moduleName     string
			name           string
			bindings       []BindingType
			orderByBinding map[BindingType]float64
			schedule       []ScheduleConfig
		}{
			{
				"all-bindings",
				"111-all-bindings/hooks/all",
				[]BindingType{BeforeHelm, AfterHelm, AfterDeleteHelm, OnStartup, Schedule},
				map[BindingType]float64{
					BeforeHelm:      1,
					AfterHelm:       1,
					AfterDeleteHelm: 1,
					OnStartup:       1,
				},
				[]ScheduleConfig{
					{
						Crontab:      "* * * * *",
						AllowFailure: true,
					},
				},
			},
		}

		for _, exp := range expectations {
			t.Run(exp.name, func(t *testing.T) {
				expectedModuleHook := createModuleHook(exp.moduleName, exp.name, exp.bindings, exp.orderByBinding, exp.schedule)

				moduleHook, err := mm.GetModuleHook(expectedModuleHook.Name)
				if err != nil {
					t.Fatal(err)
				}

				if !reflect.DeepEqual(expectedModuleHook, moduleHook) {
					t.Errorf("\n[EXPECTED]: \n%#v\n[GOT]: \n%#v", expectedModuleHook.Hook, moduleHook.Hook)
				}
			})
		}
	}
}

func testGetModuleHooksInOrder(mm *MainModuleManager) func(t *testing.T) {
	return func(t *testing.T) {
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
}

func testRunModule(mm *MainModuleManager) func(t *testing.T) {
	return func(t *testing.T) {
		t.Skip() // TODO: stub kube_config_manager

		checkModuleNames := []string{"update-kube-module-config", "update-module-dynamic"}

		var kubeModuleConfigValuesExpectations = map[string]utils.Values{
			"update-kube-module-config": {},
			"update-module-dynamic":     {},
		}

		var moduleDynamicValuesExpectations = map[string]utils.Values{
			"update-kube-module-config": {},
			"update-module-dynamic":     {},
		}

		testKubeModulesConfigValues := func(t *testing.T, moduleName string) {
			if kubeModuleConfigValuesExpectations[moduleName] != nil {
				t.Run("kubeModulesConfigValues", func(t *testing.T) {
					if !reflect.DeepEqual(kubeModuleConfigValuesExpectations[moduleName], mm.kubeModulesConfigValues[moduleName]) {
						t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", kubeModuleConfigValuesExpectations[moduleName], mm.kubeModulesConfigValues[moduleName])
					}
				})
			}
		}

		testModulesDynamicValues := func(t *testing.T, moduleName string) {
			if moduleDynamicValuesExpectations[moduleName] != nil {
				t.Run("modulesDynamicValues", func(t *testing.T) {
					if !reflect.DeepEqual(moduleDynamicValuesExpectations[moduleName], mm.modulesDynamicValues[moduleName]) {
						t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", moduleDynamicValuesExpectations[moduleName], mm.modulesDynamicValues[moduleName])
					}
				})
			}
		}

		for _, checkModuleName := range checkModuleNames {
			t.Run(checkModuleName, func(t *testing.T) {
				if err := mm.RunModule(checkModuleName); err != nil {
					t.Fatal(err)
				}

				testKubeModulesConfigValues(t, checkModuleName)
				testModulesDynamicValues(t, checkModuleName)
			})
		}
	}
}

func testRunModuleHook(_ *MainModuleManager) func(t *testing.T) {
	return func(t *testing.T) {
		t.Skip() // TODO: stub kube_config_manager
	}
}

func TestInit_initGlobalHooks(t *testing.T) {
	beforeTest(t)

	mm := &MainModuleManager{}
	mm.helm = MockHelmClient{}
	if err := mm.initGlobalHooks(); err != nil {
		t.Fatal(err)
	}

	t.Run("GetGlobalHook", testGlobalHook(mm))
	t.Run("GetGlobalHooksInOrder", testGetGlobalHooksInOrder(mm))
	t.Run("RunGlobalHook", testRunGlobalHook(mm))
}

func testGlobalHook(mm *MainModuleManager) func(t *testing.T) {
	return func(t *testing.T) {
		createGlobalHook := func(name string, bindings []BindingType, orderByBindings map[BindingType]float64, schedules []ScheduleConfig) *GlobalHook {
			globalHook := mm.newGlobalHook()
			globalHook.moduleManager = mm
			globalHook.Name = name
			globalHook.Path = filepath.Join(WorkingDir, name)
			globalHook.Schedules = schedules
			globalHook.Bindings = bindings
			globalHook.OrderByBinding = orderByBindings

			return globalHook
		}

		expectations := []struct {
			name           string
			bindings       []BindingType
			orderByBinding map[BindingType]float64
			schedule       []ScheduleConfig
		}{
			{
				"global-hooks/111-all-bindings/all",
				[]BindingType{BeforeAll, AfterAll, OnStartup, Schedule},
				map[BindingType]float64{
					BeforeAll: 1,
					AfterAll:  1,
					OnStartup: 1,
				},
				[]ScheduleConfig{
					{
						Crontab:      "* * * * *",
						AllowFailure: true,
					},
				},
			},
		}

		for _, exp := range expectations {
			t.Run(exp.name, func(t *testing.T) {
				expectedGlobalHook := createGlobalHook(exp.name, exp.bindings, exp.orderByBinding, exp.schedule)

				globalHook, err := mm.GetGlobalHook(expectedGlobalHook.Name)
				if err != nil {
					t.Fatal(err)
				}

				if !reflect.DeepEqual(expectedGlobalHook, globalHook) {
					t.Errorf("\n[EXPECTED]: \n%#v\n[GOT]: \n%#v", expectedGlobalHook.Hook, globalHook.Hook)
				}
			})
		}
	}
}

func testGetGlobalHooksInOrder(mm *MainModuleManager) func(t *testing.T) {
	return func(t *testing.T) {
		var expectations = []struct {
			testName    string
			bindingType BindingType
			hooksOrder  []string
		}{
			{
				testName:    "hooks",
				bindingType: AfterAll,
				hooksOrder: []string{
					"global-hooks/111-all-bindings/all",
					"global-hooks/109-before-all-binding-hooks/b",
					"global-hooks/109-before-all-binding-hooks/c",
					"global-hooks/109-before-all-binding-hooks/a",
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
}

func testRunGlobalHook(_ *MainModuleManager) func(t *testing.T) {
	return func(t *testing.T) {
		t.Skip() // TODO: stub kube_config_manager
	}
}
