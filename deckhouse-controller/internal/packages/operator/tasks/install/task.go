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

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "package-install"
)

type installer interface {
	Install(ctx context.Context, registry, instance, packageName, version string) error
}

type statusService interface {
	SetConditionTrue(name string, conditionName status.ConditionName)
	HandleError(name string, err error)
}

type task struct {
	instance    string
	packageName string
	version     string
	registry    registry.Registry

	installer installer
	status    statusService

	logger *log.Logger
}

func NewTask(name, pack, version string, reg registry.Registry, status statusService, installer installer, logger *log.Logger) queue.Task {
	return &task{
		instance:    name,
		packageName: pack,
		version:     version,
		registry:    reg,
		installer:   installer,
		status:      status,
		logger:      logger.Named(taskTracer),
	}
}

func (t *task) String() string {
	return fmt.Sprintf("Install:%s", t.version)
}

func (t *task) Execute(ctx context.Context) error {
	logger := t.logger.With(
		slog.String("name", t.instance),
		slog.String("package", t.packageName),
		slog.String("registry", t.registry.Name),
		slog.String("version", t.version))

	// Install (mount) package to apps directory
	logger.Debug("install application")
	if err := t.installer.Install(ctx, t.registry.Name, t.instance, t.packageName, t.version); err != nil {
		t.status.HandleError(t.instance, err)
		return fmt.Errorf("install application: %w", err)
	}

	t.status.SetConditionTrue(t.instance, status.ConditionReadyOnFilesystem)

	return nil
}
