/*
Copyright 2023 Flant JSC

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

package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	addonoperator "github.com/flant/addon-operator/pkg/addon-operator"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules/events"
	"github.com/flant/addon-operator/pkg/utils"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	coordv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/validation"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/confighandler"
	deckhouserelease "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/deckhouse-release"
	moduleconfig "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/config"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/docbuilder"
	moduleoverride "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/override"
	modulerelease "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/release"
	modulesource "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/source"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/go_lib/configtools"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	labelDocsLease = "deckhouse.io/documentation-builder-sync"
	labelApp       = "app"
	labelName      = "name"
	labelHeritage  = "heritage"
	labelModule    = "module"
)

type controller struct {
	runtimeManager     manager.Manager
	preflightCountDown *sync.WaitGroup

	moduleLoader *moduleloader.Loader

	deckhouseConfigCh <-chan utils.Values

	embeddedPolicy *helpers.ModuleUpdatePolicySpecContainer
	settings       *helpers.DeckhouseSettingsContainer
	logger         *log.Logger
}

func Start(ctx context.Context, operator *addonoperator.AddonOperator, logger *log.Logger) error {
	logger = logger.Named("deckhouse-controller")

	addToScheme := []func(s *runtime.Scheme) error{
		corev1.AddToScheme,
		coordv1.AddToScheme,
		v1alpha1.AddToScheme,
		v1alpha2.AddToScheme,
		appsv1.AddToScheme,
	}

	scheme := runtime.NewScheme()
	for _, add := range addToScheme {
		if err := add(scheme); err != nil {
			return fmt.Errorf("add to scheme: %w", err)
		}
	}

	// Setting the controller-runtime logger to a no-op logger by default,
	// unless debug mode is enabled. This is because the controller-runtime
	// logger is *very* verbose even at info level. This is not really needed,
	// but otherwise we get a warning from the controller-runtime.
	controllerruntime.SetLogger(logr.New(ctrllog.NullLogSink{}))

	runtimeManager, err := controllerruntime.NewManager(operator.KubeClient().RestConfig(), controllerruntime.Options{
		Scheme: scheme,
		BaseContext: func() context.Context {
			return ctx
		},
		// disable manager's metrics for a while
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		GracefulShutdownTimeout: ptr.To(10 * time.Second),
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				// for ModuleDocumentation controller
				&coordv1.Lease{}: {
					Namespaces: map[string]cache.Config{
						app.NamespaceDeckhouse: {
							LabelSelector: labels.SelectorFromSet(map[string]string{labelDocsLease: ""}),
						},
					},
				},
				// for ModuleRelease controller and DeckhouseRelease controller
				&corev1.Secret{}: {
					Namespaces: map[string]cache.Config{
						app.NamespaceDeckhouse: {
							LabelSelector: labels.SelectorFromSet(map[string]string{labelHeritage: app.ModuleDeckhouse, labelModule: app.ModuleDeckhouse}),
						},
						app.NamespaceKubernetes: {
							LabelSelector: labels.SelectorFromSet(map[string]string{labelName: app.ClusterConfigurationSecret}),
						},
					},
				},
				// for DeckhouseRelease controller
				&corev1.Pod{}: {
					Namespaces: map[string]cache.Config{
						app.NamespaceDeckhouse: {
							LabelSelector: labels.SelectorFromSet(map[string]string{labelApp: app.ModuleDeckhouse}),
						},
					},
				},
				// for DeckhouseRelease controller
				&corev1.ConfigMap{}: {
					Namespaces: map[string]cache.Config{
						app.NamespaceDeckhouse: {
							LabelSelector: labels.SelectorFromSet(map[string]string{labelHeritage: app.ModuleDeckhouse}),
						},
					},
				},
				// for deckhouse.io apis
				&v1alpha1.Module{}:              {},
				&v1alpha1.ModuleConfig{}:        {},
				&v1alpha1.ModuleDocumentation{}: {},
				&v1alpha1.ModuleRelease{}:       {},
				&v1alpha1.ModuleSource{}:        {},
				&v1alpha2.ModuleUpdatePolicy{}:  {},
				&v1alpha2.ModulePullOverride{}:  {},
				&v1alpha1.DeckhouseRelease{}:    {},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("create controller runtime manager: %w", err)
	}

	deckhouseConfigCh := make(chan utils.Values, 1)

	configHandler := confighandler.New(runtimeManager.GetClient(), deckhouseConfigCh)
	operator.SetupKubeConfigManager(configHandler)

	// init module manager
	if err = operator.Setup(); err != nil {
		return fmt.Errorf("setup operator: %w", err)
	}

	moduleEventCh := make(chan events.ModuleEvent, 350)
	operator.ModuleManager.SetModuleEventsChannel(moduleEventCh)

	// register extenders
	for _, extender := range extenders.Extenders() {
		if err = operator.ModuleManager.AddExtender(extender); err != nil {
			return fmt.Errorf("add extender: %w", err)
		}
	}

	// create a default policy, it'll be filled in with relevant settings from the deckhouse moduleConfig
	embeddedPolicy := helpers.NewModuleUpdatePolicySpecContainer(&v1alpha2.ModuleUpdatePolicySpec{
		Update: v1alpha2.ModuleUpdatePolicySpecUpdate{
			Mode: "Auto",
		},
		ReleaseChannel: "Stable",
	})

	dc := dependency.NewDependencyContainer()
	settingsContainer := helpers.NewDeckhouseSettingsContainer(nil)

	// do not start operator until controllers preflight checks done
	preflightCountDown := new(sync.WaitGroup)

	loader := moduleloader.New(runtimeManager.GetClient(), dc, embeddedPolicy, logger)
	operator.ModuleManager.SetModuleLoader(loader)

	err = deckhouserelease.NewDeckhouseReleaseController(ctx, runtimeManager, dc, operator.ModuleManager, settingsContainer, operator.MetricStorage, preflightCountDown, logger)
	if err != nil {
		return fmt.Errorf("create deckhouse release controller: %w", err)
	}

	err = moduleconfig.RegisterController(runtimeManager, operator.ModuleManager, configHandler, operator.MetricStorage, loader, logger)
	if err != nil {
		return fmt.Errorf("register module config controller: %w", err)
	}

	err = modulesource.RegisterController(runtimeManager, operator.ModuleManager, dc, embeddedPolicy, logger)
	if err != nil {
		return fmt.Errorf("register module source controller: %w", err)
	}

	err = modulerelease.RegisterController(runtimeManager, operator.ModuleManager, dc, embeddedPolicy, operator.MetricStorage, logger)
	if err != nil {
		return fmt.Errorf("register module release controller: %w", err)
	}

	err = moduleoverride.RegisterController(runtimeManager, operator.ModuleManager, dc, logger)
	if err != nil {
		return fmt.Errorf("register module pull override controller: %w", err)
	}

	err = docbuilder.NewModuleDocumentationController(runtimeManager, dc, logger)
	if err != nil {
		return fmt.Errorf("create module documentation controller: %w", err)
	}

	validation.RegisterAdmissionHandlers(
		operator.AdmissionServer,
		runtimeManager.GetClient(),
		operator.ModuleManager,
		configtools.NewValidator(operator.ModuleManager),
		loader,
		operator.MetricStorage)

	deckhouseController := &controller{
		runtimeManager:     runtimeManager,
		moduleLoader:       loader,
		preflightCountDown: preflightCountDown,

		deckhouseConfigCh: deckhouseConfigCh,

		embeddedPolicy: embeddedPolicy,
		settings:       settingsContainer,
		logger:         logger,
	}

	return deckhouseController.start(ctx)
}

// start loads and ensures modules from FS, starts controllers and runs deckhouse config event loop
func (c *controller) start(ctx context.Context) error {
	// run preflight check
	c.startModulesControllers(ctx)

	// wait for cache sync
	if ok := c.runtimeManager.GetCache().WaitForCacheSync(ctx); !ok {
		return fmt.Errorf("wait for cache sync")
	}

	// sync fs with cluster state, restore or delete modules
	if err := c.moduleLoader.Sync(ctx); err != nil {
		return fmt.Errorf("init module loader: %w", err)
	}

	// load and ensure modules from FS at start
	if err := c.moduleLoader.LoadModulesFromFS(ctx); err != nil {
		return fmt.Errorf("load modules from fs: %w", err)
	}

	// update embedded policy and deckhouse settings by the deckhouse moduleConfig
	go c.syncDeckhouseSettings()

	return nil
}

// startModulesControllers starts all child controllers
func (c *controller) startModulesControllers(ctx context.Context) {
	// syncs the fs with the cluster state, starts the manager and various controllers
	go func() {
		if err := c.runtimeManager.Start(ctx); err != nil {
			c.logger.Fatalf("failed to start controller manager: %v", err)
		}
	}()

	c.logger.Info("wait for the preflight checks to run")
	c.preflightCountDown.Wait()
	c.logger.Info("the preflight checks are done")
}

// syncDeckhouseSettings updates embeddedPolicy and deckhouse settings by the deckhouse moduleConfig
func (c *controller) syncDeckhouseSettings() {
	for {
		deckhouseConfig := <-c.deckhouseConfigCh

		configBytes, _ := deckhouseConfig.AsBytes("yaml")
		settings := &helpers.DeckhouseSettings{
			ReleaseChannel: "",
		}
		settings.Update.Mode = "Auto"
		settings.Update.DisruptionApprovalMode = "Auto"

		if err := yaml.Unmarshal(configBytes, settings); err != nil {
			c.logger.Errorf("failed to unmarshal the deckhouse setting: %s", err)
			continue
		}

		c.logger.Debugf("update deckhouse settings")
		c.settings.Set(settings)

		// if deckhouse moduleConfig has releaseChannel unset, apply default releaseChannel Stable to the embedded policy
		if len(settings.ReleaseChannel) == 0 {
			settings.ReleaseChannel = "Stable"
			c.logger.Debugf("the embedded deckhouse policy release channel set to %q", settings.ReleaseChannel)
		}

		c.embeddedPolicy.Set(settings)
	}
}
