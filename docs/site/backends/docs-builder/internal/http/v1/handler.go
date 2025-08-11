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
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/docs"
)

type DocsBuilderHandler struct {
	http.Handler

	docsService *docs.Service

	logger *log.Logger
}

func NewHandler(docsService *docs.Service, logger *log.Logger) *DocsBuilderHandler {
	r := http.NewServeMux()

	var h = &DocsBuilderHandler{
		Handler:     r,
		docsService: docsService,
		logger:      logger,
	}

	r.HandleFunc("/readyz", h.handleReadyZ)
	r.HandleFunc("/healthz", h.handleHealthZ)

	r.HandleFunc("GET /api/v1/doc", h.handleGetDocsInfo)
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

func (h *DocsBuilderHandler) handleGetDocsInfo(w http.ResponseWriter, _ *http.Request) {
	h.logger.Info("getting all docs info")

	modules, err := h.docsService.GetDocumentationInfo()
	if err != nil {
		h.logger.Error("getting documentation info", log.Err(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// module name - channel code - version
	result := make(map[string]map[string]string)

	for _, mod := range modules {
		result[mod.ModuleName] = make(map[string]string)
		for _, channel := range mod.Channels {
			result[mod.ModuleName][channel.Code] = channel.Version
		}
	}

	w.WriteHeader(http.StatusOK)

	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		h.logger.Error("marshal documentation", log.Err(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (h *DocsBuilderHandler) handleUpload(w http.ResponseWriter, r *http.Request) {
	channelsStr := r.URL.Query().Get("channels")
	channels := []string{"stable"}
	if len(channelsStr) != 0 {
		channels = strings.Split(channelsStr, ",")
	}

	moduleName := r.PathValue("moduleName")
	version := r.PathValue("version")

	h.logger.Info("uploading module", slog.String("module", moduleName), slog.String("version", version), slog.String("channels", strings.Join(channels, ",")))

	err := h.docsService.Upload(r.Body, moduleName, version, channels)
	if err != nil {
		h.logger.Error("upload", log.Err(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *DocsBuilderHandler) handleBuild(w http.ResponseWriter, _ *http.Request) {
	err := h.docsService.Build()
	if err != nil {
		h.logger.Error("build", log.Err(err))
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

	h.logger.Info("deleting module", slog.String("module", moduleName), slog.String("channels", strings.Join(channels, ",")))
	err := h.docsService.Delete(moduleName, channels)
	if err != nil {
		h.logger.Error("delete", log.Err(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
