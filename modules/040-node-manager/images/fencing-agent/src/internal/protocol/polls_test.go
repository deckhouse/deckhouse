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
	"testing"

	"fencing-agent/internal/domain"
)

func question(pollID uint64, to string) domain.Question {
	return domain.Question{PollID: pollID, Network: domain.NetworkMain, Epoch: 3, To: to}
}

func answer(pollID uint64, from string) domain.Answer {
	return domain.Answer{
		Header: domain.Header{From: from}, To: "node-a", PollID: pollID,
		Epoch: 3, Network: domain.NetworkMain, Verdict: domain.VerdictSee,
	}
}

// Matching an answer to a question.
func TestPollsMatchQuestion(t *testing.T) {
	var p polls
	p.ask(question(10, "node-b"))

	if !p.match(answer(10, "node-b")) {
		t.Error("the answer to the question does not count")
	}

	// Wrong node answered: node-c answers at the address of node-b.
	if p.match(answer(10, "node-c")) {
		t.Error("the answer of node-c counts for the question to node-b")
	}

	otherEpoch := answer(10, "node-b")
	otherEpoch.Epoch = 2
	if p.match(otherEpoch) {
		t.Error("the answer with another epoch counts")
	}

	otherNetwork := answer(10, "node-b")
	otherNetwork.Network = domain.NetworkSystem
	if p.match(otherNetwork) {
		t.Error("the answer from another network counts")
	}
}

// Answers of past rounds and late answers.
func TestPollsPreviousAndLateAnswers(t *testing.T) {
	var p polls

	// A delayed answer is not a witness.
	p.ask(question(10, "node-b"))
	p.ask(question(11, "node-b"))
	if p.match(answer(10, "node-b")) {
		t.Error("the answer to the previous poll counts")
	}
	if !p.match(answer(11, "node-b")) {
		t.Error("the answer to the current poll does not count")
	}

	// A late answer within the round: node-b answers after the
	// next round of the same poll started.
	p.ask(question(12, "node-b"))
	p.ask(question(12, "node-c"))
	p.ask(question(12, "node-d"))
	if !p.match(answer(12, "node-b")) {
		t.Error("the late answer to the current poll does not count")
	}

	// A new memberlist instance: the answers to the questions of the
	// previous instance do not count.
	p.reset()
	if p.match(answer(12, "node-b")) {
		t.Error("the answer to a question of the previous epoch counts")
	}
}
