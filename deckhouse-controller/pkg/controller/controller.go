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
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	addonoperator "github.com/flant/addon-operator/pkg/addon-operator"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules/events"
	"github.com/flant/addon-operator/pkg/utils"
	"github.com/go-logr/logr"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	appsv1 "k8s.io/api/apps/v1"
	coordv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	packageoperator "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/operator"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/validation"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/confighandler"
	deckhouserelease "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/deckhouse-release"
	moduleconfig "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/config"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/docbuilder"
	moduleoverride "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/override"
	modulerelease "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/release"
	modulesource "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/source"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/objectkeeper"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application"
	applicationpackageversion "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application-package-version"
	packagerepository "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/package-repository"
	packagerepositoryoperation "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/package-repository-operation"
	d8edition "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/go_lib/configtools"
	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/moduledependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	docsLeaseLabel = "deckhouse.io/documentation-builder-sync"

	deckhouseNamespace  = "d8-system"
	kubernetesNamespace = "kube-system"

	bootstrappedGlobalValue = "clusterIsBootstrapped"
)

type DeckhouseController struct {
	runtimeManager     manager.Manager
	preflightCountDown *sync.WaitGroup

	moduleLoader *moduleloader.Loader

	deckhouseConfigCh <-chan utils.Values

	embeddedPolicy *helpers.ModuleUpdatePolicySpecContainer
	settings       *helpers.DeckhouseSettingsContainer

	defaultReleaseChannel string

	log *log.Logger
}

func NewDeckhouseController(
	ctx context.Context,
	version string,
	defaultReleaseChannel string,
	operator *addonoperator.AddonOperator,
	logger *log.Logger,
) (*DeckhouseController, error) {
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

	// inject otel tripper
	operator.KubeClient().RestConfig().Wrap(func(t http.RoundTripper) http.RoundTripper {
		return otelhttp.NewTransport(t)
	})

	// Setting the controller-runtime logger to a no-op logger by default,
	// unless debug mode is enabled. This is because the controller-runtime
	// logger is *very* verbose even at info level. This is not really needed,
	// but otherwise we get a warning from the controller-runtime.
	controllerruntime.SetLogger(logr.New(ctrllog.NullLogSink{}))

	opts := controllerruntime.Options{
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
				&v1alpha2.ModuleUpdatePolicy{}:  {},
				&v1alpha1.ModulePullOverride{}:  {},
				&v1alpha2.ModulePullOverride{}:  {},
				&v1alpha1.DeckhouseRelease{}:    {},
			},
		},
	}

	// Package system controllers (feature flag)
	if os.Getenv("DECKHOUSE_ENABLE_PACKAGE_SYSTEM") == "true" {
		opts.Cache.ByObject[&v1alpha1.PackageRepository{}] = cache.ByObject{}
		opts.Cache.ByObject[&v1alpha1.PackageRepositoryOperation{}] = cache.ByObject{}
		opts.Cache.ByObject[&v1alpha1.ApplicationPackageVersion{}] = cache.ByObject{}
		opts.Cache.ByObject[&v1alpha1.ApplicationPackage{}] = cache.ByObject{}
		opts.Cache.ByObject[&v1alpha1.Application{}] = cache.ByObject{}
	}

	runtimeManager, err := controllerruntime.NewManager(operator.KubeClient().RestConfig(), opts)
	if err != nil {
		return nil, fmt.Errorf("create controller runtime manager: %w", err)
	}

	conversionsStore := conversion.NewConversionsStore()

	deckhouseConfigCh := make(chan utils.Values, 1)

	configHandler := confighandler.New(runtimeManager.GetClient(), conversionsStore, deckhouseConfigCh)
	operator.SetupKubeConfigManager(configHandler)

	// setup module manager
	if err = operator.Setup(); err != nil {
		return nil, fmt.Errorf("setup operator: %w", err)
	}

	moduleEventCh := make(chan events.ModuleEvent, 350)
	operator.ModuleManager.SetModuleEventsChannel(moduleEventCh)
	// set chrooted environment for modules
	if len(os.Getenv("ADDON_OPERATOR_SHELL_CHROOT_DIR")) > 0 {
		setModulesEnvironment(operator)
	}

	// instantiate ModuleDependency extender
	moduledependency.Instance().SetModulesVersionHelper(func(moduleName string) (string, error) {
		module := new(v1alpha1.Module)
		if err := retry.OnError(retry.DefaultRetry, apierrors.IsServiceUnavailable, func() error {
			return runtimeManager.GetClient().Get(ctx, client.ObjectKey{Name: moduleName}, module)
		}); err != nil {
			return "", err
		}

		// set some version for the modules overridden by mpos
		if module.IsCondition(v1alpha1.ModuleConditionIsOverridden, corev1.ConditionTrue) {
			return "v2.0.0", nil
		}

		return module.GetVersion(), nil
	})

	bootstrappedHelper := func() (bool, error) {
		value, ok := operator.ModuleManager.GetGlobal().GetValues(false)[bootstrappedGlobalValue]
		if !ok {
			return false, nil
		}

		bootstrapped, ok := value.(bool)
		if !ok {
			return false, errors.New("bootstrapped value not boolean")
		}

		return bootstrapped, nil
	}

	edition, err := d8edition.Parse(version)
	if err != nil {
		return nil, fmt.Errorf("parse edition: %w", err)
	}

	exts := extenders.NewExtendersStack(edition, bootstrappedHelper, logger.Named("extenders"))

	// register extenders
	for _, extender := range exts.GetExtenders() {
		if err = operator.ModuleManager.AddExtender(extender); err != nil {
			return nil, fmt.Errorf("add extender: %w", err)
		}
	}

	// create a default policy, it'll be filled in with relevant settings from the deckhouse moduleConfig
	embeddedPolicy := helpers.NewModuleUpdatePolicySpecContainer(&v1alpha2.ModuleUpdatePolicySpec{
		Update: v1alpha2.ModuleUpdatePolicySpecUpdate{
			Mode: "Auto",
		},
		ReleaseChannel: defaultReleaseChannel,
	})

	err = metrics.RegisterDeckhouseControllerMetrics(operator.MetricStorage)
	if err != nil {
		return nil, fmt.Errorf("register deckhouse controller metrics: %w", err)
	}

	dc := dependency.NewDependencyContainer()
	settingsContainer := helpers.NewDeckhouseSettingsContainer(nil, operator.MetricStorage)

	// do not start operator until controllers preflight checks done
	preflightCountDown := new(sync.WaitGroup)

	loader := moduleloader.New(runtimeManager.GetClient(), version, operator.ModuleManager.ModulesDir, operator.ModuleManager.GlobalHooksDir, dc, exts, embeddedPolicy, conversionsStore, logger.Named("module-loader"))
	operator.ModuleManager.SetModuleLoader(loader)

	err = deckhouserelease.NewDeckhouseReleaseController(ctx, runtimeManager, dc, exts, operator.ModuleManager, settingsContainer, operator.MetricStorage, preflightCountDown, version, logger.Named("deckhouse-release-controller"))
	if err != nil {
		return nil, fmt.Errorf("create deckhouse release controller: %w", err)
	}

	err = moduleconfig.RegisterController(runtimeManager, operator.ModuleManager, conversionsStore, edition, configHandler, operator.MetricStorage, exts, logger.Named("module-config-controller"))
	if err != nil {
		return nil, fmt.Errorf("register module config controller: %w", err)
	}

	err = modulesource.RegisterController(runtimeManager, operator.ModuleManager, edition, dc, operator.MetricStorage, embeddedPolicy, logger.Named("module-source-controller"))
	if err != nil {
		return nil, fmt.Errorf("register module source controller: %w", err)
	}

	err = modulerelease.RegisterController(runtimeManager, operator.ModuleManager, loader.Installer(), dc, exts, embeddedPolicy, operator.MetricStorage, logger.Named("module-release-controller"))
	if err != nil {
		return nil, fmt.Errorf("register module release controller: %w", err)
	}

	err = moduleoverride.RegisterController(runtimeManager, operator.ModuleManager, loader, dc, logger.Named("module-pull-override-controller"))
	if err != nil {
		return nil, fmt.Errorf("register module pull override controller: %w", err)
	}

	err = docbuilder.RegisterController(runtimeManager, dc, logger.Named("module-documentation-controller"))
	if err != nil {
		return nil, fmt.Errorf("register module documentation controller: %w", err)
	}

	err = objectkeeper.RegisterController(runtimeManager, dc, logger.Named("objectkeeper-controller"))
	if err != nil {
		return nil, fmt.Errorf("register objectkeeper controller: %w", err)
	}

	// Package system controllers (feature flag)
	if os.Getenv("DECKHOUSE_ENABLE_PACKAGE_SYSTEM") == "true" {
		logger.Info("Package system controllers are enabled")

		err = packagerepository.RegisterController(runtimeManager, dc, logger.Named("package-repository-controller"))
		if err != nil {
			return nil, fmt.Errorf("register package repository controller: %w", err)
		}

		err = packagerepositoryoperation.RegisterController(runtimeManager, dc, logger.Named("package-repository-operation-controller"))
		if err != nil {
			return nil, fmt.Errorf("register package repository operation controller: %w", err)
		}

		err = applicationpackageversion.RegisterController(runtimeManager, dc, logger.Named("application-package-version-controller"))
		if err != nil {
			return nil, fmt.Errorf("register application package version controller: %w", err)
		}

		packageOperator, err := packageoperator.New(operator.ModuleManager, dc, logger)
		if err != nil {
			return nil, fmt.Errorf("create package operator: %w", err)
		}

		// package should not run before converge done
		operator.ConvergeState.SetOnConvergeStart(func() {
			logger.Debug("start converge")
			packageOperator.Scheduler().Pause()
		})

		operator.ConvergeState.SetOnConvergeFinish(func() {
			logger.Debug("finish converge")
			packageOperator.Scheduler().Resume()
		})

		err = application.RegisterController(runtimeManager, packageOperator, operator.ModuleManager, dc, logger.Named("application-controller"))
		if err != nil {
			return nil, fmt.Errorf("register application controller: %w", err)
		}
	}

	validation.RegisterAdmissionHandlers(
		operator.AdmissionServer,
		runtimeManager.GetClient(),
		operator.ModuleManager,
		configtools.NewValidator(operator.ModuleManager, conversionsStore),
		loader,
		operator.MetricStorage,
		config.NewSchemaStore(),
		settingsContainer,
		exts,
	)

	return &DeckhouseController{
		runtimeManager:     runtimeManager,
		moduleLoader:       loader,
		preflightCountDown: preflightCountDown,

		deckhouseConfigCh: deckhouseConfigCh,

		embeddedPolicy: embeddedPolicy,
		settings:       settingsContainer,

		defaultReleaseChannel: defaultReleaseChannel,

		log: logger,
	}, nil
}

func setModulesEnvironment(operator *addonoperator.AddonOperator) {
	operator.ModuleManager.AddObjectsToChrootEnvironment(getChrootObjectDescriptors()...)
}

// Start loads and ensures modules from FS, starts controllers and runs deckhouse config event loop
func (c *DeckhouseController) Start(ctx context.Context) error {
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
func (c *DeckhouseController) startModulesControllers(ctx context.Context) {
	// syncs the fs with the cluster state, starts the manager and various controllers
	go func() {
		if err := c.runtimeManager.Start(ctx); err != nil {
			c.log.Fatal("start controller manager failed", log.Err(err))
		}
	}()

	c.log.Info("waiting for the preflight checks to run")
	c.preflightCountDown.Wait()
	c.log.Info("the preflight checks are done")
}

// syncDeckhouseSettings updates embeddedPolicy and deckhouse settings by the deckhouse moduleConfig
func (c *DeckhouseController) syncDeckhouseSettings() {
	for {
		deckhouseConfig := <-c.deckhouseConfigCh

		configBytes, _ := deckhouseConfig.AsBytes("yaml")
		settings := &helpers.DeckhouseSettings{
			ReleaseChannel:           "",
			AllowExperimentalModules: false,
		}
		settings.Update.Mode = "Auto"
		settings.Update.DisruptionApprovalMode = "Auto"

		if err := yaml.Unmarshal(configBytes, settings); err != nil {
			c.log.Error("failed to unmarshal the deckhouse setting", log.Err(err))
			continue
		}

		c.log.Debug("update deckhouse settings")

		c.settings.Set(settings)

		// if deckhouse moduleConfig has releaseChannel unset, apply default releaseChannel Stable to the embedded policy
		if len(settings.ReleaseChannel) == 0 {
			settings.ReleaseChannel = c.defaultReleaseChannel
			c.log.Debug("the embedded deckhouse policy release channel set", slog.String("release_channel", settings.ReleaseChannel))
		}

		c.embeddedPolicy.Set(settings)
	}
}
