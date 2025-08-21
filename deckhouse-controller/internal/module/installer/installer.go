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
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/module/downloader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/tools/erofs"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
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
	// /deckhouse/downloaded/modules
	mount string

	downloader *downloader.Downloader

	logger *log.Logger
}

func New(clusterUUID string, dc dependency.Container, logger *log.Logger) *Installer {
	downloaded := d8env.GetDownloadedModulesDir()

	return &Installer{
		downloaded: downloaded,
		mount:      filepath.Join(downloaded, "modules"),
		downloader: downloader.New(clusterUUID, dc, logger),
		logger:     logger.Named("module-installer"),
	}
}

// GetInstalled gets all installed modules from downloaded dir
func (i *Installer) GetInstalled() (map[string]struct{}, error) {
	entries, err := os.ReadDir(i.downloaded)
	if err != nil {
		return nil, fmt.Errorf("read downloaded dir: %w", err)
	}

	installed := make(map[string]struct{})
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// skip enabled dir
		if entry.Name() == "modules" {
			continue
		}

		installed[entry.Name()] = struct{}{}
	}

	return installed, nil
}

// Download downloads module to tmp and returns path to it
func (i *Installer) Download(ctx context.Context, ms *v1alpha1.ModuleSource, module, version string) (string, error) {
	return i.downloader.Download(ctx, ms, module, version)
}

// Install creates a module image and enables the module(mount the image)
func (i *Installer) Install(ctx context.Context, module, version, modulePath string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Install")
	defer span.End()

	span.SetAttributes(attribute.String("module", module))
	span.SetAttributes(attribute.String("version", version))

	logger := i.logger.With(slog.String("name", module), slog.String("version", version))

	logger.Debug("install module")

	// /deckhouse/downloaded/modules/<module>
	mountPoint := filepath.Join(i.mount, module)

	logger.Debug("unmount old erofs image", slog.String("mount", mountPoint))
	if err := erofs.Unmount(ctx, mountPoint); err != nil {
		return fmt.Errorf("unmount the '%s' mount path: %w", mountPoint, err)
	}

	logger.Debug("close old device mapper")
	if err := erofs.CloseMapper(ctx, module); err != nil {
		return fmt.Errorf("close module mapper: %w", err)
	}

	// <version>.erofs
	image := fmt.Sprintf("%s.erofs", version)
	// /deckhouse/downloaded/<module>/<version>.erofs
	imagePath := filepath.Join(i.downloaded, module, image)

	logger.Debug("create erofs image", slog.String("path", imagePath))
	if err := erofs.CreateImage(ctx, modulePath, imagePath); err != nil {
		return fmt.Errorf("create image from the '%s' path: %w", modulePath, err)
	}

	logger.Debug("compute erofs image hash", slog.String("path", imagePath))
	if _, err := erofs.CreateImageHash(ctx, imagePath); err != nil {
		return fmt.Errorf("create image hash from the '%s' path: %w", imagePath, err)
	}

	// mounts should not be executed simultaneously
	i.mtx.Lock()
	defer i.mtx.Unlock()

	logger.Debug("mount erofs image", slog.String("path", imagePath))
	if err := erofs.Mount(ctx, imagePath, mountPoint); err != nil {
		return fmt.Errorf("mount image: %w", err)
	}

	logger.Debug("module installed")

	return nil
}

// Uninstall disables(umount the image) and deletes rgw module(delete all images)
func (i *Installer) Uninstall(ctx context.Context, module string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Uninstall")
	defer span.End()

	span.SetAttributes(attribute.String("module", module))

	logger := i.logger.With(slog.String("name", module))

	logger.Debug("uninstall module")

	shouldClear := false

	// clear module dir
	defer func() {
		if shouldClear {
			// /deckhouse/downloaded/<module>
			imagesPath := filepath.Join(i.downloaded, module)

			logger.Info("delete module dir", slog.String("path", imagesPath))
			if err := os.RemoveAll(imagesPath); err != nil {
				logger.Warn("failed to remove downloaded images", slog.String("path", imagesPath))
			}
		}

		logger.Debug("module uninstalled")
	}()

	// /deckhouse/downloaded/modules/<module>
	mountPath := filepath.Join(i.mount, module)
	if _, err := os.Stat(mountPath); err != nil {
		if os.IsNotExist(err) {
			shouldClear = true
			return nil
		}

		return fmt.Errorf("check the '%s' mount path: %w", mountPath, err)
	}

	// mounts should not be executed simultaneously
	i.mtx.Lock()
	defer i.mtx.Unlock()

	logger.Debug("unmount erofs image", slog.String("path", mountPath))
	if err := erofs.Unmount(ctx, mountPath); err != nil {
		return fmt.Errorf("unmount the '%s' path: %w", mountPath, err)
	}

	logger.Debug("close device mapper")
	if err := erofs.CloseMapper(ctx, module); err != nil {
		return fmt.Errorf("close device mapper: %w", err)
	}

	shouldClear = true

	return nil
}

// Restore tries to restore the module on fs:
// 1. If the image does not exist - download the module and create it
// 2. If the image is not mounted - mount it
func (i *Installer) Restore(ctx context.Context, ms *v1alpha1.ModuleSource, module, version string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Restore")
	defer span.End()

	span.SetAttributes(attribute.String("module", module))
	span.SetAttributes(attribute.String("version", version))
	span.SetAttributes(attribute.String("source", ms.GetName()))

	logger := i.logger.With(slog.String("name", module), slog.String("version", version))

	logger.Debug("restore module")

	// migration
	// TODO(ipaqsa): delete after 1.74
	symlink, err := utils.GetModuleSymlink(i.mount, module)
	if err != nil {
		return fmt.Errorf("get module symlink: %w", err)
	}

	// delete old symlink
	if len(symlink) > 1 {
		logger.Debug("delete module symlink", slog.String("path", symlink))
		os.RemoveAll(symlink)
	}

	// /deckhouse/downloaded/modules/<module>
	mountPoint := filepath.Join(i.mount, module)

	logger.Debug("unmount old erofs image", slog.String("path", mountPoint))
	if err = erofs.Unmount(ctx, mountPoint); err != nil {
		return fmt.Errorf("unmount the '%s' mount path: %w", mountPoint, err)
	}

	logger.Debug("close old device mapper")
	if err = erofs.CloseMapper(ctx, module); err != nil {
		return fmt.Errorf("close module mapper: %w", err)
	}

	// /deckhouse/downloaded/<module>
	modulePath := filepath.Join(i.downloaded, module)
	if _, err = os.Stat(modulePath); os.IsNotExist(err) {
		if err = os.MkdirAll(modulePath, 0755); err != nil {
			return fmt.Errorf("create the '%s' module path: %w", modulePath, err)
		}
	}

	// <version>.erofs
	image := fmt.Sprintf("%s.erofs", version)
	// /deckhouse/downloaded/<module>/<version>.erofs
	imagePath := filepath.Join(modulePath, image)

	logger.Debug("verify erofs image", slog.String("path", imagePath))

	// check if the image exists
	err = i.verifyImage(ctx, imagePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("verify image: %w", err)
	}

	// if the image does not exist - download module and create erofs image
	if os.IsNotExist(err) {
		logger.Debug("download module image", slog.String("path", imagePath))
		err = i.downloader.Extract(ctx, ms, module, version, func(ctx context.Context, rc io.ReadCloser) error {
			return erofs.CreateImageByTar(ctx, rc, imagePath)
		})
		if err != nil {
			return fmt.Errorf("extract module image to erofs: %w", err)
		}
	}

	logger.Debug("verify erofs image hash", slog.String("path", imagePath))
	// err = i.verifyImageHash(ctx, imagePath)
	// if err != nil && !os.IsNotExist(err) {
	// 	return fmt.Errorf("verify image hash: %w", err)
	// }

	// if the image hash does not exist - create it
	// if os.IsNotExist(err) {
	logger.Debug("compute erofs image hash", slog.String("path", imagePath))
	hash, err := erofs.CreateImageHash(ctx, imagePath)
	if err != nil {
		return fmt.Errorf("create image hash: %w", err)
	}
	// }

	logger.Debug("create device mapper", slog.String("path", imagePath))
	if err = erofs.CreateMapper(ctx, imagePath, hash); err != nil {
		return fmt.Errorf("create device mapper: %w", err)
	}

	logger.Debug("mount erofs image", slog.String("path", imagePath))
	if err = erofs.Mount(ctx, module, mountPoint); err != nil {
		return fmt.Errorf("mount erofs image: %w", err)
	}

	logger.Debug("module restored")

	return nil
}

// verifyImage checks if module erofs image exists at path /deckhouse/downloaded/<module>/<version>.erofs
func (i *Installer) verifyImage(_ context.Context, imagePath string) error {
	// /deckhouse/downloaded/<module>/<version>.erofs
	if _, err := os.Stat(imagePath); err != nil {
		return err
	}

	return nil
}

func (i *Installer) verifyImageHash(_ context.Context, imagePath string) error {
	// /deckhouse/downloaded/<module>/<version>.erofs.verity
	hashPath := fmt.Sprintf("%s.verity", imagePath)
	if _, err := os.Stat(hashPath); err != nil {
		return err
	}

	return nil
}
