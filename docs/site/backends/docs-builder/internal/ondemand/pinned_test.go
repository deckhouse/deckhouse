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
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
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

func TestResolverPicksTheVersionOfEveryModule(t *testing.T) {
	modules := []registry.Module{
		{Name: "console", Enabled: true, Version: "main", Override: &registry.Override{Tag: "main", Digest: "sha256:aaa"}},
		{Name: "alb", Enabled: true, Version: "pr-7", Override: &registry.Override{Tag: "pr-7"}}, // the image is not pulled yet
		{Name: "prometheus", Enabled: true, Version: "v1.0.29"},
		{Name: "time-machine", Enabled: true, Version: "new-flow"}, // a branch without an override: as if disabled
		{Name: "virtualization", Version: "v1.12.0"},
		{Name: "echo"},
		{Name: "empty"},
		{Name: "flaky"},
		{Name: "gone"},
	}
	channel := map[string]string{"time-machine": "v0.9.0", "virtualization": "v1.11.2"}
	latest := map[string]string{"echo": "v0.17.0"}

	r := Resolver{
		Modules: func(context.Context) ([]registry.Module, error) { return modules, nil },
		ChannelVersion: func(_ context.Context, module string) (string, error) {
			switch {
			case module == "flaky" || module == "gone":
				return "", errors.New("registry is down")
			case channel[module] != "":
				return channel[module], nil
			}
			return "", errNoChannel
		},
		Latest: func(_ context.Context, module string) (string, error) { return latest[module], nil },
		Logger: log.NewNop(),
	}

	res, err := r.ResolveChannels(context.Background(), map[string]Target{"flaky": {Module: "flaky", Version: "v2.0.0"}})
	if err != nil {
		t.Fatal(err)
	}
	got := res.Targets

	want := []Target{
		{Module: "alb", Version: "pr-7", Rev: "pr-7"},
		{Module: "console", Version: "main", Rev: "sha256:aaa"},
		{Module: "echo", Version: "v0.17.0"},
		{Module: "flaky", Version: "v2.0.0"},
		{Module: "prometheus", Version: "v1.0.29"},
		{Module: "time-machine", Version: "v0.9.0"},
		{Module: "virtualization", Version: "v1.11.2"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("targets =\n%v\nwant\n%v", got, want)
	}
}

func TestStoreKeepsPinnedVersions(t *testing.T) {
	f := &fakeRender{size: 100}
	s, c := newTestStore(t, f, 10000)

	pinned := Target{Module: "echo", Version: "v1.0.0"}
	channel := Target{Module: "echo", Version: "v0.9.0"}
	s.Pin([]Target{pinned}, []Target{channel})
	if err := s.RenderPinned(context.Background(), pinned); err != nil {
		t.Fatal(err)
	}
	if err := s.renderPinned(context.Background(), channel, false); err != nil {
		t.Fatal(err)
	}

	for _, v := range []string{"v2.0.0", "v3.0.0", "v4.0.0"} {
		c.add(time.Second)
		renderDone(t, s, "echo", v)
	}
	c.add(2 * time.Hour) // past MaxIdle
	s.evict()

	if !exists(filepath.Join(s.cfg.Dir, "echo", "v1.0.0", "en", "llms.txt")) {
		t.Fatal("the pinned version is evicted or rendered without the AI export")
	}
	if !exists(filepath.Join(s.cfg.Dir, "echo", "v0.9.0")) {
		t.Fatal("the pinned version of a channel is evicted")
	}
	if exists(filepath.Join(s.cfg.Dir, "echo", "v4.0.0")) {
		t.Fatal("an idle version that is not pinned is kept")
	}

	// Unpinned, it goes the usual way.
	s.Pin(nil, nil)
	s.evict()
	if exists(filepath.Join(s.cfg.Dir, "echo", "v1.0.0")) || exists(filepath.Join(s.cfg.Dir, "echo", "v0.9.0")) {
		t.Fatal("an idle version is kept after it is unpinned")
	}
}

func TestStoreRenderPinnedOnlyWhenNeeded(t *testing.T) {
	f := &fakeRender{size: 10}
	s, _ := newTestStore(t, f, 1000)

	// A reader opened the version first: it has no AI export yet.
	renderDone(t, s, "echo", "v1.0.0")

	release := Target{Module: "echo", Version: "v1.0.0"}
	branch := Target{Module: "echo", Version: "main", Rev: "sha256:aaa"}
	for _, step := range []struct {
		target Target
		calls  int32
	}{
		{release, 2}, // re-rendered with the AI export
		{release, 2}, // up to date
		{branch, 3},
		{branch, 3},
		{Target{Module: "echo", Version: "main", Rev: "sha256:bbb"}, 4}, // a new push to the branch
	} {
		if err := s.RenderPinned(context.Background(), step.target); err != nil {
			t.Fatal(err)
		}
		if n := f.calls.Load(); n != step.calls {
			t.Fatalf("%+v: renders = %d, want %d", step.target, n, step.calls)
		}
	}

	if !exists(filepath.Join(s.cfg.Dir, siteDir, "en", "modules", "echo", "main", "index.html")) {
		t.Fatal("the replaced branch version is not linked for nginx")
	}
	if entries, _ := os.ReadDir(filepath.Join(s.cfg.Dir, trashDir)); len(entries) != 0 {
		t.Fatalf("trash left: %v", entries)
	}
}

func TestPinnerRetries(t *testing.T) {
	for _, tt := range []struct {
		name     string
		err      error
		retries2 bool // rendered again on the next sync
	}{
		{"a broken render is tried on the next sync", errors.New("registry is down"), true},
		{"a version without documentation waits for retryAfter", docs.ErrNoPages, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeRender{size: 10}
			s, _ := newTestStore(t, f, 1000)
			fail := atomic.Bool{}
			fail.Store(true)
			s.cfg.Render = func(ctx context.Context, module, version, workDir, outDir string, withAI bool, stage func(Stage)) error {
				if fail.Load() {
					f.calls.Add(1)
					return tt.err
				}
				return f.render(ctx, module, version, workDir, outDir, withAI, stage)
			}

			r := Resolver{
				Modules:        func(context.Context) ([]registry.Module, error) { return []registry.Module{{Name: "echo"}}, nil },
				ChannelVersion: func(context.Context, string) (string, error) { return "v1.0.0", nil },
				Logger:         log.NewNop(),
			}
			p := NewPinner(r, s, time.Hour, log.NewNop())
			c := &clock{t: time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)}
			p.now = c.now

			p.sync(context.Background())
			p.sync(context.Background())
			want := int32(1)
			if tt.retries2 {
				want = 2
			}
			if n := f.calls.Load(); n != want {
				t.Fatalf("renders after two syncs = %d, want %d", n, want)
			}

			fail.Store(false)
			c.add(time.Hour)
			s.now = c.now
			p.sync(context.Background())
			if !exists(filepath.Join(s.cfg.Dir, "echo", "v1.0.0")) {
				t.Fatal("the version is not rendered after retryAfter")
			}
		})
	}
}

func TestPinnerWarmsUpTheVersionsToUpdateTo(t *testing.T) {
	f := &fakeRender{size: 10}
	s, _ := newTestStore(t, f, 1000)
	var noDocs atomic.Int32
	s.cfg.Render = func(ctx context.Context, module, version, workDir, outDir string, withAI bool, stage func(Stage)) error {
		if version == "v1.2.0" {
			noDocs.Add(1)
			return docs.ErrNoPages
		}
		return f.render(ctx, module, version, workDir, outDir, withAI, stage)
	}

	r := Resolver{
		Modules: func(context.Context) ([]registry.Module, error) {
			return []registry.Module{{Name: "echo", Enabled: true, Version: "v1.0.0"}}, nil
		},
		Upcoming: func(context.Context) ([]registry.Release, error) {
			return []registry.Release{
				{Module: "echo", Version: "v1.1.0"},
				{Module: "echo", Version: "v1.2.0"}, // no documentation
				{Module: "echo", Version: "v1.0.0"}, // the installed one, rendered with the search
				{Module: "echo", Version: "branch"}, // not a version
				{Module: "echo", Version: "v1.1.0"}, // listed twice
			}, nil
		},
		Logger: log.NewNop(),
	}
	p := NewPinner(r, s, time.Hour, log.NewNop())

	for range 2 {
		p.sync(context.Background())
		p.warmUp(context.Background())
	}

	if !exists(filepath.Join(s.cfg.Dir, "echo", "v1.1.0", "en", "index.html")) {
		t.Fatal("the version to update to is not rendered")
	}
	if exists(filepath.Join(s.cfg.Dir, "echo", "v1.1.0", "en", "llms.txt")) {
		t.Error("the version to update to got the AI export of the search")
	}
	if s.pinned[key("echo", "v1.1.0")] {
		t.Error("the version to update to is pinned: it would never be evicted")
	}
	// The installed version and the one to update to, once each.
	if n := f.calls.Load(); n != 2 {
		t.Errorf("renders = %d, want 2", n)
	}
	// A version without documentation waits for retryAfter, as a target does.
	if n := noDocs.Load(); n != 1 {
		t.Errorf("renders of the version without documentation = %d, want 1", n)
	}
}

func TestPinnerWarmUpRetries(t *testing.T) {
	f := &fakeRender{size: 10}
	s, _ := newTestStore(t, f, 1000)
	var noDocs, broken atomic.Bool
	noDocs.Store(true)
	broken.Store(true)
	var calls atomic.Int32
	s.cfg.Render = func(ctx context.Context, module, version, workDir, outDir string, withAI bool, stage func(Stage)) error {
		calls.Add(1)
		switch {
		case version == "v1.1.0" && noDocs.Load():
			return docs.ErrNoPages
		case version == "v1.2.0" && broken.Load():
			return errors.New("registry is down")
		}
		return f.render(ctx, module, version, workDir, outDir, withAI, stage)
	}
	upcoming := []registry.Release{{Module: "echo", Version: "v1.1.0"}, {Module: "echo", Version: "v1.2.0"}}
	var listFails atomic.Bool
	r := Resolver{
		Modules: func(context.Context) ([]registry.Module, error) {
			return []registry.Module{{Name: "echo", Enabled: true, Version: "v1.0.0"}}, nil
		},
		Upcoming: func(context.Context) ([]registry.Release, error) {
			if listFails.Load() {
				return nil, errors.New("API server is down")
			}
			return upcoming, nil
		},
		Logger: log.NewNop(),
	}
	p := NewPinner(r, s, time.Hour, log.NewNop())
	c := &clock{t: time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)}
	p.now = c.now
	pass := func() {
		p.sync(context.Background())
		p.warmUp(context.Background())
	}

	pass() // the installed version, v1.1.0 without documentation, v1.2.0 broken
	broken.Store(false)
	pass() // v1.2.0 again: a broken render is tried on the next pass
	if !exists(filepath.Join(s.cfg.Dir, "echo", "v1.2.0")) || p.warmed[Target{Module: "echo", Version: "v1.1.0"}] {
		t.Fatal("a broken render is not tried again, or a version without documentation is taken for rendered")
	}

	// The list of the versions to update to fails: what failed and what was
	// rendered is kept.
	listFails.Store(true)
	before := calls.Load()
	pass()
	if calls.Load() != before {
		t.Fatalf("a failed list of the versions to update to rendered %d more", calls.Load()-before)
	}
	listFails.Store(false)

	// After retryAfter the version without documentation is tried again.
	noDocs.Store(false)
	c.add(time.Hour)
	s.now = c.now
	pass()
	if !p.warmed[Target{Module: "echo", Version: "v1.1.0"}] {
		t.Fatal("the version is not rendered ahead after retryAfter")
	}

	// A version no longer to update to leaves the rendered set.
	upcoming = upcoming[1:]
	pass()
	if p.warmed[Target{Module: "echo", Version: "v1.1.0"}] {
		t.Fatal("a version no longer to update to stays in the rendered set")
	}
}

func TestRegistryChannelsImageGoneAfterHead(t *testing.T) {
	r := newTestRegistry(t)
	r.push("src1/echo/release:stable", release("v1.0.0")...)
	r.goneByDigest.Store(true)

	// The channel is not dropped silently: the source fails.
	got, err := RegistryChannels(r.sources(), registry.NewClients())(context.Background(), "echo")
	if err == nil || errors.Is(err, errNoChannel) || got != nil {
		t.Fatalf("channels %v, err %v; want a registry error", got, err)
	}
}

func TestPinnerChannelWithoutDocsWaitsForRetryAfter(t *testing.T) {
	f := &fakeRender{size: 10}
	s, _ := newTestStore(t, f, 1000)
	var noDocs atomic.Int32
	s.cfg.Render = func(ctx context.Context, module, version, workDir, outDir string, withAI bool, stage func(Stage)) error {
		if version == "v1.1.0" {
			noDocs.Add(1)
			return docs.ErrNoPages
		}
		return f.render(ctx, module, version, workDir, outDir, withAI, stage)
	}

	r := Resolver{
		Modules: func(context.Context) ([]registry.Module, error) { return []registry.Module{{Name: "echo"}}, nil },
		Channels: func(context.Context, string) (Channels, error) {
			return Channels{"stable": "v1.0.0", "beta": "v1.1.0"}, nil
		},
		Logger: log.NewNop(),
	}
	p := NewPinner(r, s, time.Hour, log.NewNop())
	p.sync(context.Background())
	p.sync(context.Background())

	if n := noDocs.Load(); n != 1 {
		t.Fatalf("renders of the channel version without documentation = %d, want 1", n)
	}
}

func TestWriteIndex(t *testing.T) {
	f := &fakeRender{size: 10}
	s, _ := newTestStore(t, f, 1000)

	targets := []Target{{Module: "alb", Version: "v0.17.1"}, {Module: "echo", Version: "v1.0.0"}, {Module: "missing", Version: "v1.0.0"}}
	for _, tt := range targets[:2] {
		if err := s.RenderPinned(context.Background(), tt); err != nil {
			t.Fatal(err)
		}
	}
	// Opened by a reader: in the index of its module only if it is the target.
	renderDone(t, s, "echo", "v0.9.0")

	if err := s.WriteIndex(targets); err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(s.cfg.Dir, indexDir, "en")

	var search struct {
		Documents  []struct{ URL string } `json:"documents"`
		Parameters []json.RawMessage      `json:"parameters"`
	}
	readJSON(t, filepath.Join(dir, "search.json"), &search)
	var urls []string
	for _, d := range search.Documents {
		urls = append(urls, d.URL)
	}
	if want := []string{"/modules/alb/v0.17.1/", "/modules/echo/v1.0.0/"}; !reflect.DeepEqual(urls, want) || len(search.Parameters) != 2 {
		t.Fatalf("search: urls %v, %d parameters; want %v and 2", urls, len(search.Parameters), want)
	}

	llms, err := os.ReadFile(filepath.Join(dir, "llms.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(llms), "# Modules\n") != 1 || !strings.Contains(string(llms), "## alb\n") || !strings.Contains(string(llms), "## echo\n") {
		t.Fatalf("llms.txt:\n%s", llms)
	}

	var corpus struct {
		Version   int               `json:"version"`
		Documents []json.RawMessage `json:"documents"`
	}
	readJSON(t, filepath.Join(dir, "corpus.json"), &corpus)
	if corpus.Version != 1 || len(corpus.Documents) != 2 {
		t.Fatalf("corpus: version %d, %d documents", corpus.Version, len(corpus.Documents))
	}

	// nginx serves the gzip copy to browsers that accept it.
	plain, err := os.ReadFile(filepath.Join(dir, "search.json"))
	if err != nil {
		t.Fatal(err)
	}
	gz, err := os.Open(filepath.Join(dir, "search.json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	zr, err := gzip.NewReader(gz)
	if err != nil {
		t.Fatal(err)
	}
	unzipped, err := io.ReadAll(zr)
	if err != nil || !bytes.Equal(unzipped, plain) {
		t.Fatalf("search.json.gz does not match search.json (err %v)", err)
	}

	// The other language has no versions: valid empty files.
	readJSON(t, filepath.Join(s.cfg.Dir, indexDir, "ru", "corpus.json"), &corpus)
	if len(corpus.Documents) != 0 {
		t.Fatalf("ru corpus documents = %d", len(corpus.Documents))
	}
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

// release is the content of a release image of the version.
func release(version string) []entry {
	return []entry{{name: "version.json", body: `{"version": "` + version + `"}`}}
}

func TestRegistryChannelVersion(t *testing.T) {

	tests := []struct {
		name    string
		images  map[string][]entry
		want    string
		wantErr error
	}{
		{
			name:   "stable first",
			images: map[string][]entry{"src1/echo/release:stable": release("v1.2.0"), "src1/echo/release:alpha": release("v1.3.0")},
			want:   "v1.2.0",
		},
		{
			name:   "a channel pointing at a branch build is skipped",
			images: map[string][]entry{"src1/echo/release:stable": release("mr10"), "src1/echo/release:beta": release("v1.1.0")},
			want:   "v1.1.0",
		},
		{
			name:   "channels of the next source",
			images: map[string][]entry{"src2/echo/release:early-access": release("v2.0.0")},
			want:   "v2.0.0",
		},
		{
			name:   "a release image without version.json is skipped",
			images: map[string][]entry{"src1/echo/release:stable": {{name: "module.yaml", body: "name: echo"}}, "src1/echo/release:beta": release("v1.1.0")},
			want:   "v1.1.0",
		},
		{
			name:    "no channel tags",
			images:  map[string][]entry{"src1/echo/release:v1.0.0": release("v1.0.0")},
			wantErr: errNoChannel,
		},
		{
			name:    "only branch builds in the channels",
			images:  map[string][]entry{"src1/echo/release:stable": release("0.0.1")},
			wantErr: errNoChannel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRegistry(t)
			for path, files := range tt.images {
				r.push(path, files...)
			}

			got, err := RegistryChannelVersion(r.sources(), registry.NewClients())(context.Background(), "echo")
			if !errors.Is(err, tt.wantErr) || got != tt.want {
				t.Fatalf("version %q, err %v; want %q, %v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestRegistryChannelVersionAsksHeadWhileTheChannelStays(t *testing.T) {
	r := newTestRegistry(t)
	r.push("src1/echo/release:stable", release("v1.0.0")...)
	r.takeRequests()
	channelVersion := RegistryChannelVersion(r.sources(), registry.NewClients())

	pass := func(want string) []string {
		t.Helper()
		got, err := channelVersion(context.Background(), "echo")
		if err != nil || got != want {
			t.Fatalf("version %q, %v; want %q", got, err, want)
		}
		return r.takeRequests()
	}

	pass("v1.0.0")
	// The token, the /v2/ check and the manifest are kept: one HEAD.
	if got := pass("v1.0.0"); !reflect.DeepEqual(got, []string{"HEAD /v2/src1/echo/release/manifests/stable"}) {
		t.Fatalf("an unchanged channel made requests %q; want one HEAD", got)
	}

	r.push("src1/echo/release:stable", release("v1.1.0")...)
	r.takeRequests()
	moved := pass("v1.1.0")
	if !slices.ContainsFunc(moved, func(req string) bool { return strings.HasPrefix(req, "GET /v2/src1/echo/release/manifests/sha256:") }) {
		t.Fatalf("a moved channel made requests %q; want its image read by the digest", moved)
	}

	// A registry that answers HEAD without the digest is read by the tag.
	r.push("src1/echo/release:stable", release("v1.2.0")...)
	r.takeRequests()
	r.noHeadDigest.Store(true)
	if got := pass("v1.2.0"); !slices.Contains(got, "GET /v2/src1/echo/release/manifests/stable") {
		t.Fatalf("without the digest on HEAD the requests were %q; want the manifest by the tag", got)
	}
}

func TestRegistryChannelVersionOfAnIndexAsksHeadOnly(t *testing.T) {
	r := newTestRegistry(t)
	r.pushIndex("src1/echo/release:stable", release("v1.0.0")...)
	channelVersion := RegistryChannelVersion(r.sources(), registry.NewClients())

	for range 2 {
		r.takeRequests()
		got, err := channelVersion(context.Background(), "echo")
		if err != nil || got != "v1.0.0" {
			t.Fatalf("version %q, %v; want v1.0.0", got, err)
		}
	}
	// HEAD gives the digest of the index, the image read is of the platform:
	// the index digest is what is compared on the next pass.
	if got := r.takeRequests(); !reflect.DeepEqual(got, []string{"HEAD /v2/src1/echo/release/manifests/stable"}) {
		t.Fatalf("the second pass made requests %q; want one HEAD", got)
	}
}

func TestRegistryChannelVersionImageGoneAfterHead(t *testing.T) {
	r := newTestRegistry(t)
	r.push("src1/echo/release:stable", release("v1.0.0")...)
	r.push("src1/echo/release:beta", release("v1.1.0")...)
	// The channel exists, its image does not: an error the module keeps its
	// version on, not a missing channel that moves it to the next one.
	r.goneByDigest.Store(true)
	got, err := RegistryChannelVersion(r.sources(), registry.NewClients())(context.Background(), "echo")
	if err == nil || errors.Is(err, errNoChannel) || got != "" {
		t.Fatalf("version %q, err %v; want a registry error", got, err)
	}
}

// Channel tags missing right after a lookup found them, as while the release
// repository is uploaded anew, are no module without channels yet: that
// lookup fails, the next one finds none. A failed lookup in between changes
// nothing.
func TestRegistryChannelsGoneAreNotNoneAtOnce(t *testing.T) {
	tests := []struct {
		name   string
		lookup func(r *testRegistry) func() error
	}{
		{
			name: "the version of a channel",
			lookup: func(r *testRegistry) func() error {
				return errOf(RegistryChannelVersion(r.sources(), registry.NewClients()))
			},
		},
		{
			name: "the channels of the site",
			lookup: func(r *testRegistry) func() error {
				return errOf(RegistryChannels(r.sources(), registry.NewClients()))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRegistry(t)
			r.push("src1/echo/release:stable", release("v1.0.0")...)
			lookup := tt.lookup(r)
			if err := lookup(); err != nil {
				t.Fatal(err)
			}

			r.failing.Store(&[]string{"/v2/src1/"})
			if err := lookup(); err == nil || errors.Is(err, errNoChannel) || errors.Is(err, errChannelsGone) {
				t.Fatalf("err %v; want a registry error", err)
			}
			r.failing.Store(nil)

			r.untag("src1/echo/release:stable")
			if err := lookup(); !errors.Is(err, errChannelsGone) {
				t.Fatalf("err %v right after the channel was found, want %v", err, errChannelsGone)
			}
			if err := lookup(); !errors.Is(err, errNoChannel) {
				t.Fatalf("err %v on the next lookup, want %v", err, errNoChannel)
			}
		})
	}
}

// errOf is the lookup of the module echo with its error only.
func errOf[T any](lookup func(ctx context.Context, module string) (T, error)) func() error {
	return func() error {
		_, err := lookup(context.Background(), "echo")
		return err
	}
}

// A registry that took a request and sends nothing more, no answer to the
// HEAD of a channel or a blob stopping halfway, costs the module its lookup,
// not the pass: the module keeps its last target.
func TestResolveChannelsGivesUpOnASilentRegistry(t *testing.T) {
	old := resolveTimeout
	resolveTimeout = 200 * time.Millisecond
	t.Cleanup(func() { resolveTimeout = old })

	tests := []struct {
		name   string
		silent func(w http.ResponseWriter, req *http.Request) bool
	}{
		{
			name: "no answer to the HEAD of a channel",
			silent: func(_ http.ResponseWriter, req *http.Request) bool {
				return req.Method == http.MethodHead && strings.Contains(req.URL.Path, "/release/manifests/")
			},
		},
		{
			name: "a blob stops halfway",
			silent: func(w http.ResponseWriter, req *http.Request) bool {
				if req.Method != http.MethodGet || !strings.Contains(req.URL.Path, "/blobs/") {
					return false
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte{0x1f, 0x8b})
				w.(http.Flusher).Flush()
				return true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRegistry(t)
			r.push("src1/echo/release:stable", release("v1.2.0")...)
			r.silent.Store(&tt.silent)
			resolver := Resolver{
				Modules:        func(context.Context) ([]registry.Module, error) { return []registry.Module{{Name: "echo"}}, nil },
				ChannelVersion: RegistryChannelVersion(r.sources(), registry.NewClients()),
				Latest:         func(context.Context, string) (string, error) { return "", nil },
				Logger:         log.NewNop(),
			}
			last := Target{Module: "echo", Version: "v1.1.0"}

			done := make(chan Resolution, 1)
			go func() {
				res, _ := resolver.ResolveChannels(context.Background(), map[string]Target{"echo": last})
				done <- res
			}()
			select {
			case res := <-done:
				if res.Unresolved != 1 || !reflect.DeepEqual(res.Targets, []Target{last}) {
					t.Fatalf("unresolved %d, targets %v; want the module failed with its last target", res.Unresolved, res.Targets)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("ResolveChannels still waits for the registry")
			}
		})
	}
}

// A pass ends by its deadline though a step of it would go on: here a render
// that waits for a registry as long as RenderTimeout lets it.
func TestPinnerPassEndsByItsDeadline(t *testing.T) {
	s, _ := newTestStore(t, &fakeRender{}, 1000)
	s.cfg.RenderTimeout = time.Hour
	s.cfg.Render = func(ctx context.Context, _, _, _, _ string, _ bool, _ func(Stage)) error {
		<-ctx.Done()
		return ctx.Err()
	}
	r := Resolver{
		Modules: func(context.Context) ([]registry.Module, error) {
			return []registry.Module{{Name: "echo", Enabled: true, Version: "v1.0.0"}}, nil
		},
		Logger: log.NewNop(),
	}
	p := NewPinner(r, s, time.Hour, log.NewNop())
	p.passTimeout = 100 * time.Millisecond

	done := make(chan struct{})
	go func() {
		defer close(done)
		p.pass(context.Background())
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the pass goes on past its deadline")
	}
}

// The last update of the pinned versions is reported from the start of the
// first Pinner of the pod, so the alert on it fires for a pod that never
// finishes a pass as for one that stopped finishing them. The Pinner of a
// later term of the leader leaves it; a pass moves it once every module is
// resolved.
func TestPinnerReportsLastUpdateFromTheStart(t *testing.T) {
	ms := metricsstorage.NewMetricStorage(metricsstorage.WithNewRegistry())
	if err := metrics.RegisterMetrics(ms); err != nil {
		t.Fatal(err)
	}
	s, c := newTestStore(t, &fakeRender{size: 10}, 1000)
	s.cfg.Metrics = ms
	var down atomic.Bool
	down.Store(true)
	r := Resolver{
		Modules: func(context.Context) ([]registry.Module, error) { return []registry.Module{{Name: "echo"}}, nil },
		ChannelVersion: func(context.Context, string) (string, error) {
			if down.Load() {
				return "", errors.New("registry is down")
			}
			return "v1.0.0", nil
		},
		Logger: log.NewNop(),
	}
	// A term of the leader, its Lease lost at once.
	term := func() *Pinner {
		p := NewPinner(r, s, time.Hour, log.NewNop())
		p.now = c.now
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		p.Run(ctx, time.Hour)
		return p
	}

	start := c.now()
	term()
	if got := pinSuccessTime(t, ms); !got.Equal(start) {
		t.Fatalf("after the start: %v, want the start %v", got, start)
	}

	c.add(30 * time.Minute)
	p := term()
	p.sync(context.Background())
	if got := pinSuccessTime(t, ms); !got.Equal(start) {
		t.Fatalf("after a later term and a pass with the registry down: %v, want the start %v", got, start)
	}

	c.add(30 * time.Minute)
	down.Store(false)
	p.sync(context.Background())
	if got := pinSuccessTime(t, ms); !got.Equal(c.now()) {
		t.Fatalf("after a pass: %v, want %v", got, c.now())
	}
}

// A pass with nothing to publish is an update too: a cluster without
// modules from a registry, or with modules without documentation only, must
// not keep the alert on the last update going.
func TestPinnerPassWithNothingToPublishIsAnUpdate(t *testing.T) {
	for _, tt := range []struct {
		name    string
		modules []registry.Module
	}{
		{"no modules from a registry", nil},
		{"a module without documentation", []registry.Module{{Name: "echo"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ms := metricsstorage.NewMetricStorage(metricsstorage.WithNewRegistry())
			if err := metrics.RegisterMetrics(ms); err != nil {
				t.Fatal(err)
			}
			s, c := newTestStore(t, &fakeRender{}, 1000)
			s.cfg.Metrics = ms
			s.cfg.Render = func(context.Context, string, string, string, string, bool, func(Stage)) error {
				return docs.ErrNoPages
			}
			r := Resolver{
				Modules:        func(context.Context) ([]registry.Module, error) { return tt.modules, nil },
				ChannelVersion: func(context.Context, string) (string, error) { return "v1.0.0", nil },
				Logger:         log.NewNop(),
			}
			p := NewPinner(r, s, time.Hour, log.NewNop())
			p.now = c.now

			p.sync(context.Background())
			if got := pinSuccessTime(t, ms); !got.Equal(c.now()) {
				t.Fatalf("after a pass: %v, want %v", got, c.now())
			}
		})
	}
}

// A pass that publishes nothing while there is something to publish is no
// update: release repositories answering "not found" for every module, as
// after the registry moved, take every module off the site on the second
// pass, and the alert on the last update must fire.
func TestPinnerPassUnpublishingEveryModuleIsNoUpdate(t *testing.T) {
	ms := metricsstorage.NewMetricStorage(metricsstorage.WithNewRegistry())
	if err := metrics.RegisterMetrics(ms); err != nil {
		t.Fatal(err)
	}
	s, c := newTestStore(t, &fakeRender{size: 10}, 1000)
	s.cfg.Metrics = ms
	var gone atomic.Bool
	notFound := fmt.Errorf("%w: source main: %w", ErrNoVersions, &transport.Error{StatusCode: http.StatusNotFound})
	r := siteResolver()
	r.Channels = func(context.Context, string) (Channels, error) {
		if gone.Load() {
			return nil, errNoChannel
		}
		return Channels{"stable": "v1.0.0"}, nil
	}
	r.Latest = func(context.Context, string) (string, error) { return "", notFound }
	p := NewPinner(r, s, time.Hour, log.NewNop())
	p.now = c.now

	p.sync(context.Background())
	gone.Store(true)
	// The first such pass keeps the module (keepUnpublishedOnce).
	c.add(15 * time.Minute)
	p.sync(context.Background())
	last := c.now()

	for range 2 {
		c.add(15 * time.Minute)
		p.sync(context.Background())
		if len(p.indexed) != 0 {
			t.Fatalf("indexed %v, want the module off the site", p.indexed)
		}
		if got := pinSuccessTime(t, ms); !got.Equal(last) {
			t.Fatalf("after a pass with every module unpublished: %v, want the last update %v", got, last)
		}
	}
}

// Nor is a pass whose every target fails for a lasting reason other than
// having no documentation, on an empty volume: the search stays empty.
func TestPinnerPassPublishingNothingIsNoUpdate(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
	}{
		{"the Hugo build fails", fmt.Errorf("%w: template error", docs.ErrBuild)},
		{"the version is not found", fmt.Errorf("%w: no source has it", ErrVersionNotFound)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ms := metricsstorage.NewMetricStorage(metricsstorage.WithNewRegistry())
			if err := metrics.RegisterMetrics(ms); err != nil {
				t.Fatal(err)
			}
			s, c := newTestStore(t, &fakeRender{}, 1000)
			s.cfg.Metrics = ms
			s.cfg.Render = func(context.Context, string, string, string, string, bool, func(Stage)) error {
				return tt.err
			}
			r := Resolver{
				Modules:        func(context.Context) ([]registry.Module, error) { return []registry.Module{{Name: "echo"}}, nil },
				ChannelVersion: func(context.Context, string) (string, error) { return "v1.0.0", nil },
				Logger:         log.NewNop(),
			}
			p := NewPinner(r, s, time.Hour, log.NewNop())
			p.now = c.now
			start := c.now()
			s.initPinSuccessMetric(start)

			c.add(15 * time.Minute)
			p.sync(context.Background())
			if got := pinSuccessTime(t, ms); !got.Equal(start) {
				t.Fatalf("after a pass: %v, want the start %v", got, start)
			}
		})
	}
}

// pinSuccessTime is the last update of the pinned versions as reported.
func pinSuccessTime(t *testing.T, ms *metricsstorage.MetricStorage) time.Time {
	t.Helper()

	v, err := strconv.ParseFloat(metricValue(t, ms, metrics.DocsBuilderOnDemandPinSuccessSeconds), 64)
	if err != nil {
		t.Fatalf("the last update of the pinned versions is not reported: %v", err)
	}

	return time.Unix(int64(v), 0).UTC()
}

func TestStoreReaderSharesPinnedRender(t *testing.T) {
	f := &fakeRender{size: 10, release: make(chan struct{})}
	s, _ := newTestStore(t, f, 1000)

	target := Target{Module: "echo", Version: "v1.0.0"}
	done := make(chan error)
	go func() { done <- s.RenderPinned(context.Background(), target) }()

	waitStage(t, s, "echo", "v1.0.0", StageDownload)
	if st, err := s.Start("echo", "v1.0.0"); err != nil || st.Stage != StageDownload {
		t.Fatalf("reader during the pinned render: %+v, %v; want the shared download stage", st, err)
	}

	close(f.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if n := f.calls.Load(); n != 1 {
		t.Fatalf("renders = %d, want 1", n)
	}
	if !exists(filepath.Join(s.cfg.Dir, "echo", "v1.0.0", "en", "llms.txt")) {
		t.Fatal("the published version lost the AI export")
	}
}

func TestStoreRenderPinnedWaitsForReaderRender(t *testing.T) {
	f := &fakeRender{size: 10, release: make(chan struct{})}
	s, _ := newTestStore(t, f, 1000)

	if _, err := s.Start("echo", "v1.0.0"); err != nil {
		t.Fatal(err)
	}
	waitStage(t, s, "echo", "v1.0.0", StageDownload)

	done := make(chan error)
	go func() { done <- s.RenderPinned(context.Background(), Target{Module: "echo", Version: "v1.0.0"}) }()

	close(f.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	// The reader's render had no AI export, so the pinned one follows it.
	if n := f.calls.Load(); n != 2 {
		t.Fatalf("renders = %d, want 2", n)
	}
	if !exists(filepath.Join(s.cfg.Dir, "echo", "v1.0.0", "en", "llms.txt")) {
		t.Fatal("the pinned render did not replace the reader's one")
	}
}

func TestWriteIndexKeepsFilesWhenNothingNew(t *testing.T) {
	f := &fakeRender{size: 10}
	s, _ := newTestStore(t, f, 1000)
	search := filepath.Join(s.cfg.Dir, indexDir, "en", "search.json")

	// No target is published (a registry outage at start): no empty index
	// hides the one nginx falls back to.
	if err := s.WriteIndex([]Target{{Module: "echo", Version: "v1.0.0"}}); err != nil {
		t.Fatal(err)
	}
	if exists(search) {
		t.Fatal("an index is written without any published version")
	}

	target := Target{Module: "echo", Version: "v1.0.0"}
	if err := s.RenderPinned(context.Background(), target); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteIndex([]Target{target}); err != nil {
		t.Fatal(err)
	}
	// A file written again would lose the time set here.
	written := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(search, written, written); err != nil {
		t.Fatal(err)
	}

	if err := s.WriteIndex([]Target{target}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(search)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(written) {
		t.Fatal("an unchanged index is written again")
	}
}

func TestMergeCorpusWithoutDocuments(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "corpus.json"), []byte(`{"version":1}`), 0600); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := mergeCorpus([]string{dir}, &buf); err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Documents []json.RawMessage `json:"documents"`
	}
	if err := json.Unmarshal(buf.Bytes(), &corpus); err != nil || len(corpus.Documents) != 0 {
		t.Fatalf("corpus %q, err %v", buf.String(), err)
	}
}

func TestStorePinnedAboveMaxSizeKeepReaderVersions(t *testing.T) {
	// A version takes size plus a few hundred bytes of search.json, llms.txt
	// and corpus.json.
	f := &fakeRender{size: 100}
	s, _ := newTestStore(t, f, 400)

	pinned := []Target{{Module: "alb", Version: "v1.0.0"}, {Module: "echo", Version: "v1.0.0"}}
	s.Pin(pinned, nil)
	for _, tt := range pinned {
		if err := s.RenderPinned(context.Background(), tt); err != nil {
			t.Fatal(err)
		}
	}

	// The pinned versions alone are above MaxSize; a reader's version fits it.
	renderDone(t, s, "echo", "v2.0.0")
	s.evict()

	if !exists(filepath.Join(s.cfg.Dir, "echo", "v2.0.0")) {
		t.Fatal("the version a reader just opened is evicted for the size of the pinned ones")
	}
}

func TestStoreRenderPinnedReportsMissingAIExport(t *testing.T) {
	f := &fakeRender{size: 10}
	s, _ := newTestStore(t, f, 1000)
	// The export failed: RenderVersion only logs it.
	s.cfg.Render = func(ctx context.Context, module, version, workDir, outDir string, _ bool, stage func(Stage)) error {
		return f.render(ctx, module, version, workDir, outDir, false, stage)
	}

	target := Target{Module: "echo", Version: "v1.0.0"}
	if err := s.RenderPinned(context.Background(), target); !errors.Is(err, errNoAIExport) {
		t.Fatalf("err = %v, want errNoAIExport", err)
	}
	if s.versions[key("echo", "v1.0.0")].ai {
		t.Fatal("the version is marked as rendered with the AI export")
	}
}

func TestStoreReaderRendersPinnedVersionWithAIExport(t *testing.T) {
	f := &fakeRender{size: 10}
	s, _ := newTestStore(t, f, 1000)

	target := Target{Module: "echo", Version: "v1.0.0"}
	s.Pin([]Target{target}, nil)
	renderDone(t, s, "echo", "v1.0.0")
	if err := s.RenderPinned(context.Background(), target); err != nil {
		t.Fatal(err)
	}

	if n := f.calls.Load(); n != 1 {
		t.Fatalf("renders = %d, want 1", n)
	}
}

func TestPinnerKeepsPreviousTargetUntilTheNewOneIsPublished(t *testing.T) {
	f := &fakeRender{size: 10}
	s, c := newTestStore(t, f, 1000)
	var broken atomic.Bool
	s.cfg.Render = func(ctx context.Context, module, version, workDir, outDir string, withAI bool, stage func(Stage)) error {
		if version == "v2.0.0" && broken.Load() {
			return docs.ErrNoPages
		}
		return f.render(ctx, module, version, workDir, outDir, withAI, stage)
	}

	var version atomic.Value
	version.Store("v1.0.0")
	r := Resolver{
		Modules:        func(context.Context) ([]registry.Module, error) { return []registry.Module{{Name: "echo"}}, nil },
		ChannelVersion: func(context.Context, string) (string, error) { return version.Load().(string), nil },
		Logger:         log.NewNop(),
	}
	p := NewPinner(r, s, time.Hour, log.NewNop())
	p.now = c.now

	indexed := func() []string {
		var search struct {
			Documents []struct{ URL string } `json:"documents"`
		}
		readJSON(t, filepath.Join(s.cfg.Dir, indexDir, "en", "search.json"), &search)
		var urls []string
		for _, d := range search.Documents {
			urls = append(urls, d.URL)
		}
		return urls
	}

	p.sync(context.Background())

	// The channel moves to a version that does not render.
	version.Store("v2.0.0")
	broken.Store(true)
	p.sync(context.Background())
	c.add(2 * time.Hour) // past MaxIdle
	s.evict()

	if got, want := indexed(), []string{"/modules/echo/v1.0.0/"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("index %v, want %v", got, want)
	}
	if !exists(filepath.Join(s.cfg.Dir, "echo", "v1.0.0")) {
		t.Fatal("the indexed version is evicted before the new target is published")
	}
	// Its card shows the published version: it has documentation.
	var state indexedState
	readJSON(t, filepath.Join(s.cfg.Dir, indexDir, indexedFile), &state)
	if want := []Listed{{Module: "echo"}}; !reflect.DeepEqual(state.Modules, want) {
		t.Fatalf("modules = %+v, want %+v", state.Modules, want)
	}

	// Published, the new target takes its place and the old one goes.
	broken.Store(false)
	p.sync(context.Background())
	s.evict()

	if got, want := indexed(), []string{"/modules/echo/v2.0.0/"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("index %v, want %v", got, want)
	}
	if exists(filepath.Join(s.cfg.Dir, "echo", "v1.0.0")) {
		t.Fatal("the previous target is kept after the new one is published")
	}
}

func TestPinnerKeepsPreviousTargetAcrossRestart(t *testing.T) {
	f := &fakeRender{size: 10}
	s, c := newTestStore(t, f, 1000)
	var version atomic.Value
	version.Store("v1.0.0")
	r := Resolver{
		Modules: func(context.Context) ([]registry.Module, error) {
			return []registry.Module{{Name: "alb"}, {Name: "echo"}}, nil
		},
		ChannelVersion: func(_ context.Context, module string) (string, error) {
			if module == "alb" {
				return "v0.17.1", nil
			}
			return version.Load().(string), nil
		},
		Logger: log.NewNop(),
	}
	NewPinner(r, s, time.Hour, log.NewNop()).sync(context.Background())
	// The render time is the access time a restart knows.
	if err := os.Chtimes(filepath.Join(s.cfg.Dir, "echo", "v1.0.0"), c.now(), c.now()); err != nil {
		t.Fatal(err)
	}

	// After a restart the channel points at a version without documentation.
	version.Store("v2.0.0")
	cfg := s.cfg
	cfg.Render = func(context.Context, string, string, string, string, bool, func(Stage)) error { return docs.ErrNoPages }
	s = NewStore(cfg)
	s.now = c.now
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}
	c.add(2 * time.Hour) // past MaxIdle
	p := NewPinner(r, s, time.Hour, log.NewNop())
	p.now = c.now
	p.sync(context.Background())
	s.evict()

	var search struct {
		Documents []struct{ URL string } `json:"documents"`
	}
	readJSON(t, filepath.Join(s.cfg.Dir, indexDir, "en", "search.json"), &search)
	var urls []string
	for _, d := range search.Documents {
		urls = append(urls, d.URL)
	}
	if want := []string{"/modules/alb/v0.17.1/", "/modules/echo/v1.0.0/"}; !reflect.DeepEqual(urls, want) {
		t.Fatalf("index %v, want %v", urls, want)
	}
	if !exists(filepath.Join(s.cfg.Dir, "echo", "v1.0.0")) {
		t.Fatal("the indexed version is evicted after a restart")
	}
}

func TestHasAIExportNeedsEveryLanguage(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"en/index.html", "en/llms.txt", "ru/index.html"} {
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(f)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, f), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if hasAIExport(dir) {
		t.Fatal("a version whose ru export failed counts as exported")
	}

	if err := os.WriteFile(filepath.Join(dir, "ru", "llms.txt"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if !hasAIExport(dir) {
		t.Fatal("a version exported in every language does not count as exported")
	}
}

func TestRegistryChannels(t *testing.T) {
	release := func(version string) []entry {
		return []entry{{name: "version.json", body: `{"version": "` + version + `"}`}}
	}

	tests := []struct {
		name    string
		images  map[string][]entry
		want    Channels
		wantErr error
	}{
		{
			name: "every channel of the site",
			images: map[string][]entry{
				"src1/echo/release:alpha": release("v1.3.0"), "src1/echo/release:beta": release("v1.3.0"),
				"src1/echo/release:early-access": release("v1.2.0"), "src1/echo/release:stable": release("v1.2.0"),
				"src1/echo/release:rock-solid": release("v1.1.0"),
			},
			want: Channels{"alpha": "v1.3.0", "beta": "v1.3.0", "early-access": "v1.2.0", "stable": "v1.2.0", "rock-solid": "v1.1.0"},
		},
		{
			name:   "a channel pointing at a branch build is left out",
			images: map[string][]entry{"src1/echo/release:alpha": release("mr10"), "src1/echo/release:stable": release("v1.2.0")},
			want:   Channels{"stable": "v1.2.0"},
		},
		{
			name: "the channels never mix two sources",
			images: map[string][]entry{
				"src1/echo/release:alpha": release("v9.0.0"),
				"src2/echo/release:alpha": release("v1.3.0"), "src2/echo/release:stable": release("v1.2.0"),
			},
			want: Channels{"alpha": "v9.0.0"},
		},
		{
			name:   "the next source when the first has none",
			images: map[string][]entry{"src2/echo/release:stable": release("v2.0.0")},
			want:   Channels{"stable": "v2.0.0"},
		},
		{
			name:    "no channel tags",
			images:  map[string][]entry{"src1/echo/release:v1.0.0": release("v1.0.0")},
			wantErr: errNoChannel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRegistry(t)
			for path, files := range tt.images {
				r.push(path, files...)
			}

			got, err := RegistryChannels(r.sources(), registry.NewClients())(context.Background(), "echo")
			if !errors.Is(err, tt.wantErr) || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("channels %v, err %v; want %v, %v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

// A module Hugo can not build waits for retryAfter like one without pages
// instead of being rendered on every pass.
func TestPinnerBuildFailureDoesNotHoldTheUpdate(t *testing.T) {
	f := &fakeRender{size: 10}
	s, c := newTestStore(t, f, 1000)
	render := s.cfg.Render
	s.cfg.Render = func(ctx context.Context, module, version, workDir, outDir string, withAI bool, stage func(Stage)) error {
		if module == "broken" {
			f.calls.Add(1)
			return fmt.Errorf("%w: template error", docs.ErrBuild)
		}
		return render(ctx, module, version, workDir, outDir, withAI, stage)
	}
	r := Resolver{
		Modules: func(context.Context) ([]registry.Module, error) {
			return []registry.Module{{Name: "echo"}, {Name: "broken"}}, nil
		},
		ChannelVersion: func(context.Context, string) (string, error) { return "v1.0.0", nil },
		Logger:         log.NewNop(),
	}
	p := NewPinner(r, s, time.Hour, log.NewNop())
	p.now = c.now
	p.sync(context.Background())
	calls := f.calls.Load()
	p.sync(context.Background())

	if f.calls.Load() != calls {
		t.Errorf("renders %d -> %d: the module Hugo can not build is rendered on every pass", calls, f.calls.Load())
	}
	if _, ok := p.failed[Target{Module: "broken", Version: "v1.0.0"}]; !ok {
		t.Error("the module Hugo can not build is not kept as failed")
	}
}

func TestHeadUnsupported(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tt := range []struct {
		name string
		ctx  context.Context
		err  error
		want bool
	}{
		{"no digest in the answer", context.Background(), errors.New("HEAD x: response did not include Docker-Content-Digest header"), true},
		{"HEAD not allowed", context.Background(), &transport.Error{StatusCode: http.StatusMethodNotAllowed}, true},
		{"registry error", context.Background(), &transport.Error{StatusCode: http.StatusInternalServerError}, false},
		{"registry unreachable", context.Background(), &url.Error{Op: "Head", URL: "https://r.example", Err: errors.New("connection refused")}, false},
		{"deadline of the request", context.Background(), context.DeadlineExceeded, false},
		{"request given up", canceled, errors.New("HEAD x: response did not include Docker-Content-Digest header"), false},
	} {
		if got := headUnsupported(tt.ctx, tt.err); got != tt.want {
			t.Errorf("%s: %v, want %v", tt.name, got, tt.want)
		}
	}
}

// A leader whose first read of targets.json fails records nothing over it:
// the followers would unpin what it lists. The next pass reads it again.
func TestPinnerSavesNothingUntilTargetsAreRead(t *testing.T) {
	cold := newMemCold()
	putIndexed(t, cold, indexedState{
		Targets: []Target{{Module: "alb", Version: "v0.17.1"}},
		Modules: []Listed{{Module: "alb"}},
	})
	good := slices.Clone(cold.objects[indexedFile])
	s := newColdStore(t, &fakeRender{size: 10}, cold)
	s.cfg.Render = func(context.Context, string, string, string, string, bool, func(Stage)) error {
		return errors.New("registry is down")
	}
	restored := 0
	p := NewPinner(siteResolver(), s, time.Hour, log.NewNop())
	p.OnRestored = func() { restored++ }

	cold.mu.Lock()
	cold.failGet = true
	cold.mu.Unlock()
	p.sync(context.Background())
	cold.mu.Lock()
	cold.failGet = false
	got := slices.Clone(cold.objects[indexedFile])
	cold.mu.Unlock()
	if !bytes.Equal(got, good) {
		t.Fatalf("targets.json replaced while it could not be read: %s", got)
	}
	if restored != 1 {
		t.Fatalf("OnRestored called %d times after the first pass, want 1", restored)
	}

	p.sync(context.Background())
	var state indexedState
	cold.mu.Lock()
	err := json.Unmarshal(cold.objects[indexedFile], &state)
	cold.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	// Saved by the second pass, once read.
	if want := []Listed{{Module: "echo"}}; !reflect.DeepEqual(state.Modules, want) {
		t.Fatalf("modules = %+v, want %+v", state.Modules, want)
	}
	if restored != 1 {
		t.Fatalf("OnRestored called %d times after two passes, want 1", restored)
	}
}

// A module whose registry does not answer keeps the target of targets.json
// even when the file is read on the second pass only.
func TestPinnerKeepsIndexedTargetReadLate(t *testing.T) {
	cold := newMemCold()
	alb := Target{Module: "alb", Version: "v0.17.1"}
	putIndexed(t, cold, indexedState{
		Targets: []Target{alb, {Module: "echo", Version: "v1.0.0"}},
		Modules: []Listed{{Module: "alb"}, {Module: "echo"}},
	})
	s := newColdStore(t, &fakeRender{size: 10}, cold)
	r := Resolver{
		Modules: func(context.Context) ([]registry.Module, error) {
			return []registry.Module{{Name: "alb"}, {Name: "echo"}}, nil
		},
		ChannelVersion: func(_ context.Context, module string) (string, error) {
			if module == "alb" {
				return "", errors.New("registry is down")
			}
			return "v1.0.0", nil
		},
		Logger: log.NewNop(),
	}
	p := NewPinner(r, s, time.Hour, log.NewNop())

	cold.mu.Lock()
	cold.failGet = true
	cold.mu.Unlock()
	p.sync(context.Background())
	cold.mu.Lock()
	cold.failGet = false
	cold.mu.Unlock()
	p.sync(context.Background())

	var state indexedState
	cold.mu.Lock()
	err := json.Unmarshal(cold.objects[indexedFile], &state)
	cold.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(state.Targets, alb) {
		t.Fatalf("targets %v, want %v kept", state.Targets, alb)
	}
}

// A module that starts running its embedded copy leaves the list (see
// registry.Sources.Modules): the next pass drops its target and the stable
// link, so nginx serves the embedded copy. Back on a source, the next pass
// links it again.
func TestPinnerFollowsAModuleBetweenEmbeddedAndASource(t *testing.T) {
	s, c := newTestStore(t, &fakeRender{size: 10}, 1000)
	var external atomic.Bool
	external.Store(true)
	r := Resolver{
		Modules: func(context.Context) ([]registry.Module, error) {
			if external.Load() {
				return []registry.Module{{Name: "echo"}}, nil
			}
			return nil, nil
		},
		ChannelVersion: func(context.Context, string) (string, error) { return "v1.0.0", nil },
		// A release the cluster downloaded ahead from the source.
		Upcoming: func(context.Context) ([]registry.Release, error) {
			return []registry.Release{{Module: "echo", Version: "v2.0.0"}}, nil
		},
		Logger: log.NewNop(),
	}
	var relisted []string
	s.cfg.Relisted = func(module string) { relisted = append(relisted, module) }
	p := NewPinner(r, s, time.Hour, log.NewNop())
	p.now = c.now
	stable := filepath.Join(s.cfg.Dir, s.site, "en", "modules", "echo", "stable")
	check := func(want bool) {
		t.Helper()
		_, linked := os.Lstat(stable)
		_, target := s.Target("echo")
		_, menu := s.Menu("echo")
		upcoming := len(p.upcoming) > 0
		if (linked == nil) != want || target != want || menu != want || upcoming != want {
			t.Fatalf("on demand = link %v, target %v, menu %v, upcoming %v; want %v", linked == nil, target, menu, upcoming, want)
		}
	}

	p.sync(context.Background())
	check(true)

	external.Store(false)
	p.sync(context.Background())
	check(false)
	// The cached versions go each time: none opens while it is embedded.
	if want := []string{"echo", "echo"}; !reflect.DeepEqual(relisted, want) {
		t.Fatalf("relisted = %v, want %v", relisted, want)
	}

	external.Store(true)
	p.sync(context.Background())
	check(true)
	if want := []string{"echo", "echo", "echo"}; !reflect.DeepEqual(relisted, want) {
		t.Fatalf("relisted = %v, want %v", relisted, want)
	}
}

// A local module (local.Source) is an enabled module whose override tag is
// its version and whose digest is the revision of its files: a kick after
// the files change renders the version anew, and the stable channel keeps
// serving the module meanwhile.
func TestPinnerKickRendersAChangedRevision(t *testing.T) {
	f := &fakeRender{size: 10}
	s, _ := newTestStore(t, f, 1000)
	var rev atomic.Value
	rev.Store("rev-1")
	// Every pass lists the modules once, at its start.
	var passes atomic.Int32
	r := Resolver{
		Modules: func(context.Context) ([]registry.Module, error) {
			passes.Add(1)
			return []registry.Module{{Name: "echo", Enabled: true, Override: &registry.Override{Tag: "v0.0.0-local", Digest: rev.Load().(string)}}}, nil
		},
		ChannelVersion: func(context.Context, string) (string, error) { return "v0.0.0-local", nil },
		Logger:         log.NewNop(),
	}
	p := NewPinner(r, s, time.Hour, log.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		p.Run(ctx, time.Hour)
	}()

	renders := func(n int32) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for f.calls.Load() < n {
			if time.Now().After(deadline) {
				t.Fatalf("renders = %d, want %d", f.calls.Load(), n)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	// kick kicks the Pinner and returns once the pass it makes has started,
	// which is after the pass running at the kick, if any, is over.
	kick := func() {
		t.Helper()
		n := passes.Load()
		p.Kick()
		deadline := time.Now().Add(10 * time.Second)
		for passes.Load() == n {
			if time.Now().After(deadline) {
				t.Fatalf("no pass after a kick, passes = %d", n)
			}
			time.Sleep(time.Millisecond)
		}
	}
	renders(1)
	// The first pass is over, the second one is the kick without a change.
	kick()
	if menu, ok := s.Menu("echo"); !ok || menu.Channels["stable"] != "v0.0.0-local" {
		t.Fatalf("menu = %+v, %v, want the stable channel at the version", menu, ok)
	}
	// Kicked without a change: no render by the end of that pass.
	kick()
	if n := f.calls.Load(); n != 1 {
		t.Fatalf("renders after a kick without a change = %d, want 1", n)
	}

	rev.Store("rev-2")
	p.Kick()
	renders(2)

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop")
	}
	if !exists(filepath.Join(s.cfg.Dir, "echo", "v0.0.0-local")) {
		t.Fatal("the version is not published")
	}
}
