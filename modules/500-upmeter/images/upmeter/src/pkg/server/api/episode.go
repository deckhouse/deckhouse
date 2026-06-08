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
		fmt.Fprintf(w, "%d POST is required\n", http.StatusMethodNotAllowed)
		return
	}
	// check content-type
	if r.Header.Get("Content-Type") != "application/json" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "%d application/json is required\n", http.StatusBadRequest)
		return
	}

	// Decode Episodes json from body
	reqStart := time.Now()
	decoder := json.NewDecoder(r.Body)
	var data EpisodesPayload
	err := decoder.Decode(&data)
	decodeDur := time.Since(reqStart)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%d Error: %s\n", http.StatusBadRequest, err)
		return
	}

	earliest, latest := episodesSlotRange(data.Episodes)
	log.Infof("received episodes: origin=%s episodes=%d slots=%d earliest=%s latest=%s decodeDur=%s",
		data.Origin, len(data.Episodes), countEpisodeSlots(data.Episodes),
		fmtSlot(earliest), fmtSlot(latest), decodeDur)

	saveStart := time.Now()
	err = save(h.DbCtx, h.RemoteWrite, data)
	saveDur := time.Since(saveStart)
	if err != nil {
		log.Errorf("processing episodes failed: origin=%s episodes=%d saveDur=%s totalDur=%s: %v",
			data.Origin, len(data.Episodes), saveDur, time.Since(reqStart), err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%d Error: %s\n", http.StatusBadRequest, err)
		return
	}

	log.Infof("processed episodes: origin=%s episodes=%d saveDur=%s totalDur=%s",
		data.Origin, len(data.Episodes), saveDur, time.Since(reqStart))

	// Respond with empty object if everything is ok
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "{}")
}

// episodesSlotRange returns the earliest and latest time slot in the payload. Zero times are returned
// for an empty payload.
func episodesSlotRange(episodes []check.Episode) (earliest, latest time.Time) {
	if len(episodes) == 0 {
		return earliest, latest
	}
	earliest = episodes[0].TimeSlot
	latest = episodes[0].TimeSlot
	for _, ep := range episodes[1:] {
		if ep.TimeSlot.Before(earliest) {
			earliest = ep.TimeSlot
		}
		if ep.TimeSlot.After(latest) {
			latest = ep.TimeSlot
		}
	}
	return earliest, latest
}

// countEpisodeSlots counts how many distinct time slots the payload spans.
func countEpisodeSlots(episodes []check.Episode) int {
	seen := make(map[int64]struct{}, len(episodes))
	for _, ep := range episodes {
		seen[ep.TimeSlot.Unix()] = struct{}{}
	}
	return len(seen)
}

func fmtSlot(t time.Time) string {
	return t.Format("15:04:05")
}

// Save episodes to the database
func save(dbctx *dbcontext.DbContext, remoteWrite remotewrite.Exporter, data EpisodesPayload) error {
	ctx := dbctx.Start()
	defer ctx.Stop()

	episodes := data.Episodes
	origin := data.Origin

	save30sStart := time.Now()
	saved30s := entity.Save30sEpisodes(ctx, episodes)
	save30sDur := time.Since(save30sStart)

	update5mStart := time.Now()
	saved5m := entity.Update5mEpisodes(ctx, saved30s)
	update5mDur := time.Since(update5mStart)

	log.Infof("saved episodes to db: origin=%s received=%d saved30s=%d updated5m=%d save30sDur=%s update5mDur=%s",
		origin, len(episodes), len(saved30s), len(saved5m), save30sDur, update5mDur)

	// Send episodes to metrics storage

	rwStart := time.Now()
	err := remoteWrite.Export(origin, saved30s, 30*time.Second)
	if err != nil {
		return fmt.Errorf("error saving 30s episode for remote_write export: %v", err)
	}

	fulfilled5m := chooseByLatestSubSlot(saved30s, saved5m)
	if len(fulfilled5m) == 0 {
		log.Infof("queued for remote_write: origin=%s queued30s=%d queued5m=0 remoteWriteDur=%s",
			origin, len(saved30s), time.Since(rwStart))
		return nil
	}

	err = remoteWrite.Export(origin, fulfilled5m, 5*time.Minute)
	if err != nil {
		return fmt.Errorf("error saving 5m episode for remote_write export: %v", err)
	}

	log.Infof("queued for remote_write: origin=%s queued30s=%d queued5m=%d remoteWriteDur=%s",
		origin, len(saved30s), len(fulfilled5m), time.Since(rwStart))

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
