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
	"d8.io/upmeter/pkg/crd"
	dbcontext "d8.io/upmeter/pkg/db/context"
	"d8.io/upmeter/pkg/db/dao"
	"d8.io/upmeter/pkg/registry"
	"d8.io/upmeter/pkg/server/entity"
	"d8.io/upmeter/pkg/server/ranges"
)

type PublicStatusResponse struct {
	Status PublicStatus  `json:"status"`
	Rows   []GroupStatus `json:"rows"`
}

type GroupStatus struct {
	Group  string       `json:"group"`
	Status PublicStatus `json:"status"`
}

type PublicStatus string

const (
	StatusOperational PublicStatus = "Operational"
	StatusDegraded    PublicStatus = "Degraded"
	StatusOutage      PublicStatus = "Outage"
)

type PublicStatusHandler struct {
	DbCtx           *dbcontext.DbContext
	DowntimeMonitor *crd.DowntimeMonitor
	ProbeLister     registry.ProbeLister
}

func (h *PublicStatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Infoln("PublicStatus", r.RemoteAddr, r.RequestURI)

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, "%d GET is required\n", http.StatusMethodNotAllowed)
		return
	}

	statuses, status, err := h.getStatusSummary()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%d Error getting current status\n", http.StatusInternalServerError)
		log.Errorln("Cannot get status: %w", err)
		return
	}

	out, err := json.Marshal(&PublicStatusResponse{
		Rows:   statuses,
		Status: status,
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%d Error: %s\n", http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	w.Write(out)
}

// getStatusSummary returns total statuses for each group for the current partial 5m timeslot plus
// previous full 5m timeslot.
func (h *PublicStatusHandler) getStatusSummary() ([]GroupStatus, PublicStatus, error) {
	daoCtx := h.DbCtx.Start()
	defer daoCtx.Stop()

	rng := threeStepRange(time.Now())
	log.Infof("Request public status from=%d to=%d at %d", rng.From, rng.To, time.Now().Unix())

	muteTypes := []string{
		"Maintenance",
		"InfrastructureMaintenance",
		"InfrastructureAccident",
	}

	groups := h.ProbeLister.Groups()
	groupStatuses := make([]GroupStatus, 0)
	for _, group := range groups {
		ref := check.ProbeRef{
			Group: group,
			Probe: dao.GroupAggregation,
		}

		filter := &statusFilter{
			stepRange:         rng,
			probeRef:          ref,
			muteDowntimeTypes: muteTypes,
		}

		resp, err := getStatus(h.DbCtx, h.DowntimeMonitor, filter)
		if err != nil {
			log.Errorf("cannot calculate status for group %s: %v", group, err)
			return nil, StatusOutage, err
		}

		summary, err := pickSummary(ref, resp.Statuses)
		if err != nil {
			log.Errorf("cannot parse status for group %s: %v", group, err)
			return nil, StatusOutage, err
		}

		gs := GroupStatus{
			Group:  group,
			Status: calculateStatus(summary),
		}
		groupStatuses = append(groupStatuses, gs)
	}

	totalStatus := calculateTotalStatus(groupStatuses)
	return groupStatuses, totalStatus, nil
}

func threeStepRange(now time.Time) ranges.StepRange {
	step := 5 * time.Minute
	slotStart := now.Truncate(step)
	from := slotStart.Add(-2 * step)
	to := slotStart.Add(step)
	return ranges.NewStepRange(from.Unix(), to.Unix(), int64(step.Seconds()))
}

// pickSummary makes assertions
func pickSummary(ref check.ProbeRef, statuses map[string]map[string][]entity.EpisodeSummary) ([]entity.EpisodeSummary, error) {
	g := ref.Group
	p := ref.Probe

	if _, ok := statuses[g]; !ok {
		return nil, fmt.Errorf("no status for group '%s'", g)
	}
	if _, ok := statuses[g][p]; !ok {
		return nil, fmt.Errorf("no status for group '%s' probe '%s'", g, p)
	}

	episodeSummaries := statuses[g][p]
	n := len(episodeSummaries) - 1 // ignore summary column in the end
	if n != 3 {
		return nil, fmt.Errorf("bad results count %d for group '%s' probe '%s'", n, g, p)
	}

	return episodeSummaries[:3], nil
}

// calculateStatus returns the status for a group.
//
// Input array should have 3 elements
func calculateStatus(summary []entity.EpisodeSummary) PublicStatus {
	slotSize := 5 * time.Minute

	var prev, current entity.EpisodeSummary
	if len(summary) == 2 || summary[2].NoData == slotSize {
		prev, current = summary[0], summary[1]
	} else {
		prev, current = summary[1], summary[2]
	}

	// Ignore empty EpisodeSummary, i.e. when NoData equals slot size
	if current.Down == 0 && prev.Down == 0 &&
		(current.Up > 0 || (prev.Up > 0 && current.Up == 0 && current.NoData == slotSize)) {
		return StatusOperational
	}

	if current.Up == 0 && current.Muted == 0 &&
		prev.Up == 0 && prev.Muted == 0 &&
		(current.Down > 0 || (prev.Down > 0 && current.Down == 0 && current.NoData == slotSize)) {
		return StatusOutage
	}

	return StatusDegraded
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
