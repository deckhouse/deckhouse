package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"upmeter/pkg/check"
	dbcontext "upmeter/pkg/upmeter/db/context"
	"upmeter/pkg/upmeter/entity"
	"upmeter/pkg/upmeter/remotewrite"
)

type EpisodesPayload struct {
	Origin   string                  `json:"origin"`
	Episodes []check.DowntimeEpisode `json:"episodes"`
}

type DowntimeHandler struct {
	DbCtx       *dbcontext.DbContext
	RemoteWrite remotewrite.Exporter
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
	var data EpisodesPayload
	err := decoder.Decode(&data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%d Error: %s\n", http.StatusBadRequest, err)
		return
	}

	episodes := data.Episodes
	origin := data.Origin

	// Save episodes to the database
	ctx := h.DbCtx.Start()
	defer ctx.Stop()
	saved30s := entity.Save30sEpisodes(ctx, episodes)
	saved5m := entity.Update5mEpisodes(ctx, saved30s)

	// Send episodes to metrics storage
	log.Debugf("exporting 30s episodes by agent=%s", origin)
	err = h.RemoteWrite.Export(origin, saved30s, 30)
	if err != nil {
		log.Errorf("error saving 30s episode for remote_write export: %v", err)
	}

	log.Debugf("exporting 5m episodes by agent=%s", origin)
	err = h.RemoteWrite.Export(origin, saved5m, 300)
	if err != nil {
		log.Errorf("error saving 5m episode for remote_write export: %v", err)
	}

	// Respond with empty object if everything is ok
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "{}")
}
