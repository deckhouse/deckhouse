// Copyright 2024 Flant JSC
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

package pkgproxy

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/log"
	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
)

type Client struct {
	unpackedImagesPath string
}

func NewClient(unpackedImagesPath string) *Client {
	return &Client{
		unpackedImagesPath: unpackedImagesPath,
	}
}

func (c *Client) GetPackage(ctx context.Context, log log.Logger, _ *registry.ClientConfig, digest string, path string) (int64, string, io.ReadCloser, error) {
	layoutPath := c.unpackedImagesPath
	if path != "" {
		layoutPath = filepath.Join(c.unpackedImagesPath, path)
	}

	layout, err := layout.FromPath(layoutPath)
	if err != nil {
		return 0, "", nil, fmt.Errorf("error creating layout from path: %w", err)
	}

	index, err := layout.ImageIndex()
	if err != nil {
		return 0, "", nil, fmt.Errorf("error getting image index: %w", err)
	}

	hash, err := v1.NewHash(digest)
	if err != nil {
		return 0, "", nil, fmt.Errorf("error parsing image digest: %w", err)
	}

	image, err := index.Image(hash)
	if err != nil {
		return 0, "", nil, fmt.Errorf("error getting image by image digest: %w", err)
	}

	layers, err := image.Layers()
	if err != nil {
		return 0, "", nil, err
	}

	size, err := layers[len(layers)-1].Size()
	if err != nil {
		return 0, "", nil, err
	}

	digestHash, err := layers[len(layers)-1].Digest()
	if err != nil {
		return 0, "", nil, err
	}

	reader, err := layers[len(layers)-1].Compressed()
	if err != nil {
		return 0, "", nil, err
	}

	return size, digestHash.Hex, reader, nil
}
