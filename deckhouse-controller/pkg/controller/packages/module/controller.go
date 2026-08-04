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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
	packageruntime "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
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
	// finalizerRequeueAfter is the short delay used to re-read the module right after
	// its finalizer has been added.
	finalizerRequeueAfter = 500 * time.Millisecond
)

// RegisterController registers the Module controller with the manager.
func RegisterController(sync *sync.WaitGroup, runtime ctrlmanager.Manager, manager packageManager, logger *log.Logger) error {
	r := &reconciler{
		init:    sync,
		client:  runtime.GetClient(),
		manager: manager,
		logger:  logger.Named(controllerName),
	}

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
	logger  *log.Logger
}

// packageManager registers and unregisters modules in the package runtime.
type packageManager interface {
	UpdateModulesSettings(name string, settingsVersion int, settings addonutils.Values, maintenance string, enabled *bool)
	UpdateModule(repo registry.Remote, module packageruntime.Module, force bool)
	RemoveModule(name string)
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
		r.logger.Info("deleting the module", slog.String("name", req.Name))
		return r.handleDelete(ctx, module)
	}

	// handle create/update events
	return r.handleCreateOrUpdate(ctx, module)
}

// handleCreateOrUpdate validates the module's package and version, moves the module
// onto them and hands it to the package runtime.
func (r *reconciler) handleCreateOrUpdate(ctx context.Context, module *v1alpha2.Module) (ctrl.Result, error) {
	defer r.logger.Debug("module reconciled", slog.String("name", module.Name))

	// The finalizer is claimed before the module reaches the runtime: it is what
	// guarantees handleDelete gets to call RemoveModule and release the installed lists.
	if !controllerutil.ContainsFinalizer(module, v1alpha2.ModuleFinalizerStatisticRegistered) {
		patch := client.MergeFrom(module.DeepCopy())
		controllerutil.AddFinalizer(module, v1alpha2.ModuleFinalizerStatisticRegistered)

		if err := r.client.Patch(ctx, module, patch); err != nil {
			r.logger.Error("failed to add the module finalizer", slog.String("name", module.Name), log.Err(err))
			return ctrl.Result{}, fmt.Errorf("patch: %w", err)
		}

		return ctrl.Result{RequeueAfter: finalizerRequeueAfter}, nil
	}

	pkg := new(v1alpha1.ModulePackage)
	if err := r.client.Get(ctx, client.ObjectKey{Name: module.Spec.PackageName}, pkg); err != nil {
		if !apierrors.IsNotFound(err) {
			r.logger.Error("failed to get the module package", slog.String("package", module.Spec.PackageName), log.Err(err))
			return ctrl.Result{}, fmt.Errorf("get module package: %w", err)
		}

		return r.blocked(ctx, module, v1alpha2.ModuleConditionReasonModulePackageNotFound,
			fmt.Sprintf("module package %q not found", module.Spec.PackageName))
	}

	versionName := v1alpha1.MakeModulePackageVersionName(module.Spec.PackageRepositoryName, module.Spec.PackageName, module.Spec.PackageVersion)

	mpv := new(v1alpha1.ModulePackageVersion)
	if err := r.client.Get(ctx, client.ObjectKey{Name: versionName}, mpv); err != nil {
		if !apierrors.IsNotFound(err) {
			r.logger.Error("failed to get the module package version", slog.String("version", versionName), log.Err(err))
			return ctrl.Result{}, fmt.Errorf("get module package version: %w", err)
		}

		return r.blocked(ctx, module, v1alpha2.ModuleConditionReasonVersionNotFound,
			fmt.Sprintf("module package version %q not found", versionName))
	}

	// a draft version is not published, so it must never reach the runtime
	if mpv.IsDraft() {
		return r.blocked(ctx, module, v1alpha2.ModuleConditionReasonVersionIsDraft,
			fmt.Sprintf("module package version %q is draft", versionName))
	}

	repo := new(v1alpha1.PackageRepository)
	if err := r.client.Get(ctx, client.ObjectKey{Name: module.Spec.PackageRepositoryName}, repo); err != nil {
		if !apierrors.IsNotFound(err) {
			r.logger.Error("failed to get the package repository", slog.String("repository", module.Spec.PackageRepositoryName), log.Err(err))
			return ctrl.Result{}, fmt.Errorf("get package repository: %w", err)
		}

		return r.blocked(ctx, module, v1alpha2.ModuleConditionReasonRepositoryNotFound,
			fmt.Sprintf("package repository %q not found", module.Spec.PackageRepositoryName))
	}

	if err := r.relink(ctx, module, pkg, mpv); err != nil {
		r.logger.Error("failed to relink the module", slog.String("name", module.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	r.deployPackage(registry.BuildRemote(repo), module)

	patch := client.MergeFrom(module.DeepCopy())
	setOwnerReferences(module, pkg, mpv)
	delete(module.Annotations, v1alpha2.ModuleAnnotationRegistrySpecChanged)

	if err := r.client.Patch(ctx, module, patch); err != nil {
		r.logger.Error("failed to patch the module", slog.String("name", module.Name), log.Err(err))
		return ctrl.Result{}, fmt.Errorf("patch: %w", err)
	}

	if err := r.setCompleted(ctx, module, metav1.ConditionTrue, v1alpha2.ModuleConditionTypeCompleted, ""); err != nil {
		r.logger.Error("failed to update the module status", slog.String("name", module.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// deployPackage registers the module in the package runtime.
//
// Settings go first: UpdateModulesSettings records the enabled intent even for a module
// the runtime does not track yet, so the scheduler's config rule sees it the moment
// UpdateModule registers the package. Once the module is tracked, a settings-only change
// is fully applied here and the following UpdateModule detects nothing left to do, which
// keeps the light Reschedule path instead of the Disable/Deploy/Load pipeline.
//
// The update is never forced: a module package version is immutable, so a cached copy of
// it is never stale.
func (r *reconciler) deployPackage(repo registry.Remote, module *v1alpha2.Module) {
	settings := module.Spec.Settings.GetMap()

	r.manager.UpdateModulesSettings(module.Name, module.Spec.SettingsVersion, settings, module.Spec.Maintenance, module.Spec.Enabled)

	// Definition.Name carries the package name for completeness; the runtime pulls a
	// module by its own name, so today the two have to match.
	r.manager.UpdateModule(repo, packageruntime.Module{
		Name: module.Name,
		Definition: modules.Definition{
			Name:    module.Spec.PackageName,
			Version: module.Spec.PackageVersion,
		},
		Settings:        settings,
		SettingsVersion: module.Spec.SettingsVersion,
		Maintenance:     module.Spec.Maintenance,
	}, false)
}

// handleDelete detaches the module from its package and version, unregisters it from
// the package runtime and releases the finalizer.
func (r *reconciler) handleDelete(ctx context.Context, module *v1alpha2.Module) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(module, v1alpha2.ModuleFinalizerStatisticRegistered) {
		return ctrl.Result{}, nil
	}

	// Detach by owner reference, not by spec: the references name what the module was
	// actually attached to, which a spec edit just before deletion would have changed.
	if name := ownerRefName(module, v1alpha1.ModulePackageVersionKind); name != "" {
		if err := r.detachVersion(ctx, module, name); err != nil {
			r.logger.Error("failed to detach the module package version", slog.String("version", name), log.Err(err))
			return ctrl.Result{}, err
		}
	}

	if name := ownerRefName(module, v1alpha1.ModulePackageKind); name != "" {
		if err := r.detachPackage(ctx, module, name); err != nil {
			r.logger.Error("failed to detach the module package", slog.String("package", name), log.Err(err))
			return ctrl.Result{}, err
		}
	}

	r.manager.RemoveModule(module.Name)

	patch := client.MergeFrom(module.DeepCopy())
	controllerutil.RemoveFinalizer(module, v1alpha2.ModuleFinalizerStatisticRegistered)

	if err := r.client.Patch(ctx, module, patch); err != nil {
		r.logger.Error("failed to remove the module finalizer", slog.String("name", module.Name), log.Err(err))
		return ctrl.Result{}, fmt.Errorf("patch: %w", err)
	}

	return ctrl.Result{}, nil
}

// relink moves the module onto pkg and version, releasing the ones it switched away
// from. Detaching first keeps the installed counts correct when only one of the two
// changed, which is the common case of a version bump within the same package.
func (r *reconciler) relink(ctx context.Context, module *v1alpha2.Module, pkg *v1alpha1.ModulePackage, mpv *v1alpha1.ModulePackageVersion) error {
	if old := ownerRefName(module, v1alpha1.ModulePackageVersionKind); old != "" && old != mpv.Name {
		if err := r.detachVersion(ctx, module, old); err != nil {
			return err
		}
	}

	if old := ownerRefName(module, v1alpha1.ModulePackageKind); old != "" && old != pkg.Name {
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

// blocked records why the module cannot reach the runtime and asks for a retry. Every
// reason routed here needs a change to another object to clear.
func (r *reconciler) blocked(ctx context.Context, module *v1alpha2.Module, reason, message string) (ctrl.Result, error) {
	r.logger.Warn("module cannot be deployed", slog.String("name", module.Name), slog.String("reason", reason))

	if err := r.setCompleted(ctx, module, metav1.ConditionFalse, reason, message); err != nil {
		r.logger.Error("failed to update the module status", slog.String("name", module.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
}

// setCompleted patches the Completed condition, the only condition this controller owns.
func (r *reconciler) setCompleted(ctx context.Context, module *v1alpha2.Module, status metav1.ConditionStatus, reason, message string) error {
	original := module.DeepCopy()

	// SetStatusCondition keeps LastTransitionTime unless the status actually flips, so a
	// steady state patches an empty diff instead of rewriting the condition every scan.
	meta.SetStatusCondition(&module.Status.Conditions, metav1.Condition{
		Type:               v1alpha2.ModuleConditionTypeCompleted,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: module.Generation,
	})

	if err := r.client.Status().Patch(ctx, module, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch status: %w", err)
	}

	return nil
}

// setOwnerReferences points the module at its current package and version, dropping the
// references it switched away from. Both are non-controller references that block owner
// deletion, so neither object can disappear from under a running module.
func setOwnerReferences(module *v1alpha2.Module, pkg *v1alpha1.ModulePackage, version *v1alpha1.ModulePackageVersion) {
	refs := make([]metav1.OwnerReference, 0, len(module.GetOwnerReferences())+2)
	for _, ref := range module.GetOwnerReferences() {
		if ref.Kind == v1alpha1.ModulePackageKind || ref.Kind == v1alpha1.ModulePackageVersionKind {
			continue
		}

		refs = append(refs, ref)
	}

	refs = append(refs,
		ownerReference(v1alpha1.ModulePackageVersionGVK, version.Name, version.UID),
		ownerReference(v1alpha1.ModulePackageGVK, pkg.Name, pkg.UID),
	)

	module.SetOwnerReferences(refs)
}

// ownerReference builds a non-controller owner reference that blocks the owner's deletion.
func ownerReference(gvk schema.GroupVersionKind, name string, uid types.UID) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               name,
		UID:                uid,
		Controller:         new(false),
		BlockOwnerDeletion: new(true),
	}
}

// ownerRefName returns the name of the module's owner reference of the given kind, or an
// empty string when it has none. It is how a package or version switch is detected.
func ownerRefName(module *v1alpha2.Module, kind string) string {
	for _, ref := range module.GetOwnerReferences() {
		if ref.Kind == kind {
			return ref.Name
		}
	}

	return ""
}
