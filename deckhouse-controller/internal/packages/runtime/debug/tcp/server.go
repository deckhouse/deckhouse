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

// Package tcp serves the debug API over a TCP address. The deckhouse pod runs
// with hostNetwork, so the listener lives in the node's network namespace: every
// process on the node and every hostNetwork pod reaches it, and so does
// `kubectl port-forward`. Loopback is not a trust boundary here — secret-bearing
// endpoints must not be registered on this transport.
package tcp

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/debug"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// ErrNoPort is returned when the server is started without a port.
var ErrNoPort = errors.New("no port configured")

// Server serves the given handler on its TCP address.
type Server struct {
	server  *debug.Server
	address string
	port    string
}

// New creates a server for the given address and port. An empty address binds
// every interface, which publishes the routes cluster-wide — keep it on
// loopback. Nothing is bound until Start.
func New(address, port string, handler http.Handler, logger *log.Logger) *Server {
	return &Server{
		server:  debug.NewServer(handler, logger.Named("debug-tcp-server")),
		address: address,
		port:    port,
	}
}

// Start binds the address and serves the routes on it. The address is bound
// synchronously, so an address already in use is reported here rather than from
// a background goroutine.
//
// An empty port is refused rather than passed to net.Listen, which would bind an
// arbitrary free port on every interface.
func (s *Server) Start() error {
	if s.port == "" {
		return ErrNoPort
	}

	return s.server.Serve(func() (net.Listener, error) {
		// JoinHostPort, not concatenation: an IPv6 host needs its brackets.
		return net.Listen("tcp", net.JoinHostPort(s.address, s.port))
	})
}

// Shutdown stops serving and closes the listener.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
