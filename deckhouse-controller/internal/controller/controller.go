// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/go-logr/logr"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	appsv1 "k8s.io/api/apps/v1"
	coordv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	pkgruntime "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

const (
	docsLeaseLabel = "deckhouse.io/documentation-builder-sync"

	// gracefulShutdownTimeout bounds the controller-runtime manager shutdown.
	gracefulShutdownTimeout = 10 * time.Second
)

// Controller drives modules through the package runtime alone, without addon-operator.
type Controller struct {
	ctrl ctrlmanager.Manager
	// sync is held for the whole bootstrap, so a waiter cannot observe a half-restored tree
	sync *sync.WaitGroup

	manager *pkgruntime.Runtime

	embeddedPolicy *helpers.ModuleUpdatePolicySpecContainer

	settings   *helpers.DeckhouseSettingsContainer
	settingsCh <-chan addonutils.Values

	dc dependency.Container

	logger *log.Logger
}

// Build assembles the manager, the package runtime and the shared containers; it starts nothing.
func Build(ctx context.Context, rest *rest.Config, ms metricsstorage.Storage, logger *log.Logger) (*Controller, error) {
	scheme, err := buildSchema()
	if err != nil {
		return nil, fmt.Errorf("build schema: %w", err)
	}

	// Setting the controller-runtime logger to a no-op logger by default,
	// unless debug mode is enabled. This is because the controller-runtime
	// logger is *very* verbose even at info level. This is not really needed,
	// but otherwise we get a warning from the controller-runtime.
	ctrl.SetLogger(logr.New(ctrllog.NullLogSink{}))

	// inject otel tripper; the manager reads the transport when it builds its clients, so wrap first
	rest.Wrap(func(t http.RoundTripper) http.RoundTripper {
		return otelhttp.NewTransport(t)
	})

	runtime, err := ctrl.NewManager(rest, buildControllerOpts(ctx, scheme))
	if err != nil {
		return nil, fmt.Errorf("create controller runtime manager: %w", err)
	}

	// create a default policy, it'll be filled in with relevant settings from the deckhouse moduleConfig
	embeddedPolicy := helpers.NewModuleUpdatePolicySpecContainer(&v1alpha2.ModuleUpdatePolicySpec{
		Update: v1alpha2.ModuleUpdatePolicySpecUpdate{
			Mode: v1alpha2.UpdateModeAutoPatch.String(),
		},
		ReleaseChannel: app.DefaultReleaseChannel,
	})

	synced := new(sync.WaitGroup)
	dc := dependency.NewDependencyContainer()
	settingsContainer := helpers.NewDeckhouseSettingsContainer(nil, ms)

	err = metrics.RegisterDeckhouseControllerMetrics(ms)
	if err != nil {
		return nil, fmt.Errorf("register deckhouse controller metrics: %w", err)
	}

	manager, err := pkgruntime.Build(runtime.GetClient(), nil, dc, ms, logger)
	if err != nil {
		return nil, fmt.Errorf("create runtime: %w", err)
	}

	// run the manager in the background
	manager.Run()

	settingsCh := make(chan addonutils.Values)

	return &Controller{
		ctrl: runtime,
		sync: synced,

		manager: manager,

		embeddedPolicy: embeddedPolicy,

		settings:   settingsContainer,
		settingsCh: settingsCh,

		dc: dc,

		logger: logger,
	}, nil
}

// buildSchema registers every group version the manager's client and cache decode.
func buildSchema() (*runtime.Scheme, error) {
	addToScheme := []func(s *runtime.Scheme) error{
		corev1.AddToScheme,
		coordv1.AddToScheme,
		v1alpha1.AddToScheme,
		v1alpha2.AddToScheme,
		appsv1.AddToScheme,
		discoveryv1.AddToScheme,
	}

	scheme := runtime.NewScheme()
	for _, add := range addToScheme {
		if err := add(scheme); err != nil {
			return nil, fmt.Errorf("add to scheme: %w", err)
		}
	}

	return scheme, nil
}

// buildControllerOpts narrows each cached kind to the namespaces and labels a controller reads,
// so the manager does not hold every object of that kind in the cluster.
func buildControllerOpts(ctx context.Context, scheme *runtime.Scheme) ctrl.Options {
	return ctrl.Options{
		Scheme: scheme,
		BaseContext: func() context.Context {
			return ctx
		},
		// disable manager's metrics for a while
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		GracefulShutdownTimeout: new(gracefulShutdownTimeout),
		Cache: cache.Options{
			ByObject: buildCacheByObject(),
		},
	}
}

// buildCacheByObject lists the cached kinds. A kind whose CRD the cluster does not serve never
// syncs and wedges WaitForCacheSync, so the gated ones follow the flag that installs them.
func buildCacheByObject() map[client.Object]cache.ByObject {
	return map[client.Object]cache.ByObject{
		// for ModuleDocumentation controller
		&coordv1.Lease{}: {
			Namespaces: map[string]cache.Config{
				app.NamespaceDeckhouse: {
					LabelSelector: labels.SelectorFromSet(map[string]string{docsLeaseLabel: ""}),
				},
			},
		},
		// for ModuleRelease controller and DeckhouseRelease controller
		&corev1.Secret{}: {
			Namespaces: map[string]cache.Config{
				app.NamespaceDeckhouse: {
					LabelSelector: labels.SelectorFromSet(map[string]string{"heritage": "deckhouse", "module": "deckhouse"}),
				},
				app.NamespaceKubeSystem: {
					LabelSelector: labels.SelectorFromSet(map[string]string{"name": "d8-cluster-configuration"}),
				},
			},
		},
		// for DeckhouseRelease controller
		&corev1.Pod{}: {
			Namespaces: map[string]cache.Config{
				app.NamespaceDeckhouse: {
					LabelSelector: labels.SelectorFromSet(map[string]string{"app": "deckhouse"}),
				},
			},
		},
		// for DeckhouseRelease controller
		&corev1.ConfigMap{}: {
			Namespaces: map[string]cache.Config{
				app.NamespaceDeckhouse: {
					LabelSelector: labels.SelectorFromSet(map[string]string{"heritage": "deckhouse"}),
				},
			},
		},
		// for deckhouse.io apis
		&v1alpha1.Module{}:                     {},
		&v1alpha1.ModuleConfig{}:               {},
		&v1alpha1.ModuleDocumentation{}:        {},
		&v1alpha1.ModuleRelease{}:              {},
		&v1alpha1.ModuleSource{}:               {},
		&v1alpha2.ModuleUpdatePolicy{}:         {},
		&v1alpha2.ModulePullOverride{}:         {},
		&v1alpha1.DeckhouseRelease{}:           {},
		&v1alpha1.PackageRepository{}:          {},
		&v1alpha1.PackageRepositoryOperation{}: {},
		&v1alpha1.ApplicationPackageVersion{}:  {},
		&v1alpha1.ApplicationPackage{}:         {},
		&v1alpha1.Application{}:                {},
		&v1alpha1.ModulePackage{}:              {},
		&v1alpha1.ModulePackageVersion{}:       {},
		&v1alpha2.Module{}:                     {},
	}
}

// Start runs the manager, rebuilds the module tree from the cluster and hands it to the runtime.
// The scheduler is resumed only once that tree is whole, so no module is scheduled half-restored.
func (c *Controller) Start(ctx context.Context) error {
	c.sync.Add(1)
	defer c.sync.Done()

	// starts all child controllers
	go func() {
		if err := c.ctrl.Start(ctx); err != nil {
			c.logger.Fatal("start controller manager failed", log.Err(err))
		}
	}()

	// wait for cache sync
	if ok := c.ctrl.GetCache().WaitForCacheSync(ctx); !ok {
		return fmt.Errorf("wait for cache sync")
	}

	if err := c.restoreModulesV2ByOverrides(ctx); err != nil {
		return fmt.Errorf("restore modules by overrides: %w", err)
	}

	if err := c.restoreModulesV2ByReleases(ctx); err != nil {
		return fmt.Errorf("restore modules by releases: %w", err)
	}

	if err := c.deleteUnplacedModules(ctx); err != nil {
		return fmt.Errorf("delete unplaced modules: %w", err)
	}

	if err := c.syncModulesSettings(ctx); err != nil {
		return fmt.Errorf("sync modules settings: %w", err)
	}

	if err := c.loadModulesV2(ctx); err != nil {
		return fmt.Errorf("load modules v2: %w", err)
	}

	c.manager.ResumeScheduler()

	// update embedded policy and deckhouse settings by the deckhouse moduleConfig
	go c.runSyncSettingsLoop(ctx)

	return nil
}

// runSyncSettingsLoop updates the embedded policy and Deckhouse settings until ctx is canceled.
func (c *Controller) runSyncSettingsLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case config, ok := <-c.settingsCh:
			if !ok {
				return
			}

			c.syncSettings(config)
		}
	}
}

// syncSettings applies one Deckhouse module configuration update.
func (c *Controller) syncSettings(config addonutils.Values) {
	configBytes, err := config.AsBytes("json")
	if err != nil {
		c.logger.Error("failed to marshal the deckhouse settings", log.Err(err))

		return
	}

	settings := helpers.DefaultDeckhouseSettings()
	if err = json.Unmarshal(configBytes, settings); err != nil {
		c.logger.Error("failed to unmarshal the deckhouse settings", log.Err(err))

		return
	}

	c.logger.Debug("update deckhouse settings")

	c.settings.Set(settings)

	if settings.ReleaseChannel == "" {
		settings.ReleaseChannel = app.DefaultReleaseChannel
		c.logger.Debug("set embedded deckhouse policy release channel", slog.String("release_channel", settings.ReleaseChannel))
	}

	c.embeddedPolicy.Set(settings)
}
