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

package staticpod

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

const (
	healthAddr  = ":8097"
	metricsAddr = "127.0.0.1:8081"
)

type AppSettings struct {
	HostIP       string
	NodeName     string
	PodName      string
	PodNamespace string

	ImageAuth         string
	ImageDistribution string
	ImageMirrorer     string
}

func Run(ctx context.Context, cfg *rest.Config, settings AppSettings) error {
	ctrl.SetLogger(logr.FromSlogHandler(slog.Default().Handler()))
	log := ctrl.Log.WithValues("component", "Application")

	log.Info("Starting")
	defer log.Info("Stopped")

	namespace := "d8-system"

	services := &servicesManager{
		log:      slog.With("component", "Services manager"),
		settings: settings,
	}

	options := ctrl.Options{
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress:  healthAddr,
		GracefulShutdownTimeout: &[]time.Duration{10 * time.Second}[0],
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				namespace: {},
			},
			ByObject: map[client.Object]cache.ByObject{
				&corev1.Pod{}: {
					Namespaces: map[string]cache.Config{
						settings.PodNamespace: {},
					},
				},
			},
		},
	}

	// Set up the manager
	mgr, err := ctrl.NewManager(cfg, options)

	if err != nil {
		return fmt.Errorf("unable to set up manager: %w", err)
	}

	if err = mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up ready check: %w", err)
	}

	servicesCtrl := servicesController{
		Namespace:    namespace,
		Client:       mgr.GetClient(),
		Services:     services,
		NodeName:     settings.NodeName,
		PodName:      settings.PodName,
		PodNamespace: settings.PodNamespace,
	}

	if err := servicesCtrl.SetupWithManager(ctx, mgr); err != nil {
		return fmt.Errorf("unable to create services controller: %w", err)
	}

	// Start the manager
	log.Info("Starting manager")
	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("unable to start manager: %w", err)
	}

	return nil
}
