/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

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

			logger.Info("Serve validator")

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
