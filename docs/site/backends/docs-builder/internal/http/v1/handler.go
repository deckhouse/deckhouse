// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1

import (
	"io"
	"net/http"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type DocsBuilderHandler struct {
	http.Handler

	mux      *http.ServeMux
	onDemand OnDemand

	logger *log.Logger
}

func NewHandler(logger *log.Logger) *DocsBuilderHandler {
	r := http.NewServeMux()

	var h = &DocsBuilderHandler{
		Handler: r,
		mux:     r,
		logger:  logger,
	}

	r.HandleFunc("/readyz", h.handleReadyZ)
	r.HandleFunc("/healthz", h.handleHealthZ)

	return h
}

func (h *DocsBuilderHandler) handleReadyZ(w http.ResponseWriter, _ *http.Request) {
	if h.onDemand.Ready == nil || h.onDemand.Ready() {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")

		return
	}

	http.Error(w, "Waiting for the versions from S3", http.StatusInternalServerError)
}

func (h *DocsBuilderHandler) handleHealthZ(w http.ResponseWriter, _ *http.Request) {
	var err error
	if h.onDemand.Live != nil {
		err = h.onDemand.Live()
	}
	if err != nil {
		h.logger.Error("the liveness probe fails", log.Err(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok")
}
