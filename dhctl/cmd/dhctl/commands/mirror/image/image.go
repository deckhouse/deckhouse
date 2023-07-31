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
	case FileTransport:
		return os.RemoveAll(filepath.Join(i.Path(), util.AddTarGzExt(i.Tag())))
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

	switch i.RegistryTransport() {
	case DockerTransport:
		imageBuilder.WriteString(i.Path())
		if i.tag != "" && i.digest == "" {
			imageBuilder.WriteByte(':')
			imageBuilder.WriteString(i.Tag())
		}

	case FileTransport, directoryTransport:
		r := i.Path()
		if err := os.MkdirAll(r, 0o755); err != nil {
			return nil, err
		}
		image := filepath.Join(r, i.Tag())
		if err := i.extractImageFromFileRegistry(image, isSource, dryRun); err != nil {
			return nil, err
		}
		imageBuilder.WriteString(image)
	}

	if i.digest != "" {
		imageBuilder.WriteByte('@')
		imageBuilder.WriteString(i.digest)
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

func (i *ImageConfig) extractImageFromFileRegistry(p string, isSource, dryRun bool) error {
	if !isSource || dryRun {
		return nil
	}

	fileInArchive := util.AddTarGzExt(filepath.Join(i.additionalPath, i.Tag()))
	return util.NewTarGzReader(util.AddTarGzExt(i.RegistryPath()), func(h *tar.Header, r *tar.Reader) (bool, error) {
		if h.Name != fileInArchive {
			return false, nil
		}
		return true, util.MkFile(util.AddTarGzExt(p), r, h.FileInfo())
	})
}
