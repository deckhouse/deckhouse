// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package admission provides the HTTPS server used by package admission handlers.
package admission

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// readHeaderTimeout limits the time spent reading request headers.
	readHeaderTimeout = 10 * time.Second
	// readTimeout limits the time spent reading an entire request.
	readTimeout = 60 * time.Second
	// writeTimeout limits the time spent writing a response.
	writeTimeout = 30 * time.Second
	// idleTimeout limits how long an idle keep-alive connection remains open.
	idleTimeout = 120 * time.Second
	// shutdownTimeout limits graceful server shutdown.
	shutdownTimeout = 5 * time.Second

	// certificateFilename is the TLS certificate filename in certsDir.
	certificateFilename = "tls.crt"
	// privateKeyFilename is the TLS private key filename in certsDir.
	privateKeyFilename = "tls.key"
)

// Server serves package admission handlers over HTTPS.
type Server struct {
	listenPort string
	certsDir   string

	mux *http.ServeMux

	logger *log.Logger
}

// NewServer creates an admission server that listens on listenPort and loads TLS files from certsDir.
func NewServer(listenPort, certsDir string, logger *log.Logger) *Server {
	return &Server{
		listenPort: listenPort,
		certsDir:   certsDir,
		mux:        http.NewServeMux(),
		logger:     logger.Named("admission-server"),
	}
}

// RegisterHandler registers handler for route and reports malformed or conflicting routes as errors.
func (s *Server) RegisterHandler(route string, handler http.Handler) error {
	if handler == nil {
		return errors.New("handler is required")
	}

	if err := registerHandler(s.mux, route, handler); err != nil {
		return err
	}

	s.logger.Debug("registered admission route", slog.String("route", route))

	return nil
}

// registerHandler converts the panic-based http.ServeMux registration API into an error.
func registerHandler(mux *http.ServeMux, route string, handler http.Handler) error {
	var err error

	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("register admission route %q: %v", route, recovered)
			}
		}()

		mux.Handle(route, handler)
	}()

	return err
}

// Start runs the admission HTTPS server until ctx is canceled or the server fails.
// Start blocks so the caller owns the server goroutine and receives lifecycle errors.
func (s *Server) Start(ctx context.Context) error {
	certFile := filepath.Join(s.certsDir, certificateFilename)
	keyFile := filepath.Join(s.certsDir, privateKeyFilename)

	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("load admission TLS key pair: %w", err)
	}

	srv := &http.Server{
		Addr:              net.JoinHostPort("", s.listenPort),
		Handler:           s.mux,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{certificate},
			MinVersion:   tls.VersionTLS12,
		},
	}

	listener, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return fmt.Errorf("listen for admission HTTPS server: %w", err)
	}

	serveErrCh := make(chan error, 1)

	go func() {
		serveErrCh <- srv.ServeTLS(listener, "", "")
	}()

	s.logger.Info("admission server started", slog.String("address", listener.Addr().String()))

	select {
	case err := <-serveErrCh:
		if errors.Is(err, http.ErrServerClosed) && ctx.Err() != nil {
			return nil
		}

		return fmt.Errorf("serve admission HTTPS server: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()

	shutdownErr := srv.Shutdown(shutdownCtx)
	if shutdownErr != nil {
		shutdownErr = fmt.Errorf("shut down admission HTTPS server: %w", shutdownErr)

		if err := srv.Close(); err != nil {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("close admission HTTPS server: %w", err))
		}
	}

	serveErr := <-serveErrCh
	if !errors.Is(serveErr, http.ErrServerClosed) {
		shutdownErr = errors.Join(shutdownErr, fmt.Errorf("serve admission HTTPS server: %w", serveErr))
	}

	if shutdownErr != nil {
		return shutdownErr
	}

	s.logger.Info("admission server stopped")

	return nil
}
