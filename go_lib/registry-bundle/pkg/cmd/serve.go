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
	"log/slog"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/bundle"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/registry"
)

var (
	_ validation.Validatable = ServeConfig{}
)

func Serve(ctx context.Context, logger *slog.Logger, config ServeConfig) (*Server, error) {
	config = NewServeConfig().Merge(config)
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
		logger.WithGroup("bundle"),
		config.BundlePath,
	)
	if err != nil {
		err = fmt.Errorf("load bundle: %w", err)
		return nil, withClose(err)
	}
	server.bundle = bndl

	reg, err := bundle.NewRegistry(
		config.RepoPath,
		bndl,
	)
	if err != nil {
		err = fmt.Errorf("create bundle registry: %w", err)
		return nil, withClose(err)
	}

	regServe, err := registry.Serve(
		logger.WithGroup("serve"),
		config.Registry,
		reg,
	)
	if err != nil {
		err = fmt.Errorf("start registry server: %w", err)
		return nil, withClose(err)
	}
	server.regServe = regServe

	if err := ctx.Err(); err != nil {
		return nil, withClose(err)
	}

	return server, nil
}

type Server struct {
	bundle   *bundle.Bundle
	regServe *registry.Server
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

func NewServeConfig() ServeConfig {
	return ServeConfig{
		RepoPath: "system/deckhouse",
		Registry: registry.NewServeConfig(),
	}
}

type RegistryConfig = registry.ServeConfig

type ServeConfig struct {
	RepoPath   string
	BundlePath string
	Registry   RegistryConfig
}

func (c ServeConfig) Merge(other ServeConfig) ServeConfig {
	ret := ServeConfig{
		RepoPath:   c.RepoPath,
		BundlePath: c.BundlePath,
	}

	if other.RepoPath != "" {
		ret.RepoPath = other.RepoPath
	}

	if other.BundlePath != "" {
		ret.BundlePath = other.BundlePath
	}

	ret.Registry = c.Registry.Merge(other.Registry)
	return ret
}

func (c ServeConfig) Validate() error {
	return validation.ValidateStruct(&c,
		validation.Field(&c.RepoPath, validation.Required),
		validation.Field(&c.BundlePath, validation.Required),
		validation.Field(&c.Registry, validation.Required),
	)
}
