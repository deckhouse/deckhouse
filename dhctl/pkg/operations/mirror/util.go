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

package mirror

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io/fs"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
)

func readFileFromImage(img v1.Image, fileName string) (*bytes.Buffer, error) {
	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("get image layers: %w", err)
	}

	for _, layer := range layers {
		// Do not use layer.Uncompressed() here.
		// We decompress layers ourselves because decompressor that is built into go-containerregistry is bugged and sometimes returns closed streams.
		gzipLayer, err := layer.Compressed()
		if err != nil {
			return nil, fmt.Errorf("read layer: %w", err)
		}

		decompressedLayer, err := gzip.NewReader(gzipLayer)
		if err != nil {
			return nil, fmt.Errorf("unzip layer: %w", err)
		}

		tr := tar.NewReader(decompressedLayer)
		for {
			hdr, err := tr.Next()
			if err != nil {
				_ = decompressedLayer.Close()
				break
			}
			if hdr.Name != fileName {
				_ = decompressedLayer.Close()
				continue
			}

			buf := &bytes.Buffer{}
			if _, err = buf.ReadFrom(tr); err != nil {
				return nil, fmt.Errorf("buffer file data from layer: %w", err)
			}

			return buf, nil
		}
	}

	return nil, fmt.Errorf("%s: %w", fileName, fs.ErrNotExist)
}

func getImageFromLayoutByTag(l layout.Path, tag string) (v1.Image, error) {
	index, err := l.ImageIndex()
	if err != nil {
		return nil, err
	}
	indexManifest, err := index.IndexManifest()
	if err != nil {
		return nil, err
	}

	for _, imageManifest := range indexManifest.Manifests {
		for key, value := range imageManifest.Annotations {
			if key == "org.opencontainers.image.ref.name" && strings.HasSuffix(value, ":"+tag) {
				return index.Image(imageManifest.Digest)
			}
		}
	}

	return nil, nil
}
