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

package remove

import (
	"context"
	"log/slog"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "package-remove"
)

type deployerI interface {
	Cleanup(ctx context.Context, name string) error
}

type task struct {
	name string

	deployer deployerI

	logger *log.Logger
}

// NewTask creates an Remove task for package.
func NewTask(name string, deployer deployerI, logger *log.Logger) queue.Task {
	return &task{
		name:     name,
		deployer: deployer,
		logger:   logger.Named(taskTracer).With(slog.String("name", name)),
	}
}

func (t *task) String() string {
	return "Cleanup"
}

func (t *task) Execute(ctx context.Context) error {
	t.logger.Debug("cleanup package")

	return t.deployer.Cleanup(ctx, t.name)
}
