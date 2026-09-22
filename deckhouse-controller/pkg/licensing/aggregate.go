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

// dedup flattens the keys and marks every repeated id after the first as a
// Duplicate. Pasting the same string twice must not double the quota.
func dedup(keys []KeyRecords) []RecordStatus {
	out := make([]RecordStatus, 0)
	seen := make(map[string]bool)
	for _, k := range keys {
		for _, r := range k.Records {
			cur := r
			if cur.Accepted {
				if seen[cur.ID] {
					cur.Accepted = false
					cur.Reason = ReasonDuplicate
					cur.Message = "a record with the same id is already part of the policy"
				} else {
					seen[cur.ID] = true
				}
			}
			out = append(out, cur)
		}
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
// generic over the keys of resource_limits: no per-metric branches.
func limits(active []RecordStatus) (map[string]*int64, map[string][]string) {
	speaking := make([]RecordStatus, 0, len(active))
	for _, r := range active {
		if r.Platform != nil && r.Platform.DKP != nil && r.Platform.DKP.ResourceLimits != nil {
			speaking = append(speaking, r)
		}
	}

	names := make(map[string]bool)
	for _, r := range speaking {
		for m := range r.Platform.DKP.ResourceLimits {
			names[m] = true
		}
	}

	effective := make(map[string]*int64, len(names))
	granted := make(map[string][]string, len(names))
	for m := range names {
		var sum int64
		unlimited := false
		for _, r := range speaking {
			v, ok := r.Platform.DKP.ResourceLimits[m]
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
			effective[m] = nil
			continue
		}
		total := sum
		effective[m] = &total
	}
	return effective, granted
}

func graceOf(r RecordStatus, th Thresholds) time.Duration {
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
