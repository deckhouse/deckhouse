package module_manager

import (
	"fmt"
	"github.com/evanphx/json-patch"
	"os/exec"
	"path/filepath"
)

type GlobalHook struct {
	*Hook
}

type ModuleHook struct {
	*Hook
	Module *Module
}

type Hook struct {
	Name           string
	Path           string
	Binding        []BindingType
	OrderByBinding map[BindingType]float64
	Schedules      []ScheduleConfig
}

type GlobalHookConfig struct {
	HookConfig
	OnKubeNodeChange interface{} `json:"onKubeNodeChange"`
	BeforeAll        interface{} `json:"beforeAll"`
	AfterAll         interface{} `json:"afterAll"`
}

type ModuleHookConfig struct {
	HookConfig
	BeforeHelm interface{} `json:"beforeHelm"`
	AfterHelm  interface{} `json:"afterHelm"`
}

type HookConfig struct {
	OnStartup interface{}      `json:"onStartup"`
	Schedule  []ScheduleConfig `json:"schedule"`
}

type ScheduleConfig struct {
	Crontab      string
	AllowFailure bool
}

func newGlobalHook() *GlobalHook {
	globalHook := &GlobalHook{}
	globalHook.Hook = newHook()
	return globalHook
}

func newHook() *Hook {
	hook := &Hook{}
	hook.OrderByBinding = make(map[BindingType]float64)
	return hook
}

func newModuleHook() *ModuleHook {
	moduleHook := &ModuleHook{}
	moduleHook.Hook = newHook()
	return moduleHook
}

func addGlobalHook(name string, config *GlobalHookConfig) (err error) {
	var ok bool
	globalHook := newGlobalHook()
	globalHook.Name = name

	if config.BeforeAll != nil {
		globalHook.Binding = append(globalHook.Binding, BeforeAll)
		if globalHook.OrderByBinding[BeforeAll], ok = config.BeforeAll.(float64); !ok {
			return fmt.Errorf("unsuported value `%v` for binding `%s`", config.BeforeAll, BeforeAll)
		}
		globalHooksOrder[BeforeAll] = append(globalHooksOrder[BeforeAll], globalHook)
	}

	if config.AfterAll != nil {
		globalHook.Binding = append(globalHook.Binding, AfterAll)
		if globalHook.OrderByBinding[AfterAll], ok = config.AfterAll.(float64); !ok {
			return fmt.Errorf("unsuported value `%v` for binding `%s`", config.AfterAll, AfterAll)
		}
		globalHooksOrder[AfterAll] = append(globalHooksOrder[AfterAll], globalHook)
	}

	if config.OnKubeNodeChange != nil {
		globalHook.Binding = append(globalHook.Binding, OnKubeNodeChange)
		if globalHook.OrderByBinding[OnKubeNodeChange], ok = config.OnKubeNodeChange.(float64); !ok {
			return fmt.Errorf("unsuported value `%v` for binding `%s`", config.OnKubeNodeChange, OnKubeNodeChange)
		}
		globalHooksOrder[OnKubeNodeChange] = append(globalHooksOrder[OnKubeNodeChange], globalHook)
	}

	if config.OnStartup != nil {
		globalHook.Binding = append(globalHook.Binding, OnStartup)
		if globalHook.OrderByBinding[OnStartup], ok = config.OnStartup.(float64); !ok {
			return fmt.Errorf("unsuported value `%v` for binding `%s`", config.OnStartup, OnStartup)
		}
		globalHooksOrder[OnStartup] = append(globalHooksOrder[OnStartup], globalHook)
	}

	if config.Schedule != nil {
		globalHook.Binding = append(globalHook.Binding, Schedule)
		globalHook.Schedules = config.Schedule
		globalHooksOrder[Schedule] = append(globalHooksOrder[Schedule], globalHook)
	}

	globalHooksByName[name] = globalHook

	return nil
}

func addModuleHook(moduleName, name string, config *ModuleHookConfig) (err error) {
	var ok bool
	moduleHook := newModuleHook()
	moduleHook.Name = name
	if moduleHook.Module, err = GetModule(moduleName); err != nil {
		return err
	}

	if config.BeforeHelm != nil {
		moduleHook.Binding = append(moduleHook.Binding, BeforeHelm)
		if moduleHook.OrderByBinding[BeforeHelm], ok = config.BeforeHelm.(float64); !ok {
			return fmt.Errorf("unsuported value `%v` for binding `%s`", config.BeforeHelm, BeforeHelm)
		}

		addModulesHooksOrderByName(moduleName, BeforeHelm, moduleHook)
	}

	if config.AfterHelm != nil {
		moduleHook.Binding = append(moduleHook.Binding, AfterHelm)
		if moduleHook.OrderByBinding[AfterHelm], ok = config.AfterHelm.(float64); !ok {
			return fmt.Errorf("unsuported value `%v` for binding `%s`", config.AfterHelm, AfterHelm)
		}
		addModulesHooksOrderByName(moduleName, AfterHelm, moduleHook)
	}

	if config.OnStartup != nil {
		moduleHook.Binding = append(moduleHook.Binding, OnStartup)
		if moduleHook.OrderByBinding[OnStartup], ok = config.OnStartup.(float64); !ok {
			return fmt.Errorf("unsuported value `%v` for binding `%s`", config.OnStartup, OnStartup)
		}
		addModulesHooksOrderByName(moduleName, OnStartup, moduleHook)
	}

	if config.Schedule != nil {
		moduleHook.Binding = append(moduleHook.Binding, Schedule)
		moduleHook.Schedules = config.Schedule
		addModulesHooksOrderByName(moduleName, Schedule, moduleHook)
	}

	modulesHooksByName[name] = moduleHook

	return nil
}

func addModulesHooksOrderByName(moduleName string, bindingType BindingType, moduleHook *ModuleHook) {
	if modulesHooksOrderByName[moduleName] == nil {
		modulesHooksOrderByName[moduleName] = make(map[BindingType][]*ModuleHook)
	}
	modulesHooksOrderByName[moduleName][bindingType] = append(modulesHooksOrderByName[moduleName][bindingType], moduleHook)
}

func (h *GlobalHook) run() error { return nil }

func (h *GlobalHook) exec(valuesPath string) (map[string]interface{}, *jsonpatch.Patch, map[string]interface{}, *jsonpatch.Patch, error) {
	cmd := makeCommand(WorkingDir, valuesPath, h.Path, []string{})
	return execHook(filepath.Join(TempDir, "values", "hooks"), h.Name, cmd)
}

func (h *ModuleHook) run() error {
	//moduleName := h.Module.Name
	//rlog.Infof("module '%s': running %s hook '%s' ...", moduleName, h.Name)
	//
	//configVJMV, configVJPV, dynamicVJMV, dynamicVJPV, err := h.exec(valuesPath)
	//if err != nil {
	//	return fmt.Errorf("module '%s': hook '%s' FAILED: %s", moduleName, h.Name, err)
	//}
	//
	//var kubeModuleConfigValuesChanged bool
	//if kubeModulesConfigValues[moduleName], kubeModuleConfigValuesChanged, err = merge_values.ApplyJsonMergeAndPatch(kubeModulesConfigValues[moduleName], configVJMV, configVJPV); err != nil {
	//	return err
	//}
	//
	//if kubeModuleConfigValuesChanged {
	//	rlog.Debugf("module '%s': hook '%s': updating VALUES in ConfigMap:\n%s", moduleName, h.Name, valuesToString(kubeModulesConfigValues[moduleName]))
	//	err = kube_values_manager.SetModuleKubeValues(moduleName, kubeModulesConfigValues[moduleName])
	//	if err != nil {
	//		err = fmt.Errorf("module '%s': hook '%s': set kube values error: %s", moduleName, h.Name, err)
	//		return err
	//	}
	//}
	//
	//if modulesDynamicValues[moduleName], _, err = merge_values.ApplyJsonMergeAndPatch(modulesDynamicValues[moduleName], dynamicVJMV, dynamicVJPV); err != nil {
	//	err = fmt.Errorf("module '%s': hook '%s': merge values error: %s", moduleName, h.Name, err)
	//	return err
	//}

	return nil
}

func (h *ModuleHook) exec(valuesPath string) (map[string]interface{}, *jsonpatch.Patch, map[string]interface{}, *jsonpatch.Patch, error) {
	cmd := makeCommand(h.Module.Path, valuesPath, h.Path, []string{})
	return execHook(filepath.Join(TempDir, "values", "modules"), h.Name, cmd)
}

func execHook(tmpDir, hookName string, cmd *exec.Cmd) (map[string]interface{}, *jsonpatch.Patch, map[string]interface{}, *jsonpatch.Patch, error) {
	configValuesJsonMergePath := filepath.Join(tmpDir, hookName, "config_values_json_merge.json")
	if err := createResultFile(configValuesJsonMergePath); err != nil {
		return nil, nil, nil, nil, err
	}

	configValuesJsonPatchPath := filepath.Join(tmpDir, hookName, "config_values_json_patch.json")
	if err := createResultFile(configValuesJsonPatchPath); err != nil {
		return nil, nil, nil, nil, err
	}

	dynamicValuesJsonMergePath := filepath.Join(tmpDir, hookName, "dynamic_values_json_merge.json")
	if err := createResultFile(dynamicValuesJsonMergePath); err != nil {
		return nil, nil, nil, nil, err
	}

	dynamicValuesJsonPatchPath := filepath.Join(tmpDir, hookName, "dynamic_values_json_patch.json")
	if err := createResultFile(dynamicValuesJsonPatchPath); err != nil {
		return nil, nil, nil, nil, err
	}

	cmd.Env = append(
		cmd.Env,
		fmt.Sprintf("CONFIG_VALUES_JSON_MERGE_PATH=%s", configValuesJsonMergePath),
		fmt.Sprintf("CONFIG_VALUES_JSON_PATCH_PATH=%s", configValuesJsonPatchPath),
		fmt.Sprintf("DYNAMIC_VALUES_JSON_MERGE_PATH=%s", dynamicValuesJsonMergePath),
		fmt.Sprintf("DYNAMIC_VALUES_JSON_PATCH_PATH=%s", dynamicValuesJsonPatchPath),
	)

	err := execCommand(cmd)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("%s FAILED: %s", hookName, err)
	}

	configValuesJsonMergeValues, err := readValuesJsonFile(configValuesJsonMergePath)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("got bad values json from hook %s: %s", hookName, err)
	}

	configValuesJsonPatchValues, err := readJsonPatchFile(configValuesJsonPatchPath)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("got bad values json from hook %s: %s", hookName, err)
	}

	dynamicValuesJsonMergeValues, err := readValuesJsonFile(dynamicValuesJsonMergePath)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("got bad values json from hook %s: %s", hookName, err)
	}

	dynamicValuesJsonPatchValues, err := readJsonPatchFile(dynamicValuesJsonPatchPath)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("got bad values json from hook %s: %s", hookName, err)
	}

	return configValuesJsonMergeValues, configValuesJsonPatchValues, dynamicValuesJsonMergeValues, dynamicValuesJsonPatchValues, nil
}
