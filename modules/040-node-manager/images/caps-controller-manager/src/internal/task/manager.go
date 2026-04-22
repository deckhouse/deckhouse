/*
Copyright 2026 Flant JSC

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

package task

import (
	"context"
	"sync"

	ctrl "sigs.k8s.io/controller-runtime"
)

type Task func(ctx context.Context, data any) error

type taskEntry struct {
	ch  chan bool
	res error
}

type Manager struct {
	mu    sync.Mutex
	tasks map[string]*taskEntry
}

func NewTaskManager() *Manager {
	return &Manager{
		tasks: make(map[string]*taskEntry),
	}
}

// Spawn spawns a new task if it doesn't exist yet.
func (m *Manager) Spawn(ctx context.Context, id, taskType string, data any, task Task) (result error, finished bool) {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"taskID", id,
		"taskType", taskType,
	)
	ctx = ctrl.LoggerInto(ctx, log)

	m.mu.Lock()
	t, ok := m.tasks[id+taskType]
	if !ok {
		t = &taskEntry{
			ch: make(chan bool, 1),
		}
		m.tasks[id+taskType] = t

		go func() {
			log := ctrl.LoggerFrom(ctx)
			log.Info("task started")

			res := task(ctx, data)

			log.Info("task finished", "result", res)

			m.mu.Lock()
			t.res = res
			close(t.ch)
			m.mu.Unlock()
		}()
	}
	m.mu.Unlock()

	// non-blocking check
	select {
	case <-t.ch:
		m.mu.Lock()
		res := t.res
		delete(m.tasks, id+taskType)
		m.mu.Unlock()
		return res, true
	default:
		return nil, false
	}
}
