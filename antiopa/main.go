package main

import (
	"flag"
	"io"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/romana/rlog"

	"github.com/deckhouse/deckhouse/antiopa/docker_registry_manager"
	"github.com/deckhouse/deckhouse/antiopa/executor"
	"github.com/deckhouse/deckhouse/antiopa/helm"
	"github.com/deckhouse/deckhouse/antiopa/kube"
	"github.com/deckhouse/deckhouse/antiopa/kube_events_manager"
	"github.com/deckhouse/deckhouse/antiopa/metrics_storage"
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

	// registry manager — watch for antiopa image updates
	RegistryManager docker_registry_manager.DockerRegistryManager

	// schedule manager
	ScheduleManager schedule_manager.ScheduleManager
	ScheduledHooks  ScheduledHooksStorage

	KubeEventsManager kube_events_manager.KubeEventsManager
	KubeEventsHooks   KubeEventsHooksController

	MetricsStorage *metrics_storage.MetricStorage

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
	rlog.Infof("Antiopa working dir: %s", WorkingDir)

	TempDir, err = ioutil.TempDir("", "antiopa-")
	if err != nil {
		rlog.Errorf("MAIN Fatal: Cannot create antiopa temporary dir: %s", err)
		os.Exit(1)
	}
	rlog.Infof("Antiopa temporary dir: %s", TempDir)

	Hostname, err = os.Hostname()
	if err != nil {
		rlog.Errorf("MAIN Fatal: Cannot get pod name from hostname: %s", err)
		os.Exit(1)
	}
	rlog.Infof("Antiopa hostname: %s", Hostname)

	// Инициализация подключения к kube
	kube.InitKube()

	// Инициализация слежения за образом
	// TODO Antiopa может и не следить, если кластер заморожен?
	RegistryManager, err = docker_registry_manager.Init(Hostname)
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

	MetricsStorage = metrics_storage.Init()
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

	if RegistryManager != nil {
		// менеджеры - отдельные go-рутины, посылающие события в свои каналы
		RegistryManager.SetErrorCallback(func() {
			MetricsStorage.SendCounterMetric("antiopa_registry_errors", 1.0, map[string]string{})
		})
		go RegistryManager.Run()
	}
	go ModuleManager.Run()
	go ScheduleManager.Run()

	// обработчик добавления метрик
	go MetricsStorage.Run()

	// обработчик событий от менеджеров — события превращаются в таски и
	// добавляются в очередь
	go ManagersEventsHandler()

	// TasksRunner запускает задания из очереди
	go TasksRunner()

	RunAntiopaMetrics()
}

func ManagersEventsHandler() {
	for {
		select {
		// Образ antiopa изменился, нужен рестарт деплоймента (можно и не выходить)
		case newImageId := <-docker_registry_manager.ImageUpdated:
			rlog.Infof("EVENT ImageUpdated")
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
				for _, moduleChange := range moduleEvent.ModulesChanges {
					switch moduleChange.ChangeType {
					case module_manager.Enabled:
						rlog.Infof("EVENT ModulesChanged, type=Enabled")
						newTask := task.NewTask(task.ModuleRun, moduleChange.Name)
						TasksQueue.Add(newTask)
						rlog.Infof("QUEUE add ModuleRun %s", newTask.Name)

						err := KubeEventsHooks.EnableModuleHooks(moduleChange.Name, ModuleManager, KubeEventsManager)
						if err != nil {
							rlog.Errorf("MAIN_LOOP module '%s' enabled: cannot enable hooks: %s", moduleChange.Name, err)
						}

					case module_manager.Changed:
						rlog.Infof("EVENT ModulesChanged, type=Changed")
						newTask := task.NewTask(task.ModuleRun, moduleChange.Name)
						TasksQueue.Add(newTask)
						rlog.Infof("QUEUE add ModuleRun %s", newTask.Name)

					case module_manager.Disabled:
						rlog.Infof("EVENT ModulesChanged, type=Disabled")
						newTask := task.NewTask(task.ModuleDelete, moduleChange.Name)
						TasksQueue.Add(newTask)
						rlog.Infof("QUEUE add ModuleDelete %s", newTask.Name)

						err := KubeEventsHooks.DisableModuleHooks(moduleChange.Name, ModuleManager, KubeEventsManager)
						if err != nil {
							rlog.Errorf("MAIN_LOOP module '%s' disabled: cannot disable hooks: %s", moduleChange.Name, err)
						}

					case module_manager.Purged:
						rlog.Infof("EVENT ModulesChanged, type=Purged")
						newTask := task.NewTask(task.ModulePurge, moduleChange.Name)
						TasksQueue.Add(newTask)
						rlog.Infof("QUEUE add ModulePurge %s", newTask.Name)

						err := KubeEventsHooks.DisableModuleHooks(moduleChange.Name, ModuleManager, KubeEventsManager)
						if err != nil {
							rlog.Errorf("MAIN_LOOP module '%s' purged: cannot disable hooks: %s", moduleChange.Name, err)
						}
					}
				}
				// Поменялись модули, нужно пересоздать индекс хуков по расписанию
				ScheduledHooks = UpdateScheduleHooks(ScheduledHooks)
			// Изменились глобальные values, нужен рестарт всех модулей
			case module_manager.GlobalChanged:
				rlog.Infof("EVENT GlobalChanged")
				TasksQueue.ChangesDisable()
				CreateReloadAllTasks()
				TasksQueue.ChangesEnable(true)
				// Пересоздать индекс хуков по расписанию
				ScheduledHooks = UpdateScheduleHooks(ScheduledHooks)
			case module_manager.AmbigousState:
				rlog.Infof("EVENT AmbigousState")
				TasksQueue.ChangesDisable()
				// Это ошибка в module_manager. Нужно добавить задачу в начало очереди,
				// чтобы module_manager имел возможность восстановить своё состояние
				// перед запуском других задач в очереди.
				newTask := task.NewTask(task.ModuleManagerRetry, "")
				TasksQueue.Push(newTask)
				// Задержка перед выполнением retry
				TasksQueue.Push(task.NewTaskDelay(FailedModuleDelay))
				TasksQueue.ChangesEnable(true)
				rlog.Infof("QUEUE push ModuleManagerRetry, push FailedModuleDelay")
			}
		case crontab := <-schedule_manager.ScheduleCh:
			scheduleHooks := ScheduledHooks.GetHooksForSchedule(crontab)
			for _, hook := range scheduleHooks {
				var getHookErr error

				_, getHookErr = ModuleManager.GetGlobalHook(hook.Name)
				if getHookErr == nil {
					for _, scheduleConfig := range hook.Schedule {
						bindingName := scheduleConfig.Name
						if bindingName == "" {
							bindingName = module_manager.ContextBindingType[module_manager.Schedule]
						}
						newTask := task.NewTask(task.GlobalHookRun, hook.Name).
							WithBinding(module_manager.Schedule).
							WithBindingContext(module_manager.BindingContext{Binding: bindingName}).
							WithAllowFailure(scheduleConfig.AllowFailure)
						TasksQueue.Add(newTask)
						rlog.Debugf("QUEUE add GlobalHookRun@Schedule '%s'", hook.Name)
					}
					continue
				}

				_, getHookErr = ModuleManager.GetModuleHook(hook.Name)
				if getHookErr == nil {
					for _, scheduleConfig := range hook.Schedule {
						bindingName := scheduleConfig.Name
						if bindingName == "" {
							bindingName = module_manager.ContextBindingType[module_manager.Schedule]
						}
						newTask := task.NewTask(task.ModuleHookRun, hook.Name).
							WithBinding(module_manager.Schedule).
							WithBindingContext(module_manager.BindingContext{Binding: bindingName}).
							WithAllowFailure(scheduleConfig.AllowFailure)
						TasksQueue.Add(newTask)
						rlog.Debugf("QUEUE add ModuleHookRun@Schedule '%s'", hook.Name)
					}
					continue
				}

				rlog.Errorf("MAIN_LOOP hook '%s' scheduled but not found by module_manager", hook.Name)
			}
		case kubeEvent := <-kube_events_manager.KubeEventCh:
			rlog.Infof("EVENT Kube event '%s'", kubeEvent.ConfigId)

			res, err := KubeEventsHooks.HandleEvent(kubeEvent)
			if err != nil {
				rlog.Errorf("MAIN_LOOP error handling kube event '%s': %s", kubeEvent.ConfigId, err)
				break
			}

			for _, task := range res.Tasks {
				TasksQueue.Add(task)
				rlog.Infof("QUEUE add %s@%s %s", task.GetType(), task.GetBinding(), task.GetName())
			}
		case <-ManagersEventsHandlerStopCh:
			rlog.Infof("EVENT Stop")
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
		TasksQueue.Add(newTask)
		rlog.Infof("QUEUE add ModuleRun %s", moduleName)
	}

	for _, moduleName := range modulesState.ModulesToDisable {
		newTask := task.NewTask(task.ModuleDelete, moduleName)
		TasksQueue.Add(newTask)
		rlog.Infof("QUEUE add ModuleDelete %s", moduleName)
	}

	for _, moduleName := range modulesState.ReleasedUnknownModules {
		newTask := task.NewTask(task.ModulePurge, moduleName)
		TasksQueue.Add(newTask)
		rlog.Infof("QUEUE add ModulePurge %s", moduleName)
	}

	// Queue afterAll global hooks
	afterAllHooks := ModuleManager.GetGlobalHooksInOrder(module_manager.AfterAll)
	for _, hookName := range afterAllHooks {
		newTask := task.NewTask(task.GlobalHookRun, hookName).
			WithBinding(module_manager.AfterAll).
			WithBindingContext(module_manager.BindingContext{Binding: module_manager.ContextBindingType[module_manager.AfterAll]})
		TasksQueue.Add(newTask)
		rlog.Debugf("QUEUE add GlobalHookRun@AfterAll '%s'", hookName)
	}
	if len(afterAllHooks) > 0 {
		rlog.Infof("QUEUE add all GlobalHookRun@AfterAll")
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
				rlog.Infof("TASK_RUN DiscoverModulesState")
				err := runDiscoverModulesState(t)
				if err != nil {
					MetricsStorage.SendCounterMetric("antiopa_modules_discover_errors", 1.0, map[string]string{})
					t.IncrementFailureCount()
					rlog.Errorf("TASK_RUN %s failed. Will retry after delay. Failed count is %d. Error: %s", t.GetType(), t.GetFailureCount(), err)
					TasksQueue.Push(task.NewTaskDelay(FailedModuleDelay))
					rlog.Infof("QUEUE push FailedModuleDelay")
					break
				}

				TasksQueue.Pop()

			case task.ModuleRun:
				rlog.Infof("TASK_RUN ModuleRun %s", t.GetName())
				err := ModuleManager.RunModule(t.GetName())
				if err != nil {
					MetricsStorage.SendCounterMetric("antiopa_module_run_errors", 1.0, map[string]string{"module": t.GetName()})
					t.IncrementFailureCount()
					rlog.Errorf("TASK_RUN %s '%s' failed. Will retry after delay. Failed count is %d. Error: %s", t.GetType(), t.GetName(), t.GetFailureCount(), err)
					TasksQueue.Push(task.NewTaskDelay(FailedModuleDelay))
					rlog.Infof("QUEUE push FailedModuleDelay")
				} else {
					TasksQueue.Pop()
				}
			case task.ModuleDelete:
				rlog.Infof("TASK_RUN ModuleDelete %s", t.GetName())
				err := ModuleManager.DeleteModule(t.GetName())
				if err != nil {
					MetricsStorage.SendCounterMetric("antiopa_module_delete_errors", 1.0, map[string]string{"module": t.GetName()})
					t.IncrementFailureCount()
					rlog.Errorf("%s '%s' failed. Will retry after delay. Failed count is %d. Error: %s", t.GetType(), t.GetName(), t.GetFailureCount(), err)
					TasksQueue.Push(task.NewTaskDelay(FailedModuleDelay))
					rlog.Infof("QUEUE push FailedModuleDelay")
				} else {
					TasksQueue.Pop()
				}
			case task.ModuleHookRun:
				rlog.Infof("TASK_RUN ModuleHookRun@%s %s", t.GetBinding(), t.GetName())
				err := ModuleManager.RunModuleHook(t.GetName(), t.GetBinding(), t.GetBindingContext())
				if err != nil {
					moduleHook, _ := ModuleManager.GetModuleHook(t.GetName())
					hookLabel := path.Base(moduleHook.Path)
					moduleLabel := moduleHook.Module.Name

					if t.GetAllowFailure() {
						MetricsStorage.SendCounterMetric("antiopa_module_hook_allowed_errors", 1.0, map[string]string{"module": moduleLabel, "hook": hookLabel})
						TasksQueue.Pop()
					} else {
						MetricsStorage.SendCounterMetric("antiopa_module_hook_errors", 1.0, map[string]string{"module": moduleLabel, "hook": hookLabel})
						t.IncrementFailureCount()
						rlog.Errorf("%s '%s' failed. Will retry after delay. Failed count is %d. Error: %s", t.GetType(), t.GetName(), t.GetFailureCount(), err)
						TasksQueue.Push(task.NewTaskDelay(FailedModuleDelay))
						rlog.Infof("QUEUE push FailedModuleDelay")
					}
				} else {
					TasksQueue.Pop()
				}
			case task.GlobalHookRun:
				rlog.Infof("TASK_RUN GlobalHookRun@%s %s", t.GetBinding(), t.GetName())
				err := ModuleManager.RunGlobalHook(t.GetName(), t.GetBinding(), t.GetBindingContext())
				if err != nil {
					globalHook, _ := ModuleManager.GetGlobalHook(t.GetName())
					hookLabel := path.Base(globalHook.Path)

					if t.GetAllowFailure() {
						MetricsStorage.SendCounterMetric("antiopa_global_hook_allowed_errors", 1.0, map[string]string{"hook": hookLabel})
						TasksQueue.Pop()
					} else {
						MetricsStorage.SendCounterMetric("antiopa_global_hook_errors", 1.0, map[string]string{"hook": hookLabel})
						t.IncrementFailureCount()
						rlog.Errorf("TASK_RUN %s '%s' on '%s' failed. Will retry after delay. Failed count is %d. Error: %s", t.GetType(), t.GetName(), t.GetBinding(), t.GetFailureCount(), err)
						TasksQueue.Push(task.NewTaskDelay(FailedHookDelay))
					}
				} else {
					TasksQueue.Pop()
				}
			case task.ModulePurge:
				rlog.Infof("TASK_RUN ModulePurge %s", t.GetName())
				// Module for purge is unknown so log deletion error is enough
				err := HelmClient.DeleteRelease(t.GetName())
				if err != nil {
					rlog.Errorf("TASK_RUN %s helm delete '%s' failed. Error: %s", t.GetType(), t.GetName(), err)
				}
				TasksQueue.Pop()
			case task.ModuleManagerRetry:
				rlog.Infof("TASK_RUN ModuleManagerRetry")
				// TODO метрику нужно отсылать из module_manager. Cделать metric_storage глобальным!
				MetricsStorage.SendCounterMetric("antiopa_modules_discover_errors", 1.0, map[string]string{})
				ModuleManager.Retry()
				TasksQueue.Pop()
				// Add delay before retry module/hook task again
				TasksQueue.Push(task.NewTaskDelay(FailedModuleDelay))
				rlog.Infof("QUEUE push FailedModuleDelay")
			case task.Delay:
				rlog.Infof("TASK_RUN Delay for %s", t.GetDelay().String())
				TasksQueue.Pop()
				time.Sleep(t.GetDelay())
			case task.Stop:
				rlog.Infof("TASK_RUN Stop: Exiting TASK_RUN loop.")
				TasksQueue.Pop()
				return
			}

			// break if empty to prevent infinity loop
			if TasksQueue.IsEmpty() {
				rlog.Debug("Task queue is empty. Will sleep now.")
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
	rlog.Infof("QUEUE add all GlobalHookRun@OnStartup")

	onStartupHooks := ModuleManager.GetGlobalHooksInOrder(module_manager.OnStartup)

	for _, hookName := range onStartupHooks {
		newTask := task.NewTask(task.GlobalHookRun, hookName).
			WithBinding(module_manager.OnStartup).
			WithBindingContext(module_manager.BindingContext{Binding: module_manager.ContextBindingType[module_manager.OnStartup]})
		TasksQueue.Add(newTask)
		rlog.Debugf("QUEUE add GlobalHookRun@OnStartup '%s'", hookName)
	}

	return
}

func CreateReloadAllTasks() {
	rlog.Infof("QUEUE add all GlobalHookRun@BeforeAll, add DiscoverModulesState")

	// Queue beforeAll global hooks
	beforeAllHooks := ModuleManager.GetGlobalHooksInOrder(module_manager.BeforeAll)

	for _, hookName := range beforeAllHooks {
		newTask := task.NewTask(task.GlobalHookRun, hookName).
			WithBinding(module_manager.BeforeAll).
			WithBindingContext(module_manager.BindingContext{Binding: module_manager.ContextBindingType[module_manager.BeforeAll]})

		TasksQueue.Add(newTask)
		rlog.Debugf("QUEUE GlobalHookRun@BeforeAll '%s'", module_manager.BeforeAll, hookName)
	}

	TasksQueue.Add(task.NewTask(task.DiscoverModulesState, ""))
}

func RunAntiopaMetrics() {
	// antiopa live ticks
	go func() {
		for {
			MetricsStorage.SendCounterMetric("antiopa_live_ticks", 1.0, map[string]string{})
			time.Sleep(10 * time.Second)
		}
	}()

	// TasksQueue length
	go func() {
		for {
			queueLen := float64(TasksQueue.Length())
			MetricsStorage.SendGaugeMetric("antiopa_tasks_queue_length", queueLen, map[string]string{})
			time.Sleep(5 * time.Second)
		}
	}()
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
