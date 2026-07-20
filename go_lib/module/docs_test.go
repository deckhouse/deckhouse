/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package module

import (
	"archive/tar"
	"bytes"
	"io"
	"sort"
	"testing"
)

func TestIsDocsPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		// documentation directories
		{"docs", true},
		{"docs/README.md", true},
		{"docs/internal/dev.md", true},
		{"crds", true},
		{"crds/backup.yaml", true},
		{"openapi", true},
		{"openapi/config-values.yaml", true},
		{"openapi/conversions/v1.yaml", true},
		// module metadata — the whole point of this test: these must be
		// included so the in-cluster docs builder receives module.yaml/oss.yaml.
		{"module.yaml", true},
		{"oss.yaml", true},
		// exact match only: metadata is accepted at the module root, not as a
		// prefix or nested copy.
		{"module.yaml.bak", false},
		{"subdir/module.yaml", false},
		{"oss.yaml.tpl", false},
		{"crds/module.yaml", true}, // still accepted, but via the crds/ rule
		// everything else that ships in a module must be dropped.
		{"Chart.yaml", false},
		{"values.yaml", false},
		{"templates/deployment.yaml", false},
		{"hooks/main.go", false},
		{"images/backend/Dockerfile", false},
		{"README.md", false},
		{".helmignore", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := IsDocsPath(tt.path); got != tt.want {
				t.Errorf("IsDocsPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestExtractDocumentation(t *testing.T) {
	// A representative module layout: doc content, CRDs, openapi, module
	// metadata, plus files that must not leak into the documentation.
	input := []tarEntry{
		{name: "module.yaml", body: "name: test\n"},
		{name: "oss.yaml", body: "- name: lib\n"},
		{name: "docs", dir: true},
		{name: "docs/README.md", body: "# doc\n"},
		{name: "crds/backup.yaml", body: "kind: CRD\n"},
		{name: "openapi/config-values.yaml", body: "type: object\n"},
		// must be excluded
		{name: "Chart.yaml", body: "name: test\n"},
		{name: "hooks/main.go", body: "package main\n"},
		{name: "templates/deployment.yaml", body: "kind: Deployment\n"},
	}

	out := new(bytes.Buffer)
	if err := extractDocumentation(io.NopCloser(buildTar(t, input)), out); err != nil {
		t.Fatalf("extractDocumentation() error = %v", err)
	}

	got := tarNames(t, out)
	want := []string{
		"crds/backup.yaml",
		"docs",
		"docs/README.md",
		"module.yaml",
		"openapi/config-values.yaml",
		"oss.yaml",
	}
	sort.Strings(got)

	if len(got) != len(want) {
		t.Fatalf("extracted names = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("extracted names = %v, want %v", got, want)
		}
	}
}

type tarEntry struct {
	name string
	body string
	dir  bool
}

func buildTar(t *testing.T, entries []tarEntry) *bytes.Buffer {
	t.Helper()

	buf := new(bytes.Buffer)
	w := tar.NewWriter(buf)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Mode: 0o644, Size: int64(len(e.body))}
		if e.dir {
			hdr.Typeflag = tar.TypeDir
			hdr.Mode = 0o755
			hdr.Size = 0
		}
		if err := w.WriteHeader(hdr); err != nil {
			t.Fatalf("write header %q: %v", e.name, err)
		}
		if !e.dir {
			if _, err := w.Write([]byte(e.body)); err != nil {
				t.Fatalf("write body %q: %v", e.name, err)
			}
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	return buf
}

func tarNames(t *testing.T, r io.Reader) []string {
	t.Helper()

	var names []string
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read tar: %v", err)
		}
		names = append(names, hdr.Name)
	}
	return names
}
