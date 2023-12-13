package main

import (
	"io"
	"net/http"
	"sync/atomic"

	"github.com/gorilla/mux"
)

func newHandler(highAvailability bool) *mux.Router {
	var isReady atomic.Bool

	if !highAvailability {
		isReady.Store(true)
	}

	r := mux.NewRouter()
	r.Handle("/healthz/ready", newHealthzHandler(&isReady))
	r.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "OK") })
	r.Handle("/loadDocArchive/{moduleName}/{version}", newLoadHandler(src)).Methods(http.MethodPost)
	r.Handle("/build", newBuildHandler(src, dst, &isReady)).Methods(http.MethodPost)

	return r
}
