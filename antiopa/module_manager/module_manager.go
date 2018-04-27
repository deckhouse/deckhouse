package module_manager

import (
	"encoding/json"
	"fmt"
	"github.com/romana/rlog"
	"sort"

	"github.com/deckhouse/deckhouse/antiopa/helm"
	"github.com/deckhouse/deckhouse/antiopa/kube_config_manager"
	"github.com/deckhouse/deckhouse/antiopa/utils"
	"reflect"
	"strings"
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
	RunModule(moduleName string) error
	RunGlobalHook(hookName string, binding BindingType) error
	RunModuleHook(hookName string, binding BindingType) error
}

// All modules are in the right order to run/disable/purge
type ModulesState struct {
	EnabledModules         []string
	ModulesToDisable       []string
	ReleasedUnknownModules []string
}

type MainModuleManager struct {
	// Список модулей, найденных в инсталляции
	modulesByName map[string]*Module
	// Список имен модулей найденных в файловой системе и включенных согласно yaml-файлам модулей в порядке вызова.
	// Модули существующие в файловой системе но выключенные в yaml-файле не зарегистрированы в module_manager.
	allModuleNamesInOrder []string
	// Список имен модулей выключенных в kube-config
	kubeDisabledModules []string

	// Результирующий список имен включенных модулей в порядке вызова.
	// С учетом скрипта enabled, kube-config и yaml-файла для модуля.
	// Список меняется во время работы antiopa по мере возникновения событий
	// включения/выключения модулей от kube-config.
	enabledModulesInOrder []string

	globalHooksByName map[string]*GlobalHook        // name -> Hook
	globalHooksOrder  map[BindingType][]*GlobalHook // это что-то внутреннее для быстрого поиска binding -> hooks names in order, можно и по-другому сделать

	modulesHooksByName      map[string]*ModuleHook
	modulesHooksOrderByName map[string]map[BindingType][]*ModuleHook

	// файл values.yaml для всех модулей, для всех кластеров
	globalStaticValues utils.Values
	// файл values.yaml для конкретного модуля, для всех кластеров
	modulesStaticValues map[string]utils.Values

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
}

var (
	EventCh    chan Event
	WorkingDir string
	TempDir    string
)

// Типы привязок для хуков — то, от чего могут сработать хуки
type BindingType string

const (
	BeforeHelm       BindingType = "BEFORE_HELM"
	AfterHelm        BindingType = "AFTER_HELM"
	AfterDeleteHelm  BindingType = "AFTER_DELETE_HELM"
	BeforeAll        BindingType = "BEFORE_ALL"
	AfterAll         BindingType = "AFTER_ALL"
	OnKubeNodeChange BindingType = "ON_KUBE_NODE_CHANGE"
	Schedule         BindingType = "SCHEDULE"
	OnStartup        BindingType = "ON_STARTUP"
	KubeEvents       BindingType = "KUBE_EVENTS"
)

// Типы событий, отправляемые в Main — либо изменились какие-то модули и нужно
// пройти по списку и запустить/удалить/проапгрейдить модуль,
// либо поменялись глобальные values и нужно перезапустить все модули.
type EventType string

const (
	ModulesChanged EventType = "MODULES_CHANGED"
	GlobalChanged  EventType = "GLOBAL_CHANGED"
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

/*
Пример конфига:

{
    onStartup: ORDER, // оба
    beforeHelm: ORDER, // только module
    afterHelm: ORDER, // только module
    afterDeleteHelm: ORDER, // только module
    beforeAll: ORDER, // только global
    afterAll: ORDER, // только global
    onKubeNodeChange: ORDER, // только global
	onAdd: [
		{
			kind: pod|service|namespace|... ,
			selector:
			    matchExpressions: ... https://v1-6.docs.kubernetes.io/docs/api-reference/v1.6/#labelselector-v1-meta,
			    matchLabels: ... ,
            namespaceSelector:
		        matchNames: [...]
	            any: true|false
	        jqFilter: ".items[] | del(.metadata, .status)",
			allowFailure: true,
	    }
	],
	onUpdate: [
		...
	],
	onDelete: [
		...
	],
    schedule:  [
		{
			crontab: "* * * * *",
			allowFailure: true
		},
        {
			crontab: "*_/2 * * * *",
		}
	]
}
*/

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
	mm.kubeModulesConfigValues = make(map[string]utils.Values)
	mm.kubeDisabledModules = make([]string, 0)
	for _, moduleConfig := range kubeConfig.ModuleConfigs {
		if _, hasKey := mm.modulesByName[moduleConfig.ModuleName]; hasKey {
			if moduleConfig.IsEnabled {
				mm.kubeModulesConfigValues[moduleConfig.ModuleName] = moduleConfig.Values
			} else {
				mm.kubeDisabledModules = append(mm.kubeDisabledModules, moduleConfig.ModuleName)
			}
		} else {
			rlog.Warnf("Module manager: no such module '%s' available: ignoring kube config:\n%s", moduleConfig.ModuleName, moduleConfig.String())
		}
	}

	return mm, nil
}

func NewMainModuleManager(helmClient helm.HelmClient, kubeConfigManager kube_config_manager.KubeConfigManager) *MainModuleManager {
	return &MainModuleManager{
		modulesByName:               make(map[string]*Module),
		allModuleNamesInOrder:       make([]string, 0),
		kubeDisabledModules:         make([]string, 0),
		enabledModulesInOrder:       make([]string, 0),
		globalHooksByName:           make(map[string]*GlobalHook),
		globalHooksOrder:            make(map[BindingType][]*GlobalHook),
		modulesHooksByName:          make(map[string]*ModuleHook),
		modulesHooksOrderByName:     make(map[string]map[BindingType][]*ModuleHook),
		globalStaticValues:          make(utils.Values),
		modulesStaticValues:         make(map[string]utils.Values),
		kubeGlobalConfigValues:      make(utils.Values),
		kubeModulesConfigValues:     make(map[string]utils.Values),
		globalDynamicValuesPatches:  make([]utils.ValuesPatch, 0),
		modulesDynamicValuesPatches: make(map[string][]utils.ValuesPatch),
		moduleValuesChanged:         make(chan string, 1),
		globalValuesChanged:         make(chan bool, 1),

		helm:              helmClient,
		kubeConfigManager: kubeConfigManager,
	}
}

func (mm *MainModuleManager) sortUnknownModules(modules []string) []string {
	res := make([]string, 0)
	for _, module := range modules {
		res = append(res, module)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(res)))

	return res
}

func (mm *MainModuleManager) sortDisabledModules(modules []string) []string {
	res := make([]string, 0)

	for _, module := range mm.allModuleNamesInOrder {
		for _, disableModule := range modules {
			if module == disableModule {
				// prepend
				res = append([]string{module}, res...)
			}
		}
	}

	return res
}

func (mm *MainModuleManager) getDisabledModules(enabledModules []string) []string {
	res := make([]string, 0)

SearchDisabledModules:
	for _, module := range mm.allModuleNamesInOrder {
		for _, enabledModule := range enabledModules {
			if module == enabledModule {
				continue SearchDisabledModules
			}
		}

		res = append(res, module)
	}

	return res
}

func (mm *MainModuleManager) getReleasedUnknownModules(releasedModules []string) []string {
	res := make([]string, 0)

	for _, releasedModule := range releasedModules {
		if _, hasKey := mm.modulesByName[releasedModule]; !hasKey {
			res = append(res, releasedModule)
		}
	}

	return mm.sortUnknownModules(res)
}

func (mm *MainModuleManager) getReleasedDisabledModules(releasedModules []string, disabledModules []string) []string {
	res := make([]string, 0)

SearchDisabledModules:
	for _, releasedModule := range releasedModules {
		if _, hasKey := mm.modulesByName[releasedModule]; !hasKey {
			continue SearchDisabledModules
		}
		for _, disabledModule := range disabledModules {
			if disabledModule == releasedModule {
				res = append(res, releasedModule)
				continue SearchDisabledModules
			}
		}
	}

	return mm.sortDisabledModules(res)
}

func (mm *MainModuleManager) getEnabledModulesInOrder(disabledModules []string) ([]string, error) {
	res := make([]string, 0)

	rlog.Debugf("Discover enabled modules: disabled modules list: %v", disabledModules)

SearchEnabledModules:
	for _, name := range mm.allModuleNamesInOrder {
		for _, disabled := range disabledModules {
			if name == disabled {
				rlog.Infof("Discover enabled modules: module '%s' is DISABLED in config, enabled modules: %s", name, res)
				continue SearchEnabledModules
			}
		}

		// module should exist in mm.modulesByName by invariant
		moduleIsEnabled, err := mm.modulesByName[name].checkIsEnabledByScript(res)
		if err != nil {
			return nil, err
		}

		if moduleIsEnabled {
			res = append(res, name)
			rlog.Infof("Discover enabled modules: module '%s' is ENABLED, enabled modules: %s", name, res)
		} else {
			rlog.Infof("Discover enabled modules: module '%s' is DISABLED, enabled modules: %s", name, res)
		}
	}

	return res, nil
}

func (mm *MainModuleManager) getModulesToEnable(oldEnabledModules []string, newEnabledModules []string) []string {
	res := make([]string, 0)

SearchModulesToEnable:
	for _, newModule := range newEnabledModules {
		for _, oldModule := range oldEnabledModules {
			if oldModule == newModule {
				continue SearchModulesToEnable
			}
		}
		res = append(res, newModule)
	}

	return res
}

func (mm *MainModuleManager) getModulesToDisable(oldEnabledModules []string, newEnabledModules []string) []string {
	res := make([]string, 0)

SearchModulesToDisable:
	for _, oldModule := range oldEnabledModules {
		for _, newModule := range newEnabledModules {
			if newModule == oldModule {
				continue SearchModulesToDisable
			}
		}
		res = append(res, oldModule)
	}

	return res
}

type kubeUpdate struct {
	EnabledModules          []string
	KubeGlobalConfigValues  utils.Values
	KubeDisabledModules     []string
	KubeModulesConfigValues map[string]utils.Values
	Events                  []Event
}

func (mm *MainModuleManager) applyKubeUpdate(kubeUpdate *kubeUpdate) error {
	mm.kubeGlobalConfigValues = kubeUpdate.KubeGlobalConfigValues
	mm.kubeModulesConfigValues = kubeUpdate.KubeModulesConfigValues
	mm.kubeDisabledModules = kubeUpdate.KubeDisabledModules
	mm.enabledModulesInOrder = kubeUpdate.EnabledModules

	for _, event := range kubeUpdate.Events {
		EventCh <- event
	}

	return nil
}

func (mm *MainModuleManager) handleNewKubeConfig(newConfig kube_config_manager.Config) (*kubeUpdate, error) {
	rlog.Debugf("Module manager: handle new kube config")

	res := &kubeUpdate{
		EnabledModules:          mm.enabledModulesInOrder,
		KubeGlobalConfigValues:  newConfig.Values,
		KubeDisabledModules:     make([]string, 0),
		KubeModulesConfigValues: make(map[string]utils.Values),
		Events:                  make([]Event, 0),
	}

	for _, moduleConfig := range newConfig.ModuleConfigs {
		if _, hasKey := mm.modulesByName[moduleConfig.ModuleName]; hasKey {
			if !moduleConfig.IsEnabled {
				res.KubeDisabledModules = append(res.KubeDisabledModules, moduleConfig.ModuleName)
				continue
			}
			res.KubeModulesConfigValues[moduleConfig.ModuleName] = moduleConfig.Values
		} else {
			rlog.Warnf("Module manager: no such module '%s' available: ignoring kube config values:\n%s", moduleConfig.ModuleName, moduleConfig.String())
		}
	}

	res.Events = append(res.Events, Event{Type: GlobalChanged})

	return res, nil
}

func (mm *MainModuleManager) handleNewKubeModuleConfigs(moduleConfigsUpdate kube_config_manager.ModuleConfigs) (*kubeUpdate, error) {
	modulesNames := make([]string, 0)
	for module := range moduleConfigsUpdate {
		modulesNames = append(modulesNames, fmt.Sprintf("'%s'", module))
	}
	rlog.Debugf("Module manager: handle new kube modules configs: %s", strings.Join(modulesNames, ", "))

	newModuleConfigs := make(kube_config_manager.ModuleConfigs)
	for module, newModuleConfig := range moduleConfigsUpdate {
		if _, hasKey := mm.modulesByName[newModuleConfig.ModuleName]; !hasKey {
			rlog.Warnf("Module manager: no such module '%s' available: ignoring module kube config:\n%s", newModuleConfig.ModuleName, newModuleConfig.String())
			continue
		}
		newModuleConfigs[module] = newModuleConfig
	}

	res := &kubeUpdate{
		EnabledModules:          mm.enabledModulesInOrder,
		Events:                  make([]Event, 0),
		KubeGlobalConfigValues:  mm.kubeGlobalConfigValues,
		KubeDisabledModules:     make([]string, 0),
		KubeModulesConfigValues: make(map[string]utils.Values),
	}

	// construct new kube-disabled-modules list

	for _, newModuleConfig := range newModuleConfigs {
		if newModuleConfig.IsEnabled {
			continue
		}
		res.KubeDisabledModules = append(res.KubeDisabledModules, newModuleConfig.ModuleName)
	}

DisableOldDisabledModules:
	for _, oldDisabledModule := range mm.kubeDisabledModules {
		// Disable if not already disabled
		for _, disabledModule := range res.KubeDisabledModules {
			if oldDisabledModule == disabledModule {
				continue DisableOldDisabledModules
			}
		}
		// Skip new modules that is enabled now
		for _, newModuleConfig := range newModuleConfigs {
			if newModuleConfig.ModuleName == oldDisabledModule && newModuleConfig.IsEnabled {
				continue DisableOldDisabledModules
			}
		}
		res.KubeDisabledModules = append(res.KubeDisabledModules, oldDisabledModule)
	}

	rlog.Debugf("Module manager: new kube disabled modules list: %v", res.KubeDisabledModules)

	// copy current modules config values except the module being updated

	for _, newModuleConfig := range newModuleConfigs {
		if !newModuleConfig.IsEnabled {
			continue
		}
		res.KubeModulesConfigValues[newModuleConfig.ModuleName] = newModuleConfig.Values
	}

	for oldModule, oldModuleValues := range mm.kubeModulesConfigValues {
		if _, hasKey := res.KubeModulesConfigValues[oldModule]; hasKey {
			continue
		}
		res.KubeModulesConfigValues[oldModule] = oldModuleValues
	}

	// calculate new enabled-modules list for possible changes related to kube-disabled-modules list changes

	newEnabledModules, err := mm.getEnabledModulesInOrder(res.KubeDisabledModules)
	if err != nil {
		return nil, err
	}

	rlog.Debugf("Module manager: new enabled modules list: %v", newEnabledModules)

	if !reflect.DeepEqual(mm.enabledModulesInOrder, newEnabledModules) {
		rlog.Debugf("Module manager: enabled modules list changed from %v to %v: generating GlobalChanged event", mm.enabledModulesInOrder, newEnabledModules)
		res.Events = append(res.Events, Event{Type: GlobalChanged})
	} else {
		modulesChanges := make([]ModuleChange, 0)

		for _, newModuleConfig := range newModuleConfigs {
			if newModuleConfig.IsEnabled {
				modulesChanges = append(modulesChanges, ModuleChange{Name: newModuleConfig.ModuleName, ChangeType: Changed})
			} else {
				rlog.Debugf("Module manager: module '%s' remains in disabled state: ignoring update:\n%s", newModuleConfig.ModuleName, newModuleConfig.String())
			}
		}

		if len(modulesChanges) > 0 {
			rlog.Debugf("Module manager: generating ModulesChanged event: %v", modulesChanges)
			res.Events = append(res.Events, Event{Type: ModulesChanged, ModulesChanges: modulesChanges})
		}
	}

	return res, nil
}

// Module manager loop
func (mm *MainModuleManager) Run() {
	go mm.kubeConfigManager.Run()

	for {
		select {
		case <-mm.globalValuesChanged:
			rlog.Debugf("Module manager: global values")
			EventCh <- Event{Type: GlobalChanged}

		case moduleName := <-mm.moduleValuesChanged:
			rlog.Debugf("Module manager: module '%s' values changed", moduleName)

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
				rlog.Errorf("Module manager: unable to handle kube config update: %s", err)
			}
			if handleRes != nil {
				err = mm.applyKubeUpdate(handleRes)
				if err != nil {
					rlog.Errorf("Module manager: cannot apply kube config update: %s", err)
				}
			}

		case newModuleConfigs := <-kube_config_manager.ModuleConfigsUpdated:
			handleRes, err := mm.handleNewKubeModuleConfigs(newModuleConfigs)
			if err != nil {
				modulesNames := make([]string, 0)
				for _, newModuleConfig := range newModuleConfigs {
					modulesNames = append(modulesNames, fmt.Sprintf("'%s'", newModuleConfig.ModuleName))
				}
				rlog.Errorf("Module manager: unable to handle modules %s kube config update: %s", strings.Join(modulesNames, ", "), err)
			}
			if handleRes != nil {
				err = mm.applyKubeUpdate(handleRes)
				if err != nil {
					modulesNames := make([]string, 0)
					for _, newModuleConfig := range newModuleConfigs {
						modulesNames = append(modulesNames, fmt.Sprintf("'%s'", newModuleConfig.ModuleName))
					}
					rlog.Errorf("Module manager: cannot apply modules %s kube config update: %s", strings.Join(modulesNames, ", "), err)
				}
			}
		}
	}
}

func (mm *MainModuleManager) discoverModulesState(kubeDisabledModules []string) (*ModulesState, error) {
	state := &ModulesState{}

	enabledModules, err := mm.getEnabledModulesInOrder(kubeDisabledModules)
	if err != nil {
		return nil, err
	}

	oldEnabledModules := mm.enabledModulesInOrder
	mm.enabledModulesInOrder = enabledModules

	state.EnabledModules = mm.enabledModulesInOrder

	releasedModules, err := mm.helm.ListReleasesNames()
	if err != nil {
		return nil, err
	}

	state.ReleasedUnknownModules = mm.getReleasedUnknownModules(releasedModules)

	allDisabledModules := append([]string{}, kubeDisabledModules...)
	allDisabledModules = append(allDisabledModules, mm.getDisabledModules(enabledModules)...)

	// Turn off modules for which there is helm release and now module is disabled
	disabledModules := mm.getReleasedDisabledModules(releasedModules, allDisabledModules)

	// Turn off modules without charts (and thus without releases) and
	// modules with lost helm releases (may be deleted manually)
SearchDisabledModules:
	for _, oldEnabledModule := range oldEnabledModules {
		for _, enabledModule := range enabledModules {
			if oldEnabledModule == enabledModule {
				continue SearchDisabledModules
			}
		}

		for _, disabledModule := range disabledModules {
			if disabledModule == oldEnabledModule {
				continue SearchDisabledModules
			}
		}

		disabledModules = append(disabledModules, oldEnabledModule)
	}

	state.ModulesToDisable = mm.sortDisabledModules(disabledModules)

	return state, nil
}

func (mm *MainModuleManager) DiscoverModulesState() (*ModulesState, error) {
	rlog.Debugf("DiscoverModulesState: kube disabled modules: %v", mm.kubeDisabledModules)
	return mm.discoverModulesState(mm.kubeDisabledModules)
}

func (mm *MainModuleManager) GetModule(name string) (*Module, error) {
	module, exist := mm.modulesByName[name]
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

func (mm *MainModuleManager) RunModule(moduleName string) error { // запускает before-helm + helm + after-helm
	module, err := mm.GetModule(moduleName)
	if err != nil {
		return err
	}

	if err := module.run(); err != nil {
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

func (mm *MainModuleManager) RunGlobalHook(hookName string, binding BindingType) error {
	globalHook, err := mm.GetGlobalHook(hookName)
	if err != nil {
		return err
	}

	oldValuesChecksum, err := valuesChecksum(globalHook.values())
	if err != nil {
		return err
	}

	if err := globalHook.run(binding); err != nil {
		return err
	}

	newValuesChecksum, err := valuesChecksum(globalHook.values())
	if err != nil {
		return err
	}

	if newValuesChecksum != oldValuesChecksum {
		switch binding {
		case OnKubeNodeChange:
			mm.globalValuesChanged <- true
		}
	}

	return nil
}

func (mm *MainModuleManager) RunModuleHook(hookName string, binding BindingType) error {
	moduleHook, err := mm.GetModuleHook(hookName)
	if err != nil {
		return err
	}

	oldValuesChecksum, err := valuesChecksum(moduleHook.values())
	if err != nil {
		return err
	}

	if err := moduleHook.run(binding); err != nil {
		return err
	}

	newValuesChecksum, err := valuesChecksum(moduleHook.values())
	if err != nil {
		return err
	}

	if newValuesChecksum != oldValuesChecksum {
		switch binding {
		case Schedule:
			mm.moduleValuesChanged <- moduleHook.Module.Name
		}
	}

	return nil
}

func (mm *MainModuleManager) constructEnabledModulesValues(enabledModules []string) utils.Values {
	return utils.Values{
		"global": map[string]interface{}{
			"enabledModules": enabledModules,
		},
	}
}
