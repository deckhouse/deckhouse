package dao

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"upmeter/pkg/probe/types"
	dbcontext "upmeter/pkg/upmeter/db/context"
)

const CreateTableDowntime5m_latest = `
CREATE TABLE IF NOT EXISTS downtime5m (
	timeslot INTEGER NOT NULL,
    success_seconds INTEGER NOT NULL,
    fail_seconds INTEGER NOT NULL,
    unknown_seconds INTEGER NOT NULL,
    nodata_seconds INTEGER NOT NULL,
    group_name TEXT NOT NULL,
    probe_name TEXT NOT NULL
)
`

const SelectDowntime5mByTimeslotGroupProbe = `
SELECT
  rowid, timeslot, success_seconds, fail_seconds, unknown_seconds, nodata_seconds, group_name, probe_name
FROM downtime5m
WHERE
  timeslot = ? AND group_name = ? AND probe_name = ?
`

const SelectDowntime5mByTimeslotRange = `
SELECT
  rowid, timeslot, success_seconds, fail_seconds, unknown_seconds, nodata_seconds, group_name, probe_name
FROM downtime5m
WHERE
  timeslot >= ? AND timeslot <= ?
`

const SelectDowntime5mGroupProbe = `
SELECT DISTINCT group_name, probe_name
FROM downtime5m
ORDER BY 1, 2
`

const InsertDowntime5m = `
INSERT INTO downtime5m (timeslot, success_seconds, fail_seconds, unknown_seconds, nodata_seconds, group_name, probe_name)
VALUES
(?, ?, ?, ?, ?, ?, ?)
`

const UpdateDowntime5m = `
UPDATE downtime5m
SET
    success_seconds=?,
    fail_seconds=?,
    unknown_seconds=?,
    nodata_seconds=?
WHERE rowid=?
`

type Downtime5mEntity struct {
	Rowid           int64
	DowntimeEpisode types.DowntimeEpisode
}

type Downtime5mDao struct {
	DbCtx *dbcontext.DbContext
	Table string
}

func NewDowntime5mDao(dbCtx *dbcontext.DbContext) *Downtime5mDao {
	return &Downtime5mDao{
		DbCtx: dbCtx,
		Table: "downtime5m",
	}
}

func (d *Downtime5mDao) GetBySlotAndProbe(slot5m int64, group string, probe string) (Downtime5mEntity, error) {
	rows, err := d.DbCtx.StmtRunner().Query(SelectDowntime5mByTimeslotGroupProbe, slot5m, group, probe)
	if err != nil {
		return Downtime5mEntity{}, fmt.Errorf("select for TimeslotGroupProbe: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		// No entities found, return impossible rowid
		return Downtime5mEntity{Rowid: -1}, nil
	}

	var entity = Downtime5mEntity{}
	err = rows.Scan(&entity.Rowid,
		&entity.DowntimeEpisode.TimeSlot,
		&entity.DowntimeEpisode.SuccessSeconds,
		&entity.DowntimeEpisode.FailSeconds,
		&entity.DowntimeEpisode.Unknown,
		&entity.DowntimeEpisode.NoData,
		&entity.DowntimeEpisode.ProbeRef.Group,
		&entity.DowntimeEpisode.ProbeRef.Probe)
	if err != nil {
		return Downtime5mEntity{}, fmt.Errorf("row to Downtime5mEntity: %v", err)
	}

	// Assertion
	if rows.Next() {
		log.Errorf("Not consistent 5m data: more than one record for slot=%d, group='%s', probe='%s'", slot5m, group, probe)
	}

	return entity, nil
}

func (d *Downtime5mDao) ListByRange(from, to, step int64) ([]Downtime5mEntity, error) {
	rows, err := d.DbCtx.StmtRunner().Query(SelectDowntime5mByTimeslotRange, from, to)
	if err != nil {
		return nil, fmt.Errorf("select for TimeslotRange: %v", err)
	}
	defer rows.Close()

	var res = make([]Downtime5mEntity, 0)
	for rows.Next() {
		var entity = Downtime5mEntity{}
		err := rows.Scan(&entity.Rowid,
			&entity.DowntimeEpisode.TimeSlot,
			&entity.DowntimeEpisode.SuccessSeconds,
			&entity.DowntimeEpisode.FailSeconds,
			&entity.DowntimeEpisode.Unknown,
			&entity.DowntimeEpisode.NoData,
			&entity.DowntimeEpisode.ProbeRef.Group,
			&entity.DowntimeEpisode.ProbeRef.Probe)
		if err != nil {
			return nil, fmt.Errorf("row to Downtime5mEntity: %v", err)
		}

		res = append(res, entity)
	}

	return res, nil
}

func (d *Downtime5mDao) ListEpisodesByRange(from, to int64, groupName, probeName string) ([]types.DowntimeEpisode, error) {
	query := SelectDowntime5mByTimeslotRange
	queryArgs := []interface{}{
		from,
		to,
	}
	if groupName != "" {
		query += " AND group_name = ?"
		queryArgs = append(queryArgs, groupName)
	}

	// __all__ and __total__ probes should select all probes
	if !(strings.HasPrefix(probeName, "__") || strings.HasSuffix(probeName, "__")) {
		query += " AND probe_name = ?"
		queryArgs = append(queryArgs, probeName)
	}

	rows, err := d.DbCtx.StmtRunner().Query(query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("select for TimeslotRange: %v", err)
	}
	defer rows.Close()

	var res = make([]types.DowntimeEpisode, 0)
	for rows.Next() {
		var entity = Downtime5mEntity{}
		err := rows.Scan(&entity.Rowid,
			&entity.DowntimeEpisode.TimeSlot,
			&entity.DowntimeEpisode.SuccessSeconds,
			&entity.DowntimeEpisode.FailSeconds,
			&entity.DowntimeEpisode.Unknown,
			&entity.DowntimeEpisode.NoData,
			&entity.DowntimeEpisode.ProbeRef.Group,
			&entity.DowntimeEpisode.ProbeRef.Probe)
		if err != nil {
			return nil, fmt.Errorf("row to Downtime5mEntity: %v", err)
		}

		res = append(res, entity.DowntimeEpisode)
	}

	return res, nil
}

// ListEpisodeSumsForRanges returns sums of seconds for each group_name+probe_name to reduce
// calculations over full table.
// FIXME rewrite this quick hack code.
func (d *Downtime5mDao) ListEpisodeSumsForRanges(stepRanges StepRanges, groupName, probeName string) ([]types.DowntimeEpisode, error) {
	var res = make([]types.DowntimeEpisode, 0)

	var queryParts = map[string]string{
		"select": `SELECT
		sum(success_seconds), sum(fail_seconds), sum(unknown_seconds), sum(nodata_seconds)`,
		"from":  "FROM downtime5m",
		"where": "WHERE timeslot >= ? AND timeslot <= ?",
	}

	for _, stepRange := range stepRanges.Ranges {
		selectPart := queryParts["select"]
		where := queryParts["where"]
		groupBy := []string{} // GROUP BY group_name, probe_name

		queryArgs := []interface{}{
			stepRange[0],
			stepRange[1],
		}
		if groupName != "" {
			selectPart += ", group_name"
			where += " AND group_name = ?"
			queryArgs = append(queryArgs, groupName)
			groupBy = append(groupBy, "group_name")
		}

		// __all__ and __total__ probes should select all probes
		if !(strings.HasPrefix(probeName, "__") || strings.HasSuffix(probeName, "__")) {
			where += " AND probe_name = ?"
			queryArgs = append(queryArgs, probeName)
		}
		selectPart += ", probe_name"
		groupBy = append(groupBy, "probe_name")

		if len(groupBy) > 0 {
			where += " GROUP BY " + strings.Join(groupBy, ", ")
		}

		query := selectPart + " " + queryParts["from"] + " " + where

		rows, err := d.DbCtx.StmtRunner().Query(query, queryArgs...)
		if err != nil {
			return nil, fmt.Errorf("select for TimeslotRange: %v", err)
		}

		defer rows.Close()
		for rows.Next() {
			var entity = Downtime5mEntity{}
			var err error
			if len(groupBy) == 0 {
				err = rows.Scan(
					&entity.DowntimeEpisode.SuccessSeconds,
					&entity.DowntimeEpisode.FailSeconds,
					&entity.DowntimeEpisode.Unknown,
					&entity.DowntimeEpisode.NoData)
			}
			if len(groupBy) == 1 {
				err = rows.Scan(
					&entity.DowntimeEpisode.SuccessSeconds,
					&entity.DowntimeEpisode.FailSeconds,
					&entity.DowntimeEpisode.Unknown,
					&entity.DowntimeEpisode.NoData,
					&entity.DowntimeEpisode.ProbeRef.Group)
			}
			if len(groupBy) == 2 {
				err = rows.Scan(
					&entity.DowntimeEpisode.SuccessSeconds,
					&entity.DowntimeEpisode.FailSeconds,
					&entity.DowntimeEpisode.Unknown,
					&entity.DowntimeEpisode.NoData,
					&entity.DowntimeEpisode.ProbeRef.Group,
					&entity.DowntimeEpisode.ProbeRef.Probe)
			}
			if err != nil {
				return nil, fmt.Errorf("row to Downtime5mEntity: %v", err)
			}
			entity.DowntimeEpisode.TimeSlot = stepRange[0]
			res = append(res, entity.DowntimeEpisode)
		}
	}

	return res, nil
}

func (d *Downtime5mDao) ListGroupProbe() ([]types.ProbeRef, error) {
	rows, err := d.DbCtx.StmtRunner().Query(SelectDowntime5mGroupProbe)
	if err != nil {
		return nil, fmt.Errorf("select group and probe: %v", err)
	}
	defer rows.Close()

	var res = make([]types.ProbeRef, 0)
	for rows.Next() {
		var ref = types.ProbeRef{}
		err := rows.Scan(&ref.Group, &ref.Probe)
		if err != nil {
			return nil, fmt.Errorf("row to ProbeRef: %v", err)
		}
		res = append(res, ref)
		log.Infof("got probeRef=%s", ref.ProbeId())
	}

	return res, nil
}

func (d *Downtime5mDao) Insert(downtime types.DowntimeEpisode) error {
	_, err := d.DbCtx.StmtRunner().Exec(InsertDowntime5m,
		downtime.TimeSlot,
		downtime.SuccessSeconds, downtime.FailSeconds,
		downtime.Unknown, downtime.NoData,
		downtime.ProbeRef.Group, downtime.ProbeRef.Probe)
	return err
}

func (d *Downtime5mDao) Update(rowid int64, downtime types.DowntimeEpisode) error {
	_, err := d.DbCtx.StmtRunner().Exec(UpdateDowntime5m,
		downtime.SuccessSeconds, downtime.FailSeconds,
		downtime.Unknown, downtime.NoData,
		rowid)
	return err
}
