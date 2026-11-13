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

package cpview

import (
	"context"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"fencing-agent/internal/adapters/proxyclient"
)

// The own probes use the default parameters of kubernetes-api-proxy, not the
// profile.
const (
	probeInterval  = time.Second
	probeTimeout   = proxyclient.ProbeTimeout
	probeJitter    = 0.2
	failStep       = 500 * time.Millisecond
	failStepWindow = 30 * time.Second
)

// bootMillis returns CLOCK_BOOTTIME in milliseconds. Tests replace it.
var bootMillis = bootClock

// prober probes the control plane with the logic of kubernetes-api-proxy. A
// round probes all targets in parallel, one probe per target. The round
// succeeds when one or more targets are ready. The rounds run one after
// another. A round starts a delay after the start of the previous round, or
// when the previous round ends if it ends later. The number of a round is the
// time of its start by bootMillis, as probeSeq of the proxy, and it always
// grows.
type prober struct {
	ready   func(ctx context.Context, addr string) bool
	targets func() []string

	// started is the number of the last started round.
	started uint64

	mu sync.Mutex
	// seq is the number of the last applied round.
	seq uint64
	ok  bool
	// lastReady is the time of the last ready answer of a target. A ready
	// answer sets it at once, before the end of the round.
	lastReady time.Time
	// failSince is the start of the failure episode: the first round with no
	// ready target after a round with a ready target.
	failSince time.Time
}

// state returns the number and the result of the last round and the time of the last ready answer.
func (p *prober) state() (uint64, bool, time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.seq, p.ok, p.lastReady
}

// run runs the rounds one after another until ctx ends. The first round
// starts at once.
func (p *prober) run(ctx context.Context) {
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
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
		p.started = max(bootMillis(), p.started+1)
		p.round(ctx, p.started)

		timer.Reset(max(p.delay(jitterSpread())-time.Since(start), 0))
	}
}

// round probes all targets and applies the result with the number seq.
func (p *prober) round(ctx context.Context, seq uint64) {
	var (
		wg sync.WaitGroup
		ok atomic.Bool
	)

	for _, addr := range p.targets() {
		wg.Go(func() {
			if p.probe(ctx, addr) {
				ok.Store(true)
				p.markReady(time.Now())
			}
		})
	}
	wg.Wait()

	if ctx.Err() != nil {
		return
	}

	now := time.Now()

	p.mu.Lock()
	defer p.mu.Unlock()

	p.seq = seq
	p.ok = ok.Load()

	switch {
	case p.ok:
		p.failSince = time.Time{}
	case p.failSince.IsZero():
		p.failSince = now
	}
}

// markReady records a ready answer at once. So the age of the answer counts
// from the answer, not from the end of the round, and a slow target in the
// same round does not hide the answer.
func (p *prober) markReady(at time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if at.After(p.lastReady) {
		p.lastReady = at
	}
}

func (p *prober) probe(ctx context.Context, addr string) bool {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	return p.ready(ctx, addr)
}

// delay returns the time from the start of a round to the start of the next
// round. spread is in [-1, 1] and scales the jitter.
func (p *prober) delay(spread float64) time.Duration {
	base := probeInterval
	if p.failStep() {
		base = failStep
	}

	return time.Duration((1 + spread*probeJitter) * float64(base))
}

func (p *prober) failStep() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.failStepLocked(time.Now())
}

func (p *prober) failStepLocked(now time.Time) bool {
	return !p.failSince.IsZero() && now.Sub(p.failSince) < failStepWindow
}

func jitterSpread() float64 {
	return rand.Float64()*2 - 1
}
