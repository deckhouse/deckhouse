package entity

import (
	"sort"
	"time"

	log "github.com/sirupsen/logrus"

	"upmeter/pkg/probe/types"
	"upmeter/pkg/util"
)

type StatusInfo struct {
	TimeSlot  int64                    `json:"ts"`
	StartDate string                   `json:"start"`
	EndDate   string                   `json:"end"`
	Up        int64                    `json:"up"`
	Down      int64                    `json:"down"`
	Unknown   int64                    `json:"unknown"`
	Muted     int64                    `json:"muted"`
	NoData    int64                    `json:"nodata"`
	Downtimes []types.DowntimeIncident `json:"downtimes"`
}

func NewEmptyStatusInfo(stepRange []int64) *StatusInfo {
	return &StatusInfo{
		TimeSlot:  stepRange[0],
		StartDate: time.Unix(stepRange[0], 0).Format(time.RFC3339),
		EndDate:   time.Unix(stepRange[1], 0).Format(time.RFC3339),
		NoData:    stepRange[1] - stepRange[0],
	}
}

func (s *StatusInfo) AddEpisode(episode types.DowntimeEpisode) {
	s.Up += episode.SuccessSeconds
	s.Down += episode.FailSeconds
	s.Unknown += episode.Unknown
	s.NoData -= episode.SuccessSeconds + episode.FailSeconds + episode.Unknown
}

func (s *StatusInfo) Add(info *StatusInfo) {
	s.Up += info.Up
	s.Down += info.Down
	s.Unknown += info.Unknown
	s.NoData += info.NoData
}

func (s *StatusInfo) SetSeconds(up int64, down int64, unknown int64, nodata int64) {
	s.Up = up
	s.Down = down
	s.Unknown = unknown
	s.NoData = nodata
}

func (s *StatusInfo) Known() int64 {
	return s.Up + s.Down
}

func (s *StatusInfo) Avail() int64 {
	return s.Up + s.Down + s.Unknown
}

// ByTimeSlot implements sort.Interface based on the TimeSlot field.
type ByTimeSlot []StatusInfo

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

const totalProbeName = "__total__"

/**
CalculateStatuses returns arrays of StatusInfo objects for each group and probe.
Each StatusInfo object is combined from DowntimeEpisodes for a step range.
If grouping is "group", then all probes in group are summed up and
probe name is "__total__".

Method considers that there is only one DowntimeEpisode for probe and TimeSlot.

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
func CalculateStatuses(
	episodes []types.DowntimeEpisode,
	incidents []types.DowntimeIncident,
	stepRanges [][]int64,
	groupName string,
	probeName string,
) map[string]map[string][]StatusInfo {
	episodes = FilterDisabledProbesFromEpisodes(episodes)

	// Combine multiple episodes for the same probe and timeslot.
	episodes = CombineEpisodesByTimeslot(episodes)

	// Create table with empty statuses for each probe
	// map[string]map[string]map[int64]*StatusInfo
	statuses := CreateEmptyStatusesTable(episodes, stepRanges, groupName, probeName)

	// Sum up episodes for each probe by TimeSlot within each step range.
	// TODO various optimizations can be applied here.
	for _, stepRange := range stepRanges {
		for _, episode := range episodes {
			if !episode.IsInRange(stepRange[0], stepRange[1]) {
				continue
			}

			statuses[episode.ProbeRef.Group][episode.ProbeRef.Probe][stepRange[0]].AddEpisode(episode)
		}
		CalculateTotalForStepRange(statuses, stepRange)
	}

	UpdateMute(statuses, incidents, stepRanges)

	CalculateTotalForPeriod(statuses, stepRanges)

	return TransformTimestampedMapsToSortedArrays(statuses, groupName, probeName)
}

// Each group/probe should have only 1 DowntimeEpisode per Timeslot.
func CombineEpisodesByTimeslot(episodes []types.DowntimeEpisode) []types.DowntimeEpisode {
	var idx = make(map[string]map[int64][]int)
	for i, episode := range episodes {
		probeId := episode.ProbeRef.ProbeId()
		if _, ok := idx[probeId]; !ok {
			idx[probeId] = make(map[int64][]int)
		}
		if _, ok := idx[probeId][episode.TimeSlot]; !ok {
			idx[probeId][episode.TimeSlot] = make([]int, 0)
		}
		idx[probeId][episode.TimeSlot] = append(idx[probeId][episode.TimeSlot], i)
	}

	var newEpisodes = make([]types.DowntimeEpisode, 0)
	for _, timeslots := range idx {
		for _, indicies := range timeslots {
			ep := episodes[indicies[0]]
			for _, index := range indicies {
				ep = ep.CombineSeconds(episodes[index], 300)
			}
			newEpisodes = append(newEpisodes, ep)
		}
	}

	return newEpisodes
}

func CalculateTotalForStepRange(statuses map[string]map[string]map[int64]*StatusInfo, stepRange []int64) {
	// Combine group's probes into one "__total__" probe
	for group, probes := range statuses {
		totalStatusInfo := NewEmptyStatusInfo(stepRange)

		// Total Up is a minimum known Up
		// Total Down is a minimum knownSeconds - total Up
		// Total Unknown is a step seconds - minimum known seconds
		upSeconds := []int64{}
		downSeconds := []int64{}
		unknownSeconds := []int64{}
		maxKnown := int64(0)
		maxAvail := int64(0)
		for probe, infos := range probes {
			if probe == totalProbeName {
				continue
			}
			if _, ok := infos[stepRange[0]]; !ok {
				log.Errorf("Probe %s/%s has no timestamp %d!", group, probe, stepRange[0])
			}

			info := infos[stepRange[0]]
			upSeconds = append(upSeconds, info.Up)
			downSeconds = append(downSeconds, info.Down)
			unknownSeconds = append(unknownSeconds, info.Unknown)
			maxKnown = util.Max(maxKnown, info.Known())
			maxAvail = util.Max(maxAvail, info.Avail())
		}

		totalStatusInfo.Up = util.Min(upSeconds...)
		// down should not be less then known and not more then avail.
		totalStatusInfo.Down = util.ClampToRange(util.Max(downSeconds...), 0, maxAvail-totalStatusInfo.Up)
		totalStatusInfo.Unknown = maxAvail - totalStatusInfo.Up - totalStatusInfo.Down
		totalStatusInfo.NoData = (stepRange[1] - stepRange[0]) - maxAvail

		if _, ok := statuses[group][totalProbeName]; !ok {
			statuses[group][totalProbeName] = map[int64]*StatusInfo{}
		}

		statuses[group][totalProbeName][stepRange[0]] = totalStatusInfo
	}
}

// UpdateMute applies muting to a StatusInfo based on intervals described by incidents.
func UpdateMute(statuses map[string]map[string]map[int64]*StatusInfo, incidents []types.DowntimeIncident, stepRanges [][]int64) {
	for groupName := range statuses {
		for _, stepRange := range stepRanges {
			var stepDuration int64 = stepRange[1] - stepRange[0]
			var muteDuration int64 = 0
			var relatedDowntimes = make([]types.DowntimeIncident, 0)

			// calculate maximum known mute duration
			for _, incident := range incidents {
				m := incident.MuteDuration(stepRange[0], stepRange[1], groupName)
				if m == 0 {
					continue
				}
				muteDuration = util.Max(muteDuration, m)
				relatedDowntimes = append(relatedDowntimes, incident)
			}

			// Apply muteDuration to all probes in group.
			for probeName := range statuses[groupName] {
				status := statuses[groupName][probeName][stepRange[0]]

				status.Downtimes = relatedDowntimes

				// Mute Unknown first
				if muteDuration <= status.Unknown {
					status.Unknown -= muteDuration
					status.Muted = muteDuration
					continue
				}

				// Mute Down
				if muteDuration <= status.Unknown+status.Down {
					status.Down = status.Down - (muteDuration - status.Unknown)
					status.Unknown = 0
					status.Muted = muteDuration
					continue
				}

				// Do not mute Up seconds and make sure that seconds sum is not exceeded step duration
				if status.NoData == 0 {
					status.Unknown = 0
					status.Down = 0
					if muteDuration+status.Up > stepDuration {
						status.Muted = stepDuration - status.Up
					} else {
						status.Muted = muteDuration
					}
					continue
				}

				// Mute Nodata if interval in incident is more than sum of known seconds.
				if status.NoData > 0 {
					knownSeconds := status.Unknown + status.Down + status.Up
					if muteDuration-knownSeconds > 0 {
						// Do not mute 'Up' seconds
						status.Muted = muteDuration - status.Up
						status.Unknown = 0
						status.Down = 0
						// decrease no data
						status.NoData -= muteDuration - knownSeconds
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

func CalculateTotalForPeriod(statuses map[string]map[string]map[int64]*StatusInfo, stepRanges [][]int64) {
	start := stepRanges[0][0]
	end := stepRanges[len(stepRanges)-1][1]
	for groupName := range statuses {
		for probeName := range statuses[groupName] {
			totalStatus := &StatusInfo{
				TimeSlot:  -1, // -1 indicates it is a total
				StartDate: time.Unix(start, 0).Format(time.RFC3339),
				EndDate:   time.Unix(end, 0).Format(time.RFC3339),
			}

			for _, info := range statuses[groupName][probeName] {
				totalStatus.Add(info)
			}

			statuses[groupName][probeName][-1] = totalStatus
		}
	}
}

func CreateEmptyStatusesTable(episodes []types.DowntimeEpisode, stepRanges [][]int64, groupName string, probeName string) map[string]map[string]map[int64]*StatusInfo {
	// Create empty statuses for each probe.
	statuses := map[string]map[string]map[int64]*StatusInfo{}
	for _, episode := range episodes {
		group := episode.ProbeRef.Group
		probe := episode.ProbeRef.Probe

		_, ok := statuses[group]
		if !ok {
			statuses[group] = map[string]map[int64]*StatusInfo{}
		}

		_, ok = statuses[group][probe]
		if !ok {
			statuses[group][probe] = map[int64]*StatusInfo{}
		}

		for _, stepRange := range stepRanges {
			statuses[group][probe][stepRange[0]] = NewEmptyStatusInfo(stepRange)
		}
	}

	// Create empty statuses for groupName and probeName if there are no episodes and probeName is __total__.
	if probeName == totalProbeName {
		if _, ok := statuses[groupName]; !ok {
			statuses[groupName] = map[string]map[int64]*StatusInfo{}
		}
		if _, ok := statuses[groupName][probeName]; !ok {
			statuses[groupName][probeName] = map[int64]*StatusInfo{}
			for _, stepRange := range stepRanges {
				statuses[groupName][probeName][stepRange[0]] = NewEmptyStatusInfo(stepRange)
			}
		}
	}

	return statuses
}

// TransformTimestampedMapsToSortedArrays transforms each map timestamp -> StatusInfo into sorted array.
// TODO can be splited into SelectTotal|Probes and TransformToSortedArrays
func TransformTimestampedMapsToSortedArrays(statuses map[string]map[string]map[int64]*StatusInfo, groupName string, probeName string) map[string]map[string][]StatusInfo {
	// Transform maps "step->StatusInfo" in statuses to sorted arrays in StatusResponse
	res := map[string]map[string][]StatusInfo{}
	for group, probes := range statuses {
		if _, ok := res[group]; !ok {
			res[group] = map[string][]StatusInfo{}
		}
		if probeName == totalProbeName {
			res[group][totalProbeName] = make([]StatusInfo, 0)
			for _, info := range statuses[group][totalProbeName] {
				res[group][totalProbeName] = append(res[group][totalProbeName], *info)
			}
			sort.Sort(ByTimeSlot(res[group][totalProbeName]))
		} else {
			for probe, infos := range probes {
				if probe == totalProbeName {
					continue
				}
				if _, ok := res[group][probe]; !ok {
					res[group][probe] = make([]StatusInfo, 0)
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

func FilterDisabledProbesFromEpisodes(episodes []types.DowntimeEpisode) []types.DowntimeEpisode {
	res := make([]types.DowntimeEpisode, 0)

	for _, episode := range episodes {
		if types.IsProbeEnabled(episode.ProbeRef.ProbeId()) {
			res = append(res, episode)
		}
	}

	return res
}
