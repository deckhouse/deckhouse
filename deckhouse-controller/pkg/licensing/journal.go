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
	"sort"
	"time"
)

// Sample is one observation of the consumption metrics.
type Sample struct {
	At     time.Time          `json:"at"`
	Values map[string]float64 `json:"values"`
}

// Journal is the persisted observation window plus the monotonic registration
// counter, both of which live in the same ConfigMap.
type Journal struct {
	Seq     uint64   `json:"seq"`
	Samples []Sample `json:"samples"`
}

// Add appends a sample and drops everything older than retention relative to
// the new sample. Samples dated after the new one are dropped as well: a clock
// that jumped forward once must not leave an unreachable sample behind, freezing
// the window at a future timestamp forever. Samples are kept sorted by time.
func (j *Journal) Add(s Sample, retention time.Duration) {
	cutoff := s.At.Add(-retention)
	kept := make([]Sample, 0, len(j.Samples)+1)
	for _, old := range j.Samples {
		if !old.At.Before(cutoff) && !old.At.After(s.At) {
			kept = append(kept, old)
		}
	}
	kept = append(kept, s)
	sort.SliceStable(kept, func(a, b int) bool { return kept[a].At.Before(kept[b].At) })
	j.Samples = kept
}

// Stats derives the three views of a metric over the last window. Extrapolated
// is a least squares linear fit over the same window, evaluated at horizon and
// clamped at zero. With fewer than two observations all three views equal the
// latest reading: the fields must be present from day one even before the
// window is filled.
//
// window is explicit because the averaging window is a policy decision (the
// specification says seven days) while Add keeps whatever retention the caller
// asked for; reading the average off a longer retention would quietly change
// the meaning of avg_7d.
func (j Journal) Stats(name string, now, horizon time.Time, window time.Duration) MetricValue {
	cutoff := now.Add(-window)
	xs := make([]float64, 0, len(j.Samples))
	ys := make([]float64, 0, len(j.Samples))
	for _, s := range j.Samples {
		v, ok := s.Values[name]
		if !ok || s.At.Before(cutoff) {
			continue
		}
		xs = append(xs, s.At.Sub(now).Seconds())
		ys = append(ys, v)
	}
	if len(ys) == 0 {
		return MetricValue{}
	}

	instant := ys[len(ys)-1]
	if len(ys) < 2 {
		return MetricValue{Instant: instant, Avg7d: instant, Extrapolated: instant}
	}

	n := float64(len(ys))
	var sumX, sumY, sumXX, sumXY float64
	for i, x := range xs {
		sumX += x
		sumY += ys[i]
		sumXX += x * x
		sumXY += x * ys[i]
	}
	avg := sumY / n

	extrapolated := instant
	if den := n*sumXX - sumX*sumX; den != 0 {
		slope := (n*sumXY - sumX*sumY) / den
		intercept := (sumY - slope*sumX) / n
		extrapolated = slope*horizon.Sub(now).Seconds() + intercept
	}
	if extrapolated < 0 {
		extrapolated = 0
	}

	return MetricValue{Instant: instant, Avg7d: avg, Extrapolated: extrapolated}
}
