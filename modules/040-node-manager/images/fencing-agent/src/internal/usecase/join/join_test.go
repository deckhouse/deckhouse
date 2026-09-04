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
	"slices"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

type fakeLister struct {
	peers  []domain.Peer
	err    error
	calls  atomic.Int64
	onCall func(call int64)
}

func (f *fakeLister) ListNodeGroup(_ context.Context, _ string) ([]domain.Peer, error) {
	call := f.calls.Add(1)
	if f.onCall != nil {
		f.onCall(call)
	}

	return f.peers, f.err
}

type fakeCluster struct {
	failures int
	seeds    [][]string
}

func (f *fakeCluster) Join(seeds []string) (int, error) {
	f.seeds = append(f.seeds, slices.Clone(seeds))

	if len(f.seeds) <= f.failures {
		return 0, errors.New("connection refused")
	}

	return len(seeds), nil
}

func (f *fakeCluster) NumMembers() int {
	return 1
}

func newJoiner(t *testing.T, lister NodeLister, cluster Cluster) *Joiner {
	t.Helper()

	return New(lister, cluster, Params{
		NodeName:         "worker-1",
		NodeIP:           "10.0.0.1",
		NodeGroup:        "worker",
		MemberlistPort:   8500,
		APITimeout:       time.Second,
		RetryInterval:    time.Millisecond,
		MaxRetryInterval: 2 * time.Millisecond,
	}, log.NewNop())
}

func TestSeedListExcludesLocalNodeAndNodesWithoutIP(t *testing.T) {
	lister := &fakeLister{peers: []domain.Peer{
		{Name: "worker-1", IP: "10.0.0.1"},
		{Name: "worker-2", IP: "10.0.0.2"},
		{Name: "worker-3", IP: ""},
		{Name: "worker-4", IP: "10.0.0.4"},
	}}
	cluster := &fakeCluster{}

	newJoiner(t, lister, cluster).Bootstrap(t.Context())

	if len(cluster.seeds) != 1 {
		t.Fatalf("expected a single join call, got %d", len(cluster.seeds))
	}

	want := []string{"10.0.0.2:8500", "10.0.0.4:8500"}
	if !slices.Equal(cluster.seeds[0], want) {
		t.Errorf("seed list is %v, want %v", cluster.seeds[0], want)
	}
}

func TestSeedListExcludesStaleNodeWithLocalIP(t *testing.T) {
	lister := &fakeLister{peers: []domain.Peer{
		{Name: "worker-1", IP: "10.0.0.1"},
		// Leftover Node object of this very machine under its previous name.
		{Name: "worker-1-old", IP: "10.0.0.1"},
		{Name: "worker-2", IP: "10.0.0.2"},
	}}
	cluster := &fakeCluster{}

	newJoiner(t, lister, cluster).Bootstrap(t.Context())

	want := []string{"10.0.0.2:8500"}
	if len(cluster.seeds) != 1 || !slices.Equal(cluster.seeds[0], want) {
		t.Errorf("seed list is %v, want a single call with %v", cluster.seeds, want)
	}
}

func TestStaleCloneOnlyGroupStartsAlone(t *testing.T) {
	lister := &fakeLister{peers: []domain.Peer{
		{Name: "worker-1", IP: "10.0.0.1"},
		{Name: "worker-1-old", IP: "10.0.0.1"},
	}}
	cluster := &fakeCluster{}

	joiner := newJoiner(t, lister, cluster)
	joiner.Bootstrap(t.Context())

	if len(cluster.seeds) != 0 {
		t.Errorf("join must not be called against the local IP, got %v", cluster.seeds)
	}

	if !joiner.Joined() {
		t.Error("a node whose only listed peer is its own stale clone is alone and must be joined")
	}
}

func TestPeersWithoutAddressesAreNotAlone(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// Real peers, but the cloud controller has not filled in their addresses yet:
	// the agent must keep retrying, not declare itself alone.
	lister := &fakeLister{peers: []domain.Peer{
		{Name: "worker-1", IP: "10.0.0.1"},
		{Name: "worker-2", IP: ""},
		{Name: "worker-3", IP: ""},
	}}
	lister.onCall = func(call int64) {
		if call >= 3 {
			cancel()
		}
	}

	cluster := &fakeCluster{}

	joiner := newJoiner(t, lister, cluster)
	joiner.Bootstrap(ctx)

	if joiner.Joined() {
		t.Error("joined state must stay unset while peers have no addresses")
	}

	if len(cluster.seeds) != 0 {
		t.Errorf("join must not be attempted without usable seeds, got %v", cluster.seeds)
	}

	if got := lister.calls.Load(); got < 3 {
		t.Errorf("expected at least 3 listing attempts, got %d", got)
	}
}

func TestSeedListIsCappedAndSampled(t *testing.T) {
	peers := make([]domain.Peer, 0, 3*maxSeeds)
	for i := range cap(peers) {
		peers = append(peers, domain.Peer{
			Name: "worker-" + strconv.Itoa(i+100),
			IP:   "10.0.1." + strconv.Itoa(i),
		})
	}

	seen := map[string]bool{}

	for range 10 {
		cluster := &fakeCluster{}
		newJoiner(t, &fakeLister{peers: peers}, cluster).Bootstrap(t.Context())

		if len(cluster.seeds[0]) != maxSeeds {
			t.Fatalf("seed list has %d entries, want %d", len(cluster.seeds[0]), maxSeeds)
		}

		for _, seed := range cluster.seeds[0] {
			seen[seed] = true
		}
	}

	// A fixed prefix of the API response would keep hitting the same unreachable
	// nodes, so the sample has to differ between attempts.
	if len(seen) <= maxSeeds {
		t.Errorf("seed list is not sampled: %d distinct seeds over 10 attempts", len(seen))
	}
}

func TestEmptySeedListStartsAlone(t *testing.T) {
	lister := &fakeLister{peers: []domain.Peer{{Name: "worker-1", IP: "10.0.0.1"}}}
	cluster := &fakeCluster{}

	joiner := newJoiner(t, lister, cluster)
	joiner.Bootstrap(t.Context())

	if len(cluster.seeds) != 0 {
		t.Errorf("join must not be called without peers, got %v", cluster.seeds)
	}

	if !joiner.Joined() {
		t.Error("a node alone in its node group must be reported as joined")
	}
}

func TestBootstrapRetriesUntilJoinSucceeds(t *testing.T) {
	lister := &fakeLister{peers: []domain.Peer{
		{Name: "worker-1", IP: "10.0.0.1"},
		{Name: "worker-2", IP: "10.0.0.2"},
	}}
	cluster := &fakeCluster{failures: 2}

	joiner := newJoiner(t, lister, cluster)
	joiner.Bootstrap(t.Context())

	if len(cluster.seeds) != 3 {
		t.Errorf("expected 3 join attempts, got %d", len(cluster.seeds))
	}

	// The seed list is rebuilt per attempt instead of reusing the first snapshot.
	if got := lister.calls.Load(); got != 3 {
		t.Errorf("expected 3 node listings, got %d", got)
	}

	if !joiner.Joined() {
		t.Error("joined state is not set after a successful join")
	}
}

func TestBootstrapRetriesOnListError(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	lister := &fakeLister{err: errors.New("api server is down")}
	lister.onCall = func(call int64) {
		if call >= 2 {
			cancel()
		}
	}

	cluster := &fakeCluster{}

	joiner := newJoiner(t, lister, cluster)
	joiner.Bootstrap(ctx)

	if got := lister.calls.Load(); got < 2 {
		t.Errorf("expected the list to be retried, got %d attempts", got)
	}

	if len(cluster.seeds) != 0 {
		t.Errorf("join must not be attempted when the node list is unavailable, got %v", cluster.seeds)
	}

	if joiner.Joined() {
		t.Error("joined state must stay unset while the node list cannot be read")
	}
}

func TestBootstrapStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	lister := &fakeLister{err: errors.New("api server is down")}

	done := make(chan struct{})
	go func() {
		newJoiner(t, lister, &fakeCluster{}).Bootstrap(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("bootstrap did not return after the context was cancelled")
	}
}
