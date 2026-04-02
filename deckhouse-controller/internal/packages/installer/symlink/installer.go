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
	"strings"
	"sync"

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
	mu         sync.Mutex
	downloaded map[string]struct{}
	deployed   map[string]struct{}
	registry   registryService
	logger     *log.Logger
}

type registryService interface {
	Download(ctx context.Context, cred registry.Remote, out, packageName, tag string) error
}

// NewInstaller creates an Installer with the given registry service.
func NewInstaller(reg registryService, logger *log.Logger) *Installer {
	return &Installer{
		downloaded: make(map[string]struct{}),
		deployed:   make(map[string]struct{}),
		registry:   reg,
		logger:     logger.Named("symlink-installer"),
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

	// one package can be downloaded by different apps in the same time
	// so lock it to prevent downloading same package
	i.mu.Lock()
	defer i.mu.Unlock()

	versionPath := filepath.Join(downloaded, version)
	if _, err := os.Stat(versionPath); err == nil {
		i.downloaded[versionPath] = struct{}{}
		return nil
	}

	// download/extract into <downloaded>/<version> directory
	if err := i.registry.Download(ctx, repo, versionPath, name, version); err != nil {
		return newDownloadErr(err)
	}

	i.downloaded[versionPath] = struct{}{}
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

	// one package can be downloaded by different apps in the same time
	// so lock it to prevent downloading same package
	i.mu.Lock()
	defer i.mu.Unlock()

	// Remove old symlink if exists (for atomic update)
	// Use Lstat to avoid following the symlink
	if _, err := os.Lstat(deployed); err == nil {
		if err = os.Remove(deployed); err != nil {
			return newRemoveOldVersionErr(err)
		}
	}

	// Create parent directory if it does not exist (for new clusters).
	if err := os.MkdirAll(filepath.Dir(deployed), 0755); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newCreatePackageDirErr(err)
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

	i.deployed[deployed] = struct{}{}
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

	// Remove from tracking maps so cleanup won't protect stale entries.
	delete(i.deployed, deployed)
	for path := range i.downloaded {
		if strings.HasPrefix(path, downloaded+string(filepath.Separator)) {
			delete(i.downloaded, path)
		}
	}

	return nil
}

// Cleanup removes stale symlinks from deployed and stale directories from downloaded.
// Both parameters must be root directories (not package-level).
// Handles both module (<downloaded>/<package>/<version>) and
// application (<downloaded>/<registry>/<package>/<version>) layouts.
func (i *Installer) Cleanup(ctx context.Context, downloaded, deployed string, exclude ...string) {
	_, span := otel.Tracer(tracerName).Start(ctx, "Cleanup")
	defer span.End()

	i.cleanupDeployed(deployed)

	skip := make(map[string]struct{}, len(exclude)+1)
	skip[deployed] = struct{}{}
	for _, path := range exclude {
		skip[path] = struct{}{}
	}

	i.cleanDownloaded(downloaded, skip)
}

// cleanupDeployed removes symlinks not tracked in i.deployed.
func (i *Installer) cleanupDeployed(deployed string) {
	logger := i.logger.With(slog.String("deployed", deployed))

	links, err := os.ReadDir(deployed)
	if err != nil {
		return
	}

	for _, link := range links {
		linkPath := filepath.Join(deployed, link.Name())
		if _, ok := i.deployed[linkPath]; ok {
			continue
		}

		logger.Info("remove stale symlink", slog.String("path", linkPath))
		if err = os.Remove(linkPath); err != nil {
			logger.Warn("failed to remove symlink", slog.String("path", linkPath), log.Err(err))
		}
	}
}

// cleanDownloaded walks the downloaded tree and removes any directory
// not on the path to a tracked version. Prunes stale registries, packages,
// and version directories in a single pass.
// Paths in skip are preserved (e.g. deployed dir, sibling roots).
func (i *Installer) cleanDownloaded(downloaded string, skip map[string]struct{}) {
	logger := i.logger.With(slog.String("downloaded", downloaded))

	// Build ancestor set: every directory from each tracked version up to root.
	keep := make(map[string]struct{}, len(i.downloaded)*3)
	for path := range i.downloaded {
		for path != downloaded {
			keep[path] = struct{}{}
			path = filepath.Dir(path)
		}
	}

	_ = filepath.WalkDir(downloaded, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == downloaded {
			return nil
		}
		if _, ok := skip[path]; ok {
			return filepath.SkipDir
		}
		if _, ok := keep[path]; ok {
			return nil
		}

		logger.Info("remove stale path", slog.String("path", path))
		if err = os.RemoveAll(path); err != nil {
			logger.Warn("failed to remove path", slog.String("path", path), log.Err(err))
		}
		return filepath.SkipDir
	})
}
