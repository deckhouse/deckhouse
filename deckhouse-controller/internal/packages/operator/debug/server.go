/*
Copyright 2025 Flant JSC

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

// Package debug provides a Unix socket-based HTTP server for runtime debugging and inspection.
// The server exposes internal state and allows runtime operations via HTTP endpoints over a local socket.
package debug

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// Server provides a Unix socket HTTP server for debugging endpoints.
// Use Register() to add custom debug endpoints that can be accessed via curl --unix-socket.
type Server struct {
	router chi.Router

	logger *log.Logger
}

// NewServer creates a new debug server with panic recovery middleware.
// The server won't start listening until Start() is called.
func NewServer(logger *log.Logger) *Server {
	router := chi.NewRouter()
	router.Use(middleware.Recoverer) // Recover from panics in handlers

	return &Server{
		router: router,
		logger: logger,
	}
}

// Start creates and binds to a Unix socket, then starts the HTTP server in a goroutine.
// If the socket file already exists, it will be removed and recreated.
//
// Example access: curl --unix-socket /path/to/socket http://localhost/endpoint
func (s *Server) Start(socketPath string) error {
	// Ensure socket directory exists with restricted permissions
	if err := os.MkdirAll(path.Dir(socketPath), 0o700); err != nil {
		return fmt.Errorf("create socket dir '%s': %w", path.Dir(socketPath), err)
	}

	// Clean up stale socket file from previous run
	if _, err := os.Stat(socketPath); err == nil {
		if err = os.Remove(socketPath); err != nil {
			return fmt.Errorf("remove socket '%s': %w", socketPath, err)
		}
	}

	// Create Unix domain socket listener
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("create listener: %w", err)
	}

	// Start HTTP server in background - fatal errors will exit the process
	go func() {
		if err = http.Serve(listener, s.router); err != nil {
			s.logger.Error("failed to debug socket server", log.Err(err))
			os.Exit(1)
		}
	}()

	return nil
}

// Register adds a debug endpoint handler for the specified HTTP method and URL path.
// Currently, supports GET and POST methods. Other methods are silently ignored.
//
// Example: server.Register(http.MethodGet, "/status", statusHandler)
func (s *Server) Register(method string, url string, handler func(http.ResponseWriter, *http.Request)) {
	switch method {
	case http.MethodGet:
		s.router.Get(url, func(writer http.ResponseWriter, request *http.Request) {
			handler(writer, request)
		})
	case http.MethodPost:
		s.router.Post(url, func(writer http.ResponseWriter, request *http.Request) {
			handler(writer, request)
		})
		// Other HTTP methods are not supported and will be ignored
	}
}
