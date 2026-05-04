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

package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/spf13/cobra"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/bundle"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/log"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/serve"
)

var (
	_ validation.Validatable = serveConfig{}
)

func newServeCmd(logger log.Logger) *cobra.Command {
	cfg := serveConfig{}

	cmd := &cobra.Command{
		Use:   "serve <bundle-path>",
		Short: "Run the OCI distribution registry server from Deckhouse bundle",
		Long: `Serve implements the OCI Distribution Spec and Docker Registry HTTP API V2.
Data is read from Deckhouse bundle archives on disk.

bundle-path is the directory containing OCI bundle archives (*.tar chunks or whole .tar).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("validate: %w", err)
			}

			tlsCfg, err := cfg.loadTLSConfig()
			if err != nil {
				return fmt.Errorf("load TLS configuration: %w", err)
			}

			bundlePath := args[0]
			bndl, err := bundle.New(ctx, logger, bundlePath)
			if err != nil {
				return fmt.Errorf("load bundle from %q: %w", bundlePath, err)
			}

			defer func() {
				if err := bndl.Close(); err != nil {
					logger.Errorf("close bundle error: %s", err.Error())
				}
			}()

			reg, err := bundle.NewRegistry(cfg.rootRepo, bndl)
			if err != nil {
				err = fmt.Errorf("create bundle registry: %w", err)
				return err
			}

			sCfg := serve.RegistryServerConfig{
				HTTP: serve.HTTPServerConfig{
					Address: cfg.address,
					TLS:     tlsCfg,
				},
			}

			server, err := serve.NewRegistryServer(ctx, logger, reg, sCfg)
			if err != nil {
				return err
			}

			<-ctx.Done()
			reason := ctx.Err()
			if errors.Is(reason, context.Canceled) {
				logger.Infof("shutting down, reason signal")
			} else {
				logger.Infof("shutting down, reason %s", reason)
			}

			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer shutdownCancel()
			return server.Stop(shutdownCtx)
		},
	}

	cfg.setFlags(cmd)
	return cmd
}

type serveConfig struct {
	address  string
	rootRepo string
	tlsCert  string
	tlsKey   string
}

func (v *serveConfig) setFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVarP(&v.address, "address", "a", "localhost:5001", "TCP listen address (host:port)")
	f.StringVarP(&v.rootRepo, "root-repo", "r", "system/deckhouse", "virtual registry path for the merged bundle repo")
	f.StringVar(&v.tlsCert, "tls-cert", "", "TLS certificate file (requires --tls-key)")
	f.StringVar(&v.tlsKey, "tls-key", "", "TLS private key file (requires --tls-cert)")
}

func (v serveConfig) Validate() error {
	return validation.ValidateStruct(&v,
		validation.Field(&v.tlsCert,
			validation.When(v.tlsKey != "", validation.Required.Error("tls-cert is required when tls-key is provided")),
		),
		validation.Field(&v.tlsKey,
			validation.When(v.tlsCert != "", validation.Required.Error("tls-key is required when tls-cert is provided")),
		),
	)
}

func (v *serveConfig) loadTLSConfig() (*tls.Config, error) {
	if v.tlsCert == "" {
		return nil, nil
	}

	certPEM, err := os.ReadFile(v.tlsCert)
	if err != nil {
		return nil, fmt.Errorf("read certificate file %s: %w", v.tlsCert, err)
	}

	keyPEM, err := os.ReadFile(v.tlsKey)
	if err != nil {
		return nil, fmt.Errorf("read key file %s: %w", v.tlsKey, err)
	}

	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("load X509 key pair: %w", err)
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{certificate},
	}

	return config, nil
}
