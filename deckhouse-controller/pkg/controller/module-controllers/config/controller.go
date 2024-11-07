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
	"slices"
	"sync"
	"time"

	"github.com/flant/addon-operator/pkg/kube_config_manager/config"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules/events"

	metricstorage "github.com/flant/shell-operator/pkg/metric_storage"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

	moduleConfigFinalizer = "modules.deckhouse.io/module-config"

	deleteReleasesAfter = 5 * time.Minute
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

	configController, err := controller.New(controllerName, runtimeManager, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(runtimeManager).
		For(&v1alpha1.ModuleConfig{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
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
	r.log.Debugf("handle the event for the module '%s' config", moduleConfig.Name)
	r.handler.HandleEvent(ctx, moduleConfig, config.EventUpdate)

	r.log.Debugf("refresh the '%s' module config status", moduleConfig.Name)
	if err := r.refreshModuleConfigStatus(ctx, moduleConfig.Name); err != nil {
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

	enabled := module.ConditionStatus(v1alpha1.ModuleConditionEnabledByModuleConfig)

	if moduleConfig.IsEnabled() {
		// enable module
		if !enabled {
			err := utils.UpdateStatus[*v1alpha1.Module](ctx, r.client, module, func(module *v1alpha1.Module) bool {
				module.SetConditionTrue(v1alpha1.ModuleConditionEnabledByModuleConfig)
				return true
			})
			if err != nil {
				r.log.Errorf("failed to enable the '%s' module: %v", module.Name, err)
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{Requeue: true}, nil
		}
	}

	if !moduleConfig.IsEnabled() {
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
			return ctrl.Result{}, nil
		}
		// skip disabled modules
		r.log.Debugf("skip the '%s' disabled module", module.Name)
		return ctrl.Result{}, nil
	}

	// skip embedded modules
	if module.IsEmbedded() {
		r.log.Debugf("skip the '%s' embedded module", module.Name)
		return ctrl.Result{}, nil
	}

	// skip system modules
	if module.Name == "deckhouse" || module.Name == "global" {
		r.log.Debugf("skip the '%s' system module", module.Name)
		return ctrl.Result{}, nil
	}

	err := utils.Update[*v1alpha1.ModuleConfig](ctx, r.client, moduleConfig, func(obj *v1alpha1.ModuleConfig) bool {
		// to handle delete event
		if !controllerutil.ContainsFinalizer(moduleConfig, moduleConfigFinalizer) {
			controllerutil.AddFinalizer(moduleConfig, moduleConfigFinalizer)
			return true
		}
		return false
	})
	if err != nil {
		r.log.Errorf("failed to set finalizer the '%s' module: %v", module.Name, err)
		return ctrl.Result{Requeue: true}, nil
	}

	updatePolicy := module.Properties.UpdatePolicy
	// change update policy by module config
	if moduleConfig.Spec.UpdatePolicy != module.Properties.UpdatePolicy {
		updatePolicy = moduleConfig.Spec.UpdatePolicy
	}

	// change source by module config
	if moduleConfig.Spec.Source != "" && module.Properties.Source != moduleConfig.Spec.Source {
		// TODO(ipaqsa): move to validation webhook
		if !slices.Contains(module.Properties.AvailableSources, moduleConfig.Spec.Source) {
			moduleConfig.Status.Message = "The wrong source"
			if err = r.client.Status().Update(ctx, moduleConfig); err != nil {
				r.log.Errorf("failed to update the '%s' module conifg status: %v", module.Name, err)
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, nil
		}
		return r.changeModuleSource(ctx, module, moduleConfig.Spec.Source, updatePolicy)
	}

	if module.Properties.Source == "" {
		// change source by available source
		if len(module.Properties.AvailableSources) == 1 {
			return r.changeModuleSource(ctx, module, module.Properties.AvailableSources[0], updatePolicy)
		}

		// set conflict
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
	}

	// update only the update policy if nothing else has changed
	err = utils.Update[*v1alpha1.Module](ctx, r.client, module, func(obj *v1alpha1.Module) bool {
		if module.Properties.UpdatePolicy != updatePolicy {
			module.Properties.UpdatePolicy = updatePolicy
			return true
		}
		return false
	})
	if err != nil {
		r.log.Errorf("failed to update the '%s' module update policy: %v", module.Name, err)
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

func (r *reconciler) deleteModuleConfig(ctx context.Context, moduleConfig *v1alpha1.ModuleConfig) (ctrl.Result, error) {
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
	if module.Name == "deckhouse" || module.Name == "global" {
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

	err := utils.Update[*v1alpha1.Module](ctx, r.client, module, func(obj *v1alpha1.Module) bool {
		module.Properties.UpdatePolicy = ""
		module.Properties.Source = ""
		return true
	})
	if err != nil {
		r.log.Errorf("failed to update the '%s' module: %v", module.Name, err)
		return ctrl.Result{Requeue: true}, nil
	}

	err = utils.Update[*v1alpha1.ModuleConfig](ctx, r.client, moduleConfig, func(obj *v1alpha1.ModuleConfig) bool {
		if controllerutil.ContainsFinalizer(moduleConfig, moduleConfigFinalizer) {
			controllerutil.RemoveFinalizer(moduleConfig, moduleConfigFinalizer)
			return true
		}
		return false
	})
	if err != nil {
		r.log.Errorf("failed to remove finalizer from the '%s' module: %v", module.Name, err)
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

func (r *reconciler) changeModuleSource(ctx context.Context, module *v1alpha1.Module, source, updatePolicy string) (ctrl.Result, error) {
	r.log.Debugf("set new '%s' source to the '%s' module", source, module.Name)
	err := utils.UpdateStatus[*v1alpha1.Module](ctx, r.client, module, func(module *v1alpha1.Module) bool {
		module.Status.Phase = v1alpha1.ModulePhaseDownloading
		module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonChangeSource, v1alpha1.ModuleMessageChangeSource)
		return true
	})
	if err != nil {
		r.log.Errorf("failed to change the module source to the '%s' module: %v", module.Name, err)
		return ctrl.Result{Requeue: true}, nil
	}
	err = utils.Update[*v1alpha1.Module](ctx, r.client, module, func(module *v1alpha1.Module) bool {
		module.Properties.Source = source
		module.Properties.UpdatePolicy = updatePolicy
		return true
	})
	if err != nil {
		r.log.Errorf("failed to change the module source to the '%s' module: %v", module.Name, err)
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, err
}
