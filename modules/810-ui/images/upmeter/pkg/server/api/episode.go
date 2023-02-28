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
	"time"

	log "github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/check"
	dbcontext "d8.io/upmeter/pkg/db/context"
	"d8.io/upmeter/pkg/server/entity"
	"d8.io/upmeter/pkg/server/remotewrite"
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

	log.Debugf("Storing for export 30s episodes by agent=%s", origin)

	err := remoteWrite.Export(origin, saved30s, 30*time.Second)
	if err != nil {
		return fmt.Errorf("error saving 30s episode for remote_write export: %v", err)
	}

	fulfilled5m := chooseByLatestSubSlot(saved30s, saved5m)
	if len(fulfilled5m) == 0 {
		return nil
	}

	log.Debugf("Storing for export 5m episodes by agent=%s", origin)

	err = remoteWrite.Export(origin, fulfilled5m, 5*time.Minute)
	if err != nil {
		return fmt.Errorf("error saving 5m episode for remote_write export: %v", err)
	}

	return nil
}

func chooseByLatestSubSlot(saved30s, saved5m []*check.Episode) []*check.Episode {
	var (
		chosen = make([]*check.Episode, 0)
		refs   = make(map[string]struct{})
	)

	// filter refs by subslots
	for _, ep30s := range saved30s {
		if !isLastSubSlot(ep30s.TimeSlot) {
			continue
		}
		id := ep30s.ProbeRef.Id()
		slot5m := ep30s.TimeSlot.Truncate(5 * time.Minute)
		key := fmt.Sprintf("%s:%d", id, slot5m.Unix())

		refs[key] = struct{}{}
	}

	// pick fulfilled
	for _, ep5m := range saved5m {
		id := ep5m.ProbeRef.Id()
		key := fmt.Sprintf("%s:%d", id, ep5m.TimeSlot.Unix())
		if _, ok := refs[key]; !ok {
			continue
		}
		chosen = append(chosen, ep5m)
	}

	return chosen
}

// isLastSubSlot finds out whether the 30s slot is the last one within 5m slot.
func isLastSubSlot(got time.Time) bool {
	want := got.Round(5 * time.Minute).Add(-30 * time.Second)
	return got.Equal(want)
}
