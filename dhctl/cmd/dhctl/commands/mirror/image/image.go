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

package image

import (
	"archive/tar"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	_ "github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/image/transport" // Add transport for tar.gz file
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/util"
)

type ImageConfig struct {
	tag            string
	digest         string
	additionalPath string
	registry       *RegistryConfig
}

func NewImageConfig(registry *RegistryConfig, tag, digest string, additionalPaths ...string) *ImageConfig {
	return &ImageConfig{
		registry:       registry,
		tag:            tag,
		digest:         digest,
		additionalPath: filepath.Join(additionalPaths...),
	}
}

func (i *ImageConfig) close() error {
	switch i.RegistryTransport() {
	case fileTransport:
		return os.RemoveAll(i.resultImageArchive())
	}
	return nil
}

func (i *ImageConfig) copy() *ImageConfig {
	n := new(ImageConfig)
	*n = *i
	return n
}

func (i *ImageConfig) WithNewRegistry(r *RegistryConfig) *ImageConfig {
	n := i.copy()
	n.registry = r
	return n
}

func (i *ImageConfig) Digest() string {
	return i.digest
}

func (i *ImageConfig) WithDigest(d string) *ImageConfig {
	n := i.copy()
	n.digest = d
	return n
}

func (i *ImageConfig) Tag() string {
	return i.tag
}

func (i *ImageConfig) WithTag(t string) *ImageConfig {
	n := i.copy()
	n.tag = t
	return n
}

func (i *ImageConfig) imageReference(isSource, dryRun bool) (types.ImageReference, error) {
	imageBuilder := &strings.Builder{}
	imageBuilder.WriteString(i.RegistryTransport())
	imageBuilder.WriteByte(':')

	registryPath := i.Path()
	switch i.RegistryTransport() {
	case DockerTransport:
		imageBuilder.WriteString(registryPath)
		if i.Tag() != "" && i.Digest() == "" {
			imageBuilder.WriteByte(':')
			imageBuilder.WriteString(i.Tag())
		}

	case fileTransport, directoryTransport:
		if err := os.MkdirAll(registryPath, 0o755); err != nil {
			return nil, err
		}
		imageBuilder.WriteString(filepath.Join(registryPath, i.Tag()))
	}

	if digest := i.Digest(); digest != "" {
		imageBuilder.WriteByte('@')
		imageBuilder.WriteString(digest)
	}
	if i.RegistryTransport() == fileTransport && isSource && !dryRun {
		if err := i.extractImageFromFileRegistry(); err != nil {
			return nil, err
		}
	}

	return alltransports.ParseImageName(imageBuilder.String())
}

func (i *ImageConfig) Path() string {
	r := i.RegistryPath()
	if i.additionalPath == "" {
		return r
	}
	if r == "" {
		return i.additionalPath
	}
	// This is used except of "filepath.Join" because docker transport want registry to start with "//"
	return r + "/" + strings.Trim(i.additionalPath, "/")
}

func (i *ImageConfig) RegistryPath() string {
	if i.registry == nil {
		return ""
	}
	return strings.TrimRight(i.registry.Path(), "/")
}
func (i *ImageConfig) RegistryTransport() string {
	if i.registry == nil {
		return ""
	}
	return i.registry.Transport()
}

func (i *ImageConfig) AuthConfig() *types.DockerAuthConfig {
	if i.registry == nil {
		return nil
	}
	return i.registry.AuthConfig()
}

func (i *ImageConfig) extractImageFromFileRegistry() error {
	fileInArchive, resultFile := filepath.Join("/", i.fileImageInArchive()), i.resultImageArchive()
	targetDir, _ := filepath.Split(fileInArchive)

	err := util.NewTarGzReader(util.AddTarGzExt(i.RegistryPath()), func(h *tar.Header, r *tar.Reader) (bool, error) {
		dir, name := filepath.Split(util.TrimTarGzExt(h.Name))

		tagAndDigest := strings.Split(name, "@")
		tag := tagAndDigest[0]
		var digest string
		if len(tagAndDigest) > 1 {
			digest = tagAndDigest[1]
		}

		if h.Name == fileInArchive || (dir == targetDir && (digest == i.Digest() || (digest == "" && tag == i.Tag()))) {
			return true, util.MkFile(resultFile, r, h.FileInfo())
		}
		return false, nil
	})

	if err != nil {
		return fmt.Errorf("can't find image in file registry: %w", err)
	}
	return nil
}

func (i *ImageConfig) fileImageInArchive() string {
	name := filepath.Join(i.additionalPath, i.Tag())
	if d := i.Digest(); d != "" {
		name += "@" + d
	}
	return util.AddTarGzExt(name)
}

func (i *ImageConfig) resultImageArchive() string {
	return filepath.Join(i.RegistryPath(), i.fileImageInArchive())
}
