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
	"errors"
	"fmt"
	"slices"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/config"
	"fencing-agent/internal/domain"
)

func TestWriterHealthyFor(t *testing.T) {
	tests := []struct {
		name      string
		profile   v1alpha1.FencingSLAProfileSpec
		nodeCount int
		want      time.Duration
	}{
		{"critical, 1000 nodes", critical, 1000, 12200 * ms},
		{"normal, 1000 nodes", normal, 1000, 41 * s},
		{"slow, 1000 nodes", slow, 1000, 77 * s},
		{"critical, 3000 nodes", critical, 3000, 13300 * ms},
		{"normal, 3000 nodes", normal, 3000, 46700 * ms},
		{"slow, 3000 nodes", slow, 3000, 88500 * ms},
		// log10 is not less than 1.
		{"critical, 2 nodes", critical, 2, 7400 * ms},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := writerHealthyFor(tt.profile, tt.nodeCount).Round(100 * ms); got != tt.want {
				t.Errorf("writerHealthyFor = %s, want %s", got, tt.want)
			}
		})
	}
}

// The FFNS of the own node is deleted, and 30 ms later a peer is dead. The
// writer does not create its FFNS until it is healthy again for the full
// writerHealthyFor: 12.2 s with the critical profile and 1000 nodes.
func TestWriterWaitsTheFullPeriodAfterItsOwnFFNS(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, names("w", 1000), clusterOptions{down: names("w", 1000)[1:]}, armedMeta)
		w := c.writers["w-0"]
		victims := c.firstWave("w-0")

		w.SetOwn(&v1alpha1.FencingFailedNodeState{ObjectMeta: metav1.ObjectMeta{Name: "w-0"}})
		time.Sleep(s)
		w.SetOwn(nil)
		gone := time.Now()

		time.Sleep(30 * ms)
		c.die(victims[0])

		time.Sleep(gone.Add(12100 * ms).Sub(time.Now()))
		c.die(victims[1])

		time.Sleep(gone.Add(12200 * ms).Sub(time.Now()))
		third := victims[2]
		c.die(third)
		synctest.Wait()

		calls := c.api.calls()
		if len(calls) != 1 || !calls[0].at.Equal(gone.Add(12200*ms)) {
			t.Fatalf("creates = %v, want one at 12.2 s after the own FFNS is gone", calls)
		}

		if _, ok := c.api.get(third); !ok {
			t.Errorf("no FFNS %s", third)
		}
	})
}

// A loss of the control plane restarts writerHealthyFor.
func TestWriterLossOfControlPlaneRestartsThePeriod(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		all := names("w", 10)
		c := newCluster(t, all, clusterOptions{down: all[1:]}, armedMeta)
		w := c.writers["w-0"]

		w.ObserveCP(domain.CPState{Seen: true})
		w.ObserveCP(domain.CPState{Visible: true, Seen: true})
		back := time.Now()

		// writerHealthyFor is 7.4 s with 10 nodes.
		time.Sleep(7 * s)
		victims := c.firstWave("w-0")
		c.die(victims[0])
		synctest.Wait()

		if calls := c.api.calls(); len(calls) != 0 {
			t.Fatalf("creates = %v, want none 7 s after the control plane is back", calls)
		}

		time.Sleep(back.Add(7400 * ms).Sub(time.Now()))
		c.die(victims[1])
		synctest.Wait()

		if calls := c.api.calls(); len(calls) != 1 {
			t.Fatalf("creates = %v, want one 7.4 s after the control plane is back", calls)
		}
	})
}

// An agent of a Disabled group is a writer. An agent with fencing-disable is not.
func TestWriterGateDoesNotDependOnTheWatchdog(t *testing.T) {
	tests := []struct {
		name        string
		mode        string
		maintenance bool
		want        int
	}{
		{"agent of a Disabled group", config.ModeDisabled, false, 1},
		{"armed agent", config.ModeCritical, false, 1},
		{"agent in maintenance", config.ModeCritical, true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				all := names("w", 4)
				maintenance := &atomic.Bool{}
				maintenance.Store(tt.maintenance)
				c := newCluster(t, all, clusterOptions{settings: settings(tt.mode), maintenance: maintenance, down: all[1:]}, armedMeta)
				c.die(c.firstWave("w-0")[0])
				synctest.Wait()

				if got := len(c.api.calls()); got != tt.want {
					t.Errorf("creates = %d, want %d", got, tt.want)
				}
			})
		})
	}
}

// The FFNS takes bootID, armed, the Node UID and the profile from the last
// NodeMeta of the victim.
func TestWriterFFNSContent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		all := names("w", 4)
		c := newCluster(t, all, clusterOptions{down: all[1:]}, func(name string) domain.NodeMeta {
			return domain.NodeMeta{BootID: "b1", Armed: true, NodeUID: "u-1", NodeGroup: "ng-slow", Profile: "slow"}
		})
		victim := c.firstWave("w-0")[0]

		c.die(victim)
		synctest.Wait()

		ffns, ok := c.api.get(victim)
		if !ok {
			t.Fatal("no FFNS")
		}

		f := false
		wantOwner := []metav1.OwnerReference{{APIVersion: "v1", Kind: "Node", Name: victim, UID: "u-1", BlockOwnerDeletion: &f}}
		if !ownersEqual(ffns.OwnerReferences, wantOwner) {
			t.Errorf("ownerReferences = %+v, want %+v", ffns.OwnerReferences, wantOwner)
		}

		if len(ffns.Labels) != 0 {
			t.Errorf("labels = %v, want none", ffns.Labels)
		}

		want := v1alpha1.FencingFailedNodeStateSpec{
			Source: v1alpha1.SourceMemberlistDead, DetectedBy: "w-0", DetectedAt: metav1.NewMicroTime(c.start),
			BootID: "b1", Armed: true, ProfileRef: &v1alpha1.ProfileRef{Name: v1alpha1.ProfileSlow},
		}
		if !specEqual(ffns.Spec, want) {
			t.Errorf("spec = %+v, want %+v", ffns.Spec, want)
		}
	})
}

func ownersEqual(a, b []metav1.OwnerReference) bool {
	return slices.EqualFunc(a, b, func(x, y metav1.OwnerReference) bool {
		return x.APIVersion == y.APIVersion && x.Kind == y.Kind && x.Name == y.Name && x.UID == y.UID &&
			x.BlockOwnerDeletion != nil && y.BlockOwnerDeletion != nil && *x.BlockOwnerDeletion == *y.BlockOwnerDeletion &&
			x.Controller == nil
	})
}

func specEqual(a, b v1alpha1.FencingFailedNodeStateSpec) bool {
	profile := func(p *v1alpha1.ProfileRef) v1alpha1.ProfileName {
		if p == nil {
			return ""
		}

		return p.Name
	}

	return a.Source == b.Source && a.DetectedBy == b.DetectedBy && a.DetectedAt.Equal(&b.DetectedAt) &&
		a.BootID == b.BootID && a.Armed == b.Armed && (a.ProfileRef == nil) == (b.ProfileRef == nil) &&
		profile(a.ProfileRef) == profile(b.ProfileRef)
}

// The armed victim gives its node group and its profile in its NodeMeta. The
// writer does not read the Node. The FFNS gets the slower of the profile in
// the NodeMeta and the profile of the group in the own configuration: a
// NodeMeta or a configuration that is not up to date does not give a shorter
// watchdog timeout than the node has. A group that the own configuration does
// not arm gives slow.
func TestWriterProfileFromTheNodeMeta(t *testing.T) {
	for _, tt := range []struct {
		name, group, fromMeta, ownMode string
		want                           v1alpha1.ProfileName
	}{
		{"own configuration is slower", "workers", "critical", config.ModeNormal, v1alpha1.ProfileNormal},
		{"NodeMeta is slower", "workers", "slow", config.ModeCritical, v1alpha1.ProfileSlow},
		{"same profile", "workers", "normal", config.ModeNormal, v1alpha1.ProfileNormal},
		{"group not in the own configuration", "ng-new", "normal", config.ModeCritical, v1alpha1.ProfileSlow},
		{"group Disabled in the own configuration", "disabled", "critical", config.ModeCritical, v1alpha1.ProfileSlow},
	} {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				all := names("w", 4)
				c := newCluster(t, all, clusterOptions{settings: settings(tt.ownMode), down: all[1:]}, func(string) domain.NodeMeta {
					return domain.NodeMeta{BootID: "b1", Armed: true, NodeUID: "u-1", NodeGroup: tt.group, Profile: tt.fromMeta}
				})

				c.die(c.firstWave("w-0")[0])
				synctest.Wait()

				ffns, ok := c.api.get(c.firstWave("w-0")[0])
				if !ok {
					t.Fatal("no FFNS")
				}

				if ffns.Spec.ProfileRef == nil || ffns.Spec.ProfileRef.Name != tt.want {
					t.Errorf("profileRef = %v, want %s", ffns.Spec.ProfileRef, tt.want)
				}
			})
		})
	}
}

// The writer does not read the Node. Without the Node UID in the NodeMeta of
// the victim, or without the node group or the profile of an armed victim,
// the writer creates no FFNS: no taint is the safe side.
func TestWriterNoFFNSWithoutTheFieldsOfTheNodeMeta(t *testing.T) {
	for _, tt := range []struct {
		name string
		meta domain.NodeMeta
	}{
		{"no Node UID", domain.NodeMeta{BootID: "b1"}},
		{"armed without the node group", domain.NodeMeta{BootID: "b1", Armed: true, NodeUID: "u-1", Profile: "slow"}},
		{"armed without the profile", domain.NodeMeta{BootID: "b1", Armed: true, NodeUID: "u-1", NodeGroup: "workers"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				all := names("w", 4)
				c := newCluster(t, all, clusterOptions{down: all[1:]}, func(string) domain.NodeMeta { return tt.meta })

				c.die(c.firstWave("w-0")[0])
				synctest.Wait()

				if calls := c.api.calls(); len(calls) != 0 {
					t.Errorf("creates = %d, want none", len(calls))
				}
			})
		})
	}
}

// An unarmed victim needs no node group and no profile: the FFNS gets
// armed=false without a profile.
func TestWriterUnarmedVictimNeedsNoProfile(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		all := names("w", 4)
		c := newCluster(t, all, clusterOptions{down: all[1:]}, func(string) domain.NodeMeta {
			return domain.NodeMeta{BootID: "b1", NodeUID: "u-1"}
		})
		victim := c.firstWave("w-0")[0]

		c.die(victim)
		synctest.Wait()

		ffns, ok := c.api.get(victim)
		if !ok {
			t.Fatal("no FFNS")
		}

		if ffns.Spec.Armed || ffns.Spec.ProfileRef != nil {
			t.Errorf("armed = %v, profileRef = %v, want false and none", ffns.Spec.Armed, ffns.Spec.ProfileRef)
		}
	})
}

// No FFNS for a victim that left on purpose or is in maintenance.
func TestWriterNoFFNS(t *testing.T) {
	tests := []struct {
		name string
		meta domain.NodeMeta
		// leaving is true when «leaving» came from the victim.
		leaving bool
	}{
		{"leaving received", armedMeta("v"), true},
		{"leaving in the NodeMeta", domain.NodeMeta{BootID: "b1", Armed: true, NodeUID: "u-1", Leaving: true}, false},
		{"victim in maintenance", domain.NodeMeta{BootID: "b1", NodeUID: "u-1", Maintenance: true}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				all := names("w", 4)
				c := newCluster(t, all, clusterOptions{}, func(string) domain.NodeMeta { return tt.meta })
				victim := "w-3"

				for _, n := range c.nets {
					n.leaving[victim] = tt.leaving
				}

				c.die(victim)
				time.Sleep(10 * s)

				if calls := c.api.calls(); len(calls) != 0 {
					t.Errorf("creates = %v, want none", calls)
				}
			})
		})
	}
}

// The first wave creates the FFNS at 0. The waves at +2 s and +4 s get
// «created» and do not create it.
func TestWavesFirstWaveCreates(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, names("w", 10), clusterOptions{}, armedMeta)
		victim := "w-9"
		ranked := c.byRank(victim)

		c.die(victim)
		time.Sleep(10 * s)

		for _, call := range c.api.calls() {
			if !call.at.Equal(c.start) || !slices.Contains(ranked[:3], call.by) {
				t.Errorf("create by %s at %s, want only the first wave at 0", call.by, call.at.Sub(c.start))
			}
		}

		if len(c.api.objs) != 1 {
			t.Errorf("objects = %d, want 1", len(c.api.objs))
		}
	})
}

// The writer of rank 0 died with the victim, the other writers of the first
// wave fail. The second wave creates one object.
func TestWavesNextWaveCreatesWhenTheFirstFails(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		all := names("w", 10)
		victim := "w-9"
		ranked := (&cluster{names: all}).byRank(victim)

		c := newCluster(t, all, clusterOptions{down: []string{victim, ranked[0]}}, armedMeta)
		c.api.onCreate = func(_ context.Context, ffns *v1alpha1.FencingFailedNodeState) error {
			if slices.Contains(ranked[1:3], ffns.Spec.DetectedBy) {
				return errors.New("the API is not available")
			}

			return nil
		}

		c.die(victim)
		time.Sleep(10 * s)

		for _, call := range c.api.calls() {
			first := call.at.Sub(c.start) < time.Second && slices.Contains(ranked[1:3], call.by)
			second := call.at.Equal(c.start.Add(2*s)) && slices.Contains(ranked[3:6], call.by)
			if !first && !second {
				t.Errorf("create by %s at %s, want the first and the second wave only", call.by, call.at.Sub(c.start))
			}
		}

		if len(c.api.objs) != 1 {
			t.Errorf("objects = %d, want 1", len(c.api.objs))
		}
	})
}

// The victim is alive again before the wave at +4 s: that wave does not create.
func TestWavesVictimBackBeforeTheWave(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		all := names("w", 10)
		victim := "w-9"
		ranked := (&cluster{names: all}).byRank(victim)

		c := newCluster(t, all, clusterOptions{down: append([]string{victim}, ranked[:6]...)}, armedMeta)

		c.die(victim)
		time.Sleep(3 * s)
		c.back(victim, armedMeta(victim))
		time.Sleep(10 * s)

		if calls := c.api.calls(); len(calls) != 0 {
			t.Errorf("creates = %v, want none", calls)
		}
	})
}

// The first attempt ends by its timeout of 1 s. The second attempt starts
// 100 ms later and succeeds. The writer sends «created».
func TestAttemptsAfterTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		all := names("w", 4)
		c := newCluster(t, all, clusterOptions{down: all[1:]}, armedMeta)
		victim := c.firstWave("w-0")[0]

		first := true
		c.api.onCreate = func(ctx context.Context, _ *v1alpha1.FencingFailedNodeState) error {
			if first {
				first = false
				<-ctx.Done()

				return ctx.Err()
			}

			return nil
		}

		c.die(victim)
		time.Sleep(5 * s)

		calls := c.api.calls()
		if len(calls) != 2 || !calls[0].at.Equal(c.start) || !calls[1].at.Equal(c.start.Add(1100*ms)) {
			t.Fatalf("creates = %v, want at 0 and at 1.1 s", calls)
		}

		if _, ok := c.api.get(victim); !ok {
			t.Error("no FFNS")
		}

		if got := c.nets["w-0"].createdSent(victim); got != len(all)-2 {
			t.Errorf("created sent to %d peers, want %d", got, len(all)-2)
		}
	})
}

// AlreadyExists is a success: the writer sends «created».
func TestAttemptsAlreadyExists(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		all := names("w", 4)
		c := newCluster(t, all, clusterOptions{down: all[1:]}, armedMeta)
		victim := c.firstWave("w-0")[0]
		c.api.put(&v1alpha1.FencingFailedNodeState{ObjectMeta: metav1.ObjectMeta{Name: victim}})

		c.die(victim)
		time.Sleep(5 * s)

		if calls := c.api.calls(); len(calls) != 1 {
			t.Errorf("creates = %v, want one", calls)
		}

		if got := c.nets["w-0"].createdSent(victim); got != len(all)-2 {
			t.Errorf("created sent to %d peers, want %d", got, len(all)-2)
		}
	})
}

// The victim was dead for 9 s and came back with the same boot ID: the
// writers delete the FFNS.
func TestCancelVictimBackWithTheSameBootID(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, names("w", 10), clusterOptions{}, armedMeta)
		victim := "w-9"

		c.die(victim)
		time.Sleep(9 * s)

		if _, ok := c.api.get(victim); !ok {
			t.Fatal("no FFNS")
		}

		c.back(victim, armedMeta(victim))
		synctest.Wait()

		if _, ok := c.api.get(victim); ok {
			t.Error("the FFNS stays, want it deleted")
		}
	})
}

// The victim came back with the same boot ID. The writers delete the FFNS in
// the phase Suspected and keep the FFNS in the phase Proven.
func TestCancelVictimBackByThePhase(t *testing.T) {
	for _, tc := range []struct {
		phase v1alpha1.FencingFailedNodeStatePhase
		kept  bool
	}{
		{phase: v1alpha1.PhaseSuspected, kept: false},
		{phase: v1alpha1.PhaseProven, kept: true},
	} {
		t.Run(string(tc.phase), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				c := newCluster(t, names("w", 10), clusterOptions{}, armedMeta)
				victim := "w-9"

				c.die(victim)
				time.Sleep(9 * s)

				ffns, ok := c.api.get(victim)
				if !ok {
					t.Fatal("no FFNS")
				}

				ffns = ffns.DeepCopy()
				ffns.Status.Phase = tc.phase
				c.api.add(ffns)

				c.back(victim, armedMeta(victim))
				time.Sleep(10 * s)

				if _, ok := c.api.get(victim); ok != tc.kept {
					t.Errorf("FFNS kept = %t, want %t", ok, tc.kept)
				}
			})
		})
	}
}

// The victim came back with a new boot ID: the FFNS stays.
func TestCancelVictimBackWithANewBootID(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, names("w", 10), clusterOptions{}, armedMeta)
		victim := "w-9"

		c.die(victim)
		time.Sleep(9 * s)

		rebooted := armedMeta(victim)
		rebooted.BootID = "b2"
		c.back(victim, rebooted)
		time.Sleep(10 * s)

		if _, ok := c.api.get(victim); !ok {
			t.Error("the FFNS is deleted, want it kept")
		}
	})
}

// The victim came back while the create was in progress: the object did not
// exist at the event. The creator deletes it after the create.
func TestCancelVictimBackDuringTheCreate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		all := names("w", 4)
		c := newCluster(t, all, clusterOptions{down: all[1:]}, armedMeta)
		victim := c.firstWave("w-0")[0]
		c.api.onCreate = func(context.Context, *v1alpha1.FencingFailedNodeState) error {
			time.Sleep(500 * ms)

			return nil
		}

		c.die(victim)
		time.Sleep(200 * ms)
		c.back(victim, armedMeta(victim))

		time.Sleep(c.start.Add(3 * s).Sub(time.Now()))
		if len(c.api.calls()) != 1 || c.api.deletes != 1 {
			t.Fatalf("creates = %v, deletes = %d, want one create and one delete", c.api.calls(), c.api.deletes)
		}

		if _, ok := c.api.get(victim); ok {
			t.Error("the FFNS stays, want it deleted by the creator")
		}
	})
}

// A witness created the attested FFNS. The victim is alive in memberlist all
// the time: no event, no check of the creator, the FFNS stays.
func TestCancelAttestedFFNSOfAliveVictim(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, names("w", 10), clusterOptions{}, armedMeta)
		victim := "w-9"
		c.api.put(&v1alpha1.FencingFailedNodeState{
			ObjectMeta: metav1.ObjectMeta{Name: victim},
			Spec:       v1alpha1.FencingFailedNodeStateSpec{Source: v1alpha1.SourceDeclareSuicide, DetectedBy: "w-1", BootID: "boot-w-9", Armed: true},
			Status:     v1alpha1.FencingFailedNodeStateStatus{Attestation: &v1alpha1.FencingFailedNodeStateAttestation{Witness: "w-1"}},
		})

		time.Sleep(30 * s)

		if _, ok := c.api.get(victim); !ok {
			t.Error("the FFNS is deleted, want it kept")
		}
	})
}

// A witness created the attested FFNS. Then the victim is dead in memberlist
// and comes back with the same boot ID: the first writers by HRW delete the FFNS.
func TestCancelAttestedFFNSOfVictimBack(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, names("w", 10), clusterOptions{}, armedMeta)
		victim := "w-9"
		c.api.put(&v1alpha1.FencingFailedNodeState{
			ObjectMeta: metav1.ObjectMeta{Name: victim},
			Spec:       v1alpha1.FencingFailedNodeStateSpec{Source: v1alpha1.SourceDeclareSuicide, DetectedBy: "w-1", BootID: "boot-w-9", Armed: true},
			Status:     v1alpha1.FencingFailedNodeStateStatus{Attestation: &v1alpha1.FencingFailedNodeStateAttestation{Witness: "w-1"}},
		})

		c.die(victim)
		time.Sleep(10 * s)
		c.back(victim, armedMeta(victim))
		time.Sleep(10 * s)

		if _, ok := c.api.get(victim); ok {
			t.Error("the FFNS stays, want it deleted")
		}
	})
}

// The victim dies and comes back many times, also less than 2 s after the
// return. At the end the FFNS exists only if the victim is dead.
func TestCancelVictimFlaps(t *testing.T) {
	// flaps are the periods of the victim: dead, alive, dead, alive and so on.
	flaps := []time.Duration{600 * ms, 5 * s, 1400 * ms, 100 * ms, 2400 * ms, 1300 * ms, 1 * s, 1100 * ms, 200 * ms, 1200 * ms, 3900 * ms, 1 * s}

	for _, dead := range []bool{false, true} {
		t.Run(fmt.Sprintf("dead at the end %t", dead), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				victim := "w-7"
				c := newCluster(t, names("w", 8), clusterOptions{down: []string{victim}}, armedMeta)

				for i, d := range flaps {
					if i%2 == 0 {
						c.die(victim)
					} else {
						c.back(victim, armedMeta(victim))
					}

					time.Sleep(d)
				}

				if dead {
					c.die(victim)
				}

				time.Sleep(60 * s)

				if _, ok := c.api.get(victim); ok != dead {
					t.Errorf("FFNS exists = %t, want %t", ok, dead)
				}
			})
		})
	}
}

// The FFNS changes between the read and the delete: the delete gets a
// conflict, the writer reads the new version and deletes it.
func TestCancelRetriesAfterConflict(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, names("w", 4), clusterOptions{down: []string{"w-1", "w-2", "w-3"}}, armedMeta)
		victim := c.firstWave("w-0")[0]

		c.die(victim)
		synctest.Wait()

		// A status write changes the version between the first read and the first delete.
		c.api.onDelete = func(name string) {
			if c.api.deletes == 1 {
				c.api.put(c.api.objs[name])
			}
		}

		c.back(victim, armedMeta(victim))
		time.Sleep(s)

		if _, ok := c.api.get(victim); ok || c.api.deletes != 2 {
			t.Errorf("FFNS exists = %v, deletes = %d, want deleted on the second delete", ok, c.api.deletes)
		}
	})
}

// All writers lose the control plane and see it again at once. Then the
// victim comes back with the same boot ID. The writers do not pass the
// health gate. They try again each writerWaveInterval and delete the FFNS
// when they pass it: writerHealthyFor is 7.4 s with 10 nodes.
func TestCancelAfterTheHealthGate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, names("w", 10), clusterOptions{}, armedMeta)
		victim := "w-9"

		c.die(victim)
		time.Sleep(5 * s)

		if _, ok := c.api.get(victim); !ok {
			t.Fatal("no FFNS")
		}

		for _, w := range c.writers {
			w.ObserveCP(domain.CPState{Seen: true})
			w.ObserveCP(domain.CPState{Visible: true, Seen: true})
		}

		back := time.Now()
		c.back(victim, armedMeta(victim))
		synctest.Wait()

		if _, ok := c.api.get(victim); !ok {
			t.Fatal("the FFNS is deleted before the writers pass the health gate")
		}

		time.Sleep(back.Add(10 * s).Sub(time.Now()))

		if _, ok := c.api.get(victim); ok {
			t.Error("the FFNS stays, want it deleted after the writers pass the health gate")
		}
	})
}

// The writer of rank 1 lost its own FFNS 1 s ago. It sees the victim alive
// with the same boot ID, the other writers see the victim dead. The writer
// does not pass the health gate and does not delete the FFNS.
func TestCancelNotByAnUnhealthyWriter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, names("w", 10), clusterOptions{}, armedMeta)
		victim := "w-9"
		name := c.byRank(victim)[1]
		canceler := c.writers[name]

		c.die(victim)
		time.Sleep(5 * s)

		if _, ok := c.api.get(victim); !ok {
			t.Fatal("no FFNS")
		}

		canceler.SetOwn(&v1alpha1.FencingFailedNodeState{ObjectMeta: metav1.ObjectMeta{Name: name}})
		canceler.SetOwn(nil)
		time.Sleep(s)

		meta := armedMeta(victim)
		canceler.Member(victim, &meta)
		time.Sleep(5 * s)

		if _, ok := c.api.get(victim); !ok || c.api.deletes != 0 {
			t.Errorf("FFNS exists = %v, deletes = %d, want it kept without deletes", ok, c.api.deletes)
		}
	})
}

// HRW ranks the writers by the static peer list: each writer has its own
// rank, the ranks are the same on every node.
func TestRankIsAPermutation(t *testing.T) {
	c := &cluster{names: names("w", 50)}

	ranked := c.byRank("w-7")
	if slices.Contains(ranked, "") || slices.Contains(ranked, "w-7") {
		t.Fatalf("ranks = %v, want each writer once without the victim", ranked)
	}

	if slices.Equal(ranked, c.byRank("w-8")[:len(ranked)]) {
		t.Errorf("the ranks of two victims are the same: %v", ranked)
	}
}

// The API does not answer for 5 s when the victim comes back. The writers try
// again and delete the FFNS.
func TestCancelTriesAgainWhileTheAPIIsNotAvailable(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, names("w", 10), clusterOptions{}, armedMeta)
		victim := "w-9"

		c.die(victim)
		time.Sleep(9 * s)

		back := time.Now()
		c.api.onGet = func(string) error {
			if time.Since(back) < 5*s {
				return errors.New("the API is not available")
			}

			return nil
		}

		c.back(victim, armedMeta(victim))
		time.Sleep(4 * s)

		if _, ok := c.api.get(victim); !ok {
			t.Fatal("the FFNS is deleted while the API is not available")
		}

		time.Sleep(4 * s)

		if _, ok := c.api.get(victim); ok {
			t.Error("the FFNS stays, want it deleted after the API is back")
		}
	})
}

// An agent in maintenance does not delete the FFNS.
func TestCancelNotByAnAgentInMaintenance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		maintenance := &atomic.Bool{}
		c := newCluster(t, names("w", 10), clusterOptions{maintenance: maintenance}, armedMeta)
		victim := "w-9"

		c.die(victim)
		time.Sleep(9 * s)

		maintenance.Store(true)
		c.back(victim, armedMeta(victim))
		time.Sleep(10 * s)

		if _, ok := c.api.get(victim); !ok || c.api.deletes != 0 {
			t.Errorf("FFNS exists = %v, deletes = %d, want it kept without deletes", ok, c.api.deletes)
		}
	})
}

// After Stop the writer creates no FFNS.
func TestStoppedWriterCreatesNothing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		all := names("w", 4)
		c := newCluster(t, all, clusterOptions{down: all[1:]}, armedMeta)

		c.writers["w-0"].Stop()
		c.die(c.firstWave("w-0")[0])
		time.Sleep(10 * s)

		if calls := c.api.calls(); len(calls) != 0 {
			t.Errorf("creates = %v, want none after Stop", calls)
		}
	})
}
