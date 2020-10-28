package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"upmeter/pkg/probe/types"
	"upmeter/pkg/upmeter/entity"
)

func DowntimeHandler(w http.ResponseWriter, r *http.Request) {
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
	var episodes []types.DowntimeEpisode
	err := decoder.Decode(&episodes)
	//log.Infof("episodes: %+v", episodes)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%d Error: %s\n", http.StatusBadRequest, err)
		return
	}

	// Put downtime episodes to storage.
	entity.SaveDowntimeEpisodes(episodes)

	// Response with empty object if everything is ok.
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "{}")
}
