package dao

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"upmeter/pkg/check"
	dbcontext "upmeter/pkg/upmeter/db/context"
)

const CreateTableDowntime30s_latest = `
CREATE TABLE IF NOT EXISTS downtime30s (
	timeslot        INTEGER NOT NULL,
	success_seconds INTEGER NOT NULL,
	fail_seconds    INTEGER NOT NULL,
	unknown_seconds INTEGER NOT NULL,
	nodata_seconds  INTEGER NOT NULL,
	group_name      TEXT    NOT NULL,
	probe_name      TEXT    NOT NULL
)
`

type Downtime30sDao struct {
	DbCtx    *dbcontext.DbContext
	ConnPool *dbcontext.ConnPool
	Table    string
}

func NewDowntime30sDao(dbCtx *dbcontext.DbContext) *Downtime30sDao {
	return &Downtime30sDao{
		DbCtx: dbCtx,
		Table: "downtime30s",
	}
}

type Downtime30sEntity struct {
	Rowid           int64
	DowntimeEpisode check.DowntimeEpisode
}

func (d *Downtime30sDao) ListByTimestamp(slot int64) ([]Downtime30sEntity, error) {
	const SelectDowntime30SecByTimeslot = `
	SELECT  rowid, timeslot, 
		success_seconds, fail_seconds, unknown_seconds, nodata_seconds, 
		group_name, probe_name
	FROM downtime30s
	WHERE timeslot = ?
	`

	rows, err := d.DbCtx.StmtRunner().Query(SelectDowntime30SecByTimeslot, slot)
	if err != nil {
		return nil, fmt.Errorf("cannot query SELECT: %v", err)
	}
	defer rows.Close()

	var res = make([]Downtime30sEntity, 0)
	for rows.Next() {
		var entity = Downtime30sEntity{}
		err := rows.Scan(
			&entity.Rowid,
			&entity.DowntimeEpisode.TimeSlot,
			&entity.DowntimeEpisode.SuccessSeconds,
			&entity.DowntimeEpisode.FailSeconds,
			&entity.DowntimeEpisode.UnknownSeconds,
			&entity.DowntimeEpisode.NoDataSeconds,
			&entity.DowntimeEpisode.ProbeRef.Group,
			&entity.DowntimeEpisode.ProbeRef.Probe,
		)
		if err != nil {
			return nil, fmt.Errorf("cannot parse: %v", err)
		}
		res = append(res, entity)
	}

	return res, nil
}

func (d *Downtime30sDao) GetSimilar(downtime check.DowntimeEpisode) (Downtime30sEntity, error) {
	const SelectDowntime30SecByTimeslotGroupProbe = `
	SELECT  rowid, timeslot, 
		success_seconds, fail_seconds, unknown_seconds, nodata_seconds, 
		group_name, probe_name
	FROM downtime30s
	WHERE   timeslot = ?    AND 
		group_name = ?  AND 
		probe_name = ?
	`

	rows, err := d.DbCtx.StmtRunner().Query(SelectDowntime30SecByTimeslotGroupProbe, downtime.TimeSlot, downtime.ProbeRef.Group, downtime.ProbeRef.Probe)
	if err != nil {
		return Downtime30sEntity{}, fmt.Errorf("select for timestamp: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		// No entities found, return impossible rowid
		return Downtime30sEntity{Rowid: -1}, nil
	}

	var entity = Downtime30sEntity{}
	err = rows.Scan(
		&entity.Rowid,
		&entity.DowntimeEpisode.TimeSlot,
		&entity.DowntimeEpisode.SuccessSeconds,
		&entity.DowntimeEpisode.FailSeconds,
		&entity.DowntimeEpisode.UnknownSeconds,
		&entity.DowntimeEpisode.NoDataSeconds,
		&entity.DowntimeEpisode.ProbeRef.Group,
		&entity.DowntimeEpisode.ProbeRef.Probe,
	)
	if err != nil {
		return Downtime30sEntity{}, fmt.Errorf("row to Downtime30sEntity: %v", err)
	}

	// Assertion
	if rows.Next() {
		log.Warnf("inconsistent 30s data: more than one record selected for ts=%d %s", downtime.TimeSlot, downtime.ProbeRef.Id())
	}

	return entity, nil
}

func (d *Downtime30sDao) ListForRange(start int64, end int64, ref check.ProbeRef) ([]Downtime30sEntity, error) {
	const SelectDowntime30SecByTimeslotRange = `
	SELECT
	  rowid, timeslot, success_seconds, fail_seconds, unknown_seconds, nodata_seconds, group_name, probe_name
	FROM downtime30s
	WHERE
	      timeslot >= ? AND timeslot < ?
	      AND group_name = ? AND probe_name = ?
	`

	rows, err := d.DbCtx.StmtRunner().Query(SelectDowntime30SecByTimeslotRange, start, end, ref.Group, ref.Probe)
	if err != nil {
		return nil, fmt.Errorf("cannot query SELECT: %v", err)
	}
	defer rows.Close()

	var res = make([]Downtime30sEntity, 0)
	for rows.Next() {
		var entity = Downtime30sEntity{}
		err := rows.Scan(
			&entity.Rowid,
			&entity.DowntimeEpisode.TimeSlot,
			&entity.DowntimeEpisode.SuccessSeconds,
			&entity.DowntimeEpisode.FailSeconds,
			&entity.DowntimeEpisode.UnknownSeconds,
			&entity.DowntimeEpisode.NoDataSeconds,
			&entity.DowntimeEpisode.ProbeRef.Group,
			&entity.DowntimeEpisode.ProbeRef.Probe,
		)
		if err != nil {
			return nil, fmt.Errorf("cannot parse: %v", err)
		}
		res = append(res, entity)
	}

	return res, nil
}

func (d *Downtime30sDao) ListGroupProbe() ([]check.ProbeRef, error) {
	const SelectDowntime30SecGroupProbe = `
	SELECT DISTINCT group_name, probe_name
	FROM downtime30s
	ORDER BY 1, 2
	`
	rows, err := d.DbCtx.StmtRunner().Query(SelectDowntime30SecGroupProbe)
	if err != nil {
		return nil, fmt.Errorf("select group and probe: %v", err)
	}
	defer rows.Close()

	var res = make([]check.ProbeRef, 0)
	for rows.Next() {
		var ref = check.ProbeRef{}
		err := rows.Scan(&ref.Group, &ref.Probe)
		if err != nil {
			return nil, fmt.Errorf("row to ProbeRef: %v", err)
		}
		res = append(res, ref)
	}

	return res, nil
}

func (d *Downtime30sDao) Stats() ([]string, error) {
	const SelectDowntime30SecStats = `
	SELECT timeslot, count(timeslot)
	FROM downtime30s
	GROUP BY timeslot
	`

	rows, err := d.DbCtx.StmtRunner().Query(SelectDowntime30SecStats)
	if err != nil {
		return nil, fmt.Errorf("select stats: %v", err)
	}
	defer rows.Close()

	var stats = []string{}
	for rows.Next() {
		var startTime int64
		var count int64
		rows.Scan(&startTime, &count)
		stats = append(stats, fmt.Sprintf("%d %d", startTime, count))
	}

	return stats, nil
}

func (d *Downtime30sDao) SaveBatch(downtimes []check.DowntimeEpisode) error {
	for _, downtime := range downtimes {
		err := d.Insert(downtime)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *Downtime30sDao) Insert(downtime check.DowntimeEpisode) error {
	const InsertDowntime30Sec = `
	INSERT INTO downtime30s (timeslot, success_seconds, fail_seconds, unknown_seconds, nodata_seconds, group_name, probe_name)
	VALUES
	(?, ?, ?, ?, ?, ?, ?)
	`

	_, err := d.DbCtx.StmtRunner().Exec(InsertDowntime30Sec,
		downtime.TimeSlot,
		downtime.SuccessSeconds,
		downtime.FailSeconds,
		downtime.UnknownSeconds,
		downtime.NoDataSeconds,
		downtime.ProbeRef.Group,
		downtime.ProbeRef.Probe)
	return err
}

func (d *Downtime30sDao) Update(rowid int64, downtime check.DowntimeEpisode) error {
	const UpdateDowntime30SecById = `
	UPDATE downtime30s
	SET
	    success_seconds=?,
	    fail_seconds=?,
	    unknown_seconds=?,
	    nodata_seconds=?
	WHERE rowid=?
	`

	_, err := d.DbCtx.StmtRunner().Exec(UpdateDowntime30SecById,
		downtime.SuccessSeconds,
		downtime.FailSeconds,
		downtime.UnknownSeconds,
		downtime.NoDataSeconds,
		rowid)
	if err != nil {
		return err
	}
	return nil
}

func (d *Downtime30sDao) DeleteEarlierThen(tm int64) error {
	const DeleteDowntime30SecByEarlierTimestamp = `
	DELETE FROM downtime30s
	WHERE timeslot < ?
	`

	_, err := d.DbCtx.StmtRunner().Exec(DeleteDowntime30SecByEarlierTimestamp, tm)
	if err != nil {
		return err
	}
	return nil
}
