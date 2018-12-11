package task

import (
	"bytes"
	"fmt"
	"time"

	"github.com/deckhouse/deckhouse/antiopa/module_manager"
)

type TaskType string

const (
	ModuleDelete         TaskType = "TASK_MODULE_DELETE"
	ModuleRun            TaskType = "TASK_MODULE_RUN"
	ModuleHookRun        TaskType = "TASK_MODULE_HOOK_RUN"
	GlobalHookRun        TaskType = "TASK_GLOBAL_HOOK_RUN"
	DiscoverModulesState TaskType = "TASK_DISCOVER_MODULES_STATE"
	// удаление релиза без сведений о модуле
	ModulePurge TaskType = "TASK_MODULE_PURGE"
	// retry module_manager-а
	ModuleManagerRetry TaskType = "TASK_MODULE_MANAGER_RETRY"
	// вспомогательные задачи: задержка и остановка обработки
	Delay TaskType = "TASK_DELAY"
	Stop  TaskType = "TASK_STOP"
)

type Task interface {
	GetName() string
	GetType() TaskType
	GetBinding() module_manager.BindingType
	GetBindingContext() []module_manager.BindingContext
	GetFailureCount() int
	IncrementFailureCount()
	GetDelay() time.Duration
	GetAllowFailure() bool
}

type BaseTask struct {
	FailureCount   int    // failed executions count
	Name           string // name of module or hook
	Type           TaskType
	Binding        module_manager.BindingType
	BindingContext []module_manager.BindingContext
	Delay          time.Duration
	AllowFailure   bool // task considered ok if hook failed. false by default. can be true for some schedule hooks
}

func NewTask(taskType TaskType, name string) *BaseTask {
	return &BaseTask{
		FailureCount:   0,
		Name:           name,
		Type:           taskType,
		AllowFailure:   false,
		BindingContext: make([]module_manager.BindingContext, 0),
	}
}

func (t *BaseTask) GetName() string {
	return t.Name
}

func (t *BaseTask) GetType() TaskType {
	return t.Type
}

func (t *BaseTask) GetBinding() module_manager.BindingType {
	return t.Binding
}

func (t *BaseTask) GetBindingContext() []module_manager.BindingContext {
	return t.BindingContext
}

func (t *BaseTask) GetDelay() time.Duration {
	return t.Delay
}

func (t *BaseTask) GetAllowFailure() bool {
	return t.AllowFailure
}

func (t *BaseTask) WithBinding(binding module_manager.BindingType) *BaseTask {
	t.Binding = binding
	return t
}

func (t *BaseTask) WithBindingContext(context []module_manager.BindingContext) *BaseTask {
	t.BindingContext = context
	return t
}

func (t *BaseTask) AppendBindingContext(context module_manager.BindingContext) *BaseTask {
	t.BindingContext = append(t.BindingContext, context)
	return t
}

func (t *BaseTask) WithAllowFailure(allowFailure bool) *BaseTask {
	t.AllowFailure = allowFailure
	return t
}

func (t *BaseTask) DumpAsText() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("%s '%s'", t.Type, t.Name))
	if t.FailureCount > 0 {
		buf.WriteString(fmt.Sprintf(" failed %d times. ", t.FailureCount))
	}
	return buf.String()
}

func (t *BaseTask) GetFailureCount() int {
	return t.FailureCount
}

func (t *BaseTask) IncrementFailureCount() {
	t.FailureCount++
}

func NewTaskDelay(delay time.Duration) *BaseTask {
	return &BaseTask{
		Type:  Delay,
		Delay: delay,
	}
}
