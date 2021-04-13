package entity

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"upmeter/pkg/check"
	dbcontext "upmeter/pkg/upmeter/db/context"
	"upmeter/pkg/upmeter/db/dao"
)

var (
	ErrNotChanged = fmt.Errorf("not changed")
)

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

		ep, err := Save30sEpisode(ctx, episode)
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
			episode, err := Update5mEpisode(ctx, time.Unix(slot5m, 0), ref)
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
func Save30sEpisode(dbCtx *dbcontext.DbContext, episode check.Episode) (*check.Episode, error) {
	txCtx, err := dbCtx.BeginTransaction()
	if err != nil {
		return nil, err
	}
	dao30s := dao.NewEpisodeDao30s(txCtx)

	storedEpisode, err := dao30s.GetSimilar(episode)
	if err != nil {
		err := fmt.Errorf("cannot fetch similar episodes: %v", err)

		rollErr := txCtx.Rollback()
		if rollErr != nil {
			return nil, fmt.Errorf("%v, rollback: %v", err, rollErr)
		}
		return nil, err
	}

	if storedEpisode.Rowid == -1 {
		err = dao30s.Insert(episode)
		if err != nil {
			err := fmt.Errorf("cannot insert: %v", err)

			rollErr := txCtx.Rollback()
			if rollErr != nil {
				return nil, fmt.Errorf("%v, rollback: %v", err, rollErr)
			}
			return nil, err
		}
		log.Infof("Inserting 30s episode %s", episode.String())

		return &episode, txCtx.Commit()
	}

	slotSize := 30 * time.Second
	newEpisode := episode.CombineSeconds(storedEpisode.Episode, slotSize)
	err = dao30s.Update(storedEpisode.Rowid, newEpisode)
	if err != nil {
		err := fmt.Errorf("cannot update: %v", err)

		rollErr := txCtx.Rollback()
		if rollErr != nil {
			return nil, fmt.Errorf("%v, rollback: %v", err, rollErr)
		}
		return nil, err
	}
	log.Infof("Updating  30s episode %s", episode.String()) // note extra space to align with "Inserting..."

	return &newEpisode, txCtx.Commit()
}

func Update5mEpisode(dbCtx *dbcontext.DbContext, slot time.Time, ref check.ProbeRef) (*check.Episode, error) {
	// Get all records of 30s slots within a 5 min slot.
	dao30s := dao.NewEpisodeDao30s(dbCtx)
	items, err := dao30s.ListForRange(slot, slot.Add(5*time.Minute), ref)
	if err != nil {
		log.Errorf("cannot fetch 30s episodes by slot=%s ref=%q: %v", slot.Format(time.Stamp), ref.Id(), err)
		return nil, err
	}

	// Sum up 30 sec episodes.
	ep30s := check.Episode{}
	for _, item := range items {
		ep30s.Up += item.Episode.Up
		ep30s.Down += item.Episode.Down
		ep30s.Unknown += item.Episode.Unknown
		ep30s.NoData += item.Episode.NoData
	}

	txCtx, err := dbCtx.BeginTransaction()
	if err != nil {
		return nil, err
	}
	dao5m := dao.NewEpisodeDao5m(txCtx)

	// Get stored 5 min episode.
	entity5m, err := dao5m.GetBySlotAndProbe(slot, ref)
	if err != nil {
		log.Warnf("cannot fetch 5min episode by slot=%s ref=%q: %v", slot.Format(time.Stamp), ref.Id(), err)
	}

	combinedEpisode := entity5m.Episode.CombineSeconds(ep30s, 5*time.Minute)

	// Do not update 5m episode if nothing has changed in 30s episodes.
	if entity5m.Rowid != -1 && entity5m.Episode.EqualTimers(combinedEpisode) {
		rollErr := txCtx.Rollback()
		if rollErr != nil {
			return nil, fmt.Errorf("%v, and rollback %v", ErrNotChanged, rollErr)
		}
		return nil, ErrNotChanged
	}

	// Assert that new success duration is not "worse"
	if combinedEpisode.Up < entity5m.Episode.Up {
		log.Warnf("Possible bug!!! Combined Up=%d for 5m episode is worse than saved %d for slot=%s ref=%q",
			combinedEpisode.Up, entity5m.Episode.Up,
			entity5m.Episode.TimeSlot.Format(time.Stamp), entity5m.Episode.ProbeRef.Id())
	}

	// Update entity with combined seconds
	entity5m.Episode.Up = combinedEpisode.Up
	entity5m.Episode.Down = combinedEpisode.Down
	entity5m.Episode.Unknown = combinedEpisode.Unknown
	entity5m.Episode.NoData = combinedEpisode.NoData

	if entity5m.Rowid == -1 {
		// Insert
		entity5m.Episode.ProbeRef.Group = ref.Group
		entity5m.Episode.ProbeRef.Probe = ref.Probe
		entity5m.Episode.TimeSlot = slot

		err = dao5m.Insert(entity5m.Episode)
		if err != nil {
			err := fmt.Errorf("cannot insert: %v", err)

			rollErr := txCtx.Rollback()
			if rollErr != nil {
				return nil, fmt.Errorf("%v, and rollback: %v", err, rollErr)
			}
			return nil, err
		}

		if err = txCtx.Commit(); err != nil {
			return nil, err
		}
		return &entity5m.Episode, nil
	}

	// Update
	err = dao5m.Update(entity5m.Rowid, entity5m.Episode)
	if err != nil {
		err := fmt.Errorf("cannot update: %v", err)

		rollErr := txCtx.Rollback()
		if rollErr != nil {
			return nil, fmt.Errorf("%v, and rollback: %v", err, rollErr)
		}
		return nil, err
	}

	if err = txCtx.Commit(); err != nil {
		return nil, err
	}
	return &entity5m.Episode, nil
}
