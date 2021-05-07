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
