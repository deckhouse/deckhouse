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

package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/flant/docs-builder/internal/docs"
	"github.com/flant/docs-builder/internal/ondemand"
)

// coldStoreAt is a pod with an empty volume whose template revision is
// revision, on the bucket cold.
func coldStoreAt(t *testing.T, f *fakeRender, cold *memCold, revision string, configure ...func(*Config)) *Store {
	t.Helper()

	onCold := func(cfg *Config) {
		cfg.Cold = cold
		cfg.Revision = func() string { return revision }
	}
	s, _ := newTestStore(t, f, 1<<20, append([]func(*Config){onCold}, configure...)...)
	s.cold.putPause = time.Millisecond

	return s
}

func TestStoreServesArchiveOfOlderTemplate(t *testing.T) {
	cold := newMemCold()
	// Pages large enough for a gzip copy, which must not be written: the
	// pages carry SSI includes.
	f := &fakeRender{size: 4096}

	renderDone(t, coldStoreAt(t, f, cold, "rev1"), "echo", "v1.0.0")
	cold.waitFor(t, "echo/v1.0.0/rev1.tar.gz")

	// A new template: the reader gets the old archive at once.
	s := coldStoreAt(t, f, cold, "rev2")
	renderDone(t, s, "echo", "v1.0.0")
	if n := f.calls.Load(); n != 1 {
		t.Fatalf("renders = %d, want 1: the archive of the older template is served", n)
	}
	if v := s.versions[key("echo", "v1.0.0")]; v == nil || !v.stale {
		t.Fatal("the version from the older archive is not marked stale")
	}

	// The leader renders it anew in the background and replaces both the
	// archive and the published version.
	if _, err := s.rerender.pass(context.Background(), "rev2"); err != nil {
		t.Fatal(err)
	}
	if n := f.calls.Load(); n != 2 {
		t.Fatalf("renders = %d, want 2", n)
	}
	if keys := cold.keys(); !slices.Equal(keys, []string{"echo/v1.0.0/rev2.tar.gz"}) {
		t.Fatalf("archives %v, want only the one of the new template", keys)
	}
	if v := s.versions[key("echo", "v1.0.0")]; v == nil || v.stale {
		t.Fatal("the published version is not replaced by the new render")
	}
	if exists(filepath.Join(s.cfg.Dir, "echo", "v1.0.0", "en", "index.html.gz")) {
		t.Fatal("a gzip copy of a page is written: nginx would send it with the SSI includes unexpanded")
	}

	// Nothing is left to render.
	if _, err := s.rerender.pass(context.Background(), "rev2"); err != nil || f.calls.Load() != 2 {
		t.Fatalf("renders = %d, err %v: want no more", f.calls.Load(), err)
	}
}

func TestRerenderKeepsArchiveTheTemplateCannotRender(t *testing.T) {
	cold := newMemCold()
	f := &fakeRender{size: 10}
	renderDone(t, coldStoreAt(t, f, cold, "rev1"), "echo", "v1.0.0")
	cold.waitFor(t, "echo/v1.0.0/rev1.tar.gz")

	s := coldStoreAt(t, f, cold, "rev2")
	render := s.rerender.render
	var renderErr error
	s.rerender.render = func(ctx context.Context, module, version, workDir, outDir string, withAI bool, stage func(ondemand.Stage)) error {
		if renderErr != nil {
			f.calls.Add(1)
			return renderErr
		}
		return render(ctx, module, version, workDir, outDir, withAI, stage)
	}

	// A failure of the registry or of the pod is tried again on the next
	// pass, the archive is not listed as failed.
	for _, transient := range []error{fmt.Errorf("%w: registry is down", ondemand.ErrFetch), errors.New("no space left on device")} {
		renderErr = transient
		if _, err := s.rerender.pass(context.Background(), "rev2"); err != nil {
			t.Fatal(err)
		}
		if _, ok := cold.objects[rerenderFailedKey("rev2")]; ok {
			t.Fatalf("%v: the archive is listed as one the template can not render", transient)
		}
	}

	// Hugo failing on it is what the template can not do.
	renderErr = fmt.Errorf("%w: template error", docs.ErrBuild)
	if _, err := s.rerender.pass(context.Background(), "rev2"); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(cold.keys(), "echo/v1.0.0/rev1.tar.gz") {
		t.Fatal("the archive of the older template is gone after a failed render")
	}
	var failed rerenderFailed
	if err := json.Unmarshal(cold.objects[rerenderFailedKey("rev2")], &failed); err != nil || !slices.Equal(failed.Archives, []string{"echo/v1.0.0/rev1.tar.gz"}) {
		t.Fatalf("failed renders %+v, err %v", failed, err)
	}

	// Another leader does not try it again with this template.
	f.fail.Store(true)
	calls := f.calls.Load()
	if _, err := coldStoreAt(t, f, cold, "rev2").rerender.pass(context.Background(), "rev2"); err != nil || f.calls.Load() != calls {
		t.Fatalf("renders %d -> %d, err %v: the failed version is rendered again", calls, f.calls.Load(), err)
	}
}

// The archive of a target the template renders anew without the AI export
// stays as it is: replaced, it would leave the followers, which take the
// target from S3 only, without the export.
func TestRerenderKeepsAIExportOfArchive(t *testing.T) {
	cold := newMemCold()
	f := &fakeRender{size: 10}
	old := coldStoreAt(t, f, cold, "rev1")
	if err := old.RenderPinned(context.Background(), ondemand.Target{Module: "echo", Version: "v1.0.0"}); err != nil {
		t.Fatal(err)
	}
	waitBackground(old)

	f.noAI.Store(true)
	s := coldStoreAt(t, f, cold, "rev2")
	if _, err := s.rerender.pass(context.Background(), "rev2"); err != nil {
		t.Fatal(err)
	}
	if keys := cold.keys(); !slices.Equal(keys, []string{"echo/v1.0.0/rev1.ai.tar.gz", rerenderFailedKey("rev2")}) {
		t.Fatalf("archives %v, want the one with the export, listed as failed", keys)
	}
	calls := f.calls.Load()
	if _, err := s.rerender.pass(context.Background(), "rev2"); err != nil || f.calls.Load() != calls {
		t.Fatalf("renders %d -> %d, err %v: the failed version is rendered again", calls, f.calls.Load(), err)
	}
}

func TestRerenderSkipsBranch(t *testing.T) {
	cold := newMemCold()
	f := &fakeRender{size: 10}
	old := coldStoreAt(t, f, cold, "rev1")
	renderDone(t, old, "echo", "v1.0.0")
	renderDone(t, old, "alb", "v1.0.0")
	cold.waitFor(t, "echo/v1.0.0/rev1.tar.gz")
	cold.waitFor(t, "alb/v1.0.0/rev1.tar.gz")

	// The cluster does not answer: nothing is rendered, the next pass asks
	// again.
	s := coldStoreAt(t, f, cold, "rev2")
	s.cold.branch = func(context.Context, string, string) (bool, error) { return false, errors.New("apiserver is down") }
	if _, err := s.rerender.pass(context.Background(), "rev2"); err == nil {
		t.Fatal("the pass goes on while the cluster does not answer")
	}
	if n := f.calls.Load(); n != 2 {
		t.Fatalf("renders = %d, want 2", n)
	}

	// A ModulePullOverride pulls the tag of echo now: its archive goes, and
	// the pass goes on to alb.
	s.cold.branch = func(_ context.Context, module, _ string) (bool, error) { return module == "echo", nil }
	if _, err := s.rerender.pass(context.Background(), "rev2"); err != nil {
		t.Fatal(err)
	}
	if n := f.calls.Load(); n != 3 {
		t.Fatalf("renders = %d, want 3: only alb is rendered anew", n)
	}
	if keys := cold.keys(); !slices.Equal(keys, []string{"alb/v1.0.0/rev2.tar.gz"}) {
		t.Fatalf("archives %v, want only alb of the new template", keys)
	}
}

func TestRerenderWaitsForReaders(t *testing.T) {
	cold := newMemCold()
	f := &fakeRender{size: 10}
	renderDone(t, coldStoreAt(t, f, cold, "rev1"), "echo", "v1.0.0")
	cold.waitFor(t, "echo/v1.0.0/rev1.tar.gz")

	f.release = make(chan struct{})
	s := coldStoreAt(t, f, cold, "rev2")
	s.rerender.idlePoll = 10 * time.Millisecond
	if _, err := s.Start("echo", "v2.0.0"); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { _, err := s.rerender.pass(context.Background(), "rev2"); done <- err }()

	// Many polls of waitIdle pass while the reader's render runs.
	time.Sleep(20 * s.rerender.idlePoll)
	if n := f.calls.Load(); n != 2 {
		t.Fatalf("renders = %d, want 2: the background render waits for the reader", n)
	}

	close(f.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if n := f.calls.Load(); n != 3 {
		t.Fatalf("renders = %d, want 3", n)
	}
}

// Without an archive of this template the newest archive of another one is
// served; a version wanted with the AI export takes only such an archive.
func TestFromColdPicksNewestOlderArchive(t *testing.T) {
	cold := newMemCold()
	f := &fakeRender{size: 10}
	renderDone(t, coldStoreAt(t, f, cold, "rev1"), "echo", "v1.0.0")
	cold.waitFor(t, "echo/v1.0.0/rev1.tar.gz")
	// An archive of rev2 as well, uploaded later.
	cold.mu.Lock()
	cold.objects["echo/v1.0.0/rev2.tar.gz"] = cold.objects["echo/v1.0.0/rev1.tar.gz"]
	cold.modified["echo/v1.0.0/rev2.tar.gz"] = cold.modified["echo/v1.0.0/rev1.tar.gz"].Add(time.Hour)
	cold.mu.Unlock()

	s := coldStoreAt(t, f, cold, "rev3")
	if err := os.MkdirAll(filepath.Join(s.cfg.Dir, tmpDir), 0o700); err != nil {
		t.Fatal(err)
	}
	key, archive, stale := s.cold.fetch(context.Background(), "rev3", "echo", "v1.0.0", filepath.Join(t.TempDir(), "out"), false, coldOrRender)
	if key != "echo/v1.0.0/rev2.tar.gz" || !stale || archive == "" {
		t.Fatalf("key %q, stale %v: want the newest archive of another template", key, stale)
	}

	// No archive with the AI export: nothing for a pinned version.
	if key, _, _ := s.cold.fetch(context.Background(), "rev3", "echo", "v1.0.0", filepath.Join(t.TempDir(), "out"), true, coldOrRender); key != "" {
		t.Fatalf("key %q for a version wanted with the AI export", key)
	}

	// coldFresh takes only an archive of this template.
	if key, _, _ := s.cold.fetch(context.Background(), "rev3", "echo", "v1.0.0", filepath.Join(t.TempDir(), "out"), false, coldFresh); key != "" {
		t.Fatalf("key %q with coldFresh", key)
	}
}

func TestRerenderStopsAfterQuietPasses(t *testing.T) {
	cold := newMemCold()
	f := &fakeRender{}
	renderDone(t, coldStoreAt(t, f, cold, "rev1"), "echo", "v1.0.0")
	cold.waitFor(t, "echo/v1.0.0/rev1.tar.gz")

	// The leader of the new template renders the old archive, then finds
	// nothing in rerenderQuietPasses passes and stops.
	s := coldStoreAt(t, f, cold, "rev2")
	cold.mu.Lock()
	cold.bucketLists = 0
	cold.mu.Unlock()
	done := make(chan struct{})
	go func() { s.Rerender(context.Background(), time.Millisecond); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Rerender runs on after the passes without archives of older templates")
	}
	if n := f.calls.Load(); n != 2 {
		t.Fatalf("renders = %d, want 2: the old archive is rendered anew before it stops", n)
	}
	if keys := cold.keys(); !slices.Equal(keys, []string{"echo/v1.0.0/rev2.tar.gz"}) {
		t.Fatalf("archives %v, want only the one of the new template", keys)
	}
	// The pass that renders, then the quiet ones.
	cold.mu.Lock()
	defer cold.mu.Unlock()
	if want := 1 + rerenderQuietPasses; cold.bucketLists != want {
		t.Fatalf("passes over the bucket = %d, want %d", cold.bucketLists, want)
	}
}

// A list of failed renders that can not be read does not stop the passes of
// the revision: it is taken for an empty one and written anew.
func TestRerenderReplacesFailedListItCannotRead(t *testing.T) {
	cold := newMemCold()
	f := &fakeRender{size: 10}
	renderDone(t, coldStoreAt(t, f, cold, "rev1"), "echo", "v1.0.0")
	cold.waitFor(t, "echo/v1.0.0/rev1.tar.gz")

	cold.mu.Lock()
	cold.objects[rerenderFailedKey("rev2")] = []byte(`{"archives":["echo/v1.0.0/rev1.tar.gz"`)
	cold.mu.Unlock()

	s := coldStoreAt(t, f, cold, "rev2")
	calls := f.calls.Load()
	if _, err := s.rerender.pass(context.Background(), "rev2"); err != nil {
		t.Fatalf("the pass stops on a list of failed renders it can not read: %v", err)
	}
	if n := f.calls.Load(); n != calls+1 {
		t.Fatalf("renders %d -> %d, want the archive of the older template rendered", calls, n)
	}
}
