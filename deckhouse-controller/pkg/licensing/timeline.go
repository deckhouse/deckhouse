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

// timeline cuts the horizon at every start_at, expire_at and end of grace of
// the checked records, drops everything before now and evaluates the policy at
// the left boundary of each interval. Adjacent intervals that turn out
// identical are merged: a breakpoint that changes nothing is not a segment.
func timeline(base []RecordStatus, now time.Time, th Thresholds) []Segment {
	points := make([]time.Time, 0, 3*len(base))
	for _, r := range base {
		if !r.Accepted {
			continue
		}
		points = append(points, r.StartAt)
		if r.ExpireAt != nil {
			points = append(points, *r.ExpireAt, r.ExpireAt.Add(graceOf(r, th)))
		}
	}
	if len(points) == 0 {
		return nil
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Before(points[j]) })

	// The current segment starts at the latest breakpoint that is not in the
	// future, not at now: a status anchored at now would differ on every
	// reconcile and be rewritten each tick although nothing changed.
	anchor := now
	for _, p := range points {
		if !p.After(now) {
			anchor = p
		}
	}

	segments := make([]Segment, 0, len(points))
	for _, p := range points {
		if p.Before(anchor) {
			continue
		}
		if n := len(segments); n > 0 && segments[n-1].From.Equal(p) {
			continue
		}
		lim, _ := limits(activeAt(base, p))
		if len(lim) == 0 && len(segments) > 0 {
			// Every record has expired. An empty map would read as "nothing was
			// ever granted"; the honest statement is that the metrics granted
			// until now are granted zero from here on.
			lim = zeroed(segments[len(segments)-1].Limits)
		}
		seg := Segment{From: p, State: stateAt(base, p, th), Limits: lim}
		if n := len(segments); n > 0 && segments[n-1].State == seg.State && sameLimits(segments[n-1].Limits, seg.Limits) {
			continue
		}
		segments = append(segments, seg)
	}

	for i := 0; i+1 < len(segments); i++ {
		to := segments[i+1].From
		segments[i].To = &to
	}
	return segments
}

// cloneLimits copies a limit map including the values behind the pointers. A
// shallow copy would still let a caller rewrite a limit through the pointer.
func cloneLimits(src map[string]*int64) map[string]*int64 {
	out := make(map[string]*int64, len(src))
	for m, v := range src {
		if v == nil {
			out[m] = nil
			continue
		}
		n := *v
		out[m] = &n
	}
	return out
}

// zeroed maps the metric names of src to a finite zero. The values are fresh:
// a segment never shares a pointer with the segment it was derived from.
func zeroed(src map[string]*int64) map[string]*int64 {
	out := make(map[string]*int64, len(src))
	for m := range src {
		var v int64
		out[m] = &v
	}
	return out
}

func sameLimits(a, b map[string]*int64) bool {
	if len(a) != len(b) {
		return false
	}
	for m, av := range a {
		bv, ok := b[m]
		if !ok {
			return false
		}
		if (av == nil) != (bv == nil) {
			return false
		}
		if av != nil && *av != *bv {
			return false
		}
	}
	return true
}

// nextReduction is the first breakpoint where any limit goes down. Losing an
// unlimited metric counts as a reduction.
func nextReduction(segments []Segment) *Reduction {
	for i := 1; i < len(segments); i++ {
		if decreases(segments[i-1].Limits, segments[i].Limits) {
			// Copied down to the values: the reduction is published next to the
			// timeline it was derived from, and a caller editing one must not
			// silently edit the other.
			return &Reduction{
				At:   segments[i].From,
				From: cloneLimits(segments[i-1].Limits),
				To:   cloneLimits(segments[i].Limits),
			}
		}
	}
	return nil
}

func decreases(from, to map[string]*int64) bool {
	names := make(map[string]bool, len(from)+len(to))
	for m := range from {
		names[m] = true
	}
	for m := range to {
		names[m] = true
	}
	for m := range names {
		a, aOK := from[m]
		b, bOK := to[m]
		if !aOK {
			// Not granted before: anything is at least as good.
			continue
		}
		if a == nil {
			// Was unlimited: anything finite is a reduction.
			if !bOK || b != nil {
				return true
			}
			continue
		}
		if !bOK {
			if *a > 0 {
				return true
			}
			continue
		}
		if b != nil && *b < *a {
			return true
		}
	}
	return false
}
