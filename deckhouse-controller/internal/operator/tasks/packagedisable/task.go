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

package packagedisable

import (
	"context"
	"fmt"
	"log/slog"

	packagemanager "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "packageDisable"
)

type DependencyContainer interface {
	PackageManager() *packagemanager.Manager
}

func New(name string, dc DependencyContainer, logger *log.Logger) queue.Task {
	return &task{
		packageName: name,
		dc:          dc,
		logger:      logger.Named(taskTracer),
	}
}

type task struct {
	packageName string

	dc DependencyContainer

	logger *log.Logger
}

func (t *task) String() string {
	return fmt.Sprintf("Package:%s:Disable", t.packageName)
}

func (t *task) Execute(ctx context.Context) error {
	t.logger.Debug("disable package", slog.String("name", t.packageName))

	// stop kube monitors and schedules
	if err := t.dc.PackageManager().DisablePackage(ctx, t.packageName, true); err != nil {
		return fmt.Errorf("disable package '%s': %w", t.packageName, err)
	}

	return nil
}
