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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	applicationpackage "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/application-package"
	packagestatusservice "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/status-package-service"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	controllerName = "d8-application-controller"

	// Warning: Don't change this value, this controller do not do any hard work.
	// This may affect concurrent access to deployedApps field in ApplicationPackageVersion
	// If you need parallel processing, you need to implement new controller
	maxConcurrentReconciles = 1

	requeueTime = 30 * time.Second
)

type reconciler struct {
	client        client.Client
	dc            dependency.Container
	pm            applicationpackage.PackageManager
	statusService *StatusService
	logger        *log.Logger
}

type StatusService struct {
	client       client.Client
	logger       *log.Logger
	pm           applicationpackage.PackageManager
	dc           dependency.Container
	eventChannel <-chan packagestatusservice.PackageEvent

	mu sync.RWMutex
	wg sync.WaitGroup
}

func (svc *StatusService) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				svc.wg.Done()
				return
			case event := <-svc.eventChannel:
				svc.wg.Add(1)
				svc.HandleEvent(ctx, event)
				svc.wg.Done()
			}
		}
	}()
}

func (svc *StatusService) HandleEvent(ctx context.Context, event packagestatusservice.PackageEvent) {
	logger := svc.logger.With(
		slog.String("package", event.PackageName),
		slog.String("name", event.Name),
		slog.String("namespace", event.Namespace),
		slog.String("version", event.Version),
		slog.String("type", event.Type),
	)

	app := &v1alpha1.Application{}
	err := svc.client.Get(ctx, types.NamespacedName{
		Name:      event.Name,
		Namespace: event.Namespace,
	}, app)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Debug("application not found, skipping")
			return
		}
		logger.Warn("failed to get application", log.Err(err))
		return
	}

	if app.Spec.PackageName != event.PackageName || app.Spec.Version != event.Version {
		logger.Debug("application spec mismatch, skipping")
		return
	}

	status, err := svc.pm.GetPackageStatus(ctx, event.PackageName, event.Namespace, event.Version, event.Type)
	if err != nil {
		logger.Warn("failed to get package status", log.Err(err))
		return
	}

	original := app.DeepCopy()
	svc.applyConditions(app, status.Conditions)
	svc.applyInternalConditions(app, status.InternalConditions)
	err = svc.client.Status().Patch(ctx, app, client.MergeFrom(original))
	if err != nil {
		logger.Warn("failed to patch application status", log.Err(err))
	}
}

func (svc *StatusService) applyConditions(app *v1alpha1.Application, newConds []v1alpha1.ApplicationStatusCondition) {
	now := metav1.NewTime(svc.dc.GetClock().Now())

	prev := make(map[string]v1alpha1.ApplicationStatusCondition)
	for _, c := range app.Status.Conditions {
		prev[c.Type] = c
	}

	applied := make([]v1alpha1.ApplicationStatusCondition, 0, len(newConds))
	for _, c := range newConds {
		cond := c
		cond.LastProbeTime = now

		p, ok := prev[cond.Type]
		if ok {
			cond.LastTransitionTime = p.LastTransitionTime
		}

		if p.Status != cond.Status {
			cond.LastTransitionTime = now
		}
		applied = append(applied, cond)
	}

	app.Status.Conditions = applied
}

func (svc *StatusService) applyInternalConditions(app *v1alpha1.Application, newConds []v1alpha1.ApplicationInternalStatusCondition) {
	now := metav1.NewTime(svc.dc.GetClock().Now())
	prev := make(map[string]v1alpha1.ApplicationInternalStatusCondition)
	for _, c := range app.Status.InternalConditions {
		prev[c.Type] = c
	}

	applied := make([]v1alpha1.ApplicationInternalStatusCondition, 0, len(newConds))
	for _, c := range newConds {
		cond := c
		cond.LastProbeTime = now

		p, ok := prev[cond.Type]
		if ok {
			cond.LastTransitionTime = p.LastTransitionTime
		}

		if p.Status != cond.Status {
			cond.LastTransitionTime = now
		}
		applied = append(applied, cond)
	}

	app.Status.InternalConditions = applied
}

func RegisterController(
	runtimeManager manager.Manager,
	dc dependency.Container,
	pm applicationpackage.PackageManager,
	logger *log.Logger,
) error {
	eventChannel := make(chan packagestatusservice.PackageEvent, 100)

	statusService := &StatusService{
		client:       runtimeManager.GetClient(),
		logger:       logger.Named("status-service"),
		pm:           pm,
		dc:           dc,
		eventChannel: eventChannel,
	}

	pm.SetEventChannel(eventChannel)
	go statusService.Start(context.Background())

	r := &reconciler{
		client:        runtimeManager.GetClient(),
		dc:            dc,
		pm:            pm,
		statusService: statusService,
		logger:        logger,
	}

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

func (svc *StatusService) WaitForIdle(_ context.Context) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	svc.wg.Wait()
	return nil
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	res := ctrl.Result{}

	r.logger.Debug("reconciling Application", slog.String("name", req.Name), slog.String("namespace", req.Namespace))

	app := new(v1alpha1.Application)
	if err := r.client.Get(ctx, req.NamespacedName, app); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Debug("application not found", slog.String("name", req.Name), slog.String("namespace", req.Namespace))

			return res, nil
		}

		r.logger.Warn("failed to get application", slog.String("name", req.Name), slog.String("namespace", req.Namespace), log.Err(err))

		return res, err
	}

	// handle delete event
	if !app.DeletionTimestamp.IsZero() {
		return r.handleDelete(ctx, app)
	}

	// handle create/update events
	err := r.handleCreateOrUpdate(ctx, app)
	if err != nil {
		r.logger.Warn("failed to handle application", slog.String("name", app.Name), log.Err(err))

		return ctrl.Result{RequeueAfter: requeueTime}, nil
	}

	return res, nil
}

func (r *reconciler) handleCreateOrUpdate(ctx context.Context, app *v1alpha1.Application) error {
	logger := r.logger.With(slog.String("name", app.Name))

	logger.Debug("handle Application")
	defer logger.Debug("handle Application complete")

	original := app.DeepCopy()

	// check if application package exists
	ap := new(v1alpha1.ApplicationPackage)
	if err := r.client.Get(ctx, types.NamespacedName{Name: app.Spec.PackageName}, ap); err != nil {
		r.SetConditionFalse(
			app,
			v1alpha1.ApplicationConditionTypeProcessed,
			v1alpha1.ApplicationConditionReasonApplicationPackageNotFound,
			fmt.Sprintf("get ApplicationPackage for %s not found: %s", app.Spec.PackageName, err.Error()),
		)

		patchErr := r.client.Status().Patch(ctx, app, client.MergeFrom(original))
		if patchErr != nil {
			return fmt.Errorf("patch status application %s: %w", app.Name, patchErr)
		}

		return fmt.Errorf("get ApplicationPackage for %s: %w", app.Spec.PackageName, err)
	}

	apvName := v1alpha1.MakeApplicationPackageVersionName(app.Spec.PackageRepository, app.Spec.PackageName, app.Spec.Version)
	apv := new(v1alpha1.ApplicationPackageVersion)
	err := r.client.Get(ctx, types.NamespacedName{Name: apvName}, apv)
	if err != nil {
		r.SetConditionFalse(
			app,
			v1alpha1.ApplicationConditionTypeProcessed,
			v1alpha1.ApplicationConditionReasonVersionNotFound,
			fmt.Sprintf("get ApplicationPackageVersion for %s not found: %s", app.Name, err.Error()),
		)

		patchErr := r.client.Status().Patch(ctx, app, client.MergeFrom(original))
		if patchErr != nil {
			return fmt.Errorf("patch status application %s: %w", app.Name, patchErr)
		}

		return fmt.Errorf("get ApplicationPackageVersion for %s: %w", app.Name, err)
	}

	if apv.IsDraft() {
		app = r.SetConditionFalse(
			app,
			v1alpha1.ApplicationConditionTypeProcessed,
			v1alpha1.ApplicationConditionReasonVersionIsDraft,
			"ApplicationPackageVersion "+apvName+" is draft",
		)

		patchErr := r.client.Status().Patch(ctx, app, client.MergeFrom(original))
		if patchErr != nil {
			return fmt.Errorf("patch status application %s: %w", app.Name, patchErr)
		}

		return fmt.Errorf("applicationPackageVersion %s is draft", apvName)
	}

	if !apv.Status.IsAppInstalled(app.Namespace, app.Name) {
		original := apv.DeepCopy()

		apv.Status = apv.Status.AddInstalledApp(app.Namespace, app.Name)

		if err := r.client.Status().Patch(ctx, apv, client.MergeFrom(original)); err != nil {
			return fmt.Errorf("patch ApplicationPackageVersion status for %s: %w", app.Spec.PackageName, err)
		}
	}

	if !ap.Status.IsAppInstalled(app.Namespace, app.Name) {
		original := ap.DeepCopy()

		ap.Status = ap.Status.AddInstalledApp(app.Namespace, app.Name)

		if err := r.client.Status().Patch(ctx, ap, client.MergeFrom(original)); err != nil {
			return fmt.Errorf("patch ApplicationPackage status for %s: %w", app.Spec.PackageName, err)
		}
	}

	app = r.SetConditionTrue(app, v1alpha1.ApplicationConditionTypeProcessed)

	err = r.client.Status().Patch(ctx, app, client.MergeFrom(original))
	if err != nil {
		return fmt.Errorf("patch status application %s: %w", app.Name, err)
	}

	// set finalizer if it is not set
	if !controllerutil.ContainsFinalizer(app, v1alpha1.ApplicationFinalizer) {
		original = app.DeepCopy()

		controllerutil.AddFinalizer(app, v1alpha1.ApplicationFinalizer)

		err = r.client.Patch(ctx, app, client.MergeFrom(original))
		if err != nil {
			return fmt.Errorf("patch application %s: %w", app.Name, err)
		}
	}

	// call PackageOperator method (maybe PackageAdder interface)
	r.pm.AddApplication(ctx, app, &apv.Status)

	return nil
}

func (r *reconciler) handleDelete(ctx context.Context, app *v1alpha1.Application) (ctrl.Result, error) {
	res := ctrl.Result{}

	logger := r.logger.With(slog.String("name", app.Name))

	logger.Debug("handling delete Application")

	ap := new(v1alpha1.ApplicationPackage)
	err := r.client.Get(ctx, types.NamespacedName{Name: app.Spec.PackageName}, ap)
	if err != nil && !apierrors.IsNotFound(err) {
		logger.Warn("failed to get ApplicationPackage", slog.String("name", app.Spec.PackageName), log.Err(err))
		return res, fmt.Errorf("get ApplicationPackage for %s: %w", app.Spec.PackageName, err)
	}

	if ap.Status.IsAppInstalled(app.Namespace, app.Name) {
		original := ap.DeepCopy()

		ap.Status = ap.Status.RemoveInstalledApp(app.Namespace, app.Name)

		if err := r.client.Status().Patch(ctx, ap, client.MergeFrom(original)); err != nil {
			return res, fmt.Errorf("patch ApplicationPackage status for %s: %w", app.Spec.PackageName, err)
		}
	}

	apv := new(v1alpha1.ApplicationPackageVersion)
	err = r.client.Get(ctx, types.NamespacedName{Name: v1alpha1.MakeApplicationPackageVersionName(app.Spec.PackageRepository, app.Spec.PackageName, app.Spec.Version)}, apv)
	if err != nil && !apierrors.IsNotFound(err) {
		logger.Warn("failed to get ApplicationPackageVersion", slog.String("name", v1alpha1.MakeApplicationPackageVersionName(app.Spec.PackageRepository, app.Spec.PackageName, app.Spec.Version)), log.Err(err))
		return res, fmt.Errorf("get ApplicationPackageVersion for %s: %w", v1alpha1.MakeApplicationPackageVersionName(app.Spec.PackageRepository, app.Spec.PackageName, app.Spec.Version), err)
	}

	if apv.Status.IsAppInstalled(app.Namespace, app.Name) {
		original := apv.DeepCopy()

		apv.Status = apv.Status.RemoveInstalledApp(app.Namespace, app.Name)

		if err := r.client.Status().Patch(ctx, apv, client.MergeFrom(original)); err != nil {
			return res, fmt.Errorf("patch ApplicationPackageVersion status for %s: %w", app.Spec.PackageName, err)
		}
	}

	logger.Debug("deleting Application")

	r.pm.RemoveApplication(ctx, app)

	// remove finalizer
	if controllerutil.ContainsFinalizer(app, v1alpha1.ApplicationFinalizer) {
		controllerutil.RemoveFinalizer(app, v1alpha1.ApplicationFinalizer)
	}

	logger.Debug("delete Application complete")

	return res, nil
}

func (r *reconciler) SetConditionTrue(app *v1alpha1.Application, condType string) *v1alpha1.Application {
	time := metav1.NewTime(r.dc.GetClock().Now())

	for idx, cond := range app.Status.Conditions {
		if cond.Type == condType {
			app.Status.Conditions[idx].LastProbeTime = time
			if cond.Status != corev1.ConditionTrue {
				app.Status.Conditions[idx].LastTransitionTime = time
				app.Status.Conditions[idx].Status = corev1.ConditionTrue
			}

			app.Status.Conditions[idx].Reason = ""
			app.Status.Conditions[idx].Message = ""

			return app
		}
	}

	app.Status.Conditions = append(app.Status.Conditions, v1alpha1.ApplicationStatusCondition{
		Type:               condType,
		Status:             corev1.ConditionTrue,
		LastProbeTime:      time,
		LastTransitionTime: time,
	})

	return app
}

func (r *reconciler) SetConditionFalse(app *v1alpha1.Application, condType string, reason string, message string) *v1alpha1.Application {
	time := metav1.NewTime(r.dc.GetClock().Now())

	for idx, cond := range app.Status.Conditions {
		if cond.Type == condType {
			app.Status.Conditions[idx].LastProbeTime = time
			if cond.Status != corev1.ConditionFalse {
				app.Status.Conditions[idx].LastTransitionTime = time
				app.Status.Conditions[idx].Status = corev1.ConditionFalse
			}

			app.Status.Conditions[idx].Reason = reason
			app.Status.Conditions[idx].Message = message

			return app
		}
	}

	app.Status.Conditions = append(app.Status.Conditions, v1alpha1.ApplicationStatusCondition{
		Type:               condType,
		Status:             corev1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		LastProbeTime:      time,
		LastTransitionTime: time,
	})

	return app
}
