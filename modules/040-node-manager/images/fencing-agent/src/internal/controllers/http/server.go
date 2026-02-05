package http

import (
	"context"
	"errors"
	"fencing-agent/internal/helper/logger/sl"
	"fmt"
	"log/slog"
	"net/http"
	"time"

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

func (srv *Server) Run() error {
	srv.logger.Info("stating healthz server", slog.String("bindAddress", srv.bindAddr))

	if err := srv.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("healthz server failed: %w", err)
	}

}

func (srv *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	srv.logger.Info("Stopping healthz server", slog.String("bindAddress", srv.bindAddr))
	if err := srv.srv.Shutdown(ctx); err != nil {
		srv.logger.Error("Healthz server gracefull stop failed, force...", sl.Err(err))
		err = srv.srv.Close()
		if err != nil {
			srv.logger.Error("Healthz server force stop failed", sl.Err(err))
		}
	}
}
