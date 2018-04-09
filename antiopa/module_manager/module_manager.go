package module_manager

import (
	"encoding/json"
	"fmt"
	"github.com/romana/rlog"
	"reflect"
	"sort"

	"github.com/deckhouse/deckhouse/antiopa/kube_config_manager"
	"github.com/deckhouse/deckhouse/antiopa/utils"
)

var (
	EventCh chan Event

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

	// values для всех модулей, для всех кластеров
	globalConfigValues utils.Values
	// values для конкретного модуля, для всех кластеров
	globalModulesConfigValues map[string]utils.Values
	// values для всех модулей, для конкретного кластера
	kubeConfigValues utils.Values
	// values для конкретного модуля, для конкретного кластера
	kubeModulesConfigValues map[string]utils.Values
	// dynamic-values для всех модулей, для всех кластеров
	dynamicValues utils.Values
	// dynamic-values для конкретного модуля, для всех кластеров
	modulesDynamicValues map[string]utils.Values

	// Внутреннее событие: изменились values модуля.
	// Обработка -- генерация внешнего Event со всеми связанными модулями для рестарта.
	moduleValuesChanged chan string
	// Внутреннее событие: изменились глобальные values.
	// Обработка -- генерация внешнего Event для глобального рестарта всех модулей.
	globalValuesChanged chan bool

	WorkingDir string
	TempDir    string
)

// Типы привязок для хуков — то, от чего могут сработать хуки
type BindingType string

const (
	BeforeHelm       BindingType = "BEFORE_HELM"
	AfterHelm        BindingType = "AFTER_HELM"
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
    beforeAll: ORDER, // только global
    afterAll: ORDER, // только global
    onKubeNodeChange: ORDER, // только global
    schedule:
        - crontab: * * * * *
          allowFailure: true
        - crontab: *_/2 * * * *
}
*/

func Init(workingDir string, tempDir string) error {
	rlog.Info("Initializing module manager ...")

	TempDir = tempDir
	WorkingDir = workingDir

	EventCh = make(chan Event, 1)
	globalValuesChanged = make(chan bool, 1)
	moduleValuesChanged = make(chan string, 1)

	kubeConfigValues = make(utils.Values) // TODO
	rlog.Debugf("Set kubeConfigValues:\n%s", valuesToString(kubeConfigValues))

	dynamicValues = make(utils.Values)

	if err := initGlobalHooks(); err != nil {
		return err
	}

	if err := initModules(); err != nil {
		return err
	}

	kubeConfig, err := kube_config_manager.Init()
	if err != nil {
		return err
	}
	setKubeConfig(kubeConfig)

	return nil
}

func setKubeConfig(kubeConfig *kube_config_manager.Config) {

}

func getEnabledModulesInOrder(kubeDisabledModules []string) ([]string, error) {
	res := make([]string, 0)
	for _, name := range allModuleNamesInOrder {
		for _, disabled := range kubeDisabledModules {
			if name != disabled {
				res = append(res, name)
			}
		}
	}

	// TODO: check enabled script

	return res, nil
}

func getModulesToEnable(oldEnabledModules []string, newEnabledModules []string) []string {
	res := make([]string, 0)

SearchModulesToEnable:
	for _, newModule := range newEnabledModules {
		for _, oldModule := range enabledModulesInOrder {
			if oldModule == newModule {
				continue SearchModulesToEnable
			}
		}
		res = append(res, newModule)
	}

	return res
}

func getModulesToDisable(oldEnabledModules []string, newEnabledModules []string) []string {
	res := make([]string, 0)

SearchModulesToDisable:
	for _, oldModule := range enabledModulesInOrder {
		for _, newModule := range newEnabledModules {
			if newModule == oldModule {
				continue SearchModulesToDisable
			}
		}
		res = append(res, oldModule)
	}

	return res
}

func handleNewEnabledModules(oldEnabledModules []string, newEnabledModules []string) []Event {
	modulesToEnable := getModulesToEnable(oldEnabledModules, newEnabledModules)
	modulesToDisable := getModulesToDisable(oldEnabledModules, newEnabledModules)

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

func applyKubeUpdate(kubeUpdate kubeUpdate) error {
	kubeConfigValues = kubeUpdate.KubeConfigValues
	kubeModulesConfigValues = kubeUpdate.KubeModulesConfigValues
	kubeDisabledModules = kubeUpdate.KubeDisabledModules
	enabledModulesInOrder = kubeUpdate.EnabledModules

	for _, event := range kubeUpdate.Events {
		EventCh <- event
	}

	return nil
}

func handleNewKubeConfig(newConfig kube_config_manager.Config) (kubeUpdate, error) {
	res := kubeUpdate{
		EnabledModules:          make([]string, 0),
		KubeConfigValues:        newConfig.Values,
		KubeDisabledModules:     make([]string, 0),
		KubeModulesConfigValues: make(map[string]utils.Values),
		Events:                  make([]Event, 0),
	}

	for _, moduleConfig := range newConfig.ModuleConfigs {
		if !moduleConfig.IsEnabled {
			res.KubeDisabledModules = append(res.KubeDisabledModules, moduleConfig.ModuleName)
			continue
		}
		res.KubeModulesConfigValues[moduleConfig.ModuleName] = moduleConfig.Values
	}

	if !reflect.DeepEqual(kubeDisabledModules, res.KubeDisabledModules) {
		newEnabledModules, err := getEnabledModulesInOrder(res.KubeDisabledModules)
		if err != nil {
			return kubeUpdate{}, err
		}

		res.EnabledModules = newEnabledModules
		res.Events = append(res.Events, handleNewEnabledModules(enabledModulesInOrder, newEnabledModules)...)
	}

	res.Events = append(res.Events, Event{Type: GlobalChanged})

	return res, nil
}

func handleNewKubeModuleConfig(newModuleConfig utils.ModuleConfig) (kubeUpdate, error) {
	res := kubeUpdate{
		EnabledModules:          enabledModulesInOrder,
		Events:                  make([]Event, 0),
		KubeConfigValues:        kubeConfigValues,
		KubeDisabledModules:     make([]string, 0),
		KubeModulesConfigValues: make(map[string]utils.Values),
	}

	for _, disabledModuleName := range kubeDisabledModules {
		if (disabledModuleName != newModuleConfig.ModuleName) || !newModuleConfig.IsEnabled {
			res.KubeDisabledModules = append(res.KubeDisabledModules, disabledModuleName)
		}
	}

	for moduleName, moduleValues := range kubeModulesConfigValues {
		if moduleName != newModuleConfig.ModuleName {
			res.KubeModulesConfigValues[moduleName] = moduleValues
		} else if newModuleConfig.IsEnabled {
			res.KubeModulesConfigValues[newModuleConfig.ModuleName] = newModuleConfig.Values
		}
	}

	wasEnabled := true
	for _, disabledModuleName := range kubeDisabledModules {
		if disabledModuleName == newModuleConfig.ModuleName {
			wasEnabled = false
		}
	}
	if (!wasEnabled && newModuleConfig.IsEnabled) || (wasEnabled && !newModuleConfig.IsEnabled) {
		newEnabledModules, err := getEnabledModulesInOrder(kubeDisabledModules)
		if err != nil {
			return kubeUpdate{}, err
		}

		res.EnabledModules = newEnabledModules
		res.Events = append(res.Events, handleNewEnabledModules(enabledModulesInOrder, newEnabledModules)...)
	}

	if !wasEnabled && !newModuleConfig.IsEnabled {
		rlog.Debugf("Module manager: module %s remains in disabled state: ignoring update")
	} else {
		res.Events = append(res.Events, Event{
			Type: ModulesChanged,
			ModulesChanges: []ModuleChange{
				ModuleChange{Name: newModuleConfig.ModuleName, ChangeType: Changed},
			},
		})
	}

	return res, nil
}

// Module manager loop
func Run() {
	for {
		select {
		/*
		 * TODO: filter out unknown modules
		 */

		case <-globalValuesChanged:
			rlog.Debugf("Module manager: global values")
			EventCh <- Event{Type: GlobalChanged}

		case moduleName := <-moduleValuesChanged:
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
			handleRes, err := handleNewKubeConfig(newKubeConfig)
			if err != nil {
				rlog.Errorf("Module manager: unable to handle kube config update: %s", err)
			}
			err = applyKubeUpdate(handleRes)
			if err != nil {
				rlog.Errorf("Module manager: cannot apply kube config update: %s", err)
			}

		case newModuleConfig := <-kube_config_manager.ModuleConfigUpdated:
			handleRes, err := handleNewKubeModuleConfig(newModuleConfig)
			if err != nil {
				rlog.Errorf("Module manager: unable to handle module '%s' kube config update: %s", newModuleConfig.ModuleName, err)
			}
			err = applyKubeUpdate(handleRes)
			if err != nil {
				rlog.Errorf("Module manager: cannot apply module '%s' kube config update: %s", newModuleConfig.ModuleName, err)
			}
		}
	}
}

func GetModule(name string) (*Module, error) {
	module, exist := modulesByName[name]
	if exist {
		return module, nil
	} else {
		return nil, fmt.Errorf("module '%s' not found", name)
	}
}

func GetModuleNamesInOrder() []string {
	return allModuleNamesInOrder
}

func GetGlobalHook(name string) (*GlobalHook, error) {
	globalHook, exist := globalHooksByName[name]
	if exist {
		return globalHook, nil
	} else {
		return nil, fmt.Errorf("global hook '%s' not found", name)
	}
}

func GetModuleHook(name string) (*ModuleHook, error) {
	moduleHook, exist := modulesHooksByName[name]
	if exist {
		return moduleHook, nil
	} else {
		return nil, fmt.Errorf("module hook '%s' not found", name)
	}
}

func GetGlobalHooksInOrder(bindingType BindingType) ([]string, error) {
	globalHooks := globalHooksOrder[bindingType]
	sort.Slice(globalHooks[:], func(i, j int) bool {
		return globalHooks[i].OrderByBinding[bindingType] < globalHooks[j].OrderByBinding[bindingType]
	})

	var globalHooksNames []string
	for _, globalHook := range globalHooks {
		globalHooksNames = append(globalHooksNames, globalHook.Name)
	}

	return globalHooksNames, nil
}

func GetModuleHooksInOrder(moduleName string, bindingType BindingType) ([]string, error) {
	moduleHooksByBinding, ok := modulesHooksOrderByName[moduleName]
	if !ok {
		return nil, fmt.Errorf("module '%s' not found", moduleName)
	}
	moduleBindingHooks := moduleHooksByBinding[bindingType]

	sort.Slice(moduleBindingHooks[:], func(i, j int) bool {
		return moduleBindingHooks[i].OrderByBinding[bindingType] < moduleBindingHooks[j].OrderByBinding[bindingType]
	})

	var moduleHooksNames []string
	for _, moduleHook := range moduleBindingHooks {
		moduleHooksNames = append(moduleHooksNames, moduleHook.Name)
	}

	return moduleHooksNames, nil
}

/*
 * TODO: удаляет helm release (purge)
 * TODO: выполняет новый вид хука afterHelmDelete
 */
func DeleteModule(moduleName string) error { return nil }

func RunModule(moduleName string) error { // запускает before-helm + helm + after-helm
	module, err := GetModule(moduleName)
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

func RunGlobalHook(hookName string, binding BindingType) error {
	globalHook, err := GetGlobalHook(hookName)
	if err != nil {
		return err
	}

	oldValuesChecksum, err := valuesChecksum(kubeConfigValues, dynamicValues)
	if err != nil {
		return err
	}

	if err := globalHook.run(binding); err != nil {
		return err
	}

	newValuesChecksum, err := valuesChecksum(kubeConfigValues, dynamicValues)
	if err != nil {
		return err
	}

	if newValuesChecksum != oldValuesChecksum {
		switch binding {
		case OnKubeNodeChange:
			globalValuesChanged <- true
		}
	}

	return nil
}

func RunModuleHook(hookName string, binding BindingType) error {
	moduleHook, err := GetModuleHook(hookName)
	if err != nil {
		return err
	}

	oldValuesChecksum, err := valuesChecksum(kubeModulesConfigValues[moduleHook.Module.Name], modulesDynamicValues[moduleHook.Module.Name])
	if err != nil {
		return err
	}

	if err := moduleHook.run(binding); err != nil {
		return err
	}

	newValuesChecksum, err := valuesChecksum(kubeModulesConfigValues[moduleHook.Module.Name], modulesDynamicValues[moduleHook.Module.Name])
	if err != nil {
		return err
	}

	if newValuesChecksum != oldValuesChecksum {
		switch binding {
		case Schedule:
			moduleValuesChanged <- moduleHook.Module.Name
		}
	}

	return nil
}
