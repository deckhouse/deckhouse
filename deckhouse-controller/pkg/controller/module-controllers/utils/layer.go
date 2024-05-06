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

package utils

import (
	"archive/tar"
	"bytes"
	"io"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

type FakeLayer struct {
	v1.Layer

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

	// create files in a single layer
	for filename, content := range fl.FilesContent {
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

func (fl FakeLayer) Size() (int64, error) {
	return int64(len(fl.FilesContent)), nil
}
