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

package symlink

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	cp "github.com/otiai10/copy"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/d8env"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	tracerName = "installer"
)

// Installer manages module installation via symlinks.
// Modules are copied to /deckhouse/downloaded/<module>/ and symlinked
// to /deckhouse/downloaded/modules/<module> for use by the operator.
type Installer struct {
	symlinkDir string // Path to symlink directory: /deckhouse/downloaded/modules
	downloaded string // Base download path: /deckhouse/downloaded

	registry *registry.Service

	logger *log.Logger
}

// NewInstaller creates an installer that uses symlinks for module management.
// Directory structure:
//
//	/deckhouse/downloaded/<module>/     - Actual module files
//	/deckhouse/downloaded/modules/<version> -> symlink to actual module
func NewInstaller(registry *registry.Service, logger *log.Logger) *Installer {
	downloaded := d8env.GetDownloadedModulesDir()

	return &Installer{
		downloaded: downloaded,
		symlinkDir: filepath.Join(downloaded, "modules"),
		registry:   registry,
		logger:     logger.Named("symlink-installer"),
	}
}

// Install copies a module from temp location to permanent storage and creates symlink.
// Process:
//  1. Create /deckhouse/downloaded/<module>/ directory
//  2. Remove old module version if exists(atomic update)
//  3. Copy module files from tempModulePath to permanent location
//  4. Remove old symlink if exists (atomic update)
//  5. Create new symlink: /deckhouse/downloaded/<module> -> /deckhouse/downloaded/modules/<version>
func (i *Installer) Install(ctx context.Context, module, version, tempModulePath string) error {
	_, span := otel.Tracer(tracerName).Start(ctx, "Install")
	defer span.End()

	span.SetAttributes(attribute.String("module", module))
	span.SetAttributes(attribute.String("version", version))
	span.SetAttributes(attribute.String("path", tempModulePath))

	logger := i.logger.With(slog.String("name", module), slog.String("version", version))

	logger.Debug("install module")

	// Create permanent module directory: /deckhouse/downloaded/<module>
	modulePath := filepath.Join(i.downloaded, module)
	if err := os.MkdirAll(modulePath, 0755); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create module dir '%s': %w", modulePath, err)
	}

	// /deckhouse/downloaded/<module>/<version>
	versionPath := filepath.Join(modulePath, version)

	// Remove old version if exists (for atomic update)
	if _, err := os.Stat(versionPath); err != nil {
		if err = os.Remove(versionPath); err != nil {
			return fmt.Errorf("delete old version '%s': %w", versionPath, err)
		}
	}

	// Prepare symlink location: /deckhouse/downloaded/modules/<module>
	symlinkPoint := filepath.Join(i.symlinkDir, module)

	// Remove old symlink if exists (for atomic update)
	// Use Lstat to avoid following the symlink
	if _, err := os.Lstat(symlinkPoint); err == nil {
		if err = os.Remove(symlinkPoint); err != nil {
			return fmt.Errorf("delete old symlink '%s': %w", symlinkPoint, err)
		}
	}

	// Copy module files to permanent location
	if err := cp.Copy(tempModulePath, versionPath); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("copy module '%s': %w", modulePath, err)
	}

	// Create new symlink pointing to permanent location
	if err := os.Symlink(versionPath, symlinkPoint); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create symlink '%s': %w", symlinkPoint, err)
	}

	return nil
}

// Uninstall removes module symlink and cleans up module files.
// Process:
//  1. Check if symlink exists (returns early if not)
//  2. Explicitly remove symlink
//  3. Defer: Remove entire module directory /deckhouse/downloaded/<module>
//
// Two-phase cleanup ensures symlink is removed before directory cleanup
func (i *Installer) Uninstall(ctx context.Context, module string) error {
	_, span := otel.Tracer(tracerName).Start(ctx, "Uninstall")
	defer span.End()

	span.SetAttributes(attribute.String("module", module))

	logger := i.logger.With(slog.String("name", module))

	logger.Debug("uninstall module")

	// Defer cleanup: remove module directory and all its contents
	defer func() {
		// Remove permanent module directory: /deckhouse/downloaded/<module>
		modulePath := filepath.Join(i.downloaded, module)

		logger.Info("delete module dir", slog.String("path", modulePath))
		if err := os.RemoveAll(modulePath); err != nil {
			logger.Warn("failed to remove downloaded", slog.String("path", modulePath))
		}

		logger.Debug("module uninstalled")
	}()

	// Check if symlink exists: /deckhouse/downloaded/modules/<module>
	symlinkPoint := filepath.Join(i.symlinkDir, module)
	if _, err := os.Stat(symlinkPoint); err != nil {
		if os.IsNotExist(err) {
			return nil // Symlink already removed, continue with cleanup
		}

		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("check symlink '%s': %w", symlinkPoint, err)
	}

	// Explicitly remove symlink before defer cleanup runs
	// Deferred cleanup will handle module directory
	if err := os.Remove(symlinkPoint); err != nil {
		logger.Warn("failed to remove symlink", slog.String("path", symlinkPoint), log.Err(err))
		// Non-fatal: defer will clean up module directory anyway
	}

	return nil
}

// Restore downloads a module from registry and creates symlink.
// Used for recovering modules after restart or failure.
// Process:
//  1. Create /deckhouse/downloaded/<module>/ directory
//  2. Remove old symlink if exists
//  3. Download module from registry
//  4. Create symlink: /deckhouse/downloaded/<module> -> /deckhouse/downloaded/modules/<version>
func (i *Installer) Restore(ctx context.Context, ms *v1alpha1.ModuleSource, module, version string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Restore")
	defer span.End()

	span.SetAttributes(attribute.String("module", module))
	span.SetAttributes(attribute.String("version", version))
	span.SetAttributes(attribute.String("source", ms.GetName()))

	logger := i.logger.With(slog.String("name", module), slog.String("version", version))
	logger.Debug("restore module")

	// Create permanent module directory: /deckhouse/downloaded/<module>
	modulePath := filepath.Join(i.downloaded, module)
	if err := os.MkdirAll(modulePath, 0755); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create module dir '%s': %w", modulePath, err)
	}

	// Prepare symlink location: /deckhouse/downloaded/modules/<module>
	symlinkPoint := filepath.Join(i.symlinkDir, module)

	// Remove old symlink if exists (for atomic update)
	// Use Lstat to avoid following the symlink
	if _, err := os.Lstat(symlinkPoint); err == nil {
		if err = os.Remove(symlinkPoint); err != nil {
			return fmt.Errorf("delete old symlink '%s': %w", symlinkPoint, err)
		}
	}

	// Download module from registry to versioned directory:
	// /deckhouse/downloaded/<module>/<version>
	// This allows multiple versions to coexist before symlink switch
	versionPath := filepath.Join(modulePath, version)

	// Check if module version already exists
	if _, err := os.Stat(versionPath); err != nil {
		if err = i.registry.Download(ctx, registry.BuildRegistryBySource(ms), versionPath, module, version); err != nil {
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("download module '%s': %w", module, err) // Propagate download error
		}
	}

	// Create symlink pointing to permanent location
	if err := os.Symlink(versionPath, symlinkPoint); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create symlink '%s': %w", modulePath, err)
	}

	return nil
}
