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

package config

import (
	"context"
	"crypto/md5"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrlhandler "sigs.k8s.io/controller-runtime/pkg/handler"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	d8edition "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/deckhouse/deckhouse/go_lib/telemetry"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

const (
	// controllerName is the name the controller is registered under in the manager.
	controllerName = "d8-module-config-controller"

	// maxConcurrentReconciles allows a few configs to be reconciled at once; each one
	// touches only its own module, so they do not contend.
	maxConcurrentReconciles = 3
	// cacheSyncTimeout bounds the initial informer cache sync.
	cacheSyncTimeout = 3 * time.Minute
	// defaultRequeueAfter is the retry delay for a config whose module does not exist
	// yet, a state only an external change can resolve.
	defaultRequeueAfter = 3 * time.Minute

	// moduleDeckhouse and moduleGlobal name the system modules, whose source and update
	// policy are not driven by their config.
	moduleDeckhouse = "deckhouse"
	moduleGlobal    = "global"
)

// moduleCreated passes Module creation events only: a config that arrived before its
// module has to be reconciled once the module appears, while later module changes are
// driven by the config itself. Unset fields of predicate.Funcs default to true, so every
// branch is spelled out.
var moduleCreated = predicate.Funcs{
	CreateFunc:  func(_ event.CreateEvent) bool { return true },
	UpdateFunc:  func(_ event.UpdateEvent) bool { return false },
	DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
	GenericFunc: func(_ event.GenericEvent) bool { return false },
}

// enqueueModuleConfig maps a Module event onto the config of the same name.
func enqueueModuleConfig(_ context.Context, obj client.Object) []reconcile.Request {
	return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: obj.GetName()}}}
}

// RegisterController registers the ModuleConfig controller with the manager. The preflight
// wait group is owned by the caller and gates reconciliation until the controller's startup
// sync has finished.
func RegisterController(
	ctrlManager ctrlmanager.Manager,
	manager packageManager,
	preflight *sync.WaitGroup,
	edition *d8edition.Edition,
	metricStorage metricsstorage.Storage,
	logger *log.Logger,
) error {
	r := &reconciler{
		init:          preflight,
		client:        ctrlManager.GetClient(),
		manager:       manager,
		edition:       edition,
		metricStorage: metricStorage,
		logger:        logger.Named(controllerName),
	}

	if err := ctrl.NewControllerManagedBy(ctrlManager).
		Named(controllerName).
		For(&v1alpha1.ModuleConfig{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})).
		Watches(&v1alpha1.Module{}, ctrlhandler.EnqueueRequestsFromMapFunc(enqueueModuleConfig), builder.WithPredicates(moduleCreated)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: maxConcurrentReconciles,
			CacheSyncTimeout:        cacheSyncTimeout,
			NeedLeaderElection:      new(false),
		}).
		Complete(r); err != nil {
		return fmt.Errorf("complete: %w", err)
	}

	return nil
}

// reconciler reconciles ModuleConfig objects.
type reconciler struct {
	init          *sync.WaitGroup
	client        client.Client
	manager       packageManager
	edition       *d8edition.Edition
	metricStorage metricsstorage.Storage
	logger        *log.Logger
}

// packageManager carries a module's settings from its config into the package runtime.
type packageManager interface {
	UpdateModulesSettings(name string, settingsVersion int, settings addonutils.Values, maintenance string, enabled *bool)
}

// Reconcile dispatches the config to the delete or the create/update handler.
func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// wait for init
	r.init.Wait()

	r.logger.Debug("reconcile module config", slog.String("name", req.Name))
	mc := new(v1alpha1.ModuleConfig)
	if err := r.client.Get(ctx, client.ObjectKey{Name: req.Name}, mc); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Warn("module config not found", slog.String("name", req.Name))
			return ctrl.Result{}, nil
		}

		r.logger.Error("failed to get module config", slog.String("name", req.Name), log.Err(err))
		return ctrl.Result{}, fmt.Errorf("get: %w", err)
	}

	// handle delete event
	if !mc.DeletionTimestamp.IsZero() {
		r.logger.Info("deleting the module config", slog.String("name", req.Name))
		return r.handleDelete(ctx, mc)
	}

	// handle create/update events
	return r.handleCreateOrUpdate(ctx, mc)
}

// handleCreateOrUpdate pushes the config's settings into the package runtime and then
// brings the module the config names in line with it.
func (r *reconciler) handleCreateOrUpdate(ctx context.Context, mc *v1alpha1.ModuleConfig) (ctrl.Result, error) {
	r.manager.UpdateModulesSettings(mc.Name, mc.Spec.Version, mc.Spec.Settings.GetMap(), mc.Spec.Maintenance, mc.Spec.Enabled)

	module := new(v1alpha1.Module)
	if err := r.client.Get(ctx, client.ObjectKey{Name: mc.Name}, module); err != nil {
		if !apierrors.IsNotFound(err) {
			r.logger.Error("failed to get module", slog.String("name", mc.Name), log.Err(err))
			return ctrl.Result{}, fmt.Errorf("get: %w", err)
		}

		// the global config configures the whole installation, it has no module of its own
		if mc.Name == moduleGlobal {
			return ctrl.Result{}, nil
		}

		r.logger.Warn("module not found", slog.String("name", mc.Name))
		err = utils.UpdateStatus(ctx, r.client, mc, func(mc *v1alpha1.ModuleConfig) bool {
			mc.Status.Message = v1alpha1.ModuleConfigMessageUnknownModule

			return true
		})
		if err != nil {
			r.logger.Error("failed to update the module config status", slog.String("name", mc.Name), log.Err(err))
			return ctrl.Result{}, err
		}

		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	return r.processModule(ctx, mc, module)
}

// processModule toggles the module's enabled condition to match its config and, for
// downloaded modules, applies the source and update policy the config asks for.
func (r *reconciler) processModule(ctx context.Context, mc *v1alpha1.ModuleConfig, module *v1alpha1.Module) (ctrl.Result, error) {
	defer r.logger.Debug("module config reconciled", slog.String("name", mc.Name))

	// clear conflict metrics
	r.metricStorage.Grouped().ExpireGroupMetrics(fmt.Sprintf(metrics.ModuleConflictMetricGroupTemplate, module.Name))

	if err := r.addFinalizer(ctx, mc); err != nil {
		r.logger.Error("failed to add finalizer", slog.String("module", module.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	if !mc.IsEnabled() {
		return ctrl.Result{}, r.processDisabledModule(ctx, mc, module)
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

	r.setLifecycleMetrics(module, 1.0)

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

	return ctrl.Result{}, r.applySource(ctx, mc, module)
}

// processDisabledModule tears down a module its config has disabled: pending releases are
// dropped, the documentation is removed, the module is marked disabled and the one-shot
// allow-disable annotation is spent.
func (r *reconciler) processDisabledModule(ctx context.Context, mc *v1alpha1.ModuleConfig, module *v1alpha1.Module) error {
	// delete all pending releases for EnabledByModuleConfig disabled modules
	if module.IsCondition(v1alpha1.ModuleConditionEnabledByModuleConfig, corev1.ConditionTrue) {
		if err := r.deletePendingReleases(ctx, module); err != nil {
			r.logger.Error("failed to delete pending releases", slog.String("module", module.Name), log.Err(err))
			return err
		}
	}

	// a disable in the config is final, it beats a bundle that would enable the module,
	// so the module stops and its documentation goes with it
	if err := r.deleteModuleDocumentation(ctx, module); err != nil {
		r.logger.Error("failed to delete module documentation", slog.String("module", module.Name), log.Err(err))
		return err
	}

	if err := r.disableModule(ctx, module); err != nil {
		r.logger.Error("failed to disable the module", slog.String("module", module.Name), log.Err(err))
		return err
	}

	err := utils.Update(ctx, r.client, mc, func(mc *v1alpha1.ModuleConfig) bool {
		if _, ok := mc.Annotations[v1alpha1.ModuleConfigAnnotationAllowDisable]; ok {
			delete(mc.Annotations, v1alpha1.ModuleConfigAnnotationAllowDisable)

			return true
		}

		return false
	})
	if err != nil {
		r.logger.Error("failed to remove the allow-disable annotation", slog.String("name", mc.Name), log.Err(err))
		return err
	}

	// the module no longer runs, so it no longer counts as deprecated or experimental in use
	r.setLifecycleMetrics(module, 0.0)

	r.logger.Debug("skip disabled module", slog.String("name", module.Name))

	return nil
}

// deletePendingReleases drops the module's pending releases, which must not deploy once
// its config has disabled the module.
func (r *reconciler) deletePendingReleases(ctx context.Context, module *v1alpha1.Module) error {
	releases := new(v1alpha1.ModuleReleaseList)
	if err := r.client.List(ctx, releases, client.MatchingLabels{v1alpha1.ModuleReleaseLabelModule: module.Name}); err != nil {
		return fmt.Errorf("list module releases: %w", err)
	}

	for i := range releases.Items {
		if releases.Items[i].GetPhase() != v1alpha1.ModuleReleasePhasePending {
			continue
		}

		if err := r.client.Delete(ctx, &releases.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete the '%s' release: %w", releases.Items[i].Name, err)
		}
	}

	return nil
}

// applySource points the module at the source and update policy its config asks for,
// falling back to the only available source and flagging a conflict when several offer it.
func (r *reconciler) applySource(ctx context.Context, mc *v1alpha1.ModuleConfig, module *v1alpha1.Module) error {
	updatePolicy := module.Properties.UpdatePolicy
	// change update policy by module config
	if updatePolicy != mc.Spec.UpdatePolicy {
		updatePolicy = mc.Spec.UpdatePolicy
	}

	// change source by module config
	if mc.Spec.Source != "" && module.Properties.Source != mc.Spec.Source {
		if err := r.changeModuleSource(ctx, module, mc.Spec.Source, updatePolicy); err != nil {
			r.logger.Debug("failed to change source for the module", slog.String("name", module.Name), log.Err(err))
			return err
		}
	}

	if module.Properties.Source == "" {
		// change source by available source
		if len(module.Properties.AvailableSources) == 1 {
			if err := r.changeModuleSource(ctx, module, module.Properties.AvailableSources[0], updatePolicy); err != nil {
				r.logger.Debug("failed to change source for module", slog.String("name", module.Name), log.Err(err))
				return err
			}
		}

		// set conflict if there are several available sources
		if len(module.Properties.AvailableSources) > 1 {
			if err := r.setModuleConflict(ctx, module); err != nil {
				r.logger.Error("failed to set conflict to module", slog.String("name", module.Name), log.Err(err))
				return err
			}
		}
	}

	// update only the update policy if nothing else has changed
	err := utils.Update(ctx, r.client, module, func(module *v1alpha1.Module) bool {
		if module.Properties.UpdatePolicy != updatePolicy {
			module.Properties.UpdatePolicy = updatePolicy

			return true
		}

		return false
	})
	if err != nil {
		r.logger.Error("failed to update the module's update policy", slog.String("name", module.Name), log.Err(err))
		return err
	}

	return nil
}

// setModuleConflict parks a module several sources offer and fires the alert asking an
// operator to pick one through the config.
func (r *reconciler) setModuleConflict(ctx context.Context, module *v1alpha1.Module) error {
	err := utils.UpdateStatus(ctx, r.client, module, func(module *v1alpha1.Module) bool {
		module.Status.Phase = v1alpha1.ModulePhaseConflict
		module.SetConditionFalse(v1alpha1.ModuleConditionEnabledByModuleManager, "", "")
		module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonConflict, v1alpha1.ModuleMessageConflict)

		return true
	})
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	// fire alert at Conflict
	r.metricStorage.Grouped().GaugeSet(fmt.Sprintf(metrics.ModuleConflictMetricGroupTemplate, module.Name), metrics.D8ModuleAtConflict, 1.0, map[string]string{
		"module": module.Name,
	})

	return nil
}

// handleDelete disables the module its config owned, clears the config's metrics and
// releases the config's finalizer.
func (r *reconciler) handleDelete(ctx context.Context, mc *v1alpha1.ModuleConfig) (ctrl.Result, error) {
	r.manager.UpdateModulesSettings(mc.Name, 0, make(addonutils.Values), "", nil)

	r.expireConfigMetrics(mc.Name)

	module := new(v1alpha1.Module)
	if err := r.client.Get(ctx, client.ObjectKey{Name: mc.Name}, module); err != nil {
		if !apierrors.IsNotFound(err) {
			r.logger.Error("failed to get module", slog.String("name", mc.Name), log.Err(err))
			return ctrl.Result{}, fmt.Errorf("get: %w", err)
		}

		r.logger.Warn("module not found", slog.String("name", mc.Name))
		if err = r.removeFinalizer(ctx, mc); err != nil {
			r.logger.Error("failed to remove finalizer", slog.String("name", mc.Name), log.Err(err))
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	// skip system modules
	if module.Name == moduleDeckhouse || module.Name == moduleGlobal {
		r.logger.Debug("skip system module", slog.String("name", module.Name))
		return ctrl.Result{}, nil
	}

	if err := r.disableModule(ctx, module); err != nil {
		r.logger.Error("failed to disable the module", slog.String("module", module.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	// with no config left the bundle decides, so a module it still enables keeps running
	// and must keep its documentation
	if !module.IsEnabledByBundle(r.edition.Name, r.edition.Bundle) {
		if err := r.deleteModuleDocumentation(ctx, module); err != nil {
			r.logger.Error("failed to delete module documentation", slog.String("module", module.Name), log.Err(err))
			return ctrl.Result{}, err
		}
	}

	// clear downloaded module
	if !module.IsEmbedded() && !module.IsEnabledByBundle(r.edition.Name, r.edition.Bundle) {
		err := utils.Update(ctx, r.client, module, func(module *v1alpha1.Module) bool {
			module.Properties.UpdatePolicy = ""
			module.Properties.Source = ""

			return true
		})
		if err != nil {
			r.logger.Error("failed to update the module", slog.String("module", module.Name), log.Err(err))
			return ctrl.Result{}, err
		}
	}

	err := utils.UpdateStatus(ctx, r.client, module, func(module *v1alpha1.Module) bool {
		module.SetConditionUnknown(v1alpha1.ModuleConditionEnabledByModuleConfig, "", "")

		return true
	})
	if err != nil {
		r.logger.Error("failed to update the module status", slog.String("name", module.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	if err := r.removeFinalizer(ctx, mc); err != nil {
		r.logger.Error("failed to remove finalizer", slog.String("name", mc.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// changeModuleSource points the module at source, carrying the update policy over so both
// land in a single write.
func (r *reconciler) changeModuleSource(ctx context.Context, module *v1alpha1.Module, source, updatePolicy string) error {
	r.logger.Debug("set new source to the module", slog.String("module_source", source), slog.String("module", module.Name))
	err := utils.Update(ctx, r.client, module, func(module *v1alpha1.Module) bool {
		module.Properties.Source = source
		module.Properties.UpdatePolicy = updatePolicy

		return true
	})
	if err != nil {
		return fmt.Errorf("update the '%s' module: %w", module.Name, err)
	}

	return nil
}

// addFinalizer adds the finalizer that lets the controller see the config's delete event.
func (r *reconciler) addFinalizer(ctx context.Context, mc *v1alpha1.ModuleConfig) error {
	return utils.Update(ctx, r.client, mc, func(mc *v1alpha1.ModuleConfig) bool {
		if controllerutil.ContainsFinalizer(mc, v1alpha1.ModuleConfigFinalizer) {
			return false
		}

		controllerutil.AddFinalizer(mc, v1alpha1.ModuleConfigFinalizer)

		return true
	})
}

// removeFinalizer releases the config and spends its allow-disable annotation, so the
// annotation cannot survive into a recreated config.
func (r *reconciler) removeFinalizer(ctx context.Context, mc *v1alpha1.ModuleConfig) error {
	return utils.Update(ctx, r.client, mc, func(mc *v1alpha1.ModuleConfig) bool {
		var needsUpdate bool
		if controllerutil.ContainsFinalizer(mc, v1alpha1.ModuleConfigFinalizer) {
			controllerutil.RemoveFinalizer(mc, v1alpha1.ModuleConfigFinalizer)
			needsUpdate = true
		}

		if _, ok := mc.Annotations[v1alpha1.ModuleConfigAnnotationAllowDisable]; ok {
			delete(mc.Annotations, v1alpha1.ModuleConfigAnnotationAllowDisable)
			needsUpdate = true
		}

		return needsUpdate
	})
}

// disableModule clears the module's enabled condition, only on transition, so a steady
// state does not restamp the condition's probe time. Documentation is not touched here:
// whether the module actually stops depends on the caller, see deleteModuleDocumentation.
func (r *reconciler) disableModule(ctx context.Context, module *v1alpha1.Module) error {
	r.logger.Debug("disable the module", slog.String("module", module.Name))

	return utils.UpdateStatus(ctx, r.client, module, func(module *v1alpha1.Module) bool {
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

// enableModule sets the module's enabled condition, only on transition: SetConditionTrue
// restamps LastProbeTime, so an unconditional update writes status on every reconcile.
func (r *reconciler) enableModule(ctx context.Context, module *v1alpha1.Module) error {
	r.logger.Debug("enable the module", slog.String("module", module.Name))

	return utils.UpdateStatus(ctx, r.client, module, func(module *v1alpha1.Module) bool {
		if module.IsCondition(v1alpha1.ModuleConditionEnabledByModuleConfig, corev1.ConditionTrue) {
			return false
		}

		module.SetConditionTrue(v1alpha1.ModuleConditionEnabledByModuleConfig)

		return true
	})
}

// deleteModuleDocumentation drops the module's documentation so docs-builder stops serving
// it. Only call it once the module is known to stop: a module the bundle still enables goes
// on running without its config.
func (r *reconciler) deleteModuleDocumentation(ctx context.Context, module *v1alpha1.Module) error {
	if err := utils.DeleteModuleDocumentation(ctx, r.client, module.Name); err != nil {
		return fmt.Errorf("delete module documentation: %w", err)
	}

	return nil
}

// ensureModuleDocumentation restores the documentation of a re-enabled module from the
// release it already has deployed, which deleteModuleDocumentation removed.
func (r *reconciler) ensureModuleDocumentation(ctx context.Context, module *v1alpha1.Module) error {
	releases := new(v1alpha1.ModuleReleaseList)
	if err := r.client.List(ctx, releases, client.MatchingLabels{
		v1alpha1.ModuleReleaseLabelModule: module.Name,
		v1alpha1.ModuleReleaseLabelStatus: v1alpha1.ModuleReleaseLabelDeployed,
	}); err != nil {
		return fmt.Errorf("list module releases: %w", err)
	}

	for i := range releases.Items {
		release := &releases.Items[i]

		// Use mount point path: /modules/<module> (modules are mounted at /deckhouse/downloaded/modules/deployed/<module>)
		modulePath := filepath.Join("/modules/deployed", release.GetModuleName())
		moduleVersion := "v" + release.GetVersion().String()

		moduleChecksum := release.Labels[v1alpha1.ModuleReleaseLabelReleaseChecksum]
		if moduleChecksum == "" {
			moduleChecksum = fmt.Sprintf("%x", md5.Sum([]byte(moduleVersion)))
		}

		ownerRef := metav1.OwnerReference{
			APIVersion: v1alpha1.ModuleReleaseGVK.GroupVersion().String(),
			Kind:       v1alpha1.ModuleReleaseGVK.Kind,
			Name:       release.GetName(),
			UID:        release.GetUID(),
			Controller: new(true),
		}

		if err := utils.EnsureModuleDocumentation(ctx, r.client, release.GetModuleName(), module.Properties.Source, moduleChecksum, moduleVersion, modulePath, ownerRef); err != nil {
			r.logger.Error("failed to ensure module documentation for release", slog.String("name", release.Name), log.Err(err))
			return fmt.Errorf("ensure module documentation: %w", err)
		}
	}

	return nil
}

// setLifecycleMetrics reports whether a deprecated or experimental module is in use.
func (r *reconciler) setLifecycleMetrics(module *v1alpha1.Module, value float64) {
	if module.IsDeprecated() {
		r.metricStorage.GaugeSet(telemetry.WrapName(metrics.DeprecatedModuleIsEnabled), value, map[string]string{metrics.LabelModule: module.Name})
	}

	if module.IsExperimental() {
		r.metricStorage.GaugeSet(telemetry.WrapName(metrics.ExperimentalModuleIsEnabled), value, map[string]string{metrics.LabelModule: module.Name})
	}
}

// expireConfigMetrics drops every metric keyed by a config that no longer exists.
func (r *reconciler) expireConfigMetrics(name string) {
	r.metricStorage.Grouped().ExpireGroupMetrics(fmt.Sprintf(metrics.ObsoleteConfigMetricGroupTemplate, name))
	r.metricStorage.Grouped().ExpireGroupMetrics(fmt.Sprintf(metrics.ModuleConflictMetricGroupTemplate, name))

	r.metricStorage.GaugeSet(telemetry.WrapName(metrics.ExperimentalModuleIsEnabled), 0.0, map[string]string{metrics.LabelModule: name})
	r.metricStorage.GaugeSet(telemetry.WrapName(metrics.DeprecatedModuleIsEnabled), 0.0, map[string]string{metrics.LabelModule: name})
}
