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

type installer interface {
	Uninstall(ctx context.Context, downloaded, deployed, name string, keep bool) error
}

func NewModuleTask(name string, installer installer, logger *log.Logger) queue.Task {
	return &task{
		downloaded: filepath.Join(modulesDownloadedDir, name),
		deployed:   filepath.Join(modulesDeployedDir, name),
		name:       name,
		keep:       false,
		installer:  installer,
		logger:     logger.Named(taskTracer),
	}
}

func NewAppTask(instance string, installer installer, logger *log.Logger) queue.Task {
	return &task{
		// TODO(ipaqsa): design app deletion
		// downloaded: filepath.Join(appsDownloadedDir, repo.Name, name),
		deployed:  filepath.Join(appsDeployedDir, instance),
		name:      instance,
		keep:      true,
		installer: installer,
		logger:    logger.Named(taskTracer),
	}
}

type task struct {
	downloaded string
	deployed   string
	name       string
	keep       bool

	installer installer

	logger *log.Logger
}

func (t *task) String() string {
	return "Uninstall"
}

func (t *task) Execute(ctx context.Context) error {
	t.logger.Debug("uninstall package", slog.String("name", t.name))

	return t.installer.Uninstall(ctx, t.downloaded, t.deployed, t.name, t.keep)
}
