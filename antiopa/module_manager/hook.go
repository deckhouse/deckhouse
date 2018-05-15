package module_manager

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kennygrant/sanitize"
	"github.com/romana/rlog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/antiopa/executor"
	"github.com/deckhouse/deckhouse/antiopa/utils"
)

type GlobalHook struct {
	*Hook
	Config *GlobalHookConfig
}

type ModuleHook struct {
	*Hook
	Module *Module
	Config *ModuleHookConfig
}

type Hook struct {
	Name           string
	Path           string
	Bindings       []BindingType
	OrderByBinding map[BindingType]float64

	moduleManager *MainModuleManager
}

type GlobalHookConfig struct {
	HookConfig
	BeforeAll interface{} `json:"beforeAll"`
	AfterAll  interface{} `json:"afterAll"`
}

type ModuleHookConfig struct {
	HookConfig
	BeforeHelm      interface{} `json:"beforeHelm"`
	AfterHelm       interface{} `json:"afterHelm"`
	AfterDeleteHelm interface{} `json:"afterDeleteHelm"`
}

type HookConfig struct {
	OnStartup         interface{}               `json:"onStartup"`
	Schedule          []ScheduleConfig          `json:"schedule"`
	OnKubernetesEvent []OnKubernetesEventConfig `json:"onKubernetesEvent"`
}

type ScheduleConfig struct {
	Crontab      string `json:"crontab"`
	AllowFailure bool   `json:"allowFailure"`
}

type OnKubernetesEventType string

const (
	KubernetesEventOnAdd    OnKubernetesEventType = "add"
	KubernetesEventOnUpdate OnKubernetesEventType = "update"
	KubernetesEventOnDelete OnKubernetesEventType = "delete"
)

type OnKubernetesEventConfig struct {
	EventTypes        []OnKubernetesEventType `json:"event"`
	Kind              string                  `json:"kind"`
	Selector          *metav1.LabelSelector   `json:"selector"`
	NamespaceSelector *KubeNamespaceSelector  `json:"namespaceSelector"`
	JqFilter          string                  `json:"jqFilter"`
	AllowFailure      bool                    `json:"allowFailure"`
}

type KubeNamespaceSelector struct {
	MatchNames []string `json:"matchNames"`
	Any        bool     `json:"any"`
}

func (mm *MainModuleManager) newGlobalHook(name, path string, config *GlobalHookConfig) *GlobalHook {
	globalHook := &GlobalHook{}
	globalHook.Hook = mm.newHook(name, path)
	globalHook.Config = config
	return globalHook
}

func (mm *MainModuleManager) newHook(name, path string) *Hook {
	hook := &Hook{}
	hook.moduleManager = mm
	hook.Name = name
	hook.Path = path
	hook.OrderByBinding = make(map[BindingType]float64)
	return hook
}

func (mm *MainModuleManager) newModuleHook(name, path string, config *ModuleHookConfig) *ModuleHook {
	moduleHook := &ModuleHook{}
	moduleHook.Hook = mm.newHook(name, path)
	moduleHook.Config = config
	return moduleHook
}

func (mm *MainModuleManager) addGlobalHook(name, path string, config *GlobalHookConfig) (err error) {
	var ok bool
	globalHook := mm.newGlobalHook(name, path, config)

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

	if config.OnStartup != nil {
		globalHook.Bindings = append(globalHook.Bindings, OnStartup)
		if globalHook.OrderByBinding[OnStartup], ok = config.OnStartup.(float64); !ok {
			return fmt.Errorf("unsuported value '%v' for binding '%s'", config.OnStartup, OnStartup)
		}
		mm.globalHooksOrder[OnStartup] = append(mm.globalHooksOrder[OnStartup], globalHook)
	}

	if len(config.Schedule) != 0 {
		globalHook.Bindings = append(globalHook.Bindings, Schedule)
		mm.globalHooksOrder[Schedule] = append(mm.globalHooksOrder[Schedule], globalHook)
	}

	if len(config.OnKubernetesEvent) != 0 {
		globalHook.Bindings = append(globalHook.Bindings, KubeEvents)
		mm.globalHooksOrder[KubeEvents] = append(mm.globalHooksOrder[KubeEvents], globalHook)
	}

	mm.globalHooksByName[name] = globalHook

	return nil
}

func (mm *MainModuleManager) addModuleHook(moduleName, name, path string, config *ModuleHookConfig) (err error) {
	var ok bool
	moduleHook := mm.newModuleHook(name, path, config)

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

	if len(config.Schedule) != 0 {
		moduleHook.Bindings = append(moduleHook.Bindings, Schedule)
		mm.addModulesHooksOrderByName(moduleName, Schedule, moduleHook)
	}

	if len(config.OnKubernetesEvent) != 0 {
		moduleHook.Bindings = append(moduleHook.Bindings, KubeEvents)
		mm.addModulesHooksOrderByName(moduleName, KubeEvents, moduleHook)
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

type globalValuesMergeResult struct {
	// global values with root "global" key
	Values utils.Values
	// global values under root "global" key
	GlobalValues map[string]interface{}
	// original values patch argument
	ValuesPatch utils.ValuesPatch
	// whether values changed after applying patch
	ValuesChanged bool
}

func (h *GlobalHook) handleGlobalValuesPatch(currentValues utils.Values, valuesPatch utils.ValuesPatch) (*globalValuesMergeResult, error) {
	acceptableKey := "global"

	if err := validateHookValuesPatch(valuesPatch, acceptableKey); err != nil {
		return nil, fmt.Errorf("merge global values failed: %s", err)
	}

	newValuesRaw, valuesChanged, err := utils.ApplyValuesPatch(currentValues, valuesPatch)
	if err != nil {
		return nil, fmt.Errorf("merge global values failed: %s", err)
	}

	result := &globalValuesMergeResult{
		Values:        utils.Values{acceptableKey: make(map[string]interface{})},
		ValuesChanged: valuesChanged,
		ValuesPatch:   valuesPatch,
	}

	if globalValuesRaw, hasKey := newValuesRaw[acceptableKey]; hasKey {
		globalValues, ok := globalValuesRaw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected map at key '%s', got:\n%s", acceptableKey, utils.YamlToString(globalValuesRaw))
		}

		result.Values[acceptableKey] = globalValues
		result.GlobalValues = globalValues
	}

	return result, nil
}

func (h *GlobalHook) run(bindingType BindingType) error {
	rlog.Infof("Running global hook '%s' binding '%s' ...", h.Name, bindingType)

	configValuesPatch, valuesPatch, err := h.exec()
	if err != nil {
		return fmt.Errorf("global hook '%s' failed: %s", h.Name, err)
	}

	if configValuesPatch != nil {
		preparedConfigValues := utils.MergeValues(
			utils.Values{"global": map[string]interface{}{}},
			h.moduleManager.kubeGlobalConfigValues,
		)

		configValuesPatchResult, err := h.handleGlobalValuesPatch(preparedConfigValues, *configValuesPatch)
		if err != nil {
			return fmt.Errorf("global hook '%s': kube config global values update error: %s", h.Name, err)
		}

		if configValuesPatchResult.ValuesChanged {
			if err := h.moduleManager.kubeConfigManager.SetKubeGlobalValues(configValuesPatchResult.Values); err != nil {
				rlog.Debugf("Global hook '%s' kube config global values stay unchanged:\n%s", utils.ValuesToString(h.moduleManager.kubeGlobalConfigValues))
				return fmt.Errorf("global hook '%s': set kube config failed: %s", h.Name, err)
			}

			h.moduleManager.kubeGlobalConfigValues = configValuesPatchResult.Values
			rlog.Debugf("Global hook '%s': kube config global values updated:\n%s", h.Name, utils.ValuesToString(h.moduleManager.kubeGlobalConfigValues))
		}
	}

	if valuesPatch != nil {
		valuesPatchResult, err := h.handleGlobalValuesPatch(h.values(), *valuesPatch)
		if err != nil {
			return fmt.Errorf("global hook '%s': dynamic global values update error: %s", h.Name, err)
		}
		if valuesPatchResult.ValuesChanged {
			h.moduleManager.globalDynamicValuesPatches = utils.AppendValuesPatch(h.moduleManager.globalDynamicValuesPatches, valuesPatchResult.ValuesPatch)
			rlog.Debugf("Global hook '%s': global values updated:\n%s", h.Name, utils.ValuesToString(h.values()))
		}
	}

	return nil
}

func (h *GlobalHook) exec() (*utils.ValuesPatch, *utils.ValuesPatch, error) {
	configValuesPath, err := h.prepareConfigValuesJsonFile()
	if err != nil {
		return nil, nil, err
	}
	valuesPath, err := h.prepareValuesJsonFile()
	if err != nil {
		return nil, nil, err
	}
	cmd := h.moduleManager.makeHookCommand(WorkingDir, configValuesPath, valuesPath, h.Path, []string{}, []string{})

	configValuesPatchPath, err := h.prepareConfigValuesJsonPatchFile()
	if err != nil {
		return nil, nil, err
	}
	valuesPatchPath, err := h.prepareValuesJsonPatchFile()
	if err != nil {
		return nil, nil, err
	}
	return h.moduleManager.execHook(h.Name, configValuesPatchPath, valuesPatchPath, cmd)
}

func (h *GlobalHook) configValues() utils.Values {
	return utils.MergeValues(
		utils.Values{"global": map[string]interface{}{}},
		h.moduleManager.kubeGlobalConfigValues,
	)
}

func (h *GlobalHook) values() utils.Values {
	var err error

	res := utils.MergeValues(
		utils.Values{"global": map[string]interface{}{}},
		h.moduleManager.globalStaticValues,
		h.moduleManager.kubeGlobalConfigValues,
	)

	// Invariant: do not store patches that does not apply
	// Give user error for patches early, after patch receive
	for _, patch := range h.moduleManager.globalDynamicValuesPatches {
		res, _, err = utils.ApplyValuesPatch(res, patch)
		if err != nil {
			panic(err)
		}
	}

	return res
}

func (h *GlobalHook) prepareConfigValuesYamlFile() (string, error) {
	values := h.configValues()

	data := utils.MustDump(utils.DumpValuesYaml(values))
	path := filepath.Join(TempDir, fmt.Sprintf("global-hook-%s-config-values.yaml", h.SafeName()))
	err := dumpData(path, data)
	if err != nil {
		return "", err
	}

	rlog.Debugf("Prepared global hook %s config values:\n%s", h.Name, utils.ValuesToString(values))

	return path, nil
}

func (h *GlobalHook) prepareConfigValuesJsonFile() (string, error) {
	values := h.configValues()

	data := utils.MustDump(utils.DumpValuesJson(values))
	path := filepath.Join(TempDir, fmt.Sprintf("global-hook-%s-config-values.json", h.SafeName()))
	err := dumpData(path, data)
	if err != nil {
		return "", err
	}

	rlog.Debugf("Prepared global hook %s config values:\n%s", h.Name, utils.ValuesToString(values))

	return path, nil
}

func (h *GlobalHook) prepareValuesYamlFile() (string, error) {
	values := h.values()

	data := utils.MustDump(utils.DumpValuesYaml(values))
	path := filepath.Join(TempDir, fmt.Sprintf("global-hook-%s-values.yaml", h.SafeName()))
	err := dumpData(path, data)
	if err != nil {
		return "", err
	}

	rlog.Debugf("Prepared global hook %s values:\n%s", h.Name, utils.ValuesToString(values))

	return path, nil
}

func (h *GlobalHook) prepareValuesJsonFile() (string, error) {
	values := h.values()

	data := utils.MustDump(utils.DumpValuesJson(values))
	path := filepath.Join(TempDir, fmt.Sprintf("global-hook-%s-values.json", h.SafeName()))
	err := dumpData(path, data)
	if err != nil {
		return "", err
	}

	rlog.Debugf("Prepared global hook %s values:\n%s", h.Name, utils.ValuesToString(values))

	return path, nil
}

type moduleValuesMergeResult struct {
	// global values with root ModuleValuesKey key
	Values utils.Values
	// global values under root ModuleValuesKey key
	ModuleValues    map[string]interface{}
	ModuleValuesKey string
	ValuesPatch     utils.ValuesPatch
	ValuesChanged   bool
}

func (h *Hook) SafeName() string {
	return sanitize.BaseName(h.Name)
}

func (h *ModuleHook) handleModuleValuesPatch(currentValues utils.Values, valuesPatch utils.ValuesPatch) (*moduleValuesMergeResult, error) {
	moduleValuesKey := utils.ModuleNameToValuesKey(h.Module.Name)

	if err := validateHookValuesPatch(valuesPatch, moduleValuesKey); err != nil {
		return nil, fmt.Errorf("merge module '%s' values failed: %s", h.Module.Name, err)
	}

	newValuesRaw, valuesChanged, err := utils.ApplyValuesPatch(currentValues, valuesPatch)
	if err != nil {
		return nil, fmt.Errorf("merge module '%s' values failed: %s", h.Module.Name, err)
	}

	result := &moduleValuesMergeResult{
		ModuleValuesKey: moduleValuesKey,
		Values:          utils.Values{moduleValuesKey: make(map[string]interface{})},
		ValuesChanged:   valuesChanged,
		ValuesPatch:     valuesPatch,
	}

	if moduleValuesRaw, hasKey := newValuesRaw[result.ModuleValuesKey]; hasKey {
		moduleValues, ok := moduleValuesRaw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected map at key '%s', got:\n%s", result.ModuleValuesKey, utils.YamlToString(moduleValuesRaw))
		}
		result.Values[result.ModuleValuesKey] = moduleValues
		result.ModuleValues = moduleValues
	}

	return result, nil
}

func validateHookValuesPatch(valuesPatch utils.ValuesPatch, acceptableKey string) error {
	for _, op := range valuesPatch.Operations {
		if op.Op == "replace" {
			return fmt.Errorf("unsupported patch operation '%s': '%s'", op.Op, op.ToString())
		}

		pathParts := strings.Split(op.Path, "/")
		if len(pathParts) > 1 {
			affectedKey := pathParts[1]
			if affectedKey != acceptableKey {
				return fmt.Errorf("unacceptable patch operation path '%s' (only '%s' accepted): '%s'", affectedKey, acceptableKey, op.ToString())
			}
		}
	}

	return nil
}

func (h *ModuleHook) run(bindingType BindingType) error {
	moduleName := h.Module.Name
	rlog.Infof("Running module hook '%s' binding '%s' ...", h.Name, bindingType)

	configValuesPatch, valuesPatch, err := h.exec()
	if err != nil {
		return fmt.Errorf("module hook '%s' failed: %s", h.Name, err)
	}

	if configValuesPatch != nil {
		preparedConfigValues := utils.MergeValues(
			utils.Values{utils.ModuleNameToValuesKey(moduleName): map[string]interface{}{}},
			h.moduleManager.kubeModulesConfigValues[moduleName],
		)

		configValuesPatchResult, err := h.handleModuleValuesPatch(preparedConfigValues, *configValuesPatch)
		if err != nil {
			return fmt.Errorf("module hook '%s': kube module config values update error: %s", h.Name, err)
		}
		if configValuesPatchResult.ValuesChanged {
			err := h.moduleManager.kubeConfigManager.SetKubeModuleValues(moduleName, configValuesPatchResult.Values)
			if err != nil {
				rlog.Debugf("Module hook '%s' kube module config values stay unchanged:\n%s", utils.ValuesToString(h.moduleManager.kubeModulesConfigValues[moduleName]))
				return fmt.Errorf("module hook '%s': set kube module config failed: %s", h.Name, err)
			}

			h.moduleManager.kubeModulesConfigValues[moduleName] = configValuesPatchResult.Values
			rlog.Debugf("Module hook '%s': kube module '%s' config values updated:\n%s", h.Name, moduleName, utils.ValuesToString(h.moduleManager.kubeModulesConfigValues[moduleName]))
		}
	}

	if valuesPatch != nil {
		valuesPatchResult, err := h.handleModuleValuesPatch(h.values(), *valuesPatch)
		if err != nil {
			return fmt.Errorf("module hook '%s': dynamic module values update error: %s", h.Name, err)
		}
		if valuesPatchResult.ValuesChanged {
			h.moduleManager.modulesDynamicValuesPatches[moduleName] = utils.AppendValuesPatch(h.moduleManager.modulesDynamicValuesPatches[moduleName], valuesPatchResult.ValuesPatch)
			rlog.Debugf("Module hook '%s': dynamic module '%s' values updated:\n%s", h.Name, moduleName, utils.ValuesToString(h.values()))
		}
	}

	return nil
}

func (h *ModuleHook) exec() (*utils.ValuesPatch, *utils.ValuesPatch, error) {
	configValuesPath, err := h.prepareConfigValuesJsonFile()
	if err != nil {
		return nil, nil, err
	}
	valuesPath, err := h.prepareValuesJsonFile()
	if err != nil {
		return nil, nil, err
	}
	cmd := h.moduleManager.makeHookCommand(WorkingDir, configValuesPath, valuesPath, h.Path, []string{}, []string{})

	configValuesPatchPath, err := h.prepareConfigValuesJsonPatchFile()
	if err != nil {
		return nil, nil, err
	}
	valuesPatchPath, err := h.prepareValuesJsonPatchFile()
	if err != nil {
		return nil, nil, err
	}

	return h.moduleManager.execHook(h.Name, configValuesPatchPath, valuesPatchPath, cmd)
}

func (h *ModuleHook) configValues() utils.Values {
	return h.Module.configValues()
}

func (h *ModuleHook) values() utils.Values {
	return h.Module.values()
}

func (h *ModuleHook) prepareValuesJsonFile() (string, error) {
	return h.Module.prepareValuesJsonFile()
}

func (h *ModuleHook) prepareValuesYamlFile() (string, error) {
	return h.Module.prepareValuesYamlFile()
}

func (h *ModuleHook) prepareConfigValuesJsonFile() (string, error) {
	return h.Module.prepareConfigValuesJsonFile()
}

func (h *ModuleHook) prepareConfigValuesYamlFile() (string, error) {
	return h.Module.prepareConfigValuesYamlFile()
}

func prepareHookConfig(hookConfig *HookConfig) {
	for i := range hookConfig.OnKubernetesEvent {
		config := &hookConfig.OnKubernetesEvent[i]

		if config.EventTypes == nil {
			config.EventTypes = []OnKubernetesEventType{KubernetesEventOnAdd, KubernetesEventOnUpdate, KubernetesEventOnDelete}
		}

		if config.NamespaceSelector == nil {
			config.NamespaceSelector = &KubeNamespaceSelector{Any: true}
		}
	}
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

		prepareHookConfig(&hookConfig.HookConfig)

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

		prepareHookConfig(&hookConfig.HookConfig)

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

func (h *GlobalHook) prepareConfigValuesJsonPatchFile() (string, error) {
	path := filepath.Join(TempDir, fmt.Sprintf("%s.global-hook-config-values.json-patch", h.SafeName()))
	if err := createHookResultValuesFile(path); err != nil {
		return "", err
	}
	return path, nil
}

func (h *GlobalHook) prepareValuesJsonPatchFile() (string, error) {
	path := filepath.Join(TempDir, fmt.Sprintf("%s.global-hook-values.json-patch", h.SafeName()))
	if err := createHookResultValuesFile(path); err != nil {
		return "", err
	}
	return path, nil
}

func (h *ModuleHook) prepareConfigValuesJsonPatchFile() (string, error) {
	path := filepath.Join(TempDir, fmt.Sprintf("%s.global-hook-config-values.json-patch", h.SafeName()))
	if err := createHookResultValuesFile(path); err != nil {
		return "", err
	}
	return path, nil
}

func (h *ModuleHook) prepareValuesJsonPatchFile() (string, error) {
	path := filepath.Join(TempDir, fmt.Sprintf("%s.global-hook-values.json-patch", h.SafeName()))
	if err := createHookResultValuesFile(path); err != nil {
		return "", err
	}
	return path, nil
}

func (mm *MainModuleManager) execHook(hookName string, configValuesJsonPatchPath string, valuesJsonPatchPath string, cmd *exec.Cmd) (*utils.ValuesPatch, *utils.ValuesPatch, error) {
	cmd.Env = append(
		cmd.Env,
		fmt.Sprintf("CONFIG_VALUES_JSON_PATCH_PATH=%s", configValuesJsonPatchPath),
		fmt.Sprintf("VALUES_JSON_PATCH_PATH=%s", valuesJsonPatchPath),
	)

	err := executor.Run(cmd)
	if err != nil {
		return nil, nil, fmt.Errorf("%s FAILED: %s", hookName, err)
	}

	configValuesPatch, err := utils.ValuesPatchFromFile(configValuesJsonPatchPath)
	if err != nil {
		return nil, nil, fmt.Errorf("got bad config values json patch from hook %s: %s", hookName, err)
	}

	valuesPatch, err := utils.ValuesPatchFromFile(valuesJsonPatchPath)
	if err != nil {
		return nil, nil, fmt.Errorf("got bad values json patch from hook %s: %s", hookName, err)
	}

	return configValuesPatch, valuesPatch, nil
}

func createHookResultValuesFile(filePath string) error {
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return nil
	}

	file.Close()
	return nil
}

func makeCommand(dir string, entrypoint string, envs []string, args []string) *exec.Cmd {
	envs = append(os.Environ(), envs...)
	return utils.MakeCommand(dir, entrypoint, args, envs)
}

func execCommandOutput(cmd *exec.Cmd) ([]byte, error) {
	rlog.Debugf("Executing command in %s: '%s'", cmd.Dir, strings.Join(cmd.Args, " "))
	cmd.Stdout = nil

	output, err := executor.Output(cmd)
	if err != nil {
		rlog.Errorf("Command '%s' output:\n%s", strings.Join(cmd.Args, " "), string(output))
		return output, err
	}

	rlog.Debugf("Command '%s' output:\n%s", strings.Join(cmd.Args, " "), string(output))

	return output, nil
}

func (mm *MainModuleManager) makeHookCommand(dir string, configValuesPath string, valuesPath string, entrypoint string, args []string, envs []string) *exec.Cmd {
	envs = append(envs, fmt.Sprintf("CONFIG_VALUES_PATH=%s", configValuesPath))
	envs = append(envs, fmt.Sprintf("VALUES_PATH=%s", valuesPath))
	return mm.makeCommand(dir, entrypoint, args, envs)
}
