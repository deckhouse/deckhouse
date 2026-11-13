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
	"fmt"
	"math/rand/v2"
	"slices"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

// Helpers.

func peersOf(c *cluster, n int, sc script) []string {
	var names []string
	for i := range n {
		name := fmt.Sprintf("p%d", i)
		c.scripted(name, sc)
		names = append(names, name)
	}

	return names
}

func waitFor(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(monitorStep)
	}

	return cond()
}

func (n *node) stopped() bool { return !n.metrics.lastDecision(domain.DecisionStop).IsZero() }

// polls groups the questions of the node by poll ID, in order of the first send.
func polls(qs []sentMsg) [][]sentMsg {
	var ids []uint64
	byID := map[uint64][]sentMsg{}
	for _, m := range qs {
		id := m.msg.(domain.Question).PollID
		if _, ok := byID[id]; !ok {
			ids = append(ids, id)
		}
		byID[id] = append(byID[id], m)
	}
	out := make([][]sentMsg, 0, len(ids))
	for _, id := range ids {
		out = append(out, byID[id])
	}

	return out
}

// rounds groups the questions of one poll by the send time.
func rounds(poll []sentMsg) [][]sentMsg {
	var out [][]sentMsg
	for _, m := range poll {
		if len(out) == 0 || !out[len(out)-1][0].at.Equal(m.at) {
			out = append(out, nil)
		}
		out[len(out)-1] = append(out[len(out)-1], m)
	}

	return out
}

func targets(poll []sentMsg) []string {
	var names []string
	for _, m := range poll {
		if !slices.Contains(names, m.to) {
			names = append(names, m.to)
		}
	}

	return names
}

func ask(c *cluster, from, to string, lastReadyAgo time.Duration) {
	_ = c.net.send(from, to, domain.Question{PollID: 77, Network: domain.NetworkMain, To: to, LastReadyAgo: lastReadyAgo})
}

func lastAnswer(n *node) domain.Answer {
	got := n.received(isAnswer)
	if len(got) == 0 {
		return domain.Answer{}
	}

	return got[len(got)-1].msg.(domain.Answer)
}

// Rounds of the peer poll: all peers answered in the first round. The feed
// stops at the loss of the control plane, before the poll.
func TestPollAllAnswerInFirstRound(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, critical)
		x := c.agent("x", true)
		peersOf(c, 10, answering(domain.VerdictNotSee, 0))
		warmUp()
		x.cp.reach.Store(false)
		time.Sleep(critical.params.ControlPlaneLostAfter + 2*s)

		first := rounds(polls(c.questions("x"))[0])
		if len(first) < 2 || len(first[0]) != 3 {
			t.Fatalf("rounds %d, first round %d questions, want 3", len(first), len(first[0]))
		}
		if gap := first[1][0].at.Sub(first[0][0].at); gap != 2*latency {
			t.Errorf("second round after %v, want the round to close at the answers (%v), not at roundTimeout", gap, 2*latency)
		}
		if stop := x.metrics.lastDecision(domain.DecisionStop); stop.IsZero() || stop.After(first[0][0].at) {
			t.Errorf("stop at %v, want the stop before the first question at %v", stop, first[0][0].at)
		}
		c.noDeclareSuicide()
	})
}

// A round in full silence: the repeat of the question to silent peers.
func TestPollSilentRounds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, critical)
		x := c.agent("x", true)
		peersOf(c, 10, silent)
		warmUp()
		x.cp.reach.Store(false)
		time.Sleep(critical.params.ControlPlaneLostAfter + 2*s)

		rs := rounds(polls(c.questions("x"))[0])
		sizes := []int{}
		for _, r := range rs {
			sizes = append(sizes, len(r))
		}
		if !slices.Equal(sizes, []int{3, 7, 10}) {
			t.Fatalf("round sizes %v, want [3 7 10]: 3, then all the rest, then the repeat to all silent", sizes)
		}
		start := rs[0][0].at
		if d := rs[1][0].at.Sub(start); d != critical.params.RoundTimeout {
			t.Errorf("second round after %v, want roundTimeout", d)
		}
		if d := rs[2][0].at.Sub(start); d != 2*critical.params.RoundTimeout {
			t.Errorf("repeat after %v, want 2·roundTimeout", d)
		}
		if n := len(polls(c.questions("x"))); n != 1 {
			t.Errorf("polls %d, want 1", n)
		}
	})
}

// A question send error means the peer is silent.
func TestPollSendErrorIsSilence(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, critical)
		x := c.agent("x", true)
		peersOf(c, 4, answering(domain.VerdictNotSee, 0))
		c.setFail(func(from, to string) error {
			if from == "x" && to == "p0" {
				return syscall.ENETUNREACH
			}
			return nil
		})
		warmUp()
		x.cp.reach.Store(false)
		time.Sleep(critical.params.ControlPlaneLostAfter + 2*s)

		poll := polls(c.questions("x"))[0]
		var toP0 []sentMsg
		for _, m := range poll {
			if m.to == "p0" {
				toP0 = append(toP0, m)
			}
		}
		if len(toP0) != 2 {
			t.Fatalf("questions to p0 %d, want 2: the question and the repeat", len(toP0))
		}
		if d := toP0[1].at.Sub(poll[0].at); d >= critical.params.RoundTimeout {
			t.Errorf("repeat to p0 after %v, want the rounds to close early without waiting roundTimeout for the failed send", d)
		}
	})
}

// Random order of peers in the poll: a new poll has a new order.
func TestPollRandomOrder(t *testing.T) {
	var peers []domain.Peer
	for i := range 20 {
		peers = append(peers, domain.Peer{Name: fmt.Sprint(i)})
	}
	a, b := newPoll(peers), newPoll(peers)
	if slices.Equal(a.queue, b.queue) || slices.Equal(a.queue, peers) {
		t.Errorf("want a new random order for each poll")
	}
	if a.id == b.id {
		t.Errorf("want a new poll ID for each poll")
	}
}

// A long poll with answers: the feed stops at the loss of the control plane,
// not at the end of the poll.
func TestPollLongWithAnswers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, critical)
		x := c.agent("x", true)
		peersOf(c, 999, answering(domain.VerdictNotSee, 135*ms))
		warmUp()
		x.cp.reach.Store(false)
		time.Sleep(critical.params.ControlPlaneLostAfter + 5*s)

		poll := polls(c.questions("x"))[0]
		if n := len(rounds(poll)); n != 11 {
			t.Errorf("rounds %d, want 11: 3, then 10 rounds of max(3, ⌈0.1·1000⌉)", n)
		}
		if stop := x.wd.stopped(); stop.IsZero() || stop.Sub(poll[0].at) > monitorStep {
			t.Errorf("feed stopped at %v, want the stop at the start of the poll %v", stop, poll[0].at)
		}
	})
}

// A late answer to the current question counts.
func TestPollLateAnswerCounts(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, critical)
		x := c.agent("x", true)
		peersOf(c, 5, answering(domain.VerdictNotSee, 200*ms))
		warmUp()
		x.cp.reach.Store(false)
		time.Sleep(critical.params.ControlPlaneLostAfter + 3*s)
		x.metrics.mu.Lock()
		answers, silent := 0, 0
		for _, a := range x.metrics.answers {
			switch a.value {
			case string(domain.AnswerSupport):
				answers++
			case string(domain.AnswerSilent):
				silent++
			}
		}
		x.metrics.mu.Unlock()
		if answers != 5 || silent != 0 {
			t.Errorf("answers %d, silent %d; want 5 answers with the late ones and no silent peer", answers, silent)
		}
	})
}

// An answer counts only for its own question: a delayed answer to an old question is not a witness.
func TestPollStaleAnswerIgnored(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, critical)
		x := c.agent("x", true)
		c.scripted("old", func(q domain.Question) (domain.Answer, time.Duration, bool) {
			return domain.Answer{PollID: q.PollID - 1, Verdict: domain.VerdictSee}, 0, true
		})
		peersOf(c, 2, answering(domain.VerdictNotSee, 0))
		warmUp()
		x.cp.reach.Store(false)
		time.Sleep(critical.params.ControlPlaneLostAfter + 8*s)
		if !x.metrics.lastAnswer(domain.AnswerWitness).IsZero() {
			t.Errorf("the answer SEE to an old poll is a witness")
		}
		c.noDeclareSuicide()
	})
}

// The poll ends at the first answer SEE that can be a witness: the other peers
// of the round are not asked again, and no next round starts.
func TestPollEndsAtTheFirstWitness(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, critical)
		x := c.agent("x", true)
		peersOf(c, 10, seeing(3500*ms))
		warmUp()
		x.cp.reach.Store(false)
		time.Sleep(critical.params.ControlPlaneLostAfter + 3*s)

		if n := len(c.questions("x")); n != firstRound {
			t.Errorf("questions %d, want only the first round of %d", n, firstRound)
		}
		c.checkDeclareSuicide(x)
	})
}

// Answer classes: any node can be a witness. NOT SEE, unknown verdicts and
// DYING of an older agent are not a witness.
func TestAnswerClasses(t *testing.T) {
	unknownFields := func(q domain.Question) (domain.Answer, time.Duration, bool) {
		return domain.Answer{Verdict: domain.VerdictSee, UnknownFields: true}, 0, true
	}
	tests := []struct {
		name    string
		peer    string
		sc      script
		witness bool
	}{
		{name: "gap 0.8 s < witnessFresherBy is not a witness", peer: "w", sc: seeing(800 * ms)},
		{name: "gap 3.5 s > witnessFresherBy is a witness", peer: "w", sc: seeing(3500 * ms), witness: true},
		{name: "witness from a master with NoSchedule", peer: "master-0", sc: seeing(3500 * ms), witness: true},
		{name: "unknown verdict is not a witness", peer: "w", sc: answering(domain.Verdict(9), 0)},
		{name: "VerdictUnknown is not a witness", peer: "w", sc: answering(domain.VerdictUnknown, 0)},
		{name: "unknown field is not a witness", peer: "w", sc: unknownFields},
		{name: "NOT SEE is not a witness", peer: "w", sc: answering(domain.VerdictNotSee, 0)},
		{name: "DYING of an older agent is not a witness", peer: "w", sc: answering(domain.VerdictDying, 0)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				c := newCluster(t, critical)
				x := c.agent("x", true)
				c.scripted(tt.peer, tt.sc)
				warmUp()
				x.cp.reach.Store(false)
				time.Sleep(critical.params.ControlPlaneLostAfter + 8*s)

				if !x.stopped() {
					t.Fatal("no stop at the loss of the control plane")
				}
				witness := !x.metrics.lastAnswer(domain.AnswerWitness).IsZero()
				if witness != tt.witness {
					t.Fatalf("witness %v, want %v", witness, tt.witness)
				}
				if !tt.witness {
					c.noDeclareSuicide()
					return
				}
				// The freshness rule: two own probes fail after the answer, freshnessWait.
				answer := x.received(isAnswer)[0].at
				declared := c.declared("x")
				if len(declared) == 0 {
					t.Fatal("no DeclareSuicide")
				}
				if d := declared[0].at.Sub(answer); d < 100*ms || d > freshnessWait+100*ms {
					t.Errorf("DeclareSuicide %v after the answer, want ≈ freshnessWait", d)
				}
				if to := c.checkDeclareSuicide(x); to != tt.peer {
					t.Errorf("DeclareSuicide to %s, want the witness %s", to, tt.peer)
				}
			})
		})
	}
}

type manualCP chan domain.CPState

func (m manualCP) Updates() <-chan domain.CPState { return m }

// The control plane comes back before the watchdog fires: the armed agent
// feeds again.
func TestResumeWhenTheControlPlaneComesBack(t *testing.T) {
	for _, prof := range profiles {
		t.Run(prof.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				c := newCluster(t, prof)
				x := c.agent("x", true)
				peersOf(c, 3, answering(domain.VerdictNotSee, 0))
				warmUp()
				x.cp.reach.Store(false)
				if !waitFor(prof.params.ControlPlaneLostAfter+3*s, x.stopped) {
					t.Fatal("no stop")
				}
				x.cp.reach.Store(true)
				time.Sleep(prof.watchdogTimeout + s)
				if x.resumed() != 1 || !x.wd.fired().IsZero() || !x.engine.ShouldFeed() {
					t.Errorf("resumes %d, watchdog fired at %v, feed %v; want one resume and the feed", x.resumed(), x.wd.fired(), x.engine.ShouldFeed())
				}
				c.noDeclareSuicide()
			})
		})
	}
}

// The answer about self: SEE with the own lastReadyAgo while the control
// plane is visible, NOT SEE while it is lost. After DeclareSuicide the armed
// agent does not feed again. An agent without watchdog does not poll and
// feeds again when the control plane is visible.
func TestAnswerAboutSelf(t *testing.T) {
	for _, armed := range []bool{true, false} {
		t.Run(fmt.Sprintf("armed=%v", armed), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				c := newCluster(t, critical)
				x := c.agent("x", armed)
				c.scripted("w", seeing(3500*ms))
				a := c.scripted("a", silent)
				warmUp()

				ask(c, "a", "x", 5*s)
				time.Sleep(50 * ms)
				if got := lastAnswer(a); got.Verdict != domain.VerdictSee || got.LastReadyAgo > probeInterval+readInterval+latency || got.PollID != 77 || got.To != "a" {
					t.Fatalf("CP visible: answer %+v, want SEE with own lastReadyAgo", got)
				}

				x.cp.reach.Store(false)
				time.Sleep(critical.params.ControlPlaneLostAfter + 3*s)
				if !x.stopped() {
					t.Fatal("no stop at the loss of the control plane")
				}
				ask(c, "a", "x", 0)
				time.Sleep(50 * ms)
				if got := lastAnswer(a); got.Verdict != domain.VerdictNotSee || got.LastReadyAgo <= critical.params.ControlPlaneLostAfter {
					t.Errorf("after the stop: answer %+v, want NOT SEE with own lastReadyAgo", got)
				}

				x.cp.reach.Store(true)
				time.Sleep(2 * s)
				ask(c, "a", "x", 0)
				time.Sleep(50 * ms)
				if got := lastAnswer(a); got.Verdict != domain.VerdictSee {
					t.Errorf("CP back: answer %v, want SEE", got.Verdict)
				}
				if armed {
					c.checkDeclareSuicide(x)
					if x.engine.ShouldFeed() || x.wd.wasResumed() || x.resumed() != 0 {
						t.Errorf("CP back after DeclareSuicide: feed %v; want no feed", x.engine.ShouldFeed())
					}
					return
				}
				if !x.engine.ShouldFeed() || len(c.questions("x")) != 0 {
					t.Errorf("CP back at an agent without watchdog: feed %v, questions %d; want the feed and no poll", x.engine.ShouldFeed(), len(c.questions("x")))
				}
				c.noDeclareSuicide()
			})
		})
	}
}

// Agent without watchdog: the stop at the loss of the control plane has no
// effect, the agent does not poll and feeds again when the control plane is
// visible.
func TestUnarmedStopDoesNotAct(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, critical)
		x := c.agent("x", false)
		peersOf(c, 3, silent)
		warmUp()
		x.cp.reach.Store(false)
		time.Sleep(critical.params.ControlPlaneLostAfter + 2*s)
		if !x.stopped() {
			t.Fatal("no stop decision")
		}
		if n := len(c.questions("x")); n != 0 {
			t.Errorf("an agent without watchdog asked %d questions, want none", n)
		}
		time.Sleep(5 * s)
		x.cp.reach.Store(true)
		time.Sleep(s)
		if !x.engine.ShouldFeed() || x.metrics.lastDecision(domain.DecisionFeed).IsZero() {
			t.Errorf("an agent without watchdog does not return to the normal work")
		}
	})
}

// Watchdog feeding only on a fresh decision: the decision is stale.
func TestStaleDecision(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, critical)
		x := c.agent("x", true)
		warmUp()
		x.cp.paused.Store(true)
		paused := time.Now()
		time.Sleep(time.Second)
		d := x.wd.stopped().Sub(paused)
		if d <= 0 || d > decisionTTL+monitorStep {
			t.Errorf("feed stopped %v after the last read, want ≤ two reads", d)
		}
		if x.stopped() {
			t.Errorf("a stale decision is not a stop decision")
		}
	})
}

// A new decision before the first feeding after an agent restart.
func TestFirstFeedAfterNewDecision(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, critical)
		cps := make(chan domain.CPState)
		x := c.add("x")
		e := New(Deps{Transport: x, ControlPlane: manualCP(cps), StopFeed: func() bool { return true }, Metrics: &metrics{}}, log.NewNop())
		e.SetParams(critical.params)
		go func() { _ = e.Run(c.ctx) }()
		time.Sleep(time.Second)
		if e.ShouldFeed() {
			t.Fatal("feed before the first decision")
		}
		cps <- domain.CPState{Source: domain.CPSourceProxy, Visible: true, Seen: true, ReadAt: time.Now()}
		synctest.Wait()
		if !e.ShouldFeed() {
			t.Fatal("no feed after the first decision")
		}
	})
}

// Before the first read of the control plane the agent does not answer the
// peers. Its lastReadyAgo is 0 then: an answer SEE would make it a false witness.
func TestNoAnswerBeforeTheFirstRead(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, critical)
		a := c.scripted("a", silent)
		cps := make(chan domain.CPState)
		x := c.add("x")
		e := New(Deps{Transport: x, ControlPlane: manualCP(cps), StopFeed: func() bool { return false }, Metrics: &metrics{}}, log.NewNop())
		e.SetParams(critical.params)
		go func() { _ = e.Run(c.ctx) }()

		ask(c, "a", "x", critical.params.ControlPlaneLostAfter)
		time.Sleep(time.Second)
		if got := a.received(isAnswer); len(got) != 0 {
			t.Fatalf("answers before the first read: %v, want none", got)
		}

		cps <- domain.CPState{Source: domain.CPSourceProxy, Visible: true, Seen: true, ReadAt: time.Now()}
		ask(c, "a", "x", critical.params.ControlPlaneLostAfter)
		time.Sleep(50 * ms)
		if got := a.received(isAnswer); len(got) != 1 || lastAnswer(a).Verdict != domain.VerdictSee {
			t.Fatalf("answers after the first read: %v, want one answer SEE", got)
		}
	})
}

// CP visibility by the control plane visibility rule: before the first ready answer
// after the start the agent has no data about the control plane. That is not
// a loss: the agent does not poll and does not feed. It answers NOT SEE.
func TestNoDataIsNotLoss(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, critical)
		peersOf(c, 3, answering(domain.VerdictSee, 0))
		a := c.scripted("a", silent)
		cps := make(chan domain.CPState)
		x := c.add("x")
		x.metrics = &metrics{}
		e := New(Deps{Transport: x, ControlPlane: manualCP(cps), StopFeed: func() bool { return true }, Metrics: x.metrics}, log.NewNop())
		e.SetParams(critical.params)
		go func() { _ = e.Run(c.ctx) }()

		for range 5 {
			cps <- domain.CPState{Source: domain.CPSourceSelf, LastReadyAgo: 80 * ms, ReadAt: time.Now()}
			time.Sleep(readInterval)
		}

		if len(c.questions("x")) != 0 || x.metrics.pollCount() != 0 || x.stopped() {
			t.Fatalf("poll without data about the control plane: %d questions", len(c.questions("x")))
		}
		if e.ShouldFeed() {
			t.Fatal("feed without data about the control plane")
		}

		ask(c, "a", "x", time.Second)
		time.Sleep(50 * ms)
		if got := lastAnswer(a); got.Verdict != domain.VerdictNotSee {
			t.Fatalf("answer without data about the control plane is %v, want NOT SEE", got.Verdict)
		}

		// With data the same state is a loss.
		for range 2 {
			cps <- domain.CPState{Source: domain.CPSourceSelf, Seen: true, LastReadyAgo: critical.params.ControlPlaneLostAfter + s, ReadAt: time.Now()}
		}
		synctest.Wait()
		if len(c.questions("x")) == 0 {
			t.Fatal("no poll after the loss of the control plane")
		}
	})
}

// No parameters — no decision.
func TestNoParamsNoFeed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, critical)
		cps := make(chan domain.CPState)
		x := c.add("x")
		e := New(Deps{Transport: x, ControlPlane: manualCP(cps), StopFeed: func() bool { return true }, Metrics: &metrics{}}, log.NewNop())
		go func() { _ = e.Run(c.ctx) }()
		cps <- domain.CPState{Source: domain.CPSourceProxy, Visible: true, Seen: true, ReadAt: time.Now()}
		synctest.Wait()
		if e.ShouldFeed() {
			t.Fatal("feed without parameters")
		}
	})
}

type inbox chan domain.Message

func (inbox) Peers() []domain.Peer                   { return nil }
func (inbox) Peer(string) (domain.Peer, bool)        { return domain.Peer{}, false }
func (inbox) Send(domain.Peer, domain.Message) error { return nil }
func (i inbox) Received() <-chan domain.Message      { return i }

// Receipt of 'leaving': the engine gives the name of the leaving peer to Deps.Leaving.
func TestLeavingGoesToTheCallback(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		msgs := make(inbox)
		var left []string
		e := New(Deps{
			Transport: msgs, ControlPlane: manualCP(nil), StopFeed: func() bool { return true },
			Metrics: &metrics{}, Leaving: func(node string) { left = append(left, node) },
		}, log.NewNop())
		go func() { _ = e.Run(t.Context()) }()

		msgs <- domain.Leaving{Header: domain.Header{From: "worker-2"}}
		msgs <- domain.Leaving{Header: domain.Header{From: "worker-3"}}
		synctest.Wait()

		if !slices.Equal(left, []string{"worker-2", "worker-3"}) {
			t.Fatalf("left = %v, want [worker-2 worker-3]", left)
		}
	})
}

// Receipt of 'created': the engine gives the node of the message to Deps.Created.
func TestCreatedGoesToTheCallback(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		msgs := make(inbox)
		var created []string
		e := New(Deps{
			Transport: msgs, ControlPlane: manualCP(nil), StopFeed: func() bool { return true },
			Metrics: &metrics{}, Created: func(node string) { created = append(created, node) },
		}, log.NewNop())
		go func() { _ = e.Run(t.Context()) }()

		msgs <- domain.FFNSCreated{Header: domain.Header{From: "worker-2"}, Node: "worker-5"}
		synctest.Wait()

		if !slices.Equal(created, []string{"worker-5"}) {
			t.Fatalf("created = %v, want [worker-5]", created)
		}
	})
}

// The witness accepts DeclareSuicide only for its own answer SEE to the same
// node with the same poll ID, not older than controlPlaneLostAfter.
func TestDeclareSuicideAcceptance(t *testing.T) {
	declare := func(c *cluster, from, to string, pollID uint64) {
		_ = c.net.send(from, to, domain.DeclareSuicide{PollID: pollID})
	}
	tests := []struct {
		name string
		prof profile
		// lost: the witness does not see the control plane and answers NOT SEE.
		lost   bool
		from   string
		pollID uint64
		after  time.Duration
		want   []string
	}{
		{name: "critical, 1 s after the answer", prof: critical, from: "v", pollID: 77, after: s, want: []string{"v"}},
		{name: "normal, 12 s after the answer", prof: normal, from: "v", pollID: 77, after: 12 * s},
		{name: "another poll ID", prof: critical, from: "v", pollID: 78, after: s},
		{name: "forged: no question of the node", prof: critical, from: "u", pollID: 77, after: s},
		{name: "answer NOT SEE", prof: critical, lost: true, from: "v", pollID: 77, after: s},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				c := newCluster(t, tt.prof)
				w := c.agent("w", true)
				v := c.scripted("v", silent)
				c.scripted("u", silent)
				peersOf(c, 3, answering(domain.VerdictNotSee, 0))
				warmUp()
				if tt.lost {
					w.cp.reach.Store(false)
					time.Sleep(tt.prof.params.ControlPlaneLostAfter + s)
				}

				ask(c, "v", "w", 10*s)
				time.Sleep(50 * ms)
				want := domain.VerdictSee
				if tt.lost {
					want = domain.VerdictNotSee
				}
				if got := lastAnswer(v); got.Verdict != want {
					t.Fatalf("answer %v, want %v", got.Verdict, want)
				}

				time.Sleep(tt.after)
				declare(c, tt.from, "w", tt.pollID)
				time.Sleep(50 * ms)
				if got := w.attestedNodes(); !slices.Equal(got, tt.want) {
					t.Fatalf("attested %v, want %v", got, tt.want)
				}
			})
		})
	}
}

// The engine gets random control plane states, also a start without the
// control plane. After each state ShouldFeed is true only when the control
// plane was visible after the start. So the watchdog needs no own condition
// "the control plane was visible after the start" to arm.
func TestNoFeedBeforeTheControlPlaneIsVisible(t *testing.T) {
	check := func(t *testing.T, seed uint64, start []bool) {
		r := rand.New(rand.NewPCG(seed, 1))
		synctest.Test(t, func(t *testing.T) {
			c := newCluster(t, critical)
			peersOf(c, 3, answering(domain.VerdictNotSee, 0))
			cps := make(chan domain.CPState)
			x := c.add("x")
			e := New(Deps{Transport: x, ControlPlane: manualCP(cps), StopFeed: func() bool { return true },
				ResumeFeed: func() bool { return true }, Finalize: func() {}, Metrics: &metrics{}}, log.NewNop())
			e.SetParams(critical.params)
			go func() { _ = e.Run(c.ctx) }()

			seen := false
			send := func(visible bool) {
				seen = seen || visible
				cps <- domain.CPState{Source: domain.CPSourceProxy, Seen: true, Visible: visible, LastReadyAgo: 5 * s, ReadAt: time.Now()}
				for range 4 {
					time.Sleep(50 * ms)
					if e.ShouldFeed() && !seen {
						t.Fatalf("seed %d: feed before the control plane was visible", seed)
					}
				}
			}
			for _, v := range start {
				send(v)
			}
			for range 200 {
				send(r.IntN(3) == 0)
			}
		})
	}
	check(t, 0, make([]bool, 50))
	for seed := uint64(1); seed <= 20; seed++ {
		check(t, seed, nil)
	}
}
