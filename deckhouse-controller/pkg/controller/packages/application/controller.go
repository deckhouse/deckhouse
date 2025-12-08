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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager/apps"
	packageoperator "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/operator"
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
	operator *packageoperator.Operator
	status   *status.Service

	moduleManager moduleManager
	dc            dependency.Container
	logger        *log.Logger
}

type moduleManager interface {
	AreModulesInited() bool
}

func RegisterController(
	runtimeManager manager.Manager,
	operator *packageoperator.Operator,
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

		r.setConditionFalse(
			app,
			v1alpha1.ApplicationConditionTypeProcessed,
			v1alpha1.ApplicationConditionReasonApplicationPackageNotFound,
			fmt.Sprintf("ApplicationPackage '%s' not found", app.Spec.PackageName),
		)

		if err := r.client.Status().Patch(ctx, app, client.MergeFrom(original)); err != nil {
			return fmt.Errorf("patch status application %s: %w", app.Name, err)
		}

		return fmt.Errorf("get application package '%s': %w", app.Spec.PackageName, err)
	}

	apvName := v1alpha1.MakeApplicationPackageVersionName(app.Spec.PackageRepository, app.Spec.PackageName, app.Spec.Version)
	logger.Debug("check application package version exists", slog.String("apv", apvName))

	// check if application package version exists
	apv := new(v1alpha1.ApplicationPackageVersion)
	if err := r.client.Get(ctx, client.ObjectKey{Name: apvName}, apv); err != nil {
		logger.Debug("application package version not found", slog.String("apv", apvName), log.Err(err))

		r.setConditionFalse(
			app,
			v1alpha1.ApplicationConditionTypeProcessed,
			v1alpha1.ApplicationConditionReasonVersionNotFound,
			fmt.Sprintf("ApplicationPackageVersion '%s' not found", apv.Name),
		)

		if err := r.client.Status().Patch(ctx, app, client.MergeFrom(original)); err != nil {
			return fmt.Errorf("patch application status '%s': %w", app.Name, err)
		}

		return fmt.Errorf("get application package version '%s': %w", apv.Name, err)
	}

	// check if application package version is not draft
	if apv.IsDraft() {
		logger.Debug("application package version is in draft", slog.String("apv", apvName))

		app = r.setConditionFalse(
			app,
			v1alpha1.ApplicationConditionTypeProcessed,
			v1alpha1.ApplicationConditionReasonVersionIsDraft,
			"ApplicationPackageVersion "+apvName+" is in draft",
		)

		if err := r.client.Status().Patch(ctx, app, client.MergeFrom(original)); err != nil {
			return fmt.Errorf("patch application status '%s': %w", app.Name, err)
		}

		return fmt.Errorf("application package version '%s' is draft", apvName)
	}

	logger.Debug("check if application installed to ApplicationPackageVersion", slog.String("apv", apv.Name))

	if !apv.IsAppInstalled(app.Namespace, app.Name) {
		logger.Debug("application not installed to ApplicationPackageVersion, install it", slog.String("apv", apv.Name))

		patch := client.MergeFrom(apv.DeepCopy())

		apv = apv.AddInstalledApp(app.Namespace, app.Name)
		if err := r.client.Status().Patch(ctx, apv, patch); err != nil {
			return fmt.Errorf("patch application package version status '%s': %w", apv.Name, err)
		}
	}

	logger.Debug("check if application installed to ApplicationPackage", slog.String("package", ap.Name))

	if !ap.IsAppInstalled(app.Namespace, app.Name) {
		logger.Debug("application not installed to ApplicationPackage, install it", slog.String("package", ap.Name))

		patch := client.MergeFrom(ap.DeepCopy())

		ap = ap.AddInstalledApp(app.Namespace, app.Name)
		if err := r.client.Status().Patch(ctx, ap, patch); err != nil {
			return fmt.Errorf("patch application package status '%s': %w", ap.Name, err)
		}
	}

	logger.Debug("registry application to operator")
	if err := r.updateOperatorPackage(ctx, app, apv); err != nil {
		return err
	}

	app = r.setConditionTrue(app, v1alpha1.ApplicationConditionTypeProcessed)
	if err := r.client.Status().Patch(ctx, app, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch application status '%s': %w", app.Name, err)
	}

	// set finalizer if it is not set
	if !controllerutil.ContainsFinalizer(app, v1alpha1.ApplicationFinalizerStatisticRegistered) {
		logger.Debug("add finalizer to application")
		controllerutil.AddFinalizer(app, v1alpha1.ApplicationFinalizerStatisticRegistered)
	}

	app = r.addOwnerReferences(app, apv, ap)
	if err := r.client.Patch(ctx, app, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch application '%s': %w", app.Name, err)
	}

	return nil
}

func (r *reconciler) updateOperatorPackage(ctx context.Context, app *v1alpha1.Application, apv *v1alpha1.ApplicationPackageVersion) error {
	repo := new(v1alpha1.PackageRepository)
	if err := r.client.Get(ctx, client.ObjectKey{Name: app.Spec.PackageRepository}, repo); err != nil {
		return fmt.Errorf("get package repository '%s': %w", app.Spec.PackageRepository, err)
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

	r.operator.Update(repo, packageoperator.Instance{
		Name:      app.Name,
		Namespace: app.Namespace,
		Definition: apps.Definition{
			Name:         apv.Status.PackageName,
			Version:      apv.Status.Version,
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

	apvName := v1alpha1.MakeApplicationPackageVersionName(app.Spec.PackageRepository, app.Spec.PackageName, app.Spec.Version)
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
	r.operator.Remove(app.Namespace, app.Name)

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

func (r *reconciler) setConditionTrue(app *v1alpha1.Application, condType string) *v1alpha1.Application {
	now := metav1.NewTime(r.dc.GetClock().Now())

	for idx, cond := range app.Status.ResourceConditions {
		if cond.Type == condType {
			app.Status.ResourceConditions[idx].LastProbeTime = now
			if cond.Status != corev1.ConditionTrue {
				app.Status.ResourceConditions[idx].LastTransitionTime = now
				app.Status.ResourceConditions[idx].Status = corev1.ConditionTrue
			}

			app.Status.ResourceConditions[idx].Reason = ""
			app.Status.ResourceConditions[idx].Message = ""

			return app
		}
	}

	app.Status.ResourceConditions = append(app.Status.ResourceConditions, v1alpha1.ApplicationResourceStatusCondition{
		Type:               condType,
		Status:             corev1.ConditionTrue,
		LastProbeTime:      now,
		LastTransitionTime: now,
	})

	return app
}

func (r *reconciler) setConditionFalse(app *v1alpha1.Application, condType string, reason string, message string) *v1alpha1.Application {
	now := metav1.NewTime(r.dc.GetClock().Now())

	for idx, cond := range app.Status.ResourceConditions {
		if cond.Type == condType {
			app.Status.Conditions[idx].LastProbeTime = now
			if cond.Status != corev1.ConditionFalse {
				app.Status.Conditions[idx].LastTransitionTime = now
				app.Status.Conditions[idx].Status = corev1.ConditionFalse
			}

			app.Status.Conditions[idx].Reason = reason
			app.Status.Conditions[idx].Message = message

			return app
		}
	}

	app.Status.ResourceConditions = append(app.Status.ResourceConditions, v1alpha1.ApplicationResourceStatusCondition{
		Type:               condType,
		Status:             corev1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		LastProbeTime:      now,
		LastTransitionTime: now,
	})

	return app
}

func (r *reconciler) addOwnerReferences(app *v1alpha1.Application, apv *v1alpha1.ApplicationPackageVersion, ap *v1alpha1.ApplicationPackage) *v1alpha1.Application {
	logger := r.logger.With(slog.String("name", app.Name), slog.String("namespace", app.Namespace))

	ownerRefs := app.GetOwnerReferences()
	trueLink := &[]bool{true}[0]
	falseLink := &[]bool{false}[0]

	isAPVRefSet := false
	isAPRefSet := false

	// check which owner references are not set
	for _, ref := range ownerRefs {
		if ref.Kind == v1alpha1.ApplicationPackageVersionKind {
			isAPVRefSet = true
			continue
		}

		if ref.Kind == v1alpha1.ApplicationPackageKind {
			isAPRefSet = true
			continue
		}
	}

	// add owner references if they are not set
	if !isAPVRefSet {
		logger.Debug("adding ApplicationPackageVersion owner reference to application", slog.String("apv_name", apv.Name))

		ownerRefs = append(ownerRefs, metav1.OwnerReference{
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

		ownerRefs = append(ownerRefs, metav1.OwnerReference{
			APIVersion:         v1alpha1.ApplicationPackageGVK.GroupVersion().String(),
			Kind:               v1alpha1.ApplicationPackageKind,
			Name:               ap.Name,
			UID:                ap.UID,
			Controller:         falseLink,
			BlockOwnerDeletion: trueLink,
		})
	}

	app.SetOwnerReferences(ownerRefs)

	return app
}
