/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package staticpod

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

const (
	healthAddr  = ":8097"
	metricsAddr = "127.0.0.1:8081"
)

func Run(ctx context.Context, cfg *rest.Config, hostIP, nodeName string) error {
	ctrl.SetLogger(logr.FromSlogHandler(slog.Default().Handler()))
	log := ctrl.Log.WithValues("component", "Application")

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

	// Set up the manager with leader election and other options
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress:  healthAddr,
		GracefulShutdownTimeout: &[]time.Duration{10 * time.Second}[0],
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				"d8-system": {},
			},
		},
	})

	if err != nil {
		return fmt.Errorf("unable to set up manager: %w", err)
	}

	if err = mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up ready check: %w", err)
	}

	mgr.Add(manager.RunnableFunc(api.Run))

	// Start the manager
	log.Info("Starting manager")
	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("unable to start manager: %w", err)
	}

	return nil
}
