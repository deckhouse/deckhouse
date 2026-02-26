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
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/apps"
	packageoperator "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime"
	packagestatus "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/status"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	controllerName = "d8-application-controller"

	// Warning: Don't change this value, this controller do not do any hard work.
	// This may affect concurrent access to deployedApps field in ApplicationPackageVersion
	// If you need parallel processing, you need to implement new controller
	maxConcurrentReconciles = 1

	defaultRequeueTime = 30 * time.Second
)

type reconciler struct {
	init     *sync.WaitGroup
	client   client.Client
	operator packageOperator
	status   *status.Service

	moduleManager moduleManager
	dc            dependency.Container
	logger        *log.Logger
}

type moduleManager interface {
	AreModulesInited() bool
}

type packageOperator interface {
	UpdateApp(repo registry.Remote, inst packageoperator.App)
	RemoveApp(namespace, name string)
	Status() *packagestatus.Service
}

func RegisterController(
	runtimeManager manager.Manager,
	operator packageOperator,
	moduleManager moduleManager,
	dc dependency.Container,
	logger *log.Logger,
) error {
	r := &reconciler{
		init:          new(sync.WaitGroup),
		client:        runtimeManager.GetClient(),
		operator:      operator,
		moduleManager: moduleManager,
		dc:            dc,
		logger:        logger,
	}

	r.init.Add(1)

	// add preflight to set the cluster UUID
	if err := runtimeManager.Add(manager.RunnableFunc(r.preflight)); err != nil {
		return fmt.Errorf("add preflight: %w", err)
	}

	r.status = status.NewService(r.client, operator.Status().GetStatus, r.logger)
	r.status.Start(context.Background(), operator.Status().GetCh())

	applicationController, err := controller.New(controllerName, runtimeManager, controller.Options{
		MaxConcurrentReconciles: maxConcurrentReconciles,
		Reconciler:              r,
	})
	if err != nil {
		return fmt.Errorf("create controller: %w", err)
	}

	return ctrl.NewControllerManagedBy(runtimeManager).
		For(&v1alpha1.Application{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})).
		Complete(applicationController)
}

func (r *reconciler) preflight(ctx context.Context) error {
	defer r.init.Done()

	// wait until module manager init
	r.logger.Debug("wait until module manager is inited")
	if err := wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(_ context.Context) (bool, error) {
		return r.moduleManager.AreModulesInited(), nil
	}); err != nil {
		return fmt.Errorf("init module manager: %w", err)
	}

	r.logger.Debug("controller is ready")

	return nil
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

		return ctrl.Result{RequeueAfter: defaultRequeueTime}, nil
	}

	return ctrl.Result{}, nil
}

func (r *reconciler) handleCreateOrUpdate(ctx context.Context, app *v1alpha1.Application) error {
	logger := r.logger.With(slog.String("name", app.Name), slog.String("namespace", app.Namespace))

	logger.Debug("handle application")
	defer logger.Debug("handle application complete")

	original := app.DeepCopy()

	logger.Debug("check application package exists", slog.String("package", app.Spec.PackageName))

	// check if application package exists
	ap := new(v1alpha1.ApplicationPackage)
	if err := r.client.Get(ctx, client.ObjectKey{Name: app.Spec.PackageName}, ap); err != nil {
		logger.Debug("application package not found", slog.String("package", app.Spec.PackageName), log.Err(err))

		// TODO: Processed = "false"

		return fmt.Errorf("get application package '%s': %w", app.Spec.PackageName, err)
	}

	apvName := v1alpha1.MakeApplicationPackageVersionName(app.Spec.PackageRepositoryName, app.Spec.PackageName, app.Spec.PackageVersion)
	logger.Debug("check application package version exists", slog.String("apv", apvName))

	// check if application package version exists
	apv := new(v1alpha1.ApplicationPackageVersion)
	if err := r.client.Get(ctx, client.ObjectKey{Name: apvName}, apv); err != nil {
		logger.Debug("application package version not found", slog.String("apv", apvName), log.Err(err))

		// TODO: Processed = "false"

		return fmt.Errorf("get application package version '%s': %w", apv.Name, err)
	}

	// check if application package version is not draft
	if apv.IsDraft() {
		logger.Debug("application package version is in draft", slog.String("apv", apvName))

		// TODO: Processed = "false"

		return fmt.Errorf("application package version '%s' is draft", apvName)
	}

	logger.Debug("check if application installed to ApplicationPackageVersion", slog.String("apv", apv.Name))

	// Check if application is switching from a different version
	oldAPVName := r.findOldAPVReference(app)
	if oldAPVName != "" && oldAPVName != apvName {
		logger.Debug("application is switching versions, cleaning up old APV", slog.String("old_apv", oldAPVName), slog.String("new_apv", apvName))

		oldAPV := new(v1alpha1.ApplicationPackageVersion)
		if err := r.client.Get(ctx, client.ObjectKey{Name: oldAPVName}, oldAPV); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("get old application package version '%s': %w", oldAPVName, err)
			}
			logger.Debug("old APV not found, skipping cleanup", slog.String("old_apv", oldAPVName))
		} else if oldAPV.IsAppInstalled(app.Namespace, app.Name) {
			logger.Debug("removing application from old APV", slog.String("old_apv", oldAPVName))

			patch := client.MergeFrom(oldAPV.DeepCopy())
			oldAPV = oldAPV.RemoveInstalledApp(app.Namespace, app.Name)
			if err := r.client.Status().Patch(ctx, oldAPV, patch); err != nil {
				return fmt.Errorf("patch old application package version status '%s': %w", oldAPVName, err)
			}
		}
	}

	if !apv.IsAppInstalled(app.Namespace, app.Name) {
		logger.Debug("application not installed to ApplicationPackageVersion, install it", slog.String("apv", apv.Name))

		patch := client.MergeFrom(apv.DeepCopy())

		apv = apv.AddInstalledApp(app.Namespace, app.Name)
		if err := r.client.Status().Patch(ctx, apv, patch); err != nil {
			return fmt.Errorf("patch application package version status '%s': %w", apv.Name, err)
		}
	}

	logger.Debug("check if application installed to ApplicationPackage", slog.String("package", ap.Name))

	// Check if application is switching to a different package
	oldAPName := r.findOldAPReference(app)
	if oldAPName != "" && oldAPName != ap.Name {
		logger.Debug("application is switching packages, cleaning up old AP", slog.String("old_ap", oldAPName), slog.String("new_ap", ap.Name))

		oldAP := new(v1alpha1.ApplicationPackage)
		if err := r.client.Get(ctx, client.ObjectKey{Name: oldAPName}, oldAP); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("get old application package '%s': %w", oldAPName, err)
			}
			logger.Debug("old AP not found, skipping cleanup", slog.String("old_ap", oldAPName))
		} else if oldAP.IsAppInstalled(app.Namespace, app.Name) {
			logger.Debug("removing application from old AP", slog.String("old_ap", oldAPName))

			patch := client.MergeFrom(oldAP.DeepCopy())
			oldAP = oldAP.RemoveInstalledApp(app.Namespace, app.Name)
			if err := r.client.Status().Patch(ctx, oldAP, patch); err != nil {
				return fmt.Errorf("patch old application package status '%s': %w", oldAPName, err)
			}
		}
	}

	if !ap.IsAppInstalled(app.Namespace, app.Name) {
		logger.Debug("application not installed to ApplicationPackage, install it", slog.String("package", ap.Name))

		patch := client.MergeFrom(ap.DeepCopy())

		ap = ap.AddInstalledApp(app.Namespace, app.Name, app.Spec.PackageVersion)
		if err := r.client.Status().Patch(ctx, ap, patch); err != nil {
			return fmt.Errorf("patch application package status '%s': %w", ap.Name, err)
		}
	} else if ap.GetAppVersion(app.Namespace, app.Name) != app.Spec.PackageVersion {
		logger.Debug("application version changed, updating ApplicationPackage", slog.String("package", ap.Name), slog.String("new_version", app.Spec.PackageVersion))

		patch := client.MergeFrom(ap.DeepCopy())

		ap.UpdateAppVersion(app.Namespace, app.Name, app.Spec.PackageVersion)
		if err := r.client.Status().Patch(ctx, ap, patch); err != nil {
			return fmt.Errorf("patch application package status '%s': %w", ap.Name, err)
		}
	}

	logger.Debug("registry application to operator")
	if err := r.updateOperatorPackage(ctx, app, apv); err != nil {
		return err
	}

	// TODO: Processed = "true"

	// set finalizer if it is not set
	if !controllerutil.ContainsFinalizer(app, v1alpha1.ApplicationFinalizerStatisticRegistered) {
		logger.Debug("add finalizer to application")
		controllerutil.AddFinalizer(app, v1alpha1.ApplicationFinalizerStatisticRegistered)
	}

	if _, set := app.GetAnnotations()[v1alpha1.ApplicationAnnotationRegistrySpecChanged]; set {
		delete(app.ObjectMeta.Annotations, v1alpha1.ApplicationAnnotationRegistrySpecChanged)
	}

	app = r.addOwnerReferences(app, apv, ap)
	if err := r.client.Patch(ctx, app, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch application '%s': %w", app.Name, err)
	}

	return nil
}

func (r *reconciler) updateOperatorPackage(ctx context.Context, app *v1alpha1.Application, apv *v1alpha1.ApplicationPackageVersion) error {
	repo := new(v1alpha1.PackageRepository)
	if err := r.client.Get(ctx, client.ObjectKey{Name: app.Spec.PackageRepositoryName}, repo); err != nil {
		return fmt.Errorf("get package repository '%s': %w", app.Spec.PackageRepositoryName, err)
	}

	var requirements apps.Requirements
	if apv.Status.PackageMetadata != nil && apv.Status.PackageMetadata.Requirements != nil {
		var err error

		var kubernetesConstraint *semver.Constraints
		if len(apv.Status.PackageMetadata.Requirements.Kubernetes) > 0 {
			if kubernetesConstraint, err = semver.NewConstraint(apv.Status.PackageMetadata.Requirements.Kubernetes); err != nil {
				return fmt.Errorf("parse kubernetes requirement: %w", err)
			}
		}

		var deckhouseConstraint *semver.Constraints
		if len(apv.Status.PackageMetadata.Requirements.Deckhouse) > 0 {
			if deckhouseConstraint, err = semver.NewConstraint(apv.Status.PackageMetadata.Requirements.Deckhouse); err != nil {
				return fmt.Errorf("parse deckhouse requirement: %w", err)
			}
		}

		modules := make(map[string]apps.Dependency)
		for module, rawConstraint := range apv.Status.PackageMetadata.Requirements.Modules {
			raw, optional := strings.CutSuffix(rawConstraint, "!optional")
			constraint, err := semver.NewConstraint(raw)
			if err != nil {
				return fmt.Errorf("parse module requirement '%s': %w", module, err)
			}

			modules[module] = apps.Dependency{
				Constraints: constraint,
				Optional:    optional,
			}
		}

		requirements = apps.Requirements{
			Kubernetes: kubernetesConstraint,
			Deckhouse:  deckhouseConstraint,
			Modules:    modules,
		}
	}

	r.operator.UpdateApp(registry.BuildRemote(repo), packageoperator.App{
		Name:      app.Name,
		Namespace: app.Namespace,
		Definition: apps.Definition{
			Name:         app.Spec.PackageName,
			Version:      app.Spec.PackageVersion,
			Requirements: requirements,
		},
		Settings: app.Spec.Settings.GetMap(),
	})

	return nil
}

func (r *reconciler) handleDelete(ctx context.Context, app *v1alpha1.Application) error {
	logger := r.logger.With(slog.String("name", app.Name), slog.String("namespace", app.Namespace))

	logger.Debug("handle delete application")
	defer logger.Debug("handle delete application complete")

	logger.Debug("check if application package exists", slog.String("package", app.Spec.PackageName))

	ap := new(v1alpha1.ApplicationPackage)
	if err := r.client.Get(ctx, client.ObjectKey{Name: app.Spec.PackageName}, ap); err != nil && !apierrors.IsNotFound(err) {
		logger.Warn("failed to get application package", slog.String("name", app.Spec.PackageName), log.Err(err))
		return fmt.Errorf("get application package '%s': %w", app.Spec.PackageName, err)
	}

	if ap.IsAppInstalled(app.Namespace, app.Name) {
		logger.Debug("application installed to ApplicationPackage, remove it", slog.String("package", ap.Name))

		patch := client.MergeFrom(ap.DeepCopy())

		ap = ap.RemoveInstalledApp(app.Namespace, app.Name)
		if err := r.client.Status().Patch(ctx, ap, patch); err != nil {
			return fmt.Errorf("patch ApplicationPackage status for %s: %w", app.Spec.PackageName, err)
		}
	}

	apvName := v1alpha1.MakeApplicationPackageVersionName(app.Spec.PackageRepositoryName, app.Spec.PackageName, app.Spec.PackageVersion)
	logger.Debug("check if application package version exists", slog.String("package", apvName))

	apv := new(v1alpha1.ApplicationPackageVersion)
	if err := r.client.Get(ctx, client.ObjectKey{Name: apvName}, apv); err != nil && !apierrors.IsNotFound(err) {
		logger.Warn("failed to get application package version", slog.String("name", apvName), log.Err(err))
		return fmt.Errorf("get application package version '%s': %w", apvName, err)
	}

	if apv.IsAppInstalled(app.Namespace, app.Name) {
		logger.Debug("application installed to application package version, remove it", slog.String("apv", apv.Name))

		patch := client.MergeFrom(apv.DeepCopy())

		apv = apv.RemoveInstalledApp(app.Namespace, app.Name)
		if err := r.client.Status().Patch(ctx, apv, patch); err != nil {
			return fmt.Errorf("patch application package version status '%s': %w", app.Spec.PackageName, err)
		}
	}

	logger.Debug("delete application")

	// call PackageOperator method (PackageRemover interface)
	r.operator.RemoveApp(app.Namespace, app.Name)

	patch := client.MergeFrom(app.DeepCopy())

	// remove finalizer if it is set
	if controllerutil.ContainsFinalizer(app, v1alpha1.ApplicationFinalizerStatisticRegistered) {
		logger.Debug("remove finalizer from application")
		controllerutil.RemoveFinalizer(app, v1alpha1.ApplicationFinalizerStatisticRegistered)
	}

	if err := r.client.Patch(ctx, app, patch); err != nil {
		return fmt.Errorf("patch application %s: %w", app.Name, err)
	}

	return nil
}

func (r *reconciler) addOwnerReferences(app *v1alpha1.Application, apv *v1alpha1.ApplicationPackageVersion, ap *v1alpha1.ApplicationPackage) *v1alpha1.Application {
	logger := r.logger.With(slog.String("name", app.Name), slog.String("namespace", app.Namespace))

	ownerRefs := app.GetOwnerReferences()
	trueLink := ptr.To(true)
	falseLink := ptr.To(false)

	isAPVRefSet := false
	isAPRefSet := false
	newOwnerRefs := []metav1.OwnerReference{}

	// check which owner references are set and remove stale APV references
	for _, ref := range ownerRefs {
		if ref.Kind == v1alpha1.ApplicationPackageVersionKind {
			if ref.Name == apv.Name {
				isAPVRefSet = true
				newOwnerRefs = append(newOwnerRefs, ref)
			} else {
				logger.Debug("removing stale ApplicationPackageVersion owner reference", slog.String("old_apv_name", ref.Name))
			}
			continue
		}

		if ref.Kind == v1alpha1.ApplicationPackageKind {
			if ref.Name == ap.Name {
				isAPRefSet = true
				newOwnerRefs = append(newOwnerRefs, ref)
			} else {
				logger.Debug("removing stale ApplicationPackage owner reference", slog.String("old_ap_name", ref.Name))
			}
			continue
		}

		newOwnerRefs = append(newOwnerRefs, ref)
	}

	// add owner references if they are not set
	if !isAPVRefSet {
		logger.Debug("adding ApplicationPackageVersion owner reference to application", slog.String("apv_name", apv.Name))

		newOwnerRefs = append(newOwnerRefs, metav1.OwnerReference{
			APIVersion:         v1alpha1.ApplicationPackageVersionGVK.GroupVersion().String(),
			Kind:               v1alpha1.ApplicationPackageVersionKind,
			Name:               apv.Name,
			UID:                apv.UID,
			Controller:         falseLink,
			BlockOwnerDeletion: trueLink,
		})
	}

	if !isAPRefSet {
		logger.Debug("adding ApplicationPackage owner reference to application", slog.String("ap_name", ap.Name))

		newOwnerRefs = append(newOwnerRefs, metav1.OwnerReference{
			APIVersion:         v1alpha1.ApplicationPackageGVK.GroupVersion().String(),
			Kind:               v1alpha1.ApplicationPackageKind,
			Name:               ap.Name,
			UID:                ap.UID,
			Controller:         falseLink,
			BlockOwnerDeletion: trueLink,
		})
	}

	app.SetOwnerReferences(newOwnerRefs)

	return app
}

// findOldAPVReference searches for an existing ApplicationPackageVersion owner reference
// and returns its name if found. Returns empty string if no APV reference exists.
func (r *reconciler) findOldAPVReference(app *v1alpha1.Application) string {
	for _, ref := range app.GetOwnerReferences() {
		if ref.Kind == v1alpha1.ApplicationPackageVersionKind {
			return ref.Name
		}
	}
	return ""
}

// findOldAPReference searches for an existing ApplicationPackage owner reference
// and returns its name if found. Returns empty string if no AP reference exists.
func (r *reconciler) findOldAPReference(app *v1alpha1.Application) string {
	for _, ref := range app.GetOwnerReferences() {
		if ref.Kind == v1alpha1.ApplicationPackageKind {
			return ref.Name
		}
	}
	return ""
}
