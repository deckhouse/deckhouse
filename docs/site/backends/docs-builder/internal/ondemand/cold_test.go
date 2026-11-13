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
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/docs"
	"github.com/flant/docs-builder/internal/registry"
)

// memCold is a cold cache in memory.
type memCold struct {
	mu      sync.Mutex
	objects map[string][]byte
	// modified orders the objects by the time they were put.
	modified map[string]time.Time
	failGet  bool
	failPut  bool
	// failPuts is how many of the next puts fail.
	failPuts int
	// notReady answers every call as a cold cache not set up yet.
	notReady bool
	gets     int
	puts     int
	// bucketLists counts the listings of the whole bucket.
	bucketLists int
}

func newMemCold() *memCold {
	return &memCold{objects: make(map[string][]byte), modified: make(map[string]time.Time)}
}

func (m *memCold) List(_ context.Context, prefix string) ([]ColdObject, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if prefix == "" {
		m.bucketLists++
	}
	if m.notReady {
		return nil, ErrColdNotReady
	}
	var out []ColdObject
	for k, b := range m.objects {
		if strings.HasPrefix(k, prefix) {
			out = append(out, ColdObject{Key: k, Size: int64(len(b)), LastModified: m.modified[k]})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func (m *memCold) Get(_ context.Context, key string, w io.Writer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gets++
	if m.notReady {
		return ErrColdNotReady
	}
	if m.failGet {
		return errors.New("S3 is down")
	}
	b, ok := m.objects[key]
	if !ok {
		return ErrColdNotFound
	}
	_, err := w.Write(b)
	return err
}

func (m *memCold) Put(_ context.Context, key, path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.puts++
	if m.notReady {
		return ErrColdNotReady
	}
	if m.failPut {
		return errors.New("S3 is down")
	}
	if m.failPuts > 0 {
		m.failPuts--
		return errors.New("S3 is busy")
	}
	m.objects[key] = b
	m.modified[key] = time.Unix(int64(m.puts-1), 0)
	return nil
}

func (m *memCold) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.notReady {
		return ErrColdNotReady
	}
	delete(m.objects, key)
	return nil
}

func (m *memCold) keys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := make([]string, 0, len(m.objects))
	for k := range m.objects {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (m *memCold) waitFor(t *testing.T, key string) {
	t.Helper()
	m.waitUntil(t, func(keys []string) bool { return slices.Contains(keys, key) })
}

func (m *memCold) waitUntil(t *testing.T, ok func(keys []string) bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok(m.keys()) {
		if time.Now().After(deadline) {
			t.Fatalf("archives: %v", m.keys())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// waitGets waits until S3 was asked for an object n times.
func (m *memCold) waitGets(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		m.mu.Lock()
		gets := m.gets
		m.mu.Unlock()
		if gets >= n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("S3 requests = %d, want %d", gets, n)
		}
		time.Sleep(time.Millisecond)
	}
}

// waitPuts waits until S3 was asked to put an object n times: a reader's
// version is published before its upload starts.
func (m *memCold) waitPuts(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		m.mu.Lock()
		puts := m.puts
		m.mu.Unlock()
		if puts >= n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("S3 uploads = %d, want %d", puts, n)
		}
		time.Sleep(time.Millisecond)
	}
}

func newColdStore(t *testing.T, f *fakeRender, cold *memCold) *Store {
	t.Helper()

	return coldStoreAt(t, f, cold, "rev1")
}

// waitBackground waits for the work the store runs in the background: the
// stale versions looked for anew, then the uploads, which they start too.
func waitBackground(s *Store) {
	s.background.Wait()
	s.cold.background.Wait()
}

func TestStoreKeepsVersionsInColdCache(t *testing.T) {
	cold := newMemCold()
	f := &fakeRender{size: 10}

	s := newColdStore(t, f, cold)
	renderDone(t, s, "echo", "v1.0.0")
	cold.waitFor(t, "echo/v1.0.0/rev1.tar.gz")

	// A restarted pod: an empty volume, the same bucket.
	restarted := newColdStore(t, f, cold)
	renderDone(t, restarted, "echo", "v1.0.0")
	if n := f.calls.Load(); n != 1 {
		t.Fatalf("renders = %d, want 1: the version comes from the cold cache", n)
	}
	if !exists(filepath.Join(restarted.cfg.Dir, siteDir, "en", "modules", "echo", "v1.0.0", "index.html")) {
		t.Fatal("the version from the cold cache is not linked for nginx")
	}
}

func TestStoreColdCacheAndPinnedVersions(t *testing.T) {
	cold := newMemCold()
	f := &fakeRender{size: 10}
	s := newColdStore(t, f, cold)

	// A reader's archive has no AI export: the pinned version is rendered.
	renderDone(t, s, "echo", "v1.0.0")
	cold.waitFor(t, "echo/v1.0.0/rev1.tar.gz")
	if err := s.RenderPinned(context.Background(), Target{Module: "echo", Version: "v1.0.0"}); err != nil {
		t.Fatal(err)
	}
	if n := f.calls.Load(); n != 2 {
		t.Fatalf("renders = %d, want 2", n)
	}
	// The pinned archive holds the reader's one as well, so that one goes.
	cold.waitUntil(t, func(keys []string) bool {
		return slices.Equal(keys, []string{"echo/v1.0.0/rev1.ai.tar.gz"})
	})

	// After a restart a reader gets the pinned archive, AI export included,
	// so the pinner has nothing to render.
	restarted := newColdStore(t, f, cold)
	renderDone(t, restarted, "echo", "v1.0.0")
	if err := restarted.RenderPinned(context.Background(), Target{Module: "echo", Version: "v1.0.0"}); err != nil {
		t.Fatal(err)
	}
	if n := f.calls.Load(); n != 2 {
		t.Fatalf("renders = %d after the restart, want still 2", n)
	}

	// A development branch is not kept: it changes with every push.
	if err := s.RenderPinned(context.Background(), Target{Module: "echo", Version: "main", Rev: "sha256:aaa"}); err != nil {
		t.Fatal(err)
	}
	waitBackground(s)
	for _, k := range cold.keys() {
		if strings.HasPrefix(k, coldVersionPrefix("echo", "main")) {
			t.Fatalf("a branch is uploaded: %s", k)
		}
	}
}

func TestStoreKeepsBranchOutOfColdCache(t *testing.T) {
	cold := newMemCold()
	f := &fakeRender{size: 10}
	counts := func() (int, int) {
		cold.mu.Lock()
		defer cold.mu.Unlock()
		return cold.gets, cold.puts
	}
	newStore := func(isBranch func(context.Context, string, string) (bool, error)) *Store {
		s := newColdStore(t, f, cold)
		s.cold.branch = isBranch
		return s
	}
	// A ModulePullOverride of echo with a tag that looks like a release; a
	// reader opens it before the Pinner knows it.
	override := func(_ context.Context, module, version string) (bool, error) {
		return module == "echo" && version == "v1.0.0", nil
	}

	s := newStore(override)
	renderDone(t, s, "echo", "v1.0.0")
	if g, _ := counts(); g != 0 {
		t.Fatalf("S3 reads = %d for a branch, want 0", g)
	}
	// A release version opened after it shows when uploads are done.
	renderDone(t, s, "alb", "v1.0.0")
	cold.waitFor(t, "alb/v1.0.0/rev1.tar.gz")

	// A restarted pod renders the branch again without asking S3, and so
	// does one that can not tell, because the cluster does not answer.
	for _, check := range []func(context.Context, string, string) (bool, error){
		override,
		func(context.Context, string, string) (bool, error) { return false, errors.New("apiserver is down") },
	} {
		before, _ := counts()
		renderDone(t, newStore(check), "echo", "v1.0.0")
		if g, _ := counts(); g != before {
			t.Fatalf("S3 reads = %d after the restart, want %d", g, before)
		}
	}
	if n := f.calls.Load(); n != 4 {
		t.Fatalf("renders = %d, want 4: the branch is rendered after every restart", n)
	}
	// targets.json written before the branch had a revision: the restore
	// leaves it to a render instead of taking it from S3.
	before, _ := counts()
	if ok, err := newStore(override).restorePinned(context.Background(), Target{Module: "echo", Version: "v1.0.0"}, true); ok || err != nil {
		t.Fatalf("restorePinned = %v, %v for a branch, want false, nil", ok, err)
	}
	if g, _ := counts(); g != before {
		t.Fatalf("S3 reads = %d on the restore, want %d", g, before)
	}
	// Uploads run in the background: the branch would have gone up before
	// the release version that was rendered after it.
	if _, p := counts(); p != 1 {
		t.Fatalf("S3 uploads = %d, want 1", p)
	}
	if keys := cold.keys(); !slices.Equal(keys, []string{"alb/v1.0.0/rev1.tar.gz"}) {
		t.Fatalf("archives %v, want only the release version", keys)
	}
}

func TestStoreKeepsBranchCreatedDuringRenderOutOfColdCache(t *testing.T) {
	cold := newMemCold()
	f := &fakeRender{size: 10, release: make(chan struct{})}
	s := newColdStore(t, f, cold)
	var override atomic.Bool
	s.cold.branch = func(_ context.Context, module, _ string) (bool, error) {
		return module == "echo" && override.Load(), nil
	}

	// A ModulePullOverride appears while the version is rendered.
	if _, err := s.Start("echo", "v1.0.0"); err != nil {
		t.Fatal(err)
	}
	for f.calls.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	override.Store(true)
	close(f.release)
	waitStage(t, s, "echo", "v1.0.0", StageDone)

	// Uploads run in the background: the branch would have gone up before
	// the release version that was rendered after it.
	renderDone(t, s, "alb", "v1.0.0")
	cold.waitFor(t, "alb/v1.0.0/rev1.tar.gz")
	if keys := cold.keys(); !slices.Equal(keys, []string{"alb/v1.0.0/rev1.tar.gz"}) {
		t.Fatalf("archives %v, want only the release version", keys)
	}
}

func TestStoreUploadsRestoredVersionAgain(t *testing.T) {
	cold := newMemCold()
	f := &fakeRender{size: 10}

	renderDone(t, newColdStore(t, f, cold), "echo", "v1.0.0")
	cold.waitFor(t, "echo/v1.0.0/rev1.tar.gz")

	// S3 does not track reads: a restore uploads the archive again, so the
	// eviction sees when the version was last needed.
	renderDone(t, newColdStore(t, f, cold), "echo", "v1.0.0")
	deadline := time.Now().Add(5 * time.Second)
	for {
		cold.mu.Lock()
		puts := cold.puts
		cold.mu.Unlock()
		if puts == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("uploads = %d, want 2", puts)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if n := f.calls.Load(); n != 1 {
		t.Fatalf("renders = %d, want 1", n)
	}
}

// S3 fails a request now and then: an upload is tried again.
func TestStoreUploadsAgainAfterAFailure(t *testing.T) {
	cold := newMemCold()
	cold.failPuts = 2
	s := newColdStore(t, &fakeRender{size: 10}, cold)

	renderDone(t, s, "echo", "v1.0.0")
	waitBackground(s)
	if keys := cold.keys(); !slices.Equal(keys, []string{"echo/v1.0.0/rev1.tar.gz"}) {
		t.Fatalf("archives %v after two failed uploads, want the version", keys)
	}
}

func TestStoreReplacesBrokenColdArchive(t *testing.T) {
	cold := newMemCold()
	f := &fakeRender{size: 10}

	// An archive without pages: an empty tar.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tar.NewWriter(gz).Close()
	gz.Close()
	cold.objects["echo/v1.0.0/rev1.tar.gz"] = buf.Bytes()

	s := newColdStore(t, f, cold)
	renderDone(t, s, "echo", "v1.0.0")
	if n := f.calls.Load(); n != 1 {
		t.Fatalf("renders = %d, want 1: a broken archive is rendered over", n)
	}
	// The rendered version replaces the broken archive.
	cold.waitUntil(t, func([]string) bool {
		cold.mu.Lock()
		defer cold.mu.Unlock()
		return len(cold.objects["echo/v1.0.0/rev1.tar.gz"]) > buf.Len()
	})
}

func TestStoreRendersWhenColdCacheFails(t *testing.T) {
	cold := newMemCold()
	cold.failGet = true
	f := &fakeRender{size: 10}
	s := newColdStore(t, f, cold)

	renderDone(t, s, "echo", "v1.0.0")
	if n := f.calls.Load(); n != 1 {
		t.Fatalf("renders = %d, want 1", n)
	}
	// An unavailable S3 is asked once, not for every key of the version.
	if cold.gets != 1 {
		t.Fatalf("S3 asked %d times, want 1", cold.gets)
	}
}

// A cold cache that is not set up yet has no archive for the store: the
// version is looked for under every key, rendered and uploaded once, without
// the retries of a failing upload. A follower takes it for S3 not answering.
func TestColdCacheNotReady(t *testing.T) {
	cold := newMemCold()
	cold.notReady = true
	f := &fakeRender{size: 10}
	s := newColdStore(t, f, cold)

	renderDone(t, s, "echo", "v1.0.0")
	waitBackground(s)
	if n := f.calls.Load(); n != 1 {
		t.Fatalf("renders = %d, want 1", n)
	}
	if cold.gets != 2 || cold.puts != 1 {
		t.Fatalf("S3 asked for %d archives and %d uploads, want 2 and 1", cold.gets, cold.puts)
	}

	if answered, _ := NewFollower(s, time.Hour, log.NewNop()).Sync(context.Background()); answered {
		t.Fatal("the follower takes a cold cache that is not ready for an answer")
	}
}

func TestHasPages(t *testing.T) {
	dir := t.TempDir()
	if hasPages(dir) {
		t.Fatal("an empty version has pages")
	}

	// Hugo renders the README of a module, the main page of a version, as
	// readme.html: an archive with only it is a whole version.
	if err := os.MkdirAll(filepath.Join(dir, "ru"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ru", "readme.html"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if !hasPages(dir) {
		t.Fatal("a version with readme.html has no pages")
	}
}

func TestUnpackVersionRejectsLargeArchives(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "en/index.html", Typeflag: tar.TypeReg, Mode: 0644, Size: maxColdUnpacked + 1}); err != nil {
		t.Fatal(err)
	}
	tw.Flush()
	gz.Close()

	archive := filepath.Join(t.TempDir(), "a.tar.gz")
	if err := os.WriteFile(archive, buf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := unpackVersion(archive, filepath.Join(t.TempDir(), "out")); !errors.Is(err, errColdTooLarge) {
		t.Fatalf("err = %v, want errColdTooLarge", err)
	}
}

func TestUnpackVersionRejectsPathsOutside(t *testing.T) {
	for _, name := range []string{"../escape.html", "/abs.html", "en/../../escape.html"} {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		if err := tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0644, Size: 1}); err != nil {
			t.Fatal(err)
		}
		tw.Write([]byte("x"))
		tw.Close()
		gz.Close()

		archive := filepath.Join(t.TempDir(), "a.tar.gz")
		if err := os.WriteFile(archive, buf.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		if err := unpackVersion(archive, filepath.Join(t.TempDir(), "out")); err == nil {
			t.Errorf("%s: an entry outside the version is accepted", name)
		}
	}
}

func TestEvictCold(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 11, d, 0, 0, 0, 0, time.UTC) }
	cold := newMemCold()
	for _, o := range []ColdObject{
		// Replaced by the archive of rev2: goes first although it is newer.
		{Key: "e/v1.0.0/rev1.tar.gz", Size: 10, LastModified: day(9)},
		{Key: "e/v1.0.0/rev2.tar.gz", Size: 10, LastModified: day(8)},
		// Not replaced yet: still served, goes with the readers' versions.
		{Key: "a/v1.0.0/rev1.tar.gz", Size: 20, LastModified: day(1)},
		{Key: "b/v1.0.0/rev2.ai.tar.gz", Size: 20, LastModified: day(2)},
		{Key: "c/v1.0.0/rev2.tar.gz", Size: 20, LastModified: day(4)},
		{Key: "d/v1.0.0/rev2.tar.gz", Size: 10, LastModified: day(5)},
		// The version of a release channel: the oldest archive, as the
		// pods serving it do not download it again.
		{Key: "f/v1.0.0/rev2.tar.gz", Size: 10, LastModified: day(0)},
		{Key: "rerender/rev2.json", Size: 1, LastModified: day(1)},
	} {
		cold.objects[o.Key] = make([]byte, o.Size)
		cold.modified[o.Key] = o.LastModified
	}
	cold.objects[indexedFile] = []byte(`{"targets":[{"module":"f","version":"v2.0.0"}],"channels":{"f":{"stable":"v2.0.0","alpha":"v1.0.0"}}}`)
	s := coldStoreAt(t, &fakeRender{size: 10}, cold, "rev2")

	// 100 of 100 is above 80%: down to 60%, the replaced archive first, then
	// the oldest readers' versions; the pinned versions stay although they
	// are older, the one with the AI export and the one of a release channel
	// targets.json names, and the lists stay however old.
	if err := s.EvictCold(context.Background(), 100, 0.8, 0.6); err != nil {
		t.Fatal(err)
	}
	want := []string{"b/v1.0.0/rev2.ai.tar.gz", "d/v1.0.0/rev2.tar.gz", "e/v1.0.0/rev2.tar.gz", "f/v1.0.0/rev2.tar.gz", "rerender/rev2.json", indexedFile}
	if keys := cold.keys(); !slices.Equal(keys, want) {
		t.Fatalf("left %v, want %v", keys, want)
	}

	// Below the high mark nothing goes.
	if err := s.EvictCold(context.Background(), 100, 0.8, 0.6); err != nil || len(cold.keys()) != len(want) {
		t.Fatalf("left %v, err %v", cold.keys(), err)
	}
}

func TestParseColdKey(t *testing.T) {
	for key, want := range map[string]coldArchive{
		coldKey("rev1", "echo", "v1.0.0", false): {module: "echo", version: "v1.0.0", revision: "rev1"},
		coldKey("rev1", "echo", "v1.0.0", true):  {module: "echo", version: "v1.0.0", revision: "rev1", withAI: true},
	} {
		if got, ok := parseColdKey(key); !ok || got != want {
			t.Errorf("parseColdKey(%q) = %+v, %v; want %+v", key, got, ok, want)
		}
	}
	for _, key := range []string{
		"targets.json", "rerender/rev1.json", "echo/v1.0.0/.tar.gz", "a/b/c/d.tar.gz",
		// The module and the version become paths and registry references.
		"../v1.0.0/rev1.tar.gz", "echo/../rev1.tar.gz", "Echo/v1.0.0/rev1.tar.gz", "echo/stable/rev1.tar.gz",
	} {
		if _, ok := parseColdKey(key); ok {
			t.Errorf("parseColdKey(%q) takes it for an archive", key)
		}
	}
}

func TestTemplateRevision(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.html"), []byte("a"), 0600); err != nil {
		t.Fatal(err)
	}
	r1, err := TemplateRevision(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.html"), []byte("b"), 0600); err != nil {
		t.Fatal(err)
	}
	r2, _ := TemplateRevision(dir)
	if r1 == r2 || len(r1) != 12 {
		t.Fatalf("revisions %q and %q: want 12 characters that change with the template", r1, r2)
	}
}

func TestPinnerRestoresFromColdCacheBeforeResolving(t *testing.T) {
	cold := newMemCold()
	f := &fakeRender{size: 10}
	s := newColdStore(t, f, cold)

	var mu sync.Mutex
	var order []string
	render := s.cfg.Render
	s.cfg.Render = func(ctx context.Context, module, version, workDir, outDir string, withAI bool, stage func(Stage)) error {
		mu.Lock()
		order = append(order, module)
		mu.Unlock()
		if module == "empty" {
			return docs.ErrNoPages
		}
		return render(ctx, module, version, workDir, outDir, withAI, stage)
	}

	r := Resolver{
		Modules: func(context.Context) ([]registry.Module, error) {
			return []registry.Module{
				{Name: "alb"},
				{Name: "console", Enabled: true, Override: &registry.Override{Tag: "main", Digest: "sha256:aaa"}},
				{Name: "echo"},
				{Name: "empty"},
			}, nil
		},
		Policy: ClusterPolicy{
			ChannelVersion: func(context.Context, string) (string, error) { return "v1.0.0", nil },
			Logger:         log.NewNop(),
		},
		Logger: log.NewNop(),
	}
	NewPinner(r, s, time.Hour, log.NewNop()).sync(context.Background())

	// A development branch waits for the release versions.
	if want := []string{"alb", "echo", "empty", "console"}; !slices.Equal(order, want) {
		t.Fatalf("render order %v, want %v", order, want)
	}
	cold.waitFor(t, "alb/v1.0.0/rev1.ai.tar.gz")
	cold.waitFor(t, "echo/v1.0.0/rev1.ai.tar.gz")

	// Only what is published is recorded: a version without documentation
	// is not worth restoring.
	cold.mu.Lock()
	var state indexedState
	err := json.Unmarshal(cold.objects["targets.json"], &state)
	saved := state.Targets
	// An archive lost meanwhile (evicted, a failed upload).
	delete(cold.objects, "echo/v1.0.0/rev1.ai.tar.gz")
	cold.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	var modules []string
	for _, t := range saved {
		modules = append(modules, t.Module)
	}
	if want := []string{"alb", "console", "echo"}; !slices.Equal(modules, want) {
		t.Fatalf("recorded %v, want %v", modules, want)
	}

	// A new pod: an empty volume, the registry not answering yet. The
	// release versions and the index come from the cold cache.
	restarted := newColdStore(t, f, cold)
	down := r
	down.Modules = func(context.Context) ([]registry.Module, error) { return nil, errors.New("registry is down") }
	NewPinner(down, restarted, time.Hour, log.NewNop()).sync(context.Background())

	// Neither the branch nor the version missing from S3 is rendered before
	// the registry answers.
	if n := f.calls.Load(); n != 3 {
		t.Fatalf("renders = %d, want 3: the restart renders nothing", n)
	}
	var search struct {
		Documents []struct{ URL string } `json:"documents"`
	}
	readJSON(t, filepath.Join(restarted.cfg.Dir, indexDir, "en", "search.json"), &search)
	var urls []string
	for _, d := range search.Documents {
		urls = append(urls, d.URL)
	}
	if want := []string{"/modules/alb/v1.0.0/"}; !slices.Equal(urls, want) {
		t.Fatalf("index after the restart %v, want %v", urls, want)
	}

	// One more pod, and the registry fails for alb: the module keeps the
	// target it was indexed with instead of leaving the search.
	again := newColdStore(t, f, cold)
	flaky := r
	flaky.Policy = ClusterPolicy{
		ChannelVersion: func(_ context.Context, module string) (string, error) {
			if module == "alb" {
				return "", errors.New("registry hiccup")
			}
			return "v1.0.0", nil
		},
		Logger: log.NewNop(),
	}
	NewPinner(flaky, again, time.Hour, log.NewNop()).sync(context.Background())
	readJSON(t, filepath.Join(again.cfg.Dir, indexDir, "en", "search.json"), &search)
	urls = nil
	for _, d := range search.Documents {
		urls = append(urls, d.URL)
	}
	if !slices.Contains(urls, "/modules/alb/v1.0.0/") {
		t.Fatalf("alb left the index after a registry failure right after a restart: %v", urls)
	}
}

func TestParseIndexed(t *testing.T) {
	echo := []Target{{Module: "echo", Version: "v1.0.0"}}
	for _, tt := range []struct {
		name, raw string
		want      []Target
		err       string
	}{
		{"an object", `{"targets":[{"module":"echo","version":"v1.0.0"}]}`, echo, ""},
		{"a list of targets of an older cluster", ` [{"Module":"echo","Version":"v1.0.0"}]`, echo, ""},
		// The error of the object, not of the list it is not.
		{"a broken object", `{"targets":{}}`, nil, "cannot unmarshal object into Go struct field"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			state, err := parseIndexed([]byte(tt.raw), log.NewNop())
			if tt.err != "" {
				if err == nil || !strings.Contains(err.Error(), tt.err) {
					t.Fatalf("error %v, want %q", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(state.Targets, tt.want) {
				t.Fatalf("targets %+v, want %+v", state.Targets, tt.want)
			}
		})
	}
}

// targets.json comes from outside the pod, and its names become paths and
// registry references: what is not named as one is left out.
func TestParseIndexedSkipsInvalidNames(t *testing.T) {
	raw := `{
		"targets": [{"module":"echo","version":"v1.0.0"}, {"module":"..","version":"v1.0.0"}, {"module":"echo","version":"../x"}],
		"channels": {"echo": {"stable":"v1.0.0", "beta":"..", "nightly":"v1.0.0"}, "../etc": {"stable":"v1.0.0"}},
		"modules": [{"module":"echo"}, {"module":"a/b"}]
	}`
	want := indexedState{
		Targets:  []Target{{Module: "echo", Version: "v1.0.0"}},
		Channels: map[string]Channels{"echo": {"stable": "v1.0.0"}},
		Modules:  []Listed{{Module: "echo"}},
	}

	got, err := parseIndexed([]byte(raw), log.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("state %+v, want %+v", got, want)
	}
}
