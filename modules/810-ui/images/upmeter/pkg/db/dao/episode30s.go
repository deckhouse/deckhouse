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
	"fmt"
	"strings"
	"time"

	"d8.io/upmeter/pkg/check"
	dbcontext "d8.io/upmeter/pkg/db/context"
	"d8.io/upmeter/pkg/server/ranges"
)

type EpisodeDao30s struct {
	DbCtx *dbcontext.DbContext
	Table string
}

func NewEpisodeDao30s(dbCtx *dbcontext.DbContext) *EpisodeDao30s {
	return &EpisodeDao30s{
		DbCtx: dbCtx,
		Table: "episodes_30s",
	}
}

// TODO (e.shevchenko): can be DRYed ?
func (d *EpisodeDao30s) GetBySlotAndProbe(slot time.Time, ref check.ProbeRef) (Entity, error) {
	const query = selectEntityStmt + `
	FROM
		episodes_30s
	WHERE
		timeslot = ?    AND
		group_name = ?  AND
		probe_name = ?
	`

	var entity Entity

	rows, err := d.DbCtx.StmtRunner().Query(query, slot.Unix(), ref.Group, ref.Probe)
	if err != nil {
		return entity, fmt.Errorf("cannot fetch episodes by timeslot, group_name, probe_name: %v", err)
	}
	defer rows.Close()

	records, err := parseEpisodeEntities(rows)
	if err != nil {
		return entity, err
	}
	if len(records) == 0 {
		return Entity{Rowid: -1}, nil
	}

	return records[0], nil
}

// TODO (e.shevchenko): can be DRYed ? ?
func (d *EpisodeDao30s) ListByRange(start, end time.Time, ref check.ProbeRef) ([]Entity, error) {
	const query = selectEntityStmt + `
	FROM
		episodes_30s
	WHERE
	        timeslot >= ?   AND
		timeslot < ?    AND
		group_name = ?  AND
		probe_name = ?
	`

	rows, err := d.DbCtx.StmtRunner().Query(query, start.Unix(), end.Unix(), ref.Group, ref.Probe)
	if err != nil {
		return nil, fmt.Errorf("cannot query SELECT: %v", err)
	}
	defer rows.Close()

	return parseEpisodeEntities(rows)
}

// TODO (e.shevchenko): can be DRYed ?
func (d *EpisodeDao30s) Insert(episode check.Episode) error {
	const query = `
	INSERT INTO
		episodes_30s
		(timeslot, nano_up, nano_down, nano_unknown, nano_unmeasured, group_name, probe_name)
	VALUES
	(?, ?, ?, ?, ?, ?, ?)
	`

	_, err := d.DbCtx.StmtRunner().Exec(
		query,
		episode.TimeSlot.Unix(),
		episode.Up,
		episode.Down,
		episode.Unknown,
		episode.NoData,
		episode.ProbeRef.Group,
		episode.ProbeRef.Probe,
	)
	return err
}

// TODO (e.shevchenko): can be DRYed ?
func (d *EpisodeDao30s) Update(rowid int64, episode check.Episode) error {
	const query = `
	UPDATE
		episodes_30s
	SET
		nano_up         = ?,
		nano_down       = ?,
		nano_unknown    = ?,
		nano_unmeasured = ?
	WHERE
		rowid = ?
	`

	_, err := d.DbCtx.StmtRunner().Exec(
		query,
		episode.Up,
		episode.Down,
		episode.Unknown,
		episode.NoData,
		rowid,
	)

	return err
}

// TODO (e.shevchenko): can be DRYed ? ?
func (d *EpisodeDao30s) ListBySlot(slot time.Time) ([]Entity, error) {
	const query = selectEntityStmt + `
	FROM    episodes_30s
	WHERE   timeslot = ?
	`

	rows, err := d.DbCtx.StmtRunner().Query(query, slot.Unix())
	if err != nil {
		return nil, fmt.Errorf("cannot query SELECT: %v", err)
	}
	defer rows.Close()

	return parseEpisodeEntities(rows)
}

func (d *EpisodeDao30s) ListEpisodesBySlot(slot time.Time) ([]check.Episode, error) {
	const query = selectEntityStmt + `
	FROM    episodes_30s
	WHERE   timeslot = ?
	`

	rows, err := d.DbCtx.StmtRunner().Query(query, slot.Unix())
	if err != nil {
		return nil, fmt.Errorf("cannot query SELECT: %v", err)
	}
	defer rows.Close()

	return parseEpisodesFromEntities(rows)
}

func (d *EpisodeDao30s) DeleteUpTo(slot time.Time) error {
	const query = `
	DELETE FROM episodes_30s
	WHERE timeslot <= ?
	`
	_, err := d.DbCtx.StmtRunner().Exec(query, slot.Unix())
	return err
}

func (d *EpisodeDao30s) Stats() ([]string, error) {
	const query = `
	SELECT timeslot, count(timeslot)
	FROM episodes_30s
	GROUP BY timeslot
	`

	rows, err := d.DbCtx.StmtRunner().Query(query)
	if err != nil {
		return nil, fmt.Errorf("select stats: %v", err)
	}
	defer rows.Close()

	stats := []string{}
	for rows.Next() {
		var startUnix, count int64
		rows.Scan(&startUnix, &count)
		stats = append(stats, fmt.Sprintf("%d %d", startUnix, count))
	}

	return stats, nil
}

func (d *EpisodeDao30s) SaveBatch(episodes []check.Episode) error {
	for _, ep := range episodes {
		err := d.Insert(ep)
		if err != nil {
			return fmt.Errorf("inserting episode (%s): %w", ep.String(), err)
		}
	}
	return nil
}

func (d *EpisodeDao30s) GetEarliestTimeSlot() (time.Time, error) {
	const query = `
	SELECT MIN(timeslot)
	FROM episodes_30s
	`

	slot := time.Unix(0, 0)

	rows, err := d.DbCtx.StmtRunner().Query(query)
	if err != nil {
		return slot, fmt.Errorf("select stats: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var startUnix int64
		rows.Scan(&startUnix)
		slot = time.Unix(startUnix, 0)
		break
	}

	return slot, nil
}

// ListEpisodeSumsForRanges completely duplicates the same method in EpisodeDao5m
// TODO (e.shevchenko): DRY
func (d *EpisodeDao30s) ListEpisodeSumsForRanges(rng ranges.StepRange, ref check.ProbeRef) ([]check.Episode, error) {
	res := make([]check.Episode, 0)

	queryParts := map[string]string{
		"select": `SELECT sum(nano_up), sum(nano_down), sum(nano_unknown), sum(nano_unmeasured)`,
		"from":   "FROM episodes_30s",
		"where":  "WHERE timeslot >= ? AND timeslot < ?",
	}

	for _, stepRange := range rng.Subranges {
		// Build query

		selectPart := queryParts["select"]
		where := queryParts["where"]
		var groupBy []string // GROUP BY group_name, probe_name

		queryArgs := []interface{}{
			stepRange.From,
			stepRange.To,
		}
		if ref.Group != "" {
			selectPart += ", group_name"
			where += " AND group_name = ?"
			queryArgs = append(queryArgs, ref.Group)
			groupBy = append(groupBy, "group_name")
		}

		if !areAllProbesRequested(ref.Probe) {
			// Choose specific probe
			where += " AND probe_name = ?"
			queryArgs = append(queryArgs, ref.Probe)
		}

		selectPart += ", probe_name"
		groupBy = append(groupBy, "probe_name")

		if len(groupBy) > 0 {
			where += " GROUP BY " + strings.Join(groupBy, ", ")
		}

		query := selectPart + " " + queryParts["from"] + " " + where

		// Exec and parse

		rows, err := d.DbCtx.StmtRunner().Query(query, queryArgs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			entity := Entity{}
			var err error
			if len(groupBy) == 0 {
				err = rows.Scan(
					&entity.Episode.Up,
					&entity.Episode.Down,
					&entity.Episode.Unknown,
					&entity.Episode.NoData)
			}
			if len(groupBy) == 1 {
				err = rows.Scan(
					&entity.Episode.Up,
					&entity.Episode.Down,
					&entity.Episode.Unknown,
					&entity.Episode.NoData,
					&entity.Episode.ProbeRef.Group)
			}
			if len(groupBy) == 2 {
				err = rows.Scan(
					&entity.Episode.Up,
					&entity.Episode.Down,
					&entity.Episode.Unknown,
					&entity.Episode.NoData,
					&entity.Episode.ProbeRef.Group,
					&entity.Episode.ProbeRef.Probe)
			}
			if err != nil {
				return nil, fmt.Errorf("row to entity episode: %v", err)
			}
			entity.Episode.TimeSlot = time.Unix(stepRange.From, 0)
			res = append(res, entity.Episode)
		}
	}

	return res, nil
}
