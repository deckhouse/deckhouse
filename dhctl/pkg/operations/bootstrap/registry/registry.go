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

package registry

import (
	"context"
	"errors"
	"fmt"

	"github.com/deckhouse/lib-dhctl/pkg/log"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/bundle"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/serve"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

// Params holds dependencies required to start the bundle registry.
type Params struct {
	Logger             log.Logger
	ConfigProvider     ConfigProvider
	BundlePathProvider BundlePathProvider
}

func (params Params) Validate() error {
	if params.Logger == nil {
		return fmt.Errorf("internal error: logger is required")
	}

	if params.ConfigProvider == nil {
		return fmt.Errorf("internal error: registry config provider is required")
	}

	if params.BundlePathProvider == nil {
		return fmt.Errorf("internal error: registry bundle path provider is required")
	}
	return nil
}

// Init starts a local registry when the registry mode is Local.
// Returns a Stop function to gracefully shut down the registry,
// or a no-op function if the registry was not started.
func Init(ctx context.Context, params Params) (Stop, error) {
	nop := func() {}

	if err := params.Validate(); err != nil {
		return nop, err
	}

	isLocal, err := params.ConfigProvider.IsLocal()
	if err != nil {
		return nop, err
	}
	if !isLocal {
		return nop, nil
	}

	bundlePath, err := params.BundlePathProvider()
	if err != nil {
		return nop, err
	}

	logger := params.Logger
	logger.DebugF("Up bundle registry...")

	reg := newRegistry(bundlePath)
	if err = reg.start(ctx, logger); err != nil {
		return nop, fmt.Errorf("start bundle registry: %w", err)
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

func InitWithCLIOptions(ctx context.Context, logger log.Logger, opts *options.Options) (Stop, error) {
	nop := func() {}

	if logger == nil {
		return nop, errors.New("internal error: logger is required")
	}
	if opts == nil {
		return nop, errors.New("internal error: options are required")
	}

	configProvider, err := config.RegistryConfigProvider(func() ([]string, error) {
		return config.FetchDocuments(opts.Global.ConfigPaths)
	})
	if err != nil {
		return nop, err
	}

	bundlePathProvider := func() (string, error) {
		imgBundlePath := opts.Global.ImgBundlePath
		if imgBundlePath == "" {
			return "", errors.New("--img-bundle-path is required in Local registry mode, please specify the flag")
		}
		return imgBundlePath, nil
	}

	return Init(ctx, Params{
		Logger:             logger,
		ConfigProvider:     configProvider,
		BundlePathProvider: bundlePathProvider,
	})
}

// newRegistry creates a Registry pre-configured with the bundle-specific address and repo path.
func newRegistry(bundlePath string) *registry {
	return &registry{
		repoPath:   constant.BundleRepoPath,
		address:    constant.BundleAddressWithPort,
		bundlePath: bundlePath,
	}
}

// registry wraps the OCI bundle and its HTTP registry server lifecycle.
type registry struct {
	repoPath   string
	address    string
	bundlePath string

	bundle   *bundle.Bundle
	regServe *serve.RegistryServer
}

func (r *registry) start(ctx context.Context, logger log.Logger) error {
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

func (r *registry) stop(ctx context.Context) error {
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
