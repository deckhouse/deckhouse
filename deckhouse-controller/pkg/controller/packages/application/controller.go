// Copyright 2025 Flant JSC
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

package application

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/apps"
	packageruntime "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime"
	packagestatus "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/ctrlutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/status"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// controllerName is the name the controller is registered under in the manager.
	controllerName = "d8-application-controller"

	// maxConcurrentReconciles keeps a single worker: applications sharing a package write
	// the same ApplicationPackage and ApplicationPackageVersion installed lists.
	maxConcurrentReconciles = 1

	// defaultRequeueAfter is the retry delay for states that need an external change to
	// make progress, such as a missing package or a version still in draft.
	defaultRequeueAfter = 30 * time.Second
)

// RegisterController registers the Application controller with the manager.
func RegisterController(
	runtime ctrlmanager.Manager,
	manager packageManager,
	moduleManager moduleManager,
	logger *log.Logger,
) error {
	r := &reconciler{
		init:          new(sync.WaitGroup),
		client:        runtime.GetClient(),
		manager:       manager,
		moduleManager: moduleManager,
		logger:        logger.Named(controllerName),
	}

	r.init.Add(1)

	// add preflight to set the cluster UUID
	if err := runtime.Add(ctrlmanager.RunnableFunc(r.preflight)); err != nil {
		return fmt.Errorf("add preflight: %w", err)
	}

	r.status = status.NewService(r.client, r.manager.GetStatus, r.logger)
	r.status.Start(context.Background(), r.manager.GetAppStatusQueue())

	return ctrl.NewControllerManagedBy(runtime).
		Named(controllerName).
		For(&v1alpha1.Application{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})).
		WithOptions(controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}).
		Complete(r)
}

// reconciler reconciles Application objects.
type reconciler struct {
	init          *sync.WaitGroup
	client        client.Client
	manager       packageManager
	status        *status.Service
	moduleManager moduleManager
	logger        *log.Logger
}

// moduleManager reports whether addon-operator has finished initialising modules.
type moduleManager interface {
	AreModulesInited() bool
}

// packageManager registers and unregisters applications in the package runtime.
type packageManager interface {
	UpdateApp(repo registry.Remote, inst packageruntime.App)
	RemoveApp(namespace, name string)
	GetStatus(name string) packagestatus.Status
	GetAppStatusQueue() workqueue.TypedRateLimitingInterface[string]
	Cleanup(ctx context.Context, preserve []packageruntime.PreservePackage)
}

// preflight waits for the module manager and drops runtime state no Application claims.
func (r *reconciler) preflight(ctx context.Context) error {
	defer r.init.Done()

	// wait until module manager init
	r.logger.Debug("wait until module manager is inited")
	if err := wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(_ context.Context) (bool, error) {
		return r.moduleManager.AreModulesInited(), nil
	}); err != nil {
		return fmt.Errorf("init module manager: %w", err)
	}

	appsList := new(v1alpha1.ApplicationList)
	if err := r.client.List(ctx, appsList); err != nil {
		return fmt.Errorf("list applications: %w", err)
	}

	preserve := make([]packageruntime.PreservePackage, 0, len(appsList.Items))
	for _, app := range appsList.Items {
		preserve = append(preserve, packageruntime.PreservePackage{
			PackageName: app.Spec.PackageName,
			Repository:  app.Spec.PackageRepositoryName,
			Version:     app.Spec.PackageVersion,

			ReleaseName:      apps.BuildName(app.Namespace, app.Name),
			ReleaseNamespace: app.Namespace,
		})
	}

	r.manager.Cleanup(ctx, preserve)

	r.logger.Debug("controller is ready")

	return nil
}

// Reconcile dispatches the application to the delete or the create/update handler.
func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// wait for init
	r.init.Wait()

	logger := r.logger.With(slog.String("namespace", req.Namespace), slog.String("name", req.Name))

	logger.Info("reconcile application")

	app := new(v1alpha1.Application)
	if err := r.client.Get(ctx, req.NamespacedName, app); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Debug("application not found")
			return ctrl.Result{}, nil
		}

		logger.Warn("failed to get application", log.Err(err))

		return ctrl.Result{}, err
	}

	// handle delete event
	if !app.DeletionTimestamp.IsZero() {
		if err := r.handleDelete(ctx, app); err != nil {
			return ctrl.Result{}, fmt.Errorf("delete: %w", err)
		}

		return ctrl.Result{}, nil
	}

	// handle create/update events
	if err := r.handleCreateOrUpdate(ctx, app); err != nil {
		logger.Warn("failed to handle application", log.Err(err))

		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	return ctrl.Result{}, nil
}

// handleCreateOrUpdate validates the application's package and version, moves the
// application onto them and hands it to the package runtime.
func (r *reconciler) handleCreateOrUpdate(ctx context.Context, app *v1alpha1.Application) error {
	logger := r.logger.With(slog.String("name", app.Name), slog.String("namespace", app.Namespace))

	logger.Debug("handle application")
	defer logger.Debug("handle application complete")

	original := app.DeepCopy()

	// The finalizer is claimed before the application reaches the runtime: it is what
	// guarantees handleDelete gets to call RemoveApp and release the installed lists.
	if !controllerutil.ContainsFinalizer(app, v1alpha1.ApplicationFinalizerStatisticRegistered) {
		patch := client.MergeFrom(app.DeepCopy())
		controllerutil.AddFinalizer(app, v1alpha1.ApplicationFinalizerStatisticRegistered)

		if err := r.client.Patch(ctx, app, patch); err != nil {
			logger.Error("failed to add the application finalizer", log.Err(err))
			return fmt.Errorf("patch application '%s': %w", app.Name, err)
		}

		original = app.DeepCopy()
	}

	pkg := new(v1alpha1.ApplicationPackage)
	if err := r.client.Get(ctx, client.ObjectKey{Name: app.Spec.PackageName}, pkg); err != nil {
		logger.Debug("application package not found", slog.String("package", app.Spec.PackageName), log.Err(err))

		return fmt.Errorf("get application package '%s': %w", app.Spec.PackageName, err)
	}

	apvName := v1alpha1.MakeApplicationPackageVersionName(app.Spec.PackageRepositoryName, app.Spec.PackageName, app.Spec.PackageVersion)

	apv := new(v1alpha1.ApplicationPackageVersion)
	if err := r.client.Get(ctx, client.ObjectKey{Name: apvName}, apv); err != nil {
		logger.Debug("application package version not found", slog.String("apv", apvName), log.Err(err))

		return fmt.Errorf("get application package version '%s': %w", apvName, err)
	}

	// a draft version is not published, so it must never reach the runtime
	if apv.IsDraft() {
		logger.Debug("application package version is in draft", slog.String("apv", apvName))

		return fmt.Errorf("application package version '%s' is draft", apvName)
	}

	// The repository is read before any installed list is touched, so a missing
	// repository cannot leave the lists half-updated.
	repo := new(v1alpha1.PackageRepository)
	if err := r.client.Get(ctx, client.ObjectKey{Name: app.Spec.PackageRepositoryName}, repo); err != nil {
		logger.Error("get package repository", log.Err(err))
		return fmt.Errorf("get package repository '%s': %w", app.Spec.PackageRepositoryName, err)
	}

	if err := r.relink(ctx, app, pkg, apv); err != nil {
		logger.Error("failed to relink the application", log.Err(err))
		return err
	}

	r.manager.UpdateApp(registry.BuildRemote(repo), packageruntime.App{
		Name:      app.Name,
		Namespace: app.Namespace,
		Definition: apps.Definition{
			Name:    app.Spec.PackageName,
			Version: app.Spec.PackageVersion,
		},
		Settings:    app.Spec.Settings.GetMap(),
		Maintenance: app.Spec.Maintenance,
	})

	// Both references are non-controller and block owner deletion, so neither the package
	// nor the version can disappear from under a running application.
	ctrlutils.ReplaceOwnerReferences(app,
		ctrlutils.OwnerReference(v1alpha1.ApplicationPackageVersionGVK, apv.Name, apv.UID),
		ctrlutils.OwnerReference(v1alpha1.ApplicationPackageGVK, pkg.Name, pkg.UID),
	)
	delete(app.Annotations, v1alpha1.ApplicationAnnotationRegistrySpecChanged)

	if err := r.client.Patch(ctx, app, client.MergeFrom(original)); err != nil {
		logger.Error("failed to patch application", log.Err(err))
		return fmt.Errorf("patch application '%s': %w", app.Name, err)
	}

	return nil
}

// handleDelete detaches the application from its package and version, unregisters it
// from the package runtime and releases the finalizer.
func (r *reconciler) handleDelete(ctx context.Context, app *v1alpha1.Application) error {
	logger := r.logger.With(slog.String("name", app.Name), slog.String("namespace", app.Namespace))

	logger.Debug("handle delete application")
	defer logger.Debug("handle delete application complete")

	// Detach by owner reference, not by spec: the references name what the application
	// was actually attached to, which a spec edit just before deletion would have changed.
	if name := ctrlutils.OwnerRefName(app, v1alpha1.ApplicationPackageVersionKind); name != "" {
		if err := r.detachVersion(ctx, app, name); err != nil {
			logger.Error("failed to detach the application package version", slog.String("apv", name), log.Err(err))
			return err
		}
	}

	if name := ctrlutils.OwnerRefName(app, v1alpha1.ApplicationPackageKind); name != "" {
		if err := r.detachPackage(ctx, app, name); err != nil {
			logger.Error("failed to detach the application package", slog.String("package", name), log.Err(err))
			return err
		}
	}

	// call PackageOperator method (PackageRemover interface)
	r.manager.RemoveApp(app.Namespace, app.Name)

	if !controllerutil.ContainsFinalizer(app, v1alpha1.ApplicationFinalizerStatisticRegistered) {
		return nil
	}

	patch := client.MergeFrom(app.DeepCopy())
	controllerutil.RemoveFinalizer(app, v1alpha1.ApplicationFinalizerStatisticRegistered)

	if err := r.client.Patch(ctx, app, patch); err != nil {
		logger.Error("failed to remove the application finalizer", log.Err(err))
		return fmt.Errorf("patch application %s: %w", app.Name, err)
	}

	return nil
}

// relink moves the application onto ap and apv, releasing the ones it switched away
// from. Detaching first keeps the installed counts correct when only one of the two
// changed, which is the common case of a version bump within the same package.
func (r *reconciler) relink(ctx context.Context, app *v1alpha1.Application, pkg *v1alpha1.ApplicationPackage, apv *v1alpha1.ApplicationPackageVersion) error {
	if old := ctrlutils.OwnerRefName(app, v1alpha1.ApplicationPackageVersionKind); old != "" && old != apv.Name {
		if err := r.detachVersion(ctx, app, old); err != nil {
			return err
		}
	}

	if old := ctrlutils.OwnerRefName(app, v1alpha1.ApplicationPackageKind); old != "" && old != pkg.Name {
		if err := r.detachPackage(ctx, app, old); err != nil {
			return err
		}
	}

	if err := r.attachVersion(ctx, app, apv); err != nil {
		return err
	}

	return r.attachPackage(ctx, app, pkg)
}

// attachVersion adds the application to the version's installed list.
func (r *reconciler) attachVersion(ctx context.Context, app *v1alpha1.Application, apv *v1alpha1.ApplicationPackageVersion) error {
	if apv.IsAppInstalled(app.Namespace, app.Name) {
		return nil
	}

	patch := client.MergeFrom(apv.DeepCopy())
	apv = apv.AddInstalledApp(app.Namespace, app.Name)

	if err := r.client.Status().Patch(ctx, apv, patch); err != nil {
		return fmt.Errorf("patch application package version status '%s': %w", apv.Name, err)
	}

	return nil
}

// attachPackage adds the application to the package's installed list, or refreshes the
// version recorded there when the application moved to another version of the same package.
func (r *reconciler) attachPackage(ctx context.Context, app *v1alpha1.Application, pkg *v1alpha1.ApplicationPackage) error {
	installed := pkg.IsAppInstalled(app.Namespace, app.Name)
	if installed && pkg.GetAppVersion(app.Namespace, app.Name) == app.Spec.PackageVersion {
		return nil
	}

	patch := client.MergeFrom(pkg.DeepCopy())

	if installed {
		pkg.UpdateAppVersion(app.Namespace, app.Name, app.Spec.PackageVersion)
	} else {
		pkg = pkg.AddInstalledApp(app.Namespace, app.Name, app.Spec.PackageVersion)
	}

	if err := r.client.Status().Patch(ctx, pkg, patch); err != nil {
		return fmt.Errorf("patch application package status '%s': %w", pkg.Name, err)
	}

	return nil
}

// detachVersion removes the application from the named version's installed list. A
// version that is already gone needs no cleanup.
func (r *reconciler) detachVersion(ctx context.Context, app *v1alpha1.Application, name string) error {
	apv := new(v1alpha1.ApplicationPackageVersion)
	if err := r.client.Get(ctx, client.ObjectKey{Name: name}, apv); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("get application package version '%s': %w", name, err)
	}

	if !apv.IsAppInstalled(app.Namespace, app.Name) {
		return nil
	}

	patch := client.MergeFrom(apv.DeepCopy())
	apv = apv.RemoveInstalledApp(app.Namespace, app.Name)

	if err := r.client.Status().Patch(ctx, apv, patch); err != nil {
		return fmt.Errorf("patch application package version status '%s': %w", name, err)
	}

	return nil
}

// detachPackage removes the application from the named package's installed list. A
// package that is already gone needs no cleanup.
func (r *reconciler) detachPackage(ctx context.Context, app *v1alpha1.Application, name string) error {
	ap := new(v1alpha1.ApplicationPackage)
	if err := r.client.Get(ctx, client.ObjectKey{Name: name}, ap); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("get application package '%s': %w", name, err)
	}

	if !ap.IsAppInstalled(app.Namespace, app.Name) {
		return nil
	}

	patch := client.MergeFrom(ap.DeepCopy())
	ap = ap.RemoveInstalledApp(app.Namespace, app.Name)

	if err := r.client.Status().Patch(ctx, ap, patch); err != nil {
		return fmt.Errorf("patch application package status '%s': %w", name, err)
	}

	return nil
}
