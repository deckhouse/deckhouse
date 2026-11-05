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
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/tools/verity"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
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

	metricStorage metricsstorage.Storage

	// mountMapper mounts the device mapper of a module image, tests replace it
	mountMapper func(ctx context.Context, name, mountPath string) error

	logger *log.Logger
}

func NewInstaller(registry *registry.Service, metricStorage metricsstorage.Storage, logger *log.Logger) *Installer {
	downloaded := app.DownloadedModulesDir()

	return &Installer{
		downloaded:    downloaded,
		mount:         filepath.Join(downloaded, "modules"),
		registry:      registry,
		metricStorage: metricStorage,
		mountMapper:   verity.Mount,
		logger:        logger.Named("erofs-installer"),
	}
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
	err := buildImage(modulePath, version, func(path string) error {
		return verity.CreateImage(ctx, tempModulePath, path)
	})
	if err != nil {
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
	if err = i.mountMapper(ctx, module, mountPoint); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("mount erofs image: %w", err)
	}

	logger.Debug("module installed")

	return nil
}

// Stage creates an erofs module image on the filesystem WITHOUT mounting it
// (no device mapper, no mount). The image ends up at
// /deckhouse/downloaded/<module>/<version>.erofs but the module is not exposed to
// the operator, so an embedded copy of the same name keeps serving the module.
// The image is mounted later (by Install/Restore) once the embedded copy is gone.
func (i *Installer) Stage(ctx context.Context, module, version, tempModulePath string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Stage")
	defer span.End()

	span.SetAttributes(attribute.String("module", module))
	span.SetAttributes(attribute.String("version", version))
	span.SetAttributes(attribute.String("path", tempModulePath))

	logger := i.logger.With(slog.String("name", module), slog.String("version", version))

	logger.Debug("stage module without mount")

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
	err := buildImage(modulePath, version, func(path string) error {
		return verity.CreateImage(ctx, tempModulePath, path)
	})
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create image from the temp path '%s': %w", tempModulePath, err)
	}

	logger.Debug("module staged")

	return nil
}

// StageFromRegistry ensures the erofs module image is present on the filesystem
// WITHOUT mounting it (no device mapper, no mount). Mirrors Restore but skips
// activation.
func (i *Installer) StageFromRegistry(ctx context.Context, ms *v1alpha1.ModuleSource, module, version string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "StageFromRegistry")
	defer span.End()

	span.SetAttributes(attribute.String("module", module))
	span.SetAttributes(attribute.String("version", version))
	span.SetAttributes(attribute.String("source", ms.GetName()))

	logger := i.logger.With(slog.String("name", module), slog.String("version", version))
	logger.Debug("stage module from registry without mount")

	i.clearDownloadInterruptedMetric(module)

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

	// image already present on the filesystem - nothing to do
	if err := i.verifyModule(ctx, module, version, ""); err == nil {
		logger.Debug("erofs image already present, skip staging", slog.String("path", imagePath))
		return nil
	}

	if err := i.downloadImage(ctx, logger, ms, module, version, modulePath); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	logger.Debug("module staged")

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

	i.clearDownloadInterruptedMetric(module)

	// /deckhouse/downloaded/modules/<module>
	mountPoint := filepath.Join(i.mount, module)
	if err := unmountImage(ctx, logger, module, mountPoint); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return err
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

	rootHash, err := i.registry.GetImageRootHash(ctx, registry.BuildRemote(ms), module, version)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("get image root hash: %w", err)
	}

	mounted, err := i.mountExistingImage(ctx, logger, module, version, rootHash, imagePath, mountPoint)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	if mounted {
		return nil
	}

	if err = i.downloadImage(ctx, logger, ms, module, version, modulePath); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	if err = i.mountImage(ctx, logger, module, imagePath, mountPoint); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	logger.Debug("module restored")

	return nil
}

// RestoreFromRegistry downloads the module image and mounts it. The image on the filesystem
// is replaced only by a complete download and is mounted when the download fails. The result
// reports whether the download completed.
func (i *Installer) RestoreFromRegistry(ctx context.Context, ms *v1alpha1.ModuleSource, module, version string) (bool, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "RestoreFromRegistry")
	defer span.End()

	span.SetAttributes(attribute.String("module", module))
	span.SetAttributes(attribute.String("version", version))
	span.SetAttributes(attribute.String("source", ms.GetName()))

	logger := i.logger.With(slog.String("name", module), slog.String("version", version))
	logger.Debug("restore module from registry")

	i.clearDownloadInterruptedMetric(module)

	// /deckhouse/downloaded/modules/<module>
	mountPoint := filepath.Join(i.mount, module)
	if err := unmountImage(ctx, logger, module, mountPoint); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return false, err
	}

	// /deckhouse/downloaded/<module>
	modulePath := filepath.Join(i.downloaded, module)
	if err := os.MkdirAll(modulePath, 0755); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return false, fmt.Errorf("create module dir '%s': %w", modulePath, err)
	}

	// <version>.erofs
	image := fmt.Sprintf("%s.erofs", version)
	// /deckhouse/downloaded/<module>/<version>.erofs
	imagePath := filepath.Join(modulePath, image)

	if downloadErr := i.downloadImage(ctx, logger, ms, module, version, modulePath); downloadErr != nil {
		err := i.mountOldImage(ctx, logger, module, version, imagePath, mountPoint, downloadErr)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
		}

		return false, err
	}

	if err := i.mountImage(ctx, logger, module, imagePath, mountPoint); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return false, err
	}

	logger.Debug("module restored from registry")

	return true, nil
}

// mountOldImage mounts the image on the filesystem that the failed download didn't replace.
// It returns the download error when there is no image to mount.
func (i *Installer) mountOldImage(ctx context.Context, logger *log.Logger, module, version, imagePath, mountPoint string, downloadErr error) error {
	logger.Warn("download module image failed, mount the image on the filesystem", log.Err(downloadErr))

	// the image is an earlier build, the root hash of the version in the registry doesn't describe it
	mounted, err := i.mountExistingImage(ctx, logger, module, version, "", imagePath, mountPoint)
	if err != nil {
		return err
	}

	if !mounted {
		return downloadErr
	}

	return nil
}

// unmountImage unmounts the module image and closes its device mapper
func unmountImage(ctx context.Context, logger *log.Logger, module, mountPoint string) error {
	logger.Debug("unmount old erofs image", slog.String("path", mountPoint))
	if err := verity.Unmount(ctx, mountPoint); err != nil {
		return fmt.Errorf("unmount old erofs image '%s': %w", mountPoint, err)
	}

	logger.Debug("close old device mapper")
	if err := verity.CloseMapper(ctx, module); err != nil {
		return fmt.Errorf("close module mapper: %w", err)
	}

	return nil
}

// downloadImage downloads the module image and builds the erofs image of the version from it.
// The image on the filesystem is replaced only once the new one is complete.
func (i *Installer) downloadImage(ctx context.Context, logger *log.Logger, ms *v1alpha1.ModuleSource, module, version, modulePath string) error {
	img, err := i.registry.GetImageReader(ctx, registry.BuildRemote(ms), module, version)
	if err != nil {
		i.setDownloadInterruptedMetric(module, version, err)
		return fmt.Errorf("download module image: %w", err)
	}
	defer img.Close()

	logger.Debug("create erofs image from module image", slog.String("path", modulePath))
	err = buildImage(modulePath, version, func(path string) error {
		return verity.CreateImageByTar(ctx, img, path)
	})
	if err != nil {
		return fmt.Errorf("extract module image to erofs: %w", err)
	}

	return nil
}

// mountExistingImage mounts the image already on the filesystem. It reports false when the image
// is missing or broken.
func (i *Installer) mountExistingImage(ctx context.Context, logger *log.Logger, module, version, rootHash, imagePath, mountPoint string) (bool, error) {
	logger.Debug("verify module")
	if err := i.verifyModule(ctx, module, version, rootHash); err != nil {
		logger.Warn("verify module failed", log.Err(err))
		return false, nil
	}

	logger.Debug("module verified")

	err := i.mountImage(ctx, logger, module, imagePath, mountPoint)
	if err == nil {
		return true, nil
	}

	// the image can be left broken on the filesystem, e.g. by a build interrupted before builds
	// went through a temporary file, so a failed mount reports a broken image instead of failing
	logger.Warn("mount module image failed", log.Err(err))

	if err = verity.CloseMapper(ctx, module); err != nil {
		return false, fmt.Errorf("close module mapper: %w", err)
	}

	return false, nil
}

// mountImage computes the hash tree of the image, creates the device mapper over the image and mounts it
func (i *Installer) mountImage(ctx context.Context, logger *log.Logger, module, imagePath, mountPoint string) error {
	// TODO(ipaqsa): temp solution before all modules have hash
	logger.Debug("compute erofs image hash", slog.String("path", imagePath))
	hash, err := verity.CreateImageHash(ctx, imagePath)
	if err != nil {
		return fmt.Errorf("create image hash: %w", err)
	}

	logger.Debug("create device mapper", slog.String("path", imagePath))
	if err = verity.CreateMapper(ctx, module, imagePath, hash); err != nil {
		return fmt.Errorf("create device mapper: %w", err)
	}

	logger.Debug("mount erofs image mapper", slog.String("path", imagePath))
	if err = i.mountMapper(ctx, module, mountPoint); err != nil {
		return fmt.Errorf("mount erofs image: %w", err)
	}

	return nil
}

// buildImage builds the image of the module version through a temporary file in the module dir
// and renames it onto <version>.erofs once complete. An interrupted build leaves only a temporary
// file, which the next build of the version removes, never a partial image that a restore takes
// for a complete one.
func buildImage(modulePath, version string, build func(path string) error) error {
	if err := removeTempImages(modulePath, version); err != nil {
		return fmt.Errorf("remove temporary images: %w", err)
	}

	file, err := os.CreateTemp(modulePath, tempImagePrefix(version))
	if err != nil {
		return fmt.Errorf("create temporary image: %w", err)
	}

	// only the unique name is needed, mkfs.erofs creates the file itself
	tempPath := file.Name()
	_ = file.Close()
	if err = os.Remove(tempPath); err != nil {
		return fmt.Errorf("reserve temporary image: %w", err)
	}

	if err = build(tempPath); err != nil {
		_ = os.Remove(tempPath)
		return err
	}

	// /deckhouse/downloaded/<module>/<version>.erofs
	imagePath := filepath.Join(modulePath, fmt.Sprintf("%s.erofs", version))
	if err = os.Rename(tempPath, imagePath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("store image '%s': %w", imagePath, err)
	}

	return nil
}

// removeTempImages removes the temporary images of the version left by interrupted builds
func removeTempImages(modulePath, version string) error {
	entries, err := os.ReadDir(modulePath)
	if err != nil {
		return err
	}

	prefix := tempImagePrefix(version)
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}

		if err = os.Remove(filepath.Join(modulePath, entry.Name())); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	return nil
}

// tempImagePrefix returns the name prefix of the temporary images of the version
func tempImagePrefix(version string) string {
	return fmt.Sprintf(".%s.erofs.tmp-", version)
}

// clearDownloadInterruptedMetric expires the metric behind the D8DeckhouseModuleImageDownloadInterrupted
// alert for the module. It runs when an attempt to put the module image on the filesystem starts,
// so the metric reflects the last attempt however it ends.
func (i *Installer) clearDownloadInterruptedMetric(module string) {
	i.metricStorage.Grouped().ExpireGroupMetrics(fmt.Sprintf(metrics.ModuleImageDownloadMetricGroupTemplate, module))
}

// setDownloadInterruptedMetric sets the metric behind the D8DeckhouseModuleImageDownloadInterrupted
// alert when the download of the module image layers was interrupted. The image is built from
// the downloaded copy, so a failed build isn't an interrupted download.
func (i *Installer) setDownloadInterruptedMetric(module, version string, err error) {
	if !errors.Is(err, cr.ErrImageDownload) {
		return
	}

	i.metricStorage.Grouped().GaugeSet(fmt.Sprintf(metrics.ModuleImageDownloadMetricGroupTemplate, module), metrics.D8ModuleImageDownloadInterrupted, 1, map[string]string{
		metrics.LabelModule:  module,
		metrics.LabelVersion: version,
	})
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
