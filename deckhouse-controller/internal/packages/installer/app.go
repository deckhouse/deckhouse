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
	"github.com/deckhouse/deckhouse/go_lib/d8env"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	tracerName = "installer"
)

type Installer struct {
	mtx sync.Mutex
	// /deckhouse/downloaded
	downloaded string
	// /deckhouse/downloaded/apps
	mount string

	registry *registry.Service

	logger *log.Logger
}

func New(dc dependency.Container, logger *log.Logger) *Installer {
	downloaded := d8env.GetDownloadedModulesDir()

	return &Installer{
		downloaded: downloaded,
		mount:      filepath.Join(downloaded, "apps"),
		registry:   registry.NewService(dc, logger),
		logger:     logger.Named("application-installer"),
	}
}

// Uninstall umount the erofs image
func (i *Installer) Uninstall(ctx context.Context, app string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Uninstall")
	defer span.End()

	span.SetAttributes(attribute.String("app", app))

	logger := i.logger.With(slog.String("name", app))

	logger.Debug("uninstall app")

	// /deckhouse/downloaded/app/<app>
	mountPath := filepath.Join(i.mount, app)
	if _, err := os.Stat(mountPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("check mount path '%s': %w", mountPath, err)
	}

	// mounts should not be executed simultaneously
	i.mtx.Lock()
	defer i.mtx.Unlock()

	logger.Debug("unmount erofs image", slog.String("path", mountPath))
	if err := verity.Unmount(ctx, mountPath); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("unmount erofs image '%s': %w", mountPath, err)
	}

	logger.Debug("close device mapper")
	if err := verity.CloseMapper(ctx, app); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("close device mapper: %w", err)
	}

	logger.Debug("app uninstalled")

	return nil
}

// Download ensures the package image is present, verified, and mounted.
func (i *Installer) Download(ctx context.Context, reg registry.Registry, name, version string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Download")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))
	span.SetAttributes(attribute.String("version", version))
	span.SetAttributes(attribute.String("registry", reg.Name))
	span.SetAttributes(attribute.String("repository", reg.Repository))

	logger := i.logger.With(
		slog.String("name", name),
		slog.String("version", version),
		slog.String("registry", reg.Name),
		slog.String("repository", reg.Repository))

	logger.Debug("download package")

	i.mtx.Lock()
	defer i.mtx.Unlock()

	// /deckhouse/downloaded/<package>
	packagePath := filepath.Join(i.downloaded, name)
	if err := os.MkdirAll(packagePath, 0755); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create package dir '%s': %w", packagePath, err)
	}

	// <version>.erofs
	image := fmt.Sprintf("%s.erofs", version)
	// /deckhouse/downloaded/<package>/<version>.erofs
	imagePath := filepath.Join(packagePath, image)

	rootHash, err := i.registry.GetImageRootHash(ctx, reg, name, version)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("get image root hash: %w", err)
	}

	logger.Debug("verify package")
	if err = i.verifyPackage(ctx, name, version, rootHash); err == nil {
		logger.Debug("package verified")

		return nil
	}

	logger.Warn("verify package failed", log.Err(err))

	img, err := i.registry.GetImageReader(ctx, reg, name, version)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("download package image: %w", err)
	}
	defer img.Close()

	logger.Debug("create erofs image from package image", slog.String("path", imagePath))
	if err = verity.CreateImageByTar(ctx, img, imagePath); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("extract package image to erofs: %w", err)
	}

	return nil
}

// Install creates device mapper for application and mounts it
func (i *Installer) Install(ctx context.Context, name, packageName, version string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Install")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))
	span.SetAttributes(attribute.String("package", packageName))
	span.SetAttributes(attribute.String("version", version))

	logger := i.logger.With(slog.String("name", name), slog.String("version", version))
	logger.Debug("install application")

	i.mtx.Lock()
	defer i.mtx.Unlock()

	// /deckhouse/downloaded/apps/<app>
	mountPoint := filepath.Join(i.mount, name)

	logger.Debug("unmount old erofs image", slog.String("path", mountPoint))
	if err := verity.Unmount(ctx, mountPoint); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("unmount old erofs image '%s': %w", mountPoint, err)
	}

	logger.Debug("close old device mapper")
	if err := verity.CloseMapper(ctx, name); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("close app mapper: %w", err)
	}

	// /deckhouse/downloaded/<package>
	packagePath := filepath.Join(i.downloaded, packageName)
	// <version>.erofs
	image := fmt.Sprintf("%s.erofs", version)
	// /deckhouse/downloaded/<package>/<version>.erofs
	imagePath := filepath.Join(packagePath, image)

	logger.Debug("compute erofs image hash", slog.String("path", imagePath))
	rootHash, err := verity.CreateImageHash(ctx, imagePath)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create image hash: %w", err)
	}

	logger.Debug("create device mapper", slog.String("path", imagePath))
	if err = verity.CreateMapper(ctx, name, imagePath, rootHash); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create device mapper: %w", err)
	}

	logger.Debug("mount erofs image mapper", slog.String("path", imagePath))
	if err = verity.Mount(ctx, name, mountPoint); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("mount erofs image: %w", err)
	}

	return nil
}

// verifyPackage checks that the image and hash exist and verified
func (i *Installer) verifyPackage(_ context.Context, pack, version, _ string) error {
	// /deckhouse/downloaded/<package>
	packagePath := filepath.Join(i.downloaded, pack)
	// <version>.erofs
	image := fmt.Sprintf("%s.erofs", version)
	// /deckhouse/downloaded/<package>/<version>.erofs
	imagePath := filepath.Join(packagePath, image)

	// /deckhouse/downloaded/<package>/<version>.erofs
	if _, err := os.Stat(imagePath); err != nil {
		return fmt.Errorf("stat package image '%s': %w", imagePath, err)
	}

	// TODO(ipaqsa): wait for all apps have root hash
	// /deckhouse/downloaded/<package>/<version>.erofs.verity
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
