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

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/disk"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/tools/verity"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// tracerName is the OpenTelemetry tracer name for disk operations.
	tracerName = "disk"
	// loggerName is the logger scope for erofs.
	loggerName = "erofs-backend"
)

// Backend handles package lifecycle using erofs images with dm-verity integrity.
// Operations are serialized via mutex to prevent concurrent mount/unmount conflicts.
type Backend struct {
	mu         sync.Mutex
	workingDir string
	registry   registryService
	logger     *log.Logger
}

type registryService interface {
	GetImageRootHash(ctx context.Context, cred registry.Remote, packageName, tag string) (string, error)
	GetImageReader(ctx context.Context, cred registry.Remote, packageName, tag string) (io.ReadCloser, error)
}

// NewBackend creates a disk backend for packages.
func NewBackend(registry registryService, workingDir string, logger *log.Logger) *Backend {
	return &Backend{
		registry:   registry,
		workingDir: workingDir,
		logger:     logger.Named(loggerName),
	}
}

// Attach fetches the image of version from the registry, stores it and mounts it at the deployed
// path.
func (b *Backend) Attach(ctx context.Context, repo registry.Remote, instance, packageName, version string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	dir := filepath.Join(b.workingDir, repo.Name, packageName)
	if err := b.download(ctx, repo, dir, packageName, version); err != nil {
		return err
	}

	return b.mount(ctx, dir, disk.AttachPath(b.workingDir, instance), instance, version)
}

// Cleanup unmounts deployed packages and removes package images not listed in preserve.
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
	if err := cleanupDeployed(ctx, disk.AttachDir(b.workingDir), keep.images, logger); err != nil {
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
	images   map[string]struct{}
	packages map[string]struct{}
	repos    map[string]struct{}
}

// buildCleanupKeep returns normalized paths that must survive cleanup.
func (b *Backend) buildCleanupKeep(preserve []disk.PreservePackage) cleanupKeep {
	keep := cleanupKeep{
		images:   make(map[string]struct{}, len(preserve)),
		packages: make(map[string]struct{}, len(preserve)),
		repos:    make(map[string]struct{}),
	}

	for _, item := range preserve {
		packageDir := filepath.Join(b.workingDir, item.Repository, item.Name)
		imagePath := packageImagePath(packageDir, item.Version)

		keep.images[normalizePath(imagePath)] = struct{}{}
		keep.packages[normalizePath(packageDir)] = struct{}{}
		keep.repos[normalizePath(filepath.Join(b.workingDir, item.Repository))] = struct{}{}
	}

	return keep
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

		// erofs never creates symlinks here, they are leftovers of the symlink backend
		if entry.Type()&os.ModeSymlink != 0 {
			logger.Info("delete leftover deployed symlink", slog.String("path", path))
			if err = os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}

			continue
		}

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

// cleanupPackageDirs removes package images and empty parent directories not preserved.
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

		logger.Info("delete package dir", slog.String("path", path))
		if err = os.RemoveAll(path); err != nil {
			return err
		}
	}

	return removeEmptyDir(repo)
}

// cleanupPackageImages removes package image files not listed in keepImages.
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

		logger.Info("delete package image", slog.String("path", path))
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

// download fetches the image of version from the registry and stores it as an erofs image. If that
// image already exists and passes verification, download is skipped.
func (b *Backend) download(ctx context.Context, repo registry.Remote, packageDir, name, version string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Download")
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

	imagePath := packageImagePath(packageDir, version)
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create package dir: %w", err)
	}

	rootHash, err := b.registry.GetImageRootHash(ctx, repo, name, registry.PullTag(version))
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("get image root hash: %w", err)
	}

	// skip download if image exists and passes integrity check
	logger.Debug("verify package image")
	if err = b.verifyImage(ctx, imagePath, rootHash); err == nil {
		logger.Debug("package image verified")

		return nil
	}

	// verification failed - fetch fresh image from registry
	logger.Warn("verify package image failed", log.Err(err))

	if err = cleanupTempImageFiles(packageDir, version); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("clean up temporary images: %w", err)
	}

	tempImagePath, err := createTempImagePath(packageDir, version)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create temporary image: %w", err)
	}

	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.Remove(tempImagePath)
			_ = os.Remove(verityPath(tempImagePath))
		}
	}()

	img, err := b.registry.GetImageReader(ctx, repo, name, registry.PullTag(version))
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("get image reader: %w", err)
	}
	defer img.Close()

	logger.Debug("create erofs image by package image", slog.String("path", tempImagePath))
	if err = verity.CreateImageByTar(ctx, img, tempImagePath); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create erofs image: %w", err)
	}

	if err = b.verifyImage(ctx, tempImagePath, rootHash); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("verify erofs image: %w", err)
	}

	_ = os.Remove(verityPath(tempImagePath))

	if err = os.Rename(tempImagePath, imagePath); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("store erofs image: %w", err)
	}

	cleanupTemp = false

	return nil
}

// packageImagePath returns the final erofs image path for a package version.
func packageImagePath(packageDir, version string) string {
	return filepath.Join(packageDir, fmt.Sprintf("%s.erofs", version))
}

// verityPath returns the dm-verity hash tree path for an erofs image path.
func verityPath(imagePath string) string {
	return fmt.Sprintf("%s.verity", imagePath)
}

// cleanupTempImageFiles removes stale temporary erofs images for a package version.
func cleanupTempImageFiles(packageDir, version string) error {
	entries, err := os.ReadDir(packageDir)
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

		if err = os.Remove(filepath.Join(packageDir, entry.Name())); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	return nil
}

// createTempImagePath reserves and returns a same-directory temporary erofs image path.
func createTempImagePath(packageDir, version string) (string, error) {
	file, err := os.CreateTemp(packageDir, tempImagePrefix(version))
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
func (b *Backend) mount(ctx context.Context, packageDir, deployedDir, name, version string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Mount")
	defer span.End()

	span.SetAttributes(attribute.String("package_dir", packageDir))
	span.SetAttributes(attribute.String("deployed_dir", deployedDir))
	span.SetAttributes(attribute.String("instance", name))
	span.SetAttributes(attribute.String("version", version))

	logger := b.logger.With(
		slog.String("package_dir", packageDir),
		slog.String("deployed_dir", deployedDir),
		slog.String("name", name),
		slog.String("version", version))

	logger.Debug("attach package")

	select {
	case <-ctx.Done():
		span.SetStatus(codes.Error, "context canceled")
		return ctx.Err()
	default:
	}

	// <packageDir>/<version>.erofs
	imagePath := packageImagePath(packageDir, version)

	// Compute the new image hash before unmounting the currently deployed package.
	logger.Debug("compute erofs image hash", slog.String("path", imagePath))
	rootHash, err := verity.CreateImageHash(ctx, imagePath)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("compute image hash: %w", err)
	}

	// cleanup any existing mount before deploying new version
	logger.Debug("unmount old erofs image", slog.String("path", deployedDir))
	if err = verity.Unmount(ctx, deployedDir); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("unmount previous image: %w", err)
	}

	logger.Debug("close old device mapper")
	if err = verity.CloseMapper(ctx, name); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("close previous device mapper: %w", err)
	}

	// setup dm-verity device mapper with root hash for runtime integrity checks
	logger.Debug("create device mapper", slog.String("path", deployedDir))
	if err = verity.CreateMapper(ctx, name, imagePath, rootHash); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create device mapper: %w", err)
	}

	logger.Debug("mount erofs image mapper", slog.String("path", deployedDir))
	if err = verity.Mount(ctx, name, deployedDir); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("mount image: %w", err)
	}

	return nil
}

// Detach unmounts the erofs image and closes the dm-verity mapper. The stored image stays: only
// Cleanup purges files, since every version of the package, a Candidate's included, shares its directory.
func (b *Backend) Detach(ctx context.Context, instance string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Detach")
	defer span.End()

	span.SetAttributes(attribute.String("instance", instance))

	deployedDir := disk.AttachPath(b.workingDir, instance)
	logger := b.logger.With(slog.String("name", instance))

	logger.Debug("detach package")

	// Lstat, so a dangling symlink left by the symlink backend is still undeployed
	if _, err := os.Lstat(deployedDir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("check mount path '%s': %w", deployedDir, err)
	}

	// mounts should not be executed simultaneously
	b.mu.Lock()
	defer b.mu.Unlock()

	logger.Debug("unmount erofs image", slog.String("path", deployedDir))
	if err := verity.Unmount(ctx, deployedDir); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("unmount erofs image '%s': %w", deployedDir, err)
	}

	logger.Debug("close device mapper")
	if err := verity.CloseMapper(ctx, instance); err != nil {
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
func (b *Backend) verifyImage(ctx context.Context, imagePath, rootHash string) error {
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
