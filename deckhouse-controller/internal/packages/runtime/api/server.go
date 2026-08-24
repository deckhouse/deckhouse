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
// Package api provides the shared serve plumbing of the runtime introspection
// API. The transports live in the socket and tcp subpackages and the endpoint
// tree under handlers/; this file holds only what both transports must do
// identically.
package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// readHeaderTimeout bounds how long a client may take to send request headers.
	readHeaderTimeout = 10 * time.Second
	// idleTimeout bounds how long an idle keep-alive connection is held open.
	idleTimeout = 120 * time.Second
)

// Serve binds the listener the given function returns and serves the handler on
// it in the background, returning the server to shut down. The listener is bound
// synchronously, so a busy address is reported here rather than from the
// goroutine.
//
// One listener is served once: the runtime binds it in Run and closes it in
// Stop, so there is no restart to guard against.
func Serve(listen func() (net.Listener, error), handler http.Handler, logger *log.Logger) (*http.Server, error) {
	listener, err := listen()
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	address := listener.Addr().String()
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		// No WriteTimeout on purpose: pprof streams for as long as the caller asks
		// (/debug/pprof/profile?seconds=N), and a write deadline truncates the dump.
	}

	// The goroutine ends when Shutdown closes the listener. A debug listener
	// dying must never take the process with it.
	go func() {
		if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped with error", slog.String("address", address), log.Err(err))
		}
	}()

	logger.Info("server started", slog.String("address", address))

	return srv, nil
}
