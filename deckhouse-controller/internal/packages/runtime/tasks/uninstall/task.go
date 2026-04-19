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

package uninstall

import (
	"context"
	"log/slog"
	"path/filepath"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/go_lib/d8env"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "package-uninstall"
)

var (
	modulesDownloadedDir = d8env.GetDownloadedModulesDir()
	modulesDeployedDir   = filepath.Join(modulesDownloadedDir, "modules")
	appsDownloadedDir    = filepath.Join(d8env.GetDownloadedModulesDir(), "apps")
	appsDeployedDir      = filepath.Join(appsDownloadedDir, "deployed")
)

type installerI interface {
	Uninstall(ctx context.Context, downloaded, deployed, name string, keep bool) error
}

type task struct {
	name       string
	downloaded string
	deployed   string
	keep       bool

	installer installerI

	logger *log.Logger
}

// NewAppTask creates an Uninstall task for an Application package.
// Sets keep=true to preserve downloaded images for potential reinstallation.
func NewAppTask(name string, installer installerI, logger *log.Logger) queue.Task {
	return &task{
		name: name,
		// TODO(ipaqsa): design app deletion
		// downloaded: filepath.Join(appsDownloadedDir, repo.Name, name),
		deployed:  filepath.Join(appsDeployedDir, name),
		keep:      true,
		installer: installer,
		logger:    logger.Named(taskTracer),
	}
}

// NewModuleTask creates an Uninstall task for a Module package.
// Sets keep=false to remove both deployed and downloaded directories.
func NewModuleTask(name string, installer installerI, logger *log.Logger) queue.Task {
	return &task{
		name:       name,
		downloaded: filepath.Join(modulesDownloadedDir, name),
		deployed:   filepath.Join(modulesDeployedDir, name),
		keep:       false,
		installer:  installer,
		logger:     logger.Named(taskTracer),
	}
}

func (t *task) String() string {
	return "Uninstall"
}

func (t *task) Execute(ctx context.Context) error {
	t.logger.Debug("uninstall package", slog.String("name", t.name))

	return t.installer.Uninstall(ctx, t.downloaded, t.deployed, t.name, t.keep)
}
