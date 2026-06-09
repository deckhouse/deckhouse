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
	list := make([]string, 0, len(s))
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

func (dao *ExportDAO) DeleteBefore(syncID string, slot time.Time) error {
	ctx := dao.ctx.Start()
	defer ctx.Stop()

	const q = `
	DELETE FROM export_episodes
	WHERE sync_id = @sync_id AND timeslot < @timeslot
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

// listExportEpisodesBySlotRange fetches all stored export episodes for the given sync_id whose time
// slot is within [fromUnix, toUnix] (inclusive). It loads the current state of a whole batch in a
// single query so origins can be merged in Go without a query per episode.
func listExportEpisodesBySlotRange(ctx *dbcontext.DbContext, syncID string, fromUnix, toUnix int64) ([]ExportEntity, error) {
	const query = `
	SELECT  sync_id, timeslot, group_name, probe_name,
		nano_up, nano_down, nano_unknown, nano_unmeasured,
		origins
	FROM  export_episodes
	WHERE   sync_id  = @sync_id AND
		timeslot >= @from   AND
		timeslot <= @to;
	`
	rows, err := ctx.StmtRunner().Query(
		query,
		sql.Named("sync_id", syncID),
		sql.Named("from", fromUnix),
		sql.Named("to", toUnix),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return parseExportEpisodeEntities(rows)
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

// createOrUpdateExportEpisodes persists a batch of export entities that all share the same sync_id.
//
// To avoid a query per episode, it loads the current state of all affected slots in one range query,
// merges the accumulated origins in Go, and writes everything back with batched UPSERT statements.
// A conflicting row is overwritten with the merged values; the unique index
// (sync_id, timeslot, group_name, probe_name) drives the conflict resolution.
func createOrUpdateExportEpisodes(tx *dbcontext.DbContext, entities []ExportEntity) error {
	if len(entities) == 0 {
		return nil
	}

	syncID := entities[0].SyncID
	minSlot, maxSlot := exportSlotBounds(entities)

	existing, err := listExportEpisodesBySlotRange(tx, syncID, minSlot, maxSlot)
	if err != nil {
		return fmt.Errorf("cannot fetch existing export episodes: %w", err)
	}
	stored := make(map[string]ExportEntity, len(existing))
	for _, e := range existing {
		stored[exportKey(e)] = e
	}

	for i := range entities {
		if prev, ok := stored[exportKey(entities[i])]; ok {
			entities[i].Origins.Merge(prev.Origins)
		}
	}

	if err := upsertExportEpisodes(tx, entities); err != nil {
		return fmt.Errorf("cannot save export episodes: %w", err)
	}
	return nil
}

// upsertExportEpisodes inserts or overwrites export entities in batched multi-row statements.
func upsertExportEpisodes(ctx *dbcontext.DbContext, entities []ExportEntity) error {
	for start := 0; start < len(entities); start += upsertChunkSize {
		end := start + upsertChunkSize
		if end > len(entities) {
			end = len(entities)
		}
		if err := upsertExportChunk(ctx, entities[start:end]); err != nil {
			return err
		}
	}
	return nil
}

func upsertExportChunk(ctx *dbcontext.DbContext, entities []ExportEntity) error {
	query := `
	INSERT INTO export_episodes
		(sync_id, timeslot, group_name, probe_name,
		 nano_up, nano_down, nano_unknown, nano_unmeasured,
		 origins, origins_count)
	VALUES ` + exportValuesPlaceholders(len(entities)) + `
	ON CONFLICT(sync_id, timeslot, group_name, probe_name) DO UPDATE SET
		nano_up         = excluded.nano_up,
		nano_down       = excluded.nano_down,
		nano_unknown    = excluded.nano_unknown,
		nano_unmeasured = excluded.nano_unmeasured,
		origins         = excluded.origins,
		origins_count   = excluded.origins_count`

	args := make([]interface{}, 0, len(entities)*10)
	for _, e := range entities {
		args = append(args,
			e.SyncID,
			e.Episode.TimeSlot.Unix(),
			e.Episode.ProbeRef.Group,
			e.Episode.ProbeRef.Probe,
			e.Episode.Up,
			e.Episode.Down,
			e.Episode.Unknown,
			e.Episode.NoData,
			e.Origins.String(),
			e.Origins.Size(),
		)
	}

	if _, err := ctx.StmtRunner().Exec(query, args...); err != nil {
		return fmt.Errorf("upsert into export_episodes: %w", err)
	}
	return nil
}

// exportValuesPlaceholders returns "(?, ... ten ...), ..." with one group per row.
func exportValuesPlaceholders(rows int) string {
	const oneRow = "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	groups := make([]string, rows)
	for i := range groups {
		groups[i] = oneRow
	}
	return strings.Join(groups, ", ")
}

func exportKey(e ExportEntity) string {
	return fmt.Sprintf("%d|%s|%s", e.Episode.TimeSlot.Unix(), e.Episode.ProbeRef.Group, e.Episode.ProbeRef.Probe)
}

func exportSlotBounds(entities []ExportEntity) (minUnix, maxUnix int64) {
	minUnix = entities[0].Episode.TimeSlot.Unix()
	maxUnix = minUnix
	for _, e := range entities[1:] {
		u := e.Episode.TimeSlot.Unix()
		if u < minUnix {
			minUnix = u
		}
		if u > maxUnix {
			maxUnix = u
		}
	}
	return minUnix, maxUnix
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

	// Slot is kept as number here. MIN(timeslot) can be NULL when no rows match.
	var slot sql.NullInt64
	if err = rows.Scan(&slot); err != nil {
		return 0, fmt.Errorf("cannot parse: %v", err)
	}
	if !slot.Valid {
		return 0, ErrNotFound
	}
	return slot.Int64, nil
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

	// MIN(timeslot) can be NULL when no rows match.
	var slot sql.NullInt64
	if err = rows.Scan(&slot); err != nil {
		return 0, fmt.Errorf("cannot parse: %v", err)
	}
	if !slot.Valid {
		return 0, ErrNotFound
	}
	return slot.Int64, nil
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
