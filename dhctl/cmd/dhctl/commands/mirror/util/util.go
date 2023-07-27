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

package util

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	tarGzExt = ".tar.gz"
)

func TrimTarGzExt(s string) string {
	return strings.TrimSuffix(s, tarGzExt)
}

func AddTarGzExt(s string) string {
	return TrimTarGzExt(s) + tarGzExt
}

func HasTarGzSuffix(s string) bool {
	return strings.HasSuffix(s, tarGzExt)
}

func CompressDir(dirname string, deleteAfterCompress bool) error {
	return NewTarGzWriter(AddTarGzExt(dirname), func(w *tar.Writer) error {
		walkFn := func(path string, info os.FileInfo, err error) error {
			if info.IsDir() || err != nil {
				return err
			}
			// Because of scoping we can reference the external root_directory variable
			newPath := path[len(dirname):]
			if len(newPath) == 0 {
				return nil
			}
			fr, err := os.Open(path)
			if err != nil {
				return err
			}
			defer fr.Close()

			h, err := tar.FileInfoHeader(info, newPath)
			if err != nil {
				return err
			}

			h.Name = newPath
			if err = w.WriteHeader(h); err != nil {
				return err
			}

			if _, err = io.Copy(w, fr); err != nil {
				return err
			}
			return os.RemoveAll(path)
		}
		if err := filepath.Walk(dirname, walkFn); err != nil {
			return err
		}
		return os.RemoveAll(dirname)
	})
}

func ExtractTarGz(filename string) error {
	dirName := TrimTarGzExt(filename)
	if err := os.MkdirAll(dirName, 0o755); err != nil {
		return err
	}
	resultPath := func(s string) string { return filepath.Join(dirName, s) }
	err := NewTarGzReader(filename, func(h *tar.Header, r *tar.Reader) (bool, error) {
		p := resultPath(h.Name)

		var err error
		switch h.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(p, h.FileInfo().Mode().Perm())
		case tar.TypeReg:
			err = mkFile(p, r, h.FileInfo())
		case tar.TypeSymlink:
			err = os.Symlink(p, h.Linkname)
		case tar.TypeLink:
			err = os.Link(p, h.Linkname)
		default:
			err = fmt.Errorf("extractTarGz: uknown type: %b in %s", h.Typeflag, h.Name)
		}
		return false, err
	})
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func mkFile(name string, content io.Reader, info os.FileInfo) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}

	f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, content)
	return err
}

func NewTarGzReader(archive string, handler func(*tar.Header, *tar.Reader) (bool, error)) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err != nil {
			return err
		}
		stop, err := handler(hdr, tr)
		if err != nil || stop {
			return err
		}
	}
}

func NewTarGzWriter(archive string, handler func(*tar.Writer) error) error {
	file, err := os.Create(archive)
	if err != nil {
		return err
	}
	defer file.Close()

	writer, err := gzip.NewWriterLevel(file, gzip.BestCompression)
	if err != nil {
		return err
	}
	defer writer.Close()

	tw := tar.NewWriter(writer)
	defer tw.Close()

	return handler(tw)
}
