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

package registry

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
)

var (
	_ validation.Validatable = ServeConfig{}
)

func Serve(logger *slog.Logger, config ServeConfig, registry Registry) (*Server, error) {
	config = NewServeConfig().Merge(config)

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

	srv := &http.Server{
		ReadHeaderTimeout: config.ReadHeaderTimeout,
		Handler:           NewRegistryHandler(logger, registry),
	}

	serv := &Server{
		logger:    logger,
		srv:       srv,
		serveDone: make(chan error, 1),
	}

	go func() {
		serv.serveDone <- srv.Serve(ln)
	}()

	serv.logger.Info("serving registry", "address", scheme+"://"+ln.Addr().String())
	return serv, nil
}

type Server struct {
	srv       *http.Server
	serveDone chan error

	logger *slog.Logger
}

func (s *Server) Stop(ctx context.Context) error {
	if s == nil || s.srv == nil {
		return nil
	}

	s.logger.Info("shutting down")
	errShutdown := s.srv.Shutdown(ctx)
	errServe := <-s.serveDone

	s.srv, s.serveDone = nil, nil
	if errors.Is(errServe, http.ErrServerClosed) {
		errServe = nil
	}
	return errors.Join(errShutdown, errServe)
}

func NewServeConfig() ServeConfig {
	return ServeConfig{
		Address:           "localhost:5001",
		ReadHeaderTimeout: 5 * time.Second,
	}
}

type ServeConfig struct {
	Address           string
	TLS               *tls.Config // if provided, enables HTTPS
	ReadHeaderTimeout time.Duration
}

func (c ServeConfig) Validate() error {
	return validation.ValidateStruct(&c,
		validation.Field(&c.Address, validation.Required),
		validation.Field(&c.ReadHeaderTimeout, validation.Required),
	)
}

func (c ServeConfig) Merge(other ServeConfig) ServeConfig {
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
