package v1

import (
	"net/http"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

type RegistryModulesWatcherHandler struct {
	http.Handler

	logger *log.Logger
}

func NewHandler(logger *log.Logger, metricStorage *metricsstorage.MetricStorage) *RegistryModulesWatcherHandler {
	r := http.NewServeMux()

	var h = &RegistryModulesWatcherHandler{
		Handler: r,
		logger:  logger,
	}

	r.HandleFunc("/readyz", h.handleReadyZ)
	r.HandleFunc("/healthz", h.handleHealthZ)
	r.Handle("/metrics", metricStorage.Handler())

	return h
}

func (h *RegistryModulesWatcherHandler) handleReadyZ(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("ok"))
	if err != nil {
		h.logger.Error("writing response", log.Err(err))
	}
}

func (h *RegistryModulesWatcherHandler) handleHealthZ(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("ok"))
	if err != nil {
		h.logger.Error("writing response", log.Err(err))
	}
}
