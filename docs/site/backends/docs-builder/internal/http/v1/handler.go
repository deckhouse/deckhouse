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
	"k8s.io/klog/v2"
)

type DocsBuilderHandler struct {
	http.Handler

	docsService *docs.Service
}

func NewHandler(docsService *docs.Service) *DocsBuilderHandler {
	r := http.NewServeMux()

	var h = &DocsBuilderHandler{
		Handler:     r,
		docsService: docsService,
	}

	r.HandleFunc("/readyz", h.handleReadyZ)
	r.HandleFunc("/healthz", h.handleHealthZ)

	r.HandleFunc("POST /api/v1/doc/{moduleName}/{version}", h.handleUpload)
	r.HandleFunc("DELETE /api/v1/doc/{moduleName}", h.handleDelete)
	r.HandleFunc("POST /api/v1/build", h.handleBuild)

	return h
}

func (h *DocsBuilderHandler) handleReadyZ(w http.ResponseWriter, _ *http.Request) {
	if h.docsService.IsReady() {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")

		return
	}

	http.Error(w, "Waiting for first build", http.StatusInternalServerError)
}

func (h *DocsBuilderHandler) handleHealthZ(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok")
}

func (h *DocsBuilderHandler) handleUpload(w http.ResponseWriter, r *http.Request) {
	channelsStr := r.URL.Query().Get("channels")
	channels := []string{"stable"}
	if len(channelsStr) != 0 {
		channels = strings.Split(channelsStr, ",")
	}

	moduleName := r.PathValue("moduleName")
	version := r.PathValue("version")

	klog.Infof("loading %s %s: %s", moduleName, version, channels)

	err := h.docsService.Upload(r.Body, moduleName, version, channels)
	if err != nil {
		klog.Error(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *DocsBuilderHandler) handleBuild(w http.ResponseWriter, r *http.Request) {
	err := h.docsService.Build()
	if err != nil {
		klog.Error(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *DocsBuilderHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	channelsStr := r.URL.Query().Get("channels")
	channels := []string{"stable"}
	if len(channelsStr) != 0 {
		channels = strings.Split(channelsStr, ",")
	}

	moduleName := r.PathValue("moduleName")

	klog.Infof("deleting %s: %s", moduleName, channels)
	err := h.docsService.Delete(moduleName, channels)
	if err != nil {
		klog.Error(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
