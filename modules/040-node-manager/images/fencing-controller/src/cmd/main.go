/*
Copyright 2026 Flant JSC

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
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/common"
	"fencing-controller/internal/config"
	"fencing-controller/internal/controllers/fencingfailednodestate"
)

const (
	livenessCheckName  = "ping"
	readinessCheckName = "cache-sync"
)

func main() {
	logger := newLogger()

	if err := run(logger); err != nil {
		logger.Error("fencing-controller failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *log.Logger) error {
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		return err
	}

	logger.SetLevel(log.LogLevelFromStr(cfg.LogLevel))
	ctrl.SetLogger(logr.FromSlogHandler(logger.Handler()))

	return runManager(ctrl.SetupSignalHandler(), cfg, logger)
}

func runManager(ctx context.Context, cfg *config.Config, logger *log.Logger) error {
	restConfig, err := ctrl.GetConfig()
	if err != nil {
		return fmt.Errorf("build kubernetes rest config: %w", err)
	}

	s, err := newScheme()
	if err != nil {
		return err
	}

	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:                        s,
		Metrics:                       metricsserver.Options{BindAddress: cfg.MetricsBindAddress},
		HealthProbeBindAddress:        cfg.HealthProbeBindAddress,
		LeaderElection:                cfg.LeaderElection,
		LeaderElectionID:              common.LeaderElectionID,
		LeaderElectionNamespace:       cfg.LeaderElectionNamespace,
		LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		return fmt.Errorf("create manager: %w", err)
	}

	if err := fencingfailednodestate.New(mgr.GetClient()).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("set up %s controller: %w", common.ControllerName, err)
	}

	gate := newCacheSyncGate(mgr.GetCache())
	if err := mgr.Add(gate); err != nil {
		return fmt.Errorf("add cache sync gate: %w", err)
	}

	if err := mgr.AddHealthzCheck(livenessCheckName, healthz.Ping); err != nil {
		return fmt.Errorf("add healthz check: %w", err)
	}

	if err := mgr.AddReadyzCheck(readinessCheckName, gate.Check); err != nil {
		return fmt.Errorf("add readyz check: %w", err)
	}

	logger.Info("starting fencing-controller",
		"leader_election", cfg.LeaderElection,
		"leader_election_namespace", cfg.LeaderElectionNamespace,
		"leader_election_id", common.LeaderElectionID,
		"health_probe_bind_address", cfg.HealthProbeBindAddress,
		"metrics_bind_address", cfg.MetricsBindAddress,
		"log_level", cfg.LogLevel,
	)

	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("run manager: %w", err)
	}

	logger.Info("stopped")

	return nil
}

func newScheme() (*runtime.Scheme, error) {
	s := runtime.NewScheme()

	if err := clientgoscheme.AddToScheme(s); err != nil {
		return nil, fmt.Errorf("register client-go scheme: %w", err)
	}

	if err := v1alpha1.AddToScheme(s); err != nil {
		return nil, fmt.Errorf("register %s scheme: %w", v1alpha1.GroupVersion, err)
	}

	return s, nil
}

func newLogger() *log.Logger {
	return log.NewLogger(
		log.WithOutput(os.Stdout),
		log.WithHandlerType(log.JSONHandlerType),
	)
}
