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
	"crypto/md5"
	"fmt"
	"log/slog"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/disk"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/observed"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1beta1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/ctrlutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/module/status"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// controllerName is the name the controller is registered under in the manager.
	controllerName = "d8-module-controller"

	// maxConcurrentReconciles keeps a single worker: concurrent reconciles would race on
	// the ModulePackageVersion used flag when a module bumps its version.
	maxConcurrentReconciles = 1
	// cacheSyncTimeout bounds the initial informer cache sync.
	cacheSyncTimeout = 3 * time.Minute
	// defaultRequeueAfter is the retry delay for states that need an external change to
	// make progress, such as a missing package or a version still in draft.
	defaultRequeueAfter = 30 * time.Second
	// removalRequeueAfter is how often a running teardown is re-checked. Its end reaches only the
	// status queue, not this controller, so the only way to notice is to ask the engine again.
	removalRequeueAfter = 5 * time.Second

	// globalModuleName is the reserved name of the global module, which the engine builds
	// itself out of the global hooks dir.
	globalModuleName = "global"

	// documentationDir is the modules dir of the engine, relative to the downloaded modules dir the
	// documentation controller reads ModuleDocumentation paths against.
	documentationDir = "/modules"
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
		status:  status.NewService(runtime.GetClient(), manager, logger),
		logger:  logger.Named(controllerName),
	}

	if err := runtime.Add(ctrlmanager.RunnableFunc(func(ctx context.Context) error {
		r.status.Start(ctx, r.manager.GetModuleStatusQueue())
		return nil
	})); err != nil {
		return fmt.Errorf("add status service: %w", err)
	}

	if err := ctrl.NewControllerManagedBy(runtime).
		Named(controllerName).
		For(&v1beta1.Module{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{}, currentVersionChanged())).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: maxConcurrentReconciles,
			CacheSyncTimeout:        cacheSyncTimeout,
		}).
		Complete(r); err != nil {
		return fmt.Errorf("complete: %w", err)
	}

	return nil
}

// currentVersionChanged passes a Module update whose activated version changed: the version's
// files are on disk only from then on, so that is when its documentation can be published.
func currentVersionChanged() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldModule, ok := e.ObjectOld.(*v1beta1.Module)
			if !ok {
				return false
			}

			newModule, ok := e.ObjectNew.(*v1beta1.Module)
			if !ok {
				return false
			}

			return activatedVersion(oldModule) != activatedVersion(newModule)
		},
	}
}

// activatedVersion returns the version the engine activated for the module; empty before the first.
func activatedVersion(module *v1beta1.Module) string {
	if module.Status.CurrentVersion == nil {
		return ""
	}

	return module.Status.CurrentVersion.Version
}

// reconciler reconciles Module objects.
type reconciler struct {
	init    *sync.WaitGroup
	client  client.Client
	manager packageManager
	status  *status.Service
	logger  *log.Logger
}

// packageManager hands modules and Global's settings to the package engine (engine.Engine).
type packageManager interface {
	ApplyModule(module engine.Module)
	ApplyGlobal(settings addonutils.Values, settingsVersion int)
	// RemoveModule removes the module and reports whether the removal has finished; true at once
	// for global.
	RemoveModule(name string) bool
	Report(name string) (model.Report, bool)
	Observed(name string) observed.Observation
	GetModuleStatusQueue() workqueue.TypedRateLimitingInterface[string]
}

// Reconcile dispatches the module to the delete or the create/update handler.
func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// wait for init
	r.init.Wait()

	r.logger.Debug("reconcile module", slog.String("name", req.Name))

	module := new(v1beta1.Module)
	if err := r.client.Get(ctx, client.ObjectKey{Name: req.Name}, module); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Warn("module not found", slog.String("name", req.Name))
			return ctrl.Result{}, nil
		}

		r.logger.Error("failed to get module", slog.String("name", req.Name), log.Err(err))
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	// handle delete event
	if !module.DeletionTimestamp.IsZero() {
		res, err := r.handleDelete(ctx, module)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("delete: %w", err)
		}

		return res, nil
	}

	// handle create/update events
	if err := r.handleCreateOrUpdate(ctx, module); err != nil {
		r.logger.Warn("failed to handle module", slog.String("name", req.Name), log.Err(err))

		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	return ctrl.Result{}, nil
}

// handleCreateOrUpdate validates the module's package and version, moves the module
// onto them and hands it to the package engine.
func (r *reconciler) handleCreateOrUpdate(ctx context.Context, module *v1beta1.Module) error {
	logger := r.logger.With(slog.String("name", module.Name))

	logger.Debug("handle module")
	defer logger.Debug("handle module complete")

	original := module.DeepCopy()

	// The finalizer is claimed before the module reaches the engine: it is what
	// guarantees handleDelete gets to call RemoveModule and release the version.
	if !controllerutil.ContainsFinalizer(module, v1beta1.ModuleFinalizerStatisticRegistered) {
		patch := client.MergeFrom(module.DeepCopy())
		controllerutil.AddFinalizer(module, v1beta1.ModuleFinalizerStatisticRegistered)

		if err := r.client.Patch(ctx, module, patch); err != nil {
			logger.Error("failed to add the module finalizer", log.Err(err))
			return fmt.Errorf("patch module '%s': %w", module.Name, err)
		}

		original = module.DeepCopy()
	}

	// The global module is routed ahead of every annotation: the engine holds the module
	// itself, so nothing about it is ever loaded, deployed or removed.
	if module.Name == globalModuleName {
		return r.handleGlobal(ctx, module, original)
	}

	// An embedded module ships inside the image: it has a package and a version like any other,
	// but no repository to pull it from.
	if module.IsEmbedded() {
		return r.handleEmbedded(ctx, module, original)
	}

	pkg, mpv, err := r.resolvePackage(ctx, module)
	if err != nil {
		return err
	}

	// The repository and the requirements are read before any used flag is touched, so a missing
	// repository or unparsable requirements cannot leave the versions half-updated.
	repo := new(v1alpha1.PackageRepository)
	if err := r.client.Get(ctx, client.ObjectKey{Name: module.Spec.PackageRepositoryName}, repo); err != nil {
		logger.Error("get package repository", log.Err(err))
		return fmt.Errorf("get package repository '%s': %w", module.Spec.PackageRepositoryName, err)
	}

	requirements, err := requirementsOf(mpv)
	if err != nil {
		logger.Error("parse the module package version requirements", log.Err(err))
		return fmt.Errorf("parse requirements of module package version '%s': %w", mpv.Name, err)
	}

	if err := r.relink(ctx, module, pkg, mpv); err != nil {
		logger.Error("failed to relink the module", log.Err(err))
		return err
	}

	r.manager.ApplyModule(engine.Module{
		Name:            module.Name,
		Version:         module.Spec.PackageVersion,
		Repository:      registry.BuildRemote(repo),
		Settings:        module.Spec.Settings.GetMap(),
		SettingsVersion: module.Spec.SettingsVersion,
		Maintenance:     module.Spec.Maintenance,
		Enabled:         module.Spec.Enabled,
		Requirements:    requirements,
	})

	if err := r.commit(ctx, module, original, pkg, mpv); err != nil {
		return err
	}

	if err := r.ensureDocumentation(ctx, module); err != nil {
		logger.Error("failed to ensure the module documentation", log.Err(err))
		return err
	}

	return nil
}

// handleEmbedded hands a module the image ships to the package engine. It passes the same package
// and version gate as a downloaded one — the bootstrap fills both from the module files on disk
// before any Module is written — and differs in exactly two ways: the reserved "embedded"
// repository resolves to no object, so none is read, and the files are already in place.
func (r *reconciler) handleEmbedded(ctx context.Context, module, original *v1beta1.Module) error {
	logger := r.logger.With(slog.String("name", module.Name))

	logger.Debug("handle embedded module")

	pkg, mpv, err := r.resolvePackage(ctx, module)
	if err != nil {
		return err
	}

	requirements, err := requirementsOf(mpv)
	if err != nil {
		logger.Error("parse the module package version requirements", log.Err(err))
		return fmt.Errorf("parse requirements of module package version '%s': %w", mpv.Name, err)
	}

	// relink drops the downloaded version a module the image started shipping after it had
	// already been released still points at.
	if err := r.relink(ctx, module, pkg, mpv); err != nil {
		logger.Error("failed to relink the module", log.Err(err))
		return err
	}

	// Embedded: the engine loads the files the image carries, at the version the image ships.
	r.manager.ApplyModule(engine.Module{
		Name:            module.Name,
		Embedded:        true,
		Settings:        module.Spec.Settings.GetMap(),
		SettingsVersion: module.Spec.SettingsVersion,
		Maintenance:     module.Spec.Maintenance,
		Enabled:         module.Spec.Enabled,
		Requirements:    requirements,
	})

	return r.commit(ctx, module, original, pkg, mpv)
}

// handleGlobal hands the global module's settings to the package engine. It passes the same
// package and version gate as an embedded module — the bootstrap fills both from the global
// hooks dir before any Module is written — but the engine built the module itself at startup,
// so the settings are the only thing that crosses over.
func (r *reconciler) handleGlobal(ctx context.Context, module, original *v1beta1.Module) error {
	logger := r.logger.With(slog.String("name", module.Name))

	logger.Debug("handle global module")

	pkg, mpv, err := r.resolvePackage(ctx, module)
	if err != nil {
		return err
	}

	if err := r.relink(ctx, module, pkg, mpv); err != nil {
		logger.Error("failed to relink the module", log.Err(err))
		return err
	}

	r.manager.ApplyGlobal(module.Spec.Settings.GetMap(), module.Spec.SettingsVersion)

	return r.commit(ctx, module, original, pkg, mpv)
}

// resolvePackage reads the module's package and the version its spec names. A version still in
// draft is not published, so its metadata must never reach the engine.
func (r *reconciler) resolvePackage(ctx context.Context, module *v1beta1.Module) (*v1alpha1.ModulePackage, *v1alpha1.ModulePackageVersion, error) {
	logger := r.logger.With(slog.String("name", module.Name))

	mp := new(v1alpha1.ModulePackage)
	if err := r.client.Get(ctx, client.ObjectKey{Name: module.Name}, mp); err != nil {
		logger.Debug("module package not found", slog.String("package", module.Name), log.Err(err))

		return nil, nil, fmt.Errorf("get module package '%s': %w", module.Name, err)
	}

	versionName := fmt.Sprintf("%s-%s", module.Spec.PackageRepositoryName, module.Name)
	if !module.IsEmbedded() {
		versionName = fmt.Sprintf("%s-%s", versionName, module.Spec.PackageVersion)
	}

	mpv := new(v1alpha1.ModulePackageVersion)
	if err := r.client.Get(ctx, client.ObjectKey{Name: versionName}, mpv); err != nil {
		logger.Debug("module package version not found", slog.String("mpv", versionName), log.Err(err))
		return nil, nil, fmt.Errorf("get module package version '%s': %w", versionName, err)
	}

	if mpv.IsDraft() {
		logger.Debug("module package version is in draft", slog.String("mpv", versionName))
		return nil, nil, fmt.Errorf("module package version '%s' is draft", versionName)
	}

	return mp, mpv, nil
}

// requirementsOf parses the requirements the version publishes at the module's order; none
// without metadata.
func requirementsOf(mpv *v1alpha1.ModulePackageVersion) (schedule.Constraints, error) {
	return mpv.Status.PackageMetadata.ToScheduleConstraints()
}

// commit records the package and the version as the module's owners and clears the registry
// annotation in one patch. Both references are non-controller and block owner deletion, so
// neither the package nor the version can disappear from under a running module.
func (r *reconciler) commit(ctx context.Context, module, original *v1beta1.Module, mp *v1alpha1.ModulePackage, mpv *v1alpha1.ModulePackageVersion) error {
	ctrlutils.ReplaceOwnerReferences(module,
		ctrlutils.OwnerReference(v1alpha1.ModulePackageVersionGVK, mpv.Name, mpv.UID),
		ctrlutils.OwnerReference(v1alpha1.ModulePackageGVK, mp.Name, mp.UID),
	)
	delete(module.Annotations, v1alpha1.PackageAnnotationRegistrySpecChanged)

	if err := r.client.Patch(ctx, module, client.MergeFrom(original)); err != nil {
		r.logger.Error("failed to patch the module", slog.String("name", module.Name), log.Err(err))
		return fmt.Errorf("patch module '%s': %w", module.Name, err)
	}

	return nil
}

// ensureDocumentation publishes the documentation of the version the engine activated, as a
// ModuleDocumentation the module controls, so it goes away with the module. Nothing is published
// before the first activation: until then the files are not on disk. A documentation the module
// does not control, such as a legacy release's, or one being deleted is left alone.
func (r *reconciler) ensureDocumentation(ctx context.Context, module *v1beta1.Module) error {
	version := activatedVersion(module)
	if version == "" {
		return nil
	}

	spec := v1alpha1.ModuleDocumentationSpec{
		Version:  version,
		Path:     disk.AttachPath(documentationDir, disk.Instance(module.Name, version)),
		Checksum: fmt.Sprintf("%x", md5.Sum([]byte(version))),
	}

	md := new(v1alpha1.ModuleDocumentation)
	if err := r.client.Get(ctx, client.ObjectKey{Name: module.Name}, md); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get module documentation '%s': %w", module.Name, err)
		}

		md = &v1alpha1.ModuleDocumentation{
			ObjectMeta: metav1.ObjectMeta{
				Name: module.Name,
				Labels: map[string]string{
					v1alpha1.ModuleReleaseLabelModule: module.Name,
					v1alpha1.ModuleReleaseLabelSource: module.Spec.PackageRepositoryName,
				},
				OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(module, v1beta1.ModuleGVK)},
			},
			Spec: spec,
		}

		if err = r.client.Create(ctx, md); err != nil {
			return fmt.Errorf("create module documentation '%s': %w", module.Name, err)
		}

		return nil
	}

	if !md.DeletionTimestamp.IsZero() || !metav1.IsControlledBy(md, module) || md.Spec == spec {
		return nil
	}

	patch := client.MergeFrom(md.DeepCopy())
	md.Spec = spec

	if err := r.client.Patch(ctx, md, patch); err != nil {
		return fmt.Errorf("patch module documentation '%s': %w", module.Name, err)
	}

	return nil
}

// handleDelete removes the module from the package engine and, once the engine reports the
// removal finished, releases the module's package version and the finalizer. The wait is unbounded
// on purpose: holding the finalizer is what keeps a Helm release from outliving the CR that owns
// it, so a teardown the queue keeps retrying keeps the module in Terminating rather than orphaning
// its resources.
func (r *reconciler) handleDelete(ctx context.Context, module *v1beta1.Module) (ctrl.Result, error) {
	logger := r.logger.With(slog.String("name", module.Name))

	logger.Debug("handle delete module")
	defer logger.Debug("handle delete module complete")

	// The engine removes the module asynchronously — the release, the hooks, the files — so the
	// removal is polled until it reports done. Global reports done at once: it is never removed,
	// the whole engine renders against it.
	if !r.manager.RemoveModule(module.Name) {
		logger.Info("module is still being removed by the engine")

		return ctrl.Result{RequeueAfter: removalRequeueAfter}, nil
	}

	// The removal only holds until the next start: the image still ships the module, so the
	// bootstrap recreates the resource and loads it again.
	if module.IsEmbedded() {
		logger.Info("embedded module is removed, it will be placed again on the next start")
	}

	// Detach only after the teardown: releasing the version any earlier lets it be garbage
	// collected — and its package files removed from disk — while the uninstall still needs them.
	//
	// Detach by owner reference, not by spec: the reference names what the module was
	// actually attached to, which a spec edit just before deletion would have changed.
	if name := ctrlutils.OwnerRefName(module, v1alpha1.ModulePackageVersionKind); name != "" {
		if err := r.detachVersion(ctx, name); err != nil {
			logger.Error("failed to detach the module package version", slog.String("mpv", name), log.Err(err))
			return ctrl.Result{}, err
		}
	}

	// The package is named after the module, so the reference and the name agree; the reference is
	// still preferred, and the name only carries a module attached before the reference was
	// written.
	pkgName := ctrlutils.OwnerRefName(module, v1alpha1.ModulePackageKind)
	if pkgName == "" {
		pkgName = module.Name
	}

	if err := r.detachPackage(ctx, pkgName); err != nil {
		logger.Error("failed to detach the module package", slog.String("package", pkgName), log.Err(err))
		return ctrl.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(module, v1beta1.ModuleFinalizerStatisticRegistered) {
		return ctrl.Result{}, nil
	}

	patch := client.MergeFrom(module.DeepCopy())
	controllerutil.RemoveFinalizer(module, v1beta1.ModuleFinalizerStatisticRegistered)

	if err := r.client.Patch(ctx, module, patch); err != nil {
		logger.Error("failed to remove the module finalizer", log.Err(err))
		return ctrl.Result{}, fmt.Errorf("patch module '%s': %w", module.Name, err)
	}

	return ctrl.Result{}, nil
}

// relink marks mpv as used by the module and records the version in pkg, releasing the version
// the module switched away from. Releasing first keeps the flags correct across a version bump.
//
// Only the version is ever switched away from: a module package is named after its module, so the
// module cannot move to another one and there is no old package to release here.
func (r *reconciler) relink(ctx context.Context, module *v1beta1.Module, pkg *v1alpha1.ModulePackage, mpv *v1alpha1.ModulePackageVersion) error {
	if old := ctrlutils.OwnerRefName(module, v1alpha1.ModulePackageVersionKind); old != "" && old != mpv.Name {
		if err := r.detachVersion(ctx, old); err != nil {
			return err
		}
	}

	if err := r.attachVersion(ctx, mpv); err != nil {
		return err
	}

	return r.attachPackage(ctx, module, pkg)
}

// attachVersion marks the version as used, so it cannot be deleted under the module.
func (r *reconciler) attachVersion(ctx context.Context, mpv *v1alpha1.ModulePackageVersion) error {
	if mpv.Status.Used {
		return nil
	}

	patch := client.MergeFrom(mpv.DeepCopy())
	mpv.Status.Used = true

	if err := r.client.Status().Patch(ctx, mpv, patch); err != nil {
		return fmt.Errorf("patch module package version status: %w", err)
	}

	return nil
}

// attachPackage records the module and the version it runs on in the package status, so a
// reader of the package can tell which version is in use without listing modules.
func (r *reconciler) attachPackage(ctx context.Context, module *v1beta1.Module, pkg *v1alpha1.ModulePackage) error {
	patch := client.MergeFrom(pkg.DeepCopy())

	if !pkg.AddInstalledModule(module.Spec.PackageVersion, module.Spec.PackageRepositoryName) {
		return nil
	}

	if err := r.client.Status().Patch(ctx, pkg, patch); err != nil {
		return fmt.Errorf("patch module package status '%s': %w", pkg.Name, err)
	}

	return nil
}

// detachPackage clears the named package's used-by entry. A package that is already gone,
// or was never recorded, needs no cleanup.
func (r *reconciler) detachPackage(ctx context.Context, name string) error {
	pkg := new(v1alpha1.ModulePackage)
	if err := r.client.Get(ctx, client.ObjectKey{Name: name}, pkg); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("get module package '%s': %w", name, err)
	}

	patch := client.MergeFrom(pkg.DeepCopy())

	if !pkg.RemoveInstalledModule() {
		return nil
	}

	if err := r.client.Status().Patch(ctx, pkg, patch); err != nil {
		return fmt.Errorf("patch module package status '%s': %w", name, err)
	}

	return nil
}

// detachVersion clears the named version's used flag. A version that is already gone,
// or was never marked, needs no cleanup.
func (r *reconciler) detachVersion(ctx context.Context, name string) error {
	version := new(v1alpha1.ModulePackageVersion)
	if err := r.client.Get(ctx, client.ObjectKey{Name: name}, version); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("get module package version: %w", err)
	}

	if !version.Status.Used {
		return nil
	}

	patch := client.MergeFrom(version.DeepCopy())
	version.Status.Used = false

	if err := r.client.Status().Patch(ctx, version, patch); err != nil {
		return fmt.Errorf("patch module package version status: %w", err)
	}

	return nil
}
