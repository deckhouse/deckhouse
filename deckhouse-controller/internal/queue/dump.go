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
	"sigs.k8s.io/yaml"
)

type dump struct {
	Queues map[string]dumpQueue `json:"queues" yaml:"queues"`
}

type dumpQueue struct {
	Name   string     `json:"name" yaml:"name"`
	Number int        `json:"number" yaml:"number"`
	Tasks  []dumpTask `json:"tasks,omitempty" yaml:"tasks,omitempty"`
}

type dumpTask struct {
	Index int     `json:"index" yaml:"index"`
	Name  string  `json:"name" yaml:"name"`
	Error *string `json:"error,omitempty" yaml:"error,omitempty"`
}

// Dump creates dump of all queues
func (s *Service) Dump() []byte {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	d := &dump{
		Queues: make(map[string]dumpQueue),
	}

	for name, q := range s.queues {
		d.Queues[name] = q.dump()
	}

	marshalled, _ := yaml.Marshal(d)
	return marshalled
}

// dump creates queue dump for debug
func (q *queue) dump() dumpQueue {
	q.mu.Lock()
	defer q.mu.Unlock()

	tasks := q.getTasksDump()

	return dumpQueue{
		Name:   q.name,
		Number: len(tasks),
		Tasks:  tasks,
	}
}

func (q *queue) getTasksDump() []dumpTask {
	var tasks []dumpTask // nolint:prealloc

	index := 1
	for wrapper := range q.deque.Iter() {
		var errStr *string
		if wrapper.err != nil {
			s := wrapper.err.Error()
			errStr = &s
		}

		tasks = append(tasks, dumpTask{
			Index: index,
			Name:  wrapper.task.String(),
			Error: errStr,
		})

		index++
	}

	return tasks
}
