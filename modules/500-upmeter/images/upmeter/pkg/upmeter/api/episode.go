package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	"upmeter/pkg/check"
	dbcontext "upmeter/pkg/upmeter/db/context"
	"upmeter/pkg/upmeter/entity"
	"upmeter/pkg/upmeter/remotewrite"
)

type EpisodesPayload struct {
	Origin   string          `json:"origin"`
	Episodes []check.Episode `json:"episodes"`
}

type AddEpisodesHandler struct {
	DbCtx       *dbcontext.DbContext
	RemoteWrite remotewrite.Exporter
}

func (h *AddEpisodesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Infoln("Downtime", r.RemoteAddr, r.RequestURI)

	if r.Method != http.MethodPost {
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

	// Decode Episodes json from body
	decoder := json.NewDecoder(r.Body)
	var data EpisodesPayload
	err := decoder.Decode(&data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%d Error: %s\n", http.StatusBadRequest, err)
		return
	}

	err = save(h.DbCtx, h.RemoteWrite, data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%d Error: %s\n", http.StatusBadRequest, err)
		return
	}

	// Respond with empty object if everything is ok
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "{}")
}

// Save episodes to the database
func save(dbctx *dbcontext.DbContext, remoteWrite remotewrite.Exporter, data EpisodesPayload) error {
	ctx := dbctx.Start()
	defer ctx.Stop()

	episodes := data.Episodes
	origin := data.Origin

	saved30s := entity.Save30sEpisodes(ctx, episodes)
	saved5m := entity.Update5mEpisodes(ctx, saved30s)

	// Send episodes to metrics storage

	log.Debugf("exporting 30s episodes by agent=%s", origin)

	err := remoteWrite.Export(origin, saved30s, 30*time.Second)
	if err != nil {
		return fmt.Errorf("error saving 30s episode for remote_write export: %v", err)
	}

	log.Debugf("exporting 5m episodes by agent=%s", origin)

	err = remoteWrite.Export(origin, saved5m, 5*time.Minute)
	if err != nil {
		return fmt.Errorf("error saving 5m episode for remote_write export: %v", err)
	}

	return nil
}
