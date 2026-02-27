/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/lib/logger/sl"
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
	srv.logger.Info("starting healthz server", slog.String("bind_address", srv.bindAddr))

	if err := srv.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("healthz server failed: %w", err)
	}
	return nil
}

func (srv *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	srv.logger.Info("Stopping healthz server", slog.String("bind_address", srv.bindAddr))
	if err := srv.srv.Shutdown(ctx); err != nil {
		srv.logger.Error("Healthz server graceful stop failed, force...", sl.Err(err))
		err = srv.srv.Close()
		if err != nil {
			srv.logger.Error("Healthz server force stop failed", sl.Err(err))
		}
	}
}
