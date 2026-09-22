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
				active(recordA, map[string]*int64{MetricVCPU: i64(50), MetricServers: i64(10)}),
				active(recordB, map[string]*int64{MetricVCPU: i64(40), MetricServers: i64(5)}),
			),
			wantVCPU:  i64(90),
			wantNodes: i64(15),
		},
		{
			name: "A12 three records in one package",
			keys: oneKey(
				active(recordA, map[string]*int64{MetricVCPU: i64(50)}),
				active(recordB, map[string]*int64{MetricVCPU: i64(40)}),
				active(recordC, map[string]*int64{MetricVCPU: i64(10)}),
			),
			wantVCPU: i64(100),
		},
		{
			name: "A12 three records in three packages",
			keys: []KeyRecords{
				{Key: "a", Records: []RecordStatus{active(recordA, map[string]*int64{MetricVCPU: i64(50)})}},
				{Key: "b", Records: []RecordStatus{active(recordB, map[string]*int64{MetricVCPU: i64(40)})}},
				{Key: "c", Records: []RecordStatus{active(recordC, map[string]*int64{MetricVCPU: i64(10)})}},
			},
			wantVCPU: i64(100),
		},
		{
			name: "A3 expired record does not contribute",
			keys: oneKey(
				active(recordA, map[string]*int64{MetricVCPU: i64(50)}),
				wl(recordB, "2025-01-01T00:00:00Z", "2026-01-01T00:00:00Z", map[string]*int64{MetricVCPU: i64(40)}),
			),
			wantVCPU: i64(50),
		},
		{
			name: "A6 record starting in the future does not contribute yet",
			keys: oneKey(
				active(recordA, map[string]*int64{MetricVCPU: i64(50)}),
				wl(recordB, "2026-09-01T00:00:00Z", "2027-01-01T00:00:00Z", map[string]*int64{MetricVCPU: i64(40)}),
			),
			wantVCPU: i64(50),
		},
		{
			name: "A9 the same record pasted twice counts once",
			keys: []KeyRecords{
				{Key: "a", Records: []RecordStatus{active(recordA, map[string]*int64{MetricVCPU: i64(50)})}},
				{Key: "b", Records: []RecordStatus{active(recordA, map[string]*int64{MetricVCPU: i64(50)})}},
			},
			wantVCPU: i64(50),
		},
		{
			name: "A10 explicit null limit means unlimited",
			keys: oneKey(
				active(recordA, map[string]*int64{MetricVCPU: nil}),
				active(recordB, map[string]*int64{MetricVCPU: i64(40)}),
			),
			wantVCPU:      nil,
			wantUnlimited: true,
		},
		{
			name: "A10a a record without dkp stays silent",
			keys: oneKey(
				RecordStatus{Record: Record{Type: TypePlatform, ID: recordA, StartAt: ts("2026-01-01T00:00:00Z"),
					Platform: &Platform{Expansions: []byte(`{"AdvancedStorage":{}}`)}}, Accepted: true},
				active(recordB, map[string]*int64{MetricVCPU: i64(40)}),
			),
			wantVCPU: i64(40),
		},
		{
			name: "A10b edition only does not touch the quota",
			keys: oneKey(
				RecordStatus{Record: Record{Type: TypePlatform, ID: recordA, StartAt: ts("2026-01-01T00:00:00Z"),
					Platform: &Platform{DKP: &DKPLimits{Edition: "Ultimate"}}}, Accepted: true},
				active(recordB, map[string]*int64{MetricVCPU: i64(40)}),
			),
			wantVCPU: i64(40),
		},
		{
			name: "A10c an empty map says unlimited explicitly",
			keys: oneKey(
				active(recordA, map[string]*int64{}),
				active(recordB, map[string]*int64{MetricVCPU: i64(40)}),
			),
			wantVCPU:      nil,
			wantUnlimited: true,
		},
		{
			name: "A11 a zero limit is not unlimited",
			keys: oneKey(
				active(recordA, map[string]*int64{MetricVCPU: i64(0)}),
				active(recordB, map[string]*int64{MetricVCPU: i64(40)}),
			),
			wantVCPU: i64(40),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := Compute(input(now, tc.keys))
			assertLimit(t, res, MetricVCPU, tc.wantVCPU)
			if tc.wantNodes != nil {
				assertLimit(t, res, MetricServers, tc.wantNodes)
			}
			if unlimited := len(res.Unlimited) > 0; unlimited != tc.wantUnlimited {
				t.Fatalf("Unlimited = %v, want %v", res.Unlimited, tc.wantUnlimited)
			}
		})
	}
}

func assertLimit(t *testing.T, res Result, metric string, want *int64) {
	t.Helper()
	got, ok := res.Limits.Values[metric]
	if !ok {
		t.Fatalf("metric %q is absent from %v", metric, res.Limits.Values)
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
	res := Compute(input(ts("2026-05-01T00:00:00Z"), oneKey(
		wl(recordA, "2026-01-01T00:00:00Z", "2026-07-01T00:00:00Z", map[string]*int64{MetricVCPU: i64(50)}),
		wl(recordB, "2026-01-01T00:00:00Z", "2026-11-01T00:00:00Z", map[string]*int64{MetricVCPU: i64(40)}),
	)))

	granted := strings.Join(res.GrantedBy[MetricVCPU], " ")
	for _, want := range []string{"record:" + recordA + " (+50)", "record:" + recordB + " (+40)"} {
		if !strings.Contains(granted, want) {
			t.Fatalf("grantedBy = %v, want it to mention %q", res.GrantedBy[MetricVCPU], want)
		}
	}
}

// A9: the duplicate keeps its own status with the Duplicate reason.
func TestDuplicateStatus(t *testing.T) {
	first := wl(recordA, "2026-01-01T00:00:00Z", "2026-11-01T00:00:00Z", map[string]*int64{MetricVCPU: i64(50)})
	res := Compute(input(ts("2026-05-01T00:00:00Z"), []KeyRecords{
		{Key: "a", JTI: keyOldJTI, Records: []RecordStatus{first}},
		{Key: "b", JTI: keyNewJTI, Records: []RecordStatus{first}},
	}))

	if res.Counts.Records != 2 || res.Counts.Accepted != 1 || res.Counts.Rejected != 1 {
		t.Fatalf("counts = %+v", res.Counts)
	}
	if res.Records[1].Reason != ReasonDuplicate {
		t.Fatalf("second copy: %+v", res.Records[1])
	}
	if res.Counts.Keys != 2 {
		t.Fatalf("keys = %d", res.Counts.Keys)
	}
	// A16: the duplicate contributes nothing and leaves the registration
	// request unchanged.
	if len(res.AcceptedRecords) != 1 {
		t.Fatalf("records = %v, want the record listed once", res.AcceptedRecords)
	}
}

// A12: no keys at all is a violation and grants nothing.
func TestComputeWithoutRecords(t *testing.T) {
	res := Compute(input(ts("2026-05-01T00:00:00Z"), nil, nd("a", 12)))

	if res.State != StateViolation {
		t.Fatalf("state = %q, want %q", res.State, StateViolation)
	}
	if len(res.Limits.Values) != 0 || res.Limits.Speaking {
		t.Fatalf("limits = %+v", res.Limits)
	}
	if res.Counts.Records != 0 || res.Counts.Keys != 0 {
		t.Fatalf("counts = %+v", res.Counts)
	}
}
