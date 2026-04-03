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

package symlink

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
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	tracerName = "installer"
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
	Download(ctx context.Context, cred registry.Remote, out, packageName, tag string) error
}

func NewDeployer(svc registryService, downloaded string, logger *log.Logger) *Deployer {
	return &Deployer{
		downloads: filepath.Join(downloaded, "downloads"),
		deployed:  filepath.Join(downloaded, "deployed"),

		links: make(map[string]string),

		registry: svc,
		logger:   logger.Named("symlink-deployer"),
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

	if err := d.download(ctx, repo, name, version); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("download package '%s/%s': %w", name, version, err)
	}

	logger.Debug("install package")

	if err := d.install(ctx, repo, deployed, name, version); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("install package '%s/%s': %w", name, version, err)
	}

	return nil
}

func (d *Deployer) download(ctx context.Context, repo registry.Remote, name, version string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Install")
	defer span.End()

	span.SetAttributes(attribute.String("downloaded", d.downloads))
	span.SetAttributes(attribute.String("name", name))
	span.SetAttributes(attribute.String("version", version))

	versionPath := filepath.Join(d.downloads, repo.Name, name, version)
	if _, err := os.Stat(versionPath); err == nil {
		return nil
	}

	// Create directory if it does not exist (for new clusters).
	if err := os.MkdirAll(d.downloads, 0755); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newCreatePackageDirErr(err)
	}

	if err := d.registry.Download(ctx, repo, versionPath, name, version); err != nil {
		return newDownloadErr(err)
	}

	return nil
}

func (d *Deployer) install(ctx context.Context, repo registry.Remote, deployed, name, version string) error {
	_, span := otel.Tracer(tracerName).Start(ctx, "install")
	defer span.End()

	span.SetAttributes(attribute.String("downloaded", d.downloads))
	span.SetAttributes(attribute.String("deployed", d.deployed))
	span.SetAttributes(attribute.String("name", name))
	span.SetAttributes(attribute.String("version", version))

	versionPath := filepath.Join(d.downloads, repo.Name, name, version)
	linkPath := filepath.Join(d.deployed, deployed)

	// Create directory if it does not exist (for new clusters).
	if err := os.MkdirAll(d.deployed, 0755); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newCreatePackageDirErr(err)
	}

	// Remove old symlink if exists (for atomic update)
	// Use Lstat to avoid following the symlink
	if _, err := os.Lstat(linkPath); err == nil {
		if err = os.Remove(linkPath); err != nil {
			return newRemoveOldVersionErr(err)
		}
	}

	// <downloaded>/<version>
	if _, err := os.Stat(versionPath); err != nil {
		return newCheckVersionErr(err)
	}

	// Create new symlink pointing to permanent location
	if err := os.Symlink(versionPath, linkPath); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newCreateSymlinkErr(err)
	}

	d.links[versionPath] = deployed

	return nil
}

// downloaded cleanup:
// Goal: cleanup only unused links and downloaded registries and packages
//
// downloaded/downloads/<registry>/<package>/<version>
// downloaded/deployed/<app>

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

	// Remove the symlink from deployed/
	linkPath := filepath.Join(d.deployed, name)
	if _, err := os.Lstat(linkPath); err == nil {
		if err = os.Remove(linkPath); err != nil {
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("remove symlink '%s': %w", linkPath, err)
		}

		logger.Info("removed symlink", slog.String("path", linkPath))
	}

	// Collect download paths owned by this deployed name, then remove them
	for downloadPath, link := range d.links {
		if link != name {
			continue
		}

		delete(d.links, downloadPath)

		// Skip removal if another deployed name still references this download
		if d.isDownloadReferenced(downloadPath) {
			continue
		}

		if err := os.RemoveAll(downloadPath); err != nil {
			logger.Warn("failed to remove download directory",
				slog.String("path", downloadPath), log.Err(err))
			continue
		}

		logger.Info("removed download directory", slog.String("path", downloadPath))

		// Try to remove empty parent directories up to the downloads root:
		// downloads/<registry>/<package>/<version> → clean <package>, then <registry>
		d.removeEmptyParents(downloadPath, logger)
	}

	return nil
}

// isDownloadReferenced returns true if any link (other than entries already
// deleted from the map) still points to the given download path.
func (d *Deployer) isDownloadReferenced(downloadPath string) bool {
	for dp := range d.links {
		if dp == downloadPath {
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
