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
	"path/filepath"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/go_lib/d8env"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "package-download"
)

var (
	modulesDownloadedDir = d8env.GetDownloadedModulesDir()
	appsDownloadedDir    = filepath.Join(d8env.GetDownloadedModulesDir(), "apps")
)

type downloaderI interface {
	Download(ctx context.Context, repo registry.Remote, downloaded, name, version string) error
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

	repository registry.Remote

	downloader downloaderI
	status     statusService

	logger *log.Logger
}

func NewModuleTask(name, version string, repo registry.Remote, downloader downloaderI, status statusService, logger *log.Logger) queue.Task {
	return &task{
		name:        name,
		packageName: name,
		version:     version,
		downloaded:  filepath.Join(modulesDownloadedDir, name),
		repository:  repo,
		downloader:  downloader,
		status:      status,
		logger:      logger.Named(taskTracer),
	}
}

func NewAppTask(instance, name, version string, repo registry.Remote, downloader downloaderI, status statusService, logger *log.Logger) queue.Task {
	return &task{
		downloaded:  filepath.Join(appsDownloadedDir, repo.Name, name),
		packageName: name,
		name:        instance,
		version:     version,
		repository:  repo,
		downloader:  downloader,
		status:      status,
		logger:      logger.Named(taskTracer),
	}
}

func (t *task) String() string {
	return fmt.Sprintf("Download:%s", t.version)
}

func (t *task) Execute(ctx context.Context) error {
	logger := t.logger.With(
		slog.String("name", t.name),
		slog.String("downloaded", t.downloaded),
		slog.String("repository", t.repository.Name),
		slog.String("version", t.version))

	// download package from repository
	logger.Debug("download package")
	if err := t.downloader.Download(ctx, t.repository, t.downloaded, t.packageName, t.version); err != nil {
		t.status.HandleError(t.name, err)
		return fmt.Errorf("download package: %w", err)
	}

	t.status.SetConditionTrue(t.name, status.ConditionDownloaded)

	return nil
}
