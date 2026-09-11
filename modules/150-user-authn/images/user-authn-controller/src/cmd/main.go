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
	controllerName          = "user-authn-controller"
	leaderElectionNamespace = "d8-user-authn"

	// The lease is renewed against the API server, and controller-runtime exits the process when a
	// renewal misses RenewDeadline (the manager's OnStoppedLeading returns an error from Start). The
	// defaults - 15 s lease, 10 s renew - turn every short API-server absence into a restart of the
	// one non-HA pod, which throws away a warm cache for nothing. These give it a minute to come
	// back; a longer outage still restarts the pod, and readiness has reported it long before.
	leaseDuration = 60 * time.Second
	renewDeadline = 40 * time.Second
	retryPeriod   = 8 * time.Second
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
	// Leader election is always on. It used to follow HA_MODE, but two pods of this Deployment
	// exist outside HA too: the non-HA rolling update surges the new pod before the old one is
	// gone, and both reconcile and write UserAccounts and DexProviderChecks for that window.
	lease, renew, retry := leaseDuration, renewDeadline, retryPeriod
	opts := manager.Options{
		LeaderElection:                true,
		LeaderElectionID:              controllerName,
		LeaderElectionNamespace:       leaderElectionNamespace,
		LeaderElectionReleaseOnCancel: true,
		LeaseDuration:                 &lease,
		RenewDeadline:                 &renew,
		RetryPeriod:                   &retry,
		Scheme:                        scheme,
		GracefulShutdownTimeout:       &timeout,
		HealthProbeBindAddress:        ":9090",
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

	return opts
}

const (
	cacheSyncCheckTimeout = 2 * time.Second
	// apiAccessCheckTimeout keeps the readiness check inside the probe's own budget (3 s).
	apiAccessCheckTimeout = 2 * time.Second
)

// addHealthChecks wires the probes. Liveness is a plain ping: a restart cures nothing this
// controller runs into and throws away a warm cache. Readiness asks two things: cache-sync, that
// the informers caught up at least once, and api-access, that the controller can read the API
// under its own identity right now - the informers stay "synced" after access is lost, so with
// cache-sync alone a controller that could do nothing kept both probes at 200.
func addHealthChecks(mgr manager.Manager) error {
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("add healthz check: %w", err)
	}
	if err := mgr.AddReadyzCheck("cache-sync", cacheSyncCheck(mgr.GetCache())); err != nil {
		return fmt.Errorf("add readyz check: %w", err)
	}
	if err := mgr.AddReadyzCheck("api-access", apiAccessCheck(mgr.GetAPIReader())); err != nil {
		return fmt.Errorf("add readyz check: %w", err)
	}
	return nil
}

// apiAccessCheck reports whether the controller can read the objects it reconciles, bypassing the
// cache: one list of one object per probe.
func apiAccessCheck(reader client.Reader) healthz.Checker {
	return func(req *http.Request) error {
		ctx, cancel := context.WithTimeout(req.Context(), apiAccessCheckTimeout)
		defer cancel()

		if err := reader.List(ctx, &v1alpha1.UserAccountList{}, client.Limit(1)); err != nil {
			return fmt.Errorf("list UserAccounts: %w", err)
		}
		return nil
	}
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
