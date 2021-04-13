package dao

import (
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"upmeter/pkg/check"
	dbcontext "upmeter/pkg/upmeter/db/context"
)

const (
	everyProbePlaceholder      = "__all__"
	aggregatedProbePlaceholder = "__total__"
)

// __all__ and __total__ probes should select all probes
func areAllProbesRequested(probeName string) bool {
	return probeName == everyProbePlaceholder || probeName == aggregatedProbePlaceholder
}

type Episode5m struct {
	Rowid   int64
	Episode check.Episode
}

type EpisodeDao5m struct {
	DbCtx *dbcontext.DbContext
	Table string
}

func NewEpisodeDao5m(dbCtx *dbcontext.DbContext) *EpisodeDao5m {
	return &EpisodeDao5m{
		DbCtx: dbCtx,
		Table: "episodes_5m",
	}
}

func (d *EpisodeDao5m) GetBySlotAndProbe(slot time.Time, ref check.ProbeRef) (Episode5m, error) {
	const SelectDowntime5mByTimeslotGroupProbe = `
	SELECT
		rowid, timeslot,
		nano_up, nano_down, nano_unknown, nano_unmeasured,
		group_name, probe_name
	FROM
		episodes_5m
	WHERE
		timeslot = ? AND group_name = ? AND probe_name = ?
	`

	rows, err := d.DbCtx.StmtRunner().Query(SelectDowntime5mByTimeslotGroupProbe, slot.Unix(), ref.Group, ref.Probe)
	if err != nil {
		return Episode5m{}, fmt.Errorf("select for TimeslotGroupProbe: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		// No entities found, return impossible rowid
		return Episode5m{Rowid: -1}, nil
	}

	var entity = Episode5m{}
	var slotUnix int64
	err = rows.Scan(
		&entity.Rowid,
		&slotUnix,
		&entity.Episode.Up,
		&entity.Episode.Down,
		&entity.Episode.Unknown,
		&entity.Episode.NoData,
		&entity.Episode.ProbeRef.Group,
		&entity.Episode.ProbeRef.Probe,
	)
	if err != nil {
		return Episode5m{}, fmt.Errorf("row to Episode5m: %v", err)
	}
	entity.Episode.TimeSlot = time.Unix(slotUnix, 0)

	// Assertion
	if rows.Next() {
		log.Errorf("Not consistent 5m data: more than one record for slot=%s, group='%s', probe='%s'", slot.Format(time.Stamp), ref.Group, ref.Probe)
	}

	return entity, nil
}

func (d *EpisodeDao5m) ListByRange(from, to int64) ([]Episode5m, error) {
	const SelectDowntime5mByTimeslotRange = `
	SELECT
		rowid, timeslot,
		nano_up, nano_down, nano_unknown, nano_unmeasured,
		group_name, probe_name
	FROM episodes_5m
	WHERE
		timeslot >= ? AND timeslot < ?
	`

	rows, err := d.DbCtx.StmtRunner().Query(SelectDowntime5mByTimeslotRange, from, to)
	if err != nil {
		return nil, fmt.Errorf("select for TimeslotRange: %v", err)
	}
	defer rows.Close()

	var res = make([]Episode5m, 0)
	for rows.Next() {
		var entity = Episode5m{}
		var slotUnix int64
		err := rows.Scan(
			&entity.Rowid,
			&slotUnix,
			&entity.Episode.Up,
			&entity.Episode.Down,
			&entity.Episode.Unknown,
			&entity.Episode.NoData,
			&entity.Episode.ProbeRef.Group,
			&entity.Episode.ProbeRef.Probe,
		)
		if err != nil {
			return nil, fmt.Errorf("row to Episode5m: %v", err)
		}
		entity.Episode.TimeSlot = time.Unix(slotUnix, 0)

		res = append(res, entity)
	}

	return res, nil
}

func (d *EpisodeDao5m) ListEpisodesByRange(from, to int64, ref check.ProbeRef) ([]check.Episode, error) {
	var query = `
	SELECT
		rowid, timeslot,
		nano_up, nano_down, nano_unknown, nano_unmeasured,
		group_name, probe_name
	FROM
		episodes_5m
	WHERE
		timeslot >= ? AND timeslot < ?
	`

	queryArgs := []interface{}{from, to}
	if ref.Group != "" {
		query += " AND group_name = ?"
		queryArgs = append(queryArgs, ref.Group)
	}

	// __all__ and __total__ probes should select all probes
	if !areAllProbesRequested(ref.Probe) {
		query += " AND probe_name = ?"
		queryArgs = append(queryArgs, ref.Probe)
	}

	rows, err := d.DbCtx.StmtRunner().Query(query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("select for TimeslotRange: %v", err)
	}
	defer rows.Close()

	var res = make([]check.Episode, 0)
	var slotUnix int64
	for rows.Next() {
		var entity = Episode5m{}
		err := rows.Scan(
			&entity.Rowid,
			&slotUnix,
			&entity.Episode.Up,
			&entity.Episode.Down,
			&entity.Episode.Unknown,
			&entity.Episode.NoData,
			&entity.Episode.ProbeRef.Group,
			&entity.Episode.ProbeRef.Probe,
		)
		if err != nil {
			return nil, fmt.Errorf("row to Episode5m: %v", err)
		}
		entity.Episode.TimeSlot = time.Unix(slotUnix, 0)
		res = append(res, entity.Episode)
	}

	return res, nil
}

// ListEpisodeSumsForRanges returns sums of seconds for each group_name+probe_name to reduce
// calculations over full table.
// FIXME rewrite this quick hack code.
func (d *EpisodeDao5m) ListEpisodeSumsForRanges(stepRanges check.StepRanges, ref check.ProbeRef) ([]check.Episode, error) {
	var res = make([]check.Episode, 0)

	var queryParts = map[string]string{
		"select": `SELECT sum(nano_up), sum(nano_down), sum(nano_unknown), sum(nano_unmeasured)`,
		"from":   "FROM episodes_5m",
		"where":  "WHERE timeslot >= ? AND timeslot < ?",
	}

	for _, stepRange := range stepRanges.Ranges {
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

		// __all__ and __total__ probes should select all probes
		if !areAllProbesRequested(ref.Probe) {
			where += " AND probe_name = ?"
			queryArgs = append(queryArgs, ref.Probe)
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
			var entity = Episode5m{}
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
				return nil, fmt.Errorf("row to Episode5m: %v", err)
			}
			entity.Episode.TimeSlot = time.Unix(stepRange.From, 0)
			res = append(res, entity.Episode)
		}
	}

	return res, nil
}

func (d *EpisodeDao5m) ListGroupProbe() ([]check.ProbeRef, error) {
	const SelectDowntime5mGroupProbe = `
	SELECT DISTINCT group_name, probe_name
	FROM episodes_5m
	ORDER BY 1, 2
	`

	rows, err := d.DbCtx.StmtRunner().Query(SelectDowntime5mGroupProbe)
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
		log.Infof("got probeRef=%s", ref.Id())
	}

	return res, nil
}

func (d *EpisodeDao5m) Insert(downtime check.Episode) error {
	const InsertDowntime5m = `
	INSERT INTO episodes_5m (timeslot, nano_up, nano_down, nano_unknown, nano_unmeasured, group_name, probe_name)
	VALUES
	(?, ?, ?, ?, ?, ?, ?)
	`

	_, err := d.DbCtx.StmtRunner().Exec(
		InsertDowntime5m,
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

func (d *EpisodeDao5m) Update(rowid int64, downtime check.Episode) error {
	const UpdateDowntime5m = `
	UPDATE episodes_5m
	SET
		nano_up         = ?,
		nano_down       = ?,
		nano_unknown    = ?,
		nano_unmeasured = ?
	WHERE rowid = ?
	`

	_, err := d.DbCtx.StmtRunner().Exec(
		UpdateDowntime5m,
		downtime.Up,
		downtime.Down,
		downtime.Unknown,
		downtime.NoData,
		rowid)
	return err
}
