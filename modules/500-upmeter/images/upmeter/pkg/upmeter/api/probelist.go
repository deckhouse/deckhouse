package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"upmeter/pkg/upmeter/db"
)

func ProbeListHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("ProbeList", r.RemoteAddr, r.RequestURI)

	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, "%d GET is required\n", http.StatusMethodNotAllowed)
		return
	}

	/*
		select group, probe from downtime
	*/
	probeRefs, err := db.Downtime30s.ListGroupProbe()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%d Error: %s\n", http.StatusInternalServerError, err)
		return
	}

	out, err := json.Marshal(probeRefs)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%d Error: %s\n", http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	w.Write(out)
	//fmt.Fprintf(w, "{}")
}
