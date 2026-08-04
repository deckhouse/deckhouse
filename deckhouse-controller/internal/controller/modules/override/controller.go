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

package override

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// controllerName is the name the controller is registered under in the manager.
	controllerName = "d8-module-override-controller"

	// maxConcurrentReconciles keeps a single worker, so overrides are handled one at a time.
	maxConcurrentReconciles = 1
	// cacheSyncTimeout bounds the initial informer cache sync.
	cacheSyncTimeout = 3 * time.Minute
	// defaultRequeueAfter is the retry delay for states that need an external change to
	// make progress, such as a missing module or a missing module source.
	defaultRequeueAfter = time.Minute
	// finalizerRequeueAfter is the short delay used to re-read the override right after
	// its finalizer has been added.
	finalizerRequeueAfter = 500 * time.Millisecond
	// defaultScanInterval mirrors the CRD default and backs a non-positive interval.
	defaultScanInterval = 15 * time.Second
	// digestFetchTimeout bounds a digest lookup so an unreachable registry cannot stall the
	// controller's single worker.
	digestFetchTimeout = 30 * time.Second
)

// RegisterController registers the ModulePullOverride controller with the manager.
// The initWG wait group is owned by the caller and gates reconciliation until the
// controller's startup sync has finished.
func RegisterController(
	ctrlManager ctrlmanager.Manager,
	manager packageManager,
	preflight *sync.WaitGroup,
	logger *log.Logger,
) error {
	r := &reconciler{
		init:    preflight,
		client:  ctrlManager.GetClient(),
		manager: manager,
		logger:  logger.Named(controllerName),
	}

	if err := ctrl.NewControllerManagedBy(ctrlManager).
		Named(controllerName).
		For(&v1alpha2.ModulePullOverride{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})).
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

// reconciler reconciles ModulePullOverride objects.
type reconciler struct {
	init    *sync.WaitGroup
	client  client.Client
	manager packageManager
	logger  *log.Logger
}

// packageManager registers and unregisters overridden modules in the package runtime.
type packageManager interface {
	GetModuleDigest(ctx context.Context, remote registry.Remote, name, tag string) (string, error)
	UpdateModule(repo registry.Remote, module runtime.Module, force bool)
	RemoveModule(name string)
}

// Reconcile dispatches the override to the delete or the create/update handler.
func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// wait for init
	r.init.Wait()

	r.logger.Debug("reconcile module pull override", slog.String("name", req.Name))
	mpo := new(v1alpha2.ModulePullOverride)
	if err := r.client.Get(ctx, client.ObjectKey{Name: req.Name}, mpo); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Warn("module pull override not found", slog.String("name", req.Name))
			return ctrl.Result{}, nil
		}

		r.logger.Error("failed to get module pull override", slog.String("name", req.Name), log.Err(err))
		return ctrl.Result{Requeue: true}, nil
	}

	// handle delete event
	if !mpo.DeletionTimestamp.IsZero() {
		r.logger.Info("deleting the module pull override", slog.String("name", req.Name))
		return r.handleDelete(ctx, mpo)
	}

	// handle create/update events
	return r.handleCreateOrUpdate(ctx, mpo)
}

// handleCreateOrUpdate validates the override's module and source, resolves the image
// digest behind the override's tag and, when it changed, redeploys the package.
func (r *reconciler) handleCreateOrUpdate(ctx context.Context, mpo *v1alpha2.ModulePullOverride) (ctrl.Result, error) {
	defer r.logger.Debug("module pull override reconciled", slog.String("name", mpo.Name))

	module := new(v1alpha1.Module)
	if err := r.client.Get(ctx, client.ObjectKey{Name: mpo.Name}, module); err != nil {
		if !apierrors.IsNotFound(err) {
			r.logger.Error("failed to get module", slog.String("name", mpo.Name), log.Err(err))

			return ctrl.Result{}, fmt.Errorf("get: %w", err)
		}

		r.logger.Warn("module not found", slog.String("name", mpo.Name))
		if uerr := r.setStatusMessage(ctx, mpo, v1alpha1.ModulePullOverrideMessageModuleNotFound); uerr != nil {
			r.logger.Error("failed to update the module pull override status", slog.String("name", mpo.Name), log.Err(uerr))
			return ctrl.Result{}, uerr
		}

		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	// skip embedded modules
	if module.IsEmbedded() {
		r.logger.Debug("module is embedded, skip it", slog.String("name", mpo.Name))
		if uerr := r.setStatusMessage(ctx, mpo, v1alpha1.ModulePullOverrideMessageModuleEmbedded); uerr != nil {
			r.logger.Error("failed to update the module pull override status", slog.String("name", mpo.Name), log.Err(uerr))
			return ctrl.Result{}, uerr
		}

		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	// source must be
	if module.Properties.Source == "" {
		r.logger.Debug("module does not have an active source, skip it", slog.String("name", mpo.Name))
		if uerr := r.setStatusMessage(ctx, mpo, v1alpha1.ModulePullOverrideMessageNoSource); uerr != nil {
			r.logger.Error("failed to update the module pull override status", slog.String("name", mpo.Name), log.Err(uerr))
			return ctrl.Result{}, uerr
		}

		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	// set condition overridden for the module, only on transition: SetConditionTrue
	// restamps LastProbeTime, so an unconditional update writes status on every scan
	err := utils.UpdateStatus(ctx, r.client, module, func(module *v1alpha1.Module) bool {
		if module.IsCondition(v1alpha1.ModuleConditionIsOverridden, corev1.ConditionTrue) {
			return false
		}

		module.SetConditionTrue(v1alpha1.ModuleConditionIsOverridden)

		return true
	})
	if err != nil {
		r.logger.Error("failed to update module", slog.String("name", mpo.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	// set finalizer if it is not set
	if !controllerutil.ContainsFinalizer(mpo, v1alpha1.ModulePullOverrideFinalizer) {
		controllerutil.AddFinalizer(mpo, v1alpha1.ModulePullOverrideFinalizer)
		if err = r.client.Update(ctx, mpo); err != nil {
			r.logger.Error("failed to update the module pull override", slog.String("name", mpo.Name), log.Err(err))
		}

		return ctrl.Result{RequeueAfter: finalizerRequeueAfter}, nil
	}

	source := new(v1alpha1.ModuleSource)
	if err = r.client.Get(ctx, client.ObjectKey{Name: module.Properties.Source}, source); err != nil {
		if !apierrors.IsNotFound(err) {
			r.logger.Error("failed to get the module source for the module pull override", slog.String("source_name", module.Properties.Source), slog.String("target", mpo.Name), log.Err(err))
			return ctrl.Result{}, fmt.Errorf("get: %w", err)
		}

		if uerr := r.setStatusMessage(ctx, mpo, v1alpha1.ModulePullOverrideMessageSourceNotFound); uerr != nil {
			r.logger.Error("failed to update the module pull override status", slog.String("name", mpo.Name), log.Err(uerr))
			return ctrl.Result{}, uerr
		}

		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	digestCtx, cancel := context.WithTimeout(ctx, digestFetchTimeout)
	digest, err := r.manager.GetModuleDigest(digestCtx, registry.BuildRemote(source), mpo.Name, mpo.Spec.ImageTag)
	cancel()

	if err != nil {
		if uerr := r.setStatusMessage(ctx, mpo, fmt.Sprintf("Download error: %v", err)); uerr != nil {
			r.logger.Error("failed to update the module pull override status", slog.String("name", mpo.Name), log.Err(uerr))
			return ctrl.Result{}, uerr
		}

		r.logger.Error("failed to download dev image tag for the module pull override", slog.String("name", mpo.Name), log.Err(err))
		return ctrl.Result{RequeueAfter: scanInterval(mpo)}, nil
	}

	// check if module is up-to-date
	if digest == mpo.Status.ImageDigest {
		r.logger.Debug("module is up to date", slog.String("name", mpo.Name))
		if uerr := r.setStatusMessage(ctx, mpo, v1alpha1.ModulePullOverrideMessageReady); uerr != nil {
			r.logger.Error("failed to update the module pull override status", slog.String("name", mpo.Name), log.Err(uerr))
			return ctrl.Result{}, uerr
		}

		return ctrl.Result{RequeueAfter: scanInterval(mpo)}, nil
	}

	if err = r.deployPackage(ctx, source, mpo); err != nil {
		r.logger.Error("failed to deploy package", slog.String("module", mpo.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	mpo.Status.Message = v1alpha1.ModulePullOverrideMessageReady
	mpo.Status.ImageDigest = digest

	if err = r.updateModulePullOverrideStatus(ctx, mpo); err != nil {
		r.logger.Error("failed to update the module pull override status", slog.String("name", mpo.Name), log.Err(err))
		return ctrl.Result{}, err
	}

// Use mount point path: /modules/deployed/<module> (modules are mounted at /deckhouse/downloaded/modules/deployed/<module>)
	modulePath := filepath.Join("/modules/deployed", mpo.GetModuleName())
	ownerRef := metav1.OwnerReference{
		APIVersion: v1alpha2.ModulePullOverrideGVK.GroupVersion().String(),
		Kind:       v1alpha2.ModulePullOverrideGVK.Kind,
		Name:       mpo.GetName(),
		UID:        mpo.GetUID(),
		Controller: new(true),
	}

	if err = utils.EnsureModuleDocumentation(ctx, r.client, mpo.Name, module.Properties.Source, mpo.Status.ImageDigest, mpo.Spec.ImageTag, modulePath, ownerRef); err != nil {
		r.logger.Error("failed to ensure module documentation for the module pull override", slog.String("name", mpo.Name), log.Err(err))
		return ctrl.Result{}, fmt.Errorf("ensure module documentation: %w", err)
	}

	return ctrl.Result{RequeueAfter: scanInterval(mpo)}, nil
}

// scanInterval returns the delay before the next digest check. A non-positive interval
// reads as "do not requeue", which would silently stop scanning the override.
func scanInterval(mpo *v1alpha2.ModulePullOverride) time.Duration {
	if mpo.Spec.ScanInterval.Duration <= 0 {
		return defaultScanInterval
	}

	return mpo.Spec.ScanInterval.Duration
}

// deployPackage registers the overridden module in the package runtime, carrying over
// the settings from its module config, which is optional.
//
// The update is forced: an override pins a mutable tag, so the runtime sees an unchanged
// version and would reuse the copy it already deployed.
func (r *reconciler) deployPackage(ctx context.Context, source *v1alpha1.ModuleSource, mpo *v1alpha2.ModulePullOverride) error {
	config := new(v1alpha1.ModuleConfig)
	if err := r.client.Get(ctx, client.ObjectKeyFromObject(mpo), config); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("get module config: %w", err)
	}

	r.manager.UpdateModule(registry.BuildRemote(source), runtime.Module{
		Name: mpo.Name,
		Definition: modules.Definition{
			Version: mpo.Spec.ImageTag,
		},
		Settings:        config.Spec.Settings.GetMap(),
		Maintenance:     config.Spec.Maintenance,
		SettingsVersion: config.Spec.Version,
	}, true)

	return nil
}

// handleDelete unregisters the overridden module, clears the overridden
// condition on the module and releases the override's finalizer.
func (r *reconciler) handleDelete(ctx context.Context, mpo *v1alpha2.ModulePullOverride) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(mpo, v1alpha1.ModulePullOverrideFinalizer) {
		module := new(v1alpha1.Module)
		if err := r.client.Get(ctx, client.ObjectKey{Name: mpo.GetName()}, module); err != nil {
			if !apierrors.IsNotFound(err) {
				r.logger.Error("failed to get the module", slog.String("name", mpo.GetName()), log.Err(err))
				return ctrl.Result{Requeue: true}, nil
			}

			controllerutil.RemoveFinalizer(mpo, v1alpha1.ModulePullOverrideFinalizer)
			if err = r.client.Update(ctx, mpo); err != nil {
				r.logger.Error("failed to remove finalizer for the module pull override", slog.String("name", mpo.Name), log.Err(err))
				return ctrl.Result{Requeue: true}, nil
			}

			return ctrl.Result{}, nil
		}

		r.manager.RemoveModule(mpo.Name)

		err := utils.UpdateStatus(ctx, r.client, module, func(module *v1alpha1.Module) bool {
			module.SetConditionFalse(v1alpha1.ModuleConditionIsOverridden, "", "")
			return true
		})
		if err != nil {
			r.logger.Error("failed to update the module status", slog.String("name", mpo.Name), log.Err(err))
			return ctrl.Result{Requeue: true}, nil
		}

		controllerutil.RemoveFinalizer(mpo, v1alpha1.ModulePullOverrideFinalizer)
		if err = r.client.Update(ctx, mpo); err != nil {
			r.logger.Error("failed to remove finalizer for the module pull override", slog.String("name", mpo.Name), log.Err(err))
			return ctrl.Result{Requeue: true}, nil
		}
	}

	return ctrl.Result{}, nil
}

// setStatusMessage persists message as the override's status message. It is a no-op when
// the message is already set, so a steady state does not generate repeated status writes.
func (r *reconciler) setStatusMessage(ctx context.Context, mpo *v1alpha2.ModulePullOverride, message string) error {
	if mpo.Status.Message == message {
		return nil
	}

	mpo.Status.Message = message

	return r.updateModulePullOverrideStatus(ctx, mpo)
}

// updateModulePullOverrideStatus stamps the update time and writes the override status.
func (r *reconciler) updateModulePullOverrideStatus(ctx context.Context, mpo *v1alpha2.ModulePullOverride) error {
mpo.Status.UpdatedAt = metav1.NewTime(time.Now().UTC())
	if err := r.client.Status().Update(ctx, mpo); err != nil {
		return fmt.Errorf("update: %w", err)
	}

	return nil
}
