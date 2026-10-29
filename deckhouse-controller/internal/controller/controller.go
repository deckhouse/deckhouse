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
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	klient "github.com/flant/kube-client/client"
	"github.com/go-logr/logr"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	appsv1 "k8s.io/api/apps/v1"
	coordv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/controller/pkgsync"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/disk"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1beta1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/validation"
	deckhouserelease "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/deckhouse-release"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/docbuilder"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/objectkeeper"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application"
	applicationpackageversion "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application-package-version"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/module"
	modulepackageversion "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/module-package-version"
	packagerepository "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/package-repository"
	packagerepositoryoperation "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/package-repository-operation"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

const (
	docsLeaseLabel = "deckhouse.io/documentation-builder-sync"

	// gracefulShutdownTimeout bounds the controller-runtime manager shutdown.
	gracefulShutdownTimeout = 10 * time.Second

	// controllerName is the name of the controller.
	controllerName = "deckhouse-controller"
)

// Controller drives modules through the package engine alone, without addon-operator.
type Controller struct {
	ctrl ctrlmanager.Manager
	// sync is held for the whole bootstrap, so a waiter cannot observe a half-restored tree
	sync *sync.WaitGroup
	// mu orders Start's sync.Add before Stop's Wait; stopped: Stop ran, a later Start does nothing.
	mu      sync.Mutex
	stopped bool

	manager *engine.Engine

	embeddedPolicy *helpers.ModuleUpdatePolicySpecContainer

	deckhouseSettings    *helpers.DeckhouseSettingsContainer
	applicationsSettings *helpers.ApplicationsSettingsContainer

	dc dependency.Container

	logger *log.Logger
}

// Build assembles the manager, the package engine and the shared containers; it starts nothing.
func Build(ctx context.Context, client *klient.Client, ms metricsstorage.Storage, logger *log.Logger) (*Controller, error) {
	scheme, err := buildSchema()
	if err != nil {
		return nil, fmt.Errorf("build schema: %w", err)
	}

	// Setting the controller-runtime logger to a no-op logger by default,
	// unless debug mode is enabled. This is because the controller-runtime
	// logger is *very* verbose even at info level. This is not really needed,
	// but otherwise we get a warning from the controller-runtime.
	ctrl.SetLogger(logr.New(ctrllog.NullLogSink{}))

	rest := client.RestConfig()

	// inject otel tripper; the manager reads the transport when it builds its clients, so wrap first
	rest.Wrap(func(t http.RoundTripper) http.RoundTripper {
		return otelhttp.NewTransport(t)
	})

	var webhookServer webhook.Server

	admission, serveWebhooks := app.TakeOverAdmissionServer()
	if serveWebhooks {
		listenPort, err := strconv.Atoi(admission.ListenPort)
		if err != nil {
			return nil, fmt.Errorf("parse admission server listen port: %w", err)
		}

		webhookServer = webhook.NewServer(webhook.Options{
			Port:    listenPort,
			CertDir: admission.CertsDir,
		})
	}

	runtime, err := ctrl.NewManager(rest, buildControllerOpts(ctx, scheme, webhookServer))
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

	manager, err := engine.Build(runtime.GetClient(), nil, ms, logger, engine.Config{EnableModuleV2: true})
	if err != nil {
		return nil, fmt.Errorf("create package engine: %w", err)
	}

	deckhouseSettings := helpers.NewDeckhouseSettingsContainer(nil, ms)
	applicationsSettings := helpers.NewApplicationsSettingsContainer()
	synced := new(sync.WaitGroup)
	dc := dependency.NewDependencyContainer()

	// TODO: get rid of stack extenders, its no longer needed, but deckhouse release controller still depends on it
	exts := extenders.NewExtendersStack(app.Version, nil, nil, logger.Named("extenders"))
	err = deckhouserelease.NewDeckhouseReleaseController(ctx, runtime, dc, exts, manager, deckhouseSettings, ms, logger)
	if err != nil {
		return nil, fmt.Errorf("create deckhouse release controller: %w", err)
	}

	err = applicationpackageversion.RegisterController(runtime, dc, logger)
	if err != nil {
		return nil, fmt.Errorf("register application package version controller: %w", err)
	}

	edit, err := edition.Parse()
	if err != nil {
		return nil, fmt.Errorf("parse edition: %w", err)
	}

	err = modulepackageversion.RegisterController(runtime, edit, dc, logger)
	if err != nil {
		return nil, fmt.Errorf("register module package version controller: %w", err)
	}

	err = module.RegisterController(synced, runtime, manager, dc, logger)
	if err != nil {
		return nil, fmt.Errorf("register module controller: %w", err)
	}

	err = application.RegisterController(synced, runtime, manager, nil, deckhouseSettings, applicationsSettings, logger)
	if err != nil {
		return nil, fmt.Errorf("register application controller: %w", err)
	}

	err = packagerepository.RegisterController(runtime, dc, logger)
	if err != nil {
		return nil, fmt.Errorf("register package repository controller: %w", err)
	}

	err = packagerepositoryoperation.RegisterController(runtime, dc, logger)
	if err != nil {
		return nil, fmt.Errorf("register package repository operation controller: %w", err)
	}

	err = docbuilder.RegisterController(runtime, dc, logger.Named("module-documentation-controller"))
	if err != nil {
		return nil, fmt.Errorf("register module documentation controller: %w", err)
	}

	err = objectkeeper.RegisterController(runtime, dc, logger.Named("objectkeeper-controller"))
	if err != nil {
		return nil, fmt.Errorf("register objectkeeper controller: %w", err)
	}

	if serveWebhooks {
		// GetWebhookServer, not the server above: this call adds it to the runnables.
		validation.RegisterAdmissionHandlersV2(
			runtime.GetWebhookServer(),
			runtime.GetClient(),
			manager,
			ms,
			config.NewSchemaStore(nil),
			deckhouseSettings,
			exts,
		)
	}

	return &Controller{
		ctrl: runtime,
		sync: synced,

		manager: manager,

		embeddedPolicy: embeddedPolicy,

		deckhouseSettings:    deckhouseSettings,
		applicationsSettings: applicationsSettings,

		dc: dc,

		logger: logger.Named(controllerName),
	}, nil
}

// buildSchema registers every group version the manager's client and cache decode.
func buildSchema() (*runtime.Scheme, error) {
	addToScheme := []func(s *runtime.Scheme) error{
		corev1.AddToScheme,
		coordv1.AddToScheme,
		v1alpha1.AddToScheme,
		v1alpha2.AddToScheme,
		v1beta1.AddToScheme,
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
func buildControllerOpts(ctx context.Context, scheme *runtime.Scheme, webhookServer webhook.Server) ctrl.Options {
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
		WebhookServer:           webhookServer,
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
		&v1beta1.Module{}:                      {},
	}
}

// Start runs the manager, cleans the disk up and boots the engine from the cluster: Global, every
// installed module and application. Boot restores each from its
// status.currentVersion and status.lastAppliedConfiguration; a Boot error stops deckhouse.
func (c *Controller) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return errors.New("controller stopped")
	}

	c.sync.Add(1)
	c.mu.Unlock()

	defer c.sync.Done()

	// start the engine's services; the hook event loop must run before Boot
	c.manager.Start()

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

	// The old module stack recorded its packages in module releases, and the
	// image ships the embedded modules; give each of them a package version
	// object, and the user module sources their repositories, while the
	// controllers still wait for the sync.
	if err := pkgsync.Sync(ctx, c.ctrl.GetAPIReader(), c.ctrl.GetClient(), c.dc, c.logger); err != nil {
		return fmt.Errorf("sync package objects: %w", err)
	}

	// this list decides what is deleted and what is restored, so a lagging watch would read as a
	// package gone
	applications := new(v1alpha1.ApplicationList)
	if err := c.ctrl.GetAPIReader().List(ctx, applications); err != nil {
		return fmt.Errorf("list applications: %w", err)
	}

	modules := new(v1beta1.ModuleList)
	if err := c.ctrl.GetAPIReader().List(ctx, modules); err != nil {
		return fmt.Errorf("list modules: %w", err)
	}

	// The controllers wait for the sync, so nothing applies a package before Boot. A leak must
	// not stop the boot.
	if err := c.cleanupPackages(ctx, applications.Items, modules.Items); err != nil {
		c.logger.Warn("failed to cleanup packages", log.Err(err))
	}

	if err := c.bootPackages(ctx, applications.Items, modules.Items); err != nil {
		return fmt.Errorf("boot packages: %w", err)
	}

	// update embedded policy and deckhouse settings by the deckhouse moduleConfig
	go c.runSyncSettingsLoop(ctx)

	return nil
}

// Stop waits for a Start in progress (the engine may stop only after Boot returned), then stops
// the package engine; a Start after it does nothing.
func (c *Controller) Stop() {
	c.mu.Lock()
	c.stopped = true
	c.mu.Unlock()

	c.sync.Wait()
	c.manager.Stop()
}

// Engine returns the package engine.
func (c *Controller) Engine() *engine.Engine {
	return c.manager
}

// runSyncSettingsLoop updates the embedded policy, the Deckhouse settings and the global
// applications settings until ctx is canceled.
func (c *Controller) runSyncSettingsLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case config, ok := <-c.manager.GetDeckhouseSettingsCh():
			if !ok {
				return
			}

			c.syncDeckhouseSettings(config)
		case config, ok := <-c.manager.GetGlobalSettingsCh():
			if !ok {
				return
			}

			c.syncGlobalSettings(config)
		}
	}
}

// syncGlobalSettings applies one global module configuration update.
func (c *Controller) syncGlobalSettings(config addonutils.Values) {
	c.logger.Debug("update global applications settings")

	if err := c.applicationsSettings.Set(config); err != nil {
		c.logger.Error("failed to parse the global applications settings", log.Err(err))
	}
}

// syncDeckhouseSettings applies one Deckhouse module configuration update.
func (c *Controller) syncDeckhouseSettings(config addonutils.Values) {
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

	c.deckhouseSettings.Set(settings)

	if settings.ReleaseChannel == "" {
		settings.ReleaseChannel = app.DefaultReleaseChannel
		c.logger.Debug("set embedded deckhouse policy release channel", slog.String("release_channel", settings.ReleaseChannel))
	}

	c.embeddedPolicy.Set(settings)
}

// bootPackages restores Global, every installed module and every installed application. A
// resource being deleted is restored to be uninstalled as at runtime (hooks, release, purge),
// unless restorable rejects it (force-removed, or a disabled module); the controller's remove then
// deletes its release by name.
func (c *Controller) bootPackages(ctx context.Context, applications []v1alpha1.Application, modules []v1beta1.Module) error {
	bootApps := make([]engine.BootApplication, 0, len(applications))
	for i := range applications {
		instance := &applications[i]
		if !application.Restorable(instance) {
			continue
		}

		boot, err := application.BootOf(ctx, c.ctrl.GetClient(), instance)
		if err != nil {
			return fmt.Errorf("boot application '%s/%s': %w", instance.Namespace, instance.Name, err)
		}

		bootApps = append(bootApps, boot)
	}

	// one repository backs many modules, so each is resolved once
	remotes := make(map[string]registry.Remote)

	var global []byte

	bootModules := make([]engine.BootModule, 0, len(modules))
	for i := range modules {
		module := &modules[i]
		if !restorable(module) {
			c.logger.Debug("skip booting a deleted module: nothing installed, force-removed or disabled", slog.String("module", module.Name))
			continue
		}

		// The engine built the global module itself; only its settings checkpoint crosses over.
		if module.Name == "global" {
			global = module.Status.LastAppliedConfiguration.Raw
			continue
		}

		// every module has a repository, an embedded one too; without one it cannot be restored
		if module.Spec.PackageRepositoryName == "" {
			c.logger.Warn("module has no repository, skip booting it", slog.String("module", module.Name))
			continue
		}

		boot := engine.BootModule{
			Name:        module.Name,
			Embedded:    module.IsEmbedded(),
			LastApplied: module.Status.LastAppliedConfiguration.Raw,
			Maintenance: module.Spec.Maintenance,
			Enabled:     module.Spec.Enabled,
			Removing:    !module.DeletionTimestamp.IsZero(),
		}

		if module.Status.CurrentVersion != nil {
			boot.CurrentVersion = module.Status.CurrentVersion.Version
		}

		// an embedded module is in the image and its repository resolves to nothing
		if !boot.Embedded && boot.CurrentVersion != "" {
			remote, ok := remotes[module.Spec.PackageRepositoryName]
			if !ok {
				repo := new(v1alpha1.PackageRepository)
				if err := c.ctrl.GetClient().Get(ctx, client.ObjectKey{Name: module.Spec.PackageRepositoryName}, repo); err != nil {
					return fmt.Errorf("get package repository '%s' of the module '%s': %w",
						module.Spec.PackageRepositoryName, module.Name, err)
				}

				remote = registry.BuildRemote(repo)
				remotes[module.Spec.PackageRepositoryName] = remote
			}

			boot.Repository = remote
		}

		bootModules = append(bootModules, boot)
	}

	return c.manager.Boot(ctx, global, bootApps, bootModules)
}

// cleanupPackages hands the engine every package the cluster still claims, so it drops the rest.
// A terminating instance is kept as in bootPackages: Boot needs its image to uninstall it. An
// embedded module's image is the deckhouse image, not the disk's, so it names nothing. No Module
// listed at all is refused: the sync has just written the global and the embedded ones, so an
// empty list is a read gone wrong, and acting on it would purge every module image.
func (c *Controller) cleanupPackages(ctx context.Context, applications []v1alpha1.Application, modules []v1beta1.Module) error {
	if len(modules) == 0 {
		return errors.New("no module listed")
	}

	preserveApps := make([]disk.PreservePackage, 0, 2*len(applications))
	for i := range applications {
		instance := &applications[i]
		if !application.Restorable(instance) {
			continue
		}

		preserveApps = append(preserveApps, application.PreserveOf(instance)...)
	}

	preserveModules := make([]disk.PreservePackage, 0, 2*len(modules))
	for i := range modules {
		module := &modules[i]
		if !restorable(module) || module.IsEmbedded() {
			continue
		}

		preserveModules = append(preserveModules, disk.PreservePackage{
			Name: module.Name, Repository: module.Spec.PackageRepositoryName, Version: module.Spec.PackageVersion,
		})

		if module.Status.CurrentVersion != nil {
			preserveModules = append(preserveModules, disk.PreservePackage{
				Name: module.Name, Repository: module.Spec.PackageRepositoryName, Version: module.Status.CurrentVersion.Version,
			})
		}
	}

	return c.manager.Cleanup(ctx, preserveApps, preserveModules)
}

// restorable is application.Restorable for a module: Boot gets it and Cleanup keeps its image,
// except one being deleted with no activated version or removedWithoutRestore.
func restorable(module *v1beta1.Module) bool {
	if module.DeletionTimestamp.IsZero() {
		return true
	}

	return module.Status.CurrentVersion != nil && module.Status.CurrentVersion.Version != "" && !removedWithoutRestore(module)
}

// removedWithoutRestore reports whether a module being deleted is left to the controller's remove,
// which deletes its release by name: force-removed, or disabled by an explicit spec.enabled false.
// A package is restored for removal only for what needs it loaded, the deletion hooks and the
// purge; a disabled module has neither (its disable already ran or skipped the deletion hooks, and
// modules have no purge), while its restore costs an image and fails Boot when the registry lost it.
func removedWithoutRestore(module *v1beta1.Module) bool {
	return v1alpha1.IsForceRemoved(module) || (module.Spec.Enabled != nil && !*module.Spec.Enabled)
}
