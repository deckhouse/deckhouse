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
	"path/filepath"
	"regexp"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/module/installer/erofs"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/module/installer/symlink"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/tools/verity"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/app"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type Installer struct {
	registry *registry.Service

	downloaded string
	embedded   string
	installer  installer
}

type installer interface {
	Restore(ctx context.Context, source *v1alpha1.ModuleSource, module, version string) error
	Install(ctx context.Context, module, version, tempModulePath string) error
	Uninstall(ctx context.Context, module string) error
	// Stage materializes a module on the filesystem from a temp dir without
	// activating it (no symlink/mount). Used to pre-download a module while an
	// embedded copy of the same name is still serving it.
	Stage(ctx context.Context, module, version, tempModulePath string) error
	// StageFromRegistry materializes a module on the filesystem from the registry
	// without activating it (no symlink/mount).
	StageFromRegistry(ctx context.Context, source *v1alpha1.ModuleSource, module, version string) error
}

func New(dc dependency.Container, logger *log.Logger) *Installer {
	i := new(Installer)

	i.downloaded = app.DownloadedModulesDir()
	i.embedded = app.EmbeddedModulesDir
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

// GetInstalled gets all installed modules from <downloaded>/modules dir
func (i *Installer) GetInstalled() (map[string]struct{}, error) {
	entries, err := os.ReadDir(filepath.Join(i.downloaded, "modules"))
	if err != nil {
		return nil, fmt.Errorf("read installed dir: %w", err)
	}

	// Pattern to match optional weight prefix: 920-modulename -> modulename
	weightPattern := regexp.MustCompile(`^(?:[0-9]+-)?(.+)$`)

	installed := make(map[string]struct{})
	for _, entry := range entries {
		if !entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
			continue
		}

		name := entry.Name()
		// Remove weight prefix if present
		if matches := weightPattern.FindStringSubmatch(name); len(matches) > 1 {
			name = matches[1]
		}

		installed[name] = struct{}{}
	}

	return installed, nil
}

func (i *Installer) GetImageDigest(ctx context.Context, source *v1alpha1.ModuleSource, moduleName, version string) (string, error) {
	return i.registry.GetImageDigest(ctx, registry.BuildRemote(source), moduleName, version)
}

func (i *Installer) Download(ctx context.Context, source *v1alpha1.ModuleSource, moduleName, version string) (string, error) {
	tmp, err := os.MkdirTemp("", "package*")
	if err != nil {
		return "", fmt.Errorf("create tmp directory: %w", err)
	}

	if err = i.registry.Download(ctx, registry.BuildRemote(source), tmp, moduleName, version); err != nil {
		return "", err
	}

	return tmp, nil
}

func (i *Installer) Install(ctx context.Context, module, version, tempModulePath string) error {
	return i.installer.Install(ctx, module, version, tempModulePath)
}

// Stage materializes a module on the filesystem from a temp dir without activating
// it (no symlink/mount), so an embedded copy of the same name keeps serving the
// module until Deckhouse drops the embedded copy on upgrade.
func (i *Installer) Stage(ctx context.Context, module, version, tempModulePath string) error {
	return i.installer.Stage(ctx, module, version, tempModulePath)
}

// StageFromRegistry materializes a module on the filesystem from the registry
// without activating it (no symlink/mount).
func (i *Installer) StageFromRegistry(ctx context.Context, source *v1alpha1.ModuleSource, module, version string) error {
	return i.installer.StageFromRegistry(ctx, source, module, version)
}

// IsEmbeddedPresent reports whether an embedded copy of the module is shipped on
// the filesystem. While it is present, the module search path resolves the module
// to its embedded copy, so a downloaded module of the same name must not be
// activated (only staged).
//
// Embedded modules are shipped with an optional weight prefix in their directory
// name (e.g. 040-node-manager), while module holds the bare name (node-manager),
// so the lookup must strip the prefix instead of stat'ing the bare name directly.
func (i *Installer) IsEmbeddedPresent(module string) bool {
	if i.embedded == "" {
		return false
	}

	entries, err := os.ReadDir(i.embedded)
	if err != nil {
		// no embedded modules dir (or it is unreadable) - nothing is embedded
		return false
	}

	// Match an optional weight prefix: 040-node-manager -> node-manager.
	weightPattern := regexp.MustCompile(`^(?:[0-9]+-)?(.+)$`)
	for _, entry := range entries {
		// embedded modules are plain directories, but tolerate symlinks too
		if !entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
			continue
		}

		name := entry.Name()
		if matches := weightPattern.FindStringSubmatch(name); len(matches) > 1 {
			name = matches[1]
		}

		if name == module {
			return true
		}
	}

	return false
}

func (i *Installer) Uninstall(ctx context.Context, module string) error {
	i.deleteWeightedModuleSymlinks(module)

	return i.installer.Uninstall(ctx, module)
}

// Restore ensures the module image is present, verified, and mounted.
func (i *Installer) Restore(ctx context.Context, ms *v1alpha1.ModuleSource, module, version string) error {
	i.deleteWeightedModuleSymlinks(module)

	return i.installer.Restore(ctx, ms, module, version)
}

// deleteModuleSymlink walks over the modules dir and deletes module symlinks by regexp
// TODO(ipaqsa): delete after 1.74
func (i *Installer) deleteWeightedModuleSymlinks(moduleName string) {
	moduleRegexp := regexp.MustCompile(`^(([0-9]+)-)?(` + moduleName + `)$`)
	_ = filepath.WalkDir(filepath.Join(i.downloaded, "modules"), func(path string, d os.DirEntry, _ error) error {
		if !moduleRegexp.MatchString(d.Name()) {
			return nil
		}

		os.RemoveAll(path)

		return filepath.SkipDir
	})
}
