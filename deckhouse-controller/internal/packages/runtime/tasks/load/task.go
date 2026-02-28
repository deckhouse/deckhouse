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
	"errors"
	"fmt"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/go_lib/d8env"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "package-load"
)

var (
	embeddedDeployedDir  = "modules"
	modulesDownloadedDir = d8env.GetDownloadedModulesDir()
	modulesDeployedDir   = filepath.Join(modulesDownloadedDir, "modules")
	appsDownloadedDir    = filepath.Join(d8env.GetDownloadedModulesDir(), "apps")
	appsDeployedDir      = filepath.Join(appsDownloadedDir, "deployed")
)

type loader func(ctx context.Context, repo registry.Remote, path string) (string, error)

type statusService interface {
	HandleError(name string, err error)
	SetVersion(name string, version string)
}

type task struct {
	name       string
	deployed   string
	repository registry.Remote

	loader loader
	status statusService

	logger *log.Logger
}

// NewAppTask creates a Load task for an Application package.
// The deployed path points to apps/deployed/{name} where the package is mounted.
func NewAppTask(name string, repo registry.Remote, loader loader, status statusService, logger *log.Logger) queue.Task {
	return &task{
		name:       name,
		deployed:   filepath.Join(appsDeployedDir, name),
		repository: repo,
		loader:     loader,
		status:     status,
		logger:     logger.Named(taskTracer).With("name", name),
	}
}

// NewModuleTask creates a Load task for a Module package.
// The deployed path points to modules/{name} where the module is mounted.
func NewModuleTask(name string, repo registry.Remote, loader loader, status statusService, logger *log.Logger) queue.Task {
	return &task{
		name:       name,
		deployed:   filepath.Join(modulesDeployedDir, name),
		repository: repo,
		loader:     loader,
		status:     status,
		logger:     logger.Named(taskTracer).With("name", name),
	}
}

// NewEmbeddedTask creates a Load task for an embedded Module package.
// The deployed path points to modules/{name} where the module is stored.
func NewEmbeddedTask(name string, loader loader, status statusService, logger *log.Logger) queue.Task {
	return &task{
		name:     name,
		deployed: filepath.Join(embeddedDeployedDir, name),
		loader:   loader,
		status:   status,
		logger:   logger.Named(taskTracer).With("name", name),
	}
}

func (t *task) String() string {
	return "Load"
}

func (t *task) Execute(ctx context.Context) error {
	// Load package into package manager (parse hooks, values, chart)
	t.logger.Debug("load package")
	version, err := t.loader(ctx, t.repository, t.deployed)
	if err != nil {
		t.status.HandleError(t.name, err)
		return fmt.Errorf("load package: %w", err)
	}

	t.status.SetVersion(t.name, version)

	// Signal that package is loaded and waiting for the scheduler to enable it.
	// The WaitConverge condition indicates the package is ready but not yet running.
	t.status.HandleError(t.name, &status.Error{
		Err: errors.New("wait for converge done"),
		Conditions: []status.Condition{
			{
				Type:    status.ConditionWaitConverge,
				Status:  metav1.ConditionTrue,
				Reason:  "WaitConverge",
				Message: "wait for converge done",
			},
		},
	})

	return nil
}
