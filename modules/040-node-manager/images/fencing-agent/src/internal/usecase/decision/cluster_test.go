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
	"slices"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"fencing-agent/internal/domain"
)

// The decision on several engines in one process with a fake
// network, fake control plane sources and fake watchdogs.

// freshnessWait ≈ 2·failStepInterval + proxyTimeout of the freshness rule.
const freshnessWait = 2*failStep + probeTimeout

func names(nodes []*node) []string {
	var out []string
	for _, n := range nodes {
		out = append(out, n.name)
	}

	return out
}

// healthy checks that the node did not stop the feed.
func healthy(t *testing.T, nodes ...*node) {
	t.Helper()
	for _, n := range nodes {
		if n.stopped() || !n.wd.stopped().IsZero() || !n.wd.fired().IsZero() {
			t.Errorf("%s: stop %v, feed stopped at %v, watchdog fired at %v", n.name, n.stopped(), n.wd.stopped(), n.wd.fired())
		}
	}
}

// stoppedWithin checks that the feed stopped not later than bound after t0
// and the watchdog fired watchdog.timeout after the last feed.
func stoppedWithin(t *testing.T, n *node, t0 time.Time, bound, twd time.Duration) {
	t.Helper()
	stop := n.wd.stopped()
	if stop.IsZero() || stop.Sub(t0) > bound {
		t.Errorf("%s: feed stopped %v after the failure, want ≤ %v", n.name, stop.Sub(t0), bound)
	}
	if fired := n.wd.fired(); fired.IsZero() || fired.Sub(t0) > bound+twd {
		t.Errorf("%s: watchdog fired %v after the failure, want ≤ %v", n.name, fired.Sub(t0), bound+twd)
	}
}

// notFired checks that the watchdog of the node did not fire.
func notFired(t *testing.T, nodes ...*node) {
	t.Helper()
	for _, n := range nodes {
		if fired := n.wd.fired(); !fired.IsZero() {
			t.Errorf("%s: watchdog fired at %v", n.name, fired)
		}
	}
}

// fired checks that the watchdog of the node fired.
func fired(t *testing.T, nodes ...*node) {
	t.Helper()
	for _, n := range nodes {
		if n.wd.fired().IsZero() {
			t.Errorf("%s: watchdog did not fire", n.name)
		}
	}
}

// Isolation of one node on all networks, and a master isolated on all networks.
func TestIsolationOnAllNetworks(t *testing.T) {
	for _, prof := range profiles {
		for _, sendErr := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/send error %v", prof.name, sendErr), func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					c := newCluster(t, prof)
					nodes := c.agents(true, "master-3", "w1", "w2", "w3")
					x := nodes[0]
					warmUp()
					t0 := time.Now()
					c.isolate(x.name)
					if sendErr {
						c.setFail(func(from, _ string) error {
							if from == x.name {
								return syscall.ENETUNREACH
							}
							return nil
						})
					}
					x.cp.reach.Store(false)
					time.Sleep(prof.params.ControlPlaneLostAfter + prof.watchdogTimeout + 3*s)

					// critical: stop ≤ 4.35 s, the node stops ≤ 9.35 s.
					stoppedWithin(t, x, t0, prof.params.ControlPlaneLostAfter+200*ms, prof.watchdogTimeout)
					healthy(t, nodes[1:]...)
					c.noDeclareSuicide()
				})
			})
		}
	}
}

// A witness on the main network at a real loss of API access.
func TestWitnessOnMainNetwork(t *testing.T) {
	for _, prof := range profiles {
		t.Run(prof.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				c := newCluster(t, prof)
				nodes := c.agents(true, "x", "w1", "w2", "master-1")
				x := nodes[0]
				warmUp()
				t0 := time.Now()
				x.cp.reach.Store(false)
				time.Sleep(prof.params.ControlPlaneLostAfter + prof.watchdogTimeout + 3*s)

				stoppedWithin(t, x, t0, prof.params.ControlPlaneLostAfter+200*ms, prof.watchdogTimeout)
				if x.metrics.lastAnswer(domain.AnswerWitness).IsZero() {
					t.Errorf("no witness")
				}
				if d := x.wd.stopped().Sub(t0); d < prof.params.ControlPlaneLostAfter-probeInterval {
					t.Errorf("stop %v after the failure, before the loss of CP", d)
				}
				healthy(t, nodes[1:]...)

				// The witness accepts DeclareSuicide: it answered SEE with this poll ID.
				witness := c.checkDeclareSuicide(x)
				for _, n := range nodes[1:] {
					want := []string(nil)
					if n.name == witness {
						want = []string{"x"}
					}
					if got := n.attestedNodes(); !slices.Equal(got, want) {
						t.Errorf("%s attested %v, want %v", n.name, got, want)
					}
				}
			})
		})
	}
}

// segment makes the nodes of seg lose the control plane and the links to rest.
func segment(c *cluster, seg, rest []*node) {
	c.cut(names(seg), names(rest))
	for _, n := range seg {
		n.cp.reach.Store(false)
	}
}

// A segment without CP, and a rack with the master in the minority: the nodes
// of the segment stop, and there is no witness for them.
func TestSegmentWithoutCP(t *testing.T) {
	for _, prof := range profiles {
		t.Run(prof.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				c := newCluster(t, prof)
				seg := c.agents(true, "master-3", "w1", "w2")
				rest := c.agents(true, "master-1", "master-2", "w3")
				warmUp()
				segment(c, seg, rest)
				time.Sleep(prof.params.ControlPlaneLostAfter + prof.watchdogTimeout + 3*s)

				fired(t, seg...)
				healthy(t, rest...)
				c.noDeclareSuicide()
				for _, n := range rest {
					if n.metrics.pollCount() != 0 {
						t.Errorf("%s polled with CP visible", n.name)
					}
				}
			})
		})
	}
}

// A full loss of the CP a few seconds after the agent of x restarted: the
// peers still hold «leaving» of the old agent of x. The hold keeps x out of
// the polls of the peers, but the peers answer x.
func TestFullLossInLeavingHold(t *testing.T) {
	for _, prof := range profiles {
		t.Run(prof.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				c := newCluster(t, prof)
				nodes := c.agents(true, "x", "w1", "w2", "master-1")
				x := nodes[0]
				for _, n := range nodes[1:] {
					n.hold(x.name)
				}
				warmUp()
				for _, n := range nodes {
					n.cp.reach.Store(false)
				}
				time.Sleep(prof.params.ControlPlaneLostAfter + 2*s)

				c.noDeclareSuicide()
				for _, n := range nodes[1:] {
					for _, q := range c.questions(n.name) {
						if q.to == x.name {
							t.Errorf("%s asked %s in the hold of «leaving»", n.name, x.name)
						}
					}
				}
				if len(x.received(func(m domain.Message) bool { _, ok := m.(domain.Answer); return ok })) == 0 {
					t.Errorf("x got no answer from the peers")
				}
			})
		})
	}
}

// The CP returns after a segment before the watchdog fires: each node of the
// segment feeds again. Nobody sends DeclareSuicide: the peers that answer do
// not see the CP, and the others are out of reach.
func TestReturnAfterSegment(t *testing.T) {
	for _, prof := range profiles {
		for _, fast := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/fast fail %v", prof.name, fast), func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					c := newCluster(t, prof)
					seg := c.agents(true, "y", "x", "z")
					rest := c.agents(true, "w1", "w2")
					for _, n := range seg {
						if fast {
							n.cp.failFast()
						}
					}
					warmUp()
					segment(c, seg, rest)
					if !waitFor(prof.params.ControlPlaneLostAfter+5*s, func() bool { return seg[0].stopped() && seg[1].stopped() && seg[2].stopped() }) {
						t.Fatal("no stop")
					}
					time.Sleep(s)
					c.setDown(nil)
					for _, n := range seg {
						n.cp.reach.Store(true)
					}
					time.Sleep(prof.watchdogTimeout + s)

					notFired(t, seg...)
					healthy(t, rest...)
					c.noDeclareSuicide()
					for _, n := range seg {
						if n.resumed() != 1 || !n.engine.ShouldFeed() {
							t.Errorf("%s: resumes %d, feed %v; want one resume and the feed", n.name, n.resumed(), n.engine.ShouldFeed())
						}
					}
				})
			})
		}
	}
}

// A short pause of the CP, about controlPlaneLostAfter long: the CP is not
// available on all nodes for 4.35 s. The apiservers answer "not ready" at
// once, or the probes end by the timeout. The nodes that lose the CP stop and
// feed again: the freshness rule gives no witness, and no watchdog fires.
func TestShortPause(t *testing.T) {
	for _, fast := range []bool{true, false} {
		t.Run(fmt.Sprintf("fast fail %v", fast), func(t *testing.T) { testShortPause(t, fast) })
	}
}

func testShortPause(t *testing.T, fast bool) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, critical)
		nodes := c.agents(true, "master-1", "master-2", "master-3", "w1", "w2", "w3", "w4", "w5")
		for _, n := range nodes {
			if fast {
				n.cp.failFast()
			}
		}
		warmUp()
		lost := 0
		for range 10 {
			for _, n := range nodes {
				n.cp.reach.Store(false)
			}
			time.Sleep(4350 * ms)
			for _, n := range nodes {
				n.cp.reach.Store(true)
			}
			time.Sleep(10*s + randDuration(probeInterval))
		}
		for _, n := range nodes {
			lost += n.metrics.pollCount()
		}
		if lost == 0 {
			t.Fatal("no node lost CP: the scenario does not check the freshness rule")
		}
		notFired(t, nodes...)
		c.noDeclareSuicide()
	})
}

// The control plane of the whole cluster pauses for 6 s, longer than
// controlPlaneLostAfter. The nodes that lose the control plane first get SEE
// from the nodes with a later probe. The freshness rule gives no witness, and
// each node feeds again before its watchdog fires.
func TestResumeAfterTheClusterPause(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, critical)
		all := c.agents(true, "n1", "n2", "n3", "n4", "n5", "n6", "n7", "n8")
		warmUp()
		for _, n := range all {
			n.cp.reach.Store(false)
		}
		time.Sleep(6 * s)
		for _, n := range all {
			n.cp.reach.Store(true)
		}
		time.Sleep(15 * s)

		notFired(t, all...)
		c.noDeclareSuicide()
		for _, n := range all {
			if n.resumed() != 1 {
				t.Errorf("%s: resumes %d, want 1", n.name, n.resumed())
			}
		}
	})
}

// Only x loses the control plane; the peers see it. x sends DeclareSuicide
// after the stop. The control plane comes back 2 s after the stop: x does not
// feed again, and its watchdog fires.
func TestNoResumeAfterDeclareSuicide(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, critical)
		c.agents(true, "p1", "p2", "p3")
		x := c.agent("x", true)
		warmUp()
		x.cp.reach.Store(false)
		if !waitFor(critical.params.ControlPlaneLostAfter+3*s, x.stopped) {
			t.Fatal("no stop")
		}
		time.Sleep(2 * s)
		x.cp.reach.Store(true)
		time.Sleep(10 * s)

		c.checkDeclareSuicide(x)
		fired(t, x)
		if x.resumed() != 0 {
			t.Errorf("resumes %d after DeclareSuicide, want 0", x.resumed())
		}
	})
}

// The end of a pause: x loses the control plane at the moment when it comes
// back. A peer whose probe came back first answers SEE with a fresh probe.
// The freshness rule waits for the own probes: the next own probe succeeds, x
// does not send DeclareSuicide and feeds again.
func TestFreshnessRuleKeepsTheResumeAtTheEndOfThePause(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, critical)
		c.scripted("p1", seeing(10*s))
		x := c.agent("x", true)
		warmUp()
		x.cp.reach.Store(false)
		if !waitFor(critical.params.ControlPlaneLostAfter+3*s, x.stopped) {
			t.Fatal("no stop")
		}
		x.cp.reach.Store(true)
		time.Sleep(10 * s)

		c.noDeclareSuicide()
		notFired(t, x)
		if x.resumed() != 1 {
			t.Errorf("resumes %d, want 1", x.resumed())
		}
	})
}

// A segment with unarmed peers: the agents without watchdog do not poll.
func TestDisabledPeers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, critical)
		x := c.agent("x", true)
		disabled := c.agents(false, "u1", "u2", "u3")
		rest := c.agents(true, "w1", "w2")
		warmUp()
		segment(c, append([]*node{x}, disabled...), rest)
		time.Sleep(critical.params.ControlPlaneLostAfter + critical.watchdogTimeout + 3*s)

		fired(t, x)
		healthy(t, rest...)
		for _, n := range disabled {
			if q := len(c.questions(n.name)); q != 0 {
				t.Errorf("%s without watchdog asked %d questions, want none", n.name, q)
			}
		}
		c.noDeclareSuicide()
	})
}

// The proxy is unavailable for 4–6 s. That is not a CP loss: the source
// switches to self probes and keeps the CP visible; the decision does not
// start a poll.
func TestProxyUnavailable(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, critical)
		nodes := c.agents(true, "x", "w1", "w2")
		x := nodes[0]
		warmUp()
		x.cp.source.Store(domain.CPSourceSelf)
		time.Sleep(5 * s)
		x.cp.source.Store(domain.CPSourceProxy)
		time.Sleep(2 * s)
		if x.metrics.pollCount() != 0 {
			t.Errorf("poll started while the proxy is unavailable")
		}
		healthy(t, nodes...)
	})
}

// The numbers of profile normal: CP lost after 9.1 s and two reads, and the
// stop at the loss.
func TestNormalProfileNumbers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCluster(t, normal)
		seg := c.agents(true, "x", "w1")
		rest := c.agents(true, "w2")
		warmUp()
		x := seg[0]
		t0 := time.Now()
		segment(c, seg, rest)
		time.Sleep(normal.params.ControlPlaneLostAfter + 3*s)
		if d := x.wd.stopped().Sub(t0); d < normal.params.ControlPlaneLostAfter-probeInterval || d > normal.params.ControlPlaneLostAfter+200*ms {
			t.Errorf("feed stopped %v after the failure, want controlPlaneLostAfter = 9.1 s and two reads", d)
		}
	})
}
