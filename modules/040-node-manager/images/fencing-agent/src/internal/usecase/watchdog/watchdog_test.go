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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

var (
	critical = Params{Timeout: 3 * time.Second, FeedInterval: 250 * time.Millisecond}
	normal   = Params{Timeout: 15 * time.Second, FeedInterval: time.Second}
	slow     = Params{Timeout: 30 * time.Second, FeedInterval: 2 * time.Second}
)

// journal records the calls to the fakes in order.
type journal struct {
	mu      sync.Mutex
	entries []string
}

func (j *journal) add(format string, args ...any) {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.entries = append(j.entries, fmt.Sprintf(format, args...))
}

func (j *journal) all() []string {
	j.mu.Lock()
	defer j.mu.Unlock()

	return slices.Clone(j.entries)
}

func (j *journal) count(entry string) int {
	n := 0

	for _, e := range j.all() {
		if e == entry {
			n++
		}
	}

	return n
}

// index returns the position of the first entry after from, or -1.
func (j *journal) index(entry string, from int) int {
	entries := j.all()
	for i := from; i < len(entries); i++ {
		if entries[i] == entry {
			return i
		}
	}

	return -1
}

type fakeDevice struct {
	j            *journal
	noSetTimeout bool
	noMagic      bool
	// accepted returns the timeout that the driver keeps after WDIOC_SETTIMEOUT.
	accepted func(time.Duration) time.Duration
	timeout  time.Duration
	// stop is called by Magic Close. It can be nil.
	stop func()
	// keepAliveErr is the error of KeepAlive.
	keepAliveErr error
}

func (d *fakeDevice) Identity() string          { return "Software Watchdog" }
func (d *fakeDevice) SetTimeoutSupported() bool { return !d.noSetTimeout }
func (d *fakeDevice) MagicCloseSupported() bool { return !d.noMagic }

func (d *fakeDevice) KeepAlive() error {
	d.j.add("keepalive")

	return d.keepAliveErr
}

func (d *fakeDevice) SetTimeout(timeout time.Duration) error {
	d.j.add("settimeout %s", timeout)

	d.timeout = timeout
	if d.accepted != nil {
		d.timeout = d.accepted(timeout)
	}

	return nil
}

func (d *fakeDevice) GetTimeout() (time.Duration, error) {
	d.j.add("gettimeout")

	return d.timeout, nil
}

func (d *fakeDevice) MagicClose() error {
	d.j.add("magicclose")

	if d.stop != nil {
		d.stop()
	}

	return nil
}

type fakeNode struct {
	mu      sync.Mutex
	signals domain.NodeSignals
}

func (n *fakeNode) Snapshot() (domain.NodeSignals, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.signals, true
}

func (n *fakeNode) maintenance(annotations ...string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.signals.Maintenance = len(annotations) > 0
	n.signals.MaintenanceReasons = annotations
}

type fakeStatus struct{ j *journal }

func (s fakeStatus) SetCondition(reason domain.Reason, message string) {
	s.j.add("condition %s", reason)
}
func (fakeStatus) SetCPSource(domain.CPSource) {}
func (fakeStatus) SetProfile(string)           {}

type fakeEvents struct{ j *journal }

func (e fakeEvents) Warning(reason, message string) {
	e.j.add("event %s", reason)
	e.j.add("event message %s", message)
}

type harness struct {
	t      *testing.T
	j      *journal
	m      *Manager
	node   *fakeNode
	device *fakeDevice
	dir    string
	// decided is closed: the engine has decided.
	decided chan struct{}

	mu         sync.Mutex
	active     bool
	held       bool
	hasPeer    bool
	shouldFeed bool
	nowayout   error
	openErr    error
	meta       domain.NodeMeta
	opens      int
}

func newHarness(t *testing.T, p Params) *harness {
	t.Helper()

	h := &harness{
		t: t, j: &journal{}, node: &fakeNode{}, dir: t.TempDir(), decided: make(chan struct{}),
		hasPeer: true, shouldFeed: true,
	}
	close(h.decided)
	// As the kernel: the open starts the timer, and Magic Close stops it and
	// closes the device.
	h.device = &fakeDevice{j: h.j, stop: func() {
		h.mu.Lock()
		defer h.mu.Unlock()

		h.active, h.held = false, false
	}}
	h.write("panic", "0\n")
	h.write("cmdline", "BOOT_IMAGE=/vmlinuz root=/dev/sda1 softdog.soft_panic=1\n")

	h.m = New(Deps{
		Open: func() (Device, error) {
			h.mu.Lock()
			defer h.mu.Unlock()

			if h.openErr != nil {
				return nil, h.openErr
			}

			// As the kernel: the device has one holder.
			if h.held {
				h.j.add("open EBUSY")

				return nil, fmt.Errorf("open /dev/watchdog: %w", syscall.EBUSY)
			}

			h.opens++
			h.active, h.held = true, true
			h.j.add("open")

			return h.device, nil
		},
		Nowayout: func() (bool, error) {
			h.mu.Lock()
			defer h.mu.Unlock()

			if h.nowayout != nil {
				return false, h.nowayout
			}

			return false, nil
		},
		Active:     func() (bool, error) { return h.get(&h.active), nil },
		Node:       h.node,
		HasPeer:    func() bool { return h.get(&h.hasPeer) },
		ShouldFeed: func() bool { return h.get(&h.shouldFeed) },
		Decided:    h.decided,
		Status:     fakeStatus{j: h.j},
		Events:     fakeEvents{j: h.j},
		UpdateMeta: func(update func(*domain.NodeMeta)) {
			h.mu.Lock()
			update(&h.meta)
			meta := h.meta
			h.mu.Unlock()

			h.j.add("meta armed=%t maintenance=%t", meta.Armed, meta.Maintenance)
		},
		KernelPanicPath: filepath.Join(h.dir, "panic"),
		CmdlinePath:     filepath.Join(h.dir, "cmdline"),
		DevicePath:      "/dev/watchdog",
	}, log.NewNop())
	h.m.Apply(p)

	return h
}

func (h *harness) get(v *bool) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	return *v
}

func (h *harness) setBool(v *bool, value bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	*v = value
}

func (h *harness) write(name, content string) {
	h.t.Helper()

	if err := os.WriteFile(filepath.Join(h.dir, name), []byte(content), 0o600); err != nil {
		h.t.Fatal(err)
	}
}

// stopEngine stops the feed as the engine does: the decision does not let
// the agent feed, and StopFeed stops the feed of the armed device.
func (h *harness) stopEngine() {
	h.setBool(&h.shouldFeed, false)
	h.m.StopFeed()
}

// step does one step and publishes it, as Run does on each tick.
func (h *harness) step() {
	h.m.mu.Lock()
	defer h.m.mu.Unlock()

	h.m.step()
	h.m.publish()
}

// run starts Run in a synctest bubble and stops it at the end of the test.
func (h *harness) run() {
	ctx, cancel := context.WithCancel(h.t.Context())
	done := make(chan struct{})

	go func() {
		defer close(done)

		h.m.Run(ctx)
	}()

	h.t.Cleanup(func() {
		cancel()
		<-done
	})
}

func (h *harness) wantReason(want domain.Reason) {
	h.t.Helper()

	if got := h.m.Reason(); got != want {
		h.t.Fatalf("reason = %q (%s), want %q", got, h.m.message, want)
	}
}

func (h *harness) wantMessage(part string) {
	h.t.Helper()

	if !strings.Contains(h.m.message, part) {
		h.t.Fatalf("message = %q, want it to contain %q", h.m.message, part)
	}
}

func (h *harness) wantArmed(want bool) {
	h.t.Helper()

	if got := h.m.Armed(); got != want {
		h.t.Fatalf("Armed() = %t, want %t; journal %v", got, want, h.j.all())
	}
}

func TestArmsWhenAllConditionsHold(t *testing.T) {
	h := newHarness(t, critical)

	h.step()

	h.wantArmed(true)
	h.wantReason(domain.ReasonArmed)

	want := []string{"open", "settimeout 3s", "gettimeout", "meta armed=true maintenance=false", "condition Armed"}
	if got := h.j.all(); !slices.Equal(got, want) {
		t.Fatalf("journal = %v, want %v", got, want)
	}
}

func TestDoesNotArmWhileAConditionIsFalse(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(h *harness)
		reason  domain.Reason
		message string
		opened  bool
	}{
		{
			name:    "a single node",
			prepare: func(h *harness) { h.hasPeer = false },
			reason:  domain.ReasonNoEvacuationTarget,
		},
		{
			name:    "kernel.panic is 10",
			prepare: func(h *harness) { h.write("panic", "10\n") },
			reason:  domain.ReasonKernelPanicMismatch,
			message: "kernel.panic is 10",
		},
		{
			name:    "no fresh decision",
			prepare: func(h *harness) { h.shouldFeed = false },
			reason:  domain.ReasonWaitingForControlPlane,
			message: "no fresh decision",
		},
		{
			name: "the driver keeps another timeout",
			prepare: func(h *harness) {
				h.device.accepted = func(time.Duration) time.Duration { return 60 * time.Second }
			},
			reason:  domain.ReasonWatchdogTimeoutMismatch,
			message: "WDIOC_GETTIMEOUT returned 1m0s",
			opened:  true,
		},
		{
			name:    "the driver has no WDIOF_SETTIMEOUT",
			prepare: func(h *harness) { h.device.noSetTimeout = true },
			reason:  domain.ReasonWatchdogTimeoutMismatch,
			message: "no WDIOF_SETTIMEOUT",
			opened:  true,
		},
		{
			name:    "the driver has no WDIOF_MAGICCLOSE",
			prepare: func(h *harness) { h.device.noMagic = true },
			reason:  domain.ReasonWatchdogTimeoutMismatch,
			message: "no WDIOF_MAGICCLOSE",
			opened:  true,
		},
		{
			name:    "the driver has nowayout",
			prepare: func(h *harness) { h.nowayout = errors.New("nowayout is 1") },
			reason:  domain.ReasonWatchdogTimeoutMismatch,
			message: "nowayout",
		},
		{
			name:    "the pod has no watchdog device",
			prepare: func(h *harness) { h.nowayout = fmt.Errorf("stat %q: %w", "/dev/watchdog", syscall.ENOENT) },
			reason:  domain.ReasonWatchdogDeviceMissing,
			message: "the watchdog device /dev/watchdog is not available in the pod",
		},
		{
			name: "the device goes away before the open",
			prepare: func(h *harness) {
				h.openErr = &fs.PathError{Op: "open", Path: "/dev/watchdog", Err: syscall.ENOENT}
			},
			reason:  domain.ReasonWatchdogDeviceMissing,
			message: "the watchdog device /dev/watchdog is not available in the pod",
		},
		{
			name:    "the device node has no driver",
			prepare: func(h *harness) { h.openErr = &fs.PathError{Op: "open", Path: "/dev/watchdog", Err: syscall.ENODEV} },
			reason:  domain.ReasonWatchdogDeviceMissing,
			message: "the watchdog device /dev/watchdog is not available in the pod",
		},
		{
			name:    "the device node has no device",
			prepare: func(h *harness) { h.openErr = &fs.PathError{Op: "open", Path: "/dev/watchdog", Err: syscall.ENXIO} },
			reason:  domain.ReasonWatchdogDeviceMissing,
			message: "the watchdog device /dev/watchdog is not available in the pod",
		},
		{
			name:    "the device cannot be read",
			prepare: func(h *harness) { h.nowayout = fmt.Errorf("stat %q: %w", "/dev/watchdog", syscall.EACCES) },
			reason:  domain.ReasonWatchdogTimeoutMismatch,
			message: "permission denied",
		},
		{
			name:    "another process holds the device",
			prepare: func(h *harness) { h.openErr = fmt.Errorf("open /dev/watchdog: %w", syscall.EBUSY) },
			reason:  domain.ReasonWatchdogTimeoutMismatch,
			message: "busy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, normal)
			tt.prepare(h)

			h.step()

			h.wantArmed(false)
			h.wantReason(tt.reason)
			h.wantMessage(tt.message)

			if tt.opened != (h.opens > 0) {
				t.Fatalf("device opened = %t, want %t", h.opens > 0, tt.opened)
			}

			// The refused device is closed with Magic Close.
			if tt.opened && h.j.count("magicclose") != 1 {
				t.Fatalf("journal = %v, want one Magic Close", h.j.all())
			}

			if h.j.count("keepalive") != 0 {
				t.Fatal("the agent fed an unarmed device")
			}
		})
	}
}

func TestArmsWhenASecondNodeComes(t *testing.T) {
	h := newHarness(t, critical)
	h.hasPeer = false

	h.step()
	h.wantReason(domain.ReasonNoEvacuationTarget)

	h.setBool(&h.hasPeer, true)
	h.step()

	h.wantArmed(true)
	h.wantReason(domain.ReasonArmed)
}

func TestKeepsArmedWhenThePeersGo(t *testing.T) {
	h := newHarness(t, normal)
	h.step()

	h.setBool(&h.hasPeer, false)
	h.step()

	h.wantArmed(true)
	h.wantReason(domain.ReasonArmed)
}

func TestKernelPanicIsCheckedAgainEveryTenSeconds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, slow)
		h.write("panic", "10\n")
		h.m.Start()
		h.run()

		synctest.Wait()
		h.wantReason(domain.ReasonKernelPanicMismatch)

		// sysctl-tuner sets the value. The agent arms without a restart in the next check.
		h.write("panic", "0\n")
		time.Sleep(CheckInterval)
		synctest.Wait()

		h.wantArmed(true)
		h.wantReason(domain.ReasonArmed)
		h.wantTimeout(t, 30*time.Second)
	})
}

func TestArmsWhenTheDeviceAppears(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, normal)
		h.nowayout = fmt.Errorf("stat %q: %w", "/host/dev/watchdog", syscall.ENOENT)
		h.m.Start()
		h.run()

		synctest.Wait()
		h.wantArmed(false)
		h.wantReason(domain.ReasonWatchdogDeviceMissing)
		h.wantMessage("the watchdog device /dev/watchdog is not available in the pod: stat")
		h.wantMessage("no such file or directory")

		if strings.Contains(h.m.message, "nowayout") {
			t.Fatalf("message = %q, want no word about nowayout", h.m.message)
		}

		if h.meta.Armed {
			t.Fatal("NodeMeta tells armed without a device")
		}

		// The agent tries again after CheckInterval, not before.
		h.mu.Lock()
		h.nowayout = nil
		h.mu.Unlock()
		time.Sleep(CheckInterval - time.Second)
		synctest.Wait()
		h.wantReason(domain.ReasonWatchdogDeviceMissing)

		// softdog creates the device after the pod starts. The agent arms without a restart.
		time.Sleep(time.Second)
		synctest.Wait()

		h.wantArmed(true)
		h.wantReason(domain.ReasonArmed)

		if !h.meta.Armed {
			t.Fatal("NodeMeta tells not armed after the arm")
		}
	})
}

func (h *harness) wantTimeout(t *testing.T, want time.Duration) {
	t.Helper()

	if h.device.timeout != want {
		t.Fatalf("device timeout = %s, want %s", h.device.timeout, want)
	}
}

func TestKernelPanicChangeOnAnArmedNodeDisarms(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, normal)
		h.run()
		time.Sleep(normal.FeedInterval)
		synctest.Wait()
		h.wantArmed(true)

		h.write("panic", "10\n")
		time.Sleep(CheckInterval)
		synctest.Wait()

		h.wantArmed(false)
		h.wantReason(domain.ReasonKernelPanicMismatch)

		if h.j.count("magicclose") != 1 {
			t.Fatalf("journal = %v, want one Magic Close", h.j.all())
		}

		// The agent does not change kernel.panic.
		if raw, _ := os.ReadFile(filepath.Join(h.dir, "panic")); string(raw) != "10\n" {
			t.Fatalf("kernel.panic file = %q, the agent changed it", raw)
		}

		h.write("panic", "0\n")
		time.Sleep(CheckInterval)
		synctest.Wait()

		h.wantArmed(true)
		h.wantReason(domain.ReasonArmed)
	})
}

func TestEngineNeedsSoftPanicOnTheCommandLine(t *testing.T) {
	engine := critical
	engine.Engine = true

	h := newHarness(t, engine)
	h.write("cmdline", "BOOT_IMAGE=/vmlinuz root=/dev/sda1 softdog.soft_panic=0\n")

	h.step()

	h.wantArmed(false)
	h.wantReason(domain.ReasonKernelPanicMismatch)
	h.wantMessage("softdog.soft_panic=1 is not on the kernel command line")

	h = newHarness(t, engine)
	h.step()
	h.wantArmed(true)
}

func TestCommandLineIsNotCheckedOutsideEngine(t *testing.T) {
	h := newHarness(t, critical)
	h.write("cmdline", "BOOT_IMAGE=/vmlinuz\n")

	h.step()

	h.wantArmed(true)
}

func TestFeedsOnEachIntervalWhileTheDecisionIsFresh(t *testing.T) {
	for _, p := range []Params{critical, normal, slow} {
		t.Run(p.Timeout.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				h := newHarness(t, p)
				h.m.Start()
				h.run()

				time.Sleep(10*p.FeedInterval + p.FeedInterval/2)
				synctest.Wait()

				if got := h.j.count("keepalive"); got != 10 {
					t.Fatalf("feeds in 10 intervals of %s = %d, want 10", p.FeedInterval, got)
				}
			})
		})
	}
}

func TestStopOfTheFeedIsNotReversible(t *testing.T) {
	h := newHarness(t, critical)
	h.step()
	h.step()

	h.stopEngine()
	h.step()

	if h.j.count("event WatchdogFeedStopped") != 1 {
		t.Fatalf("journal = %v, want the event of the stop", h.j.all())
	}

	// The manager keeps the stop even if the engine lets it feed.
	h.setBool(&h.shouldFeed, true)
	for range 10 {
		h.step()
	}

	if got := h.j.count("keepalive"); got != 1 {
		t.Fatalf("feeds = %d, want 1 (only before the stop)", got)
	}

	// The armed agent answers DYING after the stop.
	h.wantArmed(true)

	// Maintenance and SIGTERM do not disarm after the stop.
	h.node.maintenance(domain.FencingDisableAnnotation)
	h.step()
	h.m.Disarm()

	if h.j.count("magicclose") != 0 {
		t.Fatalf("journal = %v, want no Magic Close after the stop", h.j.all())
	}
}

func TestStopOfTheEngineBeforeTheStepIsFinal(t *testing.T) {
	// The engine decided to stop and answers DYING. A reason to disarm that
	// comes before the next step does not do Magic Close.
	tests := []struct {
		name    string
		trigger func(h *harness)
	}{
		{name: "SIGTERM", trigger: func(h *harness) { h.m.Disarm() }},
		{name: "maintenance", trigger: func(h *harness) {
			h.node.maintenance(domain.FencingDisableAnnotation)
			h.step()
		}},
		{name: "switch to Disabled", trigger: func(h *harness) {
			p := critical
			p.Disabled = true
			h.m.Apply(p)
			h.step()
		}},
		{name: "kernel.panic mismatch", trigger: func(h *harness) {
			h.write("panic", "10\n")
			h.m.mu.Lock()
			h.m.kernelCheckedAt = time.Time{}
			h.m.mu.Unlock()
			h.step()
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, critical)
			h.step()
			h.step()

			h.stopEngine()
			tt.trigger(h)

			if h.j.count("magicclose") != 0 {
				t.Fatalf("journal = %v, want no Magic Close after the stop of the engine", h.j.all())
			}

			if h.j.count("event WatchdogFeedStopped") != 1 {
				t.Fatalf("journal = %v, want the event of the stop", h.j.all())
			}

			h.wantArmed(true)

			// The stop is final.
			h.setBool(&h.shouldFeed, true)
			h.step()

			if got := h.j.count("keepalive"); got != 1 {
				t.Fatalf("feeds = %d, want 1 (only before the stop)", got)
			}
		})
	}
}

func TestDecisionThatIsNotFreshIsNotAStop(t *testing.T) {
	// A pause of the VM or of the process makes the decision old. The step
	// does not feed, and the feed continues at the next fresh decision.
	h := newHarness(t, critical)
	h.step()
	h.step()

	h.setBool(&h.shouldFeed, false)
	h.step()

	if got := h.j.count("keepalive"); got != 1 {
		t.Fatalf("feeds = %d, want 1 (no feed without a fresh decision)", got)
	}

	h.setBool(&h.shouldFeed, true)
	h.step()

	if got := h.j.count("keepalive"); got != 2 || h.j.count("event WatchdogFeedStopped") != 0 {
		t.Fatalf("journal = %v, want the feed after the fresh decision and no stop", h.j.all())
	}

	// SIGTERM without a fresh decision disarms.
	h.setBool(&h.shouldFeed, false)
	h.m.Disarm()

	if h.j.count("magicclose") != 1 {
		t.Fatalf("journal = %v, want Magic Close at the stop of the agent", h.j.all())
	}

	h.wantArmed(false)
}

func TestUnarmedAgentHasNoIrreversibleStop(t *testing.T) {
	h := newHarness(t, critical)
	h.hasPeer = false
	h.step()

	h.setBool(&h.shouldFeed, false)
	h.step()
	h.setBool(&h.shouldFeed, true)
	h.setBool(&h.hasPeer, true)
	h.step()

	h.wantArmed(true)
}

func TestFirstFeedOnlyAfterANewDecision(t *testing.T) {
	// The process restarted. Open and WDIOC_SETTIMEOUT restart the timer, so
	// the agent does not open the device before a fresh decision.
	h := newHarness(t, critical)
	h.shouldFeed = false

	h.m.Start()
	h.step()

	if h.opens != 0 {
		t.Fatal("the agent opened the device before a fresh decision")
	}

	h.setBool(&h.shouldFeed, true)
	h.step()

	if h.opens != 1 || h.j.index("open", 0) < 0 {
		t.Fatalf("journal = %v, want the device opened after the decision", h.j.all())
	}
}

func TestStartDoesNotPublish(t *testing.T) {
	h := newHarness(t, critical)

	h.m.Start()
	h.wantArmed(true)

	if h.j.index("condition Armed", 0) >= 0 || h.j.index("meta armed=true maintenance=false", 0) >= 0 {
		t.Fatalf("journal = %v, Start published", h.j.all())
	}

	synctest.Test(t, func(t *testing.T) {
		h2 := newHarness(t, critical)
		h2.m.Start()
		h2.run()
		synctest.Wait()

		if h2.j.count("condition Armed") != 1 {
			t.Fatalf("journal = %v, want Run to publish the condition", h2.j.all())
		}
	})
}

func TestMaintenanceDisarmsAndArmsAgainAfterTheRemoval(t *testing.T) {
	h := newHarness(t, normal)
	h.step()
	h.wantArmed(true)

	h.node.maintenance(domain.FencingDisableAnnotation)
	h.step()

	h.wantArmed(false)
	h.wantReason(domain.ReasonMaintenance)
	h.wantMessage(domain.FencingDisableAnnotation)

	at := h.j.index("magicclose", 0)
	if at < 0 || h.j.index("meta armed=false maintenance=true", at) < 0 {
		t.Fatalf("journal = %v, want Magic Close, then armed=false and maintenance in NodeMeta", h.j.all())
	}

	// The agent does not arm while the annotation is present.
	h.step()
	h.wantArmed(false)

	h.node.maintenance()
	h.step()

	h.wantArmed(true)
	h.wantReason(domain.ReasonArmed)

	if h.j.index("meta armed=true maintenance=false", at) < 0 {
		t.Fatalf("journal = %v, want armed=true without maintenance in NodeMeta", h.j.all())
	}
}

func TestEachMaintenanceAnnotationDisarms(t *testing.T) {
	for _, annotation := range domain.MaintenanceAnnotations() {
		t.Run(annotation, func(t *testing.T) {
			h := newHarness(t, normal)
			h.step()

			h.node.maintenance(annotation)
			h.step()

			h.wantArmed(false)
			h.wantReason(domain.ReasonMaintenance)
		})
	}
}

func TestDisabledGroupDoesNotOpenTheDevice(t *testing.T) {
	h := newHarness(t, Params{Disabled: true, FeedInterval: time.Second})
	h.write("panic", "10\n")

	h.step()
	h.node.maintenance(domain.ApprovedAnnotation)
	h.step()

	h.wantReason(domain.ReasonGroupDisabled)
	h.wantArmed(false)

	if h.opens != 0 {
		t.Fatal("the agent of a Disabled group opened the device")
	}

	// The agent of a Disabled group publishes maintenance in NodeMeta too.
	if h.j.index("meta armed=false maintenance=true", 0) < 0 {
		t.Fatalf("journal = %v, want maintenance in NodeMeta", h.j.all())
	}
}

func TestSwitchToDisabledDisarms(t *testing.T) {
	h := newHarness(t, normal)
	h.step()

	h.m.Apply(Params{Disabled: true, FeedInterval: time.Second})
	h.step()

	h.wantArmed(false)
	h.wantReason(domain.ReasonGroupDisabled)

	if h.j.count("magicclose") != 1 {
		t.Fatalf("journal = %v, want one Magic Close", h.j.all())
	}
}

func TestSlowToCriticalSetsTheTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, slow)
		h.m.Start()
		h.run()
		synctest.Wait()

		from := len(h.j.all())
		h.m.Apply(critical)
		time.Sleep(slow.FeedInterval)
		synctest.Wait()

		if h.j.index("settimeout 3s", from) < 0 {
			t.Fatalf("journal = %v, want WDIOC_SETTIMEOUT 3 s", h.j.all()[from:])
		}

		h.wantTimeout(t, 3*time.Second)
		h.wantReason(domain.ReasonArmed)

		// The new feed interval is in use.
		before := h.j.count("keepalive")
		time.Sleep(critical.FeedInterval * 4)
		synctest.Wait()

		if got := h.j.count("keepalive") - before; got != 4 {
			t.Fatalf("feeds in 1 s after the switch = %d, want 4", got)
		}
	})
}

func TestCriticalToSlowSetsTheTimeout(t *testing.T) {
	h := newHarness(t, critical)
	h.step()

	from := len(h.j.all())
	h.m.Apply(slow)
	h.step()

	if h.j.index("settimeout 30s", from) < 0 {
		t.Fatalf("journal = %v, want WDIOC_SETTIMEOUT 30 s", h.j.all()[from:])
	}

	h.wantTimeout(t, 30*time.Second)
}

func TestNewTimeoutMismatchDisarms(t *testing.T) {
	h := newHarness(t, slow)
	h.step()

	h.device.accepted = func(time.Duration) time.Duration { return 30 * time.Second }
	h.m.Apply(critical)
	h.step()

	h.wantArmed(false)
	h.wantReason(domain.ReasonWatchdogTimeoutMismatch)

	if h.j.count("magicclose") != 1 {
		t.Fatalf("journal = %v, want one Magic Close", h.j.all())
	}
}

func TestDisarmDoesMagicClose(t *testing.T) {
	h := newHarness(t, critical)
	h.step()

	h.m.Disarm()

	h.wantArmed(false)

	// Disarm does not open the device that the agent holds.
	if h.j.count("magicclose") != 1 || h.j.count("open EBUSY") != 0 {
		t.Fatalf("journal = %v, want one Magic Close and no second open", h.j.all())
	}

	// No new arm after the planned stop.
	h.step()
	h.wantArmed(false)
}

func TestRunDoesNotPublishAfterDisarm(t *testing.T) {
	// SIGTERM can come before the first publication of Run. The condition
	// AgentStopping of the agent must stay the last.
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, critical)
		h.m.Start()
		h.m.Disarm()

		from := len(h.j.all())

		h.run()
		time.Sleep(time.Second)
		synctest.Wait()

		if got := h.j.all()[from:]; len(got) != 0 {
			t.Fatalf("published after Disarm: %v", got)
		}
	})
}

func TestEndOfRunDoesNotDisarm(t *testing.T) {
	// A crash of the agent cancels the context of Run. The watchdog stays armed.
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, critical)
		h.m.Start()

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})

		go func() {
			defer close(done)

			h.m.Run(ctx)
		}()

		time.Sleep(time.Second)
		cancel()
		<-done

		h.wantArmed(true)

		if h.j.count("magicclose") != 0 {
			t.Fatalf("journal = %v, want no Magic Close", h.j.all())
		}
	})
}

// The first decision of the engine comes after the step of Start. Run does a
// step at once and does not wait for the feed interval.
func TestRunArmsAtTheFirstDecision(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, slow)
		h.decided = make(chan struct{})
		h.m.deps.Decided = h.decided
		h.shouldFeed = false

		h.m.Start()
		h.run()
		synctest.Wait()
		h.wantArmed(false)

		h.setBool(&h.shouldFeed, true)
		close(h.decided)
		synctest.Wait()

		h.wantArmed(true)
		h.wantReason(domain.ReasonArmed)
	})
}

// newResumeHarness has an active device that the previous process left armed.
// The engine has not decided yet.
func newResumeHarness(t *testing.T, p Params) *harness {
	h := newHarness(t, p)
	h.decided = make(chan struct{})
	h.m.deps.Decided = h.decided
	h.active = true
	h.shouldFeed = false

	return h
}

// resume runs Resume until the end of the test. visible gives the result of
// each read of the control plane. The returned channel is closed when Resume
// returns.
func (h *harness) resume(visible *atomic.Bool) <-chan struct{} {
	ctx, cancel := context.WithCancel(h.t.Context())
	done := make(chan struct{})

	go func() {
		defer close(done)

		h.m.Resume(ctx, func(context.Context) bool { return visible.Load() })
	}()

	h.t.Cleanup(func() {
		cancel()
		<-done
	})

	return done
}

func returned(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}

func TestResumeDisarmsTheActiveDeviceOnAVisibleControlPlane(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newResumeHarness(t, critical)

		var visible atomic.Bool
		visible.Store(true)
		done := h.resume(&visible)
		synctest.Wait()

		// Magic Close at the first read, without a feed. Then the condition
		// tells that the agent is not armed.
		want := []string{"open", "magicclose", "condition WaitingForControlPlane"}
		if got := h.j.all(); !slices.Equal(got, want) {
			t.Fatalf("journal = %v, want %v", got, want)
		}

		if !returned(done) || h.get(&h.active) {
			t.Fatal("want an inactive device and the end of Resume after Magic Close")
		}

		h.wantArmed(false)

		// The first decision: the step arms the device as usual.
		h.setBool(&h.shouldFeed, true)
		close(h.decided)
		h.step()

		h.wantArmed(true)
		h.wantReason(domain.ReasonArmed)
	})
}

func TestResumeDoesNotTouchTheDeviceWithoutTheControlPlane(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newResumeHarness(t, critical)

		var visible atomic.Bool
		done := h.resume(&visible)

		time.Sleep(time.Second)
		synctest.Wait()

		// The Settings come. Before the first decision the step does not arm.
		h.step()

		// The device stays active: the watchdog expires if nothing changes.
		if h.j.count("open") != 0 || h.j.count("keepalive") != 0 || h.j.count("magicclose") != 0 || !h.get(&h.active) {
			t.Fatalf("journal = %v, want no open of the active device", h.j.all())
		}

		// The control plane is visible again before the watchdog expires.
		visible.Store(true)
		time.Sleep(resumeInterval)
		synctest.Wait()

		if h.j.count("magicclose") != 1 || h.j.count("keepalive") != 0 || !returned(done) {
			t.Fatalf("journal = %v, want Magic Close without a feed", h.j.all())
		}
	})
}

func TestResumeAndDisarmDoNotOpenAnInactiveDevice(t *testing.T) {
	h := newResumeHarness(t, critical)
	h.active = false

	reads := 0
	h.m.Resume(t.Context(), func(context.Context) bool { reads++; return true })
	h.m.Disarm()

	if got := h.j.all(); len(got) != 0 || reads != 0 {
		t.Fatalf("journal = %v, reads = %d, want no open and no read of the control plane", got, reads)
	}
}

func TestDisarmDuringTheResumeWaitDoesMagicClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newResumeHarness(t, critical)

		var visible atomic.Bool
		done := h.resume(&visible)
		synctest.Wait()

		h.m.Disarm()

		want := []string{"open", "magicclose"}
		if got := h.j.all(); !slices.Equal(got, want) {
			t.Fatalf("journal = %v, want %v", got, want)
		}

		visible.Store(true)
		time.Sleep(resumeInterval)
		synctest.Wait()

		if !returned(done) || !slices.Equal(h.j.all(), want) {
			t.Fatalf("journal = %v, want the end of Resume without a second open", h.j.all())
		}

		h.wantArmed(false)
	})
}

func TestResumeAfterDisarmDoesNothing(t *testing.T) {
	h := newResumeHarness(t, critical)
	h.m.Disarm()

	h.m.Resume(t.Context(), func(context.Context) bool { return true })

	want := []string{"open", "magicclose"}
	if got := h.j.all(); !slices.Equal(got, want) {
		t.Fatalf("journal = %v, want %v", got, want)
	}
}

func TestResumeEndsAtTheFirstDecision(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newResumeHarness(t, critical)

		var visible atomic.Bool
		done := h.resume(&visible)

		close(h.decided)
		time.Sleep(resumeInterval)
		synctest.Wait()

		// After the first decision the steps own the device.
		visible.Store(true)
		time.Sleep(time.Second)
		synctest.Wait()

		if !returned(done) || h.j.count("open") != 0 || h.j.count("magicclose") != 0 {
			t.Fatalf("journal = %v, want the end of Resume without an open", h.j.all())
		}
	})
}

// The step arms the active device before Resume sees the control plane. Resume
// stops and does not disarm the armed device.
func TestResumeEndsAtTheArm(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newResumeHarness(t, critical)

		var visible atomic.Bool
		done := h.resume(&visible)
		synctest.Wait()

		// The engine has a fresh decision, but the channel Decided stays open in
		// the test: only the arm ends the wait.
		h.setBool(&h.shouldFeed, true)
		h.step()
		h.wantArmed(true)

		visible.Store(true)
		time.Sleep(time.Second)
		synctest.Wait()

		if !returned(done) || h.j.count("magicclose") != 0 || h.j.count("open EBUSY") != 0 {
			t.Fatalf("journal = %v, want the end of Resume without a second open and Magic Close", h.j.all())
		}

		h.wantArmed(true)
	})
}

func TestResumeIsSilentWithoutTheDevice(t *testing.T) {
	h := newHarness(t, Params{Disabled: true, FeedInterval: time.Second})

	var out bytes.Buffer

	h.m.logger = log.NewLogger(log.WithOutput(&out))
	h.m.deps.Active = func() (bool, error) { return false, fmt.Errorf("stat %q: %w", "/dev/watchdog", fs.ErrNotExist) }

	reads := 0
	visible := func(context.Context) bool { reads++; return true }

	h.m.Resume(t.Context(), visible)

	if out.Len() != 0 || reads != 0 || len(h.j.all()) != 0 {
		t.Fatalf("log = %q, reads = %d, want no record and no read for a missing device", out.String(), reads)
	}

	h.m.deps.Active = func() (bool, error) { return false, errors.New("sysfs is not readable") }

	h.m.Resume(t.Context(), visible)

	if out.Len() == 0 || reads != 0 || len(h.j.all()) != 0 {
		t.Fatal("a read error of the state must stay in the log, and the device must stay closed")
	}
}

func TestConditionChangeIsPublishedOnce(t *testing.T) {
	h := newHarness(t, critical)

	var out bytes.Buffer

	h.m.logger = log.NewLogger(log.WithOutput(&out))

	h.step()
	h.step()

	if got := h.j.count("condition Armed"); got != 1 {
		t.Fatalf("published %d times, want 1; journal %v", got, h.j.all())
	}

	// The status writer logs the condition. The manager does not log it again.
	if strings.Contains(out.String(), "fencing condition") {
		t.Fatalf("log = %q, the manager must not log the condition", out.String())
	}
}

// StopFeed stops the feed of the armed device at once, without a step, and
// tells whether the device is armed.
func TestStopFeedStopsAtOnce(t *testing.T) {
	h := newHarness(t, critical)

	if h.m.StopFeed() {
		t.Fatal("StopFeed = true before the arm")
	}

	h.step()
	h.step()

	if !h.m.StopFeed() || !h.m.Stopped() {
		t.Fatalf("StopFeed = false or Stopped = false on the armed device; journal %v", h.j.all())
	}

	if h.j.count("event WatchdogFeedStopped") != 1 {
		t.Fatalf("journal = %v, want the event of the stop at StopFeed", h.j.all())
	}

	// The engine still lets the step feed: the stop is final.
	for range 5 {
		h.step()
	}

	if got := h.j.count("keepalive"); got != 1 {
		t.Fatalf("feeds = %d, want 1 (only before the stop)", got)
	}
}

// The own FencingFailedNodeState in the phase Proven or a later phase stops
// the feed of the armed device at once and sets the watchdog timeout to 1 s.
// An empty phase later does not take this back.
func TestProvenStopsTheFeedAndSetsOneSecond(t *testing.T) {
	h := newHarness(t, critical)
	h.step()
	h.step()

	h.m.SetProven("Proven")

	if !h.m.Stopped() || h.j.count("settimeout 1s") != 1 || h.j.count("event WatchdogFeedStopped") != 1 {
		t.Fatalf("journal = %v, want the stop and WDIOC_SETTIMEOUT 1s at once", h.j.all())
	}

	msg := "event message Watchdog feed stopped: the own FencingFailedNodeState is in the phase Proven. The node stops when the watchdog timeout 1s expires"
	if h.j.index(msg, 0) < h.j.index("settimeout 1s", 0) {
		t.Fatalf("journal = %v, want the event with the cause and the timeout 1s after WDIOC_SETTIMEOUT", h.j.all())
	}

	h.wantTimeout(t, time.Second)

	h.m.SetProven("")
	for range 5 {
		h.step()
	}

	if got := h.j.count("keepalive"); got != 1 {
		t.Fatalf("feeds = %d, want 1 (only before the stop)", got)
	}

	h.m.Disarm()

	if h.j.count("magicclose") != 0 {
		t.Fatalf("journal = %v, want no Magic Close after the stop", h.j.all())
	}
}

// After the stop by the engine, Proven still sets the watchdog timeout to 1 s.
func TestProvenAfterTheStopOfTheEngineSetsOneSecond(t *testing.T) {
	h := newHarness(t, slow)
	h.step()
	h.step()

	h.m.StopFeed()
	h.m.SetProven("Proven")

	if h.j.count("settimeout 1s") != 1 || h.j.count("event WatchdogFeedStopped") != 1 {
		t.Fatalf("journal = %v, want one stop and WDIOC_SETTIMEOUT 1s", h.j.all())
	}
}

// An agent without the armed watchdog does not stop and does not arm while
// its FencingFailedNodeState is proven. It arms after the phase changes.
func TestProvenDoesNotStopOrArmAnUnarmedAgent(t *testing.T) {
	h := newHarness(t, critical)

	h.m.SetProven("Proven")
	h.step()
	h.step()

	if h.opens != 0 || h.m.Stopped() || h.j.count("settimeout 1s") != 0 {
		t.Fatalf("journal = %v, want no arm and no stop", h.j.all())
	}

	h.wantReason(domain.ReasonWaitingForControlPlane)

	h.m.SetProven("")
	h.step()
	h.step()

	h.wantArmed(true)

	if h.m.Stopped() || h.j.count("keepalive") != 1 {
		t.Fatalf("journal = %v, want the normal feed", h.j.all())
	}
}

// Close tells whether the watchdog is armed and whether the feed is stopped.
// It does not disarm. Disarm after
// Close does Magic Close only when the feed is not stopped.
func TestCloseTellsTheStopOfTheFeed(t *testing.T) {
	tests := []struct {
		name    string
		stop    func(h *harness)
		stopped bool
	}{
		{name: "the feed goes on", stop: func(*harness) {}},
		{name: "the engine stops the feed before the next step", stop: func(h *harness) { h.stopEngine() }, stopped: true},
		{name: "the own FencingFailedNodeState is proven", stop: func(h *harness) { h.m.SetProven("Proven") }, stopped: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, critical)
			h.step()
			h.step()

			tt.stop(h)

			if armed, stopped := h.m.Close(); !armed || stopped != tt.stopped {
				t.Fatalf("Close = %t, %t, want true, %t", armed, stopped, tt.stopped)
			}

			// No feed and no Magic Close after Close.
			h.step()

			if h.j.count("keepalive") != 1 || h.j.count("magicclose") != 0 {
				t.Fatalf("journal = %v, want one feed before Close and no Magic Close", h.j.all())
			}

			if got := h.m.Disarm(); got == tt.stopped {
				t.Fatalf("Disarm = %t, want %t", got, !tt.stopped)
			}

			want := 1
			if tt.stopped {
				want = 0
			}

			if got := h.j.count("magicclose"); got != want {
				t.Fatalf("journal = %v, want %d Magic Close", h.j.all(), want)
			}
		})
	}
}

// An agent that does not hold the device and has no active device of the
// previous process is not armed.
func TestCloseOfAnUnarmedAgent(t *testing.T) {
	h := newResumeHarness(t, critical)
	h.active = false

	if armed, stopped := h.m.Close(); armed || stopped {
		t.Fatalf("Close = %t, %t, want false, false", armed, stopped)
	}
}

// A stale bind mount of the device in the pod gives "permission denied" on
// each open. After staleDeviceDenials such refusals in sequence the manager
// asks to stop the agent: the restart of the container makes a new bind mount.
// A successful open resets the count. A missing device does not count.
func TestStaleDeviceStopsTheAgent(t *testing.T) {
	denied := fmt.Errorf("open watchdog device %q: %w", "/dev/watchdog", fs.ErrPermission)
	missing := fmt.Errorf("open watchdog device %q: %w", "/dev/watchdog", fs.ErrNotExist)

	for _, tt := range []struct {
		name  string
		opens []error
		want  int
	}{
		{"three refusals", []error{denied, denied, denied}, 1},
		{"a refusal, then a successful open", []error{denied, denied, nil, denied, denied}, 0},
		{"a missing device", []error{missing, missing, missing, missing}, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, critical)

			var fails []error

			h.m.deps.Fail = func(err error) { fails = append(fails, err) }

			for _, err := range tt.opens {
				h.mu.Lock()
				h.openErr = err
				h.mu.Unlock()
				h.m.mu.Lock()
				h.m.retryAt = time.Time{}
				h.m.mu.Unlock()
				h.step()

				if err == nil {
					// Disarm, so that the next step opens the device again.
					h.m.mu.Lock()
					h.m.magicClose("test")
					h.m.mu.Unlock()
				}
			}

			if len(fails) != tt.want {
				t.Fatalf("stop requests = %d (%v), want %d; journal %v", len(fails), fails, tt.want, h.j.all())
			}
		})
	}
}
