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
	"strings"
	"sync"
	"time"

	"github.com/flant/addon-operator/pkg/kube_config_manager/config"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules/events"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/controller/pkgsync"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/confighandler"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	d8edition "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/deckhouse/deckhouse/go_lib/configtools"
	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	"github.com/deckhouse/deckhouse/go_lib/telemetry"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

const (
	controllerName = "d8-module-config-controller"

	maxConcurrentReconciles = 3

	moduleNotFoundInterval = 3 * time.Minute

	moduleDeckhouse = "deckhouse"
	moduleGlobal    = "global"
)

func RegisterController(
	runtimeManager manager.Manager,
	mm moduleManager,
	pm packageManager,
	conversionsStore *conversion.ConversionsStore,
	edition *d8edition.Edition,
	handler *confighandler.Handler,
	ms metricsstorage.Storage,
	exts extenders.IExtendersStack,
	logger *log.Logger,
) error {
	r := &reconciler{
		init:             new(sync.WaitGroup),
		client:           runtimeManager.GetClient(),
		logger:           logger,
		handler:          handler,
		conversionsStore: conversionsStore,
		moduleManager:    mm,
		packageManager:   pm,
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

	if err := ctrl.NewControllerManagedBy(runtimeManager).
		Named(controllerName).
		For(&v1alpha1.ModuleConfig{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})).
		// a module that gets its object, or its first package, picks up the config that waited
		// for it; a module without a config has nothing to reconcile
		Watches(&v1alpha2.Module{}, ctrlhandler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
			if err := r.client.Get(ctx, client.ObjectKey{Name: obj.GetName()}, new(v1alpha1.ModuleConfig)); err != nil {
				return nil
			}

			return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: obj.GetName()}}}
		}), builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(_ event.CreateEvent) bool {
				return true
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				oldModule, ok := updateEvent.ObjectOld.(*v1alpha2.Module)
				if !ok {
					return false
				}

				newModule, ok := updateEvent.ObjectNew.(*v1alpha2.Module)
				if !ok {
					return false
				}

				return !oldModule.IsInstalled() && newModule.IsInstalled()
			},
			DeleteFunc: func(_ event.DeleteEvent) bool {
				return false
			},
			GenericFunc: func(_ event.GenericEvent) bool {
				return false
			},
		})).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: maxConcurrentReconciles,
			NeedLeaderElection:      ptr.To(false),
		}).
		Complete(r); err != nil {
		return fmt.Errorf("complete: %w", err)
	}
	return nil
}

type reconciler struct {
	init             *sync.WaitGroup
	client           client.Client
	conversionsStore *conversion.ConversionsStore
	edition          *d8edition.Edition
	handler          *confighandler.Handler
	moduleManager    moduleManager
	packageManager   packageManager
	metricStorage    metricsstorage.Storage
	configValidator  *configtools.Validator
	exts             extenders.IExtendersStack
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

type packageManager interface {
	UpdateModulesSettings(name string, settingsVersion int, settings addonutils.Values, maintenance string, enabled *bool)
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
		return ctrl.Result{}, fmt.Errorf("get: %w", err)
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
	// send an event to addon-operator only if the module exists, or it is the global one
	basicModule := r.moduleManager.GetModule(moduleConfig.Name)
	if moduleConfig.Name == moduleGlobal || basicModule != nil {
		r.logger.Debug("send event to operator", slog.String("name", moduleConfig.Name), slog.Bool("enabled", moduleConfig.IsEnabled()))
		r.handler.HandleEvent(moduleConfig, config.EventUpdate)
	}

	// update modules settings in the package manager
	r.packageManager.UpdateModulesSettings(
		moduleConfig.Name,
		moduleConfig.Spec.Version,
		moduleConfig.Spec.Settings.GetMap(),
		moduleConfig.Spec.Maintenance,
		moduleConfig.Spec.Enabled)

	if err := r.refreshModuleConfig(ctx, moduleConfig.Name); err != nil {
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	if moduleConfig.Name == moduleGlobal {
		return ctrl.Result{}, nil
	}

	module := new(v1alpha2.Module)
	if err := r.client.Get(ctx, client.ObjectKey{Name: moduleConfig.Name}, module); err != nil {
		if apierrors.IsNotFound(err) {
			return r.handleNotInstalledModule(ctx, moduleConfig)
		}

		return ctrl.Result{}, err
	}

	return r.processModule(ctx, moduleConfig, module)
}

// handleNotInstalledModule settles a config whose module has no object: a module some source
// offers is waiting for its first deploy, and the Module create event brings the config back,
// while a name no source knows is reported and checked again later.
func (r *reconciler) handleNotInstalledModule(ctx context.Context, moduleConfig *v1alpha1.ModuleConfig) (ctrl.Result, error) {
	moduleSourceNames, err := r.listingModuleSources(ctx, moduleConfig.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	if len(moduleSourceNames) == 0 {
		r.logger.Warn("module not found", slog.String("name", moduleConfig.Name))
		err = utils.UpdateStatus[*v1alpha1.ModuleConfig](ctx, r.client, moduleConfig, func(moduleConfig *v1alpha1.ModuleConfig) bool {
			moduleConfig.Status.Message = v1alpha1.ModuleConfigMessageUnknownModule
			return true
		})
		if err != nil {
			r.logger.Error("failed to update module config", slog.String("name", moduleConfig.Name), log.Err(err))
			return ctrl.Result{}, err
		}

		return ctrl.Result{RequeueAfter: moduleNotFoundInterval}, nil
	}

	if err := r.addFinalizer(ctx, moduleConfig); err != nil {
		r.logger.Error("failed to add finalizer", slog.String("module", moduleConfig.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	if _, err := r.reportConflict(ctx, moduleConfig, moduleSourceNames); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reportConflict tells the config about the sources of a module nothing installed: the
// message and the alert while the config enables the module, several sources offer it and
// none is picked, since no source installs such a module; a stale conflict message is
// cleared and the alert withdrawn otherwise. Reports whether the module is in conflict.
func (r *reconciler) reportConflict(ctx context.Context, moduleConfig *v1alpha1.ModuleConfig, moduleSourceNames []string) (bool, error) {
	metricGroup := fmt.Sprintf(metrics.ModuleConflictMetricGroupTemplate, moduleConfig.Name)
	r.metricStorage.Grouped().ExpireGroupMetrics(metricGroup)

	conflict := utils.HasModuleSourceConflict(moduleConfig.IsEnabled(), pkgsync.ConfiguredModuleSource(moduleConfig), moduleSourceNames)

	message := ""
	if conflict {
		r.logger.Debug("module has several available sources", slog.String("name", moduleConfig.Name))
		message = fmt.Sprintf("%s: %s", v1alpha1.ModuleMessageConflict, strings.Join(moduleSourceNames, ", "))
	}

	err := utils.UpdateStatus[*v1alpha1.ModuleConfig](ctx, r.client, moduleConfig, func(moduleConfig *v1alpha1.ModuleConfig) bool {
		// other writers own the other messages
		if moduleConfig.Status.Message == message || (message == "" && !strings.HasPrefix(moduleConfig.Status.Message, v1alpha1.ModuleMessageConflict)) {
			return false
		}

		moduleConfig.Status.Message = message

		return true
	})
	if err != nil {
		r.logger.Error("failed to update module config", slog.String("name", moduleConfig.Name), log.Err(err))
		return false, err
	}

	if conflict {
		// fire alert at Conflict
		r.metricStorage.Grouped().GaugeSet(metricGroup, metrics.D8ModuleAtConflict, 1.0, map[string]string{
			"module": moduleConfig.Name,
		})
	}

	return conflict, nil
}

// listingModuleSources returns the module sources that list the module.
func (r *reconciler) listingModuleSources(ctx context.Context, moduleName string) ([]string, error) {
	moduleSources := new(v1alpha1.ModuleSourceList)
	if err := r.client.List(ctx, moduleSources); err != nil {
		return nil, fmt.Errorf("list module sources: %w", err)
	}

	return moduleSources.Offering(moduleName), nil
}

func (r *reconciler) processModule(ctx context.Context, moduleConfig *v1alpha1.ModuleConfig, module *v1alpha2.Module) (ctrl.Result, error) {
	defer r.logger.Debug("module config reconciled", slog.String("name", moduleConfig.Name))

	if err := r.addFinalizer(ctx, moduleConfig); err != nil {
		r.logger.Error("failed to add finalizer", slog.String("module", module.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	if module.IsInstalled() {
		// an installed module comes from one source: no conflict
		metricGroup := fmt.Sprintf(metrics.ModuleConflictMetricGroupTemplate, module.Name)
		r.metricStorage.Grouped().ExpireGroupMetrics(metricGroup)
	} else {
		// a module nothing installed is in conflict while the config enables it, several
		// sources offer it and none is picked: the config and the module both tell
		moduleSourceNames, err := r.listingModuleSources(ctx, moduleConfig.Name)
		if err != nil {
			r.logger.Error("failed to list module sources", slog.String("module", module.Name), log.Err(err))
			return ctrl.Result{}, err
		}

		conflict, err := r.reportConflict(ctx, moduleConfig, moduleSourceNames)
		if err != nil {
			return ctrl.Result{}, err
		}

		err = utils.UpdateStatus[*v1alpha2.Module](ctx, r.client, module, func(module *v1alpha2.Module) bool {
			return module.ApplyNotInstalledState(conflict)
		})
		if err != nil {
			r.logger.Error("failed to update the module status", slog.String("module", module.Name), log.Err(err))
			return ctrl.Result{}, err
		}
	}

	// the config fields of the module spec belong to the module config
	if err := r.mirrorModuleConfig(ctx, module, moduleConfig); err != nil {
		r.logger.Error("failed to mirror the module config", slog.String("module", module.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	metadata, err := utils.ModuleMetadata(ctx, r.client, module)
	if err != nil {
		r.logger.Error("failed to get the module metadata", slog.String("module", module.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	if !moduleConfig.IsEnabled() {
		// delete all pending releases for EnabledByModuleConfig disabled modules
		if module.IsCondition(v1alpha1.ModuleConditionEnabledByModuleConfig, metav1.ConditionTrue) {
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
						return ctrl.Result{}, fmt.Errorf("delete: %w", err)
					}
				}
			}
		}

		if err := r.disableModule(ctx, module, metadata); err != nil {
			r.logger.Error("failed to disable the module", slog.String("module", module.Name), log.Err(err))
			return ctrl.Result{}, err
		}

		err := utils.Update(ctx, r.client, moduleConfig, func(moduleConfig *v1alpha1.ModuleConfig) bool {
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

		// Reset deprecated and experimental metrics when module is disabled
		if isDeprecated(metadata) {
			r.metricStorage.GaugeSet(telemetry.WrapName(metrics.DeprecatedModuleIsEnabled), 0.0, map[string]string{metrics.LabelModule: moduleConfig.GetName()})
		}
		if isExperimental(metadata) {
			r.metricStorage.GaugeSet(telemetry.WrapName(metrics.ExperimentalModuleIsEnabled), 0.0, map[string]string{metrics.LabelModule: moduleConfig.GetName()})
		}

		// skip disabled modules
		r.logger.Debug("skip disabled module", slog.String("name", module.Name))
		return ctrl.Result{}, nil
	}

	if err := r.enableModule(ctx, module); err != nil {
		r.logger.Error("failed to enable the module", slog.String("module", module.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	// restore documentation for the re-enabled module from its deployed release
	if err := r.ensureModuleDocumentation(ctx, module); err != nil {
		r.logger.Error("failed to ensure module documentation", slog.String("module", module.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	if isExperimental(metadata) {
		r.metricStorage.GaugeSet(telemetry.WrapName(metrics.ExperimentalModuleIsEnabled), 1.0, map[string]string{metrics.LabelModule: moduleConfig.GetName()})
	}

	if isDeprecated(metadata) {
		r.metricStorage.GaugeSet(telemetry.WrapName(metrics.DeprecatedModuleIsEnabled), 1.0, map[string]string{metrics.LabelModule: moduleConfig.GetName()})
	}

	return ctrl.Result{}, nil
}

// mirrorModuleConfig writes the config fields of the module spec: settings, their version,
// the maintenance mode, the enabled intent and the update policy. Where the package comes
// from is not the config's to decide, so the repository and the version stay.
func (r *reconciler) mirrorModuleConfig(ctx context.Context, module *v1alpha2.Module, moduleConfig *v1alpha1.ModuleConfig) error {
	patch := client.MergeFrom(module.DeepCopy())

	module.Spec.Settings = moduleConfig.Spec.Settings
	module.Spec.SettingsVersion = moduleConfig.Spec.Version
	module.Spec.Maintenance = moduleConfig.Spec.Maintenance
	module.Spec.Enabled = moduleConfig.Spec.Enabled
	module.Spec.UpdatePolicy = moduleConfig.Spec.UpdatePolicy

	return r.patchModule(ctx, module, patch)
}

// clearModuleConfig drops the config fields of the module spec once the config is gone.
func (r *reconciler) clearModuleConfig(ctx context.Context, module *v1alpha2.Module) error {
	patch := client.MergeFrom(module.DeepCopy())

	module.Spec.Settings = nil
	module.Spec.SettingsVersion = 0
	module.Spec.Maintenance = ""
	module.Spec.Enabled = nil
	module.Spec.UpdatePolicy = ""

	return r.patchModule(ctx, module, patch)
}

// patchModule writes the module when the patch carries a change.
func (r *reconciler) patchModule(ctx context.Context, module *v1alpha2.Module, patch client.Patch) error {
	data, err := patch.Data(module)
	if err != nil {
		return fmt.Errorf("build patch: %w", err)
	}

	if string(data) == "{}" {
		return nil
	}

	if err := r.client.Patch(ctx, module, client.RawPatch(patch.Type(), data)); err != nil {
		return fmt.Errorf("patch the '%s' module: %w", module.Name, err)
	}

	return nil
}

// isExperimental reports whether the module metadata marks the module experimental; unknown
// metadata does not.
func isExperimental(metadata *v1alpha1.ModulePackageVersionStatusMetadata) bool {
	return metadata != nil && metadata.Stage == v1alpha1.ExperimentalModuleStage
}

// isDeprecated reports whether the module metadata marks the module deprecated; unknown
// metadata does not.
func isDeprecated(metadata *v1alpha1.ModulePackageVersionStatusMetadata) bool {
	return metadata != nil && metadata.Stage == v1alpha1.DeprecatedModuleStage
}

// enabledByBundle reports whether the edition enables the module by default; unknown
// metadata does not.
func (r *reconciler) enabledByBundle(metadata *v1alpha1.ModulePackageVersionStatusMetadata) bool {
	return metadata != nil && metadata.Licensing.IsEnabledInBundle(r.edition.Name, r.edition.Bundle)
}

func (r *reconciler) deleteModuleConfig(ctx context.Context, moduleConfig *v1alpha1.ModuleConfig) (ctrl.Result, error) {
	// send event to addon-operator
	r.handler.HandleEvent(moduleConfig, config.EventDelete)

	// clear obsolete metrics
	metricGroup := fmt.Sprintf(metrics.ObsoleteConfigMetricGroupTemplate, moduleConfig.Name)
	r.metricStorage.Grouped().ExpireGroupMetrics(metricGroup)

	// clear conflict metrics
	metricGroup = fmt.Sprintf(metrics.ModuleConflictMetricGroupTemplate, moduleConfig.Name)
	r.metricStorage.Grouped().ExpireGroupMetrics(metricGroup)

	r.metricStorage.GaugeSet(telemetry.WrapName(metrics.ExperimentalModuleIsEnabled), 0.0, map[string]string{metrics.LabelModule: moduleConfig.GetName()})
	r.metricStorage.GaugeSet(telemetry.WrapName(metrics.DeprecatedModuleIsEnabled), 0.0, map[string]string{metrics.LabelModule: moduleConfig.GetName()})

	module := new(v1alpha2.Module)
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

	metadata, err := utils.ModuleMetadata(ctx, r.client, module)
	if err != nil {
		r.logger.Error("failed to get the module metadata", slog.String("module", module.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	// disable module
	if err := r.disableModule(ctx, module, metadata); err != nil {
		r.logger.Error("failed to disable the module", slog.String("module", module.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	if err := r.clearModuleConfig(ctx, module); err != nil {
		r.logger.Error("failed to clear the module config", slog.String("module", module.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	err = utils.UpdateStatus[*v1alpha2.Module](ctx, r.client, module, func(module *v1alpha2.Module) bool {
		module.SetConditionUnknown(v1alpha1.ModuleConditionEnabledByModuleConfig, v1alpha1.ModuleReasonUnknown, "")

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

func (r *reconciler) disableModule(ctx context.Context, module *v1alpha2.Module, metadata *v1alpha1.ModulePackageVersionStatusMetadata) error {
	r.logger.Debug("disable the module", slog.String("module", module.Name))

	// remove module documentation immediately on disable so docs-builder drops it
	if err := utils.DeleteModuleDocumentation(ctx, r.client, module.Name); err != nil {
		return fmt.Errorf("delete module documentation: %w", err)
	}

	return utils.UpdateStatus[*v1alpha2.Module](ctx, r.client, module, func(module *v1alpha2.Module) bool {
		if module.IsCondition(v1alpha1.ModuleConditionEnabledByModuleConfig, metav1.ConditionFalse) {
			return false
		}

		switch {
		case !module.IsInstalled(),
			module.Status.Phase == v1alpha1.ModulePhaseConflict,
			module.Status.Phase == v1alpha1.ModulePhaseDownloading,
			module.Status.Phase == v1alpha1.ModulePhaseDownloadingError:
			// a module nothing installed goes back to the available state: one in conflict or
			// fetching its first release receives no event that would move it
			module.SetNotInstalledStatus()
		default:
			if !r.enabledByBundle(metadata) {
				module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonDisabled, v1alpha1.ModuleMessageDisabled)
			}
		}

		module.SetConditionFalse(v1alpha1.ModuleConditionEnabledByModuleConfig, v1alpha1.ModuleReasonDisabled, "")
		module.SetConditionUnknown(v1alpha1.ModuleConditionLastReleaseDeployed, v1alpha1.ModuleReasonUnknown, "")

		return true
	})
}

func (r *reconciler) enableModule(ctx context.Context, module *v1alpha2.Module) error {
	r.logger.Debug("enable the module", slog.String("module", module.Name))
	return utils.UpdateStatus[*v1alpha2.Module](ctx, r.client, module, func(module *v1alpha2.Module) bool {
		if module.IsCondition(v1alpha1.ModuleConditionEnabledByModuleConfig, metav1.ConditionTrue) {
			return false
		}
		module.SetConditionTrue(v1alpha1.ModuleConditionEnabledByModuleConfig, v1alpha1.ModuleReasonEnabled)

		return true
	})
}

func (r *reconciler) ensureModuleDocumentation(ctx context.Context, module *v1alpha2.Module) error {
	releases := new(v1alpha1.ModuleReleaseList)
	if err := r.client.List(ctx, releases, client.MatchingLabels{
		v1alpha1.ModuleReleaseLabelModule: module.Name,
		v1alpha1.ModuleReleaseLabelStatus: v1alpha1.ModuleReleaseLabelDeployed,
	}); err != nil {
		return fmt.Errorf("list module releases: %w", err)
	}

	for i := range releases.Items {
		if err := utils.EnsureModuleDocumentationForRelease(ctx, r.client, &releases.Items[i]); err != nil {
			return fmt.Errorf("ensure module documentation: %w", err)
		}
	}

	return nil
}
