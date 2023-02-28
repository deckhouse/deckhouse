/*
Copyright 2023 Flant JSC

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

package api

import (
	"fmt"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/monitor/downtime"
	"d8.io/upmeter/pkg/server/ranges"
)

func fetchIncidents(monitor *downtime.Monitor, muteDowntimeTypes []string, group string, rng ranges.StepRange) ([]check.DowntimeIncident, error) {
	allIncidents, err := monitor.List()
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
