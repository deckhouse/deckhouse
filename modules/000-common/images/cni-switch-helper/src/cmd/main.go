/*
Copyright 2025 Flant JSC

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
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	networkv1alpha1 "deckhouse.io/cni-switch-helper/api/v1alpha1"
	"deckhouse.io/cni-switch-helper/internal/controller"
	"deckhouse.io/cni-switch-helper/internal/webhook"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(networkv1alpha1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var probeAddr string
	var mode string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metrics endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&mode, "mode", "controller", "Mode to run the application in: 'controller' or 'webhook'")

	opts := zap.Options{
		Development: false,
	}
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	config := ctrl.GetConfigOrDie()
	setupLog.Info("Kubernetes connection details",
		"Host", config.Host,
		"KUBERNETES_SERVICE_HOST", os.Getenv("KUBERNETES_SERVICE_HOST"),
		"KUBERNETES_SERVICE_PORT", os.Getenv("KUBERNETES_SERVICE_PORT"))

	if mode == "webhook" {
		setupLog.Info("Starting in webhook mode")
		if err := startWebhookServer(probeAddr); err != nil {
			setupLog.Error(err, "problem running webhook server")
			os.Exit(1)
		}
		return
	}

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := (&controller.CNIMigrationReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "CNIMigration")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("Starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func newHealthzHandler(checker healthz.Checker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := checker(r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}

func startWebhookServer(probeAddr string) error {
	var eg errgroup.Group

	// Goroutine for the health probe server
	eg.Go(func() error {
		healthMux := http.NewServeMux()
		healthMux.HandleFunc("/healthz", newHealthzHandler(healthz.Ping))
		healthMux.HandleFunc("/readyz", newHealthzHandler(healthz.Ping))
		setupLog.Info("Starting health probe server", "addr", probeAddr)
		return http.ListenAndServe(probeAddr, healthMux)
	})

	// Goroutine for the main webhook server
	eg.Go(func() error {
		webhookClient, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
		if err != nil {
			return fmt.Errorf("creating webhook client: %w", err)
		}

		podAnnotator := &webhook.PodAnnotator{
			Client: webhookClient,
		}

		mux := http.NewServeMux()
		mux.HandleFunc("/mutate-pod", podAnnotator.Handle)

		const webhookServerPort = 42443
		server := &http.Server{
			Addr:         fmt.Sprintf(":%d", webhookServerPort),
			Handler:      mux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		}
		setupLog.Info("Webhook server starting", "port", webhookServerPort)
		return server.ListenAndServeTLS("/etc/tls/tls.crt", "/etc/tls/tls.key")
	})

	// Wait for either server to return an error
	return eg.Wait()
}
