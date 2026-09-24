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
	"time"
)

// Covered names the keys the controller deletes because another installed key
// makes them redundant. The license server issues every new key with verbatim
// copies (the same id and the same content) of the records of the previous keys
// that are still in force or yet to come, and drops the ones that already ran
// out. So a record of an old key is covered when another key carries a copy of
// it, or when it ran out past its own grace period: expire_at plus grace_days, or
// the default grace, strictly before now. A record still in grace is not covered,
// since the Grace state reads it. A key is covered when every record of it is
// covered and at least one is a copy: a key of nothing but records that ran out
// is the business of Expired and its last-key guard. The result maps each
// covered key to the key that covers it, which is never covered itself.
//
// Two keys that cover each other (the same live records) are decided by issue:
// the one issued last (by iat, then by the larger jti, then by the smaller name)
// stays. A record that shares an id with another but differs in content is not a
// copy: both keys stay and the second copy is a Duplicate. A key rejected as a
// whole is not in keys, so it is never covered and never covers.
//
// The keys are the record sets ParsePackage produced, before deduplication.
func Covered(keys []KeyRecords, now time.Time, th Thresholds) map[string]string {
	above := func(a, b KeyRecords) bool {
		if !covers(a, b, now, th) {
			return false
		}
		return !covers(b, a, now, th) || issuedLater(a, b)
	}

	covered := make(map[string]bool)
	for i := range keys {
		for j := range keys {
			if i != j && above(keys[j], keys[i]) {
				covered[keys[i].Key] = true
				break
			}
		}
	}

	// A covered key is named after the latest key above it that is not covered
	// itself. Records that ran out make "above" non-transitive, so such a key may
	// not exist; the key then stays, which is always safe, and the next pass
	// decides again on what is left.
	out := make(map[string]string, len(covered))
	for i := range keys {
		if !covered[keys[i].Key] {
			continue
		}
		var by *KeyRecords
		for j := range keys {
			c := &keys[j]
			if covered[c.Key] || !above(*c, keys[i]) {
				continue
			}
			if by == nil || issuedLater(*c, *by) {
				by = c
			}
		}
		if by != nil {
			out[keys[i].Key] = by.Key
		}
	}
	return out
}

// covers reports whether every record of b is either copied in a or ran out past
// its grace, with at least one copy.
func covers(a, b KeyRecords, now time.Time, th Thresholds) bool {
	copied := false
	for _, r := range b.Records {
		switch {
		case copiedIn(a, r):
			copied = true
		case !lapsedPastGrace(r, now, th):
			return false
		}
	}
	return copied
}

// copiedIn reports whether a carries a verbatim copy of r. A record whose
// content could not be decoded is compared on nothing but its id, so it is
// never a copy.
func copiedIn(a KeyRecords, r RecordStatus) bool {
	if r.Reason == ReasonSchemaViolation {
		return false
	}
	for _, c := range a.Records {
		if c.Reason != ReasonSchemaViolation && c.ID == r.ID && sameRecord(c.Record, r.Record) {
			return true
		}
	}
	return false
}

// lapsedPastGrace reports whether a record that passed verification, or one of
// a type this build does not know, ran out past its own grace period. A record
// rejected for a reason the customer has to act on never lapses here, the way it
// never does for Expired.
func lapsedPastGrace(r RecordStatus, now time.Time, th Thresholds) bool {
	if !r.Accepted && r.Reason != ReasonUnsupportedType {
		return false
	}
	return r.ExpireAt != nil && now.After(r.ExpireAt.Add(GraceOf(r, th)))
}

// sameRecord compares two parsed records field by field, instants by value.
func sameRecord(a, b Record) bool {
	if !a.StartAt.Equal(b.StartAt) || !sameInstant(a.ExpireAt, b.ExpireAt) {
		return false
	}
	a.StartAt, b.StartAt = time.Time{}, time.Time{}
	a.ExpireAt, b.ExpireAt = nil, nil
	return reflect.DeepEqual(a, b)
}

func sameInstant(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

// issuedLater orders keys by issue: iat, then jti, then the smaller name, so
// that the order is total and the survivor does not depend on listing order.
func issuedLater(a, b KeyRecords) bool {
	switch {
	case !a.IssuedAt.Equal(b.IssuedAt):
		return a.IssuedAt.After(b.IssuedAt)
	case a.JTI != b.JTI:
		return a.JTI > b.JTI
	default:
		return a.Key < b.Key
	}
}
