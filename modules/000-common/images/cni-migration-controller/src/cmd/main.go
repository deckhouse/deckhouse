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
	"log/slog"
	"os"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	deckhouselog "github.com/deckhouse/deckhouse/pkg/log"
	networkv1alpha1 "deckhouse.io/cni-migration/api/v1alpha1"
	"deckhouse.io/cni-migration/internal/agent"
	"deckhouse.io/cni-migration/internal/manager"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(networkv1alpha1.AddToScheme(scheme))
}

type healthFileRunner struct{}

func (h *healthFileRunner) Start(ctx context.Context) error {
	setupLog.Info("Creating health check file", "path", "/tmp/healthz")
	f, err := os.Create("/tmp/healthz")
	if err != nil {
		return err
	}
	_ = f.Close()

	<-ctx.Done()

	setupLog.Info("Removing health check file", "path", "/tmp/healthz")
	return os.Remove("/tmp/healthz")
}

func main() {
	var mode string
	var migrationName string
	var waitForWebhooks string

	flag.StringVar(&mode, "mode", "manager", "Mode to run the application in: 'manager', 'agent' or 'healthcheck'")
	flag.StringVar(&migrationName, "migration-name", "", "Name of the CNIMigration resource to process")
	flag.StringVar(&waitForWebhooks, "wait-for-webhooks", "", "Comma-separated list of webhooks to wait for deletion")

	flag.Parse()

	// Configure Deckhouse structured logger
	logger := deckhouselog.NewLogger(
		deckhouselog.WithLevel(slog.LevelInfo),
		deckhouselog.WithHandlerType(deckhouselog.JSONHandlerType),
	)
	deckhouselog.SetDefault(logger)

	// Set logger for controller-runtime
	ctrl.SetLogger(logr.FromSlogHandler(logger.Handler()))

	config := ctrl.GetConfigOrDie()
	config.QPS = 20.0
	config.Burst = 100
	setupLog.Info("Kubernetes connection details", "Host", config.Host)

	if mode == "manager" {
		setupLog.Info("Starting in MANAGER mode")

		mgr, err := ctrl.NewManager(config, ctrl.Options{
			Scheme:                 scheme,
			Metrics:                metricsserver.Options{BindAddress: "0"},
			HealthProbeBindAddress: "0",
		})
		if err != nil {
			setupLog.Error(err, "unable to start manager")
			os.Exit(1)
		}

		if err := (&manager.CNIMigrationReconciler{
			Client:          mgr.GetClient(),
			Scheme:          mgr.GetScheme(),
			MigrationName:   migrationName,
			WaitForWebhooks: waitForWebhooks,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create manager controller", "controller", "CNIMigration")
			os.Exit(1)
		}

		// Register a Runnable to manage the health file
		if err := mgr.Add(&healthFileRunner{}); err != nil {
			setupLog.Error(err, "unable to set up health file")
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
		setupLog.Info("Starting in AGENT mode")

		nodeName := os.Getenv("NODE_NAME")
		if nodeName == "" {
			setupLog.Error(nil, "NODE_NAME env var is required for agent mode")
			os.Exit(1)
		}

		mgr, err := ctrl.NewManager(config, ctrl.Options{
			Scheme:                 scheme,
			Metrics:                metricsserver.Options{BindAddress: "0"},
			HealthProbeBindAddress: "0",
			// Filter cache to include only Pods scheduled on this node and the node's CNINodeMigration object.
			Cache: cache.Options{
				ByObject: map[client.Object]cache.ByObject{
					&corev1.Pod{}: {
						Field: fields.OneTermEqualSelector("spec.nodeName", nodeName),
					},
					&networkv1alpha1.CNINodeMigration{}: {
						Field: fields.OneTermEqualSelector("metadata.name", nodeName),
					},
				},
			},
		})
		if err != nil {
			setupLog.Error(err, "unable to start agent manager")
			os.Exit(1)
		}

		if err := (&agent.CNIAgentReconciler{
			Client:        mgr.GetClient(),
			Scheme:        mgr.GetScheme(),
			MigrationName: migrationName,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create agent controller", "controller", "CNIAgent")
			os.Exit(1)
		}

		// Register a Runnable to manage the health file
		if err := mgr.Add(&healthFileRunner{}); err != nil {
			setupLog.Error(err, "unable to set up health file")
			os.Exit(1)
		}

		setupLog.Info("Starting agent")
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			setupLog.Error(err, "problem running agent")
			os.Exit(1)
		}
		return
	}

	if mode == "healthcheck" {
		if _, err := os.Stat("/tmp/healthz"); err == nil {
			os.Exit(0)
		}
		os.Exit(1)
	}

	setupLog.Error(fmt.Errorf("invalid mode: %s", mode), "supported modes: manager, agent, healthcheck")
	os.Exit(1)
}
