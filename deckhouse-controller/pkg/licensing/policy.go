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
	"fmt"
	"sort"
	"time"
)

// Compliance states. NoUpdateRight is not computed in this iteration: Update
// records are not interpreted.
const (
	StateValid     = "Valid"
	StateWarning   = "Warning"
	StateGrace     = "Grace"
	StateViolation = "Violation"
)

// Compliance reasons. They explain a non-Valid state and are published as
// EffectiveLicense .status.compliance.reason. ReasonExpired and ReasonRevoked
// are shared with the record level reasons above: same word, same meaning.
const (
	ReasonUnregistered       = "Unregistered"
	ReasonLimitsExceeded     = "LimitsExceeded"
	ReasonExpiringSoon       = "ExpiringSoon"
	ReasonLimitsApproaching  = "LimitsApproaching"
	ReasonProjectedOverLimit = "ProjectedOverLimit"
)

// KeyRecords is the verified content of one ClusterLicense.
type KeyRecords struct {
	// Key is the ClusterLicense name.
	Key     string
	Records []RecordStatus
}

// Thresholds are the commercial knobs of the compliance state machine. They are
// parameters, not constants: sales has not approved the defaults.
type Thresholds struct {
	// DefaultGrace applies only to expired records that carry no grace_days.
	DefaultGrace time.Duration
	// WarningRatio is the share of the limit at which instant consumption warns.
	WarningRatio float64
	// ExpiringSoon is how long before its expiry an active record warns.
	ExpiringSoon time.Duration
}

// DefaultThresholds returns the controller defaults: 14 days of grace, a
// warning at 90% of the limit and 30 days of expiry notice.
func DefaultThresholds() Thresholds {
	return Thresholds{
		DefaultGrace: 14 * 24 * time.Hour,
		WarningRatio: 0.9,
		ExpiringSoon: 30 * 24 * time.Hour,
	}
}

// Segment is one interval of the policy timeline. To is nil for the open ended
// last segment. A nil value in Limits means unlimited for that metric; a metric
// missing from Limits is not granted at all.
type Segment struct {
	From   time.Time
	To     *time.Time
	State  string
	Limits map[string]*int64
}

// Reduction is the next point in time where any limit goes down.
type Reduction struct {
	At   time.Time
	From map[string]*int64
	To   map[string]*int64
}

// Result is the whole computed policy at a point in time.
type Result struct {
	State string
	// Reason names the check that produced a non-Valid State. It is empty while
	// the state is Valid.
	Reason string
	// Effective maps a metric to its limit; a nil value means unlimited.
	Effective map[string]*int64
	// Unlimited reports whether at least one metric ended up unlimited.
	Unlimited bool
	// WithinLimits reports whether instant consumption stays at or below every
	// finite limit. It is true when no finite limit is granted at all.
	WithinLimits bool
	// GrantedBy lists every record backing a metric, so that the customer can
	// see the quota survived on the second grant when the first one expired.
	GrantedBy map[string][]string
	// Records are all records with their final verdict.
	Records       []RecordStatus
	Timeline      []Segment
	NextReduction *Reduction
	ExpiringSoon  []RecordStatus
	Counts        struct {
		Packages, Records, Accepted, Rejected int
	}
}

// Active reports whether a record contributes to the policy at t. Boundaries
// are half-open, [start_at, expire_at): a renewal starting exactly when its
// predecessor expires leaves neither a gap nor an overlap.
func Active(r RecordStatus, t time.Time) bool {
	return r.Accepted && !t.Before(r.StartAt) && (r.ExpireAt == nil || t.Before(*r.ExpireAt))
}

// Compute turns the verified records of every key into the effective policy.
func Compute(keys []KeyRecords, metrics map[string]MetricValue, now time.Time, th Thresholds) Result {
	// Dedup is first-wins, so the order of the keys decides which copy of a
	// record contributes. Listing order is whatever the API server returned, so
	// it is normalized here: the same key set must always produce the same
	// Result, byte for byte.
	ordered := make([]KeyRecords, len(keys))
	copy(ordered, keys)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Key < ordered[j].Key })

	base := dedup(ordered)

	// Records that passed the record level checks and survived deduplication are
	// the input of every later stage, evaluated at whatever point in time the
	// stage cares about.
	final := make([]RecordStatus, len(base))
	copy(final, base)
	ext := extinguish(base, now)
	for i := range final {
		if !final[i].Accepted {
			continue
		}
		switch e, gone := ext[final[i].ID]; {
		case gone:
			final[i].Accepted = false
			final[i].Reason = e.reason
			final[i].RenewedBy = e.by
			final[i].Message = fmt.Sprintf("extinguished by record %s", e.by)
		case now.Before(final[i].StartAt):
			final[i].Accepted = false
			final[i].Reason = ReasonNotYetValid
			final[i].Message = fmt.Sprintf("starts at %s", final[i].StartAt.UTC().Format(time.RFC3339))
		case final[i].ExpireAt != nil && !now.Before(*final[i].ExpireAt):
			final[i].Accepted = false
			final[i].Reason = ReasonExpired
			final[i].Message = fmt.Sprintf("expired at %s", final[i].ExpireAt.UTC().Format(time.RFC3339))
		}
	}

	effective, grantedBy := limits(activeAt(base, now))

	res := Result{
		State:     StateValid,
		Effective: effective,
		GrantedBy: grantedBy,
		Records:   final,
		Timeline:  timeline(base, now, th),
	}
	for _, v := range effective {
		if v == nil {
			res.Unlimited = true
		}
	}
	res.NextReduction = nextReduction(res.Timeline)
	res.State, res.Reason = state(base, final, effective, metrics, now, th)
	res.WithinLimits = withinLimits(effective, metrics)

	for _, r := range final {
		if Active(r, now) && r.ExpireAt != nil && r.ExpireAt.Sub(now) < th.ExpiringSoon {
			res.ExpiringSoon = append(res.ExpiringSoon, r)
		}
	}

	res.Counts.Packages = len(keys)
	res.Counts.Records = len(final)
	for _, r := range final {
		if r.Accepted {
			res.Counts.Accepted++
		} else {
			res.Counts.Rejected++
		}
	}

	return res
}

// stateAt is the part of the compliance state that depends on records only, so
// it can be evaluated at any point of the timeline. Metrics are not projected
// into the future, so no Warning here.
func stateAt(base []RecordStatus, t time.Time, th Thresholds) string {
	if len(activeAt(base, t)) > 0 {
		return StateValid
	}
	last := lastExpired(base, t)
	if last == nil {
		return StateViolation
	}
	if t.Before(last.ExpireAt.Add(graceOf(*last, th))) {
		return StateGrace
	}
	return StateViolation
}

// state is the compliance verdict together with the reason for it. Checks are
// ordered by severity, and every loop over the metrics walks their names in
// sorted order so that two metrics failing different checks still produce the
// same reason on every reconcile.
func state(base, final []RecordStatus, effective map[string]*int64, metrics map[string]MetricValue, now time.Time, th Thresholds) (string, string) {
	// A commercial revocation is a violation immediately and without grace.
	// A reissue only removes the contribution of the record.
	for _, r := range final {
		if r.Reason != ReasonRevoked {
			continue
		}
		switch r.RevokedReason {
		case ReasonContractTerminated, ReasonNonPayment, ReasonAbuse:
			return StateViolation, ReasonRevoked
		}
	}

	names := make([]string, 0, len(effective))
	for name := range effective {
		names = append(names, name)
	}
	sort.Strings(names)

	// Sustained overuse. Entry needs the seven day average, exit is immediate.
	for _, name := range names {
		if over(metrics[name].Avg7d, effective[name]) {
			return StateViolation, ReasonLimitsExceeded
		}
	}

	if s := stateAt(base, now, th); s != StateValid {
		// A cluster that never had a single record that passed the record level
		// checks was never registered; it did not lose rights, it never had any.
		if s == StateViolation && !anyVerified(base) {
			return s, ReasonUnregistered
		}
		return s, ReasonExpired
	}

	for _, name := range names {
		limit := effective[name]
		m := metrics[name]
		if over(m.Instant, limit) {
			return StateWarning, ReasonLimitsApproaching
		}
		// The ratio check is skipped for a zero limit: everything is at 90% of
		// zero, and the plain over-limit check above already covers real usage.
		if limit != nil && *limit > 0 && m.Instant >= th.WarningRatio*float64(*limit) {
			return StateWarning, ReasonLimitsApproaching
		}
	}
	for _, name := range names {
		if over(metrics[name].Extrapolated, effective[name]) {
			return StateWarning, ReasonProjectedOverLimit
		}
	}
	for _, r := range final {
		if Active(r, now) && r.ExpireAt != nil && r.ExpireAt.Sub(now) < th.ExpiringSoon {
			return StateWarning, ReasonExpiringSoon
		}
	}
	return StateValid, ""
}

func over(v float64, limit *int64) bool {
	return limit != nil && v > float64(*limit)
}

// anyVerified reports whether at least one record of at least one key passed
// the record level checks of the specification.
func anyVerified(base []RecordStatus) bool {
	for _, r := range base {
		if r.Accepted {
			return true
		}
	}
	return false
}

// withinLimits is the instant consumption check on its own, reported next to
// the compliance state: the state is a verdict over time, this is a snapshot.
func withinLimits(effective map[string]*int64, metrics map[string]MetricValue) bool {
	for name, limit := range effective {
		if over(metrics[name].Instant, limit) {
			return false
		}
	}
	return true
}
