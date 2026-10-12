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
}

type logEvent struct {
	msg  string
	args []any
}

type ListConfig struct {
	logger *log.Logger

	healthcheckInterval time.Duration
	healthcheckTimeout  time.Duration
	healthcheckJitter   float64
	downHold            time.Duration
	stallTimeout        time.Duration

	probe     func(context.Context, *Upstream) (probeResult, error)
	now       func() time.Time
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
			downHold:            5 * time.Second,
			probe:               func(ctx context.Context, u *Upstream) (probeResult, error) { return u.probe(ctx) },
			now:                 time.Now,
		},
		cmp:     cmp,
		current: -1,
	}

	for _, opt := range options {
		if err := opt(&list.listConfig); err != nil {
			return nil, err
		}
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

func (list *List) runLoop(ctx context.Context) {
	defer list.healthWg.Done()

	list.round(ctx)

	timer := time.NewTimer(list.firstInterval())
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		list.round(ctx)
		timer.Reset(list.nextInterval())
	}
}

func (list *List) firstInterval() time.Duration {
	if list.healthcheckJitter <= 0 {
		return list.healthcheckInterval
	}

	return time.Duration(rand.Float64() * float64(list.healthcheckInterval))
}

func (list *List) nextInterval() time.Duration {
	spread := (rand.Float64()*2 - 1) * list.healthcheckJitter

	return time.Duration((1 + spread) * float64(list.healthcheckInterval))
}

type outcome struct {
	value probeResult
	ok    bool
}

func probeAll(
	ctx context.Context,
	backends []*Upstream,
	timeout time.Duration,
	probe func(context.Context, *Upstream) (probeResult, error),
	logger *log.Logger,
) []outcome {
	out := make([]outcome, len(backends))

	var wg sync.WaitGroup
	for i, b := range backends {
		wg.Go(func() {
			probeCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			v, err := probe(probeCtx, b)
			if err != nil {
				if logger != nil {
					logger.Warn("probe skipped", slog.String("upstream", b.Addr), slog.String("error", err.Error()))
				}

				return
			}

			out[i] = outcome{value: v, ok: true}
		})
	}
	wg.Wait()

	return out
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

func (list *List) unlockAndLog() {
	pending := list.pending
	list.pending = nil
	list.mu.Unlock()

	for _, e := range pending {
		list.logger.Info(e.msg, e.args...)
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
	now := list.now()

	list.mu.RLock()
	defer list.mu.RUnlock()

	out := make([]ExportNode, 0, len(list.nodes))
	for i, n := range list.nodes {
		out = append(out, ExportNode{
			Upstream:      n.backend.Addr,
			Score:         n.score,
			Tier:          n.tier,
			EffectiveTier: list.effectiveTierLocked(i),
			Missing:       n.missing,
			LastProbe:     n.lastProbe(),
			Warm:          n.warm,
			Downed:        n.downed,
			Held:          now.Before(n.heldUntil),
			Stalling:      n.stalling,
			Evictions:     n.evictions,
		})
	}

	return out, nil
}
