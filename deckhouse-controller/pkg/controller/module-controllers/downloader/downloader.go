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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/flant/shell-operator/pkg/utils/measure"
	crv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/iancoleman/strcase"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers/reginjector"
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
}

func NewModuleDownloader(dc dependency.Container, downloadedModulesDir string, ms *v1alpha1.ModuleSource, registryOptions []cr.Option) *ModuleDownloader {
	return &ModuleDownloader{
		dc:                   dc,
		downloadedModulesDir: downloadedModulesDir,
		ms:                   ms,
		registryOptions:      registryOptions,
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
		return "", nil, err
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
func (md *ModuleDownloader) DownloadMetadataFromReleaseChannel(ctx context.Context, moduleName, releaseChannel string) (ModuleDownloadResult, error) {
	_, span := otel.Tracer(tracerName).Start(ctx, "DownloadMetadataFromReleaseChannel")
	defer span.End()

	span.SetAttributes(attribute.String("module", moduleName))
	span.SetAttributes(attribute.String("releaseChannel", releaseChannel))

	var res ModuleDownloadResult

	// moduleVersion, checksum, changelog, err := md.fetchModuleReleaseMetadataFromReleaseChannel(moduleName, releaseChannel)
	ImageInfo, err := md.fetchModuleReleaseMetadataFromReleaseChannel(moduleName, releaseChannel)
	if err != nil {
		return res, err
	}

	res.Checksum = ImageInfo.Digest.String()
	res.ModuleVersion = "v" + ImageInfo.Metadata.Version
	res.Changelog = ImageInfo.Metadata.Changelog

	return res, nil
}

// DownloadMetadataByVersion downloads only module release image with metadata: version.json
// does not fetch and install the desired version on the module, only fetches its module definition
func (md *ModuleDownloader) DownloadMetadataByVersion(moduleName, moduleVersion string) (ModuleDownloadResult, error) {
	var res ModuleDownloadResult

	// moduleVersion, checksum, changelog, err := md.fetchModuleReleaseMetadataByVersion(moduleName, moduleVersion)
	imageInfo, err := md.fetchModuleReleaseMetadataByVersion(moduleName, moduleVersion)
	if err != nil {
		return res, err
	}

	res.Checksum = imageInfo.Digest.String()
	res.ModuleVersion = moduleVersion
	res.Changelog = imageInfo.Metadata.Changelog
	if imageInfo.Metadata.ModuleDefinition != nil {
		res.ModuleDefinition = imageInfo.Metadata.ModuleDefinition
	}

	return res, nil
}

// DownloadModuleDefinitionByVersion returns a module definition from the repo by the module's name and version(tag)
func (md *ModuleDownloader) DownloadModuleDefinitionByVersion(moduleName, moduleVersion string) (*moduletypes.Definition, error) {
	img, err := md.fetchImage(moduleName, moduleVersion)
	if err != nil {
		return nil, err
	}

	return md.fetchModuleDefinitionFromImage(moduleName, img)
}

func (md *ModuleDownloader) GetDocumentationArchive(moduleName, moduleVersion string) (io.ReadCloser, error) {
	if !strings.HasPrefix(moduleVersion, "v") {
		moduleVersion = "v" + moduleVersion
	}

	img, err := md.fetchImage(moduleName, moduleVersion)
	if err != nil {
		return nil, fmt.Errorf("fetch image: %w", err)
	}

	return moduletools.ExtractDocs(img)
}

func (md *ModuleDownloader) fetchImage(moduleName, imageTag string) (crv1.Image, error) {
	regCli, err := md.dc.GetRegistryClient(path.Join(md.ms.Spec.Registry.Repo, moduleName), md.registryOptions...)
	if err != nil {
		return nil, fmt.Errorf("fetch module error: %v", err)
	}

	return regCli.Image(context.TODO(), imageTag)
}

func (md *ModuleDownloader) storeModule(moduleStorePath string, img crv1.Image) (*DownloadStatistic, error) {
	_ = os.RemoveAll(moduleStorePath)

	ds, err := md.copyModuleToFS(moduleStorePath, img)
	if err != nil {
		return nil, fmt.Errorf("copy module error: %v", err)
	}

	// inject registry to values
	err = reginjector.InjectRegistryToModuleValues(moduleStorePath, md.ms)
	if err != nil {
		return nil, fmt.Errorf("inject registry error: %v", err)
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
		return nil, err
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

		ds.Size += uint32(hdr.Size)

		if err != nil {
			return nil, fmt.Errorf("tar reader next: %w", err)
		}

		if strings.Contains(hdr.Name, "..") {
			// CWE-22 check, prevents path traversal
			return nil, fmt.Errorf("path traversal detected in the module archive: malicious path %v", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path.Join(rootPath, hdr.Name), 0o700); err != nil {
				return nil, err
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

func (md *ModuleDownloader) fetchModuleReleaseMetadataFromReleaseChannel(moduleName, releaseChannel string) (*ImageInfo, error) {
	log.Info("fetching module release",
		slog.String("path", path.Join(md.ms.Spec.Registry.Repo, moduleName, "release")),
		slog.String("releasechannel", releaseChannel),
	)

	log.Debug("module metadata",
		slog.String("module", moduleName),
	)

	imageInfo := &ImageInfo{}

	// fill imageInfo.Image
	regCli, err := md.dc.GetRegistryClient(path.Join(md.ms.Spec.Registry.Repo, moduleName, "release"), md.registryOptions...)
	if err != nil {
		return nil, fmt.Errorf("fetch release image error: %w", err)
	}

	img, err := regCli.Image(context.TODO(), strcase.ToKebab(releaseChannel))
	if err != nil {
		return nil, fmt.Errorf("fetch image error: %w", err)
	}
	imageInfo.Image = img

	// fill imageInfo.Diest
	digest, err := img.Digest()
	if err != nil {
		return nil, fmt.Errorf("fetch digest error: %w", err)
	}
	imageInfo.Digest = digest

	// fill imageInfo.Metadata
	moduleMetadata, err := md.fetchModuleReleaseMetadata(img)
	if err != nil {
		return nil, fmt.Errorf("fetch release metadata error: %w", err)
	}

	imageInfo.Metadata = &moduleMetadata

	if moduleMetadata.Version == "" {
		return nil, fmt.Errorf("module %q metadata malformed: no version found", moduleName)
	}

	// return "v" + moduleMetadata.Version.String(), digest.String(), moduleMetadata.Changelog, nil
	return imageInfo, nil
}

func (md *ModuleDownloader) fetchModuleReleaseMetadataByVersion(moduleName, moduleVersion string) (*ImageInfo, error) {
	imageInfo := &ImageInfo{}

	// fill imageInfo.Image
	regCli, err := md.dc.GetRegistryClient(path.Join(md.ms.Spec.Registry.Repo, moduleName, "release"), md.registryOptions...)
	if err != nil {
		return imageInfo, fmt.Errorf("fetch release image error: %v", err)
	}

	img, err := regCli.Image(context.TODO(), moduleVersion)
	if err != nil {
		return imageInfo, fmt.Errorf("fetch image error: %v", err)
	}

	imageInfo.Image = img

	// fill imageInfo.Digest
	digest, err := img.Digest()
	if err != nil {
		return imageInfo, fmt.Errorf("fetch digest error: %v", err)
	}

	imageInfo.Digest = digest

	// fill imageInfo.Metadata
	moduleMetadata, err := md.fetchModuleReleaseMetadata(img)
	if err != nil {
		return imageInfo, fmt.Errorf("fetch release metadata error: %v", err)
	}

	imageInfo.Metadata = &moduleMetadata

	if moduleMetadata.Version == "" {
		return imageInfo, fmt.Errorf("module %q metadata malformed: no version found", moduleName)
	}

	// return "v" + moduleMetadata.Version.String(), digest.String(), moduleMetadata.Changelog, nil
	return imageInfo, nil
}

func (md *ModuleDownloader) fetchModuleDefinitionFromFS(name, path string) *moduletypes.Definition {
	def := &moduletypes.Definition{
		Name:   name,
		Weight: defaultModuleWeight,
		Path:   path,
	}

	defPath := filepath.Join(path, moduletypes.DefinitionFile)

	if _, err := os.Stat(defPath); err != nil {
		return def
	}

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

func (md *ModuleDownloader) fetchModuleDefinitionFromImage(moduleName string, img crv1.Image) (*moduletypes.Definition, error) {
	def := &moduletypes.Definition{
		Name:   moduleName,
		Weight: defaultModuleWeight,
	}

	rc, err := cr.Extract(img)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	buf := bytes.NewBuffer(nil)

	if err = untarModuleDefinition(rc, buf); err != nil {
		return def, err
	}

	if buf.Len() == 0 {
		return def, nil
	}

	if err = yaml.NewDecoder(buf).Decode(def); err != nil {
		return def, err
	}

	return def, nil
}

func (md *ModuleDownloader) fetchModuleReleaseMetadata(img crv1.Image) (ModuleReleaseMetadata, error) {
	var meta ModuleReleaseMetadata

	rc, err := cr.Extract(img)
	if err != nil {
		return meta, fmt.Errorf("extract: %w", err)
	}
	defer rc.Close()

	rr := &releaseReader{
		versionReader:   bytes.NewBuffer(nil),
		changelogReader: bytes.NewBuffer(nil),
		moduleReader:    bytes.NewBuffer(nil),
	}

	if err = rr.untarMetadata(rc); err != nil {
		return meta, fmt.Errorf("untar metadata: %w", err)
	}

	if rr.versionReader.Len() > 0 {
		err = json.NewDecoder(rr.versionReader).Decode(&meta)
		if err != nil {
			return meta, fmt.Errorf("json decode: %w", err)
		}
	}

	if rr.changelogReader.Len() > 0 {
		var changelog map[string]any
		err = yaml.NewDecoder(rr.changelogReader).Decode(&changelog)
		if err != nil {
			meta.Changelog = make(map[string]any)
			return meta, nil
		}
		meta.Changelog = changelog
	}

	if rr.moduleReader.Len() > 0 {
		var ModuleDefinition moduletypes.Definition
		err = yaml.NewDecoder(rr.moduleReader).Decode(&ModuleDefinition)
		if err != nil {
			meta.ModuleDefinition = nil
			return meta, nil
		}

		meta.ModuleDefinition = &ModuleDefinition
	}

	return meta, err
}

func untarModuleDefinition(rc io.ReadCloser, rw io.Writer) error {
	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of archive
			return nil
		}
		if err != nil {
			return err
		}
		if strings.HasPrefix(hdr.Name, ".werf") {
			continue
		}

		switch hdr.Name {
		case "module.yaml":
			_, err = io.Copy(rw, tr)
			if err != nil {
				return err
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

type ModuleReleaseMetadata struct {
	Version string `json:"version"`

	Changelog        map[string]any          `json:"-"`
	ModuleDefinition *moduletypes.Definition `json:"module,omitempty"`
}

const (
	metricUpdatingFailedGroup = "d8_updating_is_failed"
	serviceName               = "check-release"
	ltsChannelName            = "lts"
)

type ImageInfo struct {
	Metadata *ModuleReleaseMetadata
	Image    crv1.Image
	Digest   crv1.Hash
}

func (md *ModuleDownloader) GetNewImageInfo(ctx context.Context, moduleName, moduleVersion string) (*ImageInfo, error) {
	_, span := otel.Tracer(serviceName).Start(ctx, "getNewImageInfo")
	defer span.End()

	regCli, err := md.dc.GetRegistryClient(path.Join(md.ms.Spec.Registry.Repo, moduleName, "release"), md.registryOptions...)
	if err != nil {
		return nil, fmt.Errorf("fetch release image error: %v", err)
	}

	image, err := regCli.Image(context.TODO(), moduleVersion)
	if err != nil {
		return nil, fmt.Errorf("fetch image error: %v", err)
	}

	imageDigest, err := image.Digest()
	if err != nil {
		return nil, fmt.Errorf("get image digest: %w", err)
	}

	releaseMeta, err := md.fetchModuleReleaseMetadata(image)
	if err != nil {
		return nil, fmt.Errorf("fetch image metadata: %w", err)
	}

	if releaseMeta.Version == "" {
		return nil, fmt.Errorf("version not found, probably image is broken or layer does not exist")
	}

	return &ImageInfo{
		Image:    image,
		Digest:   imageDigest,
		Metadata: &releaseMeta,
	}, nil
}
