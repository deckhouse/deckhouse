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
	"context"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/server"
)

func newServeCmd(logger *slog.Logger) *cobra.Command {
	var configGetter server.ConfigGetter

	cmd := &cobra.Command{
		Use:   server.ServeCommand,
		Short: "Serve the validate action over gRPC",
		Long:  "Serve implements the validate action of the dhctl provider validator protocol.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			validator, err := server.Start(
				configGetter().Merge(server.Config{Logger: logger}),
				server.NewValidateService(Validator{}),
			)

			if err != nil {
				return fmt.Errorf("start validator: %w", err)
			}

			addr := validator.Addr()
			if addr != nil {
				logger.Info("Serve validator", "address", addr.String())
			} else {
				logger.Info("Serve validator")
			}

			<-ctx.Done()
			reason := ctx.Err()
			if reason == context.Canceled {
				logger.Info("shutting down", "reason", "signal")
			} else {
				logger.Info("shutting down", "reason", reason)
			}

			if err := validator.Stop(); err != nil {
				return fmt.Errorf("stop validator: %w", err)
			}

			return nil
		},
	}

	configGetter = server.ConfigGetterFromFlags(cmd.Flags())
	return cmd
}
