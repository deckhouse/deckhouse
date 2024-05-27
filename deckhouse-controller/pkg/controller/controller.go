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
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules/events"
	"github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/shell-operator/pkg/metric_storage"
	"github.com/go-logr/logr"
	log "github.com/sirupsen/logrus"
	coordv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/pointer"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/client/clientset/versioned"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/models"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/docbuilder"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/release"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/source"
	d8utils "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	d8config "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
	"github.com/deckhouse/deckhouse/go_lib/deckhouse-config/conversion"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const (
	epochLabelKey  = "deckhouse.io/epoch"
	docsLeaseLabel = "deckhouse.io/documentation-builder-sync"
	namespace      = "d8-system"
)

var (
	epochLabelValue = fmt.Sprintf("%d", rand.Uint32())
	bundleName      = os.Getenv("DECKHOUSE_BUNDLE")
)

type DeckhouseController struct {
	ctx                context.Context
	mgr                manager.Manager
	preflightCountDown *sync.WaitGroup

	dirs       []string
	mm         *module_manager.ModuleManager // probably it's better to set it via the interface
	kubeClient *versioned.Clientset

	metricStorage *metric_storage.MetricStorage

	deckhouseModules map[string]*models.DeckhouseModule
	// <module-name>: <module-source>
	sourceModules           map[string]string
	embeddedDeckhousePolicy *v1alpha1.ModuleUpdatePolicySpec
}

func NewDeckhouseController(ctx context.Context, config *rest.Config, mm *module_manager.ModuleManager, metricStorage *metric_storage.MetricStorage) (*DeckhouseController, error) {
	mcClient, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	dc := dependency.NewDependencyContainer()
	embeddedDeckhousePolicy := &v1alpha1.ModuleUpdatePolicySpec{
		Update: v1alpha1.ModuleUpdatePolicySpecUpdate{
			Mode: "Auto",
		},
		ReleaseChannel: "Stable",
	}

	scheme := runtime.NewScheme()

	for _, add := range []func(s *runtime.Scheme) error{corev1.AddToScheme, coordv1.AddToScheme, v1alpha1.AddToScheme} {
		err = add(scheme)
		if err != nil {
			return nil, err
		}
	}

	// Setting the controller-runtime logger to a no-op logger by default,
	// unless debug mode is enabled. This is because the controller-runtime
	// logger is *very* verbose even at info level. This is not really needed,
	// but otherwise we get a warning from the controller-runtime.
	controllerruntime.SetLogger(logr.New(ctrllog.NullLogSink{}))

	mgr, err := controllerruntime.NewManager(config, controllerruntime.Options{
		Scheme: scheme,
		BaseContext: func() context.Context {
			return ctx
		},
		GracefulShutdownTimeout: pointer.Duration(10 * time.Second),
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				// for ModuleDocumentation controller
				&coordv1.Lease{}: {
					Namespaces: map[string]cache.Config{
						namespace: {
							LabelSelector: labels.SelectorFromSet(map[string]string{docsLeaseLabel: ""}),
						},
					},
				},
				// for ModuleRelease controller
				&corev1.Secret{}: {
					Namespaces: map[string]cache.Config{
						namespace: {
							LabelSelector: labels.SelectorFromSet(map[string]string{"heritage": "deckhouse", "module": "deckhouse"}),
						},
					},
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	err = source.NewModuleSourceController(mgr, dc, embeddedDeckhousePolicy)
	if err != nil {
		return nil, err
	}

	var preflightCountDown sync.WaitGroup

	err = release.NewModuleReleaseController(mgr, dc, embeddedDeckhousePolicy, mm, metricStorage, &preflightCountDown)
	if err != nil {
		return nil, err
	}

	err = release.NewModulePullOverrideController(mgr, dc, mm, &preflightCountDown)
	if err != nil {
		return nil, err
	}

	err = docbuilder.NewModuleDocumentationController(mgr, dc)
	if err != nil {
		return nil, err
	}

	return &DeckhouseController{
		ctx:                ctx,
		kubeClient:         mcClient,
		dirs:               utils.SplitToPaths(mm.ModulesDir),
		mm:                 mm,
		mgr:                mgr,
		preflightCountDown: &preflightCountDown,

		deckhouseModules:        make(map[string]*models.DeckhouseModule),
		sourceModules:           make(map[string]string),
		embeddedDeckhousePolicy: embeddedDeckhousePolicy,
		metricStorage:           metricStorage,
	}, nil
}

func (dml *DeckhouseController) discoverDeckhouseModules(ctx context.Context, moduleEventC <-chan events.ModuleEvent, deckhouseConfigC <-chan utils.Values) error {
	err := dml.searchAndLoadDeckhouseModules()
	if err != nil {
		return fmt.Errorf("search and load Deckhouse modules: %w", err)
	}

	// we have to get all source module for deployed releases
	err = dml.setupSourceModules(ctx)
	if err != nil {
		return err
	}

	go dml.runEventLoop(moduleEventC)
	go dml.runDeckhouseConfigObserver(deckhouseConfigC)

	// Init modules' and modules configs' statuses as soon as Module Manager's moduleset gets Inited flag (all modules are registered)
	go func() {
		// Check if Module Manager has been initialized
		_ = wait.PollUntilContextCancel(dml.ctx, d8utils.SyncedPollPeriod, false,
			func(context.Context) (bool, error) {
				return dml.mm.AreModulesInited(), nil
			})

		err := dml.InitModulesAndConfigsStatuses()
		if err != nil {
			log.Errorf("Error occurred when setting modules and module configs' initial statuses: %s", err)
		}
	}()

	return nil
}

// really, don't like this method, because it doesn't use cache
// we can't use Manager.Client here, because it's cache is not started yet.
// but another way is to make some reactive storage, which will collect modules without sources and update them
func (dml *DeckhouseController) setupSourceModules(ctx context.Context) error {
	// fetch modules source for deployed releases
	mrList, err := dml.kubeClient.DeckhouseV1alpha1().ModuleReleases().List(ctx, v1.ListOptions{})
	if err != nil {
		return err
	}

	for _, rl := range mrList.Items {
		if rl.Status.Phase != v1alpha1.PhaseDeployed {
			continue
		}
		if !rl.ObjectMeta.DeletionTimestamp.IsZero() {
			continue
		}

		if _, ok := dml.sourceModules[rl.GetModuleName()]; !ok {
			// ignore modules that are already marked as Embedded
			dml.sourceModules[rl.GetModuleName()] = rl.GetModuleSource()
		}
	}

	return nil
}

// Start function starts all child controllers linked with Modules
func (dml *DeckhouseController) Start(ctx context.Context, moduleEventC <-chan events.ModuleEvent, deckhouseConfigC <-chan utils.Values) error {
	if os.Getenv("EXTERNAL_MODULES_DIR") == "" {
		return nil
	}

	// syncs the fs with the cluster state, starts the manager and various controllers
	go func() {
		err := dml.mgr.Start(ctx)
		if err != nil {
			log.Fatalf("Start controller manager failed: %s", err)
		}
	}()

	log.Info("Waiting for the preflight checks to run")
	dml.preflightCountDown.Wait()
	log.Info("The preflight checks are done")

	// discovers modules on the fs, runs modules events loop (register/delete/etc)
	return dml.discoverDeckhouseModules(ctx, moduleEventC, deckhouseConfigC)
}

func (dml *DeckhouseController) runDeckhouseConfigObserver(deckhouseConfigC <-chan utils.Values) {
	for {
		cfg := <-deckhouseConfigC

		b, _ := cfg.AsBytes("yaml")
		mups := &v1alpha1.ModuleUpdatePolicySpec{
			Update: v1alpha1.ModuleUpdatePolicySpecUpdate{
				Mode: "Auto",
			},
			ReleaseChannel: "Stable",
		}
		err := yaml.Unmarshal(b, mups)
		if err != nil {
			log.Errorf("Error occurred during the Deckhouse embedded policy build: %s", err)
			continue
		}
		dml.embeddedDeckhousePolicy.ReleaseChannel = mups.ReleaseChannel
		dml.embeddedDeckhousePolicy.Update.Mode = mups.Update.Mode
		dml.embeddedDeckhousePolicy.Update.Windows = mups.Update.Windows
	}
}

// InitModulesAndConfigsStatuses inits and moduleconfigs' status fields at start up
func (dml *DeckhouseController) InitModulesAndConfigsStatuses() error {
	return retry.OnError(retry.DefaultRetry, errors.IsServiceUnavailable, func() error {
		modules, err := dml.kubeClient.DeckhouseV1alpha1().Modules().List(dml.ctx, v1.ListOptions{})
		if err != nil {
			return err
		}

		for _, module := range modules.Items {
			err := dml.updateModuleStatus(module.Name)
			if err != nil {
				log.Errorf("Error occurred during the module %q status update: %s", module.Name, err)
				return err
			}
		}

		configs, err := dml.kubeClient.DeckhouseV1alpha1().ModuleConfigs().List(dml.ctx, v1.ListOptions{})
		if err != nil {
			return err
		}

		for _, config := range configs.Items {
			err := dml.updateModuleConfigStatus(config.Name)
			if err != nil {
				log.Errorf("Error occurred during the module config %q status update: %s", config.Name, err)
				return err
			}
		}
		return nil
	})
}

func (dml *DeckhouseController) runEventLoop(moduleEventCh <-chan events.ModuleEvent) {
	for event := range moduleEventCh {
		// events without module name or for non-existent modules (module configs)
		switch event.EventType {
		case events.FirstConvergeDone:
			err := dml.handleConvergeDone()
			if err != nil {
				log.Errorf("Error occurred during the converge done: %s", err)
			}
			continue

		case events.ModuleConfigChanged:
			if d8config.IsServiceInited() {
				err := dml.updateModuleConfigStatus(event.ModuleName)
				if err != nil {
					log.Errorf("Error occurred when updating module config %s: %s", event.ModuleName, err)
				}
			}
			continue
		}

		mod, ok := dml.deckhouseModules[event.ModuleName]
		if !ok {
			log.Errorf("Module %q registered but not found in Deckhouse. Possible bug?", event.ModuleName)
			continue
		}

		switch event.EventType {
		case events.ModuleRegistered:
			err := dml.handleModuleRegistration(mod)
			if err != nil {
				log.Errorf("Error occurred during the module %q registration: %s", mod.GetBasicModule().GetName(), err)
				continue
			}

		case events.ModuleEnabled:
			err := dml.handleEnabledModule(mod, true)
			if err != nil {
				log.Errorf("Error occurred during the module %q turning on: %s", mod.GetBasicModule().GetName(), err)
				continue
			}

		case events.ModuleDisabled:
			err := dml.handleEnabledModule(mod, false)
			if err != nil {
				log.Errorf("Error occurred during the module %q turning off: %s", mod.GetBasicModule().GetName(), err)
				continue
			}

		case events.ModuleStateChanged:
			err := dml.updateModuleStatus(event.ModuleName)
			if err != nil {
				log.Errorf("Error occurred during the module %q status update: %s", event.ModuleName, err)
				continue
			}
		}
	}
}

func (dml *DeckhouseController) updateModuleConfigStatus(configName string) error {
	return retry.OnError(retry.DefaultRetry, errors.IsServiceUnavailable, func() error {
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			metricGroup := fmt.Sprintf("%s_%s", "obsoleteVersion", configName)
			dml.metricStorage.GroupedVault.ExpireGroupMetrics(metricGroup)
			moduleConfig, moduleErr := dml.kubeClient.DeckhouseV1alpha1().ModuleConfigs().Get(dml.ctx, configName, v1.GetOptions{})

			// if module config found
			if moduleErr == nil {
				newModuleConfigStatus := d8config.Service().StatusReporter().ForConfig(moduleConfig)
				if (moduleConfig.Status.Message != newModuleConfigStatus.Message) || (moduleConfig.Status.Version != newModuleConfigStatus.Version) {
					moduleConfig.Status.Message = newModuleConfigStatus.Message
					moduleConfig.Status.Version = newModuleConfigStatus.Version

					log.Debugf(
						"Update /status for moduleconfig/%s: version '%s' to %s', message '%s' to '%s'",
						moduleConfig.Name,
						moduleConfig.Status.Version, newModuleConfigStatus.Version,
						moduleConfig.Status.Message, newModuleConfigStatus.Message,
					)

					_, err := dml.kubeClient.DeckhouseV1alpha1().ModuleConfigs().UpdateStatus(dml.ctx, moduleConfig, v1.UpdateOptions{})
					if err != nil {
						return err
					}
				}

				// update metrics
				converter := conversion.Store().Get(moduleConfig.Name)

				if moduleConfig.Spec.Version > 0 && moduleConfig.Spec.Version < converter.LatestVersion() {
					dml.metricStorage.GroupedVault.GaugeSet(metricGroup, "module_config_obsolete_version", 1.0, map[string]string{
						"name":    moduleConfig.Name,
						"version": strconv.Itoa(moduleConfig.Spec.Version),
						"latest":  strconv.Itoa(converter.LatestVersion()),
					})
				}
			}

			// update the related module if it exists
			if moduleErr == nil || (moduleErr != nil && errors.IsNotFound(moduleErr)) {
				err := dml.updateModuleStatus(configName)
				// it's possible that such a module doesn't exist
				if err != nil && !errors.IsNotFound(err) {
					return err
				}
			}
			return moduleErr
		})
	})
}

func (dml *DeckhouseController) updateModuleStatus(moduleName string) error {
	return retry.OnError(retry.DefaultRetry, errors.IsServiceUnavailable, func() error {
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			module, err := dml.kubeClient.DeckhouseV1alpha1().Modules().Get(dml.ctx, moduleName, v1.GetOptions{})
			if err != nil {
				return err
			}

			moduleConfig, err := dml.kubeClient.DeckhouseV1alpha1().ModuleConfigs().Get(dml.ctx, moduleName, v1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					moduleConfig = nil
				} else {
					return err
				}
			}

			newModuleStatus := d8config.Service().StatusReporter().ForModule(module, moduleConfig, bundleName)
			if module.Status.Status != newModuleStatus.Status || module.Status.Message != newModuleStatus.Message || module.Status.HooksState != newModuleStatus.HooksState {
				module.Status.Status = newModuleStatus.Status
				module.Status.Message = newModuleStatus.Message
				module.Status.HooksState = newModuleStatus.HooksState

				log.Debugf("Update /status for module/%s: status '%s' to '%s', message '%s' to '%s'", moduleName, module.Status.Status, newModuleStatus.Status, module.Status.Message, newModuleStatus.Message)

				_, err = dml.kubeClient.DeckhouseV1alpha1().Modules().UpdateStatus(dml.ctx, module, v1.UpdateOptions{})
				return err
			}
			return nil
		})
	})
}

// handleConvergeDone after converge we delete all absent Modules CR, which were not filled during this operator startup
func (dml *DeckhouseController) handleConvergeDone() error {
	epochLabelStr := fmt.Sprintf("%s!=%s", epochLabelKey, epochLabelValue)
	return retry.OnError(retry.DefaultRetry, errors.IsServiceUnavailable, func() error {
		return dml.kubeClient.DeckhouseV1alpha1().Modules().DeleteCollection(dml.ctx, v1.DeleteOptions{}, v1.ListOptions{LabelSelector: epochLabelStr})
	})
}

func (dml *DeckhouseController) handleModulePurge(m *models.DeckhouseModule) error {
	return retry.OnError(retry.DefaultRetry, errors.IsServiceUnavailable, func() error {
		return dml.kubeClient.DeckhouseV1alpha1().Modules().Delete(dml.ctx, m.GetBasicModule().GetName(), v1.DeleteOptions{})
	})
}

func (dml *DeckhouseController) handleModuleRegistration(m *models.DeckhouseModule) error {
	return retry.OnError(retry.DefaultRetry, errors.IsServiceUnavailable, func() error {
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			moduleName := m.GetBasicModule().GetName()
			src := dml.sourceModules[moduleName]
			newModule := m.AsKubeObject(src)
			newModule.SetLabels(map[string]string{epochLabelKey: epochLabelValue})

			// update d8service state
			d8config.Service().AddModuleNameToSource(moduleName, src)
			d8config.Service().AddPossibleName(moduleName)

			existModule, err := dml.kubeClient.DeckhouseV1alpha1().Modules().Get(dml.ctx, newModule.GetName(), v1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					_, err = dml.kubeClient.DeckhouseV1alpha1().Modules().Create(dml.ctx, newModule, v1.CreateOptions{})
					return err
				}

				return err
			}

			existModule.Properties = newModule.Properties
			if len(existModule.Labels) == 0 {
				existModule.SetLabels(map[string]string{epochLabelKey: epochLabelValue})
			} else {
				existModule.Labels[epochLabelKey] = epochLabelValue
			}

			_, err = dml.kubeClient.DeckhouseV1alpha1().Modules().Update(dml.ctx, existModule, v1.UpdateOptions{})

			return err
		})
	})
}

func (dml *DeckhouseController) handleEnabledModule(m *models.DeckhouseModule, enable bool) error {
	return retry.OnError(retry.DefaultRetry, errors.IsServiceUnavailable, func() error {
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			module, err := dml.kubeClient.DeckhouseV1alpha1().Modules().Get(dml.ctx, m.GetBasicModule().GetName(), v1.GetOptions{})
			if err != nil {
				return err
			}

			module.Properties.State = "Disabled"
			module.Status.Status = "Disabled"
			if enable {
				module.Properties.State = "Enabled"
				module.Status.Status = "Enabled"
			}

			_, err = dml.kubeClient.DeckhouseV1alpha1().Modules().Update(dml.ctx, module, v1.UpdateOptions{})
			if err != nil {
				return err
			}

			err = dml.updateModuleStatus(module.Name)
			if err != nil {
				log.Errorf("Error occurred during the module %q status update: %s", module.Name, err)
				return err
			}

			return nil
		})
	})
}
