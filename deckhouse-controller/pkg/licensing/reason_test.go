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

// Result.Reason names the check that produced the state, so that the caller
// does not have to reverse engineer it from the metrics.
func TestComputeReason(t *testing.T) {
	live := wl(recordA, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", map[string]*int64{"vCPU": i64(100)})
	now := ts("2026-05-01T00:00:00Z")

	cases := []struct {
		name    string
		records []RecordStatus
		metrics map[string]MetricValue
		state   string
		reason  string
	}{
		{
			name:    "valid has no reason",
			records: []RecordStatus{live},
			metrics: map[string]MetricValue{"vCPU": {Instant: 10, Avg7d: 10, Extrapolated: 10}},
			state:   StateValid,
		},
		{
			name:    "the seven day average is above the limit",
			records: []RecordStatus{live},
			metrics: map[string]MetricValue{"vCPU": {Instant: 120, Avg7d: 110, Extrapolated: 130}},
			state:   StateViolation,
			reason:  ReasonLimitsExceeded,
		},
		{
			name:    "instant at the warning ratio is still valid, with a note",
			records: []RecordStatus{live},
			metrics: map[string]MetricValue{"vCPU": {Instant: 95, Avg7d: 80, Extrapolated: 90}},
			state:   StateValid,
			reason:  ReasonLimitsApproaching,
		},
		{
			name:    "spending the whole quota is normal",
			records: []RecordStatus{live},
			metrics: map[string]MetricValue{"vCPU": {Instant: 100, Avg7d: 80, Extrapolated: 100}},
			state:   StateValid,
			reason:  ReasonLimitsApproaching,
		},
		{
			name:    "one over the limit warns",
			records: []RecordStatus{live},
			metrics: map[string]MetricValue{"vCPU": {Instant: 101, Avg7d: 80, Extrapolated: 90}},
			state:   StateWarning,
			reason:  ReasonLimitsExceeded,
		},
		{
			name:    "an unlimited metric never approaches anything",
			records: []RecordStatus{wl(recordB, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", map[string]*int64{"vCPU": nil})},
			metrics: map[string]MetricValue{"vCPU": {Instant: 1e9, Avg7d: 1e9, Extrapolated: 1e9}},
			state:   StateValid,
		},
		{
			name:    "a zero limit does not approach itself",
			records: []RecordStatus{wl(recordB, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", map[string]*int64{"vCPU": i64(0)})},
			metrics: map[string]MetricValue{"vCPU": {Instant: 0, Avg7d: 0, Extrapolated: 0}},
			state:   StateValid,
		},
		{
			name:    "only the extrapolation is above the limit",
			records: []RecordStatus{live},
			metrics: map[string]MetricValue{"vCPU": {Instant: 70, Avg7d: 68, Extrapolated: 120}},
			state:   StateWarning,
			reason:  ReasonProjectedOverLimit,
		},
		{
			name:    "an active record expires within the window",
			records: []RecordStatus{wl(recordB, "2026-01-01T00:00:00Z", "2026-05-10T00:00:00Z", map[string]*int64{"vCPU": i64(100)})},
			metrics: map[string]MetricValue{"vCPU": {Instant: 10, Avg7d: 10, Extrapolated: 10}},
			state:   StateWarning,
			reason:  ReasonExpiringSoon,
		},
		{
			name:    "everything has expired",
			records: []RecordStatus{wl(recordC, "2026-01-01T00:00:00Z", "2026-02-01T00:00:00Z", map[string]*int64{"vCPU": i64(100)})},
			metrics: map[string]MetricValue{"vCPU": {Instant: 10, Avg7d: 10, Extrapolated: 10}},
			state:   StateViolation,
			reason:  ReasonExpired,
		},
		{
			// The grace period is still expiry, told earlier: the cluster lost
			// the right, it just has not been penalised for it yet.
			name:    "the grace period is running",
			records: []RecordStatus{wl(recordC, "2026-01-01T00:00:00Z", "2026-04-25T00:00:00Z", map[string]*int64{"vCPU": i64(100)})},
			metrics: map[string]MetricValue{"vCPU": {Instant: 10, Avg7d: 10, Extrapolated: 10}},
			state:   StateGrace,
			reason:  ReasonExpired,
		},
		{
			// Nothing ever passed the record level checks: the cluster did not
			// lose its rights, it never had any.
			name:    "no record ever passed verification",
			records: []RecordStatus{rejected(recordA, ReasonClusterMismatch)},
			metrics: map[string]MetricValue{"vCPU": {Instant: 10}},
			state:   StateViolation,
			reason:  ReasonUnregistered,
		},
		{
			name:   "no keys at all",
			state:  StateViolation,
			reason: ReasonUnregistered,
		},
		{
			// A rejected record next to a live one is not an unregistered
			// cluster, and a live record is not a violation.
			name:    "a rejected record next to a live one",
			records: []RecordStatus{rejected(recordB, ReasonSchemaViolation), live},
			metrics: map[string]MetricValue{"vCPU": {Instant: 10, Avg7d: 10, Extrapolated: 10}},
			state:   StateValid,
		},
		{
			// The seven day average outranks the instant reading: sustained
			// overuse is a violation even while the cluster is back under.
			name:    "the average is over while the instant reading is not",
			records: []RecordStatus{live},
			metrics: map[string]MetricValue{"vCPU": {Instant: 50, Avg7d: 130, Extrapolated: 40}},
			state:   StateViolation,
			reason:  ReasonLimitsExceeded,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := Compute(oneKey(tc.records...), tc.metrics, now, DefaultThresholds())
			if res.State != tc.state || res.Reason != tc.reason {
				t.Fatalf("state/reason = %q/%q, want %q/%q", res.State, res.Reason, tc.state, tc.reason)
			}
		})
	}
}

// A commercial revocation outranks every other check.
func TestComputeReasonRevoked(t *testing.T) {
	revoked := wl(recordA, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", map[string]*int64{"vCPU": i64(100)})
	revoked.Accepted = false
	revoked.Reason = ReasonRevoked
	revoked.RevokedReason = ReasonNonPayment

	res := Compute(oneKey(revoked), map[string]MetricValue{"vCPU": {Instant: 1000, Avg7d: 1000}}, ts("2026-05-01T00:00:00Z"), DefaultThresholds())
	if res.State != StateViolation || res.Reason != ReasonRevoked {
		t.Fatalf("state/reason = %q/%q, want %q/%q", res.State, res.Reason, StateViolation, ReasonRevoked)
	}
}

// rejected builds a record that did not pass the record level checks.
func rejected(id, reason string) RecordStatus {
	r := wl(id, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", map[string]*int64{"vCPU": i64(100)})
	r.Accepted = false
	r.Reason = reason
	return r
}

// M6: withinLimits is the instant snapshot, reported next to the state rather
// than folded into it. A cluster may be Valid and over the limit at the same
// moment, and the two fields must say exactly that.
func TestWithinLimits(t *testing.T) {
	now := ts("2026-05-01T00:00:00Z")
	finite := wl(recordA, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", map[string]*int64{"vCPU": i64(100)})
	unlimited := wl(recordB, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", map[string]*int64{"vCPU": nil})

	cases := []struct {
		name    string
		records []RecordStatus
		metrics map[string]MetricValue
		want    bool
	}{
		{
			name:    "under the limit",
			records: []RecordStatus{finite},
			metrics: map[string]MetricValue{"vCPU": {Instant: 99}},
			want:    true,
		},
		{
			name:    "exactly at the limit is still within it",
			records: []RecordStatus{finite},
			metrics: map[string]MetricValue{"vCPU": {Instant: 100}},
			want:    true,
		},
		{
			name:    "one over",
			records: []RecordStatus{finite},
			metrics: map[string]MetricValue{"vCPU": {Instant: 101}},
			want:    false,
		},
		{
			name:    "an unlimited metric is never exceeded",
			records: []RecordStatus{unlimited},
			metrics: map[string]MetricValue{"vCPU": {Instant: 1e9}},
			want:    true,
		},
		{
			name:    "no finite limits at all",
			metrics: map[string]MetricValue{"vCPU": {Instant: 1e9}},
			want:    true,
		},
		{
			// The average is what decides the state; the instant reading is
			// what decides this field.
			name:    "the average is over but the instant reading is not",
			records: []RecordStatus{finite},
			metrics: map[string]MetricValue{"vCPU": {Instant: 10, Avg7d: 200}},
			want:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := Compute(oneKey(tc.records...), tc.metrics, now, DefaultThresholds())
			if res.WithinLimits != tc.want {
				t.Fatalf("withinLimits = %v, want %v (effective %v)", res.WithinLimits, tc.want, res.Effective)
			}
		})
	}
}
