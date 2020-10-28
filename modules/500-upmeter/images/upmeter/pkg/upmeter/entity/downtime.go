package entity

import (
	"time"

	log "github.com/sirupsen/logrus"

	"upmeter/pkg/probe/types"
	"upmeter/pkg/upmeter/db/dao"
)

// SaveDowntimeEpisodes stores 30 sec downtime episodes into database.
// It also clears old records and update 5 minute episodes in a database.
func SaveDowntimeEpisodes(episodes []types.DowntimeEpisode) {
	minTimeslot := time.Now().Unix() - 24*60*60
	probesInFiveMinSlots := make(map[int64]map[string]types.ProbeRef)
	for _, episode := range episodes {
		// Ignore episodes older then 24h.
		if episode.TimeSlot < minTimeslot {
			log.Infof("Ignore episode: %s", episode.DumpString())
			continue
		}

		if !episode.IsCorrect(30) {
			log.Infof("Possible bug!!! Ignore incorrect episode: %s", episode.DumpString())
			continue
		}

		storedEpisode, err := dao.Downtime30s.GetSimilar(episode)
		if err != nil {
			log.Errorf("Get stored episode for ts=%d, probe='%s': %v", episode.TimeSlot, episode.ProbeRef.ProbeId(), err)
			continue
		}

		if storedEpisode.Rowid == -1 {
			err = dao.Downtime30s.Save(episode)
			if err != nil {
				log.Errorf("Save new episode for ts=%d, probe='%s': %v", episode.TimeSlot, episode.ProbeRef.ProbeId(), err)
				continue
			}
			log.Infof("Save episode: ts=%d probe='%s' s=%d f=%d", episode.TimeSlot, episode.ProbeRef.ProbeId(), episode.SuccessSeconds, episode.FailSeconds)
		} else {
			newEpisode := episode.CombineSeconds(storedEpisode.DowntimeEpisode, 30)
			err = dao.Downtime30s.Update(storedEpisode.Rowid, newEpisode)
			if err != nil {
				log.Errorf("Update episode for ts=%d, probe='%s': %v", episode.TimeSlot, episode.ProbeRef.ProbeId(), err)
				continue
			}
			log.Infof("Update episode: ts=%d probe='%s' s=%d f=%d", episode.TimeSlot, episode.ProbeRef.ProbeId(), episode.SuccessSeconds, episode.FailSeconds)
		}

		// Save involved 5 min slot and ProbeRef to update 5m episodes later
		slot5m := Get5MinSlot(episode.TimeSlot)
		if _, ok := probesInFiveMinSlots[slot5m]; !ok {
			probesInFiveMinSlots[slot5m] = make(map[string]types.ProbeRef)
		}
		probesInFiveMinSlots[slot5m][episode.ProbeRef.ProbeId()] = episode.ProbeRef
	}

	// Update involved 5 min episodes
	for slot5m, probeRefs := range probesInFiveMinSlots {
		for _, probeRef := range probeRefs {
			Update5MinStorage(slot5m, probeRef.Group, probeRef.Probe)
		}
	}

}

func Update5MinStorage(slot5m int64, group string, probe string) {
	// Get all records of 30s slots within 5 min slot
	items, err := dao.Downtime30s.ListForRange(slot5m, slot5m+299, group, probe)
	if err != nil {
		log.Errorf("List episodes for 5 min slot %d for group='%s' and probe='%s': %v", slot5m, group, probe, err)
	}

	// Sum up 30 sec episodes.
	totalDowntime30s := types.DowntimeEpisode{}
	for _, item := range items {
		totalDowntime30s.SuccessSeconds += item.DowntimeEpisode.SuccessSeconds
		totalDowntime30s.FailSeconds += item.DowntimeEpisode.FailSeconds
		totalDowntime30s.Unknown += item.DowntimeEpisode.Unknown
		totalDowntime30s.NoData += item.DowntimeEpisode.NoData
	}

	// Get stored 5 min episode.
	entity5m, err := dao.Downtime5m.GetBySlotAndProbe(slot5m, group, probe)
	if err != nil {
		log.Errorf("Get 5min episode: slot %d for group='%s' and probe='%s': %v", slot5m, group, probe, err)
	}

	combinedEpisode := entity5m.DowntimeEpisode.CombineSeconds(totalDowntime30s, 300)

	// Do not update 5m episode if nothing has changed in 30s episodes.
	if entity5m.Rowid != -1 && entity5m.DowntimeEpisode.IsEqualSeconds(combinedEpisode) {
		log.Infof("5m episode is not changed: %s", entity5m.DowntimeEpisode.DumpString())
		return
	}

	// Assert that new success duration is not "worse"
	if combinedEpisode.SuccessSeconds < entity5m.DowntimeEpisode.SuccessSeconds {
		log.Errorf("Possible bug!!! Combined SuccessDuration for 5m episode '%d' is worse than saved '%d' for slot=%d probe='%s'",
			combinedEpisode.SuccessSeconds, entity5m.DowntimeEpisode.SuccessSeconds,
			entity5m.DowntimeEpisode.TimeSlot, entity5m.DowntimeEpisode.ProbeRef.ProbeId())
	}

	// Update entity with combined seconds
	entity5m.DowntimeEpisode.SuccessSeconds = combinedEpisode.SuccessSeconds
	entity5m.DowntimeEpisode.FailSeconds = combinedEpisode.FailSeconds
	entity5m.DowntimeEpisode.Unknown = combinedEpisode.Unknown
	entity5m.DowntimeEpisode.NoData = combinedEpisode.NoData

	if entity5m.Rowid == -1 {
		// create
		entity5m.DowntimeEpisode.ProbeRef.Group = group
		entity5m.DowntimeEpisode.ProbeRef.Probe = probe
		entity5m.DowntimeEpisode.TimeSlot = slot5m
		err = dao.Downtime5m.Save(entity5m.DowntimeEpisode)
		log.Infof("Save 5m episode: %s", entity5m.DowntimeEpisode.DumpString())
	} else {
		// update
		err = dao.Downtime5m.Update(entity5m.Rowid, entity5m.DowntimeEpisode)
		if err != nil {
			log.Errorf("Update 5m episode for ts=%d, probe='%s': %v", entity5m.DowntimeEpisode.TimeSlot, entity5m.DowntimeEpisode.ProbeRef.ProbeId(), err)
		} else {
			log.Infof("Update 5m episode: %s", entity5m.DowntimeEpisode.DumpString())
		}
	}
}
