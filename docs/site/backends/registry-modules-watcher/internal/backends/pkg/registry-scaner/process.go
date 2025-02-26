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

package registryscaner

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"registry-modules-watcher/internal"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/module-sdk/pkg/dependency/cr"
)

func (s *registryscaner) processRegistries(ctx context.Context) {
	s.logger.Info("start scanning registries")

	for _, registry := range s.registryClients {
		modules, err := registry.Modules(ctx)
		if err != nil {
			s.logger.Error("registry is unavailable", slog.String("registry", registry.Name()), log.Err(err))
			continue
		}

		s.logger.Info("found modules", slog.Any("modules", modules), slog.String("registry", registry.Name()))

		s.processModules(ctx, registry, modules)
	}
}

func (s *registryscaner) processModules(ctx context.Context, registry Client, modules []string) {
	for _, module := range modules {
		tags, err := registry.ListTags(ctx, module)
		if err != nil {
			s.logger.Error("list tags", log.Err(err))
			continue
		}

		s.processReleaseChannels(ctx, registry.Name(), module, filterReleaseChannelsFromTags(tags))
	}
}

func (s *registryscaner) processReleaseChannels(ctx context.Context, registry, module string, releaseChannels []string) {
	versions := make([]internal.VersionData, 0, len(releaseChannels))

	for _, releaseChannel := range releaseChannels {
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

	// Update cache with processed versions
	for _, ver := range versions {
		s.cache.SetTar(ver)
		s.cache.SetReleaseChecksum(ver)
	}
}

func (s *registryscaner) processReleaseChannel(ctx context.Context, registry, module, releaseChannel string) (*internal.VersionData, error) {
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
		TarLen:         0,
		Image:          releaseImage,
	}

	// Check if we already have this release in cache
	releaseChecksum, ok := s.cache.GetReleaseChecksum(*versionData)
	if ok && releaseChecksum == versionData.Checksum {
		version, tarFile, ok := s.cache.GetReleaseVersionData(*versionData)
		if ok {
			versionData.Version = version
			versionData.TarFile = tarFile
			return versionData, nil
		}
	}

	// Extract version from image
	version, err := extractVersionFromImage(versionData.Image)
	if err != nil {
		return nil, fmt.Errorf("extract version from image: %w", err)
	}
	versionData.Version = version

	// Extract tar file
	tarFile, err := s.extractTar(ctx, *versionData)
	if err != nil {
		return nil, fmt.Errorf("extract tar: %w", err)
	}
	versionData.TarFile = tarFile

	return versionData, nil
}

// cache is populated
// 1.2.3 was not removed and remains in cache, form a list for deletion
// 1.2.4 was removed from cache and taken into the list (already existed)
// 1.2.5 was removed from cache and taken into the list (already existed)
// versions arrived
// 1.2.4
// 1.2.5
// 1.2.6 was not found and added to the list (needs to be added)
func (s *registryscaner) extractTar(ctx context.Context, version internal.VersionData) ([]byte, error) {
	image, err := s.registryClients[version.Registry].Image(ctx, version.ModuleName, version.Version)
	if err != nil {
		s.logger.Error("get image", log.Err(err))
		return nil, err
	}

	tarFile, err := s.extractDocumentation(image)
	if err != nil {
		s.logger.Error("extract documentation", log.Err(err))
		return nil, err
	}

	return tarFile, nil
}
func (s *registryscaner) extractDocumentation(image v1.Image) ([]byte, error) {
	readCloser, err := cr.Extract(image)
	if err != nil {
		return nil, fmt.Errorf("extract: %w", err)
	}
	defer readCloser.Close()

	tarFile := bytes.NewBuffer(nil)
	tarWriter := tar.NewWriter(tarFile)
	defer tarWriter.Close()
	tarReader := tar.NewReader(readCloser)

	// Create directories
	dirs := []string{"docs", "openapi", "crds"}
	for _, dir := range dirs {
		if err := tarWriter.WriteHeader(&tar.Header{
			Typeflag: tar.TypeDir,
			Name:     dir,
			Mode:     0700,
		}); err != nil {
			return nil, fmt.Errorf("write directory header for %s: %w", dir, err)
		}
	}

	// Copy files from source tar to destination tar
	for {
		hdr, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		// Check if file belongs to one of our target directories
		shouldCopy := false
		for _, dir := range dirs {
			if strings.Contains(hdr.Name, dir+"/") {
				shouldCopy = true
				break
			}
		}

		if shouldCopy {
			buf := bytes.NewBuffer(nil)
			if _, err := io.Copy(buf, tarReader); err != nil {
				return nil, fmt.Errorf("copy file content: %w", err)
			}

			if err := tarWriter.WriteHeader(hdr); err != nil {
				return nil, fmt.Errorf("write file header: %w", err)
			}

			if _, err := tarWriter.Write(buf.Bytes()); err != nil {
				return nil, fmt.Errorf("write file content: %w", err)
			}
		}
	}

	return tarFile.Bytes(), nil
}

func extractVersionFromImage(releaseImage v1.Image) (string, error) {
	// exactly local type
	type versionJSON struct {
		Version string `json:"version"`
	}

	readCloser, err := cr.Extract(releaseImage)
	if err != nil {
		return "", err
	}
	defer readCloser.Close()

	tarReader := tar.NewReader(readCloser)
	for {
		hdr, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				return "", fmt.Errorf("version is not set")
			}

			return "", err
		}

		if hdr.Typeflag == tar.TypeReg && hdr.Name == "version.json" {
			buf := bytes.NewBuffer(nil)
			if _, err = io.Copy(buf, tarReader); err != nil {
				return "", err
			}

			v := versionJSON{}
			if err := json.Unmarshal(buf.Bytes(), &v); err != nil {
				return "", err
			}

			if v.Version != "" {
				return v.Version, nil
			}
		}
	}
}

func filterReleaseChannelsFromTags(tags []string) []string {
	releaseChannels := make([]string, 0)
	for _, tag := range tags {
		if _, ok := releaseChannelsTags[tag]; ok {
			releaseChannels = append(releaseChannels, tag)
		}
	}
	return releaseChannels
}
