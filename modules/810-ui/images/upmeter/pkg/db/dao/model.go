/*
Copyright 2023 Flant JSC

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
	"database/sql"
	"time"

	"d8.io/upmeter/pkg/check"
)

type Entity struct {
	Rowid   int64
	Episode check.Episode
}

// selectEntityStmt establishes the order of fields on the episode record
const selectEntityStmt = `
	SELECT
		rowid, timeslot,
		nano_up, nano_down, nano_unknown, nano_unmeasured,
		group_name, probe_name
	`

// parseEpisodeEntities relies on the order of fields in the SELECT statement in `selectEntityStmt` above
func parseEpisodeEntities(rows *sql.Rows) ([]Entity, error) {
	records := make([]Entity, 0)

	for rows.Next() {
		var ep check.Episode
		var slotUnix, rowid int64

		err := rows.Scan(
			&rowid,
			&slotUnix,

			&ep.Up,
			&ep.Down,
			&ep.Unknown,
			&ep.NoData,

			&ep.ProbeRef.Group,
			&ep.ProbeRef.Probe,
		)
		if err != nil {
			return nil, err
		}
		ep.TimeSlot = time.Unix(slotUnix, 0)

		records = append(records, Entity{Rowid: rowid, Episode: ep})
	}

	return records, nil
}

func parseEpisodesFromEntities(rows *sql.Rows) ([]check.Episode, error) {
	records, err := parseEpisodeEntities(rows)
	if err != nil {
		return nil, err
	}
	var episodes []check.Episode
	for _, r := range records {
		episodes = append(episodes, r.Episode)
	}
	return episodes, nil
}
