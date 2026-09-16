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
	"strings"
	"testing"
)

// A1, A6, A9, A10, A10a, A10b, A10c, A11, A12: aggregation of quotas.
func TestComputeEffectiveLimits(t *testing.T) {
	now := ts("2026-05-01T00:00:00Z")
	active := func(id string, limits map[string]*int64) RecordStatus {
		return wl(id, "2026-01-01T00:00:00Z", "2026-11-01T00:00:00Z", limits)
	}

	cases := []struct {
		name          string
		keys          []KeyRecords
		wantVCPU      *int64
		wantNodes     *int64
		wantUnlimited bool
	}{
		{
			name: "A1 two active records add up",
			keys: oneKey(
				active(recordA, map[string]*int64{"vCPU": i64(50), "nodes": i64(10)}),
				active(recordB, map[string]*int64{"vCPU": i64(40), "nodes": i64(5)}),
			),
			wantVCPU:  i64(90),
			wantNodes: i64(15),
		},
		{
			name: "A12 three records in one package",
			keys: oneKey(
				active(recordA, map[string]*int64{"vCPU": i64(50)}),
				active(recordB, map[string]*int64{"vCPU": i64(40)}),
				active(recordC, map[string]*int64{"vCPU": i64(10)}),
			),
			wantVCPU: i64(100),
		},
		{
			name: "A12 three records in three packages",
			keys: []KeyRecords{
				{Key: "a", Records: []RecordStatus{active(recordA, map[string]*int64{"vCPU": i64(50)})}},
				{Key: "b", Records: []RecordStatus{active(recordB, map[string]*int64{"vCPU": i64(40)})}},
				{Key: "c", Records: []RecordStatus{active(recordC, map[string]*int64{"vCPU": i64(10)})}},
			},
			wantVCPU: i64(100),
		},
		{
			name: "A3 expired record does not contribute",
			keys: oneKey(
				active(recordA, map[string]*int64{"vCPU": i64(50)}),
				wl(recordB, "2025-01-01T00:00:00Z", "2026-01-01T00:00:00Z", map[string]*int64{"vCPU": i64(40)}),
			),
			wantVCPU: i64(50),
		},
		{
			name: "A6 record starting in the future does not contribute yet",
			keys: oneKey(
				active(recordA, map[string]*int64{"vCPU": i64(50)}),
				wl(recordB, "2026-09-01T00:00:00Z", "2027-01-01T00:00:00Z", map[string]*int64{"vCPU": i64(40)}),
			),
			wantVCPU: i64(50),
		},
		{
			name: "A9 the same record pasted twice counts once",
			keys: []KeyRecords{
				{Key: "a", Records: []RecordStatus{active(recordA, map[string]*int64{"vCPU": i64(50)})}},
				{Key: "b", Records: []RecordStatus{active(recordA, map[string]*int64{"vCPU": i64(50)})}},
			},
			wantVCPU: i64(50),
		},
		{
			name: "A10 explicit null limit means unlimited",
			keys: oneKey(
				active(recordA, map[string]*int64{"vCPU": nil}),
				active(recordB, map[string]*int64{"vCPU": i64(40)}),
			),
			wantVCPU:      nil,
			wantUnlimited: true,
		},
		{
			name: "A10a a record without dkp stays silent",
			keys: oneKey(
				RecordStatus{Record: Record{Type: TypeWorkload, ID: recordA, StartAt: ts("2026-01-01T00:00:00Z"),
					Workload: &Workload{Expansions: []byte(`{"AdvancedStorage":{}}`)}}, Accepted: true},
				active(recordB, map[string]*int64{"vCPU": i64(40)}),
			),
			wantVCPU: i64(40),
		},
		{
			name: "A10b edition only does not touch the quota",
			keys: oneKey(
				RecordStatus{Record: Record{Type: TypeWorkload, ID: recordA, StartAt: ts("2026-01-01T00:00:00Z"),
					Workload: &Workload{DKP: &DKPLimits{Edition: "Ultimate"}}}, Accepted: true},
				active(recordB, map[string]*int64{"vCPU": i64(40)}),
			),
			wantVCPU: i64(40),
		},
		{
			name: "A10c an empty map says unlimited explicitly",
			keys: oneKey(
				active(recordA, map[string]*int64{}),
				active(recordB, map[string]*int64{"vCPU": i64(40)}),
			),
			wantVCPU:      nil,
			wantUnlimited: true,
		},
		{
			name: "A11 a zero limit is not unlimited",
			keys: oneKey(
				active(recordA, map[string]*int64{"vCPU": i64(0)}),
				active(recordB, map[string]*int64{"vCPU": i64(40)}),
			),
			wantVCPU: i64(40),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := Compute(tc.keys, nil, now, DefaultThresholds())
			assertLimit(t, res, "vCPU", tc.wantVCPU)
			if tc.wantNodes != nil {
				assertLimit(t, res, "nodes", tc.wantNodes)
			}
			if res.Unlimited != tc.wantUnlimited {
				t.Fatalf("Unlimited = %v, want %v", res.Unlimited, tc.wantUnlimited)
			}
		})
	}
}

func assertLimit(t *testing.T, res Result, metric string, want *int64) {
	t.Helper()
	got, ok := res.Effective[metric]
	if !ok {
		t.Fatalf("metric %q is absent from %v", metric, res.Effective)
	}
	switch {
	case want == nil && got != nil:
		t.Fatalf("%s = %d, want unlimited", metric, *got)
	case want != nil && got == nil:
		t.Fatalf("%s is unlimited, want %d", metric, *want)
	case want != nil && *got != *want:
		t.Fatalf("%s = %d, want %d", metric, *got, *want)
	}
}

// A1: every grant behind a metric is listed, so the customer sees which one
// survived when the other expires.
func TestGrantedByListsEveryGrant(t *testing.T) {
	res := Compute(oneKey(
		wl(recordA, "2026-01-01T00:00:00Z", "2026-07-01T00:00:00Z", map[string]*int64{"vCPU": i64(50)}),
		wl(recordB, "2026-01-01T00:00:00Z", "2026-11-01T00:00:00Z", map[string]*int64{"vCPU": i64(40)}),
	), nil, ts("2026-05-01T00:00:00Z"), DefaultThresholds())

	granted := strings.Join(res.GrantedBy["vCPU"], " ")
	for _, want := range []string{"record:" + recordA + " (+50)", "record:" + recordB + " (+40)"} {
		if !strings.Contains(granted, want) {
			t.Fatalf("grantedBy = %v, want it to mention %q", res.GrantedBy["vCPU"], want)
		}
	}
}

// A9: the duplicate keeps its own status with the Duplicate reason.
func TestDuplicateStatus(t *testing.T) {
	first := wl(recordA, "2026-01-01T00:00:00Z", "2026-11-01T00:00:00Z", map[string]*int64{"vCPU": i64(50)})
	res := Compute([]KeyRecords{
		{Key: "a", Records: []RecordStatus{first}},
		{Key: "b", Records: []RecordStatus{first}},
	}, nil, ts("2026-05-01T00:00:00Z"), DefaultThresholds())

	if res.Counts.Records != 2 || res.Counts.Accepted != 1 || res.Counts.Rejected != 1 {
		t.Fatalf("counts = %+v", res.Counts)
	}
	if res.Records[1].Reason != ReasonDuplicate {
		t.Fatalf("second copy: %+v", res.Records[1])
	}
	if res.Counts.Packages != 2 {
		t.Fatalf("packages = %d", res.Counts.Packages)
	}
}

// A17: no packages at all is a violation with an empty timeline.
func TestComputeWithoutRecords(t *testing.T) {
	res := Compute(nil, map[string]MetricValue{"vCPU": {Instant: 12}}, ts("2026-05-01T00:00:00Z"), DefaultThresholds())

	if res.State != StateViolation {
		t.Fatalf("state = %q, want %q", res.State, StateViolation)
	}
	if len(res.Effective) != 0 || len(res.Timeline) != 0 {
		t.Fatalf("effective = %v, timeline = %v", res.Effective, res.Timeline)
	}
	if res.Counts.Records != 0 || res.Counts.Packages != 0 {
		t.Fatalf("counts = %+v", res.Counts)
	}
}

// A4, A5, A5a, A5b, A5c, A13, A14, A15, A16: the compliance state machine.
func TestComplianceState(t *testing.T) {
	now := ts("2026-05-01T00:00:00Z")
	expiredAt := func(when string, graceDays *int) RecordStatus {
		r := wl(recordA, "2026-01-01T00:00:00Z", when, map[string]*int64{"vCPU": i64(100)})
		r.GraceDays = graceDays
		return r
	}
	live := wl(recordA, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", map[string]*int64{"vCPU": i64(100)})
	grace30 := 30
	grace5 := 5

	cases := []struct {
		name    string
		records []RecordStatus
		metrics map[string]MetricValue
		want    string
	}{
		{
			name:    "A4 expired three days ago",
			records: []RecordStatus{expiredAt("2026-04-28T00:00:00Z", nil)},
			want:    StateGrace,
		},
		{
			name:    "A5 expired twenty days ago",
			records: []RecordStatus{expiredAt("2026-04-11T00:00:00Z", nil)},
			want:    StateViolation,
		},
		{
			name:    "A5a grace_days 30, expired twenty days ago",
			records: []RecordStatus{expiredAt("2026-04-11T00:00:00Z", &grace30)},
			want:    StateGrace,
		},
		{
			name:    "A5b no grace_days, expired twenty days ago",
			records: []RecordStatus{expiredAt("2026-04-11T00:00:00Z", nil)},
			want:    StateViolation,
		},
		{
			name: "A5c the window comes from the record that expired last",
			records: []RecordStatus{
				func() RecordStatus {
					r := wl(recordA, "2026-01-01T00:00:00Z", "2026-03-01T00:00:00Z", map[string]*int64{"vCPU": i64(100)})
					r.GraceDays = &grace30
					return r
				}(),
				func() RecordStatus {
					r := wl(recordB, "2026-01-01T00:00:00Z", "2026-04-28T00:00:00Z", map[string]*int64{"vCPU": i64(100)})
					r.GraceDays = &grace5
					return r
				}(),
			},
			// 2026-04-28 + 5 days is still ahead: taking the window from the
			// record that expired first (30 days from 2026-03-01) would have
			// ended on 2026-03-31 and reported a violation.
			want: StateGrace,
		},
		{
			name:    "A13 instant at 95% of the limit is still valid",
			records: []RecordStatus{live},
			metrics: map[string]MetricValue{"vCPU": {Instant: 95, Avg7d: 80, Extrapolated: 90}},
			want:    StateValid,
		},
		{
			name:    "A13a instant exactly at the limit is still valid",
			records: []RecordStatus{live},
			metrics: map[string]MetricValue{"vCPU": {Instant: 100, Avg7d: 80, Extrapolated: 100}},
			want:    StateValid,
		},
		{
			name:    "A13b one over the limit warns",
			records: []RecordStatus{live},
			metrics: map[string]MetricValue{"vCPU": {Instant: 101, Avg7d: 80, Extrapolated: 90}},
			want:    StateWarning,
		},
		{
			name:    "A14 extrapolation above the limit",
			records: []RecordStatus{live},
			metrics: map[string]MetricValue{"vCPU": {Instant: 70, Avg7d: 68, Extrapolated: 120}},
			want:    StateWarning,
		},
		{
			name:    "instant above the limit without persistence",
			records: []RecordStatus{live},
			metrics: map[string]MetricValue{"vCPU": {Instant: 120, Avg7d: 68, Extrapolated: 70}},
			want:    StateWarning,
		},
		{
			name:    "A15 the seven day average is above the limit",
			records: []RecordStatus{live},
			metrics: map[string]MetricValue{"vCPU": {Instant: 120, Avg7d: 110, Extrapolated: 130}},
			want:    StateViolation,
		},
		{
			name:    "A16 the average came back, valid on the first recomputation",
			records: []RecordStatus{live},
			metrics: map[string]MetricValue{"vCPU": {Instant: 40, Avg7d: 42, Extrapolated: 45}},
			want:    StateValid,
		},
		{
			name:    "unlimited metrics never exceed anything",
			records: []RecordStatus{wl(recordA, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", map[string]*int64{"vCPU": nil})},
			metrics: map[string]MetricValue{"vCPU": {Instant: 1e9, Avg7d: 1e9, Extrapolated: 1e9}},
			want:    StateValid,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Compute(oneKey(tc.records...), tc.metrics, now, DefaultThresholds()).State; got != tc.want {
				t.Fatalf("state = %q, want %q", got, tc.want)
			}
		})
	}
}

// An active record close to its expiry warns, and it shows up in ExpiringSoon.
func TestExpiringSoonWarns(t *testing.T) {
	now := ts("2026-05-01T00:00:00Z")
	res := Compute(oneKey(
		wl(recordA, "2026-01-01T00:00:00Z", "2026-05-20T00:00:00Z", map[string]*int64{"vCPU": i64(100)}),
	), map[string]MetricValue{"vCPU": {Instant: 1}}, now, DefaultThresholds())

	if res.State != StateWarning {
		t.Fatalf("state = %q, want %q", res.State, StateWarning)
	}
	if len(res.ExpiringSoon) != 1 || res.ExpiringSoon[0].ID != recordA {
		t.Fatalf("expiringSoon = %+v", res.ExpiringSoon)
	}
}
