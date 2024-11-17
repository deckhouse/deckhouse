/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"embeded-registry-manager/internal/controllers/registry_controller"
	staticpodmanager "embeded-registry-manager/internal/static-pod"
	httpclient "embeded-registry-manager/internal/utils/http_client"
)

const (
	metricsBindAddressPort = "127.0.0.1:8081"
	healthListenAddr       = ":8097"
)

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	log := ctrl.Log.WithValues("component", "main")

	log.Info("Starting embedded registry manager")

	// Load Kubernetes configuration
	cfg, err := rest.InClusterConfig()
	if err != nil {
		log.Error(err, "Unable to get kubeconfig")
		os.Exit(1)
	}

	// Create Kubernetes client
	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Error(err, "Unable to create Kubernetes client")
		os.Exit(1)
	}

	// Create custom HTTP client
	httpClient, err := httpclient.NewDefaultHttpClient()
	if err != nil {
		ctrl.Log.Error(err, "Unable to create HTTP client")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()

	context.AfterFunc(ctx, func() {
		ctrl.Log.Info("Received shutdown signal")
	})

	ctx, ctxCancel := context.WithCancel(ctx)
	var wg sync.WaitGroup

	// Start static pod manager
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer ctxCancel()
		defer log.Info("Static pod manager done")

		log.Info("Starting static pod manager")

		if err := staticpodmanager.Run(ctx); err != nil {
			log.Error(err, "Static pod manager error")
		}
	}()

	// Set up and start manager
	err = setupAndStartManager(ctx, cfg, kubeClient, httpClient)

	if err != nil {
		ctrl.Log.Error(err, "Failed to start the embedded registry manager")
	}

	log.Info("Waiting for background operations")

	ctxCancel()
	wg.Wait()

	log.Info("Bye!")

	if err != nil {
		os.Exit(1)
	}
}

// setupAndStartManager sets up the manager, adds components, and starts the manager
func setupAndStartManager(ctx context.Context, cfg *rest.Config, kubeClient *kubernetes.Clientset, httpClient *httpclient.Client) error {
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

	// Create a new instance of RegistryReconciler
	reconciler := registry_controller.RegistryReconciler{
		Client:     mgr.GetClient(),
		APIReader:  mgr.GetAPIReader(),
		Scheme:     mgr.GetScheme(),
		KubeClient: kubeClient,
		Recorder:   mgr.GetEventRecorderFor("embedded-registry-controller"),
		HttpClient: httpClient,
	}

	// Set up the controller with the manager
	if err := reconciler.SetupWithManager(mgr, ctx); err != nil {
		return fmt.Errorf("unable to create controller: %w", err)
	}

	// Add leader status update runnable
	err = mgr.Add(leaderRunnableFunc(func(ctx context.Context) error {
		// Call SecretsStartupCheckCreate with the existing reconciler instance
		if err := reconciler.SecretsStartupCheckCreate(ctx); err != nil {
			return fmt.Errorf("failed to initialize secrets: %w", err)
		}

		// Wait until the context is done to handle graceful shutdown
		<-ctx.Done()
		return nil
	}))

	if err != nil {
		return fmt.Errorf("unable to add leader runnable: %w", err)
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
