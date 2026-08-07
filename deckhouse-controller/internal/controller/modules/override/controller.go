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
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// controllerName is the name the controller is registered under in the manager.
	controllerName = "d8-module-override-controller"

	// maxConcurrentReconciles is safe above one: an override only ever writes the module of
	// the same name, and override names are unique cluster-wide.
	maxConcurrentReconciles = 3

	// cacheSyncTimeout bounds the initial informer cache sync.
	cacheSyncTimeout = 3 * time.Minute

	// defaultRequeueAfter is the retry delay for an override waiting on an object another
	// controller has to create first.
	defaultRequeueAfter = time.Minute

	// defaultScanInterval keeps a pinned tag rescanned when the override carries no
	// interval of its own, which is what a repush under that tag relies on.
	defaultScanInterval = 15 * time.Second
)

var (
	// errUnknownModule reports that the override names a module the cluster does not know.
	errUnknownModule = errors.New("unknown module")

	// errNoRepository reports that the repository serving the overridden module cannot be
	// resolved without guessing.
	errNoRepository = errors.New("no repository")
)

// RegisterController registers the ModulePullOverride controller with the manager.
func RegisterController(synced *sync.WaitGroup, runtime ctrlmanager.Manager, dc dependency.Container, logger *log.Logger) error {
	r := &reconciler{
		init:     synced,
		client:   runtime.GetClient(),
		registry: registry.NewService(dc, logger),
		dc:       dc,
		logger:   logger.Named(controllerName),
	}

	if err := ctrl.NewControllerManagedBy(runtime).
		Named(controllerName).
		For(&v1alpha2.ModulePullOverride{}).
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

// reconciler pins the module named by a ModulePullOverride to the override's image tag.
type reconciler struct {
	init     *sync.WaitGroup
	client   client.Client
	registry digestResolver
	dc       dependency.Container
	logger   *log.Logger
}

// digestResolver reports the digest an image tag currently points at.
type digestResolver interface {
	GetImageDigest(ctx context.Context, remote registry.Remote, packageName, tag string) (string, error)
}

// Reconcile dispatches the override to the create/update handler.
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

		return ctrl.Result{}, fmt.Errorf("get module pull override '%s': %w", req.Name, err)
	}

	// handle delete event
	if !mpo.DeletionTimestamp.IsZero() {
		if err := r.handleDelete(ctx, mpo); err != nil {
			return ctrl.Result{}, fmt.Errorf("delete: %w", err)
		}

		return ctrl.Result{}, nil
	}

	// handle create/update events
	return r.handleCreateOrUpdate(ctx, mpo)
}

// handleCreateOrUpdate moves the module onto the overridden tag whenever the image behind
// that tag differs from the one the override last applied.
func (r *reconciler) handleCreateOrUpdate(ctx context.Context, mpo *v1alpha2.ModulePullOverride) (ctrl.Result, error) {
	logger := r.logger.With(slog.String("name", mpo.Name))

	logger.Debug("handle module pull override")
	defer logger.Debug("handle module pull override complete")

	module := new(v1alpha2.Module)
	if err := r.client.Get(ctx, client.ObjectKey{Name: mpo.Name}, module); err != nil {
		if !apierrors.IsNotFound(err) {
			logger.Error("failed to get the module", log.Err(err))
			return ctrl.Result{}, fmt.Errorf("get module '%s': %w", mpo.Name, err)
		}

		module = nil
	}

	repository, err := r.resolveRepository(ctx, module, mpo.Name)
	if err != nil {
		switch {
		case errors.Is(err, errUnknownModule):
			logger.Warn("module not found")
			return r.park(ctx, mpo, v1alpha1.ModulePullOverrideMessageModuleNotFound)

		case errors.Is(err, errNoRepository):
			logger.Warn("cannot resolve the repository of the module")
			return r.park(ctx, mpo, v1alpha1.ModulePullOverrideMessageNoSource)
		}

		logger.Error("failed to resolve the repository of the module", log.Err(err))

		return ctrl.Result{}, err
	}

	repo := new(v1alpha1.PackageRepository)
	if err = r.client.Get(ctx, client.ObjectKey{Name: repository}, repo); err != nil {
		if !apierrors.IsNotFound(err) {
			logger.Error("failed to get the package repository", slog.String("repository", repository), log.Err(err))
			return ctrl.Result{}, fmt.Errorf("get package repository '%s': %w", repository, err)
		}

		logger.Warn("package repository not found", slog.String("repository", repository))

		return r.park(ctx, mpo, v1alpha1.ModulePullOverrideMessageSourceNotFound)
	}

	digest, err := r.registry.GetImageDigest(ctx, registry.BuildRemote(repo), mpo.Name, mpo.Spec.ImageTag)
	if err != nil {
		logger.Error("failed to resolve the image digest", slog.String("tag", mpo.Spec.ImageTag), log.Err(err))

		if serr := r.setStatus(ctx, mpo, fmt.Sprintf("Download error: %v", err), mpo.Status.ImageDigest); serr != nil {
			return ctrl.Result{}, serr
		}

		return ctrl.Result{RequeueAfter: scanInterval(mpo)}, nil
	}

	// status.imageDigest is the digest the override applied, not the one the registry last
	// reported, so an unchanged image leaves the module alone however often it is rescanned.
	if module != nil && module.Spec.PackageVersion == mpo.Spec.ImageTag &&
		module.Spec.PackageRepositoryName == repository && mpo.Status.ImageDigest == digest {
		logger.Debug("module is up to date", slog.String("digest", digest))

		if err = r.setStatus(ctx, mpo, v1alpha1.ModulePullOverrideMessageReady, digest); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{RequeueAfter: scanInterval(mpo)}, nil
	}

	if err = r.ensureModule(ctx, module, mpo, repository, digest); err != nil {
		logger.Error("failed to ensure the module", log.Err(err))
		return ctrl.Result{}, err
	}

	// Recording the digest after the module carries it is what makes a write that failed
	// halfway retry instead of counting as an applied override.
	if err = r.setStatus(ctx, mpo, v1alpha1.ModulePullOverrideMessageReady, digest); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("module pinned to the overridden tag",
		slog.String("tag", mpo.Spec.ImageTag), slog.String("digest", digest))

	return ctrl.Result{RequeueAfter: scanInterval(mpo)}, nil
}

// handleDelete releases the finalizer the pre-package override controller left behind. The
// module keeps the pinned tag: nothing here restores the released version.
func (r *reconciler) handleDelete(ctx context.Context, mpo *v1alpha2.ModulePullOverride) error {
	if !controllerutil.ContainsFinalizer(mpo, v1alpha1.ModulePullOverrideFinalizer) {
		return nil
	}

	r.logger.Debug("release the module pull override", slog.String("name", mpo.Name))

	patch := client.MergeFrom(mpo.DeepCopy())
	controllerutil.RemoveFinalizer(mpo, v1alpha1.ModulePullOverrideFinalizer)

	if err := r.client.Patch(ctx, mpo, patch); err != nil {
		r.logger.Error("failed to remove the module pull override finalizer", slog.String("name", mpo.Name), log.Err(err))
		return fmt.Errorf("patch module pull override '%s': %w", mpo.Name, err)
	}

	return nil
}

// resolveRepository names the repository serving the overridden module: the one the module
// already sits on, or, for a module that does not exist yet, the only repository the package
// is available in. An ambiguous choice is left to the user rather than guessed.
func (r *reconciler) resolveRepository(ctx context.Context, module *v1alpha2.Module, name string) (string, error) {
	if module != nil && module.Spec.PackageRepositoryName != "" {
		return module.Spec.PackageRepositoryName, nil
	}

	pkg := new(v1alpha1.ModulePackage)
	if err := r.client.Get(ctx, client.ObjectKey{Name: name}, pkg); err != nil {
		if !apierrors.IsNotFound(err) {
			return "", fmt.Errorf("get module package '%s': %w", name, err)
		}

		return "", errUnknownModule
	}

	if len(pkg.Status.AvailableRepositories) != 1 {
		return "", errNoRepository
	}

	return pkg.Status.AvailableRepositories[0], nil
}

// ensureModule places the module on the overridden tag and stamps the force annotation with
// the digest behind it, which is the only change a repush under an unchanged tag produces.
func (r *reconciler) ensureModule(ctx context.Context, module *v1alpha2.Module, mpo *v1alpha2.ModulePullOverride, repository, digest string) error {
	if module == nil {
		module = &v1alpha2.Module{
			ObjectMeta: metav1.ObjectMeta{
				Name:        mpo.Name,
				Annotations: map[string]string{v1alpha2.ModuleAnnotationForce: digest},
			},
			Spec: v1alpha2.ModuleSpec{
				PackageRepositoryName: repository,
				PackageVersion:        mpo.Spec.ImageTag,
			},
		}

		// AlreadyExists means only that the informer cache had not caught up; the next
		// scan patches the module this one missed.
		if err := r.client.Create(ctx, module); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("create module '%s': %w", mpo.Name, err)
		}

		return nil
	}

	patch := client.MergeFrom(module.DeepCopy())

	if module.Annotations == nil {
		module.Annotations = make(map[string]string)
	}

	module.Annotations[v1alpha2.ModuleAnnotationForce] = digest
	module.Spec.PackageRepositoryName = repository
	module.Spec.PackageVersion = mpo.Spec.ImageTag

	if err := r.client.Patch(ctx, module, patch); err != nil {
		return fmt.Errorf("patch module '%s': %w", mpo.Name, err)
	}

	return nil
}

// park records why the override cannot proceed and retries later, keeping the applied digest
// so the override still redeploys only on a real image change once it can proceed.
func (r *reconciler) park(ctx context.Context, mpo *v1alpha2.ModulePullOverride, message string) (ctrl.Result, error) {
	if err := r.setStatus(ctx, mpo, message, mpo.Status.ImageDigest); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
}

// setStatus records the override's outcome, leaving the object untouched when nothing moved.
func (r *reconciler) setStatus(ctx context.Context, mpo *v1alpha2.ModulePullOverride, message, digest string) error {
	if mpo.Status.Message == message && mpo.Status.ImageDigest == digest {
		return nil
	}

	patch := client.MergeFrom(mpo.DeepCopy())
	mpo.Status.Message = message
	mpo.Status.ImageDigest = digest
	mpo.Status.UpdatedAt = metav1.NewTime(r.dc.GetClock().Now().UTC())

	if err := r.client.Status().Patch(ctx, mpo, patch); err != nil {
		r.logger.Error("failed to patch the module pull override status", slog.String("name", mpo.Name), log.Err(err))
		return fmt.Errorf("patch module pull override status '%s': %w", mpo.Name, err)
	}

	return nil
}

// scanInterval is how long to wait before rescanning the pinned tag.
func scanInterval(mpo *v1alpha2.ModulePullOverride) time.Duration {
	if mpo.Spec.ScanInterval.Duration <= 0 {
		return defaultScanInterval
	}

	return mpo.Spec.ScanInterval.Duration
}
