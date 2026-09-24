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

// dedup marks every later occurrence of a record id Duplicate: pasting the same
// string twice must not double the quota. The records of a covered key are
// visited last, so the copy the covering key carries is the one that counts. A
// record of a covered key that nobody else carries ran out past its grace (see
// Covered) and is marked Expired, so that a covered key contributes nothing and
// deleting it changes nothing. The output keeps the order of keys: each key owns
// the next len(Records) of it.
func dedup(keys []KeyRecords, covered map[string]string) []RecordStatus {
	seen := make(map[string]bool)
	perKey := make([][]RecordStatus, len(keys))
	for _, last := range []bool{false, true} {
		for i, k := range keys {
			if _, c := covered[k.Key]; c != last {
				continue
			}
			for _, r := range k.Records {
				cur := r
				switch {
				case !cur.Accepted:
				case seen[cur.ID]:
					cur.Accepted = false
					cur.Reason = ReasonDuplicate
					cur.Message = "a record with the same id is already part of the policy"
				case last:
					cur.Accepted = false
					cur.Reason = ReasonExpired
					cur.Message = expiredMessage(cur)
				default:
					seen[cur.ID] = true
				}
				perKey[i] = append(perKey[i], cur)
			}
		}
	}
	out := make([]RecordStatus, 0)
	for _, records := range perKey {
		out = append(out, records...)
	}
	return out
}

type extinction struct {
	by     string
	reason string
}

// extinguish computes the set of records extinguished at t. A successor whose
// start_at has come extinguishes everything it renews or supersedes, whatever
// the state of the predecessor and whatever the state of the successor itself:
// that is what makes a chain collapse to its last link and a cycle collapse to
// nothing.
func extinguish(base []RecordStatus, t time.Time) map[string]extinction {
	out := make(map[string]extinction)
	for _, s := range base {
		if !s.Accepted || t.Before(s.StartAt) {
			continue
		}
		for _, id := range s.Renews {
			claim(out, id, extinction{by: s.ID, reason: ReasonRenewed})
		}
		for _, id := range s.Supersedes {
			claim(out, id, extinction{by: s.ID, reason: ReasonSuperseded})
		}
	}
	return out
}

// claim records who extinguished id. Several successors extinguishing the same
// predecessor is ambiguous, and any of them is a correct answer, so the tie is
// broken on the successor id: whichever order the keys arrived in, the status
// names the same successor.
func claim(out map[string]extinction, id string, e extinction) {
	if cur, taken := out[id]; taken && cur.by <= e.by {
		return
	}
	out[id] = e
}

// supersedeCycles returns the sorted ids of the records that extinguish each
// other in a cycle. Every one of them is already excluded by extinguish, since a
// cycle extinguishes all of its members; naming them is diagnostics, so that a
// key set that quietly grants nothing does not look like one that grants nothing
// on purpose (specification 9.7).
func supersedeCycles(base []RecordStatus) []string {
	successors := make(map[string][]string, len(base))
	for _, r := range base {
		if !r.Accepted {
			continue
		}
		successors[r.ID] = append(append([]string(nil), r.Renews...), r.Supersedes...)
	}

	const (
		visiting = 1
		done     = 2
	)
	mark := make(map[string]int, len(successors))
	onCycle := make(map[string]bool)

	var walk func(id string) bool
	walk = func(id string) bool {
		switch mark[id] {
		case visiting:
			onCycle[id] = true
			return true
		case done:
			return onCycle[id]
		}
		mark[id] = visiting
		found := false
		for _, next := range successors[id] {
			if walk(next) {
				onCycle[id] = true
				found = true
			}
		}
		mark[id] = done
		return found
	}
	for _, r := range base {
		if r.Accepted {
			walk(r.ID)
		}
	}

	ids := make([]string, 0, len(onCycle))
	for id := range onCycle {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func activeAt(base []RecordStatus, t time.Time) []RecordStatus {
	ext := extinguish(base, t)
	out := make([]RecordStatus, 0, len(base))
	for _, r := range base {
		if _, gone := ext[r.ID]; gone {
			continue
		}
		if Active(r, t) {
			out = append(out, r)
		}
	}
	return out
}

// limits aggregates the numeric quotas of the platform scope. The walk is
// generic over the keys of resource_limits: no per-metric branches here. What an
// absent metric means is decided by Limits.Of, next to the allocation, which is
// the only place that knows the three metric names.
func limits(active []RecordStatus) (Limits, map[string][]string) {
	speaking := make([]RecordStatus, 0, len(active))
	for _, r := range active {
		if r.Platform != nil && r.Platform.ResourceLimits != nil {
			speaking = append(speaking, r)
		}
	}

	names := make(map[string]bool)
	for _, r := range speaking {
		for m := range r.Platform.ResourceLimits {
			names[m] = true
		}
	}

	values := make(map[string]*int64, len(names))
	granted := make(map[string][]string, len(names))
	for m := range names {
		var sum int64
		unlimited := false
		for _, r := range speaking {
			v, ok := r.Platform.ResourceLimits[m]
			if !ok || v == nil {
				unlimited = true
				granted[m] = append(granted[m], fmt.Sprintf("record:%s (unlimited)", r.ID))
				continue
			}
			sum += *v
			granted[m] = append(granted[m], fmt.Sprintf("record:%s (+%d)", r.ID, *v))
		}
		// GrantedBy is published verbatim, so it is sorted: the entries carry
		// the record id first, so this orders them by id.
		sort.Strings(granted[m])
		if unlimited {
			values[m] = nil
			continue
		}
		total := sum
		values[m] = &total
	}
	return Limits{Values: values, Speaking: len(speaking) > 0}, granted
}

// GraceOf is the grace window of one record: its own grace_days, or the
// controller default when the record carries none.
func GraceOf(r RecordStatus, th Thresholds) time.Duration {
	if r.GraceDays != nil {
		return time.Duration(*r.GraceDays) * 24 * time.Hour
	}
	return th.DefaultGrace
}

// lastExpired returns the record that expired last before t, which is the one
// whose grace_days defines the grace window.
func lastExpired(base []RecordStatus, t time.Time) *RecordStatus {
	var last *RecordStatus
	for i := range base {
		r := &base[i]
		if !r.Accepted || r.ExpireAt == nil || t.Before(*r.ExpireAt) {
			continue
		}
		if last == nil || r.ExpireAt.After(*last.ExpireAt) {
			last = r
		}
	}
	return last
}
