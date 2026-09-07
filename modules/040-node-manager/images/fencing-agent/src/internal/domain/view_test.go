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
	"testing"
)

func peers(names ...string) []Peer {
	out := make([]Peer, 0, len(names))

	for _, name := range names {
		out = append(out, Peer{Name: name, IP: "10.0.0.1", UID: "uid-" + name})
	}

	return out
}

func seenAlive(names ...string) *SeenAlive {
	seen := &SeenAlive{}
	seen.Observe(names)

	return seen
}

func failedNames(t *testing.T, view View, seen *SeenAlive) []string {
	t.Helper()

	names := make([]string, 0)

	for _, peer := range view.Failed(seen) {
		if peer.UID != "uid-"+peer.Name {
			t.Errorf("peer %q lost its UID on the way through the view: %q", peer.Name, peer.UID)
		}

		names = append(names, peer.Name)
	}

	return names
}

func TestNewViewKeepsOnlyExpectedMembers(t *testing.T) {
	// A member of another NodeGroup, or one whose Node was already removed, must
	// not count towards quorum; a Node that never joined gossip is not alive.
	view := NewView(peers("worker-1", "worker-2", "worker-3"), []string{"worker-2", "worker-1", "stranger"})

	if got := view.Alive(); !slices.Equal(got, []string{"worker-1", "worker-2"}) {
		t.Errorf("Alive() = %v, want the sorted intersection [worker-1 worker-2]", got)
	}

	if got, want := view.ExpectedCount(), 3; got != want {
		t.Errorf("ExpectedCount() = %d, want %d", got, want)
	}
}

func TestNewViewIgnoresDuplicateMembers(t *testing.T) {
	view := NewView(peers("worker-1", "worker-2", "worker-3"), []string{"worker-1", "worker-1"})

	if got, want := view.AliveCount(), 1; got != want {
		t.Errorf("AliveCount() = %d, want %d: a duplicate must not inflate the count", got, want)
	}
}

func TestViewQuorum(t *testing.T) {
	tests := []struct {
		expected, alive int
		wantQuorum      int
		wantHas         bool
	}{
		{expected: 1, alive: 1, wantQuorum: 1, wantHas: true},
		{expected: 2, alive: 1, wantQuorum: 2, wantHas: false},
		{expected: 2, alive: 2, wantQuorum: 2, wantHas: true},
		{expected: 3, alive: 1, wantQuorum: 2, wantHas: false},
		{expected: 3, alive: 2, wantQuorum: 2, wantHas: true},
		{expected: 5, alive: 2, wantQuorum: 3, wantHas: false},
		{expected: 5, alive: 3, wantQuorum: 3, wantHas: true},
		{expected: 1000, alive: 500, wantQuorum: 501, wantHas: false},
		{expected: 1000, alive: 501, wantQuorum: 501, wantHas: true},
	}

	for _, tt := range tests {
		expected := peers(nodeNames(tt.expected)...)
		view := NewView(expected, nodeNames(tt.alive))

		if got := view.QuorumSize(); got != tt.wantQuorum {
			t.Errorf("QuorumSize() with %d expected = %d, want %d", tt.expected, got, tt.wantQuorum)
		}

		if got := view.HasQuorum(); got != tt.wantHas {
			t.Errorf("HasQuorum() with %d of %d alive = %v, want %v", tt.alive, tt.expected, got, tt.wantHas)
		}
	}
}

func TestViewFailedNeedsThePeerSeenAliveFirst(t *testing.T) {
	expected := peers("worker-1", "worker-2", "worker-3")
	view := NewView(expected, []string{"worker-1"})

	// Right after start the local gossip view is still filling in, so a peer
	// never observed alive is unknown, not failed.
	if got := failedNames(t, view, seenAlive()); len(got) != 0 {
		t.Errorf("Failed() = %v, want none: no peer has been seen alive yet", got)
	}

	if got := failedNames(t, view, seenAlive("worker-3", "worker-2")); !slices.Equal(got, []string{"worker-2", "worker-3"}) {
		t.Errorf("Failed() = %v, want [worker-2 worker-3] sorted by name", got)
	}

	if got := failedNames(t, view, seenAlive("worker-2")); !slices.Equal(got, []string{"worker-2"}) {
		t.Errorf("Failed() = %v, want only the peer that was seen alive", got)
	}
}

func TestViewFailedExcludesAliveMembers(t *testing.T) {
	expected := peers("worker-1", "worker-2")
	view := NewView(expected, []string{"worker-1", "worker-2"})

	if got := failedNames(t, view, seenAlive("worker-1", "worker-2")); len(got) != 0 {
		t.Errorf("Failed() = %v, want none while every peer is alive", got)
	}
}

func TestViewPeerLookup(t *testing.T) {
	view := NewView(peers("worker-1"), nil)

	peer, ok := view.Peer("worker-1")
	if !ok || peer.UID != "uid-worker-1" {
		t.Errorf("Peer(worker-1) = (%+v, %v), want the expected peer with its UID", peer, ok)
	}

	if _, ok := view.Peer("worker-9"); ok {
		t.Error("Peer(worker-9) reported a peer the NodeGroup does not expect")
	}
}

func TestSeenAliveZeroValueIsUsable(t *testing.T) {
	var seen SeenAlive

	if seen.Has("worker-1") {
		t.Error("an empty SeenAlive reported a peer as seen")
	}

	seen.Observe([]string{"worker-1"})

	if !seen.Has("worker-1") {
		t.Error("Observe did not record the peer")
	}
}

func TestSeenAliveRetainForgetsRemovedNodes(t *testing.T) {
	// A Node recreated under a known name has to be seen alive again before it
	// can be reported failed, so scale-down must clear the memory.
	seen := seenAlive("worker-1", "worker-2")
	seen.Retain(peers("worker-1"))

	if !seen.Has("worker-1") {
		t.Error("Retain dropped a peer the NodeGroup still expects")
	}

	if seen.Has("worker-2") {
		t.Error("Retain kept a peer the NodeGroup no longer expects")
	}
}
