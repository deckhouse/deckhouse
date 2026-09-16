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

func TestJournalAddPrunesAndSorts(t *testing.T) {
	base := ts("2026-05-01T00:00:00Z")
	var j Journal

	// One sample older than the retention window, and one arriving twice at the
	// same instant.
	j.Add(Sample{At: base.Add(-8 * 24 * time.Hour), Values: map[string]float64{"vCPU": 1}}, 7*24*time.Hour)
	j.Add(Sample{At: base.Add(-3 * time.Hour), Values: map[string]float64{"vCPU": 2}}, 7*24*time.Hour)
	j.Add(Sample{At: base.Add(-2 * time.Hour), Values: map[string]float64{"vCPU": 3}}, 7*24*time.Hour)
	j.Add(Sample{At: base, Values: map[string]float64{"vCPU": 4}}, 7*24*time.Hour)

	if len(j.Samples) != 3 {
		t.Fatalf("samples = %d, want the eight day old one pruned", len(j.Samples))
	}
	for i := 1; i < len(j.Samples); i++ {
		if j.Samples[i].At.Before(j.Samples[i-1].At) {
			t.Fatalf("samples are not sorted: %v", j.Samples)
		}
	}
	if got := j.Stats("vCPU", base, base, 7*24*time.Hour).Instant; got != 4 {
		t.Fatalf("instant = %v, want the latest sample", got)
	}
}

// A sample dated after the one being added came from a clock that was wrong.
// Keeping it would leave the window anchored in the future forever.
func TestJournalAddDropsFutureSamples(t *testing.T) {
	base := ts("2026-05-01T00:00:00Z")
	var j Journal

	j.Add(Sample{At: base.Add(72 * time.Hour), Values: map[string]float64{"vCPU": 9}}, 7*24*time.Hour)
	j.Add(Sample{At: base, Values: map[string]float64{"vCPU": 4}}, 7*24*time.Hour)

	if len(j.Samples) != 1 {
		t.Fatalf("samples = %+v, want the future one dropped", j.Samples)
	}
	if got := j.Samples[0].Values["vCPU"]; got != 4 {
		t.Fatalf("kept the sample with vCPU = %v, want the one just added", got)
	}
	if got := j.Stats("vCPU", base, base, 7*24*time.Hour).Instant; got != 4 {
		t.Fatalf("instant = %v, want 4", got)
	}
}

func TestJournalStats(t *testing.T) {
	base := ts("2026-05-01T00:00:00Z")

	// A synthetic ramp: +1 vCPU per hour over 24 hours.
	var ramp Journal
	for i := range 24 {
		ramp.Add(Sample{
			At:     base.Add(time.Duration(i-23) * time.Hour),
			Values: map[string]float64{"vCPU": float64(100 + i)},
		}, 7*24*time.Hour)
	}

	stats := ramp.Stats("vCPU", base, base.Add(24*time.Hour), 7*24*time.Hour)
	if stats.Instant != 123 {
		t.Fatalf("instant = %v, want 123", stats.Instant)
	}
	// Mean of 100..123.
	if math.Abs(stats.Avg7d-111.5) > 1e-9 {
		t.Fatalf("avg = %v, want 111.5", stats.Avg7d)
	}
	// A perfect ramp extrapolates exactly one more day ahead: 123 + 24.
	if math.Abs(stats.Extrapolated-147) > 1e-6 {
		t.Fatalf("extrapolated = %v, want 147", stats.Extrapolated)
	}

	// A falling ramp is clamped at zero rather than going negative.
	var falling Journal
	for i := range 10 {
		falling.Add(Sample{
			At:     base.Add(time.Duration(i-9) * time.Hour),
			Values: map[string]float64{"vCPU": float64(10 - i)},
		}, 7*24*time.Hour)
	}
	if got := falling.Stats("vCPU", base, base.Add(48*time.Hour), 7*24*time.Hour).Extrapolated; got != 0 {
		t.Fatalf("extrapolated = %v, want the clamp at 0", got)
	}

	// Fewer than two samples: all three views fall back to the latest reading,
	// the fields are present from day one.
	var single Journal
	single.Add(Sample{At: base, Values: map[string]float64{"vCPU": 7}}, 7*24*time.Hour)
	if got := single.Stats("vCPU", base, base.Add(30*24*time.Hour), 7*24*time.Hour); got != (MetricValue{7, 7, 7}) {
		t.Fatalf("single sample stats = %+v", got)
	}

	// An unknown metric is a zero value, not a panic.
	if got := ramp.Stats("storage_tb", base, base.Add(time.Hour), 7*24*time.Hour); got != (MetricValue{}) {
		t.Fatalf("unknown metric = %+v", got)
	}

	// A flat series extrapolates flat.
	var flat Journal
	for i := range 5 {
		flat.Add(Sample{At: base.Add(time.Duration(i-4) * time.Hour), Values: map[string]float64{"vCPU": 42}}, 7*24*time.Hour)
	}
	got := flat.Stats("vCPU", base, base.Add(72*time.Hour), 7*24*time.Hour)
	if math.Abs(got.Extrapolated-42) > 1e-6 || got.Avg7d != 42 || got.Instant != 42 {
		t.Fatalf("flat series = %+v", got)
	}
}

// L3: the window is what avg_7d and the regression are taken over, not the
// whole retained journal. A journal holding 30 days of history must still
// answer with a seven day average.
func TestJournalStatsHonoursWindow(t *testing.T) {
	base := ts("2026-05-01T00:00:00Z")

	// 30 daily samples: flat at 10 until the window opens, then a +1/day ramp
	// from 100 over the eight samples the seven day window covers.
	var j Journal
	for i := range 30 {
		at := base.Add(time.Duration(i-29) * 24 * time.Hour)
		v := 10.0
		if i >= 22 {
			v = float64(100 + (i - 22))
		}
		j.Add(Sample{At: at, Values: map[string]float64{"vCPU": v}}, 40*24*time.Hour)
	}
	if len(j.Samples) != 30 {
		t.Fatalf("samples = %d, want the full retention kept", len(j.Samples))
	}

	// Window of 7 days: samples at now-7d..now, i.e. 100..107.
	week := j.Stats("vCPU", base, base.Add(24*time.Hour), 7*24*time.Hour)
	if week.Instant != 107 {
		t.Fatalf("instant = %v, want 107", week.Instant)
	}
	if math.Abs(week.Avg7d-103.5) > 1e-9 {
		t.Fatalf("avg over the window = %v, want 103.5", week.Avg7d)
	}
	// A clean +1/day ramp, one more day ahead.
	if math.Abs(week.Extrapolated-108) > 1e-6 {
		t.Fatalf("extrapolated = %v, want 108", week.Extrapolated)
	}

	// The same journal over 30 days drags the flat prehistory in.
	month := j.Stats("vCPU", base, base.Add(24*time.Hour), 30*24*time.Hour)
	if month.Avg7d >= week.Avg7d {
		t.Fatalf("30d avg = %v, 7d avg = %v: the window changed nothing", month.Avg7d, week.Avg7d)
	}

	// A window shorter than the sampling interval leaves a single sample.
	if got := j.Stats("vCPU", base, base.Add(24*time.Hour), time.Hour); got != (MetricValue{107, 107, 107}) {
		t.Fatalf("one sample window = %+v", got)
	}
}
