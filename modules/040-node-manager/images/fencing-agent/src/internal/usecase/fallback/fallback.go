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

package fallback

import (
	"context"
	"fmt"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/domain"
)

const (
	idleTick = time.Second

	maxFailures = 5
)

const (
	reasonEntered         = "FencingFallbackEntered"
	reasonLeft            = "FencingFallbackLeft"
	reasonHeartbeatFailed = "FencingFallbackHeartbeatFailed"
)

type AliveLister interface {
	Members() []string
	Changed() <-chan struct{}
}

type ExpectedSnapshotter interface {
	Snapshot() ([]domain.Peer, int, int)
}

type StateStore interface {
	List(ctx context.Context) ([]v1alpha1.FencingFailedNodeState, error)
	Create(ctx context.Context, peer domain.Peer) (bool, error)
	Heartbeat(ctx context.Context, name string, fallback v1alpha1.FencingFailedNodeStateFallback) error
	Delete(ctx context.Context, name string, uid types.UID) error
}

type EventRecorder interface {
	Normal(reason, message string)
	Warning(reason, message string)
}

type Params struct {
	Node       domain.NodeIdentity
	Heartbeat  time.Duration
	APITimeout time.Duration
}

type Deps struct {
	Alive    AliveLister
	Expected ExpectedSnapshotter
	States   StateStore
	Events   EventRecorder
	Now      func() time.Time
}

type Snapshot struct {
	Observed     bool
	HasQuorum    bool
	Alive        int
	Expected     int
	Quorum       int
	Active       bool
	APIReachable bool
	QuorumLostAt time.Time
}

type Monitor struct {
	params Params
	deps   Deps
	logger *log.Logger

	startedAt time.Time

	mu       sync.Mutex
	snapshot Snapshot

	nextBeat       time.Time
	failures       int
	deleted        types.UID
	recordVanished bool
}

func New(params Params, deps Deps, logger *log.Logger) *Monitor {
	if deps.Now == nil {
		deps.Now = time.Now
	}

	return &Monitor{
		params:    params,
		deps:      deps,
		logger:    logger,
		startedAt: deps.Now().Truncate(time.Second),
		snapshot:  Snapshot{APIReachable: true},
	}
}

func (m *Monitor) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.snapshot
}

func (m *Monitor) ShouldFeed() (bool, string) {
	s := m.Snapshot()

	switch {
	case !s.Observed:
		return true, "fallback monitor has not run yet"
	case s.HasQuorum:
		return true, ""
	case s.Active && s.APIReachable:
		return true, ""
	default:
		return false, fmt.Sprintf("no gossip quorum (%d of %d alive, quorum %d) and the Kubernetes API is unreachable",
			s.Alive, s.Expected, s.Quorum)
	}
}

func (m *Monitor) Run(ctx context.Context) error {
	changed := m.deps.Alive.Changed()

	for {
		m.reconcile(ctx)

		timer := time.NewTimer(m.wait())

		select {
		case <-ctx.Done():
			timer.Stop()

			return nil
		case <-timer.C:
		case <-changed:
			timer.Stop()
		}
	}
}

func (m *Monitor) wait() time.Duration {
	if !m.Snapshot().Active {
		return idleTick
	}

	return max(m.nextBeat.Sub(m.deps.Now()), 0)
}

func (m *Monitor) reconcile(ctx context.Context) {
	expected, _, _ := m.deps.Expected.Snapshot()
	view := domain.NewView(expected, m.deps.Alive.Members())
	now := m.deps.Now()

	s := m.Snapshot()
	s.Observed = true
	s.HasQuorum = view.HasQuorum()
	s.Alive, s.Expected, s.Quorum = view.AliveCount(), view.ExpectedCount(), view.QuorumSize()

	switch {
	case s.HasQuorum && s.Active:
		s.Active = false
		s.APIReachable = true
		s.QuorumLostAt = time.Time{}
		m.failures = 0
		m.recordVanished = false

		m.logger.Info("gossip quorum is back, leaving fallback mode", "alive", s.Alive, "expected", s.Expected, "quorum", s.Quorum)
		m.deps.Events.Normal(reasonLeft, fmt.Sprintf("left fallback mode: %d of %d nodes alive, quorum %d", s.Alive, s.Expected, s.Quorum))
	case !s.HasQuorum && !s.Active:
		s.Active = true
		s.APIReachable = true
		s.QuorumLostAt = now
		m.nextBeat = now

		m.logger.Warn("gossip quorum lost, entering fallback mode", "alive", s.Alive, "expected", s.Expected, "quorum", s.Quorum)
		m.deps.Events.Normal(reasonEntered, fmt.Sprintf("entered fallback mode: %d of %d nodes alive, quorum %d", s.Alive, s.Expected, s.Quorum))
	}

	m.store(s)

	if s.HasQuorum {
		m.removeOwnRecord(ctx)

		return
	}

	if now.Before(m.nextBeat) {
		return
	}

	m.beat(ctx, s.QuorumLostAt)
}

func (m *Monitor) removeOwnRecord(ctx context.Context) {
	states, err := m.deps.States.List(ctx)
	if err != nil {
		m.logger.Warn("list of fencing states failed, own fallback record not checked", "error", err)

		return
	}

	for _, state := range states {
		if state.Name != m.params.Node.Name || state.Status.Fallback == nil || state.CreationTimestamp.Time.Before(m.startedAt) {
			continue
		}

		if state.UID == m.deleted {
			continue
		}

		if err := m.deps.States.Delete(ctx, state.Name, state.UID); err != nil {
			m.logger.Warn("own fallback record was not removed, retrying next pass", "error", err)

			return
		}

		m.deleted = state.UID

		m.logger.Info("own fallback record removed")
	}
}

func (m *Monitor) beat(ctx context.Context, quorumLostAt time.Time) {
	now := m.deps.Now()
	m.nextBeat = now.Add(m.params.Heartbeat)

	err := m.write(ctx, now, quorumLostAt)
	if err == nil {
		if m.failures > 0 {
			m.logger.Info("fallback heartbeat reaches the Kubernetes API again", "failures", m.failures)
		}

		m.failures = 0
		m.recordVanished = false
		m.setAPIReachable(true)

		return
	}

	if apierrors.IsNotFound(err) {
		m.setAPIReachable(true)

		if !m.recordVanished {
			m.recordVanished = true
			m.logger.Info("own record was removed by a peer between create and heartbeat, retrying next beat")
		}

		return
	}

	m.failures++
	m.setAPIReachable(false)

	if m.failures == maxFailures {
		m.logger.Error("fallback heartbeat keeps failing, this node is not protected from evacuation",
			"error", err,
			"attempts", m.failures,
		)
		m.deps.Events.Warning(reasonHeartbeatFailed, fmt.Sprintf(
			"fallback heartbeat failed %d times in a row, this node is not protected from evacuation: %v",
			m.failures, err,
		))

		return
	}

	m.logger.Warn("fallback heartbeat failed", "error", err, "attempt", m.failures)
}

func (m *Monitor) write(ctx context.Context, now, quorumLostAt time.Time) error {
	attemptCtx := ctx

	if m.params.APITimeout > 0 {
		var cancel context.CancelFunc

		attemptCtx, cancel = context.WithTimeout(ctx, m.params.APITimeout)
		defer cancel()
	}

	at := metav1.NewMicroTime(now)
	lost := metav1.NewTime(quorumLostAt)

	section := v1alpha1.FencingFailedNodeStateFallback{
		Active:            true,
		LastHeartbeatAt:   &at,
		QuorumLostAt:      &lost,
		APIReachable:      true,
		HeartbeatInterval: metav1.Duration{Duration: m.params.Heartbeat},
	}

	err := m.deps.States.Heartbeat(attemptCtx, m.params.Node.Name, section)
	if !apierrors.IsNotFound(err) {
		return err
	}

	self := domain.Peer{Name: m.params.Node.Name, IP: m.params.Node.IP, UID: m.params.Node.UID}

	if _, err := m.deps.States.Create(attemptCtx, self); err != nil {
		return err
	}

	return m.deps.States.Heartbeat(attemptCtx, m.params.Node.Name, section)
}

func (m *Monitor) store(s Snapshot) {
	m.mu.Lock()
	m.snapshot = s
	m.mu.Unlock()
}

func (m *Monitor) setAPIReachable(reachable bool) {
	m.mu.Lock()
	m.snapshot.APIReachable = reachable
	m.mu.Unlock()
}
