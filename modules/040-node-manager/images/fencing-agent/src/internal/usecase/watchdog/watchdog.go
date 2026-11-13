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

// Package watchdog arms, feeds and disarms the watchdog of the node and sets
// the reason of the Fencing condition.
package watchdog

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

// Device is the watchdog device. The adapter in internal/adapters/watchdog
// implements it.
type Device interface {
	Identity() string
	SetTimeoutSupported() bool
	MagicCloseSupported() bool
	KeepAlive() error
	SetTimeout(timeout time.Duration) error
	GetTimeout() (time.Duration, error)
	MagicClose() error
}

// StateSource gives the last state of the own Node. It returns false until the
// Node is observed.
type StateSource interface {
	Snapshot() (domain.NodeSignals, bool)
}

// EventRecorder writes Kubernetes Events on the own Node.
type EventRecorder interface {
	Warning(reason, message string)
}

const (
	// CheckInterval is the period of the kernel.panic check and of a new
	// attempt to arm after a device failure.
	CheckInterval = 10 * time.Second

	softPanicArg = "softdog.soft_panic=1"

	// resumeInterval is the period of the control plane reads before the
	// first decision of the engine.
	resumeInterval = 200 * time.Millisecond

	// provenTimeout is the watchdog timeout after the stop by the own
	// FencingFailedNodeState in the phase Proven or a later phase.
	provenTimeout = time.Second

	// engineStop is the cause of the stop of the feed by the decision engine.
	engineStop = "the decision engine stopped it"

	// staleDeviceDenials is the number of the opens in sequence that fail with
	// "permission denied" before the manager stops the agent.
	staleDeviceDenials = 3
)

// Params come from the profile of the node group.
type Params struct {
	// Disabled is true for a node group in the mode Disabled.
	Disabled bool
	// Timeout is watchdog.timeout of the profile.
	Timeout      time.Duration
	FeedInterval time.Duration
	// Engine is true on a Deckhouse Engine node.
	Engine bool
}

type Deps struct {
	// Open opens the device. Open starts the timer.
	Open func() (Device, error)
	// Nowayout tells whether the kernel ignores Magic Close. It does not open the device.
	Nowayout func() (bool, error)
	// Active tells whether the timer of the device runs. It does not open the device.
	Active func() (bool, error)
	Node   StateSource
	// HasPeer tells whether another node has an agent.
	HasPeer func() bool
	// ShouldFeed tells whether a fresh decision lets the agent feed now.
	ShouldFeed func() bool
	// Decided is closed after the first decision of the engine.
	Decided <-chan struct{}
	Status  domain.Status
	Events  EventRecorder
	// UpdateMeta changes the own NodeMeta and publishes it.
	UpdateMeta func(update func(*domain.NodeMeta))
	// OnArmed gets each change of Armed. It can be nil.
	OnArmed func(armed bool)
	// KernelPanicPath and CmdlinePath are /proc/sys/kernel/panic and /proc/cmdline when empty.
	KernelPanicPath string
	CmdlinePath     string
	// DevicePath is the path of the watchdog device in the pod.
	DevicePath string
	// Fail stops the agent with err. The restart of the container makes a
	// new bind mount of the device in the pod.
	Fail func(err error)
}

// Manager owns the watchdog device. All methods are safe for concurrent use.
type Manager struct {
	deps   Deps
	logger *log.Logger

	mu     sync.Mutex
	params Params
	device Device
	// timeout is the timeout of the armed device.
	timeout time.Duration
	// stopped is set when the feed of the armed device stops. ResumeFeed
	// resets it until final is set.
	stopped bool
	// resumed is set by ResumeFeed. The first feed after it writes the log
	// line about the resumed feed and resets it.
	resumed bool
	// final is set by Finalize and by the phase Proven. After it the stop
	// does not reverse.
	final bool
	// provenPhase is the phase of the own FencingFailedNodeState while it is
	// Proven or a later phase, else empty.
	provenPhase string
	// closed is set by Close and Disarm. After it the steps and publish do nothing.
	closed bool
	// startDecided is set when the step of Start has the first decision.
	startDecided bool

	kernelCheckedAt time.Time
	kernelMismatch  string
	retryAt         time.Time
	// denials counts the opens in sequence that fail with "permission denied".
	denials int

	reason, publishedReason   domain.Reason
	message, publishedMessage string
	meta, publishedMeta       nodeMeta

	armed atomic.Bool
}

// nodeMeta holds the NodeMeta fields that the manager owns.
type nodeMeta struct {
	armed       bool
	maintenance bool
}

func New(deps Deps, logger *log.Logger) *Manager {
	if deps.KernelPanicPath == "" {
		deps.KernelPanicPath = "/proc/sys/kernel/panic"
	}

	if deps.CmdlinePath == "" {
		deps.CmdlinePath = "/proc/cmdline"
	}

	return &Manager{deps: deps, logger: logger}
}

// Apply sets new params. The next step applies them: it disarms the device for
// the mode Disabled and sets the new timeout on the armed device.
func (m *Manager) Apply(p Params) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.params.Disabled != p.Disabled || m.params.Engine != p.Engine {
		m.kernelCheckedAt = time.Time{}
	}

	m.params = p
	m.retryAt = time.Time{}
}

// Start does the first step and does not publish. Call Apply before Start.
func (m *Manager) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.startDecided = m.decided()
	m.step()
}

// Run publishes the state of Start and then does one step in each feed
// interval until ctx ends. It also does a step at the first decision of the
// engine. The end of ctx does not disarm the watchdog.
func (m *Manager) Run(ctx context.Context) {
	decided := m.deps.Decided

	m.mu.Lock()
	m.publish()

	if m.startDecided {
		decided = nil
	}
	m.mu.Unlock()

	timer := time.NewTimer(m.interval())
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-decided:
			// The first decision can let the agent arm. Do not wait for the next interval.
			decided = nil
		case <-timer.C:
		}

		m.mu.Lock()
		m.step()
		m.publish()
		m.mu.Unlock()

		timer.Reset(m.interval())
	}
}

// Resume disarms the device that the previous process left armed: that
// process stopped without Magic Close. Resume reads the control plane in each
// resumeInterval. When visible is true, Resume does Magic Close, and the steps
// arm the device as usual. When visible is false, Resume does not touch the
// device, and the watchdog expires. The wait stops at the arm, at the first
// decision of the engine and at Disarm. Resume does not open a device that is
// not active. Call Resume before Start.
func (m *Manager) Resume(ctx context.Context, visible func(ctx context.Context) bool) {
	if !m.active() {
		return
	}

	ticker := time.NewTicker(resumeInterval)
	defer ticker.Stop()

	for {
		ok := visible(ctx)

		m.mu.Lock()
		waits := !m.closed && m.device == nil && !m.decided()

		if waits && ok && m.closeActive("the control plane is visible at the agent start") {
			m.set(domain.ReasonWaitingForControlPlane, "the watchdog that the previous agent process left armed is disarmed")
			m.publish()
		}
		m.mu.Unlock()

		if !waits || ok {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Close stops the steps of the manager at the planned stop of the agent. It
// tells whether the watchdog is armed: the agent holds the device, or the
// device of the previous process is active. It also tells whether the feed of
// the armed device is stopped. After Close the stop does not reverse: the
// watchdog stays armed. A decision that is not fresh is not a stop.
func (m *Manager) Close() (bool, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	stopped := m.close()

	return m.device != nil || m.active(), stopped
}

// Disarm does Magic Close on the planned stop of the agent, also for the
// active device of the previous process. After the stop of the feed it does
// not disarm and returns false.
func (m *Manager) Disarm() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.close() {
		return false
	}

	if m.device == nil {
		m.closeActive("the agent stops")
	}

	m.magicClose("the agent stops")
	m.publishMeta()

	return true
}

// close stops the steps and tells whether the feed is stopped. The caller
// holds mu.
func (m *Manager) close() bool {
	m.closed = true

	return m.stopped
}

// StopFeed stops the feed of the armed device. It tells whether the device is
// armed. After StopFeed returns, the device gets no feed until ResumeFeed.
func (m *Manager) StopFeed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.device == nil {
		return false
	}

	if !m.stopped {
		m.stopFeed(engineStop)
	}

	return true
}

// ResumeFeed lets the steps feed again after StopFeed. The first feed comes at
// the next step and writes the log line about the resumed feed. ResumeFeed
// tells whether the feed may go on. It does not resume after Finalize, after
// the phase Proven and after Close.
func (m *Manager) ResumeFeed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.final || m.closed {
		return false
	}

	if m.stopped {
		m.stopped, m.resumed = false, true
		m.set(domain.ReasonArmed, "")
	}

	return true
}

// Finalize makes the stop of the feed final.
func (m *Manager) Finalize() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.final = true
}

// Final tells whether the stop of the feed does not reverse.
func (m *Manager) Final() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.final
}

// SetProven takes the phase of the own FencingFailedNodeState when it is
// Proven or a later phase, else an empty phase. With such a phase the manager
// stops the feed of the armed device for good and sets the watchdog timeout
// to 1 s at once. It does not arm the device while the phase is set.
func (m *Manager) SetProven(phase string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.provenPhase = phase
	m.expire()
}

// Stopped tells whether the feed of the armed device is stopped.
func (m *Manager) Stopped() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.stopped
}

// Armed tells whether the agent holds the armed device. It stays true after
// the stop of the feed.
func (m *Manager) Armed() bool {
	return m.armed.Load()
}

// Reason returns the reason of the Fencing condition. It is empty before Start.
func (m *Manager) Reason() domain.Reason {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.reason
}

// decided tells whether the engine has decided.
func (m *Manager) decided() bool {
	select {
	case <-m.deps.Decided:
		return true
	default:
		return false
	}
}

func (m *Manager) interval() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.params.FeedInterval <= 0 {
		return time.Second
	}

	return m.params.FeedInterval
}

// step does one pass of the policy. The caller holds mu.
func (m *Manager) step() {
	m.expire()

	if m.closed || m.stopped {
		return
	}

	node, _ := m.deps.Node.Snapshot()

	m.meta.maintenance = node.Maintenance
	m.checkKernel()

	switch {
	case m.params.Disabled:
		m.magicClose("fencing is disabled for the node group")
		m.set(domain.ReasonGroupDisabled, "")
	case node.Maintenance:
		m.magicClose("node maintenance")
		m.set(domain.ReasonMaintenance, "maintenance annotations: "+strings.Join(node.MaintenanceReasons, ", "))
	case m.kernelMismatch != "":
		m.magicClose("kernel settings mismatch")
		m.set(domain.ReasonKernelPanicMismatch, m.kernelMismatch)
	case m.device != nil:
		// Without a fresh decision the step does not feed: the watchdog
		// expires if no fresh decision comes in the watchdog timeout.
		if m.deps.ShouldFeed() {
			m.feed()
		}
	default:
		m.arm()
	}
}

// checkKernel reads the kernel settings once in CheckInterval. The caller holds mu.
func (m *Manager) checkKernel() {
	if m.params.Disabled || time.Since(m.kernelCheckedAt) < CheckInterval {
		return
	}

	m.kernelCheckedAt = time.Now()
	m.kernelMismatch = m.readKernel()
}

// readKernel returns an empty string when the kernel settings are correct.
// The agent does not change them.
func (m *Manager) readKernel() string {
	raw, err := os.ReadFile(m.deps.KernelPanicPath)
	if err != nil {
		return fmt.Sprintf("cannot read %s: %v", m.deps.KernelPanicPath, err)
	}

	if value := strings.TrimSpace(string(raw)); value != "0" {
		return fmt.Sprintf("kernel.panic is %s, expected 0", value)
	}

	if !m.params.Engine {
		return ""
	}

	raw, err = os.ReadFile(m.deps.CmdlinePath)
	if err != nil {
		return fmt.Sprintf("cannot read %s: %v", m.deps.CmdlinePath, err)
	}

	if !slices.Contains(strings.Fields(string(raw)), softPanicArg) {
		return softPanicArg + " is not on the kernel command line: softdog would reboot the node, not stop it"
	}

	return ""
}

// stopFeed stops the feed of the armed device. why is the cause of the stop.
// The caller holds mu.
func (m *Manager) stopFeed(why string) {
	m.stopped = true
	msg := fmt.Sprintf("Watchdog feed stopped: %s. The node stops when the watchdog timeout %s expires", why, m.timeout)
	if !m.final {
		msg += ", unless the feed resumes before it"
	}

	// Plain Log does not add a stack trace, which has no value for this expected stop.
	m.logger.Log(context.Background(), log.LevelError.Level(), msg)
	m.deps.Events.Warning("WatchdogFeedStopped", msg)
	m.set(domain.ReasonArmed, "the feed is stopped")
}

// expire stops the feed of the armed device for good and sets the watchdog
// timeout to provenTimeout while proven is true. The caller holds mu.
func (m *Manager) expire() {
	if m.provenPhase == "" || m.device == nil {
		return
	}

	why := "the own FencingFailedNodeState is in the phase " + m.provenPhase

	if m.timeout != provenTimeout {
		// WDIOC_SETTIMEOUT also restarts the timer: the node stops in 1 s.
		if err := m.device.SetTimeout(provenTimeout); err != nil {
			m.logger.Error("set the watchdog timeout", "reason", why, "error", err)
		} else {
			if m.stopped {
				m.logger.Warn("watchdog timeout set", "reason", why, "from", m.timeout.String(), "to", provenTimeout.String())
			}

			m.timeout = provenTimeout
		}
	}

	m.final = true

	if !m.stopped {
		m.stopFeed(why)
	}
}

// feed feeds the armed device. step calls it only on a fresh decision. The
// caller holds mu.
func (m *Manager) feed() {
	if m.timeout != m.params.Timeout && !m.setTimeout() {
		return
	}

	if err := m.device.KeepAlive(); err != nil {
		m.logger.Error("feed watchdog", "error", err)
	} else if m.resumed {
		m.resumed = false
		m.logger.Warn("Watchdog feed resumed: the device got the first feed after the stop by the decision engine")
	}

	m.set(domain.ReasonArmed, "")
}

// setTimeout applies the new watchdog timeout to the armed device. The caller
// holds mu.
func (m *Manager) setTimeout() bool {
	want := m.params.Timeout

	if err := confirmTimeout(m.device, want); err != nil {
		m.magicClose("watchdog timeout mismatch")
		m.set(domain.ReasonWatchdogTimeoutMismatch, err.Error())
		m.retryAt = time.Now().Add(CheckInterval)

		return false
	}

	m.logger.Info("watchdog timeout changed", "from", m.timeout.String(), "to", want.String())
	m.timeout = want

	return true
}

// arm opens the device when all arm conditions are true. Open and
// WDIOC_SETTIMEOUT restart the timer, so the agent arms only on a fresh
// decision to feed. The caller holds mu.
func (m *Manager) arm() {
	// A fresh decision to feed also means that the last read saw the control
	// plane: the engine stops the feed at the loss of the control plane.
	switch {
	case m.provenPhase != "":
		m.set(domain.ReasonWaitingForControlPlane, "the own FencingFailedNodeState is in the phase "+m.provenPhase)

		return
	case !m.deps.HasPeer():
		m.set(domain.ReasonNoEvacuationTarget, "no other node has a fencing agent")

		return
	case time.Now().Before(m.retryAt):
		return
	case !m.deps.ShouldFeed():
		m.set(domain.ReasonWaitingForControlPlane, "no fresh decision to feed the watchdog")

		return
	}

	if err := m.open(); err != nil {
		if deviceMissing(err) {
			// The node setup gives the device later. The next try finds it.
			m.set(domain.ReasonWatchdogDeviceMissing, fmt.Sprintf("the watchdog device %s is not available in the pod: %v", m.deps.DevicePath, err))
		} else {
			m.set(domain.ReasonWatchdogTimeoutMismatch, err.Error())
		}

		m.retryAt = time.Now().Add(CheckInterval)
		m.countDenial(err)

		return
	}

	m.denials = 0
	m.logger.Info("watchdog armed", "identity", m.device.Identity(), "timeout", m.timeout.String())
	m.set(domain.ReasonArmed, "")
}

// countDenial counts an open that fails with "permission denied". Such opens
// in sequence come from a stale bind mount of the device in the pod: the
// kernel module softdog was loaded again, and the host has a new device. The
// mount stays stale until the container restarts, so after
// staleDeviceDenials of them the manager stops the agent. The device is not
// armed: the open failed. The caller holds mu.
func (m *Manager) countDenial(err error) {
	if !errors.Is(err, fs.ErrPermission) {
		m.denials = 0

		return
	}

	m.denials++
	if m.denials < staleDeviceDenials || m.device != nil || m.deps.Fail == nil {
		return
	}

	m.logger.Error("the watchdog device in the pod is stale: the agent stops, and the restart of the container gives a new device",
		"device", m.deps.DevicePath, "denials", m.denials, "error", err)
	m.deps.Fail(fmt.Errorf("the watchdog device %s in the pod is stale: %w", m.deps.DevicePath, err))
}

// open opens the device and confirms the watchdog timeout. After a refusal it closes the
// device with Magic Close. The caller holds mu.
func (m *Manager) open() error {
	nowayout, err := m.deps.Nowayout()
	if deviceMissing(err) {
		return err
	}

	if err != nil {
		return fmt.Errorf("cannot read the nowayout setting of the watchdog: %w", err)
	}

	if nowayout {
		return errors.New("the watchdog driver has nowayout: Magic Close cannot disarm it")
	}

	device, err := m.deps.Open()
	if err != nil {
		return err
	}

	var refusal error

	switch {
	case !device.SetTimeoutSupported():
		refusal = fmt.Errorf("the watchdog driver %q has no WDIOF_SETTIMEOUT", device.Identity())
	case !device.MagicCloseSupported():
		refusal = fmt.Errorf("the watchdog driver %q has no WDIOF_MAGICCLOSE", device.Identity())
	default:
		refusal = confirmTimeout(device, m.params.Timeout)
	}

	if refusal != nil {
		if err := device.MagicClose(); err != nil {
			m.logger.Error("magic close after a refusal to arm", "error", err)
		}

		return refusal
	}

	m.device = device
	m.timeout = m.params.Timeout
	m.setArmed(true)
	m.meta.armed = true

	return nil
}

// deviceMissing tells whether the error is a missing device node or a device
// node without a loaded driver.
func deviceMissing(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENODEV) || errors.Is(err, syscall.ENXIO)
}

// confirmTimeout sets the timeout and reads it back.
func confirmTimeout(device Device, want time.Duration) error {
	if err := device.SetTimeout(want); err != nil {
		return err
	}

	got, err := device.GetTimeout()
	if err != nil {
		return err
	}

	if got != want {
		return fmt.Errorf("WDIOC_GETTIMEOUT returned %s after WDIOC_SETTIMEOUT %s", got, want)
	}

	return nil
}

// active tells whether the timer of the device runs. A Disabled group has no
// watchdog device: that is not an error.
func (m *Manager) active() bool {
	active, err := m.deps.Active()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		m.logger.Info("cannot read the watchdog state", "error", err)
	}

	return err == nil && active
}

// closeActive does Magic Close on the active device that the agent does not
// hold. It does not open a device that is not active: the open starts the
// timer, and with nowayout Magic Close does not stop it. It returns true after
// Magic Close. The caller holds mu, so the open does not collide with arm.
func (m *Manager) closeActive(why string) bool {
	if !m.active() {
		return false
	}

	device, err := m.deps.Open()
	if err != nil {
		m.logger.Error("open the active watchdog", "error", err)

		return false
	}

	if err := device.MagicClose(); err != nil {
		m.logger.Error("magic close", "reason", why, "error", err)

		return false
	}

	m.logger.Info("watchdog disarmed", "reason", why)

	return true
}

// magicClose disarms the armed device. The caller holds mu.
func (m *Manager) magicClose(why string) {
	if m.device == nil {
		return
	}

	if err := m.device.MagicClose(); err != nil {
		m.logger.Error("magic close", "reason", why, "error", err)
	} else {
		m.logger.Info("watchdog disarmed", "reason", why)
	}

	m.device = nil
	m.timeout = 0
	m.resumed = false
	m.setArmed(false)
	m.meta.armed = false
}

func (m *Manager) setArmed(armed bool) {
	m.armed.Store(armed)

	if m.deps.OnArmed != nil {
		m.deps.OnArmed(armed)
	}
}

func (m *Manager) set(reason domain.Reason, message string) {
	m.reason, m.message = reason, message
}

// publish sends the changes of the condition and of NodeMeta. After Disarm the
// agent writes the last condition, so publish does nothing. The caller holds mu.
func (m *Manager) publish() {
	if m.closed {
		return
	}

	m.publishMeta()

	if m.reason == m.publishedReason && m.message == m.publishedMessage {
		return
	}

	m.publishedReason, m.publishedMessage = m.reason, m.message
	m.deps.Status.SetCondition(m.reason, m.message)
}

// publishMeta sends the changes of NodeMeta. The caller holds mu.
func (m *Manager) publishMeta() {
	if m.meta == m.publishedMeta {
		return
	}

	m.publishedMeta = m.meta
	meta := m.meta
	m.deps.UpdateMeta(func(nm *domain.NodeMeta) {
		nm.Armed, nm.Maintenance = meta.armed, meta.maintenance
	})
}
