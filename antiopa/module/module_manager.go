package module

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
  "time"
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
    BeforeHelm BindingType = "BEFORE_HELM"
    AfterHelm BindingType = "AFTER_HELM"
    BeforeAll BindingType = "BEFORE_ALL"
    AfterAll BindingType = "AFTER_ALL"
    OnKubeNodeChange BindingType = "ON_KUBE_NODE_CHANGE"
    Schedule BindingType = "SCHEDULE"
    OnStartup BindingType = "ON_STARTUP"
)

type EventType string

const (
    ModuleEnabled EventType = "MODULE_ENABLED"
    ModuleChanged EventType = "MODULE_CHANGED"
    ModuleDisabled EventType = "MODULE_DISABLED"
    GlobalHooksChanged EventType = "GLOBAL_HOOKS_CHANGED"
)

type Event struct {
    ModuleNames []string
    Type EventType
}

type GlobalHook struct {
    Binding []BindingType
    Name string
    Config GlobalHookConfig
}

type ModuleHook struct {
    Binding []BindingType // TODO: выделить общую часть с GlobalHook
    Name string
    Config ModuleHookConfig
}

type GlobalHookConfig struct { // для json
    //HookConfig
    OnKubeNodeChange *int
    BeforeAll *int
}

type ModuleHookConfig struct { // для json
    //HookConfig
    BeforeHelm *int
    AfterHelm *int
}

type HookConifig struct {
    OnStartup *int
    Schedule []ScheduleConfig
}

type ScheduleConfig struct {
    Crontab string
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

func GetModuleNamesInOrder() []string {return nil}
func GetGlobalHooksInOrder(bindingType BindingType) ([]string, error) {return nil, nil}
func GetModuleHooksInOrder(moduleName string, bindingType BindingType) ([]string, error) {return nil,nil}
func GetModule(name string) (*Module, error) {return nil, nil}
func GetGlobalHook(name string) (*GlobalHook, error) {return nil,nil}
func GetModuleHook(name string) (*ModuleHook, error) {return nil,nil}
func RunModule(moduleName string) error {return nil} // запускает before-helm + helm + after-helm
func RunGlobalHook(name string) error {return nil}
func RunModuleHook(name string) error {return nil}

var (
    EventCh <-chan Event
	// список имен модулей в порядке вызова
    modulesOrder []string

    globalHooks map[string]*Hook // name -> Hook
    goduleHooks map[string]*Hook // name -> Hook

    globalHooksOrder map[BindingType][]string // это что-то внутреннее для быстрого поиска binding -> hooks names in order, можно и по-другому сделать

	hooksByName                  map[string]Hook
	beforeHelmHooksOrderByModule map[string][]string
	afterHelmHooksOrderByModule  map[string][]string
	// список модулей, найденных в инсталляции
	modulesByName map[string]Module

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
	modulesDir := filepath.Join(WorkingDir, "modules")

	files, err := ioutil.ReadDir(modulesDir)
	if err != nil {
		return fmt.Errorf("cannot list modules directory %s: %s", modulesDir, err)
	}

	var validmoduleName = regexp.MustCompile(`^[0-9][0-9][0-9]-(.*)$`)

	res := make([]Module, 0)
	badModulesDirs := make([]string, 0)

	for _, file := range files {
		if file.IsDir() {
			matchRes := validmoduleName.FindStringSubmatch(file.Name())
			if matchRes != nil {
				module := Module{
					Name:          matchRes[1],
					DirectoryName: file.Name(),
					Path:          filepath.Join(modulesDir, file.Name()),
				}

				isEnabled, err := isModuleEnabled(module)
				if err != nil {
					return err
				}
				if isEnabled {
					res = append(res, module)
				}
			} else {
				badModulesDirs = append(badModulesDirs, filepath.Join(modulesDir, file.Name()))
			}
		}
	}

	if len(badModulesDirs) > 0 {
		return fmt.Errorf("bad module directory names, must match regex `%s`: %s", validmoduleName, strings.Join(badModulesDirs, ", "))
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

func isModuleEnabled(module Module) (bool, error) {
	enabledScriptPath := filepath.Join(module.DirectoryName, "enabled")

	_, err := os.Stat(enabledScriptPath)
	if os.IsNotExist(err) {
		return true, nil
	} else if err != nil {
		return false, err
	}

	return true, nil

	// TODO
	// cmd := exec.Command(enabledScriptPath, args...)
	// cmd.Env = append(cmd.Env, os.Environ()...)
	// cmd.Env = append(
	// 	cmd.Env,
	// 	fmt.Sprintf("TILLER_NAMESPACE=%s", HelmTillerNamespace()),
	// )
	// cmd.Dir = dir
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr
}
