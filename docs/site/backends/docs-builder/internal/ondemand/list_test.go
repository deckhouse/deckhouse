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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/remote/transport"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/flant/docs-builder/internal/docs"
	"github.com/flant/docs-builder/internal/metrics"
	"github.com/flant/docs-builder/internal/registry"
)

// listRender is a RenderListFunc whose page is the data it got.
type listRender struct {
	calls atomic.Int32
	fail  atomic.Bool
}

func (l *listRender) render(_ context.Context, data []byte, _ string) (map[string][]byte, error) {
	l.calls.Add(1)
	if l.fail.Load() {
		return nil, docs.ErrBuild
	}

	return map[string][]byte{"en": data}, nil
}

// listedCards reads the cards of the page written by listRender.
func listedCards(t *testing.T, s *Store) []card {
	t.Helper()

	var page struct{ Modules []card }
	readJSON(t, filepath.Join(s.cfg.Dir, modulesListPage("en")), &page)

	return page.Modules
}

// summaryRender is fakeRender that also writes the summary of the module.
func summaryRender(f *fakeRender) RenderFunc {
	return func(ctx context.Context, module, version, workDir, outDir string, withAI bool, stage func(Stage)) error {
		if err := f.render(ctx, module, version, workDir, outDir, withAI, stage); err != nil {
			return err
		}
		raw, _ := json.Marshal(docs.Summary{Title: map[string]string{"en": "The " + module}, Stage: "Preview"})

		return os.WriteFile(filepath.Join(outDir, docs.SummaryFile), raw, 0600)
	}
}

func TestWriteList(t *testing.T) {
	f := &fakeRender{size: 10}
	s, _ := newTestStore(t, f, 1<<20)
	s.cfg.Render = summaryRender(f)
	l := &listRender{}
	s.catalog.render = l.render
	ctx := context.Background()

	if err := s.renderPinned(ctx, Target{Module: "echo", Version: "v1.0.0"}, false); err != nil {
		t.Fatal(err)
	}
	modules := []Listed{
		{Module: "echo", Enabled: true, Stage: "General Availability"},
		{Module: "alb", Stage: "Experimental"},
		{Module: "bare", NoDocs: true},
		{Module: "broken", Failed: true},
	}
	targets := []Target{{Module: "echo", Version: "v1.0.0"}, {Module: "alb", Version: "v2.0.0"}, {Module: "broken", Version: "v1.0.0"}}

	// A gzip copy an older docs-builder left on the volume.
	if err := os.MkdirAll(filepath.Dir(filepath.Join(s.cfg.Dir, modulesListPage("en"))), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.cfg.Dir, modulesListPage("en")+".gz"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteList(ctx, modules, targets); err != nil {
		t.Fatal(err)
	}
	want := []card{
		// Not published yet: by its name, with the stage the cluster knows.
		{Module: "alb", Stage: "Experimental", Pending: true},
		{Module: "bare", NoDocs: true},
		// Its target failed for a lasting reason: no promise of details.
		{Module: "broken"},
		// The summary of the published target wins.
		{Module: "echo", Title: map[string]string{"en": "The echo"}, Stage: "Preview", Enabled: true},
	}
	if got := listedCards(t, s); !reflect.DeepEqual(got, want) {
		t.Fatalf("cards = %+v, want %+v", got, want)
	}
	// The page carries SSI includes: a gzip copy would be served without them.
	if _, err := os.Stat(filepath.Join(s.cfg.Dir, modulesListPage("en")+".gz")); err == nil {
		t.Fatal("gzip copy of the list page")
	}

	// The same cards: no build.
	if err := s.WriteList(ctx, modules, targets); err != nil || l.calls.Load() != 1 {
		t.Fatalf("unchanged list: err %v, builds %d, want 1", err, l.calls.Load())
	}

	// A failed build keeps the previous page and is tried again next time.
	l.fail.Store(true)
	if err := s.WriteList(ctx, modules[:1], targets); !errors.Is(err, docs.ErrBuild) {
		t.Fatalf("err = %v, want ErrBuild", err)
	}
	if got := listedCards(t, s); len(got) != len(modules) {
		t.Fatalf("cards after a failed build = %+v, want the previous ones", got)
	}
	l.fail.Store(false)
	if err := s.WriteList(ctx, modules[:1], targets); err != nil || len(listedCards(t, s)) != 1 {
		t.Fatalf("after the failure: err %v, cards %+v", err, listedCards(t, s))
	}

	// A pass that listed no modules: the list is empty, not the old one.
	if err := s.WriteList(ctx, []Listed{}, targets); err != nil || len(listedCards(t, s)) != 0 {
		t.Fatalf("no modules: err %v, cards %+v", err, listedCards(t, s))
	}
}

func TestPinnerListsEveryModule(t *testing.T) {
	f := &fakeRender{size: 10}
	s, _ := newTestStore(t, f, 1<<20)
	l := &listRender{}
	s.catalog.render = l.render
	var firstList []card
	var firstState indexedState
	render := summaryRender(f)
	s.cfg.Render = func(ctx context.Context, module, version, workDir, outDir string, withAI bool, stage func(Stage)) error {
		// The list is there before the first render, on the followers too.
		if firstList == nil {
			firstList = listedCards(t, s)
			readJSON(t, filepath.Join(s.cfg.Dir, indexDir, indexedFile), &firstState)
		}
		if module == "empty" {
			return docs.ErrNoPages
		}
		return render(ctx, module, version, workDir, outDir, withAI, stage)
	}

	r := Resolver{
		Modules: func(context.Context) ([]registry.Module, error) {
			return []registry.Module{
				{Name: "echo", Enabled: true, Version: "v1.0.0", Stage: "General Availability"},
				{Name: "empty"},
				{Name: "none"},
			}, nil
		},
		Policy: ClusterPolicy{
			ChannelVersion: func(_ context.Context, module string) (string, error) {
				if module == "none" {
					return "", ErrNoChannel
				}
				return "v2.0.0", nil
			},
			Latest: func(context.Context, string) (string, error) { return "", nil },
			Logger: log.NewNop(),
		},
		Logger: log.NewNop(),
	}
	NewPinner(r, s, time.Hour, log.NewNop()).sync(context.Background())

	wantFirst := []card{
		{Module: "echo", Stage: "General Availability", Enabled: true, Pending: true},
		{Module: "empty", Pending: true},
		// No version to render at all.
		{Module: "none", NoDocs: true},
	}
	if !reflect.DeepEqual(firstList, wantFirst) {
		t.Fatalf("first list = %+v, want %+v", firstList, wantFirst)
	}
	if len(firstState.Modules) != 3 {
		t.Fatalf("modules in targets.json before the renders = %+v, want all three", firstState.Modules)
	}

	wantLast := []card{
		{Module: "echo", Title: map[string]string{"en": "The echo"}, Stage: "Preview", Enabled: true},
		// Its version has no documentation pages.
		{Module: "empty", NoDocs: true},
		{Module: "none", NoDocs: true},
	}
	if got := listedCards(t, s); !reflect.DeepEqual(got, wantLast) {
		t.Fatalf("list = %+v, want %+v", got, wantLast)
	}

	// The cluster serves the target at the address of the module.
	if got := channelLink(t, s, "echo", "stable"); got != filepath.Join("..", "..", "..", "..", "echo", "v1.0.0", "en") {
		t.Fatalf("stable link of echo = %q", got)
	}
	if target, ok := s.Target("empty"); !ok || target.Version != "v2.0.0" {
		t.Fatalf("target of empty = %v, %v", target, ok)
	}

	// The modules go to targets.json for the followers and the restart.
	var state indexedState
	readJSON(t, filepath.Join(s.cfg.Dir, indexDir, indexedFile), &state)
	wantModules := []Listed{
		{Module: "echo", Enabled: true, Stage: "General Availability"},
		{Module: "empty", NoDocs: true},
		{Module: "none", NoDocs: true},
	}
	if !reflect.DeepEqual(state.Modules, wantModules) {
		t.Fatalf("modules in targets.json = %+v, want %+v", state.Modules, wantModules)
	}
}

func TestFollowerListsModulesOfLeader(t *testing.T) {
	_, cold, s, fl := leaderWithFollower(t)
	l := &listRender{}
	s.catalog.render = l.render

	// The leader recorded no list (its RenderList is not set, as in a
	// docs-builder before the list): the modules come from the targets.
	var state indexedState
	if err := json.Unmarshal(cold.objects[indexedFile], &state); err != nil {
		t.Fatal(err)
	}
	state.Modules = nil
	putIndexed(t, cold, state)
	if answered, complete := fl.Sync(context.Background()); !answered || !complete {
		t.Fatalf("sync: answered %v, complete %v", answered, complete)
	}
	if got, want := listedCards(t, s), []card{{Module: "echo"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("cards = %+v, want %+v", got, want)
	}

	state.Modules = []Listed{{Module: "echo"}, {Module: "later", Stage: "Preview"}}
	putIndexed(t, cold, state)
	fl.Sync(context.Background())
	if got, want := listedCards(t, s), []card{{Module: "echo"}, {Module: "later", Stage: "Preview", Pending: true}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("cards = %+v, want %+v", got, want)
	}
	if target, ok := s.Target("echo"); !ok || target != state.Targets[0] {
		t.Fatalf("target of echo = %v, %v", target, ok)
	}
}

func putIndexed(t *testing.T, cold *memCold, state indexedState) {
	t.Helper()

	raw, _ := json.Marshal(state)
	path := filepath.Join(t.TempDir(), indexedFile)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := cold.Put(context.Background(), indexedFile, path); err != nil {
		t.Fatal(err)
	}
}

func TestLocalOnlyPinnerKeepsTargetsOutOfS3(t *testing.T) {
	cold := newMemCold()
	s := newColdStore(t, &fakeRender{size: 10}, cold)
	NewFallbackPinner(siteResolver(), s, time.Hour, log.NewNop()).sync(context.Background())

	// A follower reading the registry itself keeps its targets on the volume.
	if _, err := os.Stat(filepath.Join(s.cfg.Dir, indexDir, indexedFile)); err != nil {
		t.Fatal(err)
	}
	for _, key := range cold.keys() {
		if key == indexedFile {
			t.Fatal("targets.json of the leader replaced in S3")
		}
	}
}

func TestResolverKeepsModuleWhoseVersionFails(t *testing.T) {
	r := Resolver{
		Modules: func(context.Context) ([]registry.Module, error) { return []registry.Module{{Name: "echo"}}, nil },
		Policy: ClusterPolicy{
			ChannelVersion: func(context.Context, string) (string, error) { return "", errors.New("registry is down") },
			Logger:         log.NewNop(),
		},
		Logger: log.NewNop(),
	}

	res, err := r.ResolveChannels(context.Background(), map[string]Target{"echo": {Module: "echo", Version: "v1.0.0"}})
	if err != nil {
		t.Fatal(err)
	}
	// A registry that does not answer is no reason to mark it without docs.
	if want := []Listed{{Module: "echo"}}; !reflect.DeepEqual(res.Modules, want) {
		t.Fatalf("modules = %+v, want %+v", res.Modules, want)
	}
	if want := []Target{{Module: "echo", Version: "v1.0.0"}}; !reflect.DeepEqual(res.Targets, want) || res.Unresolved != 1 {
		t.Fatalf("targets = %+v, unresolved %d; want the previous target, 1", res.Targets, res.Unresolved)
	}
}

// A disabled module without channel tags and without a release repository
// has nothing published: no documentation, not a failure, in a cluster and
// on the public site.
func TestResolverModuleWithoutReleaseRepository(t *testing.T) {
	notFound := &transport.Error{StatusCode: http.StatusNotFound}
	// The newest version of the module, as versions.Versions.Latest gives it.
	latest := func(context.Context, string) (string, error) {
		return "", fmt.Errorf("%w: source main: %w", ErrNoVersions, notFound)
	}
	for _, site := range []bool{false, true} {
		r := Resolver{
			Modules: func(context.Context) ([]registry.Module, error) { return []registry.Module{{Name: "echo"}}, nil },
			Policy: ClusterPolicy{
				ChannelVersion: func(context.Context, string) (string, error) { return "", ErrNoChannel },
				Latest:         latest,
				Logger:         log.NewNop(),
			},
			Logger: log.NewNop(),
		}
		if site {
			r.Policy = SitePolicy{
				Channels: func(context.Context, string) (Channels, error) { return nil, ErrNoChannel },
				Latest:   latest,
			}
		}

		res, err := r.ResolveChannels(context.Background(), map[string]Target{"echo": {Module: "echo", Version: "v1.0.0"}})
		if err != nil {
			t.Fatal(err)
		}
		if res.Unresolved != 0 || len(res.Targets) != 0 {
			t.Fatalf("site %v: unresolved %d, targets %v", site, res.Unresolved, res.Targets)
		}
		if want := []Listed{{Module: "echo", NoDocs: true}}; !reflect.DeepEqual(res.Modules, want) {
			t.Fatalf("site %v: modules = %+v, want %+v", site, res.Modules, want)
		}
		if !res.Unpublished["echo"] {
			t.Fatalf("site %v: echo is not reported unpublished", site)
		}
	}
}

// A published module that turns unpublished keeps its target one pass: a
// registry uploading it anew answers "not found" for a while. A module that
// had none has no documentation at once.
func TestPinnerDropsUnpublishedModuleOnTheSecondPass(t *testing.T) {
	s, _ := newTestStore(t, &fakeRender{size: 10}, 1000)
	var gone atomic.Bool
	notFound := fmt.Errorf("%w: source main: %w", ErrNoVersions, &transport.Error{StatusCode: http.StatusNotFound})
	r := Resolver{
		Modules: func(context.Context) ([]registry.Module, error) {
			return []registry.Module{{Name: "bare"}, {Name: "echo"}}, nil
		},
		Policy: ClusterPolicy{
			ChannelVersion: func(_ context.Context, module string) (string, error) {
				if module == "bare" || gone.Load() {
					return "", notFound
				}
				return "v1.0.0", nil
			},
			Logger: log.NewNop(),
		},
		Logger: log.NewNop(),
	}
	p := NewPinner(r, s, time.Hour, log.NewNop())
	state := func() indexedState {
		var state indexedState
		readJSON(t, filepath.Join(s.cfg.Dir, indexDir, indexedFile), &state)
		return state
	}
	echo := Target{Module: "echo", Version: "v1.0.0"}

	p.sync(context.Background())
	gone.Store(true)
	p.sync(context.Background())
	got := state()
	if want := []Target{echo}; !reflect.DeepEqual(got.Targets, want) {
		t.Fatalf("first unpublished pass: targets %v, want %v", got.Targets, want)
	}
	if want := []Listed{{Module: "bare", NoDocs: true}, {Module: "echo"}}; !reflect.DeepEqual(got.Modules, want) {
		t.Fatalf("first unpublished pass: modules %+v, want %+v", got.Modules, want)
	}

	p.sync(context.Background())
	got = state()
	if len(got.Targets) != 0 {
		t.Fatalf("second unpublished pass: targets %v, want none", got.Targets)
	}
	if want := []Listed{{Module: "bare", NoDocs: true}, {Module: "echo", NoDocs: true}}; !reflect.DeepEqual(got.Modules, want) {
		t.Fatalf("second unpublished pass: modules %+v, want %+v", got.Modules, want)
	}
}

func TestStopFallbackWaitsForIt(t *testing.T) {
	var done atomic.Bool
	stop := startFallback(context.Background(), func(ctx context.Context) {
		<-ctx.Done()
		time.Sleep(20 * time.Millisecond)
		done.Store(true)
	})
	stop()
	if !done.Load() {
		t.Fatal("stop returned before the fallback did")
	}
}

func TestFallbackPinnerListsFromTheVolume(t *testing.T) {
	cold := newMemCold()
	s := newColdStore(t, &fakeRender{size: 10}, cold)
	l := &listRender{}
	s.catalog.render = l.render
	NewFallbackPinner(siteResolver(), s, time.Hour, log.NewNop()).sync(context.Background())

	// A restart of the pod while S3 does not answer starts from these.
	var state indexedState
	readJSON(t, filepath.Join(s.cfg.Dir, indexDir, indexedFile), &state)
	if want := []Listed{{Module: "echo"}}; !reflect.DeepEqual(state.Modules, want) {
		t.Fatalf("modules on the volume = %+v, want %+v", state.Modules, want)
	}
	if _, ok := s.Menu("echo"); !ok || len(listedCards(t, s)) != 1 {
		t.Fatalf("menu %v, cards %+v", ok, listedCards(t, s))
	}
}

func TestChannelKeepsPublishedVersionOverPasses(t *testing.T) {
	f := &fakeRender{size: 10}
	s, _ := newTestStore(t, f, 1<<20)
	render := s.cfg.Render
	s.cfg.Render = func(ctx context.Context, module, version, workDir, outDir string, withAI bool, stage func(Stage)) error {
		if version == "v2.0.0" {
			return docs.ErrNoPages
		}
		return render(ctx, module, version, workDir, outDir, withAI, stage)
	}
	stable := atomic.Value{}
	stable.Store("v1.0.0")
	r := Resolver{
		Modules: func(context.Context) ([]registry.Module, error) { return []registry.Module{{Name: "echo"}}, nil },
		Policy: SitePolicy{
			Channels: func(context.Context, string) (Channels, error) {
				return Channels{"stable": stable.Load().(string)}, nil
			},
		},
		Logger: log.NewNop(),
	}
	p := NewPinner(r, s, time.Hour, log.NewNop())
	p.sync(context.Background())

	// The channel moves to a version without documentation, for two passes.
	stable.Store("v2.0.0")
	for pass := 1; pass <= 2; pass++ {
		p.sync(context.Background())
		var state indexedState
		readJSON(t, filepath.Join(s.cfg.Dir, indexDir, indexedFile), &state)
		if got := state.Channels["echo"]["stable"]; got != "v1.0.0" {
			t.Fatalf("pass %d: stable in targets.json = %q, want the published v1.0.0", pass, got)
		}
	}
}

// A development branch (ModulePullOverride) is never in S3: a follower
// renders it itself, and the address of the module and the version menu
// both lead to it.
func TestFollowerServesTheBranchOfAPullOverride(t *testing.T) {
	cold := newMemCold()
	f := &fakeRender{size: 10}
	s := newColdStore(t, f, cold)
	fl := NewFollower(s, time.Hour, log.NewNop())
	ctx := context.Background()

	// The module ran v1.0.0 before: its version is here, linked as stable.
	if err := s.renderPinned(ctx, Target{Module: "console", Version: "v1.0.0"}, false); err != nil {
		t.Fatal(err)
	}
	s.SetChannels(map[string]Channels{"console": {"stable": "v1.0.0"}})

	branch := Target{Module: "console", Version: "main", Rev: "sha256:aaa"}
	state := indexedState{
		Targets:  []Target{branch},
		Channels: map[string]Channels{"console": {"stable": "main"}},
		Modules:  []Listed{{Module: "console", Enabled: true}},
	}
	putIndexed(t, cold, state)

	// The branch does not render yet: the module keeps the version it had,
	// and the menu says so rather than the branch it does not serve.
	f.fail.Store(true)
	fl.Sync(ctx)
	if got := channelLink(t, s, "console", "stable"); !strings.Contains(got, "v1.0.0") {
		t.Fatalf("stable link = %q, want v1.0.0 while the branch fails", got)
	}
	if menu, _ := s.Menu("console"); menu.Channels["stable"] != "v1.0.0" {
		t.Fatalf("menu = %+v, want the served v1.0.0", menu)
	}

	// The failure passes: the next sync tries the branch again by itself.
	f.fail.Store(false)
	fl.Sync(ctx)
	if got := channelLink(t, s, "console", "stable"); !strings.Contains(got, "main") {
		t.Fatalf("stable link = %q, want the branch", got)
	}
	if menu, _ := s.Menu("console"); menu.Channels["stable"] != "main" {
		t.Fatalf("menu = %+v, want the branch", menu)
	}
	// An upload runs in the background.
	waitBackground(s)
	for _, k := range cold.keys() {
		if strings.HasPrefix(k, "console/main/") {
			t.Fatalf("the branch went to S3: %s", k)
		}
	}
}

// A branch that fails for a lasting reason (Hugo, no pages) is not tried on
// every sync: the follower tries it again after retryAfter, as the leader
// does.
func TestFollowerRetriesALastingBranchFailureLater(t *testing.T) {
	cold := newMemCold()
	f := &fakeRender{size: 10, failErr: fmt.Errorf("hugo: %w", docs.ErrBuild)}
	s := newColdStore(t, f, cold)
	fl := NewFollower(s, time.Hour, log.NewNop())
	c := &clock{t: time.Unix(1_800_000_000, 0)}
	fl.now = c.now
	ctx := context.Background()

	putIndexed(t, cold, indexedState{
		Targets:  []Target{{Module: "console", Version: "main", Rev: "sha256:aaa"}},
		Channels: map[string]Channels{"console": {"stable": "main"}},
		Modules:  []Listed{{Module: "console", Enabled: true}},
	})

	f.fail.Store(true)
	fl.Sync(ctx)
	f.fail.Store(false)
	calls := f.calls.Load()
	fl.Sync(ctx)
	if got := f.calls.Load(); got != calls {
		t.Fatalf("renders = %d, want %d: a lasting failure waits for retryAfter", got, calls)
	}

	c.add(time.Hour)
	fl.Sync(ctx)
	if got := channelLink(t, s, "console", "stable"); !strings.Contains(got, "main") {
		t.Fatalf("stable link = %q, want the branch after retryAfter", got)
	}
}

// A first pass on an empty cache renders every module for minutes: the list
// and targets.json get a module as soon as its version is published, before
// the pass ends.
func TestPinnerShowsProgressOfThePass(t *testing.T) {
	f := &fakeRender{size: 10}
	s, _ := newTestStore(t, f, 1<<20)
	l := &listRender{}
	s.catalog.render = l.render
	render := summaryRender(f)
	var seen []card
	var seenState indexedState
	s.cfg.Render = func(ctx context.Context, module, version, workDir, outDir string, withAI bool, stage func(Stage)) error {
		if module == "zeta" && seen == nil {
			seen = listedCards(t, s)
			readJSON(t, filepath.Join(s.cfg.Dir, indexDir, indexedFile), &seenState)
		}
		return render(ctx, module, version, workDir, outDir, withAI, stage)
	}
	r := Resolver{
		Modules: func(context.Context) ([]registry.Module, error) {
			return []registry.Module{{Name: "alpha", Version: "v1.0.0"}, {Name: "zeta", Version: "v1.0.0"}}, nil
		},
		Policy: ClusterPolicy{
			ChannelVersion: func(context.Context, string) (string, error) { return "v1.0.0", nil },
			Latest:         func(context.Context, string) (string, error) { return "", nil },
			Logger:         log.NewNop(),
		},
		Logger: log.NewNop(),
	}
	p := NewPinner(r, s, time.Hour, log.NewNop())
	p.progressEvery = 0
	p.sync(context.Background())

	want := []card{
		{Module: "alpha", Title: map[string]string{"en": "The alpha"}, Stage: "Preview"},
		{Module: "zeta", Pending: true},
	}
	if !reflect.DeepEqual(seen, want) {
		t.Fatalf("list while zeta renders = %+v, want %+v", seen, want)
	}
	// The followers get the published version from targets.json.
	if want := []Target{{Module: "alpha", Version: "v1.0.0"}}; !reflect.DeepEqual(seenState.Targets, want) {
		t.Fatalf("targets.json while zeta renders = %+v, want %+v", seenState.Targets, want)
	}
}

// A restarted leader lists the modules of the saved targets.json before
// the registry answers.
func TestPinnerListsSavedModulesBeforeTheRegistry(t *testing.T) {
	s, _ := newTestStore(t, &fakeRender{size: 10}, 1000)
	s.catalog.render = (&listRender{}).render
	if _, err := s.saveIndexedLocal(indexedState{
		Targets: []Target{{Module: "echo", Version: "v1.0.0"}},
		Modules: []Listed{{Module: "echo", Enabled: true}, {Module: "later"}},
	}); err != nil {
		t.Fatal(err)
	}
	asked, answer := make(chan struct{}), make(chan struct{})
	r := Resolver{
		Modules: func(ctx context.Context) ([]registry.Module, error) {
			close(asked)
			select {
			case <-answer:
			case <-ctx.Done():
			}
			return nil, errors.New("registry is down")
		},
		Policy: ClusterPolicy{Logger: log.NewNop()},
		Logger: log.NewNop(),
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		NewPinner(r, s, time.Hour, log.NewNop()).sync(context.Background())
	}()
	defer func() { close(answer); <-done }()

	<-asked
	want := []card{{Module: "echo", Enabled: true, Pending: true}, {Module: "later", Pending: true}}
	if got := listedCards(t, s); !reflect.DeepEqual(got, want) {
		t.Fatalf("cards = %+v, want %+v", got, want)
	}
}

func TestWriteListReportsFailure(t *testing.T) {
	ms := metricsstorage.NewMetricStorage(metricsstorage.WithNewRegistry())
	if err := metrics.RegisterMetrics(ms); err != nil {
		t.Fatal(err)
	}
	s, _ := newTestStore(t, &fakeRender{size: 10}, 1000)
	s.cfg.Metrics = ms
	l := &listRender{}
	s.catalog.render = l.render
	gauge := func() string {
		rec := httptest.NewRecorder()
		ms.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
		for _, line := range strings.Split(rec.Body.String(), "\n") {
			if strings.HasPrefix(line, metrics.DocsBuilderOnDemandModulesListFailed+" ") {
				return strings.TrimPrefix(line, metrics.DocsBuilderOnDemandModulesListFailed+" ")
			}
		}
		return ""
	}
	write := func() { _ = s.WriteList(context.Background(), []Listed{{Module: "echo"}}, nil) }

	l.fail.Store(true)
	write()
	if got := gauge(); got != "1" {
		t.Fatalf("after a failed build: %q, want 1", got)
	}
	l.fail.Store(false)
	write()
	if got := gauge(); got != "0" {
		t.Fatalf("after a build: %q, want 0", got)
	}
	// The same cards: the page is not built again and stays served.
	calls := l.calls.Load()
	write()
	if got := gauge(); got != "0" || l.calls.Load() != calls {
		t.Fatalf("the same cards: gauge %q, builds %d -> %d", got, calls, l.calls.Load())
	}
}
