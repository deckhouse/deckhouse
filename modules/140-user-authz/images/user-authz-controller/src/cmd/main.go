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
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"go.uber.org/zap/zapcore"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metrics "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	v1 "user-authz-controller/api/v1"
	"user-authz-controller/api/v1alpha1"
	"user-authz-controller/internal/controller/bindings"
	"user-authz-controller/internal/desired"
)

const (
	controllerName          = "user-authz-controller"
	leaderElectionNamespace = "d8-user-authz"

	haModeEnv                  = "HA_MODE"
	maxConcurrentReconcilesEnv = "MAX_CONCURRENT_RECONCILES"
	kubeQPSEnv                 = "KUBE_CLIENT_QPS"
	kubeBurstEnv               = "KUBE_CLIENT_BURST"

	// The controller adopts every binding of the module on its first start; tens of thousands of
	// objects at the client-go default of 5 QPS would take hours, so the limits are explicit.
	defaultKubeQPS   = 50
	defaultKubeBurst = 100
)

func main() {
	ctrl.SetLogger(zap.New(zap.Level(zapcore.Level(-4)), zap.StacktraceLevel(zapcore.PanicLevel)))
	logger := ctrl.Log.WithName(controllerName)

	ctx := ctrl.SetupSignalHandler()

	runtimeManager, err := setupRuntimeManager()
	if err != nil {
		exitOnError(logger, err, "unable to set up runtime manager")
	}

	if err = runtimeManager.Start(ctx); err != nil {
		exitOnError(logger, err, "unable to start runtime manager")
	}
}

func exitOnError(logger logr.Logger, err error, msg string) {
	logger.Error(err, msg)
	os.Exit(1)
}

func setupRuntimeManager() (ctrl.Manager, error) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("add client-go scheme: %w", err)
	}
	if err := v1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("add deckhouse.io/v1 scheme: %w", err)
	}
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("add deckhouse.io/v1alpha1 scheme: %w", err)
	}

	cfg, err := ctrl.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("get kubeconfig: %w", err)
	}
	cfg.QPS = float32(envInt(kubeQPSEnv, defaultKubeQPS))
	cfg.Burst = envInt(kubeBurstEnv, defaultKubeBurst)

	runtimeManager, err := ctrl.NewManager(cfg, newManagerOptions(scheme))
	if err != nil {
		return nil, fmt.Errorf("create runtime manager: %w", err)
	}

	if err = addHealthChecks(runtimeManager); err != nil {
		return nil, err
	}

	if err = bindings.Register(runtimeManager, bindings.Options{
		MaxConcurrentReconciles: envInt(maxConcurrentReconcilesEnv, bindings.DefaultMaxConcurrentReconciles),
	}); err != nil {
		return nil, fmt.Errorf("register bindings controllers: %w", err)
	}

	return runtimeManager, nil
}

func newManagerOptions(scheme *runtime.Scheme) manager.Options {
	timeout := 10 * time.Second

	// Only the module's own bindings are cached: on large clusters unrelated (Cluster)RoleBindings
	// would otherwise dominate the informer memory.
	moduleBindings := cache.ByObject{
		Label: labels.SelectorFromSet(labels.Set{
			desired.LabelHeritage: desired.HeritageValue,
			desired.LabelModule:   desired.ModuleName,
		}),
	}

	opts := manager.Options{
		LeaderElection:          false,
		Scheme:                  scheme,
		GracefulShutdownTimeout: &timeout,
		HealthProbeBindAddress:  ":9090",
		Metrics: metrics.Options{
			BindAddress: ":9091",
		},
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&rbacv1.ClusterRoleBinding{}: moduleBindings,
				&rbacv1.RoleBinding{}:        moduleBindings,
			},
		},
	}

	if os.Getenv(haModeEnv) == "true" {
		opts.LeaderElection = true
		opts.LeaderElectionID = controllerName
		opts.LeaderElectionNamespace = leaderElectionNamespace
	}

	return opts
}

func envInt(name string, def int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return def
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return def
	}
	return value
}

const cacheSyncCheckTimeout = 2 * time.Second

func addHealthChecks(mgr manager.Manager) error {
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("add healthz check: %w", err)
	}
	if err := mgr.AddReadyzCheck("cache-sync", cacheSyncCheck(mgr.GetCache())); err != nil {
		return fmt.Errorf("add readyz check: %w", err)
	}
	return nil
}

func cacheSyncCheck(c cache.Cache) healthz.Checker {
	return func(req *http.Request) error {
		ctx, cancel := context.WithTimeout(req.Context(), cacheSyncCheckTimeout)
		defer cancel()

		if !c.WaitForCacheSync(ctx) {
			return fmt.Errorf("informer cache is not synced")
		}
		return nil
	}
}
