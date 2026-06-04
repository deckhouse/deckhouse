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

package serve

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/log"
)

var (
	_ validation.Validatable = HTTPServerConfig{}
)

func NewHTTPServer(logger log.Logger, handler http.Handler, config HTTPServerConfig) (*HTTPServer, error) {
	logger.Infof("starting http server")

	config = NewHTTPServerConfig().Merge(config)

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	var (
		ln     net.Listener
		scheme string
		err    error
	)

	if config.TLS != nil {
		scheme = "https"
		ln, err = tls.Listen("tcp", config.Address, config.TLS)
	} else {
		scheme = "http"
		ln, err = net.Listen("tcp", config.Address)
	}

	if err != nil {
		return nil, fmt.Errorf("listen on %s://%s: %w", scheme, config.Address, err)
	}

	handler = withStatusLogging(logger, handler)

	srv := &http.Server{
		ReadHeaderTimeout: config.ReadHeaderTimeout,
		Handler:           handler,
	}

	s := &HTTPServer{
		logger:    logger,
		srv:       srv,
		serveDone: make(chan error, 1),
	}

	go func() {
		s.serveDone <- srv.Serve(ln)
	}()

	logger.Infof("http server listening on %s", scheme+"://"+ln.Addr().String())
	return s, nil
}

type HTTPServer struct {
	srv       *http.Server
	serveDone chan error

	logger log.Logger
}

func (s *HTTPServer) Stop(ctx context.Context) error {
	if s == nil || s.srv == nil {
		return nil
	}

	s.logger.Infof("shutting down http server")

	err := s.srv.Shutdown(ctx)
	serveErr := <-s.serveDone

	if !errors.Is(serveErr, http.ErrServerClosed) {
		err = errors.Join(err, serveErr)
	}

	s.srv, s.serveDone = nil, nil
	return err
}

func NewHTTPServerConfig() HTTPServerConfig {
	return HTTPServerConfig{
		Address:           "localhost:5001",
		ReadHeaderTimeout: 5 * time.Second,
	}
}

type HTTPServerConfig struct {
	Address           string
	TLS               *tls.Config // if provided, enables HTTPS
	ReadHeaderTimeout time.Duration
}

func (c HTTPServerConfig) Validate() error {
	return validation.ValidateStruct(&c,
		validation.Field(&c.Address, validation.Required),
		validation.Field(&c.ReadHeaderTimeout, validation.Required),
	)
}

func (c HTTPServerConfig) Merge(other HTTPServerConfig) HTTPServerConfig {
	ret := c

	if other.Address != "" {
		ret.Address = other.Address
	}

	if other.TLS != nil {
		ret.TLS = other.TLS
	}

	if other.ReadHeaderTimeout != 0 {
		ret.ReadHeaderTimeout = other.ReadHeaderTimeout
	}

	return ret
}

type statusLogging struct {
	http.ResponseWriter
	status int
}

func (sw *statusLogging) WriteHeader(status int) {
	sw.status = status
	sw.ResponseWriter.WriteHeader(status)
}

func withStatusLogging(logger log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusLogging{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		logger.Infof("request method=%s url=%s status=%d", r.Method, r.URL.String(), sw.status)
	})
}
