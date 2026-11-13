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
	"context"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

// probeLog records the start and the end of every probe.
type probeLog struct {
	mu           sync.Mutex
	starts, ends []time.Time
}

// hang waits until the probe timeout and answers not ready.
func (l *probeLog) hang(ctx context.Context, _ string) bool {
	l.mu.Lock()
	l.starts = append(l.starts, time.Now())
	l.mu.Unlock()

	<-ctx.Done()

	l.mu.Lock()
	l.ends = append(l.ends, time.Now())
	l.mu.Unlock()

	return false
}

// runProber runs the prober for d and stops it.
func runProber(p *prober, d time.Duration) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		p.run(ctx)
	}()

	time.Sleep(d)
	cancel()
	<-done
}

// In normal mode a round that ends before the interval does not move the
// schedule: the next round starts the interval after the start of the
// previous round.
func TestProberScheduleFromRoundStart(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var starts []time.Time

		p := &prober{
			targets: func() []string { return []string{"a"} },
			ready: func(context.Context, string) bool {
				starts = append(starts, time.Now())
				time.Sleep(probeTimeout - time.Millisecond)

				return true
			},
		}

		runProber(p, time.Minute)

		if len(starts) < 40 {
			t.Fatalf("rounds = %d, want one about every %v", len(starts), probeInterval)
		}

		for i := 1; i < len(starts); i++ {
			if d := starts[i].Sub(starts[i-1]); !between(d, jittered(probeInterval, -1), jittered(probeInterval, 1)) {
				t.Fatalf("round %d started %v after the previous one, want %v ±%v", i, d, probeInterval, probeJitter)
			}
		}
	})
}

// In fail-step a round with a hung target ends before the next round starts.
func TestProberFailStepWaitsForRoundEnd(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var l probeLog

		p := &prober{targets: func() []string { return []string{"a"} }, ready: l.hang}

		runProber(p, 10*time.Second)

		if len(l.starts) < 10 {
			t.Fatalf("rounds = %d, want fail-step rounds", len(l.starts))
		}

		for i := 1; i < len(l.starts); i++ {
			if l.starts[i].Before(l.ends[i-1]) {
				t.Fatalf("round %d started %v before the previous round ended", i, l.ends[i-1].Sub(l.starts[i]))
			}

			if d := l.starts[i].Sub(l.starts[i-1]); !between(d, probeTimeout, jittered(failStep, 1)) {
				t.Fatalf("round %d started %v after the previous one, want %v–%v", i, d, probeTimeout, jittered(failStep, 1))
			}
		}
	})
}

// One target hangs until the timeout in every round, and another target
// answers ready at once. When a target answers at once, the age of the ready
// answer stays below the interval with the largest jitter.
func TestProberLastReadyWithHungTarget(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var l probeLog

		p := &prober{
			targets: func() []string { return []string{"a", "b"} },
			ready: func(ctx context.Context, addr string) bool {
				if addr == "a" {
					return true
				}

				return l.hang(ctx, addr)
			},
		}

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})

		go func() {
			defer close(done)
			p.run(ctx)
		}()

		limit := jittered(probeInterval, 1)

		var worst time.Duration

		time.Sleep(probeTimeout + time.Millisecond)

		for range 3000 {
			_, _, lastReady := p.state()
			worst = max(worst, time.Since(lastReady))

			time.Sleep(10 * time.Millisecond)
		}

		cancel()
		<-done

		if worst > limit {
			t.Fatalf("age of the ready answer = %v, want <= %v", worst, limit)
		}

		t.Logf("age of the ready answer: <= %v (limit %v)", worst, limit)
	})
}

// A ready answer is visible at once, also when another target keeps the
// round open. Its age is counted from the answer, not from the end of the
// round.
func TestProberLastReadyFromAnswer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		gate := make(chan struct{})

		p := &prober{
			targets: func() []string { return []string{"a", "b"} },
			ready: func(_ context.Context, addr string) bool {
				if addr == "b" {
					<-gate
				}

				return true
			},
		}

		answer := time.Now()
		done := make(chan struct{})

		go func() {
			defer close(done)
			p.round(context.Background(), 1)
		}()

		// "a" answered, "b" keeps the round open.
		synctest.Wait()
		time.Sleep(300 * time.Millisecond)

		if _, _, lastReady := p.state(); !lastReady.Equal(answer) {
			t.Errorf("ready answer before the end of the round at %v, want %v", lastReady, answer)
		}

		close(gate)
		<-done

		seq, ok, lastReady := p.state()
		if seq != 1 || !ok {
			t.Fatalf("round = %d %v, want 1 true", seq, ok)
		}

		if !lastReady.Equal(time.Now()) {
			t.Fatalf("age of the ready answer after the end of the round = %v, want 0", time.Since(lastReady))
		}
	})
}
