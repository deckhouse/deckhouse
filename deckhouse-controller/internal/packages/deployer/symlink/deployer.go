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

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/deployer"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// tracerName is the OpenTelemetry tracer name for deployer operations.
	tracerName = "deployer"
	// deployedDir is the deployed packages directory name.
	deployedDir = "deployed"
	// loggerName is the logger scope for symlink deployment.
	loggerName = "symlink-deployer"
)

// Deployer handles package lifecycle using symlinks instead of dm-verity mounts.
// Simpler alternative for environments where dm-verity is unavailable.
type Deployer struct {
	mu         sync.Mutex
	workingDir string
	registry   registryService
	logger     *log.Logger
}

type registryService interface {
	Download(ctx context.Context, cred registry.Remote, out, packageName, tag string) error
}

// NewDeployer creates a Deployer for packages.
func NewDeployer(reg registryService, workingDir string, logger *log.Logger) *Deployer {
	return &Deployer{
		registry:   reg,
		workingDir: workingDir,
		logger:     logger.Named(loggerName),
	}
}

// Deploy fetches a package image from the registry and exposes it at the deployed path.
func (d *Deployer) Deploy(ctx context.Context, repo registry.Remote, packageName, deployedName, version string) error {
	// one package can be downloaded by different apps in the same time
	// so lock it to prevent downloading same package
	d.mu.Lock()
	defer d.mu.Unlock()

	downloaded := d.downloadedPath(repo.Name, packageName)
	if err := d.download(ctx, repo, downloaded, packageName, version); err != nil {
		return err
	}

	return d.symlink(ctx, downloaded, d.deployedPath(deployedName), deployedName, version)
}

// Cleanup removes deployed symlinks and downloaded package versions not listed in preserve.
func (d *Deployer) Cleanup(ctx context.Context, preserve []deployer.PreservePackage) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Cleanup")
	defer span.End()

	span.SetAttributes(attribute.String("downloaded", d.workingDir))
	span.SetAttributes(attribute.String("deployed", d.deployedRoot()))

	logger := d.logger.With(
		slog.String("downloaded", d.workingDir),
		slog.String("deployed", d.deployedRoot()))

	logger.Debug("cleanup packages")

	d.mu.Lock()
	defer d.mu.Unlock()

	keep := d.buildCleanupKeep(preserve)
	if err := cleanupDeployed(ctx, d.deployedRoot(), keep.versions, logger); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("cleanup deployed: %w", err)
	}

	if err := cleanupDownloaded(ctx, d.workingDir, keep, logger); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("cleanup downloaded: %w", err)
	}

	return nil
}

// downloadedPath returns a package download directory under the deployer root.
func (d *Deployer) downloadedPath(repository, packageName string) string {
	return filepath.Join(d.workingDir, repository, packageName)
}

// deployedRoot returns the directory containing deployed package symlinks.
func (d *Deployer) deployedRoot() string {
	return filepath.Join(d.workingDir, deployedDir)
}

// deployedPath returns a package deployed path under the deployer root.
func (d *Deployer) deployedPath(deployedName string) string {
	return filepath.Join(d.deployedRoot(), deployedName)
}

type cleanupKeep struct {
	versions map[string]struct{}
	packages map[string]struct{}
	repos    map[string]struct{}
}

// buildCleanupKeep returns normalized paths that must survive cleanup.
func (d *Deployer) buildCleanupKeep(preserve []deployer.PreservePackage) cleanupKeep {
	keep := cleanupKeep{
		versions: make(map[string]struct{}, len(preserve)),
		packages: make(map[string]struct{}, len(preserve)),
		repos:    make(map[string]struct{}),
	}

	for _, item := range preserve {
		packageDir := d.cleanupPackageDir(item)
		versionDir := filepath.Join(packageDir, item.Version)

		keep.versions[normalizePath(versionDir)] = struct{}{}
		keep.packages[normalizePath(packageDir)] = struct{}{}
		keep.repos[normalizePath(filepath.Join(d.workingDir, item.Repository))] = struct{}{}
	}

	return keep
}

// cleanupPackageDir returns the workingDir package directory for a preserved package.
func (d *Deployer) cleanupPackageDir(item deployer.PreservePackage) string {
	return filepath.Join(d.workingDir, item.Repository, item.Name)
}

// cleanupDeployed removes symlinks whose targets are not preserved downloaded versions.
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

// cleanupDownloaded removes package versions and empty parent directories not preserved.
// The deployed subdirectory is skipped here — it is handled by cleanupDeployed.
func cleanupDownloaded(ctx context.Context, downloaded string, keep cleanupKeep, logger *log.Logger) error {
	entries, err := os.ReadDir(downloaded)
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

		if entry.Name() == deployedDir {
			continue
		}

		path := filepath.Join(downloaded, entry.Name())

		if _, ok := keep.repos[normalizePath(path)]; ok {
			if err = cleanupRepoDir(ctx, path, keep, logger); err != nil {
				return err
			}

			continue
		}

		logger.Info("delete downloaded dir", slog.String("path", path))
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

		logger.Info("delete downloaded package dir", slog.String("path", path))
		if err = os.RemoveAll(path); err != nil {
			return err
		}
	}

	return removeEmptyDir(repo)
}

// cleanupPackageVersions removes downloaded versions not listed in keepVersions.
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

		logger.Info("delete downloaded version dir", slog.String("path", path))
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

// download fetches a package image into a versioned directory via an atomic temporary directory rename.
func (d *Deployer) download(ctx context.Context, repo registry.Remote, downloaded, name, version string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "download")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))
	span.SetAttributes(attribute.String("version", version))
	span.SetAttributes(attribute.String("downloaded", downloaded))
	span.SetAttributes(attribute.String("repository", repo.Name))
	span.SetAttributes(attribute.String("registry", repo.Repository))

	logger := d.logger.With(
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

	versionPath := filepath.Join(downloaded, version)
	if _, err := os.Stat(versionPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return newCheckVersionErr(err)
	}

	if err := os.MkdirAll(downloaded, 0755); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newCreatePackageDirErr(err)
	}

	if err := cleanupTempDownloadDirs(downloaded, version); err != nil {
		return newDownloadErr(err)
	}

	tempDir, err := os.MkdirTemp(downloaded, tempDownloadPrefix(version))
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newCreatePackageDirErr(err)
	}

	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.RemoveAll(tempDir)
		}
	}()

	// download/extract into a temporary directory, then atomically publish it as <downloaded>/<version>
	if err = d.registry.Download(ctx, repo, tempDir, name, version); err != nil {
		return newDownloadErr(err)
	}

	if err = os.Rename(tempDir, versionPath); err != nil {
		return newDownloadErr(err)
	}
	cleanupTemp = false

	return nil
}

// cleanupTempDownloadDirs removes stale temporary download directories for a package version.
func cleanupTempDownloadDirs(downloaded, version string) error {
	entries, err := os.ReadDir(downloaded)
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

		if err = os.RemoveAll(filepath.Join(downloaded, entry.Name())); err != nil {
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
func (d *Deployer) symlink(ctx context.Context, downloaded, deployed, name, version string) error {
	_, span := otel.Tracer(tracerName).Start(ctx, "symlink")
	defer span.End()

	span.SetAttributes(attribute.String("downloaded", downloaded))
	span.SetAttributes(attribute.String("deployed", deployed))
	span.SetAttributes(attribute.String("name", name))
	span.SetAttributes(attribute.String("version", version))

	logger := d.logger.With(
		slog.String("downloaded", downloaded),
		slog.String("deployed", deployed),
		slog.String("name", name),
		slog.String("version", version))

	logger.Debug("deploy package")

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

	return nil
}

// Undeploy removes the symlink. If keep=false, also deletes downloaded files.
func (d *Deployer) Undeploy(ctx context.Context, deployedName string, keep bool) error {
	_, span := otel.Tracer(tracerName).Start(ctx, "Undeploy")
	defer span.End()

	span.SetAttributes(attribute.String("name", deployedName))

	deployed := d.deployedPath(deployedName)
	logger := d.logger.With(slog.String("name", deployedName))

	logger.Debug("undeploy package")

	target, err := os.Readlink(deployed)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		span.SetStatus(codes.Error, err.Error())
		return newCheckMountErr(err)
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// clear package dir
	defer func() {
		if keep {
			return
		}

		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(deployed), target)
		}

		downloaded := filepath.Dir(target)
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
