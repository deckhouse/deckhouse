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

package run

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// cutOffFromTheLeader is the error both followers of the H2b stand carried from their copies against a
// leader they could not reach.
const cutOffFromTheLeader = "discovering what to copy: reading the image set of main: " +
	"the image set could not be read from 10.12.6.95:5001"

// frozenH2b is what master-0 read on the H2b stand once the powered-off leader, master-1, was counted
// lost: master-2, the fuller, with the entry it wrote before its syncer was cut from the API server, and
// master-0 itself, empty.
func frozenH2b() []registryv1alpha1.StorageReplicaStatus {
	return []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleFollower},
		{Node: "master-2", Role: registryv1alpha1.ReplicaRoleFollower, VerifiedDigests: 277, Error: cutOffFromTheLeader},
	}
}

// TestAFrozenFullestReplicaIsPassedOver is H2b of a DVP stand. The leader was powered off while the
// fuller follower's syncer could not reach the API server, its node staying Ready, so its entry stood as
// it was. The empty replica waited for it in silence, and nobody led for four minutes: the frozen one
// came back after three and took the lease a minute later. Now the empty one waits two minutes, saying
// for whom, and stands.
func TestAFrozenFullestReplicaIsPassedOver(t *testing.T) {
	t0 := time.Date(2026, 10, 30, 14, 33, 17, 0, time.UTC)

	for _, lease := range []Lease{
		{Known: true, CanFill: true},                     // past its grace: nobody leads
		{Known: true, CanFill: true, Holder: "master-1"}, // in its grace, the holder's report dropped
	} {
		var logged bytes.Buffer
		empty := &Waiting{For: 2 * time.Minute, Log: slog.New(slog.NewTextHandler(&logged, nil))}
		fuller := &Waiting{For: 2 * time.Minute}

		first := empty.WhyNotLead("master-0", frozenH2b(), "", lease, t0)
		assert.Contains(t, first, "master-2 holds the most of it (277 digests)", "lease %+v", lease)
		assert.Contains(t, first,
			"master-2 is passed over if it has not taken the lease 2m0s after this replica first chose it")
		for _, after := range []time.Duration{15 * time.Second, 105 * time.Second, 119 * time.Second} {
			assert.Equal(t, first, empty.WhyNotLead("master-0", frozenH2b(), "", lease, t0.Add(after)),
				"the same words at every review, so that they are logged once: %s", after)
		}
		assert.Empty(t, empty.WhyNotLead("master-0", frozenH2b(), "", lease, t0.Add(2*time.Minute)),
			"two minutes on, master-2 has not taken the lease, and master-0 stands")
		assert.Empty(t, empty.WhyNotLead("master-0", frozenH2b(), "", lease, t0.Add(135*time.Second)),
			"and goes on standing")
		assert.Equal(t, 1, strings.Count(logged.String(), "passing over a replica"), "said once")
		assert.Contains(t, logged.String(), "replica=master-2")
		assert.Contains(t, logged.String(), "waited=2m0s")

		// The frozen one, could it judge, would stand throughout: it is the choice.
		for _, after := range []time.Duration{0, 2 * time.Minute, time.Hour} {
			assert.Empty(t, fuller.WhyNotLead("master-2", frozenH2b(), "", lease, t0.Add(after)))
		}
	}
}

// TestTheFullerReplicaStillWins: on the H2, G2 and K stands the fuller replica took the lease 60 to 70 s
// after it became the choice — client-go watches a lease unchanged for its duration first. Two minutes
// is longer, so the others see it lead before they give up on it, and stop waiting; the next time
// nobody leads, they wait afresh.
func TestTheFullerReplicaStillWins(t *testing.T) {
	t0 := time.Date(2026, 10, 30, 13, 10, 0, 0, time.UTC)
	replicas := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleFollower},
		{Node: "master-2", Role: registryv1alpha1.ReplicaRoleFollower, VerifiedDigests: 322, Error: cutOffFromTheLeader},
	}
	released := Lease{Known: true, CanFill: true}
	taken := Lease{Known: true, CanFill: true, Holder: "master-2", Live: true}
	empty := &Waiting{For: 2 * time.Minute}

	assert.NotEmpty(t, empty.WhyNotLead("master-0", replicas, "", released, t0))
	assert.NotEmpty(t, empty.WhyNotLead("master-0", replicas, "", released, t0.Add(75*time.Second)))
	why := empty.WhyNotLead("master-0", replicas, "", taken, t0.Add(86*time.Second))
	assert.Contains(t, why, "master-2 leads", "it took the lease 86 s after it became the choice")
	assert.NotContains(t, why, "passed over")

	// The lease released again, much later: a new spell, and a whole wait, rather than the one that ran
	// out long ago.
	later := t0.Add(5 * time.Minute)
	assert.NotEmpty(t, empty.WhyNotLead("master-0", replicas, "", released, later))
	assert.NotEmpty(t, empty.WhyNotLead("master-0", replicas, "", released, later.Add(119*time.Second)))
	assert.Empty(t, empty.WhyNotLead("master-0", replicas, "", released, later.Add(2*time.Minute)))

	// K: a fuller replica whose registry does not answer is no choice at all, and nobody waits on it.
	failing := append([]registryv1alpha1.StorageReplicaStatus(nil), replicas...)
	failing[1].HealthError = "this replica's registry is not answering: 10.12.6.92:5001: connection refused"
	assert.Empty(t, (&Waiting{For: 2 * time.Minute}).WhyNotLead("master-0", failing, "", released, t0))
}

// TestOnlyOneReplicaStandsAtATime: every replica that can judge waits on the same choice, from about the
// same moment, and passes it over in the same order, so it is one replica that stands at a time —
// whoever the rule chooses among those left. Three masters: master-2 frozen with the most, master-1 in
// the middle, master-0 empty, the two live ones reviewing 14 s apart.
func TestOnlyOneReplicaStandsAtATime(t *testing.T) {
	t0 := time.Date(2026, 10, 30, 14, 33, 17, 0, time.UTC)
	reported := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0"},
		{Node: "master-1", VerifiedDigests: 120, Error: cutOffFromTheLeader},
		{Node: "master-2", VerifiedDigests: 277, Error: cutOffFromTheLeader},
	}
	const takes = 71 * time.Second // from standing to holding the lease, as on the stands

	t.Run("the next fullest stands, and the empty one waits on it", func(t *testing.T) {
		waits := map[string]*Waiting{"master-0": {For: 2 * time.Minute}, "master-1": {For: 2 * time.Minute}}
		offset := map[string]time.Duration{"master-1": 0, "master-0": 14 * time.Second}
		standing := map[string]bool{}
		var stood, held time.Time

		for tick := time.Duration(0); tick <= 10*time.Minute; tick += time.Second {
			now := t0.Add(tick)
			lease := Lease{Known: true, CanFill: true}
			if !stood.IsZero() && !now.Before(stood.Add(takes)) {
				lease = Lease{Known: true, CanFill: true, Holder: "master-1", Live: true}
				if held.IsZero() {
					held = now
				}
			}
			for node, w := range waits {
				if (tick-offset[node])%(15*time.Second) != 0 || tick < offset[node] {
					continue
				}
				standing[node] = w.WhyNotLead(node, reported, "", lease, now) == ""
				if node == "master-1" && standing[node] && stood.IsZero() {
					stood = now
				}
			}
			require.False(t, standing["master-0"] && standing["master-1"], "both stand at %s", tick)
			if !held.IsZero() {
				require.False(t, standing["master-0"], "master-0 stands beside the leader at %s", tick)
			}
		}
		assert.Equal(t, t0.Add(2*time.Minute), stood, "master-1 stands once master-2 has had two minutes")
		assert.Equal(t, t0.Add(2*time.Minute+takes), held)
	})

	t.Run("with every fuller one frozen, the empty one stands", func(t *testing.T) {
		empty := &Waiting{For: 2 * time.Minute}
		lease := Lease{Known: true, CanFill: true}
		var stood time.Duration
		for tick := 14 * time.Second; tick <= 10*time.Minute; tick += 15 * time.Second {
			if empty.WhyNotLead("master-0", reported, "", lease, t0.Add(tick)) == "" {
				stood = tick
				break
			}
		}
		require.NotZero(t, stood, "nobody ever stood")
		assert.LessOrEqual(t, stood, 14*time.Second+2*2*time.Minute+15*time.Second,
			"within a review of passing over both, from its first look")
	})
}

// TestAPassOverKeepsTheRanking: a passed-over replica leaves the rule as it was among the rest — a
// replica that cannot serve is chosen last, and a store with room before a fuller one without.
func TestAPassOverKeepsTheRanking(t *testing.T) {
	t0 := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	released := Lease{Known: true, CanFill: true}

	t.Run("a replica that cannot serve waits on one that can, and passes nobody over", func(t *testing.T) {
		const failing = "this replica's store does not accept writes: read-only file system"
		replicas := []registryv1alpha1.StorageReplicaStatus{
			{Node: "master-0", VerifiedDigests: 300, HealthError: failing},
			{Node: "master-1", VerifiedDigests: 100},
		}
		unable := &Waiting{For: 2 * time.Minute}

		why := unable.WhyNotLead("master-0", replicas, "", released, t0)
		assert.Equal(t, "this replica cannot serve what it holds: "+failing, why, "and no countdown: it passes nobody over")
		for tick := 15 * time.Second; tick <= time.Hour; tick += 15 * time.Second {
			require.Equal(t, why, unable.WhyNotLead("master-0", replicas, "", released, t0.Add(tick)),
				"master-1 frozen for %s: a replica that cannot serve does not take the lease from under it", tick)
		}

		assert.Empty(t, (&Waiting{For: 2 * time.Minute}).WhyNotLead("master-1", replicas, "", released, t0),
			"the one that can serve stands at once")

		every := append([]registryv1alpha1.StorageReplicaStatus(nil), replicas...)
		every[1].HealthError = "this replica's registry is not answering: 10.12.6.92:5001: connection refused"
		assert.Empty(t, (&Waiting{For: 2 * time.Minute}).WhyNotLead("master-0", every, "", released, t0),
			"with every replica unable to serve, there is nothing better, and the election decides")
	})

	t.Run("nor over one it passed over while it could serve", func(t *testing.T) {
		const failing = "this replica's registry is not answering: 127.0.0.1:5001: connection refused"
		healthy := frozenH2b()
		waits := &Waiting{For: 2 * time.Minute}

		assert.NotEmpty(t, waits.WhyNotLead("master-0", healthy, "", released, t0))
		assert.Empty(t, waits.WhyNotLead("master-0", healthy, "", released, t0.Add(2*time.Minute)),
			"master-2 passed over, master-0 stands")

		failed := frozenH2b()
		failed[0].HealthError = failing
		assert.Equal(t, "this replica cannot serve what it holds: "+failing,
			waits.WhyNotLead("master-0", failed, "", released, t0.Add(135*time.Second)),
			"its registry failed before it took the lease: it steps aside for the frozen master-2")
		assert.Empty(t, waits.WhyNotLead("master-0", healthy, "", released, t0.Add(150*time.Second)),
			"and once it can serve again, the pass-over it made stands")
	})

	t.Run("a store without room waits on those with room, the fullest first", func(t *testing.T) {
		noRoom := &registryv1alpha1.StoreUsage{Writable: false, Reason: registryv1alpha1.StoreBudgetExhausted}
		replicas := []registryv1alpha1.StorageReplicaStatus{
			{Node: "master-0", VerifiedDigests: 300, Store: noRoom},
			{Node: "master-1", VerifiedDigests: 100},
			{Node: "master-2", VerifiedDigests: 50},
		}
		full := &Waiting{For: 2 * time.Minute}

		assert.Contains(t, full.WhyNotLead("master-0", replicas, "", released, t0),
			"master-1 holds the most among those that can (100 digests)")
		why := full.WhyNotLead("master-0", replicas, "", released, t0.Add(2*time.Minute))
		assert.Contains(t, why, "master-2 holds the most among those that can (50 digests)",
			"master-1 passed over, the next with room is the choice")
		assert.Contains(t, why, "master-2 is passed over if it has not taken the lease")
		assert.NotEmpty(t, full.WhyNotLead("master-0", replicas, "", released, t0.Add(239*time.Second)),
			"and it has its own two minutes")
		assert.Empty(t, full.WhyNotLead("master-0", replicas, "", released, t0.Add(4*time.Minute)),
			"every store with room passed over: the fullest is the best source of what there is")
	})
}

// TestNoPassOverWhileTheLeaseIsUnknownOrLive: an unreadable lease is no evidence that the chosen replica
// did not take it, and a live one cannot be taken by anybody. The wait already run is kept.
func TestNoPassOverWhileTheLeaseIsUnknownOrLive(t *testing.T) {
	t0 := time.Date(2026, 10, 30, 14, 33, 17, 0, time.UTC)
	released := Lease{Known: true, CanFill: true}

	t.Run("unknown", func(t *testing.T) {
		empty := &Waiting{For: 2 * time.Minute}
		first := empty.WhyNotLead("master-0", frozenH2b(), "", released, t0)
		assert.Equal(t, first, empty.WhyNotLead("master-0", frozenH2b(), "", Lease{}, t0.Add(130*time.Second)),
			"the lease could not be read: still waiting, in the same words")
		assert.Empty(t, empty.WhyNotLead("master-0", frozenH2b(), "", released, t0.Add(135*time.Second)),
			"read again, and nobody has it: the wait ran out a while ago")
	})

	t.Run("live, held by a replica without a report", func(t *testing.T) {
		empty := &Waiting{For: 2 * time.Minute}
		held := Lease{Known: true, CanFill: true, Holder: "master-1", Live: true}
		for tick := time.Duration(0); tick <= 5*time.Minute; tick += 15 * time.Second {
			assert.NotEmpty(t, empty.WhyNotLead("master-0", frozenH2b(), "", held, t0.Add(tick)), "at %s", tick)
		}
		expired := Lease{Known: true, CanFill: true, Holder: "master-1"}
		since := t0.Add(5*time.Minute + 15*time.Second)
		assert.NotEmpty(t, empty.WhyNotLead("master-0", frozenH2b(), "", expired, since),
			"the wait starts once the lease is not live")
		assert.NotEmpty(t, empty.WhyNotLead("master-0", frozenH2b(), "", expired, since.Add(119*time.Second)))
		assert.Empty(t, empty.WhyNotLead("master-0", frozenH2b(), "", expired, since.Add(2*time.Minute)))
	})
}

// TestADegradedReadDoesNotRestartTheWait: a node or the pods that cannot be read keep the reports they
// would decide, so one review in the middle of H2b's wait can bring back the dead leader's entry, full
// and Leader. That is no evidence that anybody leads, and the pass-over keeps its time; only a lease
// seen live ends the spell.
func TestADegradedReadDoesNotRestartTheWait(t *testing.T) {
	t0 := time.Date(2026, 10, 30, 14, 33, 17, 0, time.UTC)
	const set = "sha256:main"
	deadLeader := registryv1alpha1.StorageReplicaStatus{
		Node: "master-1", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedDigests: 446, VerifiedSet: set,
	}
	unread := append(frozenH2b(), deadLeader)
	released := Lease{Known: true, CanFill: true}

	for name, lease := range map[string]Lease{
		"the lease read, nobody holding it":        released,
		"the lease unread, the roles deciding":     {CanFill: true},
		"the lease in its grace, the holder there": {Known: true, CanFill: true, Holder: "master-1"},
	} {
		t.Run(name, func(t *testing.T) {
			empty := &Waiting{For: 2 * time.Minute}
			assert.Contains(t, empty.WhyNotLead("master-0", frozenH2b(), set, released, t0), "master-2 holds the most")
			assert.NotEmpty(t, empty.WhyNotLead("master-0", unread, set, lease, t0.Add(105*time.Second)),
				"the dead leader's report is back for one review")
			assert.Empty(t, empty.WhyNotLead("master-0", frozenH2b(), set, released, t0.Add(2*time.Minute)),
				"master-2 is passed over when it would have been")
			assert.NotEmpty(t, empty.WhyNotLead("master-0", unread, set, lease, t0.Add(135*time.Second)),
				"the same read once it stands makes it step aside, as it did before passing over existed")
			assert.Empty(t, empty.WhyNotLead("master-0", frozenH2b(), set, released, t0.Add(150*time.Second)),
				"and it stands again at the next review, not two minutes later")
		})
	}

	t.Run("a lease seen live ends the spell", func(t *testing.T) {
		empty := &Waiting{For: 2 * time.Minute}
		assert.NotEmpty(t, empty.WhyNotLead("master-0", frozenH2b(), set, released, t0))
		assert.NotEmpty(t, empty.WhyNotLead("master-0", unread, set,
			Lease{Known: true, CanFill: true, Holder: "master-1", Live: true}, t0.Add(105*time.Second)))
		assert.NotEmpty(t, empty.WhyNotLead("master-0", frozenH2b(), set, released, t0.Add(2*time.Minute)),
			"master-1 was seen leading, so this is a new spell and a whole wait")
		assert.NotEmpty(t, empty.WhyNotLead("master-0", frozenH2b(), set, released, t0.Add(239*time.Second)))
		assert.Empty(t, empty.WhyNotLead("master-0", frozenH2b(), set, released, t0.Add(4*time.Minute)))
	})
}

// TestAReplicaNeverPassesItselfOver: the replica the rule chooses stands for as long as it is chosen,
// however long it has not got the lease — it may be the one replica left.
func TestAReplicaNeverPassesItselfOver(t *testing.T) {
	t0 := time.Date(2026, 10, 30, 14, 33, 17, 0, time.UTC)
	fuller := &Waiting{For: 2 * time.Minute}
	for tick := time.Duration(0); tick <= time.Hour; tick += 15 * time.Second {
		require.Empty(t, fuller.WhyNotLead("master-2", frozenH2b(), "", Lease{Known: true, CanFill: true}, t0.Add(tick)),
			"at %s", tick)
	}
}

// TestZeroWaitNeverPassesOver: without a wait, the rule is what it is without Waiting, in its words.
func TestZeroWaitNeverPassesOver(t *testing.T) {
	t0 := time.Date(2026, 10, 30, 14, 33, 17, 0, time.UTC)
	lease := Lease{Known: true, CanFill: true}
	never := &Waiting{}
	for _, after := range []time.Duration{0, 2 * time.Minute, time.Hour} {
		assert.Equal(t, WhyNotLead("master-0", frozenH2b(), "", lease),
			never.WhyNotLead("master-0", frozenH2b(), "", lease, t0.Add(after)))
	}
}
