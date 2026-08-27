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

package aiexport

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportWithoutManifest(t *testing.T) {
	if err := Export(t.TempDir(), "en"); err != nil {
		t.Fatalf("Export: %v", err)
	}
}

func TestExport(t *testing.T) {
	publicDir := t.TempDir()

	writeFile(t, filepath.Join(publicDir, "en", "modules", "prompp", "stable", "readme.html"),
		`<html><body><div class="docs"><div class="docs__wrap-title"><h1 class="docs__title">Prompp</h1></div>`+
			`<div class="post-content"><p>Fast Prometheus.</p><h2 id="usage">Usage</h2><p>Enable it.</p></div>`+
			`</div></body></html>`)

	manifest := Manifest{
		Version:     1,
		ProductCode: "external-modules",
		Generator:   "hugo",
		Lang:        "en",
		BaseURL:     "https://deckhouse.io",
		Title:       "Deckhouse Platform Modules",
		Description: "Kubernetes is flexibly and rapidly expanded by Deckhouse Platform modules.",
		Documents: []ManifestDocument{
			{
				Title:       "Prompp",
				URL:         "/modules/prompp/stable/readme.html",
				HTMLPath:    "/en/modules/prompp/stable/readme.html",
				Module:      "prompp",
				Channel:     "stable",
				Version:     "v1.2.3",
				Description: "A drop-in Prometheus replacement.",
				Keywords:    []string{"prometheus", ""},
				Tags:        []string{"monitoring"},
				Editions:    []string{"ce", "ee"},
				Stage:       "General Availability",
			},
			// Listed but never rendered — must be skipped, not fatal.
			{
				Title:    "Ghost",
				URL:      "/modules/ghost/stable/readme.html",
				HTMLPath: "/en/modules/ghost/stable/readme.html",
				Module:   "ghost",
			},
		},
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	writeFile(t, filepath.Join(publicDir, "en", "ai", "ai.json"), string(encoded))

	if err := Export(publicDir, "en"); err != nil {
		t.Fatalf("Export: %v", err)
	}

	markdown := readFile(t, filepath.Join(publicDir, "en", "modules", "prompp", "stable", "readme.md"))
	if want := "# Prompp\n\nFast Prometheus.\n\n## Usage\n\nEnable it.\n"; markdown != want {
		t.Errorf("readme.md:\ngot:\n%s\nwant:\n%s", markdown, want)
	}

	var corpus Corpus
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(publicDir, "en", "modules", "external-corpus.json"))), &corpus); err != nil {
		t.Fatalf("parse external-corpus.json: %v", err)
	}

	if len(corpus.Documents) != 1 {
		t.Fatalf("got %d documents, want 1", len(corpus.Documents))
	}

	document := corpus.Documents[0]
	if document.MdURL != "/modules/prompp/stable/readme.md" {
		t.Errorf("mdUrl = %q", document.MdURL)
	}
	if document.Path != "modules/prompp/stable/readme" {
		t.Errorf("path = %q", document.Path)
	}
	if document.ModuleType != "external" || document.Channel != "stable" {
		t.Errorf("moduleType = %q, channel = %q", document.ModuleType, document.Channel)
	}
	if want := []string{"prometheus", "monitoring"}; !equalStrings(document.Keywords, want) {
		t.Errorf("keywords = %#v, want %#v", document.Keywords, want)
	}
	if !strings.HasPrefix(document.ContentHash, "sha256:") {
		t.Errorf("contentHash = %q", document.ContentHash)
	}
	if len(document.Chunks) != 2 {
		t.Errorf("got %d chunks, want 2", len(document.Chunks))
	}

	llms := readFile(t, filepath.Join(publicDir, "en", "modules", "external-llms.txt"))
	for _, want := range []string{
		"# Deckhouse Platform Modules",
		"> The content below is for Deckhouse Platform external modules.",
		"> Note that the documented version may differ from the version actually used in a cluster.",
		"## prompp",
		"- [Prompp](https://deckhouse.io/modules/prompp/stable/readme.md): A drop-in Prometheus replacement.",
	} {
		if !strings.Contains(llms, want) {
			t.Errorf("external-llms.txt is missing %q:\n%s", want, llms)
		}
	}

	// The corpora are listed by the `llms.txt` of the DKP documentation, the
	// entry point of the site (`AIRoot` of `_plugins/ai_export.rb`). This index
	// is linked from there and does not repeat them.
	if strings.Contains(llms, "## Optional") {
		t.Errorf("external-llms.txt has an Optional section:\n%s", llms)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(content)
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}

	return true
}
