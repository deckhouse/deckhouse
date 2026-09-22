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
	"time"
)

// full builds a record carrying all three metrics, the way the license server
// issues them.
func full(id, start, expire string, servers, vcpu, cores int64) RecordStatus {
	return wl(id, start, expire, map[string]*int64{
		MetricServers: i64(servers), MetricVCPU: i64(vcpu), MetricCores: i64(cores),
	})
}

// specKey is the three record key of specification 9.1: the paid remainder of
// the previous key, the top-up to the new profile, and the new term.
func specKey() []RecordStatus {
	return []RecordStatus{
		full(recordA, "2026-09-22T12:00:00Z", "2026-10-22T00:00:00Z", 10, 100, 0),
		full(recordB, "2026-09-22T12:00:00Z", "2026-10-22T00:00:00Z", 2, 100, 0),
		full(recordC, "2026-10-22T00:00:00Z", "2027-03-22T00:00:00Z", 12, 200, 0),
	}
}

// A1, A2: the sum of the active records is the target profile at every point of
// the term, and the half-open boundaries leave no step at the seam.
func TestAggregationOverTheReissuedKey(t *testing.T) {
	for _, when := range []string{"2026-09-23T00:00:00Z", "2026-10-22T00:00:00Z", "2027-01-01T00:00:00Z"} {
		t.Run(when, func(t *testing.T) {
			res := Compute(input(ts(when), oneKey(specKey()...)))
			assertLimit(t, res, MetricServers, i64(12))
			assertLimit(t, res, MetricVCPU, i64(200))
		})
	}
}

// A1: both grants of a metric are listed, so that the customer can see which one
// carried the quota once the other expired.
func TestGrantedByNamesEveryRecord(t *testing.T) {
	res := Compute(input(ts("2026-09-23T00:00:00Z"), oneKey(specKey()...)))
	if got := res.GrantedBy[MetricServers]; len(got) != 2 {
		t.Fatalf("grantedBy[servers] = %v, want two entries", got)
	}
}

// A17: the key is valid until the last of its records expires; the steps inside
// it are not shown to the customer.
func TestKeyValidUntilIsTheLastExpiry(t *testing.T) {
	res := Compute(input(ts("2026-09-23T00:00:00Z"), oneKey(specKey()...)))
	if res.Key == nil || res.Key.ValidUntil == nil {
		t.Fatalf("key = %+v", res.Key)
	}
	if want := ts("2027-03-22T00:00:00Z"); !res.Key.ValidUntil.Equal(want) {
		t.Fatalf("validUntil = %s, want %s", res.Key.ValidUntil, want)
	}
	if res.Key.JTI != testPackageJTI {
		t.Fatalf("jti = %q", res.Key.JTI)
	}
}

// A3, A4, A5, A6: the grace window comes from the record that expired last.
func TestGraceWindow(t *testing.T) {
	now := ts("2026-05-01T00:00:00Z")
	expiredAt := func(id, when string, graceDays *int) RecordStatus {
		r := full(id, "2026-01-01T00:00:00Z", when, 10, 100, 0)
		r.GraceDays = graceDays
		return r
	}
	grace30, grace5 := 30, 5

	cases := []struct {
		name     string
		records  []RecordStatus
		state    string
		licensed bool
	}{
		{
			name:    "A3 expired three days ago, default grace",
			records: []RecordStatus{expiredAt(recordA, "2026-04-28T00:00:00Z", nil)},
			state:   StateGrace,
		},
		{
			name:    "A4 expired twenty days ago",
			records: []RecordStatus{expiredAt(recordA, "2026-04-11T00:00:00Z", nil)},
			state:   StateViolation,
		},
		{
			name:    "A5 grace_days 30, expired twenty days ago",
			records: []RecordStatus{expiredAt(recordA, "2026-04-11T00:00:00Z", &grace30)},
			state:   StateGrace,
		},
		{
			name: "A6 the window comes from the record that expired last",
			records: []RecordStatus{
				expiredAt(recordA, "2026-03-01T00:00:00Z", &grace30),
				expiredAt(recordB, "2026-04-28T00:00:00Z", &grace5),
			},
			state: StateGrace,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := Compute(input(now, oneKey(tc.records...)))
			if res.State != tc.state || res.Reason != ReasonExpired {
				t.Fatalf("state/reason = %q/%q, want %q/%q", res.State, res.Reason, tc.state, ReasonExpired)
			}
			if res.Licensed != tc.licensed {
				t.Fatalf("licensed = %v, want %v", res.Licensed, tc.licensed)
			}
		})
	}
}

// A7, A8, A9, A10: unlicensed nodes warn immediately and become a violation only
// after they have been there for the whole window. There is no journal: one
// timestamp in the status carries the whole rule.
func TestUnlicensedNodesOverTime(t *testing.T) {
	now := ts("2026-05-01T00:00:00Z")
	// One server licence for two nodes: the smaller one is not covered.
	keys := oneKey(full(recordA, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", 1, 0, 0))
	over := []Node{nd("big", 32), nd("small", 16)}

	first := Compute(input(now, keys, over...))
	if first.State != StateWarning || first.Reason != ReasonUnlicensedNodes {
		t.Fatalf("A7 state/reason = %q/%q", first.State, first.Reason)
	}
	if first.OverLimitSince == nil || !first.OverLimitSince.Equal(now) {
		t.Fatalf("A7 overLimitSince = %v, want %s", first.OverLimitSince, now)
	}
	if !first.Licensed {
		t.Fatalf("A7 a warning is still licensed")
	}

	sixDays := input(now.Add(6*24*time.Hour), keys, over...)
	sixDays.OverLimitSince = first.OverLimitSince
	if res := Compute(sixDays); res.State != StateWarning {
		t.Fatalf("A8 state = %q, want %q", res.State, StateWarning)
	}

	sevenDays := input(now.Add(7*24*time.Hour), keys, over...)
	sevenDays.OverLimitSince = first.OverLimitSince
	violation := Compute(sevenDays)
	if violation.State != StateViolation || violation.Reason != ReasonUnlicensedNodes {
		t.Fatalf("A9 state/reason = %q/%q", violation.State, violation.Reason)
	}
	if violation.Licensed {
		t.Fatalf("A9 a violation is not licensed")
	}

	recovered := input(now.Add(8*24*time.Hour), keys, nd("big", 32))
	recovered.OverLimitSince = violation.OverLimitSince
	back := Compute(recovered)
	if back.State != StateValid || back.OverLimitSince != nil {
		t.Fatalf("A10 state = %q, overLimitSince = %v", back.State, back.OverLimitSince)
	}
}

// A11: an expiring key warns, and the warning does not turn the cluster off.
func TestKeyExpiringWarns(t *testing.T) {
	now := ts("2026-05-01T00:00:00Z")
	res := Compute(input(now, oneKey(full(recordA, "2026-01-01T00:00:00Z", "2026-05-30T00:00:00Z", 10, 100, 0)), nd("a", 8)))

	if res.State != StateWarning || res.Reason != ReasonExpiringSoon {
		t.Fatalf("state/reason = %q/%q", res.State, res.Reason)
	}
	if !res.Expiring || !res.Licensed {
		t.Fatalf("expiring = %v, licensed = %v", res.Expiring, res.Licensed)
	}
}

// A12: a cluster with no key at all is in violation, holds no key info, and
// still has to be able to register: every licensable node is unlicensed.
func TestNoKeysAtAll(t *testing.T) {
	res := Compute(input(ts("2026-05-01T00:00:00Z"), nil, nd("a", 8), nd("b", 4)))

	if res.State != StateViolation || res.Reason != ReasonUnregistered || res.Licensed {
		t.Fatalf("state/reason/licensed = %q/%q/%v", res.State, res.Reason, res.Licensed)
	}
	if res.Key != nil {
		t.Fatalf("key = %+v, want none", res.Key)
	}
	if len(res.Allocation.Unlicensed) != 2 || len(res.ActiveKeys) != 0 {
		t.Fatalf("allocation = %+v, activeKeys = %v", res.Allocation, res.ActiveKeys)
	}
}

// A commercial revocation outranks everything else and skips the grace period.
func TestCommercialRevocationIsAnImmediateViolation(t *testing.T) {
	revoked := full(recordA, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", 10, 100, 0)
	revoked.Accepted = false
	revoked.Reason = ReasonRevoked
	revoked.RevokedReason = ReasonNonPayment

	res := Compute(input(ts("2026-05-01T00:00:00Z"), oneKey(revoked)))
	if res.State != StateViolation || res.Reason != ReasonRevoked {
		t.Fatalf("state/reason = %q/%q", res.State, res.Reason)
	}
}

// P14: a record revoked because it was reissued contributes nothing, and that is
// all: the cluster is not in breach of anything.
func TestReissueRevocationIsNotAViolation(t *testing.T) {
	reissued := full(recordA, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", 10, 100, 0)
	reissued.Accepted = false
	reissued.Reason = ReasonRevoked
	reissued.RevokedReason = ReasonReissued

	live := full(recordB, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", 12, 200, 0)

	res := Compute(input(ts("2026-05-01T00:00:00Z"), oneKey(reissued, live), nd("a", 8)))
	if res.State != StateValid {
		t.Fatalf("state = %q, want %q", res.State, StateValid)
	}
	assertLimit(t, res, MetricServers, i64(12))
}

// A rejected record next to a live one is neither an unregistered cluster nor a
// violation, but it is reported so that the customer can fix it.
func TestRejectedRecordNextToALiveOne(t *testing.T) {
	res := Compute(input(ts("2026-05-01T00:00:00Z"), oneKey(
		rejectedRec(recordB, ReasonSchemaViolation),
		full(recordA, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", 10, 100, 0),
	), nd("a", 8)))

	if res.State != StateValid {
		t.Fatalf("state = %q", res.State)
	}
	if len(res.Rejected) != 1 {
		t.Fatalf("rejected = %v", res.Rejected)
	}
}

// Every benign rejection stays out of the list the customer is asked to act on.
func TestBenignRejectionsAreNotReported(t *testing.T) {
	for _, reason := range []string{ReasonUnsupportedType, ReasonExpired, ReasonNotYetValid, ReasonSuperseded, ReasonRenewed} {
		t.Run(reason, func(t *testing.T) {
			res := Compute(input(ts("2026-05-01T00:00:00Z"), oneKey(rejectedRec(recordA, reason))))
			if len(res.Rejected) != 0 {
				t.Fatalf("rejected = %v", res.Rejected)
			}
		})
	}
}
