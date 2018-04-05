package module_manager

import (
	"encoding/json"
	"fmt"
	"github.com/romana/rlog"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	_ "github.com/deckhouse/deckhouse/antiopa/kube_config_manager"
	"github.com/deckhouse/deckhouse/antiopa/merge_values"
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
	EventCh <-chan Event

	// список модулей, найденных в инсталляции
	modulesByName map[string]*Module
	// список имен модулей в порядке вызова
	modulesOrder []string

	globalHooksByName map[string]*GlobalHook        // name -> Hook
	globalHooksOrder  map[BindingType][]*GlobalHook // это что-то внутреннее для быстрого поиска binding -> hooks names in order, можно и по-другому сделать

	modulesHooksByName      map[string]*ModuleHook
	modulesHooksOrderByName map[string]map[BindingType][]*ModuleHook

	// values для всех модулей, для всех кластеров
	globalConfigValues map[interface{}]interface{}
	// values для конкретного модуля, для всех кластеров
	globalModulesConfigValues map[string]map[interface{}]interface{}
	// values для всех модулей, для конкретного кластера
	kubeConfigValues map[interface{}]interface{}
	// values для конкретного модуля, для конкретного кластера
	kubeModulesConfigValues map[string]map[interface{}]interface{}
	// dynamic-values для всех модулей, для всех кластеров
	dynamicValues map[interface{}]interface{}
	// dynamic-values для конкретного модуля, для всех кластеров
	modulesDynamicValues map[string]map[interface{}]interface{}

	WorkingDir string
	TempDir    string
)

// TODO: remove this
var (
	retryModulesNamesQueue []string
	retryAll               bool

	valuesChanged        bool
	modulesValuesChanged map[string]bool
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
		time.Sleep(time.Duration(1) * time.Second)

		/*
		 * TODO: Watch kube_values_manager.ConfigUpdated
		 * TODO: Watch kube_values_manager.ModuleConfigUpdated
		 * TODO: Send events to EventCh
		 */
	}
}

func GetModuleNamesInOrder() []string {
	return modulesOrder
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

func GetModule(name string) (*Module, error) {
	module, exist := modulesByName[name]
	if exist {
		return module, nil
	} else {
		return nil, fmt.Errorf("module `%s` not found", name)
	}
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

func DeleteModule(moduleName string) error { return nil }

func RunModule(moduleName string) error { // запускает before-helm + helm + after-helm
	moduleHooksBeforeHelm, err := GetModuleHooksInOrder(moduleName, BeforeHelm)
	if err != nil {
		return err
	}

	for _, moduleHookName := range moduleHooksBeforeHelm {
		moduleHook, err := GetModuleHook(moduleHookName)
		if err != nil {
			return err
		}

		if err := moduleHook.run(); err != nil {
			return err
		}
	}

	// TODO RunHelm

	moduleHooksAfterHelm, err := GetModuleHooksInOrder(moduleName, AfterHelm)
	if err != nil {
		return err
	}

	for _, moduleHookName := range moduleHooksAfterHelm {
		moduleHook, err := GetModuleHook(moduleHookName)
		if err != nil {
			return err
		}

		if err := moduleHook.run(); err != nil {
			return err
		}
	}

	return nil
}

/* TODO:
Добавить DeleteModule(moduleName), который:
 * удаляет helm release (purge)
 * выполняет новый вид хука afterHelmDelete
*/

func RunGlobalHook(name string, binding BindingType) error { return nil }
func RunModuleHook(name string, binding BindingType) error { return nil }

func initGlobalHooks() error {
	rlog.Debug("Init global hooks")

	globalHooksOrder = make(map[BindingType][]*GlobalHook)
	globalHooksByName = make(map[string]*GlobalHook)

	hooksDir := filepath.Join(WorkingDir, "global-hooks")

	err := initHooks(hooksDir, func(hookName string, output []byte) error {
		hookConfig := &GlobalHookConfig{}
		if err := json.Unmarshal(output, hookConfig); err != nil {
			return fmt.Errorf("unmarshaling global hook `%s` json failed: %s", hookName, err.Error())
		}

		if err := addGlobalHook(hookName, hookConfig); err != nil {
			return fmt.Errorf("adding global hook `%s` failed: %s", hookName, err.Error())
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func initModules() error {
	rlog.Debug("Init modules")

	modulesByName = make(map[string]*Module)
	modulesHooksByName = make(map[string]*ModuleHook)
	modulesHooksOrderByName = make(map[string]map[BindingType][]*ModuleHook)

	modulesDir := filepath.Join(WorkingDir, "modules")

	files, err := ioutil.ReadDir(modulesDir) // returns a list of modules sorted by filename
	if err != nil {
		return fmt.Errorf("cannot list modules directory %s: %s", modulesDir, err)
	}

	modulesValues, err := readModulesValues()
	if err != nil {
		return err
	}

	var validModuleName = regexp.MustCompile(`^[0-9][0-9][0-9]-(.*)$`)

	badModulesDirs := make([]string, 0)

	for _, file := range files {
		if file.IsDir() {
			matchRes := validModuleName.FindStringSubmatch(file.Name())
			if matchRes != nil {
				moduleName := matchRes[1]
				modulePath := filepath.Join(modulesDir, file.Name())

				module := &Module{
					Name:          moduleName,
					DirectoryName: file.Name(),
					Path:          modulePath,
				}

				moduleValues, err := readModuleValues(module)
				if err != nil {
					return err
				}

				// TODO: change module enabled from values (global, module) logic
				moduleValues = merge_values.MergeValues(modulesValues, moduleValues)
				moduleEnabledValue := true
				if val, exist := moduleValues[module.Name]; exist {
					if boolVal, ok := val.(bool); ok {
						moduleEnabledValue = boolVal
					} else {
						// TODO ?!
					}
				}

				isEnabled, err := module.isEnabled()
				if err != nil {
					return err
				}

				if moduleEnabledValue && isEnabled {
					modulesByName[module.Name] = module
					modulesOrder = append(modulesOrder, module.Name)

					if err = initModuleHooks(module); err != nil {
						return err
					}
				}
			} else {
				badModulesDirs = append(badModulesDirs, filepath.Join(modulesDir, file.Name()))
			}
		}
	}

	if len(badModulesDirs) > 0 {
		return fmt.Errorf("bad module directory names, must match regex `%s`: %s", validModuleName, strings.Join(badModulesDirs, ", "))
	}

	return nil
}

func initModuleHooks(module *Module) error {
	hooksDir := filepath.Join(module.Path, "hooks")

	err := initHooks(hooksDir, func(hookName string, output []byte) error {
		hookConfig := &ModuleHookConfig{}
		if err := json.Unmarshal(output, hookConfig); err != nil {
			return fmt.Errorf("unmarshaling module hook `%s` json failed: %s", module.Name, err.Error())
		}

		if err := addModuleHook(module.Name, hookName, hookConfig); err != nil {
			return fmt.Errorf("adding module hook `%s` failed: %s", module.Name, err.Error())
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func initHooks(hooksDir string, addHook func(hookName string, output []byte) error) error {
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return nil
	}

	hooksRelativePaths, err := getExecutableFilesPaths(hooksDir) // returns a list of executable hooks sorted by filename
	if err != nil {
		return err
	}

	for _, hookPath := range hooksRelativePaths {
		hookName := filepath.Base(hookPath)

		cmd := makeCommand(WorkingDir, "", hookPath, []string{"--config"})
		output, err := execCommandOutput(cmd)
		if err != nil {
			return err
		}

		if err := addHook(hookName, output); err != nil {
			return err
		}
	}

	return nil
}
