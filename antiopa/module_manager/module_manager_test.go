package module_manager

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/deckhouse/deckhouse/antiopa/helm"
)

func TestGetModule(t *testing.T) {
	expectedModule := &Module{Name: "module"}
	mm := &MainModuleManager{}
	mm.modulesByName = make(map[string]*Module)
	mm.modulesByName["module"] = expectedModule

	module, err := mm.GetModule("module")
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(expectedModule, module) {
		t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectedModule, module)
	}

	module, err = mm.GetModule("non-exist")
	if err == nil {
		t.Error("Expected error!")
	} else if !strings.HasPrefix(err.Error(), "module 'non-exist' not found") {
		t.Errorf("Got unexpected error: %s", err)
	}
}

func TestGetModuleHook(t *testing.T) {
	expectedModuleHook := &ModuleHook{Hook: &Hook{Name: "hook"}}
	mm := &MainModuleManager{}
	mm.modulesHooksByName = make(map[string]*ModuleHook)
	mm.modulesHooksByName["hook"] = expectedModuleHook

	moduleHook, err := mm.GetModuleHook("hook")
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(expectedModuleHook, moduleHook) {
		t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectedModuleHook, moduleHook)
	}

	moduleHook, err = mm.GetModuleHook("non-exist")
	if err == nil {
		t.Error("Expected error!")
	} else if !strings.HasPrefix(err.Error(), "module hook 'non-exist' not found") {
		t.Errorf("Got unexpected error: %s", err)
	}
}

func TestGetModuleNamesInOrder(t *testing.T) {
	expectedModuleNamesInOrder := []string{"4", "3", "1", "2"}
	mm := &MainModuleManager{}
	mm.allModuleNamesInOrder = expectedModuleNamesInOrder

	moduleNamesInOrder := mm.GetModuleNamesInOrder()

	if !reflect.DeepEqual(expectedModuleNamesInOrder, moduleNamesInOrder) {
		t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectedModuleNamesInOrder, moduleNamesInOrder)
	}
}

func TestGetModuleHooksInOrder(t *testing.T) {
	mm := &MainModuleManager{}
	mm.modulesByName = map[string]*Module{"module": {Name: "module"}}
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

func TestGetGlobalHook(t *testing.T) {
	expectedGlobalHook := &GlobalHook{Hook: &Hook{Name: "hook"}}
	mm := &MainModuleManager{}
	mm.globalHooksByName = make(map[string]*GlobalHook)
	mm.globalHooksByName["hook"] = expectedGlobalHook

	moduleHook, err := mm.GetGlobalHook("hook")
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(expectedGlobalHook, moduleHook) {
		t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectedGlobalHook, moduleHook)
	}

	moduleHook, err = mm.GetGlobalHook("non-exist")
	if err == nil {
		t.Error("Expected error!")
	} else if !strings.HasPrefix(err.Error(), "global hook 'non-exist' not found") {
		t.Errorf("Got unexpected error: %s", err)
	}
}

func TestGetGlobalHooksInOrder(t *testing.T) {
	mm := &MainModuleManager{}
	mm.globalHooksOrder = map[BindingType][]*GlobalHook{
		BeforeHelm: {
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
	}

	expectations := []struct {
		binding                    BindingType
		expectedGlobalHooksInOrder []string
	}{
		{
			BeforeHelm,
			[]string{"hook-2", "hook-3", "hook-1"},
		},
		{
			AfterHelm,
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

func TestModulesToPurgeAndDisableOnInit(t *testing.T) {
	mm := MainModuleManager{}

	releasedModules := []string{"module-1", "module-2", "module-3", "module-5", "module-6", "module-9"}

	mm.modulesByName = make(map[string]*Module)
	mm.modulesByName["module-1"] = &Module{Name: "module-1", DirectoryName: "001-module-1", Path: "some/path/001-module-1"}
	mm.modulesByName["module-3"] = &Module{Name: "module-3", DirectoryName: "003-module-3", Path: "some/path/003-module-3"}
	mm.modulesByName["module-4"] = &Module{Name: "module-4", DirectoryName: "004-module-4", Path: "some/path/004-module-4"}
	mm.modulesByName["module-7"] = &Module{Name: "module-7", DirectoryName: "007-module-7", Path: "some/path/007-module-7"}
	mm.modulesByName["module-8"] = &Module{Name: "module-8", DirectoryName: "008-module-8", Path: "some/path/008-module-8"}
	mm.modulesByName["module-9"] = &Module{Name: "module-9", DirectoryName: "009-module-9", Path: "some/path/009-module-9"}
	mm.allModuleNamesInOrder = []string{"module-1", "module-3", "module-4", "module-7", "module-8", "module-9"}

	kubeDisabledModules := []string{"module-3", "module-5", "module-7", "module-9"}

	toPurge := mm.getReleasedModulesToPurge(releasedModules)
	if !reflect.DeepEqual([]string{"module-2", "module-5", "module-6"}, toPurge) {
		t.Errorf("Got unexpected released modules to purge list: %+v", toPurge)
	}

	toDisable := mm.getReleasedModulesToDisable(releasedModules, kubeDisabledModules)
	if !reflect.DeepEqual([]string{"module-3", "module-9"}, toDisable) {
		t.Errorf("Got unexpected released modules to disable list: %+v", toDisable)
	}
}

func checkEnabledModules(mm *MainModuleManager, kubeDisabledModules []string, expectedEnabledModulesList []string) error {
	enabledModules, err := mm.getEnabledModulesInOrder(kubeDisabledModules)
	if err != nil {
		return err
	}

	if !reflect.DeepEqual(enabledModules, expectedEnabledModulesList) {
		return fmt.Errorf("Expected %+v enabled modules list, got %+v", expectedEnabledModulesList, enabledModules)
	}

	return nil
}

func TestEnabledModules(t *testing.T) {
	_, testFile, _, _ := runtime.Caller(0)
	testDirectory := filepath.Dir(testFile)
	WorkingDir = filepath.Join(testDirectory, "test_enabled_modules")

	var err error
	TempDir, err = ioutil.TempDir("", "antiopa-")
	if err != nil {
		t.Fatal(err)
	}

	mm := &MainModuleManager{}
	mm.helm = &helm.HelmClientProto{}

	if err = mm.initModulesIndex(); err != nil {
		t.Fatal(err)
	}

	if err = checkEnabledModules(mm, []string{}, []string{"alpha", "gamma", "delta", "epsilon", "zeta", "eta"}); err != nil {
		t.Error(err)
	}

	if err = checkEnabledModules(mm, []string{"beta"}, []string{"alpha", "gamma", "delta", "epsilon", "zeta", "eta"}); err != nil {
		t.Error(err)
	}

	if err = checkEnabledModules(mm, []string{"beta", "eta"}, []string{"alpha", "gamma", "delta", "epsilon", "zeta"}); err != nil {
		t.Error(err)
	}

	if err = checkEnabledModules(mm, []string{"beta", "eta", "epsilon"}, []string{"alpha", "gamma", "delta"}); err != nil {
		t.Error(err)
	}

	if err = checkEnabledModules(mm, []string{"beta", "eta", "epsilon", "alpha"}, []string{}); err != nil {
		t.Error(err)
	}

	if err = checkEnabledModules(mm, []string{"alpha"}, []string{"epsilon", "eta"}); err != nil {
		t.Error(err)
	}
}
