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

package deploy

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
	taskTracer = "package-deploy"
)

var (
	modulesDownloadedDir = d8env.GetDownloadedModulesDir()
	modulesDeployedDir   = filepath.Join(modulesDownloadedDir, "modules")
	appsDownloadedDir    = filepath.Join(d8env.GetDownloadedModulesDir(), "apps")
	appsDeployedDir      = filepath.Join(appsDownloadedDir, "deployed")
)

type deployerI interface {
	Deploy(ctx context.Context, repo registry.Remote, downloaded, deployed, packageName, name, version string) error
}

type statusService interface {
	SetConditionTrue(name string, cond status.ConditionType)
	HandleError(name string, err error)
}

type task struct {
	name        string
	packageName string
	version     string
	downloaded  string
	deployed    string

	repository registry.Remote

	deployer deployerI
	status   statusService

	logger *log.Logger
}

// NewModuleTask creates a Deploy task for a Module package.
// The package is cached under downloaded/{name} and exposed at downloaded/modules/{name}.
func NewModuleTask(name, version string, repo registry.Remote, deployer deployerI, status statusService, logger *log.Logger) queue.Task {
	return &task{
		name:        name,
		packageName: name,
		version:     version,
		downloaded:  filepath.Join(modulesDownloadedDir, name),
		deployed:    filepath.Join(modulesDeployedDir, name),
		repository:  repo,
		deployer:    deployer,
		status:      status,
		logger:      logger.Named(taskTracer),
	}
}

// NewAppTask creates a Deploy task for an Application package.
// The package is cached under apps/{repo}/{package} and exposed at apps/deployed/{instance}.
func NewAppTask(instance, name, version string, repo registry.Remote, deployer deployerI, status statusService, logger *log.Logger) queue.Task {
	return &task{
		name:        instance,
		packageName: name,
		version:     version,
		downloaded:  filepath.Join(appsDownloadedDir, repo.Name, name),
		deployed:    filepath.Join(appsDeployedDir, instance),
		repository:  repo,
		deployer:    deployer,
		status:      status,
		logger:      logger.Named(taskTracer),
	}
}

func (t *task) String() string {
	return fmt.Sprintf("Deploy:%s", t.version)
}

func (t *task) Execute(ctx context.Context) error {
	logger := t.logger.With(
		slog.String("name", t.name),
		slog.String("downloaded", t.downloaded),
		slog.String("repository", t.repository.Name),
		slog.String("version", t.version))

	// Cache package content locally and expose it at the path consumed by the load task.
	logger.Debug("deploy package")
	if err := t.deployer.Deploy(ctx, t.repository, t.downloaded, t.deployed, t.packageName, t.name, t.version); err != nil {
		t.status.HandleError(t.name, err)
		return fmt.Errorf("deploy package: %w", err)
	}

	t.status.SetConditionTrue(t.name, status.ConditionReadyOnFilesystem)

	return nil
}
