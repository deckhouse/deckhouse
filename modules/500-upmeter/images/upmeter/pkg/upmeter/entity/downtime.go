package entity

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"upmeter/pkg/checks"
	dbcontext "upmeter/pkg/upmeter/db/context"
	"upmeter/pkg/upmeter/db/dao"
)

// SaveDowntimeEpisodes stores 30 sec downtime episodes into database.
// It also clears old records and update 5 minute episodes in a database.
func SaveDowntimeEpisodes(dbCtx *dbcontext.DbContext, episodes []checks.DowntimeEpisode) {
	saveCtx := dbCtx.Start()
	defer saveCtx.Stop()

	minTimeslot := time.Now().Unix() - 24*60*60
	probesInFiveMinSlots := make(map[int64]map[string]checks.ProbeRef)
	for _, episode := range episodes {
		// Ignore episodes older then 24h.
		if episode.TimeSlot < minTimeslot {
			log.Infof("Ignore episode: %s", episode.DumpString())
			continue
		}

		if !episode.IsCorrect(30) {
			log.Errorf("Possible bug!!! Ignore incorrect episode: %s", episode.DumpString())
			continue
		}

		err := Save30sEpisode(saveCtx, episode)
		if err != nil {
			log.Errorf("Save episode for ts=%d, probe='%s': %v", episode.TimeSlot, episode.ProbeRef.Id(), err)
		}

		// Save involved 5 min slot and ProbeRef to update 5m episodes later
		slot5m := Calculate5MinSlot(episode.TimeSlot)
		if _, ok := probesInFiveMinSlots[slot5m]; !ok {
			probesInFiveMinSlots[slot5m] = make(map[string]checks.ProbeRef)
		}
		probesInFiveMinSlots[slot5m][episode.ProbeRef.Id()] = episode.ProbeRef
	}

	// Update involved 5 min episodes
	for slot5m, probeRefs := range probesInFiveMinSlots {
		for _, probeRef := range probeRefs {
			err := Update5MinStorage(saveCtx, slot5m, probeRef)
			if err != nil {
				log.Errorf("Save 5m episode for ts=%d, probe='%s': %v", slot5m, probeRef.Id(), err)
			}
		}
	}
}

// Insert or Update a DowntimeEpisode using a transaction.
func Save30sEpisode(dbCtx *dbcontext.DbContext, episode checks.DowntimeEpisode) error {
	txCtx, err := dbCtx.BeginTransaction()
	if err != nil {
		return err
	}
	dao30s := dao.NewDowntime30sDao(txCtx)

	storedEpisode, err := dao30s.GetSimilar(episode)
	if err != nil {
		log.Errorf("Get similar 30s episode for ts=%d, probe='%s': %v", episode.TimeSlot, episode.ProbeRef.Id(), err)
		rollErr := txCtx.Rollback()
		if rollErr != nil {
			return fmt.Errorf("select 30s similar episode for ts=%d, probe='%s': %v, rollback: %v", episode.TimeSlot, episode.ProbeRef.Id(), err, rollErr)
		}
		return err
	}

	if storedEpisode.Rowid == -1 {
		err = dao30s.Insert(episode)
		if err != nil {
			log.Errorf("Insert 30s episode for ts=%d, probe='%s': %v", episode.TimeSlot, episode.ProbeRef.Id(), err)
			rollErr := txCtx.Rollback()
			if rollErr != nil {
				return fmt.Errorf("insert 30s episode for ts=%d, probe='%s': %v, rollback: %v", episode.TimeSlot, episode.ProbeRef.Id(), err, rollErr)
			}
			return fmt.Errorf("insert 30s episode for ts=%d, probe='%s': %v", episode.TimeSlot, episode.ProbeRef.Id(), err)
		}
		log.Infof("Insert 30s episode: ts=%d probe='%s' s=%d f=%d", episode.TimeSlot, episode.ProbeRef.Id(), episode.SuccessSeconds, episode.FailSeconds)
	} else {
		newEpisode := episode.CombineSeconds(storedEpisode.DowntimeEpisode, 30)
		err = dao30s.Update(storedEpisode.Rowid, newEpisode)
		if err != nil {
			log.Errorf("Update 30s episode for ts=%d, probe='%s': %v", episode.TimeSlot, episode.ProbeRef.Id(), err)
			rollErr := txCtx.Rollback()
			if rollErr != nil {
				return fmt.Errorf("update 30s episode for ts=%d, probe='%s': %v, rollback: %v", episode.TimeSlot, episode.ProbeRef.Id(), err, rollErr)
			}
			return fmt.Errorf("update 30s episode for ts=%d, probe='%s': %v", episode.TimeSlot, episode.ProbeRef.Id(), err)
		}
		log.Infof("Update 30s episode: ts=%d probe='%s' s=%d f=%d", episode.TimeSlot, episode.ProbeRef.Id(), episode.SuccessSeconds, episode.FailSeconds)
	}

	return txCtx.Commit()
}

func Update5MinStorage(dbCtx *dbcontext.DbContext, slot5m int64, ref checks.ProbeRef) error {
	// Get all records of 30s slots within a 5 min slot.
	dao30s := dao.NewDowntime30sDao(dbCtx)
	items, err := dao30s.ListForRange(slot5m, slot5m+299, ref.Group, ref.Probe)
	if err != nil {
		log.Errorf("List episodes for 5 min slot %d for group='%s' and probe='%s': %v", slot5m, ref.Group, ref.Probe, err)
	}

	// Sum up 30 sec episodes.
	totalDowntime30s := checks.DowntimeEpisode{}
	for _, item := range items {
		totalDowntime30s.SuccessSeconds += item.DowntimeEpisode.SuccessSeconds
		totalDowntime30s.FailSeconds += item.DowntimeEpisode.FailSeconds
		totalDowntime30s.UnknownSeconds += item.DowntimeEpisode.UnknownSeconds
		totalDowntime30s.NoDataSeconds += item.DowntimeEpisode.NoDataSeconds
	}

	txCtx, err := dbCtx.BeginTransaction()
	if err != nil {
		return err
	}
	dao5m := dao.NewDowntime5mDao(txCtx)

	// Get stored 5 min episode.
	entity5m, err := dao5m.GetBySlotAndProbe(slot5m, ref)
	if err != nil {
		log.Errorf("Get 5min episode: slot %d for group='%s' and probe='%s': %v", slot5m, ref.Group, ref.Probe, err)
	}

	combinedEpisode := entity5m.DowntimeEpisode.CombineSeconds(totalDowntime30s, 300)

	// Do not update 5m episode if nothing has changed in 30s episodes.
	if entity5m.Rowid != -1 && entity5m.DowntimeEpisode.IsEqualSeconds(combinedEpisode) {
		log.Infof("5m episode is not changed: %s", entity5m.DowntimeEpisode.DumpString())
		rollErr := txCtx.Rollback()
		if rollErr != nil {
			return fmt.Errorf("get 5m episode for ts=%d, probe='%s/%s': rollback: %v", slot5m, ref.Group, ref.Probe, rollErr)
		}
		return nil
	}

	// Assert that new success duration is not "worse"
	if combinedEpisode.SuccessSeconds < entity5m.DowntimeEpisode.SuccessSeconds {
		log.Errorf("Possible bug!!! Combined SuccessDuration for 5m episode '%d' is worse than saved '%d' for slot=%d probe='%s'",
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
			log.Errorf("Insert 5m episode for ts=%d, probe='%s': %v", entity5m.DowntimeEpisode.TimeSlot, entity5m.DowntimeEpisode.ProbeRef.Id(), err)
			rollErr := txCtx.Rollback()
			if rollErr != nil {
				return fmt.Errorf("insert 5m episode for ts=%d, probe='%s': %v, rollback: %v", entity5m.DowntimeEpisode.TimeSlot, entity5m.DowntimeEpisode.ProbeRef.Id(), err, rollErr)
			}
			return fmt.Errorf("insert 5m episode for ts=%d, probe='%s': %v", entity5m.DowntimeEpisode.TimeSlot, entity5m.DowntimeEpisode.ProbeRef.Id(), err)
		}
		log.Infof("Save 5m episode: %s", entity5m.DowntimeEpisode.DumpString())
	} else {
		// Update
		err = dao5m.Update(entity5m.Rowid, entity5m.DowntimeEpisode)
		if err != nil {
			log.Errorf("Update 5m episode for ts=%d, probe='%s': %v", entity5m.DowntimeEpisode.TimeSlot, entity5m.DowntimeEpisode.ProbeRef.Id(), err)
			rollErr := txCtx.Rollback()
			if rollErr != nil {
				return fmt.Errorf("update 5m episode for ts=%d, probe='%s': %v, rollback: %v", entity5m.DowntimeEpisode.TimeSlot, entity5m.DowntimeEpisode.ProbeRef.Id(), err, rollErr)
			}
			return fmt.Errorf("update 5m episode for ts=%d, probe='%s': %v", entity5m.DowntimeEpisode.TimeSlot, entity5m.DowntimeEpisode.ProbeRef.Id(), err)
		}
		log.Infof("Update 5m episode: %s", entity5m.DowntimeEpisode.DumpString())
	}

	return txCtx.Commit()
}
