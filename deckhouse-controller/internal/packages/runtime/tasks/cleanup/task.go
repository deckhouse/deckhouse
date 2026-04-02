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

package cleanup

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/go_lib/d8env"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "cleanup"
)

var (
	modulesDownloadedDir = d8env.GetDownloadedModulesDir()
	modulesDeployedDir   = filepath.Join(modulesDownloadedDir, "modules")

	appsDownloadedDir = filepath.Join(d8env.GetDownloadedModulesDir(), "apps")
	appsDeployedDir   = filepath.Join(appsDownloadedDir, "deployed")
)

type installerI interface {
	Cleanup(ctx context.Context, downloaded, deployed string, exclude ...string)
}

type task struct {
	installer installerI

	logger *log.Logger
}

// NewTask creates a Cleanup task that removes stale files from downloaded and deployed directories.
func NewTask(installer installerI, logger *log.Logger) queue.Task {
	return &task{
		installer: installer,
		logger:    logger.Named(taskTracer),
	}
}

func (t *task) String() string {
	return fmt.Sprintf("Cleanup")
}

func (t *task) Execute(ctx context.Context) error {
	t.logger.Debug("cleanup downloads")
	t.installer.Cleanup(ctx, modulesDownloadedDir, modulesDeployedDir, appsDownloadedDir)
	t.installer.Cleanup(ctx, appsDownloadedDir, appsDeployedDir)

	return nil
}
