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

package docs

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

func TestLoadHandlerGetLocalPath(t *testing.T) {
	tests := []struct {
		fileName string
		want     string
		wantOK   bool
	}{
		{
			"./docs/install.md",
			"/app/hugo/content/modules/moduleName/stable/install.md",
			true,
		},
		{
			"./docs",
			"/app/hugo/content/modules/moduleName/stable",
			true,
		},
		{
			"docs/install.md",
			"/app/hugo/content/modules/moduleName/stable/install.md",
			true,
		},
		{
			"docs/README_RU.md",
			"/app/hugo/content/modules/moduleName/stable/README.ru.md",
			true,
		},
		{
			"docs",
			"/app/hugo/content/modules/moduleName/stable",
			true,
		},
		{
			"docs/install.md",
			"/app/hugo/content/modules/moduleName/stable/install.md",
			true,
		},
		{
			"docs/README_RU.md",
			"/app/hugo/content/modules/moduleName/stable/README.ru.md",
			true,
		},
		{
			"docs",
			"/app/hugo/content/modules/moduleName/stable",
			true,
		},
		{
			"not-docs/file.ext",
			"",
			false,
		},
		{
			"crds/object.yaml",
			"/app/hugo/data/modules/moduleName/stable/crds/object.yaml",
			true,
		},
		{
			"crds",
			"/app/hugo/data/modules/moduleName/stable/crds",
			true,
		},
		{
			"./crds/object.yaml",
			"/app/hugo/data/modules/moduleName/stable/crds/object.yaml",
			true,
		},
		{
			"crds/object.yml",
			"/app/hugo/data/modules/moduleName/stable/crds/object.yml",
			true,
		},
		{
			"crds/object.json",
			"/app/hugo/data/modules/moduleName/stable/crds/object.json",
			true,
		},
		// The docs templates treat crds/ as a flat map (one file == one CRD).
		// Subdirectories must be rejected regardless of what's inside, so the
		// CRD section renders correctly and no garbage is loaded into data/.
		{
			"crds/native",
			"",
			false,
		},
		{
			"crds/native/object.yaml",
			"",
			false,
		},
		{
			"crds/cert-manager/cert.yaml",
			"",
			false,
		},
		{
			"crds/gatekeeper/templates/template.yaml",
			"",
			false,
		},
		// Non-data files at the top level of crds/ must be rejected too — Hugo's
		// data loader cannot unmarshal them and fails the whole module build
		// with `unmarshal of format "" is not supported`.
		{
			"crds/README.md",
			"",
			false,
		},
		{
			"crds/pull_dex_crds.sh",
			"",
			false,
		},
		{
			"crds/x-pull-crds.sh",
			"",
			false,
		},
		// And the same files under subdirectories — rejected by the no-subdir
		// rule above, but kept here as regression cases for the original bug
		// (operator-trivy: crds/native/README.md).
		{
			"crds/native/README.md",
			"",
			false,
		},
		{
			"crds/gatekeeper/README.md",
			"",
			false,
		},
		{
			"crds/native/update.sh",
			"",
			false,
		},
		// Paths that merely start with the literal "crds" must not be matched.
		{
			"crdsxxx/object.yaml",
			"",
			false,
		},
		{
			"openapi/doc-ru-config-values.yaml",
			"/app/hugo/data/modules/moduleName/stable/openapi/doc-ru-config-values.yaml",
			true,
		},
		{
			"openapi/openapi-case-tests.yaml",
			"",
			false,
		},
		{
			"./openapi/config-values.yaml",
			"/app/hugo/data/modules/moduleName/stable/openapi/config-values.yaml",
			true,
		},
		{
			"openapi",
			"/app/hugo/data/modules/moduleName/stable/openapi",
			true,
		},
		{
			"openapi",
			"/app/hugo/data/modules/moduleName/stable/openapi",
			true,
		},
		// Test cases for internal directories exclusion
		{
			"docs/internal/README.md",
			"",
			false,
		},
		{
			"docs/internals/development.md",
			"",
			false,
		},
		{
			"docs/development/HOWTO.md",
			"",
			false,
		},
		{
			"docs/dev/debug.md",
			"",
			false,
		},
		{
			"docs/internal/subfolder/file.md",
			"",
			false,
		},
		// Test that regular docs files still work
		{
			"docs/public/README.md",
			"/app/hugo/content/modules/moduleName/stable/public/README.md",
			true,
		},
		{
			"docs/configuration.md",
			"/app/hugo/content/modules/moduleName/stable/configuration.md",
			true,
		},
		{
			"module.yaml",
			"/app/hugo/data/modules/moduleName/stable/module.yaml",
			true,
		},
		{
			"oss.yaml",
			"/app/hugo/data/modules/moduleName/stable/oss.yaml",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.fileName, func(t *testing.T) {
			var svc = NewService("/app/hugo/", "", false, log.NewNop(), metricsstorage.NewMetricStorage())

			got, ok := svc.getLocalPath("moduleName", "stable", tt.fileName)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("getLocalPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestUploadCreatesRootFiles is a regression test for the case where a
// root-level file (module.yaml, oss.yaml) is not preceded by its parent
// directory entry in the tar stream. The extractor must create the parent
// directory itself instead of failing with "no such file or directory".
func TestUploadCreatesRootFiles(t *testing.T) {
	baseDir := t.TempDir()
	svc := NewService(baseDir, "", false, log.NewNop(), metricsstorage.NewMetricStorage())

	// module.yaml and oss.yaml come first, before any directory entry — this
	// is what filepath.Walk produces for a module (crds < docs < module.yaml <
	// oss.yaml < openapi), and modules without a crds/ dir hit this ordering.
	entries := []struct {
		name string
		body string
	}{
		{"module.yaml", "name: test\n"},
		{"oss.yaml", "- name: lib\n"},
		{"docs/README.md", "# doc\n"},
		{"crds/backup.yaml", "kind: CRD\n"},
		{"openapi/config-values.yaml", "type: object\n"},
	}

	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	for _, e := range entries {
		if err := tw.WriteHeader(&tar.Header{Name: e.name, Mode: 0o644, Size: int64(len(e.body))}); err != nil {
			t.Fatalf("write header %q: %v", e.name, err)
		}
		if _, err := tw.Write([]byte(e.body)); err != nil {
			t.Fatalf("write body %q: %v", e.name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}

	if err := svc.Upload(io.NopCloser(buf), "test-module", "v1.0.0", []string{"stable"}); err != nil {
		t.Fatalf("Upload() error = %v", err)
	}

	want := map[string]string{
		filepath.Join(baseDir, "data/modules/test-module/stable/module.yaml"):                "name: test\n",
		filepath.Join(baseDir, "data/modules/test-module/stable/oss.yaml"):                   "- name: lib\n",
		filepath.Join(baseDir, "data/modules/test-module/stable/crds/backup.yaml"):           "kind: CRD\n",
		filepath.Join(baseDir, "data/modules/test-module/stable/openapi/config-values.yaml"): "type: object\n",
		filepath.Join(baseDir, "content/modules/test-module/stable/README.md"):               "# doc\n",
	}
	for path, body := range want {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("expected file %q: %v", path, err)
			continue
		}
		if string(got) != body {
			t.Errorf("file %q = %q, want %q", path, got, body)
		}
	}
}
