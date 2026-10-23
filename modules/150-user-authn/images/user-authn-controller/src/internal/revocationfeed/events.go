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

package revocationfeed

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"user-authn-controller/internal/revocation"
)

// version is the version of the event format. A client ignores the data of a newer one.
const version = 1

const (
	eventSnapshot   = "snapshot"
	eventRevocation = "revocation"
	eventHeartbeat  = "heartbeat"
)

// snapshotEntry is a key of a snapshot. G is set for pairs only.
type snapshotEntry struct {
	K   string `json:"k"`
	G   uint64 `json:"g,omitempty"`
	Exp int64  `json:"exp"`
}

// snapshotData is one part of a snapshot.
type snapshotData struct {
	V         int             `json:"v"`
	Part      int             `json:"part"`
	Last      bool            `json:"last"`
	Retention int64           `json:"retention"`
	Pairs     []snapshotEntry `json:"pairs"`
	Sessions  []snapshotEntry `json:"sessions"`
	Grants    []snapshotEntry `json:"grants"`
}

// revocationData is a revocation event. G is set for pairs only.
type revocationData struct {
	V    int    `json:"v"`
	Type string `json:"type"`
	K    string `json:"k"`
	G    uint64 `json:"g,omitempty"`
	Exp  int64  `json:"exp"`
	At   int64  `json:"at"`
}

type heartbeatData struct {
	V int   `json:"v"`
	T int64 `json:"t"`
}

// keyOf is the kind of the key of a revocation and what the key is made of, or false for a type
// the feed does not send.
func keyOf(rev revocation.Revocation) (string, string, bool) {
	switch rev.Type {
	case revocation.TypePair:
		return keyPair, rev.Subject, rev.Subject != ""
	case revocation.TypeSession:
		return keySession, rev.SessionID, rev.SessionID != ""
	case revocation.TypeGrant:
		return keyGrant, rev.GrantID, rev.GrantID != ""
	default:
		return "", "", false
	}
}

// recordOf is the record of a revocation, or false for one the feed does not send.
func recordOf(rev revocation.Revocation, retention time.Duration) (record, bool) {
	kind, value, ok := keyOf(rev)
	if !ok {
		return record{}, false
	}
	rec := record{
		typ:     rev.Type,
		kind:    kind,
		value:   value,
		exp:     rev.Expires(retention).Unix(),
		at:      rev.Time().Unix(),
		created: rev.Time(),
		clients: rev.Clients,
	}
	if rev.Type == revocation.TypePair {
		rec.generation = rev.Generation
	}
	if !rev.CreatedAt.IsZero() {
		// The writer's time has nanoseconds; the creation timestamp has whole seconds.
		rec.created = rev.CreatedAt
	}
	return rec, true
}

// revocationEvent is the data of the event of rec for a client with salt.
func revocationEvent(rec record, salt []byte) ([]byte, error) {
	return json.Marshal(revocationData{
		V:    version,
		Type: rec.typ,
		K:    Key(salt, rec.kind, rec.value),
		G:    rec.generation,
		Exp:  rec.exp,
		At:   rec.at,
	})
}

func heartbeatEvent(now time.Time) ([]byte, error) {
	return json.Marshal(heartbeatData{V: version, T: now.Unix()})
}

// snapshotParts builds the snapshot of revs for a client with salt: one entry per key, a pair with
// the highest generation of its revocations, every entry with the latest expiry. Expired revocations
// are left out. Each part holds at most partSize entries; there is always at least one part.
func snapshotParts(revs []revocation.Revocation, salt []byte, retention time.Duration, now time.Time, partSize int) ([][]byte, error) {
	pairs := map[string]snapshotEntry{}
	sessions := map[string]snapshotEntry{}
	grants := map[string]snapshotEntry{}
	for _, rev := range revs {
		kind, value, ok := keyOf(rev)
		if !ok {
			continue
		}
		exp := rev.Expires(retention).Unix()
		if exp < now.Unix() {
			continue
		}
		key := Key(salt, kind, value)
		var set map[string]snapshotEntry
		switch rev.Type {
		case revocation.TypePair:
			set = pairs
		case revocation.TypeSession:
			set = sessions
		default:
			set = grants
		}
		entry := set[key]
		entry.K = key
		entry.Exp = max(entry.Exp, exp)
		if rev.Type == revocation.TypePair {
			entry.G = max(entry.G, rev.Generation)
		}
		set[key] = entry
	}

	type keyed struct {
		typ   string
		entry snapshotEntry
	}
	var all []keyed
	for _, set := range []struct {
		typ     string
		entries map[string]snapshotEntry
	}{
		{typ: revocation.TypePair, entries: pairs},
		{typ: revocation.TypeSession, entries: sessions},
		{typ: revocation.TypeGrant, entries: grants},
	} {
		entries := make([]snapshotEntry, 0, len(set.entries))
		for _, e := range set.entries {
			entries = append(entries, e)
		}
		slices.SortFunc(entries, func(a, b snapshotEntry) int { return cmp.Compare(a.K, b.K) })
		for _, e := range entries {
			all = append(all, keyed{typ: set.typ, entry: e})
		}
	}

	var parts [][]byte
	for start := 0; start == 0 || start < len(all); start += partSize {
		end := min(start+partSize, len(all))
		data := snapshotData{
			V:         version,
			Part:      len(parts),
			Last:      end == len(all),
			Retention: int64(retention / time.Second),
			Pairs:     []snapshotEntry{},
			Sessions:  []snapshotEntry{},
			Grants:    []snapshotEntry{},
		}
		for _, k := range all[start:end] {
			switch k.typ {
			case revocation.TypePair:
				data.Pairs = append(data.Pairs, k.entry)
			case revocation.TypeSession:
				data.Sessions = append(data.Sessions, k.entry)
			default:
				data.Grants = append(data.Grants, k.entry)
			}
		}
		raw, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("marshal snapshot part %d: %w", len(parts), err)
		}
		parts = append(parts, raw)
	}
	return parts, nil
}
