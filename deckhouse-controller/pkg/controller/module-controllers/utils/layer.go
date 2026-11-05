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

package utils // nolint:revive

import (
	"archive/tar"
	"bytes"
	"io"
	"maps"
	"slices"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type FakeLayer struct {
	FilesContent map[string]string // pair: filename - file content
}

func (fl FakeLayer) Uncompressed() (io.ReadCloser, error) {
	result := bytes.NewBuffer(nil)
	if fl.FilesContent == nil {
		fl.FilesContent = make(map[string]string)
	}

	if len(fl.FilesContent) == 0 {
		return io.NopCloser(result), nil
	}

	wr := tar.NewWriter(result)

	// create files in a single layer, in the same order every time to keep the digest
	for _, filename := range slices.Sorted(maps.Keys(fl.FilesContent)) {
		content := fl.FilesContent[filename]
		if strings.Contains(filename, "/") {
			dirs := strings.Split(filename, "/")
			for i := 0; i < len(dirs)-1; i++ {
				hdr := &tar.Header{
					Name:     dirs[i],
					Typeflag: tar.TypeDir,
					Mode:     0777,
				}
				_ = wr.WriteHeader(hdr)
			}
		}

		hdr := &tar.Header{
			Name:     filename,
			Typeflag: tar.TypeReg,
			Mode:     0600,
			Size:     int64(len(content)),
		}
		_ = wr.WriteHeader(hdr)
		_, _ = wr.Write([]byte(content))
	}
	_ = wr.Close()

	return io.NopCloser(result), nil
}

// layer serves the content as an uncompressed layer, so an image of fake layers
// can be copied like an image from a registry
func (fl FakeLayer) layer() v1.Layer {
	rc, _ := fl.Uncompressed()
	content, _ := io.ReadAll(rc)

	return static.NewLayer(content, types.OCIUncompressedLayer)
}

func (fl FakeLayer) Digest() (v1.Hash, error) {
	return fl.layer().Digest()
}

func (fl FakeLayer) DiffID() (v1.Hash, error) {
	return fl.layer().DiffID()
}

func (fl FakeLayer) Compressed() (io.ReadCloser, error) {
	return fl.layer().Compressed()
}

func (fl FakeLayer) Size() (int64, error) {
	return fl.layer().Size()
}

func (fl FakeLayer) MediaType() (types.MediaType, error) {
	return fl.layer().MediaType()
}
