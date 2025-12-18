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

package download

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
	taskTracer = "package-download"
)

type downloader interface {
	Download(ctx context.Context, reg registry.Registry, packageName, version string) error
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

	downloader downloader
	status     statusService

	logger *log.Logger
}

func NewTask(name, pack, version string, reg registry.Registry, status statusService, downloader downloader, logger *log.Logger) queue.Task {
	return &task{
		instance:    name,
		packageName: pack,
		version:     version,
		registry:    reg,
		downloader:  downloader,
		status:      status,
		logger:      logger.Named(taskTracer),
	}
}

func (t *task) String() string {
	return "Download"
}

func (t *task) Execute(ctx context.Context) error {
	logger := t.logger.With(
		slog.String("name", t.instance),
		slog.String("package", t.packageName),
		slog.String("registry", t.registry.Name),
		slog.String("version", t.version))

	// Download package from repository
	logger.Debug("download package")
	if err := t.downloader.Download(ctx, t.registry, t.packageName, t.version); err != nil {
		t.status.HandleError(t.instance, err)
		return fmt.Errorf("download package: %w", err)
	}

	t.status.SetConditionTrue(t.instance, status.ConditionDownloaded)

	return nil
}
