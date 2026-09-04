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

// Package membership tracks which Nodes should be in the NodeGroup and derives
// the quorum size from that count.
//
// Every Node with the group label counts: no readiness or deletion filter.
// Filtering by health would shrink the quorum along with a failing cluster and
// let both halves of a partition think they still hold it. An unreachable API
// freezes the informer on the last known state, so the view stays local, as the
// fast path needs.
package membership

import (
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

type Membership struct {
	logger *log.Logger

	mu     sync.Mutex
	peers  map[string]domain.Peer
	synced bool
}

func New(logger *log.Logger) *Membership {
	return &Membership{
		logger: logger,
		peers:  make(map[string]domain.Peer),
	}
}

// Upsert records a Node of the group. Logging starts after MarkSynced, so the
// initial cache fill does not flood it.
func (m *Membership) Upsert(peer domain.Peer) {
	m.mu.Lock()
	previous, existed := m.peers[peer.Name]
	m.peers[peer.Name] = peer
	expected, quorum, announce := len(m.peers), domain.QuorumSize(len(m.peers)), m.synced
	m.mu.Unlock()

	if announce && (!existed || previous != peer) {
		m.logger.Info("expected membership changed",
			"member", peer.Name,
			"expected", expected,
			"quorum", quorum,
		)
	}
}

func (m *Membership) Delete(name string) {
	m.mu.Lock()
	_, existed := m.peers[name]
	delete(m.peers, name)
	expected, quorum, announce := len(m.peers), domain.QuorumSize(len(m.peers)), m.synced
	m.mu.Unlock()

	if announce && existed {
		m.logger.Info("expected membership changed",
			"removed", name,
			"expected", expected,
			"quorum", quorum,
		)
	}
}

// MarkSynced runs once the informer finished its initial fill: it logs the
// starting view and unmutes per-change logging.
func (m *Membership) MarkSynced() {
	m.mu.Lock()
	m.synced = true
	expected, quorum := len(m.peers), domain.QuorumSize(len(m.peers))
	m.mu.Unlock()

	m.logger.Info("expected membership synced", "expected", expected, "quorum", quorum)
}

// Snapshot returns the expected peers sorted by name, their count and the quorum
// size.
func (m *Membership) Snapshot() ([]domain.Peer, int, int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	peers := make([]domain.Peer, 0, len(m.peers))
	for _, peer := range m.peers {
		peers = append(peers, peer)
	}

	slices.SortFunc(peers, func(a, b domain.Peer) int { return strings.Compare(a.Name, b.Name) })

	return peers, len(peers), domain.QuorumSize(len(peers))
}

// ListNodeGroup serves the join usecase from the informer cache instead of a
// direct API LIST. The arguments only satisfy the NodeLister interface.
func (m *Membership) ListNodeGroup(_ context.Context, _ string) ([]domain.Peer, error) {
	peers, _, _ := m.Snapshot()

	return peers, nil
}
