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
	"hash/fnv"
	"slices"
)

// scoreSeparator keeps the hashed pair unambiguous: without it ("worker-1",
// "0-a") and ("worker-10", "-a") would hash the very same bytes and the two
// failed peers would share a score.
const scoreSeparator = "\x00"

// DesignatedWriter names the single agent responsible for reporting failedNode
// to Kubernetes. Every agent runs this over its own alive set and reaches the
// same answer while the sets agree, so the role needs no election protocol and
// has no leader to lose. It returns an empty string when no candidate is left.
//
// While views still differ two agents may both act; that costs one rejected
// create, never a second incident record.
func DesignatedWriter(alive []string, failedNode string) string {
	var (
		writer string
		best   uint64
	)

	for _, candidate := range alive {
		// A failed peer cannot report itself. It is normally absent from the
		// alive set already; this only makes that independent of the caller.
		if candidate == failedNode {
			continue
		}

		if score := writerScore(candidate, failedNode); preferred(candidate, score, writer, best) {
			writer, best = candidate, score
		}
	}

	return writer
}

// preferred reports whether a candidate outranks the current best. Ties break on
// the name, so even a hash collision elects one writer instead of two.
func preferred(candidate string, candidateScore uint64, current string, currentScore uint64) bool {
	switch {
	case current == "":
		return true
	case candidateScore != currentScore:
		return candidateScore < currentScore
	default:
		return candidate < current
	}
}

// WriterRank places a candidate in the order the writer role falls through:
// 0 is the designated writer, 1 the agent that takes over if the record never
// appears, and so on. A node that is not a candidate gets -1.
//
// The role being a pure function of the alive set is what removes the leader
// election, but it also means an elected agent that cannot reach Kubernetes — or
// one that restarted and so never saw the peer alive — would hold every incident
// it is elected for while all the others defer to it. The rank is how they stop
// deferring, still without talking to each other.
func WriterRank(alive []string, failedNode, candidate string) int {
	if candidate == failedNode || !slices.Contains(alive, candidate) {
		return -1
	}

	score := writerScore(candidate, failedNode)
	rank := 0

	for _, other := range alive {
		if other == failedNode || other == candidate {
			continue
		}

		if preferred(other, writerScore(other, failedNode), candidate, score) {
			rank++
		}
	}

	return rank
}

func writerScore(node, failedNode string) uint64 {
	sum := fnv.New64a()

	// hash.Hash promises Write never returns an error.
	_, _ = sum.Write([]byte(node + scoreSeparator + failedNode))

	return avalanche(sum.Sum64())
}

// avalanche is the MurmurHash3 finalizer. FNV-1a alone barely mixes its high
// bits, and those are what an unsigned comparison decides on: with the failed
// name as a common suffix the candidate order came out nearly the same for every
// incident, so the role stuck to a handful of agents. Measured over 501 alive
// and 499 failed peers named worker-N, the worst agent took 100 incidents
// without this step and 5 with it, against ~4 for an ideal hash.
func avalanche(sum uint64) uint64 {
	sum ^= sum >> 33
	sum *= 0xff51afd7ed558ccd
	sum ^= sum >> 33
	sum *= 0xc4ceb9fe1a85ec53
	sum ^= sum >> 33

	return sum
}
