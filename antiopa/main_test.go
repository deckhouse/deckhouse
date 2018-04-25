package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/antiopa/helm"
	"github.com/deckhouse/deckhouse/antiopa/module_manager"
	"github.com/deckhouse/deckhouse/antiopa/task"
	"strconv"
	"strings"
)

type ModuleManagerMock struct {
	BeforeHookErrorsCount   int
	TestModuleErrorsCount   int
	DeleteModuleErrorsCount int
}

var mainTestGlobalHooksMap = map[module_manager.BindingType][]string{
	module_manager.OnStartup: {
		"hook_1__31", "hook_2__32",
	},
	module_manager.BeforeAll: {
		"before_hook_1__51", "before_hook_2__52",
	},
	module_manager.AfterAll: {
		"after_hook_1__201", "after_hook_2__202",
	},
}

var runOrder = []int{}

var globalT *testing.T

func (m *ModuleManagerMock) Run() {
	fmt.Println("ModuleManagerMock Run")
}

func (m *ModuleManagerMock) GetModule(name string) (*module_manager.Module, error) {
	panic("implement GetModule")
}

func (m *ModuleManagerMock) DiscoverModulesState() (*module_manager.ModulesState, error) {
	return &module_manager.ModulesState{
		[]string{"test_module_1__101", "test_module_2__102"},
		[]string{"disabled_module_1__111", "disabled_2__112", "disabled_3.14__113"},
		[]string{"unknown_module_1__121", "abandoned_1__122", "forgotten_3.14__123"},
	}, nil
}

func (m *ModuleManagerMock) GetGlobalHook(name string) (*module_manager.GlobalHook, error) {
	panic("implement GetGlobalHook")
}

func (m *ModuleManagerMock) GetModuleHook(name string) (*module_manager.ModuleHook, error) {
	panic("implement GetModuleHook")
}

func (m *ModuleManagerMock) GetGlobalHooksInOrder(bindingType module_manager.BindingType) []string {
	return mainTestGlobalHooksMap[bindingType]
}

func (m *ModuleManagerMock) GetModuleHooksInOrder(moduleName string, bindingType module_manager.BindingType) ([]string, error) {
	return []string{"test_module_hook_1", "test_module_hook_2"}, nil
}

func (m *ModuleManagerMock) DeleteModule(moduleName string) error {
	addRunOrder(moduleName)
	fmt.Printf("ModuleManagerMock DeleteModule '%s'\n", moduleName)
	if strings.Contains(moduleName, "disabled_module_1") && m.DeleteModuleErrorsCount > 0 {
		m.DeleteModuleErrorsCount--
		return fmt.Errorf("fake module delete error: helm run error")
	}
	return nil
}

func (m *ModuleManagerMock) RunModule(moduleName string) error {
	addRunOrder(moduleName)
	fmt.Printf("ModuleManagerMock RunModule '%s'\n", moduleName)
	if strings.Contains(moduleName, "test_module_2") && m.TestModuleErrorsCount > 0 {
		m.TestModuleErrorsCount--
		return fmt.Errorf("fake module error: /bin/bash not found")
	}
	return nil
}

func (m *ModuleManagerMock) RunGlobalHook(hookName string, binding module_manager.BindingType) error {
	addRunOrder(hookName)
	fmt.Printf("Run global hook name '%s' binding '%s'\n", hookName, binding)
	if strings.Contains(hookName, "before_hook_1") && m.BeforeHookErrorsCount > 0 {
		m.BeforeHookErrorsCount--
		return fmt.Errorf("fake module error: /bin/bash not found")
	}
	return nil
}

func (m *ModuleManagerMock) RunModuleHook(hookName string, binding module_manager.BindingType) error {
	panic("implement RunModuleHook")
}

type MockHelmClient struct {
	helm.HelmClient
	DeleteReleaseErrorsCount int
}

func (h MockHelmClient) CommandEnv() []string {
	return []string{}
}

func (h MockHelmClient) DeleteRelease(name string) error {
	addRunOrder(name)
	fmt.Printf("HelmClient: DeleteRelease '%s'\n", name)
	if strings.Contains(name, "abandoned_2") && h.DeleteReleaseErrorsCount > 0 {
		h.DeleteReleaseErrorsCount--
		return fmt.Errorf("fake helm error: helm syntax error")
	}
	return nil
}

func addRunOrder(name string) {
	if !strings.Contains(name, "__") {
		return
	}
	order := strings.Split(name, "__")[1]
	orderI, err := strconv.Atoi(order)
	if err != nil {
		globalT.Fatalf("Cannot parse number from order '%s' from name '%s'", order, name)
	}
	runOrder = append(runOrder, orderI)
}

type QueueDumperTest struct {
}

func (q *QueueDumperTest) QueueChangeCallback() {
	headTask, _ := TasksQueue.Peek()
	if headTask != nil {
		fmt.Printf("head task now is '%s', len=%d\n", headTask.GetName(), TasksQueue.Length())
	}
}

// Тест заполнения очереди заданиями при запуске и прогон TaskRunner
// после прогона очередь должна быть пустой
func TestMain_TaskRunner_CreateOnStartupTasks(t *testing.T) {
	runOrder = []int{}

	// Mock ModuleManager
	ModuleManager = &ModuleManagerMock{}

	assert.Equal(t, 0, 0)
	fmt.Println("Create queue")
	// Fill a queue
	TasksQueue = task.NewTasksQueue()
	// watcher for more verbosity of CreateStartupTasks and
	TasksQueue.AddWatcher(&QueueDumperTest{})
	TasksQueue.ChangesEnable(true)

	// Add StartupTasks
	CreateOnStartupTasks()

	expectedCount := len(ModuleManager.GetGlobalHooksInOrder(module_manager.OnStartup))
	assert.Equalf(t, expectedCount, TasksQueue.Length(), "queue length is not relevant to global hooks OnStartup", TasksQueue.Length())

	// add stop task
	stopTask := task.NewTask(task.Stop, "stop runner")
	TasksQueue.Add(stopTask)

	fmt.Println("Start task runner")
	TasksRunner()

	assert.Equalf(t, 0, TasksQueue.Length(), "%d tasks remain in queue after TasksRunner", TasksQueue.Length())
}

// Тест заполнения очереди через ModuleManager и его канал EventCh
// Проверяется, что очередь будет заполнена нужным количеством заданий
func TestMain_ModulesEventsHandler(t *testing.T) {
	module_manager.EventCh = make(chan module_manager.Event, 1)
	ManagersEventsHandlerStopCh = make(chan struct{}, 1)

	// Mock ModuleManager
	ModuleManager = &ModuleManagerMock{}

	assert.Equal(t, 0, 0)
	fmt.Println("Create queue")
	// Fill a queue
	TasksQueue = task.NewTasksQueue()
	// watcher for more verbosity of CreateStartupTasks and
	TasksQueue.AddWatcher(&QueueDumperTest{})
	TasksQueue.ChangesEnable(true)

	go func(ch chan module_manager.Event) {
		ch <- module_manager.Event{
			Type: module_manager.ModulesChanged,
			ModulesChanges: []module_manager.ModuleChange{
				{
					Name:       "test_module_1",
					ChangeType: module_manager.Changed,
				},
				{
					Name:       "test_module_2",
					ChangeType: module_manager.Disabled,
				},
			},
		}
		ch <- module_manager.Event{
			Type: module_manager.ModulesChanged,
			ModulesChanges: []module_manager.ModuleChange{
				{
					Name:       "test_module_purged",
					ChangeType: module_manager.Purged,
				},
				{
					Name:       "test_module_enabled",
					ChangeType: module_manager.Enabled,
				},
			},
		}

		ch <- module_manager.Event{
			Type: module_manager.GlobalChanged,
		}
	}(module_manager.EventCh)

	go ManagersEventsHandler()

	time.Sleep(100 * time.Millisecond)
	ManagersEventsHandlerStopCh <- struct{}{}

	expectedCount := 4 // count of ModuleChange in previous go routine
	expectedCount += len(ModuleManager.GetGlobalHooksInOrder(module_manager.BeforeAll))
	expectedCount += 1 // DiscoverModulesState task

	assert.Equal(t, expectedCount, TasksQueue.Length())
}

// Тест совместной работы ManagersEventsHandler и TaskRunner.
// один модуль выдаёт ошибку, TaskRunner должен его перезапускать, не запуская другие модули
// проверяется, что модули запускаются по порядку (порядок в runOrder — суффикс имени "__число")
func TestMain_Run_With_InfiniteModuleError(t *testing.T) {
	// Настройки задержек при ошибках и пустой очереди, чтобы тест побыстрее завершался.
	QueueIsEmptyDelay = 50 * time.Millisecond
	FailedHookDelay = 50 * time.Millisecond
	FailedModuleDelay = 50 * time.Millisecond

	module_manager.EventCh = make(chan module_manager.Event, 1)
	ManagersEventsHandlerStopCh = make(chan struct{}, 1)

	runOrder = []int{}

	HelmClient = MockHelmClient{
		DeleteReleaseErrorsCount: 0,
	}

	// Mock ModuleManager
	ModuleManager = &ModuleManagerMock{
		BeforeHookErrorsCount:   0,
		TestModuleErrorsCount:   10000,
		DeleteModuleErrorsCount: 0,
	}

	assert.Equal(t, 0, 0)
	fmt.Println("Create queue")
	// Fill a queue
	TasksQueue = task.NewTasksQueue()
	// watcher for more verbosity of CreateStartupTasks and
	TasksQueue.AddWatcher(&QueueDumperTest{})
	TasksQueue.ChangesEnable(true)

	Run()

	time.Sleep(1000 * time.Millisecond)
	// Stop events handler
	ManagersEventsHandlerStopCh <- struct{}{}
	// stop tasks runner: add stop task
	stopTask := task.NewTask(task.Stop, "stop runner")
	TasksQueue.Push(stopTask)

	fmt.Println("wait for queueIsEmptyDelay")
	time.Sleep(100 * time.Millisecond)

	assert.True(t, TasksQueue.Length() > 0, "queue is empty with errored module %d", TasksQueue.Length())

	accum := 0
	for _, ord := range runOrder {
		assert.True(t, ord >= accum, "detect unordered execution: '%d' '%d'\n%+v", accum, ord, runOrder)
		accum = ord
	}

	fmt.Printf("runOrder: %+v", runOrder)
}

// Тест совместной работы ManagersEventsHandler и TaskRunner.
// Модули и хуки выдают ошибки, TaskRunner должен их перезапускать, не запуская следующие задания.
// Проверяется, что модули и хуки запускаются по порядку (порядок в runOrder — суффикс имени "__число")
func TestMain_Run_With_RecoverableErrors(t *testing.T) {
	// Настройки задержек при ошибках и пустой очереди, чтобы тест побыстрее завершался.
	QueueIsEmptyDelay = 50 * time.Millisecond
	FailedHookDelay = 50 * time.Millisecond
	FailedModuleDelay = 50 * time.Millisecond

	module_manager.EventCh = make(chan module_manager.Event, 1)
	ManagersEventsHandlerStopCh = make(chan struct{}, 1)

	runOrder = []int{}

	HelmClient = MockHelmClient{
		DeleteReleaseErrorsCount: 3,
	}

	// Mock ModuleManager
	ModuleManager = &ModuleManagerMock{
		BeforeHookErrorsCount:   3,
		TestModuleErrorsCount:   6,
		DeleteModuleErrorsCount: 2,
	}

	assert.Equal(t, 0, 0)
	fmt.Println("Create queue")
	// Fill a queue
	TasksQueue = task.NewTasksQueue()
	// watcher for more verbosity of CreateStartupTasks and
	TasksQueue.AddWatcher(&QueueDumperTest{})
	TasksQueue.ChangesEnable(true)

	Run()

	time.Sleep(1000 * time.Millisecond)
	// Stop events handler
	ManagersEventsHandlerStopCh <- struct{}{}
	// stop tasks runner: add stop task
	stopTask := task.NewTask(task.Stop, "stop runner")
	TasksQueue.Add(stopTask)

	fmt.Println("wait for queueIsEmptyDelay")
	time.Sleep(100 * time.Millisecond)

	assert.Equalf(t, 0, TasksQueue.Length(), "%d tasks remain in queue after TasksRunner", TasksQueue.Length())

	accum := 0
	for _, ord := range runOrder {
		assert.True(t, ord >= accum, "detect unordered execution: '%d' '%d'\n%+v", accum, ord, runOrder)
		accum = ord
	}

	fmt.Printf("runOrder: %+v", runOrder)
}
