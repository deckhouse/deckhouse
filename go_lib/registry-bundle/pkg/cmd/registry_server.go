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

package cmd

import (
	"context"
	"errors"
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/bundle"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/log"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/registry"
)

var (
	_ validation.Validatable = ServerConfig{}
	_ validation.Validatable = RegistryConfig{}
	_ validation.Validatable = BundleConfig{}
)

func NewServer(ctx context.Context, config ServerConfig) (*Server, error) {
	config = NewConfig().Merge(config)
	if err := config.Validate(); err != nil {
		return nil, err
	}

	server := &Server{}

	withClose := func(err error) error {
		closeErr := server.Stop(ctx)
		if closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return err
	}

	bndl, err := bundle.New(
		ctx,
		config.Bundle.Logger,
		config.Bundle.BundlePath,
	)
	if err != nil {
		err = fmt.Errorf("load bundle: %w", err)
		return nil, withClose(err)
	}
	server.bundle = bndl

	reg, err := bundle.NewRegistry(
		config.Registry.RepoPath,
		bndl,
	)
	if err != nil {
		err = fmt.Errorf("create bundle registry: %w", err)
		return nil, withClose(err)
	}

	handler := withStatusLogging(
		config.Registry.Logger,
		registry.NewRegistryHandler(
			config.Registry.Logger,
			reg,
		),
	)

	httpServer, err := newHTTPServer(
		config.Registry.Logger,
		config.Registry.HTTP,
		handler,
	)
	if err != nil {
		err = fmt.Errorf("serve bundle registry: %w", err)
		return nil, withClose(err)
	}
	server.regServe = httpServer

	if err := ctx.Err(); err != nil {
		return nil, withClose(err)
	}

	return server, nil
}

type Server struct {
	bundle   *bundle.Bundle
	regServe *HTTPServer
}

func (s *Server) Stop(ctx context.Context) error {
	if s == nil {
		return nil
	}

	var err error

	if s.regServe != nil {
		err = errors.Join(
			err,
			s.regServe.Stop(ctx),
		)
	}

	if s.bundle != nil {
		err = errors.Join(
			err,
			s.bundle.Close(),
		)
	}

	s.regServe = nil
	s.bundle = nil
	return err
}

func NewConfig() ServerConfig {
	return ServerConfig{
		Registry: RegistryConfig{
			RepoPath: "system/deckhouse",
			HTTP:     NewHTTPServerConfig(),
		},
		Bundle: BundleConfig{},
	}
}

type ServerConfig struct {
	Registry RegistryConfig
	Bundle   BundleConfig
}

func (c ServerConfig) Merge(other ServerConfig) ServerConfig {
	ret := ServerConfig{}
	ret.Registry = c.Registry.Merge(other.Registry)
	ret.Bundle = c.Bundle.Merge(other.Bundle)
	return ret
}

func (c ServerConfig) Validate() error {
	return validation.ValidateStruct(&c,
		validation.Field(&c.Registry, validation.Required),
		validation.Field(&c.Bundle, validation.Required),
	)
}

type RegistryConfig struct {
	Logger   log.Logger
	RepoPath string
	HTTP     HTTPServerConfig
}

func (c RegistryConfig) Merge(other RegistryConfig) RegistryConfig {
	ret := c

	if other.Logger != nil {
		ret.Logger = other.Logger
	}

	if other.RepoPath != "" {
		ret.RepoPath = other.RepoPath
	}

	ret.HTTP = c.HTTP.Merge(other.HTTP)
	return ret
}

func (c RegistryConfig) Validate() error {
	return validation.ValidateStruct(&c,
		validation.Field(&c.HTTP, validation.Required),
		validation.Field(&c.Logger, validation.Required),
		validation.Field(&c.RepoPath, validation.Required),
	)
}

type BundleConfig struct {
	Logger     log.Logger
	BundlePath string
}

func (c BundleConfig) Merge(other BundleConfig) BundleConfig {
	ret := c

	if other.Logger != nil {
		ret.Logger = other.Logger
	}

	if other.BundlePath != "" {
		ret.BundlePath = other.BundlePath
	}
	return ret
}

func (c BundleConfig) Validate() error {
	return validation.ValidateStruct(&c,
		validation.Field(&c.Logger, validation.Required),
		validation.Field(&c.BundlePath, validation.Required),
	)
}
