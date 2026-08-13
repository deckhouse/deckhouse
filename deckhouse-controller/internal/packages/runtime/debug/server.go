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

// Package debug provides an HTTP server and client for runtime introspection.
// The server serves a Unix socket and a TCP address simultaneously; both
// listeners share one router, so every registered endpoint is reachable through
// either transport.
package debug

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// socketMode restricts the debug socket to its owner.
	socketMode = 0o600
	// socketDirMode restricts the debug socket directory to its owner.
	socketDirMode = 0o700

	// readHeaderTimeout bounds how long a client may take to send request headers.
	readHeaderTimeout = 10 * time.Second
	// idleTimeout bounds how long an idle keep-alive connection is held open.
	idleTimeout = 120 * time.Second
)

var (
	// ErrAlreadyStarted is returned when Start is called on a running server.
	ErrAlreadyStarted = errors.New("server already started")
	// ErrNoListeners is returned when neither a socket path nor a port is configured.
	ErrNoListeners = errors.New("no listeners configured")
	// ErrUnsupportedMethod is returned when Register is called with a method chi cannot route.
	ErrUnsupportedMethod = errors.New("unsupported http method")
)

// supportedMethods are the HTTP methods Register accepts; chi panics on anything else.
var supportedMethods = map[string]struct{}{
	http.MethodGet:     {},
	http.MethodHead:    {},
	http.MethodPost:    {},
	http.MethodPut:     {},
	http.MethodPatch:   {},
	http.MethodDelete:  {},
	http.MethodOptions: {},
}

// Config declares the debug server's listeners; at least one must be set.
type Config struct {
	// SocketPath is the Unix socket to bind; empty disables the socket listener.
	SocketPath string
	// Address is the TCP host to bind; empty binds every interface when Port is set.
	Address string
	// Port is the TCP port to bind; empty disables the TCP listener.
	Port string
}

// Server serves debug endpoints over a Unix socket and a TCP address at the
// same time. Both listeners share one chi router, so a route registered once is
// reachable through either transport.
//
// Register every endpoint before calling Start: chi builds its routing tree on
// registration and does not tolerate writes while requests are being routed.
type Server struct {
	cfg    Config
	router chi.Router

	mu      sync.Mutex
	started bool
	servers []*http.Server
	addrs   []string

	wg sync.WaitGroup

	logger *log.Logger
}

// NewServer creates a debug server with panic recovery, pprof and route discovery.
// No listener is bound until Start is called.
func NewServer(cfg Config, logger *log.Logger) *Server {
	router := chi.NewRouter()
	router.Use(middleware.Recoverer) // Recover from panics in handlers

	router.Mount("/debug", middleware.Profiler())

	s := &Server{
		cfg:    cfg,
		router: router,
		logger: logger.Named("debug-server"),
	}

	s.registerDiscovery()

	return s
}

// registerDiscovery exposes a plain-text listing of every registered route.
// The pprof subtree is collapsed into a single line instead of enumerated.
func (s *Server) registerDiscovery() {
	s.router.Get("/discovery", func(writer http.ResponseWriter, _ *http.Request) {
		buf := bytes.NewBuffer(nil)

		walkFn := func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
			if strings.HasPrefix(route, "/debug/") {
				return nil
			}

			_, _ = fmt.Fprintf(buf, "%s %s\n", method, route)

			return nil
		}

		if err := chi.Walk(s.router, walkFn); err != nil {
			http.Error(writer, fmt.Sprintf("walk routes: %v", err), http.StatusInternalServerError)
			return
		}

		buf.WriteString("GET /debug/pprof/*\n")

		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(buf.Bytes())
	})
}

// Start binds every configured listener and serves the router on each of them.
// Listeners are bound synchronously, so an address already in use is reported
// here rather than from a background goroutine; serving itself runs in the
// background. A partial failure closes the listeners already bound.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return ErrAlreadyStarted
	}

	listeners, err := s.listen()
	if err != nil {
		return err
	}

	for _, listener := range listeners {
		s.serve(listener)
	}

	s.started = true

	s.logger.Info("debug server started", slog.Any("addresses", s.addrs))

	return nil
}

// listen binds every configured listener, closing the ones already bound on failure.
func (s *Server) listen() ([]net.Listener, error) {
	listeners := make([]net.Listener, 0, 2)

	closeAll := func() {
		for _, listener := range listeners {
			_ = listener.Close()
		}
	}

	if s.cfg.SocketPath != "" {
		listener, err := listenSocket(s.cfg.SocketPath)
		if err != nil {
			return nil, fmt.Errorf("listen on socket: %w", err)
		}

		listeners = append(listeners, listener)
	}

	if s.cfg.Port != "" {
		// JoinHostPort, not concatenation: an IPv6 host needs its brackets.
		address := net.JoinHostPort(s.cfg.Address, s.cfg.Port)

		listener, err := net.Listen("tcp", address)
		if err != nil {
			closeAll()
			return nil, fmt.Errorf("listen on '%s': %w", address, err)
		}

		listeners = append(listeners, listener)
	}

	if len(listeners) == 0 {
		return nil, ErrNoListeners
	}

	return listeners, nil
}

// listenSocket binds a Unix socket, replacing a stale file left by a previous run.
//
// The socket is chmod'ed after binding: net.Listen creates it with 0777 &^ umask,
// which would let any user in the container drive the debug API.
//
// Example access: curl --unix-socket /path/to/socket http://localhost/endpoint
func listenSocket(socketPath string) (net.Listener, error) {
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

// serve runs the router on the listener and tracks the server for shutdown.
// Callers must hold s.mu.
func (s *Server) serve(listener net.Listener) {
	address := listener.Addr().String()

	srv := &http.Server{
		Handler:           s.router,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		// No WriteTimeout on purpose: pprof streams for as long as the caller asks
		// (/debug/pprof/profile?seconds=N), and a write deadline truncates the dump.
	}

	s.servers = append(s.servers, srv)
	s.addrs = append(s.addrs, address)

	// A debug listener dying must never take the process with it.
	s.wg.Go(func() {
		if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("debug server stopped with error", slog.String("address", address), log.Err(err))
		}
	})
}

// Stop gracefully shuts every listener down and waits for in-flight requests to
// drain or ctx to expire. Closing the Unix listener unlinks its socket file.
// Safe to call when Start was never run or after a previous Stop.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	servers := s.servers
	s.servers = nil
	s.addrs = nil
	s.started = false
	s.mu.Unlock()

	errs := make([]error, 0, len(servers))

	for _, srv := range servers {
		if err := srv.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	// Shutdown closes the listeners before returning, so Serve has already exited.
	s.wg.Wait()

	return errors.Join(errs...)
}

// Addrs returns the addresses the server is bound to, empty when it is not running.
func (s *Server) Addrs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]string(nil), s.addrs...)
}

// Register adds a debug endpoint for the given HTTP method and path.
//
// It must be called before Start: chi builds its routing tree on registration
// and racing that against a live listener corrupts routing. Registering after
// Start, or with a method chi cannot route, returns an error and adds nothing.
//
// Example: server.Register(http.MethodGet, "/status", statusHandler)
func (s *Server) Register(method string, url string, handler http.HandlerFunc) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return fmt.Errorf("register '%s %s': %w", method, url, ErrAlreadyStarted)
	}

	if _, ok := supportedMethods[strings.ToUpper(method)]; !ok {
		return fmt.Errorf("register '%s %s': %w", method, url, ErrUnsupportedMethod)
	}

	s.router.MethodFunc(method, url, handler)

	return nil
}
