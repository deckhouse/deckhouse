package task

import (
	"bytes"
	"fmt"
	"time"
)

type TaskType string

const (
	Module        TaskType = "TASK_MODULE"
	ModuleDelete  TaskType = "TASK_MODULE_DELETE"
	ModuleUpgrade TaskType = "TASK_MODULE_UPGRADE"
	ModuleRun     TaskType = "TASK_MODULE_RUN"
	Hook          TaskType = "TASK_HOOK"
	Delay         TaskType = "TASK_DELAY"
)

type Task struct {
	FailureCount int    // failed executions count
	Name         string // name of module or hook
	Type         TaskType
}

func NewTask(taskType TaskType, name string) *Task {
	return &Task{
		FailureCount: 0,
		Name:         name,
		Type:         taskType,
	}
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
