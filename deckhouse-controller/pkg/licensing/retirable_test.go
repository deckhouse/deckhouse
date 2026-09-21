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
	"reflect"
	"testing"
)

// K-series of the spec: which keys the customer may delete without touching the
// policy. The verdict is always read off the Result, because Retirable is only
// meaningful on the final per-record statuses Compute produced.
func TestRetirableKSeries(t *testing.T) {
	const now = "2026-05-01T00:00:00Z"
	limits := map[string]*int64{"vCPU": i64(100)}

	// The successor lives in its own key, which is what renewal looks like in a
	// cluster: the customer installs a new key next to the old one.
	renewedBy := func(start string) []KeyRecords {
		successor := wl(recordB, start, "2027-01-01T00:00:00Z", limits)
		successor.Renews = []string{recordA}
		return []KeyRecords{
			{Key: "license-1", Records: []RecordStatus{wl(recordA, "2026-01-01T00:00:00Z", "2026-07-01T00:00:00Z", limits)}},
			{Key: "license-2", Records: []RecordStatus{successor}},
		}
	}

	revoked := func(reason string) RecordStatus {
		r := wl(recordA, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", limits)
		r.Accepted = false
		r.Reason = ReasonRevoked
		r.RevokedReason = reason
		return r
	}

	cases := []struct {
		name string
		// keys under test; the verdict is read off license-1.
		keys []KeyRecords
		want bool
	}{
		{"K1 single expired record", oneKey(
			wl(recordA, "2026-01-01T00:00:00Z", "2026-02-01T00:00:00Z", limits),
		), true},
		{"K2 renewed by a successor already in force", renewedBy("2026-04-01T00:00:00Z"), true},
		{"K3 successor starts in the future", renewedBy("2026-09-01T00:00:00Z"), false},
		{"K4 not yet valid", oneKey(
			wl(recordA, "2026-09-01T00:00:00Z", "2027-01-01T00:00:00Z", limits),
		), false},
		{"K5 all records of an unsupported type", oneKey(
			rejected(recordA, ReasonUnsupportedType),
			rejected(recordB, ReasonUnsupportedType),
		), false},
		{"K5a one unsupported record among expired ones", oneKey(
			wl(recordA, "2026-01-01T00:00:00Z", "2026-02-01T00:00:00Z", limits),
			rejected(recordB, ReasonUnsupportedType),
		), false},
		{"K6 revoked, contract terminated", oneKey(revoked(ReasonContractTerminated)), true},
		{"K6a revoked, reissued", oneKey(revoked(ReasonReissued)), true},
		{"rejected for a foreign cluster", oneKey(rejected(recordA, ReasonClusterMismatch)), true},
		{"rejected as a duplicate", oneKey(rejected(recordA, ReasonDuplicate)), true},
		{"rejected on schema", oneKey(rejected(recordA, ReasonSchemaViolation)), true},
		{"an active record keeps the key in use", oneKey(
			wl(recordA, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", limits),
		), false},
		{"no records at all", oneKey(), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := Compute(tc.keys, nil, nil, ts(now), DefaultThresholds())
			if got := res.Retirable["license-1"]; got != tc.want {
				t.Fatalf("retirable = %v, want %v (records: %v)", got, tc.want, res.Records)
			}

			want := 0
			if tc.want {
				want = 1
			}
			if res.Counts.Retirable != want {
				t.Fatalf("counts.retirable = %d, want %d", res.Counts.Retirable, want)
			}
		})
	}
}

// K7: deleting a retirable key changes neither the policy nor the timeline, so
// the renews reference the deleted key carried becomes a no-op. This is the
// whole promise behind offering the deletion at all.
func TestRetirableDeletionIsANoOp(t *testing.T) {
	now := ts("2026-05-01T00:00:00Z")
	limits := map[string]*int64{"vCPU": i64(100)}

	old := wl(recordA, "2026-01-01T00:00:00Z", "2026-07-01T00:00:00Z", limits)
	successor := wl(recordB, "2026-04-01T00:00:00Z", "2027-01-01T00:00:00Z", limits)
	successor.Renews = []string{recordA}

	both := []KeyRecords{
		{Key: "license-old", Records: []RecordStatus{old}},
		{Key: "license-new", Records: []RecordStatus{successor}},
	}

	full := Compute(both, nil, nil, now, DefaultThresholds())
	if !full.Retirable["license-old"] {
		t.Fatalf("the renewed key is not retirable: %v", full.Records)
	}
	if full.Retirable["license-new"] {
		t.Fatalf("the key in force is retirable")
	}

	pruned := Compute(both[1:], nil, nil, now, DefaultThresholds())

	if !reflect.DeepEqual(full.Effective, pruned.Effective) {
		t.Fatalf("effective limits changed: %v -> %v", full.Effective, pruned.Effective)
	}
	if !reflect.DeepEqual(full.Timeline, pruned.Timeline) {
		t.Fatalf("timeline changed: %v -> %v", full.Timeline, pruned.Timeline)
	}
	if !reflect.DeepEqual(full.NextReduction, pruned.NextReduction) {
		t.Fatalf("next reduction changed: %v -> %v", full.NextReduction, pruned.NextReduction)
	}
	if full.State != pruned.State || full.Reason != pruned.Reason {
		t.Fatalf("state changed: %q/%q -> %q/%q", full.State, full.Reason, pruned.State, pruned.Reason)
	}
}
