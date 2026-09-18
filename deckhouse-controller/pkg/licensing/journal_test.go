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

package licensing

import (
	"math"
	"testing"
	"time"
)

const (
	testWindow    = 7 * 24 * time.Hour
	testRetention = 8 * 24 * time.Hour
	noCeiling     = 1e9
)

// dailyJournal lays one sample per day down to now, the oldest one a day before
// the window opens so that the window is covered.
func dailyJournal(now time.Time, values ...float64) *Journal {
	j := new(Journal)
	last := len(values) - 1
	for i, v := range values {
		j.Add(Sample{
			At:     now.Add(time.Duration(i-last) * 24 * time.Hour),
			Values: map[string]float64{"vCPU": v},
		}, testRetention)
	}
	return j
}

// hourlyJournal lays one sample per hour over the last span, which is how the
// controller actually fills the window.
func hourlyJournal(now time.Time, span time.Duration, value func(at time.Time) float64) *Journal {
	j := new(Journal)
	for at := now.Add(-span); !at.After(now); at = at.Add(time.Hour) {
		j.Add(Sample{At: at, Values: map[string]float64{"vCPU": value(at)}}, testRetention)
	}
	return j
}

func TestJournalAddPrunesAndSorts(t *testing.T) {
	base := ts("2026-05-01T00:00:00Z")
	var j Journal

	// One sample older than the retention window, and one arriving twice at the
	// same instant.
	j.Add(Sample{At: base.Add(-9 * 24 * time.Hour), Values: map[string]float64{"vCPU": 1}}, testRetention)
	j.Add(Sample{At: base.Add(-3 * time.Hour), Values: map[string]float64{"vCPU": 2}}, testRetention)
	j.Add(Sample{At: base.Add(-2 * time.Hour), Values: map[string]float64{"vCPU": 3}}, testRetention)
	j.Add(Sample{At: base, Values: map[string]float64{"vCPU": 4}}, testRetention)

	if len(j.Samples) != 3 {
		t.Fatalf("samples = %d, want the nine day old one pruned", len(j.Samples))
	}
	for i := 1; i < len(j.Samples); i++ {
		if j.Samples[i].At.Before(j.Samples[i-1].At) {
			t.Fatalf("samples are not sorted: %v", j.Samples)
		}
	}
	if got := j.Stats("vCPU", base, base, testWindow, noCeiling).Instant; got != 4 {
		t.Fatalf("instant = %v, want the latest sample", got)
	}
}

// A sample dated after the one being added came from a clock that was wrong.
// Keeping it would leave the window anchored in the future forever.
func TestJournalAddDropsFutureSamples(t *testing.T) {
	base := ts("2026-05-01T00:00:00Z")
	var j Journal

	j.Add(Sample{At: base.Add(72 * time.Hour), Values: map[string]float64{"vCPU": 9}}, testRetention)
	j.Add(Sample{At: base, Values: map[string]float64{"vCPU": 4}}, testRetention)

	if len(j.Samples) != 1 {
		t.Fatalf("samples = %+v, want the future one dropped", j.Samples)
	}
	if got := j.Samples[0].Values["vCPU"]; got != 4 {
		t.Fatalf("kept the sample with vCPU = %v, want the one just added", got)
	}
}

// Instant and avg_7d are present from the first observation, rounded to a tenth.
func TestJournalStatsInstantAndAverage(t *testing.T) {
	base := ts("2026-05-01T00:00:00Z")

	var j Journal
	for i, v := range []float64{3, 4, 4.44, 5.55} {
		j.Add(Sample{At: base.Add(time.Duration(i-3) * time.Hour), Values: map[string]float64{"vCPU": v}}, testRetention)
	}

	got := j.Stats("vCPU", base, base.Add(24*time.Hour), testWindow, noCeiling)
	if got.Instant != 5.6 {
		t.Fatalf("instant = %v, want 5.6", got.Instant)
	}
	// Mean of 3, 4, 4.44 and 5.55 is 4.2475, one decimal away from 4.2.
	if got.Avg7d != 4.2 {
		t.Fatalf("avg = %v, want 4.2", got.Avg7d)
	}

	// An unknown metric is a zero value, not a panic.
	if other := j.Stats("storage_tb", base, base, testWindow, noCeiling); other.Instant != 0 || other.Extrapolated != nil {
		t.Fatalf("unknown metric = %+v, want the zero value", other)
	}
}

// The window is what the views are taken over, not the whole retained journal.
func TestJournalStatsHonoursWindow(t *testing.T) {
	base := ts("2026-05-01T00:00:00Z")

	// Flat at 10 before the window opens, then 100..107 inside it.
	var j Journal
	for i := range 30 {
		at := base.Add(time.Duration(i-29) * 24 * time.Hour)
		v := 10.0
		if i >= 22 {
			v = float64(100 + (i - 22))
		}
		j.Add(Sample{At: at, Values: map[string]float64{"vCPU": v}}, 40*24*time.Hour)
	}

	week := j.Stats("vCPU", base, base.Add(24*time.Hour), testWindow, noCeiling)
	if week.Instant != 107 {
		t.Fatalf("instant = %v, want 107", week.Instant)
	}
	if week.Avg7d != 103.5 {
		t.Fatalf("avg over the window = %v, want 103.5", week.Avg7d)
	}

	month := j.Stats("vCPU", base, base.Add(24*time.Hour), 30*24*time.Hour, noCeiling)
	if month.Avg7d >= week.Avg7d {
		t.Fatalf("30d avg = %v, 7d avg = %v: the window changed nothing", month.Avg7d, week.Avg7d)
	}
}

// A projection is only published once there is something to project from: the
// window has to be covered and hold more than one day.
func TestJournalStatsWithoutEnoughHistory(t *testing.T) {
	base := ts("2026-05-01T00:00:00Z")
	horizon := base.Add(180 * 24 * time.Hour)

	// Two days of a cluster that briefly doubled. This is the shape that used to
	// project a two node cluster at a hundred nodes.
	var short Journal
	for i := range 48 {
		v := 2.0
		if i == 30 {
			v = 3
		}
		short.Add(Sample{At: base.Add(time.Duration(i-47) * time.Hour), Values: map[string]float64{"vCPU": v}}, testRetention)
	}

	got := short.Stats("vCPU", base, horizon, testWindow, 24)
	if got.Extrapolated != nil {
		t.Fatalf("extrapolated = %v, want null on a two day journal", *got.Extrapolated)
	}
	if got.Instant != 2 {
		t.Fatalf("instant = %v, want 2", got.Instant)
	}

	// The window is covered, but everything inside it landed on one day.
	var oneDay Journal
	oneDay.Add(Sample{At: base.Add(-9 * 24 * time.Hour), Values: map[string]float64{"vCPU": 2}}, 30*24*time.Hour)
	oneDay.Add(Sample{At: base, Values: map[string]float64{"vCPU": 2}}, 30*24*time.Hour)
	if got := oneDay.Stats("vCPU", base, horizon, testWindow, 24); got.Extrapolated != nil {
		t.Fatalf("extrapolated = %v, want null on a single daily bucket", *got.Extrapolated)
	}
}

func TestJournalStatsExtrapolation(t *testing.T) {
	base := ts("2026-05-01T00:00:00Z")
	horizon := base.Add(30 * 24 * time.Hour)

	cases := []struct {
		name    string
		values  []float64
		ceiling float64
		want    float64
	}{
		{
			// Nothing moved, so nothing is projected to move.
			name:    "a flat week projects the same value",
			values:  []float64{8, 8, 8, 8, 8, 8, 8, 8, 8},
			ceiling: 24,
			want:    8,
		},
		{
			// A real ramp is followed, and then capped: the projection is an
			// early warning, not a number anyone should read literally.
			name:    "a monotonic ramp is followed up to the cap",
			values:  []float64{1, 2, 3, 4, 5, 6, 7, 8, 9},
			ceiling: 30,
			want:    30,
		},
		{
			// The regression that started this: one day above, a week of
			// nothing, and a straight line through them aimed at the moon.
			name:    "one spike falls back to the plain average",
			values:  []float64{2, 2, 20, 2, 2, 2, 2, 2, 2},
			ceiling: 24,
			want:    4.3,
		},
		{
			// A cluster that shrank projects what it consumes, never a negative.
			name:    "a falling week never projects below what is there",
			values:  []float64{10, 9, 8, 7, 6, 5, 4, 3, 2},
			ceiling: 30,
			want:    5.5,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dailyJournal(base, tc.values...).Stats("vCPU", base, horizon, testWindow, tc.ceiling)
			if got.Extrapolated == nil {
				t.Fatalf("extrapolated is null, want %v", tc.want)
			}
			if math.Abs(*got.Extrapolated-tc.want) > 1e-9 {
				t.Fatalf("extrapolated = %v, want %v", *got.Extrapolated, tc.want)
			}
		})
	}
}

// Every published view is cut to one decimal, so that the least squares noise of
// an unchanged window does not rewrite the status on every reconcile.
func TestJournalStatsRoundsToOneDecimal(t *testing.T) {
	base := ts("2026-05-01T00:00:00Z")

	j := dailyJournal(base, 1.01, 2.02, 3.03, 4.04, 5.05, 6.06, 7.07, 8.08, 9.09)
	got := j.Stats("vCPU", base, base.Add(36*time.Hour), testWindow, 1000)

	for _, v := range []float64{got.Instant, got.Avg7d, *got.Extrapolated} {
		if math.Abs(v*10-math.Round(v*10)) > 1e-9 {
			t.Fatalf("value %v carries more than one decimal", v)
		}
	}
}

func TestExceededThroughout(t *testing.T) {
	base := ts("2026-05-01T00:00:00Z")

	cases := []struct {
		name   string
		values []float64
		limit  int64
		want   bool
	}{
		{
			name:   "every observation of the window is above the limit",
			values: []float64{3, 3, 3, 3, 3, 3, 3, 3, 3},
			limit:  2,
			want:   true,
		},
		{
			// One reading back within the quota and the exceedance was not
			// continuous. This is the exit: removing the node ends it at once.
			name:   "one observation came back under the limit",
			values: []float64{3, 3, 3, 3, 3, 3, 3, 3, 2},
			limit:  2,
		},
		{
			name:   "exactly at the limit is not above it",
			values: []float64{2, 2, 2, 2, 2, 2, 2, 2, 2},
			limit:  2,
		},
		{
			// Consumption is above the limit, but not for long enough to know
			// whether it stays there.
			name:   "the journal does not cover the window yet",
			values: []float64{9, 9, 9},
			limit:  2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			j := dailyJournal(base, tc.values...)
			if got := j.ExceededThroughout("vCPU", base, testWindow, tc.limit); got != tc.want {
				t.Fatalf("exceededThroughout = %v, want %v", got, tc.want)
			}
		})
	}

	// A metric that was never observed was never exceeded.
	if dailyJournal(base, 9, 9, 9, 9, 9, 9, 9, 9, 9).ExceededThroughout("nodes", base, testWindow, 0) {
		t.Fatal("an unobserved metric reports a sustained exceedance")
	}
}
