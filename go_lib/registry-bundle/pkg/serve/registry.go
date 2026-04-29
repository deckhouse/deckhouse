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
	"errors"
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/log"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/registry"
)

var (
	_ validation.Validatable = RegistryServerConfig{}
)

func NewRegistryServer(ctx context.Context, logger log.Logger, reg registry.Registry, config RegistryServerConfig) (*RegistryServer, error) {
	logger.Infof("starting registry server")

	config = NewRegistryServerConfig().Merge(config)
	if err := config.Validate(); err != nil {
		return nil, err
	}

	server := &RegistryServer{logger: logger}

	withClose := func(err error) error {
		closeErr := server.Stop(ctx)
		if closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return err
	}

	handler := registry.NewRegistryHandler(logger, reg)

	httpServer, err := NewHTTPServer(logger, handler, config.HTTP)
	if err != nil {
		err = fmt.Errorf("serve registry: %w", err)
		return nil, withClose(err)
	}

	server.serv = httpServer

	if err := ctx.Err(); err != nil {
		return nil, withClose(err)
	}
	return server, nil
}

type RegistryServer struct {
	serv   *HTTPServer
	logger log.Logger
}

func (s *RegistryServer) Stop(ctx context.Context) error {
	if s == nil {
		return nil
	}

	s.logger.Infof("shutting down registry server")
	err := s.serv.Stop(ctx)

	s.serv = nil
	return err
}

func NewRegistryServerConfig() RegistryServerConfig {
	return RegistryServerConfig{
		HTTP: NewHTTPServerConfig(),
	}
}

type RegistryServerConfig struct {
	HTTP HTTPServerConfig
}

func (c RegistryServerConfig) Merge(other RegistryServerConfig) RegistryServerConfig {
	ret := c
	ret.HTTP = c.HTTP.Merge(other.HTTP)
	return ret
}

func (c RegistryServerConfig) Validate() error {
	return validation.ValidateStruct(&c,
		validation.Field(&c.HTTP, validation.Required),
	)
}
