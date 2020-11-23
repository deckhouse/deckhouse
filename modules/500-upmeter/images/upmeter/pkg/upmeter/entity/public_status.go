package entity

import (
	"errors"
	log "github.com/sirupsen/logrus"
	"sort"
	"time"
	"upmeter/pkg/crd"
	"upmeter/pkg/upmeter/db/dao"
)

type GroupStatusInfo struct {
	Group  string `json:"group"`
	Status string `json:"status"`
}

const okStatus = "Operational"
const warnStatus = "Degraded"
const failStatus = "Outage"

// CurrentStatusForGroups returns total statuses for each group
// for the current partial 5m timeslot plus previous full 5m timeslot.
func CurrentStatusForGroups(monitor *crd.Monitor) ([]GroupStatusInfo, string, error) {
	/*
		select group, probe from downtime
	*/
	probeRefs, err := dao.Downtime5m.ListGroupProbe()
	if err != nil {
		log.Errorf("List groups: %v", err)
		return nil, "", errors.New("")
	}

	probeRefs = FilterDisabledProbesFromGroupProbeList(probeRefs)

	groupsMap := map[string]struct{}{}
	for _, probeRef := range probeRefs {
		groupsMap[probeRef.Group] = struct{}{}
	}

	groups := []string{}
	for group := range groupsMap {
		groups = append(groups, group)
	}

	sort.Strings(groups)

	muteDowntimeTypes := []string{
		"Maintenance",
		"InfrastructureMaintenance",
		"InfrastructureAccident",
	}

	// Request 3 latest timeslots.
	nowSeconds := time.Now().Unix()
	var step int64 = 300
	from := (nowSeconds/step)*step - step*2
	to := from + 3*step

	stepRanges := CalculateAdjustedStepRanges(from, to, step)

	log.Infof("Request public status from=%d to=%d at %d", stepRanges.From, stepRanges.To, nowSeconds)

	var currentStatuses = make([]GroupStatusInfo, 0)

	for _, groupName := range groups {
		episodes, err := dao.Downtime5m.ListEpisodesByRange(from, to, groupName, totalProbeName)
		if err != nil {
			log.Errorf("List episodes: %+v", err)
			return nil, "", errors.New("")
		}

		incidents := monitor.FilterDowntimeIncidents(from, to, groupName, muteDowntimeTypes)

		statuses := CalculateStatuses(episodes, incidents, stepRanges.Ranges, groupName, totalProbeName)

		// Asserts
		if _, ok := statuses[groupName]; !ok {
			log.Errorf("No status for group '%s'", groupName)
			continue
		}
		if _, ok := statuses[groupName][totalProbeName]; !ok {
			log.Errorf("No status for group '%s' probe '%s'", groupName, totalProbeName)
			continue
		}
		if len(statuses[groupName][totalProbeName]) != 3 {
			log.Errorf("Bad results count %d for group '%s' probe '%s'", len(statuses[groupName][totalProbeName]), groupName, totalProbeName)
			continue
		}

		info := statuses[groupName][totalProbeName]

		currentStatuses = append(currentStatuses, GroupStatusInfo{
			Group:  groupName,
			Status: CalculateCurrentStatus(info),
		})
	}

	return currentStatuses, CalculateTotalStatus(currentStatuses), nil
}

// CalculateCurrentStatus returns ok/warn/fail status for a group.
//
// Input array should have 3
func CalculateCurrentStatus(info []StatusInfo) string {
	var prev, current StatusInfo
	if len(info) == 2 || info[2].NoData == 300 {
		prev = info[0]
		current = info[1]
	} else {
		prev = info[1]
		current = info[2]
	}

	// Ignore empty StatusInfo, i.e. when NoData equals step
	if current.Down == 0 && prev.Down == 0 &&
		(current.Up > 0 || (prev.Up > 0 && current.Up == 0 && current.NoData == 300)) {
		return okStatus
	}

	if current.Up == 0 && current.Muted == 0 &&
		prev.Up == 0 && prev.Muted == 0 &&
		(current.Down > 0 || (prev.Down > 0 && current.Down == 0 && current.NoData == 300)) {
		return failStatus
	}

	return warnStatus
}

// CalculateTotalStatus returns total ok/warn/fail status.
func CalculateTotalStatus(statuses []GroupStatusInfo) string {
	warn := false
	for _, info := range statuses {
		switch info.Status {
		case warnStatus:
			warn = true
		case failStatus:
			return failStatus
		}
	}
	if warn {
		return warnStatus
	}
	return okStatus
}
