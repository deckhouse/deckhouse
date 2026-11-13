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
	"fmt"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"fencing-agent/internal/domain"
)

func probe(seq uint64, ok bool) domain.CPState {
	return domain.CPState{ProbeSeq: seq, ProbeOK: ok}
}

// freshnessAt starts the rule with the answer at the boot time ms.
func freshnessAt(ms uint64) *Freshness {
	clock := bootMillis
	defer func() { bootMillis = clock }()

	bootMillis = func() uint64 { return ms }

	return NewFreshness(bootMillis())
}

func TestFreshness(t *testing.T) {
	tests := []struct {
		name  string
		after []domain.CPState
		want  []Fresh
	}{
		{
			name:  "two failed probes with larger numbers",
			after: []domain.CPState{probe(10, false), probe(11, false), probe(11, false), probe(12, false)},
			want:  []Fresh{FreshWait, FreshWait, FreshWait, FreshWitness},
		},
		{
			name:  "cycles started before the answer do not count",
			after: []domain.CPState{probe(9, false), probe(10, false), probe(11, false), probe(13, false)},
			want:  []Fresh{FreshWait, FreshWait, FreshWait, FreshWitness},
		},
		{
			name:  "a success of a cycle started before the answer does not cancel",
			after: []domain.CPState{probe(9, true), probe(10, true), probe(11, false), probe(13, false)},
			want:  []Fresh{FreshWait, FreshWait, FreshWait, FreshWitness},
		},
		{
			name:  "a successful probe cancels",
			after: []domain.CPState{probe(11, false), probe(12, true), probe(13, false), probe(14, false)},
			want:  []Fresh{FreshWait, FreshCanceled, FreshCanceled, FreshCanceled},
		},
		{
			name:  "the witness stays",
			after: []domain.CPState{probe(11, false), probe(12, false), probe(13, true)},
			want:  []Fresh{FreshWait, FreshWitness, FreshWitness},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := freshnessAt(10)

			for i, st := range tt.after {
				if got := f.Observe(st); got != tt.want[i] {
					t.Fatalf("Observe(seq=%d, ok=%v) = %d, want %d", st.ProbeSeq, st.ProbeOK, got, tt.want[i])
				}
			}
		})
	}
}

// freshnessRun starts the rule of freshness at the first state without a
// visible control plane and keeps the result and its time.
type freshnessRun struct {
	mu     sync.Mutex
	fr     *Freshness
	from   time.Time
	result Fresh
	at     time.Time
}

func (r *freshnessRun) observe(st domain.CPState) {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch {
	case r.fr == nil && !st.Visible && st.Seen:
		r.fr, r.from = NewFreshness(bootMillis()), st.ReadAt
	case r.fr != nil && r.result == FreshWait:
		if r.result = r.fr.Observe(st); r.result != FreshWait {
			r.at = st.ReadAt
		}
	}
}

// started returns the time of the read that started the rule, or zero.
func (r *freshnessRun) started() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.from
}

func (r *freshnessRun) get() (Fresh, time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.result, r.at.Sub(r.from)
}

// A short pause of the CP, about controlPlaneLostAfter long: the control plane is
// away for 4.35 s on critical, a SEE answer comes, the next probes succeed.
func TestShortPauseCancelsWitness(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := &fakeNode{proxy: proxyNew, cpUp: true, apis: apis}
		h := start(f, critControlPlaneLostAfter, critFastReadAfter)

		run := &freshnessRun{}
		h.setOnState(run.observe)

		time.Sleep(time.Second + offset)
		f.set(func(f *fakeNode) { f.cpUp = false })
		time.Sleep(4350 * time.Millisecond)
		f.set(func(f *fakeNode) { f.cpUp = true })
		time.Sleep(time.Second)
		h.stop()

		if res, _ := run.get(); res != FreshCanceled {
			t.Fatalf("result %d, want FreshCanceled", res)
		}

		if !h.last().Visible {
			t.Fatal("control plane not visible after the pause")
		}
	})
}

// The CP returns after a segment: the SEE answer comes while the
// own control plane is lost. When it comes back, there is no witness; when it
// does not, the answer is a witness.
func TestReturnAfterSegment(t *testing.T) {
	for _, back := range []bool{true, false} {
		synctest.Test(t, func(t *testing.T) {
			f := &fakeNode{proxy: proxyNew, cpUp: true, apis: apis}
			h := start(f, critControlPlaneLostAfter, critFastReadAfter)

			run := &freshnessRun{}
			h.setOnState(run.observe)

			time.Sleep(time.Second + offset)
			f.set(func(f *fakeNode) { f.cpUp = false })

			// The control plane comes back 150 ms after the start of the
			// rule: after the first read of the rule and before the second.
			var from time.Time
			for from.IsZero() {
				time.Sleep(10 * time.Millisecond)
				from = run.started()
			}

			time.Sleep(time.Until(from.Add(150 * time.Millisecond)))
			f.set(func(f *fakeNode) { f.cpUp = back })
			time.Sleep(time.Second)
			h.stop()

			want := FreshWitness
			if back {
				want = FreshCanceled
			}

			if res, _ := run.get(); res != want {
				t.Fatalf("control plane back=%v: result %d, want %d", back, res, want)
			}
		})
	}
}

// A real loss of API access with cpSource=self. Only the own
// rounds that start after the answer count, so the witness counts after two
// such rounds end. A round has one probe, so this is
// about freshnessWait = 2·failStepInterval + proxyTimeout when the packets are dropped, and faster when
// the connections are refused. The round in flight at the answer does not
// count. The result is the same for all profiles.
func TestRealAPILossCountsWitnessAfterNewRounds(t *testing.T) {
	for _, drop := range []bool{false, true} {
		// round is the time of a failed fail-step round.
		var round time.Duration
		if drop {
			round = probeTimeout
		}

		lo := time.Duration(float64(failStep)*(1-probeJitter)) + round
		hi := 2*time.Duration(float64(failStep)*(1+probeJitter)) + round + fastReadInterval

		for _, p := range profiles {
			t.Run(fmt.Sprintf("%s/drop=%v", p.name, drop), func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					node := &fakeNode{proxy: proxyDown, cpUp: true, hang: drop, apis: apis}
					h := start(node, p.controlPlaneLostAfter, p.fastReadAfter)

					run := &freshnessRun{}
					h.setOnState(run.observe)

					time.Sleep(time.Second + offset)
					node.set(func(n *fakeNode) { n.cpUp = false })
					time.Sleep(p.controlPlaneLostAfter + 3*time.Second)
					h.stop()

					res, took := run.get()
					t.Logf("witness after %v", took)

					if res != FreshWitness {
						t.Fatalf("result %d, want FreshWitness", res)
					}

					if !between(took, lo, hi) {
						t.Fatalf("witness after %v, want %v–%v", took, lo, hi)
					}
				})
			})
		}
	}
}
