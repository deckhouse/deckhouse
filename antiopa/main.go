package main

import (
	_ "encoding/json"
	_ "github.com/evanphx/json-patch"
	_ "gopkg.in/yaml.v2"
	_ "os/exec"
	_ "path/filepath"
	_ "strings"

	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/deckhouse/deckhouse/antiopa/docker_registry_manager"
	//	_ "github.com/deckhouse/deckhouse/antiopa/docker_registry_manager"
	//	_ "github.com/deckhouse/deckhouse/antiopa/helm"
	"github.com/deckhouse/deckhouse/antiopa/kube"
	//	_ "github.com/deckhouse/deckhouse/antiopa/kube_config_manager"
	//	_ "github.com/deckhouse/deckhouse/antiopa/kube_node_manager"
	//	_ "github.com/deckhouse/deckhouse/antiopa/kube_values_manager"
	//	_ "github.com/deckhouse/deckhouse/antiopa/merge_values"
	"github.com/deckhouse/deckhouse/antiopa/module_manager"
	//	_ "github.com/deckhouse/deckhouse/antiopa/module"
	"github.com/deckhouse/deckhouse/antiopa/helm"
	"github.com/deckhouse/deckhouse/antiopa/task"

	"github.com/romana/rlog"
)

var (
	WorkingDir string
	TempDir    string

	// Имя хоста совпадает с именем пода. Можно использовать для запросов API
	Hostname string

	// Имя файла, в который будет сбрасываться очередь
	TasksQueueDumpFilePath string

	// Очередь задач
	TasksQueue *task.TasksQueue
)

// Задержки при обработке тасков из очереди
const (
	EmptyQueueDelay   = time.Duration(5 * time.Second)
	FailedHookDelay   = time.Duration(5 * time.Second)
	FailedModuleDelay = time.Duration(5 * time.Second)
)

// Собрать настройки - директории, имя хоста, файл с дампом, namespace для tiller
// Проинициализировать все нужные объекты: helm, registry manager, module manager,
// kube events manager
// Создать пустую очередь с заданиями.
func Init() {
	rlog.Debug("Init")

	var err error

	WorkingDir, err = os.Getwd()
	if err != nil {
		rlog.Errorf("MAIN Fatal: Cannot determine antiopa working dir: %s", err)
		os.Exit(1)
	}
	rlog.Debugf("Antiopa working dir: %s", WorkingDir)

	TempDir, err = ioutil.TempDir("", "antiopa-")
	if err != nil {
		rlog.Errorf("MAIN Fatal: Cannot create antiopa temporary dir: %s", err)
		os.Exit(1)
	}
	rlog.Debugf("Antiopa temporary dir: %s", TempDir)

	Hostname, err = os.Hostname()
	if err != nil {
		rlog.Errorf("MAIN Fatal: Cannot get pod name from hostname: %s", err)
		os.Exit(1)
	}
	rlog.Debugf("Antiopa hostname: %s", Hostname)

	// Инициализация слежения за образом
	// TODO Antiopa может и не следить, если кластер заморожен?
	err = docker_registry_manager.InitRegistryManager(Hostname)
	if err != nil {
		rlog.Errorf("MAIN Fatal: Cannot initialize registry manager: %s", err)
		os.Exit(1)
	}

	// Инициализация helm — установка tiller, если его нет
	// TODO как получить tiller namespace?
	tillerNamespace := ""
	rlog.Debugf("Antiopa tiller namespace: %s", tillerNamespace)
	helm.Init(tillerNamespace)

	// Инициализация слежения за конфигом и за values
	err = module_manager.Init(WorkingDir, TempDir)
	if err != nil {
		rlog.Errorf("MAIN Fatal: Cannot initialize module manager: %s", err)
		os.Exit(1)
	}

	// TODO Инициализация слежения за событиями из kube
	// нужно по конфигам хуков создать настройки в менеджере
	// связать настройку и имя хука
	// потом, когда от менеджера придёт id настройки,
	// найти по id нужные имена хуков и добавить их запуск в очередь
	/* Примерный алгоритм поиска всех привязок по всем хукам, как глобальным, так и модульным:
	   GetModuleNamesInOrder.each {
	       GetModuleHooksInOrder(moduleName, module.Schedule).each {
	           schedule.add hook // регистрация binding
	       }

	       GetModuleHooksInOrder(moduleName, module.OnKubeNodeChange).each {
	           ... // регистрация binding
	       }
	   }

	   GetGlobalHooksInOrder(module.OnKubeNodeChange).each {...} // регистрация binding

	   GetGlobalHooksInOrder(module.OnStartup).each {RunGlobalHook(name)} // запуск по binding
	*/

	// Пустая очередь задач
	TasksQueue = task.NewTasksQueue(TasksQueueDumpFilePath)
}

// Run запускает все менеджеры, обработчик событий от менеджеров и обработчик очереди.
// Основной процесс блокируется for-select-ом в обработчике очереди.
func Run() {
	rlog.Info("MAIN: run main loop")

	// слежение за изменениями в очереди - сброс дампа в файл
	go TasksQueueDumper()

	// менеджеры - отдельные go-рутины, посылающие события в свои каналы
	go docker_registry_manager.RunRegistryManager()
	go module_manager.RunModuleManager()

	// обработчик событий от менеджеров — события превращаются в таски и
	// добавляются в очередь
	go ManagersEventsHandler()

	// TasksRunner не запускается go-рутиной, т.к. в main нет блокировки.
	// Можно в main добавить блокировку, например, от сигнала SIGTERM, тогда тут будет go-рутина.
	TasksRunner()

	/* TODO Первый запуск - добавление в очередь хуков on startup, добавление хуков beforeAll, после чего добавление всех модулей
			GetGlobalHooksInOrder(module.onstartup).each{RunGlobalHook(name)}
	   	   GetGlobalHooksInOrder(module.BeforeAll).each {RunGlobalHook(name)}
		   GetModuleNamesInOrder.each {RunModule(name)}
		   GetGlobalHooksInOrder(module.AfterAll).each {RunGlobalHook(name)}
	*/
}

func ManagersEventsHandler() {
	for {
		select {
		// Образ antiopa изменился, нужен рестарт деплоймента (можно и не выходить)
		case newImageId := <-docker_registry_manager.ImageUpdated:
			err := kube.KubeUpdateDeployment(newImageId)
			if err == nil {
				rlog.Infof("KUBE deployment update successful, exiting ...")
				os.Exit(1)
			} else {
				rlog.Errorf("KUBE deployment update error: %s", err)
			}
		// пришло событие от module_manager → перезапуск модулей или всего
		case moduleEvent := <-module_manager.EventCh:
			switch moduleEvent.Type {
			// Изменились отдельные модули
			case module_manager.ModulesChanged:
				rlog.Debug("main got ModulesChanged event")
				for _, moduleChange := range moduleEvent.ModulesChanges {
					switch moduleChange.ChangeType {
					case module_manager.Enabled:
						newTask := task.NewTask(task.ModuleRun, moduleChange.Name)
						TasksQueue.Add(newTask)
					case module_manager.Disabled:
						newTask := task.NewTask(task.ModuleDelete, moduleChange.Name)
						TasksQueue.Add(newTask)
					case module_manager.Changed:
						newTask := task.NewTask(task.ModuleUpgrade, moduleChange.Name)
						TasksQueue.Add(newTask)
					}
				}
			// Изменились глобальные values, нужен рестарт всех модулей
			case module_manager.GlobalChanged:
				rlog.Debug("main got GlobalChanged event")
				moduleNames := module_manager.GetModuleNamesInOrder()
				// TODO добавить beforeAll, afterAll!!!
				for _, moduleName := range moduleNames {
					newTask := task.NewTask(task.ModuleRun, moduleName)
					TasksQueue.Add(newTask)
				}
			}
		}
	}
}

// Обработчик один на очередь.
// Обработчик может отложить обработку следующего таска с помощью пуша в начало очереди таска задержки
func TasksRunner() {
	for {
		if TasksQueue.IsEmpty() {
			TasksQueue.Push(task.NewTaskDelay(EmptyQueueDelay))
			continue
		}
		headTask, _ := TasksQueue.Peek()
		if t, ok := headTask.(task.Task); ok {
			switch t.Type {
			case task.Module:
				// TODO реализовать RunModule
				err := RunModule(t.Name)
				if err != nil {
					t.IncrementFailureCount()
					rlog.Debugf("%s '%s' failed. Will retry after delay. Failed count is %d", t.Type, t.Name, t.FailureCount)
					TasksQueue.Push(task.NewTaskDelay(FailedModuleDelay))
				} else {
					TasksQueue.Pop()
				}
			case task.ModuleRun:
				// TODO реализовать RunModule
				err := RunModule(t.Name)
				if err != nil {
					t.IncrementFailureCount()
					rlog.Debugf("%s '%s' failed. Will retry after delay. Failed count is %d", t.Type, t.Name, t.FailureCount)
					TasksQueue.Push(task.NewTaskDelay(FailedModuleDelay))
				} else {
					TasksQueue.Pop()
				}
			case task.ModuleDelete:
				// TODO реализовать RunModule
				err := DeleteModule(t.Name)
				if err != nil {
					t.IncrementFailureCount()
					rlog.Debugf("%s '%s' failed. Will retry after delay. Failed count is %d", t.Type, t.Name, t.FailureCount)
					TasksQueue.Push(task.NewTaskDelay(FailedModuleDelay))
				} else {
					TasksQueue.Pop()
				}
			case task.ModuleUpgrade:
				// TODO реализовать RunModule
				err := UpgradeModule(t.Name)
				if err != nil {
					t.IncrementFailureCount()
					rlog.Debugf("%s '%s' failed. Will retry after delay. Failed count is %d", t.Type, t.Name, t.FailureCount)
					TasksQueue.Push(task.NewTaskDelay(FailedModuleDelay))
				} else {
					TasksQueue.Pop()
				}
			case task.Hook:
				// TODO реализовать RunHook
				err := RunHook(t.Name)
				if err != nil {
					t.IncrementFailureCount()
					rlog.Debugf("%s '%s' failed. Will retry after delay. Failed count is %d", t.Type, t.Name, t.FailureCount)
					TasksQueue.Push(task.NewTaskDelay(FailedHookDelay))
				} else {
					TasksQueue.Pop()
				}
			case task.Delay:
				td := headTask.(task.TaskDelay)
				time.Sleep(td.Delay)
				TasksQueue.Pop()
			}
		}
	}
}

// Дампер очереди в файл. Пока получается синхронно всё: Изменилась очередь, ждём, пока сдампается в файл.
func TasksQueueDumper() {
	for {
		select {
		case <-TasksQueue.EventCh():
			// Сдампить очередь в файл
			f, err := os.Create(TasksQueue.DumpFileName)
			if err != nil {
				fmt.Printf("Cannot open %s: %s\n", TasksQueue.DumpFileName, err)
			}
			_, err = io.Copy(f, TasksQueue.DumpReader())
			if err != nil {
				fmt.Printf("Cannot dump tasks to %s: %s\n", TasksQueue.DumpFileName, err)
			}
			f.Close()
			if err != nil {
				fmt.Printf("Cannot close %s: %s\n", TasksQueue.DumpFileName, err)
			}
		}
	}
}

func RunModule(moduleName string) (err error) {
	rlog.Infof("Module '%s': RUN", moduleName)
	return
}

func DeleteModule(moduleName string) (err error) {
	rlog.Infof("Module '%s': DELETE", moduleName)
	return
}

func UpgradeModule(moduleName string) (err error) {
	rlog.Infof("Module '%s': UPGRADE", moduleName)
	return
}

func RunHook(hookName string) (err error) {
	rlog.Infof("Hook '%s': RUN", hookName)
	return
}

func main() {
	Init()
	Run()
	//for {
	//	time.Sleep(time.Duration(1) * time.Second)
	//
	//	/*
	//		Initial run:
	//		* Append each global-hook with before-all binding to queue as separate task
	//		* Append each module from module.ModuleNamesOrder to queue
	//		    * append each before-helm module hook to queue as separate task
	//		    * append helm to queue as separate task
	//		    * append each after-helm module hook to queue as separate task
	//	*/
	//}
}

// func OnKubeNodeChanged() {
// 	rlog.Infof("Kube node change detected")

// 	if err := RunOnKubeNodeChangedHooks(); err != nil {
// 		rlog.Errorf("on-kube-node-change hooks error: %s", err)
// 		return
// 	}

// 	if valuesChanged {
// 		rlog.Debug("Global values changed: run all modules")
// 		RunModules()
// 	} else {
// 		for _, moduleName := range modulesOrder {
// 			if changed, exist := modulesValuesChanged[moduleName]; exist && changed {
// 				rlog.Debugf("Module `%s` values changed: run module", moduleName)
// 				RunModule(moduleName)
// 			}
// 		}
// 	}

// 	valuesChanged = false
// 	modulesValuesChanged = make(map[string]bool)
// }

// func Init() {
// rlog.Debug("Init")

// var err error

// WorkingDir, err = os.Getwd()
// if err != nil {
// 	rlog.Errorf("MAIN Fatal: Cannot determine antiopa working dir: %s", err)
// 	os.Exit(1)
// }

// TempDir, err = ioutil.TempDir("", "antiopa-")
// if err != nil {
// 	rlog.Errorf("MAIN Fatal: cannot create antiopa temporary dir: %s", err)
// 	os.Exit(1)
// }

// retryModulesNamesQueue = make([]string, 0)
// retryAll = false

// Hostname, err = os.Hostname()
// if err != nil {
// 	rlog.Errorf("MAIN Fatal: Cannot get pod name from hostname: %v", err)
// 	os.Exit(1)
// }

// InitKube()

// helm.Init(KubernetesAntiopaNamespace)

// // Initialize global enabled-modules index with descriptors
// modules, err := getEnabledModules()
// if err != nil {
// 	rlog.Errorf("Cannot detect enabled antiopa modules: %s", err)
// 	os.Exit(1)
// }
// if len(modules) == 0 {
// 	rlog.Warnf("No modules enabled")
// }
// modulesOrder = make([]string, 0)
// modulesByName = make(map[string]Module)
// for _, module := range modules {
// 	modulesByName[module.Name] = module
// 	modulesOrder = append(modulesOrder, module.Name)
// 	rlog.Debugf("Using module %s", module.Name)
// }

// globalConfigValues, err = readModulesValues()
// if err != nil {
// 	rlog.Errorf("Cannot read values: %s", err)
// 	os.Exit(1)
// }
// rlog.Debugf("Read global VALUES:\n%s", valuesToString(globalConfigValues))

// globalModulesConfigValues = make(map[string]map[interface{}]interface{})
// for _, module := range modulesByName {
// 	values, err := readModuleValues(module)
// 	if err != nil {
// 		rlog.Errorf("Cannot read module %s global values: %s", module.Name, err)
// 		os.Exit(1)
// 	}
// 	if values != nil {
// 		globalModulesConfigValues[module.Name] = values
// 		rlog.Debugf("Read module %s global VALUES:\n%s", module.Name, valuesToString(values))
// 	}
// }

// // TODO: remove InitKubeValuesManager
// res, err := InitKubeValuesManager()
// if err != nil {
// 	rlog.Errorf("Cannot initialize kube values manager: %s", err)
// 	os.Exit(1)
// }
// kubeConfigValues = res.Values
// kubeModulesConfigValues = res.ModulesValues
// rlog.Debugf("Read kube VALUES:\n%s", valuesToString(kubeConfigValues))
// for moduleName, kubeModuleValues := range kubeModulesConfigValues {
// 	rlog.Debugf("Read module %s kube VALUES:\n%s", moduleName, valuesToString(kubeModuleValues))
// }

// config, err := kube_config_manager.Init()
// if err != nil {
// 	rlog.Errorf("Cannot initialize kube config manager: %s", err)
// 	os.Exit(1)
// }
// _ = config
// // TODO: set config

// InitKubeNodeManager()

// err = InitRegistryManager()
// if err != nil {
// 	rlog.Errorf("Cannot initialize registry manager: %s", err)
// 	os.Exit(1)
// }

// dynamicValues = make(map[interface{}]interface{})
// modulesDynamicValues = make(map[string]map[interface{}]interface{})
// modulesValuesChanged = make(map[string]bool)
// }

// func Run() {
// 	rlog.Debug("Run")

// go RunKubeValuesManager()
// go RunKubeNodeManager()
// go RunRegistryManager()

// RunAll()

// retryTicker := time.NewTicker(time.Duration(30) * time.Second)

// for {
// 	select {
// 	case newKubevalues := <-KubeValuesUpdated:
// 		kubeConfigValues = newKubevalues.Values
// 		kubeModulesConfigValues = newKubevalues.ModulesValues

// 		rlog.Infof("Kube values has been updated, rerun all modules ...")

// 		RunModules()

// 	case moduleValuesUpdate := <-KubeModuleValuesUpdated:
// 		if _, hasKey := modulesByName[moduleValuesUpdate.ModuleName]; hasKey {
// 			kubeModulesConfigValues[moduleValuesUpdate.ModuleName] = moduleValuesUpdate.Values

// 			rlog.Infof("Module %s kube values has been updated, rerun ...", moduleValuesUpdate.ModuleName)

// 			RunModule(moduleValuesUpdate.ModuleName)
// 		}

// 	case <-KubeNodeChanged:
// 		OnKubeNodeChanged()

// 	case <-retryTicker.C:
// 		if retryAll {
// 			retryAll = false

// 			rlog.Infof("Retrying all modules ...")

// 			RunAll()
// 		} else if len(retryModulesNamesQueue) > 0 {
// 			retryModuleName := retryModulesNamesQueue[0]
// 			retryModulesNamesQueue = retryModulesNamesQueue[1:]

// 			rlog.Infof("Retrying module %s ...", retryModuleName)

// 			RunModule(retryModuleName)
// 		}

// 	case newImageId := <-ImageUpdated:
// 		err := KubeUpdateDeployment(newImageId)
// 		if err == nil {
// 			rlog.Infof("KUBE deployment update successful, exiting ...")
// 			os.Exit(1)
// 		} else {
// 			rlog.Errorf("KUBE deployment update error: %s", err)
// 		}
// 	}
// }
// }

// func RunAll() {
// if err := RunOnKubeNodeChangedHooks(); err != nil {
// 	retryAll = true
// 	rlog.Errorf("on-kube-node-change hooks error: %s", err)
// 	return
// }

// RunModules()
// }

// Вызов хуков при изменении опций узлов и самих узлов.
// Таким образом можно подтюнить узлы кластера.
// см. `/global-hooks/on-kube-node-change/*`

// func RunOnKubeNodeChangedHooks() error {
// 	globalHooksValuesPath, err := dumpGlobalHooksValuesYaml()
// 	if err != nil {
// 		return err
// 	}

// 	hooksDir := filepath.Join(WorkingDir, "global-hooks", "on-kube-node-change")
// 	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
// 		return nil
// 	}

// 	hooksNames, err := readDirectoryExecutableFilesNames(hooksDir)
// 	if err != nil {
// 		return err
// 	}

// 	for _, hookName := range hooksNames {
// 		rlog.Infof("Running global on-kube-node-change hook %s ...", hookName)

// 		configVJMV, configVJPV, dynamicVJMV, dynamicVJPV, err := runGlobalHook(hooksDir, hookName, globalHooksValuesPath)
// 		if err != nil {
// 			return err
// 		}

// 		var kubeConfigValuesChanged, dynamicValuesChanged bool

// 		if kubeConfigValues, kubeConfigValuesChanged, err = ApplyJsonMergeAndPatch(kubeConfigValues, configVJMV, configVJPV); err != nil {
// 			return err
// 		}

// 		if err := SetKubeValues(kubeConfigValues); err != nil {
// 			return err
// 		}

// 		if dynamicValues, dynamicValuesChanged, err = ApplyJsonMergeAndPatch(dynamicValues, dynamicVJMV, dynamicVJPV); err != nil {
// 			return err
// 		}

// 		valuesChanged = valuesChanged || kubeConfigValuesChanged || dynamicValuesChanged
// 	}

// 	return nil
// }
