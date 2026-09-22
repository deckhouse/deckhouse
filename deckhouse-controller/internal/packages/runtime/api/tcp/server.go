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
// Package tcp serves the API over a TCP address. The deckhouse pod runs with
// hostNetwork, so the listener lives in the node's network namespace: every
// process on the node and every hostNetwork pod reaches it, and so does
// `kubectl port-forward`. Loopback is not a trust boundary here — secret-bearing
// endpoints must not be registered on this transport.
package tcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/api"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// ErrNoPort is returned when the server is started without a port.
var ErrNoPort = errors.New("no port configured")

// Server serves the given handler on its TCP address.
type Server struct {
	address string
	port    string
	handler http.Handler
	srv     *http.Server
	wait    func() error

	logger *log.Logger
}

// NewServer creates a server for the given address and port. An empty address
// binds every interface, which publishes the routes cluster-wide — keep it on
// loopback. Nothing is bound until Start.
func NewServer(address, port string, handler http.Handler, logger *log.Logger) *Server {
	return &Server{
		address: address,
		port:    port,
		handler: handler,
		logger:  logger.Named("api-tcp-server"),
	}
}

// Start binds the address and serves the handler on it.
//
// An empty port is refused rather than passed to net.Listen, which would bind an
// arbitrary free port on every interface.
func (s *Server) Start() error {
	if s.port == "" {
		return ErrNoPort
	}

	address := net.JoinHostPort(s.address, s.port)

	srv, wait, err := api.Serve(func() (net.Listener, error) {
		// JoinHostPort, not concatenation: an IPv6 host needs its brackets.
		return net.Listen("tcp", address)
	}, s.handler)
	if err != nil {
		return err
	}

	s.srv, s.wait = srv, wait

	s.logger.Info("server started", slog.String("address", address))

	return nil
}

// Wait blocks until serving ends and reports why: nil after Shutdown, the serve
// error otherwise. Call it once, after Start.
func (s *Server) Wait() error {
	if s.wait == nil {
		return nil
	}

	return s.wait()
}

// Shutdown stops serving and closes the listener. Safe to call when Start was
// never reached, which happens when the other transport failed to bind.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}

	if err := s.srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	return nil
}
