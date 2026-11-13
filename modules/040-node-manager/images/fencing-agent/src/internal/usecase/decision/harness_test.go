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
	"math/rand/v2"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

const (
	ms = time.Millisecond
	s  = time.Second

	latency = 5 * ms
	// Probe timings of kubernetes-api-proxy by default.
	probeInterval = 1 * s
	failStep      = 500 * ms
	probeTimeout  = 500 * ms
	// monitorStep is the step of the fake watchdog that calls ShouldFeed.
	monitorStep = 10 * ms
)

// profile holds the decision parameters of one profile.
type profile struct {
	name            string
	params          Params
	fastReadAfter   time.Duration
	watchdogTimeout time.Duration
}

var (
	critical = profile{"critical", Params{ControlPlaneLostAfter: 4150 * ms, WitnessFresherBy: 2350 * ms, RoundTimeout: 150 * ms, ConfirmTimeout: 600 * ms}, 1200 * ms, 5 * s}
	normal   = profile{"normal", Params{ControlPlaneLostAfter: 9100 * ms, WitnessFresherBy: 2350 * ms, RoundTimeout: 150 * ms, ConfirmTimeout: 600 * ms}, 1200 * ms, 15 * s}
	slow     = profile{"slow", Params{ControlPlaneLostAfter: 20200 * ms, WitnessFresherBy: 2350 * ms, RoundTimeout: 150 * ms, ConfirmTimeout: 600 * ms}, 1200 * ms, 30 * s}
	profiles = []profile{critical, normal, slow}
)

type sentMsg struct {
	at       time.Time
	from, to string
	msg      domain.Message
}

// network delivers the messages between the nodes of one process.
type network struct {
	mu    sync.Mutex
	nodes map[string]*node
	order []*node
	// down tells that the link loses the messages. fail gives a send error.
	down func(from, to string) bool
	fail func(from, to string) error
	sent []sentMsg
}

// script answers a question for a node without an engine.
type script func(q domain.Question) (a domain.Answer, delay time.Duration, ok bool)

type node struct {
	name    string
	net     *network
	inbox   chan domain.Message
	cp      *fakeCP
	engine  *Engine
	armed   atomic.Bool
	metrics *metrics
	wd      *monitor
	script  script
	// left are the peers in the hold of «leaving». Peers does not give them,
	// as peers.List does. network.mu protects it.
	left []string

	mu  sync.Mutex
	got []sentMsg
	// feedStopped is set at the first StopFeed. sentBeforeStop is the number
	// of the messages of the network at it. armedStopped is set at a StopFeed
	// of the armed watchdog and reset at ResumeFeed, as the watchdog does.
	// final is set at Finalize. resumes counts the resumed feeds of the armed
	// watchdog. attested are the nodes of the accepted DeclareSuicide
	// messages.
	feedStopped    bool
	sentBeforeStop int
	armedStopped   bool
	final          bool
	resumes        int
	attested       []string
}

// resumeFeed is Deps.ResumeFeed of the node.
func (n *node) resumeFeed() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.final {
		return false
	}
	n.armedStopped = false
	n.resumes++

	return true
}

// finalize is Deps.Finalize of the node.
func (n *node) finalize() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.final = true
}

func (n *node) resumed() int {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.resumes
}

// stopFeed is Deps.StopFeed of the node.
func (n *node) stopFeed() bool {
	n.net.mu.Lock()
	sent := len(n.net.sent)
	n.net.mu.Unlock()

	n.mu.Lock()
	defer n.mu.Unlock()
	if !n.feedStopped {
		n.feedStopped, n.sentBeforeStop = true, sent
	}
	armed := n.armed.Load()
	n.armedStopped = n.armedStopped || armed

	return armed
}

// attest is Deps.Attest of the node.
func (n *node) attest(node string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.attested = append(n.attested, node)
}

func (n *node) attestedNodes() []string {
	n.mu.Lock()
	defer n.mu.Unlock()

	return slices.Clone(n.attested)
}

type cluster struct {
	t    *testing.T
	ctx  context.Context
	prof profile
	net  *network
}

func newCluster(t *testing.T, prof profile) *cluster {
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	return &cluster{t: t, ctx: ctx, prof: prof, net: &network{nodes: make(map[string]*node)}}
}

func (c *cluster) add(name string) *node {
	n := &node{name: name, net: c.net, inbox: make(chan domain.Message, 8192)}
	c.net.mu.Lock()
	c.net.nodes[name] = n
	c.net.order = append(c.net.order, n)
	c.net.mu.Unlock()

	return n
}

// agent adds a node with an engine, a fake control plane and a fake watchdog, and starts them.
func (c *cluster) agent(name string, armed bool) *node {
	n := c.add(name)
	n.armed.Store(armed)
	n.metrics = &metrics{}
	n.cp = newFakeCP(c.prof)
	n.engine = New(Deps{Transport: n, ControlPlane: n.cp, StopFeed: n.stopFeed, ResumeFeed: n.resumeFeed, Finalize: n.finalize, Metrics: n.metrics, Attest: n.attest}, log.NewNop())
	n.engine.SetParams(c.prof.params)
	n.wd = &monitor{}
	go func() { _ = n.engine.Run(c.ctx) }()
	n.cp.run(c.ctx, randDuration(probeInterval))
	go n.wd.run(c.ctx, n.engine, c.prof.watchdogTimeout)

	return n
}

func (c *cluster) agents(armed bool, names ...string) []*node {
	var out []*node
	for _, name := range names {
		out = append(out, c.agent(name, armed))
	}

	return out
}

// scripted adds a node that answers by the script.
func (c *cluster) scripted(name string, sc script) *node {
	n := c.add(name)
	n.script = sc

	return n
}

// cut makes the links between the two sets of nodes lose the messages.
func (c *cluster) cut(a, b []string) {
	c.setDown(func(from, to string) bool {
		return slices.Contains(a, from) && slices.Contains(b, to) || slices.Contains(b, from) && slices.Contains(a, to)
	})
}

// isolate makes all links of the node lose the messages.
func (c *cluster) isolate(name string) {
	c.setDown(func(from, to string) bool { return from == name || to == name })
}

func (c *cluster) setFail(f func(from, to string) error) {
	c.net.mu.Lock()
	c.net.fail = f
	c.net.mu.Unlock()
}

func (c *cluster) setDown(f func(from, to string) bool) {
	c.net.mu.Lock()
	c.net.down = f
	c.net.mu.Unlock()
}

// questions returns the questions that the node sent, in order.
func (c *cluster) questions(from string) []sentMsg {
	c.net.mu.Lock()
	defer c.net.mu.Unlock()
	var out []sentMsg
	for _, m := range c.net.sent {
		if _, ok := m.msg.(domain.Question); ok && m.from == from {
			out = append(out, m)
		}
	}

	return out
}

func (c *cluster) sentOf(from string, match func(domain.Message) bool) []sentMsg {
	c.net.mu.Lock()
	defer c.net.mu.Unlock()
	var out []sentMsg
	for _, m := range c.net.sent {
		if m.from == from && match(m.msg) {
			out = append(out, m)
		}
	}

	return out
}

// declared returns the DeclareSuicide messages of the node.
func (c *cluster) declared(from string) []sentMsg {
	return c.sentOf(from, func(m domain.Message) bool { _, ok := m.(domain.DeclareSuicide); return ok })
}

// checkDeclareSuicide checks that x sent one DeclareSuicide, after the stop of
// the feed, to a peer that answered SEE to x with the same poll ID. It returns
// the name of that peer.
func (c *cluster) checkDeclareSuicide(x *node) string {
	c.t.Helper()
	sent := c.declared(x.name)
	if len(sent) != 1 {
		c.t.Fatalf("%s sent %d DeclareSuicide, want 1", x.name, len(sent))
	}
	d := sent[0]
	c.net.mu.Lock()
	index := slices.IndexFunc(c.net.sent, func(m sentMsg) bool { _, ok := m.msg.(domain.DeclareSuicide); return ok && m.from == x.name })
	c.net.mu.Unlock()
	x.mu.Lock()
	stopped, before := x.feedStopped, x.sentBeforeStop
	x.mu.Unlock()
	if !stopped || index < before {
		c.t.Errorf("DeclareSuicide is message %d, the feed stopped (%v) after %d messages: want the stop first", index, stopped, before)
	}
	pollID := d.msg.(domain.DeclareSuicide).PollID
	for _, a := range x.received(isAnswer) {
		if a.from == d.to && a.msg.(domain.Answer).PollID == pollID && a.msg.(domain.Answer).Verdict == domain.VerdictSee {
			return d.to
		}
	}
	c.t.Errorf("DeclareSuicide to %s with poll ID %d: no answer SEE of %s with this poll ID", d.to, pollID, d.to)

	return d.to
}

// noDeclareSuicide checks that no agent sends DeclareSuicide.
func (c *cluster) noDeclareSuicide() {
	c.t.Helper()
	c.net.mu.Lock()
	defer c.net.mu.Unlock()
	for _, m := range c.net.sent {
		if _, ok := m.msg.(domain.DeclareSuicide); ok {
			c.t.Errorf("%s sent DeclareSuicide", m.from)
		}
	}
}

func (n *node) Peers() []domain.Peer {
	n.net.mu.Lock()
	defer n.net.mu.Unlock()
	var peers []domain.Peer
	for i, p := range n.net.order {
		if p != n && !slices.Contains(n.left, p.name) {
			peers = append(peers, domain.Peer{Name: p.name, Addr: peerAddr(i), Compatible: true})
		}
	}

	return peers
}

func (n *node) Peer(name string) (domain.Peer, bool) {
	n.net.mu.Lock()
	defer n.net.mu.Unlock()
	for i, p := range n.net.order {
		if p != n && p.name == name {
			return domain.Peer{Name: p.name, Addr: peerAddr(i), Compatible: true}, true
		}
	}

	return domain.Peer{}, false
}

func peerAddr(i int) netip.Addr {
	return netip.AddrFrom4([4]byte{10, 0, byte(i / 250), byte(i%250 + 1)})
}

// hold puts the peer in the hold of «leaving» of the node.
func (n *node) hold(peer string) {
	n.net.mu.Lock()
	n.left = append(n.left, peer)
	n.net.mu.Unlock()
}

func (n *node) Send(to domain.Peer, msg domain.Message) error {
	return n.net.send(n.name, to.Name, msg)
}

func (n *node) Received() <-chan domain.Message { return n.inbox }

func (nw *network) send(from, to string, msg domain.Message) error {
	msg = withFrom(msg, from)
	nw.mu.Lock()
	nw.sent = append(nw.sent, sentMsg{at: time.Now(), from: from, to: to, msg: msg})
	lost := nw.down != nil && nw.down(from, to)
	var err error
	if nw.fail != nil {
		err = nw.fail(from, to)
	}
	dst := nw.nodes[to]
	nw.mu.Unlock()

	if err != nil {
		return err
	}
	if !lost && dst != nil {
		time.AfterFunc(latency, func() { dst.deliver(from, msg) })
	}

	return nil
}

func (n *node) deliver(from string, msg domain.Message) {
	n.mu.Lock()
	n.got = append(n.got, sentMsg{at: time.Now(), from: from, to: n.name, msg: msg})
	n.mu.Unlock()

	if n.script == nil {
		select {
		case n.inbox <- msg:
		default:
		}
		return
	}
	q, ok := msg.(domain.Question)
	if !ok {
		return
	}
	a, delay, ok := n.script(q)
	if !ok {
		return
	}
	a.To, a.Epoch, a.Network = q.From, q.Epoch, q.Network
	if a.PollID == 0 {
		a.PollID = q.PollID
	}
	reply := func() { _ = n.net.send(n.name, q.From, a) }
	if delay <= 0 {
		// Do not start a timer that fires at once. In a synctest bubble such a
		// timer runs in the calling goroutine, and with -race Go 1.26 can crash
		// (go.dev/issue/76691, fixed in Go 1.27).
		go reply()
		return
	}
	time.AfterFunc(delay, reply)
}

func (n *node) received(match func(domain.Message) bool) []sentMsg {
	n.mu.Lock()
	defer n.mu.Unlock()
	var out []sentMsg
	for _, m := range n.got {
		if match(m.msg) {
			out = append(out, m)
		}
	}

	return out
}

func withFrom(msg domain.Message, from string) domain.Message {
	switch m := msg.(type) {
	case domain.Question:
		m.From = from
		return m
	case domain.Answer:
		m.From = from
		return m
	case domain.DeclareSuicide:
		m.From = from
		return m
	}

	return msg
}

func isAnswer(m domain.Message) bool { _, ok := m.(domain.Answer); return ok }

// Scripts.

func silent(domain.Question) (domain.Answer, time.Duration, bool) {
	return domain.Answer{}, 0, false
}

func answering(v domain.Verdict, delay time.Duration) script {
	return func(q domain.Question) (domain.Answer, time.Duration, bool) {
		return domain.Answer{Verdict: v, LastReadyAgo: q.LastReadyAgo}, delay, true
	}
}

// seeing answers SEE with lastReadyAgo less than the lastReadyAgo of the asker by gap.
func seeing(gap time.Duration) script {
	return func(q domain.Question) (domain.Answer, time.Duration, bool) {
		return domain.Answer{Verdict: domain.VerdictSee, LastReadyAgo: max(0, q.LastReadyAgo-gap)}, 0, true
	}
}

// fakeCP is the control plane source of a node. Probe rounds go as in
// kubernetes-api-proxy: the rounds run one after another, a new round starts
// probeInterval after the start of a round, failStep after the start of a
// failed round, or when the round ends if it ends later.
// The number of a round is the time of its start in milliseconds, as in
// cpview. The control plane is not visible after two reads in a row with
// lastReadyAgo more than controlPlaneLostAfter, as in cpview. A failed round takes failDelay.
type fakeCP struct {
	out                                  chan domain.CPState
	controlPlaneLostAfter, fastReadAfter time.Duration
	failDelay                            time.Duration
	reach                                atomic.Bool
	paused                               atomic.Bool
	source                               atomic.Value

	mu        sync.Mutex
	seq       uint64
	started   uint64
	ok        bool
	lastReady time.Time
	misses    int
}

func newFakeCP(prof profile) *fakeCP {
	c := &fakeCP{out: make(chan domain.CPState), controlPlaneLostAfter: prof.params.ControlPlaneLostAfter, fastReadAfter: prof.fastReadAfter, failDelay: probeTimeout, ok: true}
	c.reach.Store(true)
	c.source.Store(domain.CPSourceProxy)

	return c
}

func (c *fakeCP) Updates() <-chan domain.CPState { return c.out }

func (c *fakeCP) run(ctx context.Context, phase time.Duration) {
	c.lastReady = time.Now()
	go c.probe(ctx, phase)
	go c.read(ctx)
}

func (c *fakeCP) probe(ctx context.Context, phase time.Duration) {
	if !sleep(ctx, phase) {
		return
	}
	for {
		start := time.Now()
		c.started = max(uint64(start.UnixMilli()), c.started+1)
		number := c.started
		ok := c.reach.Load()
		c.mu.Lock()
		took, next := c.failDelay, failStep
		c.mu.Unlock()
		if ok {
			took, next = latency, probeInterval
		}
		if !sleep(ctx, took) {
			return
		}
		c.mu.Lock()
		c.seq = number
		c.ok = ok
		if ok {
			c.lastReady = start
		}
		c.mu.Unlock()
		if !sleep(ctx, max(next-took, 0)) {
			return
		}
	}
}

// failFast makes failed rounds end at once, as when the apiserver answers "not ready".
func (c *fakeCP) failFast() {
	c.mu.Lock()
	c.failDelay = latency
	c.mu.Unlock()
}

func (c *fakeCP) read(ctx context.Context) {
	for {
		now := time.Now()
		c.mu.Lock()
		lastReadyAgo := now.Sub(c.lastReady)
		if lastReadyAgo <= c.controlPlaneLostAfter {
			c.misses = 0
		} else {
			c.misses++
		}
		st := domain.CPState{Source: c.source.Load().(domain.CPSource), Visible: c.misses < 2, Seen: true, LastReadyAgo: lastReadyAgo, ProbeSeq: c.seq, ProbeOK: c.ok, ReadAt: now}
		c.mu.Unlock()
		if !c.paused.Load() {
			select {
			case c.out <- st:
			case <-ctx.Done():
				return
			}
		}
		next := 200 * ms
		if lastReadyAgo > c.fastReadAfter {
			next = 100 * ms
		}
		if !sleep(ctx, next) {
			return
		}
	}
}

// monitor is the fake watchdog: it calls ShouldFeed each monitorStep.
type monitor struct {
	mu sync.Mutex
	// stoppedAt is the first time without the feed after a feed.
	stoppedAt time.Time
	// firedAt is the time when the watchdog would fire: no feed for watchdog.timeout.
	firedAt time.Time
	resumed bool
}

func (m *monitor) run(ctx context.Context, e *Engine, twd time.Duration) {
	var lastFed time.Time
	for sleep(ctx, monitorStep) {
		now := time.Now()
		feed := e.ShouldFeed()
		m.mu.Lock()
		switch {
		case feed:
			if !m.stoppedAt.IsZero() {
				m.resumed = true
			}
			lastFed = now
		case !lastFed.IsZero() && m.stoppedAt.IsZero():
			m.stoppedAt = now
		}
		if !lastFed.IsZero() && m.firedAt.IsZero() && now.Sub(lastFed) >= twd {
			m.firedAt = lastFed.Add(twd)
		}
		m.mu.Unlock()
	}
}

func (m *monitor) stopped() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.stoppedAt
}

func (m *monitor) wasResumed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.resumed
}

func (m *monitor) fired() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.firedAt
}

type event struct {
	at    time.Time
	value string
}

type metrics struct {
	mu        sync.Mutex
	decisions []event
	answers   []event
	polls     int
}

func (m *metrics) Decision(d domain.FeedDecision) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.decisions = append(m.decisions, event{time.Now(), string(d)})
}

func (m *metrics) Poll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.polls++
}

func (m *metrics) PollAnswer(_ domain.Network, class domain.AnswerClass) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.answers = append(m.answers, event{time.Now(), string(class)})
}

func (m *metrics) ConfigInvalid(string, bool) {}

// last returns the time of the last event with the value, or a zero time.
func last(events []event, value string) time.Time {
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].value == value {
			return events[i].at
		}
	}

	return time.Time{}
}

func (m *metrics) lastDecision(d domain.FeedDecision) time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()

	return last(m.decisions, string(d))
}

func (m *metrics) lastAnswer(class domain.AnswerClass) time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()

	return last(m.answers, string(class))
}

func (m *metrics) count(d domain.FeedDecision) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, e := range m.decisions {
		if e.value == string(d) {
			n++
		}
	}

	return n
}

func (m *metrics) pollCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.polls
}

func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		// Do not start a timer that fires at once, for the same reason as in
		// node.deliver.
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// warmUp is the time before a failure in the scenarios.
func warmUp() {
	time.Sleep(2*s + randDuration(probeInterval))
}

func randDuration(limit time.Duration) time.Duration {
	return time.Duration(rand.Int64N(int64(limit) + 1))
}
