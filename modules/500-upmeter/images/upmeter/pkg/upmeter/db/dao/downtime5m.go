package dao

import (
	"database/sql"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"upmeter/pkg/probe/types"
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

const SelectDowntime5mByTimeslotRange_Group = `
SELECT
  rowid, timeslot, success_seconds, fail_seconds, unknown_seconds, nodata_seconds, group_name, probe_name
FROM downtime5m
WHERE
  timeslot >= ? AND timeslot <= ? AND group = ?
`

const SelectDowntime5mByTimeslotRange_Group_Probe = `
SELECT
  rowid, timeslot, success_seconds, fail_seconds, unknown_seconds, nodata_seconds, group_name, probe_name
FROM downtime5m
WHERE
  timeslot >= ? AND timeslot <= ? AND group_name = ? AND probe_name = ?
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

var Downtime5m = NewDowntime5mDao()

type Downtime5mDao struct {
	Dbh   *sql.DB
	Table string
}

func NewDowntime5mDao() *Downtime5mDao {
	return &Downtime5mDao{Table: "downtime5m"}
}

func (d *Downtime5mDao) GetBySlotAndProbe(slot5m int64, group string, probe string) (Downtime5mEntity, error) {
	rows, err := d.Dbh.Query(SelectDowntime5mByTimeslotGroupProbe, slot5m, group, probe)
	if err != nil {
		return Downtime5mEntity{}, fmt.Errorf("select for TimeslotGroupProbe: %v", err)
	}

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
		log.Errorf("Consistency problem: more than one record selected for slot=%d, group='%s', probe='%s'", slot5m, group, probe)
	}

	return entity, nil
}

func (d *Downtime5mDao) ListByRange(from, to, step int64) ([]Downtime5mEntity, error) {
	rows, err := d.Dbh.Query(SelectDowntime5mByTimeslotRange, from, to)
	if err != nil {
		return nil, fmt.Errorf("select for TimeslotRange: %v", err)
	}

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

	rows, err := d.Dbh.Query(query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("select for TimeslotRange: %v", err)
	}

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

func (d *Downtime5mDao) ListGroupProbe() ([]types.ProbeRef, error) {
	rows, err := d.Dbh.Query(SelectDowntime5mGroupProbe)
	if err != nil {
		return nil, fmt.Errorf("select group and probe: %v", err)
	}

	var res = make([]types.ProbeRef, 0)
	for rows.Next() {
		var ref = types.ProbeRef{}
		err := rows.Scan(&ref.Group, &ref.Probe)
		if err != nil {
			return nil, fmt.Errorf("row to ProbeRef: %v", err)
		}
		res = append(res, ref)
	}

	return res, nil
}

func (d *Downtime5mDao) Save(downtime types.DowntimeEpisode) error {
	_, err := d.Dbh.Exec(InsertDowntime5m,
		downtime.TimeSlot,
		downtime.SuccessSeconds, downtime.FailSeconds,
		downtime.Unknown, downtime.NoData,
		downtime.ProbeRef.Group, downtime.ProbeRef.Probe)
	return err
}

func (d *Downtime5mDao) Update(rowid int64, downtime types.DowntimeEpisode) error {
	_, err := d.Dbh.Exec(UpdateDowntime5m,
		downtime.SuccessSeconds, downtime.FailSeconds,
		downtime.Unknown, downtime.NoData,
		rowid)
	return err
}
