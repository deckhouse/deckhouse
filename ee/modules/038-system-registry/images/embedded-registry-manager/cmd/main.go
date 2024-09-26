package main

import (
	"context"
	"encoding/json"
	"k8s.io/client-go/tools/clientcmd"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"syscall"
	"time"

	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"embeded-registry-manager/internal/controllers"
	staticpodmanager "embeded-registry-manager/internal/manager"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type managerStatus struct {
	isLeader                bool
	staticPodManagerRunning bool
}

const metricsBindAddressPort = "127.0.0.1:8081"

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(false)))
	status := managerStatus{
		isLeader:                false,
		staticPodManagerRunning: false,
	}
	ctrl.Log.Info("Starting embedded registry manager")

	// Try to load in-cluster configuration
	cfg, err := rest.InClusterConfig()

	// If not in cluster, try to load kubeconfig from home directory
	if err != nil {
		cfg, err = clientcmd.BuildConfigFromFlags("", filepath.Join(homeDir(), ".kube", "config"))
		if err != nil {
			ctrl.Log.Error(err, "Unable to get kubeconfig:")
			os.Exit(1)
		}
	}

	// Create Kubernetes client
	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		ctrl.Log.Error(err, "Unable to create kubernetes client")
		os.Exit(1)
	}

	// Create context with cancel function
	ctx, cancel := context.WithCancel(context.Background())

	// Graceful shutdown on SIGINT and SIGTERM
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		ctrl.Log.Info("Received shutdown signal")
		cancel()
	}()

	// Start static pod manager
	go func() {
		status.staticPodManagerRunning = true
		if err := staticpodmanager.Run(ctx, kubeClient); err != nil {
			ctrl.Log.Error(err, "Failed to run static pod manager")
			status.staticPodManagerRunning = false
			os.Exit(1)
		}

	}()

	// Start health server
	go func() {
		http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			healthHandler(w, &status)
		})
		http.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
			healthHandler(w, &status)
		})

		err := http.ListenAndServe(":8097", nil)
		if err != nil {
			ctrl.Log.Error(err, "Failed to start health server")
			os.Exit(1)
		}
	}()

	// Setup manager
	mgr, err := ctrl.NewManager(cfg, manager.Options{
		Metrics: metricsserver.Options{
			BindAddress: metricsBindAddressPort,
		},
		LeaderElection:          true,
		LeaderElectionID:        "embedded-registry-manager-leader-dev",
		LeaderElectionNamespace: "d8-system",
		GracefulShutdownTimeout: &[]time.Duration{10 * time.Second}[0],
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				"d8-system": {},
			},
		},
	})
	if err != nil {
		ctrl.Log.Error(err, "Unable to set up manager")
		os.Exit(1)
	}

	// Add runnable to manager
	err = mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		status.isLeader = true
		<-ctx.Done()
		status.isLeader = false
		return nil
	}))
	if err != nil {
		ctrl.Log.Error(err, "mgr.Add error:")
		os.Exit(1)
	}

	// Setup registry controller
	if err = (&controllers.RegistryReconciler{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		KubeClient: kubeClient,
		Recorder:   mgr.GetEventRecorderFor("embedded-registry-controller"),
	}).SetupWithManager(mgr, ctx); err != nil {
		ctrl.Log.Error(err, "Unable to create controller:")
		os.Exit(1)
	}

	ctrl.Log.Info("Starting manager")
	if err := mgr.Start(ctx); err != nil {
		ctrl.Log.Error(err, "Unable to start manager:")
		os.Exit(1)
	}
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE")
}

func healthHandler(w http.ResponseWriter, status *managerStatus) {
	response := struct {
		IsLeader                bool `json:"isLeader"`
		StaticPodManagerRunning bool `json:"staticPodManagerRunning"`
	}{
		IsLeader:                status.isLeader,
		StaticPodManagerRunning: status.staticPodManagerRunning,
	}

	w.Header().Set("Content-Type", "application/json")
	if status.staticPodManagerRunning {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}

	json.NewEncoder(w).Encode(response)
}
