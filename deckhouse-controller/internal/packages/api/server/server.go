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

package server

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/tools/leaderelection"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/api/handlers/root"
	v1 "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/api/handlers/root/v1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/api/server/socket"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/api/server/tcp"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/envconfig"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

const (
	// apiSocketPath is the Unix socket the package runtime API listens on.
	apiSocketPath = "/tmp/deckhouse-debug.socket"

	// apiShutdownTimeout bounds how long shutdown waits for in-flight API requests.
	apiShutdownTimeout = 5 * time.Second
)

// convergence reports whether the controller finished its startup converge.
type convergence interface {
	IsConverged() bool
}

// Server serves the runtime API: the private tree on the Unix socket, the public one over TCP.
type Server struct {
	tcp         *tcp.Server
	socket      *socket.Server
	workers     errgroup.Group
	elector     atomic.Pointer[leaderelection.LeaderElector] // set after Start, read by probe handlers
	metrics     metricsstorage.Storage
	convergence convergence
	logger      *log.Logger
}

// New builds both transports; nothing is bound until Start.
func New(metrics metricsstorage.Storage, convergence convergence, deps v1.Deps, logger *log.Logger) *Server {
	s := &Server{
		workers:     errgroup.Group{},
		metrics:     metrics,
		convergence: convergence,
		logger:      logger.Named("api"),
	}

	rootDeps := root.Deps{
		Provider: s,
	}

	s.socket = socket.NewServer(apiSocketPath, root.NewHandler(v1.NewPrivateHandler(deps), rootDeps), logger)
	s.tcp = tcp.NewServer(envconfig.ListenAddress(), envconfig.ListenPort(), root.NewHandler(v1.NewPublicHandler(deps), rootDeps), logger)

	return s
}

// GetMetrics returns the metrics storage for this server.
func (s *Server) GetMetrics() metricsstorage.Storage {
	return s.metrics
}

// IsConverged reports whether the controller finished its startup converge.
func (s *Server) IsConverged() bool {
	return s.convergence.IsConverged()
}

// GetElector returns the leader elector for this server.
func (s *Server) GetElector() *leaderelection.LeaderElector {
	return s.elector.Load()
}

// SetElector sets the leader elector for this server.
func (s *Server) SetElector(elector *leaderelection.LeaderElector) {
	s.elector.Store(elector)
}

// Start binds the socket and the TCP listener, then watches the serve loops; only a bind failure is returned.
func (s *Server) Start() error {
	if err := s.socket.Start(); err != nil {
		return fmt.Errorf("start socket server: %w", err)
	}

	s.workers.Go(s.socket.Wait)

	if err := s.tcp.Start(); err != nil {
		return fmt.Errorf("start tcp server: %w", err)
	}

	s.workers.Go(s.tcp.Wait)

	go s.watchAPIServers()

	return nil
}

// watchAPIServers reports the first listener that stops on its own. Losing an
// introspection listener does not take the controller down: the packages keep
// converging, only the API is gone until the process restarts.
func (s *Server) watchAPIServers() {
	if err := s.workers.Wait(); err != nil {
		s.logger.Error("api server stopped serving", log.Err(err))
	}
}

// Stop closes both listeners and waits for their serve loops to
// finish; closing the socket unlinks its file.
func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), apiShutdownTimeout)
	defer cancel()
	err := errors.Join(s.socket.Shutdown(ctx), s.tcp.Shutdown(ctx))

	return errors.Join(err, s.workers.Wait())
}
