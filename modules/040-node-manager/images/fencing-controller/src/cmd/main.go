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
	"net/http"
	"os"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/adapters/fencingstate"
	"fencing-controller/internal/common"
	"fencing-controller/internal/config"
	"fencing-controller/internal/controllers/fencingfailednodestate"
	"fencing-controller/internal/usecase/noderef"
	"fencing-controller/internal/usecase/profile"
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
	bridge := logr.FromSlogHandler(logger.Handler())
	ctrl.SetLogger(bridge)
	// Leader election and the event broadcaster of client-go write through klog.
	klog.SetLogger(bridge)

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
		Scheme: s,
		// The controller watches all Nodes for the metric of the out-of-service
		// taint and to find a node that returned after a reboot. The cache keeps
		// the Nodes without the status, because the status is the largest part of
		// a Node.
		Cache: cache.Options{ByObject: map[client.Object]cache.ByObject{
			&corev1.Node{}: {Transform: withoutNodeStatus},
		}},
		Metrics:                       metricsserver.Options{BindAddress: cfg.MetricsBindAddress},
		LeaderElection:                cfg.LeaderElection,
		LeaderElectionID:              common.LeaderElectionID,
		LeaderElectionNamespace:       cfg.Namespace,
		LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		return fmt.Errorf("create manager: %w", err)
	}

	profiles := profile.NewResolver(fencingstate.NewProfiles(mgr.GetClient()))
	nodes := noderef.NewValidator()
	recorder := mgr.GetEventRecorderFor(common.ControllerName)

	// The controller reads the pods, the Instances and the machine health
	// checks from the API server, so it does not keep them in the cache. It
	// reads them only for the Node of a proven incident. It has no permission
	// to watch the machine health checks.
	reconciler := fencingfailednodestate.New(mgr.GetClient(), mgr.GetAPIReader(), cfg.Namespace, nodes, profiles, recorder)
	if err := reconciler.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("set up %s controller: %w", common.ControllerName, err)
	}

	if err := fencingfailednodestate.SetupOutOfServiceMetric(mgr); err != nil {
		return fmt.Errorf("set up out-of-service metric controller: %w", err)
	}

	gate := newCacheSyncGate(mgr.GetCache())
	if err := mgr.Add(gate); err != nil {
		return fmt.Errorf("add cache sync gate: %w", err)
	}

	// The probes use the address of the metrics, so the controller occupies one port of the node.
	probes := map[string]*healthz.Handler{
		"/healthz": {Checks: map[string]healthz.Checker{livenessCheckName: healthz.Ping}},
		"/readyz":  {Checks: map[string]healthz.Checker{readinessCheckName: gate.Check}},
	}
	for path, probe := range probes {
		if err := mgr.AddMetricsServerExtraHandler(path, http.StripPrefix(path, probe)); err != nil {
			return fmt.Errorf("add %s handler: %w", path, err)
		}
	}

	logger.Info("starting fencing-controller",
		"leader_election", cfg.LeaderElection,
		"namespace", cfg.Namespace,
		"leader_election_id", common.LeaderElectionID,
		"metrics_bind_address", cfg.MetricsBindAddress,
		"log_level", cfg.LogLevel,
	)

	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("run manager: %w", err)
	}

	logger.Info("stopped")

	return nil
}

// withoutNodeStatus removes the status and the managed fields of a Node before
// the cache stores it.
func withoutNodeStatus(obj any) (any, error) {
	if node, ok := obj.(*corev1.Node); ok {
		node.Status = corev1.NodeStatus{}
		node.ManagedFields = nil
	}

	return obj, nil
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
