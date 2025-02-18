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

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/check"
	dbcontext "d8.io/upmeter/pkg/db/context"
	"d8.io/upmeter/pkg/db/dao"
	"d8.io/upmeter/pkg/monitor/downtime"
	"d8.io/upmeter/pkg/registry"
	"d8.io/upmeter/pkg/server/entity"
	"d8.io/upmeter/pkg/server/ranges"
)

type PublicStatusResponse struct {
	Status string        `json:"status"`
	Rows   []GroupStatus `json:"rows"`
}

type PublicStatus int

const (
	StatusNoData PublicStatus = iota
	StatusOperational
	StatusOutage
	StatusDegraded
	StatusError // unexpected to have this status
)

func (s PublicStatus) Compare(s1 PublicStatus) PublicStatus {
	if s == StatusError || s1 == StatusError {
		return StatusError
	}
	if s == s1 {
		return s
	}
	if s == StatusNoData {
		return s1
	}
	if s1 == StatusNoData {
		return s
	}

	return StatusDegraded
}

func (s PublicStatus) String() string {
	switch s {
	case StatusOperational:
		return "Operational"
	case StatusDegraded:
		return "Degraded"
	case StatusOutage:
		return "Outage"
	case StatusNoData:
		return "No Data"
	default:
		return "Error"
	}
}

type GroupStatus struct {
	status PublicStatus
	Status string              `json:"status"`
	Group  string              `json:"group"`
	Probes []ProbeAvailability `json:"probes"`
}

type ProbeAvailability struct {
	// Probe is the name of the probe
	Probe string `json:"probe"`

	// Availability is the ratio represented as a fraction of 1, i.e. it is from 0 to 1 and it must
	// neve be negative
	Availability float64 `json:"availability"`

	// Status is the high-level interpretation of the probe result
	status PublicStatus
	Status string `json:"status"`
}

type PublicStatusHandler struct {
	DbCtx           *dbcontext.DbContext
	DowntimeMonitor *downtime.Monitor
	ProbeLister     registry.ProbeLister
}

func (h *PublicStatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Infoln("PublicStatus", r.RemoteAddr, r.RequestURI)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		message := fmt.Sprintf("%s not allowed, use GET\n", r.Method)
		fmt.Fprint(w, jsonError(message))
		return
	}

	peek := r.URL.Query().Get("peek") == "1"
	statuses, err := h.getGroupStatusList(peek)
	if err != nil {
		log.Errorf("Cannot get status summary (peek=%v): %v", peek, err)
		// Skipping the error because the JSON structure is defined in advance.
		out, _ := json.Marshal(&PublicStatusResponse{
			Rows:   []GroupStatus{},
			Status: StatusNoData.String(),
		})
		w.WriteHeader(http.StatusOK)
		w.Write(out)
		return
	}

	// Skipping the error because the JSON structure is defined in advance.
	totalStatus := calculateTotalStatus(statuses)
	out, _ := json.Marshal(&PublicStatusResponse{
		Rows:   statuses,
		Status: totalStatus.String(),
	})
	w.Write(out)
}

// getGroupStatusList returns the most recent complete episode. If peek is true, then 30s episodes
// are consideres instead of 5m episodes.
func (h *PublicStatusHandler) getGroupStatusList(peek bool) ([]GroupStatus, error) {
	daoCtx := h.DbCtx.Start()
	defer daoCtx.Stop()

	now := time.Now()

	var (
		lister entity.RangeEpisodeLister
		step   time.Duration
	)

	if peek {
		// get one of most recent complete 30s episodes
		step = 30 * time.Second
		lister = dao.NewEpisodeDao30s(daoCtx)
	} else {
		// get one of most recent complete 5m episodes
		step = 5 * time.Minute
		lister = dao.NewEpisodeDao5m(daoCtx)
	}

	stepRange := ranges.New(now.Add(-2*step), now, step, false)
	return h.calcStatuses(stepRange, lister)
}

func (h *PublicStatusHandler) calcStatuses(rng ranges.StepRange, lister entity.RangeEpisodeLister) ([]GroupStatus, error) {
	muteTypes := []string{
		"Maintenance",
		"InfrastructureMaintenance",
		"InfrastructureAccident",
	}

	groups := h.ProbeLister.Groups()
	groupStatuses := make([]GroupStatus, 0, len(groups))
	for _, group := range groups {
		// The group overall status
		groupRef := check.ProbeRef{
			Group: group,
			Probe: dao.GroupAggregation,
		}

		filter := &statusFilter{
			stepRange:         rng,
			probeRef:          groupRef,
			muteDowntimeTypes: muteTypes,
		}

		groupSummaryList, err := h.getProbeSummaryList(lister, filter)
		if err != nil {
			return nil, fmt.Errorf("getting summary for group %s: %v", group, err)
		}

		// Uptime per probe
		probeAvails := make([]ProbeAvailability, 0)
		for _, probeRef := range h.ProbeLister.Probes() {
			if probeRef.Group != group {
				// not so many probes in the list, not a big deal to spend n^2 iterations
				continue
			}

			filter := &statusFilter{
				stepRange:         rng,
				probeRef:          probeRef,
				muteDowntimeTypes: muteTypes,
			}

			probeSummaryList, err := h.getProbeSummaryList(lister, filter)
			if err != nil {
				return nil, fmt.Errorf("getting summary for probe %s/%s: %v", group, groupRef.Probe, err)
			}

			av := calculateAvailability(probeSummaryList)
			if av < 0 {
				continue
			}
			status := calculateStatus(probeSummaryList)
			probeAvails = append(probeAvails, ProbeAvailability{
				Probe:        probeRef.Probe,
				Availability: av,
				status:       status,
				Status:       status.String(),
			})
		}

		groupStatus := calculateStatus(groupSummaryList)
		gs := GroupStatus{
			status: groupStatus,
			Status: groupStatus.String(),
			Group:  group,
			Probes: probeAvails,
		}
		groupStatuses = append(groupStatuses, gs)
	}

	return groupStatuses, nil
}

func (h *PublicStatusHandler) getProbeSummaryList(lister entity.RangeEpisodeLister, filter *statusFilter) ([]entity.EpisodeSummary, error) {
	resp, err := getStatusSummary(lister, h.DowntimeMonitor, filter, false /* = without total */)
	if err != nil {
		return nil, fmt.Errorf("fetching summary: %w", err)
	}

	gpSummary, err := pickGroupProbeSummary(filter.probeRef, resp.Statuses)
	if err != nil {
		return nil, fmt.Errorf("picking summary: %w", err)
	}
	return gpSummary, nil
}

// Negative availability means we have no valid data
func calculateAvailability(summary []entity.EpisodeSummary) float64 {
	var worked, measured float64
	for _, s := range summary {
		worked += float64(s.Up + s.Unknown)
		measured += float64(s.Up + s.Unknown + s.Down)
	}
	if measured == 0 {
		return -1
	}
	return worked / measured
}

var ErrNoData = fmt.Errorf("no data")

// Returns the list of summaries for all probes in the group including the total column.
func pickGroupProbeSummary(ref check.ProbeRef, statuses summaryListByProbeByGroup) ([]entity.EpisodeSummary, error) {
	g := ref.Group
	p := ref.Probe

	if _, ok := statuses[g]; !ok {
		return nil, fmt.Errorf("%w for group '%s'", ErrNoData, g)
	}
	if _, ok := statuses[g][p]; !ok {
		return nil, fmt.Errorf("%s for probe '%s/%s'", ErrNoData, g, p)
	}

	episodeSummaries := statuses[g][p]
	if len(episodeSummaries) == 0 {
		return nil, fmt.Errorf("%w (empty row) for probe '%s/%s'", ErrNoData, g, p)
	}

	return episodeSummaries, nil
}

// calculateStatus returns the status for a group.
//
// Input array should have 3 elements, but might have only twoof them.
//
// The status returned is as follows:
//   - 'Operational' is when we observe only uptime
//   - 'Outage' is when we observe only downtime
//   - 'Degraded' is when we observe mixed uptime and downtime
//   - 'No Data' is when we have no data
func calculateStatus(sums []entity.EpisodeSummary) PublicStatus {
	// initial case
	if len(sums) == 0 {
		return StatusNoData
	}

	status := episodeStatus(sums[0])

	// peek case
	if len(sums) == 1 {
		return status
	}

	// muted episodes are not supported yet
	for _, s := range sums {
		status = status.Compare(episodeStatus(s))
	}
	return status
}

func episodeStatus(episode entity.EpisodeSummary) PublicStatus {
	hasUp := episode.Up > 0 || episode.Unknown > 0
	hasDown := episode.Down > 0

	switch {
	case hasDown && !hasUp:
		return StatusOutage
	case !hasDown && hasUp:
		return StatusOperational
	case hasDown && hasUp:
		return StatusDegraded
	default:
		return StatusNoData
	}
}

// calculateTotalStatus returns total cluster status.
func calculateTotalStatus(statuses []GroupStatus) PublicStatus {
	if len(statuses) == 0 {
		return StatusNoData
	}
	total := statuses[0].status
	for _, next := range statuses[1:] {
		total = total.Compare(next.status)
	}
	return total
}

func jsonError(msg string) string {
	return fmt.Sprintf(`{"error": %q}`, msg)
}
