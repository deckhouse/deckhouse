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
	"log/slog"
	"sync"
	"time"

	"github.com/flant/addon-operator/pkg/kube_config_manager/config"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules/events"
	"github.com/flant/shell-operator/pkg/metric"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrlhandler "sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/confighandler"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	d8edition "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/deckhouse/deckhouse/go_lib/configtools"
	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	"github.com/deckhouse/deckhouse/go_lib/telemetry"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	controllerName = "d8-module-config-controller"

	maxConcurrentReconciles = 3

	moduleNotFoundInterval = 3 * time.Minute

	moduleDeckhouse = "deckhouse"
	moduleGlobal    = "global"

	obsoleteConfigMetricGroup = "obsoleteVersion_%s"
	moduleConflictMetricGroup = "module_%s_at_conflict"
)

func RegisterController(
	runtimeManager manager.Manager,
	mm moduleManager,
	conversionsStore *conversion.ConversionsStore,
	edition *d8edition.Edition,
	handler *confighandler.Handler,
	ms metric.Storage,
	exts *extenders.ExtendersStack,
	logger *log.Logger,
) error {
	r := &reconciler{
		init:             new(sync.WaitGroup),
		client:           runtimeManager.GetClient(),
		logger:           logger,
		handler:          handler,
		conversionsStore: conversionsStore,
		moduleManager:    mm,
		edition:          edition,
		metricStorage:    ms,
		configValidator:  configtools.NewValidator(mm, conversionsStore),
		exts:             exts,
	}

	r.init.Add(1)

	// sync modules
	if err := runtimeManager.Add(manager.RunnableFunc(r.preflight)); err != nil {
		return fmt.Errorf("add preflight: %w", err)
	}

	configController, err := controller.New(controllerName, runtimeManager, controller.Options{
		MaxConcurrentReconciles: maxConcurrentReconciles,
		NeedLeaderElection:      ptr.To(false),
		Reconciler:              r,
	})
	if err != nil {
		return fmt.Errorf("create controller: %w", err)
	}

	return ctrl.NewControllerManagedBy(runtimeManager).
		For(&v1alpha1.ModuleConfig{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})).
		Watches(&v1alpha1.Module{}, ctrlhandler.EnqueueRequestsFromMapFunc(func(_ context.Context, obj client.Object) []reconcile.Request {
			return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: obj.(*v1alpha1.Module).Name}}}
		}), builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(_ event.CreateEvent) bool {
				return true
			},
			UpdateFunc: func(_ event.UpdateEvent) bool { return false },
			DeleteFunc: func(_ event.DeleteEvent) bool {
				return false
			},
			GenericFunc: func(_ event.GenericEvent) bool {
				return false
			},
		})).
		Complete(configController)
}

type reconciler struct {
	init             *sync.WaitGroup
	client           client.Client
	conversionsStore *conversion.ConversionsStore
	edition          *d8edition.Edition
	handler          *confighandler.Handler
	moduleManager    moduleManager
	metricStorage    metric.Storage
	configValidator  *configtools.Validator
	exts             *extenders.ExtendersStack
	logger           *log.Logger
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

	r.logger.Debug("reconciling module config", slog.String("name", req.Name))
	moduleConfig := new(v1alpha1.ModuleConfig)
	if err := r.client.Get(ctx, client.ObjectKey{Name: req.Name}, moduleConfig); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Warn("module config not found", slog.String("name", req.Name))
			return ctrl.Result{}, nil
		}

		r.logger.Error("failed to get module config", slog.String("name", req.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	// handle delete event
	if !moduleConfig.DeletionTimestamp.IsZero() {
		r.logger.Debug("deleting module config", slog.String("name", req.Name))
		return r.deleteModuleConfig(ctx, moduleConfig)
	}

	// handle create/update events
	return r.handleModuleConfig(ctx, moduleConfig)
}

// preflight waits until config kube config manager is started and runs module event loop
func (r *reconciler) preflight(ctx context.Context) error {
	r.logger.Debug("wait until kube config manager started")
	if err := wait.PollUntilContextCancel(ctx, 100*time.Millisecond, true, func(_ context.Context) (bool, error) {
		return r.handler.ModuleConfigChannelIsSet(), nil
	}); err != nil {
		return fmt.Errorf("wait until kube config manager started: %v", err)
	}

	r.init.Done()

	return r.runModuleEventLoop(ctx)
}

// runModuleEventLoop triggers module refreshing at any event from addon-operator
func (r *reconciler) runModuleEventLoop(ctx context.Context) error {
	for moduleEvent := range r.moduleManager.GetModuleEventsChannel() {
		if moduleEvent.ModuleName != "" {
			if err := r.refreshModule(ctx, moduleEvent.ModuleName); err != nil {
				r.logger.Debug("failed to handle the event for the module", slog.String("name", moduleEvent.ModuleName), log.Err(err))
			}
		}
	}

	return nil
}

func (r *reconciler) handleModuleConfig(ctx context.Context, moduleConfig *v1alpha1.ModuleConfig) (ctrl.Result, error) {
	// TODO: remove after 1.73+
	if controllerutil.ContainsFinalizer(moduleConfig, v1alpha1.ModuleConfigFinalizerOld) {
		patch := client.MergeFrom(moduleConfig.DeepCopy())
		controllerutil.RemoveFinalizer(moduleConfig, v1alpha1.ModuleConfigFinalizerOld)

		if err := r.client.Patch(ctx, moduleConfig, patch); err != nil {
			r.logger.Error("failed to remove old finalizer", slog.String("name", moduleConfig.Name), log.Err(err))
			return ctrl.Result{}, err
		}
	}

	// send an event to addon-operator only if the module exists, or it is the global one
	basicModule := r.moduleManager.GetModule(moduleConfig.Name)
	if moduleConfig.Name == moduleGlobal || basicModule != nil {
		r.handler.HandleEvent(moduleConfig, config.EventUpdate)
	}

	if err := r.refreshModuleConfig(ctx, moduleConfig.Name); err != nil {
		return ctrl.Result{Requeue: true}, nil
	}

	module := new(v1alpha1.Module)
	if err := r.client.Get(ctx, client.ObjectKey{Name: moduleConfig.Name}, module); err != nil {
		if apierrors.IsNotFound(err) {
			if moduleConfig.Name != moduleGlobal {
				r.logger.Warn("module not found", slog.String("name", moduleConfig.Name))
				err = utils.UpdateStatus[*v1alpha1.ModuleConfig](ctx, r.client, moduleConfig, func(moduleConfig *v1alpha1.ModuleConfig) bool {
					moduleConfig.Status.Message = v1alpha1.ModuleConfigMessageUnknownModule
					return true
				})
				if err != nil {
					r.logger.Error("failed to update module config", slog.String("name", moduleConfig.Name), log.Err(err))
					return ctrl.Result{}, err
				}
			}

			return ctrl.Result{RequeueAfter: moduleNotFoundInterval}, nil
		}

		return ctrl.Result{}, err
	}

	return r.processModule(ctx, moduleConfig, module)
}

func (r *reconciler) processModule(ctx context.Context, moduleConfig *v1alpha1.ModuleConfig, module *v1alpha1.Module) (ctrl.Result, error) {
	defer r.logger.Debug("module config reconciled", slog.String("name", moduleConfig.Name))

	// clear conflict metrics
	metricGroup := fmt.Sprintf(moduleConflictMetricGroup, module.Name)
	r.metricStorage.Grouped().ExpireGroupMetrics(metricGroup)

	if err := r.addFinalizer(ctx, moduleConfig); err != nil {
		r.logger.Error("failed to add finalizer", slog.String("module", module.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	if !moduleConfig.IsEnabled() {
		// delete all pending releases for EnabledByModuleConfig disabled modules
		if module.IsCondition(v1alpha1.ModuleConditionEnabledByModuleConfig, corev1.ConditionTrue) {
			releases := new(v1alpha1.ModuleReleaseList)
			selector := client.MatchingLabels{v1alpha1.ModuleReleaseLabelModule: module.Name}
			if err := r.client.List(ctx, releases, selector); err != nil {
				r.logger.Warn("list module releases", slog.String("module", module.Name), log.Err(err))
				return ctrl.Result{}, fmt.Errorf("list module releases: %w", err)
			}

			pendingReleases := make([]*v1alpha1.ModuleRelease, 0)
			for _, release := range releases.Items {
				if release.GetPhase() == v1alpha1.ModuleReleasePhasePending {
					pendingReleases = append(pendingReleases, &release)
				}
			}

			if len(pendingReleases) > 0 {
				for _, release := range pendingReleases {
					err := r.client.Delete(ctx, release)
					if err != nil && !apierrors.IsNotFound(err) {
						r.logger.Error("failed to delete pending release", slog.String("pending_release", release.Name), log.Err(err))
						return ctrl.Result{}, err
					}
				}
			}
		}

		if err := r.disableModule(ctx, module); err != nil {
			r.logger.Error("failed to disable the module", slog.String("module", module.Name), log.Err(err))
			return ctrl.Result{}, err
		}

		err := utils.Update[*v1alpha1.ModuleConfig](ctx, r.client, moduleConfig, func(moduleConfig *v1alpha1.ModuleConfig) bool {
			if _, ok := moduleConfig.ObjectMeta.Annotations[v1alpha1.ModuleConfigAnnotationAllowDisable]; ok {
				delete(moduleConfig.ObjectMeta.Annotations, v1alpha1.ModuleConfigAnnotationAllowDisable)
				return true
			}
			return false
		})
		if err != nil {
			r.logger.Error("failed to remove allow disabled annotation for module config", slog.String("name", moduleConfig.Name), log.Err(err))
			return ctrl.Result{}, err
		}

		// skip disabled modules
		r.logger.Debug("skip disabled module", slog.String("name", module.Name))
		return ctrl.Result{}, nil
	}

	if moduleConfig.IsEnabled() {
		if err := r.enableModule(ctx, module); err != nil {
			r.logger.Error("failed to enable the module", slog.String("module", module.Name), log.Err(err))
			return ctrl.Result{}, err
		}
	}

	if module.IsExperimental() {
		r.metricStorage.GaugeSet(telemetry.WrapName(metrics.ExperimentalModuleIsEnabled), 1.0, map[string]string{"module": moduleConfig.GetName()})
	}

	if err := r.addFinalizer(ctx, moduleConfig); err != nil {
		r.logger.Error("failed to add finalizer", slog.String("module", module.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	// skip system modules
	if module.Name == moduleDeckhouse || module.Name == moduleGlobal {
		r.logger.Debug("skip the system module", slog.String("name", module.Name))
		return ctrl.Result{}, nil
	}

	// skip embedded modules
	if module.IsEmbedded() {
		r.logger.Debug("skip embedded module", slog.String("name", module.Name))
		return ctrl.Result{}, nil
	}

	updatePolicy := module.Properties.UpdatePolicy
	// change update policy by module config
	if updatePolicy != moduleConfig.Spec.UpdatePolicy {
		updatePolicy = moduleConfig.Spec.UpdatePolicy
	}

	// change source by module config
	if moduleConfig.Spec.Source != "" && module.Properties.Source != moduleConfig.Spec.Source {
		if err := r.changeModuleSource(ctx, module, moduleConfig.Spec.Source, updatePolicy); err != nil {
			r.logger.Debug("failed to change source for the module", slog.String("name", module.Name), log.Err(err))
			return ctrl.Result{}, err
		}
	}

	if module.Properties.Source == "" {
		// change source by available source
		if len(module.Properties.AvailableSources) == 1 {
			if err := r.changeModuleSource(ctx, module, module.Properties.AvailableSources[0], updatePolicy); err != nil {
				r.logger.Debug("failed to change source for module", slog.String("name", module.Name), log.Err(err))
				return ctrl.Result{}, err
			}
		}

		// set conflict if there are several available sources
		if len(module.Properties.AvailableSources) > 1 {
			err := utils.UpdateStatus[*v1alpha1.Module](ctx, r.client, module, func(module *v1alpha1.Module) bool {
				module.Status.Phase = v1alpha1.ModulePhaseConflict
				module.SetConditionFalse(v1alpha1.ModuleConditionEnabledByModuleManager, "", "")
				module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonConflict, v1alpha1.ModuleMessageConflict)
				return true
			})
			if err != nil {
				r.logger.Error("failed to set conflict to module", slog.String("name", module.Name), log.Err(err))
				return ctrl.Result{}, err
			}
			// fire alert at Conflict
			r.metricStorage.Grouped().GaugeSet(metricGroup, "d8_module_at_conflict", 1.0, map[string]string{
				"moduleName": module.Name,
			})
		}
	}

	// update only the update policy if nothing else has changed
	err := utils.Update[*v1alpha1.Module](ctx, r.client, module, func(module *v1alpha1.Module) bool {
		if module.Properties.UpdatePolicy != updatePolicy {
			module.Properties.UpdatePolicy = updatePolicy
			return true
		}
		return false
	})
	if err != nil {
		r.logger.Error("failed to update module`s update policy", slog.String("name", module.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *reconciler) deleteModuleConfig(ctx context.Context, moduleConfig *v1alpha1.ModuleConfig) (ctrl.Result, error) {
	// send event to addon-operator
	r.handler.HandleEvent(moduleConfig, config.EventDelete)

	// clear obsolete metrics
	metricGroup := fmt.Sprintf(obsoleteConfigMetricGroup, moduleConfig.Name)
	r.metricStorage.Grouped().ExpireGroupMetrics(metricGroup)

	// clear conflict metrics
	metricGroup = fmt.Sprintf(moduleConflictMetricGroup, moduleConfig.Name)
	r.metricStorage.Grouped().ExpireGroupMetrics(metricGroup)

	r.metricStorage.GaugeSet(telemetry.WrapName(metrics.ExperimentalModuleIsEnabled), 0.0, map[string]string{"module": moduleConfig.GetName()})

	module := new(v1alpha1.Module)
	if err := r.client.Get(ctx, client.ObjectKey{Name: moduleConfig.Name}, module); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Warn("module not found", slog.String("name", moduleConfig.Name))
			if err = r.removeFinalizer(ctx, moduleConfig); err != nil {
				r.logger.Error("failed to remove finalizer", slog.String("module", moduleConfig.Name), log.Err(err))
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, nil
		}

		r.logger.Error("failed to get module", slog.String("name", moduleConfig.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	// skip system modules
	if module.Name == moduleDeckhouse || module.Name == moduleGlobal {
		r.logger.Debug("skip system module", slog.String("name", module.Name))
		return ctrl.Result{}, nil
	}

	// disable module
	if err := r.disableModule(ctx, module); err != nil {
		r.logger.Error("failed to disable the module", slog.String("module", module.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	// clear downloaded module
	if !module.IsEmbedded() && !module.IsEnabledByBundle(r.edition.Name, r.edition.Bundle) {
		err := utils.Update[*v1alpha1.Module](ctx, r.client, module, func(module *v1alpha1.Module) bool {
			module.Properties.UpdatePolicy = ""
			module.Properties.Source = ""
			return true
		})
		if err != nil {
			r.logger.Error("failed to update the module", slog.String("module", module.Name), log.Err(err))
			return ctrl.Result{}, err
		}
	}

	err := utils.UpdateStatus[*v1alpha1.Module](ctx, r.client, module, func(module *v1alpha1.Module) bool {
		module.SetConditionUnknown(v1alpha1.ModuleConditionEnabledByModuleConfig, "", "")

		return true
	})
	if err != nil {
		r.logger.Error("failed to update module", slog.String("name", module.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	if err := r.removeFinalizer(ctx, moduleConfig); err != nil {
		r.logger.Error("failed to remove finalizer from ModuleConfig", slog.String("module", moduleConfig.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *reconciler) changeModuleSource(ctx context.Context, module *v1alpha1.Module, source, updatePolicy string) error {
	r.logger.Debug("set new source to the module", slog.String("moduleSource", source), slog.String("module", module.Name))
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

// addFinalizer adds finalizer to the module config to handle the delete event
func (r *reconciler) addFinalizer(ctx context.Context, config *v1alpha1.ModuleConfig) error {
	return utils.Update[*v1alpha1.ModuleConfig](ctx, r.client, config, func(config *v1alpha1.ModuleConfig) bool {
		if !controllerutil.ContainsFinalizer(config, v1alpha1.ModuleConfigFinalizer) {
			controllerutil.AddFinalizer(config, v1alpha1.ModuleConfigFinalizer)
			return true
		}

		return false
	})
}

func (r *reconciler) removeFinalizer(ctx context.Context, config *v1alpha1.ModuleConfig) error {
	return utils.Update[*v1alpha1.ModuleConfig](ctx, r.client, config, func(moduleConfig *v1alpha1.ModuleConfig) bool {
		var needsUpdate bool
		if controllerutil.ContainsFinalizer(moduleConfig, v1alpha1.ModuleConfigFinalizer) {
			controllerutil.RemoveFinalizer(moduleConfig, v1alpha1.ModuleConfigFinalizer)
			needsUpdate = true
		}

		if _, ok := moduleConfig.ObjectMeta.Annotations[v1alpha1.ModuleConfigAnnotationAllowDisable]; ok {
			delete(moduleConfig.ObjectMeta.Annotations, v1alpha1.ModuleConfigAnnotationAllowDisable)
			needsUpdate = true
		}

		return needsUpdate
	})
}

func (r *reconciler) disableModule(ctx context.Context, module *v1alpha1.Module) error {
	r.logger.Debug("disable the module", slog.String("module", module.Name))
	return utils.UpdateStatus[*v1alpha1.Module](ctx, r.client, module, func(module *v1alpha1.Module) bool {
		if module.IsCondition(v1alpha1.ModuleConditionEnabledByModuleConfig, corev1.ConditionFalse) {
			return false
		}

		switch module.Status.Phase {
		case v1alpha1.ModulePhaseConflict,
			v1alpha1.ModulePhaseDownloading,
			v1alpha1.ModulePhaseDownloadingError:
			// modules in Conflict should not be installed, and they cannot receive events, so set Available phase manually
			// same thing if module is not installed
			module.Status.Phase = v1alpha1.ModulePhaseAvailable
			module.SetConditionFalse(v1alpha1.ModuleConditionEnabledByModuleManager, "", "")
			module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonNotInstalled, v1alpha1.ModuleMessageNotInstalled)
		default:
			if !module.IsEnabledByBundle(r.edition.Name, r.edition.Bundle) {
				module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonDisabled, v1alpha1.ModuleMessageDisabled)
			}
		}

		module.SetConditionFalse(v1alpha1.ModuleConditionEnabledByModuleConfig, "", "")
		module.SetConditionUnknown(v1alpha1.ModuleConditionLastReleaseDeployed, "", "")

		return true
	})
}

func (r *reconciler) enableModule(ctx context.Context, module *v1alpha1.Module) error {
	r.logger.Debug("enable the module", slog.String("module", module.Name))
	return utils.UpdateStatus[*v1alpha1.Module](ctx, r.client, module, func(module *v1alpha1.Module) bool {
		if module.IsCondition(v1alpha1.ModuleConditionEnabledByModuleConfig, corev1.ConditionTrue) {
			return false
		}
		module.SetConditionTrue(v1alpha1.ModuleConditionEnabledByModuleConfig)

		return true
	})
}
