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
	taskTracer = "package-deploy"
)

type deployerI interface {
	Deploy(ctx context.Context, remote registry.Remote, deployd, name, version string) error
}

type statusService interface {
	SetConditionTrue(name string, cond status.ConditionType)
	HandleError(name string, err error)
}

type task struct {
	name        string
	packageName string
	version     string

	repository registry.Remote

	deployerI deployerI
	status    statusService

	logger *log.Logger
}

func NewAppTask(name, packageName, version string, repo registry.Remote, deployer deployerI, status statusService, logger *log.Logger) queue.Task {
	return &task{
		name:        name,
		packageName: packageName,
		version:     version,
		repository:  repo,
		deployerI:   deployer,
		status:      status,
		logger:      logger.Named(taskTracer),
	}
}

func NewModuleTask(name, version string, repo registry.Remote, deployer deployerI, status statusService, logger *log.Logger) queue.Task {
	return &task{
		name:        name,
		packageName: name,
		version:     version,
		repository:  repo,
		deployerI:   deployer,
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
		slog.String("repository", t.repository.Name),
		slog.String("version", t.version))

	logger.Debug("deploy package")
	if err := t.deployerI.Deploy(ctx, t.repository, t.name, t.packageName, t.version); err != nil {
		t.status.HandleError(t.name, err)
		return fmt.Errorf("deploy package: %w", err)
	}

	t.status.SetConditionTrue(t.name, status.ConditionReadyOnFilesystem)

	return nil
}
