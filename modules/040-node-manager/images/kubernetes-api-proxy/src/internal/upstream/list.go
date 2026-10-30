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
	"fmt"
	"log/slog"
	"math/rand/v2"
	"slices"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"kubernetes-api-proxy/pkg/utils"
)

var ErrNoUpstreams = fmt.Errorf("no upstreams available")

type ListOption func(*ListConfig) error

func WithHealthcheckInterval(interval time.Duration) ListOption {
	return func(l *ListConfig) error {
		if interval <= 0 {
			return fmt.Errorf("healthcheck interval should be positive: %s", interval)
		}

		l.healthcheckInterval = interval

		return nil
	}
}

func WithHealthCheckJitter(jitter float64) ListOption {
	return func(l *ListConfig) error {
		if jitter < 0 || jitter > 1 {
			return fmt.Errorf("healthcheck jitter should in range [0, 1]: %f", jitter)
		}

		l.healthcheckJitter = jitter

		return nil
	}
}

func WithHealthcheckTimeout(timeout time.Duration) ListOption {
	return func(l *ListConfig) error {
		if timeout <= 0 {
			return fmt.Errorf("healthcheck timeout should be positive: %s", timeout)
		}

		l.healthcheckTimeout = timeout

		return nil
	}
}

// WithFailStep sets the probe interval while no upstream is ready and the
// longest fail-step period in one failure episode. A zero window disables
// fail-step.
func WithFailStep(interval, window time.Duration) ListOption {
	return func(l *ListConfig) error {
		if interval <= 0 {
			return fmt.Errorf("fail-step interval should be positive: %s", interval)
		}

		if window < 0 {
			return fmt.Errorf("fail-step window should not be negative: %s", window)
		}

		l.failStepInterval = interval
		l.failStepWindow = window

		return nil
	}
}

func WithDownHold(hold time.Duration) ListOption {
	return func(l *ListConfig) error {
		l.downHold = hold

		return nil
	}
}

func WithStallTimeout(timeout time.Duration) ListOption {
	return func(l *ListConfig) error {
		if timeout < 0 {
			return fmt.Errorf("stall timeout should not be negative: %s", timeout)
		}

		l.stallTimeout = timeout

		return nil
	}
}

func WithLogger(logger *log.Logger) ListOption {
	return func(l *ListConfig) error {
		l.logger = logger

		return nil
	}
}

type Tier int

type List struct { //nolint:govet
	listConfig

	cmp func(*Upstream, *Upstream) bool

	mu          sync.RWMutex
	nodes       []node
	current     int
	nobodyReady bool
	pending     []logEvent
	onStall     func(*Upstream)

	// startedSeq is the number of the last started round. probeSeq and
	// probeOK describe the last applied round.
	startedSeq uint64
	probeSeq   uint64
	probeOK    bool
	// failSince is the start of the current failure episode: the first
	// round without a ready upstream after a round with one.
	failSince time.Time
	// failStepOver is true when the fail-step window of the current failure
	// episode ended.
	failStepOver bool
}

// logEvent is a log record that waits for the end of the lock. The zero
// level is Info.
type logEvent struct {
	level slog.Level
	msg   string
	args  []any
}

type ListConfig struct {
	logger *log.Logger

	healthcheckInterval time.Duration
	healthcheckTimeout  time.Duration
	healthcheckJitter   float64
	failStepInterval    time.Duration
	failStepWindow      time.Duration
	downHold            time.Duration
	stallTimeout        time.Duration

	probe     func(context.Context, *Upstream) (probeResult, error)
	now       func() time.Time
	boot      func() uint64
	stallHook func(*Upstream, time.Time)

	healthWg        sync.WaitGroup
	healthCtxCancel context.CancelFunc
}

type listConfig = ListConfig

const (
	lowestScore  = -5.0
	highestScore = 3.0
	initialScore = 1.0

	failScoreDelta    = -1.0
	successScoreDelta = 1.0

	missingRoundsToDrop = 3
)

func byAddr(a, b *Upstream) bool {
	return a.Addr == b.Addr
}

func NewList(upstreams []*Upstream, options ...ListOption) (*List, error) {
	return NewListWithCmp(upstreams, byAddr, options...)
}

func NewListWithCmp(
	upstreams []*Upstream,
	cmp func(*Upstream, *Upstream) bool,
	options ...ListOption,
) (*List, error) {
	list, err := newList(upstreams, cmp, options...)
	if err != nil {
		return nil, err
	}

	list.start()

	return list, nil
}

func newList(
	upstreams []*Upstream,
	cmp func(*Upstream, *Upstream) bool,
	options ...ListOption,
) (*List, error) {
	list := &List{
		listConfig: listConfig{
			logger:              log.NewNop(),
			healthcheckInterval: time.Second,
			healthcheckTimeout:  500 * time.Millisecond,
			failStepInterval:    500 * time.Millisecond,
			downHold:            5 * time.Second,
			now:                 time.Now,
			boot:                bootMillis,
		},
		cmp:     cmp,
		current: -1,
	}

	for _, opt := range options {
		if err := opt(&list.listConfig); err != nil {
			return nil, err
		}
	}

	if list.probe == nil {
		dial := probeDial(list.healthcheckTimeout)
		list.probe = func(ctx context.Context, u *Upstream) (probeResult, error) { return u.probe(ctx, dial) }
	}

	list.nodes = make([]node, 0, len(upstreams))
	for _, u := range upstreams {
		list.nodes = append(list.nodes, newNode(u, false))
	}

	return list, nil
}

func newNode(u *Upstream, unsettled bool) node {
	return node{backend: u, score: initialScore, tier: tierFailed, unsettled: unsettled}
}

func (list *List) start() {
	var ctx context.Context

	ctx, list.healthCtxCancel = context.WithCancel(context.Background())

	list.healthWg.Add(1)

	go list.runLoop(ctx)
}

func (list *List) Shutdown() {
	list.healthCtxCancel()
	list.healthWg.Wait()
}

// runLoop runs the rounds one after another. The next round starts a delay
// after the start of the previous round, or when the previous round ends if
// it ends later.
func (list *List) runLoop(ctx context.Context) {
	defer list.healthWg.Done()

	timer := time.NewTimer(0)
	defer timer.Stop()

	for first := true; ; first = false {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		// The timer and the stop can come together. Do not start a round
		// after the stop.
		if ctx.Err() != nil {
			return
		}

		start := time.Now()
		failStep := list.round(ctx, list.beginRound())

		delay := list.cycleDelay(jitterSpread())
		// The second round starts after a random part of the interval, if
		// fail-step is not active.
		if first && !failStep {
			delay = list.firstInterval()
		}

		timer.Reset(max(delay-time.Since(start), 0))
	}
}

func (list *List) firstInterval() time.Duration {
	if list.healthcheckJitter <= 0 {
		return list.healthcheckInterval
	}

	return time.Duration(rand.Float64() * float64(list.healthcheckInterval))
}

func jitterSpread() float64 {
	return rand.Float64()*2 - 1
}

// cycleDelay returns the time from the start of a round to the start of the
// next round. spread is in [-1, 1] and scales the jitter.
func (list *List) cycleDelay(spread float64) time.Duration {
	now := list.now()

	list.mu.RLock()
	base := list.healthcheckInterval
	if list.failStepLocked(now) {
		base = list.failStepInterval
	}
	list.mu.RUnlock()

	return time.Duration((1 + spread*list.healthcheckJitter) * float64(base))
}

func (list *List) failStepLocked(now time.Time) bool {
	return !list.failSince.IsZero() && now.Sub(list.failSince) < list.failStepWindow
}

type outcome struct {
	value probeResult
	ok    bool
}

// probeAll probes every upstream in parallel, once per round.
func (list *List) probeAll(ctx context.Context, backends []*Upstream) []outcome {
	out := make([]outcome, len(backends))

	var wg sync.WaitGroup
	for i, b := range backends {
		wg.Go(func() {
			v, ok := list.probeOnce(ctx, b)
			out[i] = outcome{value: v, ok: ok}
		})
	}
	wg.Wait()

	return out
}

func (list *List) probeOnce(ctx context.Context, b *Upstream) (probeResult, bool) {
	probeCtx, cancel := context.WithTimeout(ctx, list.healthcheckTimeout)
	defer cancel()

	v, err := list.probe(probeCtx, b)
	if err != nil {
		list.logger.Warn("probe skipped", slog.String("upstream", b.Addr), slog.String("error", err.Error()))

		return probeResult{}, false
	}

	if v.kind == probeReady {
		list.markReady(b, list.now())
	}

	return v, true
}

// markReady records a ready answer of the upstream at once. So
// lastReadyAgoMs counts from the answer, not from the end of the round, and
// a slow upstream in the same round does not hide the answer.
func (list *List) markReady(u *Upstream, at time.Time) {
	list.mu.Lock()
	defer list.mu.Unlock()

	if i := list.indexLocked(u); i >= 0 {
		list.readyLocked(i, at)
	}
}

func (list *List) readyLocked(i int, at time.Time) {
	if at.After(list.nodes[i].lastReadyAt) {
		list.nodes[i].lastReadyAt = at
	}
}

func (list *List) backends() []*Upstream {
	list.mu.RLock()
	defer list.mu.RUnlock()

	return utils.Map(list.nodes, func(n node) *Upstream { return n.backend })
}

func (list *List) noteLocked(msg string, u *Upstream, attrs ...any) {
	list.noteListLocked(msg, append([]any{slog.String("upstream", u.Addr)}, attrs...)...)
}

func (list *List) noteListLocked(msg string, attrs ...any) {
	list.pending = append(list.pending, logEvent{msg: msg, args: attrs})
}

func (list *List) warnListLocked(msg string, attrs ...any) {
	list.pending = append(list.pending, logEvent{level: slog.LevelWarn, msg: msg, args: attrs})
}

func (list *List) unlockAndLog() {
	pending := list.pending
	list.pending = nil
	list.mu.Unlock()

	for _, e := range pending {
		list.logger.Log(context.Background(), e.level, e.msg, e.args...)
	}
}

func (list *List) indexLocked(u *Upstream) int {
	return slices.IndexFunc(list.nodes, func(n node) bool { return list.cmp(n.backend, u) })
}

func (list *List) effectiveTierLocked(i int) Tier {
	if list.nodes[i].missing > 0 {
		return tierFailed
	}

	return list.nodes[i].tier
}

func (list *List) Reconcile(newList []*Upstream) {
	list.mu.Lock()
	defer list.unlockAndLog()

	kept := make([]node, 0, len(list.nodes)+len(newList))

	for _, n := range list.nodes {
		if slices.ContainsFunc(newList, func(u *Upstream) bool { return list.cmp(u, n.backend) }) {
			n.missing = 0
			kept = append(kept, n)

			continue
		}

		n.missing++
		if n.missing == 1 {
			list.noteLocked("upstream missing from discovery", n.backend)
		}

		if n.missing >= missingRoundsToDrop {
			list.noteLocked("upstream removed", n.backend)

			continue
		}

		kept = append(kept, n)
	}

	for _, u := range newList {
		if slices.ContainsFunc(kept, func(n node) bool { return list.cmp(u, n.backend) }) {
			continue
		}

		kept = append(kept, newNode(u, len(list.nodes) > 0))
	}

	list.nodes = kept
}

func (list *List) Down(upstream *Upstream) {
	list.hold(upstream, "dial")
}

func (list *List) Silent(upstream *Upstream) {
	list.hold(upstream, "first-byte")
}

func (list *List) hold(upstream *Upstream, reason string) {
	now := list.now()

	list.mu.Lock()
	defer list.unlockAndLog()

	i := list.indexLocked(upstream)
	if i < 0 {
		return
	}

	list.nodes[i].heldUntil = now.Add(list.downHold)
	list.nodes[i].downed = true
	list.failTierLocked(i, reason)
	list.adjustScoreLocked(i, failScoreDelta)
}

func (list *List) Healthy() bool {
	list.mu.RLock()
	defer list.mu.RUnlock()

	for i := range list.nodes {
		if list.effectiveTierLocked(i) != tierFailed {
			return true
		}
	}

	return false
}

func (list *List) PickAsString() (string, error) {
	upstream, err := list.Pick()
	if err != nil {
		return "", err
	}

	return upstream.Address(), nil
}

func (list *List) ListFullAddresses() []string {
	list.mu.RLock()
	defer list.mu.RUnlock()

	return utils.Map(list.nodes, func(n node) string { return n.backend.Address() })
}

func (list *List) ExportNodes() ([]ExportNode, error) {
	list.mu.RLock()
	defer list.mu.RUnlock()

	// Read the clock under the lock. A ready answer recorded before the lock
	// is then not later than now, and lastReadyAgoMs is not negative.
	now := list.now()

	out := make([]ExportNode, 0, len(list.nodes))
	for i, n := range list.nodes {
		out = append(out, ExportNode{
			Upstream:       n.backend.Addr,
			Score:          n.score,
			Tier:           n.tier,
			EffectiveTier:  list.effectiveTierLocked(i),
			Missing:        n.missing,
			LastProbe:      n.lastProbe(),
			Warm:           n.warm,
			Downed:         n.downed,
			Held:           now.Before(n.heldUntil),
			Stalling:       n.stalling,
			Evictions:      n.evictions,
			LastReadyAgoMs: n.lastReadyAgo(now),
		})
	}

	return out, nil
}

// Probe returns the number and the result of the last applied round. The
// number is the boot clock at the start of that round. The result is true
// when at least one upstream was ready.
func (list *List) Probe() (uint64, bool) {
	list.mu.RLock()
	defer list.mu.RUnlock()

	return list.probeSeq, list.probeOK
}
