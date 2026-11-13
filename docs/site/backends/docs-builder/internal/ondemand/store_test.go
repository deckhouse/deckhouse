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
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/flant/docs-builder/internal/docs"
	"github.com/flant/docs-builder/internal/metrics"
)

// fakeRender writes size bytes of pages for every version it renders.
type fakeRender struct {
	size    int
	calls   atomic.Int32
	release chan struct{} // when set, a render waits for it
	fail    atomic.Bool
	failErr error // what a failing render returns, set before it runs
	// noAI leaves the AI export out of every render, as when it fails.
	noAI atomic.Bool
}

func (f *fakeRender) render(_ context.Context, module, version, _, outDir string, withAI bool, stage func(Stage)) error {
	f.calls.Add(1)
	stage(StageDownload)
	if f.release != nil {
		<-f.release
	}
	stage(StageRender)

	files := map[string]string{
		"index.html":  strings.Repeat("x", f.size),
		"search.json": fmt.Sprintf(`{"documents":[{"url":"/modules/%s/%s/"}],"parameters":[{"module":%q}]}`, module, version, module),
	}
	if withAI && !f.noAI.Load() {
		files["llms.txt"] = fmt.Sprintf("# Modules\n\n> Intro.\n\n## %s\n- [Overview](https://deckhouse.io/modules/%s/%s/index.md)\n\n", module, module, version)
		files["corpus.json"] = fmt.Sprintf(`{"version":1,"lang":"en","documents":[{"module":%q,"version":%q}]}`, module, version)
	}

	if err := os.MkdirAll(filepath.Join(outDir, "en"), 0700); err != nil {
		return err
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(outDir, "en", name), []byte(body), 0600); err != nil {
			return err
		}
	}

	if f.fail.Load() {
		if f.failErr != nil {
			return f.failErr
		}
		return errors.New("broken module")
	}

	return nil
}

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func newTestStore(t *testing.T, f *fakeRender, maxSize int64) (*Store, *clock) {
	t.Helper()

	c := &clock{t: time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)}
	s := NewStore(Config{
		Dir:                t.TempDir(),
		MaxSize:            maxSize,
		MaxIdle:            time.Hour,
		MaxRenders:         2,
		MaxQueue:           3,
		RenderTimeout:      time.Minute,
		RetryAfter:         time.Minute,
		NotFoundRetryAfter: time.Hour,
		Render:             f.render,
		Logger:             log.NewNop(),
	})
	s.now = c.now

	return s, c
}

func waitStage(t *testing.T, s *Store, module, version string, want Stage) Status {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		st := s.Status(module, version)
		if st.Stage == want {
			return st
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s %s: stage %q, want %q", module, version, st.Stage, want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func renderDone(t *testing.T, s *Store, module, version string) {
	t.Helper()

	if _, err := s.Start(module, version); err != nil {
		t.Fatal(err)
	}
	waitStage(t, s, module, version, StageDone)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestStoreRendersAndPublishes(t *testing.T) {
	f := &fakeRender{size: 10}
	s, _ := newTestStore(t, f, 1000)

	if st := s.Status("echo", "v1.2.3"); st.Stage != "" {
		t.Fatalf("unknown version: stage %q", st.Stage)
	}

	renderDone(t, s, "echo", "v1.2.3")

	if !exists(filepath.Join(s.cfg.Dir, "echo", "v1.2.3", "en", "index.html")) {
		t.Fatal("rendered page is not published")
	}

	// nginx reads the page under the URL layout through a link.
	if !exists(filepath.Join(s.cfg.Dir, siteDir, "en", "modules", "echo", "v1.2.3", "index.html")) {
		t.Fatal("rendered page is not linked for nginx")
	}
	if exists(filepath.Join(s.cfg.Dir, siteDir, "ru", "modules", "echo", "v1.2.3")) {
		t.Fatal("a language the version does not have is linked")
	}

	if st, _ := s.Start("echo", "v1.2.3"); st.Stage != StageDone || f.calls.Load() != 1 {
		t.Fatalf("second start: stage %q, renders %d", st.Stage, f.calls.Load())
	}

	if entries, _ := os.ReadDir(filepath.Join(s.cfg.Dir, tmpDir)); len(entries) != 0 {
		t.Fatalf("work dirs left: %v", entries)
	}
}

func TestStoreSharesOneRenderPerVersion(t *testing.T) {
	f := &fakeRender{size: 10, release: make(chan struct{})}
	s, _ := newTestStore(t, f, 1000)

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Start("echo", "v1.2.3"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	waitStage(t, s, "echo", "v1.2.3", StageDownload)
	close(f.release)
	waitStage(t, s, "echo", "v1.2.3", StageDone)

	if n := f.calls.Load(); n != 1 {
		t.Fatalf("renders = %d, want 1", n)
	}
}

func TestStoreFailedRender(t *testing.T) {
	f := &fakeRender{size: 10}
	f.fail.Store(true)
	s, c := newTestStore(t, f, 1000)

	if _, err := s.Start("echo", "v1.2.3"); err != nil {
		t.Fatal(err)
	}
	st := waitStage(t, s, "echo", "v1.2.3", StageFailed)
	if st.Error != ReasonBuild {
		t.Fatalf("error = %q", st.Error)
	}
	if st.RetryIn != 60 {
		t.Fatalf("retry in %d s, want 60", st.RetryIn)
	}
	c.add(30*time.Second + time.Millisecond)
	if st := s.Status("echo", "v1.2.3"); st.RetryIn != 30 {
		t.Fatalf("retry in %d s after 30 s, want 30", st.RetryIn)
	}
	c.add(-30*time.Second - time.Millisecond)

	if exists(filepath.Join(s.cfg.Dir, "echo", "v1.2.3")) {
		t.Fatal("a failed render is published")
	}

	if st, _ := s.Start("echo", "v1.2.3"); st.Stage != StageFailed || f.calls.Load() != 1 {
		t.Fatalf("start before RetryAfter: stage %q, renders %d", st.Stage, f.calls.Load())
	}

	f.fail.Store(false)
	c.add(time.Minute)
	renderDone(t, s, "echo", "v1.2.3")

	if n := f.calls.Load(); n != 2 {
		t.Fatalf("renders = %d, want 2", n)
	}
}

func TestStoreForgetsOldFailures(t *testing.T) {
	f := &fakeRender{size: 10}
	f.fail.Store(true)
	s, c := newTestStore(t, f, 1000)

	if _, err := s.Start("echo", "v1.2.3"); err != nil {
		t.Fatal(err)
	}
	waitStage(t, s, "echo", "v1.2.3", StageFailed)

	c.add(time.Minute)
	s.evict()

	if st := s.Status("echo", "v1.2.3"); st.Stage != "" {
		t.Fatalf("stage %q after RetryAfter, want forgotten", st.Stage)
	}
}

func TestStoreRejectsInvalidVersion(t *testing.T) {
	s, _ := newTestStore(t, &fakeRender{}, 1000)

	for _, v := range []string{"..", "stable", "v1.2", "v1.2.3/../x"} {
		if _, err := s.Start("echo", v); err == nil {
			t.Errorf("version %q accepted", v)
		}
	}
}

func TestStoreEvictsLeastRecentlyOpenedAboveMaxSize(t *testing.T) {
	f := &fakeRender{size: 100}
	s, c := newTestStore(t, f, 400) // two versions of about 170 bytes fit, three do not

	renderDone(t, s, "echo", "v1.0.0")
	c.add(time.Second)
	renderDone(t, s, "echo", "v2.0.0")
	c.add(time.Second)
	s.Touch("echo", "v1.0.0") // v2.0.0 is now the least recently opened
	c.add(time.Second)
	renderDone(t, s, "echo", "v3.0.0")
	// The render evicts right after it publishes; do it here so the check does
	// not race with it.
	s.evict()

	for v, want := range map[string]bool{"v1.0.0": true, "v2.0.0": false, "v3.0.0": true} {
		if got := exists(filepath.Join(s.cfg.Dir, "echo", v)); got != want {
			t.Errorf("%s on disk = %v, want %v", v, got, want)
		}
		if _, err := os.Lstat(filepath.Join(s.cfg.Dir, siteDir, "en", "modules", "echo", v)); (err == nil) != want {
			t.Errorf("%s linked = %v, want %v", v, err == nil, want)
		}
		if got := s.Status("echo", v).Stage == StageDone; got != want {
			t.Errorf("%s in store = %v, want %v", v, got, want)
		}
	}

	if entries, _ := os.ReadDir(filepath.Join(s.cfg.Dir, trashDir)); len(entries) != 0 {
		t.Fatalf("trash left: %v", entries)
	}
}

func TestStoreEvictsIdle(t *testing.T) {
	f := &fakeRender{size: 10}
	s, c := newTestStore(t, f, 1000)

	renderDone(t, s, "echo", "v1.0.0")
	c.add(30 * time.Minute)
	renderDone(t, s, "echo", "v2.0.0")
	c.add(31 * time.Minute)

	s.evict()

	if exists(filepath.Join(s.cfg.Dir, "echo", "v1.0.0")) {
		t.Error("idle v1.0.0 is kept")
	}
	if !exists(filepath.Join(s.cfg.Dir, "echo", "v2.0.0")) {
		t.Error("v2.0.0 is evicted before MaxIdle")
	}
}

// After a restart the pinned versions are not known until the first Pin:
// nothing is evicted before it, for firstPinWait at most.
func TestStoreWaitsForPinBeforeEvicting(t *testing.T) {
	restarted := func(t *testing.T) (*Store, *clock) {
		t.Helper()

		s, c := newTestStore(t, &fakeRender{size: 10}, 1000)
		renderDone(t, s, "echo", "v1.0.0")

		cfg := s.cfg
		cfg.WaitForPin = true
		cfg.MaxIdle = time.Minute
		s = NewStore(cfg)
		s.now, s.started = c.now, c.now()
		if err := s.Load(); err != nil {
			t.Fatal(err)
		}
		// Load takes the time of the files for the last access, which is
		// not the time of the clock here.
		s.Touch("echo", "v1.0.0")

		c.add(2 * cfg.MaxIdle)
		s.evict()
		if st := s.Status("echo", "v1.0.0"); st.Stage != StageDone {
			t.Fatalf("stage %q before the first Pin, want done: the idle version is evicted", st.Stage)
		}

		return s, c
	}

	t.Run("until the first Pin", func(t *testing.T) {
		s, _ := restarted(t)
		s.Pin(nil, nil)
		s.evict()
		if exists(filepath.Join(s.cfg.Dir, "echo", "v1.0.0")) {
			t.Fatal("the idle version that is not pinned is kept after the first Pin")
		}
	})

	t.Run("a pinned version stays", func(t *testing.T) {
		s, _ := restarted(t)
		s.Pin([]Target{{Module: "echo", Version: "v1.0.0"}}, nil)
		s.evict()
		if !exists(filepath.Join(s.cfg.Dir, "echo", "v1.0.0")) {
			t.Fatal("the pinned version is evicted")
		}
	})

	t.Run("until the wait is over", func(t *testing.T) {
		s, c := restarted(t)
		c.add(firstPinWait)
		s.evict()
		if exists(filepath.Join(s.cfg.Dir, "echo", "v1.0.0")) {
			t.Fatal("the idle version is kept after firstPinWait without a Pin")
		}
	})
}

func TestStoreLoad(t *testing.T) {
	s, _ := newTestStore(t, &fakeRender{}, 1000)
	dir := s.cfg.Dir

	for _, p := range []string{
		"echo/v1.2.3/en/index.html",
		"console/v1.70.5/ru/index.html",
		"echo/.partial/en/index.html", // not a version
		tmpDir + "/echo-v9.9.9-1/out/en/index.html",
		trashDir + "/echo-v0.1.0-1/version/en/index.html",
		siteDir + "/en/modules/gone/v0.0.1/index.html", // a link tree of a removed version
	} {
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(p)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, p), []byte("page"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	if err := s.Load(); err != nil {
		t.Fatal(err)
	}

	for _, kv := range [][2]string{{"echo", "v1.2.3"}, {"console", "v1.70.5"}} {
		if st := s.Status(kv[0], kv[1]); st.Stage != StageDone {
			t.Errorf("%s %s: stage %q, want done", kv[0], kv[1], st.Stage)
		}
	}
	if len(s.versions) != 2 {
		t.Errorf("versions = %d, want 2", len(s.versions))
	}
	if s.versions["echo/v1.2.3"].size != 4 {
		t.Errorf("size = %d, want 4", s.versions["echo/v1.2.3"].size)
	}
	for _, d := range []string{tmpDir, trashDir} {
		if exists(filepath.Join(dir, d)) {
			t.Errorf("%s is not cleaned", d)
		}
	}

	for _, p := range []string{"en/modules/echo/v1.2.3/index.html", "ru/modules/console/v1.70.5/index.html"} {
		if !exists(filepath.Join(dir, siteDir, p)) {
			t.Errorf("%s is not linked after Load", p)
		}
	}
	if exists(filepath.Join(dir, siteDir, "en", "modules", "gone")) {
		t.Error("links of a removed version are kept")
	}
	for _, d := range []string{siteDir + ".new", siteDir + ".old"} {
		if exists(filepath.Join(dir, d)) {
			t.Errorf("%s is left after the link tree was replaced", d)
		}
	}

	// Links of published versions now go to the served tree again.
	renderDone(t, s, "echo", "v2.0.0")
	if !exists(filepath.Join(dir, siteDir, "en", "modules", "echo", "v2.0.0")) {
		t.Error("a version rendered after Load is not linked in the served tree")
	}
}

func TestStoreRefusesRendersAboveMaxQueue(t *testing.T) {
	f := &fakeRender{size: 10, release: make(chan struct{})}
	s, _ := newTestStore(t, f, 1000)

	for _, v := range []string{"v1.0.0", "v2.0.0", "v3.0.0"} {
		if _, err := s.Start("echo", v); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Start("echo", "v4.0.0"); !errors.Is(err, ErrBusy) {
		t.Fatalf("err = %v, want ErrBusy", err)
	}
	// A version already in the queue still reports its status.
	if _, err := s.Start("echo", "v1.0.0"); err != nil {
		t.Fatal(err)
	}

	close(f.release)
	waitStage(t, s, "echo", "v3.0.0", StageDone)
	for _, v := range []string{"v1.0.0", "v2.0.0"} {
		waitStage(t, s, "echo", v, StageDone)
	}
	if _, err := s.Start("echo", "v4.0.0"); err != nil {
		t.Fatalf("after the queue drained: %v", err)
	}
	waitStage(t, s, "echo", "v4.0.0", StageDone)
}

func TestStoreKeepsVersionLinkedWhenEvictionFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root renames out of a read-only directory")
	}

	f := &fakeRender{size: 10}
	s, c := newTestStore(t, f, 1000)
	renderDone(t, s, "echo", "v1.0.0")

	// The version can not be moved out of its module directory.
	moduleDir := filepath.Join(s.cfg.Dir, "echo")
	if err := os.Chmod(moduleDir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(moduleDir, 0755) })

	c.add(2 * time.Hour)
	s.evict()

	if !exists(filepath.Join(s.cfg.Dir, siteDir, "en", "modules", "echo", "v1.0.0", "index.html")) {
		t.Fatal("a version that failed to be evicted is no longer linked for nginx")
	}
	if st := s.Status("echo", "v1.0.0"); st.Stage != StageDone {
		t.Fatalf("stage %q, want done", st.Stage)
	}
}

func TestStoreKeepsNotFoundLonger(t *testing.T) {
	f := &fakeRender{size: 10}
	s, c := newTestStore(t, f, 1000)
	s.cfg.Render = func(context.Context, string, string, string, string, bool, func(Stage)) error {
		f.calls.Add(1)
		return fmt.Errorf("source deckhouse: %w", ErrVersionNotFound)
	}

	if _, err := s.Start("echo", "v9.9.9"); err != nil {
		t.Fatal(err)
	}
	waitStage(t, s, "echo", "v9.9.9", StageFailed)

	c.add(time.Minute) // RetryAfter of a broken render
	s.evict()
	if st, _ := s.Start("echo", "v9.9.9"); st.Stage != StageFailed || !st.NotFound || f.calls.Load() != 1 {
		t.Fatalf("after RetryAfter: %+v, renders %d; want the not-found result kept", st, f.calls.Load())
	}

	c.add(time.Hour)
	if _, err := s.Start("echo", "v9.9.9"); err != nil {
		t.Fatal(err)
	}
	waitStage(t, s, "echo", "v9.9.9", StageFailed)
	if n := f.calls.Load(); n != 2 {
		t.Fatalf("renders = %d after NotFoundRetryAfter, want 2", n)
	}
}

// A development branch failing for a lasting reason is counted until it
// renders, publishes without the AI export, or stops being a target.
func TestStoreCountsFailedBranches(t *testing.T) {
	ms := metricsstorage.NewMetricStorage(metricsstorage.WithNewRegistry())
	if err := metrics.RegisterMetrics(ms); err != nil {
		t.Fatal(err)
	}
	f := &fakeRender{size: 10}
	s, _ := newTestStore(t, f, 1000)
	s.cfg.Metrics = ms
	var mode atomic.Value
	mode.Store("broken")
	s.cfg.Render = func(ctx context.Context, module, version, workDir, outDir string, withAI bool, stage func(Stage)) error {
		switch mode.Load() {
		case "broken":
			return fmt.Errorf("hugo: %w", docs.ErrBuild)
		case "no AI":
			withAI = false
		}
		return f.render(ctx, module, version, workDir, outDir, withAI, stage)
	}
	gauge := func() string {
		rec := httptest.NewRecorder()
		ms.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
		for _, line := range strings.Split(rec.Body.String(), "\n") {
			if v, ok := strings.CutPrefix(line, metrics.DocsBuilderOnDemandFailedBranches+" "); ok {
				return v
			}
		}
		return ""
	}
	branch := Target{Module: "echo", Version: "main", Rev: "sha256:aaa"}
	fail := func() {
		t.Helper()
		mode.Store("broken")
		s.SetTargets([]Target{branch})
		if err := s.RenderPinned(context.Background(), branch); err == nil {
			t.Fatal("the broken branch rendered")
		}
		if _, failed := s.BranchFailure(branch); !failed || gauge() != "1" {
			t.Fatalf("failed branch: recorded %v, gauge %q", failed, gauge())
		}
	}

	fail()
	s.SetTargets(nil)
	if _, failed := s.BranchFailure(branch); failed || gauge() != "0" {
		t.Fatalf("no target any more: recorded %v, gauge %q", failed, gauge())
	}

	fail()
	mode.Store("no AI")
	if err := s.RenderPinned(context.Background(), branch); !errors.Is(err, errNoAIExport) {
		t.Fatalf("err = %v, want errNoAIExport", err)
	}
	if _, failed := s.BranchFailure(branch); failed || gauge() != "0" {
		t.Fatalf("published without the AI export: recorded %v, gauge %q", failed, gauge())
	}
}

func metricValue(t *testing.T, ms *metricsstorage.MetricStorage, name string) string {
	t.Helper()

	rec := httptest.NewRecorder()
	ms.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if v, ok := strings.CutPrefix(line, name+" "); ok {
			return v
		}
	}

	return ""
}

// Pinned versions are never evicted; their size is reported against their
// room on the cache volume, so an alert tells when they crowd it. The
// readers' versions, the volume and the count of each kind are reported too,
// so a dashboard shows when to raise the volume.
func TestStoreReportsPinnedSize(t *testing.T) {
	ms := metricsstorage.NewMetricStorage(metricsstorage.WithNewRegistry())
	if err := metrics.RegisterMetrics(ms); err != nil {
		t.Fatal(err)
	}
	s, _ := newTestStore(t, &fakeRender{size: 10}, 1000)
	s.cfg.Metrics = ms
	s.cfg.PinnedBudget = 500

	renderDone(t, s, "echo", "v1.0.0")
	renderDone(t, s, "echo", "v1.1.0")
	renderDone(t, s, "echo", "v1.2.0")
	// A version listed both as a target and as a channel counts as a target.
	s.Pin([]Target{{Module: "echo", Version: "v1.1.0"}}, []Target{{Module: "echo", Version: "v1.2.0"}, {Module: "echo", Version: "v1.1.0"}})

	kinds := func() string {
		var out []string
		for _, kind := range []string{"target", "channel", "reader"} {
			out = append(out, kind+"="+metricValue(t, ms, metrics.DocsBuilderOnDemandCacheKindVersions+`{kind="`+kind+`"}`))
		}
		return strings.Join(out, " ")
	}
	num := func(name string) int64 {
		v, err := strconv.ParseFloat(metricValue(t, ms, name), 64)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return int64(v)
	}

	if got := kinds(); got != "target=1 channel=1 reader=1" {
		t.Errorf("versions by kind: %s", got)
	}
	pinned, readers, total := num(metrics.DocsBuilderOnDemandCachePinnedBytes), num(metrics.DocsBuilderOnDemandCacheReadersBytes), num(metrics.DocsBuilderOnDemandCacheBytes)
	if pinned == 0 || readers == 0 || pinned+readers != total || pinned != 2*readers {
		t.Errorf("pinned %d + readers %d bytes, want two versions and one of the total %d", pinned, readers, total)
	}
	for name, want := range map[string]int64{
		metrics.DocsBuilderOnDemandCachePinnedBudget:  500,
		metrics.DocsBuilderOnDemandCacheReadersBudget: 1000,
		metrics.DocsBuilderOnDemandCacheVolumeBytes:   1500,
	} {
		if got := num(name); got != want {
			t.Errorf("%s = %d, want %d", name, got, want)
		}
	}

	s.Pin(nil, nil)
	if got := kinds(); got != "target=0 channel=0 reader=3" {
		t.Errorf("nothing pinned: versions by kind: %s", got)
	}
	if pinned := num(metrics.DocsBuilderOnDemandCachePinnedBytes); pinned != 0 {
		t.Errorf("nothing pinned: pinned bytes = %d", pinned)
	}
	if readers := num(metrics.DocsBuilderOnDemandCacheReadersBytes); readers != total {
		t.Errorf("nothing pinned: readers bytes = %d, want %d", readers, total)
	}
}

// Without the size of the volume neither it nor the pinned budget is
// reported: a zero would read as a full volume.
func TestStoreReportsNoVolumeWhenUnknown(t *testing.T) {
	ms := metricsstorage.NewMetricStorage(metricsstorage.WithNewRegistry())
	if err := metrics.RegisterMetrics(ms); err != nil {
		t.Fatal(err)
	}
	s, _ := newTestStore(t, &fakeRender{size: 10}, 1000)
	s.cfg.Metrics = ms

	renderDone(t, s, "echo", "v1.0.0")
	for _, name := range []string{metrics.DocsBuilderOnDemandCacheVolumeBytes, metrics.DocsBuilderOnDemandCachePinnedBudget} {
		if v := metricValue(t, ms, name); v != "" {
			t.Errorf("%s = %q, want none", name, v)
		}
	}
	if v := metricValue(t, ms, metrics.DocsBuilderOnDemandCacheReadersBudget); v != "1000" {
		t.Errorf("readers budget = %q, want 1000", v)
	}
}

// An upload that fails while the bucket is ready is counted; one before the
// bucket is ready is not, the S3 ready gauge tells that.
func TestStoreCountsFailedUploads(t *testing.T) {
	ms := metricsstorage.NewMetricStorage(metricsstorage.WithNewRegistry())
	if err := metrics.RegisterMetrics(ms); err != nil {
		t.Fatal(err)
	}
	cold := newMemCold()
	cold.failPut = true
	s := newColdStore(t, &fakeRender{size: 10}, cold)
	s.cfg.Metrics = ms

	renderDone(t, s, "echo", "v1.0.0")
	deadline := time.Now().Add(5 * time.Second)
	for metricValue(t, ms, metrics.DocsBuilderOnDemandS3UploadFailures) != "1" {
		if time.Now().After(deadline) {
			t.Fatalf("failed uploads = %q, want 1", metricValue(t, ms, metrics.DocsBuilderOnDemandS3UploadFailures))
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// A render that fails to take the place of a published version leaves that
// version served: it is the last good one.
func TestStoreKeepsReplacedVersionWhenPublishFails(t *testing.T) {
	f := &fakeRender{size: 10}
	s, _ := newTestStore(t, f, 1000)
	renderDone(t, s, "echo", "v1.0.0")

	// The new render is not there to be renamed into place.
	old, err := s.publish("echo", "v1.0.0", filepath.Join(s.cfg.Dir, tmpDir, "gone"), &published{size: 1})
	if err == nil {
		t.Fatal("a missing render is published")
	}
	if old != "" {
		t.Fatalf("the replaced version is left in the trash to be removed: %s", old)
	}
	if !exists(filepath.Join(s.cfg.Dir, siteDir, "en", "modules", "echo", "v1.0.0", "index.html")) {
		t.Fatal("the replaced version is no longer linked for nginx")
	}
	if st := s.Status("echo", "v1.0.0"); st.Stage != StageDone {
		t.Fatalf("stage %q, want done", st.Stage)
	}
	if entries, _ := os.ReadDir(filepath.Join(s.cfg.Dir, trashDir)); len(entries) != 0 {
		t.Fatalf("trash left: %v", entries)
	}
}
