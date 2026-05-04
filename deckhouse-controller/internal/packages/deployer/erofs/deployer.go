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

const tracerName = "deployer"

// Deployer handles package lifecycle using erofs images with dm-verity integrity.
// Operations are serialized via mutex to prevent concurrent mount/unmount conflicts.
type Deployer struct {
	mu       sync.Mutex
	registry registryService
	logger   *log.Logger
}

type registryService interface {
	GetImageRootHash(ctx context.Context, cred registry.Remote, packageName, tag string) (string, error)
	GetImageReader(ctx context.Context, cred registry.Remote, packageName, tag string) (io.ReadCloser, error)
}

// NewDeployer creates a Deployer with the given registry service.
func NewDeployer(registry registryService, logger *log.Logger) *Deployer {
	return &Deployer{
		registry: registry,
		logger:   logger.Named("erofs-deployer"),
	}
}

// Deploy fetches a package image from the registry and mounts it at the deployed path.
func (d *Deployer) Deploy(ctx context.Context, repo registry.Remote, downloaded, deployed, packageName, name, version string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.download(ctx, repo, downloaded, packageName, version); err != nil {
		return err
	}

	return d.mount(ctx, downloaded, deployed, name, version)
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
func (d *Deployer) Undeploy(ctx context.Context, downloaded, deployed, name string, keep bool) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Undeploy")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))

	logger := d.logger.With(slog.String("name", name))

	logger.Debug("undeploy package")

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
	d.mu.Lock()
	defer d.mu.Unlock()

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

	logger.Debug("package undeployed")

	return nil
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
