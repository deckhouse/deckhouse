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

package load

import (
	"context"
	"fmt"
	"log/slog"

	addonutils "github.com/flant/addon-operator/pkg/utils"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "package-load"
)

type manager interface {
	LoadPackage(ctx context.Context, namespace, name string) error
	ApplySettings(name string, settings addonutils.Values) error
}

type task struct {
	packageName string
	namespace   string

	manager  manager
	settings addonutils.Values

	logger *log.Logger
}

func NewTask(namespace, name string, settings addonutils.Values, manager manager, logger *log.Logger) queue.Task {
	return &task{
		packageName: name,
		namespace:   namespace,
		manager:     manager,
		settings:    settings,
		logger:      logger.Named(taskTracer),
	}
}

func (t *task) String() string {
	return "Load"
}

func (t *task) Execute(ctx context.Context) error {
	// Load package into package manager (parse hooks, values, chart)
	t.logger.Debug("load package", slog.String("name", t.packageName))
	if err := t.manager.LoadPackage(ctx, t.namespace, t.packageName); err != nil {
		return fmt.Errorf("load package: %w", err)
	}

	t.logger.Debug("apply initial settings", slog.String("name", t.packageName))
	if err := t.manager.ApplySettings(t.packageName, t.settings); err != nil {
		return fmt.Errorf("apply initial settings: %w", err)
	}

	return nil
}
