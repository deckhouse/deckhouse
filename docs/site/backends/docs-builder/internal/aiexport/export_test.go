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

	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestExportWithoutManifest(t *testing.T) {
	if err := Export(t.TempDir(), "en", log.NewNop()); err != nil {
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

	if err := Export(publicDir, "en", log.NewNop()); err != nil {
		t.Fatalf("Export: %v", err)
	}

	// The directory root README is normalized to index.md (no authored index in
	// the directory), so a directory link resolves to it without a redirect.
	markdown := readFile(t, filepath.Join(publicDir, "en", "modules", "prompp", "stable", "index.md"))

	front, body, ok := strings.Cut(markdown, "---\n\n")
	if !ok {
		t.Fatalf("index.md has no frontmatter delimiter:\n%s", markdown)
	}

	// The heading keeps its HTML anchor as a `{#id}` attribute.
	if want := "# Prompp\n\nFast Prometheus.\n\n## Usage {#usage}\n\nEnable it.\n"; body != want {
		t.Errorf("index.md body:\ngot:\n%s\nwant:\n%s", body, want)
	}

	for _, want := range []string{
		"---\ntitle: Prompp",
		"description: A drop-in Prometheus replacement.",
		"canonical: https://deckhouse.io/modules/prompp/stable/readme.html",
		"lang: en",
		"version: v1.2.3",
		"module: prompp",
		"moduleType: external",
		"channel: stable",
		"- ce",
		"stage: General Availability",
	} {
		if !strings.Contains(front, want) {
			t.Errorf("readme.md frontmatter is missing %q:\n%s", want, front)
		}
	}

	// productCode is a corpus-level field only; it must not leak into the header.
	if strings.Contains(front, "productCode") {
		t.Errorf("frontmatter should not contain productCode:\n%s", front)
	}

	var corpus Corpus
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(publicDir, "en", "modules", "external-corpus.json"))), &corpus); err != nil {
		t.Fatalf("parse external-corpus.json: %v", err)
	}

	// The corpus embeds its own schema, and that schema must actually describe
	// the corpus the exporter just wrote.
	assertCorpusMatchesSchema(t, readFile(t, filepath.Join(publicDir, "en", "modules", "external-corpus.json")))

	if len(corpus.Documents) != 1 {
		t.Fatalf("got %d documents, want 1", len(corpus.Documents))
	}

	document := corpus.Documents[0]
	if document.MdURL != "/modules/prompp/stable/index.md" {
		t.Errorf("mdUrl = %q", document.MdURL)
	}
	if document.Path != "modules/prompp/stable/readme" {
		t.Errorf("path = %q", document.Path)
	}
	if document.ModuleType != "external" || document.Channel != "stable" {
		t.Errorf("moduleType = %q, channel = %q", document.ModuleType, document.Channel)
	}
	if document.Version != "v1.2.3" {
		t.Errorf("version = %q", document.Version)
	}
	if want := []string{"prometheus", "monitoring"}; !equalStrings(document.Keywords, want) {
		t.Errorf("keywords = %#v, want %#v", document.Keywords, want)
	}
	if !strings.HasPrefix(document.ContentHash, "sha256:") {
		t.Errorf("contentHash = %q", document.ContentHash)
	}
	if len(document.Chunks) != 2 {
		t.Fatalf("got %d chunks, want 2", len(document.Chunks))
	}
	// The `{#usage}` attribute must not stop scanHeadings from re-attaching the
	// anchor to the H2 chunk — that anchor is why headings carry the id at all.
	if document.Chunks[1].Anchor != "usage" {
		t.Errorf("H2 chunk anchor = %q, want %q", document.Chunks[1].Anchor, "usage")
	}

	llms := readFile(t, filepath.Join(publicDir, "en", "modules", "external-llms.txt"))
	for _, want := range []string{
		"# Deckhouse Platform Modules",
		// The manifest carries no localized preamble, so the English defaults are
		// used: the intro is a blockquote, the version note a plain line.
		"> " + defaultLLMsIntro,
		defaultLLMsVersionNote,
		"## prompp",
		"- [Prompp](https://deckhouse.io/modules/prompp/stable/index.md): A drop-in Prometheus replacement.",
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

func TestExportNormalizesReadmeToIndex(t *testing.T) {
	publicDir := t.TempDir()

	// `solo` has only a README; `paired` has both a README and an authored
	// index page in the same directory.
	page := func(title string) string {
		return `<html><body><div class="post-content"><p>` + title + `.</p></div></body></html>`
	}
	writeFile(t, filepath.Join(publicDir, "en", "modules", "solo", "stable", "readme.html"), page("Solo"))
	writeFile(t, filepath.Join(publicDir, "en", "modules", "paired", "stable", "readme.html"), page("Paired readme"))
	writeFile(t, filepath.Join(publicDir, "en", "modules", "paired", "stable", "index.html"), page("Paired index"))

	manifest := Manifest{
		Version: 1, Lang: "en", BaseURL: "https://deckhouse.io", Title: "Modules",
		Documents: []ManifestDocument{
			{Title: "Solo", URL: "/modules/solo/stable/readme.html", HTMLPath: "/en/modules/solo/stable/readme.html", Module: "solo"},
			{Title: "Paired readme", URL: "/modules/paired/stable/readme.html", HTMLPath: "/en/modules/paired/stable/readme.html", Module: "paired"},
			{Title: "Paired index", URL: "/modules/paired/stable/index.html", HTMLPath: "/en/modules/paired/stable/index.html", Module: "paired"},
		},
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	writeFile(t, filepath.Join(publicDir, "en", "ai", "ai.json"), string(encoded))

	if err := Export(publicDir, "en", log.NewNop()); err != nil {
		t.Fatalf("Export: %v", err)
	}

	// A lone README normalizes to index.md; readme.md must not be written.
	assertFileExists(t, filepath.Join(publicDir, "en", "modules", "solo", "stable", "index.md"))
	assertFileAbsent(t, filepath.Join(publicDir, "en", "modules", "solo", "stable", "readme.md"))

	// With an authored index in the directory, the README keeps its own name so
	// it does not clobber the index.
	assertFileExists(t, filepath.Join(publicDir, "en", "modules", "paired", "stable", "readme.md"))
	assertFileExists(t, filepath.Join(publicDir, "en", "modules", "paired", "stable", "index.md"))

	var corpus Corpus
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(publicDir, "en", "modules", "external-corpus.json"))), &corpus); err != nil {
		t.Fatalf("parse external-corpus.json: %v", err)
	}

	mdURLByTitle := map[string]string{}
	for _, doc := range corpus.Documents {
		mdURLByTitle[doc.Title] = doc.MdURL
	}
	for title, want := range map[string]string{
		"Solo":          "/modules/solo/stable/index.md",
		"Paired readme": "/modules/paired/stable/readme.md",
		"Paired index":  "/modules/paired/stable/index.md",
	} {
		if got := mdURLByTitle[title]; got != want {
			t.Errorf("mdUrl of %q = %q, want %q", title, got, want)
		}
	}
}

func TestExportRewritesInternalLinks(t *testing.T) {
	publicDir := t.TempDir()

	writeFile(t, filepath.Join(publicDir, "en", "modules", "demo", "stable", "readme.html"),
		`<html><body><div class="post-content"><p>`+
			`<a href="/modules/demo/stable/cr.html">CR</a> `+
			`<a href="/modules/other/stable/">Other</a> `+
			`<a href="/modules/demo/stable/cr.html#config">Config</a> `+
			`<a href="#section">Section</a> `+
			`<a href="readme.html#other-section">Self</a> `+
			`<a href="/modules/widget/stable/readme.html">Widget</a> `+
			`<a href="https://deckhouse.io/products/kubernetes-platform/documentation/v1/architecture/versioning/">Versioning</a> `+
			`<a href="https://deckhouse.io/products/kubernetes-platform/documentation/v1/user/web/ui.html">UI</a> `+
			`<a href="https://example.com/guide/">External</a> `+
			`<a href="/downloads/deckhouse-cli/">Downloads</a> `+
			`<a href="/images/logo.png">Logo</a>`+
			`</p></div></body></html>`)

	manifest := Manifest{
		Version: 1, Lang: "en", BaseURL: "https://deckhouse.io", Title: "Modules",
		Documents: []ManifestDocument{
			{Title: "Demo", URL: "/modules/demo/stable/readme.html", HTMLPath: "/en/modules/demo/stable/readme.html", Module: "demo"},
		},
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	writeFile(t, filepath.Join(publicDir, "en", "ai", "ai.json"), string(encoded))

	if err := Export(publicDir, "en", log.NewNop()); err != nil {
		t.Fatalf("Export: %v", err)
	}

	markdown := readFile(t, filepath.Join(publicDir, "en", "modules", "demo", "stable", "index.md"))

	for _, want := range []string{
		// A plain root-relative `.html` target.
		"(https://deckhouse.io/modules/demo/stable/cr.md)",
		// A directory always resolves to index.md.
		"(https://deckhouse.io/modules/other/stable/index.md)",
		// Fragment is preserved.
		"(https://deckhouse.io/modules/demo/stable/cr.md#config)",
		// A bare in-page anchor points at this page's own file — index.md, the
		// normalized README, not readme.md.
		"(https://deckhouse.io/modules/demo/stable/index.md#section)",
		// An explicit link to the README follows the same normalization.
		"(https://deckhouse.io/modules/demo/stable/index.md#other-section)",
		// A link to another module's README lands on its index.md too.
		"(https://deckhouse.io/modules/widget/stable/index.md)",
		// A same-site link written absolute is still rewritten — directory form.
		"(https://deckhouse.io/products/kubernetes-platform/documentation/v1/architecture/versioning/index.md)",
		// ...and .html form.
		"(https://deckhouse.io/products/kubernetes-platform/documentation/v1/user/web/ui.md)",
		// A link to another host is left untouched.
		"(https://example.com/guide/)",
		// A directory outside the documentation and modules space keeps its HTML URL.
		"(https://deckhouse.io/downloads/deckhouse-cli/)",
		// A non-page asset is left untouched.
		"(https://deckhouse.io/images/logo.png)",
	} {
		if !strings.Contains(markdown, want) {
			t.Errorf("readme.md is missing link %q:\n%s", want, markdown)
		}
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

func assertFileExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected %s to exist: %v", path, err)
	}
}

func assertFileAbsent(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected %s to be absent, stat err = %v", path, err)
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
