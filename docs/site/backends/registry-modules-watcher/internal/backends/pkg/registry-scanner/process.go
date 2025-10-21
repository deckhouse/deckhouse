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

package registryscanner

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	crv1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/module-sdk/pkg/dependency/cr"

	"registry-modules-watcher/internal"
	"registry-modules-watcher/internal/backends"
	"registry-modules-watcher/internal/metrics"
)

const (
	ImageAnnotationSignature = "io.deckhouse.delivery-kit.signature"
)

// Constants for directory structure
var (
	documentationDirs = []string{"docs", "openapi", "openapi/conversions", "crds"}
)

const versionFileName = "version.json"

type ImageMetadata struct {
	Version               string
	ModuleDefinitionFound bool
}

func (s *registryscanner) processRegistries(ctx context.Context) []backends.DocumentationTask {
	s.logger.Info("start scanning registries")

	versions := make([]internal.VersionData, 0, len(s.registryClients))

	for _, registry := range s.registryClients {
		modules, err := registry.Modules(ctx)
		if err != nil {
			s.logger.Error("registry is unavailable",
				slog.String("registry", registry.Name()),
				log.Err(err))
			continue
		}

		s.logger.Debug("found modules",
			slog.Any("modules", modules),
			slog.String("registry", registry.Name()))

		vers := s.processModules(ctx, registry, modules)
		versions = append(versions, vers...)
	}

	return s.cache.SyncWithRegistryVersions(versions)
}

func (s *registryscanner) processModules(ctx context.Context, registry Client, modules []string) []internal.VersionData {
	versions := make([]internal.VersionData, 0, len(modules))

	for _, module := range modules {
		s.logger.Info("processing module", slog.String("module", module))

		tags, err := registry.ListTags(ctx, module)
		if err != nil {
			s.logger.Error("failed to list tags",
				slog.String("module", module),
				slog.String("registry", registry.Name()),
				log.Err(err))
			continue
		}

		releaseChannels := getReleaseChannelsFromTags(tags)
		vers := s.processReleaseChannels(ctx, registry.Name(), module, releaseChannels)
		versions = append(versions, vers...)
	}

	return versions
}

func (s *registryscanner) processReleaseChannels(ctx context.Context, registry, module string, releaseChannels []string) []internal.VersionData {
	versions := make([]internal.VersionData, 0, len(releaseChannels))

	for _, releaseChannel := range releaseChannels {
		s.logger.Info("processing channel", slog.String("module", module), slog.String("channel", releaseChannel))

		versionData, err := s.processReleaseChannel(ctx, registry, module, releaseChannel)
		if err != nil {
			s.logger.Error("failed to process release channel",
				slog.String("registry", registry),
				slog.String("module", module),
				slog.String("channel", releaseChannel),
				log.Err(err))
			continue
		}

		if versionData != nil {
			versions = append(versions, *versionData)
		}
	}

	return versions
}

func (s *registryscanner) processReleaseChannel(ctx context.Context, registry, module, releaseChannel string) (*internal.VersionData, error) {
	releaseImage, err := s.registryClients[registry].ReleaseImage(ctx, module, releaseChannel)
	if err != nil {
		return nil, fmt.Errorf("get release image: %w", err)
	}

	releaseDigest, err := releaseImage.Digest()
	if err != nil {
		return nil, fmt.Errorf("get digest: %w", err)
	}

	versionData := &internal.VersionData{
		Registry:       registry,
		ModuleName:     module,
		ReleaseChannel: releaseChannel,
		Checksum:       releaseDigest.String(),
		Version:        "",
		TarFile:        make([]byte, 0),
		Image:          releaseImage,
	}

	// Search across all channels by checksum
	version, tarFile := s.cache.GetGetReleaseVersionData(versionData)
	if version != "" {
		versionData.Version = version
		versionData.TarFile = tarFile

		return versionData, nil
	}

	manifest, err := releaseImage.Manifest()
	if err != nil {
		return nil, fmt.Errorf("get manifest: %w", err)
	}

	// manifest must exist
	if manifest == nil {
		return nil, fmt.Errorf("manifest is nil")
	}

	// check that the module sign annotation exists
	if len(manifest.Annotations) == 0 || manifest.Annotations[ImageAnnotationSignature] == "" {
		s.ms.GaugeSet(metrics.RegistryScannerNoModuleSign, 1.0, map[string]string{"module": module})
	}

	// Extract version from image
	imageMeta, err := getMetadataFromImage(versionData.Image)
	if err != nil {
		return nil, fmt.Errorf("extract version from image: %w", err)
	}
	s.logger.Debug("module.yaml state",
		slog.String("module", module),
		slog.String("channel", releaseChannel),
		slog.Bool("found", imageMeta.ModuleDefinitionFound))
	if !imageMeta.ModuleDefinitionFound {
		s.ms.GaugeSet(metrics.RegistryScannerNoModuleYamlMetric, 1.0, map[string]string{"module": module})
	}

	versionData.Version = imageMeta.Version

	// Extract tar file
	tarFile, err = s.extractTar(ctx, versionData)
	if err != nil {
		return nil, fmt.Errorf("extract tar: %w", err)
	}
	versionData.TarFile = tarFile

	return versionData, nil
}

func (s *registryscanner) extractTar(ctx context.Context, version *internal.VersionData) ([]byte, error) {
	// Before pull
	timeBeforePull := time.Now()
	image, err := s.registryClients[version.Registry].Image(ctx, version.ModuleName, version.Version)
	if err != nil {
		return nil, fmt.Errorf("get image: %w", err)
	}

	// Pull and untar
	tarFile, err := s.extractDocumentation(image)
	if err != nil {
		return nil, fmt.Errorf("extract documentation: %w", err)
	}

	// Calculate pull time metrics
	pullTime := time.Since(timeBeforePull).Seconds()
	s.ms.HistogramObserve(metrics.RegistryPullSecondsMetric, pullTime, nil, nil)

	return tarFile, nil
}

func (s *registryscanner) extractDocumentation(image crv1.Image) ([]byte, error) {
	readCloser, err := cr.Extract(image)
	if err != nil {
		return nil, fmt.Errorf("extract: %w", err)
	}
	defer readCloser.Close()

	tarFile := bytes.NewBuffer(nil)
	tarWriter := tar.NewWriter(tarFile)
	defer tarWriter.Close()

	// Create directories structure
	if err := createDocumentationDirectoryStructure(tarWriter); err != nil {
		return nil, err
	}

	// Copy relevant files from source tar to destination tar
	if err := s.copyDocumentationFiles(readCloser, tarWriter); err != nil {
		return nil, err
	}

	return tarFile.Bytes(), nil
}

func createDocumentationDirectoryStructure(tarWriter *tar.Writer) error {
	for _, dir := range documentationDirs {
		if err := tarWriter.WriteHeader(&tar.Header{
			Typeflag: tar.TypeDir,
			Name:     dir,
			Mode:     0700,
		}); err != nil {
			return fmt.Errorf("write directory header for %s: %w", dir, err)
		}
	}
	return nil
}

func (s *registryscanner) copyDocumentationFiles(source io.Reader, tarWriter *tar.Writer) error {
	tarReader := tar.NewReader(source)

	for {
		hdr, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("tar reader next: %w", err)
		}

		if isDocumentationFile(hdr.Name) {
			buf := bytes.NewBuffer(nil)
			if _, err = io.Copy(buf, tarReader); err != nil {
				return fmt.Errorf("copy file content: %w", err)
			}

			if err = tarWriter.WriteHeader(hdr); err != nil {
				return fmt.Errorf("write file header: %w", err)
			}

			if _, err = tarWriter.Write(buf.Bytes()); err != nil {
				return fmt.Errorf("write file content: %w", err)
			}

			s.logger.Debug("copied file", slog.String("file", hdr.Name))
		}
	}

	return nil
}

func isDocumentationFile(filename string) bool {
	if filepath.Ext(filename) == ".go" {
		return false
	}

	for _, dir := range documentationDirs {
		if strings.Contains(filename, dir+"/") {
			return true
		}
	}
	return false
}

func getMetadataFromImage(releaseImage crv1.Image) (*ImageMetadata, error) {
	readCloser, err := cr.Extract(releaseImage)
	if err != nil {
		return nil, fmt.Errorf("extract image: %w", err)
	}
	defer readCloser.Close()

	metadata := &ImageMetadata{}

	tarReader := tar.NewReader(readCloser)
	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			// end of archive
			return metadata, nil
		}
		if err != nil {
			return nil, fmt.Errorf("tar reader next: %w", err)
		}

		switch hdr.Name {
		case versionFileName:
			metadata.Version, err = parseVersionFromTarFile(tarReader)
			if err != nil {
				return nil, err
			}
		case "module.yaml":
			buf := bytes.NewBuffer(nil)
			if _, err := io.Copy(buf, tarReader); err != nil {
				continue // ignore error
			}
			if len(buf.Bytes()) > 0 {
				metadata.ModuleDefinitionFound = true
			}
		}
	}
}

func parseVersionFromTarFile(reader io.Reader) (string, error) {
	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, reader); err != nil {
		return "", fmt.Errorf("copy version file content: %w", err)
	}

	var versionJSON struct {
		Version string `json:"version"`
	}

	if err := json.Unmarshal(buf.Bytes(), &versionJSON); err != nil {
		return "", fmt.Errorf("unmarshal version data: %w", err)
	}

	if versionJSON.Version == "" {
		return "", fmt.Errorf("version field is empty")
	}

	return versionJSON.Version, nil
}

func getReleaseChannelsFromTags(tags []string) []string {
	releaseChannels := make([]string, 0)

	for _, tag := range tags {
		if _, ok := releaseChannelsTags[tag]; ok {
			releaseChannels = append(releaseChannels, tag)
		}
	}

	return releaseChannels
}
