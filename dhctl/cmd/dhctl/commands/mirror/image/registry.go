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
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/containers/image/v5/directory"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/types"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/image/transport"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/util"
)

var (
	DockerTransport            = docker.Transport.Name()
	fileTransport              = transport.Transport.Name()
	directoryTransport         = directory.Transport.Name()
	ErrNoSuchRegistryTransport = errors.New("no such transport for images. should be 'file:path', 'docker://docker-reference' or 'dir:path'")
	ErrDirNotImplemented       = errors.New("dir transport: list tags not implemented")
)

type RegistryConfig struct {
	path       string
	transport  string
	authConfig *types.DockerAuthConfig
}

func MustNewRegistry(registryPath string, dockerCfg *types.DockerAuthConfig) *RegistryConfig {
	r, err := NewRegistry(registryPath, dockerCfg)
	if err != nil {
		panic(err)
	}
	return r
}

func NewRegistry(registryPath string, dockerCfg *types.DockerAuthConfig) (*RegistryConfig, error) {
	transportName, withinTransport, f := strings.Cut(util.TrimTarGzExt(registryPath), ":")
	if !f {
		return nil, fmt.Errorf("can't find transport for '%s'", registryPath)
	}

	if transportName != DockerTransport && transportName != fileTransport && transportName != directoryTransport {
		return nil, ErrNoSuchRegistryTransport
	}

	return &RegistryConfig{
		path:       withinTransport,
		transport:  transportName,
		authConfig: dockerCfg,
	}, nil
}

func (r *RegistryConfig) Commit() error {
	switch r.Transport() {
	case fileTransport:
		return util.CompressDir(util.TrimTarGzExt(r.Path()), true)
	}
	return nil
}

func (r *RegistryConfig) Close() error {
	switch r.Transport() {
	case fileTransport:
		return os.RemoveAll(util.TrimTarGzExt(r.Path()))
	}
	return nil
}

func (r *RegistryConfig) copy() *RegistryConfig {
	n := new(RegistryConfig)
	*n = *r
	return n
}

func (r *RegistryConfig) Path() string {
	return r.path
}

func (r *RegistryConfig) Transport() string {
	return r.transport
}

func (r *RegistryConfig) AuthConfig() *types.DockerAuthConfig {
	return r.authConfig
}

func (r *RegistryConfig) WithAuthConfig(cfg *types.DockerAuthConfig) *RegistryConfig {
	n := r.copy()
	n.authConfig = cfg
	return n
}

func (r *RegistryConfig) ListTags(ctx context.Context, opts ...ListOption) ([]string, error) {
	switch r.Transport() {
	case DockerTransport:
		return listDockerTags(ctx, r, opts...)
	case fileTransport:
		return listFileTags(r.Path())
	case directoryTransport:
		return nil, ErrDirNotImplemented
	}
	return nil, ErrNoSuchRegistryTransport
}

func listDockerTags(ctx context.Context, r *RegistryConfig, opts ...ListOption) ([]string, error) {
	imgRef, err := NewImageConfig(r, "", "").imageReference(false, true)
	if err != nil {
		return nil, err
	}

	listOpts := &listOptions{}
	opts = append(opts, withAuth(r.AuthConfig()))
	for _, opt := range opts {
		opt(listOpts)
	}
	return docker.GetRepositoryTags(ctx, listOpts.sysCtx, imgRef)
}

func listFileTags(p string) ([]string, error) {
	uniqueTags := make(map[string]bool, 0)
	separator := string(filepath.Separator)
	err := util.NewTarGzReader(util.AddTarGzExt(p), func(h *tar.Header, r *tar.Reader) (bool, error) {
		splitted := strings.Split(strings.TrimPrefix(h.Name, separator), separator)
		if len(splitted) > 0 && util.HasTarGzSuffix(splitted[0]) {
			uniqueTags[util.TrimTarGzExt(splitted[0])] = true
		}
		return false, nil
	})
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}

	tags := make([]string, 0, len(uniqueTags))
	for tag := range uniqueTags {
		tags = append(tags, tag)
	}

	sort.Strings(tags)
	return tags, nil
}
