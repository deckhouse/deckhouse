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
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// controllerName is the name the controller is registered under in the manager.
	controllerName = "d8-module-config-controller"

	// maxConcurrentReconciles is safe above one: a config only ever writes the module of
	// the same name, and config names are unique cluster-wide.
	maxConcurrentReconciles = 3

	// moduleNotFoundInterval is the retry delay for a config whose module does not exist
	// yet, backing up the module watch for modules the informer cache has not seen.
	moduleNotFoundInterval = 3 * time.Minute

	// moduleGlobal is the config that carries platform-wide settings and has no module.
	moduleGlobal = "global"

	// moduleDeckhouse is the config that carries Deckhouse settings and has no module.
	moduleDeckhouse = "deckhouse"
)

// errModuleNotFound reports that the config names a module that does not exist. The
// config is parked until one appears instead of retried on the workqueue backoff.
var errModuleNotFound = errors.New("module not found")

// moduleAppeared passes only module creations: a config reconciled before its module
// needs a second pass, while later module changes are the module controller's business.
var moduleAppeared = predicate.Funcs{
	CreateFunc:  func(_ event.CreateEvent) bool { return true },
	UpdateFunc:  func(_ event.UpdateEvent) bool { return false },
	DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
	GenericFunc: func(_ event.GenericEvent) bool { return false },
}

// RegisterController registers the ModuleConfig controller with the manager.
func RegisterController(synced *sync.WaitGroup, runtime ctrlmanager.Manager, settingsCh chan<- addonutils.Values, logger *log.Logger) error {
	r := &reconciler{
		init:       synced,
		client:     runtime.GetClient(),
		settingsCh: settingsCh,
		logger:     logger.Named(controllerName),
	}

	if err := ctrl.NewControllerManagedBy(runtime).
		Named(controllerName).
		For(&v1alpha1.ModuleConfig{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})).
		Watches(&v1alpha2.Module{}, ctrlhandler.EnqueueRequestsFromMapFunc(enqueueSameName), builder.WithPredicates(moduleAppeared)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: maxConcurrentReconciles,
			// settings are mirrored by every replica, not only by the leader
			NeedLeaderElection: new(false),
		}).
		Complete(r); err != nil {
		return fmt.Errorf("complete: %w", err)
	}

	return nil
}

// enqueueSameName maps a module to the config that configures it, which shares its name.
func enqueueSameName(_ context.Context, obj client.Object) []reconcile.Request {
	return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: obj.GetName()}}}
}

// reconciler mirrors ModuleConfig settings onto the module of the same name.
type reconciler struct {
	init       *sync.WaitGroup
	client     client.Client
	settingsCh chan<- addonutils.Values
	logger     *log.Logger
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

		return ctrl.Result{}, fmt.Errorf("get module config '%s': %w", req.Name, err)
	}

	// handle delete event
	if !mc.DeletionTimestamp.IsZero() {
		if err := r.handleDelete(ctx, mc); err != nil {
			return ctrl.Result{}, fmt.Errorf("delete: %w", err)
		}

		return ctrl.Result{}, nil
	}

	// handle create/update events
	if err := r.handleCreateOrUpdate(ctx, mc); err != nil {
		if errors.Is(err, errModuleNotFound) {
			return ctrl.Result{RequeueAfter: moduleNotFoundInterval}, nil
		}

		return ctrl.Result{}, fmt.Errorf("create or update: %w", err)
	}

	return ctrl.Result{}, nil
}

// handleCreateOrUpdate copies the config's settings onto its module.
func (r *reconciler) handleCreateOrUpdate(ctx context.Context, mc *v1alpha1.ModuleConfig) error {
	logger := r.logger.With(slog.String("name", mc.Name))

	logger.Debug("handle module config")
	defer logger.Debug("handle module config complete")

	if mc.Name == moduleGlobal {
		return nil
	}

	module := new(v1alpha2.Module)
	if err := r.client.Get(ctx, client.ObjectKey{Name: mc.Name}, module); err != nil {
		if !apierrors.IsNotFound(err) {
			logger.Error("failed to get the module", log.Err(err))
			return fmt.Errorf("get module '%s': %w", mc.Name, err)
		}

		// the global config has no module by design, so it is not an unknown one
		if mc.Name == moduleGlobal {
			return errModuleNotFound
		}

		logger.Warn("module not found")

		if err := r.markUnknownModule(ctx, mc); err != nil {
			logger.Error("failed to mark the module config as unknown", log.Err(err))
			return err
		}

		return errModuleNotFound
	}

	// The finalizer is claimed before the settings reach the module: it is what
	// guarantees handleDelete gets to clear them again.
	if err := r.addFinalizer(ctx, mc); err != nil {
		logger.Error("failed to add the module config finalizer", log.Err(err))
		return fmt.Errorf("add finalizer: %w", err)
	}

	patch := client.MergeFrom(module.DeepCopy())
	module.Spec.Settings = mc.Spec.Settings
	module.Spec.SettingsVersion = mc.Spec.Version
	module.Spec.Maintenance = mc.Spec.Maintenance
	module.Spec.Enabled = mc.Spec.Enabled

	if err := r.client.Patch(ctx, module, patch); err != nil {
		logger.Error("failed to patch the module", log.Err(err))
		return fmt.Errorf("patch module '%s': %w", mc.Name, err)
	}

	// update embedded settings for the deckhouse
	if mc.Name == moduleDeckhouse {
		r.settingsCh <- mc.Spec.Settings.GetMap()
	}

	return nil
}

// handleDelete returns the module to its unconfigured state and releases the config.
func (r *reconciler) handleDelete(ctx context.Context, mc *v1alpha1.ModuleConfig) error {
	logger := r.logger.With(slog.String("name", mc.Name))

	logger.Debug("handle delete module config")
	defer logger.Debug("handle delete module config complete")

	module := new(v1alpha2.Module)
	if err := r.client.Get(ctx, client.ObjectKey{Name: mc.Name}, module); err != nil {
		if !apierrors.IsNotFound(err) {
			logger.Error("failed to get the module", log.Err(err))
			return fmt.Errorf("get module '%s': %w", mc.Name, err)
		}

		// a module that is already gone has no settings left to clear
		logger.Warn("module not found")

		return r.releaseConfig(ctx, mc)
	}

	// Clearing every mirrored field, rather than only the ones this config set, is what
	// returns the module to the defaults it had before the config existed.
	patch := client.MergeFrom(module.DeepCopy())
	module.Spec.Settings = nil
	module.Spec.SettingsVersion = 0
	module.Spec.Maintenance = ""
	module.Spec.Enabled = nil

	if err := r.client.Patch(ctx, module, patch); err != nil {
		logger.Error("failed to patch the module", log.Err(err))
		return fmt.Errorf("patch module '%s': %w", mc.Name, err)
	}

	return r.releaseConfig(ctx, mc)
}

// markUnknownModule records on the config that no module of that name exists.
func (r *reconciler) markUnknownModule(ctx context.Context, mc *v1alpha1.ModuleConfig) error {
	if mc.Status.Message == v1alpha1.ModuleConfigMessageUnknownModule {
		return nil
	}

	patch := client.MergeFrom(mc.DeepCopy())
	mc.Status.Message = v1alpha1.ModuleConfigMessageUnknownModule

	if err := r.client.Status().Patch(ctx, mc, patch); err != nil {
		return fmt.Errorf("patch module config status '%s': %w", mc.Name, err)
	}

	return nil
}

// addFinalizer claims the config so its deletion reaches handleDelete.
func (r *reconciler) addFinalizer(ctx context.Context, mc *v1alpha1.ModuleConfig) error {
	if controllerutil.ContainsFinalizer(mc, v1alpha1.ModuleConfigFinalizer) {
		return nil
	}

	patch := client.MergeFrom(mc.DeepCopy())
	controllerutil.AddFinalizer(mc, v1alpha1.ModuleConfigFinalizer)

	if err := r.client.Patch(ctx, mc, patch); err != nil {
		return fmt.Errorf("patch module config '%s': %w", mc.Name, err)
	}

	return nil
}

// releaseConfig drops the finalizer and the one-shot allow-disabling annotation, which
// must not outlive the config that carried it.
func (r *reconciler) releaseConfig(ctx context.Context, mc *v1alpha1.ModuleConfig) error {
	_, allowDisable := mc.Annotations[v1alpha1.ModuleConfigAnnotationAllowDisable]

	if !controllerutil.ContainsFinalizer(mc, v1alpha1.ModuleConfigFinalizer) && !allowDisable {
		return nil
	}

	patch := client.MergeFrom(mc.DeepCopy())
	controllerutil.RemoveFinalizer(mc, v1alpha1.ModuleConfigFinalizer)
	delete(mc.Annotations, v1alpha1.ModuleConfigAnnotationAllowDisable)

	if err := r.client.Patch(ctx, mc, patch); err != nil {
		return fmt.Errorf("patch module config '%s': %w", mc.Name, err)
	}

	return nil
}
