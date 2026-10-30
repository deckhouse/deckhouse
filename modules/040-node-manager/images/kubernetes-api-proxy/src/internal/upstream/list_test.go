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

package upstream

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type fakeProbes struct {
	mu      sync.Mutex
	results map[string]probeResult
}

func newFakeProbes() *fakeProbes {
	return &fakeProbes{results: map[string]probeResult{}}
}

func (f *fakeProbes) set(addr string, kind probeKind, latency time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.results[addr] = probeResult{kind: kind, latency: latency}
}

func (f *fakeProbes) probe(_ context.Context, u *Upstream) (probeResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	r, found := f.results[u.Addr]
	if !found {
		return probeResult{}, fmt.Errorf("no fake result for %s", u.Addr)
	}

	return r, nil
}

func withProbes(p *fakeProbes) ListOption {
	return func(l *ListConfig) error {
		l.probe = p.probe

		return nil
	}
}

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.now = c.now.Add(d)
}

func ups(addrs ...string) []*Upstream {
	out := make([]*Upstream, 0, len(addrs))
	for _, addr := range addrs {
		out = append(out, NewUpstream(addr, nil))
	}

	return out
}

func newTestList(t *testing.T, addrs ...string) (*List, *fakeProbes, *fakeClock) {
	t.Helper()

	probes := newFakeProbes()
	for _, addr := range addrs {
		probes.set(addr, probeReady, time.Millisecond)
	}

	clock := &fakeClock{now: time.Unix(0, 0)}

	list, err := newList(nil, byAddr, WithHealthcheckInterval(time.Hour), WithDownHold(5*time.Second), withProbes(probes))
	if err != nil {
		t.Fatal(err)
	}

	list.now = clock.Now
	list.Reconcile(ups(addrs...))

	return list, probes, clock
}

func round(list *List) {
	list.round(context.Background(), list.beginRound())
}

func picks(t *testing.T, list *List, n int) map[string]int {
	t.Helper()

	got := map[string]int{}
	for range n {
		u, err := list.Pick()
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}

		got[u.Addr]++
	}

	return got
}

func exported(t *testing.T, list *List, addr string) ExportNode {
	t.Helper()

	nodes, _ := list.ExportNodes()
	for _, n := range nodes {
		if n.Upstream == addr {
			return n
		}
	}

	t.Fatalf("upstream %s not in list", addr)

	return ExportNode{}
}

func TestList_NewList(t *testing.T) {
	tests := []struct {
		name    string
		options []ListOption
		wantErr bool
	}{
		{name: "valid options", options: []ListOption{WithHealthcheckInterval(2 * time.Second)}},
		{name: "invalid jitter", options: []ListOption{WithHealthCheckJitter(1.5)}, wantErr: true},
		{name: "invalid jitter range", options: []ListOption{WithHealthCheckJitter(-0.5)}, wantErr: true},
		{name: "zero timeout", options: []ListOption{WithHealthcheckTimeout(0)}, wantErr: true},
		{name: "negative timeout", options: []ListOption{WithHealthcheckTimeout(-time.Second)}, wantErr: true},
		{name: "zero interval", options: []ListOption{WithHealthcheckInterval(0)}, wantErr: true},
		{name: "valid fail-step", options: []ListOption{WithFailStep(200*time.Millisecond, 4300*time.Millisecond)}},
		{name: "fail-step off", options: []ListOption{WithFailStep(200*time.Millisecond, 0)}},
		{name: "zero fail-step interval", options: []ListOption{WithFailStep(0, time.Second)}, wantErr: true},
		{name: "negative fail-step window", options: []ListOption{WithFailStep(200*time.Millisecond, -time.Second)}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list, err := NewList([]*Upstream{}, tt.options...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewList() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil {
				list.Shutdown()
			}
		})
	}
}

func TestPick_NoUpstreams(t *testing.T) {
	list, err := NewList([]*Upstream{})
	if err != nil {
		t.Fatalf("NewList() error = %v", err)
	}
	defer list.Shutdown()

	if _, err := list.Pick(); !errors.Is(err, ErrNoUpstreams) {
		t.Errorf("Pick() error = %v, want %v", err, ErrNoUpstreams)
	}
}

func TestPickNot_SkipsExcluded(t *testing.T) {
	list, _, _ := newTestList(t, "a", "b", "c")
	round(list)

	for range 6 {
		u, err := list.PickNot(NewUpstream("a", nil))
		if err != nil {
			t.Fatalf("PickNot() error = %v", err)
		}

		if u.Addr == "a" {
			t.Fatal("PickNot() returned the excluded upstream")
		}
	}
}

func TestPickNot_SingleUpstream(t *testing.T) {
	list, _, _ := newTestList(t, "a")
	round(list)

	if _, err := list.PickNot(NewUpstream("a", nil)); !errors.Is(err, ErrNoUpstreams) {
		t.Errorf("PickNot() error = %v, want %v", err, ErrNoUpstreams)
	}
}

func TestPick_Order(t *testing.T) {
	tests := []struct {
		name  string
		nodes []node
		want  []string
	}{
		{
			name: "round-robin inside a tier, score ignored while non-negative",
			nodes: []node{
				{backend: NewUpstream("a", nil), score: 1, tier: 2},
				{backend: NewUpstream("b", nil), score: 3, tier: 2},
			},
			want: []string{"a", "b", "a", "b"},
		},
		{
			name: "lower tier wins",
			nodes: []node{
				{backend: NewUpstream("a", nil), score: 3, tier: 3},
				{backend: NewUpstream("b", nil), score: 0, tier: 2},
			},
			want: []string{"b", "b", "b", "b"},
		},
		{
			name: "missing node loses to any present node",
			nodes: []node{
				{backend: NewUpstream("a", nil), score: 3, tier: 0, missing: 1},
				{backend: NewUpstream("b", nil), score: 3, tier: 5},
			},
			want: []string{"b", "b", "b", "b"},
		},
		{
			name: "alive node beats failed one regardless of score",
			nodes: []node{
				{backend: NewUpstream("a", nil), score: 3, tier: tierFailed},
				{backend: NewUpstream("b", nil), score: -3, tier: 4},
			},
			want: []string{"b", "b", "b", "b"},
		},
		{
			name: "among alive nodes non-negative score wins",
			nodes: []node{
				{backend: NewUpstream("a", nil), score: -1, tier: 0},
				{backend: NewUpstream("b", nil), score: 0, tier: 5},
			},
			want: []string{"b", "b", "b", "b"},
		},
		{
			name: "alive negatives share by tier, not by score",
			nodes: []node{
				{backend: NewUpstream("a", nil), score: -1, tier: 0},
				{backend: NewUpstream("b", nil), score: -4, tier: 0},
				{backend: NewUpstream("c", nil), score: -1, tier: 3},
			},
			want: []string{"a", "b", "a", "b"},
		},
		{
			name: "all failed and negative: highest score",
			nodes: []node{
				{backend: NewUpstream("a", nil), score: -3, tier: tierFailed},
				{backend: NewUpstream("b", nil), score: -1, tier: tierFailed},
			},
			want: []string{"b", "b", "b", "b"},
		},
		{
			name: "all failed and equal: round-robin",
			nodes: []node{
				{backend: NewUpstream("a", nil), score: -5, tier: tierFailed},
				{backend: NewUpstream("b", nil), score: -5, tier: tierFailed},
			},
			want: []string{"a", "b", "a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := &List{cmp: byAddr, current: -1, nodes: tt.nodes}

			for i, want := range tt.want {
				u, err := list.Pick()
				if err != nil {
					t.Fatalf("Pick() #%d error = %v", i, err)
				}

				if u.Addr != want {
					t.Errorf("Pick() #%d = %s, want %s", i, u.Addr, want)
				}
			}
		})
	}
}

func TestPick_FailedRank(t *testing.T) {
	tests := []struct {
		name  string
		nodes []node
		want  string
	}{
		{
			name: "silent beats penalized",
			nodes: []node{
				{backend: NewUpstream("a", nil), score: 1, tier: tierFailed, penalized: true},
				{backend: NewUpstream("b", nil), score: 1, tier: tierFailed},
			},
			want: "b",
		},
		{
			name: "penalized beats warm",
			nodes: []node{
				{backend: NewUpstream("a", nil), score: 1, tier: tierFailed, penalized: true, warm: true},
				{backend: NewUpstream("b", nil), score: 1, tier: tierFailed, penalized: true},
			},
			want: "b",
		},
		{
			name: "warm beats downed",
			nodes: []node{
				{backend: NewUpstream("a", nil), score: 1, tier: tierFailed, downed: true},
				{backend: NewUpstream("b", nil), score: 1, tier: tierFailed, warm: true},
			},
			want: "b",
		},
		{
			name: "warm beats unreachable",
			nodes: []node{
				{backend: NewUpstream("a", nil), score: 1, tier: tierFailed, lastKind: probeHard},
				{backend: NewUpstream("b", nil), score: 1, tier: tierFailed, warm: true},
			},
			want: "b",
		},
		{
			name: "unreachable beats missing",
			nodes: []node{
				{backend: NewUpstream("a", nil), score: 1, tier: 0, missing: 1},
				{backend: NewUpstream("b", nil), score: 1, tier: tierFailed, lastKind: probeHard},
			},
			want: "b",
		},
		{
			name: "rank before score sign",
			nodes: []node{
				{backend: NewUpstream("a", nil), score: 3, tier: tierFailed, penalized: true},
				{backend: NewUpstream("b", nil), score: -3, tier: tierFailed},
			},
			want: "b",
		},
		{
			name: "equal rank: non-negative score first",
			nodes: []node{
				{backend: NewUpstream("a", nil), score: -1, tier: tierFailed, penalized: true},
				{backend: NewUpstream("b", nil), score: 1, tier: tierFailed, penalized: true},
			},
			want: "b",
		},
		{
			name: "equal rank and negative: higher score",
			nodes: []node{
				{backend: NewUpstream("a", nil), score: -3, tier: tierFailed, downed: true},
				{backend: NewUpstream("b", nil), score: -1, tier: tierFailed, lastKind: probeHard},
			},
			want: "b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := &List{cmp: byAddr, current: -1, nodes: tt.nodes}

			if got := picks(t, list, 4); got[tt.want] != 4 {
				t.Errorf("picks = %v, want %s:4", got, tt.want)
			}
		})
	}
}

func TestRound_ShareRule(t *testing.T) {
	tests := []struct {
		name  string
		kinds map[string]probeKind
		want  []string
	}{
		{
			name:  "one not-ready against two silent: silent ones are picked",
			kinds: map[string]probeKind{"a": probeNotReady, "b": probeSoft, "c": probeSoft},
			want:  []string{"b", "c"},
		},
		{
			name:  "two not-ready against one silent: nobody ready, not-ready ones are picked",
			kinds: map[string]probeKind{"a": probeNotReady, "b": probeNotReady, "c": probeSoft},
			want:  []string{"a", "b"},
		},
		{
			name:  "tie of one against one: silent one is picked",
			kinds: map[string]probeKind{"a": probeNotReady, "b": probeSoft},
			want:  []string{"b"},
		},
		{
			name:  "tie of two against two: silent ones are picked",
			kinds: map[string]probeKind{"a": probeNotReady, "b": probeNotReady, "c": probeSoft, "d": probeSoft},
			want:  []string{"c", "d"},
		},
		{
			name:  "all not-ready: round-robin over all",
			kinds: map[string]probeKind{"a": probeNotReady, "b": probeNotReady, "c": probeNotReady},
			want:  []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list, probes, _ := newTestList(t, slices.Sorted(maps.Keys(tt.kinds))...)
			round(list)

			for addr, kind := range tt.kinds {
				probes.set(addr, kind, time.Millisecond)
			}

			round(list)

			want := map[string]int{}
			for _, addr := range tt.want {
				want[addr] = 2
			}

			if got := picks(t, list, 2*len(tt.want)); !maps.Equal(got, want) {
				t.Errorf("picks = %v, want %v", got, want)
			}
		})
	}
}

func TestRound_LogsModeSwitch(t *testing.T) {
	var buf bytes.Buffer

	list, probes, _ := newTestList(t, "a", "b")
	list.logger = log.NewLogger(log.WithOutput(&buf), log.WithLevel(slog.LevelInfo))
	round(list)

	probes.set("a", probeNotReady, time.Millisecond)
	probes.set("b", probeNotReady, time.Millisecond)
	round(list)
	round(list)

	if got := strings.Count(buf.String(), "no upstream ready"); got != 1 {
		t.Errorf("nobody-ready engaged logged %d times, want 1:\n%s", got, buf.String())
	}

	probes.set("a", probeReady, time.Millisecond)
	round(list)
	round(list)

	if got := strings.Count(buf.String(), "lose priority again"); got != 1 {
		t.Errorf("nobody-ready disengaged logged %d times, want 1:\n%s", got, buf.String())
	}
}

func TestRound_WarmNodeDoesNotVote(t *testing.T) {
	list, probes, _ := newTestList(t, "a", "b", "c")
	round(list)

	probes.set("a", probeHard, 0)
	round(list)

	probes.set("a", probeNotReady, time.Millisecond)
	probes.set("b", probeNotReady, time.Millisecond)
	probes.set("c", probeSoft, 0)
	round(list)

	if n := exported(t, list, "a"); !n.Warm || n.LastProbe != "not-ready" {
		t.Fatalf("a = %+v, want warm after unreachable then not-ready", n)
	}

	if got := picks(t, list, 4); got["c"] != 4 {
		t.Errorf("picks = %v, want c:4 (a does not vote, b against c is a tie)", got)
	}
}

func TestRound_WarmNodeNotRestoredWhileNobodyReady(t *testing.T) {
	list, probes, _ := newTestList(t, "a", "b", "c")
	round(list)

	probes.set("a", probeHard, 0)
	round(list)

	for _, addr := range []string{"a", "b", "c"} {
		probes.set(addr, probeNotReady, time.Millisecond)
	}

	round(list)

	if got := picks(t, list, 4); got["a"] != 0 || got["b"] != 2 || got["c"] != 2 {
		t.Errorf("picks = %v, want b:2 c:2", got)
	}

	if n := exported(t, list, "a"); n.Tier != tierFailed || !n.Warm {
		t.Errorf("a = %+v, want warm and failed", n)
	}

	probes.set("a", probeReady, time.Millisecond)
	round(list)

	if n := exported(t, list, "a"); n.Warm || n.Tier != 0 {
		t.Errorf("a after ready = %+v, want warm cleared and tier 0", n)
	}

	if got := picks(t, list, 4); got["a"] != 4 {
		t.Errorf("picks = %v, want a:4", got)
	}
}

func TestRound_MissingNodeDoesNotVote(t *testing.T) {
	list, probes, _ := newTestList(t, "a", "b", "c")
	round(list)

	list.Reconcile(ups("b", "c"))
	probes.set("a", probeNotReady, time.Millisecond)
	probes.set("b", probeNotReady, time.Millisecond)
	probes.set("c", probeSoft, 0)
	round(list)

	if got := picks(t, list, 4); got["c"] != 4 {
		t.Errorf("picks = %v, want c:4 (missing a does not vote, b against c is a tie)", got)
	}
}

func TestRound_OneSlowWhileNobodyReady(t *testing.T) {
	list, probes, _ := newTestList(t, "m0", "m1", "m2")
	round(list)

	probes.set("m0", probeNotReady, 2*time.Millisecond)
	probes.set("m1", probeSoft, 0)
	probes.set("m2", probeNotReady, 30*time.Millisecond)
	round(list)

	if got := picks(t, list, 6); got["m0"] != 6 {
		t.Errorf("picks = %v, want m0:6 (fastest answer, m1 timed out)", got)
	}

	for addr, want := range map[string]ExportNode{
		"m0": {Score: 2, Tier: 0},
		"m1": {Score: 2, Tier: tierFailed},
		"m2": {Score: 2, Tier: 2},
	} {
		if n := exported(t, list, addr); n.Score != want.Score || n.Tier != want.Tier {
			t.Errorf("%s = %+v, want score %v tier %v", addr, n, want.Score, want.Tier)
		}
	}
}

func TestRound_NobodyReadyRoundRobin(t *testing.T) {
	list, probes, _ := newTestList(t, "m0", "m1", "m2")
	for _, addr := range []string{"m0", "m1", "m2"} {
		probes.set(addr, probeNotReady, time.Millisecond)
	}

	for range 10 {
		round(list)
	}

	if got := picks(t, list, 6); got["m0"] != 2 || got["m1"] != 2 || got["m2"] != 2 {
		t.Errorf("picks = %v, want 2 each", got)
	}

	if n := exported(t, list, "m0"); n.Score != initialScore {
		t.Errorf("m0 score = %v, want %v (no penalty when nobody is ready)", n.Score, initialScore)
	}
}

func TestRound_NotReadyWhileOthersReady(t *testing.T) {
	list, probes, _ := newTestList(t, "m0", "m1", "m2")
	round(list)
	round(list)

	probes.set("m1", probeNotReady, time.Millisecond)
	round(list)

	if n := exported(t, list, "m1"); n.Tier != tierFailed || n.Score != 2 {
		t.Fatalf("m1 = %+v, want tier failed score 2", n)
	}

	if got := picks(t, list, 6); got["m1"] != 0 {
		t.Errorf("picks = %v, want no m1", got)
	}

	for range 3 {
		round(list)
	}

	if n := exported(t, list, "m1"); n.Score >= 0 {
		t.Errorf("m1 score after four not-ready rounds = %v, want negative", n.Score)
	}

	probes.set("m1", probeReady, time.Millisecond)
	round(list)

	if n := exported(t, list, "m1"); n.Tier != 0 {
		t.Errorf("m1 after recovery = %+v, want tier 0", n)
	}
}

func TestRound_AllSoftRoundRobin(t *testing.T) {
	list, probes, _ := newTestList(t, "m0", "m1", "m2")
	for _, addr := range []string{"m0", "m1", "m2"} {
		probes.set(addr, probeSoft, 0)
	}

	round(list)

	if got := picks(t, list, 6); got["m0"] != 2 || got["m1"] != 2 || got["m2"] != 2 {
		t.Errorf("picks = %v, want 2 each", got)
	}
}

func TestRound_AllHardPicksMaxScore(t *testing.T) {
	list, probes, _ := newTestList(t, "m0", "m1", "m2")
	for _, addr := range []string{"m0", "m1", "m2"} {
		probes.set(addr, probeHard, 0)
	}

	round(list)
	round(list)

	probes.set("m0", probeSoft, 0)
	probes.set("m2", probeSoft, 0)
	round(list)

	if n := exported(t, list, "m1"); n.Score != -2 {
		t.Fatalf("m1 score = %v, want -2", n.Score)
	}

	if got := picks(t, list, 6); got["m1"] != 0 || got["m0"] != 3 || got["m2"] != 3 {
		t.Errorf("picks = %v, want m0:3 m2:3", got)
	}
}

func TestRound_SlowTierNotPicked(t *testing.T) {
	list, probes, _ := newTestList(t, "m0", "m1", "m2")
	probes.set("m1", probeReady, 300*time.Millisecond)

	for range 3 {
		round(list)
	}

	if got := picks(t, list, 6); got["m1"] != 0 {
		t.Errorf("picks = %v, want no m1", got)
	}

	if n := exported(t, list, "m1"); n.Tier != 5 || n.Score != highestScore {
		t.Errorf("m1 = %+v, want tier 5 score %v", n, highestScore)
	}
}

func TestRound_ProbeErrorSkipsNode(t *testing.T) {
	list, _, _ := newTestList(t, "m0")
	list.Reconcile(ups("m0", "m1"))

	round(list)

	if n := exported(t, list, "m1"); n.Tier != tierFailed || n.Score != initialScore {
		t.Errorf("m1 = %+v, want untouched", n)
	}

	if n := exported(t, list, "m0"); n.Tier != 0 || n.Score != 2 {
		t.Errorf("m0 = %+v, want tier 0 score 2", n)
	}
}

func TestRound_CancelledContextChangesNothing(t *testing.T) {
	list, _, _ := newTestList(t, "m0")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	list.round(ctx, list.beginRound())

	if n := exported(t, list, "m0"); n.Tier != tierFailed || n.Score != initialScore {
		t.Errorf("m0 = %+v, want untouched", n)
	}
}

func TestReconcile_NewNodeWaitsForReady(t *testing.T) {
	list, probes, _ := newTestList(t, "m0")
	round(list)

	list.Reconcile(ups("m0", "m1"))

	if n := exported(t, list, "m1"); n.Tier != tierFailed || n.Score != initialScore {
		t.Errorf("new m1 = %+v, want tier failed score 1", n)
	}

	if got := picks(t, list, 4); got["m1"] != 0 {
		t.Errorf("picks before first ready = %v, want no m1", got)
	}

	probes.set("m1", probeReady, time.Millisecond)
	round(list)

	if got := picks(t, list, 4); got["m1"] != 2 {
		t.Errorf("picks after first ready = %v, want m1:2", got)
	}
}

func TestReconcile_FreshNodeNotReadyIsWarm(t *testing.T) {
	list, probes, _ := newTestList(t, "a", "b")
	round(list)

	list.Reconcile(ups("a", "b", "c"))
	probes.set("c", probeNotReady, time.Millisecond)
	round(list)

	if n := exported(t, list, "c"); !n.Warm {
		t.Errorf("c = %+v, want warm: joined a running list and answered not-ready", n)
	}
}

func TestRound_FirstNotReadyIsNotWarm(t *testing.T) {
	probes := newFakeProbes()
	probes.set("a", probeNotReady, time.Millisecond)

	fromNew, err := newList(ups("a"), byAddr, withProbes(probes))
	if err != nil {
		t.Fatal(err)
	}

	fromReconcile, err := newList(nil, byAddr, withProbes(probes))
	if err != nil {
		t.Fatal(err)
	}

	fromReconcile.Reconcile(ups("a"))

	for name, list := range map[string]*List{"newList": fromNew, "first Reconcile": fromReconcile} {
		t.Run(name, func(t *testing.T) {
			round(list)

			if n := exported(t, list, "a"); n.Warm {
				t.Errorf("a = %+v, want not warm: nothing is known at start", n)
			}
		})
	}
}

func TestExportNodes_ProbeState(t *testing.T) {
	list, probes, _ := newTestList(t, "a", "b")

	if n := exported(t, list, "a"); n.LastProbe != "" || n.Warm {
		t.Errorf("a before any probe = %+v, want empty lastProbe and not warm", n)
	}

	probes.set("a", probeHard, 0)
	round(list)

	if n := exported(t, list, "a"); n.LastProbe != "unreachable" || n.Warm {
		t.Errorf("a = %+v, want lastProbe unreachable and not warm", n)
	}

	probes.set("a", probeNotReady, time.Millisecond)
	round(list)

	if n := exported(t, list, "a"); n.LastProbe != "not-ready" || !n.Warm {
		t.Errorf("a = %+v, want lastProbe not-ready and warm", n)
	}
}

func TestReconcile_DedupesAddresses(t *testing.T) {
	list, _, _ := newTestList(t)
	list.Reconcile(ups("m0", "m0", "m1"))

	if got := list.ListFullAddresses(); len(got) != 2 {
		t.Errorf("addresses = %v, want 2 unique", got)
	}
}

func TestReconcile_MissingNode(t *testing.T) {
	list, _, _ := newTestList(t, "m0", "m1", "m2")
	round(list)

	all := ups("m0", "m1", "m2")
	without := ups("m0", "m2")

	list.Reconcile(without)

	if n := exported(t, list, "m1"); n.Missing != 1 || n.EffectiveTier != tierFailed || n.Tier != 0 || n.Score != 2 {
		t.Errorf("m1 after one miss = %+v, want missing 1, effective tier failed, tier 0, score 2", n)
	}

	if got := picks(t, list, 4); got["m1"] != 0 {
		t.Errorf("picks while missing = %v, want no m1", got)
	}

	list.Reconcile(all)

	if n := exported(t, list, "m1"); n.Missing != 0 || n.EffectiveTier != 0 {
		t.Errorf("m1 after return = %+v, want missing 0 effective tier 0", n)
	}

	if got := picks(t, list, 6); got["m1"] != 2 {
		t.Errorf("picks after return = %v, want m1:2", got)
	}

	list.Reconcile(without)
	list.Reconcile(without)

	if got := list.ListFullAddresses(); len(got) != 3 {
		t.Fatalf("addresses after two misses = %v, want 3", got)
	}

	list.Reconcile(without)

	if got := list.ListFullAddresses(); len(got) != 2 {
		t.Errorf("addresses after three misses = %v, want m0 m2", got)
	}
}

func TestDown_HoldsFailedTier(t *testing.T) {
	list, _, clock := newTestList(t, "m0", "m1")
	round(list)

	list.Down(NewUpstream("m1", nil))
	round(list)

	if n := exported(t, list, "m1"); n.Tier != tierFailed {
		t.Fatalf("m1 right after Down and a ready probe = %+v, want tier failed", n)
	}

	if got := picks(t, list, 4); got["m1"] != 0 {
		t.Errorf("picks during hold = %v, want no m1", got)
	}

	clock.Advance(5 * time.Second)
	round(list)

	if got := picks(t, list, 4); got["m1"] != 2 {
		t.Errorf("picks after hold = %v, want m1:2", got)
	}
}

func TestDown_Storm(t *testing.T) {
	list, _, clock := newTestList(t, "m0", "m1")
	round(list)

	m1 := NewUpstream("m1", nil)
	for range 1000 {
		list.Down(m1)
	}

	if n := exported(t, list, "m1"); n.Score != lowestScore || n.Tier != tierFailed {
		t.Errorf("m1 = %+v, want score %v tier failed", n, lowestScore)
	}

	if got := picks(t, list, 4); got["m0"] != 4 {
		t.Errorf("picks = %v, want m0:4", got)
	}

	clock.Advance(5 * time.Second)
	for range 4 {
		round(list)
	}

	if got := picks(t, list, 4); got["m0"] != 4 {
		t.Errorf("picks while m1 score is negative = %v, want m0:4", got)
	}

	round(list)

	if n := exported(t, list, "m1"); n.Score != 0 {
		t.Fatalf("m1 after five ready rounds = %+v, want score 0", n)
	}

	if got := picks(t, list, 4); got["m1"] != 2 {
		t.Errorf("picks after recovery = %v, want m1:2", got)
	}
}

func TestHealthy(t *testing.T) {
	list, probes, _ := newTestList(t, "m0", "m1")

	if list.Healthy() {
		t.Error("Healthy() before any ready probe = true, want false")
	}

	round(list)

	before := list.current
	if !list.Healthy() {
		t.Error("Healthy() after ready probes = false, want true")
	}

	if list.current != before {
		t.Error("Healthy() moved the round-robin cursor")
	}

	probes.set("m0", probeSoft, 0)
	probes.set("m1", probeSoft, 0)
	round(list)

	if list.Healthy() {
		t.Error("Healthy() with every upstream failed = true, want false")
	}
}

func TestProbeAll_ParallelWithTimeout(t *testing.T) {
	block := func(ctx context.Context, _ *Upstream) (probeResult, error) {
		<-ctx.Done()

		return probeResult{kind: probeSoft}, nil
	}

	list, err := newList(nil, byAddr, WithHealthcheckTimeout(100*time.Millisecond), withProbe(block))
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	out := list.probeAll(context.Background(), ups("a", "b", "c"))

	if elapsed := time.Since(start); elapsed >= 300*time.Millisecond {
		t.Errorf("three blocked probes with 100ms timeout took %v, want parallel and bounded", elapsed)
	}

	for i, o := range out {
		if !o.ok || o.value.kind != probeSoft {
			t.Errorf("out[%d] = %+v, want soft", i, o)
		}
	}
}

func TestSilent_HoldsLikeDown(t *testing.T) {
	list, _, clock := newTestList(t, "a", "b")
	round(list)

	a := NewUpstream("a", nil)
	before := exported(t, list, "a").Score

	list.Silent(a)
	list.Silent(a)

	if n := exported(t, list, "a"); n.Tier != tierFailed || n.Score != before-2 || !n.Held {
		t.Fatalf("after two Silent a = %+v, want failed tier, score %v, held", n, before-2)
	}

	clock.Advance(4 * time.Second)
	list.Silent(a)
	clock.Advance(4 * time.Second)

	if n := exported(t, list, "a"); !n.Held {
		t.Errorf("hold was not extended by the last Silent: %+v", n)
	}
}

func TestDown_LogsReason(t *testing.T) {
	var buf bytes.Buffer

	list, _, _ := newTestList(t, "a", "b")
	list.logger = log.NewLogger(log.WithOutput(&buf), log.WithLevel(slog.LevelInfo))
	round(list)

	list.Down(NewUpstream("a", nil))

	if !strings.Contains(buf.String(), "dial") {
		t.Errorf("Down did not log reason dial:\n%s", buf.String())
	}

	list.Silent(NewUpstream("b", nil))

	if !strings.Contains(buf.String(), "first-byte") {
		t.Errorf("Silent did not log reason first-byte:\n%s", buf.String())
	}
}

func TestExportNodes_HeldAndDowned(t *testing.T) {
	list, _, clock := newTestList(t, "a", "b")
	round(list)

	if n := exported(t, list, "a"); n.Held || n.Downed {
		t.Fatalf("a before Down = %+v, want not held and not downed", n)
	}

	list.Down(NewUpstream("a", nil))

	if n := exported(t, list, "a"); !n.Held || !n.Downed {
		t.Errorf("a after Down = %+v, want held and downed", n)
	}

	clock.Advance(6 * time.Second)

	if n := exported(t, list, "a"); n.Held {
		t.Errorf("a after the hold = %+v, want not held", n)
	}

	round(list)

	if n := exported(t, list, "a"); n.Downed {
		t.Errorf("a after a probe = %+v, want downed cleared", n)
	}
}

func TestRound_WarmingUpLoggedOnce(t *testing.T) {
	var buf bytes.Buffer

	list, probes, _ := newTestList(t, "a", "b")
	list.logger = log.NewLogger(log.WithOutput(&buf), log.WithLevel(slog.LevelInfo))
	round(list)

	for _, kind := range []probeKind{probeHard, probeNotReady, probeHard, probeNotReady} {
		probes.set("a", kind, time.Millisecond)
		round(list)
	}

	if got := strings.Count(buf.String(), "warming up"); got != 1 {
		t.Errorf("warming up logged %d times, want 1:\n%s", got, buf.String())
	}
}

func TestRound_EmptyElectorateKeepsMode(t *testing.T) {
	var buf bytes.Buffer

	list, probes, _ := newTestList(t, "a", "b", "c")
	list.logger = log.NewLogger(log.WithOutput(&buf), log.WithLevel(slog.LevelInfo))
	round(list)

	for _, addr := range []string{"a", "b", "c"} {
		probes.set(addr, probeNotReady, time.Millisecond)
	}

	round(list)

	list.Reconcile(ups("a", "b"))
	probes.set("a", probeHard, 0)
	probes.set("b", probeHard, 0)

	before := exported(t, list, "c").Score

	round(list)

	if strings.Contains(buf.String(), "lose priority again") {
		t.Errorf("mode switched off with no voters:\n%s", buf.String())
	}

	if got := exported(t, list, "c").Score; got != before {
		t.Errorf("missing not-ready c went from %v to %v in a round with no voters, want the kept mode applied", before, got)
	}

	probes.set("a", probeReady, time.Millisecond)
	round(list)

	if got := strings.Count(buf.String(), "lose priority again"); got != 1 {
		t.Errorf("mode switched off %d times after a ready answer, want 1:\n%s", got, buf.String())
	}
}

func TestReconcile_FreshKeptThroughSoft(t *testing.T) {
	list, probes, _ := newTestList(t, "a", "b")
	round(list)

	list.Reconcile(ups("a", "b", "c"))
	probes.set("c", probeSoft, 0)
	round(list)

	probes.set("c", probeNotReady, time.Millisecond)
	round(list)

	if n := exported(t, list, "c"); !n.Warm {
		t.Errorf("c = %+v, want warm: joined, timed out once, then not-ready", n)
	}
}

func TestReconcile_FreshClearedByReady(t *testing.T) {
	list, probes, _ := newTestList(t, "a", "b")
	round(list)

	list.Reconcile(ups("a", "b", "c"))
	probes.set("c", probeReady, time.Millisecond)
	round(list)

	for _, addr := range []string{"a", "b", "c"} {
		probes.set(addr, probeNotReady, time.Millisecond)
	}

	round(list)

	if n := exported(t, list, "c"); n.Warm {
		t.Errorf("c = %+v, want not warm: it was ready before the cluster-wide not-ready", n)
	}

	if got := picks(t, list, 6); got["c"] != 2 {
		t.Errorf("picks = %v, want c:2 (balanced with a and b)", got)
	}
}

func TestRound_WarmNotReadyCostsAPoint(t *testing.T) {
	list, probes, _ := newTestList(t, "a", "b", "c")
	round(list)

	before := exported(t, list, "a").Score

	probes.set("a", probeHard, 0)
	round(list)

	afterHard := exported(t, list, "a").Score
	if afterHard != before+failScoreDelta {
		t.Fatalf("score after hard = %v, want %v", afterHard, before+failScoreDelta)
	}

	for _, addr := range []string{"a", "b", "c"} {
		probes.set(addr, probeNotReady, time.Millisecond)
	}

	round(list)

	if n := exported(t, list, "a"); n.Score != afterHard+failScoreDelta || !n.Warm {
		t.Errorf("a = %+v, want warm and score %v", n, afterHard+failScoreDelta)
	}
}

func TestRound_HardProbesDoNotVote(t *testing.T) {
	list, probes, _ := newTestList(t, "a", "b", "c")
	round(list)

	probes.set("a", probeHard, 0)
	probes.set("b", probeHard, 0)
	probes.set("c", probeNotReady, time.Millisecond)
	round(list)

	if got := picks(t, list, 2); got["c"] != 2 {
		t.Errorf("picks = %v, want c:2", got)
	}

	if n := exported(t, list, "c"); n.Tier == tierFailed {
		t.Errorf("c = %+v, want live tier (nobody ready, c is the only voter)", n)
	}

	if !list.Healthy() {
		t.Error("Healthy() = false, want true (c restored by nobody-ready)")
	}
}

func TestRound_HeldNotReadyIsNotPenalizedWhileNobodyReady(t *testing.T) {
	list, probes, _ := newTestList(t, "a", "b", "c")
	round(list)

	probes.set("a", probeNotReady, time.Millisecond)
	probes.set("c", probeNotReady, time.Millisecond)
	probes.set("b", probeSoft, time.Millisecond)
	round(list)

	list.Down(NewUpstream("a", nil))
	list.Down(NewUpstream("c", nil))
	round(list)
	round(list)

	list.mu.RLock()

	for _, addr := range []string{"a", "c"} {
		i := list.indexLocked(NewUpstream(addr, nil))
		if list.nodes[i].penalized {
			t.Errorf("%s: penalized under nobody-ready, want rank 0 (got %d)", addr, list.failRankLocked(i))
		}
	}

	list.mu.RUnlock()

	if got := picks(t, list, 6); got["a"] == 0 || got["c"] == 0 {
		t.Errorf("picks = %v, want a and c among picks", got)
	}
}

func TestPickNot_AlternatesAndMovesCursor(t *testing.T) {
	list, _, _ := newTestList(t, "a", "b", "c")
	round(list)

	var got []string

	for range 4 {
		u, err := list.PickNot(NewUpstream("a", nil))
		if err != nil {
			t.Fatalf("PickNot() error = %v", err)
		}

		got = append(got, u.Addr)
	}

	for range 2 {
		u, err := list.Pick()
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}

		got = append(got, u.Addr)
	}

	if want := []string{"b", "c", "b", "c", "a", "b"}; !slices.Equal(got, want) {
		t.Errorf("sequence = %v, want %v", got, want)
	}
}

func TestRound_RestartSeenThroughSoft(t *testing.T) {
	list, probes, _ := newTestList(t, "a", "b", "c")
	round(list)

	for _, kind := range []probeKind{probeHard, probeSoft, probeNotReady} {
		probes.set("a", kind, time.Millisecond)
		round(list)
	}

	if n := exported(t, list, "a"); !n.Warm {
		t.Errorf("a = %+v, want warm: unreachable, then a probe timeout, then not-ready is a restart", n)
	}
}

func TestDown_UnknownUpstreamIsNoop(t *testing.T) {
	list, _, _ := newTestList(t, "a", "b")
	round(list)

	before, err := list.ExportNodes()
	if err != nil {
		t.Fatal(err)
	}

	list.Down(NewUpstream("zzz", nil))
	list.Silent(NewUpstream("zzz", nil))

	after, err := list.ExportNodes()
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(before, after) {
		t.Errorf("nodes changed by an unknown upstream: %+v -> %+v", before, after)
	}
}
