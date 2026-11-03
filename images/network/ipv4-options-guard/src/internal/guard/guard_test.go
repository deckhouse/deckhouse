/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package guard

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// fakeBackend counts Remove calls per netns; a pointer, so Backend values compare by identity.
type fakeBackend struct {
	name     string
	fail     map[string]bool
	removed  map[string]int
	counters map[string]uint64
	applied  Applied
	onRemove func(netns string)
}

func (f *fakeBackend) Name() string                               { return f.name }
func (f *fakeBackend) Apply(*NetNS, State) (Applied, error)       { return f.applied, nil }
func (f *fakeBackend) Read(*NetNS) (State, bool)                  { return State{}, false }
func (f *fakeBackend) Counters(*NetNS) (map[string]uint64, error) { return f.counters, nil }

func (f *fakeBackend) Remove(ns *NetNS) (State, bool, error) {
	f.removed[ns.Name]++
	if f.onRemove != nil {
		f.onRemove(ns.Name)
	}
	if f.fail[ns.Name] {
		return State{}, false, errors.New("remove failed")
	}
	return State{}, false, nil
}

// Disabled runs restore, which only calls the backends: no setns, so no privileges are needed.
func newStaleEnv(t *testing.T, netns ...string) (*Guard, *fakeBackend, string) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(settings, []byte(`{"mode":"Disabled"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	netnsDir := filepath.Join(dir, "netns")
	if err := os.Mkdir(netnsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range netns {
		if err := os.Symlink("/proc/self/ns/net", filepath.Join(netnsDir, n)); err != nil {
			t.Fatal(err)
		}
	}
	g := New(Config{SettingsFile: settings, NetNSDir: netnsDir, HostNetNSPath: "/proc/self/ns/net"},
		slog.New(slog.NewTextHandler(io.Discard, nil)), NewMetrics(prometheus.NewRegistry()))
	stale := &fakeBackend{name: "old", fail: map[string]bool{}, removed: map[string]int{}}
	g.backend = &fakeBackend{name: "new", fail: map[string]bool{}, removed: map[string]int{}}
	g.stale, g.staleDone = stale, map[string]bool{}
	return g, stale, netnsDir
}

func TestStaleCleanedPerNetNS(t *testing.T) {
	g, stale, netnsDir := newStaleEnv(t, "a", "b", "bad")
	stale.fail["bad"] = true

	g.resync()
	if g.stale == nil {
		t.Fatal("stale dropped although a netns failed")
	}
	// Only the failing netns is looked at again.
	g.resync()
	for name, want := range map[string]int{hostNetNS: 1, "a": 1, "b": 1, "bad": 2} {
		if stale.removed[name] != want {
			t.Errorf("%s: %d removals, want %d", name, stale.removed[name], want)
		}
	}

	// A new netns is cleaned too; a removed one is forgotten.
	if err := os.Symlink("/proc/self/ns/net", filepath.Join(netnsDir, "c")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(netnsDir, "b")); err != nil {
		t.Fatal(err)
	}
	g.resync()
	if stale.removed["c"] != 1 || g.staleDone["b"] {
		t.Errorf("new netns removals %d, removed netns still tracked %v", stale.removed["c"], g.staleDone["b"])
	}

	// Once nothing fails, the stale backend is dropped and not called any more.
	stale.fail["bad"] = false
	g.resync()
	if g.stale != nil {
		t.Fatal("stale kept after a resync without failures")
	}
	before := stale.removed["a"]
	g.resync()
	if stale.removed["a"] != before {
		t.Fatal("stale called after it was dropped")
	}
}

func TestStaleKeptWhenNetNSDirUnreadable(t *testing.T) {
	g, _, netnsDir := newStaleEnv(t)
	if err := os.Remove(netnsDir); err != nil {
		t.Fatal(err)
	}
	// The pod netns were not listed, so they may still hold the stale filter.
	g.resync()
	if g.stale == nil {
		t.Fatal("stale dropped without listing the pod netns")
	}
}

func TestStaleResetOnBackendSwitch(t *testing.T) {
	g, stale, _ := newStaleEnv(t, "a")
	g.resync() // a is cleaned, nothing failed: stale dropped
	if g.stale != nil {
		t.Fatal("stale kept after a clean resync")
	}
	// Switching back makes the previous backend stale again: every netns is cleaned anew.
	g.stale, g.staleDone = stale, map[string]bool{}
	g.resync()
	if stale.removed["a"] != 2 {
		t.Fatalf("a: %d removals, want 2", stale.removed["a"])
	}
}

func TestNetNSCreatedDuringResyncKept(t *testing.T) {
	g, stale, netnsDir := newStaleEnv(t, "a")
	// After the listing, while the resync works on a, a pod netns appears and reconcileNew takes its baseline.
	stale.onRemove = func(netns string) {
		if netns != "a" {
			return
		}
		if err := os.Symlink("/proc/self/ns/net", filepath.Join(netnsDir, "late")); err != nil {
			t.Fatal(err)
		}
		g.mu.Lock()
		g.counters["late"] = map[string]uint64{ruleNames[0]: 0}
		g.mu.Unlock()
	}
	g.counters["removed"] = map[string]uint64{ruleNames[0]: 0}
	g.resync()
	if g.counters["late"] == nil {
		t.Fatal("the baseline of a netns created during the resync is dropped")
	}
	if g.counters["removed"] != nil {
		t.Fatal("the counters of a removed netns are kept")
	}
}

func TestBypassedReported(t *testing.T) {
	g, _, _ := newStaleEnv(t)
	g.stale = nil
	// Audit on the host netns only: Apply runs, MPTCP of pods is not touched, so no setns is needed.
	if err := os.WriteFile(g.cfg.SettingsFile, []byte(`{"mode":"Audit"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	b := g.backend.(*fakeBackend)
	bypassed := func() float64 {
		var m dto.Metric
		if err := g.m.bypassed.Write(&m); err != nil {
			t.Fatal(err)
		}
		return m.GetGauge().GetValue()
	}

	b.applied = Applied{Bypassed: true}
	g.resync()
	if bypassed() != 1 {
		t.Fatalf("bypassed netns: %v, want 1", bypassed())
	}
	b.applied = Applied{}
	g.resync()
	if bypassed() != 0 {
		t.Fatalf("bypassed netns after the rule is gone: %v, want 0", bypassed())
	}
}

func TestCollectStartsFromBaseline(t *testing.T) {
	g := New(Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)), NewMetrics(prometheus.NewRegistry()))
	b := &fakeBackend{counters: map[string]uint64{ruleNames[0]: 100}}
	ns := &NetNS{Name: "a"}
	matched := func() float64 {
		var m dto.Metric
		if err := g.m.matched.WithLabelValues(ruleNames[0]).Write(&m); err != nil {
			t.Fatal(err)
		}
		return m.GetCounter().GetValue()
	}

	// Packets counted before the guard (re)started are not reported again.
	g.collect(ns, b)
	if matched() != 0 {
		t.Fatalf("baseline reported: %v", matched())
	}
	b.counters[ruleNames[0]] = 105
	g.collect(ns, b)
	if matched() != 5 {
		t.Fatalf("got %v, want 5", matched())
	}
}
