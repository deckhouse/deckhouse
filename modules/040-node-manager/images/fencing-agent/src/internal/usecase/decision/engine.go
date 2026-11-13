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
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
	"fencing-agent/internal/usecase/cpview"
)

// bootMillis is the clock of the probe cycle numbers. Tests replace it.
var bootMillis = cpview.BootMillis

const (
	// readInterval is the slow interval of the control plane reads.
	readInterval = 200 * time.Millisecond
	// decisionTTL: a decision older than two reads does not let the agent feed.
	decisionTTL = 2 * readInterval
)

// witness is an answer SEE that is fresher by more than witnessFresherBy. It
// waits for the freshness rule.
type witness struct {
	peer string
	// pollID is the poll of the answer.
	pollID uint64
	fresh  *cpview.Freshness
}

// seeAnswer is an answer SEE of the agent to the question of node.
type seeAnswer struct {
	node   string
	pollID uint64
}

// Engine decides whether the agent may feed its watchdog and answers the questions of the peers.
//
// The agent stops the feed at once when the control plane is lost. An agent
// with the armed watchdog then polls the peers for a witness: a peer that sees
// the control plane. It sends DeclareSuicide to the witness. The stop is
// reversible: when the control plane is visible again before the watchdog
// fires, the agent feeds again. The stop is final after DeclareSuicide and
// when the watchdog does not resume the feed.
type Engine struct {
	deps   Deps
	logger *log.Logger
	params atomic.Pointer[Params]

	feedMu sync.Mutex
	// feedUntil is the end of the last positive decision.
	feedUntil time.Time

	// decided is closed after the first decision.
	decided    chan struct{}
	decideOnce sync.Once

	// Only the Run goroutine uses the fields below.
	p     Params
	cp    domain.CPState
	hasCP bool
	// lost is true while the control plane is not visible. The two reads in a
	// row are in CPState.Visible.
	lost      bool
	decidedAt time.Time
	// stopped is true while the agent does not feed. stoppedAt is the start
	// of the stop.
	stopped   bool
	stoppedAt time.Time
	// armed is true when the stop came while the watchdog was armed.
	armed bool
	// final is true when the stop does not reverse.
	final     bool
	poll      *poll
	witnesses []witness
	// seeAnswers keep the time of each own answer SEE. Only a DeclareSuicide
	// to one of them not older than ControlPlaneLostAfter is accepted.
	seeAnswers map[seeAnswer]time.Time

	pollTimer *time.Timer
}

// New makes an Engine. It does not decide until SetParams.
func New(deps Deps, logger *log.Logger) *Engine {
	return &Engine{
		deps:       deps,
		logger:     logger,
		seeAnswers: make(map[seeAnswer]time.Time),
		decided:    make(chan struct{}),
		pollTimer:  stoppedTimer(),
	}
}

// SetParams sets the parameters for the next events.
func (e *Engine) SetParams(p Params) {
	e.params.Store(&p)
}

// ShouldFeed tells whether a fresh decision lets the agent feed the watchdog now.
func (e *Engine) ShouldFeed() bool {
	e.feedMu.Lock()
	defer e.feedMu.Unlock()

	return time.Now().Before(e.feedUntil)
}

// Decided is closed after the first decision. Before it ShouldFeed has no data.
func (e *Engine) Decided() <-chan struct{} {
	return e.decided
}

// Run blocks until ctx ends.
func (e *Engine) Run(ctx context.Context) error {
	cps := e.deps.ControlPlane.Updates()
	msgs := e.deps.Transport.Received()

	for {
		var event func(now time.Time)
		select {
		case <-ctx.Done():
			return nil
		case cp, ok := <-cps:
			if !ok {
				cps = nil
				continue
			}
			event = func(now time.Time) { e.onCP(cp, now) }
		case msg, ok := <-msgs:
			if !ok {
				msgs = nil
				continue
			}
			event = func(now time.Time) { e.onMessage(msg, now) }
		case <-e.pollTimer.C:
			event = e.onPollTimer
		}
		e.handle(event)
	}
}

func (e *Engine) handle(event func(now time.Time)) {
	now := time.Now()
	if p := e.params.Load(); p != nil {
		e.p = *p
	}
	event(now)
	e.publish()

	if !e.decidedAt.IsZero() {
		e.decideOnce.Do(func() { close(e.decided) })
	}
}

func (e *Engine) publish() {
	var until time.Time
	if !e.stopped {
		until = e.decidedAt.Add(decisionTTL)
	}

	e.feedMu.Lock()
	e.feedUntil = until
	e.feedMu.Unlock()
}

func (e *Engine) onCP(cp domain.CPState, now time.Time) {
	e.cp, e.hasCP = cp, true
	e.lost = !cp.Visible
	e.forgetSeeAnswers(now)

	// Before the first ready answer after the start the agent has no data
	// about the control plane. That is not a loss: no poll and no feed.
	if e.p.ControlPlaneLostAfter == 0 || !cp.Seen {
		return
	}
	e.decidedAt = now

	switch {
	case !e.lost && e.stopped:
		e.resume(now)
	case !e.lost:
	case !e.stopped:
		e.stop("control plane is lost", now)
		if e.armed {
			e.logger.Info("control plane is lost, poll the peers for a witness", "last_ready_ago", e.lastReadyAgo(now).String())
			e.startPoll(e.deps.Transport.Peers(), now)
		}
	default:
		e.checkWitnesses(cp)
	}
}

// checkWitnesses applies the freshness rule of cpview to each witness.
func (e *Engine) checkWitnesses(cp domain.CPState) {
	kept := e.witnesses[:0]
	for _, w := range e.witnesses {
		switch w.fresh.Observe(cp) {
		case cpview.FreshCanceled:
			continue
		case cpview.FreshWitness:
			e.deps.Metrics.PollAnswer(domain.NetworkMain, domain.AnswerWitness)
			e.declareSuicide(w)
			e.witnesses = nil
			return
		case cpview.FreshWait:
		}
		kept = append(kept, w)
	}
	e.witnesses = kept
}

func (e *Engine) onMessage(msg domain.Message, now time.Time) {
	switch m := msg.(type) {
	case domain.Question:
		e.answer(m, now)
	case domain.Answer:
		e.onAnswer(m, now)
	case domain.DeclareSuicide:
		e.onDeclareSuicide(m, now)
	case domain.Leaving:
		e.onLeaving(m.From)
	case domain.FFNSCreated:
		if e.deps.Created != nil {
			e.deps.Created(m.Node)
		}
	}
	// The other messages have no action.
}

// answer tells the asker the own view from the last read, without the API.
func (e *Engine) answer(q domain.Question, now time.Time) {
	if !e.hasCP {
		return
	}
	peer, ok := e.deps.Transport.Peer(q.From)
	if !ok {
		e.logger.Debug("question from a node out of the peer list", "node", q.From)
		return
	}

	a := domain.Answer{
		To:           q.From,
		PollID:       q.PollID,
		Epoch:        q.Epoch,
		Network:      q.Network,
		Verdict:      domain.VerdictSee,
		LastReadyAgo: e.lastReadyAgo(now),
	}
	if e.lost {
		a.Verdict = domain.VerdictNotSee
	}
	if err := e.deps.Transport.Send(peer, a); err != nil {
		e.logger.Debug("send answer", "node", q.From, "error", err.Error())

		return
	}
	if a.Verdict == domain.VerdictSee {
		e.seeAnswers[seeAnswer{node: q.From, pollID: q.PollID}] = now
	}
}

// onDeclareSuicide accepts DeclareSuicide only when the agent answered SEE to
// the same node with the same poll ID not more than ControlPlaneLostAfter ago.
// It gives the node of an accepted message to Deps.Attest. A rejected message
// has no other effect.
func (e *Engine) onDeclareSuicide(m domain.DeclareSuicide, now time.Time) {
	at, ok := e.seeAnswers[seeAnswer{node: m.From, pollID: m.PollID}]
	reason := ""
	switch {
	case !ok:
		reason = "no own answer SEE with this poll ID"
	case now.Sub(at) > e.p.ControlPlaneLostAfter:
		reason = "the own answer SEE is older than controlPlaneLostAfter"
	}
	if reason != "" {
		e.logger.Warn("DeclareSuicide rejected", "node", m.From, "poll_id", m.PollID, "reason", reason)
		return
	}
	e.logger.Warn("DeclareSuicide accepted", "node", m.From, "poll_id", m.PollID)
	if e.deps.Attest != nil {
		e.deps.Attest(m.From)
	}
}

// forgetSeeAnswers removes the answers SEE older than ControlPlaneLostAfter.
// They give no accepted DeclareSuicide.
func (e *Engine) forgetSeeAnswers(now time.Time) {
	for key, at := range e.seeAnswers {
		if now.Sub(at) > e.p.ControlPlaneLostAfter {
			delete(e.seeAnswers, key)
		}
	}
}

// declareSuicide sends DeclareSuicide with the poll ID of the answer of the
// witness. The agent sends it only after the stop of the feed of its armed
// watchdog. An agent without watchdog does not send it. Before the send the
// stop becomes final: the witness writes the attestation, and the controller
// takes it as a node that does not feed the watchdog any more.
func (e *Engine) declareSuicide(w witness) {
	if !e.armed || e.final {
		return
	}
	peer, ok := e.deps.Transport.Peer(w.peer)
	if !ok {
		e.logger.Warn("no DeclareSuicide: the witness is not in the peer list", "witness", w.peer)
		return
	}
	e.final = true
	e.deps.Finalize()
	if err := e.deps.Transport.Send(peer, domain.DeclareSuicide{PollID: w.pollID}); err != nil {
		e.logger.Warn("send DeclareSuicide", "witness", w.peer, "poll_id", w.pollID, "error", err.Error())
		return
	}
	e.logger.Warn("DeclareSuicide sent, the stop of the feed is final", "witness", w.peer, "poll_id", w.pollID,
		"stopped_for", time.Since(e.stoppedAt).String())
}

// onAnswer takes an answer of the poll. The first answer SEE that is fresher
// by more than witnessFresherBy ends the poll: it waits for the freshness
// rule. Other answers do not give a witness.
func (e *Engine) onAnswer(a domain.Answer, now time.Time) {
	p := e.poll
	if p == nil || a.PollID != p.id {
		return
	}
	q := p.asked[a.From]
	if q == nil || q.answered {
		return
	}
	q.answered = true
	if slices.Contains(p.round, a.From) {
		p.answered = true
	}

	// NOT SEE, a verdict of a newer agent and DYING of an older agent are
	// not a witness.
	if a.Verdict == domain.VerdictSee && !a.UnknownFields && q.ownLastReadyAgo-a.LastReadyAgo > e.p.WitnessFresherBy {
		e.witnesses = append(e.witnesses, witness{peer: a.From, pollID: a.PollID, fresh: cpview.NewFreshness(bootMillis())})
		e.endPoll()

		return
	}
	e.deps.Metrics.PollAnswer(domain.NetworkMain, domain.AnswerSupport)

	if p.roundClosed() {
		e.closeRound(now)
	}
}

func (e *Engine) onLeaving(node string) {
	if e.deps.Leaving != nil {
		e.deps.Leaving(node)
	}
}

// startPoll asks the peers in rounds.
func (e *Engine) startPoll(peers []domain.Peer, now time.Time) {
	e.endPoll()
	e.poll = newPoll(peers)
	e.deps.Metrics.Poll()
	e.askRound(firstRound, e.p.RoundTimeout, now)
}

func (e *Engine) askRound(size int, wait time.Duration, now time.Time) {
	p := e.poll
	size = min(size, len(p.queue))
	peers := p.queue[:size]
	p.queue = p.queue[size:]
	e.sendRound(peers, wait, now)
}

// sendRound asks the peers and waits for the answers. A send error is silence.
func (e *Engine) sendRound(peers []domain.Peer, wait time.Duration, now time.Time) {
	p := e.poll
	p.round = p.round[:0]
	p.answered = false
	ownLastReadyAgo := e.lastReadyAgo(now)
	for _, peer := range peers {
		q := p.asked[peer.Name]
		if q == nil {
			q = &question{peer: peer, ownLastReadyAgo: ownLastReadyAgo}
			p.asked[peer.Name] = q
		}
		err := e.deps.Transport.Send(peer, domain.Question{
			PollID:       p.id,
			Network:      domain.NetworkMain,
			To:           peer.Name,
			Addr:         peer.Addr,
			LastReadyAgo: ownLastReadyAgo,
		})
		q.failed = err != nil
		p.round = append(p.round, peer.Name)
	}
	e.pollTimer.Reset(wait)
	if p.roundClosed() {
		e.closeRound(now)
	}
}

// closeRound starts the next round, then the repeat to the silent peers, then ends the poll.
func (e *Engine) closeRound(now time.Time) {
	p := e.poll
	switch {
	case len(p.queue) > 0:
		e.askRound(p.nextRoundSize(), e.p.RoundTimeout, now)
	case !p.repeated && len(p.silent()) > 0:
		p.repeated = true
		e.sendRound(p.silent(), e.p.ConfirmTimeout, now)
	default:
		for range p.silent() {
			e.deps.Metrics.PollAnswer(domain.NetworkMain, domain.AnswerSilent)
		}
		e.endPoll()
		e.logger.Info("poll ended without a witness: no peer sees the control plane")
	}
}

func (e *Engine) onPollTimer(now time.Time) {
	if e.poll != nil {
		e.closeRound(now)
	}
}

// resume feeds again when the control plane is visible after a stop that is
// not final. With the own FFNS the control plane is visible only after a
// successful heartbeat.
func (e *Engine) resume(now time.Time) {
	if e.final {
		return
	}
	stoppedFor := now.Sub(e.stoppedAt).String()
	if e.armed && !e.deps.ResumeFeed() {
		e.final = true
		e.logger.Warn("control plane is visible again, but the watchdog does not resume the feed: the stop is final",
			"stopped_for", stoppedFor)
		return
	}
	armed := e.armed
	e.endPoll()
	e.witnesses = nil
	e.stopped, e.armed = false, false
	e.deps.Metrics.Decision(domain.DecisionFeed)
	// The decision is not the feed: the watchdog logs the first feed after it.
	if armed {
		e.logger.Warn("decision to feed again", "reason", "the control plane is visible again", "armed", true, "stopped_for", stoppedFor)
		return
	}
	e.logger.Info("decision to feed again", "reason", "the control plane is visible again", "armed", false, "stopped_for", stoppedFor)
}

// stop stops the feed. When stop returns, the armed watchdog gets no feed.
func (e *Engine) stop(reason string, now time.Time) {
	e.endPoll()
	e.witnesses = nil
	e.stopped, e.stoppedAt = true, now
	e.armed = e.deps.StopFeed()
	e.deps.Metrics.Decision(domain.DecisionStop)
	e.logger.Warn("stop feeding the watchdog", "reason", reason, "armed", e.armed)
}

func (e *Engine) endPoll() {
	e.poll = nil
	e.pollTimer.Stop()
}

// lastReadyAgo is the own lastReadyAgo now.
func (e *Engine) lastReadyAgo(now time.Time) time.Duration {
	if e.cp.ReadAt.IsZero() {
		return e.cp.LastReadyAgo
	}

	return e.cp.LastReadyAgo + now.Sub(e.cp.ReadAt)
}

func stoppedTimer() *time.Timer {
	t := time.NewTimer(time.Hour)
	t.Stop()

	return t
}
