package main

import (
	"io/ioutil"
	"os"
	"time"

	"github.com/deckhouse/deckhouse/antiopa/docker_registry_manager"
	"github.com/deckhouse/deckhouse/antiopa/helm"
	"github.com/deckhouse/deckhouse/antiopa/kube"
	"github.com/deckhouse/deckhouse/antiopa/kube_node_manager"
	"github.com/deckhouse/deckhouse/antiopa/module_manager"
	"github.com/deckhouse/deckhouse/antiopa/task"
	"github.com/deckhouse/deckhouse/antiopa/utils"

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

	// TODO Когда будет schedule_manager - удалить
	schedule_manager_ScheduleEventCh chan struct{}

	// module manager object
	ModuleManager module_manager.ModuleManager

	// chan for stopping ManagersEventsHandler infinite loop
	ManagersEventsHandlerStopCh chan struct{}

	// helm client object
	HelmClient helm.HelmClient
)

const DefaultTasksQueueDumpFilePath = "/tmp/antiopa-tasks-queue"

// Задержки при обработке тасков из очереди
const (
	QueueIsEmptyDelay = 3 * time.Second
	FailedHookDelay   = 5 * time.Second
	FailedModuleDelay = 5 * time.Second
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

	// Инициализация подключения к kube
	kube.InitKube()

	// Инициализация слежения за образом
	// TODO Antiopa может и не следить, если кластер заморожен?
	err = docker_registry_manager.InitRegistryManager(Hostname)
	if err != nil {
		rlog.Errorf("MAIN Fatal: Cannot initialize registry manager: %s", err)
		os.Exit(1)
	}

	// Инициализация helm — установка tiller, если его нет
	// TODO KubernetesAntiopaNamespace — имя поменяется, это старая переменная
	tillerNamespace := kube.KubernetesAntiopaNamespace
	rlog.Debugf("Antiopa tiller namespace: %s", tillerNamespace)
	HelmClient, err = helm.Init(tillerNamespace)
	if err != nil {
		rlog.Errorf("MAIN Fatal: cannot initialize helm: %s", err)
		os.Exit(1)
	}

	// Инициализация слежения за конфигом и за values
	ModuleManager, err = module_manager.Init(WorkingDir, TempDir, HelmClient)
	if err != nil {
		rlog.Errorf("MAIN Fatal: Cannot initialize module manager: %s", err)
		os.Exit(1)
	}

	// Пустая очередь задач.
	TasksQueue = task.NewTasksQueue()

	// Дампер для сброса изменений в очереди во временный файл
	// TODO определить файл через переменную окружения?
	TasksQueueDumpFilePath = DefaultTasksQueueDumpFilePath
	rlog.Debugf("Antiopa tasks queue dump file '%s'", TasksQueueDumpFilePath)
	queueWatcher := task.NewTasksQueueDumper(TasksQueueDumpFilePath, TasksQueue)
	TasksQueue.AddWatcher(queueWatcher)

	// Инициализация слежения за событиями onKubeNodeChange (пока нет kube_event_manager)
	kube_node_manager.InitKubeNodeManager()

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

	// Инициализация хуков по расписанию - карта scheduleId → []ScheduleHook
	schedule_manager_ScheduleEventCh = make(chan struct{}, 1)
	RegisterScheduleHooks()
	// Инициализация хуков по событиям от kube - карта kubeEventId → []KubeEventHook
	// RegisterKubeEventHooks()
}

// Run запускает все менеджеры, обработчик событий от менеджеров и обработчик очереди.
// Основной процесс блокируется for-select-ом в обработчике очереди.
func Run() {
	rlog.Info("MAIN: run main loop")

	// Загрузить в очередь onStartup хуки и запуск всех модулей.
	// слежение за измененияи включить только после всей загрузки
	rlog.Info("MAIN: add onStartup, beforeAll, module and afterAll tasks")
	TasksQueue.ChangesDisable()
	CreateAfterInitTasks()
	CreateOnStartupTasks()
	CreateReloadAllTasks()
	TasksQueue.ChangesEnable(true)

	// менеджеры - отдельные go-рутины, посылающие события в свои каналы
	go docker_registry_manager.RunRegistryManager()
	go ModuleManager.Run()
	go kube_node_manager.RunKubeNodeManager()

	// обработчик событий от менеджеров — события превращаются в таски и
	// добавляются в очередь
	go ManagersEventsHandler()

	// TasksRunner запускает задания из очереди
	go TasksRunner()

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
			// событие от module_manager может прийти, если изменился состав модулей
			// поэтому нужно заново зарегистрировать событийные хуки
			// RegisterScheduledHooks()
			// RegisterKubeEventHooks()
			switch moduleEvent.Type {
			// Изменились отдельные модули
			case module_manager.ModulesChanged:
				rlog.Debug("main got ModulesChanged event")
				for _, moduleChange := range moduleEvent.ModulesChanges {
					switch moduleChange.ChangeType {
					case module_manager.Enabled, module_manager.Changed:
						newTask := task.NewTask(task.ModuleRun, moduleChange.Name)
						TasksQueue.Add(newTask)
					case module_manager.Disabled:
						newTask := task.NewTask(task.ModuleDelete, moduleChange.Name)
						TasksQueue.Add(newTask)
					case module_manager.Purged:
						newTask := task.NewTask(task.ModulePurge, moduleChange.Name)
						TasksQueue.Add(newTask)
					}
				}
			// Изменились глобальные values, нужен рестарт всех модулей
			case module_manager.GlobalChanged:
				rlog.Debug("main got GlobalChanged event")
				TasksQueue.ChangesDisable()
				CreateReloadAllTasks()
				TasksQueue.ChangesEnable(true)
			}
		case <-kube_node_manager.KubeNodeChanged:
			// Добавить выполнение глобальных хуков по событию KubeNodeChange
			TasksQueue.ChangesDisable()
			hookNames := ModuleManager.GetGlobalHooksInOrder(module_manager.OnKubeNodeChange)
			for _, hookName := range hookNames {
				newTask := task.NewTask(task.GlobalHookRun, hookName).WithBinding(module_manager.OnKubeNodeChange)
				TasksQueue.Add(newTask)
				rlog.Debugf("KubeNodeChange: queued global hook '%s'", hookName)
			}
			TasksQueue.ChangesEnable(true)
		// TODO поменять, когда появится schedule_manager
		//case scheduleId := <-schedule_manager.ScheduleEventCh:
		case scheduleId := <-schedule_manager_ScheduleEventCh:
			scheduleHooks := GetScheduleHooks(scheduleId)
			for _, hook := range scheduleHooks {
				var getHookErr error

				_, getHookErr = ModuleManager.GetGlobalHook(hook.Name)
				if getHookErr == nil {
					newTask := task.NewTask(task.GlobalHookRun, hook.Name).
						WithBinding(module_manager.Schedule).
						WithAllowFailure(hook.Schedule.AllowFailure)
					rlog.Debugf("Schedule: queued global hook '%s'", hook.Name)
					TasksQueue.Add(newTask)
					break
				}

				_, getHookErr = ModuleManager.GetModuleHook(hook.Name)
				if getHookErr == nil {
					newTask := task.NewTask(task.ModuleHookRun, hook.Name).
						WithBinding(module_manager.Schedule).
						WithAllowFailure(hook.Schedule.AllowFailure)
					rlog.Debugf("Schedule: queued hook '%s'", hook.Name)
					TasksQueue.Add(newTask)
					break
				}

				rlog.Errorf("hook '%s' scheduled but not found by module_manager", hook.Name)
			}
		case <-ManagersEventsHandlerStopCh:
			return
		}
	}
}

// Обработчик один на очередь.
// Обработчик может отложить обработку следующего таска с помощью пуша в начало очереди таска задержки
// TODO пока только один обработчик, всё ок. Но лучше, чтобы очередь позволяла удалять только то, чему ранее был сделан peek.
// Т.е. кто взял в обработку задание, тот его и удалил из очереди. Сейчас Peek-нуть может одна го-рутина, другая добавит,
// первая Pop-нет задание — новое задание пропало, второй раз будет обработано одно и тоже.
func TasksRunner() {
	for {
		if TasksQueue.IsEmpty() {
			rlog.Debugf("TasksRunner: queue is empty. Wait for tasks.")
			time.Sleep(QueueIsEmptyDelay)
		}
		for {
			t, _ := TasksQueue.Peek()
			if t == nil {
				break
			}

			switch t.GetType() {
			case task.ModuleRun:
				err := ModuleManager.RunModule(t.GetName())
				if err != nil {
					t.IncrementFailureCount()
					rlog.Debugf("%s '%s' failed. Will retry after delay. Failed count is %d", t.GetType(), t.GetName(), t.GetFailureCount())
					TasksQueue.Push(task.NewTaskDelay(FailedModuleDelay))
				} else {
					TasksQueue.Pop()
				}
			case task.ModuleDelete:
				err := ModuleManager.DeleteModule(t.GetName())
				if err != nil {
					t.IncrementFailureCount()
					rlog.Debugf("%s '%s' failed. Will retry after delay. Failed count is %d", t.GetType(), t.GetName(), t.GetFailureCount())
					TasksQueue.Push(task.NewTaskDelay(FailedModuleDelay))
				} else {
					TasksQueue.Pop()
				}
			case task.ModuleHookRun:
				err := ModuleManager.RunModuleHook(t.GetName(), t.GetBinding())
				if err != nil && !t.GetAllowFailure() {
					t.IncrementFailureCount()
					rlog.Debugf("%s '%s' failed. Will retry after delay. Failed count is %d", t.GetType(), t.GetName(), t.GetFailureCount())
					TasksQueue.Push(task.NewTaskDelay(FailedModuleDelay))
				} else {
					TasksQueue.Pop()
				}
			case task.GlobalHookRun:
				err := ModuleManager.RunGlobalHook(t.GetName(), t.GetBinding())
				if err != nil && !t.GetAllowFailure() {
					t.IncrementFailureCount()
					rlog.Debugf("%s '%s' on '%s' failed. Will retry after delay. Failed count is %d", t.GetType(), t.GetName(), t.GetBinding(), t.GetFailureCount())
					TasksQueue.Push(task.NewTaskDelay(FailedHookDelay))
				} else {
					TasksQueue.Pop()
				}
			case task.ModulePurge:
				// если вызван purge, то про модуль ничего неизвестно, поэтому ошибку
				// удаления достаточно записать в лог
				err := HelmClient.DeleteRelease(t.GetName())
				if err != nil {
					rlog.Errorf("Module purge for '%s' failed.", t.GetName())
				}
				TasksQueue.Pop()
			case task.Delay:
				time.Sleep(t.GetDelay())
				TasksQueue.Pop()
			case task.Stop:
				rlog.Infof("TaskRunner got stop task. Exiting runner loop.")
				TasksQueue.Pop()
				return
			}

			// break if empty to prevent infinity loop
			if TasksQueue.IsEmpty() {
				break
			}
		}
	}
}

// Работа с событийными хуками

type ScheduleIdType struct{}

type ScheduleHook struct {
	Name     string
	Schedule module_manager.ScheduleConfig
}

type KubeEventHook struct {
	Name           string
	OnCreateConfig struct{}
	OnChangeConfig struct{}
}

var ScheduleHooks map[ScheduleIdType][]*ScheduleHook

func GetScheduleHooks(scheduleId ScheduleIdType) []*ScheduleHook {
	return ScheduleHooks[scheduleId]
}

func RegisterScheduleHooks() {
	ScheduleHooks = make(map[ScheduleIdType][]*ScheduleHook)
	// Примерный алгоритм

	return
}

func CreateAfterInitTasks() {
	purgeModules := ModuleManager.GetModulesToPurgeOnInit()

	for _, moduleName := range purgeModules {
		newTask := task.NewTask(task.ModulePurge, moduleName)
		TasksQueue.Add(newTask)
		rlog.Debugf("AfterInit: queued module purge '%s'", moduleName)
	}

	deleteModules := ModuleManager.GetModulesToDisableOnInit()

	for _, moduleName := range deleteModules {
		newTask := task.NewTask(task.ModuleDelete, moduleName)
		TasksQueue.Add(newTask)
		rlog.Debugf("AfterInit: queued module delete '%s'", moduleName)
	}
}

/*
Первый запуск - добавление в очередь хуков on startup, добавление хуков beforeAll, после чего добавление всех модулей
	GetGlobalHooksInOrder(module.onstartup).each{RunGlobalHook(name)}
	  GetGlobalHooksInOrder(module.BeforeAll).each {RunGlobalHook(name)}
   GetModuleNamesInOrder.each {RunModule(name)}
   GetGlobalHooksInOrder(module.AfterAll).each {RunGlobalHook(name)}

		Initial run:
		* Append each global-hook with before-all binding to queue as separate task
		* Append each module from module.ModuleNamesOrder to queue
		    * append each before-helm module hook to queue as separate task
		    * append helm to queue as separate task
		    * append each after-helm module hook to queue as separate task
*/
func CreateOnStartupTasks() {
	onStartupHooks := ModuleManager.GetGlobalHooksInOrder(module_manager.OnStartup)

	for _, hookName := range onStartupHooks {
		newTask := task.NewTask(task.GlobalHookRun, hookName).WithBinding(module_manager.OnStartup)
		TasksQueue.Add(newTask)
		rlog.Debugf("OnStartup: queued global hook '%s'", hookName)
	}

	return
}

func CreateReloadAllTasks() {
	// Queue beforeAll global hooks
	beforeAllHooks := ModuleManager.GetGlobalHooksInOrder(module_manager.BeforeAll)

	for _, hookName := range beforeAllHooks {
		newTask := task.NewTask(task.GlobalHookRun, hookName).WithBinding(module_manager.BeforeAll)
		TasksQueue.Add(newTask)
		rlog.Debugf("ReloadAll BeforeAll: queued global hook '%s'", hookName)
	}

	// Queue modules
	moduleNames := ModuleManager.GetModuleNamesInOrder()
	for _, moduleName := range moduleNames {
		newTask := task.NewTask(task.ModuleRun, moduleName)
		rlog.Debugf("ReloadAll Module: queued module run '%s'", moduleName)
		TasksQueue.Add(newTask)
	}

	// Queue afterAll global hooks
	afterAllHooks := ModuleManager.GetGlobalHooksInOrder(module_manager.AfterAll)

	for _, hookName := range afterAllHooks {
		newTask := task.NewTask(task.GlobalHookRun, hookName).WithBinding(module_manager.AfterAll)
		TasksQueue.Add(newTask)
		rlog.Debugf("ReloadAll AfterAll: queued global hook '%s'", hookName)
	}

	return
}

func main() {
	// настроить всё необходимое
	Init()

	// запустить менеджеры и обработчики
	Run()

	// Блокировка main на сигналах от os.
	utils.WaitForProcessInterruption()
}
