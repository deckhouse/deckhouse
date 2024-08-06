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
	Status PublicStatus  `json:"status"`
	Rows   []GroupStatus `json:"rows"`
}

type GroupStatus struct {
	Group  string              `json:"group"`
	Status PublicStatus        `json:"status"`
	Probes []ProbeAvailability `json:"probes"`
}

type ProbeAvailability struct {
	// Probe is the name of the probe
	Probe string `json:"probe"`

	// Availability is the ratio represented as a fraction of 1, i.e. it is from 0 to 1 and it must
	// neve be negative
	Availability float64 `json:"availability"`

	// Status is the high-level interpretation of the probe result
	Status PublicStatus `json:"status"`
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

	wantsPeek := r.URL.Query().Get("peek") == "1"
	statuses, err := h.getGroupStatusList(wantsPeek)
	if err != nil {
		log.Errorf("Cannot get status summary (peek=%v): %v", wantsPeek, err)
		// Skipping the error because the JSON structure is defined in advance.
		out, _ := json.Marshal(&PublicStatusResponse{
			Rows:   []GroupStatus{},
			Status: StatusNoData,
		})
		w.WriteHeader(http.StatusOK)
		w.Write(out)
		return
	}

	// Skipping the error because the JSON structure is defined in advance.
	totalStatus := calculateTotalStatus(statuses)
	out, _ := json.Marshal(&PublicStatusResponse{
		Rows:   statuses,
		Status: totalStatus,
	})
	w.Write(out)
}

// getGroupStatusList returns total statuses for each group for the current partial 5m timeslot plus
// previous full 5m timeslot. If peek is true, then the current partial 5m timeslot is not included.
func (h *PublicStatusHandler) getGroupStatusList(peek bool) ([]GroupStatus, error) {
	daoCtx := h.DbCtx.Start()
	defer daoCtx.Stop()

	now := time.Now()

	if peek {
		// Observe only last fulfilled 30 seconds for the speed of availability calculation
		return h.calcStatuses(
			new30SecondsStepRange(now),
			dao.NewEpisodeDao30s(daoCtx),
			func(ref check.ProbeRef, statuses summaryListByProbeByGroup) ([]entity.EpisodeSummary, error) {
				slotSize := 30 * time.Second
				return pickGroupProbeSummaryByLastCompleteEpisode(ref, statuses, slotSize)
			},
		)
	}

	// Observe 10 minutes of fulfilled data for accuracy
	return h.calcStatuses(
		new15MinutesStepRange(now),
		dao.NewEpisodeDao5m(daoCtx),
		pickGroupProbeSummary,
	)
}

type summaryPicker func(ref check.ProbeRef, statuses summaryListByProbeByGroup) ([]entity.EpisodeSummary, error)

func (h *PublicStatusHandler) calcStatuses(
	rng ranges.StepRange,
	lister entity.RangeEpisodeLister,
	pickSummary summaryPicker,
) ([]GroupStatus, error) {
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

		groupSummary, err := h.episodeSummaryList(lister, filter, pickSummary)
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

			probeSummary, err := h.episodeSummaryList(lister, filter, pickSummary)
			if err != nil {
				return nil, fmt.Errorf("getting summary for probe %s/%s: %v", group, groupRef.Probe, err)
			}

			av := calculateAvailability(probeSummary)
			if av < 0 {
				continue
			}
			probeAvails = append(probeAvails, ProbeAvailability{
				Probe:        probeRef.Probe,
				Availability: av,
				Status:       calculateStatus(probeSummary),
			})
		}

		gs := GroupStatus{
			Group:  group,
			Status: calculateStatus(groupSummary),
			Probes: probeAvails,
		}
		groupStatuses = append(groupStatuses, gs)
	}

	return groupStatuses, nil
}

func (h *PublicStatusHandler) episodeSummaryList(
	lister entity.RangeEpisodeLister,
	filter *statusFilter,
	pickSummary summaryPicker,
) ([]entity.EpisodeSummary, error) {
	resp, err := getStatusSummary(lister, h.DowntimeMonitor, filter)
	if err != nil {
		return nil, fmt.Errorf("fetching summary: %w", err)
	}

	gpSummary, err := pickSummary(filter.probeRef, resp.Statuses)
	if err != nil {
		return nil, fmt.Errorf("flattening summary: %w", err)
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

func new15MinutesStepRange(now time.Time) ranges.StepRange {
	step := 5 * time.Minute
	slotStart := now.Truncate(step)
	from := slotStart.Add(-2 * step)
	to := slotStart.Add(step)
	return ranges.New5MinStepRange(from.Unix(), to.Unix(), int64(step.Seconds()))
}

func new30SecondsStepRange(now time.Time) ranges.StepRange {
	step := 30 * time.Second
	to := now.Truncate(step)
	from := to.Add(-step)
	return ranges.New30SecStepRange(from.Unix(), to.Unix(), int64(step.Seconds()))
}

var ErrNoData = fmt.Errorf("no data")

// pickGroupProbeSummaryByLastCompleteEpisode returns the last fulfilled episode summary for the given probe.
func pickGroupProbeSummaryByLastCompleteEpisode(ref check.ProbeRef, statuses summaryListByProbeByGroup, slotSize time.Duration) ([]entity.EpisodeSummary, error) {
	summary, err := pickGroupProbeSummary(ref, statuses)
	if err != nil {
		return nil, fmt.Errorf("getting summary for slot size %s: %w", slotSize, err)
	}
	lastFulfilled, found := shrinkToFulfilled(summary, slotSize)
	if !found {
		return nil, fmt.Errorf("shrinking data to last complete episode: %w", ErrNoData)
	}
	return []entity.EpisodeSummary{lastFulfilled}, nil
}

// pickGroupProbeSummary
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

	// Summary column in the end is implied, ignore summary column in the end
	n := len(episodeSummaries) - 1
	if n == 1 {
		return episodeSummaries, nil
	}
	if n != 3 {
		return nil, fmt.Errorf("unexpected count %d!=3 for probe '%s/%s'", n, g, p)
	}

	return episodeSummaries[:3], nil
}

func shrinkToFulfilled(summary []entity.EpisodeSummary, slotSize time.Duration) (entity.EpisodeSummary, bool) {
	for i := len(summary) - 1; i >= 0; i-- {
		if summary[i].Complete() {
			return summary[i], true
		}
	}
	return entity.EpisodeSummary{NoData: slotSize}, false
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
	warn := false
	for _, info := range statuses {
		switch info.Status {
		case StatusDegraded:
			warn = true
		case StatusOutage:
			return StatusOutage
		}
	}
	if warn {
		return StatusDegraded
	}
	return StatusOperational
}

func jsonError(msg string) string {
	return fmt.Sprintf(`{"error": %q}`, msg)
}
