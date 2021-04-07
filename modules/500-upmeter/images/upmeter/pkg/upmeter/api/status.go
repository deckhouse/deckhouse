package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"upmeter/pkg/check"
	"upmeter/pkg/crd"
	dbcontext "upmeter/pkg/upmeter/db/context"
	"upmeter/pkg/upmeter/db/dao"
	"upmeter/pkg/upmeter/entity"
)

type StatusResponse struct {
	Step      int64                                     `json:"step"`
	From      int64                                     `json:"from"`
	To        int64                                     `json:"to"`
	Statuses  map[string]map[string][]entity.StatusInfo `json:"statuses"`
	Episodes  []check.DowntimeEpisode                   `json:"episodes"`
	Incidents []check.DowntimeIncident                  `json:"incidents"`
}

type StatusRangeHandler struct {
	DbCtx           *dbcontext.DbContext
	DowntimeMonitor *crd.DowntimeMonitor
}

func (h *StatusRangeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Infoln("StatusRange", r.RemoteAddr, r.RequestURI)

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, "%d GET is required\n", http.StatusMethodNotAllowed)
		return
	}

	input, err := parseStatusInput(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp, err := getStatus(h.DbCtx, h.DowntimeMonitor, input)
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

type statusInput struct {
	timerange timerange
	probe     check.ProbeRef
	muteTypes []string
}

func parseStatusInput(r *http.Request) (*statusInput, error) {
	query := r.URL.Query()

	timerange, err := DecodeFromToStep(query.Get("from"), query.Get("to"), query.Get("step"))
	if err != nil {
		return nil, fmt.Errorf("cannot parse time range: %v", err)
	}

	groupName := query.Get("group")
	probeName := query.Get("probe")
	if groupName == "" {
		return nil, fmt.Errorf("'group' is required")
	}

	muteDowntimeTypes := decodeMuteDowntimeTypes(query.Get("muteDowntimeTypes"))

	parsed := &statusInput{
		timerange: timerange,
		probe:     check.ProbeRef{Group: groupName, Probe: probeName},
		muteTypes: muteDowntimeTypes,
	}

	return parsed, nil
}

func getStatus(dbctx *dbcontext.DbContext, monitor *crd.DowntimeMonitor, input *statusInput) (*StatusResponse, error) {
	// Adjust range to step slots.
	stepRanges := entity.CalculateAdjustedStepRanges(input.timerange.from, input.timerange.to, input.timerange.step)

	log.Infof("[from to step] input [%d %d %d] adjusted to [%d, %d, %d]",
		input.timerange.from, input.timerange.to, input.timerange.step,
		stepRanges.From, stepRanges.To, stepRanges.Step)

	daoCtx := dbctx.Start()
	defer daoCtx.Stop()

	dao5m := dao.NewDowntime5mDao(daoCtx)
	episodes, err := dao5m.ListEpisodeSumsForRanges(stepRanges, input.probe)
	if err != nil {
		return nil, err
	}

	muteDowntimeTypes := input.muteTypes
	if len(muteDowntimeTypes) == 0 {
		muteDowntimeTypes = []string{
			"Maintenance",
			"InfrastructureMaintenance",
			"InfrastructureAccident",
		}
	}

	incidents := monitor.FilterDowntimeIncidents(stepRanges.From, stepRanges.To, input.probe.Group, muteDowntimeTypes)

	statuses := entity.CalculateStatuses(episodes, incidents, stepRanges.Ranges, input.probe)

	body := &StatusResponse{
		Statuses: statuses,
		Step:     stepRanges.Step,
		From:     stepRanges.From,
		To:       stepRanges.To,
		//Episodes:  episodes, // To much data, only for debug.
		Incidents: incidents,
	}

	return body, nil
}
