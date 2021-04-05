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
func Save30sEpisodes(ctx *dbcontext.DbContext, episodes []check.DowntimeEpisode) []*check.DowntimeEpisode {

	saved := make([]*check.DowntimeEpisode, 0)
	dayAgo := time.Now().Add(-24 * time.Hour)

	for _, episode := range episodes {
		// Ignore episodes older then 24h.
		episodeStart := time.Unix(episode.TimeSlot, 0)
		if episodeStart.Before(dayAgo) {
			log.Warnf("Ignoring outdated episode: %s", episode.DumpString())
			continue
		}

		if !episode.IsCorrect(30) {
			log.Errorf("Possible bug!!! Ignoring incorrect episode: %s", episode.DumpString())
			continue
		}

		ep, err := Save30sEpisode(ctx, episode)
		if err != nil {
			log.Errorf("cannot save episode slot=%d, red=%q: %v", episode.TimeSlot, episode.ProbeRef.Id(), err)
			continue
		}
		saved = append(saved, ep)
	}

	return saved

}

// Update5mEpisodes stores 5 min downtime episodes into database.
func Update5mEpisodes(ctx *dbcontext.DbContext, episodes30s []*check.DowntimeEpisode) []*check.DowntimeEpisode {
	// slot -> unique probe ID
	probesBySlot5m := make(map[int64]map[string]check.ProbeRef)
	saved := make([]*check.DowntimeEpisode, 0)

	for _, episode := range episodes30s {
		epStart := time.Unix(episode.TimeSlot, 0)
		slot5m := epStart.Truncate(5 * time.Minute).Unix()

		if _, ok := probesBySlot5m[slot5m]; !ok {
			probesBySlot5m[slot5m] = make(map[string]check.ProbeRef)
		}

		probesBySlot5m[slot5m][episode.ProbeRef.Id()] = episode.ProbeRef
	}

	for slot5m, probeRefs := range probesBySlot5m {
		for _, ref := range probeRefs {
			episode, err := Update5mEpisode(ctx, slot5m, ref)
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

// Insert or Update a DowntimeEpisode using a transaction.
func Save30sEpisode(dbCtx *dbcontext.DbContext, episode check.DowntimeEpisode) (*check.DowntimeEpisode, error) {
	txCtx, err := dbCtx.BeginTransaction()
	if err != nil {
		return nil, err
	}
	dao30s := dao.NewDowntime30sDao(txCtx)

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
		log.Infof("Inserting 30s episode %s", episode.DumpString())

		return &episode, txCtx.Commit()
	}

	newEpisode := episode.CombineSeconds(storedEpisode.DowntimeEpisode, 30)
	err = dao30s.Update(storedEpisode.Rowid, newEpisode)
	if err != nil {
		err := fmt.Errorf("cannot update: %v", err)

		rollErr := txCtx.Rollback()
		if rollErr != nil {
			return nil, fmt.Errorf("%v, rollback: %v", err, rollErr)
		}
		return nil, err
	}
	log.Infof("Updating  30s episode %s", episode.DumpString()) // note extra space to align with "Inserting..."

	return &newEpisode, txCtx.Commit()
}

func Update5mEpisode(dbCtx *dbcontext.DbContext, slot5m int64, ref check.ProbeRef) (*check.DowntimeEpisode, error) {
	// Get all records of 30s slots within a 5 min slot.
	dao30s := dao.NewDowntime30sDao(dbCtx)
	items, err := dao30s.ListForRange(slot5m, slot5m+300, ref)
	if err != nil {
		log.Errorf("cannot fetch 30s episodes by slot=%d ref=%q: %v", slot5m, ref.Id(), err)
		return nil, err
	}

	// Sum up 30 sec episodes.
	totalDowntime30s := check.DowntimeEpisode{}
	for _, item := range items {
		totalDowntime30s.SuccessSeconds += item.DowntimeEpisode.SuccessSeconds
		totalDowntime30s.FailSeconds += item.DowntimeEpisode.FailSeconds
		totalDowntime30s.UnknownSeconds += item.DowntimeEpisode.UnknownSeconds
		totalDowntime30s.NoDataSeconds += item.DowntimeEpisode.NoDataSeconds
	}

	txCtx, err := dbCtx.BeginTransaction()
	if err != nil {
		return nil, err
	}
	dao5m := dao.NewDowntime5mDao(txCtx)

	// Get stored 5 min episode.
	entity5m, err := dao5m.GetBySlotAndProbe(slot5m, ref)
	if err != nil {
		log.Warnf("cannot fetch 5min episode by slot=%d ref=%q: %v", slot5m, ref.Id(), err)
	}

	combinedEpisode := entity5m.DowntimeEpisode.CombineSeconds(totalDowntime30s, 300)

	// Do not update 5m episode if nothing has changed in 30s episodes.
	if entity5m.Rowid != -1 && entity5m.DowntimeEpisode.IsEqualSeconds(combinedEpisode) {
		rollErr := txCtx.Rollback()
		if rollErr != nil {
			return nil, fmt.Errorf("%v, and rollback %v", ErrNotChanged, rollErr)
		}
		return nil, ErrNotChanged
	}

	// Assert that new success duration is not "worse"
	if combinedEpisode.SuccessSeconds < entity5m.DowntimeEpisode.SuccessSeconds {
		log.Warnf("Possible bug!!! Combined SuccessSeconds=%d for 5m episode is worse than saved %d for slot=%d ref=%q",
			combinedEpisode.SuccessSeconds, entity5m.DowntimeEpisode.SuccessSeconds,
			entity5m.DowntimeEpisode.TimeSlot, entity5m.DowntimeEpisode.ProbeRef.Id())
	}

	// Update entity with combined seconds
	entity5m.DowntimeEpisode.SuccessSeconds = combinedEpisode.SuccessSeconds
	entity5m.DowntimeEpisode.FailSeconds = combinedEpisode.FailSeconds
	entity5m.DowntimeEpisode.UnknownSeconds = combinedEpisode.UnknownSeconds
	entity5m.DowntimeEpisode.NoDataSeconds = combinedEpisode.NoDataSeconds

	if entity5m.Rowid == -1 {
		// Insert
		entity5m.DowntimeEpisode.ProbeRef.Group = ref.Group
		entity5m.DowntimeEpisode.ProbeRef.Probe = ref.Probe
		entity5m.DowntimeEpisode.TimeSlot = slot5m

		err = dao5m.Insert(entity5m.DowntimeEpisode)
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
		return &entity5m.DowntimeEpisode, nil
	}

	// Update
	err = dao5m.Update(entity5m.Rowid, entity5m.DowntimeEpisode)
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
	return &entity5m.DowntimeEpisode, nil
}
