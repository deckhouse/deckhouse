package module_manager

import (
	"encoding/json"
	"fmt"
	"github.com/deckhouse/deckhouse/antiopa/kube_values_manager"
	"github.com/deckhouse/deckhouse/antiopa/utils"
	"github.com/evanphx/json-patch"
	"github.com/romana/rlog"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	Bindings       []BindingType
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
	BeforeHelm      interface{} `json:"beforeHelm"`
	AfterHelm       interface{} `json:"afterHelm"`
	AfterDeleteHelm interface{} `json:"afterDeleteHelm"`
}

type HookConfig struct {
	OnStartup interface{}      `json:"onStartup"`
	Schedule  []ScheduleConfig `json:"schedule"`
}

type ScheduleConfig struct {
	Crontab      string `json:"crontab"`
	AllowFailure bool   `json:"allowFailure"`
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

func addGlobalHook(name, path string, config *GlobalHookConfig) (err error) {
	var ok bool
	globalHook := newGlobalHook()
	globalHook.Name = name
	globalHook.Path = path

	if config.BeforeAll != nil {
		globalHook.Bindings = append(globalHook.Bindings, BeforeAll)
		if globalHook.OrderByBinding[BeforeAll], ok = config.BeforeAll.(float64); !ok {
			return fmt.Errorf("unsuported value '%v' for binding '%s'", config.BeforeAll, BeforeAll)
		}
		globalHooksOrder[BeforeAll] = append(globalHooksOrder[BeforeAll], globalHook)
	}

	if config.AfterAll != nil {
		globalHook.Bindings = append(globalHook.Bindings, AfterAll)
		if globalHook.OrderByBinding[AfterAll], ok = config.AfterAll.(float64); !ok {
			return fmt.Errorf("unsuported value '%v' for binding '%s'", config.AfterAll, AfterAll)
		}
		globalHooksOrder[AfterAll] = append(globalHooksOrder[AfterAll], globalHook)
	}

	if config.OnKubeNodeChange != nil {
		globalHook.Bindings = append(globalHook.Bindings, OnKubeNodeChange)
		if globalHook.OrderByBinding[OnKubeNodeChange], ok = config.OnKubeNodeChange.(float64); !ok {
			return fmt.Errorf("unsuported value '%v' for binding '%s'", config.OnKubeNodeChange, OnKubeNodeChange)
		}
		globalHooksOrder[OnKubeNodeChange] = append(globalHooksOrder[OnKubeNodeChange], globalHook)
	}

	if config.OnStartup != nil {
		globalHook.Bindings = append(globalHook.Bindings, OnStartup)
		if globalHook.OrderByBinding[OnStartup], ok = config.OnStartup.(float64); !ok {
			return fmt.Errorf("unsuported value '%v' for binding '%s'", config.OnStartup, OnStartup)
		}
		globalHooksOrder[OnStartup] = append(globalHooksOrder[OnStartup], globalHook)
	}

	if config.Schedule != nil {
		globalHook.Bindings = append(globalHook.Bindings, Schedule)
		globalHook.Schedules = config.Schedule
		globalHooksOrder[Schedule] = append(globalHooksOrder[Schedule], globalHook)
	}

	globalHooksByName[name] = globalHook

	return nil
}

func addModuleHook(moduleName, name, path string, config *ModuleHookConfig) (err error) {
	var ok bool
	moduleHook := newModuleHook()
	moduleHook.Name = name
	moduleHook.Path = path

	if moduleHook.Module, err = GetModule(moduleName); err != nil {
		return err
	}

	if config.BeforeHelm != nil {
		moduleHook.Bindings = append(moduleHook.Bindings, BeforeHelm)
		if moduleHook.OrderByBinding[BeforeHelm], ok = config.BeforeHelm.(float64); !ok {
			return fmt.Errorf("unsuported value '%v' for binding '%s'", config.BeforeHelm, BeforeHelm)
		}

		addModulesHooksOrderByName(moduleName, BeforeHelm, moduleHook)
	}

	if config.AfterHelm != nil {
		moduleHook.Bindings = append(moduleHook.Bindings, AfterHelm)
		if moduleHook.OrderByBinding[AfterHelm], ok = config.AfterHelm.(float64); !ok {
			return fmt.Errorf("unsuported value '%v' for binding '%s'", config.AfterHelm, AfterHelm)
		}
		addModulesHooksOrderByName(moduleName, AfterHelm, moduleHook)
	}

	if config.AfterDeleteHelm != nil {
		moduleHook.Bindings = append(moduleHook.Bindings, AfterDeleteHelm)
		if moduleHook.OrderByBinding[AfterDeleteHelm], ok = config.AfterDeleteHelm.(float64); !ok {
			return fmt.Errorf("unsuported value '%v' for binding '%s'", config.AfterDeleteHelm, AfterDeleteHelm)
		}
		addModulesHooksOrderByName(moduleName, AfterDeleteHelm, moduleHook)
	}

	if config.OnStartup != nil {
		moduleHook.Bindings = append(moduleHook.Bindings, OnStartup)
		if moduleHook.OrderByBinding[OnStartup], ok = config.OnStartup.(float64); !ok {
			return fmt.Errorf("unsuported value '%v' for binding '%s'", config.OnStartup, OnStartup)
		}
		addModulesHooksOrderByName(moduleName, OnStartup, moduleHook)
	}

	if config.Schedule != nil {
		moduleHook.Bindings = append(moduleHook.Bindings, Schedule)
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

func (h *GlobalHook) run(bindingType BindingType) error {
	rlog.Infof("Running global hook '%s' binding '%s' ...", h.Name, bindingType)

	configVJMV, configVJPV, dynamicVJMV, dynamicVJPV, err := h.exec()
	if err != nil {
		return fmt.Errorf("global hook '%s' failed: %s", h.Name, err)
	}

	var kubeConfigValuesChanged, dynamicValuesChanged bool

	if kubeConfigValues, kubeConfigValuesChanged, err = utils.ApplyJsonMergeAndPatch(kubeConfigValues, configVJMV, configVJPV); err != nil {
		return fmt.Errorf("global hook '%s': merge values failed: %s", h.Name, err)
	}

	if kubeConfigValuesChanged {
		rlog.Debugf("Global hook '%s': updating kubeConfigValues:\n%s", h.Name, valuesToString(kubeConfigValues))
		if err := kube_values_manager.SetKubeValues(kubeConfigValues); err != nil {
			return fmt.Errorf("global hook '%s': set kube values failed: %s", h.Name, err)
		}
	}

	if dynamicValues, dynamicValuesChanged, err = utils.ApplyJsonMergeAndPatch(dynamicValues, dynamicVJMV, dynamicVJPV); err != nil {
		return fmt.Errorf("global hook '%s': merge values failed: %s", h.Name, err)
	}

	if dynamicValuesChanged {
		rlog.Debugf("Global hook '%s': updating dynamicValues:\n%s", h.Name, valuesToString(dynamicValues))
	}

	return nil
}

func (h *GlobalHook) exec() (map[string]interface{}, *jsonpatch.Patch, map[string]interface{}, *jsonpatch.Patch, error) {
	valuesPath, err := h.prepareValuesPath()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	cmd := makeCommand(WorkingDir, valuesPath, h.Path, []string{})
	return execHook(filepath.Join(TempDir, "values", "hooks"), h.Name, cmd)
}

func (h *GlobalHook) prepareValuesPath() (string, error) {
	valuesPath, err := dumpValuesYaml("global-hooks.yaml", h.values())
	if err != nil {
		return "", err
	}
	return valuesPath, nil
}

func (h *GlobalHook) values() utils.Values {
	return utils.MergeValues(globalConfigValues, kubeConfigValues, dynamicValues)
}

func (h *ModuleHook) run(bindingType BindingType) error {
	moduleName := h.Module.Name
	rlog.Infof("Running hook '%s' binding '%s' ...", h.Name, bindingType)

	configVJMV, configVJPV, dynamicVJMV, dynamicVJPV, err := h.exec()
	if err != nil {
		return fmt.Errorf("hook '%s' failed: %s", h.Name, err)
	}

	var kubeModuleConfigValuesChanged, moduleDynamicValuesChanged bool

	if kubeModulesConfigValues[moduleName], kubeModuleConfigValuesChanged, err = utils.ApplyJsonMergeAndPatch(kubeModulesConfigValues[moduleName], configVJMV, configVJPV); err != nil {
		return err
	}

	if kubeModuleConfigValuesChanged {
		rlog.Debugf("Hook '%s': updating kubeModulesConfigValues[%s]:\n%s", h.Name, moduleName, valuesToString(kubeModulesConfigValues[moduleName]))
		err = kube_values_manager.SetModuleKubeValues(moduleName, kubeModulesConfigValues[moduleName])
		if err != nil {
			return fmt.Errorf("hook '%s': set kube values failed: %s", h.Name, err)
		}
	}

	if modulesDynamicValues[moduleName], moduleDynamicValuesChanged, err = utils.ApplyJsonMergeAndPatch(modulesDynamicValues[moduleName], dynamicVJMV, dynamicVJPV); err != nil {
		return fmt.Errorf("hook '%s': merge values failed: %s", h.Name, err)
	}

	if moduleDynamicValuesChanged {
		rlog.Debugf("Hook '%s': updating modulesDynamicValues[%s]:\n%s", h.Name, moduleName, valuesToString(modulesDynamicValues[moduleName]))
	}

	return nil
}

func (h *ModuleHook) exec() (map[string]interface{}, *jsonpatch.Patch, map[string]interface{}, *jsonpatch.Patch, error) {
	valuesPath, err := h.prepareValuesPath()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	cmd := makeCommand(WorkingDir, valuesPath, h.Path, []string{})
	return execHook(filepath.Join(TempDir, "values", "modules"), h.Name, cmd)
}

func (h *ModuleHook) prepareValuesPath() (string, error) {
	return h.Module.prepareValuesPath()
}

func initGlobalHooks() error {
	rlog.Info("Initializing global hooks ...")

	globalHooksOrder = make(map[BindingType][]*GlobalHook)
	globalHooksByName = make(map[string]*GlobalHook)

	hooksDir := filepath.Join(WorkingDir, "global-hooks")

	err := initHooks(hooksDir, func(hookPath string, output []byte) error {
		hookName, err := filepath.Rel(WorkingDir, hookPath)
		if err != nil {
			return err
		}

		rlog.Infof("Initializing global hook '%s' ...", hookName)

		hookConfig := &GlobalHookConfig{}
		if err := json.Unmarshal(output, hookConfig); err != nil {
			return fmt.Errorf("unmarshaling global hook '%s' json failed: %s", hookName, err.Error())
		}

		if err := addGlobalHook(hookName, hookPath, hookConfig); err != nil {
			return fmt.Errorf("adding global hook '%s' failed: %s", hookName, err.Error())
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func initModuleHooks(module *Module) error {
	rlog.Infof("Initializing module '%s' hooks ...", module.Name)

	hooksDir := filepath.Join(module.Path, "hooks")

	err := initHooks(hooksDir, func(hookPath string, output []byte) error {
		hookName, err := filepath.Rel(filepath.Dir(module.Path), hookPath)
		if err != nil {
			return err
		}

		rlog.Infof("Initializing hook '%s' ...", hookName)

		hookConfig := &ModuleHookConfig{}
		if err := json.Unmarshal(output, hookConfig); err != nil {
			return fmt.Errorf("unmarshaling module hook '%s' json failed: %s", hookName, err.Error())
		}

		if err := addModuleHook(module.Name, hookName, hookPath, hookConfig); err != nil {
			return fmt.Errorf("adding module hook '%s' failed: %s", hookName, err.Error())
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func initHooks(hooksDir string, addHook func(hookPath string, output []byte) error) error {
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return nil
	}

	hooksRelativePaths, err := getExecutableFilesPaths(hooksDir) // returns a list of executable hooks sorted by filename
	if err != nil {
		return err
	}

	for _, hookPath := range hooksRelativePaths {
		cmd := makeCommand(WorkingDir, "", hookPath, []string{"--config"})
		output, err := execCommandOutput(cmd)
		if err != nil {
			return err
		}

		if err := addHook(hookPath, output); err != nil {
			return err
		}
	}

	return nil
}

func execHook(tmpDir, hookName string, cmd *exec.Cmd) (map[string]interface{}, *jsonpatch.Patch, map[string]interface{}, *jsonpatch.Patch, error) {
	configValuesJsonMergePath := filepath.Join(tmpDir, hookName, "config_values_json_merge.json")
	if err := createHookResultValuesFile(configValuesJsonMergePath); err != nil {
		return nil, nil, nil, nil, err
	}

	configValuesJsonPatchPath := filepath.Join(tmpDir, hookName, "config_values_json_patch.json")
	if err := createHookResultValuesFile(configValuesJsonPatchPath); err != nil {
		return nil, nil, nil, nil, err
	}

	dynamicValuesJsonMergePath := filepath.Join(tmpDir, hookName, "dynamic_values_json_merge.json")
	if err := createHookResultValuesFile(dynamicValuesJsonMergePath); err != nil {
		return nil, nil, nil, nil, err
	}

	dynamicValuesJsonPatchPath := filepath.Join(tmpDir, hookName, "dynamic_values_json_patch.json")
	if err := createHookResultValuesFile(dynamicValuesJsonPatchPath); err != nil {
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

func createHookResultValuesFile(filePath string) error {
	os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return nil
	}

	file.Close()
	return nil
}

func readValuesJsonFile(filePath string) (map[string]interface{}, error) {
	valuesJson, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %s", filePath, err)
	}

	if len(valuesJson) == 0 {
		return make(map[string]interface{}), nil
	}

	var res map[string]interface{}

	err = json.Unmarshal(valuesJson, &res)
	if err != nil {
		return nil, fmt.Errorf("bad %s: %s", filePath, err)
	}

	return res, nil
}

func readJsonPatchFile(filePath string) (*jsonpatch.Patch, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %s", filePath, err)
	}

	if len(data) == 0 {
		return nil, nil
	}

	patch, err := jsonpatch.DecodePatch(data)
	if err != nil {
		return nil, fmt.Errorf("bad %s: %s", filePath, err)
	}

	return &patch, nil
}

func execCommandOutput(cmd *exec.Cmd) ([]byte, error) {
	rlog.Debugf("Executing command in %s: '%s'", cmd.Dir, strings.Join(cmd.Args, " "))
	cmd.Stdout = nil

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	rlog.Debugf("Command '%s' output:\n%s", strings.Join(cmd.Args, " "), string(output))

	return output, nil
}
