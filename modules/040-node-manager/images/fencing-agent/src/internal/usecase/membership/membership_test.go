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

package membership

import (
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

func TestQuorumFollowsMembershipChanges(t *testing.T) {
	m := New(log.NewNop())

	for _, name := range []string{"worker-1", "worker-2", "worker-3"} {
		m.Upsert(domain.Peer{Name: name, IP: "10.0.0.1"})
	}

	if _, expected, quorum := m.Snapshot(); expected != 3 || quorum != 2 {
		t.Errorf("expected 3/2, got %d/%d", expected, quorum)
	}

	// Growth: quorum must follow without any restart.
	m.Upsert(domain.Peer{Name: "worker-4", IP: "10.0.0.4"})
	m.Upsert(domain.Peer{Name: "worker-5", IP: "10.0.0.5"})

	if _, expected, quorum := m.Snapshot(); expected != 5 || quorum != 3 {
		t.Errorf("expected 5/3, got %d/%d", expected, quorum)
	}

	// Shrink: a deleted Node leaves the expected membership.
	m.Delete("worker-5")
	m.Delete("worker-4")
	m.Delete("worker-3")

	if _, expected, quorum := m.Snapshot(); expected != 2 || quorum != 2 {
		t.Errorf("expected 2/2, got %d/%d", expected, quorum)
	}
}

func TestUpsertIsIdempotentAndUpdatesIP(t *testing.T) {
	m := New(log.NewNop())

	m.Upsert(domain.Peer{Name: "worker-1", IP: "10.0.0.1"})
	m.Upsert(domain.Peer{Name: "worker-1", IP: "10.0.0.1"})
	m.Upsert(domain.Peer{Name: "worker-1", IP: "10.0.9.9"})

	peers, expected, _ := m.Snapshot()
	if expected != 1 || len(peers) != 1 || peers[0].IP != "10.0.9.9" {
		t.Errorf("unexpected snapshot: %v (expected=%d)", peers, expected)
	}
}

func TestDeleteUnknownPeerIsNoop(t *testing.T) {
	m := New(log.NewNop())
	m.Upsert(domain.Peer{Name: "worker-1", IP: "10.0.0.1"})

	m.Delete("ghost")

	if _, expected, _ := m.Snapshot(); expected != 1 {
		t.Errorf("expected 1 peer, got %d", expected)
	}
}

// ListNodeGroup adapts the snapshot to the join usecase's seed source; the
// arguments only satisfy that interface.
func TestListNodeGroupReturnsSnapshotCopy(t *testing.T) {
	m := New(log.NewNop())
	m.Upsert(domain.Peer{Name: "worker-1", IP: "10.0.0.1"})

	peers, err := m.ListNodeGroup(t.Context(), "worker")
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(peers) != 1 || peers[0].Name != "worker-1" {
		t.Fatalf("unexpected peers %v", peers)
	}

	peers[0].Name = "mutated"

	if fresh, _, _ := m.Snapshot(); fresh[0].Name != "worker-1" {
		t.Error("ListNodeGroup must return a copy, internal state was mutated")
	}
}

func TestSnapshotIsSortedByName(t *testing.T) {
	m := New(log.NewNop())
	m.Upsert(domain.Peer{Name: "worker-2", IP: "10.0.0.2"})
	m.Upsert(domain.Peer{Name: "worker-1", IP: "10.0.0.1"})

	peers, _, _ := m.Snapshot()
	if peers[0].Name != "worker-1" || peers[1].Name != "worker-2" {
		t.Errorf("snapshot is not sorted: %v", peers)
	}
}

func TestExpectedGenerationMovesOnlyOnRealChange(t *testing.T) {
	m := New(log.NewNop())
	m.Upsert(domain.Peer{Name: "worker-1", IP: "10.0.0.1", UID: "uid-1"})

	peers, first := m.Expected()
	if len(peers) != 1 {
		t.Fatalf("got %d peers, want 1", len(peers))
	}

	// The same peer again is not a change.
	m.Upsert(domain.Peer{Name: "worker-1", IP: "10.0.0.1", UID: "uid-1"})

	if _, again := m.Expected(); again != first {
		t.Errorf("generation moved from %d to %d on an identical upsert", first, again)
	}

	// A different IP, a new peer and a removal are.
	m.Upsert(domain.Peer{Name: "worker-1", IP: "10.0.0.2", UID: "uid-1"})
	_, afterIP := m.Expected()
	m.Upsert(domain.Peer{Name: "worker-2", IP: "10.0.0.3", UID: "uid-2"})
	_, afterAdd := m.Expected()
	m.Delete("worker-2")
	_, afterDelete := m.Expected()

	if !(first < afterIP && afterIP < afterAdd && afterAdd < afterDelete) {
		t.Errorf("generations %d %d %d %d, want strictly increasing", first, afterIP, afterAdd, afterDelete)
	}

	// Deleting what is not there is not a change either.
	m.Delete("worker-9")

	if _, noop := m.Expected(); noop != afterDelete {
		t.Errorf("generation moved from %d to %d on a no-op delete", afterDelete, noop)
	}
}

func TestExpectedIsSharedUntilTheNextChange(t *testing.T) {
	m := New(log.NewNop())
	m.Upsert(domain.Peer{Name: "worker-1", IP: "10.0.0.1", UID: "uid-1"})

	a, _ := m.Expected()
	b, _ := m.Expected()

	if &a[0] != &b[0] {
		t.Error("two idle reads built two slices, the cache is not doing its job")
	}

	m.Upsert(domain.Peer{Name: "worker-2", IP: "10.0.0.2", UID: "uid-2"})

	if c, _ := m.Expected(); len(c) != 2 || &c[0] == &a[0] {
		t.Error("a change did not rebuild the slice")
	}
}
