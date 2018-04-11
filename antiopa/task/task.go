package task

import (
	"bytes"
	"fmt"
	"time"

	"github.com/deckhouse/deckhouse/antiopa/module_manager"
)

type TaskType string

const (
	ModuleDelete  TaskType = "TASK_MODULE_DELETE"
	ModuleRun     TaskType = "TASK_MODULE_RUN"
	ModuleHookRun TaskType = "TASK_MODULE_HOOK_RUN"
	GlobalHookRun TaskType = "TASK_GLOBAL_HOOK_RUN"
	// удаление релиза без сведений о модуле
	ModulePurge TaskType = "TASK_MODULE_PURGE"
	// вспомогательные задачи: задержка и остановка обработки
	Delay TaskType = "TASK_DELAY"
	Stop  TaskType = "TASK_STOP"
)

type Task struct {
	FailureCount int    // failed executions count
	Name         string // name of module or hook
	Type         TaskType
	Binding      module_manager.BindingType
	AllowFailure bool // task considered ok if hook failed. false by default. can be true for some schedule hooks
}

func NewTask(taskType TaskType, name string) *Task {
	return &Task{
		FailureCount: 0,
		Name:         name,
		Type:         taskType,
		AllowFailure: false,
	}
}

func (t *Task) WithBinding(binding module_manager.BindingType) *Task {
	t.Binding = binding
	return t
}

func (t *Task) WithAllowFailure(allowFailure bool) *Task {
	t.AllowFailure = allowFailure
	return t
}

func (t *Task) DumpAsText() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("%s '%s'", t.Type, t.Name))
	if t.FailureCount > 0 {
		buf.WriteString(fmt.Sprintf(" failed %d times. ", t.FailureCount))
	}
	return buf.String()
}

func (t *Task) IncrementFailureCount() {
	t.FailureCount++
}

type TaskDelay struct {
	Task
	Delay time.Duration
}

func NewTaskDelay(delay time.Duration) *TaskDelay {
	return &TaskDelay{
		Task: Task{
			Type: Delay,
		},
		Delay: delay,
	}
}
