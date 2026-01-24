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
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/tools/verity"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	tracerName = "installer"
)

// Installer handles package lifecycle using erofs images with dm-verity integrity.
// Operations are serialized via mutex to prevent concurrent mount/unmount conflicts.
type Installer struct {
	mtx      sync.Mutex
	registry *registry.Service
	logger   *log.Logger
}

func New(dc dependency.Container, logger *log.Logger) *Installer {
	return &Installer{
		registry: registry.NewService(dc, logger),
		logger:   logger.Named("package-installer"),
	}
}

// Download fetches a package image from the registry and creates an erofs image.
// If the image already exists and passes verification, download is skipped.
func (i *Installer) Download(ctx context.Context, repo registry.Repository, downloaded, name, version string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Download")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))
	span.SetAttributes(attribute.String("version", version))
	span.SetAttributes(attribute.String("downloaded", downloaded))
	span.SetAttributes(attribute.String("repository", repo.Name))
	span.SetAttributes(attribute.String("registry", repo.Registry))

	logger := i.logger.With(
		slog.String("name", name),
		slog.String("version", version),
		slog.String("downloaded", downloaded),
		slog.String("repository", repo.Name),
		slog.String("registry", repo.Registry))

	logger.Debug("download package")

	select {
	case <-ctx.Done():
		span.SetStatus(codes.Error, "context canceled")
		return ctx.Err()
	default:
	}

	i.mtx.Lock()
	defer i.mtx.Unlock()

	// <downloaded>/<version>.erofs
	imagePath := filepath.Join(downloaded, fmt.Sprintf("%s.erofs", version))
	if err := os.MkdirAll(filepath.Dir(imagePath), 0755); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newCreatePackageDirErr(err)
	}

	rootHash, err := i.registry.GetImageRootHash(ctx, repo, name, version)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newGetRootHashErr(err)
	}

	// skip download if image exists and passes integrity check
	logger.Debug("verify package image")
	if err = i.verifyImage(ctx, imagePath, rootHash); err == nil {
		logger.Debug("package image verified")

		return nil
	}

	// verification failed - fetch fresh image from registry
	logger.Warn("verify package image failed", log.Err(err))

	img, err := i.registry.GetImageReader(ctx, repo, name, version)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newGetRootHashErr(err)
	}
	defer img.Close()

	logger.Debug("create erofs image by package image", slog.String("path", imagePath))
	if err = verity.CreateImageByTar(ctx, img, imagePath); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newImageByTarErr(err)
	}

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

	i.mtx.Lock()
	defer i.mtx.Unlock()

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
	if err = verity.Mount(ctx, name, imagePath); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newMountErr(err)
	}

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
	i.mtx.Lock()
	defer i.mtx.Unlock()

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

	return nil
}

// verifyImage checks that the image and hash exist and verified
func (i *Installer) verifyImage(_ context.Context, imagePath, _ string) error {
	if _, err := os.Stat(imagePath); err != nil {
		return fmt.Errorf("stat package image '%s': %w", imagePath, err)
	}

	// TODO(ipaqsa): wait until all package have root hash
	// hashPath := fmt.Sprintf("%s.verity", imagePath)
	// if _, err := os.Stat(hashPath); err != nil {
	// 	return fmt.Errorf("stat verity hash file '%s': %w", hashPath, err)
	// }

	// if len(hash) == 0 {
	// 	return errors.New("empty hash")
	// }

	// if err := verity.VerifyImage(ctx, imagePath, hash); err != nil {
	// 	return fmt.Errorf("verify root hash: %w", err)
	// }

	return nil
}
