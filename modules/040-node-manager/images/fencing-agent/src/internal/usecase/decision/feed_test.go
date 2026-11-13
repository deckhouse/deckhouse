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

package decision

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/adapters/proxyclient"
	"fencing-agent/internal/domain"
	"fencing-agent/internal/usecase/cpview"
	"fencing-agent/internal/usecase/watchdog"
)

// slowProxy is a local proxy that accepts the connection to /upstreams and
// answers after delay, or never when delay is 0. The apiserver is ready.
type slowProxy struct{ delay time.Duration }

func (p slowProxy) Upstreams(ctx context.Context) (proxyclient.Upstreams, error) {
	if p.delay == 0 {
		<-ctx.Done()
		return proxyclient.Upstreams{}, ctx.Err()
	}

	timer := time.NewTimer(p.delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return proxyclient.Upstreams{}, ctx.Err()
	case <-timer.C:
		return proxyclient.Upstreams{HasProbe: true, ProbeSeq: bootMillis(), ProbeOK: true, LastReadyAgo: []time.Duration{0}}, nil
	}
}

func (slowProxy) Ready(context.Context, string) bool { return true }
func (slowProxy) APIServers() []string               { return []string{"10.0.0.1:6443"} }

type nopStatus struct{}

func (nopStatus) SetCondition(domain.Reason, string) {}
func (nopStatus) SetCPSource(domain.CPSource)        {}
func (nopStatus) SetProfile(string)                  {}

type noPeers struct{ in chan domain.Message }

func (noPeers) Peers() []domain.Peer                   { return nil }
func (noPeers) Peer(string) (domain.Peer, bool)        { return domain.Peer{}, false }
func (noPeers) Send(domain.Peer, domain.Message) error { return nil }
func (t noPeers) Received() <-chan domain.Message      { return t.in }

type feedDevice struct{ feeds atomic.Int64 }

func (*feedDevice) Identity() string                   { return "softdog" }
func (*feedDevice) SetTimeoutSupported() bool          { return true }
func (*feedDevice) MagicCloseSupported() bool          { return true }
func (d *feedDevice) KeepAlive() error                 { d.feeds.Add(1); return nil }
func (*feedDevice) SetTimeout(time.Duration) error     { return nil }
func (*feedDevice) GetTimeout() (time.Duration, error) { return critical.watchdogTimeout, nil }
func (*feedDevice) MagicClose() error                  { return nil }

type observedNode struct{}

func (observedNode) Snapshot() (domain.NodeSignals, bool) { return domain.NodeSignals{}, true }

type stopEvents struct{ stops atomic.Int64 }

func (e *stopEvents) Warning(reason, _ string) {
	if reason == "WatchdogFeedStopped" {
		e.stops.Add(1)
	}
}

// Proxy unavailability is not a CP loss. The proxy accepts the connection and
// does not answer in time: the own probes see the control plane, the reads
// stay every 200 ms, so the armed critical node never stops the feed.
func TestSlowUpstreamsDoNotStopTheFeed(t *testing.T) {
	for _, tt := range []struct {
		name  string
		delay time.Duration
	}{
		{"no answer", 0},
		{"answer in 190 ms", 190 * ms},
		{"answer in 200 ms", 200 * ms},
	} {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				testFeedWithProxy(t, slowProxy{delay: tt.delay}, 5*time.Minute)
			})
		})
	}
}

func testFeedWithProxy(t *testing.T, proxy cpview.Client, period time.Duration) {
	panicPath := filepath.Join(t.TempDir(), "panic")
	if err := os.WriteFile(panicPath, []byte("0\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	p := critical.params
	view := cpview.New(proxy, nopStatus{}, func() []string { return nil }, p.ControlPlaneLostAfter, critical.fastReadAfter, log.NewNop())

	var (
		armed atomic.Bool
		m     *watchdog.Manager
	)
	e := New(Deps{Transport: noPeers{in: make(chan domain.Message)}, ControlPlane: view, StopFeed: m.StopFeed, ResumeFeed: m.ResumeFeed, Finalize: m.Finalize, Metrics: &metrics{}}, log.NewNop())
	e.SetParams(p)

	device, events := &feedDevice{}, &stopEvents{}
	m = watchdog.New(watchdog.Deps{
		Open:            func() (watchdog.Device, error) { return device, nil },
		Nowayout:        func() (bool, error) { return false, nil },
		Node:            observedNode{},
		HasPeer:         func() bool { return true },
		ShouldFeed:      e.ShouldFeed,
		Decided:         e.Decided(),
		Status:          nopStatus{},
		Events:          events,
		UpdateMeta:      func(func(*domain.NodeMeta)) {},
		OnArmed:         armed.Store,
		KernelPanicPath: panicPath,
	}, log.NewNop())
	m.Apply(watchdog.Params{Timeout: critical.watchdogTimeout, FeedInterval: 250 * ms})

	ctx, cancel := context.WithCancel(t.Context())
	var wg sync.WaitGroup
	wg.Go(func() { _ = view.Run(ctx) })
	wg.Go(func() { _ = e.Run(ctx) })
	wg.Go(func() { m.Run(ctx) })

	// The own probes see the control plane after the first round.
	time.Sleep(2 * s)
	if !m.Armed() {
		t.Fatal("the watchdog is not armed")
	}

	// Look for a gap in the decision between two feed steps.
	var gaps int
	for end := time.Now().Add(period); time.Now().Before(end); time.Sleep(monitorStep) {
		if !e.ShouldFeed() {
			gaps++
		}
	}

	cancel()
	wg.Wait()

	if gaps != 0 || events.stops.Load() != 0 {
		t.Fatalf("no fresh decision %d times, feed stops %d; want none", gaps, events.stops.Load())
	}

	if want := int64(period / (250 * ms)); device.feeds.Load() < want {
		t.Fatalf("feeds = %d, want at least %d", device.feeds.Load(), want)
	}
}

type openedDevice struct {
	feedDevice

	openedAt time.Time
}

// Arm conditions: the agent arms at the first
// decision after the first read of the control plane. It does not wait for the
// next feed interval.
func TestArmsAtTheFirstDecision(t *testing.T) {
	for _, tt := range []struct {
		prof profile
		feed time.Duration
	}{
		{critical, 250 * ms},
		{normal, s},
		{slow, 2 * s},
	} {
		t.Run(tt.prof.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				panicPath := filepath.Join(t.TempDir(), "panic")
				if err := os.WriteFile(panicPath, []byte("0\n"), 0o600); err != nil {
					t.Fatal(err)
				}

				view := cpview.New(slowProxy{delay: ms}, nopStatus{}, func() []string { return nil }, tt.prof.params.ControlPlaneLostAfter, tt.prof.fastReadAfter, log.NewNop())

				var (
					armed atomic.Bool
					m     *watchdog.Manager
				)
				e := New(Deps{Transport: noPeers{in: make(chan domain.Message)}, ControlPlane: view, StopFeed: m.StopFeed, ResumeFeed: m.ResumeFeed, Finalize: m.Finalize, Metrics: &metrics{}}, log.NewNop())
				e.SetParams(tt.prof.params)

				device := &openedDevice{}
				m = watchdog.New(watchdog.Deps{
					Open: func() (watchdog.Device, error) {
						device.openedAt = time.Now()
						return device, nil
					},
					Nowayout:        func() (bool, error) { return false, nil },
					Node:            observedNode{},
					HasPeer:         func() bool { return true },
					ShouldFeed:      e.ShouldFeed,
					Decided:         e.Decided(),
					Status:          nopStatus{},
					Events:          &stopEvents{},
					UpdateMeta:      func(func(*domain.NodeMeta)) {},
					OnArmed:         armed.Store,
					KernelPanicPath: panicPath,
				}, log.NewNop())
				m.Apply(watchdog.Params{Timeout: tt.prof.watchdogTimeout, FeedInterval: tt.feed})

				ctx, cancel := context.WithCancel(t.Context())
				var wg sync.WaitGroup

				// The order of the agent start: the reads begin, then Start and Run.
				start := time.Now()
				wg.Go(func() { _ = view.Run(ctx) })
				wg.Go(func() { _ = e.Run(ctx) })
				m.Start()
				wg.Go(func() { m.Run(ctx) })

				time.Sleep(10 * ms)
				synctest.Wait()

				opened := device.openedAt
				cancel()
				wg.Wait()

				// The first read takes 1 ms. The first feed interval is 250 ms or more.
				if opened.IsZero() {
					t.Fatal("not armed 10 ms after the start, want the arm at the first read")
				}

				if d := opened.Sub(start); d > 10*ms {
					t.Fatalf("armed %v after the start, want at the first read", d)
				}
			})
		})
	}
}

// No CP data after the start: a read without
// data is not a decision.
func TestDecidedAfterTheFirstReadWithData(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cp := make(manualCP)
		e := New(Deps{Transport: noPeers{in: make(chan domain.Message)}, ControlPlane: cp, StopFeed: func() bool { return true }, Metrics: &metrics{}}, log.NewNop())
		e.SetParams(critical.params)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		go func() { _ = e.Run(ctx) }()

		decided := func() bool {
			synctest.Wait()

			select {
			case <-e.Decided():
				return true
			default:
				return false
			}
		}

		cp <- domain.CPState{ReadAt: time.Now()}
		if decided() {
			t.Fatal("decided on a read without data")
		}

		cp <- domain.CPState{Visible: true, Seen: true, ReadAt: time.Now()}
		if !decided() || !e.ShouldFeed() {
			t.Fatal("no decision to feed after a read with data")
		}
	})
}

// fedNode is an engine and a watchdog manager on manual control plane reads.
type fedNode struct {
	tr     *recorder
	engine *Engine
	wd     *watchdog.Manager
	device *feedDevice
	events *stopEvents
	cp     manualCP
}

func startFedNode(t *testing.T) *fedNode {
	t.Helper()

	return startFedNodeWith(t, false, log.NewNop())
}

// recorder takes the questions of the test and keeps the answers of the engine.
type recorder struct {
	noPeers

	mu      sync.Mutex
	answers []domain.Answer
}

func (*recorder) Peer(name string) (domain.Peer, bool) { return domain.Peer{Name: name}, true }

func (r *recorder) Send(_ domain.Peer, msg domain.Message) error {
	if a, ok := msg.(domain.Answer); ok {
		r.mu.Lock()
		r.answers = append(r.answers, a)
		r.mu.Unlock()
	}

	return nil
}

// ask sends a question and returns the verdict of the answer.
func (n *fedNode) ask(t *testing.T) domain.Verdict {
	t.Helper()

	n.tr.in <- domain.Question{Header: domain.Header{From: "w-2"}, PollID: 5}
	synctest.Wait()

	n.tr.mu.Lock()
	defer n.tr.mu.Unlock()

	if len(n.tr.answers) == 0 {
		t.Fatal("no answer")
	}

	return n.tr.answers[len(n.tr.answers)-1].Verdict
}

// startFedNodeWith starts the engine and the watchdog manager of a node. The
// node group of a disabled node has no watchdog. The engine and the manager
// write to logger.
func startFedNodeWith(t *testing.T, disabled bool, logger *log.Logger) *fedNode {
	t.Helper()

	panicPath := filepath.Join(t.TempDir(), "panic")
	if err := os.WriteFile(panicPath, []byte("0\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	n := &fedNode{device: &feedDevice{}, events: &stopEvents{}, cp: make(manualCP), tr: &recorder{noPeers: noPeers{in: make(chan domain.Message)}}}

	var armed atomic.Bool
	n.engine = New(Deps{
		Transport: n.tr, ControlPlane: n.cp, Metrics: &metrics{},
		StopFeed: func() bool { return n.wd.StopFeed() }, ResumeFeed: func() bool { return n.wd.ResumeFeed() },
		Finalize: func() { n.wd.Finalize() },
	}, logger)
	n.engine.SetParams(critical.params)

	n.wd = watchdog.New(watchdog.Deps{
		Open:            func() (watchdog.Device, error) { return n.device, nil },
		Nowayout:        func() (bool, error) { return false, nil },
		Node:            observedNode{},
		HasPeer:         func() bool { return true },
		ShouldFeed:      n.engine.ShouldFeed,
		Decided:         n.engine.Decided(),
		Status:          nopStatus{},
		Events:          n.events,
		UpdateMeta:      func(func(*domain.NodeMeta)) {},
		OnArmed:         armed.Store,
		KernelPanicPath: panicPath,
	}, logger)
	n.wd.Apply(watchdog.Params{Disabled: disabled, Timeout: critical.watchdogTimeout, FeedInterval: 250 * ms})

	ctx, cancel := context.WithCancel(t.Context())
	var wg sync.WaitGroup
	wg.Go(func() { _ = n.engine.Run(ctx) })
	wg.Go(func() { n.wd.Run(ctx) })
	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})

	return n
}

// read sends reads every 200 ms for d.
func (n *fedNode) read(visible bool, d time.Duration) {
	for end := time.Now().Add(d); time.Now().Before(end); time.Sleep(readInterval) {
		n.cp <- domain.CPState{Visible: visible, Seen: true, ReadAt: time.Now()}
	}
}

// A pause of the process or of the VM stops the reads for longer than the
// life of a decision. The control plane stays visible, so the feed continues
// after the pause.
func TestPauseOfTheReadsDoesNotStopTheFeed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := startFedNode(t)

		n.read(true, 3*s)
		time.Sleep(450 * ms)
		n.read(true, 3*s)

		before := n.device.feeds.Load()
		n.read(true, 3*s)

		if n.events.stops.Load() != 0 {
			t.Fatal("the feed stopped after a pause of the reads")
		}

		if got := n.device.feeds.Load() - before; got < 10 {
			t.Fatalf("feeds in 3 s after the pause = %d, want at least 10", got)
		}
	})
}

// After the engine stops the feed of the armed watchdog, the feed continues
// when the control plane is visible again. After Finalize, as after
// DeclareSuicide, it does not continue.
func TestStopOfTheArmedEngineReversesUntilFinal(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := startFedNode(t)

		n.read(true, 2*s)

		if !n.wd.Armed() {
			t.Fatal("the watchdog is not armed")
		}

		// No peers: the loss of the control plane stops the feed.
		n.read(false, s)

		if n.events.stops.Load() != 1 {
			t.Fatalf("feed stops = %d, want 1", n.events.stops.Load())
		}

		if got := n.ask(t); got != domain.VerdictNotSee {
			t.Fatalf("answer %v after the stop, want NOT SEE", got)
		}

		before := n.device.feeds.Load()
		n.read(true, s)

		if got := n.device.feeds.Load(); got == before {
			t.Fatal("no feed after the control plane is visible again")
		}

		n.read(false, s)
		n.wd.Finalize()

		before = n.device.feeds.Load()
		n.read(true, 3*s)

		if got := n.device.feeds.Load(); got != before {
			t.Fatalf("feeds after the final stop = %d, want none", got-before)
		}

		if !n.wd.Armed() {
			t.Fatal("the watchdog is disarmed after the stop")
		}

		// The planned stop of the agent does not disarm after the stop of the feed.
		if armed, stopped := n.wd.Close(); !armed || !stopped || n.wd.Disarm() {
			t.Fatalf("Close = %t, %t; want true, true and no Magic Close", armed, stopped)
		}
	})
}

// syncLog is a log output that several goroutines can write.
type syncLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *syncLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.buf.Write(p)
}

func (l *syncLog) count(part string) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	return strings.Count(l.buf.String(), part)
}

// The decision to feed again is not the feed. The log tells that the feed
// resumed only after the device gets the first feed after the resume.
func TestResumeIsLoggedAfterTheFeed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		out := &syncLog{}
		n := startFedNodeWith(t, false, log.NewLogger(log.WithOutput(out)))

		n.read(true, 2*s)
		n.read(false, s)

		if n.events.stops.Load() != 1 {
			t.Fatalf("feed stops = %d, want 1", n.events.stops.Load())
		}

		// Send the read between two feed steps.
		time.Sleep(10 * ms)
		synctest.Wait()

		before := n.device.feeds.Load()
		n.cp <- domain.CPState{Visible: true, Seen: true, ReadAt: time.Now()}
		synctest.Wait()

		if !n.engine.ShouldFeed() || n.device.feeds.Load() != before {
			t.Fatalf("decision to feed %v, feeds %d; want the decision without a feed", n.engine.ShouldFeed(), n.device.feeds.Load()-before)
		}

		if got := out.count("feed resumed"); got != 0 {
			t.Fatalf("lines about the resumed feed = %d before the feed, want 0", got)
		}

		n.read(true, s)

		if n.device.feeds.Load() == before {
			t.Fatal("no feed after the control plane is visible again")
		}

		if got := out.count("feed resumed"); got != 1 {
			t.Fatalf("lines about the resumed feed = %d after the feeds, want 1", got)
		}
	})
}

// The own FencingFailedNodeState is proven while the control plane is
// visible: the armed agent stops the feed for good. An agent without watchdog
// does not stop. Both answer SEE: the answer tells only the own view of the
// control plane.
func TestProvenStopsTheFeedForGood(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("disabled=%v", disabled), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				n := startFedNodeWith(t, disabled, log.NewNop())

				n.read(true, 2*s)
				n.wd.SetProven("Proven")
				n.read(true, s)

				before := n.device.feeds.Load()
				n.read(true, 2*s)

				if got := n.device.feeds.Load(); got != before {
					t.Fatalf("feeds after the proven FFNS = %d, want none", got-before)
				}

				if n.wd.Final() == disabled {
					t.Fatalf("final stop %v, want %v", n.wd.Final(), !disabled)
				}

				if got := n.ask(t); got != domain.VerdictSee {
					t.Fatalf("answer %v after the proven FFNS, want SEE", got)
				}
			})
		})
	}
}
