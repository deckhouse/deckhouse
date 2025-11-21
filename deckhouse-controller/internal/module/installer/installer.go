/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package installer

import (
	"context"
	"fmt"
	"os"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/module/installer/erofs"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/module/installer/symlink"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/tools/verity"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/d8env"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type Installer struct {
	registry *registry.Service

	downloaded string
	installer  installer
}

type installer interface {
	Restore(ctx context.Context, source *v1alpha1.ModuleSource, module, version string) error
	Install(ctx context.Context, module, version, tempModulePath string) error
	Uninstall(ctx context.Context, module string) error
}

func New(dc dependency.Container, logger *log.Logger) *Installer {
	i := new(Installer)

	i.downloaded = d8env.GetDownloadedModulesDir()
	i.registry = registry.NewService(dc, logger)
	i.installer = symlink.NewInstaller(i.registry, logger)

	if verity.IsSupported() {
		logger.Info("erofs supported, use erofs installer")
		i.installer = erofs.NewInstaller(i.registry, logger)
	}

	return i
}

func (i *Installer) SetClusterUUID(id string) {
	i.registry.SetClusterUUID(id)
}

// GetDownloaded gets all downloaded modules from downloaded dir
func (i *Installer) GetDownloaded() (map[string]struct{}, error) {
	entries, err := os.ReadDir(i.downloaded)
	if err != nil {
		return nil, fmt.Errorf("read downloaded dir: %w", err)
	}

	downloaded := make(map[string]struct{})
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// skip enabled dirs
		if entry.Name() == "modules" || entry.Name() == "apps" {
			continue
		}

		downloaded[entry.Name()] = struct{}{}
	}

	return downloaded, nil
}

func (i *Installer) GetImageDigest(ctx context.Context, source *v1alpha1.ModuleSource, moduleName, version string) (string, error) {
	return i.registry.GetImageDigest(ctx, registry.BuildRegistryBySource(source), moduleName, version)
}

func (i *Installer) Download(ctx context.Context, source *v1alpha1.ModuleSource, moduleName, version string) (string, error) {
	tmp, err := os.MkdirTemp("", "package*")
	if err != nil {
		return "", fmt.Errorf("create tmp directory: %w", err)
	}

	if err = i.registry.Download(ctx, registry.BuildRegistryBySource(source), tmp, moduleName, version); err != nil {
		return "", err
	}

	return tmp, nil
}

func (i *Installer) Install(ctx context.Context, module, version, tempModulePath string) error {
	return i.installer.Install(ctx, module, version, tempModulePath)
}

func (i *Installer) Uninstall(ctx context.Context, module string) error {
	return i.installer.Uninstall(ctx, module)
}

// Restore ensures the module image is present, verified, and mounted.
func (i *Installer) Restore(ctx context.Context, ms *v1alpha1.ModuleSource, module, version string) error {
	return i.installer.Restore(ctx, ms, module, version)
}
