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

package entity

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/db"
	dbcontext "d8.io/upmeter/pkg/db/context"
	"d8.io/upmeter/pkg/db/dao"
)

var ErrNotChanged = fmt.Errorf("not changed")

const (
	slotSize30s = 30 * time.Second
	slotSize5m  = 5 * time.Minute
)

// Save30sEpisodes stores 30 sec downtime episodes using the provided transaction context.
//
// To avoid a query per episode, it loads the current state of all affected slots in one range query,
// merges the incoming episodes with the stored ones (keeping the existing Combine semantics that
// deduplicate data coming from multiple agents), and writes everything back with batched UPSERT
// statements. Outdated and malformed episodes are skipped; a database error aborts the whole batch
// so the caller can roll back the single transaction and let the agent retry (saving is idempotent).
func Save30sEpisodes(tx *dbcontext.DbContext, episodes []check.Episode) ([]*check.Episode, error) {
	dayAgo := time.Now().Add(-24 * time.Hour)

	valid := make([]check.Episode, 0, len(episodes))
	for _, episode := range episodes {
		// Ignore episodes older then 24h.
		if episode.TimeSlot.Before(dayAgo) {
			log.Warnf("Ignoring outdated episode: %s", episode.String())
			continue
		}
		if !episode.IsCorrect(slotSize30s) {
			log.Errorf("Possible bug!!! Ignoring incorrect episode: %s", episode.String())
			continue
		}
		valid = append(valid, episode)
	}
	if len(valid) == 0 {
		return []*check.Episode{}, nil
	}

	dao30s := dao.NewEpisodeDao30s(tx)

	// Load the current state of all affected slots in a single query.
	minSlot, maxSlot := slotBounds(valid)
	existingEntities, err := dao30s.ListEntitiesBySlotRange(minSlot, maxSlot)
	if err != nil {
		return nil, fmt.Errorf("loading existing 30s episodes: %w", err)
	}
	stored := make(map[string]check.Episode, len(existingEntities))
	for _, e := range existingEntities {
		stored[episodeKey(e.Episode.TimeSlot, e.Episode.ProbeRef)] = e.Episode
	}

	// Merge incoming episodes with the stored ones.
	merged := make([]check.Episode, 0, len(valid))
	saved := make([]*check.Episode, 0, len(valid))
	for _, episode := range valid {
		final := episode
		if prev, ok := stored[episodeKey(episode.TimeSlot, episode.ProbeRef)]; ok {
			final = episode.Combine(prev, slotSize30s)
		}
		merged = append(merged, final)

		ep := final
		saved = append(saved, &ep)
	}

	if err := dao30s.UpsertEpisodes(merged); err != nil {
		return nil, fmt.Errorf("saving 30s episodes: %w", err)
	}

	return saved, nil
}

// Update5mEpisodes recalculates 5 min downtime episodes using the provided transaction context.
//
// It refreshes only the (5m slot, probe) pairs touched by the given 30s episodes. Instead of a sum
// query and a read per pair, it sums all 30s episodes grouped by 5m slot and probe in one query,
// loads the stored 5m rows in one range query, merges in Go (keeping the existing Combine and
// "skip unchanged" semantics), and writes the changed rows with batched UPSERT statements.
func Update5mEpisodes(tx *dbcontext.DbContext, episodes30s []*check.Episode) ([]*check.Episode, error) {
	if len(episodes30s) == 0 {
		return []*check.Episode{}, nil
	}

	// The set of (5m slot, probe) pairs to refresh, derived from the saved 30s episodes.
	wanted := make(map[string]struct{}, len(episodes30s))
	var min5m, max5m int64
	first := true
	for _, ep := range episodes30s {
		slot5m := ep.TimeSlot.Truncate(slotSize5m).Unix()
		wanted[slot5mKey(slot5m, ep.ProbeRef)] = struct{}{}
		if first || slot5m < min5m {
			min5m = slot5m
		}
		if first || slot5m > max5m {
			max5m = slot5m
		}
		first = false
	}

	dao30s := dao.NewEpisodeDao30s(tx)
	dao5m := dao.NewEpisodeDao5m(tx)

	// Sum all 30s episodes grouped by their parent 5m slot and probe in one query. The range covers
	// every affected 5m window fully so the sums account for sub-slots stored by previous batches.
	fifthMinSeconds := int64(slotSize5m / time.Second)
	sums, err := dao30s.Sum30sGroupedBy5m(min5m, max5m+fifthMinSeconds)
	if err != nil {
		return nil, fmt.Errorf("summing 30s episodes by 5m slot: %w", err)
	}

	// Load the stored 5m rows for the affected range in one query.
	existingEntities, err := dao5m.ListEntitiesBySlotRange(min5m, max5m)
	if err != nil {
		return nil, fmt.Errorf("loading existing 5m episodes: %w", err)
	}
	stored := make(map[string]check.Episode, len(existingEntities))
	for _, e := range existingEntities {
		stored[slot5mKey(e.Episode.TimeSlot.Unix(), e.Episode.ProbeRef)] = e.Episode
	}

	toWrite := make([]check.Episode, 0, len(sums))
	saved := make([]*check.Episode, 0, len(sums))
	for _, summed := range sums {
		key := slot5mKey(summed.TimeSlot.Unix(), summed.ProbeRef)
		if _, ok := wanted[key]; !ok {
			continue
		}

		prev, found := stored[key]
		combined := prev.Combine(summed, slotSize5m)
		if found && combined.EqualTimers(prev) {
			// Nothing changed, do not rewrite or re-export.
			continue
		}
		if combined.Up < prev.Up {
			log.Warnf("Possible bug!!! Combined Up=%d for 5m episode is worse than saved %d for slot=%s ref=%q",
				combined.Up, prev.Up, summed.TimeSlot.Format(time.Stamp), summed.ProbeRef.Id())
		}
		combined.ProbeRef = summed.ProbeRef
		combined.TimeSlot = summed.TimeSlot

		toWrite = append(toWrite, combined)

		ep := combined
		saved = append(saved, &ep)
	}

	if err := dao5m.UpsertEpisodes(toWrite); err != nil {
		return nil, fmt.Errorf("saving 5m episodes: %w", err)
	}

	return saved, nil
}

func episodeKey(slot time.Time, ref check.ProbeRef) string {
	return fmt.Sprintf("%d|%s|%s", slot.Unix(), ref.Group, ref.Probe)
}

func slot5mKey(slot5mUnix int64, ref check.ProbeRef) string {
	return fmt.Sprintf("%d|%s|%s", slot5mUnix, ref.Group, ref.Probe)
}

func slotBounds(episodes []check.Episode) (minUnix, maxUnix int64) {
	minUnix = episodes[0].TimeSlot.Unix()
	maxUnix = episodes[0].TimeSlot.Unix()
	for _, ep := range episodes[1:] {
		u := ep.TimeSlot.Unix()
		if u < minUnix {
			minUnix = u
		}
		if u > maxUnix {
			maxUnix = u
		}
	}
	return minUnix, maxUnix
}

// Insert or Update a Episode using a transaction.
func save30sEpisode(dbCtx *dbcontext.DbContext, episode check.Episode) (*check.Episode, error) {
	trans, err := db.NewTx(dbCtx)
	if err != nil {
		return nil, err
	}
	saved, err := txSave30sEpisode(trans.Ctx(), episode)
	return saved, trans.Act(err)
}

func update5mEpisode(dbCtx *dbcontext.DbContext, slot time.Time, ref check.ProbeRef) (*check.Episode, error) {
	trans, err := db.NewTx(dbCtx)
	if err != nil {
		return nil, err
	}
	ep, err := txUpdate5mEpisode(trans.Ctx(), slot, ref)
	return ep, trans.Act(err)
}

func txSave30sEpisode(tx *dbcontext.DbContext, episode check.Episode) (*check.Episode, error) {
	dao30s := dao.NewEpisodeDao30s(tx)
	storedEntity, err := dao30s.GetBySlotAndProbe(episode.TimeSlot, episode.ProbeRef)
	if err != nil {
		err := fmt.Errorf("cannot fetch similar episodes: %v", err)
		return nil, err
	}

	if storedEntity.Rowid == -1 {
		// none saved yet, insert new one
		log.Debugf("Inserting 30s episode %s", episode.String())

		err = dao30s.Insert(episode)
		if err != nil {
			err = fmt.Errorf("cannot insert: %v", err)
		}
		return &episode, err
	}

	slotSize := 30 * time.Second
	combined := episode.Combine(storedEntity.Episode, slotSize)

	// note extra whitespace to align with "Inserting..."
	log.Debugf("Updating  30s episode %s", episode.String())

	err = dao30s.Update(storedEntity.Rowid, combined)
	if err != nil {
		err = fmt.Errorf("cannot update: %v", err)
	}
	return &combined, err
}

func txUpdate5mEpisode(tx *dbcontext.DbContext, slot time.Time, ref check.ProbeRef) (*check.Episode, error) {
	// Sum up 30 sec episodes.
	summed, err := txSum30sEpisodes(tx, slot, ref)
	if err != nil {
		return nil, err
	}

	dao5m := dao.NewEpisodeDao5m(tx)

	// Get stored 5 min episode.
	storedEntity, err := dao5m.GetBySlotAndProbe(slot, ref)
	if err != nil {
		err = fmt.Errorf("cannot fetch 5min episode by slot=%s ref=%q: %v", slot.Format(time.Stamp), ref.Id(), err)
		return nil, err
	}

	isFound := storedEntity.Rowid != -1
	stored := storedEntity.Episode
	combined := stored.Combine(*summed, 5*time.Minute)
	didNotChange := stored.EqualTimers(combined)

	// Do not update 5m episode if nothing has changed in 30s episodes.
	if isFound && didNotChange {
		return nil, ErrNotChanged
	}

	// Assert that new success duration is not "worse"
	if combined.Up < stored.Up {
		log.Warnf("Possible bug!!! Combined Up=%d for 5m episode is worse than saved %d for slot=%s ref=%q",
			combined.Up, stored.Up,
			stored.TimeSlot.Format(time.Stamp), stored.ProbeRef.Id())
	}

	if !isFound {
		// Insert
		combined.ProbeRef.Group = ref.Group
		combined.ProbeRef.Probe = ref.Probe
		combined.TimeSlot = slot

		err = dao5m.Insert(combined)
		if err != nil {
			err = fmt.Errorf("cannot insert 5m episode: %v", err)
			return nil, err
		}
		return &combined, nil
	}

	// Update
	err = dao5m.Update(storedEntity.Rowid, combined)
	if err != nil {
		err = fmt.Errorf("cannot update 5m episode: %v", err)
		return nil, err
	}
	return &combined, nil
}

func txSum30sEpisodes(tx *dbcontext.DbContext, slot time.Time, ref check.ProbeRef) (*check.Episode, error) {
	// Get all records of 30s slots within current 5 min slot.
	dao30s := dao.NewEpisodeDao30s(tx)
	entities30s, err := dao30s.ListByRange(slot, slot.Add(5*time.Minute), ref)
	if err != nil {
		err = fmt.Errorf("cannot fetch 30s episodes by slot=%s ref=%q: %v", slot.Format(time.Stamp), ref.Id(), err)
		return nil, err
	}

	// Sum up 30 sec episodes.
	sumEpisode := check.Episode{}
	for _, item := range entities30s {
		sumEpisode.Up += item.Episode.Up
		sumEpisode.Down += item.Episode.Down
		sumEpisode.Unknown += item.Episode.Unknown
		sumEpisode.NoData += item.Episode.NoData
	}

	return &sumEpisode, nil
}
