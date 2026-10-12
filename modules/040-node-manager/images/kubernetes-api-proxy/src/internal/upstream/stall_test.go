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
	"context"
	"slices"
	"strings"
	"testing"
	"time"
)

const testStallTimeout = 3 * time.Second

type stallRig struct {
	list    *List
	probes  *fakeProbes
	clock   *fakeClock
	starts  []time.Time
	evicted []string
}

func newStallRig(t *testing.T, stall time.Duration, addrs ...string) *stallRig {
	t.Helper()

	rig := &stallRig{probes: newFakeProbes(), clock: &fakeClock{now: time.Unix(0, 0)}}
	for _, addr := range addrs {
		rig.probes.set(addr, probeReady, time.Millisecond)
	}

	list, err := newList(nil, byAddr, WithHealthcheckInterval(time.Hour), WithDownHold(5*time.Second), withProbes(rig.probes), WithStallTimeout(stall))
	if err != nil {
		t.Fatal(err)
	}

	list.now = rig.clock.Now
	list.stallHook = func(_ *Upstream, start time.Time) { rig.starts = append(rig.starts, start) }
	list.OnStall(func(u *Upstream) { rig.evicted = append(rig.evicted, u.Addr) })
	list.Reconcile(ups(addrs...))
	rig.list = list

	return rig
}

func (rig *stallRig) rounds(n int) {
	for range n {
		round(rig.list)
		rig.clock.Advance(time.Second)
	}
}

func (rig *stallRig) warmUp() {
	rig.rounds(7)
}

func (rig *stallRig) hang(addr string) time.Time {
	rig.probes.set(addr, probeSoft, 0)
	round(rig.list)

	return rig.clock.Now()
}

func (rig *stallRig) timeout(addr string, start time.Time) {
	rig.list.stallDone(NewUpstream(addr, nil), start, probeResult{}, true)
}

func (rig *stallRig) node(t *testing.T, addr string) *node {
	t.Helper()

	i := rig.list.indexLocked(NewUpstream(addr, nil))
	if i < 0 {
		t.Fatalf("upstream %s not in list", addr)
	}

	return &rig.list.nodes[i]
}

func TestStall_TimeoutWithWitnessFires(t *testing.T) {
	rig := newStallRig(t, testStallTimeout, "w", "x")
	rig.warmUp()

	start := rig.hang("x")
	rig.clock.Advance(testStallTimeout)
	rig.timeout("x", start)

	if !slices.Equal(rig.evicted, []string{"x"}) {
		t.Errorf("evicted = %v, want [x]", rig.evicted)
	}

	if got := exported(t, rig.list, "x").Evictions; got != 1 {
		t.Errorf("evictions = %d, want 1", got)
	}
}

func TestStall_NoWitness(t *testing.T) {
	tests := []struct {
		name  string
		addrs []string
		setup func(t *testing.T, rig *stallRig)
	}{
		{name: "witness not ready", setup: func(_ *testing.T, rig *stallRig) {
			rig.probes.set("w", probeNotReady, time.Millisecond)
			round(rig.list)
		}},
		{name: "witness warm", setup: func(t *testing.T, rig *stallRig) { rig.node(t, "w").warm = true }},
		{name: "witness missing", setup: func(_ *testing.T, rig *stallRig) { rig.list.Reconcile(ups("x")) }},
		{name: "witness score below zero", setup: func(t *testing.T, rig *stallRig) { rig.node(t, "w").score = -1 }},
		{name: "witness held", setup: func(_ *testing.T, rig *stallRig) { rig.list.Silent(NewUpstream("w", nil)) }},
		{name: "witness ready for less than the window", setup: func(t *testing.T, rig *stallRig) {
			rig.node(t, "w").readySince = rig.clock.Now().Add(-2 * time.Second)
		}},
		{name: "witness has not answered since the stall probe started", setup: func(t *testing.T, rig *stallRig) {
			rig.node(t, "w").answeredAt = rig.clock.Now().Add(-testStallTimeout - time.Millisecond)
		}},
		{name: "nobody ready", setup: func(_ *testing.T, rig *stallRig) { rig.list.nobodyReady = true }},
		{name: "single upstream", addrs: []string{"x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addrs := tt.addrs
			if addrs == nil {
				addrs = []string{"w", "x"}
			}

			rig := newStallRig(t, testStallTimeout, addrs...)
			rig.warmUp()

			start := rig.hang("x")
			rig.clock.Advance(testStallTimeout)

			if tt.setup != nil {
				tt.setup(t, rig)
			}

			rig.timeout("x", start)

			if len(rig.evicted) > 0 {
				t.Errorf("evicted = %v, want none", rig.evicted)
			}
		})
	}
}

func TestStall_AllSilentDoesNotFire(t *testing.T) {
	addrs := []string{"a", "b", "c"}
	orders := [][]string{{"a", "b", "c"}, {"c", "b", "a"}, {"b", "a", "c"}}

	for _, order := range orders {
		t.Run(strings.Join(order, ""), func(t *testing.T) {
			rig := newStallRig(t, testStallTimeout, addrs...)
			rig.warmUp()

			for _, addr := range addrs {
				rig.probes.set(addr, probeSoft, 0)
			}

			round(rig.list)
			start := rig.clock.Now()
			rig.clock.Advance(testStallTimeout)

			for _, addr := range order {
				rig.timeout(addr, start)
			}

			if len(rig.evicted) > 0 {
				t.Errorf("evicted = %v, want none: an equally silent upstream is not a witness", rig.evicted)
			}
		})
	}
}

func TestStall_LateAnswerDoesNotFire(t *testing.T) {
	tests := []struct {
		name          string
		kind          probeKind
		keepsReadyRun bool
	}{
		{name: "late ready", kind: probeReady, keepsReadyRun: true},
		{name: "late not ready", kind: probeNotReady},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rig := newStallRig(t, testStallTimeout, "w", "x")
			rig.warmUp()

			readySince := rig.node(t, "x").readySince
			start := rig.hang("x")
			rig.clock.Advance(testStallTimeout - time.Millisecond)
			rig.list.stallDone(NewUpstream("x", nil), start, probeResult{kind: tt.kind}, false)

			x := rig.node(t, "x")
			if x.stalling || !x.answeredAt.Equal(rig.clock.Now()) {
				t.Errorf("stalling = %t, answeredAt = %s; want false, %s", x.stalling, x.answeredAt, rig.clock.Now())
			}

			if got := x.readySince.Equal(readySince) && !readySince.IsZero(); got != tt.keepsReadyRun {
				t.Errorf("readySince = %s (was %s), kept = %t, want %t", x.readySince, readySince, got, tt.keepsReadyRun)
			}

			if len(rig.evicted) > 0 {
				t.Errorf("evicted = %v, want none", rig.evicted)
			}
		})
	}
}

func TestStall_FastErrorDoesNotFire(t *testing.T) {
	rig := newStallRig(t, testStallTimeout, "w", "x")
	rig.warmUp()

	start := rig.hang("x")
	rig.probes.set("x", probeHard, 0)
	rig.list.stallProbe(context.Background(), NewUpstream("x", nil), start)

	if len(rig.evicted) > 0 {
		t.Errorf("evicted = %v, want none", rig.evicted)
	}

	if rig.node(t, "x").stalling {
		t.Error("stalling = true after a fast error, want false")
	}
}

func TestStall_AnsweredSinceStartDoesNotFire(t *testing.T) {
	rig := newStallRig(t, testStallTimeout, "w", "x")
	rig.warmUp()

	start := rig.hang("x")
	rig.clock.Advance(time.Second)
	rig.probes.set("x", probeNotReady, time.Millisecond)
	round(rig.list)
	rig.clock.Advance(testStallTimeout - time.Second)
	rig.timeout("x", start)

	if len(rig.evicted) > 0 {
		t.Errorf("evicted = %v, want none", rig.evicted)
	}
}

func TestStall_LateWitness(t *testing.T) {
	rig := newStallRig(t, testStallTimeout, "w", "x")
	rig.warmUp()

	rig.probes.set("w", probeSoft, 0)

	start := rig.hang("x")
	for range 8 {
		rig.clock.Advance(time.Second)
		round(rig.list)
	}

	rig.list.stallDone(NewUpstream("w", nil), start, probeResult{kind: probeReady, latency: 2 * time.Second}, false)
	rig.timeout("x", start)

	if !slices.Equal(rig.evicted, []string{"x"}) {
		t.Errorf("evicted = %v, want [x]: a late 200 from a stall probe must count as a witness", rig.evicted)
	}
}

func TestStall_SingleFlight(t *testing.T) {
	rig := newStallRig(t, testStallTimeout, "w", "x")
	rig.warmUp()

	rig.hang("x")
	rig.rounds(2)

	if len(rig.starts) != 1 {
		t.Errorf("stall probes started = %d, want 1", len(rig.starts))
	}
}

func TestStall_HookRunsUnlocked(t *testing.T) {
	rig := newStallRig(t, testStallTimeout, "w", "x")
	rig.warmUp()

	called := false

	rig.list.OnStall(func(*Upstream) {
		called = true

		if _, err := rig.list.Pick(); err != nil {
			t.Errorf("Pick() error = %v", err)
		}

		if _, err := rig.list.ExportNodes(); err != nil {
			t.Errorf("ExportNodes() error = %v", err)
		}
	})

	start := rig.hang("x")
	rig.clock.Advance(testStallTimeout)

	done := make(chan struct{})

	go func() {
		defer close(done)

		rig.timeout("x", start)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stallDone did not return: the hook ran under the list lock")
	}

	if !called {
		t.Error("hook was not called")
	}
}

func blockingProbe(hung string) func(context.Context, *Upstream) (probeResult, error) {
	return func(ctx context.Context, u *Upstream) (probeResult, error) {
		if u.Addr != hung {
			return probeResult{kind: probeReady, latency: time.Millisecond}, nil
		}

		<-ctx.Done()

		return probeResult{kind: probeSoft}, nil
	}
}

func withProbe(probe func(context.Context, *Upstream) (probeResult, error)) ListOption {
	return func(l *ListConfig) error {
		l.probe = probe

		return nil
	}
}

func startedStallList(t *testing.T, stall time.Duration) *List {
	t.Helper()

	list, err := NewList(ups("w", "x"),
		WithHealthcheckInterval(20*time.Millisecond),
		WithHealthcheckTimeout(10*time.Millisecond),
		WithStallTimeout(stall),
		withProbe(blockingProbe("x")),
	)
	if err != nil {
		t.Fatal(err)
	}

	return list
}

func TestStall_RealProbeTimeout(t *testing.T) {
	list := startedStallList(t, 50*time.Millisecond)
	t.Cleanup(list.Shutdown)

	fired := make(chan string, 1)

	list.OnStall(func(u *Upstream) {
		select {
		case fired <- u.Addr:
		default:
		}
	})

	select {
	case addr := <-fired:
		if addr != "x" {
			t.Errorf("hook called with %s, want x", addr)
		}
	case <-time.After(time.Second):
		t.Fatal("hook was not called for the hung upstream")
	}
}

func TestStall_ShutdownWaits(t *testing.T) {
	list := startedStallList(t, time.Hour)

	deadline := time.Now().Add(time.Second)
	for !exported(t, list, "x").Stalling {
		if time.Now().After(deadline) {
			t.Fatal("no stall probe started")
		}

		time.Sleep(5 * time.Millisecond)
	}

	done := make(chan struct{})

	go func() {
		defer close(done)

		list.Shutdown()
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Shutdown did not return with a stall probe in flight")
	}
}

func TestStall_Disabled(t *testing.T) {
	rig := newStallRig(t, 0, "w", "x")
	rig.warmUp()

	rig.hang("x")
	rig.rounds(5)

	if len(rig.starts) != 0 || len(rig.evicted) != 0 {
		t.Errorf("starts = %d, evicted = %v; want none with the stall timeout off", len(rig.starts), rig.evicted)
	}
}

func TestStall_NegativeTimeoutRejected(t *testing.T) {
	if _, err := newList(nil, byAddr, WithStallTimeout(-time.Second)); err == nil {
		t.Error("newList() error = nil, want an error for a negative stall timeout")
	}
}

func TestStall_RoundUnchanged(t *testing.T) {
	addrs := []string{"a", "b", "c"}
	off := newStallRig(t, 0, addrs...)
	on := newStallRig(t, testStallTimeout, addrs...)

	pending := map[string]time.Time{}

	on.list.stallHook = func(u *Upstream, start time.Time) { pending[u.Addr] = start }

	sequence := []results{
		every(ready(time.Millisecond), addrs...),
		{"a": soft, "b": ready(time.Millisecond), "c": hard},
		{"a": soft, "b": ready(20 * time.Millisecond), "c": notReady(time.Millisecond)},
		{"a": soft, "b": notReady(time.Millisecond), "c": notReady(time.Millisecond)},
		{"a": hard, "b": soft, "c": notReady(time.Millisecond)},
		{"a": notReady(time.Millisecond), "b": soft, "c": ready(time.Millisecond)},
		every(ready(time.Millisecond), addrs...),
	}

	for k, probes := range sequence {
		for range 4 {
			for _, rig := range []*stallRig{off, on} {
				for addr, r := range probes {
					rig.probes.set(addr, r.kind, r.latency)
				}

				rig.clock.Advance(time.Second)
				round(rig.list)
			}

			for addr, start := range pending {
				delete(pending, addr)
				on.timeout(addr, start)
			}

			assertSameModel(t, k, off.list, on.list)
		}
	}

	if len(on.evicted) == 0 {
		t.Error("no eviction happened: the sequence does not exercise the hook")
	}
}

func assertSameModel(t *testing.T, step int, off, on *List) {
	t.Helper()

	want, _ := off.ExportNodes()
	got, _ := on.ExportNodes()

	for i := range got {
		got[i].Stalling, got[i].Evictions = false, 0
	}

	if !slices.Equal(got, want) {
		t.Fatalf("step %d: model with stall probes = %+v, want %+v", step, got, want)
	}
}
