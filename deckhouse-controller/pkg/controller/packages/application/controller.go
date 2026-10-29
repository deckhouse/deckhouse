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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/disk"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/observed"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/resourcesize"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/ctrlutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/status"
	appupdate "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/update"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// controllerName is the name the controller is registered under in the manager.
	controllerName = "d8-application-controller"

	// maxConcurrentReconciles keeps a single worker: applications sharing a package write
	// the same ApplicationPackage and ApplicationPackageVersion installed lists.
	maxConcurrentReconciles = 1

	// removalRequeueAfter is how often a running teardown is re-checked. Its end reaches only the
	// status queue, not this controller, so the only way to notice is to ask the engine again.
	removalRequeueAfter = 5 * time.Second
)

// RegisterController registers the Application controller with the manager.
func RegisterController(
	synced *sync.WaitGroup,
	runtime ctrlmanager.Manager,
	manager packageManager,
	moduleManager moduleManager,
	deckhouseSettings deckhouseSettings,
	applicationsSettings applicationsSettings,
	logger *log.Logger,
) error {
	r := &reconciler{
		init:                 synced,
		client:               runtime.GetClient(),
		manager:              manager,
		moduleManager:        moduleManager,
		deckhouseSettings:    deckhouseSettings,
		applicationsSettings: applicationsSettings,
		status:               status.NewService(runtime.GetClient(), manager, logger),
		updater:              appupdate.NewUpdater(runtime.GetClient()),
		logger:               logger.Named(controllerName),
	}

	if !app.ModuleV2Enabled() {
		r.init.Add(1)
		if err := runtime.Add(ctrlmanager.RunnableFunc(r.preflight)); err != nil {
			return fmt.Errorf("add preflight: %w", err)
		}
	}

	if err := runtime.Add(ctrlmanager.RunnableFunc(func(ctx context.Context) error {
		r.status.Start(ctx, r.manager.GetAppStatusQueue())
		return nil
	})); err != nil {
		return fmt.Errorf("add status service: %w", err)
	}

	return ctrl.NewControllerManagedBy(runtime).
		Named(controllerName).
		// The automatic update steps on the installed version, UpdateInstalled and Ready, which bump no generation.
		For(&v1alpha1.Application{}, builder.WithPredicates(
			predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{}, updateInputChanged()))).
		// The update settings of an application fall back to global.applications.update and
		// then to the deckhouse ones, so a change of either is merged into every application again.
		WatchesRawSource(r.requeueOnSettingsChange(applicationsSettings.SubscribeUpdate(), deckhouseSettings.SubscribeUpdate())).
		// A new version of the package, or a change of its rules, moves the update plan; a global event filter
		// would drop these, since status and labels bump no generation.
		Watches(&v1alpha1.ApplicationPackageVersion{}, handler.EnqueueRequestsFromMapFunc(r.requestsForVersion),
			builder.WithPredicates(planInputChanged())).
		WithOptions(controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}).
		Complete(r)
}

// reconciler reconciles Application objects.
type reconciler struct {
	init                 *sync.WaitGroup
	client               client.Client
	manager              packageManager
	status               *status.Service
	updater              *appupdate.Updater
	moduleManager        moduleManager
	deckhouseSettings    deckhouseSettings
	applicationsSettings applicationsSettings
	logger               *log.Logger
}

// moduleManager reports whether addon-operator has finished initialising modules.
type moduleManager interface {
	AreModulesInited() bool
}

// packageManager hands applications to the package engine (engine.Engine).
type packageManager interface {
	ApplyApplication(app engine.Application)
	// RemoveApplication removes the application and reports whether the removal has finished.
	RemoveApplication(namespace, name string) bool
	Report(name string) (model.Report, bool)
	Observed(name string) observed.Observation
	GetAppStatusQueue() workqueue.TypedRateLimitingInterface[string]
	Cleanup(ctx context.Context, apps, modules []disk.PreservePackage) error
	Boot(ctx context.Context, global []byte, bootApps []engine.BootApplication, bootModules []engine.BootModule) error
}

// preflight waits for the module manager, then cleans the disk up and restores every installed
// application in the engine (without EnableModuleV2; with it the controller tree boots). A Boot
// error stops the manager: deckhouse must restart.
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

	preserve := make([]disk.PreservePackage, 0, 2*len(appsList.Items))
	boot := make([]engine.BootApplication, 0, len(appsList.Items))

	for i := range appsList.Items {
		app := &appsList.Items[i]

		// One being deleted is booted to be uninstalled as at runtime; one Restorable rejects gets
		// its release deleted by name on Remove.
		if !Restorable(app) {
			continue
		}

		preserve = append(preserve, PreserveOf(app)...)

		bootApp, err := BootOf(ctx, r.client, app)
		if err != nil {
			return fmt.Errorf("boot application '%s/%s': %w", app.Namespace, app.Name, err)
		}

		boot = append(boot, bootApp)
	}

	// A leak must not stop the boot.
	if err := r.manager.Cleanup(ctx, preserve, nil); err != nil {
		r.logger.Warn("failed to cleanup application packages", log.Err(err))
	}

	if err := r.manager.Boot(ctx, nil, boot, nil); err != nil {
		return fmt.Errorf("boot applications: %w", err)
	}

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
		res, err := r.handleDelete(ctx, app)
		if err != nil {
			logger.Warn("failed to delete application", log.Err(err))
			return ctrl.Result{}, fmt.Errorf("delete: %w", err)
		}

		return res, nil
	}

	// handle create/update events
	if err := r.handleCreateOrUpdate(ctx, app); err != nil {
		logger.Warn("failed to handle application", log.Err(err))
		return ctrl.Result{}, fmt.Errorf("handle: %w", err)
	}

	return ctrl.Result{}, nil
}

// handleCreateOrUpdate validates the application's package and version, moves the
// application onto them and hands it to the package engine.
func (r *reconciler) handleCreateOrUpdate(ctx context.Context, app *v1alpha1.Application) error {
	logger := r.logger.With(slog.String("name", app.Name), slog.String("namespace", app.Namespace))

	logger.Debug("handle application")
	defer logger.Debug("handle application complete")

	// The finalizer is claimed before the application reaches the engine: it is what
	// guarantees handleDelete gets to call RemoveApplication and release the installed lists.
	if !controllerutil.ContainsFinalizer(app, v1alpha1.ApplicationFinalizerStatisticRegistered) {
		patch := client.MergeFrom(app.DeepCopy())
		controllerutil.AddFinalizer(app, v1alpha1.ApplicationFinalizerStatisticRegistered)

		if err := r.client.Patch(ctx, app, patch); err != nil {
			logger.Error("failed to add the application finalizer", log.Err(err))
			return fmt.Errorf("patch application '%s': %w", app.Name, err)
		}
	}

	// status.update goes before the package: the version resolved below is the one it moves to.
	if err := r.updater.Update(ctx, app); err != nil {
		logger.Error("failed to update the automatic update status", log.Err(err))
		return fmt.Errorf("update status of application '%s': %w", app.Name, err)
	}

	original := app.DeepCopy()

	ap, apv, err := r.resolvePackage(ctx, app)
	if err != nil {
		logger.Error("failed to resolve the package", log.Err(err))
		return err
	}

	// The repository and the requirements are read before any installed list is touched, so a
	// missing repository or unparsable requirements cannot leave the lists half-updated.
	repo := new(v1alpha1.PackageRepository)
	if err := r.client.Get(ctx, client.ObjectKey{Name: app.Spec.PackageRepositoryName}, repo); err != nil {
		logger.Error("get package repository", log.Err(err))
		return fmt.Errorf("get package repository '%s': %w", app.Spec.PackageRepositoryName, err)
	}

	requirements, err := requirementsOf(apv)
	if err != nil {
		logger.Error("parse the application package version requirements", log.Err(err))
		return fmt.Errorf("parse requirements of application package version '%s': %w", apv.Name, err)
	}

	if err := r.relink(ctx, app, ap, apv); err != nil {
		logger.Error("failed to relink the application", log.Err(err))
		return err
	}

	// The update windows hold the switch to a new version, also of a version the user set:
	// spec.update, else the global settings, decide when it happens, unless the apply-now
	// annotation names the version the engine runs.
	r.manager.ApplyApplication(engine.Application{
		Namespace:        app.Namespace,
		Name:             app.Name,
		PackageName:      app.Spec.PackageName,
		Version:          apv.Spec.PackageVersion,
		Repository:       registry.BuildRemote(repo),
		Settings:         app.Spec.Settings.GetMap(),
		SettingsVersion:  app.Spec.SettingsVersion,
		Maintenance:      app.Spec.Maintenance,
		ResourceRequests: toResourceRequests(app.Spec.ResourceRequests),
		UpdateSettings:   r.updateSettings(app),
		ApplyNow:         applyNow(app),
		Requirements:     requirements,
	})

	return r.commit(ctx, app, original, ap, apv)
}

// resolvePackage fetches the package and the version the engine runs (Application.TargetVersion), refusing a draft version.
func (r *reconciler) resolvePackage(ctx context.Context, app *v1alpha1.Application) (*v1alpha1.ApplicationPackage, *v1alpha1.ApplicationPackageVersion, error) {
	logger := r.logger.With(slog.String("name", app.Name), slog.String("namespace", app.Namespace))

	ap := new(v1alpha1.ApplicationPackage)
	if err := r.client.Get(ctx, client.ObjectKey{Name: app.Spec.PackageName}, ap); err != nil {
		logger.Debug("application package not found", slog.String("package", app.Spec.PackageName), log.Err(err))

		return nil, nil, fmt.Errorf("get application package '%s': %w", app.Spec.PackageName, err)
	}

	apvName := v1alpha1.MakeApplicationPackageVersionName(app.Spec.PackageRepositoryName, app.Spec.PackageName, app.TargetVersion())

	apv := new(v1alpha1.ApplicationPackageVersion)
	if err := r.client.Get(ctx, client.ObjectKey{Name: apvName}, apv); err != nil {
		logger.Debug("application package version not found", slog.String("apv", apvName), log.Err(err))
		return nil, nil, fmt.Errorf("get application package version '%s': %w", apvName, err)
	}

	// a draft version is not published, so it must never reach the engine
	if apv.IsDraft() {
		logger.Debug("application package version is in draft", slog.String("apv", apvName))
		return nil, nil, fmt.Errorf("application package version '%s' is draft", apvName)
	}

	return ap, apv, nil
}

// requirementsOf parses the requirements the version publishes; none without metadata.
func requirementsOf(apv *v1alpha1.ApplicationPackageVersion) (schedule.Constraints, error) {
	var reqs *v1alpha1.PackageRequirements
	if meta := apv.Status.PackageMetadata; meta != nil {
		reqs = meta.Requirements
	}

	return reqs.ToScheduleConstraints()
}

// commit points the owner references at ap and apv, clears the registry-spec-changed
// annotation and patches the application against original.
func (r *reconciler) commit(ctx context.Context, app, original *v1alpha1.Application, ap *v1alpha1.ApplicationPackage, apv *v1alpha1.ApplicationPackageVersion) error {
	// Both references are non-controller and block owner deletion, so neither the package
	// nor the version can disappear from under a running application.
	ctrlutils.ReplaceOwnerReferences(app,
		ctrlutils.OwnerReference(v1alpha1.ApplicationPackageVersionGVK, apv.Name, apv.UID),
		ctrlutils.OwnerReference(v1alpha1.ApplicationPackageGVK, ap.Name, ap.UID),
	)
	delete(app.Annotations, v1alpha1.PackageAnnotationRegistrySpecChanged)

	if err := r.client.Patch(ctx, app, client.MergeFrom(original)); err != nil {
		r.logger.Error("failed to patch application", slog.String("name", app.Name), log.Err(err))

		return fmt.Errorf("patch application '%s': %w", app.Name, err)
	}

	return nil
}

// handleDelete removes the application from the package engine and, once the engine reports the
// removal finished, detaches the application from its package and version and releases the
// finalizer. The wait is unbounded on purpose: holding the finalizer is what keeps a Helm release
// from outliving the CR that owns it, so a removal the engine keeps retrying keeps the application
// in Terminating rather than orphaning its resources.
func (r *reconciler) handleDelete(ctx context.Context, app *v1alpha1.Application) (ctrl.Result, error) {
	logger := r.logger.With(slog.String("name", app.Name), slog.String("namespace", app.Namespace))

	logger.Debug("handle delete application")
	defer logger.Debug("handle delete application complete")

	// The engine removes the application asynchronously — the release, the orphans beside it, the
	// files — so the removal is polled until it reports done.
	if !r.manager.RemoveApplication(app.Namespace, app.Name) {
		logger.Info("application is still being removed by the engine")

		return ctrl.Result{RequeueAfter: removalRequeueAfter}, nil
	}

	// Detach only after the teardown: releasing the version any earlier lets it be garbage
	// collected — and its package files removed from disk — while the uninstall still needs them.
	//
	// Detach by owner reference, falling back to the spec: the reference names what the
	// application was actually attached to, which a spec edit just before deletion would have
	// changed, but it is written after the attach, so an attached application can lack it.
	apvName := ctrlutils.OwnerRefName(app, v1alpha1.ApplicationPackageVersionKind)
	if apvName == "" {
		apvName = v1alpha1.MakeApplicationPackageVersionName(app.Spec.PackageRepositoryName, app.Spec.PackageName, app.TargetVersion())
	}

	if err := r.detachVersion(ctx, app, apvName); err != nil {
		logger.Error("failed to detach the application package version", slog.String("apv", apvName), log.Err(err))
		return ctrl.Result{}, err
	}

	pkgName := ctrlutils.OwnerRefName(app, v1alpha1.ApplicationPackageKind)
	if pkgName == "" {
		pkgName = app.Spec.PackageName
	}

	if err := r.detachPackage(ctx, app, pkgName); err != nil {
		logger.Error("failed to detach the application package", slog.String("package", pkgName), log.Err(err))
		return ctrl.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(app, v1alpha1.ApplicationFinalizerStatisticRegistered) {
		return ctrl.Result{}, nil
	}

	patch := client.MergeFrom(app.DeepCopy())
	controllerutil.RemoveFinalizer(app, v1alpha1.ApplicationFinalizerStatisticRegistered)

	if err := r.client.Patch(ctx, app, patch); err != nil {
		logger.Error("failed to remove the application finalizer", log.Err(err))
		return ctrl.Result{}, fmt.Errorf("patch application %s: %w", app.Name, err)
	}

	return ctrl.Result{}, nil
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

	return r.attachPackage(ctx, app, pkg, apv.Spec.PackageVersion)
}

// attachVersion adds the application to the version's installed list.
func (r *reconciler) attachVersion(ctx context.Context, app *v1alpha1.Application, apv *v1alpha1.ApplicationPackageVersion) error {
	patch := client.MergeFrom(apv.DeepCopy())

	if !apv.AddInstalledApp(app.Namespace, app.Name) {
		return nil
	}

	if err := r.client.Status().Patch(ctx, apv, patch); err != nil {
		return fmt.Errorf("patch application package version status '%s': %w", apv.Name, err)
	}

	return nil
}

// attachPackage adds the application to the package's installed list, or refreshes the
// version recorded there when the application moved to another version of the same package.
func (r *reconciler) attachPackage(ctx context.Context, app *v1alpha1.Application, pkg *v1alpha1.ApplicationPackage, version string) error {
	patch := client.MergeFrom(pkg.DeepCopy())

	if !pkg.AddInstalledApp(app.Namespace, app.Name, version, app.Spec.PackageRepositoryName) {
		return nil
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

	patch := client.MergeFrom(apv.DeepCopy())

	if !apv.RemoveInstalledApp(app.Namespace, app.Name) {
		return nil
	}

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

	patch := client.MergeFrom(ap.DeepCopy())

	if !ap.RemoveInstalledApp(app.Namespace, app.Name) {
		return nil
	}

	if err := r.client.Status().Patch(ctx, ap, patch); err != nil {
		return fmt.Errorf("patch application package status '%s': %w", name, err)
	}

	return nil
}

// PreserveOf is what Cleanup keeps of an installed application: the images of the version it asks
// for (Application.TargetVersion) and the one it runs (status), one entry each.
func PreserveOf(app *v1alpha1.Application) []disk.PreservePackage {
	preserve := []disk.PreservePackage{
		{Name: app.Spec.PackageName, Repository: app.Spec.PackageRepositoryName, Version: app.TargetVersion()},
	}

	if app.Status.CurrentVersion != nil {
		preserve = append(preserve, disk.PreservePackage{
			Name: app.Spec.PackageName, Repository: app.Spec.PackageRepositoryName, Version: app.Status.CurrentVersion.Version,
		})
	}

	return preserve
}

// Restorable reports whether Boot gets app and Cleanup keeps its image: always, except one being
// deleted with no activated version (nothing to uninstall) or force-removed (its release is
// deleted by name).
func Restorable(app *v1alpha1.Application) bool {
	if app.DeletionTimestamp.IsZero() {
		return true
	}

	return app.Status.CurrentVersion != nil && app.Status.CurrentVersion.Version != "" && !v1alpha1.IsForceRemoved(app)
}

// BootOf is what Boot restores of an installed application: its activated version with its
// settings checkpoint (status.lastAppliedConfiguration), from the repository in spec; one being
// deleted is restored to be uninstalled. An application with no activated version needs no
// repository: it restores nothing.
func BootOf(ctx context.Context, cli client.Reader, app *v1alpha1.Application) (engine.BootApplication, error) {
	boot := engine.BootApplication{
		Namespace:        app.Namespace,
		Name:             app.Name,
		PackageName:      app.Spec.PackageName,
		LastApplied:      app.Status.LastAppliedConfiguration.Raw,
		Maintenance:      app.Spec.Maintenance,
		ResourceRequests: toResourceRequests(app.Spec.ResourceRequests),
		Removing:         !app.DeletionTimestamp.IsZero(),
	}

	if app.Status.CurrentVersion == nil || app.Status.CurrentVersion.Version == "" {
		return boot, nil
	}

	boot.CurrentVersion = app.Status.CurrentVersion.Version

	repo := new(v1alpha1.PackageRepository)
	if err := cli.Get(ctx, client.ObjectKey{Name: app.Spec.PackageRepositoryName}, repo); err != nil {
		return engine.BootApplication{}, fmt.Errorf("get package repository '%s': %w", app.Spec.PackageRepositoryName, err)
	}

	boot.Repository = registry.BuildRemote(repo)

	return boot, nil
}

// toResourceRequests projects the CR's per-workload overrides onto the runtime
// shape. Quantities are carried as canonical strings: the CR schema has already
// validated them, and the overlay writes them straight into rendered manifests.
//
// Everything is copied rather than aliased, because the result outlives this
// reconcile in the runtime store, where it is compared against the next one.
func toResourceRequests(requests []v1alpha1.ApplicationResourceRequest) []resourcesize.Request {
	if len(requests) == 0 {
		return nil
	}

	out := make([]resourcesize.Request, 0, len(requests))

	for _, request := range requests {
		var containers []resourcesize.Container

		if len(request.Containers) > 0 {
			containers = make([]resourcesize.Container, 0, len(request.Containers))
			for _, container := range request.Containers {
				containers = append(containers, resourcesize.Container{
					Name:     container.Name,
					Requests: toQuantities(container.Resources.Requests),
					Limits:   toQuantities(container.Resources.Limits),
				})
			}
		}

		var claimTemplates []resourcesize.VolumeClaimTemplate

		if len(request.VolumeClaimTemplates) > 0 {
			claimTemplates = make([]resourcesize.VolumeClaimTemplate, 0, len(request.VolumeClaimTemplates))
			for _, template := range request.VolumeClaimTemplates {
				claimTemplates = append(claimTemplates, resourcesize.VolumeClaimTemplate{
					Name:    template.Name,
					Storage: template.Storage.String(),
				})
			}
		}

		var replicas *int32
		if request.Replicas != nil {
			value := *request.Replicas
			replicas = &value
		}

		out = append(out, resourcesize.Request{
			Kind:                 request.Kind,
			Name:                 request.Name,
			Replicas:             replicas,
			Containers:           containers,
			VolumeClaimTemplates: claimTemplates,
		})
	}

	return out
}

// toQuantities flattens a ResourceList into resource name to canonical quantity
// string, returning nil for an empty list so the comparison in the runtime store
// does not see an empty map and a missing one as different.
func toQuantities(list corev1.ResourceList) map[string]string {
	if len(list) == 0 {
		return nil
	}

	out := make(map[string]string, len(list))
	for name, quantity := range list {
		out[string(name)] = quantity.String()
	}

	return out
}
