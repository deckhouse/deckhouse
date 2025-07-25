package v1

import (
	"net/http"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type RegistryModulesWatcherHandler struct {
	http.Handler

	logger *log.Logger
}

func NewHandler(logger *log.Logger, metricRegistry *prometheus.Registry) *RegistryModulesWatcherHandler {
	r := http.NewServeMux()

	var h = &RegistryModulesWatcherHandler{
		Handler: r,
		logger:  logger,
	}

	r.HandleFunc("/readyz", h.handleReadyZ)
	r.HandleFunc("/healthz", h.handleHealthZ)
	r.Handle("/metrics", promhttp.HandlerFor(metricRegistry, promhttp.HandlerOpts{}))

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
