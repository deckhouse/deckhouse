package module_manager

import (
	"encoding/json"
	"fmt"
	"github.com/romana/rlog"
	"sort"

	"github.com/deckhouse/deckhouse/antiopa/utils"
)

/*

Module run:
* before-helm
* helm
* after-helm

All run:
* before-all
* run modules
* after-all

*/

var (
	EventCh chan Event

	// список модулей, найденных в инсталляции
	modulesByName map[string]*Module
	// список имен модулей в порядке вызова
	modulesOrder []string

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

// TODO: remove this
var (
	retryModulesNamesQueue []string
	retryAll               bool
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
	rlog.Debug("Init module manager")

	TempDir = tempDir
	WorkingDir = workingDir

	EventCh = make(chan Event, 1)
	globalValuesChanged = make(chan bool, 1)
	moduleValuesChanged = make(chan string, 1)

	if err := initGlobalHooks(); err != nil {
		return err
	}

	if err := initModules(); err != nil {
		return err
	}

	return nil
}

// Module manager loop
func RunModuleManager() {
	for {
		select {
		/*
		* TODO: Watch kube_values_manager.ConfigUpdated
		* TODO: Watch kube_values_manager.ModuleConfigUpdated
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
					ModuleChange{Name: moduleName, ChangeType: Changed},
				},
			}
		}
	}
}

func GetModule(name string) (*Module, error) {
	module, exist := modulesByName[name]
	if exist {
		return module, nil
	} else {
		return nil, fmt.Errorf("module `%s` not found", name)
	}
}

func GetModuleNamesInOrder() []string {
	return modulesOrder
}

func GetGlobalHook(name string) (*GlobalHook, error) {
	globalHook, exist := globalHooksByName[name]
	if exist {
		return globalHook, nil
	} else {
		return nil, fmt.Errorf("global hook `%s` not found", name)
	}
}

func GetModuleHook(name string) (*ModuleHook, error) {
	moduleHook, exist := modulesHooksByName[name]
	if exist {
		return moduleHook, nil
	} else {
		return nil, fmt.Errorf("module hook `%s` not found", name)
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
		return nil, fmt.Errorf("module `%s` not found", moduleName)
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

	if err := globalHook.run(); err != nil {
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

	if err := moduleHook.run(); err != nil {
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
