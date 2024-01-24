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

	"caps-controller-manager/internal/providerid"
)

type taskManager struct {
	tasksMutex sync.Mutex
	tasks      map[providerid.ProviderID]*bool
}

func newTaskManager() *taskManager {
	return &taskManager{
		tasks: make(map[providerid.ProviderID]*bool),
	}
}

// spawn spawns a new task if it doesn't exist yet.
func (m *taskManager) spawn(providerID providerid.ProviderID, task func() bool) bool {
	m.tasksMutex.Lock()
	defer m.tasksMutex.Unlock()

	// Avoid spawning multiple tasks for the same providerID.
	done, ok := m.tasks[providerID]
	if ok {
		if done == nil {
			return false
		}

		delete(m.tasks, providerID)

		return *done
	}

	m.tasks[providerID] = nil

	go func() {
		var done bool

		defer func() {
			m.tasksMutex.Lock()
			defer m.tasksMutex.Unlock()

			m.tasks[providerID] = &done
		}()

		done = task()
	}()

	return false
}
