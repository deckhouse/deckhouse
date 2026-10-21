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
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/disk"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// tracerName is the OpenTelemetry tracer name for disk operations.
	tracerName = "disk"
	// loggerName is the logger scope for symlink.
	loggerName = "symlink-backend"
)

// Backend handles package lifecycle using symlinks instead of dm-verity mounts.
// Simpler alternative for environments where dm-verity is unavailable.
type Backend struct {
	mu         sync.Mutex
	workingDir string
	registry   registryService
	logger     *log.Logger
}

type registryService interface {
	Download(ctx context.Context, cred registry.Remote, out, packageName, tag string) error
}

// NewBackend creates a disk backend for packages.
func NewBackend(reg registryService, workingDir string, logger *log.Logger) *Backend {
	return &Backend{
		registry:   reg,
		workingDir: workingDir,
		logger:     logger.Named(loggerName),
	}
}

// Attach fetches the image of version from the registry, stores it and exposes it at the deployed
// path; a stored copy is reused unchecked.
func (b *Backend) Attach(ctx context.Context, repo registry.Remote, instance, packageName, version string) error {
	// the same package may be deployed by multiple apps concurrently; serialize to avoid duplicate work
	b.mu.Lock()
	defer b.mu.Unlock()

	packageDir := filepath.Join(b.workingDir, repo.Name, packageName)
	if err := b.download(ctx, repo, packageDir, packageName, version); err != nil {
		return err
	}

	return b.symlink(ctx, packageDir, disk.AttachPath(b.workingDir, instance), instance, version)
}

// Cleanup removes deployed symlinks and package versions not listed in preserve.
func (b *Backend) Cleanup(ctx context.Context, preserve []disk.PreservePackage) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Cleanup")
	defer span.End()

	span.SetAttributes(attribute.String("working_dir", b.workingDir))
	span.SetAttributes(attribute.String("deployed", disk.AttachDir(b.workingDir)))

	logger := b.logger.With(
		slog.String("working_dir", b.workingDir),
		slog.String("deployed", disk.AttachDir(b.workingDir)))

	logger.Debug("cleanup packages")

	b.mu.Lock()
	defer b.mu.Unlock()

	keep := b.buildCleanupKeep(preserve)
	if err := cleanupDeployed(ctx, disk.AttachDir(b.workingDir), keep.versions, logger); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("cleanup deployed: %w", err)
	}

	if err := cleanupPackageDirs(ctx, b.workingDir, keep, logger); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("cleanup package dirs: %w", err)
	}

	return nil
}

type cleanupKeep struct {
	versions map[string]struct{}
	packages map[string]struct{}
	repos    map[string]struct{}
}

// buildCleanupKeep returns normalized paths that must survive cleanup.
func (b *Backend) buildCleanupKeep(preserve []disk.PreservePackage) cleanupKeep {
	keep := cleanupKeep{
		versions: make(map[string]struct{}, len(preserve)),
		packages: make(map[string]struct{}, len(preserve)),
		repos:    make(map[string]struct{}),
	}

	for _, item := range preserve {
		packageDir := filepath.Join(b.workingDir, item.Repository, item.Name)
		versionDir := filepath.Join(packageDir, item.Version)

		keep.versions[normalizePath(versionDir)] = struct{}{}
		keep.packages[normalizePath(packageDir)] = struct{}{}
		keep.repos[normalizePath(filepath.Join(b.workingDir, item.Repository))] = struct{}{}
	}

	return keep
}

// cleanupDeployed removes symlinks whose targets are not preserved package versions.
func cleanupDeployed(ctx context.Context, deployed string, keepVersions map[string]struct{}, logger *log.Logger) error {
	entries, err := os.ReadDir(deployed)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	for _, entry := range entries {
		if err = ctx.Err(); err != nil {
			return err
		}

		path := filepath.Join(deployed, entry.Name())
		info, statErr := os.Lstat(path)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				continue
			}

			return statErr
		}

		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}

		target, readErr := os.Readlink(path)
		if readErr != nil {
			return readErr
		}

		if !filepath.IsAbs(target) {
			target = filepath.Join(deployed, target)
		}

		if _, ok := keepVersions[normalizePath(target)]; ok {
			continue
		}

		logger.Info("delete deployed symlink", slog.String("path", path))
		if err = os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	return nil
}

// cleanupPackageDirs removes package versions and empty parent directories not preserved.
// The deployed subdirectory is skipped here — it is handled by cleanupDeployed.
func cleanupPackageDirs(ctx context.Context, packageDir string, keep cleanupKeep, logger *log.Logger) error {
	entries, err := os.ReadDir(packageDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	for _, entry := range entries {
		if err = ctx.Err(); err != nil {
			return err
		}

		if filepath.Join(packageDir, entry.Name()) == disk.AttachDir(packageDir) {
			continue
		}

		path := filepath.Join(packageDir, entry.Name())

		if _, ok := keep.repos[normalizePath(path)]; ok {
			if err = cleanupRepoDir(ctx, path, keep, logger); err != nil {
				return err
			}

			continue
		}

		logger.Info("delete package dir", slog.String("path", path))
		if err = os.RemoveAll(path); err != nil {
			return err
		}
	}

	return nil
}

// cleanupRepoDir removes package directories not preserved under an application repository.
func cleanupRepoDir(ctx context.Context, repo string, keep cleanupKeep, logger *log.Logger) error {
	entries, err := os.ReadDir(repo)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	for _, entry := range entries {
		if err = ctx.Err(); err != nil {
			return err
		}

		path := filepath.Join(repo, entry.Name())
		if _, ok := keep.packages[normalizePath(path)]; ok {
			if err = cleanupPackageVersions(ctx, path, keep.versions, logger); err != nil {
				return err
			}

			continue
		}

		logger.Info("delete package dir", slog.String("path", path))
		if err = os.RemoveAll(path); err != nil {
			return err
		}
	}

	return removeEmptyDir(repo)
}

// cleanupPackageVersions removes package versions not listed in keepVersions.
func cleanupPackageVersions(ctx context.Context, packageDir string, keepVersions map[string]struct{}, logger *log.Logger) error {
	entries, err := os.ReadDir(packageDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	for _, entry := range entries {
		if err = ctx.Err(); err != nil {
			return err
		}

		path := filepath.Join(packageDir, entry.Name())
		if _, ok := keepVersions[normalizePath(path)]; ok {
			continue
		}

		logger.Info("delete package version dir", slog.String("path", path))
		if err = os.RemoveAll(path); err != nil {
			return err
		}
	}

	return removeEmptyDir(packageDir)
}

// removeEmptyDir removes path only when it has no entries.
func removeEmptyDir(path string) error {
	err := os.Remove(path)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	if errors.Is(err, syscall.ENOTEMPTY) || errors.Is(err, syscall.EEXIST) {
		return nil
	}

	return err
}

// normalizePath returns a comparable absolute clean path when possible.
func normalizePath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}

	return filepath.Clean(abs)
}

// download fetches the image of version into its directory via an atomic temporary directory
// rename. A cached version directory is reused.
func (b *Backend) download(ctx context.Context, repo registry.Remote, packageDir, name, version string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "download")
	defer span.End()

	span.SetAttributes(attribute.String("package", name))
	span.SetAttributes(attribute.String("version", version))
	span.SetAttributes(attribute.String("package_dir", packageDir))
	span.SetAttributes(attribute.String("repository", repo.Name))
	span.SetAttributes(attribute.String("registry", repo.Repository))

	logger := b.logger.With(
		slog.String("name", name),
		slog.String("version", version),
		slog.String("package_dir", packageDir),
		slog.String("repository", repo.Name),
		slog.String("registry", repo.Repository))

	logger.Debug("download package")

	select {
	case <-ctx.Done():
		span.SetStatus(codes.Error, "context canceled")
		return ctx.Err()
	default:
	}

	versionPath := filepath.Join(packageDir, version)

	switch _, err := os.Stat(versionPath); {
	case err == nil:
		return nil
	case !os.IsNotExist(err):
		return fmt.Errorf("check stored version: %w", err)
	}

	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create package dir: %w", err)
	}

	if err := cleanupTempDownloadDirs(packageDir, version); err != nil {
		return fmt.Errorf("clean up temporary downloads: %w", err)
	}

	tempDir, err := os.MkdirTemp(packageDir, tempDownloadPrefix(version))
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create temporary download dir: %w", err)
	}

	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.RemoveAll(tempDir)
		}
	}()

	// download/extract into a temporary directory, then atomically publish it as <packageDir>/<version>
	if err = b.registry.Download(ctx, repo, tempDir, name, version); err != nil {
		return fmt.Errorf("download package: %w", err)
	}

	if err = os.Rename(tempDir, versionPath); err != nil {
		return fmt.Errorf("store downloaded package: %w", err)
	}
	cleanupTemp = false

	return nil
}

// cleanupTempDownloadDirs removes stale temporary download directories for a package version.
func cleanupTempDownloadDirs(packageDir, version string) error {
	entries, err := os.ReadDir(packageDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	prefix := tempDownloadPrefix(version)
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}

		if err = os.RemoveAll(filepath.Join(packageDir, entry.Name())); err != nil {
			return err
		}
	}

	return nil
}

// tempDownloadPrefix returns a filesystem-safe temporary download directory prefix.
func tempDownloadPrefix(version string) string {
	return "." + strings.NewReplacer("/", "_", string(os.PathSeparator), "_").Replace(version) + ".tmp-"
}

// symlink creates a symlink from deployed path to the workingDir version directory.
// Removes any existing symlink for atomic version switching.
func (b *Backend) symlink(ctx context.Context, packageDir, deployed, name, version string) error {
	_, span := otel.Tracer(tracerName).Start(ctx, "symlink")
	defer span.End()

	span.SetAttributes(attribute.String("package_dir", packageDir))
	span.SetAttributes(attribute.String("deployed", deployed))
	span.SetAttributes(attribute.String("instance", name))
	span.SetAttributes(attribute.String("version", version))

	logger := b.logger.With(
		slog.String("package_dir", packageDir),
		slog.String("deployed", deployed),
		slog.String("name", name),
		slog.String("version", version))

	logger.Debug("attach package")

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
			return fmt.Errorf("remove old symlink: %w", err)
		}
	}

	// Create parent directory if it does not exist (for new clusters).
	if err := os.MkdirAll(filepath.Dir(deployed), 0o755); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create deployed dir: %w", err)
	}

	// <packageDir>/<version>
	versionPath := filepath.Join(packageDir, version)
	if _, err := os.Stat(versionPath); err != nil {
		return fmt.Errorf("check stored version: %w", err)
	}

	// Create new symlink pointing to permanent location
	if err := os.Symlink(versionPath, deployed); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create symlink: %w", err)
	}

	return nil
}

// Detach removes the symlink. The stored version stays: only Cleanup purges files.
func (b *Backend) Detach(ctx context.Context, instance string) error {
	_, span := otel.Tracer(tracerName).Start(ctx, "Detach")
	defer span.End()

	span.SetAttributes(attribute.String("instance", instance))

	deployed := disk.AttachPath(b.workingDir, instance)
	logger := b.logger.With(slog.String("name", instance))

	logger.Debug("detach package")

	if _, err := os.Readlink(deployed); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("check deployed symlink: %w", err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if err := os.Remove(deployed); err != nil {
		logger.Warn("failed to remove symlink", slog.String("path", deployed), log.Err(err))
	}

	return nil
}
