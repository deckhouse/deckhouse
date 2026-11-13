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

package decision

import (
	"math/rand/v2"
	"slices"
	"time"

	"fencing-agent/internal/domain"
)

// firstRound is the number of peers in the first round of a full poll.
const firstRound = 3

type question struct {
	peer domain.Peer
	// ownLastReadyAgo is the own lastReadyAgo at the first send of the question.
	ownLastReadyAgo time.Duration
	answered        bool
	// failed is true when the last send returned an error.
	failed bool
}

// poll is one poll of all peers in rounds with one poll ID. The order of the
// peers is random.
type poll struct {
	id uint64
	// nodeCount is the number of nodes in the static list with the own node.
	nodeCount int
	// queue holds the peers that are not asked yet.
	queue []domain.Peer
	asked map[string]*question
	// round holds the names of the peers of the current round or of the repeat.
	round []string
	// answered is true when a peer of the current round answered.
	answered bool
	repeated bool
}

func newPoll(peers []domain.Peer) *poll {
	p := &poll{
		id:        rand.Uint64(),
		nodeCount: len(peers) + 1,
		queue:     slices.Clone(peers),
		asked:     make(map[string]*question, len(peers)),
	}
	rand.Shuffle(len(p.queue), func(i, j int) { p.queue[i], p.queue[j] = p.queue[j], p.queue[i] })

	return p
}

// nextRoundSize is max(3, ⌈0.1·nodeCount⌉), or all remaining peers after a round without answers.
func (p *poll) nextRoundSize() int {
	if !p.answered {
		return len(p.queue)
	}

	return max(firstRound, (p.nodeCount+9)/10)
}

// roundClosed tells that each question of the current round has an answer or failed to send.
func (p *poll) roundClosed() bool {
	for _, name := range p.round {
		if q := p.asked[name]; !q.answered && !q.failed {
			return false
		}
	}

	return true
}

func (p *poll) silent() []domain.Peer {
	var peers []domain.Peer
	for _, q := range p.asked {
		if !q.answered {
			peers = append(peers, q.peer)
		}
	}

	return peers
}
