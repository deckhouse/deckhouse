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
	"fmt"
	"io/fs"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

func readFileFromImage(img v1.Image, fileName string) (*bytes.Buffer, error) {
	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("get image layers: %w", err)
	}

	for _, layer := range layers {
		decompressedLayer, err := layer.Uncompressed()
		if err != nil {
			return nil, fmt.Errorf("decompress layer: %w", err)
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
