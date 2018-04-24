package module_manager

import (
	"encoding/json"
	"fmt"
	"github.com/romana/rlog"
	"reflect"
	"sort"

	"github.com/deckhouse/deckhouse/antiopa/helm"
	"github.com/deckhouse/deckhouse/antiopa/kube_config_manager"
	"github.com/deckhouse/deckhouse/antiopa/utils"
)

type ModuleManager interface {
	Run()
	DiscoverEnabledModules() ([]string, error)
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
	GetModulesToDisableOnInit() []string
	GetModulesToPurgeOnInit() []string
}

type MainModuleManager struct {
	// Список модулей, найденных в инсталляции
	modulesByName map[string]*Module
	// Список имен модулей найденных в файловой системе и включенных согласно yaml-файлам модулей в порядке вызова.
	// Модули существующие в файловой системе но выключенные в yaml-файле не зарегистрированы в module_manager.
	allModuleNamesInOrder []string
	// Список имен модулей выключенных в kube-config
	kubeDisabledModules []string
	// Список имен модулей, для которых есть файлы, они выключены в kube-config, но для которых есть helm релиз — их нужно удалить.
	releasedModulesToDisable []string
	// Список имен модулей, у которых отсутствуют файлы, но для которых есть helm релиз — их нужно удалить.
	releasedModulesToPurge []string
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
			rlog.Warnf("Module manager: no such module '%s' available: ignoring kube config values: %s", moduleConfig.ModuleName, utils.ValuesToString(moduleConfig.Values))
		}
	}

	releasedModules, err := mm.helm.ListReleases()
	if err != nil {
		return nil, err
	}

	mm.releasedModulesToPurge = mm.getReleasedModulesToPurge(releasedModules)
	mm.releasedModulesToDisable = mm.getReleasedModulesToDisable(releasedModules, mm.kubeDisabledModules)

	return mm, nil
}

func NewMainModuleManager(helmClient helm.HelmClient, kubeConfigManager kube_config_manager.KubeConfigManager) *MainModuleManager {
	return &MainModuleManager{
		modulesByName:               make(map[string]*Module),
		allModuleNamesInOrder:       make([]string, 0),
		kubeDisabledModules:         make([]string, 0),
		releasedModulesToDisable:    make([]string, 0),
		releasedModulesToPurge:      make([]string, 0),
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

func (mm *MainModuleManager) GetModulesToDisableOnInit() []string {
	return mm.releasedModulesToDisable
}

func (mm *MainModuleManager) GetModulesToPurgeOnInit() []string {
	return mm.releasedModulesToPurge
}

func (mm *MainModuleManager) getReleasedModulesToPurge(releasedModules []string) []string {
	res := make([]string, 0)

	for _, releasedModule := range releasedModules {
		if _, hasKey := mm.modulesByName[releasedModule]; !hasKey {
			res = append(res, releasedModule)
		}
	}

	return res
}

func (mm *MainModuleManager) getReleasedModulesToDisable(releasedModules []string, disabledModules []string) []string {
	res := make([]string, 0)

SearchModulesToDisable:
	for _, releasedModule := range releasedModules {
		if _, hasKey := mm.modulesByName[releasedModule]; !hasKey {
			continue SearchModulesToDisable
		}
		for _, disabledModule := range disabledModules {
			if disabledModule == releasedModule {
				res = append(res, releasedModule)
				continue SearchModulesToDisable
			}
		}
	}

	return res
}

func (mm *MainModuleManager) getEnabledModulesInOrder(disabledModules []string) ([]string, error) {
	res := make([]string, 0)

SearchEnabledModules:
	for _, name := range mm.allModuleNamesInOrder {
		for _, disabled := range disabledModules {
			if name == disabled {
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
		}
	}

	return res, nil
}

func (mm *MainModuleManager) getModulesToEnable(oldEnabledModules []string, newEnabledModules []string) []string {
	res := make([]string, 0)

SearchModulesToEnable:
	for _, newModule := range newEnabledModules {
		for _, oldModule := range mm.enabledModulesInOrder {
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
	for _, oldModule := range mm.enabledModulesInOrder {
		for _, newModule := range newEnabledModules {
			if newModule == oldModule {
				continue SearchModulesToDisable
			}
		}
		res = append(res, oldModule)
	}

	return res
}

func (mm *MainModuleManager) handleNewEnabledModules(oldEnabledModules []string, newEnabledModules []string) []Event {
	modulesToEnable := mm.getModulesToEnable(oldEnabledModules, newEnabledModules)
	modulesToDisable := mm.getModulesToDisable(oldEnabledModules, newEnabledModules)

	event := Event{
		Type:           ModulesChanged,
		ModulesChanges: make([]ModuleChange, 0),
	}

	for _, disableModule := range modulesToDisable {
		event.ModulesChanges = append(event.ModulesChanges, ModuleChange{
			Name:       disableModule,
			ChangeType: Disabled,
		})
	}

	for _, enableModule := range modulesToEnable {
		event.ModulesChanges = append(event.ModulesChanges, ModuleChange{
			Name:       enableModule,
			ChangeType: Enabled,
		})
	}

	return []Event{event}
}

type kubeUpdate struct {
	EnabledModules          []string
	KubeConfigValues        utils.Values
	KubeDisabledModules     []string
	KubeModulesConfigValues map[string]utils.Values
	Events                  []Event
}

func (mm *MainModuleManager) applyKubeUpdate(kubeUpdate kubeUpdate) error {
	mm.kubeGlobalConfigValues = kubeUpdate.KubeConfigValues
	mm.kubeModulesConfigValues = kubeUpdate.KubeModulesConfigValues
	mm.kubeDisabledModules = kubeUpdate.KubeDisabledModules
	mm.enabledModulesInOrder = kubeUpdate.EnabledModules

	for _, event := range kubeUpdate.Events {
		EventCh <- event
	}

	return nil
}

func (mm *MainModuleManager) handleNewKubeConfig(newConfig kube_config_manager.Config) (kubeUpdate, error) {
	res := kubeUpdate{
		EnabledModules:          make([]string, 0),
		KubeConfigValues:        newConfig.Values,
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
			rlog.Warnf("Module manager: no such module '%s' available: ignoring kube config values: %s", moduleConfig.ModuleName, utils.ValuesToString(moduleConfig.Values))
		}
	}

	if !reflect.DeepEqual(mm.kubeDisabledModules, res.KubeDisabledModules) {
		newEnabledModules, err := mm.getEnabledModulesInOrder(res.KubeDisabledModules)
		if err != nil {
			return kubeUpdate{}, err
		}

		res.EnabledModules = newEnabledModules
		res.Events = append(res.Events, mm.handleNewEnabledModules(mm.enabledModulesInOrder, newEnabledModules)...)
	}

	res.Events = append(res.Events, Event{Type: GlobalChanged})

	return res, nil
}

func (mm *MainModuleManager) handleNewKubeModuleConfig(newModuleConfig utils.ModuleConfig) (kubeUpdate, error) {
	if _, hasKey := mm.modulesByName[newModuleConfig.ModuleName]; !hasKey {
		rlog.Warnf("Module manager: no such module '%s' available: ignoring kube config values: %s", newModuleConfig.ModuleName, utils.ValuesToString(newModuleConfig.Values))

		return kubeUpdate{
			EnabledModules:          mm.enabledModulesInOrder,
			Events:                  make([]Event, 0),
			KubeConfigValues:        mm.kubeGlobalConfigValues,
			KubeDisabledModules:     mm.kubeDisabledModules,
			KubeModulesConfigValues: mm.kubeModulesConfigValues,
		}, nil
	}

	res := kubeUpdate{
		EnabledModules:          mm.enabledModulesInOrder,
		Events:                  make([]Event, 0),
		KubeConfigValues:        mm.kubeGlobalConfigValues,
		KubeDisabledModules:     make([]string, 0),
		KubeModulesConfigValues: make(map[string]utils.Values),
	}

	for _, disabledModuleName := range mm.kubeDisabledModules {
		if (disabledModuleName != newModuleConfig.ModuleName) || !newModuleConfig.IsEnabled {
			res.KubeDisabledModules = append(res.KubeDisabledModules, disabledModuleName)
		}
	}

	for moduleName, moduleValues := range mm.kubeModulesConfigValues {
		if moduleName != newModuleConfig.ModuleName {
			res.KubeModulesConfigValues[moduleName] = moduleValues
		} else if newModuleConfig.IsEnabled {
			res.KubeModulesConfigValues[newModuleConfig.ModuleName] = newModuleConfig.Values
		}
	}

	wasEnabled := true
	for _, disabledModuleName := range mm.kubeDisabledModules {
		if disabledModuleName == newModuleConfig.ModuleName {
			wasEnabled = false
		}
	}
	if (!wasEnabled && newModuleConfig.IsEnabled) || (wasEnabled && !newModuleConfig.IsEnabled) {
		newEnabledModules, err := mm.getEnabledModulesInOrder(mm.kubeDisabledModules)
		if err != nil {
			return kubeUpdate{}, err
		}

		res.EnabledModules = newEnabledModules
		res.Events = append(res.Events, mm.handleNewEnabledModules(mm.enabledModulesInOrder, newEnabledModules)...)
	}

	if !wasEnabled && !newModuleConfig.IsEnabled {
		rlog.Debugf("Module manager: module '%s' remains in disabled state: ignoring update", newModuleConfig.ModuleName)
	} else {
		res.Events = append(res.Events, Event{
			Type: ModulesChanged,
			ModulesChanges: []ModuleChange{
				{Name: newModuleConfig.ModuleName, ChangeType: Changed},
			},
		})
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
			err = mm.applyKubeUpdate(handleRes)
			if err != nil {
				rlog.Errorf("Module manager: cannot apply kube config update: %s", err)
			}

		case newModuleConfig := <-kube_config_manager.ModuleConfigUpdated:
			handleRes, err := mm.handleNewKubeModuleConfig(newModuleConfig)
			if err != nil {
				rlog.Errorf("Module manager: unable to handle module '%s' kube config update: %s", newModuleConfig.ModuleName, err)
			}
			err = mm.applyKubeUpdate(handleRes)
			if err != nil {
				rlog.Errorf("Module manager: cannot apply module '%s' kube config update: %s", newModuleConfig.ModuleName, err)
			}
		}
	}
}

func (mm *MainModuleManager) DiscoverEnabledModules() ([]string, error) {
	enabledModules, err := mm.getEnabledModulesInOrder(mm.kubeDisabledModules)
	if err != nil {
		return nil, err
	}
	mm.enabledModulesInOrder = enabledModules

	return mm.enabledModulesInOrder, nil
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
	return mm.allModuleNamesInOrder
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
