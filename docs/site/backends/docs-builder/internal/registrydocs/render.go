// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registrydocs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/flant/docs-builder/internal/docs"
	"github.com/flant/docs-builder/internal/ondemand"
	"github.com/flant/docs-builder/internal/registry"
)

// Render takes the documentation of the version from the first source that
// has it and renders it with the site template from initDir. With ms it
// reports how long the download (by the image the documentation came from)
// and the Hugo render took.
func Render(sources ModuleSources, clients *registry.Clients, initDir string, logger *log.Logger, ms *metricsstorage.MetricStorage) ondemand.RenderFunc {
	return func(ctx context.Context, module, version, workDir, outDir string, withAI bool, stage func(ondemand.Stage)) error {
		stage(ondemand.StageDownload)

		start := time.Now()
		archivePath := filepath.Join(workDir, "docs.tar")
		image, err := fetchDocs(ctx, sources, clients, module, version, archivePath, logger)
		if err != nil {
			return fmt.Errorf("%w: %w", ondemand.ErrFetch, err)
		}
		ondemand.ObserveStage(ms, "download", image, start)

		stage(ondemand.StageRender)

		archive, err := os.Open(archivePath)
		if err != nil {
			return fmt.Errorf("open archive: %w", err)
		}
		defer archive.Close()

		start = time.Now()
		if err := docs.RenderVersion(ctx, initDir, filepath.Join(workDir, "site"), outDir, module, version, archive, withAI, logger); err != nil {
			return err
		}
		ondemand.ObserveStage(ms, "render", "", start)

		return nil
	}
}

// fetchDocs writes the documentation of the version to path. For each source
// it takes the release image first: it is much smaller than the full module
// image and carries the same documentation in recent module builds. Older
// builds have no docs in the release image, so then the full image is used.
// A development branch (a tag that is not a version, see ModulePullOverride)
// is taken from the full image only: that is the image the cluster runs.
// A release tag in one source does not guarantee the images, so the next
// source is tried when one has neither. An image without docs/ still leaves
// its archive at path: RenderVersion then reports ErrNoPages, unless another
// image could not be fetched for a reason other than its absence. It returns
// the kind of the image the archive came from: release or module.
func fetchDocs(ctx context.Context, sources ModuleSources, clients *registry.Clients, module, version, path string, logger *log.Logger) (string, error) {
	srcs, err := moduleSources(ctx, sources, module)
	if ondemand.Unpublished(err) {
		return "", fmt.Errorf("%w: %w", ondemand.ErrVersionNotFound, err)
	}
	if err != nil {
		return "", err
	}

	var errs []error
	notFound, fetched := true, false
	kind := ""
	for _, src := range srcs {
		client, err := clients.For(src)
		if err != nil {
			errs = append(errs, fmt.Errorf("source %s: %w", src.Name, err))
			notFound = false
			continue
		}

		type source struct {
			kind  string
			fetch func(context.Context, string, string) (v1.Image, error)
		}
		images := []source{{"release", client.ReleaseImage}, {"module", client.Image}}
		if docs.ValidateModuleVersion(module, version) != nil {
			images = images[1:]
		}
		for _, image := range images {
			img, err := image.fetch(ctx, module, version)
			if err != nil {
				errs = append(errs, fmt.Errorf("source %s, %s image: %w", src.Name, image.kind, err))
				notFound = notFound && errors.Is(err, registry.ErrNotFound)
				continue
			}

			hasDocs, err := writeArchive(img, path)
			if err != nil {
				return "", fmt.Errorf("source %s, %s image: %w", src.Name, image.kind, err)
			}
			fetched, kind = true, image.kind

			if hasDocs {
				logger.Info("documentation fetched", slog.String("module", module), slog.String("version", version),
					slog.String("module_source", src.Name), slog.String("image", image.kind))
				return image.kind, nil
			}
		}
	}

	// An image without docs is the final answer only when every image that
	// was not fetched does not exist; a registry error may hide the docs.
	if fetched && notFound {
		return kind, nil
	}

	err = fmt.Errorf("no source has %s %s: %w", module, version, errors.Join(errs...))
	if notFound {
		return "", fmt.Errorf("%w: %w", ondemand.ErrVersionNotFound, err)
	}

	return "", err
}

func writeArchive(img v1.Image, path string) (bool, error) {
	f, err := os.Create(path)
	if err != nil {
		return false, fmt.Errorf("create archive: %w", err)
	}

	hasDocs, err := DocsArchive(img, f)
	if err != nil {
		f.Close()
		return false, fmt.Errorf("extract documentation: %w", err)
	}

	if err := f.Close(); err != nil {
		return false, fmt.Errorf("close archive: %w", err)
	}

	return hasDocs, nil
}
