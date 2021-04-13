package dao

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"upmeter/pkg/check"
	dbcontext "upmeter/pkg/upmeter/db/context"
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

type EpisodeEntity30s struct {
	Rowid   int64
	Episode check.Episode
}

func (d *EpisodeDao30s) ListBySlot(slot time.Time) ([]EpisodeEntity30s, error) {
	const SelectDowntime30SecByTimeslot = `
	SELECT
		rowid, timeslot,
		nano_up, nano_down, nano_unknown, nano_unmeasured,
		group_name, probe_name
	FROM episodes_30s
	WHERE timeslot = ?
	`

	rows, err := d.DbCtx.StmtRunner().Query(SelectDowntime30SecByTimeslot, slot.Unix())
	if err != nil {
		return nil, fmt.Errorf("cannot query SELECT: %v", err)
	}
	defer rows.Close()

	var res = make([]EpisodeEntity30s, 0)
	var slotTimestamp int64
	for rows.Next() {
		var entity = EpisodeEntity30s{}
		err := rows.Scan(
			&entity.Rowid,
			&slotTimestamp,
			&entity.Episode.Up,
			&entity.Episode.Down,
			&entity.Episode.Unknown,
			&entity.Episode.NoData,
			&entity.Episode.ProbeRef.Group,
			&entity.Episode.ProbeRef.Probe,
		)
		if err != nil {
			return nil, fmt.Errorf("cannot parse: %v", err)
		}
		entity.Episode.TimeSlot = time.Unix(slotTimestamp, 0)
		res = append(res, entity)
	}

	return res, nil
}

func (d *EpisodeDao30s) GetSimilar(episode check.Episode) (EpisodeEntity30s, error) {
	const SelectDowntime30SecByTimeslotGroupProbe = `
	SELECT
		rowid, timeslot,
		nano_up, nano_down, nano_unknown, nano_unmeasured,
		group_name, probe_name
	FROM episodes_30s
	WHERE   timeslot = ?    AND
		group_name = ?  AND
		probe_name = ?
	`

	rows, err := d.DbCtx.StmtRunner().Query(SelectDowntime30SecByTimeslotGroupProbe, episode.TimeSlot.Unix(), episode.ProbeRef.Group, episode.ProbeRef.Probe)
	if err != nil {
		return EpisodeEntity30s{}, fmt.Errorf("select for timestamp: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		// No entities found, return impossible rowid
		return EpisodeEntity30s{Rowid: -1}, nil
	}

	var entity = EpisodeEntity30s{}
	var startUnix int64
	err = rows.Scan(
		&entity.Rowid,
		&startUnix,
		&entity.Episode.Up,
		&entity.Episode.Down,
		&entity.Episode.Unknown,
		&entity.Episode.NoData,
		&entity.Episode.ProbeRef.Group,
		&entity.Episode.ProbeRef.Probe,
	)
	if err != nil {
		return EpisodeEntity30s{}, fmt.Errorf("row to EpisodeEntity30s: %v", err)
	}
	entity.Episode.TimeSlot = time.Unix(startUnix, 0)

	// Assertion
	if rows.Next() {
		log.Warnf("inconsistent 30s data: more than one record selected for ts=%d %s", episode.TimeSlot.Unix(), episode.ProbeRef.Id())
	}

	return entity, nil
}

func (d *EpisodeDao30s) ListForRange(start, end time.Time, ref check.ProbeRef) ([]EpisodeEntity30s, error) {
	const SelectDowntime30SecByTimeslotRange = `
	SELECT
		rowid, timeslot,
		nano_up, nano_down, nano_unknown, nano_unmeasured,
		group_name, probe_name
	FROM
		episodes_30s
	WHERE
	      timeslot >= ? AND timeslot < ?
	      AND group_name = ? AND probe_name = ?
	`

	rows, err := d.DbCtx.StmtRunner().Query(SelectDowntime30SecByTimeslotRange, start.Unix(), end.Unix(), ref.Group, ref.Probe)
	if err != nil {
		return nil, fmt.Errorf("cannot query SELECT: %v", err)
	}
	defer rows.Close()

	var res = make([]EpisodeEntity30s, 0)
	var startUnix int64
	for rows.Next() {
		var entity = EpisodeEntity30s{}
		err := rows.Scan(
			&entity.Rowid,
			&startUnix,
			&entity.Episode.Up,
			&entity.Episode.Down,
			&entity.Episode.Unknown,
			&entity.Episode.NoData,
			&entity.Episode.ProbeRef.Group,
			&entity.Episode.ProbeRef.Probe,
		)
		if err != nil {
			return nil, fmt.Errorf("cannot parse: %v", err)
		}
		entity.Episode.TimeSlot = time.Unix(startUnix, 0)
		res = append(res, entity)
	}

	return res, nil
}

func (d *EpisodeDao30s) ListGroupProbe() ([]check.ProbeRef, error) {
	const SelectDowntime30SecGroupProbe = `
	SELECT DISTINCT group_name, probe_name
	FROM episodes_30s
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

func (d *EpisodeDao30s) Stats() ([]string, error) {
	const SelectDowntime30SecStats = `
	SELECT timeslot, count(timeslot)
	FROM episodes_30s
	GROUP BY timeslot
	`

	rows, err := d.DbCtx.StmtRunner().Query(SelectDowntime30SecStats)
	if err != nil {
		return nil, fmt.Errorf("select stats: %v", err)
	}
	defer rows.Close()

	var stats = []string{}
	for rows.Next() {
		var startUnix, count int64
		rows.Scan(&startUnix, &count)
		stats = append(stats, fmt.Sprintf("%d %d", startUnix, count))
	}

	return stats, nil
}

func (d *EpisodeDao30s) SaveBatch(downtimes []check.Episode) error {
	for _, downtime := range downtimes {
		err := d.Insert(downtime)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *EpisodeDao30s) Insert(downtime check.Episode) error {
	const query = `
	INSERT INTO episodes_30s (timeslot, nano_up, nano_down, nano_unknown, nano_unmeasured, group_name, probe_name)
	VALUES
	(?, ?, ?, ?, ?, ?, ?)
	`

	_, err := d.DbCtx.StmtRunner().Exec(
		query,
		downtime.TimeSlot.Unix(),
		downtime.Up,
		downtime.Down,
		downtime.Unknown,
		downtime.NoData,
		downtime.ProbeRef.Group,
		downtime.ProbeRef.Probe,
	)
	return err
}

func (d *EpisodeDao30s) Update(rowid int64, downtime check.Episode) error {
	const UpdateDowntime30SecById = `
	UPDATE episodes_30s
	SET
		nano_up         = ?,
		nano_down       = ?,
		nano_unknown    = ?,
		nano_unmeasured = ?
	WHERE rowid = ?
	`

	_, err := d.DbCtx.StmtRunner().Exec(
		UpdateDowntime30SecById,
		downtime.Up,
		downtime.Down,
		downtime.Unknown,
		downtime.NoData,
		rowid,
	)
	if err != nil {
		return err
	}
	return nil
}

func (d *EpisodeDao30s) DeleteEarlierThen(slot time.Time) error {
	const q = `
	DELETE FROM episodes_30s
	WHERE timeslot < ?
	`

	_, err := d.DbCtx.StmtRunner().Exec(q, slot.Unix())
	if err != nil {
		return err
	}
	return nil
}
