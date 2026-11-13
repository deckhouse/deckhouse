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
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/domain"
)

// The only writer saw the control plane come back 7 s before the death of
// the victim: it does not pass the health gate of 7.4 s at the death. It
// creates the FFNS after the gate, on the next writerWaveInterval.
func TestCreateAfterTheHealthGate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		all := names("w", 10)
		c := newCluster(t, all, clusterOptions{down: all[1:]}, armedMeta)
		w := c.writers["w-0"]
		w.ObserveCP(domain.CPState{Seen: true})
		w.ObserveCP(domain.CPState{Visible: true, Seen: true})
		back := time.Now()
		time.Sleep(7 * s)
		victim := c.firstWave("w-0")[0]
		c.die(victim)
		time.Sleep(60 * s)

		calls := c.api.calls()
		if len(calls) != 1 || len(c.api.objs) != 1 {
			t.Fatalf("creates %d, objects %d; want one FFNS after the gate", len(calls), len(c.api.objs))
		}

		if d := calls[0].at.Sub(back); d < 7400*ms || d > 7400*ms+2*s {
			t.Errorf("create %v after the control plane came back, want after the gate of 7.4 s and not later than one writerWaveInterval", d)
		}
	})
}

// The victim comes back while the writer waits for the health gate: the
// retry stops, and no FFNS comes.
func TestCreateRetryStopsWhenThePeerComesBack(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		all := names("w", 10)
		c := newCluster(t, all, clusterOptions{down: all[1:]}, armedMeta)
		w := c.writers["w-0"]
		w.ObserveCP(domain.CPState{Seen: true})
		w.ObserveCP(domain.CPState{Visible: true, Seen: true})
		time.Sleep(1 * s)
		victim := c.firstWave("w-0")[0]
		c.die(victim)
		time.Sleep(2 * s)
		c.back(victim, armedMeta(victim))
		time.Sleep(60 * s)

		if n := len(c.api.calls()); n != 0 {
			t.Errorf("creates %d after the return, want 0", n)
		}
	})
}

// The server stores the FFNS, but the create times out. The only writer that
// saw the death has the rank 3 or more: it is not one of the first writers by
// HRW that delete the FFNS when the peer comes back. The peer comes back
// during the create or after the attempts. The writer deletes the FFNS that
// can exist.
func TestTimedOutCreateIsDeletedWhenThePeerComesBack(t *testing.T) {
	for _, during := range []bool{true, false} {
		t.Run(fmt.Sprintf("back during the create %v", during), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				all := names("w", 10)
				c := newCluster(t, all, clusterOptions{down: all[1:]}, armedMeta)
				victim := ""
				for _, v := range all[1:] {
					if rank(v, "w-0", c.nets["w-0"].Peers()) >= cancelers {
						victim = v
						break
					}
				}
				if victim == "" {
					t.Fatal("no victim with the rank 3 or more")
				}

				c.api.onCreate = func(ctx context.Context, ffns *v1alpha1.FencingFailedNodeState) error {
					c.api.add(ffns.DeepCopy())
					<-ctx.Done()

					return ctx.Err()
				}

				c.die(victim)
				wave := time.Duration(rank(victim, "w-0", c.nets["w-0"].Peers())/waveSize) * 2 * s
				if during {
					time.Sleep(wave + 50*ms)
				} else {
					time.Sleep(wave + 10*s)
				}
				c.back(victim, armedMeta(victim))
				time.Sleep(60 * s)

				if _, ok := c.api.get(victim); ok {
					t.Errorf("the FFNS of the peer that came back stays: creates %d, deletes %d", len(c.api.calls()), c.api.deletes)
				}
			})
		})
	}
}
