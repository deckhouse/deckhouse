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
	"maps"
	"slices"
	"testing"
	"time"
)

type results = map[string]probeResult

const (
	ms             = time.Millisecond
	discoverPeriod = 5
	simTolerance   = 0.1
	simTotalSlack  = 2.0
)

var (
	soft = probeResult{kind: probeSoft}
	hard = probeResult{kind: probeHard}
)

func ready(lat time.Duration) probeResult {
	return probeResult{kind: probeReady, latency: lat}
}

func notReady(lat time.Duration) probeResult {
	return probeResult{kind: probeNotReady, latency: lat}
}

func every(r probeResult, addrs ...string) results {
	out := make(results, len(addrs))
	for _, addr := range addrs {
		out[addr] = r
	}

	return out
}

func merge(parts ...results) results {
	out := results{}
	for _, part := range parts {
		maps.Copy(out, part)
	}

	return out
}

func warm(nodes []string, lat time.Duration, n int) step {
	return step{note: "warm", probes: every(ready(lat), nodes...), rounds: n}
}

func discovery(nodes []string, steps []step) []step {
	out := make([]step, 0, len(steps))
	slice := nodes

	for _, s := range steps {
		if s.slice != nil {
			slice = s.slice
		}

		for range max(s.rounds, 1) {
			r := s
			r.rounds, r.slice = 1, nil

			if (len(out)+1)%discoverPeriod == 0 {
				r.slice = slice
			}

			out = append(out, r)
		}
	}

	return out
}

type simScenario struct {
	name       string
	nodes      []string
	steps      []step
	want       float64
	hung       float64
	noGood     float64
	wait       float64
	falseKills int
}

func simScenarios() []simScenario {
	return slices.Concat(simBasic(), simHunt3m(), simPort3m(), simHunt5m(), simPort5m(), simHuntMDC(), simHunt3MDC(), simPortMDC(), simHunt4())
}

func simBasic() []simScenario {
	m3 := []string{"m0", "m1", "m2"}
	m1 := []string{"m1"}
	single := []string{"m0"}

	return []simScenario{
		{
			name: "A. Yuriy: m1 shuts down (500), m0/m2 time out 3 rounds", nodes: m3,
			steps: []step{
				warm(m3, ms, 3),
				{note: "m1 500, others ready", probes: results{"m0": ready(ms), "m1": notReady(ms), "m2": ready(ms)}, bad: m1},
				{note: "m1 500, others timeout", probes: results{"m0": soft, "m1": notReady(ms), "m2": soft}, bad: m1, rounds: 3},
				{note: "m1 500, others ready", probes: results{"m0": ready(ms), "m1": notReady(ms), "m2": ready(ms)}, bad: m1},
			},
		},
		{
			name: "C. etcd stall, everyone healthy (all fast 500)", nodes: m3,
			steps: []step{
				warm(m3, ms, 3),
				{note: "all 500", probes: every(notReady(ms), m3...), rounds: 6},
			},
		},
		{
			name: "3m S2: etcd stall, m0 LA-sick: cached 200 2 rounds while m1/m2 fast 500, then m0 timeout", nodes: m3,
			steps: []step{
				warm(m3, ms, 3),
				{note: "m0 cached 200", probes: results{"m0": ready(ms), "m1": notReady(ms), "m2": notReady(ms)}, bad: m3, rounds: 2},
				{note: "m0 timeout, m1/m2 500", probes: results{"m0": soft, "m1": notReady(ms), "m2": notReady(ms)}, bad: single, rounds: 10},
			},
			want: 2, noGood: 2,
		},
		{
			name: "1m-S1 single master graceful restart: 15s 500, 8s refused+Down, 30s warm-up 500", nodes: single,
			steps: []step{
				warm(single, ms, 3),
				{note: "shutdown-delay 500", probes: results{"m0": notReady(ms)}, bad: single, rounds: 15},
				{note: "process down, dials refused", probes: results{"m0": hard}, downs: single, bad: single, rounds: 8},
				{note: "warm-up fast 500", probes: results{"m0": notReady(2 * ms)}, bad: single, rounds: 30},
				{note: "ready", probes: results{"m0": ready(ms)}, rounds: 5},
			},
			want: 53, hung: 8, noGood: 53,
		},
	}
}

func simHunt3m() []simScenario {
	m3 := []string{"m0", "m1", "m2"}
	m4 := []string{"m0", "m1", "m2", "m3"}
	m0, m1, m2 := []string{"m0"}, []string{"m1"}, []string{"m2"}
	m01, m12, m23 := []string{"m0", "m1"}, []string{"m1", "m2"}, []string{"m2", "m3"}
	slow := ready(1500 * ms)

	return []simScenario{
		{
			name: "H1: m1 restart (warm-up 500) + m2 local etcd OOM-killed (fast 500), m0 absorbs storm, slow 200 1500ms", nodes: m3,
			steps: []step{
				warm(m3, ms, 3),
				{note: "m1 graceful shutdown 500", probes: results{"m0": ready(ms), "m1": notReady(ms), "m2": ready(ms)}, bad: m1},
				{note: "m1 listener closed, storm", slice: []string{"m0", "m2"}, probes: results{"m0": slow, "m1": hard, "m2": slow}, downs: []string{"m1", "m1"}, bad: m1, rounds: 2},
				{note: "m2 etcd OOM: fast 500", slice: []string{"m0", "m2"}, probes: results{"m0": slow, "m1": hard, "m2": notReady(ms)}, bad: m12, rounds: 2},
				{note: "m1 back as NEW, warming", slice: m3, probes: results{"m0": slow, "m1": notReady(ms), "m2": notReady(ms)}, bad: m12, rounds: 10},
				{note: "m2 etcd back", slice: m3, probes: results{"m0": ready(30 * ms), "m1": notReady(ms), "m2": ready(ms)}, bad: m1, rounds: 4},
			},
		},
		{
			name: "H3-4: m1 fast 500 (broken etcd path), m0 hung (livelock), m2 slow-alive 1.5s", nodes: m3,
			steps: []step{
				warm(m3, ms, 3),
				{note: "m1 etcd path broken", probes: results{"m0": ready(ms), "m1": notReady(ms), "m2": ready(ms)}, bad: m1},
				{note: "m0 hung, m2 slow 200, m1 500", probes: results{"m0": soft, "m1": notReady(ms), "m2": slow}, bad: m01, rounds: 15},
				{note: "m0 OOM-killed (refused)", probes: results{"m0": hard, "m1": notReady(ms), "m2": slow}, downs: m0, bad: m01, rounds: 3},
			},
			wait: 0.2,
		},
		{
			name: "H3-3: 4 entries, member add without remove: NEW m3 late 200 then hang; m0/m1 fast 500; m2 dead", nodes: m3,
			steps: []step{
				warm(m3, ms, 3),
				{note: "m2 VM deleted: warm conn hangs", probes: results{"m0": ready(ms), "m1": ready(ms), "m2": soft}, bad: m2, rounds: 2},
				{note: "m3 added (NEW), late 200; quorum lost: m0/m1 fast 500", slice: m4, probes: results{"m0": notReady(ms), "m1": notReady(ms), "m2": soft, "m3": slow}, downs: m2, bad: m23},
				{note: "m3 hangs; m0/m1 fast 500; m2 dials time out", probes: results{"m0": notReady(ms), "m1": notReady(ms), "m2": soft, "m3": soft}, downs: m2, bad: m23, rounds: 12},
				{note: "m2 member removed: quorum back", probes: results{"m0": ready(ms), "m1": ready(ms), "m2": hard, "m3": soft}, downs: m2, bad: m23, rounds: 3},
			},
			want: 1, wait: 0.68,
		},
	}
}

func restart3m(nodes []string, m string, down, storm, warmup, back int) []step {
	others := slices.DeleteFunc(slices.Clone(nodes), func(a string) bool { return a == m })
	mk := func(mine, theirs probeResult) results {
		return merge(every(theirs, others...), results{m: mine})
	}
	bad := []string{m}

	out := []step{{note: m + " graceful 500, endpoint removed", slice: others, probes: mk(notReady(ms), ready(ms)), bad: bad, rounds: 2}}
	for i := range down + warmup {
		s := step{note: m + " warming 500 (out of slice)", probes: mk(notReady(ms), ready(20*ms)), bad: bad}
		if i < down {
			s.note, s.downs, s.probes = m+" down (refused)", []string{m, m}, mk(hard, ready(ms))
		}

		if i < storm {
			s.note += ", storm"
			s.probes = mk(s.probes[m], ready(1500*ms))
		}

		out = append(out, s)
	}

	return append(out, step{note: m + " ready, back in slice", slice: nodes, probes: mk(ready(ms), ready(ms)), rounds: back})
}

func simPort3m() []simScenario {
	m3 := []string{"m0", "m1", "m2"}

	full := []step{warm(m3, ms, 3)}
	for _, m := range m3 {
		full = append(full, restart3m(m3, m, 5, 3, 15, 3)...)
	}

	return []simScenario{
		{
			name:  "3m-S1 rolling restart of m1, storm on m0/m2, warming m1 out of slice",
			nodes: m3, steps: discovery(m3, append([]step{warm(m3, ms, 3)}, restart3m(m3, "m1", 4, 8, 14, 3)...)),
		},
		{name: "3m-S8 full rolling restart m0->m1->m2", nodes: m3, steps: discovery(m3, full)},
	}
}

func simHunt5m() []simScenario {
	m3 := []string{"m0", "m1", "m2"}
	m5 := []string{"m0", "m1", "m2", "m3", "m4"}
	m4 := []string{"m0", "m1", "m2", "m3"}
	w := []string{"m0", "m1", "m2"}
	m12, m23 := []string{"m1", "m2"}, []string{"m2", "m3"}
	survivors := every(ready(2500*ms), "m3", "m4")
	slow := ready(2500 * ms)

	return []simScenario{
		{
			name: "H2 3m cascading OOM: m1/m2 warming fast 500, m0 slow alive 200 after 2.5s", nodes: m3,
			steps: []step{
				warm(m3, ms, 5),
				{note: "m1/m2 OOM-killed: refused", probes: results{"m0": slow, "m1": hard, "m2": hard}, bad: m12, rounds: 2},
				{note: "m1/m2 back, first readyz slow", probes: results{"m0": slow, "m1": soft, "m2": soft}, bad: m12},
				{note: "m1/m2 warming, m0 slow", probes: results{"m0": slow, "m1": notReady(ms), "m2": notReady(ms)}, bad: m12, rounds: 15},
				{note: "m1 warm, m2 warming, m0 slow", probes: results{"m0": slow, "m1": ready(ms), "m2": notReady(ms)}, bad: []string{"m2"}, rounds: 5},
				{note: "all warm", probes: every(ready(ms), m3...), rounds: 3},
			},
			want: 0.01, hung: 0.01, wait: 0.03,
		},
		{
			name: "H6 5m: 3 apiservers OOM -> warming fast 500 (majority), 2 survivors 200 after 2.5s", nodes: m5,
			steps: []step{
				warm(m5, ms, 3),
				{note: "OOM restart: refused", probes: merge(every(hard, w...), survivors), bad: w, rounds: 2},
				{note: "warming 500", probes: merge(every(notReady(ms), w...), survivors), bad: w, rounds: 15},
			},
		},
		{
			name: "H3-5m-4: 4 entries, member add with a dying m2: quorum lost, m2 late 200 then hang, m3 warming", nodes: m3,
			steps: []step{
				{note: "warm", probes: merge(every(ready(ms), "m0", "m1"), results{"m2": ready(200 * ms)}), rounds: 3},
				{note: "m3 joins, cached 200", slice: m4, probes: merge(every(ready(ms), "m0", "m1"), results{"m2": ready(200 * ms), "m3": notReady(ms)}), bad: []string{"m3"}},
				{note: "quorum lost, m2 cached 200 after 2.5s", slice: m4, probes: merge(every(notReady(ms), "m0", "m1", "m3"), results{"m2": slow}), bad: m23},
				{note: "quorum lost, m2 hung", slice: m4, probes: merge(every(notReady(ms), "m0", "m1", "m3"), results{"m2": soft}), bad: m23, rounds: 14},
				{note: "quorum back, m3 warming", slice: m4, probes: merge(every(ready(ms), "m0", "m1"), results{"m2": soft, "m3": notReady(ms)}), bad: m23, rounds: 8},
			},
		},
	}
}

func simPort5m() []simScenario {
	m5 := []string{"m0", "m1", "m2", "m3", "m4"}
	m7 := []string{"m0", "m1", "m2", "m3", "m4", "m5", "m6"}
	rest := m7[1:]
	m0, m01 := []string{"m0"}, []string{"m0", "m1"}
	storm7 := func(over results) results {
		return merge(results{"m1": ready(1200 * ms), "m2": ready(1500 * ms), "m3": ready(900 * ms), "m4": ready(2000 * ms), "m5": ready(1500 * ms), "m6": ready(2500 * ms)}, over)
	}

	dcb := results{"m2": hard, "m3": hard, "m4": hard}
	broken := results{"m3": notReady(2 * ms), "m4": notReady(2 * ms)}
	busy := results{"m0": ready(1500 * ms), "m1": ready(1800 * ms)}
	cascadeA := merge(busy, broken, results{"m2": notReady(300 * ms)})
	cascadeB := merge(busy, broken, results{"m2": notReady(2200 * ms)})
	m34 := []string{"m3", "m4"}

	return []simScenario{
		{
			name: "5m-S3b m0 restart storm, m1 loses etcd inside the storm", nodes: m7,
			steps: discovery(m7, []step{
				warm(m7, ms, 3),
				{note: "m0 shutdown 500", slice: rest, probes: merge(every(ready(ms), m7...), results{"m0": notReady(ms)}), bad: m0, rounds: 4},
				{note: "m0 exited (refused); storm", slice: rest, probes: storm7(results{"m0": hard}), bad: m0, rounds: 2},
				{note: "storm; m0 back warming", slice: m7, probes: storm7(results{"m0": notReady(ms)}), bad: m0, rounds: 2},
				{note: "storm; m1 etcd broken", slice: m7, probes: storm7(results{"m0": notReady(ms), "m1": notReady(2 * ms)}), bad: m01, rounds: 8},
				{note: "storm over; m1 still broken", slice: m7, probes: merge(every(ready(5*ms), m7...), results{"m0": notReady(ms), "m1": notReady(2 * ms)}), bad: m01, rounds: 3},
			}),
		},
		{
			name: "5m-S4 majority loss, 3 of 5 lose power", nodes: m5,
			steps: discovery(m5, []step{
				warm(m5, 2*ms, 3),
				{note: "m0,m1 cached 200; DC-B down", probes: merge(every(ready(2*ms), m5...), dcb), bad: m5, rounds: 2},
				{note: "m0,m1 500; DC-B down", probes: merge(every(notReady(2*ms), m5...), dcb), bad: m5, rounds: 15},
				{note: "DC-B up warming; m0,m1 still 500", probes: every(notReady(2*ms), m5...), bad: m5, rounds: 3},
				{note: "m0 ok; m1 cached 500; DC-B warming", probes: merge(every(notReady(2*ms), m5...), results{"m0": ready(2 * ms)}), bad: m5[1:], rounds: 2},
				{note: "m0,m1 ok; DC-B warming", probes: merge(every(notReady(2*ms), m5...), results{"m0": ready(2 * ms), "m1": ready(2 * ms)}), bad: m5[2:], rounds: 20},
				{note: "all ready", probes: every(ready(2*ms), m5...), rounds: 3},
			}),
			want: 20, noGood: 20,
		},
		{
			name: "5m-S2 minority etcd loss + cascade, m2 slow/late 500", nodes: m5,
			steps: discovery(m5, []step{
				warm(m5, 3*ms, 3),
				{note: "m3,m4 first etcd ping: late 500", probes: merge(every(ready(5*ms), m5...), every(notReady(2200*ms), m34...)), bad: m34},
				{note: "m3,m4 fast 500, others ok", probes: merge(every(ready(5*ms), m5...), broken), bad: m34},
				{note: "cascade A: m2 500 300ms", probes: cascadeA, bad: m34},
				{note: "cascade B: m2 late 500", probes: cascadeB, bad: m34},
				{note: "cascade A: m2 500 300ms", probes: cascadeA, bad: m34},
				{note: "cascade B: m2 late 500", probes: cascadeB, bad: m34},
				{note: "cascade A: m2 500 300ms", probes: cascadeA, bad: m34, rounds: 2},
				{note: "cascade B: m2 late 500", probes: cascadeB, bad: m34, rounds: 2},
				{note: "survivors recover", probes: merge(every(ready(20*ms), m5...), broken), bad: m34, rounds: 4},
				{note: "m3,m4 lease expired, left slice", slice: m5[:3], probes: merge(every(ready(5*ms), m5...), broken), bad: m34, rounds: 6},
			}),
			want: 4,
		},
	}
}

func simHuntMDC() []simScenario {
	m221 := []string{"a0", "a1", "b0", "b1", "c0"}
	m32 := []string{"a0", "a1", "a2", "b0", "b1"}
	m21 := []string{"a0", "a1", "c0"}
	remote4 := []string{"a0", "a1", "b0", "b1"}
	b01, c0, c0a1 := []string{"b0", "b1"}, []string{"c0"}, []string{"c0", "a1"}
	hang := []string{"b0", "b1", "c0"}
	noA1 := []string{"a0", "c0"}
	storm := ready(1500 * ms)
	congested := func(b probeResult) results {
		return results{"a0": ready(6000 * ms), "a1": ready(6050 * ms), "a2": ready(6100 * ms), "b0": b, "b1": b}
	}

	return []simScenario{
		{
			name: "mdc_1 2+2+1 decommission c0 + etcd re-election, DC-B hosts overloaded", nodes: m221,
			steps: []step{
				{note: "warm", probes: results{"a0": ready(ms), "a1": ready(ms), "b0": ready(20 * ms), "b1": ready(20 * ms), "c0": ready(40 * ms)}, rounds: 3},
				{note: "c0 gone, cached 200", slice: remote4, probes: results{"a0": ready(ms), "a1": ready(ms), "b0": storm, "b1": storm, "c0": hard}, bad: hang, rounds: 2},
				{note: "stall: a 500, b hang", slice: remote4, probes: results{"a0": notReady(ms), "a1": notReady(ms), "b0": notReady(2200 * ms), "b1": notReady(2200 * ms), "c0": hard}, bad: hang, rounds: 8},
				{note: "etcd back, b slow", slice: remote4, probes: results{"a0": ready(ms), "a1": ready(ms), "b0": ready(1200 * ms), "b1": ready(1200 * ms)}, rounds: 2},
				{note: "all fine", slice: remote4, probes: results{"a0": ready(ms), "a1": ready(ms), "b0": ready(20 * ms), "b1": ready(20 * ms)}, rounds: 3},
			},
			want: 8,
		},
		{
			name: "mdc_2 3+2 proxy in DC-B, etcd link down (b fast 500), a over bufferbloated path RTT 2s => readyz 6s", nodes: m32,
			steps: []step{
				{note: "warm", probes: merge(every(ready(20*ms), "a0", "a1", "a2"), every(ready(ms), b01...)), rounds: 3},
				{note: "split, b cached 200", probes: congested(ready(ms)), rounds: 2},
				{note: "split: b no leader", probes: congested(notReady(ms)), bad: b01, rounds: 25},
			},
			want: 2.03, wait: 0.39,
		},
		{
			name: "mdc_3 2+1 c0 lost etcd leader + a1 apiserver rollout restart, a0 alone under storm", nodes: m21,
			steps: []step{
				{note: "warm", probes: results{"a0": ready(ms), "a1": ready(ms), "c0": ready(20 * ms)}, rounds: 3},
				{note: "c0 no leader", probes: results{"a0": ready(ms), "a1": ready(ms), "c0": notReady(20 * ms)}, bad: c0, rounds: 5},
				{note: "a1 shutdown 500, storm", slice: noA1, probes: results{"a0": storm, "a1": notReady(ms), "c0": notReady(20 * ms)}, bad: c0a1, rounds: 2},
				{note: "a1 closed", slice: noA1, probes: results{"a0": storm, "a1": hard, "c0": notReady(20 * ms)}, bad: c0a1, rounds: 2},
				{note: "a1 new, warming", slice: m21, probes: results{"a0": storm, "a1": notReady(ms), "c0": notReady(20 * ms)}, bad: c0a1, rounds: 8},
				{note: "a1 warming, storm eases", slice: m21, probes: results{"a0": ready(800 * ms), "a1": notReady(ms), "c0": notReady(20 * ms)}, bad: c0a1, rounds: 8},
				{note: "a1 ready", slice: m21, probes: results{"a0": ready(2 * ms), "a1": ready(ms), "c0": notReady(20 * ms)}, bad: c0, rounds: 8},
			},
		},
	}
}

func simHunt3MDC() []simScenario {
	m32 := []string{"a0", "a1", "a2", "b0", "b1"}
	b01 := []string{"b0", "b1"}
	slowA := merge(results{"a0": ready(1300 * ms), "a1": ready(1500 * ms), "a2": ready(1700 * ms)}, every(notReady(ms), b01...))
	downs := [][]string{{"a0"}, {"a1", "a2"}, {"a2"}, {"a0", "a1"}, {"a1"}, {"a0", "a2"}}

	cut := make([]step, 0, 40)

	for i := range 40 {
		s := step{note: "cut: b no leader, a slow, dials to a dropped", probes: slowA, bad: b01}
		if i >= 5 {
			s.downs = downs[i%len(downs)]
		}

		cut = append(cut, s)
	}

	warmStep := step{note: "warm", probes: merge(every(ready(20*ms), "a0", "a1", "a2"), every(ready(ms), b01...)), rounds: 3}

	return []simScenario{
		{
			name:  "mdc_3 3+2 proxy in DC-B, etcd interconnect cut, DC-A over a saturated backup firewall, sporadic dial drops",
			nodes: m32, steps: append([]step{warmStep}, cut...),
		},
		{name: "mdc_3b same, proxy restarted mid-incident (all nodes new, score 1)", nodes: m32, steps: cut[3:]},
	}
}

func simPortMDC() []simScenario {
	m221 := []string{"a0", "a1", "b0", "b1", "c0"}
	remote4 := []string{"a0", "a1", "b0", "b1"}
	c0 := []string{"c0"}
	fast := results{"a0": ready(20 * ms), "a1": ready(20 * ms), "b0": ready(40 * ms), "b1": ready(40 * ms)}
	slow := results{"a0": ready(1200 * ms), "a1": ready(1200 * ms), "b0": ready(2000 * ms), "b1": ready(2000 * ms)}
	lossy := ready(2500 * ms)
	warmStep := step{note: "warm", probes: merge(fast, results{"c0": ready(ms)}), rounds: 3}

	md1b := []step{
		warmStep,
		{note: "remote late 200, c0 cached 200", probes: merge(every(lossy, remote4...), results{"c0": ready(ms)}), rounds: 3},
	}

	for i := range 20 {
		r, tag := lossy, "late200"
		if i%2 == 1 {
			r, tag = hard, "HARD"
		}

		var slice []string
		if i >= 12 {
			slice = remote4
		}

		md1b = append(md1b, step{note: "c0 500, remote " + tag, probes: merge(every(r, remote4...), results{"c0": notReady(2 * ms)}), slice: slice, bad: c0})
	}

	md1b = append(md1b,
		step{note: "link heals, c0 500", probes: merge(fast, results{"c0": notReady(2 * ms)}), slice: remote4, bad: c0, rounds: 3},
		step{note: "c0 back new, all ready", probes: merge(fast, results{"c0": ready(ms)}), slice: m221, rounds: 6},
	)

	return []simScenario{
		{
			name: "MD1 2+2+1 node@C: uplink congested (remote late 200), c0 loses quorum, leaves slice", nodes: m221,
			steps: []step{
				warmStep,
				{note: "remote late 200, c0 cached 200", probes: merge(slow, results{"c0": ready(ms)}), rounds: 3},
				{note: "remote late 200, c0 500", probes: merge(slow, results{"c0": notReady(2 * ms)}), bad: c0, rounds: 12},
				{note: "c0 out of slice, c0 500", probes: merge(slow, results{"c0": notReady(2 * ms)}), slice: remote4, bad: c0, rounds: 8},
				{note: "link heals, c0 catching up", probes: merge(fast, results{"c0": notReady(2 * ms)}), slice: remote4, bad: c0, rounds: 3},
				{note: "c0 back new, all ready", probes: merge(fast, results{"c0": ready(ms)}), slice: m221, rounds: 3},
			},
		},
		{name: "MD1b 2+2+1 node@C: lossy uplink (late200/HARD), c0 loses quorum, leaves slice", nodes: m221, steps: md1b, want: 10, hung: 4},
		{
			name: "MD5 2+2+1 node@C: global etcd stall, c0 cached 200 then overloaded (late 500)", nodes: m221,
			steps: []step{
				warmStep,
				{note: "stall: c0 cached 200, remote 500", probes: merge(every(notReady(3*ms), remote4...), results{"c0": ready(ms)}), bad: m221, rounds: 2},
				{note: "stall: c0 late 500 2.2s, remote 500", probes: merge(every(notReady(3*ms), remote4...), results{"c0": notReady(2200 * ms)}), bad: m221, rounds: 10},
				{note: "etcd back: remote R, c0 late 500 2.5s", probes: merge(fast, results{"c0": notReady(2500 * ms)}), bad: c0, rounds: 3},
				{note: "all ready", probes: merge(fast, results{"c0": ready(ms)}), rounds: 2},
			},
			want: 12, noGood: 12,
		},
		{
			name: "MD7 2+2+1 node@C: authz outage (all 500), c0 overloaded (late 500)", nodes: m221,
			steps: []step{
				warmStep,
				{note: "all 500", probes: results{"a0": notReady(20 * ms), "a1": notReady(20 * ms), "b0": notReady(40 * ms), "b1": notReady(40 * ms), "c0": notReady(ms)}, rounds: 3},
				{note: "remote 500, c0 late 500", probes: merge(every(notReady(30*ms), remote4...), results{"c0": notReady(2500 * ms)}), bad: c0, rounds: 10},
				{note: "webhook back", probes: merge(fast, results{"c0": ready(ms)}), rounds: 2},
			},
		},
	}
}

func simHunt4() []simScenario {
	m3 := []string{"m0", "m1", "m2"}
	m5 := []string{"m0", "m1", "m2", "m3", "m4"}
	m7 := []string{"m0", "m1", "m2", "m3", "m4", "m5", "m6"}
	m2, m6 := []string{"m2"}, []string{"m6"}
	m12, m34 := []string{"m1", "m2"}, []string{"m3", "m4"}
	p1 := results{"m0": ready(2000 * ms), "m1": ready(2400 * ms), "m2": ready(2800 * ms), "m3": ready(3200 * ms), "m4": ready(3600 * ms), "m5": ready(4000 * ms)}
	p2 := results{"m0": ready(3100 * ms), "m1": ready(3300 * ms), "m2": ready(3500 * ms), "m3": ready(3600 * ms), "m4": ready(3900 * ms), "m5": ready(4300 * ms)}
	storm5 := results{"m0": ready(1200 * ms), "m1": ready(1600 * ms), "m2": ready(2000 * ms)}
	storm3 := results{"m0": ready(1500 * ms), "m1": ready(2000 * ms)}

	burst := []step{warm(m3, ms, 3)}
	for range 10 {
		burst = append(burst,
			step{note: "m2 frozen (credits), m0/m1 slow", probes: merge(storm3, results{"m2": soft}), bad: m2, rounds: 4},
			step{note: "m2 running, m0/m1 slow", probes: merge(storm3, results{"m2": ready(ms)}), rounds: 2},
		)
	}

	burst = append(burst, warm(m3, ms, 3))

	return []simScenario{
		{
			name: "H4-1 3m: m2 OOM->warming, m1 swap-hung, m0 the only server with a 3.5s handshake", nodes: m3,
			steps: []step{
				warm(m3, ms, 3),
				{note: "storm: m2 refused, m1 hung, m0 3.5s", probes: results{"m0": ready(3500 * ms), "m1": soft, "m2": hard}, bad: m12, rounds: 3},
				{note: "m2 warming fast 500, m1 hung, m0 3.5s", probes: results{"m0": ready(3500 * ms), "m1": soft, "m2": notReady(ms)}, bad: m12, rounds: 12},
				{note: "m2 warming, m1 hung, m0 2.5s", probes: results{"m0": ready(2500 * ms), "m1": soft, "m2": notReady(ms)}, bad: m12, rounds: 10},
				{note: "m2 warming, m1 hung, m0 1.2s", probes: results{"m0": ready(1200 * ms), "m1": soft, "m2": notReady(ms)}, bad: m12, rounds: 5},
				{note: "m2 ready, m1 back, m0 1.2s", probes: results{"m0": ready(1200 * ms), "m1": ready(ms), "m2": ready(ms)}, rounds: 3},
				warm(m3, ms, 3),
			},
			want: 0.03, hung: 0.03, wait: 0.09,
		},
		{
			name: "H4-3 7m proxy package update: cluster-wide reconnect storm, handshakes 2.0-4.0s straddle T, scripted cascade, m6 warming", nodes: m7,
			steps: []step{
				{note: "storm: m6 warming", probes: merge(p1, results{"m6": notReady(ms)}), bad: m6, rounds: 10},
				{note: "storm + retry load: all over 3s", probes: merge(p2, results{"m6": notReady(ms)}), bad: m6, rounds: 10},
				{note: "draining: 1.5s", probes: merge(every(ready(1500*ms), m7[:6]...), results{"m6": notReady(ms)}), bad: m6, rounds: 10},
				{note: "calm, m6 warming", probes: merge(every(ready(ms), m7[:6]...), results{"m6": notReady(ms)}), bad: m6, rounds: 10},
				warm(m7, ms, 3),
			},
			wait: 0.06, falseKills: 1,
		},
		{
			name: "H4-4 5m: datastore stall hangs m3+m4 (kernel alive), m0..m2 slow-alive survivors", nodes: m5,
			steps: []step{
				warm(m5, ms, 3),
				{note: "m3/m4 hung, survivors slow", probes: merge(storm5, every(soft, m34...)), bad: m34, rounds: 30},
				{note: "datastore back: m3/m4 200, survivors 1.2s", probes: merge(every(ready(1200*ms), m3...), every(ready(ms), m34...)), rounds: 5},
				warm(m5, ms, 3),
			},
			want: 0.04, hung: 0.04, wait: 0.32,
		},
		{
			name: "H4-5 3m: m2 VM paused 30s (SYN dropped: 5s dial timeout), m0/m1 slow-alive under the reconnects", nodes: m3,
			steps: []step{
				warm(m3, ms, 3),
				{note: "m2 paused, m0/m1 1.5/2.2s", probes: results{"m0": ready(1500 * ms), "m1": ready(2200 * ms), "m2": soft}, blackhole: m2, bad: m2, rounds: 20},
				{note: "m2 paused, m0/m1 0.8/1.0s", probes: results{"m0": ready(800 * ms), "m1": ready(1000 * ms), "m2": soft}, blackhole: m2, bad: m2, rounds: 10},
				{note: "m2 resumed, catching up: late 200", probes: results{"m0": ready(ms), "m1": ready(ms), "m2": ready(900 * ms)}, rounds: 3},
				warm(m3, ms, 3),
			},
			wait: 0.2,
		},
		{name: "H4-6 3m: m2 burstable VM frozen 4s / running 2s x10, m0/m1 slow-alive storm", nodes: m3, steps: burst, wait: 0.87},
	}
}

func TestSim_Scenarios(t *testing.T) {
	for _, sc := range simScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			got := simulate(t, sc.nodes, sc.steps)
			t.Logf("badSeconds=%.2f badHung=%.2f badLate=%.2f badFast=%.2f noGood=%.0f waitSeconds=%.2f, want badSeconds<=%.2f badHung<=%.2f noGood=%.0f waitSeconds<=%.2f; %s",
				got.badSeconds, got.badHung, got.badLate, got.badFast, got.noGood, got.waitSeconds, sc.want, sc.hung, sc.noGood, sc.wait, got.longLived())

			if got.badSeconds <= sc.want+simTolerance && got.badHung <= sc.hung+simTolerance && got.noGood == sc.noGood && got.waitSeconds <= sc.wait+simTolerance && got.llFalseKills <= sc.falseKills {
				return
			}

			t.Errorf("bad share over the simulator's bound:\n%s", got.dump())
		})
	}
}

func TestSim_Totals(t *testing.T) {
	var got, want float64

	for _, sc := range simScenarios() {
		got += simulate(t, sc.nodes, sc.steps).badSeconds
		want += sc.want
	}

	if got > want+simTotalSlack {
		t.Errorf("total badSeconds = %.2f, want <= %.2f", got, want+simTotalSlack)
	}
}

type llScenario struct {
	name  string
	nodes []string
	steps []step
	want  string
	ok    func(on, off report) bool
}

func llScenarios() []llScenario {
	m3 := []string{"m0", "m1", "m2"}
	m0, m01 := []string{"m0"}, []string{"m0", "m1"}
	hung := func(probes results, bad []string, rounds int) []step {
		return []step{warm(m3, ms, 5), {note: "incident", probes: probes, bad: bad, rounds: rounds}}
	}

	return []llScenario{
		{
			name: "LL1 m0 handler livelock", nodes: m3,
			steps: hung(results{"m0": soft, "m1": ready(ms), "m2": ready(ms)}, m0, 50),
			want:  "on: evictions>=1 maxStuck<=5 over10=0 falseKills=0; off: over40>0 maxStuck>=45",
			ok: func(on, off report) bool {
				return on.llEvictions >= 1 && on.llMaxStuck <= 5 && on.llOver10 == 0 && on.llFalseKills == 0 &&
					off.llOver40 > 0 && off.llMaxStuck >= simPingExit
			},
		},
		{
			name: "LL2 Yuriy: m0 hung under a storm, survivors 1.5s", nodes: m3,
			steps: hung(results{"m0": soft, "m1": ready(1500 * ms), "m2": ready(1500 * ms)}, m0, 50),
			want:  "on: evictions>=1 over10=0 falseKills=0; off: over40>0",
			ok: func(on, off report) bool {
				return on.llEvictions >= 1 && on.llOver10 == 0 && on.llFalseKills == 0 && off.llOver40 > 0
			},
		},
		{
			name: "LL3 all slow-alive", nodes: m3,
			steps: hung(results{"m0": ready(1200 * ms), "m1": ready(2000 * ms), "m2": ready(2500 * ms)}, nil, 30),
			want:  "on: evictions=0",
			ok:    func(on, _ report) bool { return on.llEvictions == 0 },
		},
		{
			name: "LL4 etcd down: m0 hung, m1/m2 fast 500", nodes: m3,
			steps: hung(results{"m0": soft, "m1": notReady(ms), "m2": notReady(ms)}, m3, 30),
			want:  "on: evictions=0",
			ok:    func(on, _ report) bool { return on.llEvictions == 0 },
		},
		{
			name: "LL5 cached-200 witness", nodes: m3,
			steps: []step{
				warm(m3, ms, 5),
				{note: "m1 cached 200", probes: results{"m0": soft, "m1": ready(ms), "m2": notReady(ms)}, bad: []string{"m0", "m2"}, rounds: 2},
				{note: "m1 500", probes: results{"m0": soft, "m1": notReady(ms), "m2": notReady(ms)}, bad: m3, rounds: 28},
			},
			want: "on: evictions=0",
			ok:   func(on, _ report) bool { return on.llEvictions == 0 },
		},
		{
			name: "LL6 single upstream", nodes: m0,
			steps: []step{warm(m0, ms, 5), {note: "m0 hung", probes: results{"m0": soft}, bad: m0, rounds: 50}},
			want:  "on: evictions=0, same as off",
			ok: func(on, off report) bool {
				return on.llEvictions == 0 && on.longLived() == off.longLived()
			},
		},
		{
			name: "LL7 two of three hung", nodes: m3,
			steps: hung(results{"m0": soft, "m1": soft, "m2": ready(ms)}, m01, 30),
			want:  "on: over10=0 falseKills=0",
			ok:    func(on, _ report) bool { return on.llOver10 == 0 && on.llFalseKills == 0 },
		},
		{
			name: "LL8 slow but answering below the stall timeout", nodes: m3,
			steps: hung(results{"m0": ready(2500 * ms), "m1": ready(ms), "m2": ready(ms)}, nil, 30),
			want:  "on: evictions=0",
			ok:    func(on, _ report) bool { return on.llEvictions == 0 },
		},
		{
			name: "LL9 proxy restart mid-hang", nodes: m3,
			steps: []step{{note: "m0 hung", probes: results{"m0": soft, "m1": ready(ms), "m2": ready(ms)}, bad: m0, rounds: 30}},
			want:  "on: first eviction at round>=6, over10=0",
			ok:    func(on, _ report) bool { return on.llFirstEviction >= 6 && on.llOver10 == 0 },
		},
		{
			name: "LL10 all silent: no witness among equally hung upstreams", nodes: m3,
			steps: hung(results{"m0": soft, "m1": soft, "m2": soft}, m3, 30),
			want:  "on: evictions=0",
			ok:    func(on, _ report) bool { return on.llEvictions == 0 },
		},
		{
			name: "LL-gap fast 500 on one node is out of scope", nodes: m3,
			steps: hung(results{"m0": notReady(ms), "m1": ready(ms), "m2": ready(ms)}, m0, 50),
			want:  "on: evictions=0 over40>0",
			ok:    func(on, _ report) bool { return on.llEvictions == 0 && on.llOver40 > 0 },
		},
	}
}

func TestSim_LongLived(t *testing.T) {
	for _, sc := range llScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			on := simulate(t, sc.nodes, sc.steps)
			off := simulate(t, sc.nodes, sc.steps, WithStallTimeout(0))
			t.Logf("on:  %s", on.longLived())
			t.Logf("off: %s", off.longLived())

			if !sc.ok(on, off) {
				t.Errorf("long-lived connections out of bounds, want %s", sc.want)
			}
		})
	}
}
