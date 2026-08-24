// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package queue

import (
	"slices"
	"time"
)

// Dump is a snapshot of the queues and the tasks waiting in them.
type Dump struct {
	Queues map[string]QueueDump `json:"queues"`
}

// QueueDump is a snapshot of one queue.
type QueueDump struct {
	Length int        `json:"length"`
	Tasks  []TaskDump `json:"tasks,omitempty"`
}

// TaskDump is a snapshot of one task waiting in a queue.
type TaskDump struct {
	Index     int     `json:"index"`
	Name      string  `json:"name"`
	Enqueued  string  `json:"enqueued"`
	NextRetry string  `json:"next_retry"`
	Error     *string `json:"error,omitempty"`
}

// Dump creates dump of all queues.
func (s *Service) Dump(include ...string) Dump {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	queues := make(map[string]QueueDump, len(s.queues))
	for name, q := range s.queues {
		if len(include) == 0 || slices.Contains(include, q.name) {
			queues[name] = q.dump()
		}
	}

	return Dump{
		Queues: queues,
	}
}

// dump creates queue dump for debug
func (q *queue) dump() QueueDump {
	q.mu.Lock()
	defer q.mu.Unlock()

	tasks := q.getTasksDump()

	return QueueDump{
		Length: len(tasks),
		Tasks:  tasks,
	}
}

func (q *queue) getTasksDump() []TaskDump {
	var tasks []TaskDump // nolint:prealloc

	index := 1
	for wrapper := range q.deque.Iter() {
		var errStr *string
		if wrapper.err != nil {
			s := wrapper.err.Error()
			errStr = &s
		}

		tasks = append(tasks, TaskDump{
			Index:     index,
			Name:      wrapper.task.String(),
			Enqueued:  time.Since(wrapper.enqueuedAt).String(),
			NextRetry: time.Until(wrapper.nextRetry).String(),
			Error:     errStr,
		})

		index++
	}

	return tasks
}
