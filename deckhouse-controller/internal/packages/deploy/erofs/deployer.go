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
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/tools/verity"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	tracerName = "deployer"
)

type Deployer struct {
	mu sync.Mutex

	downloads string
	deployed  string

	links map[string]string

	registry registryService
	logger   *log.Logger
}

type registryService interface {
	GetImageRootHash(ctx context.Context, cred registry.Remote, packageName, tag string) (string, error)
	GetImageReader(ctx context.Context, cred registry.Remote, packageName, tag string) (io.ReadCloser, error)
}

func NewDeployer(svc registryService, downloaded string, logger *log.Logger) *Deployer {
	return &Deployer{
		downloads: filepath.Join(downloaded, "downloads"),
		deployed:  filepath.Join(downloaded, "deployed"),

		links: make(map[string]string),

		registry: svc,
		logger:   logger.Named("erofs-deployer"),
	}
}

func (d *Deployer) Deploy(ctx context.Context, repo registry.Remote, deployed, name, version string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Deploy")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))
	span.SetAttributes(attribute.String("version", version))
	span.SetAttributes(attribute.String("downloaded", d.downloads))
	span.SetAttributes(attribute.String("deployed", d.deployed))
	span.SetAttributes(attribute.String("repository", repo.Name))
	span.SetAttributes(attribute.String("registry", repo.Repository))

	logger := d.logger.With(
		slog.String("name", name),
		slog.String("version", version),
		slog.String("downloaded", d.downloads),
		slog.String("deployed", d.deployed),
		slog.String("repository", repo.Name),
		slog.String("registry", repo.Repository))

	select {
	case <-ctx.Done():
		span.SetStatus(codes.Error, "context canceled")
		return ctx.Err()
	default:
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	logger.Debug("download package")

	imagePath, err := d.download(ctx, repo, name, version)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("download package '%s/%s': %w", name, version, err)
	}

	logger.Debug("install package")

	if err = d.install(ctx, deployed, name, imagePath); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("install package '%s/%s': %w", name, version, err)
	}

	d.links[imagePath] = deployed

	return nil
}

// download fetches a package image from the registry and creates an erofs image.
// If the image already exists and passes verification, download is skipped.
// Returns the path to the erofs image file.
func (d *Deployer) download(ctx context.Context, repo registry.Remote, name, version string) (string, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "download")
	defer span.End()

	span.SetAttributes(attribute.String("downloaded", d.downloads))
	span.SetAttributes(attribute.String("name", name))
	span.SetAttributes(attribute.String("version", version))

	// downloads/<registry>/<package>/<version>.erofs
	imagePath := filepath.Join(d.downloads, repo.Name, name, fmt.Sprintf("%s.erofs", version))

	rootHash, err := d.registry.GetImageRootHash(ctx, repo, name, version)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", newGetRootHashErr(err)
	}

	// skip download if image exists and passes integrity check
	if err = d.verifyImage(imagePath, rootHash); err == nil {
		return imagePath, nil
	}

	// Create directory if it does not exist (for new clusters).
	if err = os.MkdirAll(filepath.Dir(imagePath), 0755); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", newCreatePackageDirErr(err)
	}

	// verification failed - fetch fresh image from registry
	img, err := d.registry.GetImageReader(ctx, repo, name, version)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", newGetImageReaderErr(err)
	}
	defer img.Close()

	if err = verity.CreateImageByTar(ctx, img, imagePath); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", newImageByTarErr(err)
	}

	return imagePath, nil
}

// install mounts an erofs image using dm-verity for integrity verification.
// Flow: unmount old → close mapper → compute hash → create mapper → mount new.
func (d *Deployer) install(ctx context.Context, deployed, name, imagePath string) error {
	_, span := otel.Tracer(tracerName).Start(ctx, "install")
	defer span.End()

	mountPath := filepath.Join(d.deployed, deployed)

	span.SetAttributes(attribute.String("imagePath", imagePath))
	span.SetAttributes(attribute.String("mountPath", mountPath))
	span.SetAttributes(attribute.String("name", name))

	// cleanup any existing mount before installing new version
	if err := verity.Unmount(ctx, mountPath); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newUnmountErr(err)
	}

	if err := verity.CloseMapper(ctx, name); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newCloseDeviceMapperErr(err)
	}

	// compute dm-verity root hash for integrity verification during reads
	rootHash, err := verity.CreateImageHash(ctx, imagePath)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newComputeHashErr(err)
	}

	// setup dm-verity device mapper with root hash for runtime integrity checks
	if err = verity.CreateMapper(ctx, name, imagePath, rootHash); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newCreateDeviceMapperErr(err)
	}

	if err = verity.Mount(ctx, name, mountPath); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newMountErr(err)
	}

	return nil
}

func (d *Deployer) Cleanup(ctx context.Context, name string) error {
	_, span := otel.Tracer(tracerName).Start(ctx, "Cleanup")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))
	span.SetAttributes(attribute.String("downloaded", d.downloads))
	span.SetAttributes(attribute.String("deployed", d.deployed))

	logger := d.logger.With(
		slog.String("name", name),
		slog.String("downloaded", d.downloads),
		slog.String("deployed", d.deployed))

	d.mu.Lock()
	defer d.mu.Unlock()

	logger.Debug("cleanup package")

	// Unmount the erofs image and close the device mapper
	mountPath := filepath.Join(d.deployed, name)
	if _, err := os.Stat(mountPath); err == nil {
		if err = verity.Unmount(ctx, mountPath); err != nil {
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("unmount '%s': %w", mountPath, err)
		}

		if err = verity.CloseMapper(ctx, name); err != nil {
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("close device mapper '%s': %w", name, err)
		}

		logger.Info("unmounted package", slog.String("path", mountPath))
	}

	// Remove downloaded image files owned by this deployed name
	for imagePath, link := range d.links {
		if link != name {
			continue
		}

		delete(d.links, imagePath)

		// Skip removal if another deployed name still references this image
		if d.isDownloadReferenced(imagePath) {
			continue
		}

		if err := os.Remove(imagePath); err != nil && !os.IsNotExist(err) {
			logger.Warn("failed to remove image file",
				slog.String("path", imagePath), log.Err(err))
			continue
		}

		logger.Info("removed image file", slog.String("path", imagePath))

		// Try to remove empty parent directories up to the downloads root:
		// downloads/<registry>/<package>/<version>.erofs → clean <package>, then <registry>
		d.removeEmptyParents(imagePath, logger)
	}

	return nil
}

// isDownloadReferenced returns true if any link (other than entries already
// deleted from the map) still points to the given image path.
func (d *Deployer) isDownloadReferenced(imagePath string) bool {
	for ip := range d.links {
		if ip == imagePath {
			return true
		}
	}

	return false
}

// removeEmptyParents walks up from path toward d.downloads, removing each
// directory only if it is empty. Stops at d.downloads (never removes it).
func (d *Deployer) removeEmptyParents(path string, logger *log.Logger) {
	for dir := filepath.Dir(path); dir != d.downloads; dir = filepath.Dir(dir) {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			break
		}

		if err = os.Remove(dir); err != nil {
			logger.Warn("failed to remove empty directory",
				slog.String("path", dir), log.Err(err))
			break
		}

		logger.Debug("removed empty directory", slog.String("path", dir))
	}
}

// verifyImage checks that the image exists and is valid.
func (d *Deployer) verifyImage(imagePath, _ string) error {
	if _, err := os.Stat(imagePath); err != nil {
		return fmt.Errorf("stat package image '%s': %w", imagePath, err)
	}

	// TODO(ipaqsa): before implementing verify mechanism wait until all packages have root hash

	return nil
}
