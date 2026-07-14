// Copyright 2025 Flant JSC
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

package reconcilertest

import (
	"archive/tar"
	"bytes"
	"io"
	"strings"

	crv1 "github.com/google/go-containerregistry/pkg/v1"
	crfake "github.com/google/go-containerregistry/pkg/v1/fake"
)

// FakeLayer is an in-memory OCI layer whose uncompressed content is a tar archive
// built from FilesContent (filename -> content). It mirrors the long-standing
// helper in module-controllers/utils so registry-backed controllers can be tested
// without a real registry.
type FakeLayer struct {
	crv1.Layer

	FilesContent map[string]string
}

func (fl FakeLayer) Uncompressed() (io.ReadCloser, error) {
	result := bytes.NewBuffer(nil)
	if len(fl.FilesContent) == 0 {
		return io.NopCloser(result), nil
	}

	wr := tar.NewWriter(result)
	for filename, content := range fl.FilesContent {
		if strings.Contains(filename, "/") {
			dirs := strings.Split(filename, "/")
			for i := 0; i < len(dirs)-1; i++ {
				_ = wr.WriteHeader(&tar.Header{
					Name:     dirs[i],
					Typeflag: tar.TypeDir,
					Mode:     0o777,
				})
			}
		}

		_ = wr.WriteHeader(&tar.Header{
			Name:     filename,
			Typeflag: tar.TypeReg,
			Mode:     0o600,
			Size:     int64(len(content)),
		})
		_, _ = wr.Write([]byte(content))
	}
	_ = wr.Close()

	return io.NopCloser(result), nil
}

func (fl FakeLayer) Size() (int64, error) {
	return int64(len(fl.FilesContent)), nil
}

// Image returns a fake OCI image with a single layer carrying files. Pass nil
// when the test only needs the image to exist without specific content.
func Image(files map[string]string) *crfake.FakeImage {
	return &crfake.FakeImage{
		ManifestStub: func() (*crv1.Manifest, error) {
			return &crv1.Manifest{Layers: []crv1.Descriptor{}}, nil
		},
		LayersStub: func() ([]crv1.Layer, error) {
			return []crv1.Layer{&FakeLayer{FilesContent: files}}, nil
		},
		DigestStub: func() (crv1.Hash, error) {
			return crv1.Hash{Algorithm: "sha256"}, nil
		},
	}
}
