package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"upmeter/pkg/crd"
	dbcontext "upmeter/pkg/upmeter/db/context"
	"upmeter/pkg/upmeter/entity"
)

type PublicStatusResponse struct {
	Rows   []entity.GroupStatusInfo `json:"rows"`
	Status string                   `json:"status"`
}

type PublicStatusHandler struct {
	DbCtx           *dbcontext.DbContext
	DowntimeMonitor *crd.DowntimeMonitor
}

func (h *PublicStatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Infoln("PublicStatus", r.RemoteAddr, r.RequestURI)

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, "%d GET is required\n", http.StatusMethodNotAllowed)
		return
	}

	statuses, status, err := entity.CurrentStatusForGroups(h.DbCtx, h.DowntimeMonitor)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%d Error getting current status\n", http.StatusInternalServerError)
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
