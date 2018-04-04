package module

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

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

type EventType string

const (
	ModuleEnabled      EventType = "MODULE_ENABLED"
	ModuleChanged      EventType = "MODULE_CHANGED"
	ModuleDisabled     EventType = "MODULE_DISABLED"
	GlobalHooksChanged EventType = "GLOBAL_HOOKS_CHANGED"
)

type Event struct {
	ModuleNames []string
	Type        EventType
}

type GlobalHook struct {
	Hook
	Name string
}

func addGlobalHook(name string, config *GlobalHookConfig) {
	globalHook := &GlobalHook{Hook{}, name}

	if config.BeforeAll != nil {
		globalHook.Binding = append(globalHook.Binding, BeforeHelm)
		globalHook.OrderByBinding[BeforeAll] = *config.BeforeAll
		globalHooksOrder[BeforeAll] = append(globalHooksOrder[BeforeAll], globalHook)
	}

	if config.AfterAll != nil {
		globalHook.Binding = append(globalHook.Binding, AfterAll)
		globalHook.OrderByBinding[AfterAll] = *config.AfterAll
		globalHooksOrder[AfterAll] = append(globalHooksOrder[AfterAll], globalHook)
	}

	if config.OnKubeNodeChange != nil {
		globalHook.Binding = append(globalHook.Binding, OnKubeNodeChange)
		globalHook.OrderByBinding[OnKubeNodeChange] = *config.OnKubeNodeChange
		globalHooksOrder[OnKubeNodeChange] = append(globalHooksOrder[OnKubeNodeChange], globalHook)
	}

	if config.OnStartup != nil {
		globalHook.Binding = append(globalHook.Binding, OnStartup)
		globalHook.OrderByBinding[OnStartup] = *config.OnStartup
		globalHooksOrder[OnStartup] = append(globalHooksOrder[OnStartup], globalHook)
	}

	if config.Schedule != nil {
		globalHook.Binding = append(globalHook.Binding, Schedule)
		globalHook.Schedules = config.Schedule
		globalHooksOrder[Schedule] = append(globalHooksOrder[Schedule], globalHook)
	}

	globalHooksByName[name] = globalHook
}

type ModuleHook struct {
	Hook
	Name string
}

func addModuleHook(moduleName, name string, config *ModuleHookConfig) {
	moduleHook := &ModuleHook{Hook{}, name}

	if config.BeforeHelm != nil {
		moduleHook.Binding = append(moduleHook.Binding, BeforeHelm)
		moduleHook.OrderByBinding[BeforeHelm] = *config.BeforeHelm
		modulesHooksOrderByName[moduleName][BeforeHelm] = append(modulesHooksOrderByName[moduleName][BeforeHelm], moduleHook)
	}

	if config.AfterHelm != nil {
		moduleHook.Binding = append(moduleHook.Binding, AfterHelm)
		moduleHook.OrderByBinding[AfterHelm] = *config.AfterHelm
		modulesHooksOrderByName[moduleName][AfterHelm] = append(modulesHooksOrderByName[moduleName][AfterHelm], moduleHook)
	}

	if config.OnStartup != nil {
		moduleHook.Binding = append(moduleHook.Binding, OnStartup)
		moduleHook.OrderByBinding[OnStartup] = *config.OnStartup
		modulesHooksOrderByName[moduleName][OnStartup] = append(modulesHooksOrderByName[moduleName][OnStartup], moduleHook)
	}

	if config.Schedule != nil {
		moduleHook.Binding = append(moduleHook.Binding, Schedule)
		moduleHook.Schedules = config.Schedule
		modulesHooksOrderByName[moduleName][Schedule] = append(modulesHooksOrderByName[moduleName][Schedule], moduleHook)
	}

	modulesHooksByName[name] = moduleHook
}

type Hook struct {
	Binding        []BindingType
	OrderByBinding map[BindingType]int
	Schedules      []ScheduleConfig
}

type GlobalHookConfig struct { // для json
	HookConfig
	OnKubeNodeChange *int
	BeforeAll        *int
	AfterAll         *int
}

type ModuleHookConfig struct { // для json
	HookConfig
	BeforeHelm *int
	AfterHelm  *int
}

type HookConfig struct {
	OnStartup *int
	Schedule  []ScheduleConfig
}

type ScheduleConfig struct {
	Crontab      string
	AllowFailure bool
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
	moduleHooks := modulesHooksOrderByName[moduleName][bindingType]
	sort.Slice(moduleHooks[:], func(i, j int) bool {
		return moduleHooks[i].OrderByBinding[bindingType] < moduleHooks[j].OrderByBinding[bindingType]
	})

	var moduleHooksNames []string
	for _, moduleHook := range moduleHooks {
		moduleHooksNames = append(moduleHooksNames, moduleHook.Name)
	}

	return moduleHooksNames, nil
}

func GetModule(name string) (*Module, error) {
	module, exist := modulesByName[name]
	if exist {
		return module, nil
	} else {
		return nil, nil // TODO
	}
}

func GetGlobalHook(name string) (*GlobalHook, error) {
	globalHook, exist := globalHooksByName[name]
	if exist {
		return globalHook, nil
	} else {
		return nil, nil // TODO
	}
}

func GetModuleHook(name string) (*ModuleHook, error) {
	moduleHook, exist := modulesHooksByName[name]
	if exist {
		return moduleHook, nil
	} else {
		return nil, nil // TODO
	}
}

func RunModule(moduleName string) error { // запускает before-helm + helm + after-helm
	moduleHooksBeforeHelm, err := GetModuleHooksInOrder(moduleName, BeforeHelm)
	if err != nil {
		return err
	}

	for _, moduleHookName := range moduleHooksBeforeHelm {
		if err := RunModuleHook(moduleHookName); err != nil {
			return err
		}
	}

	// TODO RunHelm

	moduleHooksAfterHelm, err := GetModuleHooksInOrder(moduleName, AfterHelm)
	if err != nil {
		return err
	}

	for _, moduleHookName := range moduleHooksAfterHelm {
		if err := RunModuleHook(moduleHookName); err != nil {
			return err
		}
	}

	return nil
}

func RunGlobalHook(name string) error { return nil }
func RunModuleHook(name string) error { return nil }

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

func Init(workingDir string, tempDir string) error {
	TempDir = tempDir
	WorkingDir = workingDir

	if err := InitGlobalHooks(); err != nil {
		return err
	}

	if err := InitModules(); err != nil {
		return err
	}

	return nil
}

func InitGlobalHooks() error {
	globalHooksOrder = make(map[BindingType][]*GlobalHook)
	globalHooksByName = make(map[string]*GlobalHook)

	hooksDir := filepath.Join(WorkingDir, "global-hooks")

	err := initHooks(hooksDir, func(hookName string, output []byte) error {
		hookConfig := &GlobalHookConfig{}
		if err := yaml.Unmarshal(output, hookConfig); err != nil {
			return err
		}

		addGlobalHook(hookName, hookConfig)

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func InitModules() error {
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
				modulePath := filepath.Join(modulesDir, file.Name())

				module := &Module{
					Name:          matchRes[1],
					DirectoryName: file.Name(),
					Path:          modulePath,
				}

				moduleValues, err := readModuleValues(module)
				if err != nil {
					return err
				}

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

					if err = InitModuleHooks(module); err != nil {
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

func (m *Module) isEnabled() (bool, error) {
	enabledScriptPath := filepath.Join(m.DirectoryName, "enabled")

	_, err := os.Stat(enabledScriptPath)
	if os.IsNotExist(err) {
		return true, nil
	} else if err != nil {
		return false, err
	}

	// TODO: generate and pass enabled modules, modulesOrder
	if err := execCommand(makeCommand(m.Path, "", enabledScriptPath, []string{})); err != nil {
		return false, err
	}

	return true, nil
}

func InitModuleHooks(module *Module) error {
	hooksDir := filepath.Join(module.Path, "hooks")

	err := initHooks(hooksDir, func(hookName string, output []byte) error {
		hookConfig := &ModuleHookConfig{}
		if err := yaml.Unmarshal(output, hookConfig); err != nil {
			return err
		}

		addModuleHook(module.Name, hookName, hookConfig)

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

	hooksNames, err := readDirectoryExecutableFilesNames(hooksDir) // returns a list of executable hooks sorted by filename
	if err != nil {
		return err
	}

	for _, hookName := range hooksNames {
		// TODO: generate and pass values
		cmd := makeCommand(WorkingDir, "", filepath.Join(hooksDir, hookName), []string{"--config"})
		if err := execCommand(cmd); err != nil {
			return err
		}

		if output, err := cmd.Output(); err != nil {
			return err
		} else {
			if err := addHook(hookName, output); err != nil {
				return err
			}
		}
	}

	return nil
}

func Run() {
	for {
		time.Sleep(time.Duration(1) * time.Second)

		/*
		 * TODO: Watch kube_values_manager.ConfigUpdated
		 * TODO: Watch kube_values_manager.ModuleConfigUpdated
		 * TODO: Send events to EventCh
		 */
	}
}
