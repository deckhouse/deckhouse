/*
Copyright 2026 Flant JSC

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

// Package server runs the registry-agent's TLS HTTP server.
package server

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

// Server serves an http.Handler over TLS, loading the certificate from disk on
// each TLS handshake so rotated certs are picked up without a restart.
type Server struct {
	addr     string
	certFile string
	keyFile  string
	handler  http.Handler
	bound    atomic.Pointer[string]
}

// New builds a Server. addr is host:port (":5001" in production); certFile/keyFile
// are the TLS material; h is the handler to serve.
func New(addr, certFile, keyFile string, h http.Handler) *Server {
	return &Server{addr: addr, certFile: certFile, keyFile: keyFile, handler: h}
}

// Addr returns the actually-bound address (useful when addr uses :0 in tests),
// or "" before Start binds.
func (s *Server) Addr() string {
	if p := s.bound.Load(); p != nil {
		return *p
	}
	return ""
}

// Start serves until ctx is cancelled, then shuts down gracefully.
func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	bound := ln.Addr().String()
	s.bound.Store(&bound)

	srv := &http.Server{
		Handler:           s.handler,
		ReadHeaderTimeout: 10 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion:     tls.VersionTLS12,
			GetCertificate: s.getCertificate,
		},
	}

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	err = srv.ServeTLS(ln, "", "")
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// getCertificate loads the cert/key from disk per handshake (cheap, picks up
// rotation).
func (s *Server) getCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(s.certFile, s.keyFile)
	if err != nil {
		return nil, err
	}
	return &cert, nil
}
