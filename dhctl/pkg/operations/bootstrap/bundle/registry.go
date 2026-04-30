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

package bundle

import (
	"context"
	"errors"
	"fmt"

	"github.com/deckhouse/lib-dhctl/pkg/log"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/bundle"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/serve"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

type RegistryParams struct {
	BundlePath     string
	LoggerProvider log.LoggerProvider
}

func (params RegistryParams) Validate() error {
	if params.BundlePath == "" {
		return fmt.Errorf("bundle path is required")
	}

	if params.LoggerProvider == nil {
		return fmt.Errorf("logger provider is required")
	}

	return nil
}

type StopRegistry func()

func StartRegistry(ctx context.Context, params RegistryParams) (StopRegistry, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	logger := params.LoggerProvider()
	logger.DebugF("Up bundle registry...")

	reg := newRegistry(params.BundlePath)
	if err := reg.start(ctx, logger); err != nil {
		return nil, err
	}

	return func() {
		logger.DebugF("Stopping bundle registry...")
		if err := reg.stop(ctx); err != nil {
			logger.ErrorF("Bundle registry: stopped with error: %s", err.Error())
		} else {
			logger.DebugF("Bundle registry: stopped")
		}
	}, nil
}

func newRegistry(bundlePath string) *Registry {
	return &Registry{
		repoPath:   constant.BundleRepoPath,
		address:    constant.BundleAddressWithPort,
		bundlePath: bundlePath,
	}
}

type Registry struct {
	repoPath   string
	address    string
	bundlePath string

	bundle   *bundle.Bundle
	regServe *serve.RegistryServer
}

func (r *Registry) start(ctx context.Context, logger log.Logger) error {
	withStop := func(err error) error {
		closeErr := r.stop(ctx)
		if closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return err
	}

	bndlLogger := newLogger(logger)
	regSrvLogger := newLogger(logger).
		WithInfoAsDebug().
		WithPrefix("Bundle registry: ")

	bndlLogger.Infof("Loading bundle...")
	bndl, err := bundle.New(ctx, bndlLogger, r.bundlePath)
	if err != nil {
		return withStop(
			fmt.Errorf("load bundle: %w", err),
		)
	}
	r.bundle = bndl

	reg, err := bundle.NewRegistry(r.repoPath, bndl)
	if err != nil {
		return withStop(
			fmt.Errorf("create bundle registry: %w", err),
		)
	}

	serv, err := serve.NewRegistryServer(ctx, regSrvLogger, reg, serve.RegistryServerConfig{
		HTTP: serve.HTTPServerConfig{Address: r.address},
	})
	if err != nil {
		return withStop(
			fmt.Errorf("start bundle registry: %w", err),
		)
	}
	r.regServe = serv

	if err := ctx.Err(); err != nil {
		return withStop(
			fmt.Errorf("start bundle registry: %w", err),
		)
	}
	return nil
}

func (r *Registry) stop(ctx context.Context) error {
	if r == nil {
		return nil
	}

	var errs []error
	if r.regServe != nil {
		errs = append(errs, r.regServe.Stop(ctx))
		r.regServe = nil
	}
	if r.bundle != nil {
		errs = append(errs, r.bundle.Close())
		r.bundle = nil
	}
	return errors.Join(errs...)
}
