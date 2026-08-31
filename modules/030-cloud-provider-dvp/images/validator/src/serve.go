// Copyright 2025 Flant JSC
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

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/server"
)

type serveConfig struct {
	network string
	address string
}

func newServeCmd() *cobra.Command {
	cfg := serveConfig{}

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the validate action over gRPC",
		Long: `Serve implements the validate action of the dhctl provider validator protocol.

dhctl starts this command with the address it has chosen, calls it once and stops it
with SIGTERM. There is no default address: the caller picks one per run.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			validator, err := server.Start(
				server.Config{Network: cfg.network, Address: cfg.address},
				server.NewValidateService(dvpValidator{}),
			)
			if err != nil {
				return fmt.Errorf("start validator: %w", err)
			}

			<-cmd.Context().Done()

			if err := validator.Stop(); err != nil {
				return fmt.Errorf("stop validator: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&cfg.address, "address", "",
		"address to serve on: host:port, or a socket path when --network=unix")
	cmd.Flags().StringVar(&cfg.network, "network", server.DefaultNetwork,
		"network to serve on: unix or tcp")

	return cmd
}
