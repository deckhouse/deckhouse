package agent

import (
	"cloud-provider-static/internal/providerid"
	"sync"
)

// Agent is a task agent. It spawns tasks and stores their results. It is
// thread-safe. It is used to avoid spawning the same task multiple times.
type Agent struct {
	tasksMutex sync.Mutex
	tasks      map[providerid.ProviderID]*taskResult
}

type taskResult struct {
	value interface{}
}

// NewAgent creates a new task agent.
func NewAgent() *Agent {
	return &Agent{
		tasks: make(map[providerid.ProviderID]*taskResult),
	}
}

// spawn spawns a new task if it doesn't exist yet.
func (a *Agent) spawn(providerID providerid.ProviderID, fn func() interface{}) {
	a.tasksMutex.Lock()
	defer a.tasksMutex.Unlock()

	_, ok := a.tasks[providerID]
	if ok {
		return
	}

	a.tasks[providerID] = &taskResult{}

	go func() {
		var result interface{}

		defer func() {
			a.tasksMutex.Lock()
			defer a.tasksMutex.Unlock()

			a.tasks[providerID].value = result
		}()

		result = fn()
	}()
}

// getTaskResult returns the result of a task.
func (a *Agent) getTaskResult(providerID providerid.ProviderID) interface{} {
	a.tasksMutex.Lock()
	defer a.tasksMutex.Unlock()

	return a.tasks[providerID].value
}

// deleteTaskResult deletes the result of a task.
func (a *Agent) deleteTaskResult(providerID providerid.ProviderID) {
	a.tasksMutex.Lock()
	defer a.tasksMutex.Unlock()

	delete(a.tasks, providerID)
}
