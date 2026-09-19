// Copyright 2023 Flant JSC
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

package downloader

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/shell-operator/pkg/utils/measure"
	crv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/iancoleman/strcase"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	moduletools "github.com/deckhouse/deckhouse/go_lib/module"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	defaultModuleWeight = 900
	DefaultDevVersion   = "dev"

	tracerName = "downloader"
)

type ModuleDownloader struct {
	dc                   dependency.Container
	downloadedModulesDir string

	ms              *v1alpha1.ModuleSource
	registryOptions []cr.Option
	logger          *log.Logger
}

func NewModuleDownloader(dc dependency.Container, downloadedModulesDir string, ms *v1alpha1.ModuleSource, logger *log.Logger, registryOptions []cr.Option) *ModuleDownloader {
	return &ModuleDownloader{
		dc:                   dc,
		downloadedModulesDir: downloadedModulesDir,
		ms:                   ms,
		registryOptions:      registryOptions,
		logger:               logger,
	}
}

type ModuleDownloadResult struct {
	Checksum      string
	ModuleVersion string

	ModuleDefinition *moduletypes.Definition
	Changelog        map[string]any
}

// DownloadDevImageTag downloads image tag and store it in the .../<moduleName>/dev fs path
// if checksum is equal to a module image digest - do nothing
// otherwise return new digest
func (md *ModuleDownloader) DownloadDevImageTag(moduleName, imageTag, checksum string) (string, *moduletypes.Definition, error) {
	moduleStorePath := path.Join(md.downloadedModulesDir, moduleName, DefaultDevVersion)

	img, err := md.fetchImage(moduleName, imageTag)
	if err != nil {
		return "", nil, err
	}

	digest, err := img.Digest()
	if err != nil {
		return "", nil, fmt.Errorf("digest: %w", err)
	}

	if digest.String() == checksum {
		// module is up-to-date
		return "", nil, nil
	}

	if _, err = md.fetchAndCopyModuleByVersion(moduleName, imageTag, moduleStorePath); err != nil {
		return "", nil, err
	}

	return digest.String(), md.fetchModuleDefinitionFromFS(moduleName, moduleStorePath), nil
}

func (md *ModuleDownloader) DownloadByModuleVersion(ctx context.Context, moduleName, moduleVersion string) (*DownloadStatistic, error) {
	_, span := otel.Tracer(tracerName).Start(ctx, "DownloadByModuleVersion")
	defer span.End()

	if !strings.HasPrefix(moduleVersion, "v") {
		moduleVersion = "v" + moduleVersion
	}

	moduleVersionPath := path.Join(md.downloadedModulesDir, moduleName, moduleVersion)

	return md.fetchAndCopyModuleByVersion(moduleName, moduleVersion, moduleVersionPath)
}

// DownloadMetadataFromReleaseChannel downloads only module release image with metadata: version.json, checksum.json(soon)
// does not fetch and install the desired version on the module, only fetches its module definition
func (md *ModuleDownloader) DownloadMetadataFromReleaseChannel(ctx context.Context, moduleName, releaseChannel string) (*ModuleDownloadResult, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "DownloadMetadataFromReleaseChannel")
	defer span.End()

	span.SetAttributes(attribute.String("module", moduleName))
	span.SetAttributes(attribute.String("releaseChannel", releaseChannel))

	// the release image is addressed by a kebab-cased channel name exactly the way it is
	// addressed by an explicit version tag, so both paths go through the same
	// cr.ResolveChannel call
	info, err := md.resolveRelease(ctx, moduleName, strcase.ToKebab(releaseChannel))
	if err != nil {
		return nil, err
	}

	return releaseResult(info)
}

// DownloadReleaseImageInfoByVersion downloads only module release image with metadata: version.json
// does not fetch and install the desired version on the module, only fetches its module definition
func (md *ModuleDownloader) DownloadReleaseImageInfoByVersion(ctx context.Context, moduleName, moduleVersion string) (*ModuleDownloadResult, error) {
	info, err := md.resolveRelease(ctx, moduleName, moduleVersion)
	if err != nil {
		return nil, fmt.Errorf("fetch module release: %w", err)
	}

	res, err := releaseResult(info)
	if err != nil {
		return nil, err
	}
	// the caller asked for this exact version, keep answering in its terms
	res.ModuleVersion = moduleVersion

	if res.ModuleDefinition != nil {
		return res, nil
	}

	md.logger.Info("can not find module definition in metadata, extracting from image",
		slog.String("module_name", moduleName),
		slog.String("module_version", moduleVersion),
	)

	// fetch module image
	img, err := md.fetchImage(moduleName, moduleVersion)
	if err != nil {
		return nil, fmt.Errorf("fetch image: %w", err)
	}

	def, err := md.fetchModuleDefinitionFromModuleImage(moduleName, img)
	if err != nil {
		return nil, fmt.Errorf("fetch module definition: %w", err)
	}

	res.ModuleDefinition = def

	md.logger.Info("module definition extracted from image",
		slog.String("module_name", moduleName),
		slog.String("module_version", moduleVersion),
		slog.Any("def", def),
	)

	return res, nil
}

func (md *ModuleDownloader) GetDocumentationArchive(moduleName, moduleVersion string) (io.ReadCloser, error) {
	if !strings.HasPrefix(moduleVersion, "v") {
		moduleVersion = "v" + moduleVersion
	}

	img, err := md.fetchImage(moduleName, moduleVersion)
	if err != nil {
		return nil, fmt.Errorf("fetch image: %w", err)
	}

	docs, err := moduletools.ExtractDocs(img)
	if err != nil {
		return nil, fmt.Errorf("extract docs: %w", err)
	}
	return docs, nil
}

func (md *ModuleDownloader) fetchImage(moduleName, imageTag string) (crv1.Image, error) {
	regCli, err := md.dc.GetRegistryClient(path.Join(md.ms.Spec.Registry.Repo, moduleName), md.registryOptions...)
	if err != nil {
		return nil, fmt.Errorf("fetch module error: %v", err)
	}

	img, err := regCli.Image(context.TODO(), imageTag)
	if err != nil {
		return nil, fmt.Errorf("image: %w", err)
	}
	return img, nil
}

func (md *ModuleDownloader) storeModule(moduleStorePath string, img crv1.Image) (*DownloadStatistic, error) {
	_ = os.RemoveAll(moduleStorePath)

	ds, err := md.copyModuleToFS(moduleStorePath, img)
	if err != nil {
		return nil, fmt.Errorf("copy module error: %v", err)
	}

	return ds, nil
}

func (md *ModuleDownloader) fetchAndCopyModuleByVersion(moduleName, moduleVersion, moduleVersionPath string) (*DownloadStatistic, error) {
	// TODO: if module exists on fs - skip this step

	img, err := md.fetchImage(moduleName, moduleVersion)
	if err != nil {
		return nil, err
	}

	return md.storeModule(moduleVersionPath, img)
}

func (md *ModuleDownloader) copyModuleToFS(rootPath string, img crv1.Image) (*DownloadStatistic, error) {
	rc, err := cr.Extract(img)
	if err != nil {
		return nil, fmt.Errorf("extract: %w", err)
	}
	defer rc.Close()

	ds, err := md.copyLayersToFS(rootPath, rc)
	if err != nil {
		return nil, fmt.Errorf("copy tar to fs: %w", err)
	}

	return ds, nil
}

func (md *ModuleDownloader) copyLayersToFS(rootPath string, rc io.ReadCloser) (*DownloadStatistic, error) {
	ds := new(DownloadStatistic)
	defer measure.Duration(func(d time.Duration) {
		ds.PullDuration = d
		if os.Getenv("D8_IS_TESTS_ENVIRONMENT") == "true" {
			ds.PullDuration, _ = time.ParseDuration("555s")
		}
	})()

	if err := os.MkdirAll(rootPath, 0o700); err != nil {
		return nil, fmt.Errorf("mkdir root path: %w", err)
	}

	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of archive
			return ds, nil
		}
		// Read the header only after the error check: tar.Next returns a nil header
		// along with any error other than io.EOF.
		if err != nil {
			return nil, fmt.Errorf("tar reader next: %w", err)
		}

		ds.Size += uint32(hdr.Size)

		if strings.Contains(hdr.Name, "..") {
			// CWE-22 check, prevents path traversal
			return nil, fmt.Errorf("path traversal detected in the module archive: malicious path %v", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path.Join(rootPath, hdr.Name), 0o700); err != nil {
				return nil, fmt.Errorf("mkdir all: %w", err)
			}
		case tar.TypeReg:
			outFile, err := os.Create(path.Join(rootPath, hdr.Name))
			if err != nil {
				return nil, fmt.Errorf("create file: %w", err)
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return nil, fmt.Errorf("copy: %w", err)
			}
			outFile.Close()

			// remove only 'user' permission bit, E.x.: 644 => 600, 755 => 700
			if err = os.Chmod(outFile.Name(), os.FileMode(hdr.Mode)&0o700); err != nil {
				return nil, fmt.Errorf("chmod: %w", err)
			}
		case tar.TypeSymlink:
			link := path.Join(rootPath, hdr.Name)
			if isRel(hdr.Linkname, link) && isRel(hdr.Name, link) {
				if err := os.Symlink(hdr.Linkname, link); err != nil {
					return nil, fmt.Errorf("create symlink: %w", err)
				}
			}

		case tar.TypeLink:
			if err = os.Link(path.Join(rootPath, hdr.Linkname), path.Join(rootPath, hdr.Name)); err != nil {
				return nil, fmt.Errorf("create hardlink: %w", err)
			}

		default:
			return nil, errors.New("unknown tar type")
		}
	}
}

// resolveRelease reads <repo>/<module>/release:<tag> through the shared go_lib/dependency/cr
// package. The tag is either a kebab-cased release channel or an explicit version - the release
// image is addressed identically in both cases.
// Image fetch path example: registry.deckhouse.io/deckhouse/ce/modules/$moduleName/release:$tag
func (md *ModuleDownloader) resolveRelease(ctx context.Context, moduleName, tag string) (cr.ReleaseInfo, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "resolveRelease")
	defer span.End()

	md.logger.Info("fetching module release metadata",
		slog.String("path", path.Join(md.ms.Spec.Registry.Repo, moduleName, "release")),
		slog.String("tag", tag),
	)

	regCli, err := md.dc.GetRegistryClient(path.Join(md.ms.Spec.Registry.Repo, moduleName, "release"), md.registryOptions...)
	if err != nil {
		return cr.ReleaseInfo{}, fmt.Errorf("fetch release image error: %w", err)
	}

	info, err := cr.ResolveChannel(ctx, regCli, tag)
	if err != nil {
		return info, fmt.Errorf("get image info: %w", err)
	}

	return info, nil
}

// releaseResult turns the opaque release metadata into what the controller works with: the
// cr package keeps the version as a raw string because dev builds ship non-semver versions, but
// the controller orders releases and therefore has to insist on a semver here.
func releaseResult(info cr.ReleaseInfo) (*ModuleDownloadResult, error) {
	version, err := semver.NewVersion(info.Version)
	if err != nil {
		return nil, fmt.Errorf("parse version %q: %w", info.Version, err)
	}

	res := &ModuleDownloadResult{
		Checksum:      info.Digest,
		ModuleVersion: "v" + version.String(),
		Changelog:     info.Changelog,
	}

	// module.yaml stays undecoded in the shared cr package: the definition type lives here and
	// drags addon-operator with it, which is exactly what the shared module must not carry.
	if len(info.ModuleYAML) > 0 {
		def := new(moduletypes.Definition)
		if err = yaml.Unmarshal(info.ModuleYAML, def); err != nil {
			return nil, fmt.Errorf("unmarshal module yaml failed: %w", err)
		}
		res.ModuleDefinition = def
	}

	return res, nil
}

func (md *ModuleDownloader) fetchModuleDefinitionFromFS(name, path string) *moduletypes.Definition {
	def := &moduletypes.Definition{
		Name:   name,
		Weight: defaultModuleWeight,
		Path:   path,
	}

	defPath := filepath.Join(path, moduletypes.DefinitionFile)

	// do not add os.Stat check, because os.Open will return error if file does not exist
	f, err := os.Open(defPath)
	if err != nil {
		return def
	}
	defer f.Close()

	if err = yaml.NewDecoder(f).Decode(def); err != nil {
		return def
	}

	return def
}

func (md *ModuleDownloader) fetchModuleDefinitionFromModuleImage(moduleName string, img crv1.Image) (*moduletypes.Definition, error) {
	def := &moduletypes.Definition{
		Name:   moduleName,
		Weight: defaultModuleWeight,
	}

	rc, err := cr.Extract(img)
	if err != nil {
		return nil, fmt.Errorf("extract: %w", err)
	}
	defer rc.Close()

	rr := &moduleReader{
		moduleReader: bytes.NewBuffer(nil),
	}

	if err = rr.untarMetadata(rc); err != nil {
		return def, fmt.Errorf("untar metadata: %w", err)
	}

	if rr.moduleReader.Len() > 0 {
		err = yaml.NewDecoder(rr.moduleReader).Decode(def)
		if err != nil {
			return nil, fmt.Errorf("yaml decode: %w", err)
		}
	}

	return def, nil
}

type moduleReader struct {
	moduleReader *bytes.Buffer
}

func (rr *moduleReader) untarMetadata(rc io.ReadCloser) error {
	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of archive
			return nil
		}

		if err != nil {
			return fmt.Errorf("next: %w", err)
		}

		if strings.HasPrefix(hdr.Name, ".werf") {
			continue
		}

		switch strings.ToLower(hdr.Name) {
		case "module.yaml":
			_, err := io.Copy(rr.moduleReader, tr)
			if err != nil {
				return fmt.Errorf("copy: %w", err)
			}

			return nil
		default:

			continue
		}
	}
}

func isRel(candidate, target string) bool {
	// GOOD: resolves all symbolic links before checking
	// that `candidate` does not escape from `target`
	if filepath.IsAbs(candidate) {
		return false
	}
	realpath, err := filepath.EvalSymlinks(filepath.Join(target, candidate))
	if err != nil {
		return false
	}
	relpath, err := filepath.Rel(target, realpath)
	return err == nil && !strings.HasPrefix(filepath.Clean(relpath), "..")
}
