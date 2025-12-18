/*
Copyright 2023 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"sync"

	"github.com/go-logr/logr"
)

type taskManager struct {
	tasksMutex sync.Mutex
	tasks      map[taskID]*bool
	logger     logr.Logger
}

type taskID string

func newTaskManager(logger logr.Logger) *taskManager {
	return &taskManager{
		tasks:  make(map[taskID]*bool),
		logger: logger,
	}
}

// spawn spawns a new task if it doesn't exist yet.
func (m *taskManager) spawn(taskID taskID, task func() bool) *bool {
	m.logger.V(2).Info("Starting spawn task", "id", taskID)
	defer m.logger.V(2).Info("Finished spawn task", "id", taskID)

	m.tasksMutex.Lock()
	defer m.tasksMutex.Unlock()

	// Avoid spawning multiple tasks for the same taskID.
	done, ok := m.tasks[taskID]
	m.logger.V(2).Info("Has task with id", "id", taskID, "ok", ok)
	if ok {
		m.logger.V(2).Info("Task with id present", "id", taskID, "done", done)

		if done == nil {
			return nil
		}

		delete(m.tasks, taskID)

		m.logger.V(2).Info("Task with id deleted from manager and return result", "id", taskID, "done", done)

		return done
	}

	m.logger.V(2).Info("Starting gorutine with task", "id", taskID)

	m.tasks[taskID] = nil

	go func() {
		var done bool

		defer func() {
			m.tasksMutex.Lock()
			defer m.tasksMutex.Unlock()

			m.tasks[taskID] = &done

			m.logger.V(2).Info("Task written to state", "id", taskID, "done", done, "map_variable", m.tasks[taskID])
		}()

		res := task()

		m.logger.V(2).Info("Task with finished with id and result", "id", taskID, "result", res)

		done = res

		m.logger.V(2).Info("Task result write to variable", "id", taskID, "done", done)
	}()

	return nil
}
