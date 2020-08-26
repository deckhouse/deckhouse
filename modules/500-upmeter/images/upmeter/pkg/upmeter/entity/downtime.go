package entity

import (
	"time"

	log "github.com/sirupsen/logrus"

	"upmeter/pkg/probe/types"
	"upmeter/pkg/upmeter/db"
)

// TODO save into sqlite database
func SaveDowntimeEpisodes(episodes []types.DowntimeEpisode) {
	ageLimit := time.Now().Unix() - 24*60*60
	probesInFiveMinSlots := make(map[int64]map[string]types.ProbeRef)
	for _, episode := range episodes {
		//log.Infof("got episode for probe '%s'", episode.ProbeRef.ProbeId())
		// Ignore episodes older then 24h.
		if episode.TimeSlot < ageLimit {
			log.Infof("Ignore episode: ts=%d probe='%s' s=%d f=%d", episode.TimeSlot, episode.ProbeRef.ProbeId(), episode.SuccessSeconds, episode.FailSeconds)
			continue
		}
		// Clamp seconds to 30
		if episode.SuccessSeconds > 30 || episode.FailSeconds > 30 || episode.SuccessSeconds+episode.FailSeconds > 30 {
			log.Infof("Possible bug!!! Ignore episode with incorrect seconds: ts=%d probe='%s' s=%d f=%d", episode.TimeSlot, episode.ProbeRef.ProbeId(), episode.SuccessSeconds, episode.FailSeconds)
			continue
		}

		storedEpisode, err := db.Downtime30s.GetSimilar(episode)
		if err != nil {
			log.Errorf("Get stored episode for ts=%d, probe='%s': %v", episode.TimeSlot, episode.ProbeRef.ProbeId(), err)
			continue
		}

		if storedEpisode.Rowid == -1 {
			err = db.Downtime30s.Save(episode)
			if err != nil {
				log.Errorf("Save new episode for ts=%d, probe='%s': %v", episode.TimeSlot, episode.ProbeRef.ProbeId(), err)
				continue
			}
			log.Infof("Save episode: ts=%d probe='%s' s=%d f=%d", episode.TimeSlot, episode.ProbeRef.ProbeId(), episode.SuccessSeconds, episode.FailSeconds)
		} else {
			newEpisode := CombineEpisodes(episode, storedEpisode.DowntimeEpisode)
			err = db.Downtime30s.Update(storedEpisode.Rowid, newEpisode)
			if err != nil {
				log.Errorf("Update episode for ts=%d, probe='%s': %v", episode.TimeSlot, episode.ProbeRef.ProbeId(), err)
				continue
			}
			log.Infof("Update episode: ts=%d probe='%s' s=%d f=%d", episode.TimeSlot, episode.ProbeRef.ProbeId(), episode.SuccessSeconds, episode.FailSeconds)
		}

		// save 5 min slot
		slot5m := Get5MinSlot(episode.TimeSlot)
		if _, ok := probesInFiveMinSlots[slot5m]; !ok {
			probesInFiveMinSlots[slot5m] = make(map[string]types.ProbeRef)
		}
		probesInFiveMinSlots[slot5m][episode.ProbeRef.ProbeId()] = episode.ProbeRef
	}

	for slot5m, probeRefs := range probesInFiveMinSlots {
		for _, probeRef := range probeRefs {
			Update5MinStorage(slot5m, probeRef.Group, probeRef.Probe)
		}
	}

}

func Update5MinStorage(slot5m int64, group string, probe string) {
	// Get all records of 30s slots within 5 min slot
	items, err := db.Downtime30s.ListForRange(slot5m, slot5m+299, group, probe)
	if err != nil {
		log.Errorf("List episodes for 5 min slot %d for group='%s' and probe='%s': %v", slot5m, group, probe, err)
	}

	var successSeconds, failSeconds int64
	for _, item := range items {
		successSeconds += item.DowntimeEpisode.SuccessSeconds
		failSeconds += item.DowntimeEpisode.FailSeconds
	}

	entity5m, err := db.Downtime5m.GetBySlotAndProbe(slot5m, group, probe)
	if err != nil {
		log.Errorf("Get 5min episode: slot %d for group='%s' and probe='%s': %v", slot5m, group, probe, err)
	}

	newSuccess, newFail := CombineSeconds(
		entity5m.DowntimeEpisode.SuccessSeconds, entity5m.DowntimeEpisode.FailSeconds,
		successSeconds, failSeconds)

	// Do not update 5m episode if nothing has changed in 30s episodes.
	if entity5m.Rowid != -1 && entity5m.DowntimeEpisode.SuccessSeconds == newSuccess && entity5m.DowntimeEpisode.FailSeconds == newFail {
		log.Infof("5m episode not changed: ts=%d probe='%s' s=%d f=%d", entity5m.DowntimeEpisode.TimeSlot, entity5m.DowntimeEpisode.ProbeRef.ProbeId(), entity5m.DowntimeEpisode.SuccessSeconds, entity5m.DowntimeEpisode.FailSeconds)

		return
	}

	// Assert that new success duration is not "worse"
	if newSuccess < entity5m.DowntimeEpisode.SuccessSeconds {
		log.Errorf("Possible bug!!! Combined SuccessDuration for 5m episode '%d' is worse than saved '%d' for slot=%d probe='%s'",
			successSeconds, entity5m.DowntimeEpisode.SuccessSeconds,
			entity5m.DowntimeEpisode.TimeSlot, entity5m.DowntimeEpisode.ProbeRef.ProbeId())
	}

	entity5m.DowntimeEpisode.SuccessSeconds = newSuccess
	entity5m.DowntimeEpisode.FailSeconds = newFail

	if entity5m.Rowid == -1 {
		// save
		entity5m.DowntimeEpisode.ProbeRef.Group = group
		entity5m.DowntimeEpisode.ProbeRef.Probe = probe
		entity5m.DowntimeEpisode.TimeSlot = slot5m
		err = db.Downtime5m.Save(entity5m.DowntimeEpisode)
		log.Infof("Save 5m episode: ts=%d probe='%s' s=%d f=%d", entity5m.DowntimeEpisode.TimeSlot, entity5m.DowntimeEpisode.ProbeRef.ProbeId(), entity5m.DowntimeEpisode.SuccessSeconds, entity5m.DowntimeEpisode.FailSeconds)

	} else {
		// update
		err = db.Downtime5m.Update(entity5m.Rowid, entity5m.DowntimeEpisode)
		if err != nil {
			log.Errorf("Update 5m episode for ts=%d, probe='%s': %v", entity5m.DowntimeEpisode.TimeSlot, entity5m.DowntimeEpisode.ProbeRef.ProbeId(), err)
		} else {
			log.Infof("Update 5m episode: ts=%d probe='%s' s=%d f=%d", entity5m.DowntimeEpisode.TimeSlot, entity5m.DowntimeEpisode.ProbeRef.ProbeId(), entity5m.DowntimeEpisode.SuccessSeconds, entity5m.DowntimeEpisode.FailSeconds)
		}
	}
}

// CombineEpisodes returns combined DowntimeEpisodes with preference to maximize success time and minimize unknown time from failed time.
func CombineEpisodes(stored types.DowntimeEpisode, new types.DowntimeEpisode) types.DowntimeEpisode {
	// assert timestamps are equal
	if stored.TimeSlot != new.TimeSlot {
		log.Errorf("Possible bug!!! Try to combine episodes with different timestamps: %d %d", stored.TimeSlot, new.TimeSlot)
	}

	res := types.DowntimeEpisode{
		ProbeRef: types.ProbeRef{
			Group: stored.ProbeRef.Group,
			Probe: stored.ProbeRef.Probe,
		},
		TimeSlot: stored.TimeSlot,
	}

	res.SuccessSeconds, res.FailSeconds = CombineSeconds(stored.SuccessSeconds, stored.FailSeconds,
		new.SuccessSeconds, new.FailSeconds)

	return res
}

func CombineSeconds(aSuccess, aFail, bSuccess, bFail int64) (successSeconds, failSeconds int64) {
	var betterSuccess = aSuccess
	var betterFail = aFail
	var worseSuccess = bSuccess
	var worseFail = bFail
	if aSuccess < bSuccess {
		betterSuccess = bSuccess
		betterFail = bFail
		worseSuccess = aSuccess
		worseFail = aFail
	}

	successSeconds = betterSuccess
	failSeconds = betterFail

	// If the worse seconds has more observed information and
	// the worse seconds has more failed seconds, increase failed seconds
	// for the better seconds, but not to overlap its success seconds.
	knownSecondsInBetter := betterSuccess + betterFail
	knownSecondsInWorse := worseSuccess + worseFail
	if knownSecondsInBetter < knownSecondsInWorse {
		failSeconds = ClampToRange(worseFail,
			betterFail,
			knownSecondsInWorse-betterSuccess)
	}
	return
}
