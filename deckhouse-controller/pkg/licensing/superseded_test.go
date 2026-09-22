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

const (
	keyOldJTI = "11111111-1111-4111-8111-111111111111"
	keyNewJTI = "22222222-2222-4222-8222-222222222222"
	recordD   = "3c9f55d2-1111-4222-8333-444455556666"
)

// R-series of specification 15.6: which keys the controller may delete. Only a
// key whose every record was extinguished by an accepted successor already in
// force qualifies; the verdict is read off the Result, because it is only
// meaningful on the final per-record statuses Compute produced.
func TestSupersededRSeries(t *testing.T) {
	const now = "2026-05-01T00:00:00Z"
	limits := map[string]*int64{MetricServers: i64(10), MetricVCPU: i64(100), MetricCores: i64(0)}

	// The successor lives in its own key, which is what a reissue looks like in
	// a cluster: the customer installs the new key next to the old one.
	reissue := func(start string, supersedes []string, accepted bool) []KeyRecords {
		successor := wl(recordC, start, "2027-01-01T00:00:00Z", limits)
		successor.Supersedes = supersedes
		if !accepted {
			successor.Accepted = false
			successor.Reason = ReasonSchemaViolation
		}
		return []KeyRecords{
			{Key: "license-1", JTI: keyOldJTI, Records: []RecordStatus{
				wl(recordA, "2026-01-01T00:00:00Z", "2026-07-01T00:00:00Z", limits),
				wl(recordB, "2026-01-01T00:00:00Z", "2026-07-01T00:00:00Z", limits),
			}},
			{Key: "license-2", JTI: keyNewJTI, Records: []RecordStatus{successor}},
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
		keys []KeyRecords
		want bool
	}{
		{"R1 every record superseded by a key already in force", reissue("2026-04-01T00:00:00Z", []string{recordA, recordB}, true), true},
		{"R3 only part of the records superseded", reissue("2026-04-01T00:00:00Z", []string{recordA}, true), false},
		{"R4 the successor starts in the future", reissue("2026-09-01T00:00:00Z", []string{recordA, recordB}, true), false},
		{"R5 the successor was rejected", reissue("2026-04-01T00:00:00Z", []string{recordA, recordB}, false), false},
		{"R6 expired without a successor", oneKey(
			wl(recordA, "2026-01-01T00:00:00Z", "2026-02-01T00:00:00Z", limits),
		), false},
		{"a record of an unknown type keeps the key", oneKey(
			rejectedRec(recordA, ReasonUnsupportedType),
		), false},
		{"a rejected key is never deleted", oneKey(
			rejectedRec(recordA, ReasonClusterMismatch),
		), false},
		{"a malformed record keeps the key", oneKey(
			rejectedRec(recordA, ReasonSchemaViolation),
		), false},
		{"a revoked key is never deleted", oneKey(revoked(ReasonContractTerminated)), false},
		{"an active record keeps the key in force", oneKey(
			wl(recordA, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", limits),
		), false},
		{"a key that is not valid yet", oneKey(
			wl(recordA, "2026-09-01T00:00:00Z", "2027-01-01T00:00:00Z", limits),
		), false},
		{"no records at all", oneKey(), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := Compute(input(ts(now), tc.keys))
			if got := res.Superseded["license-1"]; got != tc.want {
				t.Fatalf("superseded = %v, want %v (records: %+v)", got, tc.want, res.Records)
			}
			want := 0
			if tc.want {
				want = 1
			}
			if res.Counts.Superseded != want {
				t.Fatalf("counts.superseded = %d, want %d", res.Counts.Superseded, want)
			}
		})
	}
}

// R7: renews extinguishes exactly the way supersedes does, and the reason says
// which of the two it was.
func TestRenewsSupersedesTheSameWay(t *testing.T) {
	limits := map[string]*int64{MetricVCPU: i64(100)}
	successor := wl(recordC, "2026-04-01T00:00:00Z", "2027-01-01T00:00:00Z", limits)
	successor.Renews = []string{recordA}

	res := Compute(input(ts("2026-05-01T00:00:00Z"), []KeyRecords{
		{Key: "license-1", JTI: keyOldJTI, Records: []RecordStatus{
			wl(recordA, "2026-01-01T00:00:00Z", "2026-07-01T00:00:00Z", limits),
		}},
		{Key: "license-2", JTI: keyNewJTI, Records: []RecordStatus{successor}},
	}))

	if !res.Superseded["license-1"] {
		t.Fatalf("a renewed key is not superseded: %+v", res.Records)
	}
	if st := statusOf(t, res, recordA); st.Reason != ReasonRenewed {
		t.Fatalf("reason = %q, want %q", st.Reason, ReasonRenewed)
	}
	if got := res.SupersededBy["license-1"]; got != keyNewJTI {
		t.Fatalf("supersededBy = %q, want %q", got, keyNewJTI)
	}
}

// R9: superseding an id that is not installed is a no-op, so the policy is the
// sum of both records.
func TestSupersedingAnAbsentRecordIsANoOp(t *testing.T) {
	limits := map[string]*int64{MetricVCPU: i64(50)}
	successor := wl(recordB, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", limits)
	successor.Supersedes = []string{recordD}

	res := Compute(input(ts("2026-05-01T00:00:00Z"), oneKey(
		wl(recordA, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", limits),
		successor,
	)))
	if got := limitOf(t, res, MetricVCPU); got != 100 {
		t.Fatalf("vCPU = %d, want 100", got)
	}
}

// R2, R10: deleting a superseded key changes neither the policy nor what goes
// into the registration request. That is the whole promise behind deleting it.
func TestDeletingASupersededKeyChangesNothing(t *testing.T) {
	now := ts("2026-05-01T00:00:00Z")
	limits := map[string]*int64{MetricServers: i64(12), MetricVCPU: i64(200), MetricCores: i64(0)}

	successor := wl(recordB, "2026-04-01T00:00:00Z", "2027-01-01T00:00:00Z", limits)
	successor.Supersedes = []string{recordA}

	both := []KeyRecords{
		{Key: "license-old", JTI: keyOldJTI, Records: []RecordStatus{
			wl(recordA, "2026-01-01T00:00:00Z", "2026-07-01T00:00:00Z", limits),
		}},
		{Key: "license-new", JTI: keyNewJTI, Records: []RecordStatus{successor}},
	}
	nodes := []Node{nd("a", 32), nd("b", 16)}

	before := Compute(input(now, both, nodes...))
	if !before.Superseded["license-old"] || before.Superseded["license-new"] {
		t.Fatalf("superseded = %v", before.Superseded)
	}
	if before.SupersededBy["license-old"] != keyNewJTI {
		t.Fatalf("supersededBy = %v", before.SupersededBy)
	}

	after := Compute(input(now, both[1:], nodes...))

	if !reflect.DeepEqual(before.Limits, after.Limits) {
		t.Fatalf("limits changed: %+v -> %+v", before.Limits, after.Limits)
	}
	if !reflect.DeepEqual(before.Allocation, after.Allocation) {
		t.Fatalf("allocation changed: %+v -> %+v", before.Allocation, after.Allocation)
	}
	if before.State != after.State || before.Reason != after.Reason {
		t.Fatalf("state changed: %q/%q -> %q/%q", before.State, before.Reason, after.State, after.Reason)
	}
	// R10: the superseded records leave the registration request, so the
	// license server supersedes only what is still installed.
	if !reflect.DeepEqual(before.AcceptedRecords, []string{recordB}) {
		t.Fatalf("records = %v, want only the successor", before.AcceptedRecords)
	}
	if !reflect.DeepEqual(before.ActiveKeys, []string{keyNewJTI}) {
		t.Fatalf("activeKeys = %v, want only the new key", before.ActiveKeys)
	}
}

// D1, D2, D3: what goes into the registration request.
func TestAcceptedRecordsAndActiveKeys(t *testing.T) {
	now := ts("2026-05-01T00:00:00Z")
	limits := map[string]*int64{MetricVCPU: i64(100)}

	t.Run("D1 no keys", func(t *testing.T) {
		res := Compute(input(now, nil))
		if len(res.AcceptedRecords) != 0 || len(res.ActiveKeys) != 0 {
			t.Fatalf("records = %v, activeKeys = %v", res.AcceptedRecords, res.ActiveKeys)
		}
	})

	t.Run("D2 one accepted key", func(t *testing.T) {
		res := Compute(input(now, oneKey(wl(recordA, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", limits))))
		if !reflect.DeepEqual(res.AcceptedRecords, []string{recordA}) {
			t.Fatalf("records = %v", res.AcceptedRecords)
		}
		if !reflect.DeepEqual(res.ActiveKeys, []string{testPackageJTI}) {
			t.Fatalf("activeKeys = %v", res.ActiveKeys)
		}
	})

	t.Run("D3 only the accepted records are listed", func(t *testing.T) {
		res := Compute(input(now, oneKey(
			wl(recordA, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", limits),
			rejectedRec(recordB, ReasonSchemaViolation),
		)))
		if !reflect.DeepEqual(res.AcceptedRecords, []string{recordA}) {
			t.Fatalf("records = %v", res.AcceptedRecords)
		}
	})

	t.Run("a record that is not valid yet still has to be superseded", func(t *testing.T) {
		res := Compute(input(now, oneKey(
			wl(recordA, "2026-01-01T00:00:00Z", "2026-09-01T00:00:00Z", limits),
			wl(recordB, "2026-09-01T00:00:00Z", "2027-01-01T00:00:00Z", limits),
		)))
		if !reflect.DeepEqual(res.AcceptedRecords, []string{recordB, recordA}) &&
			!reflect.DeepEqual(res.AcceptedRecords, []string{recordA, recordB}) {
			t.Fatalf("records = %v, want both", res.AcceptedRecords)
		}
	})
}
