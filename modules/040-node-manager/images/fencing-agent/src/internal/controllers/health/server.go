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

// Package health serves liveness and readiness probes of fencing-agent.
package health

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const shutdownTimeout = 5 * time.Second

type Server struct {
	addr   string
	logger *log.Logger
}

func NewServer(addr string, logger *log.Logger) *Server {
	return &Server{addr: addr, logger: logger}
}

func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	probe := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
	mux.HandleFunc("/healthz", probe)
	mux.HandleFunc("/readyz", probe)

	srv := &http.Server{
		Addr:              s.addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	s.logger.Info("health server started", "address", s.addr)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}

		if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
			return err
		}

		return nil
	}
}
