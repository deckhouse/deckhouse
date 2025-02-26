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
		modules, err := registry.Modules()
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
		tags, err := registry.ListTags(module)
		if err != nil {
			s.logger.Error("list tags", log.Err(err))
			continue
		}

		s.processReleaseChannels(ctx, registry.Name(), module, filterReleaseChannelsFromTags(tags))
	}
}

func (s *registryscaner) processReleaseChannels(ctx context.Context, registry, module string, releaseChannels []string) {
	vers := make([]internal.VersionData, 0, len(releaseChannels))

	for _, releaseChannel := range releaseChannels {
		releaseImage, err := s.registryClients[registry].ReleaseImage(module, releaseChannel)
		if err != nil {
			s.logger.Error("get releae image", log.Err(err))
			continue
		}

		// if the checksum for the release channel matches - skip processing of the release channel
		releaseDigest, err := releaseImage.Digest()
		if err != nil {
			s.logger.Error("get digest", log.Err(err))
			continue
		}

		ver := internal.VersionData{
			Registry:       registry,
			ModuleName:     module,
			ReleaseChannel: releaseChannel,
			Checksum:       releaseDigest.String(),
			Version:        "",
			TarFile:        make([]byte, 0),
			TarLen:         0,
			Image:          releaseImage,
		}

		releaseChecksum, ok := s.cache.GetReleaseChecksum(ver)
		if ok && releaseChecksum == ver.Checksum {
			version, tarFile, ok := s.cache.GetReleaseVersionData(ver)
			if ok {
				ver.Version = version
				ver.TarFile = tarFile

				vers = append(vers, ver)
			}

			continue
		}

		version, err := extractVersionFromImage(ver.Image)
		if err != nil {
			s.logger.Error("extract version from image", log.Err(err))
			continue
		}

		ver.Version = version

		tarFile, err := s.extractTar(ver)
		if err != nil {
			s.logger.Error("extract tar", log.Err(err))
			continue
		}

		ver.TarFile = tarFile

		vers = append(vers, ver)
	}

	for _, ver := range vers {
		s.cache.SetTar(ver)
		s.cache.SetReleaseChecksum(ver)
	}
}

// cache is populated
// 1.2.3 was not removed and remains in cache, form a list for deletion
// 1.2.4 was removed from cache and taken into the list (already existed)
// 1.2.5 was removed from cache and taken into the list (already existed)
// versions arrived
// 1.2.4
// 1.2.5
// 1.2.6 was not found and added to the list (needs to be added)
func (s *registryscaner) extractTar(version internal.VersionData) ([]byte, error) {
	image, err := s.registryClients[version.Registry].Image(version.ModuleName, version.Version)
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
	tarReader := tar.NewReader(readCloser)

	// "docs" directory
	err = tarWriter.WriteHeader(&tar.Header{
		Typeflag: tar.TypeDir,
		Name:     "docs",
		Mode:     0700,
	})
	if err != nil {
		s.logger.Error("write header", log.Err(err))
		return nil, err
	}

	// "openapi" directory
	err = tarWriter.WriteHeader(&tar.Header{
		Typeflag: tar.TypeDir,
		Name:     "openapi",
		Mode:     0700,
	})
	if err != nil {
		s.logger.Error("write header", log.Err(err))
		return nil, err
	}

	// "crds" directory
	err = tarWriter.WriteHeader(&tar.Header{
		Typeflag: tar.TypeDir,
		Name:     "crds",
		Mode:     0700,
	})
	if err != nil {
		s.logger.Error("write header", log.Err(err))
		return nil, err
	}

	for {
		hdr, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				return tarFile.Bytes(), nil
			}

			return nil, err
		}

		// TODO: short duplicate
		if strings.Contains(hdr.Name, "docs/") {
			buf := bytes.NewBuffer(nil)
			if _, err := io.Copy(buf, tarReader); err != nil {
				s.logger.Error("copy", log.Err(err))
			}

			if err := tarWriter.WriteHeader(hdr); err != nil {
				s.logger.Error("write header", log.Err(err))
			}

			if _, err := tarWriter.Write(buf.Bytes()); err != nil {
				s.logger.Error("write", log.Err(err))
			}
		}

		if strings.Contains(hdr.Name, "openapi/") {
			buf := bytes.NewBuffer(nil)
			if _, err := io.Copy(buf, tarReader); err != nil {
				s.logger.Error("copy", log.Err(err))
			}

			if err := tarWriter.WriteHeader(hdr); err != nil {
				s.logger.Error("write header", log.Err(err))
			}

			if _, err := tarWriter.Write(buf.Bytes()); err != nil {
				s.logger.Error("write", log.Err(err))
			}
		}

		if strings.Contains(hdr.Name, "crds/") {
			buf := bytes.NewBuffer(nil)
			if _, err := io.Copy(buf, tarReader); err != nil {
				s.logger.Error("copy", log.Err(err))
			}

			if err := tarWriter.WriteHeader(hdr); err != nil {
				s.logger.Error("write header", log.Err(err))
			}

			if _, err := tarWriter.Write(buf.Bytes()); err != nil {
				s.logger.Error("write", log.Err(err))
			}
		}
	}
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
