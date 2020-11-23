package api

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net/http"
	"upmeter/pkg/crd"
	"upmeter/pkg/upmeter/entity"
)

type PublicStatusResponse struct {
	Rows   []entity.GroupStatusInfo `json:"rows"`
	Status string                   `json:"status"`
}

type PublicStatusHandler struct {
	CrdMonitor *crd.Monitor
}

func NewPublicStatusHandler() *PublicStatusHandler {
	return &PublicStatusHandler{}
}

func (h *PublicStatusHandler) WithCRDMonitor(crdMonitor *crd.Monitor) {
	h.CrdMonitor = crdMonitor
}

func (h *PublicStatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("PublicStatus", r.RemoteAddr, r.RequestURI)

	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, "%d GET is required\n", http.StatusMethodNotAllowed)
		return
	}

	statuses, status, err := entity.CurrentStatusForGroups(h.CrdMonitor)
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
