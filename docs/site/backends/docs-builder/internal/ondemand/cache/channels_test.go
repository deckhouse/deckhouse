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
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/ondemand"
	"github.com/flant/docs-builder/internal/registry"
)

func TestResolverChannels(t *testing.T) {
	r := Resolver{
		Modules: func(context.Context) ([]registry.Module, error) {
			return []registry.Module{{Name: "echo"}, {Name: "bare"}, {Name: "solid"}}, nil
		},
		Policy: SitePolicy{
			Channels: func(_ context.Context, module string) (ondemand.Channels, error) {
				switch module {
				case "echo":
					return ondemand.Channels{"alpha": "v2.0.0", "stable": "v1.0.0", "rock-solid": "v0.9.0"}, nil
				case "solid":
					return ondemand.Channels{"rock-solid": "v0.9.0"}, nil
				}
				return nil, ondemand.ErrNoChannel
			},
			Latest: func(context.Context, string) (string, error) { return "v3.0.0", nil },
		},
		Logger: log.NewNop(),
	}

	res, err := r.ResolveChannels(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	targets, chans := res.Targets, res.Channels
	wantTargets := []ondemand.Target{{Module: "bare", Version: "v3.0.0"}, {Module: "echo", Version: "v1.0.0"}, {Module: "solid", Version: "v0.9.0"}}
	if !reflect.DeepEqual(targets, wantTargets) {
		t.Errorf("targets = %v, want %v", targets, wantTargets)
	}
	wantChans := map[string]ondemand.Channels{
		"echo": {"alpha": "v2.0.0", "stable": "v1.0.0", "rock-solid": "v0.9.0"},
		// A module without channel tags gets its newest version as stable.
		"bare":  {"stable": "v3.0.0"},
		"solid": {"rock-solid": "v0.9.0"},
	}
	if !reflect.DeepEqual(chans, wantChans) {
		t.Errorf("channels = %v, want %v", chans, wantChans)
	}
}

func channelLink(t *testing.T, s *Store, module, channel string) string {
	t.Helper()

	target, err := os.Readlink(filepath.Join(s.cfg.Dir, siteDir, "en", "modules", module, channel))
	if err != nil {
		return ""
	}

	return target
}

func TestPinnerServesChannels(t *testing.T) {
	f := &fakeRender{size: 10}
	s, _ := newTestStore(t, f, 1000)
	var mu sync.Mutex
	chans := ondemand.Channels{"alpha": "v2.0.0", "beta": "v2.0.0", "stable": "v1.0.0"}
	r := Resolver{
		Modules: func(context.Context) ([]registry.Module, error) { return []registry.Module{{Name: "echo"}}, nil },
		Policy: SitePolicy{
			Channels: func(context.Context, string) (ondemand.Channels, error) {
				mu.Lock()
				defer mu.Unlock()
				out := ondemand.Channels{}
				for k, v := range chans {
					out[k] = v
				}
				return out, nil
			},
		},
		Logger: log.NewNop(),
	}
	p := NewPinner(r, s, time.Hour, log.NewNop())
	p.sync(context.Background())

	// One render per version: two channels on one version share it.
	if n := f.calls.Load(); n != 2 {
		t.Fatalf("renders = %d, want 2", n)
	}
	for channel, version := range map[string]string{"alpha": "v2.0.0", "beta": "v2.0.0", "stable": "v1.0.0", "early-access": ""} {
		want := ""
		if version != "" {
			want = filepath.Join("..", "..", "..", "..", "echo", version, "en")
		}
		if got := channelLink(t, s, "echo", channel); got != want {
			t.Errorf("%s -> %q, want %q", channel, got, want)
		}
	}
	// Only the version of the most stable channel feeds the search.
	if !s.versions[key("echo", "v1.0.0")].ai || s.versions[key("echo", "v2.0.0")].ai {
		t.Error("the AI export is rendered for a version other than the stable one")
	}
	state, err := s.loadIndexed(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]ondemand.Channels{"echo": {"alpha": "v2.0.0", "beta": "v2.0.0", "stable": "v1.0.0"}}; !reflect.DeepEqual(state.Channels, want) {
		t.Errorf("recorded channels %v, want %v", state.Channels, want)
	}

	// alpha moves on, beta goes away.
	mu.Lock()
	chans = ondemand.Channels{"alpha": "v3.0.0", "stable": "v1.0.0"}
	mu.Unlock()
	p.sync(context.Background())
	if got := channelLink(t, s, "echo", "alpha"); got != filepath.Join("..", "..", "..", "..", "echo", "v3.0.0", "en") {
		t.Errorf("alpha -> %q after the move", got)
	}
	if got := channelLink(t, s, "echo", "beta"); got != "" {
		t.Errorf("beta -> %q, want no link", got)
	}

	// A restart keeps the channel pages served before the first pass.
	restarted := NewStore(s.cfg)
	if err := restarted.Load(); err != nil {
		t.Fatal(err)
	}
	if got := channelLink(t, restarted, "echo", "stable"); got != filepath.Join("..", "..", "..", "..", "echo", "v1.0.0", "en") {
		t.Errorf("stable -> %q after a restart", got)
	}
}

func TestSiteIndexLeadsToModuleRoot(t *testing.T) {
	f := &fakeRender{size: 10}
	s, _ := newTestStore(t, f, 1000, func(cfg *Config) { cfg.IndexAtModuleRoot = true })
	renderDone(t, s, "echo", "v1.0.0")
	s.LinkChannels("echo", ondemand.Channels{"stable": "v1.0.0"})

	if err := s.WriteIndex([]ondemand.Target{{Module: "echo", Version: "v1.0.0"}}); err != nil {
		t.Fatal(err)
	}
	var search struct {
		Documents []struct{ URL string } `json:"documents"`
	}
	readJSON(t, filepath.Join(s.cfg.Dir, indexDir, "en", "search.json"), &search)
	if len(search.Documents) != 1 || search.Documents[0].URL != "/modules/echo/" {
		t.Fatalf("search documents %+v, want the module root", search.Documents)
	}
}

// A module with no stable channel is not at its root: the search leads to
// the version.
func TestSiteIndexWithoutStable(t *testing.T) {
	f := &fakeRender{size: 10}
	s, _ := newTestStore(t, f, 1000, func(cfg *Config) { cfg.IndexAtModuleRoot = true })
	renderDone(t, s, "echo", "v2.0.0")
	s.LinkChannels("echo", ondemand.Channels{"alpha": "v2.0.0"})

	if err := s.WriteIndex([]ondemand.Target{{Module: "echo", Version: "v2.0.0"}}); err != nil {
		t.Fatal(err)
	}
	var search struct {
		Documents []struct{ URL string } `json:"documents"`
	}
	readJSON(t, filepath.Join(s.cfg.Dir, indexDir, "en", "search.json"), &search)
	if len(search.Documents) != 1 || search.Documents[0].URL != "/modules/echo/v2.0.0/" {
		t.Fatalf("search documents %+v, want the version", search.Documents)
	}
}

// A channel moving to a version that is not rendered yet keeps serving the
// old one, pinned, until the new one is published.
func TestPinnerKeepsChannelUntilNewVersionIsPublished(t *testing.T) {
	f := &fakeRender{size: 10}
	s, c := newTestStore(t, f, 1000)
	var mu sync.Mutex
	chans := ondemand.Channels{"alpha": "v2.0.0", "stable": "v1.0.0"}
	var broken bool
	render := s.cfg.Render
	s.cfg.Render = func(ctx context.Context, module, version, workDir, outDir string, withAI bool, stage func(ondemand.Stage)) error {
		mu.Lock()
		b := broken && version == "v3.0.0"
		mu.Unlock()
		if b {
			return errors.New("registry is down")
		}
		return render(ctx, module, version, workDir, outDir, withAI, stage)
	}
	r := Resolver{
		Modules: func(context.Context) ([]registry.Module, error) { return []registry.Module{{Name: "echo"}}, nil },
		Policy: SitePolicy{
			Channels: func(context.Context, string) (ondemand.Channels, error) {
				mu.Lock()
				defer mu.Unlock()
				out := ondemand.Channels{}
				for k, v := range chans {
					out[k] = v
				}
				return out, nil
			},
		},
		Logger: log.NewNop(),
	}
	p := NewPinner(r, s, time.Hour, log.NewNop())
	p.now = c.now
	p.sync(context.Background())

	mu.Lock()
	chans = ondemand.Channels{"alpha": "v3.0.0", "stable": "v1.0.0"}
	broken = true
	mu.Unlock()
	p.sync(context.Background())
	c.add(30 * 24 * time.Hour) // past MaxIdle
	s.evict()

	if got := channelLink(t, s, "echo", "alpha"); got != filepath.Join("..", "..", "..", "..", "echo", "v2.0.0", "en") {
		t.Fatalf("alpha -> %q, want the old version until the new one renders", got)
	}
	if !exists(filepath.Join(s.cfg.Dir, "echo", "v2.0.0")) {
		t.Fatal("the version alpha still serves is evicted")
	}

	mu.Lock()
	broken = false
	mu.Unlock()
	p.sync(context.Background())
	if got := channelLink(t, s, "echo", "alpha"); got != filepath.Join("..", "..", "..", "..", "echo", "v3.0.0", "en") {
		t.Fatalf("alpha -> %q once v3.0.0 is published", got)
	}
}

// Channel tags missing for a pass, as while the release repository is
// uploaded anew, leave the module its target and channels: the newest
// version, a pre-release here, does not become its only channel. Missing on
// the next pass too, the module is one without channels.
func TestPinnerKeepsChannelsMissingForAPass(t *testing.T) {
	// The lookup of every pass: the tags, then the tags gone right after a
	// lookup found them, then gone still (registrydocs.Channels).
	lookups := []func() (ondemand.Channels, error){
		func() (ondemand.Channels, error) {
			return ondemand.Channels{"stable": "v1.0.0", "alpha": "v2.0.0"}, nil
		},
		func() (ondemand.Channels, error) { return nil, ondemand.ErrChannelsGone },
		func() (ondemand.Channels, error) { return nil, ondemand.ErrNoChannel },
	}
	var pass atomic.Int32
	s, _ := newTestStore(t, &fakeRender{size: 10}, 1000)
	resolver := Resolver{
		Modules: func(context.Context) ([]registry.Module, error) { return []registry.Module{{Name: "echo"}}, nil },
		Policy: SitePolicy{
			Channels: func(context.Context, string) (ondemand.Channels, error) { return lookups[pass.Load()]() },
			Latest:   func(context.Context, string) (string, error) { return "v3.0.0-rc.1", nil },
		},
		Logger: log.NewNop(),
	}
	p := NewPinner(resolver, s, time.Hour, log.NewNop())
	p.sync(context.Background())

	pass.Add(1)
	p.sync(context.Background())
	for channel, version := range map[string]string{"stable": "v1.0.0", "alpha": "v2.0.0"} {
		if got, want := channelLink(t, s, "echo", channel), filepath.Join("..", "..", "..", "..", "echo", version, "en"); got != want {
			t.Fatalf("%s -> %q for the pass without the tags, want %q", channel, got, want)
		}
	}
	if target, ok := s.Target("echo"); !ok || target.Version != "v1.0.0" {
		t.Fatalf("target %v, %v for the pass without the tags, want v1.0.0", target, ok)
	}

	pass.Add(1)
	p.sync(context.Background())
	if got, want := channelLink(t, s, "echo", "stable"), filepath.Join("..", "..", "..", "..", "echo", "v3.0.0-rc.1", "en"); got != want {
		t.Fatalf("stable -> %q on the next pass, want %q", got, want)
	}
	if got := channelLink(t, s, "echo", "alpha"); got != "" {
		t.Fatalf("alpha -> %q on the next pass, want no link", got)
	}
}

// A module gone from the registries loses its channel links, which would
// point at a version that is evicted.
func TestSetChannelsRemovesGoneModules(t *testing.T) {
	f := &fakeRender{size: 10}
	s, _ := newTestStore(t, f, 1000)
	renderDone(t, s, "echo", "v1.0.0")
	renderDone(t, s, "alb", "v1.0.0")

	s.SetChannels(map[string]ondemand.Channels{"echo": {"stable": "v1.0.0"}, "alb": {"stable": "v1.0.0"}})
	s.SetChannels(map[string]ondemand.Channels{"alb": {"stable": "v1.0.0"}})

	if got := channelLink(t, s, "echo", "stable"); got != "" {
		t.Errorf("echo/stable -> %q, want no link for a gone module", got)
	}
	if got := channelLink(t, s, "alb", "stable"); got == "" {
		t.Error("alb/stable lost its link")
	}
	if s.channels.Stable("echo") != "" {
		t.Error("the gone module is still at its root for the search")
	}
}

// The modules and the versions of the channels may come from targets.json
// in S3 and become paths: what is not named as one is not touched.
func TestLinkChannelsSkipsInvalidNames(t *testing.T) {
	f := &fakeRender{size: 10}
	s, _ := newTestStore(t, f, 1000)
	renderDone(t, s, "echo", "v1.0.0")
	s.SetChannels(map[string]ondemand.Channels{"echo": {"stable": "v1.0.0"}})

	// The module ".." is the directory above the modules: a file named as
	// a channel there is not a channel link to remove.
	outside := filepath.Join(s.cfg.Dir, siteDir, "en", "stable")
	if err := os.WriteFile(outside, nil, 0600); err != nil {
		t.Fatal(err)
	}
	s.SetChannels(map[string]ondemand.Channels{"echo": {"stable": "v1.0.0"}, "..": {"alpha": "v1.0.0"}})
	if !exists(outside) {
		t.Fatal("a file outside the module is removed as a channel link")
	}
	if got := channelLink(t, s, "echo", "stable"); got == "" {
		t.Fatal("echo/stable lost its link")
	}

	// A version that is not one makes no link.
	s.LinkChannels("echo", ondemand.Channels{"stable": "../../../../outside", "beta": "v1.0.0"})
	if got := channelLink(t, s, "echo", "stable"); got != "" {
		t.Fatalf("echo/stable -> %q, want no link", got)
	}
	if got := channelLink(t, s, "echo", "beta"); got == "" {
		t.Fatal("echo/beta is not linked")
	}
}
