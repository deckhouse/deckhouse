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
	"os"
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
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/confighandler"
	deckhouserelease "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/deckhouse-release"
	moduleconfig "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/config"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/docbuilder"
	modulerelease "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/release"
	modulesource "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/source"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/go_lib/d8env"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	docsLeaseLabel = "deckhouse.io/documentation-builder-sync"

	deckhouseNamespace  = "d8-system"
	kubernetesNamespace = "kube-system"
)

type DeckhouseController struct {
	runtimeManager     manager.Manager
	preflightCountDown *sync.WaitGroup

	moduleLoader *moduleloader.Loader

	deckhouseConfigCh <-chan utils.Values

	embeddedPolicy *helpers.ModuleUpdatePolicySpecContainer
	settings       *helpers.DeckhouseSettingsContainer
	logger         *log.Logger
}

func NewDeckhouseController(ctx context.Context, version string, operator *addonoperator.AddonOperator, logger *log.Logger) (*DeckhouseController, error) {
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
			return nil, fmt.Errorf("add to scheme: %w", err)
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
						deckhouseNamespace: {
							LabelSelector: labels.SelectorFromSet(map[string]string{docsLeaseLabel: ""}),
						},
					},
				},
				// for ModuleRelease controller and DeckhouseRelease controller
				&corev1.Secret{}: {
					Namespaces: map[string]cache.Config{
						deckhouseNamespace: {
							LabelSelector: labels.SelectorFromSet(map[string]string{"heritage": "deckhouse", "module": "deckhouse"}),
						},
						kubernetesNamespace: {
							LabelSelector: labels.SelectorFromSet(map[string]string{"name": "d8-cluster-configuration"}),
						},
					},
				},
				// for DeckhouseRelease controller
				&corev1.Pod{}: {
					Namespaces: map[string]cache.Config{
						deckhouseNamespace: {
							LabelSelector: labels.SelectorFromSet(map[string]string{"app": "deckhouse"}),
						},
					},
				},
				// for DeckhouseRelease controller
				&corev1.ConfigMap{}: {
					Namespaces: map[string]cache.Config{
						deckhouseNamespace: {
							LabelSelector: labels.SelectorFromSet(map[string]string{"heritage": "deckhouse"}),
						},
					},
				},
				// for deckhouse.io apis
				&v1alpha1.Module{}:              {},
				&v1alpha1.ModuleConfig{}:        {},
				&v1alpha1.ModuleDocumentation{}: {},
				&v1alpha1.ModuleRelease{}:       {},
				&v1alpha1.ModuleSource{}:        {},
				&v1alpha1.ModuleUpdatePolicy{}:  {},
				&v1alpha1.ModulePullOverride{}:  {},
				&v1alpha1.DeckhouseRelease{}:    {},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("controller runtime manager creating: %w", err)
	}

	deckhouseConfigCh := make(chan utils.Values, 1)

	configHandler := confighandler.New(runtimeManager.GetClient(), deckhouseConfigCh, logger.Named("config-handler"))
	operator.SetupKubeConfigManager(configHandler)

	// init module manager
	if err = operator.Setup(); err != nil {
		return nil, err
	}

	moduleEventCh := make(chan events.ModuleEvent, 350)
	operator.ModuleManager.SetModuleEventsChannel(moduleEventCh)

	// register extenders
	for _, extender := range extenders.Extenders() {
		if err = operator.ModuleManager.AddExtender(extender); err != nil {
			return nil, fmt.Errorf("add extender: %w", err)
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

	preflightCountDown := new(sync.WaitGroup)

	bundle := os.Getenv("DECKHOUSE_BUNDLE")

	loader := moduleloader.New(runtimeManager.GetClient(), version, operator.ModuleManager.ModulesDir, embeddedPolicy, logger.Named("module-loader"))
	operator.ModuleManager.SetModuleLoader(loader)

	err = deckhouserelease.NewDeckhouseReleaseController(ctx, runtimeManager, dc, operator.ModuleManager, settingsContainer, operator.MetricStorage, preflightCountDown, logger.Named("deckhouse-release-controller"))
	if err != nil {
		return nil, err
	}

	err = moduleconfig.RegisterController(runtimeManager, configHandler, operator.ModuleManager, operator.MetricStorage, loader, bundle, logger.Named("module-config-controller"))
	if err != nil {
		return nil, err
	}

	err = modulesource.RegisterController(runtimeManager, dc, embeddedPolicy, logger.Named("module-source-controller"))
	if err != nil {
		return nil, err
	}

	err = modulerelease.NewModuleReleaseController(runtimeManager, dc, embeddedPolicy, operator.ModuleManager, operator.MetricStorage, preflightCountDown, logger.Named("module-release-controller"))
	if err != nil {
		return nil, err
	}

	err = modulerelease.NewModulePullOverrideController(runtimeManager, dc, operator.ModuleManager, preflightCountDown, logger.Named("module-pull-override-controller"))
	if err != nil {
		return nil, err
	}

	err = docbuilder.NewModuleDocumentationController(runtimeManager, dc, logger.Named("module-documentation-controller"))
	if err != nil {
		return nil, err
	}

	validation.RegisterAdmissionHandlers(operator, loader, operator.MetricStorage)

	return &DeckhouseController{
		runtimeManager:     runtimeManager,
		moduleLoader:       loader,
		preflightCountDown: preflightCountDown,

		deckhouseConfigCh: deckhouseConfigCh,

		embeddedPolicy: embeddedPolicy,
		settings:       settingsContainer,
		logger:         logger,
	}, nil
}

// Start loads and ensures modules from FS, starts pluggable controllers and runs deckhouse config event loop
func (c *DeckhouseController) Start(ctx context.Context) error {
	// run preflight checks first
	if d8env.GetDownloadedModulesDir() != "" {
		c.startPluggableModulesControllers(ctx)
	}

	// load module and ensure from FS at start
	if err := c.moduleLoader.LoadModulesFromFS(ctx); err != nil {
		return err
	}

	go c.runDeckhouseConfigObserver()

	return nil
}

// startPluggableModulesControllers starts all child controllers linked with Modules
func (c *DeckhouseController) startPluggableModulesControllers(ctx context.Context) {
	// syncs the fs with the cluster state, starts the manager and various controllers
	go func() {
		if err := c.runtimeManager.Start(ctx); err != nil {
			c.logger.Fatalf("start controller manager failed: %s", err)
		}
	}()

	c.logger.Info("waiting for the preflight checks to run")
	c.preflightCountDown.Wait()
	c.logger.Info("the preflight checks are done")
}

// runDeckhouseConfigObserver updates embeddedPolicy and deckhouseSettings with the configuration from the deckhouse moduleConfig
func (c *DeckhouseController) runDeckhouseConfigObserver() {
	for {
		deckhouseConfig := <-c.deckhouseConfigCh

		configBytes, _ := deckhouseConfig.AsBytes("yaml")
		settings := &helpers.DeckhouseSettings{
			ReleaseChannel: "",
		}
		settings.Update.Mode = "Auto"
		settings.Update.DisruptionApprovalMode = "Auto"

		if err := yaml.Unmarshal(configBytes, settings); err != nil {
			c.logger.Errorf("error occurred during the Deckhouse settings unmarshalling: %s", err)
			continue
		}

		c.settings.Set(settings)

		// if deckhouse moduleConfig has releaseChannel unset, apply default releaseChannel Stable to the embedded policy
		if len(settings.ReleaseChannel) == 0 {
			settings.ReleaseChannel = "Stable"
			c.logger.Debugf("embedded deckhouse policy release channel set to %s", settings.ReleaseChannel)
		}
		c.embeddedPolicy.Set(settings)
	}
}
