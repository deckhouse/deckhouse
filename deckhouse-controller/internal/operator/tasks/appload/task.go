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

package appload

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/installer"
	packagemanager "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "appLoad"
)

type DependencyContainer interface {
	Installer() *installer.Installer
	PackageManager() *packagemanager.Manager
}

type Config struct {
	AppName    string
	Package    string
	Version    string
	Settings   map[string]interface{}
	Repository *v1alpha1.PackageRepository
}

type task struct {
	appName     string
	packageName string
	version     string
	settings    map[string]interface{}
	repository  *v1alpha1.PackageRepository

	dc DependencyContainer

	logger *log.Logger
}

func New(conf Config, dc DependencyContainer, logger *log.Logger) queue.Task {
	return &task{
		appName:     conf.AppName,
		packageName: conf.Package,
		version:     conf.Version,
		repository:  conf.Repository,
		settings:    conf.Settings,
		dc:          dc,
		logger:      logger.Named(taskTracer),
	}
}

func (t *task) String() string {
	return fmt.Sprintf("Application:%s:Load", t.appName)
}

func (t *task) Execute(ctx context.Context) error {
	if err := t.dc.Installer().Ensure(ctx, t.repository, t.appName, t.packageName, t.version); err != nil {
		return fmt.Errorf("ensure application: %w", err)
	}

	if err := t.dc.PackageManager().LoadApplication(ctx, t.appName, t.settings); err != nil {
		return fmt.Errorf("load application: %w", err)
	}

	return nil
}
