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

package docs

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestValidateModuleVersion(t *testing.T) {
	tests := []struct {
		module, version string
		ok              bool
	}{
		{"virtualization", "v1.13.0", true},
		{"virtualization", "v1.13.0-rc.1", true},
		{"echo", "v0.17.0", true},
		{"echo", "stable", false},
		{"echo", "v1.2", false},
		{"echo", "1.2.3", false},
		{"echo", "..", false},
		{"echo", "v1.2.3/../x", false},
		{"../echo", "v1.2.3", false},
		{"Echo", "v1.2.3", false},
		// A version is rendered as a tag: lower case and no longer than one.
		{"echo", "v1.2.3-RC.1", false},
		{"echo", "v1.2.3-" + strings.Repeat("a", 121), true},
		{"echo", "v1.2.3-" + strings.Repeat("a", 122), false},
	}

	for _, tt := range tests {
		err := ValidateModuleVersion(tt.module, tt.version)
		if (err == nil) != tt.ok {
			t.Errorf("%s %s: err = %v, want ok %v", tt.module, tt.version, err, tt.ok)
		}
		if err := ValidateModuleTag(tt.module, tt.version); tt.ok && err != nil {
			t.Errorf("%s %s is a version that can not be rendered: %v", tt.module, tt.version, err)
		}
	}
}

func TestValidateModuleTag(t *testing.T) {
	tests := []struct {
		module, tag string
		ok          bool
	}{
		{"console", "main", true},
		{"time-machine", "new-flow", true},
		{"echo", "v1.2.3", true},
		{"echo", "pr_42", true},
		{"echo", "Feature-X", false},
		{"echo", "versions", false},
		{"echo", "stable", false},
		{"echo", "latest", false},
		{"echo", "stable-fix", true},
		{"echo", ".", false},
		{"echo", "..", false},
		{"echo", ".hidden", false},
		{"echo", "a/b", false},
		{"echo", "", false},
		{"../echo", "main", false},
	}

	for _, tt := range tests {
		err := ValidateModuleTag(tt.module, tt.tag)
		if (err == nil) != tt.ok {
			t.Errorf("%s %s: err = %v, want ok %v", tt.module, tt.tag, err, tt.ok)
		}
	}
}

func TestRenderVersionWithoutDocs(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, body := range map[string]string{
		"module.yaml":                "name: echo",
		"openapi/config-values.yaml": "type: object",
	} {
		if err := tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	err := RenderVersion(context.Background(), t.TempDir(), filepath.Join(dir, "site"), filepath.Join(dir, "out"), "echo", "v0.17.0", &buf, false, log.NewNop())
	if !errors.Is(err, ErrNoPages) {
		t.Fatalf("err = %v, want ErrNoPages", err)
	}
}

// Hugo drops a module whose pages it can not read and builds the rest of
// the site: the version has no pages then, which is a build failure too, so
// it is not rendered again and again.
func TestRenderVersionBrokenPagesIsBuildError(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	body := "---\ntitle: [Echo\n---\n\nEcho answers.\n"
	if err := tw.WriteHeader(&tar.Header{Name: "docs/README.md", Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	err := RenderVersion(context.Background(), "../../../docs-builder-template", filepath.Join(dir, "site"), filepath.Join(dir, "out"), "echo", "v0.18.0", &buf, false, log.NewNop())
	if !errors.Is(err, ErrBuild) {
		t.Fatalf("err = %v, want ErrBuild", err)
	}
}

func TestRenderVersionIndex(t *testing.T) {
	for _, withAI := range []bool{false, true} {
		t.Run(fmt.Sprintf("withAI=%v", withAI), func(t *testing.T) {
			var buf bytes.Buffer
			tw := tar.NewWriter(&buf)
			for name, body := range map[string]string{
				"module.yaml":       "name: echo",
				"docs/README.md":    "---\ntitle: Echo\ndescription: Echo module.\n---\n\nEcho answers. See [FAQ](/modules/echo/faq.html).\n",
				"docs/FAQ.md":       "---\ntitle: FAQ\n---\n\nHow does echo answer?\n",
				"docs/README_RU.md": "---\ntitle: Echo\n---\n\nEcho answers in Russian.\n",
			} {
				if err := tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(body))}); err != nil {
					t.Fatal(err)
				}
				if _, err := tw.Write([]byte(body)); err != nil {
					t.Fatal(err)
				}
			}
			if err := tw.Close(); err != nil {
				t.Fatal(err)
			}

			dir := t.TempDir()
			out := filepath.Join(dir, "out")
			err := RenderVersion(context.Background(), "../../../docs-builder-template", filepath.Join(dir, "site"), out, "echo", "v0.18.0", &buf, withAI, log.NewNop())
			if err != nil {
				t.Fatal(err)
			}

			index, err := os.ReadFile(filepath.Join(out, "en", "search.json"))
			if err != nil {
				t.Fatal(err)
			}
			var parsed struct {
				Documents []struct {
					URL string `json:"url"`
				} `json:"documents"`
			}
			if err := json.Unmarshal(index, &parsed); err != nil {
				t.Fatalf("search.json: %v", err)
			}
			urls := make([]string, 0, len(parsed.Documents))
			for _, d := range parsed.Documents {
				urls = append(urls, d.URL)
			}
			if !slices.Contains(urls, "/modules/echo/v0.18.0/faq.html") {
				t.Errorf("search.json urls = %v, want the FAQ of the version", urls)
			}

			page, err := os.ReadFile(filepath.Join(out, "en", "readme.html"))
			if err != nil {
				t.Fatal(err)
			}
			// Without the language: the site tells the languages by the domain,
			// and the template adds the language only in the module mode.
			if !bytes.Contains(page, []byte(",/modules/echo/v0.18.0/search.json:")) {
				t.Error("the version page does not search the version index")
			}
			if !bytes.Contains(page, []byte(`href="./faq.html"`)) {
				t.Error("a link to the own module leaves the version")
			}

			_, err = os.Stat(filepath.Join(out, "en", "llms.txt"))
			if withAI != (err == nil) {
				t.Errorf("llms.txt present = %v, want %v", err == nil, withAI)
			}
			_, err = os.Stat(filepath.Join(out, "en", "faq.md"))
			if withAI != (err == nil) {
				t.Errorf("faq.md present = %v, want %v", err == nil, withAI)
			}
		})
	}
}

// Version sites live in their own directories and render at the same time:
// a render must not wait for another one or for the main site.
func TestRenderVersionConcurrently(t *testing.T) {
	archive := func() *bytes.Buffer {
		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)
		for name, body := range map[string]string{
			"module.yaml":    "name: echo",
			"docs/README.md": "---\ntitle: Echo\n---\n\nEcho answers.\n",
		} {
			if err := tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(body))}); err != nil {
				t.Fatal(err)
			}
			if _, err := tw.Write([]byte(body)); err != nil {
				t.Fatal(err)
			}
		}
		if err := tw.Close(); err != nil {
			t.Fatal(err)
		}
		return &buf
	}

	errs := make(chan error, 3)
	for _, version := range []string{"v1.0.0", "v1.1.0", "v1.2.0"} {
		go func() {
			dir := t.TempDir()
			errs <- RenderVersion(context.Background(), "../../../docs-builder-template", filepath.Join(dir, "site"), filepath.Join(dir, "out"), "echo", version, archive(), false, log.NewNop())
		}()
	}
	for range 3 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
}

func TestRenderVersionKeepsLinksInVersion(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, body := range map[string]string{
		"module.yaml":           "name: echo",
		"docs/README.md":        "---\ntitle: Echo\n---\n\nSee [the design](/modules/echo/guides/design.html).\n",
		"docs/guides/DESIGN.md": "---\ntitle: Design\n---\n\nSee [the configuration](/modules/echo/configuration.html).\n",
	} {
		if err := tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	if err := RenderVersion(context.Background(), "../../../docs-builder-template", filepath.Join(dir, "site"), out, "echo", "v1.2.0", &buf, false, log.NewNop()); err != nil {
		t.Fatal(err)
	}

	for page, want := range map[string]string{
		"readme.html":        `href="./guides/design.html"`,
		"guides/design.html": `href="../configuration.html"`,
	} {
		html, err := os.ReadFile(filepath.Join(out, "en", page))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(html, []byte(want)) {
			t.Errorf("%s: a link to the own module leaves the version, want %s", page, want)
		}
	}
}
