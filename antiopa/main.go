package main

import (
	"flag"
	"io"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/romana/rlog"

	"github.com/deckhouse/deckhouse/antiopa/docker_registry_manager"
	"github.com/deckhouse/deckhouse/antiopa/executor"
	"github.com/deckhouse/deckhouse/antiopa/helm"
	"github.com/deckhouse/deckhouse/antiopa/kube"
	"github.com/deckhouse/deckhouse/antiopa/kube_events_manager"
	"github.com/deckhouse/deckhouse/antiopa/module_manager"
	"github.com/deckhouse/deckhouse/antiopa/schedule_manager"
	"github.com/deckhouse/deckhouse/antiopa/task"
	"github.com/deckhouse/deckhouse/antiopa/utils"
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

	// module manager object
	ModuleManager module_manager.ModuleManager

	// schedule manager
	ScheduleManager schedule_manager.ScheduleManager
	ScheduledHooks  ScheduledHooksStorage

	KubeEventsManager kube_events_manager.KubeEventsManager
	KubeEventsHooks   KubeEventsHooksController

	// chan for stopping ManagersEventsHandler infinite loop
	ManagersEventsHandlerStopCh chan struct{}

	// helm client object
	HelmClient helm.HelmClient
)

const DefaultTasksQueueDumpFilePath = "/tmp/antiopa-tasks-queue"

// Задержки при обработке тасков из очереди
var (
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

	// Инициализация хуков по расписанию - карта scheduleId → []ScheduleHook
	ScheduleManager, err = schedule_manager.Init()
	if err != nil {
		rlog.Errorf("MAIN Fatal: Cannot initialize schedule manager: %s", err)
		os.Exit(1)
	}

	KubeEventsManager, err = kube_events_manager.Init()
	if err != nil {
		rlog.Errorf("MAIN Fatal: Cannot initialize kube events manager: %s", err)
		os.Exit(1)
	}
	KubeEventsHooks = NewMainKubeEventsHooksController()
}

// Run запускает все менеджеры, обработчик событий от менеджеров и обработчик очереди.
// Основной процесс блокируется for-select-ом в обработчике очереди.
func Run() {
	rlog.Info("MAIN: run main loop")

	// Загрузить в очередь onStartup хуки и запуск всех модулей.
	// слежение за измененияи включить только после всей загрузки
	rlog.Info("MAIN: add onStartup, beforeAll, module and afterAll tasks")
	TasksQueue.ChangesDisable()

	CreateOnStartupTasks()
	CreateReloadAllTasks()

	KubeEventsHooks.EnableGlobalHooks(ModuleManager, KubeEventsManager)

	TasksQueue.ChangesEnable(true)

	// менеджеры - отдельные go-рутины, посылающие события в свои каналы
	go docker_registry_manager.RunRegistryManager()
	go ModuleManager.Run()
	go ScheduleManager.Run()

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
				rlog.Debug("main: got ModulesChanged event")
				for _, moduleChange := range moduleEvent.ModulesChanges {
					switch moduleChange.ChangeType {
					case module_manager.Enabled:
						newTask := task.NewTask(task.ModuleRun, moduleChange.Name)
						TasksQueue.Add(newTask)

						err := KubeEventsHooks.EnableModuleHooks(moduleChange.Name, ModuleManager, KubeEventsManager)
						if err != nil {
							rlog.Errorf("main: cannot enable module '%s' hooks: %s", moduleChange.Name, err)
						}

					case module_manager.Changed:
						newTask := task.NewTask(task.ModuleRun, moduleChange.Name)
						TasksQueue.Add(newTask)

					case module_manager.Disabled:
						newTask := task.NewTask(task.ModuleDelete, moduleChange.Name)
						TasksQueue.Add(newTask)

						err := KubeEventsHooks.DisableModuleHooks(moduleChange.Name, ModuleManager, KubeEventsManager)
						if err != nil {
							rlog.Errorf("main: cannot enable module '%s' hooks: %s", moduleChange.Name, err)
						}

					case module_manager.Purged:
						newTask := task.NewTask(task.ModulePurge, moduleChange.Name)
						TasksQueue.Add(newTask)

						err := KubeEventsHooks.DisableModuleHooks(moduleChange.Name, ModuleManager, KubeEventsManager)
						if err != nil {
							rlog.Errorf("main: cannot enable module '%s' hooks: %s", moduleChange.Name, err)
						}
					}
				}
				// Поменялись модули, нужно пересоздать индекс хуков по расписанию
				ScheduledHooks = UpdateScheduleHooks(ScheduledHooks)
			// Изменились глобальные values, нужен рестарт всех модулей
			case module_manager.GlobalChanged:
				rlog.Debug("main: got GlobalChanged event")
				TasksQueue.ChangesDisable()
				CreateReloadAllTasks()
				TasksQueue.ChangesEnable(true)
				// Пересоздать индекс хуков по расписанию
				ScheduledHooks = UpdateScheduleHooks(ScheduledHooks)
			}
		case crontab := <-schedule_manager.ScheduleCh:
			scheduleHooks := ScheduledHooks.GetHooksForSchedule(crontab)
			for _, hook := range scheduleHooks {
				var getHookErr error

				_, getHookErr = ModuleManager.GetGlobalHook(hook.Name)
				if getHookErr == nil {
					for _, scheduleConfig := range hook.Schedule {
						newTask := task.NewTask(task.GlobalHookRun, hook.Name).
							WithBinding(module_manager.Schedule).
							WithAllowFailure(scheduleConfig.AllowFailure)
						rlog.Debugf("Schedule: queued global hook '%s'", hook.Name)
						TasksQueue.Add(newTask)
					}
					continue
				}

				_, getHookErr = ModuleManager.GetModuleHook(hook.Name)
				if getHookErr == nil {
					for _, scheduleConfig := range hook.Schedule {
						newTask := task.NewTask(task.ModuleHookRun, hook.Name).
							WithBinding(module_manager.Schedule).
							WithAllowFailure(scheduleConfig.AllowFailure)
						rlog.Debugf("Schedule: queued hook '%s'", hook.Name)
						TasksQueue.Add(newTask)
					}
					continue
				}

				rlog.Errorf("hook '%s' scheduled but not found by module_manager", hook.Name)
			}
		case configId := <-kube_events_manager.KubeEventCh:
			rlog.Debugf("main: got kube event '%s'", configId)

			res, err := KubeEventsHooks.HandleEvent(configId)
			if err != nil {
				rlog.Errorf("main: error handling kube event '%s': %s", configId, err)
				break
			}

			for _, task := range res.Tasks {
				TasksQueue.Add(task)
				rlog.Debugf("main: queued %s '%s' with binding %s", task.GetType(), task.GetName(), task.GetBinding())
			}
		case <-ManagersEventsHandlerStopCh:
			return
		}
	}
}

func runDiscoverModulesState(_ task.Task) error {
	modulesState, err := ModuleManager.DiscoverModulesState()
	if err != nil {
		return err
	}

	for _, moduleName := range modulesState.EnabledModules {
		newTask := task.NewTask(task.ModuleRun, moduleName)
		rlog.Debugf("DiscoverModulesState: queued module run '%s'", moduleName)
		TasksQueue.Add(newTask)
	}

	for _, moduleName := range modulesState.ModulesToDisable {
		newTask := task.NewTask(task.ModuleDelete, moduleName)
		TasksQueue.Add(newTask)
		rlog.Debugf("DiscoverModulesState: queued module delete for disabled module '%s'", moduleName)
	}

	for _, moduleName := range modulesState.ReleasedUnknownModules {
		newTask := task.NewTask(task.ModulePurge, moduleName)
		TasksQueue.Add(newTask)
		rlog.Debugf("DiscoverModulesState: queued module purge for unknown module '%s'", moduleName)
	}

	// Queue afterAll global hooks
	afterAllHooks := ModuleManager.GetGlobalHooksInOrder(module_manager.AfterAll)
	for _, hookName := range afterAllHooks {
		newTask := task.NewTask(task.GlobalHookRun, hookName).WithBinding(module_manager.AfterAll)
		TasksQueue.Add(newTask)
		rlog.Debugf("DiscoverModulesState: queued global %s hook '%s'", module_manager.AfterAll, hookName)
	}

	ScheduledHooks = UpdateScheduleHooks(nil)

	// Enable kube events hooks for newly enabled modules
	for _, moduleName := range modulesState.EnabledModules {
		err = KubeEventsHooks.EnableModuleHooks(moduleName, ModuleManager, KubeEventsManager)
		if err != nil {
			return err
		}
	}

	// Disable kube events hooks for newly disabled modules
	for _, moduleName := range modulesState.ModulesToDisable {
		err = KubeEventsHooks.DisableModuleHooks(moduleName, ModuleManager, KubeEventsManager)
		if err != nil {
			return err
		}
	}

	return nil
}

// Обработчик один на очередь.
// Обработчик может отложить обработку следующего таска с помощью пуша в начало очереди таска задержки
// TODO пока только один обработчик, всё ок. Но лучше, чтобы очередь позволяла удалять только то, чему ранее был сделан peek.
// Т.е. кто взял в обработку задание, тот его и удалил из очереди. Сейчас Peek-нуть может одна го-рутина, другая добавит,
// первая Pop-нет задание — новое задание пропало, второй раз будет обработано одно и тоже.
func TasksRunner() {
	for {
		if TasksQueue.IsEmpty() {
			time.Sleep(QueueIsEmptyDelay)
		}
		for {
			t, _ := TasksQueue.Peek()
			if t == nil {
				break
			}

			switch t.GetType() {
			case task.DiscoverModulesState:
				err := runDiscoverModulesState(t)

				if err != nil {
					t.IncrementFailureCount()
					rlog.Errorf("%s failed. Will retry after delay. Failed count is %d. Error: %s", t.GetType(), t.GetFailureCount(), err)
					TasksQueue.Push(task.NewTaskDelay(FailedModuleDelay))
					break
				}

				TasksQueue.Pop()

			case task.ModuleRun:
				err := ModuleManager.RunModule(t.GetName())
				if err != nil {
					t.IncrementFailureCount()
					rlog.Errorf("%s '%s' failed. Will retry after delay. Failed count is %d. Error: %s", t.GetType(), t.GetName(), t.GetFailureCount(), err)
					TasksQueue.Push(task.NewTaskDelay(FailedModuleDelay))
				} else {
					TasksQueue.Pop()
				}
			case task.ModuleDelete:
				err := ModuleManager.DeleteModule(t.GetName())
				if err != nil {
					t.IncrementFailureCount()
					rlog.Errorf("%s '%s' failed. Will retry after delay. Failed count is %d. Error: %s", t.GetType(), t.GetName(), t.GetFailureCount(), err)
					TasksQueue.Push(task.NewTaskDelay(FailedModuleDelay))
				} else {
					TasksQueue.Pop()
				}
			case task.ModuleHookRun:
				err := ModuleManager.RunModuleHook(t.GetName(), t.GetBinding())
				if err != nil && !t.GetAllowFailure() {
					t.IncrementFailureCount()
					rlog.Errorf("%s '%s' failed. Will retry after delay. Failed count is %d. Error: %s", t.GetType(), t.GetName(), t.GetFailureCount(), err)
					TasksQueue.Push(task.NewTaskDelay(FailedModuleDelay))
				} else {
					TasksQueue.Pop()
				}
			case task.GlobalHookRun:
				err := ModuleManager.RunGlobalHook(t.GetName(), t.GetBinding())
				if err != nil && !t.GetAllowFailure() {
					t.IncrementFailureCount()
					rlog.Errorf("%s '%s' on '%s' failed. Will retry after delay. Failed count is %d. Error: %s", t.GetType(), t.GetName(), t.GetBinding(), t.GetFailureCount(), err)
					TasksQueue.Push(task.NewTaskDelay(FailedHookDelay))
				} else {
					TasksQueue.Pop()
				}
			case task.ModulePurge:
				// если вызван purge, то про модуль ничего неизвестно, поэтому ошибку
				// удаления достаточно записать в лог
				err := HelmClient.DeleteRelease(t.GetName())
				if err != nil {
					rlog.Errorf("%s helm delete '%s' failed. Error: %s", t.GetType(), t.GetName(), err)
				}
				TasksQueue.Pop()
			case task.Delay:
				TasksQueue.Pop()
				time.Sleep(t.GetDelay())
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
type ScheduleHook struct {
	Name     string
	Schedule []module_manager.ScheduleConfig
}

type ScheduledHooksStorage []*ScheduleHook

// Возврат всех расписаний из хранилища хуков
func (s ScheduledHooksStorage) GetCrontabs() []string {
	resMap := map[string]bool{}
	for _, hook := range s {
		for _, schedule := range hook.Schedule {
			resMap[schedule.Crontab] = true
		}
	}

	res := make([]string, len(resMap))
	for k := range resMap {
		res = append(res, k)
	}
	return res
}

// Возврат хуков, у которых есть переданное расписание
func (s ScheduledHooksStorage) GetHooksForSchedule(crontab string) []*ScheduleHook {
	res := []*ScheduleHook{}

	for _, hook := range s {
		newHook := &ScheduleHook{
			Name:     hook.Name,
			Schedule: []module_manager.ScheduleConfig{},
		}
		for _, schedule := range hook.Schedule {
			if schedule.Crontab == crontab {
				newHook.Schedule = append(newHook.Schedule, schedule)
			}
		}

		if len(newHook.Schedule) > 0 {
			res = append(res, newHook)
		}
	}

	return res
}

// Добавить хук в список хуков, выполняемых по расписанию
func (s *ScheduledHooksStorage) AddHook(hookName string, config []module_manager.ScheduleConfig) {
	for i, hook := range *s {
		if hook.Name == hookName {
			// Если хук уже есть, то изменить ему конфиг и выйти
			(*s)[i].Schedule = []module_manager.ScheduleConfig{}
			for _, item := range config {
				(*s)[i].Schedule = append((*s)[i].Schedule, item)
			}
			return
		}
	}

	newHook := &ScheduleHook{
		Name:     hookName,
		Schedule: []module_manager.ScheduleConfig{},
	}
	for _, item := range config {
		newHook.Schedule = append(newHook.Schedule, item)
	}
	*s = append(*s, newHook)

}

// Удалить сведения о хуке из хранилища
func (s *ScheduledHooksStorage) RemoveHook(hookName string) {
	tmp := ScheduledHooksStorage{}
	for _, hook := range *s {
		if hook.Name == hookName {
			continue
		}
		tmp = append(tmp, hook)
	}

	*s = tmp
}

// Создать новый набор ScheduledHooks
// вычислить разницу в ScheduledId между старым набором и новым.
// то, что было в старом наборе, но отсутстует в новом — удалить из ScheduleManager
//
func UpdateScheduleHooks(storage ScheduledHooksStorage) ScheduledHooksStorage {
	if ScheduleManager == nil {
		return nil
	}

	oldCrontabs := map[string]bool{}
	if storage != nil {
		for _, crontab := range storage.GetCrontabs() {
			oldCrontabs[crontab] = false
		}
	}

	newScheduledTasks := ScheduledHooksStorage{}

	globalHooks := ModuleManager.GetGlobalHooksInOrder(module_manager.Schedule)
LOOP_GLOBAL_HOOKS:
	for _, globalHookName := range globalHooks {
		globalHook, _ := ModuleManager.GetGlobalHook(globalHookName)
		for _, schedule := range globalHook.Config.Schedule {
			_, err := ScheduleManager.Add(schedule.Crontab)
			if err != nil {
				rlog.Errorf("Schedule: cannot add '%s' for global hook '%s': %s", schedule.Crontab, globalHookName, err)
				continue LOOP_GLOBAL_HOOKS
			}
			rlog.Debugf("Schedule: add '%s' for global hook '%s'", schedule.Crontab, globalHookName)
		}
		newScheduledTasks.AddHook(globalHook.Name, globalHook.Config.Schedule)
	}

	modules := ModuleManager.GetModuleNamesInOrder()
	for _, moduleName := range modules {
		moduleHooks, _ := ModuleManager.GetModuleHooksInOrder(moduleName, module_manager.Schedule)
	LOOP_MODULE_HOOKS:
		for _, moduleHookName := range moduleHooks {
			moduleHook, _ := ModuleManager.GetModuleHook(moduleHookName)
			for _, schedule := range moduleHook.Config.Schedule {
				_, err := ScheduleManager.Add(schedule.Crontab)
				if err != nil {
					rlog.Errorf("Schedule: cannot add '%s' for hook '%s': %s", schedule.Crontab, moduleHookName, err)
					continue LOOP_MODULE_HOOKS
				}
				rlog.Debugf("Schedule: add '%s' for hook '%s'", schedule.Crontab, moduleHookName)
			}
			newScheduledTasks.AddHook(moduleHook.Name, moduleHook.Config.Schedule)
		}
	}

	if len(oldCrontabs) > 0 {
		// Собрать новый набор расписаний. Если расписание есть в oldCrontabs, то поставить ему true.
		newCrontabs := newScheduledTasks.GetCrontabs()
		for _, crontab := range newCrontabs {
			if _, has_crontab := oldCrontabs[crontab]; has_crontab {
				oldCrontabs[crontab] = true
			}
		}

		// пройти по старому набору расписаний, если есть расписание с false, то удалить его из обработки.
		for crontab, _ := range oldCrontabs {
			if !oldCrontabs[crontab] {
				ScheduleManager.Remove(crontab)
			}
		}
	}

	return newScheduledTasks
}

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
		rlog.Debugf("ReloadAll: queued global %s hook '%s'", module_manager.BeforeAll, hookName)
	}

	TasksQueue.Add(task.NewTask(task.DiscoverModulesState, ""))
	rlog.Debugf("ReloadAll: queued discover of modules state")
}

func InitHttpServer() {
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte(`<html>
    <head><title>Antiopa</title></head>
    <body>
    <h1>Antiopa</h1>
    <pre>go tool pprof goprofex http://ANTIOPA_IP:9115/debug/pprof/profile</pre>
    </body>
    </html>`))
	})
	http.Handle("/metrics", promhttp.Handler())

	http.HandleFunc("/queue", func(writer http.ResponseWriter, request *http.Request) {
		io.Copy(writer, TasksQueue.DumpReader())
	})

	go func() {
		rlog.Info("Listening on :9115")
		if err := http.ListenAndServe(":9115", nil); err != nil {
			rlog.Error("Error starting HTTP server: %s", err)
		}
	}()
}

func main() {
	// set flag.Parsed() for glog
	flag.CommandLine.Parse([]string{})

	// Be a good parent - clean up behind the children processes.
	// Antiopa is PID1, no special config required
	go executor.Reap()

	// Включить Http сервер для pprof и prometheus client
	InitHttpServer()

	// настроить всё необходимое
	Init()

	// запустить менеджеры и обработчики
	Run()

	// Блокировка main на сигналах от os.
	utils.WaitForProcessInterruption()
}
