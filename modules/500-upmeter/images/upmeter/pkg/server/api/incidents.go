package api

import (
	"fmt"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/crd"
	"d8.io/upmeter/pkg/server/ranges"
)

func fetchIncidents(monitor *crd.DowntimeMonitor, muteDowntimeTypes []string, group string, rng ranges.StepRange) ([]check.DowntimeIncident, error) {
	allIncidents, err := monitor.GetDowntimeIncidents()
	if err != nil {
		return nil, fmt.Errorf("cannot get incidents: %v", err)
	}

	incidents := filterIncidents(
		allIncidents,
		incidentInRange(rng.From, rng.To),
		incidentAffectsGroup(group),
		incidentMutedByTypes(muteDowntimeTypes),
	)

	return incidents, nil
}

type incidentFilter func(check.DowntimeIncident) bool

func filterIncidents(incidents []check.DowntimeIncident, cbs ...incidentFilter) []check.DowntimeIncident {
	matches := make([]check.DowntimeIncident, 0)
Outer:
	for _, incident := range incidents {
		for _, cb := range cbs {
			if !cb(incident) {
				continue Outer
			}
		}
		matches = append(matches, incident)
	}
	return matches
}

func incidentInRange(from, to int64) incidentFilter {
	return func(incident check.DowntimeIncident) bool {
		return incident.Start < to && incident.End > from
	}
}

func incidentAffectsGroup(group string) incidentFilter {
	return func(incident check.DowntimeIncident) bool {
		for _, affectedGroup := range incident.Affected {
			if group == affectedGroup {
				return true
			}
		}
		return false
	}
}

func incidentMutedByTypes(muteTypes []string) incidentFilter {
	return func(incident check.DowntimeIncident) bool {
		for _, mutedType := range muteTypes {
			if mutedType == incident.Type {
				return true
			}
		}
		return false
	}
}
