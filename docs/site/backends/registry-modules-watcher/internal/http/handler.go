package v1

import (
	"io"
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
	_, _ = io.WriteString(w, "ok")
}

func (h *RegistryModulesWatcherHandler) handleHealthZ(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok")
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
