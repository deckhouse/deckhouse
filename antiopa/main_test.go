package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"fmt"
	"github.com/deckhouse/deckhouse/antiopa/module_manager"
	"github.com/deckhouse/deckhouse/antiopa/task"
	"time"
)

type ModuleManagerMock struct {
}

func (m *ModuleManagerMock) Run() {
	panic("implement Run")
}

func (m *ModuleManagerMock) GetModule(name string) (*module_manager.Module, error) {
	panic("implement GetModule")
}

func (m *ModuleManagerMock) GetModuleNamesInOrder() []string {
	panic("implement GetModuleNamesInOrder")
}

func (m *ModuleManagerMock) GetGlobalHook(name string) (*module_manager.GlobalHook, error) {
	panic("implement GetGlobalHook")
}

func (m *ModuleManagerMock) GetModuleHook(name string) (*module_manager.ModuleHook, error) {
	panic("implement GetModuleHook")
}

func (m *ModuleManagerMock) GetGlobalHooksInOrder(bindingType module_manager.BindingType) ([]string, error) {
	return []string{"hook_1", "hook_2"}, nil
}

func (m *ModuleManagerMock) GetModuleHooksInOrder(moduleName string, bindingType module_manager.BindingType) ([]string, error) {
	panic("implement GetModuleHooksInOrder")
}

func (m *ModuleManagerMock) DeleteModule(moduleName string) error {
	panic("implement DeleteModule")
}

func (m *ModuleManagerMock) RunModule(moduleName string) error {
	panic("implement RunModule")
}

func (m *ModuleManagerMock) RunGlobalHook(hookName string, binding module_manager.BindingType) error {
	fmt.Printf("Run global hook name '%s' binding '%s'\n", hookName, binding)
	return nil
}

func (m *ModuleManagerMock) RunModuleHook(hookName string, binding module_manager.BindingType) error {
	panic("implement RunModuleHook")
}

type QueueDumperTest struct {
}

func (q *QueueDumperTest) QueueChangeCallback() {
	headTask, _ := TasksQueue.Peek()
	if v, ok := headTask.(*task.Task); ok {
		fmt.Printf("head task now is '%s'\n", v.Name)
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

	TasksQueue = task.NewTasksQueue()

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
	}(module_manager.EventCh)

	go ManagersEventsHandler()

	time.Sleep(100 * time.Millisecond)
	ManagersEventsHandlerStopCh <- struct{}{}

	assert.Equal(t, 2, TasksQueue.Length())
}
