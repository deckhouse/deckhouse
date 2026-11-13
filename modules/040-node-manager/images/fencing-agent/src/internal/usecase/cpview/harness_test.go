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

package cpview

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/adapters/proxyclient"
	"fencing-agent/internal/domain"
)

// TestMain makes the boot clock follow time.Now, so it is fake inside
// synctest and the same as the clock of the fake proxy.
func TestMain(m *testing.M) {
	bootMillis = func() uint64 { return uint64(time.Now().UnixMilli()) }

	os.Exit(m.Run())
}

// Timings of the profiles: controlPlaneLostAfter and fastReadAfter.
var profiles = []struct {
	name                                 string
	controlPlaneLostAfter, fastReadAfter time.Duration
}{
	{"critical", 4150 * time.Millisecond, 1200 * time.Millisecond},
	{"normal", 9100 * time.Millisecond, 1200 * time.Millisecond},
	{"slow", 20200 * time.Millisecond, 1200 * time.Millisecond},
}

const (
	critControlPlaneLostAfter = 4150 * time.Millisecond
	critFastReadAfter         = 1200 * time.Millisecond
)

type proxyMode int

const (
	proxyNew proxyMode = iota
	proxyOld
	proxyDown
	// proxyHung accepts the connection and does not answer.
	proxyHung
)

type call struct {
	addr       string
	start, end time.Time
}

// fakeNode is a fake local proxy and a fake control plane.
type fakeNode struct {
	mu    sync.Mutex
	proxy proxyMode
	cpUp  bool
	// hang makes a probe of a lost control plane wait for its timeout (dropped
	// packets); else the probe fails at once (refused connection).
	hang bool
	// latency is the time of a ready answer.
	latency time.Duration
	// lastReady is the last ready probe of the fake proxy.
	lastReady time.Time
	// restarted makes the next read the first cycle after a proxy restart:
	// probeOK=false and an empty list.
	restarted bool
	// upstreams replaces the fake proxy when set.
	upstreams func(now time.Time) proxyclient.Upstreams

	apis   []string
	own    []string
	probes []call
	// rounds are the times of the APIServers calls: one at the start of each own round.
	rounds []time.Time
}

func (f *fakeNode) set(change func(f *fakeNode)) {
	f.mu.Lock()
	defer f.mu.Unlock()

	change(f)
}

func (f *fakeNode) Upstreams(ctx context.Context) (proxyclient.Upstreams, error) {
	f.mu.Lock()
	hung := f.proxy == proxyHung
	f.mu.Unlock()

	if hung {
		<-ctx.Done()

		return proxyclient.Upstreams{}, ctx.Err()
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	now := time.Now()

	switch {
	case f.proxy == proxyDown:
		return proxyclient.Upstreams{}, errors.New("connection refused")
	case f.proxy == proxyOld:
		return proxyclient.Upstreams{}, nil
	case f.upstreams != nil:
		return f.upstreams(now), nil
	}

	// The proxy numbers a cycle by the boot clock at its start. Here the
	// cycle starts and ends at the read.
	u := proxyclient.Upstreams{HasProbe: true, ProbeSeq: uint64(now.UnixMilli())}

	if f.restarted {
		f.restarted, f.lastReady = false, time.Time{}

		return u, nil
	}

	if f.cpUp {
		f.lastReady = now
	}

	u.ProbeOK = f.cpUp
	if !f.lastReady.IsZero() {
		u.LastReadyAgo = []time.Duration{now.Sub(f.lastReady)}
	}

	return u, nil
}

func (f *fakeNode) Ready(ctx context.Context, addr string) bool {
	f.mu.Lock()
	c := call{addr: addr, start: time.Now()}
	up, hang, latency := f.cpUp, f.hang, f.latency
	f.mu.Unlock()

	switch {
	case !up && hang:
		<-ctx.Done()
	case up && latency > 0:
		up = sleep(ctx, latency)
	}

	f.mu.Lock()
	c.end = time.Now()
	f.probes = append(f.probes, c)
	f.mu.Unlock()

	return up
}

func (f *fakeNode) APIServers() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.rounds = append(f.rounds, time.Now())

	return append([]string(nil), f.apis...)
}

func (f *fakeNode) ownIPs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.own
}

func (f *fakeNode) calls() ([]call, []time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]call(nil), f.probes...), append([]time.Time(nil), f.rounds...)
}

type fakeStatus struct {
	mu      sync.Mutex
	sources []domain.CPSource
}

func (s *fakeStatus) SetCondition(domain.Reason, string) {}
func (s *fakeStatus) SetProfile(string)                  {}

func (s *fakeStatus) SetCPSource(src domain.CPSource) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sources = append(s.sources, src)
}

func (s *fakeStatus) list() []domain.CPSource {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]domain.CPSource(nil), s.sources...)
}

// harness runs a View and keeps every state it gives.
type harness struct {
	f      *fakeNode
	status *fakeStatus
	view   *View

	mu     sync.Mutex
	states []domain.CPState
	// onState is called for each state in the reader goroutine.
	onState func(domain.CPState)

	stop func()
}

// start runs a View. Call it inside synctest.Test and call stop at the end.
func start(f *fakeNode, controlPlaneLostAfter, fastReadAfter time.Duration) *harness {
	h := &harness{f: f, status: &fakeStatus{}}
	h.view = New(f, h.status, f.ownIPs, controlPlaneLostAfter, fastReadAfter, log.NewNop())

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup

	wg.Go(func() { _ = h.view.Run(ctx) })
	wg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case st := <-h.view.Updates():
				h.mu.Lock()
				h.states = append(h.states, st)
				on := h.onState
				h.mu.Unlock()

				if on != nil {
					on(st)
				}
			}
		}
	})

	h.stop = func() {
		cancel()
		wg.Wait()
	}

	return h
}

func (h *harness) all() []domain.CPState {
	h.mu.Lock()
	defer h.mu.Unlock()

	return append([]domain.CPState(nil), h.states...)
}

func (h *harness) last() domain.CPState {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.states[len(h.states)-1]
}

func (h *harness) setOnState(on func(domain.CPState)) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.onState = on
}

// assertAlwaysVisible fails when a state after the first visible one is not visible.
func assertAlwaysVisible(t *testing.T, states []domain.CPState) {
	t.Helper()

	seen := false

	for _, st := range states {
		if st.Visible {
			seen = true

			continue
		}

		if seen {
			t.Fatalf("control plane lost at %v: %+v", st.ReadAt, st)
		}
	}

	if !seen {
		t.Fatal("control plane never visible")
	}
}

// assertSeqGrows fails when ProbeSeq goes down.
func assertSeqGrows(t *testing.T, states []domain.CPState) {
	t.Helper()

	for i := 1; i < len(states); i++ {
		if states[i].ProbeSeq < states[i-1].ProbeSeq {
			t.Fatalf("ProbeSeq went down from %d to %d at %v", states[i-1].ProbeSeq, states[i].ProbeSeq, states[i].ReadAt)
		}
	}
}

func between(d, lo, hi time.Duration) bool {
	return d >= lo && d <= hi
}

func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
