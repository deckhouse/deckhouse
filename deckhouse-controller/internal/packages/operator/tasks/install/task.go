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
	"path/filepath"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/go_lib/d8env"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "package-install"
)

var (
	modulesDownloadedDir = d8env.GetDownloadedModulesDir()
	modulesDeployedDir   = filepath.Join(modulesDownloadedDir, "modules")
	appsDownloadedDir    = filepath.Join(d8env.GetDownloadedModulesDir(), "apps")
	appsDeployedDir      = filepath.Join(appsDownloadedDir, "deployed")
)

type installer interface {
	Install(ctx context.Context, downloaded, deployed, name, version string) error
}

type statusService interface {
	SetConditionTrue(name string, cond status.ConditionType)
	HandleError(name string, err error)
}

type task struct {
	downloaded string
	deployed   string
	name       string
	version    string

	repository registry.Repository

	installer installer
	status    statusService

	logger *log.Logger
}

func NewModuleTask(name, version string, repo registry.Repository, status statusService, installer installer, logger *log.Logger) queue.Task {
	return &task{
		downloaded: filepath.Join(modulesDownloadedDir, name),
		deployed:   filepath.Join(modulesDeployedDir, name),
		name:       name,
		version:    version,
		repository: repo,
		installer:  installer,
		status:     status,
		logger:     logger.Named(taskTracer),
	}
}

func NewAppTask(instance, name, version string, repo registry.Repository, status statusService, installer installer, logger *log.Logger) queue.Task {
	return &task{
		downloaded: filepath.Join(appsDownloadedDir, repo.Name, name),
		deployed:   filepath.Join(appsDeployedDir, instance),
		name:       instance,
		version:    version,
		repository: repo,
		installer:  installer,
		status:     status,
		logger:     logger.Named(taskTracer),
	}
}

func (t *task) String() string {
	return fmt.Sprintf("Install:%s", t.version)
}

func (t *task) Execute(ctx context.Context) error {
	logger := t.logger.With(
		slog.String("name", t.name),
		slog.String("downloaded", t.downloaded),
		slog.String("deployed", t.deployed),
		slog.String("repository", t.repository.Name),
		slog.String("version", t.version))

	// install (mount) package
	logger.Debug("install package")
	if err := t.installer.Install(ctx, t.downloaded, t.deployed, t.name, t.version); err != nil {
		t.status.HandleError(t.name, err)
		return fmt.Errorf("install package: %w", err)
	}

	t.status.SetConditionTrue(t.name, status.ConditionReadyOnFilesystem)

	return nil
}
