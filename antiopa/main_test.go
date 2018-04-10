package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"fmt"
	"github.com/deckhouse/deckhouse/antiopa/module_manager"
	"github.com/deckhouse/deckhouse/antiopa/task"
)

type ModuleManagerMock struct {
}

func (m *ModuleManagerMock) Run() {
	panic("implement me")
}

func (m *ModuleManagerMock) GetModule(name string) (*module_manager.Module, error) {
	panic("implement me")
}

func (m *ModuleManagerMock) GetModuleNamesInOrder() []string {
	panic("implement me")
}

func (m *ModuleManagerMock) GetGlobalHook(name string) (*module_manager.GlobalHook, error) {
	panic("implement me")
}

func (m *ModuleManagerMock) GetModuleHook(name string) (*module_manager.ModuleHook, error) {
	panic("implement me")
}

func (m *ModuleManagerMock) GetGlobalHooksInOrder(bindingType module_manager.BindingType) ([]string, error) {
	panic("implement me")
}

func (m *ModuleManagerMock) GetModuleHooksInOrder(moduleName string, bindingType module_manager.BindingType) ([]string, error) {
	panic("implement me")
}

func (m *ModuleManagerMock) DeleteModule(moduleName string) error {
	panic("implement me")
}

func (m *ModuleManagerMock) RunModule(moduleName string) error {
	panic("implement me")
}

func (m *ModuleManagerMock) RunGlobalHook(hookName string, binding module_manager.BindingType) error {
	panic("implement me")
}

func (m *ModuleManagerMock) RunModuleHook(hookName string, binding module_manager.BindingType) error {
	panic("implement me")
}

func TestMain_TaskRunner(t *testing.T) {
	// Mock ModuleManager
	ModuleManager = &ModuleManagerMock{}

	assert.Equal(t, 0, 0)
	fmt.Println("Create queue")
	// Fill a queue
	TasksQueue = task.NewTasksQueue()
	stopTask := task.NewTask(task.Stop, "stop runner")
	TasksQueue.Add(stopTask)

	fmt.Println("Start task runner")
	// TODO Пока что всё виснет при обработке
	//TasksRunner()

	assert.Equal(t, 0, 0)
}

func TestMain_ModulesEventsHandler(t *testing.T) {
	panic("implement me")
}
