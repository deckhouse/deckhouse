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
	"log/slog"
	"time"
)

func (list *List) round(ctx context.Context) {
	backends := list.backends()
	out := probeAll(ctx, backends, list.healthcheckTimeout, list.probe, list.logger)

	if ctx.Err() != nil {
		return
	}

	now := list.now()

	list.mu.Lock()
	stalls := list.applyRoundLocked(backends, out, now)
	list.unlockAndLog()

	for _, b := range stalls {
		list.startStall(ctx, b, now)
	}
}

func (list *List) applyRoundLocked(backends []*Upstream, out []outcome, now time.Time) []*Upstream {
	list.markWarmLocked(backends, out)

	nobodyReady := list.switchModeLocked(list.voteLocked(backends, out))

	for k, b := range backends {
		if out[k].ok {
			list.applyLocked(b, out[k].value, nobodyReady, now)
		}
	}

	return list.stallsLocked(backends, out)
}

func (list *List) stallsLocked(backends []*Upstream, out []outcome) []*Upstream {
	if list.stallTimeout <= 0 {
		return nil
	}

	var stalls []*Upstream

	for k, b := range backends {
		i := list.indexLocked(b)
		if i < 0 || !out[k].ok || out[k].value.kind.answered() || list.nodes[i].stalling {
			continue
		}

		list.nodes[i].stalling = true

		stalls = append(stalls, b)
	}

	return stalls
}

func (list *List) markWarmLocked(backends []*Upstream, out []outcome) {
	for k, b := range backends {
		i := list.indexLocked(b)
		if i < 0 || !out[k].ok {
			continue
		}

		list.markWarmNodeLocked(i, out[k].value.kind)
	}
}

func (list *List) markWarmNodeLocked(i int, kind probeKind) {
	n := &list.nodes[i]
	restarted := kind == probeNotReady && n.unsettled
	n.unsettled = kind == probeHard || (n.unsettled && kind == probeSoft)

	if kind == probeReady {
		n.warm = false
	}

	if restarted && !n.warm {
		n.warm = true
		list.noteLocked("upstream warming up", n.backend)
	}
}

type vote struct {
	voters, ready, notReady int
}

func (v vote) nobodyReady() bool {
	return v.ready == 0 && v.voters > 0 && v.notReady*2 > v.voters
}

func (list *List) voteLocked(backends []*Upstream, out []outcome) vote {
	var v vote

	for k, b := range backends {
		if !out[k].ok || !list.votesLocked(b, out[k].value.kind) {
			continue
		}

		v.voters++

		switch out[k].value.kind {
		case probeReady:
			v.ready++
		case probeNotReady:
			v.notReady++
		}
	}

	return v
}

func (list *List) votesLocked(u *Upstream, kind probeKind) bool {
	i := list.indexLocked(u)
	if i < 0 || kind == probeHard {
		return false
	}

	return list.nodes[i].missing == 0 && !list.nodes[i].warm
}

func (list *List) switchModeLocked(v vote) bool {
	if v.voters == 0 {
		return list.nobodyReady
	}

	nobodyReady := v.nobodyReady()
	if nobodyReady == list.nobodyReady {
		return nobodyReady
	}

	list.nobodyReady = nobodyReady

	msg := "not-ready upstreams lose priority again"
	if nobodyReady {
		msg = "no upstream ready, balancing not-ready upstreams by latency"
	}

	list.noteListLocked(msg, slog.Int("voters", v.voters), slog.Int("not_ready", v.notReady))

	return nobodyReady
}

func (list *List) applyLocked(u *Upstream, r probeResult, nobodyReady bool, now time.Time) {
	i := list.indexLocked(u)
	if i < 0 {
		return
	}

	penalized := false

	switch r.kind {
	case probeReady:
		list.restoreTierLocked(i, r.latency, now)
		list.adjustScoreLocked(i, successScoreDelta)
	case probeNotReady:
		penalized = list.applyNotReadyLocked(i, r.latency, nobodyReady, now)
	case probeSoft:
		list.failTierLocked(i, r.kind.String())
	default:
		list.failTierLocked(i, r.kind.String())
		list.adjustScoreLocked(i, failScoreDelta)
	}

	list.recordProbeLocked(i, r.kind, penalized, now)
}

func (list *List) applyNotReadyLocked(i int, latency time.Duration, nobodyReady bool, now time.Time) bool {
	switch {
	case list.nodes[i].warm:
		list.failTierLocked(i, "warming")
		list.adjustScoreLocked(i, failScoreDelta)
	case nobodyReady:
		list.restoreTierLocked(i, latency, now)

		return false
	default:
		list.failTierLocked(i, probeNotReady.String())
		list.adjustScoreLocked(i, failScoreDelta)
	}

	return true
}

func (list *List) recordProbeLocked(i int, kind probeKind, penalized bool, now time.Time) {
	n := &list.nodes[i]
	n.penalized = penalized
	n.lastKind = kind
	n.downed = false

	switch {
	case kind.answered():
		list.answerLocked(i, kind, now)
	case kind == probeHard:
		n.readySince = time.Time{}
	}
}

func (list *List) answerLocked(i int, kind probeKind, now time.Time) {
	n := &list.nodes[i]
	n.answeredAt = now

	switch {
	case kind != probeReady:
		n.readySince = time.Time{}
	case n.readySince.IsZero():
		n.readySince = now
	}
}

func (list *List) restoreTierLocked(i int, latency time.Duration, now time.Time) {
	n := &list.nodes[i]
	if now.Before(n.heldUntil) {
		return
	}

	if n.tier == tierFailed {
		list.noteLocked("upstream regained priority", n.backend)
	}

	n.tier = calcTier(latency)
}

func (list *List) failTierLocked(i int, reason string) {
	n := &list.nodes[i]
	if n.tier == tierFailed {
		return
	}

	n.tier = tierFailed
	list.noteLocked("upstream lost priority", n.backend, slog.String("reason", reason))
}

func (list *List) adjustScoreLocked(i int, delta float64) {
	n := &list.nodes[i]
	before := n.score
	n.score = min(max(n.score+delta, lowestScore), highestScore)

	switch {
	case before >= 0 && n.score < 0:
		list.noteLocked("upstream excluded", n.backend)
	case before < 0 && n.score >= 0:
		list.noteLocked("upstream included", n.backend)
	}
}
