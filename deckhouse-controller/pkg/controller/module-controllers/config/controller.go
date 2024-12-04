// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/flant/addon-operator/pkg/kube_config_manager/config"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules/events"
	metricstorage "github.com/flant/shell-operator/pkg/metric_storage"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/confighandler"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader"
	"github.com/deckhouse/deckhouse/go_lib/configtools"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	controllerName = "d8-module-config-controller"

	// if a module is disabled more than three days, it will be uninstalled at next deckhouse restart
	deleteReleasesAfter = 72 * time.Hour

	maxConcurrentReconciles = 3

	moduleDeckhouse = "deckhouse"
	moduleGlobal    = "global"
)

func RegisterController(
	runtimeManager manager.Manager,
	handler *confighandler.Handler,
	mm moduleManager,
	ms *metricstorage.MetricStorage,
	loader *moduleloader.Loader,
	bundle string,
	logger *log.Logger,
) error {
	r := &reconciler{
		init:            new(sync.WaitGroup),
		client:          runtimeManager.GetClient(),
		log:             logger,
		handler:         handler,
		moduleManager:   mm,
		metricStorage:   ms,
		moduleLoader:    loader,
		bundle:          bundle,
		configValidator: configtools.NewValidator(mm),
	}

	r.init.Add(1)

	// sync module configs
	if err := runtimeManager.Add(manager.RunnableFunc(r.syncModules)); err != nil {
		return err
	}

	configController, err := controller.New(controllerName, runtimeManager, controller.Options{
		MaxConcurrentReconciles: maxConcurrentReconciles,
		NeedLeaderElection:      ptr.To(false),
		Reconciler:              r,
	})
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(runtimeManager).
		For(&v1alpha1.ModuleConfig{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})).
		Complete(configController)
}

type reconciler struct {
	init            *sync.WaitGroup
	client          client.Client
	log             *log.Logger
	handler         *confighandler.Handler
	moduleManager   moduleManager
	metricStorage   *metricstorage.MetricStorage
	moduleLoader    *moduleloader.Loader
	configValidator *configtools.Validator
	bundle          string
}

type moduleManager interface {
	AreModulesInited() bool
	IsModuleEnabled(moduleName string) bool
	GetModuleNames() []string
	GetModule(name string) *modules.BasicModule
	GetGlobal() *modules.GlobalModule
	GetUpdatedByExtender(name string) (string, error)
	GetModuleEventsChannel() chan events.ModuleEvent
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// wait until init
	r.init.Wait()

	r.log.Debugf("reconciling the '%s' module config", req.Name)
	moduleConfig := new(v1alpha1.ModuleConfig)
	if err := r.client.Get(ctx, client.ObjectKey{Name: req.Name}, moduleConfig); err != nil {
		if apierrors.IsNotFound(err) {
			r.log.Warnf("the '%s' module config not found", req.Name)
			return ctrl.Result{}, nil
		}
		r.log.Errorf("failed to get the '%s' module config: %v", req.Name, err)
		return ctrl.Result{}, err
	}

	// handle delete event
	if !moduleConfig.DeletionTimestamp.IsZero() {
		r.log.Debugf("deleting the '%s' module config", req.Name)
		return r.deleteModuleConfig(ctx, moduleConfig)
	}

	return r.handleModuleConfig(ctx, moduleConfig)
}

func (r *reconciler) handleModuleConfig(ctx context.Context, moduleConfig *v1alpha1.ModuleConfig) (ctrl.Result, error) {
	// send event to addon-operator(it is not necessary for NotInstalled modules)
	r.handler.HandleEvent(ctx, moduleConfig, config.EventUpdate)

	if err := r.refreshModuleConfig(ctx, moduleConfig.Name); err != nil {
		return ctrl.Result{Requeue: true}, nil
	}

	module := new(v1alpha1.Module)
	if err := r.client.Get(ctx, client.ObjectKey{Name: moduleConfig.Name}, module); err != nil {
		if apierrors.IsNotFound(err) {
			r.log.Warnf("the module '%s' not found", moduleConfig.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{Requeue: true}, nil
	}

	return r.processModule(ctx, moduleConfig, module)
}

func (r *reconciler) processModule(ctx context.Context, moduleConfig *v1alpha1.ModuleConfig, module *v1alpha1.Module) (ctrl.Result, error) {
	defer r.log.Debugf("the '%s' module config reconciled", moduleConfig.Name)

	// clear conflict metrics
	metricGroup := fmt.Sprintf("module_%s_at_conflict", module.Name)
	r.metricStorage.Grouped().ExpireGroupMetrics(metricGroup)

	enabled := module.ConditionStatus(v1alpha1.ModuleConditionEnabledByModuleConfig)

	if !moduleConfig.IsEnabled() {
		// disable module
		if enabled {
			r.log.Debugf("disable the '%s' module", moduleConfig.Name)
			err := utils.UpdateStatus[*v1alpha1.Module](ctx, r.client, module, func(module *v1alpha1.Module) bool {
				// modules in Conflict should not be installed, and they cannot receive events, so set Available phase manually
				if module.Status.Phase == v1alpha1.ModulePhaseConflict {
					module.Status.Phase = v1alpha1.ModulePhaseAvailable
					module.SetConditionFalse(v1alpha1.ModuleConditionEnabledByModuleManager, "", "")
					module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonNotInstalled, v1alpha1.ModuleMessageNotInstalled)
				}
				module.SetConditionFalse(v1alpha1.ModuleConditionEnabledByModuleConfig, v1alpha1.ModuleReasonDisabled, v1alpha1.ModuleMessageDisabled)
				module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonDisabled, v1alpha1.ModuleMessageDisabled)
				return true
			})
			if err != nil {
				r.log.Errorf("failed to disable the '%s' module: %v", module.Name, err)
				return ctrl.Result{Requeue: true}, nil
			}
		}

		// skip disabled modules
		r.log.Debugf("skip the '%s' disabled module", module.Name)
		return ctrl.Result{}, nil
	}

	if moduleConfig.IsEnabled() {
		// enable module
		if !enabled {
			r.log.Debugf("enable the '%s' module", moduleConfig.Name)
			err := utils.UpdateStatus[*v1alpha1.Module](ctx, r.client, module, func(module *v1alpha1.Module) bool {
				module.SetConditionTrue(v1alpha1.ModuleConditionEnabledByModuleConfig)
				return true
			})
			if err != nil {
				r.log.Errorf("failed to enable the '%s' module: %v", module.Name, err)
				return ctrl.Result{Requeue: true}, nil
			}
		}
	}

	// set finalizer
	err := utils.Update[*v1alpha1.ModuleConfig](ctx, r.client, moduleConfig, func(moduleConfig *v1alpha1.ModuleConfig) bool {
		// to handle delete event
		if !controllerutil.ContainsFinalizer(moduleConfig, v1alpha1.ModuleConfigFinalizer) {
			controllerutil.AddFinalizer(moduleConfig, v1alpha1.ModuleConfigFinalizer)
			return true
		}
		return false
	})
	if err != nil {
		r.log.Errorf("failed to set finalizer the '%s' module config: %v", moduleConfig.Name, err)
		return ctrl.Result{Requeue: true}, nil
	}

	// skip system modules
	if module.Name == moduleDeckhouse || module.Name == moduleGlobal {
		r.log.Debugf("skip the '%s' system module", module.Name)
		return ctrl.Result{}, nil
	}

	// skip embedded modules
	if module.IsEmbedded() {
		r.log.Debugf("skip the '%s' embedded module", module.Name)
		return ctrl.Result{}, nil
	}

	updatePolicy := module.Properties.UpdatePolicy
	// change update policy by module config
	if updatePolicy != moduleConfig.Spec.UpdatePolicy {
		updatePolicy = moduleConfig.Spec.UpdatePolicy
	}

	// change source by module config
	if moduleConfig.Spec.Source != "" && module.Properties.Source != moduleConfig.Spec.Source {
		if err = r.changeModuleSource(ctx, module, moduleConfig.Spec.Source, updatePolicy); err != nil {
			r.log.Debugf("failed to change source for the '%s' module: %v", module.Name, err)
			return ctrl.Result{Requeue: true}, nil
		}
	}

	if module.Properties.Source == "" {
		// change source by available source
		if len(module.Properties.AvailableSources) == 1 {
			if err = r.changeModuleSource(ctx, module, module.Properties.AvailableSources[0], updatePolicy); err != nil {
				r.log.Debugf("failed to change source for the '%s' module: %v", module.Name, err)
				return ctrl.Result{Requeue: true}, nil
			}
		}

		if len(module.Properties.AvailableSources) > 1 {
			// set conflict if there are several available sources
			err = utils.UpdateStatus[*v1alpha1.Module](ctx, r.client, module, func(module *v1alpha1.Module) bool {
				module.Status.Phase = v1alpha1.ModulePhaseConflict
				module.SetConditionFalse(v1alpha1.ModuleConditionEnabledByModuleManager, v1alpha1.ModuleReasonConflict, v1alpha1.ModuleMessageConflict)
				module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonConflict, v1alpha1.ModuleMessageConflict)
				return true
			})
			if err != nil {
				r.log.Errorf("failed to set conlflict to the '%s' module: %v", module.Name, err)
				return ctrl.Result{Requeue: true}, nil
			}
			// fire alert at Conflict
			r.metricStorage.Grouped().GaugeSet(metricGroup, "d8_module_at_conflict", 1.0, map[string]string{
				"moduleName": module.Name,
			})
		}
	}

	// update only the update policy if nothing else has changed
	err = utils.Update[*v1alpha1.Module](ctx, r.client, module, func(module *v1alpha1.Module) bool {
		if module.Properties.UpdatePolicy != updatePolicy {
			module.Properties.UpdatePolicy = updatePolicy
			return true
		}
		return false
	})
	if err != nil {
		r.log.Errorf("failed to update the '%s' module`s update policy: %v", module.Name, err)
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

func (r *reconciler) deleteModuleConfig(ctx context.Context, moduleConfig *v1alpha1.ModuleConfig) (ctrl.Result, error) {
	// send event to addon-operator
	r.handler.HandleEvent(ctx, moduleConfig, config.EventDelete)

	module := new(v1alpha1.Module)
	if err := r.client.Get(ctx, client.ObjectKey{Name: moduleConfig.Name}, module); err != nil {
		if apierrors.IsNotFound(err) {
			r.log.Warnf("the module '%s' not found", moduleConfig.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// skip embedded modules
	if module.IsEmbedded() {
		r.log.Debugf("skip the '%s' embedded module", module.Name)
		return ctrl.Result{}, nil
	}

	// skip system modules
	if module.Name == moduleDeckhouse || module.Name == moduleGlobal {
		r.log.Debugf("skip the '%s' system module", module.Name)
		return ctrl.Result{}, nil
	}

	enabled := module.ConditionStatus(v1alpha1.ModuleConditionEnabledByModuleConfig)

	// disable module
	if enabled {
		err := utils.UpdateStatus[*v1alpha1.Module](ctx, r.client, module, func(module *v1alpha1.Module) bool {
			module.SetConditionFalse(v1alpha1.ModuleConditionEnabledByModuleConfig, v1alpha1.ModuleReasonDisabled, v1alpha1.ModuleMessageDisabled)
			module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonDisabled, v1alpha1.ModuleMessageDisabled)
			return true
		})
		if err != nil {
			r.log.Errorf("failed to disable the '%s' module: %v", module.Name, err)
			return ctrl.Result{Requeue: true}, nil
		}
	}

	err := utils.Update[*v1alpha1.Module](ctx, r.client, module, func(module *v1alpha1.Module) bool {
		module.Properties.UpdatePolicy = ""
		module.Properties.Source = ""
		return true
	})
	if err != nil {
		r.log.Errorf("failed to update the '%s' module: %v", module.Name, err)
		return ctrl.Result{Requeue: true}, nil
	}

	err = utils.Update[*v1alpha1.ModuleConfig](ctx, r.client, moduleConfig, func(moduleConfig *v1alpha1.ModuleConfig) bool {
		if controllerutil.ContainsFinalizer(moduleConfig, v1alpha1.ModuleConfigFinalizer) {
			controllerutil.RemoveFinalizer(moduleConfig, v1alpha1.ModuleConfigFinalizer)
			return true
		}
		return false
	})
	if err != nil {
		r.log.Errorf("failed to remove finalizer from the '%s' module config: %v", moduleConfig.Name, err)
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

func (r *reconciler) changeModuleSource(ctx context.Context, module *v1alpha1.Module, source, updatePolicy string) error {
	r.log.Debugf("set new '%s' source to the '%s' module", source, module.Name)
	err := utils.Update[*v1alpha1.Module](ctx, r.client, module, func(module *v1alpha1.Module) bool {
		module.Properties.Source = source
		module.Properties.UpdatePolicy = updatePolicy
		return true
	})
	if err != nil {
		return fmt.Errorf("update the '%s' module: %w", module.Name, err)
	}
	return nil
}
