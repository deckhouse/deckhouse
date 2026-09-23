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

package join

import (
	"bytes"
	"context"
	"errors"
	"net"
	"runtime"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"fencing-agent/internal/domain"
	"fencing-agent/internal/logtest"
)

func TestSeedListExcludesLocalNodeAndNodesWithoutIP(t *testing.T) {
	nodes, expected := mirroredGroup(
		selfPeer(),
		domain.Peer{Name: "worker-2", IP: "10.0.0.2"},
		domain.Peer{Name: "worker-3", IP: ""},
		domain.Peer{Name: "worker-4", IP: "10.0.0.4"},
	)
	cluster := &fakeCluster{}

	newJoiner(t, nodes, expected, cluster).Bootstrap(t.Context())

	assertJoinedOnce(t, cluster, "10.0.0.2:8500", "10.0.0.4:8500")

	if got := slices.Sorted(slices.Values(nodes.candidateGets())); !slices.Equal(got, []string{"worker-2", "worker-3", "worker-4"}) {
		t.Errorf("candidate reads are %v, want worker-2, worker-3 and worker-4 once each", got)
	}
}

func TestSeedListExcludesStaleNodeWithLocalIP(t *testing.T) {
	nodes, expected := mirroredGroup(
		selfPeer(),
		domain.Peer{Name: "worker-1-old", IP: testNodeIP},
		domain.Peer{Name: "worker-2", IP: "10.0.0.2"},
	)
	cluster := &fakeCluster{}

	var logs bytes.Buffer

	New(nodes, expected, cluster, joinerParams(), logtest.NewJSONLogger(&logs)).Bootstrap(t.Context())

	assertJoinedOnce(t, cluster, "10.0.0.2:8500")

	if got := slices.Sorted(slices.Values(nodes.candidateGets())); !slices.Equal(got, []string{"worker-1-old", "worker-2"}) {
		t.Errorf("candidate reads are %v, want worker-1-old and worker-2 once each", got)
	}

	records := logtest.Drain(t, &logs)
	logtest.AssertSnakeCaseKeys(t, records)
	assertLocalIPDrop(t, records, "worker-1-old")
}

func TestStaleCloneOnlyGroupStartsAlone(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		nodes, expected := mirroredGroup(
			selfPeer(),
			domain.Peer{Name: "worker-1-old", IP: testNodeIP},
		)
		cancelOnOwnRead(nodes, 2, cancel)
		cluster := &fakeCluster{}

		var logs bytes.Buffer

		joiner := New(nodes, expected, cluster, joinerParams(), logtest.NewJSONLogger(&logs))
		joiner.Bootstrap(ctx)

		if joins := cluster.joins(); len(joins) != 0 {
			t.Errorf("join must not be called against the local IP, got %v", joins)
		}

		if !joiner.Joined() {
			t.Error("a node whose only listed peer is its own stale clone is alone and must be joined")
		}

		if got := nodes.gets(); !slices.Equal(got, []string{testNodeName}) {
			t.Errorf("reads are %v, want exactly the one read of the own Node", got)
		}

		records := logtest.Drain(t, &logs)
		logtest.AssertSnakeCaseKeys(t, records)

		clones := logtest.WithMsg(records, cloneMsg)
		if len(clones) != 1 || clones[0].Level() != "warn" || clones[0].Str("member") != "worker-1-old" {
			t.Errorf("clone records are %v, want one warning with member worker-1-old", clones)
		}

		if alone := logtest.WithMsg(records, aloneMsg); len(alone) != 1 || alone[0].Level() != "info" {
			t.Errorf("alone records are %v, want one info record", alone)
		}

		if cloneAt, aloneAt := indexOfMsg(records, cloneMsg), indexOfMsg(records, aloneMsg); cloneAt > aloneAt {
			t.Errorf("the clone record comes after the alone record in %v, want it before", records)
		}

		if dropped := logtest.WithMsg(records, droppedMsg); len(dropped) != 0 {
			t.Errorf("drop records are %v, want none: the clone is not a candidate", dropped)
		}
	})
}

func TestSlotsSampleWithinEachClass(t *testing.T) {
	notAlive, alive := peerNames("d", 6), peerNames("a", 6)
	nodes, expected, cluster := slotGroup(notAlive, alive)
	joiner := newJoiner(t, nodes, expected, cluster)

	seenNotAlive, seenAlive := map[string]bool{}, map[string]bool{}

	for range 50 {
		picked := attemptPicks(t, joiner, nodes)
		if len(picked) > 3 {
			t.Fatalf("candidate reads are %v, want at most 3 per attempt", picked)
		}

		for _, name := range picked {
			if slices.Contains(notAlive, name) {
				seenNotAlive[name] = true
			} else {
				seenAlive[name] = true
			}
		}
	}

	if len(seenNotAlive) <= 2 {
		t.Errorf("not-alive slots are not sampled: %d distinct peers over 50 attempts", len(seenNotAlive))
	}

	if len(seenAlive) <= 1 {
		t.Errorf("the alive slot is not sampled: %d distinct peers over 50 attempts", len(seenAlive))
	}
}

func TestAGroupWithoutPeersStartsAlone(t *testing.T) {
	nodes, expected := mirroredGroup(selfPeer())
	cluster := &fakeCluster{}

	joiner := newJoiner(t, nodes, expected, cluster)
	joiner.Bootstrap(t.Context())

	if joins := cluster.joins(); len(joins) != 0 {
		t.Errorf("join must not be called without peers, got %v", joins)
	}

	if !joiner.Joined() {
		t.Error("a node alone in its node group must be reported as joined")
	}

	if got := nodes.gets(); len(got) != 1 {
		t.Errorf("reads are %v, want exactly the one read of the own Node", got)
	}
}

func groupWithoutThisNode() (*fakeNodes, *fakeExpected) {
	nodes, expected := mirroredGroup(domain.Peer{Name: "worker-2", IP: "10.0.0.2", UID: "uid-2"})
	nodes.setAnswer(testNodeName, nodeAnswer{record: selfRecord()})

	return nodes, expected
}

func TestCacheWithoutThisNodeIsAFailedAttempt(t *testing.T) {
	t.Run("attempt", func(t *testing.T) {
		nodes, expected := groupWithoutThisNode()
		cluster := &fakeCluster{}

		err := newJoiner(t, nodes, expected, cluster).Attempt(t.Context())

		if err == nil {
			t.Fatal("attempt succeeded, want a failure while the cache does not list this node")
		}

		if errors.Is(err, ErrNotMember) {
			t.Errorf("attempt returned %v, want a transient failure: a lagging cache is not a verdict", err)
		}

		if joins := cluster.joins(); len(joins) != 0 {
			t.Errorf("join was called with %v, want none", joins)
		}
	})
}

func groupWithEveryCandidateDropped() (*fakeNodes, *fakeExpected) {
	nodes, expected := mirroredGroup(
		selfPeer(),
		domain.Peer{Name: "worker-2", IP: "10.0.0.2"},
		domain.Peer{Name: "worker-3", IP: "10.0.0.3"},
		domain.Peer{Name: "worker-4", IP: "10.0.0.4"},
		domain.Peer{Name: "worker-5", IP: "10.0.0.5"},
		domain.Peer{Name: "worker-6", IP: "10.0.0.6"},
		domain.Peer{Name: "worker-7", IP: "10.0.0.7"},
	)
	nodes.setAnswer("worker-2", nodeAnswer{err: notFound("worker-2")})
	nodes.setAnswer("worker-3", nodeAnswer{err: notFound("worker-3")})
	nodes.setAnswer("worker-4", nodeAnswer{record: withGroup(peerRecord("worker-4", "10.0.0.4"), "worker-2")})
	nodes.setAnswer("worker-5", nodeAnswer{record: withGroup(peerRecord("worker-5", "10.0.0.5"), "worker-2")})
	nodes.setAnswer("worker-6", nodeAnswer{record: peerRecord("worker-6", "")})
	nodes.setAnswer("worker-7", nodeAnswer{record: peerRecord("worker-7", "")})

	return nodes, expected
}

func TestDroppedCandidatesAreNotReplaced(t *testing.T) {
	t.Run("attempt", func(t *testing.T) {
		nodes, expected := groupWithEveryCandidateDropped()
		cluster := &fakeCluster{}

		err := newJoiner(t, nodes, expected, cluster).Attempt(t.Context())

		if err == nil {
			t.Fatal("attempt succeeded, want a failure when every candidate is dropped")
		}

		if errors.Is(err, ErrNotMember) {
			t.Errorf("attempt returned %v, want a transient failure: dropped candidates are not a verdict on this node", err)
		}

		gets := nodes.candidateGets()
		if len(gets) != maxSeeds {
			t.Errorf("candidate reads are %v, want exactly %d: a dropped candidate is not replaced", gets, maxSeeds)
		}

		if distinct := slices.Compact(slices.Sorted(slices.Values(gets))); len(distinct) != len(gets) {
			t.Errorf("candidate reads are %v, want each candidate read once", gets)
		}

		if joins := cluster.joins(); len(joins) != 0 {
			t.Errorf("join was called with %v, want none without a usable seed", joins)
		}
	})
}

func TestCandidateDropRules(t *testing.T) {
	cases := []struct {
		name    string
		cacheIP string
		answer  nodeAnswer
		reason  string
		level   string
	}{
		{
			name:    "not found",
			cacheIP: "10.0.0.3",
			answer:  nodeAnswer{err: notFound("worker-3")},
			reason:  "not_found",
			level:   "info",
		},
		{
			name:    "label with an empty value",
			cacheIP: "10.0.0.3",
			answer:  nodeAnswer{record: withGroup(peerRecord("worker-3", "10.0.0.3"), "")},
			reason:  "left_node_group",
			level:   "info",
		},
		{
			name:    "relabeled into another group",
			cacheIP: "10.0.0.3",
			answer:  nodeAnswer{record: withGroup(peerRecord("worker-3", "10.0.0.3"), "worker-2")},
			reason:  "left_node_group",
			level:   "info",
		},
		{
			name:    "no InternalIP",
			cacheIP: "10.0.0.3",
			answer:  nodeAnswer{record: peerRecord("worker-3", "")},
			reason:  "no_internal_ip",
			level:   "warn",
		},
		{
			name:    "fresh InternalIP is the local one",
			cacheIP: "",
			answer:  nodeAnswer{record: peerRecord("worker-3", testNodeIP)},
			reason:  "local_internal_ip",
			level:   "warn",
		},
		{
			name:    "read failure",
			cacheIP: "10.0.0.3",
			answer:  nodeAnswer{err: apierrors.NewServiceUnavailable("etcd is unavailable")},
			reason:  "read_failed",
			level:   "warn",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nodes, expected := mirroredGroup(
				selfPeer(),
				domain.Peer{Name: "worker-2", IP: "10.0.0.2", UID: "uid-2"},
				domain.Peer{Name: "worker-3", IP: tc.cacheIP, UID: "uid-3"},
			)
			nodes.setAnswer("worker-3", tc.answer)
			cluster := &fakeCluster{}

			var logs bytes.Buffer

			joiner := New(nodes, expected, cluster, joinerParams(), logtest.NewJSONLogger(&logs))

			if err := joiner.Attempt(t.Context()); err != nil {
				t.Fatalf("attempt returned %v, want the valid candidate to be joined", err)
			}

			assertJoinedOnce(t, cluster, "10.0.0.2:8500")

			records := logtest.Drain(t, &logs)
			logtest.AssertSnakeCaseKeys(t, records)

			dropped := logtest.WithMsg(records, droppedMsg)
			if len(dropped) != 1 {
				t.Fatalf("drop records are %v, want exactly one for worker-3", dropped)
			}

			record := dropped[0]
			if record.Str("member") != "worker-3" || record.Str("reason") != tc.reason || record.Level() != tc.level {
				t.Errorf("drop record is %v, want member worker-3, reason %q at level %q", record, tc.reason, tc.level)
			}

			if _, hasError := record["error"]; hasError != (tc.reason == "read_failed") {
				t.Errorf("drop record is %v, want an error key only for a failed read", record)
			}
		})
	}
}

func TestCandidateUIDIsNotCompared(t *testing.T) {
	nodes, expected := mirroredGroup(
		selfPeer(),
		domain.Peer{Name: "worker-2", IP: "10.0.0.2", UID: "uid-2"},
	)
	nodes.setAnswer("worker-2", nodeAnswer{record: domain.NodeRecord{
		Name:      "worker-2",
		UID:       "uid-2-new",
		IP:        "10.0.0.2",
		NodeGroup: testNodeGroup,
	}})
	cluster := &fakeCluster{}

	if err := newJoiner(t, nodes, expected, cluster).Attempt(t.Context()); err != nil {
		t.Fatalf("attempt returned %v, want a candidate with a new UID to be joined", err)
	}

	assertJoinedOnce(t, cluster, "10.0.0.2:8500")
}

func TestSeedAddressesComeFromTheFreshAnswer(t *testing.T) {
	nodes, expected := mirroredGroup(
		selfPeer(),
		domain.Peer{Name: "worker-2", IP: "10.0.0.9", UID: "uid-2"},
	)
	nodes.setAnswer("worker-2", nodeAnswer{record: peerRecord("worker-2", "10.0.0.2")})
	cluster := &fakeCluster{}

	if err := newJoiner(t, nodes, expected, cluster).Attempt(t.Context()); err != nil {
		t.Fatalf("attempt returned %v, want success", err)
	}

	assertJoinedOnce(t, cluster, "10.0.0.2:8500")

	for _, seeds := range cluster.joins() {
		if slices.Contains(seeds, "10.0.0.9:8500") {
			t.Errorf("join seeds are %v, want the fresh address, never the cached 10.0.0.9", seeds)
		}
	}
}

func TestCandidateNamesComeFromTheInformerCache(t *testing.T) {
	nodes, expected := mirroredGroup(
		selfPeer(),
		domain.Peer{Name: "worker-2", IP: "10.0.0.2", UID: "uid-2"},
		domain.Peer{Name: "worker-3", IP: "10.0.0.3", UID: "uid-3"},
	)
	nodes.setAnswer("worker-7", nodeAnswer{record: peerRecord("worker-7", "10.0.0.7")})
	nodes.setAnswer("worker-9", nodeAnswer{record: peerRecord("worker-9", "10.0.0.9")})
	cluster := &fakeCluster{}
	cluster.setMembers(testNodeName, "worker-2", "worker-9")
	joiner := newJoiner(t, nodes, expected, cluster)

	for range 20 {
		picked := attemptPicks(t, joiner, nodes)

		if slices.Contains(picked, "worker-7") || slices.Contains(picked, "worker-9") {
			t.Fatalf("candidate reads are %v, want neither worker-7, known to the API only, nor worker-9, listed by gossip only", picked)
		}

		if !slices.Equal(picked, []string{"worker-2", "worker-3"}) {
			t.Fatalf("candidate reads are %v, want worker-2 and worker-3: the cached Nodes but this one", picked)
		}
	}

	for _, seeds := range cluster.joins() {
		if got := slices.Sorted(slices.Values(seeds)); !slices.Equal(got, []string{"10.0.0.2:8500", "10.0.0.3:8500"}) {
			t.Fatalf("join seeds are %v, want the addresses of worker-2 and worker-3 only", seeds)
		}
	}

	expected.setPeers([]domain.Peer{
		selfPeer(),
		{Name: "worker-2", IP: "10.0.0.2", UID: "uid-2"},
		{Name: "worker-7", IP: "10.0.0.7", UID: "uid-7"},
	})

	if picked := attemptPicks(t, joiner, nodes); !slices.Equal(picked, []string{"worker-2", "worker-7"}) {
		t.Errorf("candidate reads after the cache changed are %v, want worker-2 and worker-7", picked)
	}
}

func TestCandidatesAreTwoNotAliveAndOneAlivePeer(t *testing.T) {
	notAlive, alive := peerNames("d", 3), peerNames("a", 3)
	nodes, expected, cluster := slotGroup(notAlive, alive)
	joiner := newJoiner(t, nodes, expected, cluster)

	for range 200 {
		assertSlots(t, attemptPicks(t, joiner, nodes), notAlive, alive, 2, 1)
	}
}

func TestNotAlivePeersFillEverySlotWhenNoPeerIsAlive(t *testing.T) {
	notAlive := peerNames("d", 5)
	nodes, expected, cluster := slotGroup(notAlive, nil)
	joiner := newJoiner(t, nodes, expected, cluster)

	for range 100 {
		assertSlots(t, attemptPicks(t, joiner, nodes), notAlive, nil, 3, 0)
	}
}

func TestAlivePeersFillEverySlotWhenNoPeerIsMissing(t *testing.T) {
	alive := peerNames("a", 5)
	nodes, expected, cluster := slotGroup(nil, alive)
	joiner := newJoiner(t, nodes, expected, cluster)

	for range 100 {
		assertSlots(t, attemptPicks(t, joiner, nodes), nil, alive, 0, 3)
	}
}

func TestAShortClassDoesNotLendItsSlots(t *testing.T) {
	cases := []struct {
		name         string
		notAlive     []string
		alive        []string
		wantNotAlive int
		wantAlive    int
	}{
		{name: "one not alive, three alive", notAlive: peerNames("d", 1), alive: peerNames("a", 3), wantNotAlive: 1, wantAlive: 1},
		{name: "four not alive, one alive", notAlive: peerNames("d", 4), alive: peerNames("a", 1), wantNotAlive: 2, wantAlive: 1},
		{name: "one of each", notAlive: peerNames("d", 1), alive: peerNames("a", 1), wantNotAlive: 1, wantAlive: 1},
		{name: "one not alive only", notAlive: peerNames("d", 1), wantNotAlive: 1},
		{name: "one alive only", alive: peerNames("a", 1), wantAlive: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nodes, expected, cluster := slotGroup(tc.notAlive, tc.alive)
			joiner := newJoiner(t, nodes, expected, cluster)

			for range 50 {
				assertSlots(t, attemptPicks(t, joiner, nodes), tc.notAlive, tc.alive, tc.wantNotAlive, tc.wantAlive)
			}
		})
	}
}

func TestGossipMembersOutsideTheGroupAreIgnored(t *testing.T) {
	nodes, expected, cluster := slotGroup([]string{"d1", "d2"}, []string{"a1"})
	nodes.setAnswer("ghost", nodeAnswer{record: peerRecord("ghost", "10.0.9.9")})
	cluster.setMembers(testNodeName, "ghost", "a1")
	joiner := newJoiner(t, nodes, expected, cluster)

	notAlive, alive, clones, err := joiner.candidates()
	if err != nil {
		t.Fatalf("candidates returned %v, want the cache split", err)
	}

	if !slices.Equal(notAlive, []string{"d1", "d2"}) || !slices.Equal(alive, []string{"a1"}) || len(clones) != 0 {
		t.Errorf("candidates are not alive %v, alive %v, clones %v; want [d1 d2], [a1] and none", notAlive, alive, clones)
	}

	for range 50 {
		if picked := attemptPicks(t, joiner, nodes); !slices.Equal(picked, []string{"a1", "d1", "d2"}) {
			t.Fatalf("candidate reads are %v, want a1, d1 and d2", picked)
		}
	}

	if got := nodes.getsOf("ghost"); got != 0 {
		t.Errorf("ghost was read %d times, want never: only the cache names candidates", got)
	}
}

func TestTheOwnNodeIsNeverACandidate(t *testing.T) {
	cases := []struct {
		name     string
		notAlive []string
		alive    []string
	}{
		{name: "a not-alive peer", notAlive: []string{"d1"}},
		{name: "not-alive and alive peers", notAlive: []string{"d1", "d2"}, alive: []string{"a1"}},
		{name: "alive peers", alive: []string{"a1", "a2"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nodes, expected, cluster := slotGroup(tc.notAlive, tc.alive)
			joiner := newJoiner(t, nodes, expected, cluster)

			notAlive, alive, _, err := joiner.candidates()
			if err != nil {
				t.Fatalf("candidates returned %v, want the cache split", err)
			}

			if slices.Contains(notAlive, testNodeName) || slices.Contains(alive, testNodeName) {
				t.Errorf("candidates are not alive %v and alive %v, want neither to hold the own Node", notAlive, alive)
			}

			for range 20 {
				attemptPicks(t, joiner, nodes)

				if got := nodes.getsOf(testNodeName); got != 1 {
					t.Fatalf("the own Node was read %d times in one attempt, want once", got)
				}
			}

			selfSeed := net.JoinHostPort(testNodeIP, strconv.Itoa(testPort))
			for _, seeds := range cluster.joins() {
				if slices.Contains(seeds, selfSeed) {
					t.Errorf("join seeds are %v, want the own address never among them", seeds)
				}
			}
		})
	}
}

func TestCandidateSelectionDoesNotMutateTheSharedExpectedSlice(t *testing.T) {
	cases := []struct {
		name             string
		concurrentReader bool
	}{
		{name: "attempts only"},
		{name: "with a concurrent reader", concurrentReader: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			peers := []domain.Peer{
				{Name: "worker-0", IP: "10.0.0.10", UID: "uid-0"},
				selfPeer(),
				{Name: "worker-1-old", IP: testNodeIP, UID: "uid-1-old"},
				{Name: "worker-2", IP: "10.0.0.2", UID: "uid-2"},
				{Name: "worker-3", IP: "10.0.0.3", UID: "uid-3"},
				{Name: "worker-4", IP: "10.0.0.4", UID: "uid-4"},
				{Name: "worker-5", IP: "10.0.0.5", UID: "uid-5"},
			}
			nodes, _ := mirroredGroup(peers...)

			shared := make([]domain.Peer, len(peers), len(peers)+4)
			copy(shared, peers)
			before := slices.Clone(shared[:cap(shared)])

			expected := &fakeExpected{peers: shared, revision: 1}
			cluster := &fakeCluster{}
			cluster.setMembers(testNodeName, "worker-2", "worker-4")
			joiner := newJoiner(t, nodes, expected, cluster)

			stop := make(chan struct{})

			var (
				reader  sync.WaitGroup
				changed atomic.Bool
			)

			if tc.concurrentReader {
				reader.Go(func() {
					for {
						select {
						case <-stop:
							return
						default:
						}

						for i, peer := range shared[:cap(shared)] {
							if peer != before[i] {
								changed.Store(true)
							}
						}

						runtime.Gosched()
					}
				})
			}

			for attempt := range 50 {
				if err := joiner.Attempt(t.Context()); err != nil {
					t.Errorf("attempt %d returned %v, want success", attempt+1, err)

					break
				}
			}

			close(stop)
			reader.Wait()

			if got := shared[:cap(shared)]; !slices.Equal(got, before) {
				t.Errorf("the shared expected slice is %v after the attempts, want it unchanged %v", got, before)
			}

			if changed.Load() {
				t.Error("a concurrent reader of the shared expected slice saw it change during the attempts")
			}
		})
	}
}

func TestStaleClonePrefilterAppliesOnlyToTheAloneRule(t *testing.T) {
	cases := []struct {
		name    string
		members []string
	}{
		{name: "neither alive", members: []string{testNodeName}},
		{name: "clone alive", members: []string{testNodeName, "worker-1-old"}},
		{name: "peer alive", members: []string{testNodeName, "worker-2"}},
		{name: "both alive", members: []string{testNodeName, "worker-1-old", "worker-2"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nodes, expected := mirroredGroup(
				selfPeer(),
				domain.Peer{Name: "worker-1-old", IP: testNodeIP},
				domain.Peer{Name: "worker-2", IP: "10.0.0.2"},
			)
			cluster := &fakeCluster{}
			cluster.setMembers(tc.members...)

			var logs bytes.Buffer

			joiner := New(nodes, expected, cluster, joinerParams(), logtest.NewJSONLogger(&logs))

			if err := joiner.Attempt(t.Context()); err != nil {
				t.Fatalf("attempt returned %v, want worker-2 to be joined", err)
			}

			if got := slices.Sorted(slices.Values(nodes.candidateGets())); !slices.Equal(got, []string{"worker-1-old", "worker-2"}) {
				t.Errorf("candidate reads are %v, want worker-1-old and worker-2 once each", got)
			}

			assertJoinedOnce(t, cluster, "10.0.0.2:8500")

			records := logtest.Drain(t, &logs)
			logtest.AssertSnakeCaseKeys(t, records)
			assertLocalIPDrop(t, records, "worker-1-old")

			if got := logtest.WithMsg(records, cloneMsg); len(got) != 0 {
				t.Errorf("clone records are %v, want none outside the alone rule", got)
			}

			if got := logtest.WithMsg(records, aloneMsg); len(got) != 0 {
				t.Errorf("alone records are %v, want none next to a real peer", got)
			}
		})
	}
}

const (
	cloneMsg = "node shares the local InternalIP, not counted as a peer"
	aloneMsg = "no peers in node group, starting alone"
)

func peerNames(prefix string, n int) []string {
	names := make([]string, 0, n)
	for i := range n {
		names = append(names, prefix+strconv.Itoa(i+1))
	}

	return names
}

func slotGroup(notAlive, alive []string) (*fakeNodes, *fakeExpected, *fakeCluster) {
	peers := make([]domain.Peer, 0, 1+len(notAlive)+len(alive))
	peers = append(peers, selfPeer())

	for i, name := range slices.Concat(notAlive, alive) {
		peers = append(peers, domain.Peer{Name: name, IP: "10.0.1." + strconv.Itoa(i+1)})
	}

	nodes, expected := mirroredGroup(peers...)

	cluster := &fakeCluster{}
	cluster.setMembers(slices.Concat([]string{testNodeName}, alive)...)

	return nodes, expected, cluster
}

func attemptPicks(t *testing.T, joiner *Joiner, nodes *fakeNodes) []string {
	t.Helper()

	nodes.resetJournal()

	if err := joiner.Attempt(t.Context()); err != nil {
		t.Fatalf("attempt returned %v, want success", err)
	}

	return slices.Sorted(slices.Values(nodes.candidateGets()))
}

func assertSlots(t *testing.T, picked, notAlive, alive []string, wantNotAlive, wantAlive int) {
	t.Helper()

	if distinct := slices.Compact(slices.Clone(picked)); len(distinct) != len(picked) {
		t.Fatalf("candidate reads are %v, want each peer read at most once", picked)
	}

	gotNotAlive, gotAlive := 0, 0

	for _, name := range picked {
		switch {
		case slices.Contains(notAlive, name):
			gotNotAlive++
		case slices.Contains(alive, name):
			gotAlive++
		default:
			t.Fatalf("candidate reads are %v, want only peers of the group, %q is not one", picked, name)
		}
	}

	if gotNotAlive != wantNotAlive || gotAlive != wantAlive {
		t.Fatalf("candidate reads are %v: %d not alive and %d alive, want %d and %d",
			picked, gotNotAlive, gotAlive, wantNotAlive, wantAlive)
	}
}

func assertLocalIPDrop(t *testing.T, records []logtest.Record, member string) {
	t.Helper()

	dropped := logtest.WithMsg(records, droppedMsg)
	if len(dropped) != 1 {
		t.Errorf("drop records are %v, want exactly one for %s", dropped, member)

		return
	}

	record := dropped[0]
	if record.Str("member") != member || record.Str("reason") != "local_internal_ip" || record.Level() != "warn" {
		t.Errorf("drop record is %v, want member %s, reason local_internal_ip at level warn", record, member)
	}
}

func indexOfMsg(records []logtest.Record, msg string) int {
	return slices.IndexFunc(records, func(record logtest.Record) bool { return record.Msg() == msg })
}
