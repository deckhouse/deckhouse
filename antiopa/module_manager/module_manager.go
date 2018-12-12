package module_manager

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/romana/rlog"

	"github.com/deckhouse/deckhouse/antiopa/helm"
	"github.com/deckhouse/deckhouse/antiopa/kube_config_manager"
	"github.com/deckhouse/deckhouse/antiopa/utils"
)

type ModuleManager interface {
	Run()
	DiscoverModulesState() (*ModulesState, error)
	GetModule(name string) (*Module, error)
	GetModuleNamesInOrder() []string
	GetGlobalHook(name string) (*GlobalHook, error)
	GetModuleHook(name string) (*ModuleHook, error)
	GetGlobalHooksInOrder(bindingType BindingType) []string
	GetModuleHooksInOrder(moduleName string, bindingType BindingType) ([]string, error)
	DeleteModule(moduleName string) error
	RunModule(moduleName string, onStartup bool) error
	RunGlobalHook(hookName string, binding BindingType, bindingContext []BindingContext) error
	RunModuleHook(hookName string, binding BindingType, bindingContext []BindingContext) error
	Retry()
}

// All modules are in the right order to run/disable/purge
type ModulesState struct {
	EnabledModules         []string
	ModulesToDisable       []string
	ReleasedUnknownModules []string
}

type MainModuleManager struct {
	// Index of all modules in modules directory
	allModulesByName map[string]*Module

	// Ordered list of all modules names for ordered iterations of allModulesByName
	allModulesNamesInOrder []string

	// List of modules enabled by values.yaml or by kube config
	// This list can be changed on ConfigMap changes
	enabledModulesByConfig []string

	// Результирующий список имен включенных модулей в порядке вызова.
	// С учетом скрипта enabled, kube-config и yaml-файла для модуля.
	// Список меняется во время работы antiopa по мере возникновения событий
	// включения/выключения модулей от kube-config.
	enabledModulesInOrder []string

	globalHooksByName map[string]*GlobalHook        // name -> Hook
	globalHooksOrder  map[BindingType][]*GlobalHook // это что-то внутреннее для быстрого поиска binding -> hooks names in order, можно и по-другому сделать

	modulesHooksByName      map[string]*ModuleHook
	modulesHooksOrderByName map[string]map[BindingType][]*ModuleHook

	// global static values from modules/values.yaml file
	globalStaticValues utils.Values

	// values для всех модулей, для конкретного кластера
	kubeGlobalConfigValues utils.Values
	// values для конкретного модуля, для конкретного кластера
	kubeModulesConfigValues map[string]utils.Values

	// Invariant: do not store patches that does not apply
	// Give user error for patches early, after patch receive

	// values для всех модулей, для конкретного инстанса antiopa-pod
	globalDynamicValuesPatches []utils.ValuesPatch
	// values для конкретного модуля, для конкретного инстанса antiopa-pod
	modulesDynamicValuesPatches map[string][]utils.ValuesPatch

	// Внутреннее событие: изменились values модуля.
	// Обработка -- генерация внешнего Event со всеми связанными модулями для рестарта.
	moduleValuesChanged chan string
	// Внутреннее событие: изменились глобальные values.
	// Обработка -- генерация внешнего Event для глобального рестарта всех модулей.
	globalValuesChanged chan bool

	helm              helm.HelmClient
	kubeConfigManager kube_config_manager.KubeConfigManager

	// Сохранение новых конфигов из kube, на случай ошибки обработки
	moduleConfigsUpdateBeforeAmbiguos kube_config_manager.ModuleConfigs
	retryOnAmbigous                   chan bool
}

var (
	EventCh    chan Event
	WorkingDir string
	TempDir    string
)

// Типы привязок для хуков — то, от чего могут сработать хуки
type BindingType string

const (
	BeforeHelm      BindingType = "BEFORE_HELM"
	AfterHelm       BindingType = "AFTER_HELM"
	AfterDeleteHelm BindingType = "AFTER_DELETE_HELM"
	BeforeAll       BindingType = "BEFORE_ALL"
	AfterAll        BindingType = "AFTER_ALL"
	Schedule        BindingType = "SCHEDULE"
	OnStartup       BindingType = "ON_STARTUP"
	KubeEvents      BindingType = "KUBE_EVENTS"
)

var ContextBindingType = map[BindingType]string{
	BeforeHelm:      "beforeHelm",
	AfterHelm:       "afterHelm",
	AfterDeleteHelm: "afterDeleteHelm",
	BeforeAll:       "beforeAll",
	AfterAll:        "afterAll",
	Schedule:        "schedule",
	OnStartup:       "onStartup",
	KubeEvents:      "onKubernetesEvent",
}

// Additional info from schedule and kube events
type BindingContext struct {
	Binding           string `json:"binding"`
	ResourceEvent     string `json:"resourceEvent,omitempty"`
	ResourceNamespace string `json:"resourceNamespace,omitempty"`
	ResourceKind      string `json:"resourceKind,omitempty"`
	ResourceName      string `json:"resourceName,omitempty"`
}

// Типы событий, отправляемые в Main — либо изменились какие-то модули и нужно
// пройти по списку и запустить/удалить/проапгрейдить модуль,
// либо поменялись глобальные values и нужно перезапустить все модули.
type EventType string

const (
	ModulesChanged EventType = "MODULES_CHANGED"
	GlobalChanged  EventType = "GLOBAL_CHANGED"
	AmbigousState  EventType = "AMBIGOUS_STATE"
)

type ChangeType string

const (
	Enabled  ChangeType = "MODULE_ENABLED"  // модуль включился
	Disabled ChangeType = "MODULE_DISABLED" // модуль выключился, возможно нужно запустить helm delete
	Changed  ChangeType = "MODULE_CHANGED"  // поменялись values, нужен helm upgrade
	Purged   ChangeType = "MODULE_PURGED"   // удалились файлы о модуле, нужно просто удалить helm релиз
)

// Имя модуля и вариант изменения
type ModuleChange struct {
	Name       string
	ChangeType ChangeType
}

// Событие для Main
type Event struct {
	ModulesChanges []ModuleChange
	Type           EventType
}

func Init(workingDir string, tempDir string, helmClient helm.HelmClient) (ModuleManager, error) {
	rlog.Info("Initializing module manager ...")

	TempDir = tempDir
	WorkingDir = workingDir
	EventCh = make(chan Event, 1)

	mm := NewMainModuleManager(helmClient, nil)

	if err := mm.initGlobalHooks(); err != nil {
		return nil, err
	}

	if err := mm.initModulesIndex(); err != nil {
		return nil, err
	}

	kcm, err := kube_config_manager.Init()
	if err != nil {
		return nil, err
	}
	mm.kubeConfigManager = kcm
	kubeConfig := mm.kubeConfigManager.InitialConfig()

	mm.kubeGlobalConfigValues = kubeConfig.Values

	var unknown []utils.ModuleConfig
	mm.enabledModulesByConfig, mm.kubeModulesConfigValues, unknown = mm.calculateEnabledModulesByConfig(kubeConfig.ModuleConfigs)

	for _, config := range unknown {
		rlog.Warnf("MODULE_MANAGER Init: ignore kube config for absent module: \n%s",
			config.String(),
		)
	}

	return mm, nil
}

func NewMainModuleManager(helmClient helm.HelmClient, kubeConfigManager kube_config_manager.KubeConfigManager) *MainModuleManager {
	return &MainModuleManager{
		allModulesByName:            make(map[string]*Module),
		allModulesNamesInOrder:      make([]string, 0),
		enabledModulesByConfig:      make([]string, 0),
		enabledModulesInOrder:       make([]string, 0),
		globalHooksByName:           make(map[string]*GlobalHook),
		globalHooksOrder:            make(map[BindingType][]*GlobalHook),
		modulesHooksByName:          make(map[string]*ModuleHook),
		modulesHooksOrderByName:     make(map[string]map[BindingType][]*ModuleHook),
		globalStaticValues:          make(utils.Values),
		kubeGlobalConfigValues:      make(utils.Values),
		kubeModulesConfigValues:     make(map[string]utils.Values),
		globalDynamicValuesPatches:  make([]utils.ValuesPatch, 0),
		modulesDynamicValuesPatches: make(map[string][]utils.ValuesPatch),

		moduleValuesChanged: make(chan string, 1),
		globalValuesChanged: make(chan bool, 1),

		helm:              helmClient,
		kubeConfigManager: kubeConfigManager,

		moduleConfigsUpdateBeforeAmbiguos: make(kube_config_manager.ModuleConfigs),
		retryOnAmbigous:                   make(chan bool, 1),
	}
}

// determineEnableStateWithScript runs enable script for each module that is enabled by config.
// Enable script receives a list of previously enabled modules.
func (mm *MainModuleManager) determineEnableStateWithScript(enabledByConfig []string) ([]string, error) {
	enabledModules := make([]string, 0)
	//rlog.Infof("Run enable scripts for modules list: %s", enabledByConfig)

	for _, name := range utils.SortByReference(enabledByConfig, mm.allModulesNamesInOrder) {
		module := mm.allModulesByName[name]
		moduleIsEnabled, err := module.checkIsEnabledByScript(enabledModules)
		if err != nil {
			return nil, err
		}

		if moduleIsEnabled {
			enabledModules = append(enabledModules, name)
		}
	}

	//rlog.Info("Modules enabled with script: %s", enabledModules)
	return enabledModules, nil
}

type kubeUpdate struct {
	EnabledModulesByConfig  []string
	KubeGlobalConfigValues  utils.Values
	KubeModulesConfigValues map[string]utils.Values
	Events                  []Event
}

func (mm *MainModuleManager) applyKubeUpdate(kubeUpdate *kubeUpdate) error {
	rlog.Debugf("Apply kubeupdate %+v", kubeUpdate)
	mm.kubeGlobalConfigValues = kubeUpdate.KubeGlobalConfigValues
	mm.kubeModulesConfigValues = kubeUpdate.KubeModulesConfigValues
	mm.enabledModulesByConfig = kubeUpdate.EnabledModulesByConfig

	for _, event := range kubeUpdate.Events {
		EventCh <- event
	}

	return nil
}

func (mm *MainModuleManager) handleNewKubeConfig(newConfig kube_config_manager.Config) (*kubeUpdate, error) {
	rlog.Debugf("MODULE_MANAGER handle new kube config")

	res := &kubeUpdate{
		KubeGlobalConfigValues: newConfig.Values,
		Events:                 []Event{{Type: GlobalChanged}},
	}

	var unknown []utils.ModuleConfig
	res.EnabledModulesByConfig, res.KubeModulesConfigValues, unknown = mm.calculateEnabledModulesByConfig(newConfig.ModuleConfigs)

	for _, moduleConfig := range unknown {
		rlog.Warnf("MODULE_MANAGER new kube config: Ignore kube config for absent module: \n%s",
			moduleConfig.String(),
		)
	}

	return res, nil
}

func (mm *MainModuleManager) handleNewKubeModuleConfigs(moduleConfigs kube_config_manager.ModuleConfigs) (*kubeUpdate, error) {
	rlog.Debugf("MODULE_MANAGER handle changes in module sections")

	res := &kubeUpdate{
		Events:                 make([]Event, 0),
		KubeGlobalConfigValues: mm.kubeGlobalConfigValues,
	}

	// NOTE: values for non changed modules were copied from mm.kubeModulesConfigValues[moduleName].
	// Now calculateEnabledModulesByConfig got values for modules from moduleConfigs — as they are in ConfigMap now.
	// TODO this should not be a problem because of a checksum matching in kube_config_manager
	var unknown []utils.ModuleConfig
	res.EnabledModulesByConfig, res.KubeModulesConfigValues, unknown = mm.calculateEnabledModulesByConfig(moduleConfigs)

	for _, moduleConfig := range unknown {
		rlog.Warnf("HANDLE_CM_UPD ignore module section for unknown module '%s':\n%s",
			moduleConfig.ModuleName, moduleConfig.String())
	}

	// Detect removed module sections for statically enabled modules.
	// This removal should be handled like kube config update.
	updateAfterRemoval := make(map[string]bool, 0)
	for moduleName, module := range mm.allModulesByName {
		_, hasKubeConfig := moduleConfigs[moduleName]
		if !hasKubeConfig && module.StaticConfig.IsEnabled {
			if _, hasValues := mm.kubeModulesConfigValues[moduleName]; hasValues {
				updateAfterRemoval[moduleName] = true
			}
		}
	}

	// New version of mm.enabledModulesByConfig
	res.EnabledModulesByConfig = utils.SortByReference(res.EnabledModulesByConfig, mm.allModulesNamesInOrder)

	// Run enable scripts
	rlog.Infof("HANDLE_CM_UPD run `enabled` for %s", res.EnabledModulesByConfig)
	enabledModules, err := mm.determineEnableStateWithScript(res.EnabledModulesByConfig)
	if err != nil {
		return nil, err
	}
	rlog.Infof("HANDLE_CM_UPD enabled modules %s", enabledModules)

	// Configure events
	if !reflect.DeepEqual(mm.enabledModulesInOrder, enabledModules) {
		// Enabled modules set is changed — return GlobalChanged event, that will
		// create a Discover task, run enabled scripts again, init new module hooks,
		// update mm.enabledModulesInOrder
		rlog.Debugf("HANDLE_CM_UPD enabledByConfig changed from %v to %v: generate GlobalChanged event", mm.enabledModulesByConfig, res.EnabledModulesByConfig)
		res.Events = append(res.Events, Event{Type: GlobalChanged})
	} else {
		// Enabled modules set is not changed, only values in configmap are changed.
		rlog.Debugf("HANDLE_CM_UPD generate ModulesChanged events...")

		moduleChanges := make([]ModuleChange, 0)

		// make Changed event for each enabled module with updated config
		for _, name := range enabledModules {
			// Module has updated kube config
			isUpdated := false
			moduleConfig, hasKubeConfig := moduleConfigs[name]

			if hasKubeConfig {
				isUpdated = moduleConfig.IsUpdated
				// skip not updated module configs
				if !isUpdated {
					rlog.Debugf("HANDLE_CM_UPD ignore module '%s': kube config is not updated", name)
					continue
				}
			}

			// Update module if kube config is removed
			_, shouldUpdateAfterRemoval := updateAfterRemoval[name]

			if (hasKubeConfig && isUpdated) || shouldUpdateAfterRemoval {
				moduleChanges = append(moduleChanges, ModuleChange{Name: name, ChangeType: Changed})
			}
		}

		if len(moduleChanges) > 0 {
			rlog.Infof("HANDLE_CM_UPD fire ModulesChanged event for %d modules", len(moduleChanges))
			rlog.Debugf("HANDLE_CM_UPD event changes: %v", moduleChanges)
			res.Events = append(res.Events, Event{Type: ModulesChanged, ModulesChanges: moduleChanges})
		}
	}

	return res, nil
}

// calculateEnabledModulesByConfig determine enable state for all modules by values.yaml and configmap configuration.
// Method returns list of enabled modules and their values. Also the map of disabled modules and a list of unknown
// keys in a ConfigMap.
//
// Module is enabled by config if module section in ConfigMap is a map or an array
// or ConfigMap has no module section and module has a map or an array in values.yaml
func (mm *MainModuleManager) calculateEnabledModulesByConfig(moduleConfigs kube_config_manager.ModuleConfigs) (enabled []string, values map[string]utils.Values, unknown []utils.ModuleConfig) {
	values = make(map[string]utils.Values)

	for moduleName, module := range mm.allModulesByName {
		kubeConfig, hasKubeConfig := moduleConfigs[moduleName]
		if hasKubeConfig {
			if kubeConfig.IsEnabled {
				enabled = append(enabled, moduleName)
				values[moduleName] = kubeConfig.Values
			}
			rlog.Debugf("Module %s: static enabled %v, kubeConfig: enabled %v, updated %v",
				module.Name,
				module.StaticConfig.IsEnabled,
				kubeConfig.IsEnabled,
				kubeConfig.IsUpdated)
		} else {
			if module.StaticConfig.IsEnabled {
				enabled = append(enabled, moduleName)
			}
			rlog.Debugf("Module %s: static enabled %v, no kubeConfig", module.Name, module.StaticConfig.IsEnabled)
		}
	}

	for _, kubeConfig := range moduleConfigs {
		if _, hasKey := mm.allModulesByName[kubeConfig.ModuleName]; !hasKey {
			unknown = append(unknown, kubeConfig)
		}
	}

	enabled = utils.SortByReference(enabled, mm.allModulesNamesInOrder)

	return
}

// Module manager loop
func (mm *MainModuleManager) Run() {
	go mm.kubeConfigManager.Run()

	for {
		select {
		case <-mm.globalValuesChanged:
			rlog.Debugf("MODULE_MANAGER_RUN global values")
			EventCh <- Event{Type: GlobalChanged}

		case moduleName := <-mm.moduleValuesChanged:
			rlog.Debugf("MODULE_MANAGER_RUN module '%s' values changed", moduleName)

			// Перезапускать enabled-скрипт не нужно, т.к.
			// изменение values модуля не может вызвать
			// изменение состояния включенности модуля
			EventCh <- Event{
				Type: ModulesChanged,
				ModulesChanges: []ModuleChange{
					{Name: moduleName, ChangeType: Changed},
				},
			}

		case newKubeConfig := <-kube_config_manager.ConfigUpdated:
			handleRes, err := mm.handleNewKubeConfig(newKubeConfig)
			if err != nil {
				rlog.Errorf("MODULE_MANAGER_RUN unable to handle kube config update: %s", err)
			}
			if handleRes != nil {
				err = mm.applyKubeUpdate(handleRes)
				if err != nil {
					rlog.Errorf("MODULE_MANAGER_RUN cannot apply kube config update: %s", err)
				}
			}

		case newModuleConfigs := <-kube_config_manager.ModuleConfigsUpdated:
			// Сбросить запомненные перед ошибкой конфиги
			mm.moduleConfigsUpdateBeforeAmbiguos = kube_config_manager.ModuleConfigs{}

			handleRes, err := mm.handleNewKubeModuleConfigs(newModuleConfigs)
			if err != nil {
				mm.moduleConfigsUpdateBeforeAmbiguos = newModuleConfigs
				modulesNames := make([]string, 0)
				for _, newModuleConfig := range newModuleConfigs {
					modulesNames = append(modulesNames, fmt.Sprintf("'%s'", newModuleConfig.ModuleName))
				}
				rlog.Errorf("MODULE_MANAGER_RUN unable to handle modules %s kube config update: %s", strings.Join(modulesNames, ", "), err)
			}
			if handleRes != nil {
				err = mm.applyKubeUpdate(handleRes)
				if err != nil {
					modulesNames := make([]string, 0)
					for _, newModuleConfig := range newModuleConfigs {
						modulesNames = append(modulesNames, fmt.Sprintf("'%s'", newModuleConfig.ModuleName))
					}
					rlog.Errorf("MODULE_MANAGER_RUN cannot apply modules %s kube config update: %s", strings.Join(modulesNames, ", "), err)
				}
			}

		case <-mm.retryOnAmbigous:
			if len(mm.moduleConfigsUpdateBeforeAmbiguos) != 0 {
				rlog.Infof("MODULE_MANAGER_RUN Retry saved moduleConfigs: %v", mm.moduleConfigsUpdateBeforeAmbiguos)
				kube_config_manager.ModuleConfigsUpdated <- mm.moduleConfigsUpdateBeforeAmbiguos
			} else {
				rlog.Debugf("MODULE_MANAGER_RUN Retry IS NOT needed")
			}
		}
	}
}

func (mm *MainModuleManager) Retry() {
	rlog.Debugf("MODULE_MANAGER Retry on ambigous")
	mm.retryOnAmbigous <- true
}

// discoverModulesState calculate new arrays for enabled modules, to be disabled modules and to be purged modules
// This method needs updated mm.enabledModulesByConfig
func (mm *MainModuleManager) discoverModulesState() (*ModulesState, error) {
	// EnabledModules — modules that should be run
	// ModulesToDisable — modules that should be deleted
	// ReleasedUnknownModules — modules that should be purged
	state := &ModulesState{}

	releasedModules, err := mm.helm.ListReleasesNames(nil)
	if err != nil {
		return nil, err
	}

	// calculate unknown released modules to purge them in reverse order
	state.ReleasedUnknownModules = utils.ListSubtract(releasedModules, mm.allModulesNamesInOrder)
	state.ReleasedUnknownModules = utils.SortReverse(state.ReleasedUnknownModules)
	if len(state.ReleasedUnknownModules) > 0 {
		rlog.Infof("DISCOVER found modules with releases: %s", state.ReleasedUnknownModules)
	}

	// ignore unknown released modules for next operations
	releasedModules = utils.ListIntersection(releasedModules, mm.allModulesNamesInOrder)

	// modules finally enabled with enable script
	// no need to refresh mm.enabledModulesByConfig because
	// it is updated before in Init or applyKubeUpdate
	rlog.Infof("DISCOVER run `enabled` for %s", mm.enabledModulesByConfig)
	enabledModules, err := mm.determineEnableStateWithScript(mm.enabledModulesByConfig)
	rlog.Infof("DISCOVER enabled modules %s", enabledModules)
	if err != nil {
		return nil, err
	}

	for _, moduleName := range enabledModules {
		if err = mm.initModuleHooks(mm.allModulesByName[moduleName]); err != nil {
			return nil, err
		}
	}

	state.EnabledModules = enabledModules

	// Calculate modules that has helm release and are disabled for now.
	// Sort them in reverse order for proper deletion.
	state.ModulesToDisable = utils.ListSubtract(mm.allModulesNamesInOrder, enabledModules)
	state.ModulesToDisable = utils.ListIntersection(state.ModulesToDisable, releasedModules)
	state.ModulesToDisable = utils.SortReverseByReference(state.ModulesToDisable, mm.allModulesNamesInOrder)

	return state, nil
}

// DiscoverModulesState handles DiscoverModulesState event
// This method needs updated mm.enabledModulesByConfig and mm.kubeModulesConfigValues
func (mm *MainModuleManager) DiscoverModulesState() (state *ModulesState, err error) {
	rlog.Debugf("DISCOVER state:\n"+
		"    mm.enabledModulesByConfig: %v\n"+
		"    mm.enabledModulesInOrder:  %v\n",
		mm.enabledModulesByConfig,
		mm.enabledModulesInOrder)

	state, err = mm.discoverModulesState()
	if err != nil {
		return nil, err
	}
	mm.enabledModulesInOrder = state.EnabledModules

	rlog.Debugf("DISCOVER state results:\n"+
		"    mm.enabledModulesByConfig: %v\n"+
		"    EnabledModules: %v\n"+
		"    ReleasedUnknownModules: %v\n"+
		"    ModulesToDisable: %v\n",
		mm.enabledModulesByConfig,
		mm.enabledModulesInOrder,
		state.ReleasedUnknownModules,
		state.ModulesToDisable)
	return
}

func (mm *MainModuleManager) GetModule(name string) (*Module, error) {
	module, exist := mm.allModulesByName[name]
	if exist {
		return module, nil
	} else {
		return nil, fmt.Errorf("module '%s' not found", name)
	}
}

func (mm *MainModuleManager) GetModuleNamesInOrder() []string {
	return mm.enabledModulesInOrder
}

func (mm *MainModuleManager) GetGlobalHook(name string) (*GlobalHook, error) {
	globalHook, exist := mm.globalHooksByName[name]
	if exist {
		return globalHook, nil
	} else {
		return nil, fmt.Errorf("global hook '%s' not found", name)
	}
}

func (mm *MainModuleManager) GetModuleHook(name string) (*ModuleHook, error) {
	moduleHook, exist := mm.modulesHooksByName[name]
	if exist {
		return moduleHook, nil
	} else {
		return nil, fmt.Errorf("module hook '%s' not found", name)
	}
}

func (mm *MainModuleManager) GetGlobalHooksInOrder(bindingType BindingType) []string {
	globalHooks, ok := mm.globalHooksOrder[bindingType]
	if !ok {
		return []string{}
	}

	sort.Slice(globalHooks[:], func(i, j int) bool {
		return globalHooks[i].OrderByBinding[bindingType] < globalHooks[j].OrderByBinding[bindingType]
	})

	var globalHooksNames []string
	for _, globalHook := range globalHooks {
		globalHooksNames = append(globalHooksNames, globalHook.Name)
	}

	return globalHooksNames
}

func (mm *MainModuleManager) GetModuleHooksInOrder(moduleName string, bindingType BindingType) ([]string, error) {
	if _, err := mm.GetModule(moduleName); err != nil {
		return nil, err
	}

	moduleHooksByBinding, ok := mm.modulesHooksOrderByName[moduleName]
	if !ok {
		return []string{}, nil
	}

	moduleBindingHooks, ok := moduleHooksByBinding[bindingType]
	if !ok {
		return []string{}, nil
	}

	sort.Slice(moduleBindingHooks[:], func(i, j int) bool {
		return moduleBindingHooks[i].OrderByBinding[bindingType] < moduleBindingHooks[j].OrderByBinding[bindingType]
	})

	var moduleHooksNames []string
	for _, moduleHook := range moduleBindingHooks {
		moduleHooksNames = append(moduleHooksNames, moduleHook.Name)
	}

	return moduleHooksNames, nil
}

func (mm *MainModuleManager) DeleteModule(moduleName string) error {
	module, err := mm.GetModule(moduleName)
	if err != nil {
		return err
	}

	if err := module.delete(); err != nil {
		return err
	}

	return nil
}

func (mm *MainModuleManager) RunModule(moduleName string, onStartup bool) error { // запускает before-helm + helm + after-helm
	module, err := mm.GetModule(moduleName)
	if err != nil {
		return err
	}

	if err := module.run(onStartup); err != nil {
		return err
	}

	return nil
}

func valuesChecksum(valuesArr ...utils.Values) (string, error) {
	valuesJson, err := json.Marshal(utils.MergeValues(valuesArr...))
	if err != nil {
		return "", err
	}
	return utils.CalculateChecksum(string(valuesJson)), nil
}

func (mm *MainModuleManager) RunGlobalHook(hookName string, binding BindingType, bindingContext []BindingContext) error {
	globalHook, err := mm.GetGlobalHook(hookName)
	if err != nil {
		return err
	}

	oldValuesChecksum, err := valuesChecksum(globalHook.values())
	if err != nil {
		return err
	}

	if err := globalHook.run(binding, bindingContext); err != nil {
		return err
	}

	newValuesChecksum, err := valuesChecksum(globalHook.values())
	if err != nil {
		return err
	}

	if newValuesChecksum != oldValuesChecksum {
		switch binding {
		case Schedule, KubeEvents:
			mm.globalValuesChanged <- true
		}
	}

	return nil
}

func (mm *MainModuleManager) RunModuleHook(hookName string, binding BindingType, bindingContext []BindingContext) error {
	moduleHook, err := mm.GetModuleHook(hookName)
	if err != nil {
		return err
	}

	oldValuesChecksum, err := valuesChecksum(moduleHook.values())
	if err != nil {
		return err
	}

	if err := moduleHook.run(binding, bindingContext); err != nil {
		return err
	}

	newValuesChecksum, err := valuesChecksum(moduleHook.values())
	if err != nil {
		return err
	}

	if newValuesChecksum != oldValuesChecksum {
		switch binding {
		case Schedule, KubeEvents:
			mm.moduleValuesChanged <- moduleHook.Module.Name
		}
	}

	return nil
}
