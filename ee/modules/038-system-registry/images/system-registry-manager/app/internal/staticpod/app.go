/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package staticpod

import (
	"context"
	"fmt"
	"log/slog"

	"golang.org/x/sync/errgroup"
)

func Run(ctx context.Context, hostIP, nodeName string) error {
	log := slog.
		With("component", "Application")

	log.Info("Starting")
	defer log.Info("Stopped")

	services := &servicesManager{
		log:      slog.With("component", "Services manager"),
		hostIP:   hostIP,
		nodeName: nodeName,
	}

	api := &apiServer{
		log:      slog.With("component", "HTTP API"),
		services: services,
	}

	workers, workersCtx := errgroup.WithContext(ctx)

	workers.Go(func() error {
		if err := api.Run(workersCtx); err != nil && ctx.Err() == nil {
			return fmt.Errorf("HTTP API error: %w", err)
		}
		return nil
	})

	workers.Go(func() error {
		if err := runHealthServer(workersCtx); err != nil {
			return fmt.Errorf("HealthServer error: %w", err)
		}
		return nil
	})

	<-ctx.Done()

	log.Info("Waiting for processes done")
	if err := workers.Wait(); err != nil {
		return fmt.Errorf("workers error: %w", err)
	}

	return nil
}
