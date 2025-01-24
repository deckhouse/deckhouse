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
	"slices"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"k8s.io/klog"

	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
)

func (s *registryscaner) processRegistries(ctx context.Context) {
	klog.V(4).Info("start scanning registries")
	for _, registry := range s.registryClients {
		modules, err := registry.Modules()
		if err != nil {
			klog.Errorf("registry %v is unavailable. err: %v", registry.Name(), err)
			continue
		}
		klog.V(4).Infof("found modules: %v in %q", modules, registry.Name())

		s.processModules(ctx, registry, modules)
	}
}

func (s *registryscaner) processModules(ctx context.Context, registry Client, modules []string) {
	for _, module := range modules {
		tags, err := registry.ListTags(module)
		if err != nil {
			klog.Error(err)
			continue
		}

		s.processReleaseChannels(ctx, registry.Name(), module, filterReleaseChannelsFromTags(tags))
	}

	// remove deleted modules from cache
	for _, m := range s.cache.GetModules(registry.Name()) {
		if !slices.Contains(modules, m) {
			s.cache.DeleteModule(registry.Name(), m)
		}
	}
}

func (s *registryscaner) processReleaseChannels(ctx context.Context, registry, module string, releaseChannels []string) {
	for _, releaseChannel := range releaseChannels {
		releaseImage, err := s.registryClients[registry].ReleaseImage(module, releaseChannel)
		if err != nil {
			klog.Error(err)
			continue
		}

		// if the checksum for the release channel matches - skip processing of the release channel
		releaseDigest, err := releaseImage.Digest()
		if err != nil {
			klog.Error(err)
			continue
		}
		releaseChecksum, ok := s.cache.GetReleaseChecksum(registry, module, releaseChannel)
		if ok && releaseChecksum == releaseDigest.String() {
			continue
		}

		version, err := extractVersionFromImage(releaseImage)
		if err != nil {
			klog.Error(err)
			continue
		}

		if err := s.processVersion(ctx, registry, module, version, releaseChannel); err == nil {
			s.cache.SetReleaseChecksum(registry, module, releaseChannel, releaseDigest.String())
		}
	}
}

func (s *registryscaner) processVersion(_ context.Context, registry, module, version, releaseChannel string) error {
	image, err := s.registryClients[registry].Image(module, version)
	if err != nil {
		klog.Error(err)
		return err
	}

	tarFile, err := extractDocumentation(image)
	if err != nil {
		klog.Error(err)
		return err
	}

	s.cache.SetTar(registry, module, version, releaseChannel, tarFile)

	return nil
}

func extractDocumentation(image v1.Image) ([]byte, error) {
	readCloser, err := cr.Extract(image)
	if err != nil {
		return nil, err
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
		klog.Error(err)
		return nil, err
	}

	// "openapi" directory
	err = tarWriter.WriteHeader(&tar.Header{
		Typeflag: tar.TypeDir,
		Name:     "openapi",
		Mode:     0700,
	})
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	// "crds" directory
	err = tarWriter.WriteHeader(&tar.Header{
		Typeflag: tar.TypeDir,
		Name:     "crds",
		Mode:     0700,
	})
	if err != nil {
		klog.Error(err)
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

		if strings.Contains(hdr.Name, "docs/") {
			buf := bytes.NewBuffer(nil)
			if _, err := io.Copy(buf, tarReader); err != nil {
				klog.Error(err)
			}

			if err := tarWriter.WriteHeader(hdr); err != nil {
				klog.Error(err)
			}

			if _, err := tarWriter.Write(buf.Bytes()); err != nil {
				klog.Error(err)
			}
		}

		if strings.Contains(hdr.Name, "openapi/") {
			buf := bytes.NewBuffer(nil)
			if _, err := io.Copy(buf, tarReader); err != nil {
				klog.Error(err)
			}

			if err := tarWriter.WriteHeader(hdr); err != nil {
				klog.Error(err)
			}

			if _, err := tarWriter.Write(buf.Bytes()); err != nil {
				klog.Error(err)
			}
		}

		if strings.Contains(hdr.Name, "crds/") {
			buf := bytes.NewBuffer(nil)
			if _, err := io.Copy(buf, tarReader); err != nil {
				klog.Error(err)
			}

			if err := tarWriter.WriteHeader(hdr); err != nil {
				klog.Error(err)
			}

			if _, err := tarWriter.Write(buf.Bytes()); err != nil {
				klog.Error(err)
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
