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

type Installer struct {
	symlinkDir string
	downloaded string

	registry *registry.Service

	logger *log.Logger
}

func NewInstaller(registry *registry.Service, logger *log.Logger) *Installer {
	downloaded := d8env.GetDownloadedModulesDir()

	return &Installer{
		downloaded: downloaded,
		symlinkDir: filepath.Join(downloaded, "modules"),
		registry:   registry,
		logger:     logger,
	}
}

func (i *Installer) Install(ctx context.Context, module, version, tempModulePath string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Install")
	defer span.End()

	span.SetAttributes(attribute.String("module", module))
	span.SetAttributes(attribute.String("version", version))
	span.SetAttributes(attribute.String("path", tempModulePath))

	logger := i.logger.With(slog.String("name", module), slog.String("version", version))

	logger.Debug("install module")

	// /deckhouse/downloaded/<module>
	modulePath := filepath.Join(i.downloaded, module)
	if err := os.MkdirAll(modulePath, 0755); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create module dir '%s': %w", modulePath, err)
	}

	if err := cp.Copy(tempModulePath, modulePath); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("copy module '%s': %w", modulePath, err)
	}

	// /deckhouse/downloaded/modules/<module>
	symlinkPoint := filepath.Join(i.symlinkDir, module)

	// delete the old module symlink if exists
	if _, err := os.Lstat(symlinkPoint); err == nil {
		if err = os.Remove(symlinkPoint); err != nil {
			return fmt.Errorf("delete old symlink '%s': %w", symlinkPoint, err)
		}
	}

	if err := os.Symlink(modulePath, symlinkPoint); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create symlink '%s': %w", symlinkPoint, err)
	}

	return nil
}

func (i *Installer) Uninstall(ctx context.Context, module string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Uninstall")
	defer span.End()

	span.SetAttributes(attribute.String("module", module))

	logger := i.logger.With(slog.String("name", module))

	logger.Debug("uninstall module")

	// clear module dir
	defer func() {
		// /deckhouse/downloaded/<module>
		versionsPath := filepath.Join(i.downloaded, module)

		logger.Info("delete module dir", slog.String("path", versionsPath))
		if err := os.RemoveAll(versionsPath); err != nil {
			logger.Warn("failed to remove downloaded", slog.String("path", versionsPath))
		}

		logger.Debug("module uninstalled")
	}()

	// /deckhouse/downloaded/modules/<module>
	symlinkPoint := filepath.Join(i.symlinkDir, module)
	if _, err := os.Stat(symlinkPoint); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("check symlink '%s': %w", symlinkPoint, err)
	}

	return nil
}

func (i *Installer) Restore(ctx context.Context, ms *v1alpha1.ModuleSource, module, version string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Restore")
	defer span.End()

	span.SetAttributes(attribute.String("module", module))
	span.SetAttributes(attribute.String("version", version))
	span.SetAttributes(attribute.String("source", ms.GetName()))

	logger := i.logger.With(slog.String("name", module), slog.String("version", version))
	logger.Debug("restore module")

	modulePath := filepath.Join(i.downloaded, module)
	if err := os.MkdirAll(modulePath, 0755); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create module dir '%s': %w", modulePath, err)
	}

	// /deckhouse/downloaded/modules/<module>
	symlinkPoint := filepath.Join(i.symlinkDir, module)

	// delete the old module symlink if exists
	if _, err := os.Lstat(symlinkPoint); err == nil {
		if err = os.Remove(symlinkPoint); err != nil {
			return fmt.Errorf("delete old symlink '%s': %w", symlinkPoint, err)
		}
	}

	versionPath := filepath.Join(modulePath, module)
	if err := i.registry.Download(ctx, registry.BuildRegistryBySource(ms), versionPath, module, version); err != nil {
		span.SetStatus(codes.Error, err.Error())
	}

	if err := os.Symlink(modulePath, filepath.Join(i.symlinkDir, module)); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create symlink '%s': %w", modulePath, err)
	}

	return nil
}
