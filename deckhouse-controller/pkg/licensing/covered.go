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
// carries every one of their records verbatim: the same id and the same content.
// The license server issues every new key with copies of all the records of the
// previous keys that are still in force or yet to come, so after a reissue the
// old key is redundant. The result maps each covered key to the key that covers
// it, which is never covered itself.
//
// Two keys with the same record set cover each other; the one issued last (by
// iat, then by the larger jti, then by the smaller name) stays. A record that
// shares an id with another but differs in content covers nothing: both keys
// stay and the second copy is a Duplicate. A key rejected as a whole is not in
// keys, so it is never covered and never covers.
//
// The keys are the record sets ParsePackage produced, before deduplication.
func Covered(keys []KeyRecords) map[string]string {
	above := func(a, b KeyRecords) bool {
		if !carriesAll(a, b) {
			return false
		}
		return !carriesAll(b, a) || issuedLater(a, b)
	}

	out := make(map[string]string)
	for i := range keys {
		for j := range keys {
			if i != j && above(keys[j], keys[i]) {
				out[keys[i].Key] = ""
				break
			}
		}
	}

	// "Above" is a strict partial order, so every covered key has a maximal key
	// above it, and a maximal key is not covered. The latest of them is named.
	for i := range keys {
		if _, covered := out[keys[i].Key]; !covered {
			continue
		}
		var by *KeyRecords
		for j := range keys {
			c := &keys[j]
			if _, taken := out[c.Key]; taken || !above(*c, keys[i]) {
				continue
			}
			if by == nil || issuedLater(*c, *by) {
				by = c
			}
		}
		if by == nil { // unreachable while "above" stays a strict order
			delete(out, keys[i].Key)
			continue
		}
		out[keys[i].Key] = by.Key
	}
	return out
}

// carriesAll reports whether a carries a verbatim copy of every record of b.
// A record whose content could not be decoded is compared on nothing but its id,
// so it neither covers nor is covered.
func carriesAll(a, b KeyRecords) bool {
	if len(b.Records) == 0 {
		return false
	}
	for _, r := range b.Records {
		if r.Reason == ReasonSchemaViolation {
			return false
		}
		found := false
		for _, c := range a.Records {
			if c.Reason != ReasonSchemaViolation && c.ID == r.ID && sameRecord(c.Record, r.Record) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
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
