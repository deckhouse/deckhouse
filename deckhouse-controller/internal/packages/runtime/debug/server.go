// Copyright 2026 Flant JSC
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

// Package debug provides the runtime introspection API: a server serving one
// listener, the endpoint handlers under handlers/, and a client for the Unix
// socket transport. The transports live in the socket and tcp subpackages, one
// server type each, and each hands its own router to a Server.
package debug

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// readHeaderTimeout bounds how long a client may take to send request headers.
	readHeaderTimeout = 10 * time.Second
	// idleTimeout bounds how long an idle keep-alive connection is held open.
	idleTimeout = 120 * time.Second
)

// ErrAlreadyStarted is returned when a server is already serving.
var ErrAlreadyStarted = errors.New("server already started")

// Server serves a handler on one listener. The socket and tcp packages hold it
// as a field and expose their own Start, which binds their listener.
type Server struct {
	handler http.Handler

	mu      sync.Mutex
	started bool
	srv     *http.Server

	wg sync.WaitGroup

	logger *log.Logger
}

// NewServer creates a server for the given handler. No listener is bound until
// Serve is called.
func NewServer(handler http.Handler, logger *log.Logger) *Server {
	return &Server{
		handler: handler,
		logger:  logger,
	}
}

// Serve binds the listener the given function returns and serves the handler on
// it; serving itself happens in the background.
//
// Binding happens under the lock, so a second call cannot touch the transport's
// resources — for the socket that would unlink the live socket file.
func (s *Server) Serve(listen func() (net.Listener, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return ErrAlreadyStarted
	}

	listener, err := listen()
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	// The goroutine below reads only these locals: Shutdown nils the server.
	address := listener.Addr().String()
	srv := &http.Server{
		Handler:           s.handler,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		// No WriteTimeout on purpose: pprof streams for as long as the caller asks
		// (/debug/pprof/profile?seconds=N), and a write deadline truncates the dump.
	}

	s.srv = srv
	s.started = true

	// A debug listener dying must never take the process with it.
	s.wg.Go(func() {
		if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("server stopped with error", slog.String("address", address), log.Err(err))
		}
	})

	s.logger.Info("server started", slog.String("address", address))

	return nil
}

// Shutdown stops serving and waits for in-flight requests to drain or ctx to
// expire. Closing a Unix listener unlinks its socket file. Safe to call when the
// server never served or after a previous Shutdown.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	srv := s.srv
	s.srv = nil
	s.started = false
	s.mu.Unlock()

	if srv == nil {
		return nil
	}

	err := srv.Shutdown(ctx)

	// Shutdown closes the listener before returning, so Serve has already exited.
	s.wg.Wait()

	if err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	return nil
}
