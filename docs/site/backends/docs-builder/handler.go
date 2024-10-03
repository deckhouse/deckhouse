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

package main

import (
	"io"
	"net/http"
	"sync/atomic"

	"github.com/gorilla/mux"
)

type DocsBuilderHandler struct {
	http.Handler

	baseDir string
	destDir string
	isReady atomic.Bool
	m       *channelMappingEditor
}

func newHandler(highAvailability bool) *DocsBuilderHandler {
	r := mux.NewRouter()

	var h = &DocsBuilderHandler{
		Handler: r,
		baseDir: src,
		destDir: dst,
		m:       newChannelMappingEditor(src),
	}

	if !highAvailability {
		h.isReady.Store(true)
	}

	r.HandleFunc("/readyz", h.handleReadyZ)
	r.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "OK") })
	r.Handle("/loadDocArchive/{moduleName}/{version}", h.newLoadHandler()).Methods(http.MethodPost)
	r.Handle("/build", h.newBuildHandler()).Methods(http.MethodPost)
	r.Handle("/doc/{moduleName}", h.newDeleteHandler()).Methods(http.MethodDelete)

	return h
}

func (h *DocsBuilderHandler) handleReadyZ(w http.ResponseWriter, _ *http.Request) {
	if h.isReady.Load() {
		_, _ = io.WriteString(w, "ok")

		return
	}

	http.Error(w, "Waiting for first build", http.StatusInternalServerError)
}

func (h *DocsBuilderHandler) newLoadHandler() *loadHandler {
	return &loadHandler{baseDir: src, channelMappingEditor: h.m}
}

func (h *DocsBuilderHandler) newBuildHandler() *buildHandler {
	return &buildHandler{
		src:                  h.baseDir,
		dst:                  h.destDir,
		wasCalled:            &h.isReady,
		channelMappingEditor: h.m,
	}
}

func (h *DocsBuilderHandler) newDeleteHandler() *deleteHandler {
	return &deleteHandler{baseDir: h.baseDir, channelMappingEditor: h.m}
}
