package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"upmeter/pkg/checks"
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
	Episodes  []checks.DowntimeEpisode                  `json:"episodes"`
	Incidents []checks.DowntimeIncident                 `json:"incidents"`
}

type StatusRangeHandler struct {
	DbCtx      *dbcontext.DbContext
	CrdMonitor *crd.Monitor
}

func (h *StatusRangeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("StatusRange", r.RemoteAddr, r.RequestURI)

	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, "%d GET is required\n", http.StatusMethodNotAllowed)
		return
	}

	// Check parameters
	from, to, step, err := DecodeFromToStep(r.URL.Query()["from"], r.URL.Query()["to"], r.URL.Query()["step"])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%d Error: %s\n", http.StatusInternalServerError, err)
		return
	}
	// Adjust range to step slots.
	stepRanges := entity.CalculateAdjustedStepRanges(from, to, step)
	log.Infof("[from to step] input [%d %d %d] adjusted to [%d, %d, %d]",
		from, to, step,
		stepRanges.From, stepRanges.To, stepRanges.Step)

	groupNameList := r.URL.Query()["group"]
	if len(groupNameList) == 0 || groupNameList[0] == "" {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%d Error: 'group' is required\n", http.StatusInternalServerError)
		return
	}
	groupName := groupNameList[0]

	probeNameList := r.URL.Query()["probe"]
	probeName := ""
	if len(probeNameList) > 0 {
		probeName = probeNameList[0]
	}

	muteDowntimeTypesArgs := r.URL.Query()["muteDowntimeTypes"]
	muteDowntimeTypes := DecodeMuteDowntimeTypes(muteDowntimeTypesArgs)
	if len(muteDowntimeTypes) == 0 {
		muteDowntimeTypes = []string{
			"Maintenance",
			"InfrastructureMaintenance",
			"InfrastructureAccident",
		}
	}

	daoCtx := h.DbCtx.Start()
	defer daoCtx.Stop()

	dao5m := dao.NewDowntime5mDao(daoCtx)
	episodes, err := dao5m.ListEpisodeSumsForRanges(stepRanges, groupName, probeName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%d Error: %s\n", http.StatusInternalServerError, err)
		return
	}

	incidents := h.CrdMonitor.FilterDowntimeIncidents(stepRanges.From, stepRanges.To, groupName, muteDowntimeTypes)

	statuses := entity.CalculateStatuses(episodes, incidents, stepRanges.Ranges, groupName, probeName)

	out, err := json.Marshal(&StatusResponse{
		Statuses: statuses,
		Step:     stepRanges.Step,
		From:     stepRanges.From,
		To:       stepRanges.To,
		//Episodes:  episodes, // To much data, only for debug.
		Incidents: incidents,
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
