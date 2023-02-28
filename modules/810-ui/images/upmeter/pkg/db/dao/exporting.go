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
	"fmt"
	"sort"
	"strings"
	"time"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/db"
	dbcontext "d8.io/upmeter/pkg/db/context"
)

const originsSep = ":"

// set is a set of strings. Not concurrency-safe.
type set map[string]struct{}

// {b, a, c} => "a:b:c"
func (s set) String() string {
	var list []string
	for el := range s {
		list = append(list, el)
	}
	sort.Strings(list)
	return strings.Join(list, originsSep)
}

func (s set) Size() int {
	return len(s)
}

func (s set) Add(el string) {
	s[el] = struct{}{}
}

func (s set) Merge(o set) {
	for el := range o {
		s.Add(el)
	}
}

func parseSet(s string) set {
	return newSet(strings.Split(s, originsSep)...)
}

func newSet(elems ...string) set {
	set := set{}
	for _, el := range elems {
		set.Add(el)
	}
	return set
}

type ExportEntity struct {
	// Episode, just an episode
	Episode check.Episode
	// SyncID is the id of a sync target for which this episodes is here
	SyncID string
	// Origins are unique sources IDs that have committed to the episode
	Origins set
}

func (e *ExportEntity) AddOrigin(o string) {
	if e.Origins == nil {
		e.Origins = set{}
	}
	e.Origins.Add(o)
}

var ErrNotFound = fmt.Errorf("not found")

type ExportDAO struct {
	ctx *dbcontext.DbContext
}

func NewExportEpisodesDAO(ctx *dbcontext.DbContext) *ExportDAO {
	return &ExportDAO{ctx}
}

func (dao *ExportDAO) Save(entities []ExportEntity) error {
	return db.WithTx(dao.ctx, func(tx *dbcontext.DbContext) error {
		return createOrUpdateExportEpisodes(tx, entities)
	})
}

func (dao *ExportDAO) GetEarliestEpisodes(syncID string, originsCount int) ([]ExportEntity, error) {
	trans, err := db.NewTx(dao.ctx)
	if err != nil {
		return nil, err
	}
	entities, err := getEarliestExportEpisodes(trans.Ctx(), syncID, originsCount)
	return entities, trans.Act(err)
}

func (dao *ExportDAO) DeleteUpTo(syncID string, slot time.Time) error {
	ctx := dao.ctx.Start()
	defer ctx.Stop()

	const q = `
	DELETE FROM export_episodes
	WHERE sync_id = @sync_id AND timeslot <= @timeslot
	`
	_, err := ctx.StmtRunner().Exec(q,
		sql.Named("sync_id", syncID),
		sql.Named("timeslot", slot.Unix()),
	)
	if err != nil {
		return err
	}
	return nil
}

// getExportEpisode fetches entity. Input entity is used as filter by sync_id, timeslot, group_name and probe_name.
func getExportEpisode(ctx *dbcontext.DbContext, filter ExportEntity) (*ExportEntity, error) {
	const query = `
	SELECT  sync_id, timeslot, group_name, probe_name,
		nano_up, nano_down, nano_unknown, nano_unmeasured,
		origins
	FROM  export_episodes
	WHERE   sync_id    = @sync_id    AND
		timeslot   = @timeslot   AND
		group_name = @group_name AND
		probe_name = @probe_name
	LIMIT 1;
	`
	rows, err := ctx.StmtRunner().Query(
		query,
		sql.Named("sync_id", filter.SyncID),
		sql.Named("timeslot", filter.Episode.TimeSlot.Unix()),
		sql.Named("group_name", filter.Episode.ProbeRef.Group),
		sql.Named("probe_name", filter.Episode.ProbeRef.Probe),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entities, err := parseExportEpisodeEntities(rows)
	if err != nil {
		return nil, err
	}
	if len(entities) == 0 {
		return nil, nil
	}

	return &entities[0], nil
}

func parseExportEpisodeEntities(rows *sql.Rows) ([]ExportEntity, error) {
	entities := make([]ExportEntity, 0)

	for rows.Next() {
		var (
			entity  ExportEntity
			origins string
		)
		var slotUnix int64
		err := rows.Scan(
			&entity.SyncID,
			&slotUnix,
			&entity.Episode.ProbeRef.Group,
			&entity.Episode.ProbeRef.Probe,
			&entity.Episode.Up,
			&entity.Episode.Down,
			&entity.Episode.Unknown,
			&entity.Episode.NoData,
			&origins,
		)
		if err != nil {
			return nil, err
		}
		entity.Episode.TimeSlot = time.Unix(slotUnix, 0)
		entity.Origins = parseSet(origins)

		entities = append(entities, entity)
	}

	return entities, nil
}

func createOrUpdateExportEpisodes(tx *dbcontext.DbContext, entities []ExportEntity) error {
	for _, entity := range entities {
		found, err := getExportEpisode(tx, entity)
		if err != nil {
			return fmt.Errorf("cannot fetch: %v", err)
		}
		if found != nil {
			entity.Origins.Merge(found.Origins)
		}
		err = saveExportEpisode(tx, entity)
		if err != nil {
			return fmt.Errorf("cannot save: %v", err)
		}
	}
	return nil
}

func saveExportEpisode(tx *dbcontext.DbContext, entity ExportEntity) error {
	const query = `
	INSERT INTO export_episodes
		(sync_id, timeslot, group_name, probe_name,
		 nano_up, nano_down, nano_unknown, nano_unmeasured,
		 origins, origins_count)
	VALUES
		(@sync_id, @timeslot, @group_name, @probe_name,
		 @nano_up, @nano_down, @nano_unknown, @nano_unmeasured,
		 @origins, @origins_count)
	ON CONFLICT
		(sync_id, timeslot, group_name, probe_name)
	DO UPDATE SET
		nano_up         = @nano_up,
		nano_down       = @nano_down,
		nano_unknown    = @nano_unknown,
		nano_unmeasured = @nano_unmeasured,
		origins         = @origins,
		origins_count   = @origins_count;
	`

	_, err := tx.StmtRunner().Exec(
		query,
		sql.Named("sync_id", entity.SyncID),
		sql.Named("timeslot", entity.Episode.TimeSlot.Unix()),

		sql.Named("group_name", entity.Episode.ProbeRef.Group),
		sql.Named("probe_name", entity.Episode.ProbeRef.Probe),

		sql.Named("nano_up", entity.Episode.Up),
		sql.Named("nano_down", entity.Episode.Down),
		sql.Named("nano_unknown", entity.Episode.Unknown),
		sql.Named("nano_unmeasured", entity.Episode.NoData),

		sql.Named("origins", entity.Origins.String()),
		sql.Named("origins_count", entity.Origins.Size()),
	)

	return err
}

func getEarliestExportEpisodes(ctx *dbcontext.DbContext, syncID string, originsCount int) ([]ExportEntity, error) {
	slot, err := getEarliestTimeSlot(ctx, syncID, originsCount)
	if err != nil {
		return nil, err
	}
	return getExportEpisodesBySyncIDAndSlot(ctx, syncID, slot)
}

// getEarliestTimeSlot finds the earliest timeslot for the sync ID. If
// - the table contains fulfilled origin counts for the syncID
// - there are expired (more than 24h ago) episodes.
//
//	Otherwise, this function returns ErrNotFound.
//
// Consider unfulfilled episodes to be fulfilled even if they are found with earlier timeslots that the desired
// origins_count have reached. By the limitation of timeseries storages, we can only export the earliest episodes.
// Since upmeter-agents add these episodes in chronological order, the desired origins_count in the middle
// of thee slot range is the mark that earlier episodes will never be fulfilled. Thus it is better to send them
// than to skip them.
//
// Case #1, we have unfulfilled episodes earlier than a fulfilled one
//
//	 N = origins count
//	 D = desired origins count
//
//	      Fresh episode fulfilled by the Dth agent, making N=D
//	                     ↓
//	  N<D   N<D   N<D   N=D   N<D   N<D   ...
//	|-----|-----|-----|-----|-----|-----|-----|-----|-----|-----|--> time (slots)
//	   ↑
//	  Send this anyway, because
//	          - there is no hope it will ever reach D
//	          - timeseries storage accept only newer samples related to existing ones
//
// Case #2, all episodes are unfulfilled, and we have 24h-old among them
//
//	   24h ago
//	      ↓
//	  N<D ↓ N<D   N<D   N<D   N<D   N<D   ...
//	|-----|-----|-----|-----|-----|-----|-----|-----|-----|-----|--> time (slots)
//	   ↑
//	  Send this anyway, because
//	          - it will never reach D, because upmeter server skips episodes that are older than 24h
//	          - again, timeseries storage accept only newer samples related to existing ones
func getEarliestTimeSlot(ctx *dbcontext.DbContext, syncID string, originsCount int) (int64, error) {
	commonSlot, err := getEarliestCommonTimeSlot(ctx, syncID)
	if err != nil {
		return 0, err
	}

	now := time.Now().Unix()
	if commonSlot < now {
		// in case of expired episodes, it does not matter how many origins are fulfilled
		return commonSlot, nil
	}

	fulfilledSlot := commonSlot
	if originsCount > 1 {
		// not found means error as well, the only corner case is the 24h expiration convention
		fulfilledSlot, err = getEarliestTimeSlotByOriginsCount(ctx, syncID, originsCount)
		if err != nil {
			return 0, err
		}
	}

	// Slots are numbers there because this is not public interface
	minSlot := minInt64(fulfilledSlot, commonSlot)
	return minSlot, nil
}

// getEarliestTimeSlotByOriginsCount finds the earliest timeslot for syncID and originsCount.
// set originsCount to -1 to search only by syncID
func getEarliestTimeSlotByOriginsCount(ctx *dbcontext.DbContext, syncID string, originsCount int) (int64, error) {
	query := `
	SELECT   MIN(timeslot)
	FROM     export_episodes
	WHERE    sync_id = @sync_id AND origins_count >= @origins_count
	GROUP BY sync_id;
	`

	rows, err := ctx.StmtRunner().Query(
		query,
		sql.Named("sync_id", syncID),
		sql.Named("origins_count", originsCount),
	)
	if err != nil {
		return 0, fmt.Errorf("cannot execute query: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return 0, ErrNotFound
	}

	// Slot is kept as number here, because this is not a high-level interface function, and we are likely
	// going to reuse the int64 value
	var slot int64
	err = rows.Scan(&slot)
	if err != nil {
		return 0, fmt.Errorf("cannot parse: %v", err)
	}

	return slot, nil
}

// getEarliestCommonTimeSlot finds the earliest timeslot for syncID
func getEarliestCommonTimeSlot(ctx *dbcontext.DbContext, syncID string) (int64, error) {
	query := `
	SELECT   MIN(timeslot)
	FROM     export_episodes
	WHERE    sync_id = @sync_id
	GROUP BY sync_id;
	`

	rows, err := ctx.StmtRunner().Query(
		query,
		sql.Named("sync_id", syncID),
	)
	if err != nil {
		return 0, fmt.Errorf("cannot execute query: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return 0, ErrNotFound
	}

	var slot int64
	err = rows.Scan(&slot)
	if err != nil {
		return 0, fmt.Errorf("cannot parse: %v", err)
	}

	return slot, nil
}

// getExportEpisodesBySyncIDAndSlot finds the episode list by slot and sync ID. The slot is number here for convenience,
// because it is not public interface.
func getExportEpisodesBySyncIDAndSlot(ctx *dbcontext.DbContext, syncID string, slot int64) ([]ExportEntity, error) {
	const query = `
	SELECT  sync_id, timeslot, group_name, probe_name,
		nano_up, nano_down, nano_unknown, nano_unmeasured,
		origins
	FROM  export_episodes
	WHERE   sync_id  = @sync_id  AND
		timeslot = @timeslot;
	`
	rows, err := ctx.StmtRunner().Query(
		query,
		sql.Named("sync_id", syncID),
		sql.Named("timeslot", slot),
	)
	if err != nil {
		return nil, fmt.Errorf("cannot execute query: %v", err)
	}
	defer rows.Close()

	return parseExportEpisodeEntities(rows)
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
