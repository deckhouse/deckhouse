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

	// maxConcurrentReconciles keeps a single worker: concurrent reconciles would race on
	// the ModulePackageVersion used flag when a module bumps its version.
	maxConcurrentReconciles = 1
	// cacheSyncTimeout bounds the initial informer cache sync.
	cacheSyncTimeout = 3 * time.Minute
	// defaultRequeueAfter is the retry delay for states that need an external change to
	// make progress, such as a missing package or a version still in draft.
	defaultRequeueAfter = 30 * time.Second
	// devRequeueAfter is how often a dev module's tag is re-resolved. A repush moves nothing
	// in the API server, so the digest behind the tag is only ever seen by looking again.
	devRequeueAfter = 15 * time.Second
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
	GetModuleDigest(ctx context.Context, repo registry.Remote, name, tag string) (string, error)
	UpdateEmbeddedModule(module packageruntime.Module)
	RemoveModule(name string)
	RemoveEmbeddedModule(name string)
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

	// A dev module follows a mutable tag, and a repush under it changes nothing the informer
	// watches, so the only way to notice one is to resolve the tag again on a timer.
	if module.IsDev() && !module.IsEmbedded() {
		return ctrl.Result{RequeueAfter: devRequeueAfter}, nil
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
	// guarantees handleDelete gets to call RemoveModule and release the version.
	if !controllerutil.ContainsFinalizer(module, v1alpha2.ModuleFinalizerStatisticRegistered) {
		patch := client.MergeFrom(module.DeepCopy())
		controllerutil.AddFinalizer(module, v1alpha2.ModuleFinalizerStatisticRegistered)

		if err := r.client.Patch(ctx, module, patch); err != nil {
			logger.Error("failed to add the module finalizer", log.Err(err))
			return fmt.Errorf("patch module '%s': %w", module.Name, err)
		}

		original = module.DeepCopy()
	}

	// An embedded module ships inside the image, so there is no package, version or repository to
	// resolve — the annotation is read before any of them, the way the bootstrap and the runtime do.
	if module.IsEmbedded() {
		return r.handleEmbedded(ctx, module, original)
	}

	// A dev module is pinned to a mutable tag, which the repository scan publishes no version
	// for, so it is routed before the package and the version are looked up — as embedded is.
	if module.IsDev() {
		return r.handleDev(ctx, module, original)
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

	// The repository is read before any used flag is touched, so a missing repository
	// cannot leave the versions half-updated.
	repo := new(v1alpha1.PackageRepository)
	if err := r.client.Get(ctx, client.ObjectKey{Name: module.Spec.PackageRepositoryName}, repo); err != nil {
		logger.Error("get package repository", log.Err(err))
		return fmt.Errorf("get package repository '%s': %w", module.Spec.PackageRepositoryName, err)
	}

	if err := r.relink(ctx, module, mpv); err != nil {
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

// handleEmbedded hands a module the image ships to the package runtime. Its files are already on
// disk, so the only thing it can drift against is its own settings — and the release path's version
// bookkeeping is unwound on the way through, for a module the image started shipping after it had
// already been downloaded.
func (r *reconciler) handleEmbedded(ctx context.Context, module, original *v1alpha2.Module) error {
	logger := r.logger.With(slog.String("name", module.Name))

	logger.Debug("handle embedded module")

	if name := ctrlutils.OwnerRefName(module, v1alpha1.ModulePackageVersionKind); name != "" {
		if err := r.detachVersion(ctx, name); err != nil {
			logger.Error("failed to detach the module package version", slog.String("mpv", name), log.Err(err))
			return fmt.Errorf("detach module package version '%s': %w", name, err)
		}
	}

	r.manager.UpdateEmbeddedModule(packageruntime.Module{
		Name:            module.Name,
		Settings:        module.Spec.Settings.GetMap(),
		SettingsVersion: module.Spec.SettingsVersion,
		Maintenance:     module.Spec.Maintenance,
		Enabled:         module.Spec.Enabled,
	})

	// A reference left behind would block its owner's deletion for ever, and an embedded module owns
	// neither a package nor a version. The registry annotation goes with them: nothing is pulled.
	ctrlutils.DropOwnerReferences(module, v1alpha1.ModulePackageVersionKind, v1alpha1.ModulePackageKind)
	delete(module.Annotations, v1alpha2.ModuleAnnotationRegistrySpecChanged)

	if err := r.client.Patch(ctx, module, client.MergeFrom(original)); err != nil {
		logger.Error("failed to patch the module", log.Err(err))
		return fmt.Errorf("patch module '%s': %w", module.Name, err)
	}

	return nil
}

// handleDev hands a module pinned to a mutable dev image tag to the package runtime, the way
// the override controller drives the addon-operator path.
//
// No ModulePackageVersion is published for such a tag, so the digest behind it takes the place of
// a version as the change signal: it is compared with the one the module was last handed over on,
// and a move forces the runtime past change detection that only ever sees the same tag. The digest
// is recorded after the handover, so a failure before it re-forces rather than skips.
func (r *reconciler) handleDev(ctx context.Context, module, original *v1alpha2.Module) error {
	logger := r.logger.With(slog.String("name", module.Name))

	logger.Debug("handle dev module")

	if module.Spec.PackageRepositoryName == "" {
		logger.Debug("dev module has no package repository")

		return fmt.Errorf("dev module '%s' has no package repository", module.Name)
	}

	repo := new(v1alpha1.PackageRepository)
	if err := r.client.Get(ctx, client.ObjectKey{Name: module.Spec.PackageRepositoryName}, repo); err != nil {
		logger.Error("failed to get the package repository", log.Err(err))
		return fmt.Errorf("get package repository '%s': %w", module.Spec.PackageRepositoryName, err)
	}

	remote := registry.BuildRemote(repo)

	digest, err := r.manager.GetModuleDigest(ctx, remote, module.Name, module.Spec.PackageVersion)
	if err != nil {
		logger.Error("failed to resolve the dev image digest", log.Err(err))
		return fmt.Errorf("get digest of the module '%s': %w", module.Name, err)
	}

	// Only the digest tells a repushed image from an untouched one, and force is what carries that
	// verdict past change detection, which sees the same tag either way.
	forced := module.Annotations[v1alpha2.ModuleAnnotationHash] != digest

	// A version the repository scan produced cannot back a dev tag, and the reference to it would
	// block its owner's deletion for ever — the module arrives with one when it was released before.
	if name := ctrlutils.OwnerRefName(module, v1alpha1.ModulePackageVersionKind); name != "" {
		if err := r.detachVersion(ctx, name); err != nil {
			logger.Error("failed to detach the module package version", slog.String("mpv", name), log.Err(err))
			return fmt.Errorf("detach module package version '%s': %w", name, err)
		}
	}

	r.manager.UpdateModule(remote, packageruntime.Module{
		Name: module.Name,
		Definition: modules.Definition{
			Name:    module.Name,
			Version: module.Spec.PackageVersion,
		},
		Settings:        module.Spec.Settings.GetMap(),
		SettingsVersion: module.Spec.SettingsVersion,
		Maintenance:     module.Spec.Maintenance,
		Enabled:         module.Spec.Enabled,
	}, forced)

	ctrlutils.DropOwnerReferences(module, v1alpha1.ModulePackageVersionKind, v1alpha1.ModulePackageKind)
	module.Annotations[v1alpha2.ModuleAnnotationHash] = digest
	delete(module.Annotations, v1alpha2.ModuleAnnotationRegistrySpecChanged)

	if err := r.client.Patch(ctx, module, client.MergeFrom(original)); err != nil {
		logger.Error("failed to patch the module", log.Err(err))
		return fmt.Errorf("patch module '%s': %w", module.Name, err)
	}

	return nil
}

// handleDelete releases the module's package version, unregisters it from the package
// runtime and releases the finalizer.
func (r *reconciler) handleDelete(ctx context.Context, module *v1alpha2.Module) error {
	logger := r.logger.With(slog.String("name", module.Name))

	logger.Debug("handle delete module")
	defer logger.Debug("handle delete module complete")

	// Detach by owner reference, not by spec: the reference names what the module was
	// actually attached to, which a spec edit just before deletion would have changed.
	if name := ctrlutils.OwnerRefName(module, v1alpha1.ModulePackageVersionKind); name != "" {
		if err := r.detachVersion(ctx, name); err != nil {
			logger.Error("failed to detach the module package version", slog.String("mpv", name), log.Err(err))
			return err
		}
	}

	// An embedded module deployed nothing, so its removal enqueues no undeploy. The removal only
	// holds until the next start: the image still ships the module, so the bootstrap recreates the
	// resource and loads it again.
	if module.IsEmbedded() {
		logger.Info("embedded module is removed, it will be placed again on the next start")

		r.manager.RemoveEmbeddedModule(module.Name)
	} else {
		r.manager.RemoveModule(module.Name)
	}

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

// relink marks mpv as used by the module, releasing the version it switched away from.
// Releasing first keeps the flags correct across a version bump.
func (r *reconciler) relink(ctx context.Context, module *v1alpha2.Module, mpv *v1alpha1.ModulePackageVersion) error {
	if old := ctrlutils.OwnerRefName(module, v1alpha1.ModulePackageVersionKind); old != "" && old != mpv.Name {
		if err := r.detachVersion(ctx, old); err != nil {
			return err
		}
	}

	return r.attachVersion(ctx, mpv)
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
