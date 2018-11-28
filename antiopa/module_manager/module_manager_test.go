package module_manager

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestMainModuleManager_GetModule(t *testing.T) {
	mm := NewMainModuleManager(nil, nil)

	expectedModule := &Module{Name: "module"}
	mm.allModulesByName["module"] = expectedModule

	module, err := mm.GetModule("module")
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(expectedModule, module) {
		t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectedModule, module)
	}

	_, err = mm.GetModule("non-exist")
	if err == nil {
		t.Error("Expected error!")
	} else if !strings.HasPrefix(err.Error(), "module 'non-exist' not found") {
		t.Errorf("Got unexpected error: %s", err)
	}
}

func TestMainModuleManager_GetModuleHook(t *testing.T) {
	mm := NewMainModuleManager(nil, nil)

	expectedModuleHook := &ModuleHook{Hook: &Hook{Name: "hook"}}
	mm.modulesHooksByName["hook"] = expectedModuleHook

	moduleHook, err := mm.GetModuleHook("hook")
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(expectedModuleHook, moduleHook) {
		t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectedModuleHook, moduleHook)
	}

	_, err = mm.GetModuleHook("non-exist")
	if err == nil {
		t.Error("Expected error!")
	} else if !strings.HasPrefix(err.Error(), "module hook 'non-exist' not found") {
		t.Errorf("Got unexpected error: %s", err)
	}
}

func TestMainModuleManager_GetModuleHooksInOrder(t *testing.T) {
	mm := NewMainModuleManager(nil, nil)

	mm.allModulesByName = map[string]*Module{"module": {Name: "module"}}
	mm.modulesHooksOrderByName = map[string]map[BindingType][]*ModuleHook{
		"module": {
			BeforeHelm: []*ModuleHook{
				{
					Hook: &Hook{
						Name:           "hook-1",
						OrderByBinding: map[BindingType]float64{BeforeHelm: 3},
					},
				},
				{
					Hook: &Hook{
						Name:           "hook-2",
						OrderByBinding: map[BindingType]float64{BeforeHelm: 1},
					},
				},
				{
					Hook: &Hook{
						Name:           "hook-3",
						OrderByBinding: map[BindingType]float64{BeforeHelm: 2},
					},
				},
			},
		},
	}

	expectations := []struct {
		moduleName                 string
		binding                    BindingType
		expectedModuleHooksInOrder []string
	}{
		{
			"module",
			BeforeHelm,
			[]string{"hook-2", "hook-3", "hook-1"},
		},
		{
			"module",
			AfterHelm,
			[]string{},
		},
	}

	for _, expectation := range expectations {
		t.Run(fmt.Sprintf("(%s, %s)", expectation.moduleName, expectation.binding), func(t *testing.T) {
			moduleHooksInOrder, err := mm.GetModuleHooksInOrder(expectation.moduleName, expectation.binding)

			if err != nil {
				t.Errorf("Got unexpected error: %s", err)
			}

			if !reflect.DeepEqual(expectation.expectedModuleHooksInOrder, moduleHooksInOrder) {
				t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectation.expectedModuleHooksInOrder, moduleHooksInOrder)
			}
		})
	}

	expectation := struct {
		moduleName                 string
		binding                    BindingType
		expectedModuleHooksInOrder []string
	}{
		"non-exist",
		BeforeHelm,
		nil,
	}

	t.Run(fmt.Sprintf("(%s, %s)", "non-exist", BeforeHelm), func(t *testing.T) {
		moduleHooksInOrder, err := mm.GetModuleHooksInOrder(expectation.moduleName, expectation.binding)

		if err.Error() != "module 'non-exist' not found" {
			t.Errorf("Got unexpected error: %s", err)
		}

		if !reflect.DeepEqual(expectation.expectedModuleHooksInOrder, moduleHooksInOrder) {
			t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectation.expectedModuleHooksInOrder, moduleHooksInOrder)
		}
	})
}

func TestMainModuleManager_GetGlobalHook(t *testing.T) {
	mm := NewMainModuleManager(nil, nil)

	expectedGlobalHook := &GlobalHook{Hook: &Hook{Name: "hook"}}
	mm.globalHooksByName["hook"] = expectedGlobalHook

	moduleHook, err := mm.GetGlobalHook("hook")
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(expectedGlobalHook, moduleHook) {
		t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectedGlobalHook, moduleHook)
	}

	_, err = mm.GetGlobalHook("non-exist")
	if err == nil {
		t.Error("Expected error!")
	} else if !strings.HasPrefix(err.Error(), "global hook 'non-exist' not found") {
		t.Errorf("Got unexpected error: %s", err)
	}
}

func TestMainModuleManager_GetGlobalHooksInOrder(t *testing.T) {
	mm := NewMainModuleManager(nil, nil)

	mm.globalHooksOrder = map[BindingType][]*GlobalHook{
		BeforeAll: {
			{
				Hook: &Hook{
					Name:           "hook-1",
					OrderByBinding: map[BindingType]float64{BeforeAll: 3},
				},
			},
			{
				Hook: &Hook{
					Name:           "hook-2",
					OrderByBinding: map[BindingType]float64{BeforeAll: 1},
				},
			},
			{
				Hook: &Hook{
					Name:           "hook-3",
					OrderByBinding: map[BindingType]float64{BeforeAll: 2},
				},
			},
		},
	}

	expectations := []struct {
		binding                    BindingType
		expectedGlobalHooksInOrder []string
	}{
		{
			BeforeAll,
			[]string{"hook-2", "hook-3", "hook-1"},
		},
		{
			AfterAll,
			[]string{},
		},
	}

	for _, expectation := range expectations {
		t.Run(fmt.Sprintf("(%s)", expectation.binding), func(t *testing.T) {
			resGlobalHooksInOrder := mm.GetGlobalHooksInOrder(expectation.binding)
			if !reflect.DeepEqual(expectation.expectedGlobalHooksInOrder, resGlobalHooksInOrder) {
				t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectation.expectedGlobalHooksInOrder, resGlobalHooksInOrder)
			}
		})
	}
}

type mockDiscoverModulesHelmClient struct {
	MockHelmClient
}

func (helm *mockDiscoverModulesHelmClient) ListReleasesNames(_ map[string]string) ([]string, error) {
	return []string{"module-1", "module-2", "module-3", "module-5", "module-6", "module-9"}, nil
}

func TestMainModuleManager_DiscoverModulesState(t *testing.T) {
	mm := NewMainModuleManager(&mockDiscoverModulesHelmClient{}, nil)

	mm.allModulesByName = make(map[string]*Module)
	mm.allModulesByName["module-1"] = &Module{Name: "module-1", DirectoryName: "001-module-1", Path: "some/path/001-module-1"}
	mm.allModulesByName["module-3"] = &Module{Name: "module-3", DirectoryName: "003-module-3", Path: "some/path/003-module-3"}
	mm.allModulesByName["module-4"] = &Module{Name: "module-4", DirectoryName: "004-module-4", Path: "some/path/004-module-4"}
	mm.allModulesByName["module-7"] = &Module{Name: "module-7", DirectoryName: "007-module-7", Path: "some/path/007-module-7"}
	mm.allModulesByName["module-8"] = &Module{Name: "module-8", DirectoryName: "008-module-8", Path: "some/path/008-module-8"}
	mm.allModulesByName["module-9"] = &Module{Name: "module-9", DirectoryName: "009-module-9", Path: "some/path/009-module-9"}
	mm.allModulesNamesInOrder = []string{"module-1", "module-3", "module-4", "module-7", "module-8", "module-9"}
	mm.enabledModulesByConfig = []string{"module-1", "module-4", "module-8"}
	//mm.kubeDisabledModules = []string{"module-3", "module-5", "module-7", "module-9"}

	modulesState, err := mm.DiscoverModulesState()
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual([]string{"module-6", "module-5", "module-2"}, modulesState.ReleasedUnknownModules) {
		t.Errorf("Got unexpected released unknown modules list: %+v", modulesState.ReleasedUnknownModules)
	}

	if !reflect.DeepEqual([]string{"module-9", "module-3"}, modulesState.ModulesToDisable) {
		t.Errorf("Got unexpected released disabled modules list: %+v", modulesState.ModulesToDisable)
	}
}

//
//func checkEnabledModules(mm *MainModuleManager, kubeDisabledModules []string, expectedEnabledModulesList []string) error {
//	enabledModules, err := mm.getEnabledModulesInOrder(kubeDisabledModules)
//	if err != nil {
//		return err
//	}
//
//	if !reflect.DeepEqual(enabledModules, expectedEnabledModulesList) {
//		return fmt.Errorf("Expected %+v enabled modules list, got %+v", expectedEnabledModulesList, enabledModules)
//	}
//
//	return nil
//}
//
//func TestEnabledModules(t *testing.T) {
//	initTempAndWorkingDirectories(t, "test_enabled_modules")
//
//	mm := NewMainModuleManager(&MockHelmClient{}, nil)
//
//	if err := mm.initModulesIndex(); err != nil {
//		t.Fatal(err)
//	}
//
//	if err := checkEnabledModules(mm, []string{}, []string{"alpha", "gamma", "delta", "epsilon", "zeta", "eta"}); err != nil {
//		t.Error(err)
//	}
//
//	if err := checkEnabledModules(mm, []string{"beta"}, []string{"alpha", "gamma", "delta", "epsilon", "zeta", "eta"}); err != nil {
//		t.Error(err)
//	}
//
//	if err := checkEnabledModules(mm, []string{"beta", "eta"}, []string{"alpha", "gamma", "delta", "epsilon", "zeta"}); err != nil {
//		t.Error(err)
//	}
//
//	if err := checkEnabledModules(mm, []string{"beta", "eta", "epsilon"}, []string{"alpha", "gamma", "delta"}); err != nil {
//		t.Error(err)
//	}
//
//	if err := checkEnabledModules(mm, []string{"beta", "eta", "epsilon", "alpha"}, []string{}); err != nil {
//		t.Error(err)
//	}
//
//	if err := checkEnabledModules(mm, []string{"alpha"}, []string{"epsilon", "eta"}); err != nil {
//		t.Error(err)
//	}
//}
