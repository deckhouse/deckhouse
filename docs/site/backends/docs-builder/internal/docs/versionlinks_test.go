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
	"fmt"
	"io/fs"
	"math/rand/v2"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/afero"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestVersionLinksFs(t *testing.T) {
	mem := afero.NewMemMapFs()
	vfs := newVersionLinksFs(mem, "/site/public", "echo", "v1.2.0-RC.1")

	for name, tc := range map[string]struct{ in, want string }{
		"/site/public/en/modules/echo/v1.2.0-rc.1/readme.html": {
			in:   `<a href="/en/modules/echo/v1.2.0-rc.1/faq.html">` + `<img src='/modules/echo/v1.2.0-rc.1/images/a.png'>` + `<a href="/modules/prometheus/">`,
			want: `<a href="./faq.html">` + `<img src='./images/a.png'>` + `<a href="/modules/prometheus/">`,
		},
		"/site/public/ru/modules/echo/v1.2.0-rc.1/guides/deep/design.html": {
			in:   `<a href="/ru/modules/echo/v1.2.0-rc.1/">` + `<div data-search-index-path="/ru/modules/echo/v1.2.0-rc.1/search.json">`,
			want: `<a href="../../">` + `<div data-search-index-path="/ru/modules/echo/v1.2.0-rc.1/search.json">`,
		},
		// Not a page of the version: written as is.
		"/site/public/en/modules/echo/v1.2.0-rc.1/search.json": {in: `"/en/modules/echo/v1.2.0-rc.1/x.html"`, want: `"/en/modules/echo/v1.2.0-rc.1/x.html"`},
		"/site/public/en/modules/index.html":                   {in: `href="/en/modules/echo/v1.2.0-rc.1/"`, want: `href="/en/modules/echo/v1.2.0-rc.1/"`},
	} {
		// Hugo creates the files today; OpenFile for writing is rewritten too.
		var f afero.File
		var err error
		if strings.HasSuffix(name, "design.html") {
			f, err = vfs.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		} else {
			f, err = vfs.Create(name)
		}
		if err != nil {
			t.Fatal(err)
		}
		// A link split between two writes is rewritten all the same.
		half := len(tc.in) / 2
		if _, err := f.Write([]byte(tc.in[:half])); err != nil {
			t.Fatal(err)
		}
		if _, err := f.WriteString(tc.in[half:]); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}

		got, err := afero.ReadFile(mem, name)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != tc.want {
			t.Errorf("%s:\n got %s\nwant %s", name, got, tc.want)
		}
	}
}

var linkAttr = regexp.MustCompile(`\b(?:href|src)="([^"#]*)"`)

// A rendered version serves its own URL and the URL of a channel that points
// at it, on the site (no language in the path) and in the cluster: every link
// to the module stays under the URL the page was opened at and finds a page.
func TestRenderVersionServesChannelURLs(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, body := range map[string]string{
		"module.yaml":              "name: echo",
		"docs/README.md":           "---\ntitle: Echo\n---\n\nSee [the design](/modules/echo/guides/design.html) and [Prometheus](/modules/prometheus/).\n",
		"docs/README_RU.md":        "---\ntitle: Echo\n---\n\nСм. [устройство](/modules/echo/guides/design.html).\n",
		"docs/CONFIGURATION.md":    "---\ntitle: Configuration\n---\n\nSettings.\n",
		"docs/guides/DESIGN.md":    "---\ntitle: Design\n---\n\nSee [the configuration](/modules/echo/configuration.html) and [the module](/modules/echo/).\n",
		"docs/CONFIGURATION_RU.md": "---\ntitle: Настройки\n---\n\nНастройки.\n",
		"docs/guides/DESIGN_RU.md": "---\ntitle: Устройство\n---\n\nСм. [настройки](/modules/echo/configuration.html).\n",
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

	for _, lang := range Languages {
		root := filepath.Join(out, lang)
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Ext(path) != ".html" {
				return err
			}
			page, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			page = filepath.ToSlash(page)
			html, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			for _, m := range linkAttr.FindAllSubmatch(html, -1) {
				link := string(m[1])
				if strings.Contains(link, "/modules/echo/v1.2.0") {
					t.Errorf("%s/%s: link %q names the version", lang, page, link)
					continue
				}
				ref, err := url.Parse(link)
				if err != nil || ref.IsAbs() || strings.HasPrefix(ref.Path, "/") {
					continue
				}
				for _, base := range []string{"/modules/echo/stable/", "/modules/echo/v1.2.0/", "/" + lang + "/modules/echo/v1.2.0/"} {
					at, _ := url.Parse(base + page)
					target := at.ResolveReference(ref).Path
					rest, ok := strings.CutPrefix(target, base)
					if !ok && target+"/" != base {
						t.Errorf("%s/%s opened at %s: link %q leaves it for %s", lang, page, base, link, target)
						continue
					}
					// The main page of a directory is readme.html (nginx index).
					if rest == "" || strings.HasSuffix(rest, "/") {
						rest += "readme.html"
					}
					if _, err := os.Stat(filepath.Join(root, rest)); err != nil {
						t.Errorf("%s/%s opened at %s: link %q finds no page %s", lang, page, base, link, rest)
					}
				}
			}

			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestRewriteOwnLinksMatchesThePattern(t *testing.T) {
	const module, version = "echo", "v1.2.0"
	own := regexp.MustCompile(fmt.Sprintf(ownLinkPattern, strings.Join(Languages, "|"), regexp.QuoteMeta(module), regexp.QuoteMeta(version)))
	pieces := []string{
		`href="`, `href='`, `src="`, `src='`, `xhref="`, `data-href="`, `HREF="`, ` `, `"`, `/en`, `/ru`, `/de`,
		`/modules/echo/v1.2.0/`, `/modules/echo/v1.2.0`, `/modules/echo/v1.3.0/`, `/modules/echoes/v1.2.0/`,
		`page.html`, `<a `, `>`, `x`, `_`, `é`, `https://a`, `/modules/Echo/v1.2.0/`,
	}
	rnd := rand.New(rand.NewPCG(1, 2))
	// One buffer for every page, as the pool gives it.
	var dst []byte
	for range 20000 {
		var b strings.Builder
		for range rnd.IntN(12) {
			b.WriteString(pieces[rnd.IntN(len(pieces))])
		}
		page := []byte(b.String())
		// A deep page: the rewritten page is longer than the original.
		for _, root := range []string{"../", "./", "", strings.Repeat("../", 6)} {
			want := own.ReplaceAll(page, []byte("${1}"+root))
			got, rewritten := rewriteOwnLinks(dst, page, []byte("/modules/"+module+"/"+version+"/"), root)
			if !bytes.Equal(got, want) {
				t.Fatalf("page %q, root %q:\n got %q\nwant %q", page, root, got, want)
			}
			if rewritten {
				dst = got[:0]
			}
		}
	}
}

func BenchmarkRewriteOwnLinks(b *testing.B) {
	page := []byte(strings.Repeat(`<p>Text <a href="/en/modules/echo/v1.2.0/faq.html">FAQ</a> and <a href="https://example.com/">out</a>.</p>`+"\n", 2000))
	own := []byte("/modules/echo/v1.2.0/")
	for b.Loop() {
		rewriteOwnLinks(nil, page, own, "../")
	}
}
