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
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/tools/verity"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	tracerName = "installer"
)

// Installer handles package lifecycle using erofs images with dm-verity integrity.
// Operations are serialized via mutex to prevent concurrent mount/unmount conflicts.
type Installer struct {
	mu         sync.Mutex
	downloaded map[string]struct{}
	deployed   map[string]struct{}
	registry   registryService
	logger     *log.Logger
}

type registryService interface {
	GetImageRootHash(ctx context.Context, cred registry.Remote, packageName, tag string) (string, error)
	GetImageReader(ctx context.Context, cred registry.Remote, packageName, tag string) (io.ReadCloser, error)
}

func NewInstaller(registry registryService, logger *log.Logger) *Installer {
	return &Installer{
		downloaded: make(map[string]struct{}),
		deployed:   make(map[string]struct{}),
		registry:   registry,
		logger:     logger.Named("erofs-installer"),
	}
}

// Download fetches a package image from the registry and creates an erofs image.
// If the image already exists and passes verification, download is skipped.
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

	i.mu.Lock()
	defer i.mu.Unlock()

	// <downloaded>/<version>.erofs
	imagePath := filepath.Join(downloaded, fmt.Sprintf("%s.erofs", version))

	rootHash, err := i.registry.GetImageRootHash(ctx, repo, name, version)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newGetRootHashErr(err)
	}

	// skip download if image exists and passes integrity check
	logger.Debug("verify package image")
	if err = i.verifyImage(ctx, imagePath, rootHash); err == nil {
		logger.Debug("package image verified")

		i.downloaded[imagePath] = struct{}{}
		return nil
	}

	// verification failed - fetch fresh image from registry
	logger.Warn("verify package image failed", log.Err(err))

	img, err := i.registry.GetImageReader(ctx, repo, name, version)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newGetImageReaderErr(err)
	}
	defer img.Close()

	logger.Debug("create erofs image by package image", slog.String("path", imagePath))
	if err = verity.CreateImageByTar(ctx, img, imagePath); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newImageByTarErr(err)
	}

	i.downloaded[imagePath] = struct{}{}
	return nil
}

// Install mounts an erofs image using dm-verity for integrity verification.
// Flow: unmount old → close mapper → compute hash → create mapper → mount new.
func (i *Installer) Install(ctx context.Context, downloaded, deployed, name, version string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Install")
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

	i.mu.Lock()
	defer i.mu.Unlock()

	// <downloaded>/<version>.erofs
	imagePath := filepath.Join(downloaded, fmt.Sprintf("%s.erofs", version))

	// cleanup any existing mount before installing new version
	logger.Debug("unmount old erofs image", slog.String("path", deployed))
	if err := verity.Unmount(ctx, deployed); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newUnmountErr(err)
	}

	logger.Debug("close old device mapper")
	if err := verity.CloseMapper(ctx, name); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newCloseDeviceMapperErr(err)
	}

	// compute dm-verity root hash for integrity verification during reads
	logger.Debug("compute erofs image hash", slog.String("path", imagePath))
	rootHash, err := verity.CreateImageHash(ctx, imagePath)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newComputeHashErr(err)
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

	i.deployed[filepath.Join(deployed, name)] = struct{}{}
	return nil
}

// Uninstall unmounts the erofs image and closes the dm-verity mapper.
// If keep=false, the downloaded image files are also deleted.
func (i *Installer) Uninstall(ctx context.Context, downloaded, deployed, name string, keep bool) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Uninstall")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))

	logger := i.logger.With(slog.String("name", name))

	logger.Debug("uninstall package")

	if _, err := os.Stat(deployed); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("check mount path '%s': %w", deployed, err)
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

	// mounts should not be executed simultaneously
	i.mu.Lock()
	defer i.mu.Unlock()

	logger.Debug("unmount erofs image", slog.String("path", deployed))
	if err := verity.Unmount(ctx, deployed); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("unmount erofs image '%s': %w", deployed, err)
	}

	logger.Debug("close device mapper")
	if err := verity.CloseMapper(ctx, name); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("close device mapper: %w", err)
	}

	logger.Debug("package uninstalled")

	// Remove from tracking maps so cleanup won't protect stale entries.
	delete(i.deployed, filepath.Join(deployed, name))
	for path := range i.downloaded {
		if strings.HasPrefix(path, downloaded+string(filepath.Separator)) {
			delete(i.downloaded, path)
		}
	}

	return nil
}

// verifyImage checks that the image and hash exist and verified
func (i *Installer) verifyImage(_ context.Context, imagePath, _ string) error {
	if _, err := os.Stat(imagePath); err != nil {
		return fmt.Errorf("stat package image '%s': %w", imagePath, err)
	}

	// TODO(ipaqsa): before implementing verify mechanism wait until all packages have root hash

	return nil
}

// Cleanup unmounts stale erofs mounts from deployed and removes stale
// .erofs/.erofs.verity files and empty directories from downloaded.
// Both parameters must be root directories (not package-level).
// Handles both module (<downloaded>/<package>/<version>.erofs) and
// application (<downloaded>/<registry>/<package>/<version>.erofs) layouts.
func (i *Installer) Cleanup(ctx context.Context, downloaded, deployed string, exclude ...string) {
	_, span := otel.Tracer(tracerName).Start(ctx, "Cleanup")
	defer span.End()

	i.cleanupDeployed(ctx, deployed)

	skip := make(map[string]struct{}, len(exclude)+1)
	skip[deployed] = struct{}{}
	for _, path := range exclude {
		skip[path] = struct{}{}
	}

	i.cleanDownloaded(downloaded, skip)
}

// cleanupDeployed unmounts erofs mounts and closes device mappers
// not tracked in i.deployed.
func (i *Installer) cleanupDeployed(ctx context.Context, deployed string) {
	logger := i.logger.With(slog.String("deployed", deployed))

	mounts, err := os.ReadDir(deployed)
	if err != nil {
		return
	}

	for _, mount := range mounts {
		mountPath := filepath.Join(deployed, mount.Name())
		if _, ok := i.deployed[mountPath]; ok {
			continue
		}

		name := mount.Name()

		logger.Info("unmount stale mount", slog.String("path", mountPath))
		if err = verity.Unmount(ctx, mountPath); err != nil {
			logger.Warn("failed to unmount", slog.String("path", mountPath), log.Err(err))
		}

		if err = verity.CloseMapper(ctx, name); err != nil {
			logger.Warn("failed to close device mapper", slog.String("name", name), log.Err(err))
		}
	}
}

// cleanDownloaded walks the downloaded tree and removes any path not on the way
// to a tracked .erofs file. Prunes stale registries, packages, version files,
// and foreign files in a single pass.
// Paths in skip are preserved (e.g. deployed dir, sibling roots).
func (i *Installer) cleanDownloaded(downloaded string, skip map[string]struct{}) {
	logger := i.logger.With(slog.String("downloaded", downloaded))

	// Build ancestor set: version prefix (without .erofs) + all parent dirs up to root.
	keep := make(map[string]struct{}, len(i.downloaded)*3)
	for path := range i.downloaded {
		prefix := strings.TrimSuffix(path, ".erofs")
		for prefix != downloaded {
			keep[prefix] = struct{}{}
			prefix = filepath.Dir(prefix)
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

		// Directories: keep if ancestor of a tracked version, remove otherwise.
		if d.IsDir() {
			if _, ok := keep[path]; ok {
				return nil
			}
			logger.Info("remove stale dir", slog.String("path", path))
			if err = os.RemoveAll(path); err != nil {
				logger.Warn("failed to remove dir", slog.String("path", path), log.Err(err))
			}
			return filepath.SkipDir
		}

		// Files: strip .erofs.verity / .erofs to get version prefix, check keep set.
		prefix := strings.TrimSuffix(path, ".verity")
		prefix = strings.TrimSuffix(prefix, ".erofs")
		if _, ok := keep[prefix]; ok {
			return nil
		}

		logger.Info("remove stale file", slog.String("path", path))
		if err = os.Remove(path); err != nil {
			logger.Warn("failed to remove file", slog.String("path", path), log.Err(err))
		}
		return nil
	})
}
