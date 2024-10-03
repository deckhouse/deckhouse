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
	"strings"

	"github.com/flant/docs-builder/internal/docs"
	"github.com/gorilla/mux"
	"k8s.io/klog/v2"
)

type DocsBuilderHandler struct {
	http.Handler

	docsService *docs.Service
}

func NewHandler(docsService *docs.Service) *DocsBuilderHandler {
	r := mux.NewRouter()

	var h = &DocsBuilderHandler{
		Handler:     r,
		docsService: docsService,
	}

	r.HandleFunc("/readyz", h.handleReadyZ)
	r.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "OK") })
	r.HandleFunc("/loadDocArchive/{moduleName}/{version}", h.handleUpload).Methods(http.MethodPost)
	r.HandleFunc("/build", h.handleBuild).Methods(http.MethodPost)
	// r.Handle("/doc/{moduleName}", h.newDeleteHandler()).Methods(http.MethodDelete)

	return h
}

func (h *DocsBuilderHandler) handleReadyZ(w http.ResponseWriter, _ *http.Request) {
	if h.docsService.IsReady() {
		_, _ = io.WriteString(w, "ok")

		return
	}

	http.Error(w, "Waiting for first build", http.StatusInternalServerError)
}

// func (h *DocsBuilderHandler) newLoadHandler() *loadHandler {
// 	return &loadHandler{baseDir: h.baseDir, channelMappingEditor: h.m}
// }

// func (h *DocsBuilderHandler) newBuildHandler() *buildHandler {
// 	return &buildHandler{
// 		src:                  h.baseDir,
// 		dst:                  h.destDir,
// 		wasCalled:            &h.isReady,
// 		channelMappingEditor: h.m,
// 	}
// }

// func (h *DocsBuilderHandler) newDeleteHandler() *deleteHandler {
// 	return &deleteHandler{baseDir: h.baseDir, channelMappingEditor: h.m}
// }

func (h *DocsBuilderHandler) handleUpload(writer http.ResponseWriter, request *http.Request) {
	channelsStr := request.URL.Query().Get("channels")
	channels := []string{"stable"}
	if len(channelsStr) != 0 {
		channels = strings.Split(channelsStr, ",")
	}

	pathVars := mux.Vars(request)
	moduleName := pathVars["moduleName"]
	version := pathVars["version"]

	klog.Infof("loading %s %s: %s", moduleName, version, channels)

	err := h.docsService.Upload(request.Body, moduleName, version, channels)
	if err != nil {
		klog.Error(err)
		http.Error(writer, "Internal server error", http.StatusInternalServerError)
		return
	}

	writer.WriteHeader(http.StatusCreated)
}

func (h *DocsBuilderHandler) handleBuild(writer http.ResponseWriter, request *http.Request) {
	err := h.docsService.Build()
	if err != nil {
		klog.Error(err)
		http.Error(writer, "Internal server error", http.StatusInternalServerError)
		return
	}

	writer.WriteHeader(http.StatusOK)
}
