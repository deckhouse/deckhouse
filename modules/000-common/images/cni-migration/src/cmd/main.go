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
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	networkv1alpha1 "deckhouse.io/cni-migration/api/v1alpha1"
	"deckhouse.io/cni-migration/internal/controller"
	"deckhouse.io/cni-migration/internal/webhook"
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
	flag.StringVar(&mode, "mode", "manager", "Mode to run the application in: 'manager' or 'agent'")

	opts := zap.Options{
		Development: false,
	}
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	config := ctrl.GetConfigOrDie()
	setupLog.Info("Kubernetes connection details", "Host", config.Host)

	if mode == "manager" {
		setupLog.Info("Starting in manager mode (controller + webhook)")

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
			setupLog.Error(err, "unable to create manager controller", "controller", "CNIMigration")
			os.Exit(1)
		}

		// Add Webhook Server as a Runnable to the manager
		if err := mgr.Add(&webhookServerRunnable{
			scheme: scheme,
			log:    setupLog.WithName("webhook"),
		}); err != nil {
			setupLog.Error(err, "unable to add webhook server to manager")
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
		return
	}

	if mode == "agent" {
		setupLog.Info("Starting in agent mode")
		mgr, err := ctrl.NewManager(config, ctrl.Options{
			Scheme:                 scheme,
			Metrics:                metricsserver.Options{BindAddress: metricsAddr},
			HealthProbeBindAddress: probeAddr,
		})
		if err != nil {
			setupLog.Error(err, "unable to start agent manager")
			os.Exit(1)
		}

		if err := (&controller.CNIAgentReconciler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create agent controller", "controller", "CNIAgent")
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

		setupLog.Info("Starting agent")
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			setupLog.Error(err, "problem running agent")
			os.Exit(1)
		}
		return
	}

	setupLog.Error(fmt.Errorf("invalid mode: %s", mode), "supported modes: manager, agent")
	os.Exit(1)
}

// webhookServerRunnable wraps the webhook server to run within the controller manager's context
type webhookServerRunnable struct {
	scheme *runtime.Scheme
	log    logr.Logger
}

func (r *webhookServerRunnable) Start(ctx context.Context) error {
	webhookClient, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: r.scheme})
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

	r.log.Info("Webhook server starting", "port", webhookServerPort)

	// Shutdown the server when context is canceled
	go func() {
		<-ctx.Done()
		r.log.Info("Shutting down webhook server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	return server.ListenAndServeTLS("/etc/tls/tls.crt", "/etc/tls/tls.key")
}
