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
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/flant/docs-builder/internal/metrics"
	"github.com/flant/docs-builder/internal/ondemand"
	"github.com/flant/docs-builder/internal/registry"
)

func siteResolver() Resolver {
	return Resolver{
		Modules: func(context.Context) ([]registry.Module, error) { return []registry.Module{{Name: "echo"}}, nil },
		Policy: SitePolicy{
			Channels: func(context.Context, string) (ondemand.Channels, error) {
				return ondemand.Channels{"alpha": "v2.0.0", "stable": "v1.0.0"}, nil
			},
		},
		Logger: log.NewNop(),
	}
}

// leaderWithFollower runs a pass of the leader on the bucket and returns a
// follower on an empty volume of its own.
// leaderWithFollower is a leader that pinned the versions of siteResolver
// on a bucket, and a follower of it; configure sets up the store of the
// follower.
func leaderWithFollower(t *testing.T, configure ...func(*Config)) (*fakeRender, *memCold, *Store, *Follower) {
	t.Helper()

	cold := newMemCold()
	f := &fakeRender{size: 10}
	leader := newColdStore(t, f, cold)
	NewPinner(siteResolver(), leader, time.Hour, log.NewNop()).sync(context.Background())
	cold.waitFor(t, "echo/v2.0.0/rev1.tar.gz")
	cold.waitFor(t, "echo/v1.0.0/rev1.ai.tar.gz")

	s := newColdStore(t, f, cold, configure...)

	return f, cold, s, NewFollower(s, time.Hour, log.NewNop())
}

func TestFollowerTakesVersionsOfLeader(t *testing.T) {
	f, cold, s, fl := leaderWithFollower(t)
	renders := f.calls.Load()

	if answered, complete := fl.Sync(context.Background()); !answered || !complete {
		t.Fatalf("answered %v, complete %v: want both", answered, complete)
	}
	if n := f.calls.Load(); n != renders {
		t.Fatalf("renders %d -> %d: a follower renders nothing the leader recorded", renders, n)
	}
	for _, v := range []string{"v1.0.0", "v2.0.0"} {
		if !s.published(ondemand.Target{Module: "echo", Version: v}) {
			t.Errorf("%s is not published from S3", v)
		}
	}
	if got := channelLink(t, s, "echo", "alpha"); got != filepath.Join("..", "..", "..", "..", "echo", "v2.0.0", "en") {
		t.Errorf("alpha -> %q", got)
	}
	if !exists(filepath.Join(s.cfg.Dir, indexDir, "en", "search.json")) {
		t.Error("no search index is built from the versions of the leader")
	}

	// An unchanged list costs one request.
	cold.mu.Lock()
	gets := cold.gets
	cold.mu.Unlock()
	fl.Sync(context.Background())
	cold.mu.Lock()
	defer cold.mu.Unlock()
	if cold.gets != gets+1 {
		t.Errorf("S3 requests %d -> %d for an unchanged list, want one", gets, cold.gets)
	}
}

// A follower that becomes the leader starts from what it followed: it
// renders and downloads nothing again.
func TestFollowerBecomesLeader(t *testing.T) {
	f, cold, s, fl := leaderWithFollower(t)
	fl.Sync(context.Background())
	renders := f.calls.Load()
	cold.mu.Lock()
	gets := cold.gets
	cold.mu.Unlock()

	restored := false
	p := NewPinner(siteResolver(), s, time.Hour, log.NewNop())
	p.OnRestored = func() { restored = true }
	p.sync(context.Background())

	if !restored {
		t.Error("OnRestored is not called")
	}
	if n := f.calls.Load(); n != renders {
		t.Errorf("renders %d -> %d after taking the lead", renders, n)
	}
	cold.mu.Lock()
	defer cold.mu.Unlock()
	if cold.gets != gets {
		t.Errorf("S3 downloads %d -> %d after taking the lead", gets, cold.gets)
	}
}

func TestFollowerFallsBackToRegistry(t *testing.T) {
	_, cold, s, fl := leaderWithFollower(t)
	if answered, complete := fl.Sync(context.Background()); !answered || !complete {
		t.Fatal("first sync")
	}
	c := advancingClock(t, cold)
	fl.now = c.now
	cold.mu.Lock()
	cold.failGet = true
	cold.mu.Unlock()

	var firsts, started, stopped atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go fl.Run(ctx, time.Millisecond, func() bool { return false }, func() { firsts.Add(1) }, func(fctx context.Context) {
		started.Add(1)
		<-fctx.Done()
		// The fallback served targets of its own up to its end.
		s.SetTargets([]ondemand.Target{{Module: "echo", Version: "v9.9.9"}})
		stopped.Add(1)
	})

	waitFor := func(what string, ok func() bool) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for !ok() {
			if time.Now().After(deadline) {
				t.Fatal(what)
			}
			time.Sleep(time.Millisecond)
		}
	}
	waitFor("no fallback with S3 down", func() bool { return started.Load() == 1 })
	if firsts.Load() != 0 {
		t.Fatal("ready before S3 answered")
	}

	cold.mu.Lock()
	cold.failGet = false
	cold.mu.Unlock()
	waitFor("the fallback goes on with S3 back", func() bool { return stopped.Load() == 1 })
	waitFor("not ready with S3 back", func() bool { return firsts.Load() == 1 })
	if started.Load() != 1 {
		t.Fatalf("fallback started %d times, want once", started.Load())
	}
	// The same targets.json of the leader is applied again over them.
	waitFor("the targets of the fallback stay", func() bool {
		target, _ := s.Target("echo")
		return target.Version == "v1.0.0"
	})
}

// After a new template the follower serves the old archives, and replaces
// them once the leader rendered them anew.
func TestFollowerReplacesStaleVersions(t *testing.T) {
	cold := newMemCold()
	f := &fakeRender{size: 10}
	NewPinner(siteResolver(), newColdStore(t, f, cold), time.Hour, log.NewNop()).sync(context.Background())
	cold.waitFor(t, "echo/v1.0.0/rev1.ai.tar.gz")
	cold.waitFor(t, "echo/v2.0.0/rev1.tar.gz")

	follower := coldStoreAt(t, f, cold, "rev2")
	fl := NewFollower(follower, time.Hour, log.NewNop())
	// Served from the older archives, the follower is ready: it does not
	// wait for the leader to render everything anew.
	if _, complete := fl.Sync(context.Background()); !complete {
		t.Fatal("not complete while every version is served")
	}
	if v := follower.versions[key("echo", "v1.0.0")]; v == nil || !v.stale {
		t.Fatal("the follower does not serve the archive of the older template")
	}

	// The leader renders both anew.
	if _, err := coldStoreAt(t, f, cold, "rev2").rerender.pass(context.Background(), "rev2"); err != nil {
		t.Fatal(err)
	}
	cold.waitFor(t, "echo/v1.0.0/rev2.ai.tar.gz")

	fl.Sync(context.Background())
	for _, v := range []string{"v1.0.0", "v2.0.0"} {
		if p := follower.versions[key("echo", v)]; p == nil || p.stale {
			t.Errorf("%s is still served from the archive of the older template", v)
		}
	}

	// A restart keeps knowing what is stale: a store on the same volume.
	cfg := follower.cfg
	cfg.Cold = newMemCold()
	stale := NewStore(cfg)
	if err := stale.Load(); err != nil {
		t.Fatal(err)
	}
	if p := stale.versions[key("echo", "v1.0.0")]; p == nil || p.stale {
		t.Error("a fresh version is taken for stale after a restart")
	}
}

func TestFollowerStopsFallbackWhenLeading(t *testing.T) {
	_, cold, _, fl := leaderWithFollower(t)
	c := advancingClock(t, cold)
	fl.now = c.now
	cold.mu.Lock()
	cold.failGet = true
	cold.mu.Unlock()

	var leading, started, stopped atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go fl.Run(ctx, time.Millisecond, func() bool { return leading.Load() == 1 }, func() {}, func(fctx context.Context) {
		started.Add(1)
		<-fctx.Done()
		stopped.Add(1)
	})

	deadline := time.Now().Add(5 * time.Second)
	for started.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("no fallback with S3 down")
		}
		time.Sleep(time.Millisecond)
	}
	leading.Store(1)
	for stopped.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the fallback runs beside the leader")
		}
		time.Sleep(time.Millisecond)
	}
}

// advancingClock is a clock the test moves by FollowFallbackAfter after
// each request the follower makes to S3, so the follower sees the time pass
// sync by sync, however often it reads the clock.
func advancingClock(t *testing.T, cold *memCold) *clock {
	t.Helper()

	c := &clock{t: time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() {
		seen := 0
		for ctx.Err() == nil {
			cold.mu.Lock()
			gets := cold.gets
			cold.mu.Unlock()
			if gets > seen {
				seen = gets
				c.add(FollowFallbackAfter)
			}
			time.Sleep(time.Millisecond)
		}
	}()

	return c
}

// targets.json may name a version whose archive the leader has not
// uploaded yet: the follower publishes the rest and takes it on a later
// sync, with the list unchanged. Until then a metric counts it.
func TestFollowerWaitsForArchiveOfLeader(t *testing.T) {
	ms := metricsstorage.NewMetricStorage(metricsstorage.WithNewRegistry())
	if err := metrics.RegisterMetrics(ms); err != nil {
		t.Fatal(err)
	}
	cold := newMemCold()
	f := &fakeRender{size: 10}
	state := `{"targets":[{"module":"echo","version":"v1.0.0"},{"module":"alb","version":"v2.0.0"}]}`
	dir := t.TempDir()
	path := filepath.Join(dir, "targets.json")
	if err := os.WriteFile(path, []byte(state), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cold.Put(context.Background(), indexedFile, path); err != nil {
		t.Fatal(err)
	}
	leader := newColdStore(t, f, cold)
	if err := leader.RenderPinned(context.Background(), ondemand.Target{Module: "echo", Version: "v1.0.0"}); err != nil {
		t.Fatal(err)
	}
	cold.waitFor(t, "echo/v1.0.0/rev1.ai.tar.gz")

	s := newColdStore(t, f, cold)
	s.cfg.Metrics = ms
	fl := NewFollower(s, time.Hour, log.NewNop())
	if answered, complete := fl.Sync(context.Background()); !answered || complete {
		t.Fatalf("answered %v, complete %v: want an incomplete sync", answered, complete)
	}
	if !s.published(ondemand.Target{Module: "echo", Version: "v1.0.0"}) {
		t.Fatal("the version with an archive is not published")
	}
	if v := metricValue(t, ms, metrics.DocsBuilderOnDemandFollowMissing); v != "1" {
		t.Fatalf("missing versions = %q, want 1", v)
	}

	if err := leader.RenderPinned(context.Background(), ondemand.Target{Module: "alb", Version: "v2.0.0"}); err != nil {
		t.Fatal(err)
	}
	cold.waitFor(t, "alb/v2.0.0/rev1.ai.tar.gz")
	if _, complete := fl.Sync(context.Background()); !complete || !s.published(ondemand.Target{Module: "alb", Version: "v2.0.0"}) {
		t.Fatal("the archive uploaded later is not taken")
	}
	if v := metricValue(t, ms, metrics.DocsBuilderOnDemandFollowMissing); v != "0" {
		t.Fatalf("missing versions = %q, want 0", v)
	}
}

// A version served from an archive of an older template stays known as
// such across a restart, so it is still replaced.
func TestStaleVersionSurvivesRestart(t *testing.T) {
	cold := newMemCold()
	f := &fakeRender{size: 10}
	renderDone(t, coldStoreAt(t, f, cold, "rev1"), "echo", "v1.0.0")
	cold.waitFor(t, "echo/v1.0.0/rev1.tar.gz")

	s := coldStoreAt(t, f, cold, "rev2")
	renderDone(t, s, "echo", "v1.0.0")
	if v := s.versions[key("echo", "v1.0.0")]; v == nil || !v.stale {
		t.Fatal("not stale before the restart")
	}

	restarted := NewStore(s.cfg)
	if err := restarted.Load(); err != nil {
		t.Fatal(err)
	}
	if v := restarted.versions[key("echo", "v1.0.0")]; v == nil || !v.stale {
		t.Fatal("the stale version is taken for current after a restart")
	}
}

// A version a reader opened from an archive of an older template is
// replaced once the leader rendered it anew, after a restart as well: an
// access to it looks for that, at most every staleCheckEvery.
func TestReaderGetsStaleVersionRenderedAnew(t *testing.T) {
	cold := newMemCold()
	f := &fakeRender{size: 10}
	old := coldStoreAt(t, f, cold, "rev1")
	renderDone(t, old, "echo", "v1.0.0")
	cold.waitPuts(t, 1)
	waitBackground(old)
	follower := coldStoreAt(t, f, cold, "rev2")
	renderDone(t, follower, "echo", "v1.0.0")
	// S3 does not track reads: the archive goes up again.
	cold.waitPuts(t, 2)
	waitBackground(follower)

	s := NewStore(follower.cfg)
	c := &clock{t: time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)}
	s.now = c.now
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}
	stale := func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		v := s.versions[key("echo", "v1.0.0")]
		return v != nil && v.stale
	}
	open := func() {
		s.Touch("echo", "v1.0.0")
		waitBackground(s)
	}

	// Not rendered anew yet: it stays.
	open()
	if !stale() {
		t.Fatal("the stale version is gone before the leader rendered it anew")
	}

	if _, err := coldStoreAt(t, f, cold, "rev2").rerender.pass(context.Background(), "rev2"); err != nil {
		t.Fatal(err)
	}
	open()
	if !stale() {
		t.Fatal("looked for again within staleCheckEvery")
	}
	c.add(staleCheckEvery)
	open()
	if stale() {
		t.Fatal("the version rendered anew does not replace the one of the older template")
	}
	if n := f.calls.Load(); n != 2 {
		t.Fatalf("renders = %d, want 2: the reader takes the version the leader rendered", n)
	}
}

// The followers take the versions of the leader from S3 only: what the
// leader serves and S3 lacks goes up on its next pass, without a render.
func TestPinnerUploadsWhatTheColdCacheLacks(t *testing.T) {
	cold := newMemCold()
	f := &fakeRender{size: 10}
	leader := newColdStore(t, f, cold)
	// While the cluster does not answer whether a pull override pulls a
	// version, the version is kept out of S3.
	var down atomic.Bool
	down.Store(true)
	leader.cold.branch = func(context.Context, string, string) (bool, error) {
		if down.Load() {
			return false, errors.New("apiserver is down")
		}
		return false, nil
	}
	p := NewPinner(siteResolver(), leader, time.Hour, log.NewNop())
	ctx := context.Background()
	pass := func() {
		p.sync(ctx)
		waitBackground(leader)
	}
	archives := []string{"echo/v1.0.0/rev1.ai.tar.gz", "echo/v2.0.0/rev1.tar.gz"}
	inCold := func() bool {
		keys := cold.keys()
		return slices.Contains(keys, archives[0]) && slices.Contains(keys, archives[1])
	}
	s := newColdStore(t, f, cold)
	fl := NewFollower(s, time.Hour, log.NewNop())
	follow := func() bool {
		_, complete := fl.Sync(ctx)
		waitBackground(s)
		return complete
	}

	pass()
	if inCold() {
		t.Fatal("uploaded while the cluster does not answer")
	}
	if follow() {
		t.Fatal("the follower is complete without the archives")
	}

	down.Store(false)
	renders := f.calls.Load()
	pass()
	if !inCold() {
		t.Fatalf("archives %v, want the versions the leader serves", cold.keys())
	}
	// Lost again: an upload that failed, an archive evicted.
	cold.mu.Lock()
	for _, k := range archives {
		delete(cold.objects, k)
	}
	cold.mu.Unlock()
	pass()
	if !inCold() {
		t.Fatalf("archives %v, want the lost ones back", cold.keys())
	}
	if n := f.calls.Load(); n != renders {
		t.Fatalf("renders %d -> %d: the archives come from the pages the leader serves", renders, n)
	}
	if !follow() {
		t.Fatal("the follower misses versions of the leader")
	}

	// A development branch stays out of S3, however it is served.
	branch := ondemand.Target{Module: "console", Version: "main", Rev: "sha256:aaa"}
	if err := leader.RenderPinned(ctx, branch); err != nil {
		t.Fatal(err)
	}
	if err := leader.uploadMissing(ctx, []ondemand.Target{branch}); err != nil {
		t.Fatal(err)
	}
	waitBackground(leader)
	for _, k := range cold.keys() {
		if strings.HasPrefix(k, "console/") {
			t.Fatalf("the branch went to S3: %s", k)
		}
	}
}

// A pinned version the leader serves from an archive of an older template
// is left to Rerender, which renders that archive anew. Once the cold cache
// has lost the archive, Rerender never gets to the version: the next pass
// renders it anew and uploads it for the followers.
func TestPinnerRendersStaleVersionsLostFromColdCache(t *testing.T) {
	cold := newMemCold()
	f := &fakeRender{size: 10}
	ctx := context.Background()
	old := coldStoreAt(t, f, cold, "rev1")
	NewPinner(siteResolver(), old, time.Hour, log.NewNop()).sync(ctx)
	waitBackground(old)

	leader := coldStoreAt(t, f, cold, "rev2")
	p := NewPinner(siteResolver(), leader, time.Hour, log.NewNop())
	pass := func() {
		p.sync(ctx)
		waitBackground(leader)
	}
	stale := func(version string) bool {
		leader.mu.Lock()
		defer leader.mu.Unlock()
		v := leader.versions[key("echo", version)]
		return v != nil && v.stale
	}

	pass()
	if !stale("v1.0.0") || !stale("v2.0.0") || f.calls.Load() != 2 {
		t.Fatalf("renders = %d: want both versions from the archives of the older template, left to Rerender", f.calls.Load())
	}

	// Evicted before Rerender got to them.
	cold.mu.Lock()
	delete(cold.objects, "echo/v1.0.0/rev1.ai.tar.gz")
	delete(cold.objects, "echo/v2.0.0/rev1.tar.gz")
	cold.mu.Unlock()
	pass()
	if stale("v1.0.0") || stale("v2.0.0") || f.calls.Load() != 4 {
		t.Fatalf("renders = %d: want both versions rendered anew", f.calls.Load())
	}
	if keys := cold.keys(); !slices.Equal(keys, []string{"echo/v1.0.0/rev2.ai.tar.gz", "echo/v2.0.0/rev2.tar.gz", indexedFile}) {
		t.Fatalf("archives %v, want both versions of the new template", keys)
	}

	follower := coldStoreAt(t, f, cold, "rev2")
	fl := NewFollower(follower, time.Hour, log.NewNop())
	if _, complete := fl.Sync(ctx); !complete {
		t.Fatal("the follower misses versions of the leader")
	}
}

// A target whose AI export failed on the leader is served without it there,
// and its archive has none: the follower serves it so as well instead of
// waiting for an archive that does not come, and takes the export once the
// leader has it.
func TestFollowerServesTargetWithoutAIExportAsLeader(t *testing.T) {
	cold := newMemCold()
	f := &fakeRender{size: 10}
	f.noAI.Store(true)
	leader := newColdStore(t, f, cold)
	ctx := context.Background()
	echo := ondemand.Target{Module: "echo", Version: "v1.0.0"}
	if err := leader.RenderPinned(ctx, echo); !errors.Is(err, ondemand.ErrNoAIExport) {
		t.Fatalf("err = %v, want ErrNoAIExport", err)
	}
	waitBackground(leader)
	putIndexed(t, cold, indexedState{Targets: []ondemand.Target{echo}, Modules: []ondemand.Listed{{Module: "echo"}}})

	s := newColdStore(t, f, cold)
	fl := NewFollower(s, time.Hour, log.NewNop())
	_, complete := fl.Sync(ctx)
	waitBackground(s)
	if !complete || !s.published(echo) {
		t.Fatal("the follower waits for the AI export the leader has not")
	}

	f.noAI.Store(false)
	if err := leader.RenderPinned(ctx, echo); err != nil {
		t.Fatal(err)
	}
	waitBackground(leader)
	if keys := cold.keys(); !slices.Equal(keys, []string{"echo/v1.0.0/rev1.ai.tar.gz", indexedFile}) {
		t.Fatalf("archives %v, want the one with the export", keys)
	}
	fl.Sync(ctx)
	waitBackground(s)
	if !hasLLMs(t, s, "echo") {
		t.Fatal("the follower does not take the AI export the leader has now")
	}
}

// A reader's version without the AI export that becomes a target is
// replaced by the archive of the leader, with the export: the search and
// llms.txt of the follower get the module.
func TestFollowerTakesAIExportOfReadersVersionBecomingTarget(t *testing.T) {
	cold := newMemCold()
	f := &fakeRender{size: 10}
	leader := newColdStore(t, f, cold)
	renderDone(t, leader, "echo", "v1.0.0")
	cold.waitPuts(t, 1)
	waitBackground(leader)
	s := newColdStore(t, f, cold)
	renderDone(t, s, "echo", "v1.0.0")
	// S3 does not track reads: the archive goes up again.
	cold.waitPuts(t, 2)
	waitBackground(s)

	ctx := context.Background()
	echo := ondemand.Target{Module: "echo", Version: "v1.0.0"}
	if err := leader.RenderPinned(ctx, echo); err != nil {
		t.Fatal(err)
	}
	waitBackground(leader)
	putIndexed(t, cold, indexedState{Targets: []ondemand.Target{echo}, Modules: []ondemand.Listed{{Module: "echo"}}})

	fl := NewFollower(s, time.Hour, log.NewNop())
	_, complete := fl.Sync(ctx)
	waitBackground(s)
	if !complete || !hasLLMs(t, s, "echo") {
		t.Fatal("the target stays served without the AI export the leader has")
	}
	if n := f.calls.Load(); n != 2 {
		t.Fatalf("renders = %d, want the two of the leader", n)
	}
}

// hasLLMs reports whether llms.txt of the store has the module.
func hasLLMs(t *testing.T, s *Store, module string) bool {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(s.cfg.Dir, indexDir, "en", "llms.txt"))
	if err != nil {
		t.Fatal(err)
	}

	return strings.Contains(string(raw), "## "+module+"\n")
}

// While the pod leads, the follower reads nothing from S3.
func TestFollowerIdleWhileLeading(t *testing.T) {
	_, cold, _, fl := leaderWithFollower(t)
	cold.mu.Lock()
	gets := cold.gets
	cold.mu.Unlock()

	// The follower asks who leads once a turn: by the third time it has had
	// two whole turns to read S3.
	var turns atomic.Int32
	turned := make(chan struct{})
	leading := func() bool {
		if turns.Add(1) == 3 {
			close(turned)
		}
		return true
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		fl.Run(ctx, time.Millisecond, leading, func() {}, func(context.Context) {})
		close(done)
	}()
	<-turned
	cancel()
	<-done

	cold.mu.Lock()
	defer cold.mu.Unlock()
	if cold.gets != gets {
		t.Fatalf("S3 requests %d -> %d while leading", gets, cold.gets)
	}
}

// With an empty bucket the leader writes targets.json a few seconds after
// it starts: the follower looks for it often until it is there, not once in
// a follow interval.
func TestFollowerLooksForTheFirstTargetsOften(t *testing.T) {
	cold := newMemCold()
	s := newColdStore(t, &fakeRender{size: 10}, cold)
	fl := NewFollower(s, time.Hour, log.NewNop())
	fl.firstEvery = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		fl.Run(ctx, time.Hour, func() bool { return false }, func() {}, func(context.Context) {})
	}()
	// The follower writes to the volume of the test until it stops.
	defer func() {
		cancel()
		<-done
	}()

	// The first look finds nothing: targets.json is written after it.
	cold.waitGets(t, 1)
	putIndexed(t, cold, indexedState{Modules: []ondemand.Listed{{Module: "echo"}}, Targets: []ondemand.Target{{Module: "echo", Version: "v1.0.0"}}})
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, ok := s.Target("echo"); ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the follower did not take targets.json written after its start")
		}
		time.Sleep(time.Millisecond)
	}
}
