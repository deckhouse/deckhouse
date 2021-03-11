package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"upmeter/pkg/check"
	dbcontext "upmeter/pkg/upmeter/db/context"
	"upmeter/pkg/upmeter/entity"
)

type DowntimeHandler struct {
	DbCtx *dbcontext.DbContext
}

func (h *DowntimeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("Downtime", r.RemoteAddr, r.RequestURI)

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, "%d POST is required\n", http.StatusBadRequest)
		return
	}
	// check content-type
	if r.Header.Get("Content-Type") != "application/json" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "%d application/json is required\n", http.StatusBadRequest)
		return
	}

	// Decode DowntimeEpisodes json from body
	decoder := json.NewDecoder(r.Body)
	var episodes []check.DowntimeEpisode
	err := decoder.Decode(&episodes)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%d Error: %s\n", http.StatusBadRequest, err)
		return
	}

	// Put downtime episodes to storage.
	entity.SaveDowntimeEpisodes(h.DbCtx, episodes)

	// Response with empty object if everything is ok.
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "{}")
}
