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
	"math/rand/v2"
	"slices"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"kubernetes-api-proxy/pkg/kubernetes"
	"kubernetes-api-proxy/pkg/utils"
)

// ErrNoUpstreams is returned from Pick when there are no upstreams considered
// available by the health checker.
var ErrNoUpstreams = fmt.Errorf("no upstreams available")

// ListOption configures a List instance during creation.
type ListOption func(*ListConfig) error

// WithHealthcheckInterval configures healthcheck interval.
func WithHealthcheckInterval(interval time.Duration) ListOption {
	return func(l *ListConfig) error {
		l.healthcheckInterval = interval

		return nil
	}
}

// WithHealthCheckJitter configures healthcheck jitter (0.0 - 1.0).
func WithHealthCheckJitter(jitter float64) ListOption {
	return func(l *ListConfig) error {
		if jitter < 0 || jitter > 1 {
			return fmt.Errorf("healthcheck jitter should in range [0, 1]: %f", jitter)
		}

		l.healthcheckJitter = jitter

		return nil
	}
}

// WithHealthcheckTimeout configures healthcheck timeout (for each backend).
func WithHealthcheckTimeout(timeout time.Duration) ListOption {
	return func(l *ListConfig) error {
		l.healthcheckTimeout = timeout

		return nil
	}
}

func WithKubernetesConfigGetter(getter kubernetes.ClusterConfigGetter) ListOption {
	return func(l *ListConfig) error {
		l.kubernetesConfigGetter = getter

		return nil
	}
}

func WithLogger(logger *log.Logger) ListOption {
	return func(l *ListConfig) error {
		l.logger = logger

		return nil
	}
}

// Tier represents a latency tier for an upstream. Lower tiers are faster.
type Tier int

// List of upstream Backends with healthchecks and different strategies to pick a node.
//
// List keeps track of Backends with score. Score is updated on health checks and via external
// interface (e.g., when the actual connection fails).
//
// The initial score is set via options (default is +1). Low and high scores defaults are (-3, +3).
// Backend score is limited by low and high scores. Each time healthcheck fails score is adjusted
// by fail delta score, and every successful check updates the score by success score delta (defaults are -1/+1).
//
// Backend might be used if its score is not negative.
type List struct { //nolint:govet
	// List maintains a set of Upstreams, periodically performs health checks,
	// and selects a backend for proxying using score- and tier-influenced
	// round-robin. Higher scores and lower tiers are preferred.
	listConfig

	cmp func(*Upstream, *Upstream) bool

	// The following fields are protected by mutex
	mu      sync.RWMutex
	nodes   []node
	current int
}

// ListConfig is a configuration for List. It is separated from List to allow
// usage of functional options without exposing type in their API.
type ListConfig struct {
	logger *log.Logger

	kubernetesConfigGetter kubernetes.ClusterConfigGetter

	healthcheckInterval time.Duration
	healthcheckTimeout  time.Duration

	healthWg        sync.WaitGroup
	healthCtxCancel context.CancelFunc

	healthcheckJitter float64

	bestTier, worstTier, initTier Tier
}

const (
	lowestScore  = -5.0
	highestScore = 3.0
	initialScore = 1.0

	failScoreDelta    = -1.0
	successScoreDelta = 1.0
)

// This allows us to hide the embedded struct from public access.
type listConfig = ListConfig

func NewList(upstreams []*Upstream, options ...ListOption) (*List, error) {
	return NewListWithCmp(
		upstreams,
		func(a, b *Upstream) bool { return a.Addr == b.Addr },
		options...,
	)
}

// NewListWithCmp initializes a new list with upstream backends and options and starts health checks.
//
// List should be stopped with `.Shutdown()`.
func NewListWithCmp(
	upstreams []*Upstream,
	cmp func(*Upstream, *Upstream) bool,
	options ...ListOption,
) (*List, error) {
	// initialize with defaults
	list := &List{
		listConfig: listConfig{
			healthcheckInterval: 1 * time.Second,
			healthcheckTimeout:  100 * time.Millisecond,
			bestTier:            0,
			worstTier:           Tier(len(tierBucket) - 1),
			initTier:            0,
		},

		cmp:     cmp,
		current: -1,
	}

	var ctx context.Context

	ctx, list.healthCtxCancel = context.WithCancel(context.Background())

	for _, opt := range options {
		if err := opt(&list.listConfig); err != nil {
			return nil, err
		}
	}

	if upstreams == nil {
		upstreams = []*Upstream{}
	}

	list.nodes = make([]node, len(upstreams))
	for i, upstream := range upstreams {
		list.nodes[i] = node{
			backend: upstream,
			score:   initialScore,
			tier:    list.initTier,
		}
	}

	list.healthWg.Add(1)

	go list.healthcheck(ctx)

	return list, nil
}

// Reconcile the list of backends with a passed list.
//
// Any new backends are added with the initial score,
// score is untouched for backends which haven't changed their score.
func (list *List) Reconcile(newList []*Upstream) {
	list.mu.Lock()
	defer list.mu.Unlock()

	if newList == nil {
		newList = []*Upstream{}
	}

	// Reconcile existing nodes with newList.
	// For nodes missing from newList: demote (worsen).
	// If the score reaches the lowest allowed (lowScore), drop the node.
	kept := make([]node, 0, len(list.nodes))

	for _, n := range list.nodes {
		present := slices.ContainsFunc(newList, func(u *Upstream) bool { return list.cmp(u, n.backend) })
		if present {
			// keep as-is
			kept = append(kept, n)
			continue
		}

		// Not present in new list: demote score, or remove if already at bottom
		if n.score > lowestScore {
			list.Down(n.backend)
		} else {
			continue
		}

		kept = append(kept, n)
	}

	list.nodes = kept

	for _, newB := range newList {
		// skip changing nodes that already been in list.nodes
		if slices.ContainsFunc(list.nodes, func(b node) bool { return list.cmp(newB, b.backend) }) {
			continue
		}

		newB.UseKubernetesConfigGetter(
			list.kubernetesConfigGetter,
		)

		list.nodes = append(list.nodes, node{
			backend: newB,
			score:   initialScore,
			tier:    list.initTier,
		})
	}
}

// Shutdown stops healthchecks.
func (list *List) Shutdown() {
	list.healthCtxCancel()

	list.healthWg.Wait()
}

// Up increases backend score by success score delta.
func (list *List) Up(upstream *Upstream) {
	list.upWithTier(upstream, -1)
}

func (list *List) upWithTier(upstream *Upstream, newTier Tier) {
	for i := range list.nodes {
		if list.cmp(list.nodes[i].backend, upstream) {
			list.nodes[i].score += successScoreDelta
			list.updateNodeTier(i, newTier)

			if list.nodes[i].score > highestScore {
				list.nodes[i].score = highestScore
			}
		}
	}
}

func (list *List) updateNodeTier(i int, newTier Tier) {
	switch {
	case newTier == -1:
		// do nothing, keep the old tier
		return
	case newTier < list.bestTier:
		newTier = list.bestTier
	case newTier > list.worstTier:
		newTier = list.worstTier
	}

	list.nodes[i].tier = newTier
}

// Down decreases backend score by fail score delta.
func (list *List) Down(upstream *Upstream) {
	list.downWithTier(upstream, -1)
}

func (list *List) downWithTier(upstream *Upstream, newTier Tier) {
	for i := range list.nodes {
		if list.cmp(list.nodes[i].backend, upstream) {
			list.nodes[i].score += failScoreDelta
			list.updateNodeTier(i, newTier)

			if list.nodes[i].score < lowestScore {
				list.nodes[i].score = lowestScore
			}
		}
	}
}

// Pick returns the next backend to be used.
//
// The default policy is to pick a healthy (non-negative score) backend in
// round-robin fashion.
func (list *List) Pick() (*Upstream, error) {
	list.mu.RLock()
	defer list.mu.RUnlock()

	nodes := list.nodes

	for tier := list.bestTier; tier <= list.worstTier; tier++ {
		for j := range nodes {
			i := (list.current + 1 + j) % len(nodes)

			if nodes[i].tier == tier && nodes[i].score >= 0 {
				list.current = i

				return nodes[list.current].backend, nil
			}
		}
	}

	return nil, ErrNoUpstreams
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

	nodes := make([]string, 0, len(list.nodes))
	for _, node := range list.nodes {
		nodes = append(nodes, node.backend.Address())
	}

	return nodes
}

func (list *List) ExportNodes() ([]ExportNode, error) {
	list.mu.RLock()
	defer list.mu.RUnlock()

	nodes := list.nodes

	exportNodes := make([]ExportNode, 0, len(nodes))
	for _, node := range nodes {
		exportNodes = append(exportNodes, ExportNode{
			Upstream: node.backend.Addr,
			Score:    node.score,
			Tier:     node.tier,
		})
	}

	return exportNodes, nil
}

func (list *List) healthcheck(ctx context.Context) {
	defer list.healthWg.Done()

	list.doHealthCheck(ctx)

	initialInterval := list.healthcheckInterval
	if list.healthcheckJitter > 0 {
		initialInterval = time.Duration(rand.Float64() * float64(list.healthcheckInterval))
	}

	timer := time.NewTimer(initialInterval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		list.doHealthCheck(ctx)

		nextInterval := time.Duration(((rand.Float64()*2-1)*list.healthcheckJitter + 1.0) * float64(list.healthcheckInterval))

		timer.Reset(nextInterval)
	}
}

func (list *List) doHealthCheck(ctx context.Context) {
	list.mu.RLock()
	backends := utils.Map(list.nodes, func(n node) *Upstream { return n.backend })
	list.mu.RUnlock()

	for _, backend := range backends {
		if ctx.Err() != nil {
			return
		}

		func() {
			localCtx, ctxCancel := context.WithTimeout(ctx, list.healthcheckTimeout)
			defer ctxCancel()

			if newTier, err := backend.HealthCheck(localCtx); err != nil {
				list.downWithTier(backend, newTier)
			} else {
				list.upWithTier(backend, newTier)
			}
		}()
	}
}
