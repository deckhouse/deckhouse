package http

import (
	"context"
	"errors"
	"fencing-agent/internal/helper/logger/sl"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type Server struct {
	srv      *http.Server
	bindAddr string
	logger   *log.Logger
}

func New(logger *log.Logger, bindAddress string) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := http.Server{Addr: bindAddress, Handler: mux}
	return &Server{
		srv:      &srv,
		bindAddr: bindAddress,
		logger:   logger,
	}
}

func (srv *Server) StartHealthzServer() {
	srv.logger.Info("Stating healthz server", slog.String("bindAddress", srv.bindAddr))

	if err := srv.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		srv.logger.Error("Healthz server failed", sl.Err(err))
	}
}

func (srv *Server) StopHealthzServer(ctx context.Context) error {
	srv.logger.Info("Stopping healthz server", slog.String("bindAddress", srv.bindAddr))
	if err := srv.srv.Shutdown(ctx); err != nil {
		srv.logger.Error("Healthz server gracefull stop failed, force...", sl.Err(err))
		err = srv.srv.Close()
		return fmt.Errorf("failed to force close healthz server: %w", err)
	}
	return nil
}
