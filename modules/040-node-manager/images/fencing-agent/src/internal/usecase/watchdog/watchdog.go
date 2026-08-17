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

// Package watchdog owns the watchdog policy of the agent: when the device is
// armed, how often it is fed, and when it is deliberately disarmed. The device
// itself is injected, so the policy is testable without a kernel.
package watchdog

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// Device is the watchdog device contract this policy needs; the adapter in
// internal/adapters/watchdog satisfies it.
type Device interface {
	Identity() string
	SetTimeoutSupported() bool
	MagicCloseSupported() bool
	KeepAlive() error
	SetTimeout(timeout time.Duration) (time.Duration, error)
	GetTimeout() (time.Duration, error)
	GetTimeLeft() (time.Duration, error)
	MagicClose() error
	ReleaseWithoutDisarm() error
}

// StateSource is the last observed state of the agent's own Node.
type StateSource interface {
	Snapshot() Snapshot
}

// EventRecorder publishes operator-facing state changes.
type EventRecorder interface {
	Normal(reason, message string)
	Warning(reason, message string)
}

// Kubernetes Event reasons.
const (
	reasonArmed           = "WatchdogArmed"
	reasonDisarmed        = "WatchdogDisarmed"
	reasonFeedFailed      = "WatchdogFeedFailed"
	reasonNotDisarmable   = "WatchdogNotDisarmable"
	reasonRefused         = "WatchdogRefused"
	reasonStarvation      = "WatchdogStarvation"
	reasonIdentityChanged = "NodeIdentityChanged"
)

// Feeding states, used to log and event a transition exactly once.
const (
	stateFeeding     = ""
	stateMaintenance = "Maintenance"
	stateRemoval     = "PlannedRemoval"
	stateGateClosed  = "FeedGateClosed"
)

const (
	// maxFeedFailures is how many consecutive keepalive failures turn the pod
	// NotReady. The device is reopened after every failure, so this counts real
	// trouble rather than a single hiccup.
	maxFeedFailures = 3

	// livenessGrace is the floor for the staleness window of the feed loop, so
	// fast profiles do not fail /healthz on ordinary scheduler jitter.
	livenessGrace = 5 * time.Second
)

// errFatal marks the failures the agent must not survive: the watchdog cannot be
// trusted to honour the SLA or to be disarmed, and a silently unfenced Node is
// worse than a CrashLoopBackOff.
var errFatal = errors.New("watchdog cannot be relied on")

type Params struct {
	FeedInterval time.Duration
	Timeout      time.Duration
}

type Deps struct {
	// Open arms the device.
	Open func() (Device, error)
	// Nowayout reports the kernel setting that makes Magic Close a no-op. It must
	// not open the device.
	Nowayout func() (bool, error)
	State    StateSource
	Events   EventRecorder
	// ShouldFeed is the quorum and fallback gate of the ADR. It is constant true
	// in this stage: the local quorum view and the fallback path do not exist yet,
	// so the watchdog only fences a Node whose agent died, hung or lost the
	// device. The seam keeps that decision out of the feed loop.
	ShouldFeed func() (bool, string)
}

type Manager struct {
	params Params
	deps   Deps
	logger *log.Logger

	// mu is held for a whole tick: the only other writer is Close on shutdown,
	// and readiness and liveness are answered from atomics instead.
	mu       sync.Mutex
	device   Device
	state    string
	detail   string
	failures int
	// cannotDisarm is set when the kernel would not honour Magic Close, either
	// because the driver does not advertise it or because a disarm already failed.
	// It is sticky: from then on planned operations keep feeding instead.
	cannotDisarm bool

	started  atomic.Bool
	ready    atomic.Bool
	lastTick atomic.Int64
}

func New(params Params, deps Deps, logger *log.Logger) *Manager {
	return &Manager{params: params, deps: deps, logger: logger}
}

// Run arms the watchdog and feeds it until ctx is cancelled. It must be called
// only after the agent joined the gossip network: an armed watchdog is a promise
// that this agent can see the cluster.
func (m *Manager) Run(ctx context.Context) error {
	if err := m.validate(); err != nil {
		m.deps.Events.Warning(reasonRefused, err.Error())

		return err
	}

	if err := m.checkNowayout(); err != nil {
		return err
	}

	m.touch()
	m.started.Store(true)

	m.logger.Info("watchdog feed loop started",
		"feed_interval", m.params.FeedInterval.String(),
		"timeout", m.params.Timeout.String(),
	)

	ticker := time.NewTicker(m.params.FeedInterval)
	defer ticker.Stop()

	// The first tick runs at once: the join has just finished and waiting a whole
	// interval would leave the Node unprotected for no reason.
	if err := m.tick(); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("watchdog feed loop stopped")

			return nil
		case <-ticker.C:
			if err := m.tick(); err != nil {
				return err
			}
		}
	}
}

// Close disarms the watchdog on a graceful shutdown. A DaemonSet rollout stops
// every agent of the NodeGroup at once and the nodes run with kernel.panic=0, so
// a missed disarm would leave the whole group panicked. A crash (SIGKILL, OOM)
// deliberately does not disarm: there the kernel keeps counting and fences the
// Node, which is the entire point of the watchdog.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.device == nil {
		return
	}

	if err := m.device.MagicClose(); err != nil {
		m.logger.Error("disarm watchdog on shutdown", "error", err)
	} else {
		m.logger.Info("watchdog disarmed on shutdown")
	}

	m.device = nil
	m.ready.Store(false)
}

// Ready reports that the watchdog policy is being carried out: armed and fed, or
// deliberately disarmed for a planned operation. It answers "is this agent
// healthy", not "is fencing armed", so maintenance does not flap pod readiness.
func (m *Manager) Ready() bool {
	return m.started.Load() && m.ready.Load()
}

// Alive reports that the feed loop is still ticking. This is diagnostics: the
// liveness probe reacts in tens of seconds while the watchdog timeout is
// seconds, so a hung loop resets the Node long before the kubelet notices.
// Before the loop starts, while the agent is still joining gossip, it is true.
func (m *Manager) Alive() bool {
	if !m.started.Load() {
		return true
	}

	last := m.lastTick.Load()
	if last == 0 {
		return true
	}

	return time.Since(time.Unix(0, last)) < max(3*m.params.FeedInterval, livenessGrace)
}

func (m *Manager) validate() error {
	if m.params.FeedInterval <= 0 {
		return fmt.Errorf("%w: feed interval must be positive, got %s", errFatal, m.params.FeedInterval)
	}

	// WDIOC_SETTIMEOUT takes whole seconds, so a sub-second timeout cannot be
	// expressed at all, and the kernel rejects anything below its min_timeout.
	if m.params.Timeout < time.Second {
		return fmt.Errorf("%w: timeout %s is below the one second the kernel API allows", errFatal, m.params.Timeout)
	}

	if 2*m.params.FeedInterval > m.params.Timeout {
		return fmt.Errorf("%w: timeout %s is below twice the feed interval %s, a single late tick would reset the node",
			errFatal, m.params.Timeout, m.params.FeedInterval)
	}

	return nil
}

// checkNowayout runs before the device is opened for the first time. With
// nowayout enabled the kernel ignores Magic Close, so maintenance could not
// disarm the watchdog and a planned operation would panic the Node. Checking it
// first also guarantees that this refusal never leaves an armed device behind.
func (m *Manager) checkNowayout() error {
	blocked, err := m.deps.Nowayout()
	if err != nil {
		m.deps.Events.Warning(reasonRefused, fmt.Sprintf("Cannot verify the kernel nowayout setting: %v", err))

		return fmt.Errorf("%w: verify the kernel nowayout setting: %w", errFatal, err)
	}

	if blocked {
		m.deps.Events.Warning(reasonRefused, "Kernel nowayout is enabled: the watchdog could never be disarmed for maintenance")

		return fmt.Errorf("%w: kernel nowayout is enabled, the watchdog could never be disarmed for maintenance", errFatal)
	}

	return nil
}

func (m *Manager) tick() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.touch()

	snapshot := m.deps.State.Snapshot()

	switch {
	case snapshot.UIDMismatch:
		// The Node was recreated under the same name: identity and profile were
		// read at startup and no longer describe this machine, and only a restart
		// can refresh them.
		m.deps.Events.Warning(reasonIdentityChanged, "Node was recreated with a different uid, restarting the agent")

		return fmt.Errorf("%w: own node was recreated with a different uid", errFatal)
	case !snapshot.Observed:
		// Arming before the own-Node cache is filled would hide maintenance
		// annotations and let the agent fence through a planned operation.
		m.ready.Store(false)

		return nil
	case snapshot.PlannedRemoval:
		return m.applyState(stateRemoval, snapshot.RemovalReason)
	case snapshot.Maintenance:
		return m.applyState(stateMaintenance, strings.Join(snapshot.MaintenanceReasons, ","))
	}

	if feed, reason := m.deps.ShouldFeed(); !feed {
		m.starve(reason)

		return nil
	}

	m.resumeFeeding()

	return m.feed()
}

// applyState disarms the watchdog on purpose: maintenance or a planned removal.
// The caller holds mu.
func (m *Manager) applyState(state, detail string) error {
	changed := m.state != state || m.detail != detail
	m.state, m.detail = state, detail

	if m.cannotDisarm {
		return m.keepFeedingThrough(state, detail, changed)
	}

	if m.device == nil {
		// A deliberate state is not a fault.
		m.ready.Store(true)

		if changed {
			m.logger.Info("fencing is disabled for this node", "state", state, "detail", detail)
		}

		return nil
	}

	if !m.device.MagicCloseSupported() {
		m.cannotDisarm = true

		m.logger.Warn("watchdog driver reports no magic close support, feeding continues through the planned operation",
			"state", state,
			"detail", detail,
			"identity", m.device.Identity(),
		)
		m.deps.Events.Warning(reasonNotDisarmable,
			fmt.Sprintf("Watchdog %q cannot be disarmed: feeding continues through %s (%s)", m.device.Identity(), state, detail))

		return m.keepFeedingThrough(state, detail, false)
	}

	err := m.device.MagicClose()

	// The descriptor is released either way, so the same device cannot be retried.
	m.device = nil
	m.failures = 0

	if err != nil {
		// The kernel kept the timer running and the descriptor is gone: reopening and
		// feeding is the only way to keep a planned operation from resetting the Node.
		m.cannotDisarm = true

		m.logger.Error("disarm watchdog, feeding continues through the planned operation",
			"state", state,
			"detail", detail,
			"error", err,
		)
		m.deps.Events.Warning(reasonDisarmed, fmt.Sprintf("Failed to disarm watchdog for %s (%s), feeding continues: %v", state, detail, err))

		return m.keepFeedingThrough(state, detail, false)
	}

	m.ready.Store(true)

	m.logger.Info("watchdog disarmed", "state", state, "detail", detail)
	m.deps.Events.Normal(reasonDisarmed, fmt.Sprintf("Watchdog disarmed: %s (%s)", state, detail))

	return nil
}

// keepFeedingThrough feeds a device that cannot be disarmed. The kernel keeps
// counting whatever the agent does, so stopping the feed would reset the Node in
// the middle of a planned operation. The caller holds mu.
func (m *Manager) keepFeedingThrough(state, detail string, changed bool) error {
	if changed {
		m.logger.Warn("watchdog cannot be disarmed, feeding continues through the planned operation",
			"state", state,
			"detail", detail,
		)
	}

	return m.feed()
}

// starve stops the keepalive without disarming: the Node is expected to reset
// when the timeout expires. This is the quorum-loss path of the ADR and cannot
// trigger while ShouldFeed is constant true. The caller holds mu.
func (m *Manager) starve(reason string) {
	m.ready.Store(false)

	if m.state == stateGateClosed {
		return
	}

	m.state, m.detail = stateGateClosed, reason

	if m.device == nil {
		// Reachable after a maintenance disarm: without an armed device the missing
		// keepalive resets nothing, so do not promise a reset that will not happen.
		m.logger.Error("feeding stopped while the watchdog is not armed: this node cannot be fenced locally", "reason", reason)
		m.deps.Events.Warning(reasonStarvation, fmt.Sprintf("Watchdog is not armed and feeding stopped: %s", reason))

		return
	}

	m.logger.Error("watchdog starvation: feeding stopped, the node will be reset when the timeout expires", "reason", reason)
	m.deps.Events.Warning(reasonStarvation, fmt.Sprintf("Watchdog feeding stopped: %s", reason))
}

// resumeFeeding reports the return to normal operation once. The caller holds mu.
func (m *Manager) resumeFeeding() {
	if m.state == stateFeeding {
		return
	}

	m.logger.Info("watchdog feeding resumed", "previous_state", m.state, "detail", m.detail)

	m.state, m.detail = stateFeeding, ""
}

// feed arms the device if needed and resets the timer. The caller holds mu.
func (m *Manager) feed() error {
	if m.device == nil {
		if err := m.arm(); err != nil {
			if errors.Is(err, errFatal) {
				return err
			}

			m.recordFailure(err)

			return nil
		}
	}

	if err := m.device.KeepAlive(); err != nil {
		m.recordFailure(fmt.Errorf("keepalive: %w", err))

		// A stale descriptor never recovers by itself (a reloaded module, an I/O
		// error). Release it without disarming, so the kernel keeps counting while
		// the next tick reopens the device.
		if releaseErr := m.device.ReleaseWithoutDisarm(); releaseErr != nil {
			m.logger.Error("release watchdog device after a failed keepalive", "error", releaseErr)
		}

		m.device = nil

		return nil
	}

	m.failures = 0
	m.ready.Store(true)

	return nil
}

// arm opens the device and applies the profile timeout. The caller holds mu.
func (m *Manager) arm() error {
	device, err := m.deps.Open()
	if err != nil {
		return fmt.Errorf("arm watchdog: %w", err)
	}

	if !device.SetTimeoutSupported() {
		return m.refuse(device, fmt.Errorf("driver %q does not report WDIOF_SETTIMEOUT, the profile timeout cannot be applied", device.Identity()))
	}

	effective, err := device.SetTimeout(m.params.Timeout)
	if err != nil {
		return m.refuse(device, fmt.Errorf("set timeout to %s: %w", m.params.Timeout, err))
	}

	// A timeout close to the feed interval turns one late tick into a reset.
	if effective < 2*m.params.FeedInterval {
		return m.refuse(device, fmt.Errorf("effective timeout %s is below twice the feed interval %s", effective, m.params.FeedInterval))
	}

	if effective != wholeSeconds(m.params.Timeout) {
		m.logger.Warn("driver adjusted the watchdog timeout",
			"requested", m.params.Timeout.String(),
			"effective", effective.String(),
		)
	}

	if !device.MagicCloseSupported() {
		m.logger.Warn("watchdog driver reports no magic close support: planned operations will keep feeding instead of disarming",
			"identity", device.Identity())
		m.deps.Events.Warning(reasonNotDisarmable,
			fmt.Sprintf("Watchdog %q cannot be disarmed: planned operations will keep feeding instead", device.Identity()))
	}

	if left, timeLeftErr := device.GetTimeLeft(); timeLeftErr != nil {
		// softdog has no get_timeleft op; that is expected and not a problem.
		if !errors.Is(timeLeftErr, errors.ErrUnsupported) {
			m.logger.Warn("read watchdog time left", "error", timeLeftErr)
		}
	} else {
		m.logger.Debug("watchdog time left", "time_left", left.String())
	}

	// Neither the failure counter nor readiness is touched here: a reopen after a
	// failed keepalive must not reset the streak, otherwise a device that accepts
	// open but rejects every ping would look healthy forever.
	m.device = device

	m.logger.Info("watchdog armed",
		"identity", device.Identity(),
		"timeout", effective.String(),
		"feed_interval", m.params.FeedInterval.String(),
	)
	m.deps.Events.Normal(reasonArmed,
		fmt.Sprintf("Watchdog %q armed with timeout %s and feed interval %s", device.Identity(), effective, m.params.FeedInterval))

	return nil
}

// refuse disarms a device the agent must not rely on and marks the failure fatal.
func (m *Manager) refuse(device Device, cause error) error {
	if err := device.MagicClose(); err != nil {
		m.logger.Error("disarm watchdog after a refusal", "error", err)
	}

	m.deps.Events.Warning(reasonRefused, fmt.Sprintf("Refusing to run with this watchdog: %v", cause))

	return fmt.Errorf("%w: %w", errFatal, cause)
}

// recordFailure counts consecutive feed failures. The caller holds mu.
func (m *Manager) recordFailure(err error) {
	m.failures++

	m.logger.Error("watchdog feed failed", "error", err, "consecutive_failures", m.failures)

	if m.failures < maxFeedFailures {
		return
	}

	m.ready.Store(false)

	// Once per streak: the loop keeps retrying every feed interval.
	if m.failures == maxFeedFailures {
		m.deps.Events.Warning(reasonFeedFailed, fmt.Sprintf("Watchdog feed failed %d times in a row: %v", m.failures, err))
	}
}

func (m *Manager) touch() {
	m.lastTick.Store(time.Now().UnixNano())
}

// wholeSeconds mirrors the rounding the device adapter applies, so the timeout
// warning only fires when the driver itself changed the value.
func wholeSeconds(timeout time.Duration) time.Duration {
	rounded := timeout.Truncate(time.Second)
	if rounded < timeout {
		rounded += time.Second
	}

	return rounded
}
