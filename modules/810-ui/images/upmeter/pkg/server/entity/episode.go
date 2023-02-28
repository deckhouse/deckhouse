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

// Save30sEpisodes stores 30 sec downtime episodes into database.
// It also clears old records and update 5 minute episodes in a database.
func Save30sEpisodes(ctx *dbcontext.DbContext, episodes []check.Episode) []*check.Episode {
	saved := make([]*check.Episode, 0)
	dayAgo := time.Now().Add(-24 * time.Hour)

	for _, episode := range episodes {
		// Ignore episodes older then 24h.
		if episode.TimeSlot.Before(dayAgo) {
			log.Warnf("Ignoring outdated episode: %s", episode.String())
			continue
		}

		if !episode.IsCorrect(30 * time.Second) {
			log.Errorf("Possible bug!!! Ignoring incorrect episode: %s", episode.String())
			continue
		}

		ep, err := save30sEpisode(ctx, episode)
		if err != nil {
			log.Errorf("cannot save episode slot=%d, red=%q: %v", episode.TimeSlot.UnixNano(), episode.ProbeRef.Id(), err)
			continue
		}
		saved = append(saved, ep)
	}

	return saved
}

// Update5mEpisodes stores 5 min downtime episodes into database.
func Update5mEpisodes(ctx *dbcontext.DbContext, episodes30s []*check.Episode) []*check.Episode {
	// slot -> unique probe ID
	bySlot := make(map[int64]map[string]check.ProbeRef)
	saved := make([]*check.Episode, 0)

	for _, episode := range episodes30s {
		slot5m := episode.TimeSlot.Truncate(5 * time.Minute).Unix()

		if _, ok := bySlot[slot5m]; !ok {
			bySlot[slot5m] = make(map[string]check.ProbeRef)
		}

		bySlot[slot5m][episode.ProbeRef.Id()] = episode.ProbeRef
	}

	for slot5m, probeRefs := range bySlot {
		for _, ref := range probeRefs {
			episode, err := update5mEpisode(ctx, time.Unix(slot5m, 0), ref)
			if err != nil {
				if err != ErrNotChanged {
					log.Errorf("Did not save 5m episode slot=%d ref=%s: %v", slot5m, ref.Id(), err)
				}
				continue
			}
			saved = append(saved, episode)
		}
	}

	return saved
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
