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

package erofs

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/tools/verity"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// tracerName is the OpenTelemetry tracer name for deployer operations.
	tracerName = "deployer"
	// deployedDir is the deployed packages directory name.
	deployedDir = "deployed"
	// loggerName is the logger scope for erofs deployment.
	loggerName = "erofs-deployer"
)

// Deployer handles package lifecycle using erofs images with dm-verity integrity.
// Operations are serialized via mutex to prevent concurrent mount/unmount conflicts.
type Deployer struct {
	mu         sync.Mutex
	workingDir string
	registry   registryService
	logger     *log.Logger
}

type registryService interface {
	GetImageRootHash(ctx context.Context, cred registry.Remote, packageName, tag string) (string, error)
	GetImageReader(ctx context.Context, cred registry.Remote, packageName, tag string) (io.ReadCloser, error)
}

// NewDeployer creates a Deployer for packages.
func NewDeployer(registry registryService, workingDir string, logger *log.Logger) *Deployer {
	return &Deployer{
		registry:   registry,
		workingDir: workingDir,
		logger:     logger.Named(loggerName),
	}
}

// Deploy fetches a package image from the registry and mounts it at the deployed path.
func (d *Deployer) Deploy(ctx context.Context, repo registry.Remote, packageName, deployedName, version string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	downloaded := d.downloadedPath(repo.Name, packageName)
	if err := d.download(ctx, repo, downloaded, packageName, version); err != nil {
		return err
	}

	return d.mount(ctx, downloaded, d.deployedPath(deployedName), deployedName, version)
}

// Cleanup unmounts deployed packages and removes downloaded images not listed in preserve.
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
	if err := cleanupDeployed(ctx, d.deployedRoot(), keep.images, logger); err != nil {
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

// deployedRoot returns the directory containing deployed package mount points.
func (d *Deployer) deployedRoot() string {
	return filepath.Join(d.workingDir, deployedDir)
}

// deployedPath returns a package deployed path under the deployer root.
func (d *Deployer) deployedPath(deployedName string) string {
	return filepath.Join(d.deployedRoot(), deployedName)
}

type cleanupKeep struct {
	images   map[string]struct{}
	packages map[string]struct{}
	repos    map[string]struct{}
}

// buildCleanupKeep returns normalized paths that must survive cleanup.
func (d *Deployer) buildCleanupKeep(preserve []deployer.PreservePackage) cleanupKeep {
	keep := cleanupKeep{
		images:   make(map[string]struct{}, len(preserve)),
		packages: make(map[string]struct{}, len(preserve)),
		repos:    make(map[string]struct{}),
	}

	for _, item := range preserve {
		packageDir := d.cleanupPackageDir(item)
		imagePath := packageImagePath(packageDir, item.Version)

		keep.images[normalizePath(imagePath)] = struct{}{}
		keep.packages[normalizePath(packageDir)] = struct{}{}
		keep.repos[normalizePath(filepath.Join(d.workingDir, item.Repository))] = struct{}{}
	}

	return keep
}

// cleanupPackageDir returns the workingDir package directory for a preserved package.
func (d *Deployer) cleanupPackageDir(item deployer.PreservePackage) string {
	return filepath.Join(d.workingDir, item.Repository, item.Name)
}

// cleanupDeployed unmounts deployed packages whose backing images are not preserved.
func cleanupDeployed(ctx context.Context, deployed string, keepImages map[string]struct{}, logger *log.Logger) error {
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
		imagePath, imageErr := verity.GetImagePathByDevice(ctx, entry.Name())
		if imageErr != nil {
			// Best-effort: a single missing/transient mapper must not block the rest of cleanup.
			logger.Warn("get erofs image path", slog.String("name", entry.Name()), log.Err(imageErr))
			continue
		}

		if _, ok := keepImages[normalizePath(imagePath)]; ok {
			continue
		}

		logger.Info("delete deployed mount", slog.String("path", path))
		if err = verity.Unmount(ctx, path); err != nil {
			return err
		}

		if err = verity.CloseMapper(ctx, entry.Name()); err != nil {
			return err
		}
	}

	return nil
}

// cleanupDownloaded removes package images and empty parent directories not preserved.
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

// cleanupRepoDir removes package directories not preserved under a repository.
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
			if err = cleanupPackageImages(ctx, path, keep.images, logger); err != nil {
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

// cleanupPackageImages removes downloaded image files not listed in keepImages.
func cleanupPackageImages(ctx context.Context, packageDir string, keepImages map[string]struct{}, logger *log.Logger) error {
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
		if _, ok := keepImages[normalizePath(path)]; ok {
			continue
		}

		logger.Info("delete downloaded image", slog.String("path", path))
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

// download fetches a package image from the registry and creates an erofs image.
// If the image already exists and passes verification, download is skipped.
func (d *Deployer) download(ctx context.Context, repo registry.Remote, downloaded, name, version string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Download")
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

	imagePath := packageImagePath(downloaded, version)
	if err := os.MkdirAll(downloaded, 0755); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newCreatePackageDirErr(err)
	}

	rootHash, err := d.registry.GetImageRootHash(ctx, repo, name, version)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newGetRootHashErr(err)
	}

	// skip download if image exists and passes integrity check
	logger.Debug("verify package image")
	if err = d.verifyImage(ctx, imagePath, rootHash); err == nil {
		logger.Debug("package image verified")

		return nil
	}

	// verification failed - fetch fresh image from registry
	logger.Warn("verify package image failed", log.Err(err))

	if err = cleanupTempImageFiles(downloaded, version); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newImageByTarErr(err)
	}

	tempImagePath, err := createTempImagePath(downloaded, version)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newCreatePackageDirErr(err)
	}

	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.Remove(tempImagePath)
			_ = os.Remove(verityPath(tempImagePath))
		}
	}()

	img, err := d.registry.GetImageReader(ctx, repo, name, version)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newGetImageReaderErr(err)
	}
	defer img.Close()

	logger.Debug("create erofs image by package image", slog.String("path", tempImagePath))
	if err = verity.CreateImageByTar(ctx, img, tempImagePath); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newImageByTarErr(err)
	}

	if err = d.verifyImage(ctx, tempImagePath, rootHash); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newImageByTarErr(err)
	}

	_ = os.Remove(verityPath(tempImagePath))

	if err = os.Rename(tempImagePath, imagePath); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newImageByTarErr(err)
	}

	cleanupTemp = false

	return nil
}

// packageImagePath returns the final erofs image path for a package version.
func packageImagePath(downloaded, version string) string {
	return filepath.Join(downloaded, fmt.Sprintf("%s.erofs", version))
}

// verityPath returns the dm-verity hash tree path for an erofs image path.
func verityPath(imagePath string) string {
	return fmt.Sprintf("%s.verity", imagePath)
}

// cleanupTempImageFiles removes stale temporary erofs images for a package version.
func cleanupTempImageFiles(downloaded, version string) error {
	entries, err := os.ReadDir(downloaded)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	prefix := tempImagePrefix(version)
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}

		if err = os.Remove(filepath.Join(downloaded, entry.Name())); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	return nil
}

// createTempImagePath reserves and returns a same-directory temporary erofs image path.
func createTempImagePath(downloaded, version string) (string, error) {
	file, err := os.CreateTemp(downloaded, tempImagePrefix(version))
	if err != nil {
		return "", err
	}

	path := file.Name()
	if err = file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}

	if err = os.Remove(path); err != nil {
		return "", err
	}

	return path, nil
}

// tempImagePrefix returns a filesystem-safe temporary erofs image filename prefix.
func tempImagePrefix(version string) string {
	return "." + strings.NewReplacer("/", "_", string(os.PathSeparator), "_").Replace(version) + ".erofs.tmp-"
}

// mount mounts an erofs image using dm-verity for integrity verification.
// Flow: compute hash → unmount old → close mapper → create mapper → mount new.
func (d *Deployer) mount(ctx context.Context, downloaded, deployed, name, version string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Deploy")
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

	// <downloaded>/<version>.erofs
	imagePath := packageImagePath(downloaded, version)

	// Compute the new image hash before unmounting the currently deployed package.
	logger.Debug("compute erofs image hash", slog.String("path", imagePath))
	rootHash, err := verity.CreateImageHash(ctx, imagePath)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newComputeHashErr(err)
	}

	// cleanup any existing mount before deploying new version
	logger.Debug("unmount old erofs image", slog.String("path", deployed))
	if err = verity.Unmount(ctx, deployed); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newUnmountErr(err)
	}

	logger.Debug("close old device mapper")
	if err = verity.CloseMapper(ctx, name); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newCloseDeviceMapperErr(err)
	}

	// setup dm-verity device mapper with root hash for runtime integrity checks
	logger.Debug("create device mapper", slog.String("path", deployed))
	if err = verity.CreateMapper(ctx, name, imagePath, rootHash); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newCreateDeviceMapperErr(err)
	}

	logger.Debug("mount erofs image mapper", slog.String("path", deployed))
	if err = verity.Mount(ctx, name, deployed); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newMountErr(err)
	}

	return nil
}

// Undeploy unmounts the erofs image and closes the dm-verity mapper.
// If keep=false, the downloaded image files are also deleted.
func (d *Deployer) Undeploy(ctx context.Context, deployedName string, keep bool) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Undeploy")
	defer span.End()

	span.SetAttributes(attribute.String("name", deployedName))

	deployed := d.deployedPath(deployedName)
	logger := d.logger.With(slog.String("name", deployedName))

	logger.Debug("undeploy package")

	if _, err := os.Stat(deployed); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("check mount path '%s': %w", deployed, err)
	}

	// mounts should not be executed simultaneously
	d.mu.Lock()
	defer d.mu.Unlock()

	// Resolve the backing image before tearing down the mapper, so the
	// deferred package-dir cleanup can target the actual download path
	// (the parent of <package>/<version>.erofs) regardless of layout.
	var packageDir string
	if !keep {
		imagePath, err := verity.GetImagePathByDevice(ctx, deployedName)
		if err != nil {
			logger.Warn("resolve image path for cleanup", log.Err(err))
		} else {
			packageDir = filepath.Dir(imagePath)
		}
	}

	defer func() {
		if keep || packageDir == "" {
			return
		}

		logger.Info("delete package dir", slog.String("path", packageDir))
		if err := os.RemoveAll(packageDir); err != nil {
			logger.Warn("failed to remove downloaded images", slog.String("path", packageDir), log.Err(err))
		}
	}()

	logger.Debug("unmount erofs image", slog.String("path", deployed))
	if err := verity.Unmount(ctx, deployed); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("unmount erofs image '%s': %w", deployed, err)
	}

	logger.Debug("close device mapper")
	if err := verity.CloseMapper(ctx, deployedName); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("close device mapper: %w", err)
	}

	logger.Debug("package undeployed")

	return nil
}

// normalizePath returns a comparable absolute clean path when possible.
func normalizePath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}

	return filepath.Clean(abs)
}

// verifyImage checks that the image exists and matches rootHash when one is provided.
func (d *Deployer) verifyImage(ctx context.Context, imagePath, rootHash string) error {
	if _, err := os.Stat(imagePath); err != nil {
		return fmt.Errorf("stat package image '%s': %w", imagePath, err)
	}

	if strings.TrimSpace(rootHash) == "" {
		return nil
	}

	computedHash, err := verity.CreateImageHash(ctx, imagePath)
	if err != nil {
		return err
	}
	if computedHash != rootHash {
		return fmt.Errorf("root hash mismatch")
	}

	return nil
}
