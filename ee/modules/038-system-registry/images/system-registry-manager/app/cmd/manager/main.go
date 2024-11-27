/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	dlog "github.com/deckhouse/deckhouse/pkg/log"

	"embeded-registry-manager/internal/controllers/registry_controller"
	"embeded-registry-manager/internal/utils/k8s"
)

const (
	metricsBindAddressPort = "127.0.0.1:8081"
	healthListenAddr       = ":8097"
	namespace              = k8s.RegistryNamespace
)

var logHandler slog.Handler = dlog.Default().Handler()

func main() {
	ctrl.SetLogger(logr.FromSlogHandler(logHandler))
	log := ctrl.Log.WithValues("component", "main")

	log.Info("Starting embedded registry manager")

	// Load Kubernetes configuration
	cfg, err := rest.InClusterConfig()
	if err != nil {
		log.Error(err, "Unable to get kubeconfig")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()

	context.AfterFunc(ctx, func() {
		log.Info("Received shutdown signal")
	})

	// Set up and start manager
	err = setupAndStartManager(ctx, cfg)
	if err != nil {
		ctrl.Log.Error(err, "Failed to start the embedded registry manager")
	}

	log.Info("Bye!")

	if err != nil {
		os.Exit(1)
	}
}

// setupAndStartManager sets up the manager, adds components, and starts the manager
func setupAndStartManager(ctx context.Context, cfg *rest.Config) error {
	// Set up the manager with leader election and other options
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Metrics: metricsserver.Options{
			BindAddress: metricsBindAddressPort,
		},
		HealthProbeBindAddress:  healthListenAddr,
		LeaderElection:          true,
		LeaderElectionID:        "embedded-registry-manager-leader",
		LeaderElectionNamespace: "d8-system",
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

	nodeController := registry_controller.NodeController{
		Client:    mgr.GetClient(),
		Namespace: namespace,
	}

	if err := nodeController.SetupWithManager(ctx, mgr); err != nil {
		return fmt.Errorf("unable to create node controller: %w", err)
	}

	stateController := registry_controller.StateController{
		Client:    mgr.GetClient(),
		Namespace: namespace,
	}

	if err := stateController.SetupWithManager(ctx, mgr); err != nil {
		return fmt.Errorf("unable to create state controller: %w", err)
	}

	// Start the manager
	ctrl.Log.Info("Starting manager")

	/*
		We use leader election, so program must be terminated after exit from this function^
		otherwise some tasks, which must be runned on leader only will continue work
	*/
	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("unable to start manager: %w", err)
	}

	return nil
}
