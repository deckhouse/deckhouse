package main

import (
	"io"
	"net/http"
	"sync/atomic"
)

func newHealthzHandler(isReady *atomic.Bool) *healthzHandler {
	return &healthzHandler{
		isReady: isReady,
	}
}

type healthzHandler struct {
	isReady *atomic.Bool
}

func (h *healthzHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.isReady.Load() {
		_, _ = io.WriteString(w, "ok")
	} else {
		http.Error(w, "Waiting for first build", http.StatusInternalServerError)
	}
}
