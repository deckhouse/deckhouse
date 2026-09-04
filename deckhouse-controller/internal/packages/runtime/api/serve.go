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
	"net"
	"net/http"
	"time"
)

const (
	// readHeaderTimeout bounds how long a client may take to send request headers.
	readHeaderTimeout = 10 * time.Second
	// idleTimeout bounds how long an idle keep-alive connection is held open.
	idleTimeout = 120 * time.Second
)

// Serve binds the listener the given function returns and serves the handler on
// it in the background. The listener is bound synchronously, so a busy address
// is reported here rather than from the goroutine.
//
// The returned wait function blocks until serving ends and reports why: nil
// after a graceful Shutdown, the serve error otherwise. It reads a single-shot
// channel, so exactly one caller waits on it — the runtime, which owns the
// decision of what a dead listener means. One listener is served once: the
// runtime binds it in Run and closes it in Stop.
func Serve(listen func() (net.Listener, error), handler http.Handler) (*http.Server, func() error, error) {
	listener, err := listen()
	if err != nil {
		return nil, nil, fmt.Errorf("listen: %w", err)
	}

	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		// No WriteTimeout on purpose: pprof streams for as long as the caller asks
		// (/debug/pprof/profile?seconds=N), and a write deadline truncates the dump.
	}

	stopped := make(chan error, 1)

	go func() {
		err := srv.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}

		stopped <- err
	}()

	return srv, func() error { return <-stopped }, nil
}
