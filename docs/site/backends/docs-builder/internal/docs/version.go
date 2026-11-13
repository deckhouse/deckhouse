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

package docs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/spf13/afero"
	"github.com/spf13/fsync"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/aiexport"
)

// versionRegexp matches a module version as it appears in a page path:
// v1.13.0, v1.13.0-rc.1. It is in lower case for the same reason as
// tagRegexp: a version is rendered as a tag, so every version must be one
// (ValidateModuleVersion checks the length as well).
var versionRegexp = regexp.MustCompile(`^v\d+\.\d+\.\d+(-[0-9a-z.-]+)?$`)

// tagRegexp matches an image tag (the OCI distribution grammar), e.g. the
// development branch of a ModulePullOverride, in lower case: Hugo lower-cases
// page paths, so a tag with upper-case letters would not find its pages. It
// never matches "." or "..".
var tagRegexp = regexp.MustCompile(`^[a-z0-9_][a-z0-9_.-]{0,127}$`)

// reservedSegments are the path segments after /modules/<module>/ that
// already have a meaning: the page listing module versions and the release
// channels. A tag taking one would hide those pages.
var reservedSegments = []string{"versions", "alpha", "beta", "early-access", "stable", "rock-solid", "latest"}

// HugoInitDir holds the site template in the image.
const HugoInitDir = hugoInitDir

const (
	// versionSiteData marks a site of one module version for the templates
	// (hugo.Data.version_site).
	versionSiteData = "data/version_site.json"
	aiContentDir    = "content/ai"
)

// Languages the documentation site is rendered in.
var Languages = []string{"ru", "en"}

// ErrNoPages means the module version has no documentation pages: its image
// carries no docs/ directory.
var ErrNoPages = errors.New("module version has no documentation pages")

// ErrBuild means Hugo could not build the pages of the version: the
// template and the documentation do not fit, as opposed to a failure of the
// pod (a full disk, a cancelled render).
var ErrBuild = errors.New("hugo can not build the pages of the version")

// The documentation files of one module version are a few megabytes; far
// above that they are not documentation and would fill the cache volume.
const (
	MaxDocsBytes = 256 << 20
	MaxDocsFiles = 20000
)

// ErrDocsTooLarge means the documentation files of the version are over
// MaxDocsBytes or MaxDocsFiles. It does not pass until the image changes.
var ErrDocsTooLarge = errors.New("the documentation files of the version are too large")

// DocsSize counts the documentation files of one version against
// MaxDocsBytes and MaxDocsFiles.
type DocsSize struct {
	files int
	bytes int64
}

// Add counts one more file of size bytes and fails with ErrDocsTooLarge once
// the files are over a limit.
func (d *DocsSize) Add(size int64) error {
	d.files++
	d.bytes += size
	if size < 0 || d.files > MaxDocsFiles || d.bytes > MaxDocsBytes {
		return fmt.Errorf("%w: more than %d files or %d bytes", ErrDocsTooLarge, MaxDocsFiles, int64(MaxDocsBytes))
	}

	return nil
}

// ValidateModuleName rejects a module name that is not safe to use as a path
// segment.
func ValidateModuleName(module string) error {
	return validateModuleName(module)
}

// ValidateModuleVersion rejects a module name or version that is not safe to
// use as a path segment.
func ValidateModuleVersion(module, version string) error {
	if err := validateModuleName(module); err != nil {
		return err
	}

	if !versionRegexp.MatchString(version) {
		return fmt.Errorf("invalid version %q: must match %s", version, versionRegexp.String())
	}

	// A version is rendered as a tag (RenderVersion).
	return ValidateModuleTag(module, version)
}

// ValidateModuleTag is ValidateModuleVersion for any image tag of the module,
// such as a development branch: the tag is only checked to be a safe path
// segment that does not take the URL of the versions page or a channel.
func ValidateModuleTag(module, tag string) error {
	if err := validateModuleName(module); err != nil {
		return err
	}

	if !tagRegexp.MatchString(tag) || slices.Contains(reservedSegments, tag) {
		return fmt.Errorf("invalid tag %q: must match %s and not be one of %v", tag, tagRegexp.String(), reservedSegments)
	}

	return nil
}

// RenderVersion renders one module version, given as an archive of its
// documentation files (see ondemand.DocsArchive), as a Hugo site of its own
// in siteDir, and moves the pages of each language to outDir/<lang>/ along
// with the search index of the version (search.json). With withAI it also writes the AI export: Markdown next to
// every page, llms.txt and corpus.json. initDir holds the site template
// (/app/hugo-init in the image). siteDir is left for the caller to remove.
func RenderVersion(ctx context.Context, initDir, siteDir, outDir, module, version string, archive io.Reader, withAI bool, logger *log.Logger) error {
	if err := ValidateModuleTag(module, version); err != nil {
		return err
	}

	public, err := filepath.Abs(filepath.Join(siteDir, "public"))
	if err != nil {
		return fmt.Errorf("publish dir: %w", err)
	}

	// Render metrics are reported by the caller.
	site := &Service{
		baseDir: siteDir,
		wrapDestination: func(fs afero.Fs) afero.Fs {
			return newVersionLinksFs(fs, public, module, version)
		},
		logger: logger,
	}

	syncer := fsync.NewSyncer()
	syncer.NoChmod = true
	syncer.NoTimes = true
	if err := syncer.Sync(siteDir, initDir); err != nil {
		return fmt.Errorf("copy site template: %w", err)
	}

	if err := os.MkdirAll(filepath.Join(siteDir, modulesDir), 0700); err != nil {
		return fmt.Errorf("mkdir modules data: %w", err)
	}

	if err := site.unpack(archive, module, version); err != nil {
		return fmt.Errorf("unpack: %w", err)
	}

	if _, err := os.Stat(filepath.Join(siteDir, contentDir, module, version)); errors.Is(err, os.ErrNotExist) {
		return ErrNoPages
	}

	// The templates index only this version and keep links inside it.
	versionSite, err := json.Marshal(map[string]string{"module": module, "version": version})
	if err != nil {
		return fmt.Errorf("marshal version site data: %w", err)
	}
	if err := os.WriteFile(filepath.Join(siteDir, versionSiteData), versionSite, 0600); err != nil {
		return fmt.Errorf("write version site data: %w", err)
	}

	// The AI manifest page costs render time for no use without the export.
	if !withAI {
		if err := os.RemoveAll(filepath.Join(siteDir, aiContentDir)); err != nil {
			return fmt.Errorf("remove ai content: %w", err)
		}
	}

	if err := site.buildHugo(ctx); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("hugo build: %w", err)
		}
		return fmt.Errorf("%w: %w", ErrBuild, err)
	}

	if err := os.MkdirAll(outDir, 0700); err != nil {
		return fmt.Errorf("mkdir out: %w", err)
	}

	if err := writeSummary(siteDir, outDir, module, version, logger); err != nil {
		return fmt.Errorf("write summary: %w", err)
	}

	rendered := 0
	for _, lang := range Languages {
		public := filepath.Join(siteDir, "public", lang)
		src := filepath.Join(public, "modules", module, version)
		if _, err := os.Stat(src); errors.Is(err, os.ErrNotExist) {
			continue
		}

		// The export writes Markdown next to the pages, so it runs before they move.
		// The docs are fine without it; degrade as the site build does.
		if withAI {
			if err := aiexport.Export(filepath.Join(siteDir, "public"), lang, logger.Named("ai_export")); err != nil {
				logger.Warn("ai export", slog.String("module", module), slog.String("version", version), slog.String("lang", lang), log.Err(err))
			}
		}

		dst := filepath.Join(outDir, lang)
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("move %s pages: %w", lang, err)
		}
		rendered++

		extras := map[string]string{
			filepath.Join(public, "search", "search.json"):           "search.json",
			filepath.Join(public, "modules", "external-llms.txt"):    "llms.txt",
			filepath.Join(public, "modules", "external-corpus.json"): "corpus.json",
		}
		for from, to := range extras {
			if err := os.Rename(from, filepath.Join(dst, to)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("move %s %s: %w", lang, to, err)
			}
		}
	}

	// buildHugo drops a module that fails to render and builds the rest of
	// the site, so a broken module shows up as missing pages, not an error.
	// It fails the same way until the template or the image changes.
	if rendered == 0 {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("hugo build: %w", err)
		}
		return fmt.Errorf("%w: module %s %s failed to render, the reason is in the docs-builder log", ErrBuild, module, version)
	}

	return nil
}
