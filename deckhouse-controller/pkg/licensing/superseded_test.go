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
	"time"
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
			// None of these keys ran out past grace except R6, which is the
			// last evidence of the licence and stays.
			if len(res.Expired) != 0 {
				t.Fatalf("expired = %v, want none", res.Expired)
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

// lapsed builds a record that no longer contributes: reason Expired, Renewed,
// Superseded, UnsupportedType or a real rejection. grace < 0 means the record
// carries no grace_days.
func lapsed(id, expire, reason string, grace int) RecordStatus {
	r := wl(id, "2026-01-01T00:00:00Z", expire, map[string]*int64{MetricVCPU: i64(100)})
	r.Accepted = false
	r.Reason = reason
	if grace >= 0 {
		r.GraceDays = &grace
	}
	return r
}

// Which keys ran out for good: every record, of any type, past its own
// expire_at plus grace. The default grace is 14 days and now is 2026-05-01.
func TestExpired(t *testing.T) {
	now := ts("2026-05-01T00:00:00Z")
	const noGrace = -1

	cases := []struct {
		name    string
		records []RecordStatus
		want    string // latest expire_at, empty when the key stays
	}{
		{"every record past the default grace", []RecordStatus{
			lapsed(recordA, "2026-02-01T00:00:00Z", ReasonExpired, noGrace),
			lapsed(recordB, "2026-03-01T00:00:00Z", ReasonExpired, noGrace),
		}, "2026-03-01T00:00:00Z"},
		{"one record still in the default grace", []RecordStatus{
			lapsed(recordA, "2026-02-01T00:00:00Z", ReasonExpired, noGrace),
			lapsed(recordB, "2026-04-25T00:00:00Z", ReasonExpired, noGrace),
		}, ""},
		{"a record's own longer grace keeps the key", []RecordStatus{
			lapsed(recordA, "2026-03-01T00:00:00Z", ReasonExpired, 90),
		}, ""},
		{"a record's own shorter grace lets it go", []RecordStatus{
			lapsed(recordA, "2026-04-25T00:00:00Z", ReasonExpired, 1),
		}, "2026-04-25T00:00:00Z"},
		{"grace ending exactly now is not past", []RecordStatus{
			lapsed(recordA, "2026-04-17T00:00:00Z", ReasonExpired, noGrace),
		}, ""},
		{"a perpetual record never runs out", []RecordStatus{
			lapsed(recordA, "2026-02-01T00:00:00Z", ReasonExpired, noGrace),
			lapsed(recordB, "", ReasonUnsupportedType, noGrace),
		}, ""},
		{"a record in force keeps the key", []RecordStatus{
			lapsed(recordA, "2026-02-01T00:00:00Z", ReasonExpired, noGrace),
			wl(recordB, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", map[string]*int64{MetricVCPU: i64(100)}),
		}, ""},
		{"a record of an unknown type counts like any other", []RecordStatus{
			lapsed(recordA, "2026-02-01T00:00:00Z", ReasonExpired, noGrace),
			lapsed(recordB, "2026-03-01T00:00:00Z", ReasonUnsupportedType, noGrace),
		}, "2026-03-01T00:00:00Z"},
		{"a running record of an unknown type keeps the key", []RecordStatus{
			lapsed(recordA, "2026-02-01T00:00:00Z", ReasonExpired, noGrace),
			lapsed(recordB, "2027-01-01T00:00:00Z", ReasonUnsupportedType, noGrace),
		}, ""},
		{"superseded and expired records past grace", []RecordStatus{
			lapsed(recordA, "2026-02-01T00:00:00Z", ReasonSuperseded, noGrace),
			lapsed(recordB, "2026-03-01T00:00:00Z", ReasonExpired, noGrace),
		}, "2026-03-01T00:00:00Z"},
		{"a rejected record keeps the key", []RecordStatus{
			lapsed(recordA, "2026-02-01T00:00:00Z", ReasonClusterMismatch, noGrace),
		}, ""},
		{"a revoked record keeps the key", []RecordStatus{
			lapsed(recordA, "2026-02-01T00:00:00Z", ReasonRevoked, noGrace),
		}, ""},
		{"a duplicate record keeps the key", []RecordStatus{
			lapsed(recordA, "2026-02-01T00:00:00Z", ReasonDuplicate, noGrace),
		}, ""},
		{"no records at all", nil, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			latest, got := Expired(tc.records, now, DefaultThresholds())
			if got != (tc.want != "") {
				t.Fatalf("expired = %v, want %v", got, tc.want != "")
			}
			if got && !latest.Equal(ts(tc.want)) {
				t.Fatalf("latest = %s, want %s", latest, tc.want)
			}
		})
	}
}

// A key that ran out past grace next to a key in force is listed for deletion,
// leaves the registration request in the same pass, and deleting it changes
// neither the policy nor the request.
func TestExpiredKeyNextToAKeyInForce(t *testing.T) {
	now := ts("2026-05-01T00:00:00Z")
	limits := map[string]*int64{MetricServers: i64(12), MetricVCPU: i64(200), MetricCores: i64(0)}

	both := []KeyRecords{
		{Key: "license-old", JTI: keyOldJTI, Records: []RecordStatus{
			wl(recordA, "2026-01-01T00:00:00Z", "2026-03-01T00:00:00Z", limits),
		}},
		{Key: "license-new", JTI: keyNewJTI, Records: []RecordStatus{
			wl(recordB, "2026-03-01T00:00:00Z", "2027-01-01T00:00:00Z", limits),
		}},
	}
	nodes := []Node{nd("a", 32), nd("b", 16)}

	before := Compute(input(now, both, nodes...))
	if got, ok := before.Expired["license-old"]; !ok || !got.Equal(ts("2026-03-01T00:00:00Z")) {
		t.Fatalf("expired = %v, want license-old at 2026-03-01", before.Expired)
	}
	if len(before.Expired) != 1 || len(before.Superseded) != 0 {
		t.Fatalf("expired = %v, superseded = %v", before.Expired, before.Superseded)
	}
	if !reflect.DeepEqual(before.AcceptedRecords, []string{recordB}) ||
		!reflect.DeepEqual(before.ActiveKeys, []string{keyNewJTI}) {
		t.Fatalf("records = %v, activeKeys = %v, want only the new key", before.AcceptedRecords, before.ActiveKeys)
	}

	after := Compute(input(now, both[1:], nodes...))
	if !reflect.DeepEqual(before.Limits, after.Limits) || !reflect.DeepEqual(before.Allocation, after.Allocation) {
		t.Fatalf("policy changed: %+v -> %+v", before.Limits, after.Limits)
	}
	if before.State != after.State || before.Reason != after.Reason || !reflect.DeepEqual(before.Key, after.Key) {
		t.Fatalf("state changed: %q/%q -> %q/%q", before.State, before.Reason, after.State, after.Reason)
	}
	if !reflect.DeepEqual(before.AcceptedRecords, after.AcceptedRecords) ||
		!reflect.DeepEqual(before.ActiveKeys, after.ActiveKeys) {
		t.Fatal("deleting the expired key changed the registration request")
	}
}

// While nothing is active, the key whose record the compliance state reads
// stays even past grace, so that the cluster keeps reading Violation/Expired
// rather than Unregistered. Older keys that ran out go.
func TestLastExpiredKeyIsKept(t *testing.T) {
	now := ts("2026-05-01T00:00:00Z")
	limits := map[string]*int64{MetricVCPU: i64(100)}

	res := Compute(input(now, []KeyRecords{
		{Key: "license-1", JTI: keyOldJTI, Records: []RecordStatus{
			wl(recordA, "2026-01-01T00:00:00Z", "2026-02-01T00:00:00Z", limits),
		}},
		{Key: "license-2", JTI: keyNewJTI, Records: []RecordStatus{
			wl(recordB, "2026-02-01T00:00:00Z", "2026-03-01T00:00:00Z", limits),
		}},
	}))
	if _, ok := res.Expired["license-1"]; !ok || len(res.Expired) != 1 {
		t.Fatalf("expired = %v, want only license-1", res.Expired)
	}
	if res.State != StateViolation || res.Reason != ReasonExpired {
		t.Fatalf("state = %q/%q", res.State, res.Reason)
	}
	if !reflect.DeepEqual(res.ActiveKeys, []string{keyNewJTI}) {
		t.Fatalf("activeKeys = %v, want the kept key", res.ActiveKeys)
	}
}

// A key whose every record is superseded is deleted as superseded even when
// its records also ran out past grace: the existing verdict wins.
func TestSupersededWinsOverExpired(t *testing.T) {
	limits := map[string]*int64{MetricVCPU: i64(100)}
	successor := wl(recordC, "2026-02-01T00:00:00Z", "2027-01-01T00:00:00Z", limits)
	successor.Supersedes = []string{recordA}

	res := Compute(input(ts("2026-05-01T00:00:00Z"), []KeyRecords{
		{Key: "license-1", JTI: keyOldJTI, Records: []RecordStatus{
			wl(recordA, "2026-01-01T00:00:00Z", "2026-03-01T00:00:00Z", limits),
		}},
		{Key: "license-2", JTI: keyNewJTI, Records: []RecordStatus{successor}},
	}))
	if !res.Superseded["license-1"] || len(res.Expired) != 0 {
		t.Fatalf("superseded = %v, expired = %v", res.Superseded, res.Expired)
	}
}

// Which keys another key carries verbatim. The license server copies every
// record of the previous keys that is still in force or yet to come into the
// new key, so the old key becomes redundant.
func TestCovered(t *testing.T) {
	limits := map[string]*int64{MetricVCPU: i64(100)}
	a := wl(recordA, "2026-01-01T00:00:00Z", "2026-07-01T00:00:00Z", limits)
	b := wl(recordB, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", limits)
	c := wl(recordC, "2026-07-01T00:00:00Z", "2027-07-01T00:00:00Z", limits)

	longer := a
	longer.ExpireAt = tsp("2026-08-01T00:00:00Z")
	bigger := a
	bigger.Platform = &Platform{Edition: "Core", ResourceLimits: map[string]*int64{MetricVCPU: i64(200)}}
	// The same instants written in another zone are the same record.
	shifted := a
	shifted.StartAt = a.StartAt.In(time.FixedZone("MSK", 3*3600))
	malformed := rejectedRec(recordA, ReasonSchemaViolation)

	key := func(name, jti, iat string, records ...RecordStatus) KeyRecords {
		k := KeyRecords{Key: name, JTI: jti, Records: records}
		if iat != "" {
			k.IssuedAt = ts(iat)
		}
		return k
	}
	const (
		older = "2026-01-01T00:00:00Z"
		newer = "2026-02-01T00:00:00Z"
	)

	cases := []struct {
		name string
		keys []KeyRecords
		want map[string]string
	}{
		{"the new key carries the old records and one more", []KeyRecords{
			key("license-a", keyOldJTI, older, a),
			key("license-b", keyNewJTI, newer, a, b),
		}, map[string]string{"license-a": "license-b"}},
		{"a strict superset wins whatever the iat", []KeyRecords{
			key("license-a", keyOldJTI, newer, a),
			key("license-b", keyNewJTI, older, a, b),
		}, map[string]string{"license-a": "license-b"}},
		{"identical record sets: the later iat stays", []KeyRecords{
			key("license-a", keyNewJTI, newer, a, b),
			key("license-b", keyOldJTI, older, b, a),
		}, map[string]string{"license-b": "license-a"}},
		{"identical record sets and iat: the larger jti stays", []KeyRecords{
			key("license-a", keyNewJTI, older, a),
			key("license-b", keyOldJTI, older, a),
		}, map[string]string{"license-b": "license-a"}},
		{"a chain is covered by its top", []KeyRecords{
			key("license-a", keyOldJTI, older, a),
			key("license-b", keyNewJTI, newer, a, b),
			key("license-c", testPackageJTI, "2026-03-01T00:00:00Z", a, b, c),
		}, map[string]string{"license-a": "license-c", "license-b": "license-c"}},
		{"the same instant in another zone is the same record", []KeyRecords{
			key("license-a", keyOldJTI, older, a),
			key("license-b", keyNewJTI, newer, shifted, b),
		}, map[string]string{"license-a": "license-b"}},
		{"same id with another expire_at is not a copy", []KeyRecords{
			key("license-a", keyOldJTI, older, a),
			key("license-b", keyNewJTI, newer, longer, b),
		}, map[string]string{}},
		{"same id with other limits is not a copy", []KeyRecords{
			key("license-a", keyOldJTI, older, a),
			key("license-b", keyNewJTI, newer, bigger, b),
		}, map[string]string{}},
		{"partial overlap covers nothing", []KeyRecords{
			key("license-a", keyOldJTI, older, a, b),
			key("license-b", keyNewJTI, newer, a, c),
		}, map[string]string{}},
		{"a malformed record is never covered", []KeyRecords{
			key("license-a", keyOldJTI, older, malformed),
			key("license-b", keyNewJTI, newer, malformed, b),
		}, map[string]string{}},
		{"unrelated keys", []KeyRecords{
			key("license-a", keyOldJTI, older, a),
			key("license-b", keyNewJTI, newer, b),
		}, map[string]string{}},
		{"a single key", []KeyRecords{key("license-a", keyOldJTI, older, a)}, map[string]string{}},
		{"a key without records", []KeyRecords{
			key("license-a", keyOldJTI, older),
			key("license-b", keyNewJTI, newer, a),
		}, map[string]string{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Covered(tc.keys); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("covered = %v, want %v", got, tc.want)
			}
		})
	}
}

// coveredPair is a reissue under the new issuing rule: the new key carries the
// record of the old one verbatim plus a record of its own. The old key sorts
// first, so without coverage its copy would be the accepted one and the new
// key's copy the duplicate.
func coveredPair(oldExpire string) []KeyRecords {
	limits := map[string]*int64{MetricServers: i64(12), MetricVCPU: i64(200), MetricCores: i64(0)}
	carried := wl(recordA, "2026-01-01T00:00:00Z", oldExpire, limits)
	return []KeyRecords{
		{Key: "license-a", JTI: keyOldJTI, IssuedAt: ts("2026-01-01T00:00:00Z"), Records: []RecordStatus{carried}},
		{Key: "license-b", JTI: keyNewJTI, IssuedAt: ts("2026-03-01T00:00:00Z"), Records: []RecordStatus{
			carried,
			wl(recordB, "2026-03-01T00:00:00Z", "2026-04-01T00:00:00Z", limits),
		}},
	}
}

// A covered key is listed for deletion, holds only duplicates, leaves the
// registration request in the same pass, and deleting it changes nothing.
func TestDeletingACoveredKeyChangesNothing(t *testing.T) {
	now := ts("2026-03-15T00:00:00Z")
	both := coveredPair("2026-07-01T00:00:00Z")
	nodes := []Node{nd("a", 32), nd("b", 16)}

	before := Compute(input(now, both, nodes...))
	if !reflect.DeepEqual(before.Covered, map[string]string{"license-a": "license-b"}) {
		t.Fatalf("covered = %v", before.Covered)
	}
	if len(before.Superseded) != 0 || len(before.Expired) != 0 {
		t.Fatalf("superseded = %v, expired = %v", before.Superseded, before.Expired)
	}
	// Records come in key order: license-a's copy first, then license-b's two.
	if r := before.Records[0]; r.Accepted || r.Reason != ReasonDuplicate {
		t.Fatalf("covered copy = %+v, want a duplicate", r)
	}
	if r := before.Records[1]; !r.Accepted {
		t.Fatalf("covering copy = %+v, want accepted", r)
	}
	if !reflect.DeepEqual(before.ActiveKeys, []string{keyNewJTI}) {
		t.Fatalf("activeKeys = %v, want only the covering key", before.ActiveKeys)
	}

	after := Compute(input(now, both[1:], nodes...))
	if !reflect.DeepEqual(before.Limits, after.Limits) || !reflect.DeepEqual(before.Allocation, after.Allocation) {
		t.Fatalf("policy changed: %+v -> %+v", before.Limits, after.Limits)
	}
	if before.State != after.State || before.Reason != after.Reason || !reflect.DeepEqual(before.Key, after.Key) {
		t.Fatalf("state changed: %q/%q/%+v -> %q/%q/%+v",
			before.State, before.Reason, before.Key, after.State, after.Reason, after.Key)
	}
	if !reflect.DeepEqual(before.AcceptedRecords, after.AcceptedRecords) ||
		!reflect.DeepEqual(before.ActiveKeys, after.ActiveKeys) {
		t.Fatal("deleting the covered key changed the registration request")
	}
	if !reflect.DeepEqual(before.Records[1:], after.Records) {
		t.Fatalf("records of the covering key changed:\n%+v\n%+v", before.Records[1:], after.Records)
	}
}

// Same id, different content: nothing is covered, both keys stay and the second
// copy is a Duplicate, as before.
func TestSameIDOtherContentIsNotCovered(t *testing.T) {
	both := coveredPair("2026-07-01T00:00:00Z")
	changed := both[1].Records[0]
	changed.ExpireAt = tsp("2026-08-01T00:00:00Z")
	both[1].Records = []RecordStatus{changed, both[1].Records[1]}

	res := Compute(input(ts("2026-03-15T00:00:00Z"), both))
	if len(res.Covered) != 0 {
		t.Fatalf("covered = %v, want none", res.Covered)
	}
	if r := res.Records[1]; r.ID != recordA || r.Reason != ReasonDuplicate {
		t.Fatalf("second copy = %+v, want a duplicate", r)
	}
	if !reflect.DeepEqual(res.ActiveKeys, []string{keyOldJTI, keyNewJTI}) {
		t.Fatalf("activeKeys = %v, want both", res.ActiveKeys)
	}
}

// Coverage does not get in the way of expiry. The covered key goes, and once
// everything ran out the covering key is an ordinary expired key: kept while it
// is the last evidence of the licence, deleted next to a key in force.
func TestCoveredKeyThenExpiry(t *testing.T) {
	now := ts("2026-06-01T00:00:00Z")
	both := coveredPair("2026-03-10T00:00:00Z")

	res := Compute(input(now, both))
	if !reflect.DeepEqual(res.Covered, map[string]string{"license-a": "license-b"}) {
		t.Fatalf("covered = %v", res.Covered)
	}
	if len(res.Expired) != 0 || res.State != StateViolation || res.Reason != ReasonExpired {
		t.Fatalf("expired = %v, state = %q/%q: the last evidence must stay", res.Expired, res.State, res.Reason)
	}
	if alone := Compute(input(now, both[1:])); len(alone.Expired) != 0 {
		t.Fatalf("expired = %v, the last key must stay", alone.Expired)
	}

	fresh := KeyRecords{Key: "license-c", JTI: testPackageJTI, Records: []RecordStatus{
		wl(recordC, "2026-05-01T00:00:00Z", "2027-01-01T00:00:00Z", map[string]*int64{MetricVCPU: i64(100)}),
	}}
	res = Compute(input(now, append(both, fresh)))
	if !reflect.DeepEqual(res.Covered, map[string]string{"license-a": "license-b"}) {
		t.Fatalf("covered = %v", res.Covered)
	}
	if got, ok := res.Expired["license-b"]; !ok || len(res.Expired) != 1 || !got.Equal(ts("2026-04-01T00:00:00Z")) {
		t.Fatalf("expired = %v, want license-b at 2026-04-01", res.Expired)
	}
	if !reflect.DeepEqual(res.ActiveKeys, []string{testPackageJTI}) {
		t.Fatalf("activeKeys = %v, want only the key in force", res.ActiveKeys)
	}
}
