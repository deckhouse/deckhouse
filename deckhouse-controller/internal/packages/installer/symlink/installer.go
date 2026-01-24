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
	"log/slog"
	"os"
	"path/filepath"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	tracerName = "installer"
)

// Installer handles package lifecycle using symlinks instead of dm-verity mounts.
// Simpler alternative for environments where dm-verity is unavailable.
type Installer struct {
	registry registryService
	logger   *log.Logger
}

type registryService interface {
	Download(ctx context.Context, cred registry.Remote, out, packageName, tag string) error
}

// NewInstaller creates an Installer with the given registry service.
func NewInstaller(reg registryService, logger *log.Logger) *Installer {
	return &Installer{
		registry: reg,
		logger:   logger.Named("symlink-installer"),
	}
}

// Download fetches a package image from the registry to <downloaded>/<version>.
func (i *Installer) Download(ctx context.Context, repo registry.Remote, downloaded, name, version string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Download")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))
	span.SetAttributes(attribute.String("version", version))
	span.SetAttributes(attribute.String("downloaded", downloaded))
	span.SetAttributes(attribute.String("repository", repo.Name))
	span.SetAttributes(attribute.String("registry", repo.Repository))

	logger := i.logger.With(
		slog.String("name", name),
		slog.String("version", version),
		slog.String("downloaded", downloaded),
		slog.String("repository", repo.Name),
		slog.String("registry", repo.Repository))

	logger.Debug("download package")

	select {
	case <-ctx.Done():
		span.SetStatus(codes.Error, "context canceled")
		return ctx.Err()
	default:
	}

	// <downloaded>/<version>.erofs
	imagePath := filepath.Join(downloaded, version)
	if err := os.MkdirAll(filepath.Dir(imagePath), 0755); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newCreatePackageDirErr(err)
	}

	if err := i.registry.Download(ctx, repo, imagePath, name, version); err != nil {
		return newDownloadErr(err)
	}

	return nil
}

// Install creates a symlink from deployed path to the downloaded version directory.
// Removes any existing symlink for atomic version switching.
func (i *Installer) Install(ctx context.Context, downloaded, deployed, name, version string) error {
	_, span := otel.Tracer(tracerName).Start(ctx, "Install")
	defer span.End()

	span.SetAttributes(attribute.String("downloaded", downloaded))
	span.SetAttributes(attribute.String("deployed", deployed))
	span.SetAttributes(attribute.String("name", name))
	span.SetAttributes(attribute.String("version", version))

	logger := i.logger.With(
		slog.String("downloaded", downloaded),
		slog.String("deployed", deployed),
		slog.String("name", name),
		slog.String("version", version))

	logger.Debug("install package")

	select {
	case <-ctx.Done():
		span.SetStatus(codes.Error, "context canceled")
		return ctx.Err()
	default:
	}

	// Remove old symlink if exists (for atomic update)
	// Use Lstat to avoid following the symlink
	if _, err := os.Lstat(deployed); err == nil {
		if err = os.Remove(deployed); err != nil {
			return newRemoveOldVersionErr(err)
		}
	}

	// <downloaded>/<version>
	versionPath := filepath.Join(downloaded, version)
	if _, err := os.Stat(versionPath); err != nil {
		return newCheckVersionErr(err)
	}

	// Create new symlink pointing to permanent location
	if err := os.Symlink(versionPath, deployed); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newCreateSymlinkErr(err)
	}

	return nil
}

// Uninstall removes the symlink. If keep=false, also deletes downloaded files.
func (i *Installer) Uninstall(ctx context.Context, downloaded, deployed, name string, keep bool) error {
	_, span := otel.Tracer(tracerName).Start(ctx, "Uninstall")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))

	logger := i.logger.With(slog.String("name", name))

	logger.Debug("uninstall package")

	if _, err := os.Lstat(deployed); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		span.SetStatus(codes.Error, err.Error())
		return newCheckMountErr(err)
	}

	// clear package dir
	defer func() {
		if keep {
			return
		}

		logger.Info("delete package dir", slog.String("path", downloaded))
		if err := os.RemoveAll(downloaded); err != nil {
			logger.Warn("failed to remove downloaded images", slog.String("path", downloaded))
		}
	}()

	// Explicitly remove symlink before defer cleanup runs
	// Deferred cleanup will handle module directory
	if err := os.Remove(deployed); err != nil {
		logger.Warn("failed to remove symlink", slog.String("path", deployed), log.Err(err))
		// Non-fatal: defer will clean up module directory anyway
	}

	return nil
}
