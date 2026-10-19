// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package retry

import (
	"testing"
	"time"
)

func TestNextBackoff(t *testing.T) {
	tests := []struct {
		previous time.Duration
		want     time.Duration
	}{
		{0, Min},
		{Min / 2, Min},
		{Min, 2 * Min},
		{3 * time.Second, 6 * time.Second},
		{Max / 2, Max},
		{Max, Max},
		{2 * Max, Max},
	}

	for _, tt := range tests {
		if got := nextBackoff(tt.previous, Min, Max); got != tt.want {
			t.Errorf("nextBackoff(%v) = %v, want %v", tt.previous, got, tt.want)
		}
	}
}

func TestBackoff(t *testing.T) {
	t0 := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) time.Time { return t0.Add(d) }

	type failure struct {
		key string
		at  time.Duration
	}

	type check struct {
		key  string
		at   time.Duration
		want bool
	}

	tests := []struct {
		name      string
		failures  []failure
		succeeded bool // succeeded: Succeeded after the failures.
		wantUntil time.Duration
		checks    []check
	}{
		{
			name:   "zero value holds nothing",
			checks: []check{{"run:a", 0, false}, {"", 0, false}},
		},
		{
			name:      "holds the failed key before its deadline only",
			failures:  []failure{{"run:a", 0}},
			wantUntil: Min,
			checks: []check{
				{"run:a", 0, true},
				{"run:a", Min - time.Nanosecond, true},
				{"run:a", Min, false},
				{"run:a", 2 * Min, false},
			},
		},
		{
			name:      "another key is never held",
			failures:  []failure{{"run:a", 0}},
			wantUntil: Min,
			checks:    []check{{"run:b", 0, false}, {"disable:a", 0, false}, {"uninstall", 0, false}},
		},
		{
			name:      "the same key doubles the delay",
			failures:  []failure{{"run:a", 0}, {"run:a", Min}, {"run:a", 3 * Min}},
			wantUntil: 3*Min + 4*Min,
			checks:    []check{{"run:a", 3*Min + 4*Min - time.Nanosecond, true}, {"run:a", 7 * Min, false}},
		},
		{
			name:      "a failure of another key restarts from the minimum and releases the old key",
			failures:  []failure{{"run:a", 0}, {"run:a", Min}, {"run:b", 3 * Min}},
			wantUntil: 3*Min + Min,
			checks:    []check{{"run:a", 3 * Min, false}, {"run:b", 3 * Min, true}, {"run:b", 4 * Min, false}},
		},
		{
			name: "the delay is capped",
			failures: func() []failure {
				var fs []failure
				for i := range 12 {
					fs = append(fs, failure{"run:a", time.Duration(i) * time.Hour})
				}
				return fs
			}(),
			wantUntil: 11*time.Hour + Max,
		},
		{
			name:      "success forgets the failure",
			failures:  []failure{{"run:a", 0}, {"run:a", Min}},
			succeeded: true,
			wantUntil: Min + 2*Min,
			checks:    []check{{"run:a", Min, false}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b Backoff

			var until time.Time
			for _, f := range tt.failures {
				until = b.Failed(f.key, at(f.at))
			}

			if len(tt.failures) > 0 && until != at(tt.wantUntil) {
				t.Errorf("Failed returned %v, want %v", until.Sub(t0), tt.wantUntil)
			}

			if tt.succeeded {
				b.Succeeded()
			}

			wantUntil := time.Time{}
			if len(tt.failures) > 0 && !tt.succeeded {
				wantUntil = at(tt.wantUntil)
			}

			if b.Until() != wantUntil {
				t.Errorf("Until() = %v, want %v", b.Until(), wantUntil)
			}

			for _, c := range tt.checks {
				if got := b.Holds(c.key, at(c.at)); got != c.want {
					t.Errorf("Holds(%q, +%v) = %v, want %v", c.key, c.at, got, c.want)
				}
			}
		})
	}
}

func TestBackoffSucceededRestartsFromMin(t *testing.T) {
	t0 := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

	var b Backoff
	b.Failed("run:a", t0)
	b.Failed("run:a", t0)
	b.Succeeded()

	if got := b.Failed("run:a", t0); got != t0.Add(Min) {
		t.Errorf("after Succeeded, Failed = +%v, want +%v", got.Sub(t0), Min)
	}
}

func TestEarliest(t *testing.T) {
	t0 := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

	failed := func(at time.Time) Backoff {
		var b Backoff
		b.Failed("k", at) // held for Min from at

		return b
	}

	tests := []struct {
		name     string
		backoffs []Backoff
		want     time.Time
	}{
		{name: "none"},
		{name: "nothing failed", backoffs: []Backoff{{}, {}}},
		{name: "one ahead", backoffs: []Backoff{{}, failed(t0)}, want: t0.Add(Min)},
		{name: "the earlier of two", backoffs: []Backoff{failed(t0), failed(t0.Add(-Min / 2))}, want: t0.Add(Min / 2)},
		{name: "a deadline passed", backoffs: []Backoff{failed(t0.Add(-Min))}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Earliest(t0, tt.backoffs...); !got.Equal(tt.want) {
				t.Errorf("Earliest = %v, want %v", got, tt.want)
			}
		})
	}
}

// clock is a fixed time; timer records what Arm did to it.
type clock struct{ now time.Time }

func (c clock) Now() time.Time               { return c.now }
func (c clock) NewTimer(time.Duration) Timer { return &timer{} }

type timer struct {
	stopped bool
	reset   *time.Duration
}

func (t *timer) C() <-chan time.Time { return nil }

func (t *timer) Reset(d time.Duration) bool {
	t.reset = &d
	return false
}

func (t *timer) Stop() bool {
	t.stopped = true
	return false
}

func TestArm(t *testing.T) {
	t0 := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	c := clock{now: t0}

	tests := []struct {
		name      string
		at        time.Time
		wantStop  bool
		wantReset time.Duration
	}{
		{name: "zero stops", wantStop: true},
		{name: "a future deadline fires at it", at: t0.Add(3 * time.Second), wantReset: 3 * time.Second},
		// A non-positive Reset fires a time.Timer at once.
		{name: "a passed deadline fires at once", at: t0.Add(-time.Second), wantReset: -time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := &timer{}
			Arm(tm, c, tt.at)

			if tt.wantStop {
				if !tm.stopped || tm.reset != nil {
					t.Errorf("Arm(zero): stopped=%v reset=%v, want a stop only", tm.stopped, tm.reset)
				}

				return
			}

			if tm.reset == nil || *tm.reset != tt.wantReset {
				t.Fatalf("Arm reset = %v, want %v", tm.reset, tt.wantReset)
			}
		})
	}
}

func TestBackoffWithInitial(t *testing.T) {
	t0 := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	b := Backoff{Initial: 5 * time.Second}

	for _, want := range []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second} {
		if got := b.Failed("k", t0).Sub(t0); got != want {
			t.Fatalf("Failed = +%v, want +%v", got, want)
		}
	}

	b.Succeeded()

	if got := b.Failed("k", t0).Sub(t0); got != 5*time.Second {
		t.Errorf("after Succeeded, Failed = +%v, want the initial delay kept", got)
	}
}
