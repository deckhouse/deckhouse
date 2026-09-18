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
	"sort"
	"time"
)

// trendFit is how well a least squares fit must explain the daily history
// before the projection is allowed to follow it upwards. Below it the series is
// noise around a level, not a trend.
const trendFit = 0.8

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

// ExceededThroughout reports whether name stayed above limit for the whole
// window: the journal reaches back past the start of the window and every
// observation inside it is above the limit. A window the journal does not cover
// yet is not a sustained exceedance, it is an unknown one.
//
// The limit is the current one, never the one that happened to be in force when
// a sample was taken: a customer who bought more quota is judged by what he owns
// now, and one whose quota shrank by what he owns now as well.
func (j Journal) ExceededThroughout(name string, now time.Time, window time.Duration, limit int64) bool {
	cutoff := now.Add(-window)
	if !j.covers(name, cutoff) {
		return false
	}

	seen := 0
	for _, s := range j.inWindow(name, cutoff, now) {
		if s.Values[name] <= float64(limit) {
			return false
		}
		seen++
	}
	return seen > 0
}

// covers reports whether the oldest observation of name is at or before cutoff,
// that is, whether the journal has been running long enough to answer for the
// whole window. Samples are kept sorted, so the first one carrying the metric is
// the oldest one.
func (j Journal) covers(name string, cutoff time.Time) bool {
	for _, s := range j.Samples {
		if _, ok := s.Values[name]; ok {
			return !s.At.After(cutoff)
		}
	}
	return false
}

// inWindow is every observation of name inside [cutoff, now], in order.
func (j Journal) inWindow(name string, cutoff, now time.Time) []Sample {
	out := make([]Sample, 0, len(j.Samples))
	for _, s := range j.Samples {
		if _, ok := s.Values[name]; !ok || s.At.Before(cutoff) || s.At.After(now) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// Stats derives the published views of a metric over the last window, each of
// them rounded to a tenth.
//
// Instant is the last observation and Avg7d the mean over the window; both are
// always present. Extrapolated is nil until there is something to project from:
// the journal must cover the whole window and hold at least two daily buckets.
// A projection off a two day journal is a guess, and it is published as a
// violation grade number.
//
// The fit runs over daily means rather than raw samples, so an afternoon spike
// cannot tilt it, and it is only followed when the trend is sustained: daily
// means that never fall, or a fit that explains the history well while pointing
// up. Anything else projects max(instant, avg), which is where a flat or noisy
// series is actually heading. ceiling caps the result; the caller knows the
// limit, this package does not.
//
// window is explicit because the averaging window is a policy decision (the
// specification says seven days) while Add keeps whatever retention the caller
// asked for; reading the average off a longer retention would quietly change
// the meaning of avg_7d.
func (j Journal) Stats(name string, now, horizon time.Time, window time.Duration, ceiling float64) MetricValue {
	cutoff := now.Add(-window)
	samples := j.inWindow(name, cutoff, now)
	if len(samples) == 0 {
		return MetricValue{}
	}

	instant := samples[len(samples)-1].Values[name]
	sum := 0.0
	for _, s := range samples {
		sum += s.Values[name]
	}
	avg := sum / float64(len(samples))

	out := MetricValue{Instant: round1(instant), Avg7d: round1(avg)}
	if !j.covers(name, cutoff) {
		return out
	}

	xs, ys, first := dailyMeans(samples, name, now)
	if len(ys) < 2 {
		return out
	}

	projected := math.Max(instant, avg)
	if slope, intercept, ok := sustainedTrend(xs, ys); ok {
		projected = slope*(horizon.Sub(first).Hours()/24) + intercept
	}
	projected = round1(math.Min(math.Max(projected, 0), ceiling))

	out.Extrapolated = &projected
	return out
}

// dailyMeans reduces the observations to one mean per UTC day. It returns the
// day offsets from the first bucket, the means themselves and the start of that
// first day, which is what the offsets and the horizon are measured from.
//
// The day now falls in is left out: it is still being filled, and a mean over
// the two hours of it that exist so far is not a daily mean. Kept in, an hour
// long spike this morning would read as a week ending on a rising trend.
func dailyMeans(samples []Sample, name string, now time.Time) (xs, ys []float64, first time.Time) {
	type bucket struct {
		day time.Time
		sum float64
		n   float64
	}

	buckets := make([]bucket, 0, len(samples))
	for _, s := range samples {
		day := s.At.UTC().Truncate(24 * time.Hour)
		if n := len(buckets); n > 0 && buckets[n-1].day.Equal(day) {
			buckets[n-1].sum += s.Values[name]
			buckets[n-1].n++
			continue
		}
		buckets = append(buckets, bucket{day: day, sum: s.Values[name], n: 1})
	}

	if n := len(buckets); n > 0 && buckets[n-1].day.Equal(now.UTC().Truncate(24*time.Hour)) {
		buckets = buckets[:n-1]
	}
	if len(buckets) == 0 {
		return nil, nil, time.Time{}
	}

	first = buckets[0].day
	xs = make([]float64, len(buckets))
	ys = make([]float64, len(buckets))
	for i, b := range buckets {
		xs[i] = b.day.Sub(first).Hours() / 24
		ys[i] = b.sum / b.n
	}
	return xs, ys, first
}

// sustainedTrend fits the daily means and reports whether the fit may be
// followed. It may when consumption never fell, which is a ramp, or when the fit
// explains the history well and points up. A series that merely wobbles around
// one spike is neither, and following it is how a two node cluster ends up
// projected at a hundred.
func sustainedTrend(xs, ys []float64) (slope, intercept float64, ok bool) {
	n := float64(len(ys))
	var sumX, sumY, sumXX, sumXY float64
	for i, x := range xs {
		sumX += x
		sumY += ys[i]
		sumXX += x * x
		sumXY += x * ys[i]
	}

	den := n*sumXX - sumX*sumX
	if den == 0 {
		return 0, 0, false
	}
	slope = (n*sumXY - sumX*sumY) / den
	intercept = (sumY - slope*sumX) / n

	for i := 1; i < len(ys); i++ {
		if ys[i] < ys[i-1] {
			return slope, intercept, slope > 0 && rSquared(xs, ys, slope, intercept, sumY/n) >= trendFit
		}
	}
	return slope, intercept, true
}

// rSquared is the share of the variance of ys that the fit explains. A series
// with no variance has nothing to explain, and nothing to project either.
func rSquared(xs, ys []float64, slope, intercept, mean float64) float64 {
	var residual, total float64
	for i, y := range ys {
		d := y - (slope*xs[i] + intercept)
		residual += d * d
		total += (y - mean) * (y - mean)
	}
	if total == 0 {
		return 0
	}
	return 1 - residual/total
}

// round1 is the precision every published consumption value is cut to. A tenth
// is finer than anything that can be bought, and coarse enough that the least
// squares noise of an unchanged window does not rewrite the status on every
// reconcile.
func round1(v float64) float64 { return math.Round(v*10) / 10 }
