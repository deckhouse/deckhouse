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
	"time"

	"github.com/go-logr/logr"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metrics "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"user-authn-controller/api/v1alpha1"
	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/controller/providercheck"
	"user-authn-controller/internal/controller/tlsconflict"
	"user-authn-controller/internal/controller/user"
	"user-authn-controller/internal/controller/useraccount"
	"user-authn-controller/internal/controller/userexpire"
	"user-authn-controller/internal/controller/useroperation"
)

const (
	haModeEnv               = "HA_MODE"
	controllerName          = "user-authn-controller"
	leaderElectionNamespace = "d8-user-authn"
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
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("add v1alpha1 scheme: %w", err)
	}

	cfg, err := ctrl.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("get kubeconfig: %w", err)
	}

	runtimeManager, err := ctrl.NewManager(cfg, newManagerOptions(scheme))
	if err != nil {
		return nil, fmt.Errorf("create runtime manager: %w", err)
	}

	if err = addHealthChecks(runtimeManager); err != nil {
		return nil, err
	}

	if err = useraccount.Register(runtimeManager); err != nil {
		return nil, fmt.Errorf("register useraccount controller: %w", err)
	}
	if err = user.Register(runtimeManager); err != nil {
		return nil, fmt.Errorf("register user controller: %w", err)
	}
	if err = useroperation.Register(runtimeManager); err != nil {
		return nil, fmt.Errorf("register useroperation controller: %w", err)
	}
	if err = userexpire.Register(runtimeManager); err != nil {
		return nil, fmt.Errorf("register userexpire controller: %w", err)
	}
	if err = providercheck.Register(runtimeManager); err != nil {
		return nil, fmt.Errorf("register dexprovidercheck controller: %w", err)
	}
	if err = tlsconflict.Register(runtimeManager); err != nil {
		return nil, fmt.Errorf("register tlsconflict controller: %w", err)
	}

	return runtimeManager, nil
}

func newManagerOptions(scheme *runtime.Scheme) manager.Options {
	timeout := 10 * time.Second
	opts := manager.Options{
		LeaderElection:          false,
		Scheme:                  scheme,
		GracefulShutdownTimeout: &timeout,
		HealthProbeBindAddress:  ":9090",
		Metrics: metrics.Options{
			BindAddress: ":9091",
		},
		Cache: cache.Options{
			ByObject: controller.InformerCacheByObject(),
		},
		Client: client.Options{
			Cache: &client.CacheOptions{
				Unstructured: true,
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
