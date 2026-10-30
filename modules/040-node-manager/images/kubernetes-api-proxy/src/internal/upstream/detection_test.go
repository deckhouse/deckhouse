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
	"sort"
	"testing"
	"time"
)

// timedRound is a round of the detection test. The times are from the start
// of the test.
type timedRound struct {
	start, applied time.Duration
	ok             bool
}

// detectionRounds drives the rounds of a list with the defaults on a fake
// clock, with the schedule of runLoop: the rounds run one after another, and
// the next round starts a delay after the start of the previous round, or
// when the previous round ends if it ends later. The delay comes from the
// state after the round, and it has the largest jitter. Upstream "a" answers
// ready at once to a probe that starts before lostAt, and after lostAt every
// probe of it takes the full timeout. With slow, upstream "b" never answers.
// The test runs until the last round that starts before until applies.
func detectionRounds(t *testing.T, lostAt, until time.Duration, slow bool) []timedRound {
	t.Helper()

	list, err := newList(nil, byAddr, defaultOptions()...)
	if err != nil {
		t.Fatal(err)
	}

	origin := time.Unix(1000, 0)
	clock := &fakeClock{now: origin}
	list.now = clock.Now
	list.boot = func() uint64 { return uint64(clock.Now().Sub(origin).Milliseconds()) + 1 }

	addrs := []string{"a"}
	if slow {
		addrs = append(addrs, "b")
	}

	list.Reconcile(ups(addrs...))
	backends := list.backends()

	set := func(at time.Duration) {
		clock.mu.Lock()
		clock.now = origin.Add(at)
		clock.mu.Unlock()
	}

	var rounds []timedRound

	for next := time.Duration(0); next < until; {
		set(next)

		seq := list.beginRound()

		ready := next < lostAt
		out := []outcome{{value: probeResult{kind: probeSoft}, ok: true}}

		var dur time.Duration
		if ready {
			out[0].value = probeResult{kind: probeReady, latency: time.Millisecond}
		} else {
			dur = defaultTimeout
		}

		if slow {
			out = append(out, outcome{value: probeResult{kind: probeSoft}, ok: true})
			dur = defaultTimeout
		}

		applied := next + dur
		set(applied)

		list.mu.Lock()
		list.applyRoundLocked(seq, backends, out, clock.Now())
		list.unlockAndLog()

		rounds = append(rounds, timedRound{start: next, applied: applied, ok: ready})

		next = max(next+list.cycleDelay(1), applied)
	}

	return rounds
}

// confirmWait returns the time from at until the second failed round that
// started after at applies. A client of /upstreams that reads probeSeq at
// this moment waits so long to see two failed rounds that started later. It
// returns -1 when there is no such round.
func confirmWait(rounds []timedRound, at time.Duration) time.Duration {
	i := sort.Search(len(rounds), func(i int) bool { return rounds[i].start > at })

	fails := 0
	for ; i < len(rounds); i++ {
		if rounds[i].ok {
			return -1
		}

		fails++
		if fails == 2 {
			return rounds[i].applied - at
		}
	}

	return -1
}

func maxWait(t *testing.T, rounds []timedRound, from, to time.Duration) time.Duration {
	t.Helper()

	var worst time.Duration

	for at := from; at <= to; at += 5 * time.Millisecond {
		w := confirmWait(rounds, at)
		if w < 0 {
			t.Fatalf("no two failed rounds after %v", at)
		}

		worst = max(worst, w)
	}

	return worst
}

// The control plane is lost at L, and the last ready answer is at A < L.
// The first failed round applies not later than A + Imax + TO (1.7 s), and
// fail-step starts. After that, until the fail-step window ends, the rounds
// start in fail-step, and two failed rounds that start after any moment
// apply not later than 2·fail-step·(1 + jitter) + TO (1.7 s) after it. Before
// fail-step and after its window the wait is not more than 2·Imax + TO
// (2.9 s).
func TestFailStep_ConfirmWait(t *testing.T) {
	imax := time.Duration(float64(defaultInterval) * (1 + defaultJitter))

	enterLimit := imax + defaultTimeout
	failStepLimit := 2*time.Duration(float64(failStepInterval)*(1+defaultJitter)) + defaultTimeout
	normalLimit := 2*imax + defaultTimeout

	var worstEnter, worstFailStep, worstTransition, worstAfterWindow time.Duration

	for _, slow := range []bool{false, true} {
		for lost := 5 * time.Second; lost < 5*time.Second+imax; lost += 10 * time.Millisecond {
			rounds := detectionRounds(t, lost, lost+defaultWindow+15*time.Second, slow)

			lastReady := -1
			for i, r := range rounds {
				if r.ok {
					lastReady = i
				}
			}

			answer := rounds[lastReady].start
			firstFail := rounds[lastReady+1]

			enter := firstFail.applied - answer
			if enter > enterLimit {
				t.Errorf("slow=%v lost=%v: first failed round applied %v after the last ready answer, want <= %v", slow, lost, enter, enterLimit)
			}

			worstEnter = max(worstEnter, enter)

			// A round that ends in the window gets the fail-step delay. The
			// two rounds after the moment must end before the window ends.
			windowEnd := firstFail.applied + defaultWindow

			w := maxWait(t, rounds, firstFail.applied, windowEnd-2*time.Second)
			if w > failStepLimit {
				t.Errorf("slow=%v lost=%v: wait = %v during fail-step, want <= %v", slow, lost, w, failStepLimit)
			}

			worstFailStep = max(worstFailStep, w)

			tr := maxWait(t, rounds, lost, firstFail.applied)
			if tr > normalLimit {
				t.Errorf("slow=%v lost=%v: wait = %v before fail-step, want <= %v", slow, lost, tr, normalLimit)
			}

			worstTransition = max(worstTransition, tr)

			// After the window rounds run in normal mode, as before fail-step.
			aw := maxWait(t, rounds, windowEnd+time.Second, windowEnd+5*time.Second)
			if aw > normalLimit {
				t.Errorf("slow=%v lost=%v: wait = %v after the fail-step window, want <= %v", slow, lost, aw, normalLimit)
			}

			worstAfterWindow = max(worstAfterWindow, aw)
		}
	}

	t.Logf("first failed round after the last ready answer: <= %v (limit %v)", worstEnter, enterLimit)
	t.Logf("wait during fail-step: <= %v (limit %v)", worstFailStep, failStepLimit)
	t.Logf("wait before fail-step: <= %v (limit %v)", worstTransition, normalLimit)
	t.Logf("wait after the fail-step window: <= %v (limit %v)", worstAfterWindow, normalLimit)
}
