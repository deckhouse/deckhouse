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

package cpview

import (
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"fencing-agent/internal/adapters/proxyclient"
	"fencing-agent/internal/domain"
)

// The changes of the fake node happen 50 ms after a read, so they never
// happen at the same fake time as a read.
const offset = 50 * time.Millisecond

var apis = []string{"10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"}

// At least one upstream is ready.
func TestVisibleWhenOneUpstreamReady(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := &fakeNode{apis: apis, upstreams: func(now time.Time) proxyclient.Upstreams {
			// Two upstreams are not ready and have no lastReadyAgoMs.
			return proxyclient.Upstreams{
				HasProbe:     true,
				ProbeSeq:     uint64(now.UnixMilli()),
				ProbeOK:      true,
				LastReadyAgo: []time.Duration{800 * time.Millisecond},
			}
		}}
		h := start(f, critControlPlaneLostAfter, critFastReadAfter)

		time.Sleep(2 * time.Second)
		h.stop()

		for _, st := range h.all() {
			if !st.Visible || !st.Seen || st.Source != domain.CPSourceProxy {
				t.Fatalf("want a visible control plane from the proxy, got %+v", st)
			}
		}

		if probes, _ := f.calls(); len(probes) != 0 {
			t.Fatalf("own probes must not run with the data of the proxy: %d probes", len(probes))
		}
	})
}

// API access is lost: the control plane is lost after two reads beyond controlPlaneLostAfter,
// and the reads come at the read interval of each profile.
func TestLostAfterTwoReadsBeyondW(t *testing.T) {
	for _, p := range profiles {
		t.Run(p.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := &fakeNode{proxy: proxyNew, cpUp: true, apis: apis}
				h := start(f, p.controlPlaneLostAfter, p.fastReadAfter)

				time.Sleep(time.Second + offset)
				f.set(func(f *fakeNode) { f.cpUp = false })
				time.Sleep(p.controlPlaneLostAfter + 2*time.Second)
				h.stop()

				states := h.all()
				assertLostAfterTwoReads(t, states, p.controlPlaneLostAfter)
				assertReadIntervals(t, states, p.fastReadAfter)
			})
		})
	}
}

// etcd without quorum (nobodyReady trap). proxyclient.Upstreams
// has no isHealthy, score and tier (see TestDecode): only lastReadyAgoMs counts.
func TestEtcdWithoutQuorum(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		t0 := time.Now()
		f := &fakeNode{upstreams: func(now time.Time) proxyclient.Upstreams {
			ago := now.Sub(t0) + 3*time.Second

			return proxyclient.Upstreams{
				HasProbe:     true,
				ProbeSeq:     uint64(now.UnixMilli()),
				LastReadyAgo: []time.Duration{ago, ago + time.Second, ago + 2*time.Second},
			}
		}}
		h := start(f, critControlPlaneLostAfter, critFastReadAfter)

		time.Sleep(2 * time.Second)
		h.stop()

		assertLostAfterTwoReads(t, h.all(), critControlPlaneLostAfter)
	})
}

// One failed read does not mean a CP loss.
func TestOneReadBeyondWIsNotLoss(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var lastReady time.Time

		misses := 0
		f := &fakeNode{upstreams: func(now time.Time) proxyclient.Upstreams {
			if lastReady.IsZero() {
				lastReady = now
			}

			ago := now.Sub(lastReady)
			if ago > critControlPlaneLostAfter {
				// The next read sees a ready upstream again.
				lastReady = now
				misses++
			}

			return proxyclient.Upstreams{HasProbe: true, ProbeSeq: uint64(now.UnixMilli()), LastReadyAgo: []time.Duration{ago}}
		}}
		h := start(f, critControlPlaneLostAfter, critFastReadAfter)

		time.Sleep(8 * time.Second)
		h.stop()

		if misses == 0 {
			t.Fatal("no read was beyond controlPlaneLostAfter")
		}

		assertAlwaysVisible(t, h.all())
	})
}

// Seen is false until the first ready answer and then stays true.
func TestSeen(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := &fakeNode{proxy: proxyNew, apis: apis}
		h := start(f, critControlPlaneLostAfter, critFastReadAfter)

		time.Sleep(time.Second + offset)

		if st := h.last(); st.Seen || st.Visible || st.Source != domain.CPSourceProxy {
			t.Fatalf("before the first ready answer: %+v", st)
		}

		f.set(func(f *fakeNode) { f.cpUp = true })
		time.Sleep(time.Second)

		if st := h.last(); !st.Seen || !st.Visible {
			t.Fatalf("after a ready answer: %+v", st)
		}

		f.set(func(f *fakeNode) { f.cpUp = false })
		time.Sleep(critControlPlaneLostAfter + time.Second)
		h.stop()

		if st := h.last(); !st.Seen || st.Visible {
			t.Fatalf("after the loss: %+v", st)
		}
	})
}

// Old proxy answers: the own probes go through the local proxy.
func TestOldProxyGivesSelfThroughProxy(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := &fakeNode{proxy: proxyOld, cpUp: true, apis: apis}
		h := start(f, critControlPlaneLostAfter, critFastReadAfter)

		time.Sleep(3 * time.Second)
		h.stop()

		states := h.all()
		assertAlwaysVisible(t, states)
		assertSeqGrows(t, states)

		for _, st := range states {
			if st.Source != domain.CPSourceSelf {
				t.Fatalf("want cpSource=self, got %+v", st)
			}
		}

		if got := h.status.list(); !slices.Equal(got, []domain.CPSource{domain.CPSourceSelf}) {
			t.Fatalf("published sources = %v", got)
		}

		probes, _ := f.calls()
		if len(probes) == 0 {
			t.Fatal("no own probes")
		}

		for _, c := range probes {
			if c.addr != proxyclient.ProxyAddr {
				t.Fatalf("probe of %s, want only the local proxy", c.addr)
			}
		}
	})
}

// Deckhouse Engine node: no proxy, the own probes go to the
// apiservers from the EndpointSlice.
func TestEngineProbesAPIServers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := &fakeNode{proxy: proxyDown, cpUp: true, apis: apis}
		h := start(f, critControlPlaneLostAfter, critFastReadAfter)

		time.Sleep(3 * time.Second)
		h.stop()

		states := h.all()
		assertAlwaysVisible(t, states)

		if got := h.status.list(); !slices.Equal(got, []domain.CPSource{domain.CPSourceSelf}) {
			t.Fatalf("published sources = %v", got)
		}

		assertProbed(t, f, apis)

		// The number of an own round is the boot clock at its start.
		_, rounds := f.calls()
		starts := make([]uint64, 0, len(rounds))

		for _, r := range rounds {
			starts = append(starts, uint64(r.UnixMilli()))
		}

		for _, st := range states {
			if st.ProbeSeq != 0 && !slices.Contains(starts, st.ProbeSeq) {
				t.Fatalf("ProbeSeq %d is not the start of a round", st.ProbeSeq)
			}
		}
	})
}

// The proxy on the master is unavailable. The own check on the master does
// not use the own apiserver and the local proxy as targets.
func TestMasterSkipsOwnAPIServer(t *testing.T) {
	for _, mode := range []proxyMode{proxyOld, proxyDown} {
		synctest.Test(t, func(t *testing.T) {
			f := &fakeNode{proxy: mode, cpUp: true, apis: apis, own: []string{"10.0.0.1", "203.0.113.1"}}
			h := start(f, critControlPlaneLostAfter, critFastReadAfter)

			time.Sleep(3 * time.Second)
			h.stop()

			assertAlwaysVisible(t, h.all())
			assertProbed(t, f, apis[1:])
		})
	}
}

// Single master: the EndpointSlice has only the
// own apiserver. The own probes go to it directly, not through the proxy, and
// the control plane stays visible while the proxy is unavailable longer than controlPlaneLostAfter.
func TestSingleMasterProbesOwnAPIServer(t *testing.T) {
	for _, mode := range []proxyMode{proxyOld, proxyDown} {
		synctest.Test(t, func(t *testing.T) {
			f := &fakeNode{proxy: mode, cpUp: true, apis: apis[:1], own: []string{"10.0.0.1", "203.0.113.1"}}
			h := start(f, critControlPlaneLostAfter, critFastReadAfter)

			time.Sleep(2 * critControlPlaneLostAfter)
			h.stop()

			assertAlwaysVisible(t, h.all())
			assertProbed(t, f, apis[:1])
		})
	}
}

// The proxy restarts for longer than controlPlaneLostAfter. The own check does not run in
// parallel with the proxy. The first own round gets its answers after two
// reads.
func TestProxyRestartKeepsControlPlane(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := &fakeNode{proxy: proxyNew, cpUp: true, apis: apis, latency: 300 * time.Millisecond}
		h := start(f, critControlPlaneLostAfter, critFastReadAfter)

		time.Sleep(time.Second + offset)
		down := time.Now()
		f.set(func(f *fakeNode) { f.proxy = proxyDown })

		time.Sleep(5 * time.Second)
		f.set(func(f *fakeNode) { f.proxy, f.restarted = proxyNew, true })

		time.Sleep(2 * time.Second)
		h.stop()

		states := h.all()
		assertAlwaysVisible(t, states)
		assertSeqGrows(t, states)

		want := []domain.CPSource{domain.CPSourceProxy, domain.CPSourceSelf, domain.CPSourceProxy}
		if got := h.status.list(); !slices.Equal(got, want) {
			t.Fatalf("published sources = %v, want %v", got, want)
		}

		assertProbesOnlyInSelf(t, f, states, down)
	})
}

// When the own check starts: reads of the
// proxy that fail for less than proxyLostAfter do not start the own probes and
// do not change cpSource.
func TestShortProxyFailureKeepsProxySource(t *testing.T) {
	for _, mode := range []proxyMode{proxyDown, proxyHung} {
		synctest.Test(t, func(t *testing.T) {
			f := &fakeNode{proxy: proxyNew, cpUp: true, apis: apis}
			h := start(f, critControlPlaneLostAfter, critFastReadAfter)

			time.Sleep(time.Second + offset)
			f.set(func(f *fakeNode) { f.proxy = mode })

			time.Sleep(proxyLostAfter - 2*readInterval)
			f.set(func(f *fakeNode) { f.proxy = proxyNew })

			time.Sleep(2 * time.Second)
			h.stop()

			assertAlwaysVisible(t, h.all())

			if got := h.status.list(); !slices.Equal(got, []domain.CPSource{domain.CPSourceProxy}) {
				t.Fatalf("published sources = %v, want only proxy", got)
			}

			if probes, _ := f.calls(); len(probes) != 0 {
				t.Fatalf("own probes after a short failure of the proxy: %d probes", len(probes))
			}
		})
	}
}

// A proxy that does not answer does not move the reads: they start each 200
// ms, and each 100 ms while lastReadyAgo > fastReadAfter. The control plane is lost
// no later than controlPlaneLostAfter + 0.2 s after the last ready answer of the own probes.
func TestHungProxyKeepsTheReadPeriod(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := &fakeNode{proxy: proxyHung, cpUp: true, apis: apis}
		h := start(f, critControlPlaneLostAfter, critFastReadAfter)

		time.Sleep(10*time.Second + offset)
		down := time.Now()
		f.set(func(f *fakeNode) { f.cpUp = false })

		time.Sleep(critControlPlaneLostAfter + time.Second)
		h.stop()

		states := h.all()
		for i := 1; i < len(states); i++ {
			want := readInterval
			if states[i-1].LastReadyAgo > critFastReadAfter {
				want = fastReadInterval
			}

			if got := states[i].ReadAt.Sub(states[i-1].ReadAt); got != want {
				t.Fatalf("read at %v: %v after the previous read, want %v", states[i].ReadAt, got, want)
			}
		}

		var before []domain.CPState
		var lostAt time.Time
		for _, st := range states {
			if st.ReadAt.Before(down) {
				before = append(before, st)
			} else if !st.Visible && lostAt.IsZero() {
				lostAt = st.ReadAt
			}
		}

		assertAlwaysVisible(t, before)

		if lostAt.IsZero() || lostAt.Sub(down) > critControlPlaneLostAfter+200*time.Millisecond {
			t.Fatalf("control plane lost %v after the last ready answer, want no more than controlPlaneLostAfter + 0.2 s", lostAt.Sub(down))
		}
	})
}

// bashible updated the proxy.
func TestNewProxyReplacesSelf(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := &fakeNode{proxy: proxyOld, cpUp: true, apis: apis}
		h := start(f, critControlPlaneLostAfter, critFastReadAfter)

		time.Sleep(2*time.Second + offset)
		f.set(func(f *fakeNode) { f.proxy = proxyNew })

		time.Sleep(2 * time.Second)
		h.stop()

		states := h.all()
		assertAlwaysVisible(t, states)
		assertSeqGrows(t, states)

		want := []domain.CPSource{domain.CPSourceSelf, domain.CPSourceProxy}
		if got := h.status.list(); !slices.Equal(got, want) {
			t.Fatalf("published sources = %v, want %v", got, want)
		}

		assertProbesOnlyInSelf(t, f, states, time.Time{})
	})
}

// The fields disappeared.
func TestFieldsGoneGivesSelf(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := &fakeNode{proxy: proxyNew, cpUp: true, apis: apis}
		h := start(f, critControlPlaneLostAfter, critFastReadAfter)

		time.Sleep(2*time.Second + offset)
		gone := time.Now()
		f.set(func(f *fakeNode) { f.proxy = proxyOld })

		time.Sleep(2 * time.Second)
		h.stop()

		states := h.all()
		assertAlwaysVisible(t, states)
		assertSeqGrows(t, states)

		want := []domain.CPSource{domain.CPSourceProxy, domain.CPSourceSelf}
		if got := h.status.list(); !slices.Equal(got, want) {
			t.Fatalf("published sources = %v, want %v", got, want)
		}

		assertProbesOnlyInSelf(t, f, states, gone)
	})
}

// The own probes use the default parameters of the proxy with any profile:
// interval probeInterval ±probeJitter, timeout probeTimeout, fail-step
// interval failStep ±probeJitter only in the first failStepWindow of a
// failure episode.
func TestOwnProbeTimings(t *testing.T) {
	for _, p := range profiles {
		t.Run(p.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := &fakeNode{proxy: proxyDown, cpUp: true, hang: true, apis: apis}
				h := start(f, p.controlPlaneLostAfter, p.fastReadAfter)

				time.Sleep(3*time.Second + offset)
				down := time.Now()
				f.set(func(f *fakeNode) { f.cpUp = false })

				time.Sleep(40 * time.Second)
				stop := time.Now()
				h.stop()

				probes, rounds := f.calls()

				for _, c := range probes {
					// The stop cancels the last probes.
					if c.start.After(down) && c.end.Before(stop) && c.end.Sub(c.start) != probeTimeout {
						t.Fatalf("probe at %v took %v, want %v", c.start, c.end.Sub(c.start), probeTimeout)
					}
				}

				// The first failed round starts up to imax after the loss and
				// ends probeTimeout after its start.
				imax := jittered(probeInterval, 1)
				failSince := down.Add(probeTimeout)
				failStepCount := 0

				for i := 1; i < len(rounds)-1; i++ {
					at, d := rounds[i], rounds[i+1].Sub(rounds[i])

					switch {
					case at.Before(down):
						if !between(d, jittered(probeInterval, -1), imax) {
							t.Fatalf("round at %v: interval %v, want %v ±%v", at, d, probeInterval, probeJitter)
						}
					case at.After(failSince.Add(imax)) && at.Before(failSince.Add(failStepWindow-time.Second)):
						if !between(d, jittered(failStep, -1), jittered(failStep, 1)) {
							t.Fatalf("round at %v: interval %v, want fail-step %v ±%v", at, d, failStep, probeJitter)
						}
						failStepCount++
					case at.After(failSince.Add(failStepWindow + imax)):
						if !between(d, jittered(probeInterval, -1), imax) {
							t.Fatalf("round at %v: interval %v after the fail-step window, want %v ±%v", at, d, probeInterval, probeJitter)
						}
					}
				}

				if failStepCount == 0 {
					t.Fatal("no fail-step rounds")
				}
			})
		})
	}
}

// jittered returns the interval at a jitter limit: spread is -1 or 1.
func jittered(base time.Duration, spread float64) time.Duration {
	return time.Duration((1 + spread*probeJitter) * float64(base))
}

// A round sends one probe to each target, in fail-step and in the normal
// mode after the fail-step window.
func TestOwnProbeOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := &fakeNode{proxy: proxyDown, apis: apis[:1]}
		h := start(f, critControlPlaneLostAfter, critFastReadAfter)

		time.Sleep(failStepWindow + 8*time.Second)
		h.stop()

		probes, rounds := f.calls()

		// The first round fails at once and starts the failure episode.
		t0 := rounds[0]
		failSteps, normal := 0, 0

		for _, r := range rounds[:len(rounds)-1] {
			n := 0

			for _, c := range probes {
				if c.start.Equal(r) {
					n++
				}
			}

			if n != 1 {
				t.Fatalf("round at %v: %d probes, want 1", r, n)
			}

			switch {
			case r.Before(t0.Add(failStepWindow)):
				failSteps++
			case r.After(t0.Add(failStepWindow + jittered(probeInterval, 1))):
				normal++
			}
		}

		if len(probes) > len(rounds) {
			t.Fatalf("probes = %d, rounds = %d, want one probe per round", len(probes), len(rounds))
		}

		if failSteps < 2 || normal < 2 {
			t.Fatalf("fail-step rounds: %d, normal rounds: %d", failSteps, normal)
		}
	})
}

func assertLostAfterTwoReads(t *testing.T, states []domain.CPState, controlPlaneLostAfter time.Duration) {
	t.Helper()

	lost := slices.IndexFunc(states, func(st domain.CPState) bool { return !st.Visible })
	if lost < 2 {
		t.Fatalf("want the control plane visible first and lost later, lost at read %d", lost)
	}

	if a, b, c := states[lost-2], states[lost-1], states[lost]; a.LastReadyAgo > controlPlaneLostAfter || b.LastReadyAgo <= controlPlaneLostAfter || c.LastReadyAgo <= controlPlaneLostAfter {
		t.Fatalf("want the loss on the second read beyond controlPlaneLostAfter=%v: lastReadyAgo %v, %v, %v", controlPlaneLostAfter, a.LastReadyAgo, b.LastReadyAgo, c.LastReadyAgo)
	}

	for _, st := range states[lost:] {
		if st.Visible {
			t.Fatalf("control plane visible again at %v", st.ReadAt)
		}
	}
}

// assertReadIntervals checks the read every 200 ms, and every 100 ms while
// lastReadyAgo > fastReadAfter.
func assertReadIntervals(t *testing.T, states []domain.CPState, fastReadAfter time.Duration) {
	t.Helper()

	fast := 0

	for i := 1; i < len(states); i++ {
		want := readInterval
		if states[i-1].LastReadyAgo > fastReadAfter {
			want = fastReadInterval
			fast++
		}

		if d := states[i].ReadAt.Sub(states[i-1].ReadAt); d != want {
			t.Fatalf("read at %v: interval %v, want %v (lastReadyAgo %v, fastReadAfter %v)", states[i].ReadAt, d, want, states[i-1].LastReadyAgo, fastReadAfter)
		}
	}

	if fast == 0 {
		t.Fatal("no reads with lastReadyAgo > fastReadAfter")
	}
}

// assertProbed checks that the own probes went to all of want and nowhere else.
func assertProbed(t *testing.T, f *fakeNode, want []string) {
	t.Helper()

	probes, _ := f.calls()
	got := map[string]bool{}

	for _, c := range probes {
		if !slices.Contains(want, c.addr) {
			t.Fatalf("probe of %s, want only %v", c.addr, want)
		}

		got[c.addr] = true
	}

	if len(got) != len(want) {
		t.Fatalf("probed %v, want all of %v", got, want)
	}
}

// assertProbesOnlyInSelf checks that the own probes run only after from and
// stop before the first read of the proxy after the own probes.
func assertProbesOnlyInSelf(t *testing.T, f *fakeNode, states []domain.CPState, from time.Time) {
	t.Helper()

	self := slices.IndexFunc(states, func(st domain.CPState) bool { return st.Source == domain.CPSourceSelf })
	if self < 0 {
		t.Fatal("no cpSource=self")
	}

	end := time.Time{}
	if back := slices.IndexFunc(states[self:], func(st domain.CPState) bool { return st.Source == domain.CPSourceProxy }); back >= 0 {
		end = states[self+back].ReadAt
	}

	probes, _ := f.calls()
	if len(probes) == 0 {
		t.Fatal("no own probes")
	}

	for _, c := range probes {
		if c.start.Before(from) || !end.IsZero() && c.end.After(end) {
			t.Fatalf("probe %v–%v outside cpSource=self (%v–%v)", c.start, c.end, from, end)
		}
	}
}
