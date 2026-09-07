// Copyright 2026 Flant JSC
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

package module

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	d8edition "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/flant/addon-operator/pkg/kube_config_manager/config"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules/events"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/controller/confighandler"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/module/status"
	"github.com/deckhouse/deckhouse/go_lib/configtools"
	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	"github.com/deckhouse/deckhouse/go_lib/telemetry"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

const (
	// controllerName is the name the controller is registered under in the manager.
	controllerName = "d8-modulev2-settings-controller"

	// maxConcurrentReconciles = 1

	// moduleNotFoundInterval = 3 * time.Minute

	moduleDeckhouse = "deckhouse"
	moduleGlobal    = "global"
)

// // settingsManager applies a module's settings-and-enabled change to the package runtime.
// type settingsManager interface {
// 	UpdateModulesSettings(name string, settingsVersion int, settings addonutils.Values, maintenance string, enabled *bool)
// }

// RegisterController registers the Module settings controller with the manager.
func RegisterController(
	sync *sync.WaitGroup,
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
		init:             sync,
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
		For(&v1alpha2.Module{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})).
		WithOptions(controller.Options{
			NeedLeaderElection: ptr.To(false),
		}).
		Complete(r); err != nil {
		return fmt.Errorf("complete: %w", err)
	}

	return nil
}

// reconciler reconciles Module objects.
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

// Reconcile applies a Module's settings-and-enabled change to the package runtime.
func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// wait until init
	r.init.Wait()
	res := ctrl.Result{}

	r.logger.Debug("reconciling module", slog.String("name", req.Name))
	module := new(v1alpha2.Module)
	if err := r.client.Get(ctx, client.ObjectKey{Name: req.Name}, module); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Warn("module not found", slog.String("name", req.Name))
			return res, nil
		}

		r.logger.Error("failed to get module", slog.String("name", req.Name), log.Err(err))
		return res, fmt.Errorf("get: %w", err)
	}

	// handle delete event
	if !module.DeletionTimestamp.IsZero() {
		r.logger.Debug("deleting module", slog.String("name", req.Name))
		return r.deleteModule(ctx, module)
	}

	return r.handleModule(ctx, module)
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

func (r *reconciler) handleModule(ctx context.Context, module *v1alpha2.Module) (ctrl.Result, error) {
	// send an event to addon-operator only if the module exists, or it is the global one
	basicModule := r.moduleManager.GetModule(module.Name)
	if module.Name == moduleGlobal || basicModule != nil {
		r.logger.Debug("send event to operator", slog.String("name", module.Name), slog.Bool("enabled", module.IsEnabled()))
		r.handler.HandleEvent(module, config.EventUpdate)
	}

	// apply the module settings-and-enabled change to the package runtime
	r.packageManager.UpdateModulesSettings(
		module.Name,
		module.Spec.SettingsVersion,
		module.Spec.Settings.GetMap(),
		module.Spec.Maintenance,
		module.Spec.Enabled)

	return r.processModule(ctx, module)
}

func (r *reconciler) processModule(ctx context.Context, module *v1alpha2.Module) (ctrl.Result, error) {
	res := ctrl.Result{}

	// ensure the finalizer is in place so a later delete can be intercepted
	if err := r.addFinalizer(ctx, module); err != nil {
		r.logger.Error("failed to add finalizer", slog.String("name", module.Name), log.Err(err))
		return res, err
	}

	return res, nil
}

func (r *reconciler) deleteModule(ctx context.Context, module *v1alpha2.Module) (ctrl.Result, error) {
	// send event to addon-operator
	r.handler.HandleEvent(module, config.EventDelete)

	// clear obsolete metrics
	metricGroup := fmt.Sprintf(metrics.ObsoleteConfigMetricGroupTemplate, module.Name)
	r.metricStorage.Grouped().ExpireGroupMetrics(metricGroup)

	// clear conflict metrics
	metricGroup = fmt.Sprintf(metrics.ModuleConflictMetricGroupTemplate, module.Name)
	r.metricStorage.Grouped().ExpireGroupMetrics(metricGroup)

	r.metricStorage.GaugeSet(telemetry.WrapName(metrics.ExperimentalModuleIsEnabled), 0.0, map[string]string{metrics.LabelModule: module.GetName()})
	r.metricStorage.GaugeSet(telemetry.WrapName(metrics.DeprecatedModuleIsEnabled), 0.0, map[string]string{metrics.LabelModule: module.GetName()})

	res := ctrl.Result{}

	enabledByBundle, err := r.IsModuleEnabledByBundle(ctx, module)
	if err != nil {
		return res, err
	}

	// skip system modules
	if module.Name == moduleDeckhouse || module.Name == moduleGlobal {
		r.logger.Debug("skip system module", slog.String("name", module.Name))
		return res, nil
	}

	// disable module
	if err := r.disableModule(ctx, module, enabledByBundle); err != nil {
		r.logger.Error("failed to disable the module", slog.String("module", module.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	// no-op cleanup: this controller owns only the module settings, there is
	// nothing to tear down in the package runtime here. Just release the object.
	if err := r.removeFinalizer(ctx, module); err != nil {
		r.logger.Error("failed to remove finalizer", slog.String("name", module.Name), log.Err(err))
		return res, err
	}

	return res, nil
}

func (r *reconciler) disableModule(ctx context.Context, module *v1alpha2.Module, enabledByBundle bool) error {
	r.logger.Debug("disable the module", slog.String("module", module.Name))

	// remove module documentation immediately on disable so docs-builder drops it
	if err := utils.DeleteModuleDocumentation(ctx, r.client, module.Name); err != nil {
		return fmt.Errorf("delete module documentation: %w", err)
	}

	return utils.UpdateStatus[*v1alpha2.Module](ctx, r.client, module, func(module *v1alpha2.Module) bool {
		if module.IsCondition(status.ConditionEnabled, metav1.ConditionFalse) {
			return false
		}

		switch module.Status.Summary.State {
		case status.StateFailed,
			status.StatePending:
			// modules in Conflict should not be installed, and they cannot receive events, so set Available phase manually
			// same thing if module is not installed
			module.Status.Summary.State = status.StateSuspended
			module.SetConditionFalse(status.ConditionEnabled, "", "")
			module.SetConditionFalse(status.ConditionReady, v1alpha1.ModuleReasonNotInstalled, v1alpha1.ModuleMessageNotInstalled)
		default:
			if !enabledByBundle {
				module.SetConditionFalse(status.ConditionReady, v1alpha1.ModuleReasonDisabled, v1alpha1.ModuleMessageDisabled)
			}
		}

		module.SetConditionFalse(status.ConditionEnabled, "", "")
		module.SetConditionUnknown(status.ConditionConfigurationApplied, "", "")

		return true
	})
}

// addFinalizer puts the controller's finalizer on the Module so a later delete can be handled.
func (r *reconciler) addFinalizer(ctx context.Context, module *v1alpha2.Module) error {
	return utils.Update[*v1alpha2.Module](ctx, r.client, module, func(module *v1alpha2.Module) bool {
		if !controllerutil.ContainsFinalizer(module, v1alpha2.ModuleFinalizerModuleRegistered) {
			controllerutil.AddFinalizer(module, v1alpha2.ModuleFinalizerModuleRegistered)
			return true
		}

		return false
	})
}

// removeFinalizer drops the controller's finalizer so the Module can be garbage-collected.
func (r *reconciler) removeFinalizer(ctx context.Context, module *v1alpha2.Module) error {
	return utils.Update[*v1alpha2.Module](ctx, r.client, module, func(module *v1alpha2.Module) bool {
		if controllerutil.ContainsFinalizer(module, v1alpha2.ModuleFinalizerModuleRegistered) {
			controllerutil.RemoveFinalizer(module, v1alpha2.ModuleFinalizerModuleRegistered)
			return true
		}

		return false
	})
}

func (r *reconciler) IsModuleEnabledByBundle(ctx context.Context, module *v1alpha2.Module) (bool, error) {
	mpv := &v1alpha1.ModulePackageVersion{}
	mpvName := v1alpha1.MakeModulePackageVersionName(
		module.Spec.PackageRepositoryName,
		module.Name,
		module.Spec.PackageVersion,
	)

	if err := r.client.Get(ctx, types.NamespacedName{Name: mpvName}, mpv); err != nil {
		return false, err
	}

	// if EnabledInBundles info is empty, module is not enabled by default
	if mpv.Status.PackageMetadata == nil ||
		mpv.Status.PackageMetadata.Licensing == nil ||
		mpv.Status.PackageMetadata.Licensing.Editions == nil {
		return false, nil
	}

	for edition, license := range mpv.Status.PackageMetadata.Licensing.Editions {
		if !license.Available {
			continue
		}
		if edition != "_default" && edition != r.edition.Name {
			continue
		}

		for _, bundle := range license.EnabledInBundles {
			if bundle == r.edition.Bundle {
				return true, nil
			}
		}
	}

	return false, nil
}
