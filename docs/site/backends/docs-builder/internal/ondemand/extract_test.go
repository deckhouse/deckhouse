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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"testing/iotest"

	"github.com/deckhouse/deckhouse/pkg/log"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	"github.com/flant/docs-builder/internal/docs"
)

type entry struct {
	name     string
	typeflag byte
	body     string
	linkname string
}

func layer(t *testing.T, entries ...entry) v1.Layer {
	t.Helper()

	return layerOf(t, tarOf(t, entries...))
}

// tarOf is a tar stream of the given files.
func tarOf(t *testing.T, entries ...entry) []byte {
	t.Helper()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, e := range entries {
		typeflag := e.typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		hdr := &tar.Header{Name: e.name, Typeflag: typeflag, Mode: 0o644, Size: int64(len(e.body)), Linkname: e.linkname}
		if typeflag != tar.TypeReg {
			hdr.Size = 0
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

// layerOf is a layer of the given tar stream.
func layerOf(t *testing.T, data []byte) v1.Layer {
	t.Helper()

	l, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(data)), nil
	})
	if err != nil {
		t.Fatal(err)
	}

	return l
}

// errBroken breaks the download of a layer.
var errBroken = errors.New("connection reset by peer")

// brokenLayer is a layer whose download breaks after data, the head of its
// uncompressed stream.
type brokenLayer struct {
	v1.Layer
	data []byte
}

func (l brokenLayer) Uncompressed() (io.ReadCloser, error) {
	return io.NopCloser(io.MultiReader(bytes.NewReader(l.data), iotest.ErrReader(errBroken))), nil
}

func fakeModuleImage(t *testing.T) v1.Image {
	t.Helper()

	img, err := mutate.AppendLayers(empty.Image,
		layer(t,
			entry{name: "docs", typeflag: tar.TypeDir},
			entry{name: "docs/README.md", body: "# readme"},
			entry{name: "./docs/README_RU.md", body: "# readme ru"},
			entry{name: "docs/old.md", body: "removed in the next layer"},
			entry{name: "docs/tool.go", body: "package main"},
			entry{name: "docs/link.md", typeflag: tar.TypeSymlink, linkname: "/etc/passwd"},
			entry{name: "openapi/config-values.yaml", body: "type: object"},
			entry{name: "openapi/conversions/v2.yaml", body: "conversions: []"},
			entry{name: "crds/thing.yaml", body: "kind: CustomResourceDefinition"},
			entry{name: "module.yaml", body: "name: echo"},
			entry{name: "oss.yaml", body: "- name: echo"},
			entry{name: "candi/openapi/cluster_configuration.yaml", body: "kind: DVPClusterConfiguration"},
			entry{name: "candi/openapi/doc-ru-cluster_configuration.yaml", body: "apiVersions: []"},
			entry{name: "candi/openapi/cloud_discovery_data.yaml", body: "type: object"},
			entry{name: "candi/terraform/main.tf", body: "terraform {}"},
			entry{name: "images/app/Dockerfile", body: "FROM scratch"},
			entry{name: "templates/docs/x.md", body: "not module docs"},
			entry{name: "docs/../etc/shadow", body: "traversal"},
			entry{name: "/docs/abs.md", body: "absolute"},
		),
		layer(t, entry{name: "docs/.wh.old.md"}),
	)
	if err != nil {
		t.Fatal(err)
	}

	return img
}

func archiveNames(t *testing.T, archive []byte) []string {
	t.Helper()

	var names []string
	tr := tar.NewReader(bytes.NewReader(archive))
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, hdr.Name)
	}
	slices.Sort(names)

	return names
}

func TestDocsArchive(t *testing.T) {
	var buf bytes.Buffer
	hasDocs, err := DocsArchive(fakeModuleImage(t), &buf)
	if err != nil {
		t.Fatal(err)
	}
	if !hasDocs {
		t.Fatal("docs/ is not reported")
	}

	want := []string{
		"candi/openapi/cluster_configuration.yaml",
		"candi/openapi/doc-ru-cluster_configuration.yaml",
		"crds/thing.yaml",
		"docs/README.md",
		"docs/README_RU.md",
		"module.yaml",
		"openapi/config-values.yaml",
		"openapi/conversions/v2.yaml",
		"oss.yaml",
	}
	if got := archiveNames(t, buf.Bytes()); !slices.Equal(got, want) {
		t.Fatalf("archive files:\n got %v\nwant %v", got, want)
	}
}

// mutate.Extract ends its archive before it reports a broken layer: the
// files of the layer are missing, not absent from the image.
func TestDocsArchiveFailsOnBrokenLayer(t *testing.T) {
	stream := tarOf(t, entry{name: "docs/README.md", body: "# readme"}, entry{name: "openapi/config-values.yaml", body: "type: object"})

	for name, size := range map[string]int{
		// The header and the padded body of docs/README.md.
		"between files": 1024,
		// The digest is checked after the last file.
		"after the last file": len(stream),
	} {
		t.Run(name, func(t *testing.T) {
			img, err := mutate.AppendLayers(empty.Image,
				brokenLayer{Layer: layerOf(t, stream), data: stream[:size]},
				layer(t, entry{name: "module.yaml", body: "name: echo"}),
			)
			if err != nil {
				t.Fatal(err)
			}

			if _, err := DocsArchive(img, io.Discard); !errors.Is(err, errBroken) {
				t.Fatalf("err = %v, want %v", err, errBroken)
			}
		})
	}
}

func TestDocsArchiveIsAcceptedByRenderVersion(t *testing.T) {
	var buf bytes.Buffer
	if _, err := DocsArchive(fakeModuleImage(t), &buf); err != nil {
		t.Fatal(err)
	}

	// No site template: only the unpacked files matter here, not the pages.
	site := filepath.Join(t.TempDir(), "site")
	err := docs.RenderVersion(context.Background(), t.TempDir(), site, t.TempDir(), "echo", "v1.2.3", &buf, false, log.NewNop())
	if errors.Is(err, docs.ErrNoPages) {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(site, "content/modules/echo/v1.2.3/README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "# readme" {
		t.Fatalf("README.md = %q", got)
	}
}

// The archive is written to the cache volume: an image can not fill it.
func TestDocsArchiveRejectsLargeImages(t *testing.T) {
	archive := func(t *testing.T, l v1.Layer) error {
		t.Helper()
		img, err := mutate.AppendLayers(empty.Image, l)
		if err != nil {
			t.Fatal(err)
		}
		_, err = DocsArchive(img, io.Discard)
		return err
	}

	t.Run("bytes", func(t *testing.T) {
		// The header alone: the file is refused before its body is read.
		var buf bytes.Buffer
		if err := tar.NewWriter(&buf).WriteHeader(&tar.Header{Name: "docs/README.md", Typeflag: tar.TypeReg, Mode: 0o644, Size: docs.MaxDocsBytes + 1}); err != nil {
			t.Fatal(err)
		}
		if err := archive(t, layerOf(t, buf.Bytes())); !errors.Is(err, docs.ErrDocsTooLarge) {
			t.Fatalf("err = %v, want ErrDocsTooLarge", err)
		}
	})

	t.Run("files", func(t *testing.T) {
		entries := make([]entry, docs.MaxDocsFiles+1)
		for i := range entries {
			entries[i] = entry{name: fmt.Sprintf("docs/%d.md", i)}
		}
		if err := archive(t, layer(t, entries...)); !errors.Is(err, docs.ErrDocsTooLarge) {
			t.Fatalf("err = %v, want ErrDocsTooLarge", err)
		}
	})

	t.Run("other files do not count", func(t *testing.T) {
		entries := make([]entry, docs.MaxDocsFiles+1)
		for i := range entries {
			entries[i] = entry{name: fmt.Sprintf("images/%d", i)}
		}
		if err := archive(t, layer(t, entries...)); err != nil {
			t.Fatal(err)
		}
	})
}
