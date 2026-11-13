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
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/config"
	"fencing-agent/internal/domain"
)

var visible = domain.CPState{Visible: true, Seen: true}

// ownCluster runs the writer w-0 with the peer w-1 down.
func ownCluster(t *testing.T, opts clusterOptions) (*cluster, *Writer) {
	t.Helper()

	all := names("w", 2)
	opts.down = all[1:]
	c := newCluster(t, all, opts, armedMeta)

	return c, c.writers["w-0"]
}

// putOwn creates the FFNS of w-0 in the phase and returns the version that
// the watch delivers.
func putOwn(c *cluster, phase v1alpha1.FencingFailedNodeStatePhase) *v1alpha1.FencingFailedNodeState {
	ffns := &v1alpha1.FencingFailedNodeState{ObjectMeta: metav1.ObjectMeta{Name: "w-0"}}
	ffns.Status.Phase = phase

	c.api.mu.Lock()
	defer c.api.mu.Unlock()

	c.api.put(ffns)

	return ffns.DeepCopy()
}

func heartbeats(c *cluster) int {
	n := 0

	for _, p := range c.api.patched() {
		if p.Status.Fallback != nil && p.Status.Fallback.LastHeartbeatAt != nil {
			n++
		}
	}

	return n
}

// The FFNS appears while the control plane is visible: a heartbeat at once and
// then each heartbeat.interval, on the last version, with CAS.
func TestHeartbeatEachInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, w := ownCluster(t, clusterOptions{})

		w.SetOwn(putOwn(c, v1alpha1.PhaseFallbackAlive))
		synctest.Wait()

		if got := heartbeats(c); got != 1 {
			t.Fatalf("heartbeats = %d at the appearance, want 1", got)
		}

		time.Sleep(3500 * ms)

		if got := heartbeats(c); got != 4 {
			t.Fatalf("heartbeats = %d in 3.5 s, want 4", got)
		}

		last, _ := c.api.get("w-0")
		if last.Status.Phase != v1alpha1.PhaseFallbackAlive {
			t.Errorf("phase = %q after the heartbeats, want FallbackAlive", last.Status.Phase)
		}

		if !w.OwnCP(visible).Visible {
			t.Error("the control plane is not visible with fresh heartbeats")
		}
	})
}

// No heartbeat while the control plane is not visible, also for an agent
// without watchdog, and no heartbeat after the final stop of the feed of the
// armed watchdog.
func TestNoHeartbeat(t *testing.T) {
	tests := []struct {
		name string
		set  func(w *Writer, final *atomic.Bool)
	}{
		{"control plane is not visible", func(w *Writer, _ *atomic.Bool) { w.ObserveCP(domain.CPState{Seen: true}) }},
		{"the stop of the feed is final", func(_ *Writer, final *atomic.Bool) { final.Store(true) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				final := &atomic.Bool{}
				c, w := ownCluster(t, clusterOptions{final: final})

				tt.set(w, final)
				w.SetOwn(putOwn(c, ""))
				time.Sleep(10 * s)

				if got := heartbeats(c); got != 0 {
					t.Fatalf("heartbeats = %d, want 0", got)
				}
			})
		})
	}
}

// The heartbeat fails longer than controlPlaneLostAfter: the control plane of
// the decision is lost. It is visible again after a successful heartbeat.
func TestHeartbeatFailureLosesTheControlPlane(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, w := ownCluster(t, clusterOptions{})

		var fail atomic.Bool

		c.api.onPatch = func(*v1alpha1.FencingFailedNodeState) error {
			if fail.Load() {
				return errors.New("connection refused")
			}

			return nil
		}

		w.SetOwn(putOwn(c, ""))
		synctest.Wait()
		fail.Store(true)
		last := time.Now()

		time.Sleep(4100 * ms)

		if !w.OwnCP(visible).Visible {
			t.Fatal("the control plane is lost 4.1 s after the last heartbeat, want ≤ controlPlaneLostAfter 4.15 s")
		}

		time.Sleep(last.Add(4200 * ms).Sub(time.Now()))

		if got := w.OwnCP(visible); got.Visible || !got.Seen {
			t.Fatalf("OwnCP = %+v 4.2 s after the last heartbeat, want the control plane lost", got)
		}

		fail.Store(false)
		time.Sleep(s)

		if !w.OwnCP(visible).Visible {
			t.Fatal("the control plane is not visible after a successful heartbeat")
		}
	})
}

// The FFNS appears after the start: the count of controlPlaneLostAfter starts
// when it appears, so the first heartbeat can be written.
func TestNewFFNSGivesTimeForTheFirstHeartbeat(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, w := ownCluster(t, clusterOptions{})
		c.api.onPatch = func(*v1alpha1.FencingFailedNodeState) error { return errors.New("timeout") }

		w.SetOwn(putOwn(c, ""))
		appeared := time.Now()

		if !w.OwnCP(visible).Visible {
			t.Fatal("the control plane is lost when the FFNS appears")
		}

		time.Sleep(appeared.Add(4200 * ms).Sub(time.Now()))

		if w.OwnCP(visible).Visible {
			t.Fatal("the control plane is visible 4.2 s after the FFNS appeared without a heartbeat")
		}
	})
}

// The FFNS exists at the start: the agent has no data about the control plane
// until a successful heartbeat. Until the agent knows its FFNS, it also has no data.
func TestStartWithOwnFFNSNeedsAHeartbeat(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		api := newFakeAPI()
		w := New(Deps{
			Self: "w-0", API: api, Network: &fakeNet{c: &cluster{names: []string{"w-0"}}, self: "w-0"},
			Settings:    func() config.Settings { return settings(config.ModeCritical) },
			Maintenance: func() bool { return false },
			Profile:     decisionProfile, Final: func() bool { return false },
		}, log.NewNop())

		go func() { _ = w.Run(t.Context()) }()

		if got := w.OwnCP(visible); got.Seen || got.Visible || w.Ready() {
			t.Fatalf("OwnCP = %+v, Ready = %t before the FFNS is known, want no data", got, w.Ready())
		}

		var fail atomic.Bool

		fail.Store(true)
		api.onPatch = func(*v1alpha1.FencingFailedNodeState) error {
			if fail.Load() {
				return errors.New("timeout")
			}

			return nil
		}

		ffns := &v1alpha1.FencingFailedNodeState{ObjectMeta: metav1.ObjectMeta{Name: "w-0"}}
		api.add(ffns)
		w.SetOwn(ffns.DeepCopy())
		w.ObserveCP(visible)
		time.Sleep(3 * s)

		if got := w.OwnCP(visible); got.Seen || w.Ready() {
			t.Fatalf("OwnCP = %+v, Ready = %t without a heartbeat, want no data", got, w.Ready())
		}

		fail.Store(false)
		time.Sleep(1500 * ms)

		if got := w.OwnCP(visible); !got.Visible || !w.Ready() {
			t.Fatalf("OwnCP = %+v, Ready = %t after the heartbeat, want the control plane visible", got, w.Ready())
		}
	})
}

// The FFNS is proven at the start: no heartbeat, the agent does not become ready.
func TestStartWithProvenFFNS(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, w := ownCluster(t, clusterOptions{})

		// The first report after a restart is the proven FFNS.
		w.mu.Lock()
		w.own, w.ownKnown = true, false
		w.mu.Unlock()

		w.SetOwn(putOwn(c, v1alpha1.PhaseProven))
		time.Sleep(5 * s)

		if got := heartbeats(c); got != 0 || w.Ready() {
			t.Fatalf("heartbeats = %d, Ready = %t, want no heartbeat in a proven FFNS", got, w.Ready())
		}
	})
}

// The controller changed the FFNS after the version that the agent knows: the
// heartbeat fails and does not count.
func TestHeartbeatIsCAS(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, w := ownCluster(t, clusterOptions{})

		old := putOwn(c, "")
		putOwn(c, v1alpha1.PhaseProven)

		w.SetOwn(old)
		time.Sleep(5 * s)

		if got := heartbeats(c); got != 0 {
			t.Fatalf("heartbeats = %d on an old version, want 0", got)
		}

		if w.OwnCP(visible).Visible {
			t.Fatal("the control plane is visible without a successful heartbeat")
		}
	})
}

// Two reports of the own FFNS in one episode give one recreation of
// memberlist. A new FFNS is a new episode.
func TestHygieneOncePerEpisode(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, w := ownCluster(t, clusterOptions{})
		net := c.nets["w-0"]

		w.SetOwn(putOwn(c, ""))
		w.SetOwn(putOwn(c, v1alpha1.PhaseFallbackAlive))
		w.ObserveCP(visible)
		synctest.Wait()

		if got := net.recreated(); got != 1 {
			t.Fatalf("recreations = %d in one episode, want 1", got)
		}

		w.SetOwn(nil)
		w.SetOwn(putOwn(c, ""))
		synctest.Wait()

		if got := net.recreated(); got != 2 {
			t.Fatalf("recreations = %d after a new FFNS, want 2", got)
		}
	})
}

// The agent learns of its FFNS without the control plane: it recreates
// memberlist when the control plane is visible.
func TestHygieneWaitsForTheControlPlane(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, w := ownCluster(t, clusterOptions{})
		net := c.nets["w-0"]

		w.ObserveCP(domain.CPState{Seen: true})
		w.SetOwn(putOwn(c, ""))
		synctest.Wait()

		if got := net.recreated(); got != 0 {
			t.Fatalf("recreations = %d without the control plane, want 0", got)
		}

		w.ObserveCP(visible)
		w.ObserveCP(visible)
		synctest.Wait()

		if got := net.recreated(); got != 1 {
			t.Fatalf("recreations = %d, want 1", got)
		}
	})
}

func TestProvenPhases(t *testing.T) {
	for phase, want := range map[v1alpha1.FencingFailedNodeStatePhase]bool{
		"":                          false,
		v1alpha1.PhaseSuspected:     false,
		v1alpha1.PhaseFallbackAlive: false,
		v1alpha1.PhaseUnproven:      false,
		v1alpha1.PhaseProven:        true,
		v1alpha1.PhaseQueued:        true,
		v1alpha1.PhaseEvicting:      true,
		v1alpha1.PhaseEvicted:       true,
	} {
		ffns := &v1alpha1.FencingFailedNodeState{Status: v1alpha1.FencingFailedNodeStateStatus{Phase: phase}}
		if got := Proven(ffns); got != want {
			t.Errorf("Proven(%q) = %t, want %t", phase, got, want)
		}
	}

	if Proven(nil) {
		t.Error("Proven(nil) = true, want false")
	}
}

// The agent restarts with its FFNS: the first heartbeat goes at once when the
// control plane becomes visible, not one heartbeat.interval later.
func TestHeartbeatAtOnceWhenTheControlPlaneIsVisible(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, w := ownCluster(t, clusterOptions{})

		w.ObserveCP(domain.CPState{Seen: true})
		w.SetOwn(putOwn(c, ""))
		time.Sleep(300 * ms)

		w.ObserveCP(visible)
		synctest.Wait()

		if got := heartbeats(c); got != 1 {
			t.Fatalf("heartbeats = %d at once after the control plane is visible, want 1", got)
		}
	})
}

// The controller writes the FFNS just before the first heartbeat: the
// heartbeat gets a conflict. The watch gives the new version a few
// milliseconds later. The agent writes the heartbeat again at once, not after
// heartbeat.interval.
func TestHeartbeatAgainAfterAConflict(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, w := ownCluster(t, clusterOptions{})

		old := putOwn(c, "")
		latest := putOwn(c, v1alpha1.PhaseSuspected)

		w.SetOwn(old)
		time.Sleep(6 * ms)
		w.SetOwn(latest)
		time.Sleep(300 * ms)

		if got := heartbeats(c); got != 1 {
			t.Fatalf("heartbeats = %d 0.3 s after the conflict, want 1", got)
		}
	})
}

// A slow write does not move the next heartbeat: heartbeat.interval counts
// from the start of the previous heartbeat.
func TestHeartbeatIntervalFromTheStart(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, w := ownCluster(t, clusterOptions{})

		c.api.onPatch = func(*v1alpha1.FencingFailedNodeState) error {
			time.Sleep(800 * ms)

			return nil
		}

		w.SetOwn(putOwn(c, ""))
		time.Sleep(3900 * ms)

		if got := heartbeats(c); got != 4 {
			t.Fatalf("heartbeats = %d in 3.9 s with writes of 0.8 s, want 4: one in each interval of 1 s", got)
		}
	})
}
