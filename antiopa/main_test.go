package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/antiopa/helm"
	"github.com/deckhouse/deckhouse/antiopa/module_manager"
	"github.com/deckhouse/deckhouse/antiopa/task"
)

type ModuleManagerMock struct {
}

var mainTestGlobalHooksMap = map[module_manager.BindingType][]string{
	module_manager.OnStartup: {
		"hook_1", "hook_2",
	},
	module_manager.BeforeAll: {
		"before_hook_1", "before_hook_2",
	},
	module_manager.AfterAll: {
		"after_hook_1", "after_hook_2",
	},
}

func (m *ModuleManagerMock) GetModulesToDisableOnInit() []string {
	return []string{"disabled_module_1", "disabled_2", "disabled_3.14"}
}

func (m *ModuleManagerMock) GetModulesToPurgeOnInit() []string {
	return []string{"unknown_module_1", "abandoned_2", "forgotten_3.14"}
}

func (m *ModuleManagerMock) Run() {
	fmt.Println("ModuleManagerMock Run")
}

func (m *ModuleManagerMock) GetModule(name string) (*module_manager.Module, error) {
	panic("implement GetModule")
}

func (m *ModuleManagerMock) GetModuleNamesInOrder() []string {
	return []string{"test_module_1", "test_module_2"}
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
	fmt.Printf("ModuleManagerMock DeleteModule '%s'\n", moduleName)
	return nil
}

func (m *ModuleManagerMock) RunModule(moduleName string) error {
	fmt.Printf("ModuleManagerMock RunModule '%s'\n", moduleName)
	return nil
}

func (m *ModuleManagerMock) RunGlobalHook(hookName string, binding module_manager.BindingType) error {
	fmt.Printf("Run global hook name '%s' binding '%s'\n", hookName, binding)
	return nil
}

func (m *ModuleManagerMock) RunModuleHook(hookName string, binding module_manager.BindingType) error {
	panic("implement RunModuleHook")
}

type MockHelmClient struct {
	helm.HelmClient
}

func (h MockHelmClient) CommandEnv() []string {
	return []string{}
}

func (h MockHelmClient) DeleteRelease(name string) error {
	fmt.Printf("HelmClient: DeleteRelease '%s'\n", name)
	return nil
}

type QueueDumperTest struct {
}

func (q *QueueDumperTest) QueueChangeCallback() {
	headTask, _ := TasksQueue.Peek()
	if headTask != nil {
		fmt.Printf("head task now is '%s', len=%d\n", headTask.GetName(), TasksQueue.Length())
	}
}

func TestMain_TaskRunner(t *testing.T) {
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
	expectedCount += len(ModuleManager.GetModuleNamesInOrder())
	expectedCount += len(ModuleManager.GetGlobalHooksInOrder(module_manager.AfterAll))

	assert.Equal(t, expectedCount, TasksQueue.Length())
}

func TestMain_Run(t *testing.T) {
	module_manager.EventCh = make(chan module_manager.Event, 1)
	ManagersEventsHandlerStopCh = make(chan struct{}, 1)

	HelmClient = MockHelmClient{}

	// Mock ModuleManager
	ModuleManager = &ModuleManagerMock{}

	assert.Equal(t, 0, 0)
	fmt.Println("Create queue")
	// Fill a queue
	TasksQueue = task.NewTasksQueue()
	// watcher for more verbosity of CreateStartupTasks and
	TasksQueue.AddWatcher(&QueueDumperTest{})
	TasksQueue.ChangesEnable(true)

	Run()

	time.Sleep(100 * time.Millisecond)
	// Stop events handler
	ManagersEventsHandlerStopCh <- struct{}{}
	// stop tasks runner: add stop task
	stopTask := task.NewTask(task.Stop, "stop runner")
	TasksQueue.Add(stopTask)

	fmt.Println("wait for queueIsEmptyDelay")
	time.Sleep(3100 * time.Millisecond)

	assert.Equalf(t, 0, TasksQueue.Length(), "%d tasks remain in queue after TasksRunner", TasksQueue.Length())
}
