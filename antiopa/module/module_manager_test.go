package module

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestInitModules(t *testing.T) {
	_, testFile, _, _ := runtime.Caller(0)
	testDirectory := filepath.Dir(testFile)
	WorkingDir = filepath.Join(testDirectory, "testdata")

	if err := InitModules(); err != nil {
		t.Fatal(err)
	}

	t.Run("GetModule", testGetModule)
	t.Run("GetModuleNamesInOrder", testGetModuleNamesInOrder)
	t.Run("GetModuleHook", testGetModuleHook)
	t.Run("GetModuleHooksInOrder", testGetModuleHooksInOrder)
	t.Run("RunModule", testRunModule)
}

func testGetModuleNamesInOrder(t *testing.T) {
	expectedModules := []string{"first-module", "second-module"}
	modulesInOrder := GetModuleNamesInOrder()
	if !reflect.DeepEqual(expectedModules, modulesInOrder) {
		t.Errorf("\n[EXPECTED]: %s\n[GOT]: %s", expectedModules, modulesInOrder)
	}
}

func testGetModuleHooksInOrder(t *testing.T) {
	var expectations = []struct {
		moduleName  string
		bindingType BindingType
		hooks       []string
	}{
		{
			moduleName:  "first-module",
			bindingType: BeforeHelm,
			hooks:       []string{"hook-1"},
		},
		{
			moduleName:  "first-module",
			bindingType: AfterHelm,
			hooks:       []string{"hook-2"},
		},
		{
			moduleName:  "second-module",
			bindingType: BeforeHelm,
			hooks:       []string{"hook-3"},
		},
		{
			moduleName:  "second-module",
			bindingType: AfterHelm,
			hooks:       []string{"hook-4"},
		},
	}

	for _, expectation := range expectations {
		moduleHooks, err := GetModuleHooksInOrder(expectation.moduleName, expectation.bindingType)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(expectation.hooks, moduleHooks) {
			t.Errorf("GetModuleHooksInOrder(%s, %s)\n[EXPECTED]: %s\n[GOT]: %s", expectation.moduleName, expectation.bindingType, expectation.hooks, moduleHooks)
		}
	}
}

func testGetModule(t *testing.T) {
	var expectations = []*Module{
		{
			Name:          "first-module",
			Path:          filepath.Join(WorkingDir, "modules/200-first-module"),
			DirectoryName: "200-first-module",
		},
		{
			Name:          "second-module",
			Path:          filepath.Join(WorkingDir, "modules/300-second-module"),
			DirectoryName: "300-second-module",
		},
	}

	for _, expectedModule := range expectations {
		module, err := GetModule(expectedModule.Name)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(module, expectedModule) {
			t.Errorf("GetModule(%s)\n[EXPECTED]: %s\n[GOT]: %s", expectedModule.Name, expectedModule, module)
		}
	}
}

func testGetModuleHook(t *testing.T) {
	var expectations = []*ModuleHook{}

	for _, expectedModuleHook := range expectations {
		moduleHook, err := GetModuleHook(expectedModuleHook.Name)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(moduleHook, expectedModuleHook) {
			t.Errorf("GetModuleHook(%s)\n[EXPECTED]: %v\n[GOT]: %v", expectedModuleHook.Name, expectedModuleHook, moduleHook)
		}
	}
}

func testRunModule(t *testing.T) {
}
