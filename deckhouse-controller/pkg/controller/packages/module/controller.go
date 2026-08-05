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

	addonutils "github.com/flant/addon-operator/pkg/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
	packageruntime "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime"
	packagestatus "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/ctrlutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/module/status"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// controllerName is the name the controller is registered under in the manager.
	controllerName = "d8-module-controller"

	// maxConcurrentReconciles keeps a single worker: modules sharing a package write
	// the same ModulePackage and ModulePackageVersion installed lists.
	maxConcurrentReconciles = 1
	// cacheSyncTimeout bounds the initial informer cache sync.
	cacheSyncTimeout = 3 * time.Minute
	// defaultRequeueAfter is the retry delay for states that need an external change to
	// make progress, such as a missing package or a version still in draft.
	defaultRequeueAfter = 30 * time.Second
)

// RegisterController registers the Module controller with the manager.
func RegisterController(
	sync *sync.WaitGroup,
	runtime ctrlmanager.Manager,
	manager packageManager,
	logger *log.Logger,
) error {
	r := &reconciler{
		init:    sync,
		client:  runtime.GetClient(),
		manager: manager,
		logger:  logger.Named(controllerName),
	}

	r.status = status.NewService(r.client, r.manager.GetStatus, r.logger)
	r.status.Start(context.Background(), r.manager.GetModuleStatusQueue())

	if err := ctrl.NewControllerManagedBy(runtime).
		Named(controllerName).
		For(&v1alpha2.Module{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: maxConcurrentReconciles,
			CacheSyncTimeout:        cacheSyncTimeout,
		}).
		Complete(r); err != nil {
		return fmt.Errorf("complete: %w", err)
	}

	return nil
}

// reconciler reconciles Module objects.
type reconciler struct {
	init    *sync.WaitGroup
	client  client.Client
	manager packageManager
	status  *status.Service
	logger  *log.Logger
}

// packageManager registers and unregisters modules in the package runtime.
type packageManager interface {
	UpdateModulesSettings(name string, settingsVersion int, settings addonutils.Values, maintenance string, enabled *bool)
	UpdateModule(repo registry.Remote, module packageruntime.Module, force bool)
	RemoveModule(name string)
	GetStatus(name string) packagestatus.Status
	GetModuleStatusQueue() workqueue.TypedRateLimitingInterface[string]
}

// Reconcile dispatches the module to the delete or the create/update handler.
func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// wait for init
	r.init.Wait()

	r.logger.Debug("reconcile module", slog.String("name", req.Name))

	module := new(v1alpha2.Module)
	if err := r.client.Get(ctx, client.ObjectKey{Name: req.Name}, module); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Warn("module not found", slog.String("name", req.Name))
			return ctrl.Result{}, nil
		}

		r.logger.Error("failed to get module", slog.String("name", req.Name), log.Err(err))
		return ctrl.Result{Requeue: true}, nil
	}

	// handle delete event
	if !module.DeletionTimestamp.IsZero() {
		if err := r.handleDelete(ctx, module); err != nil {
			return ctrl.Result{}, fmt.Errorf("delete: %w", err)
		}

		return ctrl.Result{}, nil
	}

	// handle create/update events
	if err := r.handleCreateOrUpdate(ctx, module); err != nil {
		r.logger.Warn("failed to handle module", slog.String("name", req.Name), log.Err(err))

		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	return ctrl.Result{}, nil
}

// handleCreateOrUpdate validates the module's package and version, moves the module
// onto them and hands it to the package runtime.
func (r *reconciler) handleCreateOrUpdate(ctx context.Context, module *v1alpha2.Module) error {
	logger := r.logger.With(slog.String("name", module.Name))

	logger.Debug("handle module")
	defer logger.Debug("handle module complete")

	original := module.DeepCopy()

	// The finalizer is claimed before the module reaches the runtime: it is what
	// guarantees handleDelete gets to call RemoveModule and release the installed lists.
	if !controllerutil.ContainsFinalizer(module, v1alpha2.ModuleFinalizerStatisticRegistered) {
		patch := client.MergeFrom(module.DeepCopy())
		controllerutil.AddFinalizer(module, v1alpha2.ModuleFinalizerStatisticRegistered)

		if err := r.client.Patch(ctx, module, patch); err != nil {
			logger.Error("failed to add the module finalizer", log.Err(err))
			return fmt.Errorf("patch module '%s': %w", module.Name, err)
		}

		original = module.DeepCopy()
	}

	pkg := new(v1alpha1.ModulePackage)
	if err := r.client.Get(ctx, client.ObjectKey{Name: module.Name}, pkg); err != nil {
		logger.Debug("module package not found", slog.String("package", module.Name), log.Err(err))

		return fmt.Errorf("get module package '%s': %w", module.Name, err)
	}

	versionName := v1alpha1.MakeModulePackageVersionName(module.Spec.PackageRepositoryName, module.Name, module.Spec.PackageVersion)

	mpv := new(v1alpha1.ModulePackageVersion)
	if err := r.client.Get(ctx, client.ObjectKey{Name: versionName}, mpv); err != nil {
		logger.Debug("module package version not found", slog.String("mpv", versionName), log.Err(err))

		return fmt.Errorf("get module package version '%s': %w", versionName, err)
	}

	// a draft version is not published, so it must never reach the runtime
	if mpv.IsDraft() {
		logger.Debug("module package version is in draft", slog.String("mpv", versionName))

		return fmt.Errorf("module package version '%s' is draft", versionName)
	}

	// The repository is read before any installed list is touched, so a missing
	// repository cannot leave the lists half-updated.
	repo := new(v1alpha1.PackageRepository)
	if err := r.client.Get(ctx, client.ObjectKey{Name: module.Spec.PackageRepositoryName}, repo); err != nil {
		logger.Error("get package repository", log.Err(err))
		return fmt.Errorf("get package repository '%s': %w", module.Spec.PackageRepositoryName, err)
	}

	if err := r.relink(ctx, module, pkg, mpv); err != nil {
		logger.Error("failed to relink the module", log.Err(err))
		return err
	}

	r.manager.UpdateModule(registry.BuildRemote(repo), packageruntime.Module{
		Name: module.Name,
		Definition: modules.Definition{
			Name:    module.Name,
			Version: module.Spec.PackageVersion,
		},
		Settings:        module.Spec.Settings.GetMap(),
		SettingsVersion: module.Spec.SettingsVersion,
		Maintenance:     module.Spec.Maintenance,
		Enabled:         module.Spec.Enabled,
	}, false)

	// Both references are non-controller and block owner deletion, so neither the package
	// nor the version can disappear from under a running module.
	ctrlutils.ReplaceOwnerReferences(module,
		ctrlutils.OwnerReference(v1alpha1.ModulePackageVersionGVK, mpv.Name, mpv.UID),
		ctrlutils.OwnerReference(v1alpha1.ModulePackageGVK, pkg.Name, pkg.UID),
	)
	delete(module.Annotations, v1alpha2.ModuleAnnotationRegistrySpecChanged)

	if err := r.client.Patch(ctx, module, client.MergeFrom(original)); err != nil {
		logger.Error("failed to patch the module", log.Err(err))
		return fmt.Errorf("patch module '%s': %w", module.Name, err)
	}

	return nil
}

// handleDelete detaches the module from its package and version, unregisters it from
// the package runtime and releases the finalizer.
func (r *reconciler) handleDelete(ctx context.Context, module *v1alpha2.Module) error {
	logger := r.logger.With(slog.String("name", module.Name))

	logger.Debug("handle delete module")
	defer logger.Debug("handle delete module complete")

	// Detach by owner reference, not by spec: the references name what the module was
	// actually attached to, which a spec edit just before deletion would have changed.
	if name := ctrlutils.OwnerRefName(module, v1alpha1.ModulePackageVersionKind); name != "" {
		if err := r.detachVersion(ctx, module, name); err != nil {
			logger.Error("failed to detach the module package version", slog.String("mpv", name), log.Err(err))
			return err
		}
	}

	if name := ctrlutils.OwnerRefName(module, v1alpha1.ModulePackageKind); name != "" {
		if err := r.detachPackage(ctx, module, name); err != nil {
			logger.Error("failed to detach the module package", slog.String("package", name), log.Err(err))
			return err
		}
	}

	r.manager.RemoveModule(module.Name)

	if !controllerutil.ContainsFinalizer(module, v1alpha2.ModuleFinalizerStatisticRegistered) {
		return nil
	}

	patch := client.MergeFrom(module.DeepCopy())
	controllerutil.RemoveFinalizer(module, v1alpha2.ModuleFinalizerStatisticRegistered)

	if err := r.client.Patch(ctx, module, patch); err != nil {
		logger.Error("failed to remove the module finalizer", log.Err(err))
		return fmt.Errorf("patch module '%s': %w", module.Name, err)
	}

	return nil
}

// relink moves the module onto pkg and version, releasing the ones it switched away
// from. Detaching first keeps the installed counts correct when only one of the two
// changed, which is the common case of a version bump within the same package.
func (r *reconciler) relink(ctx context.Context, module *v1alpha2.Module, pkg *v1alpha1.ModulePackage, mpv *v1alpha1.ModulePackageVersion) error {
	if old := ctrlutils.OwnerRefName(module, v1alpha1.ModulePackageVersionKind); old != "" && old != mpv.Name {
		if err := r.detachVersion(ctx, module, old); err != nil {
			return err
		}
	}

	if old := ctrlutils.OwnerRefName(module, v1alpha1.ModulePackageKind); old != "" && old != pkg.Name {
		if err := r.detachPackage(ctx, module, old); err != nil {
			return err
		}
	}

	if err := r.attachVersion(ctx, module, mpv); err != nil {
		return err
	}

	return r.attachPackage(ctx, module, pkg)
}

// attachVersion adds the module to the version's installed list.
func (r *reconciler) attachVersion(ctx context.Context, module *v1alpha2.Module, mpv *v1alpha1.ModulePackageVersion) error {
	if mpv.IsModuleInstalled(app.NamespaceDeckhouse, module.Name) {
		return nil
	}

	patch := client.MergeFrom(mpv.DeepCopy())
	mpv = mpv.AddInstalledModule(app.NamespaceDeckhouse, module.Name)

	if err := r.client.Status().Patch(ctx, mpv, patch); err != nil {
		return fmt.Errorf("patch module package version status: %w", err)
	}

	return nil
}

// attachPackage adds the module to the package's installed list, or refreshes the
// version recorded there when the module moved to another version of the same package.
func (r *reconciler) attachPackage(ctx context.Context, module *v1alpha2.Module, pkg *v1alpha1.ModulePackage) error {
	installed := pkg.IsModuleInstalled(app.NamespaceDeckhouse, module.Name)
	if installed && pkg.GetModuleVersion(app.NamespaceDeckhouse, module.Name) == module.Spec.PackageVersion {
		return nil
	}

	patch := client.MergeFrom(pkg.DeepCopy())

	if installed {
		pkg.UpdateModuleVersion(app.NamespaceDeckhouse, module.Name, module.Spec.PackageVersion)
	} else {
		pkg = pkg.AddInstalledModule(app.NamespaceDeckhouse, module.Name, module.Spec.PackageVersion)
	}

	if err := r.client.Status().Patch(ctx, pkg, patch); err != nil {
		return fmt.Errorf("patch module package status: %w", err)
	}

	return nil
}

// detachVersion removes the module from the named version's installed list. A version
// that is already gone needs no cleanup.
func (r *reconciler) detachVersion(ctx context.Context, module *v1alpha2.Module, name string) error {
	version := new(v1alpha1.ModulePackageVersion)
	if err := r.client.Get(ctx, client.ObjectKey{Name: name}, version); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("get module package version: %w", err)
	}

	if !version.IsModuleInstalled(app.NamespaceDeckhouse, module.Name) {
		return nil
	}

	patch := client.MergeFrom(version.DeepCopy())
	version = version.RemoveInstalledModule(app.NamespaceDeckhouse, module.Name)

	if err := r.client.Status().Patch(ctx, version, patch); err != nil {
		return fmt.Errorf("patch module package version status: %w", err)
	}

	return nil
}

// detachPackage removes the module from the named package's installed list. A package
// that is already gone needs no cleanup.
func (r *reconciler) detachPackage(ctx context.Context, module *v1alpha2.Module, name string) error {
	pkg := new(v1alpha1.ModulePackage)
	if err := r.client.Get(ctx, client.ObjectKey{Name: name}, pkg); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("get module package: %w", err)
	}

	if !pkg.IsModuleInstalled(app.NamespaceDeckhouse, module.Name) {
		return nil
	}

	patch := client.MergeFrom(pkg.DeepCopy())
	pkg = pkg.RemoveInstalledModule(app.NamespaceDeckhouse, module.Name)

	if err := r.client.Status().Patch(ctx, pkg, patch); err != nil {
		return fmt.Errorf("patch module package status: %w", err)
	}

	return nil
}
