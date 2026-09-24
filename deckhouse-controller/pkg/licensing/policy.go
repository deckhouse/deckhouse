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

// Compliance reasons, published as EffectiveLicense .status.compliance.reason.
// ReasonExpired and ReasonRevoked are shared with the record level reasons:
// same word, same meaning.
const (
	ReasonUnregistered    = "Unregistered"
	ReasonUnlicensedNodes = "UnlicensedNodes"
	ReasonExpiringSoon    = "ExpiringSoon"
)

// KeyRecords is the verified content of one ClusterLicense.
type KeyRecords struct {
	// Key is the ClusterLicense name.
	Key string
	// JTI and CustomerName come from the package envelope, not from a record.
	JTI          string
	CustomerName string
	// IssuedAt is the package iat. It only decides which of two keys carrying
	// the same records stays (see Covered); zero when it does not parse.
	IssuedAt time.Time
	Records  []RecordStatus
}

// Thresholds are the commercial knobs of the compliance state machine. They are
// parameters, not constants: sales has not approved the defaults (ADR 21).
type Thresholds struct {
	// DefaultGrace applies only to expired records that carry no grace_days.
	DefaultGrace time.Duration
	// ExpiringSoon is how long before the key expires the cluster warns.
	ExpiringSoon time.Duration
	// OverLimitWindow is how long unlicensed nodes may exist before the
	// overuse becomes a violation rather than a warning. Buying a node for an
	// afternoon is not a breach of contract.
	OverLimitWindow time.Duration
}

// DefaultThresholds returns the controller defaults: 14 days of grace, 30 days
// of expiry notice and seven days of unlicensed nodes before a violation.
func DefaultThresholds() Thresholds {
	return Thresholds{
		DefaultGrace:    14 * 24 * time.Hour,
		ExpiringSoon:    30 * 24 * time.Hour,
		OverLimitWindow: 7 * 24 * time.Hour,
	}
}

// KeyInfo describes the key in force, the way the Console shows it. The stepped
// terms of the individual records are not part of it: for the customer a
// licence is valid until one date or it is not valid at all.
type KeyInfo struct {
	Name         string
	JTI          string
	CustomerName string
	Origin       string
	// ValidUntil is the greatest expire_at of the records of the key; nil means
	// the key never expires.
	ValidUntil *time.Time
	// GraceDays comes from the record that expires last.
	GraceDays *int
}

// Input is everything Compute needs. Every field is data, so the whole policy is
// a pure function of a point in time, the installed keys and the node set.
type Input struct {
	Keys []KeyRecords
	// Nodes are the licensable nodes, free ones already excluded.
	Nodes []Node
	// PrevServers is the previous allocation, read off status.nodes[]. It only
	// breaks ties between equally sized nodes.
	PrevServers map[string]bool
	// RejectedKeys are the names of the installed keys that failed verification
	// as a whole, so they carry no records to report a reason on. They are
	// reported next to the rejected records: a key nobody can read is exactly as
	// actionable as a record nobody can read.
	RejectedKeys []string
	// OverLimitSince is the published mark of when unlicensed nodes appeared.
	// Compute returns the updated value; there is no observation journal.
	OverLimitSince *time.Time
	Now            time.Time
	Thresholds     Thresholds
}

// Result is the whole computed policy at a point in time.
type Result struct {
	State string
	// Reason names the check that produced a non-Valid State.
	Reason string
	// Licensed is the binary answer for the Console: true at Valid and Warning.
	Licensed bool

	// Limits is the aggregated quota; GrantedBy lists every record behind a
	// metric, so that the customer can see the quota survived on the second
	// grant when the first one expired.
	Limits    Limits
	GrantedBy map[string][]string
	// Unlimited lists, sorted, the metrics that ended up without a limit.
	Unlimited []string

	// Key is the key in force, nil when the cluster holds none.
	Key *KeyInfo

	Consumption map[string]int64
	Allocation  Allocation
	// OverLimitSince is the updated mark: Now on the first recompute with
	// unlicensed nodes, nil on the first one without.
	OverLimitSince *time.Time

	// Records are all records with their final verdict.
	Records []RecordStatus
	// AcceptedRecords are the sorted ids of the records that passed
	// verification and were not extinguished. They go into the registration
	// request: the license server supersedes exactly this set on a reissue.
	AcceptedRecords []string
	// ActiveKeys are the sorted jti of the installed keys that carry at least
	// one accepted record.
	ActiveKeys []string

	// Superseded is the per-key verdict of Superseded, keyed by KeyRecords.Key.
	Superseded map[string]bool
	// SupersededBy names, for every superseded key, the jti of the key that
	// extinguished it, for the KeySuperseded event.
	SupersededBy map[string]string
	// SupersedeCycle lists the record ids caught in an extinction cycle.
	SupersedeCycle []string
	// Expired names the keys the controller deletes because every record ran
	// out past its grace (see Expired), with the latest expire_at of the key,
	// for the KeyExpired event. A superseded key is never listed here, and
	// neither is the key whose record the compliance state reads while nothing
	// is active: deleting it would turn an expired licence into a cluster that
	// was never registered.
	Expired map[string]time.Time
	// Covered names the keys the controller deletes because another key carries
	// every one of their records verbatim (see Covered), with the name of that
	// key, for the KeyCovered event. A covered key is never listed as
	// superseded or expired: its records are the duplicates.
	Covered map[string]string

	// Expiring is true when the key expires within Thresholds.ExpiringSoon.
	Expiring bool
	// Rejected lists, sorted, the records rejected for a reason the customer
	// has to act on, as "<id> (<reason>)".
	Rejected []string

	Counts struct {
		Keys, Records, Accepted, Rejected, Superseded int
	}
}

// Active reports whether a record contributes to the policy at t. Boundaries
// are half-open, [start_at, expire_at): the records of one reissued key stack
// without a gap and without an overlap.
func Active(r RecordStatus, t time.Time) bool {
	return r.Accepted && !t.Before(r.StartAt) && (r.ExpireAt == nil || t.Before(*r.ExpireAt))
}

// extinguishedReasons are the verdicts that mean "a successor took over".
var extinguishedReasons = map[string]bool{
	ReasonRenewed:    true,
	ReasonSuperseded: true,
}

// Superseded reports whether every record of a key has been extinguished by an
// accepted successor whose start_at has passed (specification 8.4). Such a key
// is deleted by the controller: after a reissue it contributes nothing now and
// nothing later, and its presence would make "one key" a lie.
//
// A key that merely ran out is the business of Expired. A rejected key has to
// stay so that the customer can read the reason; a key carrying a record of an
// unknown type starts counting after a Deckhouse upgrade.
//
// The records must be the final per-record statuses Compute produced.
func Superseded(records []RecordStatus) bool {
	if len(records) == 0 {
		return false
	}
	for _, r := range records {
		if r.Accepted || !extinguishedReasons[r.Reason] {
			return false
		}
	}
	return true
}

// lapsedReasons are the verdicts of a record that can run out: it passed
// verification and expired or was taken over, or its type is unknown to this
// build, which has nothing to do with its term.
var lapsedReasons = map[string]bool{
	ReasonExpired:         true,
	ReasonRenewed:         true,
	ReasonSuperseded:      true,
	ReasonUnsupportedType: true,
}

// Expired reports whether every record of a key, of any type, ran out past its
// own grace period: expire_at plus grace_days, or plus the default grace when
// the record carries none, strictly before now. It also returns the latest
// expire_at of the key. The license server adds a new key on every reissue, so
// without this the keys accumulate for ever.
//
// A perpetual record never runs out, and a record still in grace keeps the key:
// that is what the Grace state reads. A record rejected for a reason the
// customer has to act on (revoked, cluster mismatch, duplicate, malformed)
// keeps the key too, so that the customer can read why.
//
// The records must be the final per-record statuses Compute produced.
func Expired(records []RecordStatus, now time.Time, th Thresholds) (time.Time, bool) {
	var latest time.Time
	if len(records) == 0 {
		return latest, false
	}
	for _, r := range records {
		if r.Accepted || !lapsedReasons[r.Reason] || r.ExpireAt == nil {
			return time.Time{}, false
		}
		if !now.After(r.ExpireAt.Add(GraceOf(r, th))) {
			return time.Time{}, false
		}
		if r.ExpireAt.After(latest) {
			latest = *r.ExpireAt
		}
	}
	return latest, true
}

// Compute turns the verified records of every key and the current node set into
// the effective policy.
func Compute(in Input) Result {
	// Dedup is first-wins, so the order of the keys decides which copy of a
	// record contributes. Listing order is whatever the API server returned, so
	// it is normalized here: the same key set must always produce the same
	// Result, byte for byte.
	ordered := make([]KeyRecords, len(in.Keys))
	copy(ordered, in.Keys)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Key < ordered[j].Key })

	// Coverage is decided before dedup, which visits covered keys last: the
	// copy the covering key carries is the accepted one, the covered key only
	// holds duplicates, and deleting it changes nothing.
	covered := Covered(ordered)
	live := make([]KeyRecords, 0, len(ordered))
	for _, k := range ordered {
		if _, gone := covered[k.Key]; !gone {
			live = append(live, k)
		}
	}

	base := dedup(ordered, covered)
	now := in.Now

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
		Limits:         effective,
		GrantedBy:      grantedBy,
		Records:        final,
		Consumption:    Consumption(in.Nodes),
		Allocation:     Allocate(in.Nodes, effective, in.PrevServers),
		SupersedeCycle: supersedeCycles(base),
	}
	for name, value := range effective.Values {
		if value == nil {
			res.Unlimited = append(res.Unlimited, name)
		}
	}
	sort.Strings(res.Unlimited)

	res.OverLimitSince = overLimitSince(res.Allocation, in.OverLimitSince, now)
	res.Key = keyInForce(live, final, ext)
	if res.Key != nil && res.Key.ValidUntil != nil {
		left := res.Key.ValidUntil.Sub(now)
		res.Expiring = left > 0 && left < in.Thresholds.ExpiringSoon
	}

	res.State, res.Reason = state(base, final, res, in)
	res.Licensed = res.State == StateValid || res.State == StateWarning

	res.Rejected = rejected(final, in.RejectedKeys)

	// While nothing is active the compliance state reads the record that
	// expired last. Its key stays even past grace: without it an expired
	// licence would read as a cluster that was never registered.
	var evidence *RecordStatus
	if len(activeAt(base, now)) == 0 {
		evidence = lastExpired(base, now)
	}

	// final is ordered records flattened, so each key owns the next len(Records)
	// of it. Slicing beats matching by id: the same id may legitimately appear
	// in two keys, one of them marked Duplicate.
	res.Superseded = make(map[string]bool, len(ordered))
	res.SupersededBy = make(map[string]string, len(ordered))
	res.Expired = make(map[string]time.Time)
	res.Covered = covered
	byID := make(map[string]string, len(ordered))
	for _, k := range ordered {
		byID[k.Key] = k.JTI
	}
	owner := make(map[string]string, len(base))
	at := 0
	for _, k := range ordered {
		from := at
		mine := final[at : at+len(k.Records)]
		at += len(k.Records)
		if _, gone := covered[k.Key]; gone {
			continue
		}
		for _, r := range k.Records {
			owner[r.ID] = k.Key
		}
		if Superseded(mine) {
			res.Superseded[k.Key] = true
			continue
		}
		latest, expired := Expired(mine, now, in.Thresholds)
		if !expired || holds(base[from:at], evidence) {
			continue
		}
		res.Expired[k.Key] = latest
	}
	for key := range res.Superseded {
		res.SupersededBy[key] = supersededBy(final, owner, byID, key)
	}

	// A key about to be deleted leaves the registration request in the same
	// pass, the way a superseded one does, so that the deletion changes nothing.
	res.AcceptedRecords, res.ActiveKeys = accepted(live, ext, res.Expired)

	res.Counts.Keys = len(in.Keys)
	res.Counts.Records = len(final)
	for _, r := range final {
		if r.Accepted {
			res.Counts.Accepted++
		} else {
			res.Counts.Rejected++
		}
	}
	res.Counts.Superseded = len(res.Superseded)

	return res
}

// overLimitSince keeps the single piece of state the compliance machine needs
// over time: the moment unlicensed nodes appeared. It is set on the first
// recompute that finds them and cleared by the first one that does not, so
// leaving a violation takes one pass (ADR 8.6).
func overLimitSince(a Allocation, prev *time.Time, now time.Time) *time.Time {
	if a.WithinLimits() {
		return nil
	}
	if prev != nil {
		return prev
	}
	at := now
	return &at
}

// contributing reports whether a record passed verification, survived
// deduplication and was not extinguished. Such a record is part of the policy
// now, was part of it, or will be: the license server supersedes exactly this
// set when it reissues.
func contributing(r RecordStatus, ext map[string]extinction) bool {
	if !r.Accepted {
		return false
	}
	_, gone := ext[r.ID]
	return !gone
}

// holds reports whether r points into records.
func holds(records []RecordStatus, r *RecordStatus) bool {
	for i := range records {
		if &records[i] == r {
			return true
		}
	}
	return false
}

func accepted(keys []KeyRecords, ext map[string]extinction, expired map[string]time.Time) ([]string, []string) {
	var records, activeKeys []string
	seen := make(map[string]bool)
	for _, k := range keys {
		if _, gone := expired[k.Key]; gone {
			continue
		}
		carries := false
		for _, r := range k.Records {
			if !r.Accepted || seen[r.ID] {
				continue
			}
			seen[r.ID] = true
			if _, gone := ext[r.ID]; gone {
				continue
			}
			records = append(records, r.ID)
			carries = true
		}
		if carries && k.JTI != "" {
			activeKeys = append(activeKeys, k.JTI)
		}
	}
	sort.Strings(records)
	sort.Strings(activeKeys)
	return records, activeKeys
}

// keyInForce picks the key the Console shows. With one key in the cluster the
// choice is trivial; during the moment a reissue is installed and the old key is
// not deleted yet, the one that reaches furthest into the future wins.
func keyInForce(keys []KeyRecords, final []RecordStatus, ext map[string]extinction) *KeyInfo {
	byID := make(map[string]RecordStatus, len(final))
	for _, r := range final {
		if _, taken := byID[r.ID]; !taken {
			byID[r.ID] = r
		}
	}

	var best *KeyInfo
	for _, k := range keys {
		var last *RecordStatus
		perpetual := false
		carries := false
		for i := range k.Records {
			r := k.Records[i]
			if !contributing(r, ext) {
				continue
			}
			carries = true
			if r.ExpireAt == nil {
				perpetual = true
				last = &k.Records[i]
				break
			}
			if last == nil || last.ExpireAt == nil || r.ExpireAt.After(*last.ExpireAt) {
				last = &k.Records[i]
			}
		}
		if !carries || last == nil {
			continue
		}

		info := &KeyInfo{
			Name:         k.Key,
			JTI:          k.JTI,
			CustomerName: k.CustomerName,
			Origin:       last.Origin,
			GraceDays:    last.GraceDays,
		}
		if !perpetual {
			until := *last.ExpireAt
			info.ValidUntil = &until
		}
		if best == nil || furtherThan(info, best) {
			best = info
		}
	}
	return best
}

// furtherThan orders two candidate keys: the one valid longer wins, a perpetual
// key beats every dated one, and an exact tie is broken on the object name so
// that the status does not flip between two equal keys.
func furtherThan(a, b *KeyInfo) bool {
	switch {
	case a.ValidUntil == nil && b.ValidUntil == nil:
		return a.Name < b.Name
	case a.ValidUntil == nil:
		return true
	case b.ValidUntil == nil:
		return false
	case a.ValidUntil.Equal(*b.ValidUntil):
		return a.Name < b.Name
	default:
		return a.ValidUntil.After(*b.ValidUntil)
	}
}

// supersededBy names the key that extinguished the records of a superseded key.
func supersededBy(final []RecordStatus, owner, jti map[string]string, key string) string {
	successors := make(map[string]bool)
	for _, r := range final {
		if owner[r.ID] != key || r.RenewedBy == "" {
			continue
		}
		successors[r.RenewedBy] = true
	}
	names := make([]string, 0, len(successors))
	for id := range successors {
		if name, known := owner[id]; known && name != key {
			names = append(names, jti[name])
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

// benignRejections are the reasons a record may legitimately not contribute:
// none of them is a problem the customer has to act on.
var benignRejections = map[string]bool{
	ReasonUnsupportedType: true,
	ReasonRenewed:         true,
	ReasonSuperseded:      true,
	ReasonExpired:         true,
	ReasonNotYetValid:     true,
}

func rejected(final []RecordStatus, keys []string) []string {
	out := make([]string, 0, len(final)+len(keys))
	for _, r := range final {
		if r.Accepted || benignRejections[r.Reason] {
			continue
		}
		out = append(out, fmt.Sprintf("%s (%s)", r.ID, r.Reason))
	}
	for _, key := range keys {
		out = append(out, fmt.Sprintf("key %s (Rejected)", key))
	}
	sort.Strings(out)
	return out
}

// state is the compliance verdict of specification 10.4, together with the
// reason for it. The checks are in the order the specification lists them, which
// is by severity: a violation found by any of the three beats a grace period.
func state(base, final []RecordStatus, res Result, in Input) (string, string) {
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

	if since := res.OverLimitSince; since != nil && in.Now.Sub(*since) >= in.Thresholds.OverLimitWindow {
		return StateViolation, ReasonUnlicensedNodes
	}

	if len(activeAt(base, in.Now)) == 0 {
		last := lastExpired(base, in.Now)
		// A cluster that never had a single record that passed the record level
		// checks was never registered; it did not lose rights, it never had any.
		if last == nil {
			return StateViolation, ReasonUnregistered
		}
		if in.Now.Before(last.ExpireAt.Add(GraceOf(*last, in.Thresholds))) {
			return StateGrace, ReasonExpired
		}
		return StateViolation, ReasonExpired
	}

	if !res.Allocation.WithinLimits() {
		return StateWarning, ReasonUnlicensedNodes
	}
	if res.Expiring {
		return StateWarning, ReasonExpiringSoon
	}
	return StateValid, ""
}
