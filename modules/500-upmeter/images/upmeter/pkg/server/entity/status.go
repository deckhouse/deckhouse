/*
Copyright 2021 Flant CJSC

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
	"sort"
	"time"

	log "github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/check"
	dbcontext "d8.io/upmeter/pkg/db/context"
	"d8.io/upmeter/pkg/db/dao"
	"d8.io/upmeter/pkg/server/ranges"
	utime "d8.io/upmeter/pkg/time"
	"d8.io/upmeter/pkg/util"
)

type EpisodeSummary struct {
	TimeSlot  int64                    `json:"ts"`
	StartDate string                   `json:"start"`
	EndDate   string                   `json:"end"`
	Up        time.Duration            `json:"up"`
	Down      time.Duration            `json:"down"`
	Unknown   time.Duration            `json:"unknown"`
	Muted     time.Duration            `json:"muted"`
	NoData    time.Duration            `json:"nodata"`
	Downtimes []check.DowntimeIncident `json:"downtimes"`
}

func newEpisodeSummary(stepRange ranges.Range) *EpisodeSummary {
	return &EpisodeSummary{
		TimeSlot:  stepRange.From,
		StartDate: time.Unix(stepRange.From, 0).Format(time.RFC3339),
		EndDate:   time.Unix(stepRange.To, 0).Format(time.RFC3339),
		NoData:    stepRange.Diff(),
	}
}

func (s *EpisodeSummary) addEpisode(ep check.Episode) {
	s.Up += ep.Up
	s.Down += ep.Down
	s.Unknown += ep.Unknown
	s.NoData -= ep.Up + ep.Down + ep.Unknown
}

func (s *EpisodeSummary) add(other *EpisodeSummary) {
	s.Up += other.Up
	s.Down += other.Down
	s.Unknown += other.Unknown
	s.NoData += other.NoData
	s.Muted += other.Muted
}

func (s *EpisodeSummary) Known() time.Duration {
	return s.Up + s.Down
}

func (s *EpisodeSummary) Avail() time.Duration {
	return s.Up + s.Down + s.Unknown
}

// ByTimeSlot implements sort.Interface based on the TimeSlot field.
type ByTimeSlot []EpisodeSummary

func (a ByTimeSlot) Len() int      { return len(a) }
func (a ByTimeSlot) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByTimeSlot) Less(i, j int) bool {
	if a[i].TimeSlot == -1 {
		return false
	}
	if a[j].TimeSlot == -1 {
		return true
	}
	return a[i].TimeSlot < a[j].TimeSlot
}

const TotalProbeName = "__total__"

func FetchStatuses(dbctx *dbcontext.DbContext, ref check.ProbeRef, rng ranges.StepRange, incidents []check.DowntimeIncident) (map[string]map[string][]EpisodeSummary, error) {
	daoCtx := dbctx.Start()
	defer daoCtx.Stop()

	dao5m := dao.NewEpisodeDao5m(daoCtx)
	episodes, err := dao5m.ListEpisodeSumsForRanges(rng, ref)
	if err != nil {
		return nil, err
	}

	statuses := calculateStatuses(episodes, incidents, rng.Subranges, ref)
	return statuses, nil
}

/**
calculateStatuses returns arrays of EpisodeSummary objects for each group and probe.
Each EpisodeSummary object is combined from Episodes for a step range.
If grouping is "group", then all probes in group are summed up and
probe name is "__total__".

Method considers that there is only one Episode for probe and Start.

Example output:

testGroup:
  testProbe:
  - timeslot: 0
    up: 300
    down: 0
  - timeslot: 300
    up: 300
    down: 0
  - timeslot: 900
    up: 300
    down: 0
*/
func calculateStatuses(episodes []check.Episode, incidents []check.DowntimeIncident, stepRanges []ranges.Range, ref check.ProbeRef) map[string]map[string][]EpisodeSummary {
	episodes = filterDisabledProbesFromEpisodes(episodes)

	// Combine multiple episodes for the same probe and timeslot.
	episodes = combineEpisodesByTimeslot(episodes)

	// Create table with empty statuses for each probe
	//     Group  ->  Probe  ->  Slot -> *EpisodeSummary
	// map[string]map[string]map[int64]*EpisodeSummary
	statuses := newSummaryTable(episodes, stepRanges, ref)

	// Sum up episodes for each probe by Start within each step range.
	// TODO various optimizations can be applied here.
	for _, stepRange := range stepRanges {
		for _, episode := range episodes {
			if !episode.IsInRange(stepRange.From, stepRange.To) {
				continue
			}

			statuses[episode.ProbeRef.Group][episode.ProbeRef.Probe][stepRange.From].addEpisode(episode)
		}
		calculateTotalForStepRange(statuses, stepRange)
	}

	updateMute(statuses, incidents, stepRanges)

	// Calculate group-level summaries including __total__
	calculateTotalForPeriod(statuses, stepRanges)

	return transformTimestampedMapsToSortedArrays(statuses, ref)
}

// Each group/probe should have only 1 Episode per Start.
func combineEpisodesByTimeslot(episodes []check.Episode) []check.Episode {
	idx := make(map[string]map[int64][]int)
	for i, episode := range episodes {
		probeId := episode.ProbeRef.Id()
		if _, ok := idx[probeId]; !ok {
			idx[probeId] = make(map[int64][]int)
		}
		start := episode.TimeSlot.Unix()
		if _, ok := idx[probeId][start]; !ok {
			idx[probeId][start] = make([]int, 0)
		}
		idx[probeId][start] = append(idx[probeId][start], i)
	}

	newEpisodes := make([]check.Episode, 0)
	for _, timeslots := range idx {
		for _, indices := range timeslots {
			ep := episodes[indices[0]]
			for _, index := range indices {
				ep = ep.Combine(episodes[index], 5*time.Minute)
			}
			newEpisodes = append(newEpisodes, ep)
		}
	}

	return newEpisodes
}

// calculateTotalForStepRange calculates a group episode for a range. It combines group's probes
// into one "__total__" probe. In webui it is presented as a cloumn summary for probes in a group.
//
// The total episode calculation statuses:
//      - up      - min uptime within probes
//      - down    - longest possible downtime
//      - unknown - max measured time excluding up and down calculated above
//      - nodata  - unoccupied time left after other three calculations
//
// NOTE: this episode summary is just a representation. It cannot be used for further uptime
// calculation of a group in a given timerange. Total summary varies depending on the chosen set of
// steps within the same time period.
func calculateTotalForStepRange(statuses map[string]map[string]map[int64]*EpisodeSummary, stepRange ranges.Range) {
	for group, probes := range statuses {
		totalStatusInfo := newEpisodeSummary(stepRange)

		var (
			uptimes, downtimes []time.Duration
			maxKnown, maxAvail time.Duration
		)

		for probe, infos := range probes {
			if probe == TotalProbeName {
				continue
			}
			if _, ok := infos[stepRange.From]; !ok {
				log.Errorf("Runner %s/%s has no timestamp %d!", group, probe, stepRange.From)
			}

			info := infos[stepRange.From]

			uptimes = append(uptimes, info.Up)
			downtimes = append(downtimes, info.Down)

			maxKnown = utime.Longest(maxKnown, info.Known())
			maxAvail = utime.Longest(maxAvail, info.Avail())
		}

		totalStatusInfo.Up = utime.Shortest(uptimes...)
		// down should not be less then known and not more then avail.
		totalStatusInfo.Down = utime.ClampToRange(utime.Longest(downtimes...), 0, maxAvail-totalStatusInfo.Up)
		totalStatusInfo.Unknown = maxAvail - totalStatusInfo.Up - totalStatusInfo.Down
		totalStatusInfo.NoData = stepRange.Diff() - maxAvail

		if _, ok := statuses[group][TotalProbeName]; !ok {
			statuses[group][TotalProbeName] = map[int64]*EpisodeSummary{}
		}

		statuses[group][TotalProbeName][stepRange.From] = totalStatusInfo
	}
}

// MuteDuration returns the count of seconds between 'from' and 'to'
// that are affected by this incident for particular 'group'.
func calcMuteDuration(inc check.DowntimeIncident, rng ranges.Range, group string) time.Duration {
	// Not in range
	if inc.Start >= rng.To || inc.End < rng.From {
		return 0
	}

	isAffected := false
	for _, affectedGroup := range inc.Affected {
		if group == affectedGroup {
			isAffected = true
			break
		}
	}
	if !isAffected {
		return 0
	}

	// Calculate mute duration for range [from; to]
	var (
		start = util.Max(inc.Start, rng.From)
		end   = util.Min(inc.End, rng.To)
	)

	return time.Duration(end-start) * time.Second
}

// updateMute applies muting to a EpisodeSummary based on intervals described by incidents.
func updateMute(statuses map[string]map[string]map[int64]*EpisodeSummary, incidents []check.DowntimeIncident, stepRanges []ranges.Range) {
	for group := range statuses {
		for _, stepRange := range stepRanges {
			var (
				step             = stepRange.Diff()
				muted            time.Duration
				relatedDowntimes []check.DowntimeIncident
			)

			// calculate maximum known mute duration
			for _, incident := range incidents {
				m := calcMuteDuration(incident, stepRange, group)
				if m == 0 {
					continue
				}
				muted = utime.Longest(muted, m)
				relatedDowntimes = append(relatedDowntimes, incident)
			}

			// Apply `muted` to all probes in group.
			for probeName := range statuses[group] {
				status := statuses[group][probeName][stepRange.From]

				status.Downtimes = relatedDowntimes

				// Mute Unknown first
				if muted <= status.Unknown {
					status.Unknown -= muted
					status.Muted = muted
					continue
				}

				// Mute Down
				if muted <= status.Unknown+status.Down {
					status.Down -= muted - status.Unknown
					status.Unknown = 0
					status.Muted = muted
					continue
				}

				// Do not mute Up seconds and make sure that seconds sum is not exceeded step duration
				if status.NoData == 0 {
					status.Unknown = 0
					status.Down = 0
					if muted+status.Up > step {
						status.Muted = step - status.Up
					} else {
						status.Muted = muted
					}
					continue
				}

				// Mute Nodata if interval in incident is more than sum of known seconds.
				if status.NoData > 0 {
					measured := status.Unknown + status.Down + status.Up
					if muted-measured > 0 {
						// Do not mute 'Up' seconds
						status.Muted = muted - status.Up
						status.Unknown = 0
						status.Down = 0
						// decrease no data
						status.NoData -= muted - measured
						if status.NoData < 0 {
							// This should not happen
							status.NoData = 0
						}
					}
				}
			}
		}
	}
}

// calculateTotalForPeriod calculates the total for a probe for the whole time range. It webui, it
// is rendered in the right-most 'Total' column for probes. It is the sum of all stats in the row.
func calculateTotalForPeriod(statuses map[string]map[string]map[int64]*EpisodeSummary, ranges []ranges.Range) {
	start := ranges[0].From
	end := ranges[len(ranges)-1].To
	for group := range statuses {
		for probe := range statuses[group] {
			totalStatus := &EpisodeSummary{
				TimeSlot:  -1, // -1 indicates it is a total
				StartDate: time.Unix(start, 0).Format(time.RFC3339),
				EndDate:   time.Unix(end, 0).Format(time.RFC3339),
			}

			for _, info := range statuses[group][probe] {
				totalStatus.add(info)
			}

			statuses[group][probe][-1] = totalStatus
		}
	}
}

// Create empty statuses for each probe.
// Group -> Probe -> Slot -> *EpisodeSummary
func newSummaryTable(episodes []check.Episode, stepRanges []ranges.Range, ref check.ProbeRef) map[string]map[string]map[int64]*EpisodeSummary {
	statuses := map[string]map[string]map[int64]*EpisodeSummary{}

	for _, episode := range episodes {
		group := episode.ProbeRef.Group
		probe := episode.ProbeRef.Probe

		_, ok := statuses[group]
		if !ok {
			statuses[group] = map[string]map[int64]*EpisodeSummary{}
		}

		_, ok = statuses[group][probe]
		if !ok {
			statuses[group][probe] = map[int64]*EpisodeSummary{}
		}

		for _, stepRange := range stepRanges {
			statuses[group][probe][stepRange.From] = newEpisodeSummary(stepRange)
		}
	}

	// Create empty statuses for groupName and probeName if there are no episodes and probeName is __total__.
	if ref.Probe == TotalProbeName {
		group := ref.Group

		if _, ok := statuses[group]; !ok {
			statuses[group] = map[string]map[int64]*EpisodeSummary{}
		}

		if _, ok := statuses[group][TotalProbeName]; !ok {
			statuses[group][TotalProbeName] = map[int64]*EpisodeSummary{}
		}

		for _, stepRange := range stepRanges {
			statuses[group][TotalProbeName][stepRange.From] = newEpisodeSummary(stepRange)
		}
	}

	return statuses
}

// transformTimestampedMapsToSortedArrays transforms each map timestamp -> EpisodeSummary into sorted array.
// TODO can be splited into SelectTotal|Probes and TransformToSortedArrays
func transformTimestampedMapsToSortedArrays(statuses map[string]map[string]map[int64]*EpisodeSummary, ref check.ProbeRef) map[string]map[string][]EpisodeSummary {
	// Transform maps "step->EpisodeSummary" in statuses to sorted arrays in StatusResponse
	res := map[string]map[string][]EpisodeSummary{}
	for group, probes := range statuses {
		if _, ok := res[group]; !ok {
			res[group] = map[string][]EpisodeSummary{}
		}
		if ref.Probe == TotalProbeName {
			res[group][TotalProbeName] = make([]EpisodeSummary, 0)
			for _, info := range statuses[group][TotalProbeName] {
				res[group][TotalProbeName] = append(res[group][TotalProbeName], *info)
			}
			sort.Sort(ByTimeSlot(res[group][TotalProbeName]))
		} else {
			for probe, infos := range probes {
				if probe == TotalProbeName {
					continue
				}
				if _, ok := res[group][probe]; !ok {
					res[group][probe] = make([]EpisodeSummary, 0)
				}
				for _, info := range infos {
					res[group][probe] = append(res[group][probe], *info)
				}
				sort.Sort(ByTimeSlot(res[group][probe]))
			}
		}
	}
	return res
}

func filterDisabledProbesFromEpisodes(episodes []check.Episode) []check.Episode {
	res := make([]check.Episode, 0)

	for _, episode := range episodes {
		if check.IsProbeEnabled(episode.ProbeRef.Id()) {
			res = append(res, episode)
		}
	}

	return res
}
