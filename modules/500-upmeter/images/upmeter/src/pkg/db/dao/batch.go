/*
Copyright 2021 Flant JSC

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

package dao

import (
	"fmt"
	"strings"

	"d8.io/upmeter/pkg/check"
	dbcontext "d8.io/upmeter/pkg/db/context"
)

// upsertChunkSize bounds how many episodes go into a single multi-row statement. With 7 bound
// parameters per row this stays well below SQLite's variable limit while drastically reducing the
// number of round-trips compared to a statement per episode.
const upsertChunkSize = 100

// upsert30sEpisodes inserts or overwrites the given 30s episodes in batched multi-row statements.
// The values must already be final (merged): a conflicting row is overwritten with them. The unique
// index on (timeslot, group_name, probe_name) drives the conflict resolution.
func upsert30sEpisodes(ctx *dbcontext.DbContext, episodes []check.Episode) error {
	for _, chunk := range chunkEpisodes(episodes) {
		query := `
		INSERT INTO episodes_30s
			(timeslot, nano_up, nano_down, nano_unknown, nano_unmeasured, group_name, probe_name)
		VALUES ` + valuesPlaceholders(len(chunk)) + `
		ON CONFLICT(timeslot, group_name, probe_name) DO UPDATE SET
			nano_up         = excluded.nano_up,
			nano_down       = excluded.nano_down,
			nano_unknown    = excluded.nano_unknown,
			nano_unmeasured = excluded.nano_unmeasured`

		if _, err := ctx.StmtRunner().Exec(query, episodeArgs(chunk)...); err != nil {
			return fmt.Errorf("upsert into episodes_30s: %w", err)
		}
	}
	return nil
}

// upsert5mEpisodes inserts or overwrites the given 5m episodes in batched multi-row statements.
// The values must already be final (merged): a conflicting row is overwritten with them. The unique
// index on (timeslot, group_name, probe_name) drives the conflict resolution.
func upsert5mEpisodes(ctx *dbcontext.DbContext, episodes []check.Episode) error {
	for _, chunk := range chunkEpisodes(episodes) {
		query := `
		INSERT INTO episodes_5m
			(timeslot, nano_up, nano_down, nano_unknown, nano_unmeasured, group_name, probe_name)
		VALUES ` + valuesPlaceholders(len(chunk)) + `
		ON CONFLICT(timeslot, group_name, probe_name) DO UPDATE SET
			nano_up         = excluded.nano_up,
			nano_down       = excluded.nano_down,
			nano_unknown    = excluded.nano_unknown,
			nano_unmeasured = excluded.nano_unmeasured`

		if _, err := ctx.StmtRunner().Exec(query, episodeArgs(chunk)...); err != nil {
			return fmt.Errorf("upsert into episodes_5m: %w", err)
		}
	}
	return nil
}

// chunkEpisodes splits episodes into slices of at most upsertChunkSize elements.
func chunkEpisodes(episodes []check.Episode) [][]check.Episode {
	chunks := make([][]check.Episode, 0, len(episodes)/upsertChunkSize+1)
	for start := 0; start < len(episodes); start += upsertChunkSize {
		end := start + upsertChunkSize
		if end > len(episodes) {
			end = len(episodes)
		}
		chunks = append(chunks, episodes[start:end])
	}
	return chunks
}

// valuesPlaceholders returns "(?, ?, ?, ?, ?, ?, ?), ..." with one group per row.
func valuesPlaceholders(rows int) string {
	const oneRow = "(?, ?, ?, ?, ?, ?, ?)"
	groups := make([]string, rows)
	for i := range groups {
		groups[i] = oneRow
	}
	return strings.Join(groups, ", ")
}

// episodeArgs flattens episodes into bind arguments matching the column order of the insert.
func episodeArgs(episodes []check.Episode) []interface{} {
	args := make([]interface{}, 0, len(episodes)*7)
	for _, ep := range episodes {
		args = append(args,
			ep.TimeSlot.Unix(),
			ep.Up,
			ep.Down,
			ep.Unknown,
			ep.NoData,
			ep.ProbeRef.Group,
			ep.ProbeRef.Probe,
		)
	}
	return args
}
