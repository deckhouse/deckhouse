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

package ondemand

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/flant/docs-builder/internal/docs"
	"github.com/flant/docs-builder/internal/metrics"
	"github.com/flant/docs-builder/internal/registry"
)

// Stage of a version render, shown on the placeholder page.
type Stage string

const (
	StageQueued   Stage = "queued"
	StageDownload Stage = "download"
	StageRender   Stage = "render"
	StageDone     Stage = "done"
	StageFailed   Stage = "failed"
)

// ErrVersionNotFound means no source of the module has an image of the version.
var ErrVersionNotFound = errors.New("module version not found")

// errFetch marks a failure to get the documentation from the registry, as
// opposed to a failure to build the pages.
var errFetch = errors.New("fetch documentation")

// Reasons of a failed render, shown to the reader instead of the error: the
// error names registries, repositories and cluster objects, and goes to the
// log only.
const (
	ReasonTimeout  = "timeout"
	ReasonRegistry = "registry"
	ReasonBuild    = "build"
	ReasonBusy     = "busy"
)

func reason(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return ReasonTimeout
	// The image is fetched, its files are the reason.
	case errors.Is(err, docs.ErrDocsTooLarge):
		return ReasonBuild
	case errors.Is(err, errFetch):
		return ReasonRegistry
	default:
		return ReasonBuild
	}
}

// ModuleSources finds the registries of a module: the cluster objects
// (registry.Sources) or the fixed list of the public site
// (registry.StaticSources).
type ModuleSources interface {
	ForModule(ctx context.Context, module string) ([]registry.Source, error)
}

// RenderFunc renders a module version into outDir/<lang>/, using workDir for
// temporary files, and reports its progress through stage. withAI adds the AI
// export (Markdown, llms.txt) to the pages.
type RenderFunc func(ctx context.Context, module, version, workDir, outDir string, withAI bool, stage func(Stage)) error

// RegistryRender takes the documentation of the version from the first source
// that has it and renders it with the site template from initDir.
// With ms it reports how long the download (by the image the documentation
// came from) and the Hugo render took.
func RegistryRender(sources ModuleSources, clients *registry.Clients, initDir string, logger *log.Logger, ms *metricsstorage.MetricStorage) RenderFunc {
	return func(ctx context.Context, module, version, workDir, outDir string, withAI bool, stage func(Stage)) error {
		stage(StageDownload)

		start := time.Now()
		archivePath := filepath.Join(workDir, "docs.tar")
		image, err := fetchDocs(ctx, sources, clients, module, version, archivePath, logger)
		if err != nil {
			return fmt.Errorf("%w: %w", errFetch, err)
		}
		observeStage(ms, "download", image, start)

		stage(StageRender)

		archive, err := os.Open(archivePath)
		if err != nil {
			return fmt.Errorf("open archive: %w", err)
		}
		defer archive.Close()

		start = time.Now()
		if err := docs.RenderVersion(ctx, initDir, filepath.Join(workDir, "site"), outDir, module, version, archive, withAI, logger); err != nil {
			return err
		}
		observeStage(ms, "render", "", start)

		return nil
	}
}

// observeStage reports a stage of a render that succeeded.
func observeStage(ms *metricsstorage.MetricStorage, stage, image string, start time.Time) {
	if ms == nil {
		return
	}
	ms.HistogramObserve(metrics.DocsBuilderOnDemandStageSeconds, time.Since(start).Seconds(), map[string]string{"stage": stage, "image": image}, nil)
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
	srcs, err := sources.ForModule(ctx, module)
	if errors.Is(err, registry.ErrNoSource) || apierrors.IsNotFound(err) {
		return "", fmt.Errorf("%w: %w", ErrVersionNotFound, err)
	}
	if err != nil {
		return "", fmt.Errorf("find sources: %w", err)
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
				notFound = notFound && isNotFound(err)
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
		return "", fmt.Errorf("%w: %w", ErrVersionNotFound, err)
	}

	return "", err
}

// isNotFound tells a missing image apart from a registry that could not be
// asked: only the former means the version does not exist.
func isNotFound(err error) bool {
	var terr *transport.Error
	if !errors.As(err, &terr) {
		return false
	}

	if terr.StatusCode == http.StatusNotFound {
		return true
	}

	for _, d := range terr.Errors {
		if d.Code == transport.ManifestUnknownErrorCode || d.Code == transport.NameUnknownErrorCode {
			return true
		}
	}

	return false
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
