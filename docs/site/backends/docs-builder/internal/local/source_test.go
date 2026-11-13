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

package local

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/ondemand"
)

// writeFiles writes the files under dir, making the directories.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func echoFiles() map[string]string {
	return map[string]string{
		"module.yaml":                                     "name: echo\nstage: Preview\n",
		"docs/README.md":                                  "---\ntitle: Echo\ndescription: Echo module.\n---\n\nEcho answers.\n",
		"docs/README_RU.md":                               "---\ntitle: Echo\n---\n\nEcho answers in Russian.\n",
		"openapi/config-values.yaml":                      "type: object\n",
		"crds/echo.yaml":                                  "apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\n",
		".git/HEAD":                                       "ref: refs/heads/main\n",
		"docs/.hidden.md":                                 "hidden\n",
		"images/echo/Dockerfile":                          "FROM scratch\n",
		"hooks/echo.go":                                   "package hooks\n",
		"crds/doc-ru-echo.yaml":                           "spec: {}\n",
		"openapi/doc-ru-config-values.yaml":               "type: object\n",
		"candi/openapi/cluster_configuration.yaml":        "kind: DVPClusterConfiguration\n",
		"candi/openapi/doc-ru-cluster_configuration.yaml": "apiVersions: []\n",
		"candi/openapi/cloud_discovery_data.yaml":         "type: object\n",
	}
}

func TestModulesAreFoundOneOrTwoLevelsDeep(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, filepath.Join(dir, "echo"), echoFiles())
	writeFiles(t, filepath.Join(dir, "repos", "ping"), map[string]string{"module.yaml": "name: ping\n", "docs/README.md": "# Ping\n"})
	// Not modules: a hidden directory, a module.yaml without a name, a name
	// that is not a page path.
	writeFiles(t, filepath.Join(dir, ".cache", "x"), map[string]string{"module.yaml": "name: x\n"})
	writeFiles(t, filepath.Join(dir, "nameless"), map[string]string{"module.yaml": "stage: Preview\n"})
	writeFiles(t, filepath.Join(dir, "upper"), map[string]string{"module.yaml": "name: Upper\n"})

	s, err := New(dir, "v0.0.0-local", log.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	modules, err := s.Modules(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) != 2 || modules[0].Name != "echo" || modules[1].Name != "ping" {
		t.Fatalf("modules = %+v, want echo and ping", modules)
	}
	for _, m := range modules {
		if !m.Enabled || m.Override == nil || m.Override.Tag != "v0.0.0-local" || m.Override.Digest == "" {
			t.Errorf("module %s = %+v, want enabled with the version as an override with a digest", m.Name, m)
		}
	}

	tags, err := s.Tags(context.Background(), "echo")
	if err != nil || len(tags) != 1 || tags[0] != "v0.0.0-local" {
		t.Errorf("Tags(echo) = %v, %v", tags, err)
	}
	if v, err := s.Version(context.Background(), "ping"); err != nil || v != "v0.0.0-local" {
		t.Errorf("Version(ping) = %q, %v", v, err)
	}
	if _, err := s.Tags(context.Background(), "none"); !ondemand.Unpublished(err) {
		t.Errorf("Tags(none) = %v, want an unpublished module", err)
	}
	if _, err := s.Version(context.Background(), "none"); !ondemand.Unpublished(err) {
		t.Errorf("Version(none) = %v, want an unpublished module", err)
	}
}

func TestNewRejectsWhatIsNotADirectoryOrAVersion(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := New(file, "v0.0.0-local", log.NewNop()); err == nil {
		t.Error("New(file) = nil, want an error")
	}
	if _, err := New(filepath.Dir(file), "stable", log.NewNop()); err == nil {
		t.Error("New(dir, stable) = nil, want an error")
	}
	if _, err := New(filepath.Join(t.TempDir(), "none"), "v0.0.0-local", log.NewNop()); err == nil {
		t.Error("New(missing dir) = nil, want an error")
	}
}

func TestRevisionFollowsTheFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "echo")
	writeFiles(t, dir, echoFiles())
	s, err := New(filepath.Dir(dir), "v0.0.0-local", log.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	rev := func() string {
		t.Helper()
		revs, err := s.revisions()
		if err != nil {
			t.Fatal(err)
		}
		return revs["echo"]
	}

	first := rev()
	if first != rev() {
		t.Fatal("the revision of unchanged files changed")
	}

	// A file outside of the documentation does not count.
	writeFiles(t, dir, map[string]string{"hooks/echo.go": "package hooks // changed\n"})
	if rev() != first {
		t.Error("a change of hooks/ changed the revision")
	}
	writeFiles(t, dir, map[string]string{"docs/.hidden.md": "changed\n"})
	if rev() != first {
		t.Error("a change of a hidden file changed the revision")
	}

	// A write with the same size and a later mtime.
	writeFiles(t, dir, map[string]string{"docs/README_RU.md": "---\ntitle: Echo\n---\n\nEcho answers in Russian!\n"})
	later := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(filepath.Join(dir, "docs", "README_RU.md"), later, later); err != nil {
		t.Fatal(err)
	}
	second := rev()
	if second == first {
		t.Error("a write of a page did not change the revision")
	}

	writeFiles(t, dir, map[string]string{"docs/FAQ.md": "# FAQ\n"})
	if third := rev(); third == second || third == first {
		t.Error("a new page did not change the revision")
	}
}

func TestArchiveHoldsTheDocumentationFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "echo")
	writeFiles(t, dir, echoFiles())

	var buf bytes.Buffer
	if err := writeArchive(&buf, dir); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	tr := tar.NewReader(&buf)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		got[h.Name] = string(body)
	}

	want := echoFiles()
	for _, name := range []string{".git/HEAD", "docs/.hidden.md", "images/echo/Dockerfile", "hooks/echo.go", "candi/openapi/cloud_discovery_data.yaml"} {
		delete(want, name)
	}
	if len(got) != len(want) {
		t.Errorf("archive has %d files, want %d: %v", len(got), len(want), got)
	}
	for name, body := range want {
		if got[name] != body {
			t.Errorf("archive %s = %q, want %q", name, got[name], body)
		}
	}
}

func TestRenderBuildsThePagesOfTheModule(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, filepath.Join(dir, "echo"), echoFiles())
	s, err := New(dir, "v0.0.0-local", log.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	render := s.Render("../../../docs-builder-template")

	work := t.TempDir()
	out := filepath.Join(work, "out")
	var stages []ondemand.Stage
	err = render(context.Background(), "echo", "v0.0.0-local", work, out, false, func(st ondemand.Stage) { stages = append(stages, st) })
	if err != nil {
		t.Fatal(err)
	}
	if len(stages) != 1 || stages[0] != ondemand.StageRender {
		t.Errorf("stages = %v, want render only", stages)
	}
	for _, lang := range []string{"en", "ru"} {
		page, err := os.ReadFile(filepath.Join(out, lang, "readme.html"))
		if err != nil {
			t.Fatalf("%s page: %v", lang, err)
		}
		if !bytes.Contains(page, []byte("Echo answers")) {
			t.Errorf("%s page does not hold the text of README", lang)
		}
	}

	for _, tc := range []struct{ module, version string }{{"echo", "v1.0.0"}, {"none", "v0.0.0-local"}} {
		err := render(context.Background(), tc.module, tc.version, t.TempDir(), filepath.Join(t.TempDir(), "out"), false, func(ondemand.Stage) {})
		if !errors.Is(err, ondemand.ErrVersionNotFound) {
			t.Errorf("render %s %s = %v, want ErrVersionNotFound", tc.module, tc.version, err)
		}
	}
}

func TestWatchReportsAChange(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, filepath.Join(dir, "echo"), echoFiles())
	s, err := New(dir, "v0.0.0-local", log.NewNop())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	changed := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Watch(ctx, 10*time.Millisecond, func() {
			select {
			case changed <- struct{}{}:
			default:
			}
		})
	}()

	select {
	case <-changed:
		t.Fatal("a change was reported before any")
	case <-time.After(50 * time.Millisecond):
	}

	writeFiles(t, filepath.Join(dir, "echo"), map[string]string{"docs/FAQ.md": "# FAQ\n"})
	select {
	case <-changed:
	case <-time.After(5 * time.Second):
		t.Fatal("the new page was not reported")
	}

	// A module added later is a change too.
	writeFiles(t, filepath.Join(dir, "ping"), map[string]string{"module.yaml": "name: ping\n", "docs/README.md": "# Ping\n"})
	select {
	case <-changed:
	case <-time.After(5 * time.Second):
		t.Fatal("the new module was not reported")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Watch did not stop")
	}
}
