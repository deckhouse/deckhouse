package module_manager

import (
	"encoding/json"
	"fmt"
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

	moduleManager *MainModuleManager
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

func (mm *MainModuleManager) newGlobalHook() *GlobalHook {
	globalHook := &GlobalHook{}
	globalHook.Hook = mm.newHook()
	return globalHook
}

func (mm *MainModuleManager) newHook() *Hook {
	hook := &Hook{}
	hook.moduleManager = mm
	hook.OrderByBinding = make(map[BindingType]float64)
	return hook
}

func (mm *MainModuleManager) newModuleHook() *ModuleHook {
	moduleHook := &ModuleHook{}
	moduleHook.Hook = mm.newHook()
	return moduleHook
}

func (mm *MainModuleManager) addGlobalHook(name, path string, config *GlobalHookConfig) (err error) {
	var ok bool
	globalHook := mm.newGlobalHook()
	globalHook.Name = name
	globalHook.Path = path

	if config.BeforeAll != nil {
		globalHook.Bindings = append(globalHook.Bindings, BeforeAll)
		if globalHook.OrderByBinding[BeforeAll], ok = config.BeforeAll.(float64); !ok {
			return fmt.Errorf("unsuported value '%v' for binding '%s'", config.BeforeAll, BeforeAll)
		}
		mm.globalHooksOrder[BeforeAll] = append(mm.globalHooksOrder[BeforeAll], globalHook)
	}

	if config.AfterAll != nil {
		globalHook.Bindings = append(globalHook.Bindings, AfterAll)
		if globalHook.OrderByBinding[AfterAll], ok = config.AfterAll.(float64); !ok {
			return fmt.Errorf("unsuported value '%v' for binding '%s'", config.AfterAll, AfterAll)
		}
		mm.globalHooksOrder[AfterAll] = append(mm.globalHooksOrder[AfterAll], globalHook)
	}

	if config.OnKubeNodeChange != nil {
		globalHook.Bindings = append(globalHook.Bindings, OnKubeNodeChange)
		if globalHook.OrderByBinding[OnKubeNodeChange], ok = config.OnKubeNodeChange.(float64); !ok {
			return fmt.Errorf("unsuported value '%v' for binding '%s'", config.OnKubeNodeChange, OnKubeNodeChange)
		}
		mm.globalHooksOrder[OnKubeNodeChange] = append(mm.globalHooksOrder[OnKubeNodeChange], globalHook)
	}

	if config.OnStartup != nil {
		globalHook.Bindings = append(globalHook.Bindings, OnStartup)
		if globalHook.OrderByBinding[OnStartup], ok = config.OnStartup.(float64); !ok {
			return fmt.Errorf("unsuported value '%v' for binding '%s'", config.OnStartup, OnStartup)
		}
		mm.globalHooksOrder[OnStartup] = append(mm.globalHooksOrder[OnStartup], globalHook)
	}

	if config.Schedule != nil {
		globalHook.Bindings = append(globalHook.Bindings, Schedule)
		globalHook.Schedules = config.Schedule
		mm.globalHooksOrder[Schedule] = append(mm.globalHooksOrder[Schedule], globalHook)
	}

	mm.globalHooksByName[name] = globalHook

	return nil
}

func (mm *MainModuleManager) addModuleHook(moduleName, name, path string, config *ModuleHookConfig) (err error) {
	var ok bool
	moduleHook := mm.newModuleHook()
	moduleHook.Name = name
	moduleHook.Path = path

	if moduleHook.Module, err = mm.GetModule(moduleName); err != nil {
		return err
	}

	if config.BeforeHelm != nil {
		moduleHook.Bindings = append(moduleHook.Bindings, BeforeHelm)
		if moduleHook.OrderByBinding[BeforeHelm], ok = config.BeforeHelm.(float64); !ok {
			return fmt.Errorf("unsuported value '%v' for binding '%s'", config.BeforeHelm, BeforeHelm)
		}

		mm.addModulesHooksOrderByName(moduleName, BeforeHelm, moduleHook)
	}

	if config.AfterHelm != nil {
		moduleHook.Bindings = append(moduleHook.Bindings, AfterHelm)
		if moduleHook.OrderByBinding[AfterHelm], ok = config.AfterHelm.(float64); !ok {
			return fmt.Errorf("unsuported value '%v' for binding '%s'", config.AfterHelm, AfterHelm)
		}
		mm.addModulesHooksOrderByName(moduleName, AfterHelm, moduleHook)
	}

	if config.AfterDeleteHelm != nil {
		moduleHook.Bindings = append(moduleHook.Bindings, AfterDeleteHelm)
		if moduleHook.OrderByBinding[AfterDeleteHelm], ok = config.AfterDeleteHelm.(float64); !ok {
			return fmt.Errorf("unsuported value '%v' for binding '%s'", config.AfterDeleteHelm, AfterDeleteHelm)
		}
		mm.addModulesHooksOrderByName(moduleName, AfterDeleteHelm, moduleHook)
	}

	if config.OnStartup != nil {
		moduleHook.Bindings = append(moduleHook.Bindings, OnStartup)
		if moduleHook.OrderByBinding[OnStartup], ok = config.OnStartup.(float64); !ok {
			return fmt.Errorf("unsuported value '%v' for binding '%s'", config.OnStartup, OnStartup)
		}
		mm.addModulesHooksOrderByName(moduleName, OnStartup, moduleHook)
	}

	if config.Schedule != nil {
		moduleHook.Bindings = append(moduleHook.Bindings, Schedule)
		moduleHook.Schedules = config.Schedule
		mm.addModulesHooksOrderByName(moduleName, Schedule, moduleHook)
	}

	mm.modulesHooksByName[name] = moduleHook

	return nil
}

func (mm *MainModuleManager) addModulesHooksOrderByName(moduleName string, bindingType BindingType, moduleHook *ModuleHook) {
	if mm.modulesHooksOrderByName[moduleName] == nil {
		mm.modulesHooksOrderByName[moduleName] = make(map[BindingType][]*ModuleHook)
	}
	mm.modulesHooksOrderByName[moduleName][bindingType] = append(mm.modulesHooksOrderByName[moduleName][bindingType], moduleHook)
}

func (h *GlobalHook) run(bindingType BindingType) error {
	rlog.Infof("Running global hook '%s' binding '%s' ...", h.Name, bindingType)

	configVJMV, configVJPV, dynamicVJMV, dynamicVJPV, err := h.exec()
	if err != nil {
		return fmt.Errorf("global hook '%s' failed: %s", h.Name, err)
	}

	var kubeConfigValuesChanged, dynamicValuesChanged bool

	if h.moduleManager.kubeConfigValues, kubeConfigValuesChanged, err = utils.ApplyJsonMergeAndPatch(h.moduleManager.kubeConfigValues, configVJMV, configVJPV); err != nil {
		return fmt.Errorf("global hook '%s': merge values failed: %s", h.Name, err)
	}

	if kubeConfigValuesChanged {
		rlog.Debugf("Global hook '%s': updating kube config values:\n%s", h.Name, utils.ValuesToString(h.moduleManager.kubeConfigValues))
		if err := h.moduleManager.kubeConfigManager.SetKubeValues(h.moduleManager.kubeConfigValues); err != nil {
			return fmt.Errorf("global hook '%s': set kube config failed: %s", h.Name, err)
		}
	}

	newDynamicValues, dynamicValuesChanged, err := utils.ApplyJsonMergeAndPatch(h.moduleManager.dynamicValues, dynamicVJMV, dynamicVJPV)
	if err != nil {
		return fmt.Errorf("global hook '%s': merge values failed: %s", h.Name, err)
	}
	h.moduleManager.dynamicValues = newDynamicValues

	if dynamicValuesChanged {
		rlog.Debugf("Global hook '%s': dynamic values updated:\n%s", h.Name, utils.ValuesToString(h.moduleManager.dynamicValues))
	}

	return nil
}

func (h *GlobalHook) exec() (map[string]interface{}, *jsonpatch.Patch, map[string]interface{}, *jsonpatch.Patch, error) {
	configValuesPath, err := h.prepareConfigValuesPath()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	dynamicValuesPath, err := h.prepareDynamicValuesPath()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	cmd := h.moduleManager.makeHookCommand(WorkingDir, configValuesPath, dynamicValuesPath, h.Path, []string{})
	return h.moduleManager.execHook(filepath.Join(TempDir, "values", "hooks"), h.Name, cmd)
}

func (h *GlobalHook) prepareConfigValuesPath() (string, error) {
	values := h.configValues()

	rlog.Debugf("Prepared hook %s config values:\n%s", h.Name, utils.ValuesToString(values))

	configValuesPath, err := dumpValuesYaml("global-hooks-config-values.yaml", values)
	if err != nil {
		return "", err
	}
	return configValuesPath, nil
}

func (h *GlobalHook) prepareDynamicValuesPath() (string, error) {
	values := h.dynamicValues()

	rlog.Debugf("Prepared hook %s dynamic values:\n%s", h.Name, utils.ValuesToString(values))

	dynamicValuesPath, err := dumpValuesYaml("global-hooks-dynamic-values.yaml", values)
	if err != nil {
		return "", err
	}
	return dynamicValuesPath, nil
}

func (h *GlobalHook) configValues() utils.Values {
	return utils.MergeValues(h.moduleManager.globalConfigValues, h.moduleManager.kubeConfigValues)
}

func (h *GlobalHook) dynamicValues() utils.Values {
	return h.moduleManager.dynamicValues
}

func (h *ModuleHook) run(bindingType BindingType) error {
	moduleName := h.Module.Name
	rlog.Infof("Running hook '%s' binding '%s' ...", h.Name, bindingType)

	configVJMV, configVJPV, dynamicVJMV, dynamicVJPV, err := h.exec()
	if err != nil {
		return fmt.Errorf("hook '%s' failed: %s", h.Name, err)
	}

	var kubeModuleConfigValuesChanged, moduleDynamicValuesChanged bool

	newModuleConfigValues, kubeModuleConfigValuesChanged, err := utils.ApplyJsonMergeAndPatch(h.moduleManager.kubeModulesConfigValues[moduleName], configVJMV, configVJPV)
	if err != nil {
		return err
	}
	h.moduleManager.kubeModulesConfigValues[moduleName] = newModuleConfigValues

	if kubeModuleConfigValuesChanged {
		rlog.Debugf("Hook '%s': updating kubeModulesConfigValues[%s]:\n%s", h.Name, moduleName, utils.ValuesToString(h.moduleManager.kubeModulesConfigValues[moduleName]))
		err = h.moduleManager.kubeConfigManager.SetModuleKubeValues(moduleName, h.moduleManager.kubeModulesConfigValues[moduleName])
		if err != nil {
			return fmt.Errorf("hook '%s': set kube values failed: %s", h.Name, err)
		}
	}

	newModuleDynamicValues, moduleDynamicValuesChanged, err := utils.ApplyJsonMergeAndPatch(h.moduleManager.modulesDynamicValues[moduleName], dynamicVJMV, dynamicVJPV)
	if err != nil {
		return fmt.Errorf("hook '%s': merge values failed: %s", h.Name, err)
	}
	h.moduleManager.modulesDynamicValues[moduleName] = newModuleDynamicValues

	if moduleDynamicValuesChanged {
		rlog.Debugf("Hook '%s': updating modulesDynamicValues[%s]:\n%s", h.Name, moduleName, utils.ValuesToString(h.moduleManager.modulesDynamicValues[moduleName]))
	}

	return nil
}

func (h *ModuleHook) exec() (map[string]interface{}, *jsonpatch.Patch, map[string]interface{}, *jsonpatch.Patch, error) {
	configValuesPath, err := h.prepareConfigValuesPath()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	dynamicValuesPath, err := h.prepareDynamicValuesPath()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	cmd := h.moduleManager.makeHookCommand(WorkingDir, configValuesPath, dynamicValuesPath, h.Path, []string{})
	return h.moduleManager.execHook(filepath.Join(TempDir, "values", "modules"), h.Name, cmd)
}

func (h *ModuleHook) prepareConfigValuesPath() (string, error) {
	return h.Module.prepareConfigValuesPath()
}

func (h *ModuleHook) prepareDynamicValuesPath() (string, error) {
	return h.Module.prepareDynamicValuesPath()
}

func (mm *MainModuleManager) initGlobalHooks() error {
	rlog.Info("Initializing global hooks ...")

	mm.globalHooksOrder = make(map[BindingType][]*GlobalHook)
	mm.globalHooksByName = make(map[string]*GlobalHook)

	hooksDir := filepath.Join(WorkingDir, "global-hooks")

	err := mm.initHooks(hooksDir, func(hookPath string, output []byte) error {
		hookName, err := filepath.Rel(WorkingDir, hookPath)
		if err != nil {
			return err
		}

		rlog.Infof("Initializing global hook '%s' ...", hookName)

		hookConfig := &GlobalHookConfig{}
		if err := json.Unmarshal(output, hookConfig); err != nil {
			return fmt.Errorf("unmarshaling global hook '%s' json failed: %s", hookName, err.Error())
		}

		if err := mm.addGlobalHook(hookName, hookPath, hookConfig); err != nil {
			return fmt.Errorf("adding global hook '%s' failed: %s", hookName, err.Error())
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func (mm *MainModuleManager) initModuleHooks(module *Module) error {
	rlog.Infof("Initializing module '%s' hooks ...", module.Name)

	hooksDir := filepath.Join(module.Path, "hooks")

	err := mm.initHooks(hooksDir, func(hookPath string, output []byte) error {
		hookName, err := filepath.Rel(filepath.Dir(module.Path), hookPath)
		if err != nil {
			return err
		}

		rlog.Infof("Initializing hook '%s' ...", hookName)

		hookConfig := &ModuleHookConfig{}
		if err := json.Unmarshal(output, hookConfig); err != nil {
			return fmt.Errorf("unmarshaling module hook '%s' json failed: %s", hookName, err.Error())
		}

		if err := mm.addModuleHook(module.Name, hookName, hookPath, hookConfig); err != nil {
			return fmt.Errorf("adding module hook '%s' failed: %s", hookName, err.Error())
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func (mm *MainModuleManager) initHooks(hooksDir string, addHook func(hookPath string, output []byte) error) error {
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return nil
	}

	hooksRelativePaths, err := getExecutableHooksFilesPaths(hooksDir) // returns a list of executable hooks sorted by filename
	if err != nil {
		return err
	}

	for _, hookPath := range hooksRelativePaths {
		cmd := makeCommand(WorkingDir, hookPath, []string{}, []string{"--config"})
		output, err := execCommandOutput(cmd)
		if err != nil {
			return fmt.Errorf("cannot get config for hook '%s': %s", hookPath, err)
		}

		if err := addHook(hookPath, output); err != nil {
			return err
		}
	}

	return nil
}

func (mm *MainModuleManager) execHook(tmpDir, hookName string, cmd *exec.Cmd) (map[string]interface{}, *jsonpatch.Patch, map[string]interface{}, *jsonpatch.Patch, error) {
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

func makeCommand(dir string, entrypoint string, envs []string, args []string) *exec.Cmd {
	envs = append(os.Environ(), envs...)
	return utils.MakeCommand(dir, entrypoint, args, envs)
}

func execCommandOutput(cmd *exec.Cmd) ([]byte, error) {
	rlog.Debugf("Executing command in %s: '%s'", cmd.Dir, strings.Join(cmd.Args, " "))
	cmd.Stdout = nil

	output, err := cmd.Output()
	if err != nil {
		rlog.Errorf("Command '%s' output:\n%s", strings.Join(cmd.Args, " "), string(output))
		return output, err
	}

	rlog.Debugf("Command '%s' output:\n%s", strings.Join(cmd.Args, " "), string(output))

	return output, nil
}

func (mm *MainModuleManager) makeHookCommand(dir string, configValuesPath, dynamicValuesPath string, entrypoint string, args []string) *exec.Cmd {
	envs := make([]string, 0)
	envs = append(envs, fmt.Sprintf("CONFIG_VALUES_PATH=%s", configValuesPath))
	envs = append(envs, fmt.Sprintf("DYNAMIC_VALUES_PATH=%s", dynamicValuesPath))
	return mm.makeCommand(dir, entrypoint, args, envs)
}
