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
	"fmt"
	"math/rand"
	"slices"
	"testing"
)

func designatedWriter(alive []string, failedNode string) string {
	for _, candidate := range alive {
		if WriterRank(alive, failedNode, candidate) == 0 {
			return candidate
		}
	}

	return ""
}

func nodeNames(count int) []string {
	names := make([]string, 0, count)

	for i := range count {
		names = append(names, fmt.Sprintf("worker-%d", i))
	}

	return names
}

func TestDesignatedWriterIgnoresInputOrder(t *testing.T) {
	alive := nodeNames(20)
	want := designatedWriter(alive, "worker-99")

	if want == "" {
		t.Fatal("no writer elected from a non-empty alive set")
	}

	shuffled := slices.Clone(alive)

	for range 50 {
		rand.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })

		if got := designatedWriter(shuffled, "worker-99"); got != want {
			t.Fatalf("writer changed with the order of the alive set: got %q, want %q", got, want)
		}
	}
}

func TestDesignatedWriterAgreesAcrossAgents(t *testing.T) {
	// Every agent runs the election over the same alive set and must reach the
	// same answer without talking to anyone.
	alive := nodeNames(7)

	for _, failed := range []string{"worker-0", "worker-3", "gone-node"} {
		want := designatedWriter(alive, failed)

		for range 10 {
			if got := designatedWriter(alive, failed); got != want {
				t.Fatalf("election for %q is not stable: got %q, want %q", failed, got, want)
			}
		}
	}
}

func TestDesignatedWriterExcludesTheFailedPeer(t *testing.T) {
	alive := nodeNames(5)

	for _, failed := range alive {
		if got := designatedWriter(alive, failed); got == failed {
			t.Errorf("peer %q was elected to report itself", failed)
		}
	}
}

func TestDesignatedWriterWithoutCandidates(t *testing.T) {
	if got := designatedWriter(nil, "worker-1"); got != "" {
		t.Errorf("designatedWriter(nil) = %q, want an empty name", got)
	}

	if got := designatedWriter([]string{"worker-1"}, "worker-1"); got != "" {
		t.Errorf("the only candidate is the failed peer, got %q, want an empty name", got)
	}
}

func TestDesignatedWriterSurvivesLosingOtherPeers(t *testing.T) {
	// Losing a peer that is not the writer must not move the role: the minimum
	// of a set does not change when a non-minimum element is removed. Otherwise
	// every unrelated failure would hand the incident to a different agent.
	alive := nodeNames(9)
	failed := "worker-99"
	writer := designatedWriter(alive, failed)

	for _, lost := range alive {
		if lost == writer {
			continue
		}

		remaining := slices.DeleteFunc(slices.Clone(alive), func(name string) bool { return name == lost })

		if got := designatedWriter(remaining, failed); got != writer {
			t.Errorf("losing %q moved the writer from %q to %q", lost, writer, got)
		}
	}
}

func TestDesignatedWriterSpreadsTheLoad(t *testing.T) {
	// Quorum guarantees there are always more writers than failed peers, so the
	// hash has to keep the per-agent share near one for the write burst of a
	// mass failure to stay flat. Sequential names are the hard case: they share
	// almost every byte, and a weakly mixed hash then elects the same few agents
	// for every incident.
	tests := []struct {
		alive, failed, maxShare int
	}{
		{alive: 5, failed: 4, maxShare: 3},
		{alive: 51, failed: 49, maxShare: 8},
		{alive: 201, failed: 199, maxShare: 8},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d alive %d failed", tt.alive, tt.failed), func(t *testing.T) {
			alive := nodeNames(tt.alive)
			load := make(map[string]int)

			for i := range tt.failed {
				load[designatedWriter(alive, fmt.Sprintf("dead-%d", i))]++
			}

			worst := 0

			for _, count := range load {
				worst = max(worst, count)
			}

			if worst > tt.maxShare {
				t.Errorf("one agent took %d of %d incidents (limit %d), the election is not spreading them",
					worst, tt.failed, tt.maxShare)
			}
		})
	}
}

func TestWriterScoreSeparatesTheOperands(t *testing.T) {
	// Without a separator "worker-1" + "0-a" and "worker-10" + "-a" would hash
	// the same bytes.
	if writerScore("worker-1", "0-a") == writerScore("worker-10", "-a") {
		t.Error("scores of two different pairs collide, the separator is not doing its job")
	}
}

func TestPreferredBreaksTiesOnName(t *testing.T) {
	tests := []struct {
		name           string
		candidate      string
		candidateScore uint64
		current        string
		currentScore   uint64
		want           bool
	}{
		{name: "no writer yet", candidate: "worker-2", candidateScore: 9, current: "", want: true},
		{name: "lower score wins", candidate: "worker-2", candidateScore: 1, current: "worker-1", currentScore: 2, want: true},
		{name: "higher score loses", candidate: "worker-2", candidateScore: 3, current: "worker-1", currentScore: 2, want: false},
		{name: "tie goes to the smaller name", candidate: "worker-1", candidateScore: 2, current: "worker-2", currentScore: 2, want: true},
		{name: "tie keeps the smaller name", candidate: "worker-3", candidateScore: 2, current: "worker-2", currentScore: 2, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := preferred(tt.candidate, tt.candidateScore, tt.current, tt.currentScore); got != tt.want {
				t.Errorf("preferred() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDesignatedWriterAlwaysPicksALiveCandidate(t *testing.T) {
	// The role has to land on an agent that can actually write. Electing from
	// the expected membership instead of the alive set would hand incidents to
	// nodes that are themselves gone.
	alive := nodeNames(12)

	for _, failed := range append(nodeNames(30), "unknown-node") {
		writer := designatedWriter(alive, failed)

		if failed != writer && !slices.Contains(alive, writer) {
			t.Errorf("DesignatedWriter for %q returned %q, which is not in the alive set", failed, writer)
		}
	}
}

func TestWriterRankOrdersEveryCandidateOnce(t *testing.T) {
	alive := nodeNames(9)
	failed := "worker-99"

	ranks := make(map[int]string, len(alive))

	for _, candidate := range alive {
		rank := WriterRank(alive, failed, candidate)

		if rank < 0 || rank >= len(alive) {
			t.Fatalf("rank of %q is %d, want a place in a set of %d candidates", candidate, rank, len(alive))
		}

		if taken, ok := ranks[rank]; ok {
			t.Errorf("%q and %q share rank %d, so both would take over at the same moment", taken, candidate, rank)
		}

		ranks[rank] = candidate
	}

	if got := ranks[0]; got != designatedWriter(alive, failed) {
		t.Errorf("rank zero is %q, want the designated writer %q", got, designatedWriter(alive, failed))
	}
}

func TestWriterRankRejectsNonCandidates(t *testing.T) {
	alive := nodeNames(5)

	if got := WriterRank(alive, "worker-1", "worker-1"); got != -1 {
		t.Errorf("rank of the failed peer itself = %d, want -1", got)
	}

	if got := WriterRank(alive, "worker-1", "worker-42"); got != -1 {
		t.Errorf("rank of a node outside the alive set = %d, want -1", got)
	}
}
