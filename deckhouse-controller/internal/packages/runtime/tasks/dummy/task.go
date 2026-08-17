// Copyright 2026 Flant JSC
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

package dummy

import (
	"context"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "package-dummy"
)

// NewTask creates a Dummy task: the terminal barrier of a removal with nothing to undeploy.
// The removal's teardown rides this task's OnDone, so it runs only once everything already queued
// for the package has finished — see Execute for why the teardown cannot be the task itself.
func NewTask(name string, logger *log.Logger) queue.Task {
	return &task{
		logger: logger.Named(taskTracer).With("name", name),
	}
}

// task marks the end of a removal pipeline and does nothing else.
type task struct {
	logger *log.Logger
}

func (t *task) String() string {
	return "Dummy"
}

// Execute does nothing and cannot fail, so it can neither retry nor block the queue it ends.
// The barrier is the task's position in the package's FIFO queue, not its body: the teardown stops
// that very queue and waits for it to drain, which from inside a task would wait on itself.
func (t *task) Execute(_ context.Context) error {
	t.logger.Debug("package removed, nothing to undeploy")

	return nil
}
