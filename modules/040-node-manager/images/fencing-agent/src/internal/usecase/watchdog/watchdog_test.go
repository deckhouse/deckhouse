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

package watchdog

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

// The profile timings of the Medium profile, so a wrong field assignment shows up.
const (
	testFeedInterval = 1 * time.Second
	testTimeout      = 10 * time.Second
)

type fakeDevice struct {
	mu sync.Mutex

	identity        string
	supportsTimeout bool
	supportsMagic   bool

	// effective is what the driver reports back; zero means "exactly as requested".
	effective     time.Duration
	setTimeoutErr error
	keepAliveErr  error
	magicCloseErr error
	timeLeftErr   error

	requested   []time.Duration
	keepAlives  int
	magicCloses int
	releases    int
}

func newFakeDevice() *fakeDevice {
	return &fakeDevice{
		identity:        "Software Watchdog",
		supportsTimeout: true,
		supportsMagic:   true,
		// softdog has no get_timeleft op; the manager must not care.
		timeLeftErr: errors.ErrUnsupported,
	}
}

func (d *fakeDevice) Identity() string          { return d.identity }
func (d *fakeDevice) SetTimeoutSupported() bool { return d.supportsTimeout }
func (d *fakeDevice) MagicCloseSupported() bool { return d.supportsMagic }
func (d *fakeDevice) GetTimeout() (time.Duration, error) {
	return testTimeout, nil
}

func (d *fakeDevice) SetTimeout(timeout time.Duration) (time.Duration, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.requested = append(d.requested, timeout)

	if d.setTimeoutErr != nil {
		return 0, d.setTimeoutErr
	}

	if d.effective != 0 {
		return d.effective, nil
	}

	return timeout, nil
}

func (d *fakeDevice) KeepAlive() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.keepAliveErr != nil {
		return d.keepAliveErr
	}

	d.keepAlives++

	return nil
}

func (d *fakeDevice) GetTimeLeft() (time.Duration, error) {
	if d.timeLeftErr != nil {
		return 0, d.timeLeftErr
	}

	return testTimeout, nil
}

func (d *fakeDevice) MagicClose() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.magicCloses++

	return d.magicCloseErr
}

func (d *fakeDevice) ReleaseWithoutDisarm() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.releases++

	return nil
}

func (d *fakeDevice) counters() (keepAlives, magicCloses, releases int) {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.keepAlives, d.magicCloses, d.releases
}

type fakeOpener struct {
	mu      sync.Mutex
	device  *fakeDevice
	err     error
	devices []*fakeDevice
}

func (o *fakeOpener) open() (Device, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.err != nil {
		return nil, o.err
	}

	o.devices = append(o.devices, o.device)

	return o.device, nil
}

func (o *fakeOpener) opens() int {
	o.mu.Lock()
	defer o.mu.Unlock()

	return len(o.devices)
}

type stubState struct {
	mu    sync.Mutex
	state Snapshot
}

func (s *stubState) set(state Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state = state
}

func (s *stubState) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.state
}

type recordedEvent struct {
	kind   string
	reason string
}

type fakeEvents struct {
	mu     sync.Mutex
	events []recordedEvent
}

func (e *fakeEvents) Normal(reason, _ string) {
	e.record("Normal", reason)
}

func (e *fakeEvents) Warning(reason, _ string) {
	e.record("Warning", reason)
}

func (e *fakeEvents) record(kind, reason string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.events = append(e.events, recordedEvent{kind: kind, reason: reason})
}

func (e *fakeEvents) count(reason string) int {
	e.mu.Lock()
	defer e.mu.Unlock()

	total := 0

	for _, event := range e.events {
		if event.reason == reason {
			total++
		}
	}

	return total
}

type harness struct {
	manager *Manager
	device  *fakeDevice
	opener  *fakeOpener
	state   *stubState
	events  *fakeEvents
	gate    struct {
		mu    sync.Mutex
		feed  bool
		cause string
	}
}

// newHarness wires a manager whose own Node is observed, healthy and without any
// maintenance annotation, i.e. the state in which fencing must be active.
func newHarness(t *testing.T) *harness {
	t.Helper()

	device := newFakeDevice()
	opener := &fakeOpener{device: device}
	state := &stubState{state: Snapshot{Observed: true}}
	events := &fakeEvents{}

	h := &harness{device: device, opener: opener, state: state, events: events}
	h.gate.feed = true

	h.manager = New(
		Params{FeedInterval: testFeedInterval, Timeout: testTimeout},
		Deps{
			Open:     opener.open,
			Nowayout: func() (bool, error) { return false, nil },
			State:    state,
			Events:   events,
			ShouldFeed: func() (bool, string) {
				h.gate.mu.Lock()
				defer h.gate.mu.Unlock()

				return h.gate.feed, h.gate.cause
			},
		},
		log.NewNop(),
	)

	// Run() sets this; the tests drive tick() directly to stay deterministic.
	h.manager.started.Store(true)

	return h
}

func (h *harness) closeGate(cause string) {
	h.gate.mu.Lock()
	defer h.gate.mu.Unlock()

	h.gate.feed, h.gate.cause = false, cause
}

func (h *harness) tick(t *testing.T) {
	t.Helper()

	if err := h.manager.tick(); err != nil {
		t.Fatalf("tick returned an error: %v", err)
	}
}

func TestManagerArmsOnceAndFeedsEveryTick(t *testing.T) {
	h := newHarness(t)

	h.tick(t)
	h.tick(t)
	h.tick(t)

	if h.opener.opens() != 1 {
		t.Errorf("device was opened %d times, want exactly one arming", h.opener.opens())
	}

	keepAlives, magicCloses, _ := h.device.counters()
	if keepAlives != 3 {
		t.Errorf("keepalives: %d, want one per tick (3)", keepAlives)
	}

	if magicCloses != 0 {
		t.Errorf("watchdog was disarmed %d times while fencing is active, want 0", magicCloses)
	}

	if !h.manager.Ready() {
		t.Error("manager must be ready once the watchdog is armed and fed")
	}
}

// The whole point of the stage: the kernel timeout must come from the SLA
// profile, not from the softdog module default.
func TestManagerAppliesTheProfileTimeoutToTheDevice(t *testing.T) {
	h := newHarness(t)

	h.tick(t)

	if len(h.device.requested) != 1 || h.device.requested[0] != testTimeout {
		t.Fatalf("SetTimeout calls: %v, want exactly one call with the profile timeout %s", h.device.requested, testTimeout)
	}

	if h.events.count(reasonArmed) != 1 {
		t.Errorf("armed events: %d, want 1", h.events.count(reasonArmed))
	}
}

func TestManagerRefusesADriverWithoutSetTimeoutSupport(t *testing.T) {
	h := newHarness(t)
	h.device.supportsTimeout = false

	err := h.manager.tick()
	if err == nil || !errors.Is(err, errFatal) {
		t.Fatalf("tick error is %v, want a fatal error: the profile timeout cannot be applied", err)
	}

	// The refusal must not leave an armed device behind: the process is about to exit.
	if _, magicCloses, _ := h.device.counters(); magicCloses != 1 {
		t.Errorf("magic closes: %d, want the device disarmed before the fatal exit", magicCloses)
	}

	if h.events.count(reasonRefused) != 1 {
		t.Errorf("refusal events: %d, want 1", h.events.count(reasonRefused))
	}
}

func TestManagerRefusesATimeoutTooCloseToTheFeedInterval(t *testing.T) {
	h := newHarness(t)
	// A driver that clamps 10s down to one feed interval: a single late tick would
	// then reset the Node.
	h.device.effective = testFeedInterval

	err := h.manager.tick()
	if err == nil || !errors.Is(err, errFatal) {
		t.Fatalf("tick error is %v, want a fatal error for an unusable timeout", err)
	}

	if _, magicCloses, _ := h.device.counters(); magicCloses != 1 {
		t.Errorf("magic closes: %d, want the device disarmed before the fatal exit", magicCloses)
	}
}

// A driver rounding the timeout up is a warning, not a refusal: fencing still works.
func TestManagerAcceptsADriverAdjustedTimeout(t *testing.T) {
	h := newHarness(t)
	h.device.effective = 12 * time.Second

	h.tick(t)

	if keepAlives, _, _ := h.device.counters(); keepAlives != 1 {
		t.Errorf("keepalives: %d, want the device armed and fed", keepAlives)
	}
}

func TestManagerDisarmsForMaintenanceAndReArmsAfterwards(t *testing.T) {
	h := newHarness(t)

	h.tick(t)

	h.state.set(Snapshot{
		Observed:           true,
		Maintenance:        true,
		MaintenanceReasons: []string{domain.FencingDisableAnnotation},
	})
	h.tick(t)
	h.tick(t)

	keepAlives, magicCloses, _ := h.device.counters()
	if magicCloses != 1 {
		t.Errorf("magic closes: %d, want exactly one disarm for maintenance", magicCloses)
	}

	if keepAlives != 1 {
		t.Errorf("keepalives: %d, want no feeding during maintenance", keepAlives)
	}

	if !h.manager.Ready() {
		t.Error("maintenance is a deliberate state and must not turn the pod NotReady")
	}

	// Annotation removed: fencing must come back, timeout included.
	h.state.set(Snapshot{Observed: true})
	h.tick(t)

	if h.opener.opens() != 2 {
		t.Errorf("device was opened %d times, want a re-arm after maintenance", h.opener.opens())
	}

	if len(h.device.requested) != 2 {
		t.Errorf("SetTimeout calls: %v, want the profile timeout applied again on re-arm", h.device.requested)
	}
}

func TestManagerNeverReArmsWhileTheNodeIsBeingRemoved(t *testing.T) {
	h := newHarness(t)

	h.tick(t)

	h.state.set(Snapshot{
		Observed:       true,
		PlannedRemoval: true,
		RemovalReason:  domain.RemovalReasonDeleting,
	})
	h.tick(t)
	h.tick(t)
	h.tick(t)

	keepAlives, magicCloses, _ := h.device.counters()
	if magicCloses != 1 {
		t.Errorf("magic closes: %d, want exactly one disarm for the planned removal", magicCloses)
	}

	if keepAlives != 1 || h.opener.opens() != 1 {
		t.Errorf("keepalives %d and opens %d: a Node being removed must not be fed or re-armed", keepAlives, h.opener.opens())
	}
}

// Without WDIOF_MAGICCLOSE the kernel ignores the disarm, so stopping the feed
// would panic the Node in the middle of a planned operation.
func TestManagerKeepsFeedingWhenTheDeviceCannotBeDisarmed(t *testing.T) {
	h := newHarness(t)
	h.device.supportsMagic = false

	h.tick(t)

	h.state.set(Snapshot{
		Observed:           true,
		Maintenance:        true,
		MaintenanceReasons: []string{domain.ApprovedAnnotation},
	})
	h.tick(t)
	h.tick(t)

	keepAlives, magicCloses, _ := h.device.counters()
	if magicCloses != 0 {
		t.Errorf("magic closes: %d, want none on a device that cannot be disarmed", magicCloses)
	}

	if keepAlives != 3 {
		t.Errorf("keepalives: %d, want feeding to continue through maintenance (3)", keepAlives)
	}

	if h.events.count(reasonNotDisarmable) == 0 {
		t.Error("the operator must be told the watchdog cannot be disarmed")
	}
}

// A failed disarm is the dangerous case: the kernel keeps counting while the
// descriptor is gone, so the Node would reset in the middle of the planned
// operation unless the agent reopens the device and keeps feeding.
func TestManagerKeepsFeedingWhenTheDisarmFails(t *testing.T) {
	h := newHarness(t)

	h.tick(t)

	h.device.magicCloseErr = errors.New("input/output error")
	h.state.set(Snapshot{
		Observed:           true,
		Maintenance:        true,
		MaintenanceReasons: []string{domain.FencingDisableAnnotation},
	})

	h.tick(t)
	h.tick(t)

	keepAlives, magicCloses, _ := h.device.counters()
	if magicCloses != 1 {
		t.Errorf("magic closes: %d, want one failed attempt and no retry on the lost descriptor", magicCloses)
	}

	if keepAlives != 3 {
		t.Errorf("keepalives: %d, want feeding to continue through the planned operation (3)", keepAlives)
	}

	if h.opener.opens() != 2 {
		t.Errorf("device was opened %d times, want a reopen after the descriptor was lost", h.opener.opens())
	}

	if !h.manager.Ready() {
		t.Error("feeding through a planned operation is the best available state, not a fault")
	}
}

func TestManagerStopsTheAgentWhenTheOwnNodeUIDChanged(t *testing.T) {
	h := newHarness(t)

	h.tick(t)

	h.state.set(Snapshot{Observed: true, UIDMismatch: true})

	err := h.manager.tick()
	if err == nil || !errors.Is(err, errFatal) {
		t.Fatalf("tick error is %v, want a fatal error so the pod restarts with a fresh identity", err)
	}

	if h.events.count(reasonIdentityChanged) != 1 {
		t.Errorf("identity events: %d, want 1", h.events.count(reasonIdentityChanged))
	}

	// The deferred Close in the agent is what disarms on the way out.
	h.manager.Close()

	if _, magicCloses, _ := h.device.counters(); magicCloses != 1 {
		t.Errorf("magic closes: %d, want the device disarmed on shutdown", magicCloses)
	}
}

// A failed keepalive must not disarm a working watchdog: the descriptor is
// released without the magic character, so the kernel keeps counting.
func TestManagerReopensTheDeviceAfterAFailedKeepalive(t *testing.T) {
	h := newHarness(t)

	h.tick(t)

	h.device.keepAliveErr = errors.New("input/output error")
	h.tick(t)

	_, magicCloses, releases := h.device.counters()
	if magicCloses != 0 {
		t.Errorf("magic closes: %d, want a failed keepalive never to disarm the watchdog", magicCloses)
	}

	if releases != 1 {
		t.Errorf("releases: %d, want the stale descriptor released once", releases)
	}

	h.device.keepAliveErr = nil
	h.tick(t)

	if h.opener.opens() != 2 {
		t.Errorf("device was opened %d times, want a reopen after the failure", h.opener.opens())
	}

	if !h.manager.Ready() {
		t.Error("manager must be ready again after a recovered keepalive")
	}
}

func TestManagerTurnsNotReadyAfterThreeFailedFeeds(t *testing.T) {
	h := newHarness(t)

	// Readiness is earned by a successful feed, never assumed.
	if h.manager.Ready() {
		t.Error("manager must not report ready before the watchdog has been fed once")
	}

	h.tick(t)

	h.device.keepAliveErr = errors.New("input/output error")

	h.tick(t)

	if !h.manager.Ready() {
		t.Error("a single failure must not flip readiness of a watchdog that was working")
	}

	h.tick(t)
	h.tick(t)

	if h.manager.Ready() {
		t.Error("manager must report NotReady after three consecutive failures")
	}

	h.tick(t)

	if h.events.count(reasonFeedFailed) != 1 {
		t.Errorf("feed failure events: %d, want exactly one per streak", h.events.count(reasonFeedFailed))
	}
}

func TestManagerWaitsForTheOwnNodeCacheBeforeArming(t *testing.T) {
	h := newHarness(t)
	h.state.set(Snapshot{})

	h.tick(t)

	if h.opener.opens() != 0 {
		t.Errorf("device was opened %d times, want no arming before the own Node is known", h.opener.opens())
	}

	if h.manager.Ready() {
		t.Error("manager must not report ready before the own Node cache is filled")
	}
}

// The quorum gate of the ADR: feeding stops, the device stays armed and the Node
// is expected to be reset when the timeout expires.
func TestManagerStopsFeedingWithoutDisarmingWhenTheGateCloses(t *testing.T) {
	h := newHarness(t)

	h.tick(t)
	h.closeGate("quorum lost")
	h.tick(t)
	h.tick(t)

	keepAlives, magicCloses, releases := h.device.counters()
	if keepAlives != 1 {
		t.Errorf("keepalives: %d, want feeding to stop at the closed gate", keepAlives)
	}

	if magicCloses != 0 || releases != 0 {
		t.Errorf("magic closes %d and releases %d: a closed gate must leave the watchdog armed", magicCloses, releases)
	}

	if h.manager.Ready() {
		t.Error("a starving watchdog is not a healthy state")
	}

	if h.events.count(reasonStarvation) != 1 {
		t.Errorf("starvation events: %d, want exactly one per streak", h.events.count(reasonStarvation))
	}
}

func TestManagerRefusesToRunOnANowayoutKernel(t *testing.T) {
	h := newHarness(t)
	h.manager.deps.Nowayout = func() (bool, error) { return true, nil }

	err := h.manager.Run(t.Context())
	if err == nil || !errors.Is(err, errFatal) {
		t.Fatalf("Run error is %v, want a fatal refusal on a nowayout kernel", err)
	}

	// Nothing may be opened: with nowayout the agent could not disarm what it armed.
	if h.opener.opens() != 0 {
		t.Errorf("device was opened %d times, want none", h.opener.opens())
	}

	if h.events.count(reasonRefused) != 1 {
		t.Errorf("refusal events: %d, want 1", h.events.count(reasonRefused))
	}
}

func TestManagerRefusesWhenNowayoutCannotBeVerified(t *testing.T) {
	h := newHarness(t)
	h.manager.deps.Nowayout = func() (bool, error) { return false, errors.New("no such file or directory") }

	err := h.manager.Run(t.Context())
	if err == nil || !errors.Is(err, errFatal) {
		t.Fatalf("Run error is %v, want a fatal refusal when nowayout cannot be read", err)
	}

	if h.opener.opens() != 0 {
		t.Errorf("device was opened %d times, want none", h.opener.opens())
	}
}

func TestManagerRejectsUnusableProfileTimings(t *testing.T) {
	cases := map[string]Params{
		"zero feed interval":         {FeedInterval: 0, Timeout: 10 * time.Second},
		"sub-second timeout":         {FeedInterval: 100 * time.Millisecond, Timeout: 500 * time.Millisecond},
		"timeout below two feeds":    {FeedInterval: 6 * time.Second, Timeout: 10 * time.Second},
		"timeout equal to two feeds": {FeedInterval: 5 * time.Second, Timeout: 9 * time.Second},
	}

	for name, params := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.manager.params = params

			err := h.manager.Run(t.Context())
			if err == nil || !errors.Is(err, errFatal) {
				t.Fatalf("Run error is %v, want a fatal error for %s", err, name)
			}

			if h.opener.opens() != 0 {
				t.Errorf("device was opened %d times, want none", h.opener.opens())
			}
		})
	}
}

// The loop must feed on the profile interval, and the first feed must not wait
// for a whole interval after the join.
func TestManagerFeedLoopFeedsOnTheProfileInterval(t *testing.T) {
	h := newHarness(t)
	h.manager.params = Params{FeedInterval: 10 * time.Millisecond, Timeout: 10 * time.Second}

	ctx, cancel := context.WithTimeout(t.Context(), 80*time.Millisecond)
	defer cancel()

	if err := h.manager.Run(ctx); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}

	if keepAlives, _, _ := h.device.counters(); keepAlives < 3 {
		t.Errorf("keepalives: %d, want the loop to feed repeatedly within 80ms at a 10ms interval", keepAlives)
	}
}

func TestCloseIsIdempotentAndDisarms(t *testing.T) {
	h := newHarness(t)

	h.tick(t)

	h.manager.Close()
	h.manager.Close()

	if _, magicCloses, _ := h.device.counters(); magicCloses != 1 {
		t.Errorf("magic closes: %d, want exactly one on repeated Close", magicCloses)
	}
}

func TestAliveTracksTheFeedLoop(t *testing.T) {
	h := newHarness(t)

	// Before the loop starts the agent is still joining gossip: liveness must pass.
	h.manager.started.Store(false)

	if !h.manager.Alive() {
		t.Error("liveness must pass before the feed loop starts")
	}

	h.manager.started.Store(true)
	h.manager.lastTick.Store(time.Now().Add(-time.Hour).UnixNano())

	if h.manager.Alive() {
		t.Error("liveness must fail once the feed loop stopped ticking")
	}

	h.tick(t)

	if !h.manager.Alive() {
		t.Error("liveness must pass right after a tick")
	}
}
