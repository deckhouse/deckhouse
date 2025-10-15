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

package parallel

import (
	"context"
	"sync"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type DependencyContainer interface {
	GetQueueService() *queue.Service
}

type task struct {
	name     string
	subtasks map[string]queue.Task

	logger *log.Logger

	dc DependencyContainer
}

func New(name string, subtasks map[string]queue.Task, dc DependencyContainer, logger *log.Logger) queue.Task {
	return &task{
		name:     name,
		dc:       dc,
		subtasks: subtasks,

		logger: logger.Named("parallel-" + name),
	}
}

func (t *task) Name() string {
	return "parallel-" + t.name
}

func (t *task) Execute(ctx context.Context) error {
	t.logger.Info("run parallel")
	wg := new(sync.WaitGroup)
	for queueName, sub := range t.subtasks {
		t.dc.GetQueueService().Enqueue(ctx, queueName, sub, queue.WithWait(wg))
	}

	wg.Wait()

	t.logger.Info("finished parallel")

	return nil
}
