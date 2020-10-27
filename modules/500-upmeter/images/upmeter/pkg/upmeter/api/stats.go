package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"upmeter/pkg/upmeter/db"
)

// TODO make better stats!
func Stats(w http.ResponseWriter, r *http.Request) {
	log.Println("Stats", r.RemoteAddr, r.RequestURI)

	stats, err := db.Downtime30s.Stats()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%d Error: %s\n", http.StatusBadRequest, err)
		return
	}

	out, err := json.Marshal(stats)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%d Error: %s\n", http.StatusBadRequest, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
}
