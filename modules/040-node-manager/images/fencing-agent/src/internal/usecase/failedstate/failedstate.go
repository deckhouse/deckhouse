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

package failedstate

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/domain"
)

const (
	tick = time.Second

	maxAttempts = 5
	cooldown    = 30 * time.Second

	jitterFraction = 0.2

	unexpectedGrace = 30 * time.Second

	reasonStateCreated     = "FencingStateCreated"
	reasonStateCleared     = "FencingStateCleared"
	reasonStateWriteFailed = "FencingStateWriteFailed"
)

// AliveLister reports the peers gossip sees, without touching the Kubernetes API.
type AliveLister interface {
	Members() []string
}

// ExpectedSnapshotter reports the peers the NodeGroup should have, from the Node
// informer cache.
type ExpectedSnapshotter interface {
	Snapshot() ([]domain.Peer, int, int)
}

// StateStore is the Kubernetes side of the incident record. Create and MarkFailed
// report whether the call is what changed something, so the writer announces an
// incident once and not once per agent that passes through the same state.
type StateStore interface {
	List(ctx context.Context) ([]v1alpha1.FencingFailedNodeState, error)
	Create(ctx context.Context, peer domain.Peer) (bool, error)
	MarkFailed(ctx context.Context, name string, failed v1alpha1.FencingFailedNodeStateFailed) (bool, error)
	Delete(ctx context.Context, name string, uid types.UID) error
}

type EventRecorder interface {
	Normal(reason, message string)
	Warning(reason, message string)
}

type Params struct {
	NodeName         string
	RetryInterval    time.Duration
	MaxRetryInterval time.Duration
	TakeoverDelay    time.Duration
}

type Deps struct {
	Alive    AliveLister
	Expected ExpectedSnapshotter
	States   StateStore
	Events   EventRecorder
	Changed  <-chan struct{}
	Now      func() time.Time
}

type incident struct {
	detectedAt time.Time
	attempts   int
	retryAfter time.Time
}

func (i *incident) recorded() {
	i.attempts = 0
	i.retryAfter = time.Time{}
}

type Writer struct {
	params Params
	deps   Deps
	logger *log.Logger

	seen      domain.SeenAlive
	incidents map[string]*incident
	waiting map[string]time.Time
	ownStateCleared bool
}

func New(params Params, deps Deps, logger *log.Logger) *Writer {
	if deps.Now == nil {
		deps.Now = time.Now
	}

	return &Writer{
		params:    params,
		deps:      deps,
		logger:    logger,
		incidents: make(map[string]*incident),
		waiting:   make(map[string]time.Time),
	}
}

// Run reconciles until ctx is cancelled.
func (w *Writer) Run(ctx context.Context) error {
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		w.reconcile(ctx)

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		case <-w.deps.Changed:
		}
	}
}

func (w *Writer) clearOwnState(ctx context.Context, states []v1alpha1.FencingFailedNodeState) {
	if w.ownStateCleared {
		return
	}

	for _, state := range states {
		if state.Name != w.params.NodeName {
			continue
		}

		if err := w.deps.States.Delete(ctx, state.Name, state.UID); err != nil {
			// The latch stays open, so the next pass tries again.
			w.logger.Warn("stale fencing state of this node was not removed", "error", err)

			return
		}

		w.logger.Info("stale fencing state of this node removed")
		w.deps.Events.Normal(reasonStateCleared, "removed the fencing state left over from an earlier incident on this node")
	}

	w.ownStateCleared = true
}

func (w *Writer) reconcile(ctx context.Context) {
	expected, _, _ := w.deps.Expected.Snapshot()
	view := domain.NewView(expected, w.deps.Alive.Members())

	w.seen.Observe(view.Alive())
	w.seen.Retain(expected)

	states, err := w.deps.States.List(ctx)
	if err != nil {
		w.logger.Warn("list of fencing states failed, nothing reconciled this pass", "error", err)

		return
	}

	w.clearOwnState(ctx, states)

	if !view.HasQuorum() {
		w.logger.Debug("no local quorum, fencing states are not written",
			"alive", view.AliveCount(),
			"expected", view.ExpectedCount(),
			"quorum", view.QuorumSize(),
		)

		return
	}

	existing := make(map[string]*v1alpha1.FencingFailedNodeState, len(states))
	for i := range states {
		existing[states[i].Name] = &states[i]
	}

	failed := view.Failed(&w.seen)
	w.forgetRecovered(failed)

	for _, peer := range failed {
		inc := w.incident(peer.Name)

		if !w.myTurn(domain.WriterRank(view.Alive(), peer.Name, w.params.NodeName), inc.detectedAt) {
			continue
		}

		w.report(ctx, view, peer, existing[peer.Name], inc)
	}

	for i := range states {
		w.clear(ctx, view, &states[i])
	}

	w.forgetWaitingBeyond(states)
}

func (w *Writer) myTurn(rank int, since time.Time) bool {
	if rank < 0 {
		return false
	}

	if rank == 0 {
		return true
	}

	if w.params.TakeoverDelay <= 0 {
		return false
	}

	return !w.deps.Now().Before(since.Add(w.params.TakeoverDelay * time.Duration(rank)))
}

func (w *Writer) report(
	ctx context.Context,
	view domain.View,
	peer domain.Peer,
	existing *v1alpha1.FencingFailedNodeState,
	inc *incident,
) {
	if existing != nil && existing.Status.Failed != nil {
		inc.recorded()

		return
	}

	if w.deps.Now().Before(inc.retryAfter) {
		return
	}

	if peer.UID == "" {
		w.logger.Warn("peer has no node UID in the informer cache, its incident cannot be recorded", "member", peer.Name)

		return
	}

	created := false

	if existing == nil {
		var err error

		if created, err = w.deps.States.Create(ctx, peer); err != nil {
			w.retryLater(inc, peer.Name, "creating the fencing state failed", err)

			return
		}
	}

	failed := v1alpha1.FencingFailedNodeStateFailed{
		DetectedAt: metav1.NewTime(inc.detectedAt),
		DetectedBy: w.params.NodeName,
		Reason:     v1alpha1.FailedReasonMemberlistDead,
		AliveCount: int32(view.AliveCount()),
		QuorumSize: int32(view.QuorumSize()),
	}

	recorded, err := w.deps.States.MarkFailed(ctx, peer.Name, failed)
	if err != nil {
		w.retryLater(inc, peer.Name, "recording the failed state failed", err)

		return
	}

	inc.recorded()

	if !created && !recorded {
		return
	}

	w.logger.Info("fencing state recorded",
		"member", peer.Name,
		"detected_at", inc.detectedAt.UTC().Format(time.RFC3339),
		"alive", view.AliveCount(),
		"quorum", view.QuorumSize(),
	)
	w.deps.Events.Normal(reasonStateCreated, fmt.Sprintf(
		"recorded peer %s as failed: %d of %d nodes alive, quorum %d",
		peer.Name, view.AliveCount(), view.ExpectedCount(), view.QuorumSize(),
	))
}

func (w *Writer) clear(ctx context.Context, view domain.View, state *v1alpha1.FencingFailedNodeState) {
	if state.Status.Fallback != nil {
		return
	}

	_, expected := view.Peer(state.Name)
	if expected {
		delete(w.waiting, unexpectedKey(state.Name))
	}

	var reason string

	switch {
	case view.IsAlive(state.Name):
		reason = "the peer is back in the gossip network"
	case !expected && w.unexpectedLongEnough(state.Name):
		reason = "the node is no longer part of this NodeGroup"
	default:
		delete(w.waiting, clearKey(state.Name))

		return
	}


	if !w.myTurn(domain.WriterRank(view.Alive(), state.Name, w.params.NodeName), w.waitingSince(clearKey(state.Name))) {
		return
	}

	if err := w.deps.States.Delete(ctx, state.Name, state.UID); err != nil {
		w.logger.Warn("fencing state was not removed", "member", state.Name, "reason", reason, "error", err)

		return
	}

	delete(w.incidents, state.Name)
	delete(w.waiting, clearKey(state.Name))
	delete(w.waiting, unexpectedKey(state.Name))

	w.logger.Info("fencing state removed", "member", state.Name, "reason", reason)
	w.deps.Events.Normal(reasonStateCleared, fmt.Sprintf("fencing state of %s removed: %s", state.Name, reason))
}


func (w *Writer) unexpectedLongEnough(name string) bool {
	return !w.deps.Now().Before(w.waitingSince(unexpectedKey(name)).Add(unexpectedGrace))
}

func (w *Writer) waitingSince(key string) time.Time {
	since, waiting := w.waiting[key]
	if !waiting {
		since = w.deps.Now()
		w.waiting[key] = since
	}

	return since
}

func (w *Writer) forgetWaitingBeyond(states []v1alpha1.FencingFailedNodeState) {
	live := make(map[string]struct{}, 2*len(states))

	for i := range states {
		live[clearKey(states[i].Name)] = struct{}{}
		live[unexpectedKey(states[i].Name)] = struct{}{}
	}

	for key := range w.waiting {
		if _, ok := live[key]; !ok {
			delete(w.waiting, key)
		}
	}
}

func clearKey(name string) string {
	return "clear/" + name
}

func unexpectedKey(name string) string {
	return "unexpected/" + name
}

func (w *Writer) incident(name string) *incident {
	if known, ok := w.incidents[name]; ok {
		return known
	}

	known := &incident{detectedAt: w.deps.Now()}
	w.incidents[name] = known

	w.logger.Info("peer left the gossip network", "member", name)

	return known
}


func (w *Writer) forgetRecovered(failed []domain.Peer) {
	for name := range w.incidents {
		if !slices.ContainsFunc(failed, func(peer domain.Peer) bool { return peer.Name == name }) {
			delete(w.incidents, name)
		}
	}
}

func (w *Writer) retryLater(inc *incident, member, message string, err error) {
	inc.attempts++

	if inc.attempts >= maxAttempts {
		inc.attempts = 0
		inc.retryAfter = w.deps.Now().Add(cooldown)

		w.logger.Error(message,
			"member", member,
			"error", err,
			"attempts", maxAttempts,
			"cooldown", cooldown.String(),
		)
		w.deps.Events.Warning(reasonStateWriteFailed, fmt.Sprintf(
			"could not record peer %s as failed after %d attempts, retrying in %s: %v",
			member, maxAttempts, cooldown, err,
		))

		return
	}

	delay := w.backoff(inc.attempts)
	inc.retryAfter = w.deps.Now().Add(delay)

	w.logger.Warn(message,
		"member", member,
		"error", err,
		"attempt", inc.attempts,
		"retry_after", delay.String(),
	)
}

// backoff doubles from RetryInterval up to MaxRetryInterval.
func (w *Writer) backoff(attempt int) time.Duration {
	delay := w.params.RetryInterval

	for range attempt - 1 {
		delay = min(delay*2, w.params.MaxRetryInterval)
	}

	return jitter(delay)
}

func jitter(delay time.Duration) time.Duration {
	spread := float64(delay) * jitterFraction

	return delay + time.Duration((rand.Float64()*2-1)*spread)
}
