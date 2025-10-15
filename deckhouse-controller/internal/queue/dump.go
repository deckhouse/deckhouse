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
	Name  string     `json:"name" yaml:"name"`
	Tasks []dumpTask `json:"tasks,omitempty" yaml:"tasks,omitempty"`
}

type dumpTask struct {
	Index int     `json:"index" yaml:"index"`
	Name  string  `json:"name" yaml:"name"`
	Error *string `json:"error,omitempty" yaml:"error,omitempty"`
}

// Dump creates queue dump for debug
func (q *queue) Dump() []byte {
	q.mu.Lock()
	defer q.mu.Unlock()

	marshalled, _ := yaml.Marshal(dump{
		Name:  q.name,
		Tasks: q.getTasksDump(),
	})

	return marshalled
}

func (q *queue) getTasksDump() []dumpTask {
	var tasks []dumpTask

	index := 1
	for wrapper := range q.deque.Iter() {
		var errStr *string
		if wrapper.err != nil {
			s := wrapper.err.Error()
			errStr = &s
		}

		tasks = append(tasks, dumpTask{
			Index: index,
			Name:  wrapper.task.Name(),
			Error: errStr,
		})

		index++
	}

	return tasks
}
