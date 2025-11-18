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

package uninstall

import (
	"context"
	"log/slog"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "package-uninstall"
)

type installer interface {
	Uninstall(ctx context.Context, name string) error
}

func NewTask(name string, installer installer, logger *log.Logger) queue.Task {
	return &task{
		packageName: name,
		installer:   installer,
		logger:      logger.Named(taskTracer),
	}
}

type task struct {
	packageName string

	installer installer

	logger *log.Logger
}

func (t *task) String() string {
	return "Uninstall"
}

func (t *task) Execute(ctx context.Context) error {
	t.logger.Debug("uninstall package", slog.String("name", t.packageName))

	return t.installer.Uninstall(ctx, t.packageName)
}
