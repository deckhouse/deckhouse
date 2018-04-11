package module_manager

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestGetModule(t *testing.T) {
	mm := &MainModuleManager{}
	mm.modulesByName = make(map[string]*Module)

	expectedModule := &Module{Name: "module"}
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
	mm := &MainModuleManager{}
	mm.modulesHooksByName = make(map[string]*ModuleHook)
	expectedModuleHook := &ModuleHook{Hook: &Hook{Name: "hook"}}
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
	mm := &MainModuleManager{}
	expectedModuleNamesInOrder := []string{"4", "3", "1", "2"}
	mm.allModuleNamesInOrder = expectedModuleNamesInOrder

	if !reflect.DeepEqual(expectedModuleNamesInOrder, mm.allModuleNamesInOrder) {
		t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectedModuleNamesInOrder, mm.allModuleNamesInOrder)
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
	mm := &MainModuleManager{}
	mm.globalHooksByName = make(map[string]*GlobalHook)
	expectedGlobalHook := &GlobalHook{Hook: &Hook{Name: "hook"}}
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
