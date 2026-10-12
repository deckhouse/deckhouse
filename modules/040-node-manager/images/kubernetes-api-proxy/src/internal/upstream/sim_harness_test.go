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

package upstream

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"kubernetes-api-proxy/internal/firstbyte"
)

type step struct {
	note      string
	probes    map[string]probeResult
	downs     []string
	slice     []string
	bad       []string
	rounds    int
	blackhole []string
}

type report struct {
	badSeconds, badHung, badLate, badFast, noGood, waitSeconds float64
	rounds                                                     []simRound

	llMaxStuck, llOver10, llOver40, llEvictions, llFalseKills, llPeakReconnects, llFirstEviction int
}

type simRound struct {
	note  string
	picks map[string]int
	bad   float64
	wait  float64
}

const (
	simPicksPerRound = 100
	simProbeTimeout  = 500 * time.Millisecond
	simSlowFirstByte = 1500 * time.Millisecond
	simFloor         = 3 * time.Second
	simDialTimeout   = 5 * time.Second
	simClientTimeout = 10 * time.Second
	simNoUpstream    = "none"

	simStallTimeout = 3 * time.Second
	simLongLived    = 30
	simPingExit     = 45
	simUserTimeout  = 5
)

type simPick struct {
	addr   string
	waited time.Duration
	total  time.Duration
}

type simStall struct {
	b          *Upstream
	start, due time.Time
	r          probeResult
	timedOut   bool
}

type simConn struct {
	addr  string
	stuck int
}

type sim struct {
	t       *testing.T
	list    *List
	fake    *fakeProbes
	clock   *fakeClock
	budget  *firstbyte.Budget
	rep     report
	cur     step
	prev    *step
	stalls  []simStall
	conns   []simConn
	evicted map[string]bool
}

func simulate(t *testing.T, nodes []string, steps []step, opts ...ListOption) report {
	t.Helper()

	fake := newFakeProbes()
	clock := &fakeClock{now: time.Unix(0, 0)}
	base := []ListOption{WithHealthcheckInterval(time.Hour), WithDownHold(simDialTimeout), withProbes(fake), WithStallTimeout(simStallTimeout)}

	list, err := newList(nil, byAddr, append(base, opts...)...)
	if err != nil {
		t.Fatal(err)
	}

	s := &sim{t: t, list: list, fake: fake, clock: clock, budget: firstbyte.New(simFloor), evicted: map[string]bool{}}

	list.now = clock.Now
	list.stallHook = s.stallStart
	list.OnStall(s.evict)
	list.Reconcile(ups(nodes...))

	for _, st := range steps {
		for range max(st.rounds, 1) {
			s.round(st)
		}
	}

	for i := range s.conns {
		s.closeRun(&s.conns[i])
	}

	return s.rep
}

func (s *sim) round(st step) {
	s.clock.Advance(time.Second)
	s.cur = st

	if st.slice != nil {
		s.list.Reconcile(ups(st.slice...))
	}

	s.resolveStalls()
	s.seed(st)
	s.loadProbes(st.probes)
	s.list.round(context.Background())

	for _, addr := range st.downs {
		s.list.Down(NewUpstream(addr, nil))
	}

	s.longLived(st)

	picks := make([]simPick, 0, simPicksPerRound)
	for range simPicksPerRound {
		picks = append(picks, s.pick(st))
	}

	s.record(st, picks)
	s.prev = &st
}

func (s *sim) stallStart(b *Upstream, start time.Time) {
	r, known := s.cur.probes[b.Addr]
	stall := simStall{b: b, start: start, due: start.Add(simStallTimeout), timedOut: true}

	switch {
	case !known, r.kind == probeSoft && slices.Contains(s.cur.bad, b.Addr), r.kind == probeHard && slices.Contains(s.cur.blackhole, b.Addr):
	case r.kind == probeSoft:
		stall.due, stall.r, stall.timedOut = start.Add(simSlowFirstByte), probeResult{kind: probeNotReady}, false
	case r.kind == probeHard:
		stall.due, stall.timedOut = start, false
	case r.latency < simStallTimeout:
		stall.due, stall.r, stall.timedOut = start.Add(r.latency), r, false
	}

	s.stalls = append(s.stalls, stall)
}

func (s *sim) resolveStalls() {
	now := s.clock.Now()

	var due []simStall

	s.stalls = slices.DeleteFunc(s.stalls, func(c simStall) bool {
		if c.due.After(now) {
			return false
		}

		due = append(due, c)

		return true
	})

	slices.SortStableFunc(due, func(a, b simStall) int { return a.due.Compare(b.due) })

	for _, c := range due {
		s.list.stallDone(c.b, c.start, c.r, c.timedOut)
	}
}

func (s *sim) evict(u *Upstream) {
	s.evicted[u.Addr] = true
	s.rep.llEvictions++

	if s.rep.llFirstEviction == 0 {
		s.rep.llFirstEviction = len(s.rep.rounds) + 1
	}
}

func (s *sim) seed(st step) {
	if s.conns != nil || st.note == "warm" {
		return
	}

	from := st
	if s.prev != nil {
		from = *s.prev
	}

	s.conns = make([]simConn, 0, simLongLived)
	for range simLongLived {
		s.conns = append(s.conns, simConn{addr: s.pick(from).addr})
	}
}

func (s *sim) longLived(st step) {
	reconnects := 0

	for i := range s.conns {
		c := &s.conns[i]

		switch {
		case s.evicted[c.addr]:
			if !slices.Contains(st.bad, c.addr) {
				s.rep.llFalseKills++
			}
		case c.addr != simNoUpstream && !slices.Contains(st.bad, c.addr):
			s.closeRun(c)

			continue
		default:
			c.stuck++
			if !naturalExit(st, *c) {
				continue
			}
		}

		s.closeRun(c)
		c.addr = s.pick(st).addr
		reconnects++
	}

	clear(s.evicted)
	s.rep.llPeakReconnects = max(s.rep.llPeakReconnects, reconnects)
}

func naturalExit(st step, c simConn) bool {
	r, known := st.probes[c.addr]

	switch {
	case c.addr == simNoUpstream:
		return true
	case !known || r.kind == probeSoft:
		return c.stuck >= simPingExit
	case r.kind == probeHard && slices.Contains(st.blackhole, c.addr):
		return c.stuck >= simUserTimeout
	default:
		return r.kind == probeHard
	}
}

func (s *sim) closeRun(c *simConn) {
	if c.stuck == 0 {
		return
	}

	s.rep.llMaxStuck = max(s.rep.llMaxStuck, c.stuck)

	if c.stuck > 10 {
		s.rep.llOver10++
	}

	if c.stuck > 40 {
		s.rep.llOver40++
	}

	c.stuck = 0
}

func (r report) longLived() string {
	return fmt.Sprintf("llEvictions=%d llFirstEviction=%d llMaxStuck=%d llOver10=%d llOver40=%d llFalseKills=%d llPeakReconnects=%d",
		r.llEvictions, r.llFirstEviction, r.llMaxStuck, r.llOver10, r.llOver40, r.llFalseKills, r.llPeakReconnects)
}

func (s *sim) loadProbes(probes map[string]probeResult) {
	s.fake.mu.Lock()
	defer s.fake.mu.Unlock()

	s.fake.results = make(map[string]probeResult, len(probes))
	for addr, r := range probes {
		s.fake.results[addr] = probeSeen(r)
	}
}

func probeSeen(r probeResult) probeResult {
	if r.kind == probeSoft || r.kind == probeHard || r.latency <= simProbeTimeout {
		return r
	}

	return probeResult{kind: probeSoft}
}

func (s *sim) pick(st step) simPick {
	s.t.Helper()

	first, err := s.list.Pick()
	if err != nil {
		s.t.Fatalf("Pick() error = %v", err)
	}

	backend, retried, lost := s.dialFirst(st, first)
	if backend == nil {
		return simPick{addr: simNoUpstream, waited: lost, total: time.Hour}
	}

	lat := firstByteLatency(st, backend.Addr)
	stay := simPick{addr: backend.Addr, waited: lost, total: lost + lat}

	wait := s.budget.Wait(backend.Addr)
	if lat <= wait {
		s.budget.Observe(backend.Addr, lat)

		return stay
	}

	s.expire(backend)

	if retried {
		return stay
	}

	return s.secondHop(st, backend, lost, lat, wait)
}

func (s *sim) dialFirst(st step, first *Upstream) (*Upstream, bool, time.Duration) {
	cost, ok := s.dial(st, first, simDialTimeout)
	if ok {
		return first, false, 0
	}

	next, err := s.list.PickNot(first)
	if err != nil {
		return nil, false, cost
	}

	more, ok := s.dial(st, next, simDialTimeout)
	if !ok {
		return nil, true, cost + more
	}

	return next, true, cost + more
}

func (s *sim) secondHop(st step, backend *Upstream, lost, lat, wait time.Duration) simPick {
	stay := simPick{addr: backend.Addr, waited: lost, total: lost + lat}

	next, err := s.list.PickNot(backend)
	if err != nil {
		return stay
	}

	remaining := s.budget.Chain() - wait

	cost, ok := s.dial(st, next, min(simDialTimeout, remaining))
	if !ok {
		stay.total = lost + max(lat, wait+cost)
		stay.waited = stay.total - lat

		return stay
	}

	full := s.budget.Wait(next.Addr)
	nextWait := min(full, remaining)
	nextLat := firstByteLatency(st, next.Addr)
	hop := simPick{addr: next.Addr, waited: lost + wait, total: lost + wait + nextLat}

	if nextLat <= nextWait {
		s.budget.Observe(next.Addr, nextLat)
	} else if nextWait == full {
		s.expire(next)
	}

	return hop
}

func (s *sim) dial(st step, u *Upstream, limit time.Duration) (time.Duration, bool) {
	switch {
	case slices.Contains(st.blackhole, u.Addr):
		s.list.Down(u)

		return limit, false
	case st.probes[u.Addr].kind == probeHard:
		s.list.Down(u)

		return 0, false
	default:
		return 0, true
	}
}

func (s *sim) expire(u *Upstream) {
	s.budget.Timeout(u.Addr)
	s.list.Silent(u)
}

func firstByteLatency(st step, addr string) time.Duration {
	r, known := st.probes[addr]

	switch {
	case !known:
		return 0
	case r.kind == probeSoft && slices.Contains(st.bad, addr):
		return time.Hour
	case r.kind == probeSoft:
		return simSlowFirstByte
	default:
		return r.latency
	}
}

func (s *sim) record(st step, picks []simPick) {
	counts := map[string]int{}
	bad := 0.0

	var waited time.Duration

	for _, p := range picks {
		counts[p.addr]++
		waited += p.waited

		if !slices.Contains(st.bad, p.addr) && p.addr != simNoUpstream && p.total <= simClientTimeout {
			continue
		}

		bad += 1.0 / simPicksPerRound
		s.classify(st, p)
	}

	if len(st.bad) > 0 && !slices.ContainsFunc(s.list.backends(), func(u *Upstream) bool { return !slices.Contains(st.bad, u.Addr) }) {
		s.rep.noGood++
	}

	wait := waited.Seconds() / simPicksPerRound
	s.rep.badSeconds += bad
	s.rep.waitSeconds += wait
	s.rep.rounds = append(s.rep.rounds, simRound{note: st.note, picks: counts, bad: bad, wait: wait})
}

func (s *sim) classify(st step, p simPick) {
	r, known := st.probes[p.addr]

	switch {
	case p.addr == simNoUpstream || !known || r.kind == probeSoft || r.kind == probeHard:
		s.rep.badHung += 1.0 / simPicksPerRound
	case r.latency > simProbeTimeout || p.total > simClientTimeout:
		s.rep.badLate += 1.0 / simPicksPerRound
	default:
		s.rep.badFast += 1.0 / simPicksPerRound
	}
}

func (r report) dump() string {
	var b strings.Builder

	for i, rd := range r.rounds {
		if rd.note == "warm" {
			continue
		}

		parts := make([]string, 0, len(rd.picks))
		for _, addr := range slices.Sorted(maps.Keys(rd.picks)) {
			parts = append(parts, fmt.Sprintf("%s:%d", addr, rd.picks[addr]))
		}

		fmt.Fprintf(&b, "r%-3d bad=%.2f wait=%.2f %-40s %s\n", i+1, rd.bad, rd.wait, strings.Join(parts, " "), rd.note)
	}

	return b.String()
}
