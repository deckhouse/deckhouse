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

package domain

import (
	"slices"
	"strings"
)

type View struct {
	expected map[string]Peer
	alive    []string
}

func NewView(expected []Peer, members []string) View {
	byName := make(map[string]Peer, len(expected))

	for _, peer := range expected {
		byName[peer.Name] = peer
	}

	alive := make([]string, 0, len(members))

	for _, name := range members {
		if _, ok := byName[name]; ok {
			alive = append(alive, name)
		}
	}

	slices.Sort(alive)

	// memberlist reports every member once, but a duplicate here would inflate
	// the alive count and fake a quorum.
	alive = slices.Compact(alive)

	return View{expected: byName, alive: alive}
}

// Alive returns the alive members, sorted, so callers and logs see a stable order.
func (v View) Alive() []string {
	return v.alive
}

func (v View) AliveCount() int {
	return len(v.alive)
}

func (v View) ExpectedCount() int {
	return len(v.expected)
}

// QuorumSize counts against the expected membership, never against the alive
// one: a quorum that shrinks with the failures would let both sides of a
// partition believe they still hold it.
func (v View) QuorumSize() int {
	return QuorumSize(len(v.expected))
}

func (v View) HasQuorum() bool {
	return len(v.alive) >= v.QuorumSize()
}

// IsAlive reports whether gossip currently sees the peer. Alive is sorted, so a
// large NodeGroup costs a binary search instead of a scan.
func (v View) IsAlive(name string) bool {
	_, alive := slices.BinarySearch(v.alive, name)

	return alive
}

func (v View) Peer(name string) (Peer, bool) {
	peer, ok := v.expected[name]

	return peer, ok
}

// Failed returns the expected peers gossip no longer sees, sorted by name and
// restricted to those already observed alive. A peer never seen alive may simply
// not have joined yet, which is a bootstrap state and not a failure.
func (v View) Failed(seen *SeenAlive) []Peer {
	failed := make([]Peer, 0, len(v.expected)-len(v.alive))

	for name, peer := range v.expected {
		if v.IsAlive(name) || !seen.Has(name) {
			continue
		}

		failed = append(failed, peer)
	}

	slices.SortFunc(failed, func(a, b Peer) int { return strings.Compare(a.Name, b.Name) })

	return failed
}

// SeenAlive remembers which peers this agent has observed alive at least once.
// The zero value is ready to use.
type SeenAlive struct {
	names map[string]struct{}
}

func (s *SeenAlive) Observe(names []string) {
	if s.names == nil {
		s.names = make(map[string]struct{}, len(names))
	}

	for _, name := range names {
		s.names[name] = struct{}{}
	}
}

func (s *SeenAlive) Has(name string) bool {
	_, ok := s.names[name]

	return ok
}

// Retain drops peers the NodeGroup no longer expects, so a Node recreated under
// a known name has to be seen alive again before it can be reported failed.
func (s *SeenAlive) Retain(expected []Peer) {
	keep := make(map[string]struct{}, len(expected))

	for _, peer := range expected {
		keep[peer.Name] = struct{}{}
	}

	for name := range s.names {
		if _, ok := keep[name]; !ok {
			delete(s.names, name)
		}
	}
}
