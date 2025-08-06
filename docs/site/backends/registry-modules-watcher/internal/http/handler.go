// Copyright 2025 Flant JSC
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
	"log/slog"
	"net/http"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

type RegistryModulesWatcherHandler struct {
	http.Handler

	logger *log.Logger
}

func NewHandler(logger *log.Logger) *RegistryModulesWatcherHandler {
	r := http.NewServeMux()

	var h = &RegistryModulesWatcherHandler{
		Handler: r,
		logger:  logger,
	}

	r.HandleFunc("/readyz", h.handleReadyZ)
	r.HandleFunc("/healthz", h.handleHealthZ)

	return h
}

func (h *RegistryModulesWatcherHandler) handleReadyZ(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	n, _ := w.Write([]byte("ok"))
	h.logger.Debug("handleReadyZ", slog.Int("n", n))
}

func (h *RegistryModulesWatcherHandler) handleHealthZ(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	n, _ := w.Write([]byte("ok"))
	h.logger.Debug("handleHealthZ", slog.Int("n", n))
}

func NewMetricHandler(logger *log.Logger, metricStorage *metricsstorage.MetricStorage) *RegistryModulesWatcherHandler {
	r := http.NewServeMux()

	var h = &RegistryModulesWatcherHandler{
		Handler: r,
		logger:  logger,
	}

	r.Handle("/metrics", metricStorage.Handler())

	return h
}
