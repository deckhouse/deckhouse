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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metrics "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"user-authn-controller/api/v1alpha1"
	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/controller/dexclient"
	"user-authn-controller/internal/controller/identitymigration"
	"user-authn-controller/internal/controller/kubernetesclient"
	"user-authn-controller/internal/controller/providercheck"
	"user-authn-controller/internal/controller/revoke"
	"user-authn-controller/internal/controller/tlsconflict"
	"user-authn-controller/internal/controller/user"
	"user-authn-controller/internal/controller/useraccount"
	"user-authn-controller/internal/controller/userexpire"
	"user-authn-controller/internal/controller/useroperation"
	"user-authn-controller/internal/migration"
	"user-authn-controller/internal/revocation"
	"user-authn-controller/internal/revocationfeed"
)

const (
	controllerName          = "user-authn-controller"
	leaderElectionNamespace = "d8-user-authn"
	// dexClientCommand runs the DexClient reconciler alone, in the dexclient-controller Deployment
	// under a ServiceAccount of its own: it is the only part that writes Secrets, in every
	// namespace, so it runs without the revocation feed and without any listener but health and
	// metrics. dexClientControllerName is its lease.
	dexClientCommand        = "dexclient"
	dexClientControllerName = "user-authn-dexclient-controller"

	// logLevelEnv selects the zap level (debug, info, warn, error); the default is info, as in
	// user-authz-controller.
	logLevelEnv     = "LOG_LEVEL"
	defaultLogLevel = "info"

	// idTokenTTLEnv is userAuthn.idTokenTTL, the lifetime of the ID tokens of Dex. The revocations
	// are kept for it and an hour (revocation.Retention); without it, the default of the module.
	idTokenTTLEnv     = "ID_TOKEN_TTL"
	defaultIDTokenTTL = 10 * time.Minute

	// deviceAgeEnv is the minimum device age of user-api: the devices that old are kept first when
	// the device list of a UserAccount is cut.
	deviceAgeEnv = "MIN_DEVICE_AGE"

	// revocationRetentionEnv keeps the revocations longer than revocation.Retention of the ID token
	// lifetime; a shorter value is raised to it. operationRetentionEnv is how long a UserOperation is
	// kept after its creation.
	revocationRetentionEnv = "REVOCATION_RETENTION"
	operationRetentionEnv  = "USER_OPERATION_RETENTION"

	// passwordHistoryLimitEnv is userAuthn.passwordPolicy.passwordHistoryLimit, the history of
	// replaced passwords Dex keeps; a ResetPassword operation adds the replaced hash to it. Unset
	// (no password policy) keeps no history, as Dex does.
	passwordHistoryLimitEnv = "PASSWORD_HISTORY_LIMIT"

	// revocationFeedAddrEnv is where the revocation feed listens, revocationFeedTLSDirEnv where its
	// certificate is (tls.crt, tls.key). Without the address the feed is off.
	revocationFeedAddrEnv   = "REVOCATION_FEED_ADDR"
	revocationFeedTLSDirEnv = "REVOCATION_FEED_TLS_DIR"

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
	setupLogging()
	logger := ctrl.Log.WithName(controllerName)

	ctx := ctrl.SetupSignalHandler()

	setup := setupRuntimeManager
	if len(os.Args) > 1 {
		if os.Args[1] != dexClientCommand {
			exitOnError(logger, fmt.Errorf("unknown command %q", os.Args[1]), "unable to start")
		}
		setup = setupDexClientManager
	}
	runtimeManager, err := setup()
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

	retention, err := revocationRetention(os.Getenv(idTokenTTLEnv), os.Getenv(revocationRetentionEnv))
	if err != nil {
		return nil, err
	}
	deviceAge, err := minDeviceAge(os.Getenv(deviceAgeEnv))
	if err != nil {
		return nil, err
	}
	operationRetention, err := operationRetention(os.Getenv(operationRetentionEnv))
	if err != nil {
		return nil, err
	}
	historyLimit, err := passwordHistoryLimit(os.Getenv(passwordHistoryLimitEnv))
	if err != nil {
		return nil, err
	}

	cfg, err := ctrl.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("get kubeconfig: %w", err)
	}

	opts, err := newManagerOptions(scheme)
	if err != nil {
		return nil, err
	}
	runtimeManager, err := ctrl.NewManager(cfg, opts)
	if err != nil {
		return nil, fmt.Errorf("create runtime manager: %w", err)
	}

	if err = addHealthChecks(runtimeManager, apiAccessCheck(runtimeManager.GetAPIReader(), &v1alpha1.UserAccountList{}, "UserAccounts")); err != nil {
		return nil, err
	}
	if err = controller.SetupIndexes(context.Background(), runtimeManager.GetFieldIndexer()); err != nil {
		return nil, err
	}

	if err = useraccount.Register(runtimeManager, deviceAge); err != nil {
		return nil, fmt.Errorf("register useraccount controller: %w", err)
	}
	if err = user.Register(runtimeManager); err != nil {
		return nil, fmt.Errorf("register user controller: %w", err)
	}
	if err = useroperation.Register(runtimeManager, operationRetention, historyLimit); err != nil {
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
	if err = identitymigration.Register(runtimeManager); err != nil {
		return nil, fmt.Errorf("register identitymigration controller: %w", err)
	}
	if err = revoke.Register(runtimeManager, retention); err != nil {
		return nil, fmt.Errorf("register revocation gc: %w", err)
	}
	if addr := os.Getenv(revocationFeedAddrEnv); addr != "" {
		feed := revocationfeed.Config{
			Addr:      addr,
			TLSDir:    os.Getenv(revocationFeedTLSDirEnv),
			Retention: retention,
			Limits:    revocationfeed.DefaultLimits(),
		}
		if err = revocationfeed.Register(context.Background(), runtimeManager, feed); err != nil {
			return nil, fmt.Errorf("register revocation feed: %w", err)
		}
	}
	if err = kubernetesclient.Register(runtimeManager); err != nil {
		return nil, fmt.Errorf("register kubernetesclient controller: %w", err)
	}
	if err = migration.Register(runtimeManager); err != nil {
		return nil, fmt.Errorf("register migrations: %w", err)
	}

	return runtimeManager, nil
}

// revocationRetention is how long revocations are kept, from the ID token lifetime idTokenTTL.
// revocationRetention is how long the revocations are kept: revocation.Retention of the ID token
// lifetime, or the configured retention when that is longer.
func revocationRetention(idTokenTTL, configured string) (time.Duration, error) {
	ttl := defaultIDTokenTTL
	if idTokenTTL != "" {
		parsed, err := time.ParseDuration(idTokenTTL)
		if err != nil {
			return 0, fmt.Errorf("parse %s: %w", idTokenTTLEnv, err)
		}
		if parsed <= 0 {
			return 0, fmt.Errorf("%s %q is not positive", idTokenTTLEnv, idTokenTTL)
		}
		ttl = parsed
	}
	retention := revocation.Retention(ttl)
	if configured == "" {
		return retention, nil
	}
	longer, err := time.ParseDuration(configured)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", revocationRetentionEnv, err)
	}
	return max(retention, longer), nil
}

// operationRetention reads how long a UserOperation is kept; unset is a day.
func operationRetention(value string) (time.Duration, error) {
	if value == "" {
		return useroperation.DefaultRetention, nil
	}
	retention, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", operationRetentionEnv, err)
	}
	if retention <= 0 {
		return 0, fmt.Errorf("%s %q is not positive", operationRetentionEnv, value)
	}
	return retention, nil
}

// passwordHistoryLimit reads how many replaced passwords the password policy keeps; unset is none.
func passwordHistoryLimit(value string) (uint64, error) {
	if value == "" {
		return 0, nil
	}
	limit, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", passwordHistoryLimitEnv, err)
	}
	return limit, nil
}

// minDeviceAge reads the minimum device age of user-api; unset is a day, zero lets any device.
func minDeviceAge(value string) (time.Duration, error) {
	if value == "" {
		return useraccount.DefaultEstablishedDeviceAge, nil
	}
	age, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", deviceAgeEnv, err)
	}
	if age < 0 {
		return 0, fmt.Errorf("%s %q is negative", deviceAgeEnv, value)
	}
	return age, nil
}

// setupDexClientManager is the manager of the dexclient command: the DexClient reconciler with
// the caches it reads, and nothing else.
func setupDexClientManager() (ctrl.Manager, error) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("add client-go scheme: %w", err)
	}
	cfg, err := ctrl.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("get kubeconfig: %w", err)
	}
	opts, err := newDexClientManagerOptions(scheme)
	if err != nil {
		return nil, err
	}
	runtimeManager, err := ctrl.NewManager(cfg, opts)
	if err != nil {
		return nil, fmt.Errorf("create runtime manager: %w", err)
	}
	if err = addHealthChecks(runtimeManager, apiAccessCheck(runtimeManager.GetAPIReader(), controller.List(controller.DexClientGVK), "DexClients")); err != nil {
		return nil, err
	}
	if err = dexclient.Register(runtimeManager); err != nil {
		return nil, fmt.Errorf("register dexclient controller: %w", err)
	}
	return runtimeManager, nil
}

func newManagerOptions(scheme *runtime.Scheme) (manager.Options, error) {
	clients, clientCache, err := revocationfeed.ClientCache()
	if err != nil {
		return manager.Options{}, err
	}
	byObject, err := controller.MergeCacheByObject(
		controller.InformerCacheByObject(),
		kubernetesclient.InformerCacheByObject(),
		map[client.Object]cache.ByObject{clients: clientCache},
	)
	if err != nil {
		return manager.Options{}, fmt.Errorf("merge informer cache settings: %w", err)
	}
	return managerOptions(scheme, controllerName, byObject), nil
}

// newDexClientManagerOptions caches what the DexClient reconciler watches: the labelled Secrets by
// metadata and the OAuth2Clients of d8-user-authn without their secrets.
func newDexClientManagerOptions(scheme *runtime.Scheme) (manager.Options, error) {
	clients, clientCache, err := revocationfeed.ClientCache()
	if err != nil {
		return manager.Options{}, err
	}
	byObject, err := controller.MergeCacheByObject(
		dexclient.InformerCacheByObject(),
		map[client.Object]cache.ByObject{clients: clientCache},
	)
	if err != nil {
		return manager.Options{}, fmt.Errorf("merge informer cache settings: %w", err)
	}
	return managerOptions(scheme, dexClientControllerName, byObject), nil
}

func managerOptions(scheme *runtime.Scheme, leaderElectionID string, byObject map[client.Object]cache.ByObject) manager.Options {
	timeout := 10 * time.Second
	// Leader election is always on. It used to follow HA_MODE, but two pods of a Deployment exist
	// outside HA too: the non-HA rolling update surges the new pod before the old one is gone, and
	// both would reconcile and write the same objects for that window.
	lease, renew, retry := leaseDuration, renewDeadline, retryPeriod
	return manager.Options{
		LeaderElection:                true,
		LeaderElectionID:              leaderElectionID,
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
			ByObject: byObject,
		},
		Client: client.Options{
			Cache: &client.CacheOptions{
				Unstructured: true,
				// Secrets are watched by metadata only: a read from the cache would start an
				// informer holding their data.
				DisableFor: []client.Object{&corev1.Secret{}},
			},
		},
	}
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
func addHealthChecks(mgr manager.Manager, apiAccess healthz.Checker) error {
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("add healthz check: %w", err)
	}
	if err := mgr.AddReadyzCheck("cache-sync", cacheSyncCheck(mgr.GetCache())); err != nil {
		return fmt.Errorf("add readyz check: %w", err)
	}
	if err := mgr.AddReadyzCheck("api-access", apiAccess); err != nil {
		return fmt.Errorf("add readyz check: %w", err)
	}
	return nil
}

// apiAccessCheck reports whether the controller can read the objects it reconciles, bypassing the
// cache: one list of one object per probe.
func apiAccessCheck(reader client.Reader, list client.ObjectList, what string) healthz.Checker {
	return func(req *http.Request) error {
		ctx, cancel := context.WithTimeout(req.Context(), apiAccessCheckTimeout)
		defer cancel()

		if err := reader.List(ctx, list, client.Limit(1)); err != nil {
			return fmt.Errorf("list %s: %w", what, err)
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

// logLevel parses a zap level name, falling back to info. Only the named zap levels are accepted, so
// the V(n) messages of controller-runtime below debug are dropped rather than logged as Level(-n).
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

// setupLogging makes one JSON logger, at the level named by LOG_LEVEL, the logger of controller-runtime
// and of klog, which client-go uses for leader election, so that every line is JSON with a named level.
func setupLogging(opts ...zap.Opts) logr.Logger {
	opts = append([]zap.Opts{zap.Level(logLevel(os.Getenv(logLevelEnv))), zap.StacktraceLevel(zapcore.PanicLevel)}, opts...)
	logger := zap.New(opts...)
	ctrl.SetLogger(logger)
	klog.SetLogger(logger)
	return logger
}
