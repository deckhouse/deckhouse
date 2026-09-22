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
	"testing"
)

// R1, R3, R4, R5, R6, R9, R10, R11, R14, R15: renewal and supersession.
func TestRenewsAndSupersedes(t *testing.T) {
	const boundary = "2026-07-01T00:00:00Z"
	l1 := func() RecordStatus {
		return wl(recordA, "2026-01-01T00:00:00Z", boundary, map[string]*int64{"vCPU": i64(50)})
	}
	successor := func(id, start string, limit int64, renews, supersedes []string) RecordStatus {
		r := wl(id, start, "2027-01-01T00:00:00Z", map[string]*int64{"vCPU": i64(limit)})
		r.Renews = renews
		r.Supersedes = supersedes
		return r
	}

	cases := []struct {
		name         string
		records      []RecordStatus
		now          string
		wantVCPU     int64
		wantRejected map[string]string
	}{
		{
			name:         "R1 renewal starting when the predecessor expires",
			records:      []RecordStatus{l1(), successor(recordB, boundary, 50, []string{recordA}, nil)},
			now:          "2026-06-26T00:00:00Z",
			wantVCPU:     50,
			wantRejected: map[string]string{recordB: ReasonNotYetValid},
		},
		{
			name:         "R14 exactly at the boundary the successor takes over",
			records:      []RecordStatus{l1(), successor(recordB, boundary, 50, []string{recordA}, nil)},
			now:          boundary,
			wantVCPU:     50,
			wantRejected: map[string]string{recordA: ReasonRenewed},
		},
		{
			name:         "R3 the operator got start_at wrong but renews saves the day",
			records:      []RecordStatus{l1(), successor(recordB, "2026-06-26T00:00:00Z", 50, []string{recordA}, nil)},
			now:          "2026-06-27T00:00:00Z",
			wantVCPU:     50,
			wantRejected: map[string]string{recordA: ReasonRenewed},
		},
		{
			name:     "R4 the same overlap without renews doubles the quota",
			records:  []RecordStatus{l1(), successor(recordB, "2026-06-26T00:00:00Z", 50, nil, nil)},
			now:      "2026-06-27T00:00:00Z",
			wantVCPU: 100,
		},
		{
			name:         "R5 the successor is not valid yet",
			records:      []RecordStatus{l1(), successor(recordB, "2026-06-26T00:00:00Z", 50, []string{recordA}, nil)},
			now:          "2026-06-01T00:00:00Z",
			wantVCPU:     50,
			wantRejected: map[string]string{recordB: ReasonNotYetValid},
		},
		{
			name:         "R11 a renewal for a larger volume replaces, not adds",
			records:      []RecordStatus{l1(), successor(recordB, boundary, 80, []string{recordA}, nil)},
			now:          boundary,
			wantVCPU:     80,
			wantRejected: map[string]string{recordA: ReasonRenewed},
		},
		{
			name:     "R9 renewing an id that is not installed is a no-op",
			records:  []RecordStatus{l1(), successor(recordB, "2026-01-01T00:00:00Z", 40, []string{"deadbeef-0000-0000-0000-000000000000"}, nil)},
			now:      "2026-05-01T00:00:00Z",
			wantVCPU: 90,
		},
		{
			name: "R10 a chain collapses to its last link",
			records: []RecordStatus{
				l1(),
				successor(recordB, "2026-02-01T00:00:00Z", 60, []string{recordA}, nil),
				successor(recordC, "2026-03-01T00:00:00Z", 70, []string{recordB}, nil),
			},
			now:          "2026-05-01T00:00:00Z",
			wantVCPU:     70,
			wantRejected: map[string]string{recordA: ReasonRenewed, recordB: ReasonRenewed},
		},
		{
			name: "R15 renews and supersedes in one record extinguish both sets",
			records: []RecordStatus{
				l1(),
				wl(recordC, "2026-01-01T00:00:00Z", boundary, map[string]*int64{"vCPU": i64(30)}),
				successor(recordB, "2026-02-01T00:00:00Z", 60, []string{recordA}, []string{recordC}),
			},
			now:          "2026-05-01T00:00:00Z",
			wantVCPU:     60,
			wantRejected: map[string]string{recordA: ReasonRenewed, recordC: ReasonSuperseded},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := Compute(input(ts(tc.now), oneKey(tc.records...)))
			if got := limitOf(t, res, MetricVCPU); got != tc.wantVCPU {
				t.Fatalf("vCPU = %d, want %d", got, tc.wantVCPU)
			}
			for id, reason := range tc.wantRejected {
				st := statusOf(t, res, id)
				if st.Accepted || st.Reason != reason {
					t.Fatalf("record %s: accepted=%v reason=%q, want reason %q", id, st.Accepted, st.Reason, reason)
				}
				if reason == ReasonRenewed || reason == ReasonSuperseded {
					if st.RenewedBy == "" {
						t.Fatalf("record %s: RenewedBy is empty", id)
					}
				}
			}
		})
	}
}

// A supersede cycle excludes both records rather than picking a winner.
func TestSupersedeCycleExcludesBoth(t *testing.T) {
	a := wl(recordA, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", map[string]*int64{"vCPU": i64(50)})
	a.Supersedes = []string{recordB}
	b := wl(recordB, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", map[string]*int64{"vCPU": i64(40)})
	b.Supersedes = []string{recordA}

	res := Compute(input(ts("2026-05-01T00:00:00Z"), oneKey(a, b)))
	if len(res.Limits.Values) != 0 || res.Limits.Speaking {
		t.Fatalf("limits = %+v, want nothing granted", res.Limits)
	}
	if res.Counts.Accepted != 0 {
		t.Fatalf("accepted = %d, want 0", res.Counts.Accepted)
	}
	// R8: both members of the cycle are named, so that a key set granting
	// nothing by accident does not look like one granting nothing on purpose.
	if len(res.SupersedeCycle) != 2 {
		t.Fatalf("supersedeCycle = %v, want both records", res.SupersedeCycle)
	}
}

// limitOf reads a finite limit out of a result.
func limitOf(t *testing.T, res Result, metric string) int64 {
	t.Helper()
	v, ok := res.Limits.Values[metric]
	if !ok {
		t.Fatalf("metric %q is absent from %v", metric, res.Limits.Values)
	}
	if v == nil {
		t.Fatalf("metric %q is unlimited, want a finite value", metric)
	}
	return *v
}

// Active implements half-open boundaries.
func TestActiveBoundaries(t *testing.T) {
	r := wl(recordA, "2026-01-01T00:00:00Z", "2026-07-01T00:00:00Z", nil)
	cases := []struct {
		at   string
		want bool
	}{
		{"2025-12-31T23:59:59Z", false},
		{"2026-01-01T00:00:00Z", true},
		{"2026-06-30T23:59:59Z", true},
		{"2026-07-01T00:00:00Z", false},
	}
	for _, tc := range cases {
		if got := Active(r, ts(tc.at)); got != tc.want {
			t.Fatalf("Active(%s) = %v, want %v", tc.at, got, tc.want)
		}
	}
	perpetual := wl(recordA, "2026-01-01T00:00:00Z", "", nil)
	if !Active(perpetual, ts("2999-01-01T00:00:00Z")) {
		t.Fatal("P19: a perpetual record must stay active")
	}
	if Active(RecordStatus{Record: Record{StartAt: ts("2026-01-01T00:00:00Z")}}, ts("2026-05-01T00:00:00Z")) {
		t.Fatal("a rejected record is never active")
	}
}
