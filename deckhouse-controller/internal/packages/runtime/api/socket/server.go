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
// Package socket serves the API over a Unix socket. The socket lives in the
// container's own filesystem with mode 0600, so only this process's user reaches
// it — this is the transport for endpoints carrying package values, rendered
// manifests or hook snapshots.
//
// Example access: curl --unix-socket /path/to/socket http://localhost/endpoint
package socket

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/api"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// socketMode restricts the socket to its owner.
	socketMode = 0o600
	// socketDirMode restricts the socket directory to its owner.
	socketDirMode = 0o700
)

// Server serves the given handler on the Unix socket at its path.
type Server struct {
	socketPath string
	handler    http.Handler
	srv        *http.Server

	logger *log.Logger
}

// NewServer creates a server for the given socket path. Nothing is bound until
// Start.
func NewServer(socketPath string, handler http.Handler, logger *log.Logger) *Server {
	return &Server{
		socketPath: socketPath,
		handler:    handler,
		logger:     logger.Named("api-socket-server"),
	}
}

// Start binds the socket and serves the handler on it.
func (s *Server) Start() error {
	srv, err := api.Serve(func() (net.Listener, error) {
		return listen(s.socketPath)
	}, s.handler, s.logger)
	if err != nil {
		return err
	}

	s.srv = srv

	return nil
}

// Shutdown stops serving and unlinks the socket file. Safe to call when Start
// was never reached, which happens when the other transport failed to bind.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}

	if err := s.srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	return nil
}

// listen binds the socket, replacing a stale file left by a previous run.
//
// The socket is chmod'ed after binding: net.Listen creates it with 0777 &^ umask,
// which would let any user in the container drive the API.
func listen(socketPath string) (net.Listener, error) {
	dir := path.Dir(socketPath)
	if err := os.MkdirAll(dir, socketDirMode); err != nil {
		return nil, fmt.Errorf("create socket dir '%s': %w", dir, err)
	}

	// Clean up a stale socket file from a previous run; bind fails on an existing path.
	if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("remove socket '%s': %w", socketPath, err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("create listener: %w", err)
	}

	if err = os.Chmod(socketPath, socketMode); err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("chmod socket '%s': %w", socketPath, err)
	}

	return listener, nil
}
