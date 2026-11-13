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

package writer

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/domain"
)

// Proven tells whether ffns is in the phase Proven or a later phase. Then the
// armed agent of the node stops the feed of its watchdog.
func Proven(ffns *v1alpha1.FencingFailedNodeState) bool {
	if ffns == nil {
		return false
	}

	switch ffns.Status.Phase {
	case v1alpha1.PhaseProven, v1alpha1.PhaseQueued, v1alpha1.PhaseEvicting, v1alpha1.PhaseEvicted:
		return true
	default:
		return false
	}
}

// SetOwn takes each version of the FFNS of the own node, and nil when it does
// not exist. The writer does not change ffns.
func (w *Writer) SetOwn(ffns *v1alpha1.FencingFailedNodeState) {
	w.mu.Lock()
	defer w.mu.Unlock()

	present := ffns != nil

	switch {
	case w.own && !present:
		// The FFNS is gone, or the first report tells that it does not exist.
		w.ownGoneAt = time.Now()
		w.ownSince, w.heartbeatAt, w.recreated = time.Time{}, time.Time{}, false
	case !w.own && present:
		w.ownSince = time.Now()
	}

	if present && w.ownFFNS == nil {
		w.wakeHeartbeat()
	}

	w.own, w.ownKnown, w.ownFFNS = present, true, ffns
	w.hygiene()
}

// OwnCP applies the FFNS of the own node to the control plane state of the
// decision. Until the agent knows its FFNS, and while the FFNS that existed
// at the start has no successful heartbeat, the state has no data about the
// control plane. While the FFNS exists, the control plane is visible only
// when the last successful heartbeat is not older than controlPlaneLostAfter.
// For an FFNS that appeared after the start, the count starts when it appeared.
func (w *Writer) OwnCP(cp domain.CPState) domain.CPState {
	w.mu.Lock()
	defer w.mu.Unlock()

	switch {
	case !w.ready():
		cp.Seen, cp.Visible = false, false
	case w.ownFFNS != nil:
		last := w.heartbeatAt
		if w.ownSince.After(last) {
			last = w.ownSince
		}

		if time.Since(last) > w.deps.Profile().ControlPlaneLostAfter.Duration {
			cp.Visible = false
		}
	}

	return cp
}

// Ready tells whether the agent knows its FFNS and, when the FFNS existed at
// the start, wrote a heartbeat in it. Before it the agent does not feed the
// watchdog and does not disarm the watchdog of the previous process.
func (w *Writer) Ready() bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.ready()
}

// ready is Ready. Call it with mu held.
func (w *Writer) ready() bool {
	return w.ownKnown && (w.ownFFNS == nil || !w.heartbeatAt.IsZero() || !w.ownSince.IsZero())
}

// wakeHeartbeat starts a heartbeat at once. Call it with mu held.
func (w *Writer) wakeHeartbeat() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// heartbeats writes the heartbeat in the FFNS of the own node once in
// heartbeat.interval from the start of the previous heartbeat, and at once
// when the FFNS appears and when the control plane becomes visible.
func (w *Writer) heartbeats(ctx context.Context) {
	for {
		start := time.Now()
		w.heartbeat(ctx)

		interval := w.deps.Profile().Heartbeat.Interval.Duration
		if interval <= 0 {
			interval = time.Second
		}

		t := time.NewTimer(max(interval-time.Since(start), 0))

		select {
		case <-ctx.Done():
			t.Stop()

			return
		case <-w.wake:
		case <-t.C:
		}

		t.Stop()
	}
}

// heartbeat writes status.fallback.lastHeartbeatAt in the FFNS of the own node
// while the control plane is visible. A write that fails before
// heartbeat.apiTimeout, for example with a conflict after a write of the
// controller, is done again after pauses[i], up to len(pauses) times. Each
// write takes the last version of the FFNS that the agent knows.
func (w *Writer) heartbeat(ctx context.Context) {
	for i := 0; ; i++ {
		start := time.Now()

		wrote, err := w.writeHeartbeat(ctx)
		if err == nil {
			if wrote && i > 0 {
				w.logger.Info("heartbeat written in the own FFNS after a failed write", "attempt", i+1)
			}

			return
		}

		if i == len(pauses) || time.Since(start) >= w.deps.Profile().Heartbeat.APITimeout.Duration {
			w.logger.Warn("write the heartbeat in the own FFNS", "attempt", i+1, "error", err, "next", "after heartbeat.interval")

			return
		}

		w.logger.Warn("write the heartbeat in the own FFNS", "attempt", i+1, "error", err, "next", "retry in "+pauses[i].String())

		if !sleep(ctx, pauses[i]) {
			return
		}
	}
}

// writeHeartbeat is one write of the heartbeat. The write has the
// resourceVersion of the last version of the FFNS that the agent knows: it
// fails when the controller changed the FFNS after it. There is no heartbeat
// after the final stop of the feed of the armed watchdog and in the phase
// Proven or a later phase. It tells whether it wrote the heartbeat.
func (w *Writer) writeHeartbeat(ctx context.Context) (bool, error) {
	p := w.deps.Profile().Heartbeat

	w.mu.Lock()
	base := w.ownFFNS
	allowed := base != nil && !w.cpSince.IsZero() && !w.stopped && p.Interval.Duration > 0 && !Proven(base)
	w.mu.Unlock()

	if !allowed || w.deps.Final() {
		return false, nil
	}

	at := time.Now()
	sent := metav1.NewMicroTime(at)
	ffns := base.DeepCopy()
	ffns.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{LastHeartbeatAt: &sent}

	actx, cancel := context.WithTimeout(ctx, p.APITimeout.Duration)
	err := w.deps.API.PatchStatus(actx, base, ffns)

	cancel()

	if err != nil {
		return false, err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.ownFFNS == nil {
		return true, nil
	}

	// The watch can deliver the written version later.
	if w.ownFFNS == base {
		w.ownFFNS = ffns
	}

	w.heartbeatAt = at

	return true, nil
}

// hygiene recreates the memberlist instance of the main network once for each
// FFNS of the own node, when the control plane is visible. The old instance
// does not spread its Dead states to the peers. Call it with mu held.
func (w *Writer) hygiene() {
	if w.ownFFNS == nil || w.cpSince.IsZero() || w.recreated || w.stopped || w.ctx == nil {
		return
	}

	w.recreated = true
	w.spawn(w.deps.Network.Recreate)
}
