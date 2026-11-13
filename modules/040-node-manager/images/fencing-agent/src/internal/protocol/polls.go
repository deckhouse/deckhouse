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

package protocol

import (
	"sync"

	"fencing-agent/internal/domain"
)

// polls keeps the questions of the current poll. A question with a new pollID
// starts a new poll and removes the questions of the previous poll.
type polls struct {
	mu     sync.Mutex
	pollID uint64
	asked  map[asked]struct{}
}

type asked struct {
	network domain.Network
	epoch   uint64
	// to is the name that the asker expects at the address.
	to string
}

func (p *polls) ask(q domain.Question) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.asked == nil || q.PollID != p.pollID {
		p.pollID = q.PollID
		p.asked = make(map[asked]struct{})
	}
	p.asked[asked{network: q.Network, epoch: q.Epoch, to: q.To}] = struct{}{}
}

// match tells whether a is the answer to a question of the current poll:
// the pollID, the epoch and the network are those of the question, and the
// peer that answers has the name that the question expects.
func (p *polls) match(a domain.Answer) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	_, ok := p.asked[asked{network: a.Network, epoch: a.Epoch, to: a.From}]

	return ok && a.PollID == p.pollID
}

// reset removes all questions. Answers to them do not count.
func (p *polls) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.pollID = 0
	p.asked = nil
}
