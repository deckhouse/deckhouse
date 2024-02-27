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
)

type taskManager struct {
	tasksMutex sync.Mutex
	tasks      map[taskID]*bool
}

type taskID string

func newTaskManager() *taskManager {
	return &taskManager{
		tasks: make(map[taskID]*bool),
	}
}

// spawn spawns a new task if it doesn't exist yet.
func (m *taskManager) spawn(taskID taskID, task func() bool) bool {
	m.tasksMutex.Lock()
	defer m.tasksMutex.Unlock()

	// Avoid spawning multiple tasks for the same taskID.
	done, ok := m.tasks[taskID]
	if ok {
		if done == nil {
			return false
		}

		delete(m.tasks, taskID)

		return *done
	}

	m.tasks[taskID] = nil

	go func() {
		var done bool

		defer func() {
			m.tasksMutex.Lock()
			defer m.tasksMutex.Unlock()

			m.tasks[taskID] = &done
		}()

		done = task()
	}()

	return false
}
