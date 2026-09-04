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
	"errors"
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

	maxConcurrentReconcilesEnv = "MAX_CONCURRENT_RECONCILES"
	kubeQPSEnv                 = "KUBE_CLIENT_QPS"
	kubeBurstEnv               = "KUBE_CLIENT_BURST"
	// logLevelEnv selects the zap level (debug, info, warn, error); the default is info, so the
	// per-binding messages of the initial adoption of tens of thousands of bindings stay silent.
	logLevelEnv     = "LOG_LEVEL"
	defaultLogLevel = "info"

	// The controller adopts every binding of the module on its first start: tens of thousands of
	// objects at the client-go default of 5 QPS would take hours. The limits below let eight
	// workers write at full speed; at 100 QPS the adoption of 20 000 bindings takes a few minutes.
	defaultKubeQPS   = 100
	defaultKubeBurst = 200
)

func main() {
	ctrl.SetLogger(zap.New(zap.Level(logLevel(os.Getenv(logLevelEnv))), zap.StacktraceLevel(zapcore.PanicLevel)))
	logger := ctrl.Log.WithName(controllerName)

	ctx := ctrl.SetupSignalHandler()

	runtimeManager, err := setupRuntimeManager(ctx, logger)
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

func setupRuntimeManager(ctx context.Context, logger logr.Logger) (ctrl.Manager, error) {
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
	cfg.QPS = float32(envInt(logger, kubeQPSEnv, defaultKubeQPS))
	cfg.Burst = envInt(logger, kubeBurstEnv, defaultKubeBurst)

	runtimeManager, err := ctrl.NewManager(cfg, newManagerOptions(scheme))
	if err != nil {
		return nil, fmt.Errorf("create runtime manager: %w", err)
	}

	if err = addHealthChecks(runtimeManager); err != nil {
		return nil, err
	}

	if err = bindings.Register(ctx, runtimeManager, bindings.Options{
		MaxConcurrentReconciles: envInt(logger, maxConcurrentReconcilesEnv, bindings.DefaultMaxConcurrentReconciles),
	}); err != nil {
		return nil, fmt.Errorf("register bindings controllers: %w", err)
	}

	return runtimeManager, nil
}

func newManagerOptions(scheme *runtime.Scheme) manager.Options {
	// Within the pod's terminationGracePeriodSeconds (60): a worker in the middle of the initial
	// adoption must be allowed to finish its reconcile, or every rollout ends with an error exit.
	timeout := 50 * time.Second

	// Only the module's own bindings are cached: on large clusters unrelated (Cluster)RoleBindings
	// would otherwise dominate the informer memory. managedFields are stripped from every cached
	// object for the same reason.
	moduleBindings := cache.ByObject{
		Label: labels.SelectorFromSet(labels.Set{
			desired.LabelHeritage: desired.HeritageValue,
			desired.LabelModule:   desired.ModuleName,
		}),
	}

	// Leader election is always on: even a single-replica Deployment has two pods during a rolling
	// update, and two writers of the same bindings would fight over them.
	opts := manager.Options{
		LeaderElection:                true,
		LeaderElectionID:              controllerName,
		LeaderElectionNamespace:       leaderElectionNamespace,
		LeaderElectionReleaseOnCancel: true,
		Scheme:                        scheme,
		GracefulShutdownTimeout:       &timeout,
		HealthProbeBindAddress:        ":9090",
		Metrics: metrics.Options{
			BindAddress: ":9091",
		},
		Cache: cache.Options{
			DefaultTransform: cache.TransformStripManagedFields(),
			ByObject: map[client.Object]cache.ByObject{
				&rbacv1.ClusterRoleBinding{}: moduleBindings,
				&rbacv1.RoleBinding{}:        moduleBindings,
			},
		},
	}

	return opts
}

// logLevel parses a zap level name, falling back to info.
func logLevel(name string) zapcore.Level {
	if name == "" {
		name = defaultLogLevel
	}
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(name)); err != nil {
		return zapcore.InfoLevel
	}
	return level
}

// envInt reads a positive integer from the environment, falling back to def (and saying so) when
// the variable is unset or not a positive integer.
func envInt(logger logr.Logger, name string, def int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return def
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		logger.Info("ignoring invalid environment value, using default", "name", name, "value", raw, "default", def)
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

var errCacheNotSynced = errors.New("informer cache is not synced")

func cacheSyncCheck(c cache.Cache) healthz.Checker {
	return func(req *http.Request) error {
		ctx, cancel := context.WithTimeout(req.Context(), cacheSyncCheckTimeout)
		defer cancel()

		if !c.WaitForCacheSync(ctx) {
			return errCacheNotSynced
		}
		return nil
	}
}
