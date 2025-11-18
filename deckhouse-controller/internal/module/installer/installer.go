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
	"regexp"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/tools/verity"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
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

	registry *registry.Service

	logger *log.Logger
}

func New(dc dependency.Container, logger *log.Logger) *Installer {
	downloaded := d8env.GetDownloadedModulesDir()

	return &Installer{
		downloaded: downloaded,
		mount:      filepath.Join(downloaded, "modules"),
		registry:   registry.NewService(dc, logger),
		logger:     logger.Named("module-installer"),
	}
}

func (i *Installer) SetClusterUUID(id string) {
	i.registry.SetClusterUUID(id)
}

// GetDownloaded gets all downloaded modules from downloaded dir
func (i *Installer) GetDownloaded() (map[string]struct{}, error) {
	entries, err := os.ReadDir(i.downloaded)
	if err != nil {
		return nil, fmt.Errorf("read downloaded dir: %w", err)
	}

	downloaded := make(map[string]struct{})
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// skip enabled dir
		if entry.Name() == "modules" {
			continue
		}

		downloaded[entry.Name()] = struct{}{}
	}

	return downloaded, nil
}

func (i *Installer) GetImageDigest(ctx context.Context, source *v1alpha1.ModuleSource, moduleName, version string) (string, error) {
	return i.registry.GetImageDigest(ctx, registry.BuildRegistryBySource(source), moduleName, version)
}

func (i *Installer) Download(ctx context.Context, source *v1alpha1.ModuleSource, moduleName, version string) (string, error) {
	return i.registry.Download(ctx, registry.BuildRegistryBySource(source), moduleName, version)
}

// Install creates an erofs module image and enables the module(mount the image)
func (i *Installer) Install(ctx context.Context, module, version, tempModulePath string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Install")
	defer span.End()

	span.SetAttributes(attribute.String("module", module))
	span.SetAttributes(attribute.String("version", version))
	span.SetAttributes(attribute.String("path", tempModulePath))

	logger := i.logger.With(slog.String("name", module), slog.String("version", version))

	logger.Debug("install module")

	// mounts should not be executed simultaneously
	i.mtx.Lock()
	defer i.mtx.Unlock()

	// /deckhouse/downloaded/modules/<module>
	mountPoint := filepath.Join(i.mount, module)

	logger.Debug("unmount old erofs image", slog.String("mount", mountPoint))
	if err := verity.Unmount(ctx, mountPoint); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("unmount erofs image '%s': %w", mountPoint, err)
	}

	logger.Debug("close old device mapper")
	if err := verity.CloseMapper(ctx, module); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("close module mapper: %w", err)
	}

	// /deckhouse/downloaded/<module>
	modulePath := filepath.Join(i.downloaded, module)
	if err := os.MkdirAll(modulePath, 0755); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create module dir '%s': %w", modulePath, err)
	}

	// <version>.erofs
	image := fmt.Sprintf("%s.erofs", version)
	// /deckhouse/downloaded/<module>/<version>.erofs
	imagePath := filepath.Join(modulePath, image)

	logger.Debug("create erofs image", slog.String("path", imagePath))
	if err := verity.CreateImage(ctx, tempModulePath, imagePath); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create image from the temp path '%s': %w", tempModulePath, err)
	}

	logger.Debug("compute erofs image hash", slog.String("path", imagePath))
	hash, err := verity.CreateImageHash(ctx, imagePath)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create image hash from the path '%s': %w", imagePath, err)
	}

	logger.Debug("create device mapper")
	if err = verity.CreateMapper(ctx, module, imagePath, hash); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create device mapper: %w", err)
	}

	logger.Debug("mount erofs image mapper")
	if err = verity.Mount(ctx, module, mountPoint); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("mount erofs image: %w", err)
	}

	logger.Debug("module installed")

	return nil
}

// Uninstall disables(umount the erofs image) and deletes the module(delete all images)
func (i *Installer) Uninstall(ctx context.Context, module string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Uninstall")
	defer span.End()

	span.SetAttributes(attribute.String("module", module))

	logger := i.logger.With(slog.String("name", module))

	logger.Debug("uninstall module")

	// clear module dir
	defer func() {
		// /deckhouse/downloaded/<module>
		imagesPath := filepath.Join(i.downloaded, module)

		logger.Info("delete module dir", slog.String("path", imagesPath))
		if err := os.RemoveAll(imagesPath); err != nil {
			logger.Warn("failed to remove downloaded images", slog.String("path", imagesPath))
		}

		logger.Debug("module uninstalled")
	}()

	// /deckhouse/downloaded/modules/<module>
	mountPath := filepath.Join(i.mount, module)
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
	if err := verity.CloseMapper(ctx, module); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("close device mapper: %w", err)
	}

	return nil
}

// Restore ensures the module image is present, verified, and mounted.
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
	symlink, err := i.getModuleSymlink(module)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("get module symlink: %w", err)
	}
	if len(symlink) > 0 {
		logger.Debug("delete module symlink", slog.String("path", symlink))
		os.RemoveAll(symlink)
	}

	// /deckhouse/downloaded/modules/<module>
	mountPoint := filepath.Join(i.mount, module)

	logger.Debug("unmount old erofs image", slog.String("path", mountPoint))
	if err = verity.Unmount(ctx, mountPoint); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("unmount old erofs image '%s': %w", mountPoint, err)
	}

	logger.Debug("close old device mapper")
	if err = verity.CloseMapper(ctx, module); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("close module mapper: %w", err)
	}

	// /deckhouse/downloaded/<module>
	modulePath := filepath.Join(i.downloaded, module)
	if err = os.MkdirAll(modulePath, 0755); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create module dir '%s': %w", modulePath, err)
	}

	// <version>.erofs
	image := fmt.Sprintf("%s.erofs", version)
	// /deckhouse/downloaded/<module>/<version>.erofs
	imagePath := filepath.Join(modulePath, image)

	rootHash, err := i.registry.GetImageRootHash(ctx, registry.BuildRegistryBySource(ms), module, version)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("get image root hash: %w", err)
	}

	logger.Debug("verify module")
	if err = i.verifyModule(ctx, module, version, rootHash); err == nil {
		logger.Debug("module verified")

		// TODO(ipaqsa): temp solution before all modules have hash
		logger.Debug("compute erofs image hash", slog.String("path", imagePath))
		if rootHash, err = verity.CreateImageHash(ctx, imagePath); err != nil {
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("create image hash: %w", err)
		}

		logger.Debug("create device mapper", slog.String("path", imagePath))
		if err = verity.CreateMapper(ctx, module, imagePath, rootHash); err != nil {
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("create device mapper: %w", err)
		}

		logger.Debug("mount erofs image mapper", slog.String("path", imagePath))
		if err = verity.Mount(ctx, module, mountPoint); err != nil {
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("mount erofs image: %w", err)
		}

		return nil
	}

	logger.Warn("verify module failed", log.Err(err))

	img, err := i.registry.GetImageReader(ctx, registry.BuildRegistryBySource(ms), module, version)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("download module image: %w", err)
	}
	defer img.Close()

	logger.Debug("create erofs image from module image", slog.String("path", imagePath))
	if err = verity.CreateImageByTar(ctx, img, imagePath); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("extract module image to erofs: %w", err)
	}

	logger.Debug("compute erofs image hash", slog.String("path", imagePath))
	hash, err := verity.CreateImageHash(ctx, imagePath)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create image hash: %w", err)
	}

	logger.Debug("create device mapper")
	if err = verity.CreateMapper(ctx, module, imagePath, hash); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create device mapper: %w", err)
	}

	logger.Debug("mount erofs image mapper")
	if err = verity.Mount(ctx, module, mountPoint); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("mount erofs image: %w", err)
	}

	logger.Debug("module restored")

	return nil
}

// verifyModule checks that the image and hash exist and verified
func (i *Installer) verifyModule(_ context.Context, module, version, _ string) error {
	// /deckhouse/downloaded/<module>
	modulePath := filepath.Join(i.downloaded, module)
	// <version>.erofs
	image := fmt.Sprintf("%s.erofs", version)
	// /deckhouse/downloaded/<module>/<version>.erofs
	imagePath := filepath.Join(modulePath, image)

	// /deckhouse/downloaded/<module>/<version>.erofs
	if _, err := os.Stat(imagePath); err != nil {
		return fmt.Errorf("stat module image '%s': %w", imagePath, err)
	}

	// TODO(ipaqsa): wait for all modules have root hash
	// /deckhouse/downloaded/<module>/<version>.erofs.verity
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

// getModuleSymlink walks over the root dir to find a module symlink by regexp
func (i *Installer) getModuleSymlink(moduleName string) (string, error) {
	var symlinkPath string

	moduleRegexp := regexp.MustCompile(`^(([0-9]+)-)?(` + moduleName + `)$`)
	err := filepath.WalkDir(i.mount, func(path string, d os.DirEntry, _ error) error {
		if !moduleRegexp.MatchString(d.Name()) {
			return nil
		}

		symlinkPath = path

		return filepath.SkipDir
	})

	return symlinkPath, err
}
