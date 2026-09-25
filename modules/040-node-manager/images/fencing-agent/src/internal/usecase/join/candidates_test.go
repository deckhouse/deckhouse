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

	"fencing-agent/internal/domain"
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
	assertOnlyTheOwnNodeWasRead(t, nodes)
}

func TestSeedListExcludesStaleNodeWithLocalIP(t *testing.T) {
	nodes, expected := mirroredGroup(
		selfPeer(),
		domain.Peer{Name: "worker-1-old", IP: testNodeIP},
		domain.Peer{Name: "worker-2", IP: "10.0.0.2"},
	)
	cluster := &fakeCluster{}

	newJoiner(t, nodes, expected, cluster).Bootstrap(t.Context())

	assertJoinedOnce(t, cluster, "10.0.0.2:8500")
	assertOnlyTheOwnNodeWasRead(t, nodes)
}

// A clone stayed in the alive/notAlive classes and could be sampled into the
// seeds, where only a fresh read of it dropped it. pick splits its slots between
// the classes, so a clone alone in one class also shrank the other class's sample.
func TestStaleCloneDoesNotTakeSeedSlots(t *testing.T) {
	nodes, expected := mirroredGroup(
		selfPeer(),
		domain.Peer{Name: "worker-1-old", IP: testNodeIP},
		domain.Peer{Name: "worker-2", IP: "10.0.0.2"},
		domain.Peer{Name: "worker-3", IP: "10.0.0.3"},
	)
	cluster := &fakeCluster{}
	cluster.setMembers(testNodeName, "worker-2", "worker-3")

	if err := newJoiner(t, nodes, expected, cluster).Attempt(t.Context()); err != nil {
		t.Fatalf("attempt returned %v, want both peers to be seeded", err)
	}

	assertJoinedOnce(t, cluster, "10.0.0.2:8500", "10.0.0.3:8500")
}

// Clones taking every sampled slot left the attempt without a usable address, so
// a reachable peer waited for the next backoff.
func TestStaleClonesNeverStarveTheSeedList(t *testing.T) {
	nodes, expected := mirroredGroup(
		selfPeer(),
		domain.Peer{Name: "worker-1-old", IP: testNodeIP},
		domain.Peer{Name: "worker-1-older", IP: testNodeIP},
		domain.Peer{Name: "worker-1-oldest", IP: testNodeIP},
		domain.Peer{Name: "worker-2", IP: "10.0.0.2"},
	)
	cluster := &fakeCluster{}

	if err := newJoiner(t, nodes, expected, cluster).Attempt(t.Context()); err != nil {
		t.Fatalf("attempt returned %v, want the one real peer to be seeded", err)
	}

	assertJoinedOnce(t, cluster, "10.0.0.2:8500")
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

		joiner := newJoiner(t, nodes, expected, cluster)
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
	})
}

func TestSlotsSampleWithinEachClass(t *testing.T) {
	notAlive, alive := peerNames("d", 6), peerNames("a", 6)
	nodes, expected, cluster := slotGroup(notAlive, alive)
	joiner := newJoiner(t, nodes, expected, cluster)

	seenNotAlive, seenAlive := map[string]bool{}, map[string]bool{}

	for range 50 {
		picked := attemptPicks(t, joiner, cluster, expected)
		if len(picked) > 3 {
			t.Fatalf("candidates are %v, want at most 3 per attempt", picked)
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

func groupWithoutAnyCandidateAddress() (*fakeNodes, *fakeExpected) {
	return mirroredGroup(
		selfPeer(),
		domain.Peer{Name: "worker-2", IP: ""},
		domain.Peer{Name: "worker-3", IP: ""},
		domain.Peer{Name: "worker-4", IP: ""},
		domain.Peer{Name: "worker-5", IP: ""},
		domain.Peer{Name: "worker-6", IP: ""},
		domain.Peer{Name: "worker-7", IP: ""},
	)
}

func TestAGroupWithoutAnyCandidateAddressIsAFailedAttempt(t *testing.T) {
	nodes, expected := groupWithoutAnyCandidateAddress()
	cluster := &fakeCluster{}

	err := newJoiner(t, nodes, expected, cluster).Attempt(t.Context())

	if err == nil {
		t.Fatal("attempt succeeded, want a failure while no candidate has an address")
	}

	if errors.Is(err, ErrNotMember) {
		t.Errorf("attempt returned %v, want a transient failure: peers without an address are not a verdict on this node", err)
	}

	if joins := cluster.joins(); len(joins) != 0 {
		t.Errorf("join was called with %v, want none without a usable seed", joins)
	}

	assertOnlyTheOwnNodeWasRead(t, nodes)
}

// A peer whose cached Node carries no InternalIP can never be a seed, and the
// cache says so before the slots are handed out.
func TestAPeerWithoutAnAddressDoesNotTakeSeedSlots(t *testing.T) {
	nodes, expected := mirroredGroup(
		selfPeer(),
		domain.Peer{Name: "worker-2", IP: ""},
		domain.Peer{Name: "worker-3", IP: "10.0.0.3"},
		domain.Peer{Name: "worker-4", IP: "10.0.0.4"},
		domain.Peer{Name: "worker-5", IP: "10.0.0.5"},
	)
	cluster := &fakeCluster{}
	cluster.setMembers(testNodeName, "worker-3", "worker-4", "worker-5")

	if err := newJoiner(t, nodes, expected, cluster).Attempt(t.Context()); err != nil {
		t.Fatalf("attempt returned %v, want the three peers with an address to be seeded", err)
	}

	assertJoinedOnce(t, cluster, "10.0.0.3:8500", "10.0.0.4:8500", "10.0.0.5:8500")
}

func TestSeedAddressesComeFromTheInformerCache(t *testing.T) {
	nodes, expected := mirroredGroup(
		selfPeer(),
		domain.Peer{Name: "worker-2", IP: "10.0.0.9", UID: "uid-2"},
	)
	nodes.setAnswer("worker-2", nodeAnswer{record: peerRecord("worker-2", "10.0.0.2")})
	cluster := &fakeCluster{}

	if err := newJoiner(t, nodes, expected, cluster).Attempt(t.Context()); err != nil {
		t.Fatalf("attempt returned %v, want success", err)
	}

	assertJoinedOnce(t, cluster, "10.0.0.9:8500")
	assertOnlyTheOwnNodeWasRead(t, nodes)
}

func TestCandidatesComeFromTheInformerCache(t *testing.T) {
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
		picked := attemptPicks(t, joiner, cluster, expected)

		if slices.Contains(picked, "worker-7") || slices.Contains(picked, "worker-9") {
			t.Fatalf("candidates are %v, want neither worker-7, known to the API only, nor worker-9, listed by gossip only", picked)
		}

		if !slices.Equal(picked, []string{"worker-2", "worker-3"}) {
			t.Fatalf("candidates are %v, want worker-2 and worker-3: the cached Nodes but this one", picked)
		}
	}

	assertOnlyTheOwnNodeWasRead(t, nodes)

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

	if picked := attemptPicks(t, joiner, cluster, expected); !slices.Equal(picked, []string{"worker-2", "worker-7"}) {
		t.Errorf("candidates after the cache changed are %v, want worker-2 and worker-7", picked)
	}
}

func TestCandidatesAreTwoNotAliveAndOneAlivePeer(t *testing.T) {
	notAlive, alive := peerNames("d", 3), peerNames("a", 3)
	nodes, expected, cluster := slotGroup(notAlive, alive)
	joiner := newJoiner(t, nodes, expected, cluster)

	for range 200 {
		assertSlots(t, attemptPicks(t, joiner, cluster, expected), notAlive, alive, 2, 1)
	}
}

func TestNotAlivePeersFillEverySlotWhenNoPeerIsAlive(t *testing.T) {
	notAlive := peerNames("d", 5)
	nodes, expected, cluster := slotGroup(notAlive, nil)
	joiner := newJoiner(t, nodes, expected, cluster)

	for range 100 {
		assertSlots(t, attemptPicks(t, joiner, cluster, expected), notAlive, nil, 3, 0)
	}
}

func TestAlivePeersFillEverySlotWhenNoPeerIsMissing(t *testing.T) {
	alive := peerNames("a", 5)
	nodes, expected, cluster := slotGroup(nil, alive)
	joiner := newJoiner(t, nodes, expected, cluster)

	for range 100 {
		assertSlots(t, attemptPicks(t, joiner, cluster, expected), nil, alive, 0, 3)
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
				assertSlots(t, attemptPicks(t, joiner, cluster, expected), tc.notAlive, tc.alive, tc.wantNotAlive, tc.wantAlive)
			}
		})
	}
}

func TestGossipMembersOutsideTheGroupAreIgnored(t *testing.T) {
	nodes, expected, cluster := slotGroup([]string{"d1", "d2"}, []string{"a1"})
	nodes.setAnswer("ghost", nodeAnswer{record: peerRecord("ghost", "10.0.9.9")})
	cluster.setMembers(testNodeName, "ghost", "a1")
	joiner := newJoiner(t, nodes, expected, cluster)

	c, err := joiner.candidates()
	if err != nil {
		t.Fatalf("candidates returned %v, want the cache split", err)
	}

	if !slices.Equal(peerNamesOf(c.notAlive), []string{"d1", "d2"}) ||
		!slices.Equal(peerNamesOf(c.alive), []string{"a1"}) ||
		len(c.clones) != 0 || len(c.noAddress) != 0 {
		t.Errorf("candidates are not alive %v, alive %v, clones %v, without an address %v; want [d1 d2], [a1] and none",
			peerNamesOf(c.notAlive), peerNamesOf(c.alive), c.clones, c.noAddress)
	}

	for range 50 {
		if picked := attemptPicks(t, joiner, cluster, expected); !slices.Equal(picked, []string{"a1", "d1", "d2"}) {
			t.Fatalf("candidates are %v, want a1, d1 and d2", picked)
		}
	}

	assertOnlyTheOwnNodeWasRead(t, nodes)
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

			c, err := joiner.candidates()
			if err != nil {
				t.Fatalf("candidates returned %v, want the cache split", err)
			}

			if slices.Contains(peerNamesOf(c.notAlive), testNodeName) || slices.Contains(peerNamesOf(c.alive), testNodeName) {
				t.Errorf("candidates are not alive %v and alive %v, want neither to hold the own Node",
					peerNamesOf(c.notAlive), peerNamesOf(c.alive))
			}

			for range 20 {
				nodes.resetJournal()
				attemptPicks(t, joiner, cluster, expected)

				if got := nodes.gets(); !slices.Equal(got, []string{testNodeName}) {
					t.Fatalf("reads of one attempt are %v, want the own Node once", got)
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

func TestStaleClonePrefilterIgnoresGossipLiveness(t *testing.T) {
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

			joiner := newJoiner(t, nodes, expected, cluster)

			if err := joiner.Attempt(t.Context()); err != nil {
				t.Fatalf("attempt returned %v, want worker-2 to be joined", err)
			}

			assertJoinedOnce(t, cluster, "10.0.0.2:8500")
			assertOnlyTheOwnNodeWasRead(t, nodes)
		})
	}
}

func assertOnlyTheOwnNodeWasRead(t *testing.T, nodes *fakeNodes) {
	t.Helper()

	if got := nodes.candidateGets(); len(got) != 0 {
		t.Errorf("candidates %v were read from the API, want the cache to be the only source of their addresses", got)
	}
}

func peerNamesOf(peers []domain.Peer) []string {
	names := make([]string, 0, len(peers))
	for _, peer := range peers {
		names = append(names, peer.Name)
	}

	return names
}

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

// attemptPicks reads the picked candidates off the join seeds: the cache is the
// only source of both the names and the addresses, so nothing reaches the API to
// observe any more.
func attemptPicks(t *testing.T, joiner *Joiner, cluster *fakeCluster, expected *fakeExpected) []string {
	t.Helper()

	cluster.resetJournal()

	if err := joiner.Attempt(t.Context()); err != nil {
		t.Fatalf("attempt returned %v, want success", err)
	}

	peers, _ := expected.Expected()
	byAddress := make(map[string]string, len(peers))

	for _, peer := range peers {
		byAddress[net.JoinHostPort(peer.IP, strconv.Itoa(testPort))] = peer.Name
	}

	picked := make([]string, 0, maxSeeds)

	for _, seed := range cluster.lastSeeds() {
		name, ok := byAddress[seed]
		if !ok {
			t.Fatalf("join seed %q belongs to no expected peer", seed)
		}

		picked = append(picked, name)
	}

	return slices.Sorted(slices.Values(picked))
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
