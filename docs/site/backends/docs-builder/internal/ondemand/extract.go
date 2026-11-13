// Copyright 2026 Flant JSC
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

package ondemand

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"

	"github.com/flant/docs-builder/internal/docs"
)

// Module files the documentation is built from.
var (
	docsDirs  = []string{"docs/", "openapi/", "crds/"}
	docsFiles = []string{
		"module.yaml",
		"oss.yaml",
		"candi/openapi/cluster_configuration.yaml",
		"candi/openapi/doc-ru-cluster_configuration.yaml",
	}
)

// DocsArchive writes the documentation files of a module image to w as a tar
// archive in the layout RenderVersion accepts, and reports whether the image
// has documentation pages (docs/). Layers are read as a stream, so files reach
// w while the image is still downloading. Files over docs.MaxDocsBytes or
// docs.MaxDocsFiles in total fail with docs.ErrDocsTooLarge: w is on the
// cache volume, which an image must not fill.
func DocsArchive(img v1.Image, w io.Writer) (bool, error) {
	fs := mutate.Extract(img)
	defer fs.Close()

	tr := tar.NewReader(fs)
	tw := tar.NewWriter(w)
	hasDocs := false
	var size docs.DocsSize

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return false, fmt.Errorf("read image filesystem: %w", err)
		}

		name, ok := docsPath(hdr)
		if !ok {
			continue
		}
		if err := size.Add(hdr.Size); err != nil {
			return false, err
		}
		hasDocs = hasDocs || strings.HasPrefix(name, "docs/")

		if err := tw.WriteHeader(&tar.Header{
			Typeflag: tar.TypeReg,
			Name:     name,
			Mode:     hdr.Mode,
			Size:     hdr.Size,
			ModTime:  hdr.ModTime,
		}); err != nil {
			return false, fmt.Errorf("write header %s: %w", name, err)
		}

		if _, err := io.Copy(tw, tr); err != nil {
			return false, fmt.Errorf("copy %s: %w", name, err)
		}
	}

	// mutate.Extract ends its archive before it reports a broken layer (a
	// failed download, a wrong digest), so the error comes after the end:
	// without this read the image passes for one without the layer's files.
	if _, err := io.Copy(io.Discard, fs); err != nil {
		return false, fmt.Errorf("read image filesystem: %w", err)
	}

	if err := tw.Close(); err != nil {
		return false, fmt.Errorf("close archive: %w", err)
	}

	return hasDocs, nil
}

// docsPath returns the normalized name of a documentation file, or false for
// anything else: other files, directories, links, Go sources and names that
// are absolute or step out of the image root.
func docsPath(hdr *tar.Header) (string, bool) {
	if hdr.Typeflag != tar.TypeReg {
		return "", false
	}

	name := strings.TrimPrefix(hdr.Name, "./")
	if name == "" || path.IsAbs(name) || path.Clean(name) != name || strings.Contains(name, "..") {
		return "", false
	}

	if path.Ext(name) == ".go" {
		return "", false
	}

	for _, file := range docsFiles {
		if name == file {
			return name, true
		}
	}

	for _, dir := range docsDirs {
		if strings.HasPrefix(name, dir) {
			return name, true
		}
	}

	return "", false
}
