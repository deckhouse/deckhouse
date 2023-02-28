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
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/check"
	dbcontext "d8.io/upmeter/pkg/db/context"
	"d8.io/upmeter/pkg/db/dao"
	"d8.io/upmeter/pkg/monitor/downtime"
	"d8.io/upmeter/pkg/server/entity"
	"d8.io/upmeter/pkg/server/ranges"
)

type StatusResponse struct {
	Step      int64                                         `json:"step"`
	From      int64                                         `json:"from"`
	To        int64                                         `json:"to"`
	Statuses  map[string]map[string][]entity.EpisodeSummary `json:"statuses"`
	Episodes  []check.Episode                               `json:"episodes"`
	Incidents []check.DowntimeIncident                      `json:"incidents"`
}

type StatusRangeHandler struct {
	DbCtx           *dbcontext.DbContext
	DowntimeMonitor *downtime.Monitor
}

func (h *StatusRangeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Infoln("StatusRange", r.RemoteAddr, r.RequestURI)

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, "%d GET is required\n", http.StatusMethodNotAllowed)
		return
	}

	filter, err := parseFilter(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Adjust range to 5m slots.
	rng := ranges.New5MinStepRange(filter.stepRange.From, filter.stepRange.To, filter.stepRange.Step)
	log.Infof("[from to step] input [%d %d %d] adjusted to [%d, %d, %d]",
		filter.stepRange.From, filter.stepRange.To, filter.stepRange.Step,
		rng.From, rng.To, rng.Step)
	filter.stepRange = rng

	// Force default filtering.
	if len(filter.muteDowntimeTypes) == 0 {
		filter.muteDowntimeTypes = []string{
			"Maintenance",
			"InfrastructureMaintenance",
			"InfrastructureAccident",
		}
	}

	// Query the DB
	daoCtx := h.DbCtx.Start()
	defer daoCtx.Stop()

	resp, err := getStatusSummary(dao.NewEpisodeDao5m(daoCtx), h.DowntimeMonitor, filter)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respJSON, err := json.Marshal(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%d Error: %s\n", http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	w.Write(respJSON)
}

type statusFilter struct {
	stepRange         ranges.StepRange
	probeRef          check.ProbeRef
	muteDowntimeTypes []string
}

func parseFilter(r *http.Request) (*statusFilter, error) {
	query := r.URL.Query()

	rng, err := parseStepRange(query.Get("from"), query.Get("to"), query.Get("step"))
	if err != nil {
		return nil, fmt.Errorf("cannot parse time range: %v", err)
	}

	groupName := query.Get("group")
	probeName := query.Get("probe")
	if groupName == "" {
		return nil, fmt.Errorf("'group' is required")
	}

	muteDowntimeTypes := parseDowntimeTypes(query.Get("muteDowntimeTypes"))

	parsed := &statusFilter{
		stepRange:         rng,
		probeRef:          check.ProbeRef{Group: groupName, Probe: probeName},
		muteDowntimeTypes: muteDowntimeTypes,
	}

	return parsed, nil
}

func getStatusSummary(lister entity.RangeEpisodeLister, monitor *downtime.Monitor, filter *statusFilter) (*StatusResponse, error) {
	incidents, err := fetchIncidents(monitor, filter.muteDowntimeTypes, filter.probeRef.Group, filter.stepRange)
	if err != nil {
		return nil, err
	}

	statuses, err := entity.GetSummary(lister, filter.probeRef, filter.stepRange, incidents)
	if err != nil {
		return nil, err
	}

	resp := &StatusResponse{
		Statuses:  statuses,
		Step:      filter.stepRange.Step,
		From:      filter.stepRange.From,
		To:        filter.stepRange.To,
		Incidents: incidents,
	}

	return resp, nil
}
