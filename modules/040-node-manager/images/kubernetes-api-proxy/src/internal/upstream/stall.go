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
	"errors"
	"log/slog"
	"time"
)

const noWitness = "none"

func (list *List) OnStall(fn func(*Upstream)) {
	list.mu.Lock()
	defer list.mu.Unlock()

	// ponytail: one subscriber, the single route ServeRoute installs; make it a slice when a second route appears.
	list.onStall = fn
}

func (list *List) startStall(ctx context.Context, b *Upstream, start time.Time) {
	if list.stallHook != nil {
		list.stallHook(b, start)

		return
	}

	list.healthWg.Go(func() { list.stallProbe(ctx, b, start) })
}

func (list *List) stallProbe(ctx context.Context, b *Upstream, start time.Time) {
	stallCtx, cancel := context.WithTimeout(ctx, list.stallTimeout)
	defer cancel()

	r, err := list.probe(stallCtx, b)

	if ctx.Err() != nil {
		return
	}

	if err != nil || !r.kind.answered() {
		r = probeResult{}
	}

	timedOut := !r.kind.answered() && errors.Is(stallCtx.Err(), context.DeadlineExceeded)
	list.stallDone(b, start, r, timedOut)
}

func (list *List) stallDone(b *Upstream, start time.Time, r probeResult, timedOut bool) {
	now := list.now()
	stalled := timedOut && !r.kind.answered()

	list.mu.Lock()
	witness := list.recordStallLocked(b, start, r, stalled, now)
	hook := list.onStall
	list.unlockAndLog()

	if stalled {
		list.logger.Debug("upstream stalled", slog.String("upstream", b.Addr), slog.String("witness", witnessAddr(witness)))
	}

	if witness != nil && hook != nil {
		hook(b)
	}
}

func (list *List) recordStallLocked(b *Upstream, start time.Time, r probeResult, stalled bool, now time.Time) *Upstream {
	i := list.indexLocked(b)
	if i < 0 {
		return nil
	}

	list.nodes[i].stalling = false

	switch {
	case r.kind.answered():
		list.answerLocked(i, r.kind, now)

		if r.kind == probeReady {
			list.readyLocked(i, now)
		}
	case stalled:
		return list.stalledLocked(i, start, now)
	}

	return nil
}

func (list *List) stalledLocked(i int, start, now time.Time) *Upstream {
	n := &list.nodes[i]
	n.readySince = time.Time{}

	if !n.answeredAt.Before(start) || list.nobodyReady {
		return nil
	}

	witness := list.witnessLocked(i, start, now)
	if witness != nil {
		n.evictions++
	}

	return witness
}

func (list *List) witnessLocked(x int, start, now time.Time) *Upstream {
	window := 2 * list.stallTimeout

	for i, n := range list.nodes {
		if i != x && n.witnesses(start, now, window) {
			return n.backend
		}
	}

	return nil
}

func witnessAddr(u *Upstream) string {
	if u == nil {
		return noWitness
	}

	return u.Addr
}
