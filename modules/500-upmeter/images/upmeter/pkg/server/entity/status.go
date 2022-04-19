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
	"sort"
	"time"

	"d8.io/upmeter/pkg/check"
	dbcontext "d8.io/upmeter/pkg/db/context"
	"d8.io/upmeter/pkg/db/dao"
	"d8.io/upmeter/pkg/server/ranges"
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

func Statuses(dbctx *dbcontext.DbContext, ref check.ProbeRef, rng ranges.StepRange, incidents []check.DowntimeIncident) (map[string]map[string][]EpisodeSummary, error) {
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

It is expected that there is only one Episode for probe and Start.

Returned structure is map[group][probe][dataByTime]

Example output:
aGroup:
  aProbe:
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
	// Combine multiple episodes into one for the same probe and timeslot. Basically, we deduce
	// one single episode from possible alternatives.
	episodes = combineEpisodesByTimeslot(episodes)

	// Create table with empty statuses for each probe
	//     Group  ->  Probe  ->  Slot -> *EpisodeSummary
	// map[string]map[string]map[int64]*EpisodeSummary
	statuses := newSummaryTable(episodes, stepRanges)

	// Sum up episodes for each probe by Start within each step range.
	// TODO various optimizations can be applied here.
	for _, stepRange := range stepRanges {
		for _, episode := range episodes {
			if !episode.IsInRange(stepRange.From, stepRange.To) {
				continue
			}

			statuses[episode.ProbeRef.Group][episode.ProbeRef.Probe][stepRange.From].addEpisode(episode)
		}
	}

	updateMute(statuses, incidents, stepRanges)

	// Calculate group-level summaries including __total__
	calculateTotalForPeriod(statuses, stepRanges)

	return transformTimestampedMapsToSortedArrays(statuses, ref)
}

// Each group/probe should have only 1 Episode per Start.
func combineEpisodesByTimeslot(episodes []check.Episode) []check.Episode {
	// It could have been a more shallow map map[string][]check.Episode, the key being
	//           fmt.Sprintf("%s-%d", probeId, start)
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
		start = maxInt64(inc.Start, rng.From)
		end   = minInt64(inc.End, rng.To)
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
				muted = longest(muted, m)
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
func newSummaryTable(episodes []check.Episode, stepRanges []ranges.Range) map[string]map[string]map[int64]*EpisodeSummary {
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

	return statuses
}

// transformTimestampedMapsToSortedArrays transforms each map[timestamp]EpisodeSummary into sorted array.
// TODO can be splited into SelectTotal|Probes and TransformToSortedArrays
func transformTimestampedMapsToSortedArrays(statuses map[string]map[string]map[int64]*EpisodeSummary, ref check.ProbeRef) map[string]map[string][]EpisodeSummary {
	// Transform maps "step->EpisodeSummary" in statuses to sorted arrays in StatusResponse
	res := map[string]map[string][]EpisodeSummary{}
	for group, probes := range statuses {
		if _, ok := res[group]; !ok {
			res[group] = map[string][]EpisodeSummary{}
		}
		if ref.Probe == dao.GroupAggregation {
			// Only group stats were requested
			res[group][dao.GroupAggregation] = make([]EpisodeSummary, 0)
			for _, info := range statuses[group][dao.GroupAggregation] {
				res[group][dao.GroupAggregation] = append(res[group][dao.GroupAggregation], *info)
			}
			sort.Sort(ByTimeSlot(res[group][dao.GroupAggregation]))
		} else {
			// All probes in detail were requested
			for probe, infos := range probes {
				if probe == dao.GroupAggregation {
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

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func longest(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}
