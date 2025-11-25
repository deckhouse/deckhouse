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

package install

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "package-install"
)

type installer interface {
	Download(ctx context.Context, reg registry.Registry, packageName, version string) error
	Install(ctx context.Context, registry, instance, packageName, version string) error
}

type task struct {
	instance    string
	packageName string
	version     string
	registry    registry.Registry

	installer installer

	logger *log.Logger
}

func NewTask(name, pack, version string, reg registry.Registry, installer installer, logger *log.Logger) queue.Task {
	return &task{
		instance:    name,
		packageName: pack,
		version:     version,
		registry:    reg,
		installer:   installer,
		logger:      logger.Named(taskTracer),
	}
}

func (t *task) String() string {
	return "Install"
}

func (t *task) Execute(ctx context.Context) error {
	logger := t.logger.With(
		slog.String("name", t.instance),
		slog.String("package", t.packageName),
		slog.String("registry", t.registry.Name),
		slog.String("version", t.version))

	// Download package from repository
	logger.Debug("download package")
	if err := t.installer.Download(ctx, t.registry, t.packageName, t.version); err != nil {
		return fmt.Errorf("download package: %w", err)
	}

	// Install (mount) package to apps directory
	logger.Debug("install application")
	if err := t.installer.Install(ctx, t.registry.Name, t.instance, t.packageName, t.version); err != nil {
		return fmt.Errorf("install application: %w", err)
	}

	return nil
}
