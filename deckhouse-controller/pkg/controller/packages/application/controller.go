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
}

func (svc *StatusService) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-svc.eventChannel:
				svc.handleEvent(ctx, event)
			}
		}
	}()
}

func (svc *StatusService) handleEvent(ctx context.Context, event packagestatusservice.PackageEvent) {
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

	if app.Spec.ApplicationPackageName != event.PackageName || app.Spec.Version != event.Version {
		logger.Debug("application spec mismatch, skipping")
		return
	}

	status, err := svc.pm.GetPackageStatus(ctx, event.PackageName, event.Namespace, event.Version, event.Type)
	if err != nil {
		logger.Warn("failed to get package status", log.Err(err))
		svc.updateCondition(ctx, app, false, "StatusCheckFailed", err.Error())
		return
	}

	original := app.DeepCopy()

	switch {
	case status.Error != "":
		svc.setConditionFalse(app, v1alpha1.ApplicationConditionInstalled, "PackageError", status.Error)
	case status.Installed && status.Ready:
		svc.setConditionTrue(app, v1alpha1.ApplicationConditionInstalled)
		svc.setConditionTrue(app, v1alpha1.ApplicationConditionReady)
	case status.Installed:
		svc.setConditionTrue(app, v1alpha1.ApplicationConditionInstalled)
		svc.setConditionFalse(app, v1alpha1.ApplicationConditionReady, "NotReady", "Package is installed but not ready")
	case status.Installed:
		svc.setConditionFalse(app, v1alpha1.ApplicationConditionInstalled, "NotInstalled", "Package is not installed")
	}

	err = svc.client.Status().Patch(ctx, app, client.MergeFrom(original))
	if err != nil {
		logger.Warn("failed to patch application status", log.Err(err))
	}
}

func (svc *StatusService) updateCondition(ctx context.Context, app *v1alpha1.Application, status bool, reason, message string) {
	original := app.DeepCopy()
	if status {
		svc.setConditionTrue(app, v1alpha1.ApplicationConditionInstalled)
	} else {
		svc.setConditionFalse(app, v1alpha1.ApplicationConditionInstalled, reason, message)
	}
	err := svc.client.Status().Patch(ctx, app, client.MergeFrom(original))
	if err != nil {
		svc.logger.Warn("failed to patch application status", log.Err(err))
	}
}

func (svc *StatusService) setConditionTrue(app *v1alpha1.Application, condType string) {
	time := metav1.NewTime(svc.dc.GetClock().Now())

	for idx, cond := range app.Status.Conditions {
		if cond.Type == condType {
			app.Status.Conditions[idx].LastProbeTime = time
			if cond.Status != corev1.ConditionTrue {
				app.Status.Conditions[idx].LastTransitionTime = time
				app.Status.Conditions[idx].Status = corev1.ConditionTrue
			}
			app.Status.Conditions[idx].Reason = ""
			app.Status.Conditions[idx].Message = ""
			return
		}
	}

	app.Status.Conditions = append(app.Status.Conditions, v1alpha1.ApplicationStatusCondition{
		Type:               condType,
		Status:             corev1.ConditionTrue,
		LastProbeTime:      time,
		LastTransitionTime: time,
	})
}

func (svc *StatusService) setConditionFalse(app *v1alpha1.Application, condType string, reason string, message string) {
	time := metav1.NewTime(svc.dc.GetClock().Now())

	for idx, cond := range app.Status.Conditions {
		if cond.Type == condType {
			app.Status.Conditions[idx].LastProbeTime = time
			if cond.Status != corev1.ConditionFalse {
				app.Status.Conditions[idx].LastTransitionTime = time
				app.Status.Conditions[idx].Status = corev1.ConditionFalse
			}
			app.Status.Conditions[idx].Reason = reason
			app.Status.Conditions[idx].Message = message
			return
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

	switch p := pm.(type) {
	case *applicationpackage.PackageOperator:
		p.SetEventChannel(eventChannel)
	case *applicationpackage.PackageOperatorStub:
		p.SetEventChannel(eventChannel)
	}

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
		err := r.handleDelete(ctx, app)
		if err != nil {
			r.logger.Warn("delete application", slog.String("name", app.Name), log.Err(err))

			return res, err
		}

		return res, nil
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

	logger.Debug("handling Application")

	original := app.DeepCopy()

	apvName := v1alpha1.MakeApplicationPackageVersionName(app.Spec.Repository, app.Spec.ApplicationPackageName, app.Spec.Version)
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

	// call PackageOperator method (maybe PackageAdder interface)
	r.pm.AddApplication(ctx, app, &apv.Status)

	app = r.SetConditionTrue(app, v1alpha1.ApplicationConditionTypeProcessed)

	err = r.client.Status().Patch(ctx, app, client.MergeFrom(original))
	if err != nil {
		return fmt.Errorf("patch status application %s: %w", app.Name, err)
	}

	// add finalizer
	if !controllerutil.ContainsFinalizer(app, v1alpha1.ApplicationProcessedFinalizer) {
		logger.Debug("add finalizer")
		controllerutil.AddFinalizer(app, v1alpha1.ApplicationProcessedFinalizer)
	}

	err = r.client.Patch(ctx, app, client.MergeFrom(original))
	if err != nil {
		return fmt.Errorf("patch application %s: %w", app.Name, err)
	}

	logger.Debug("handle Application complete")

	return nil
}

func (r *reconciler) handleDelete(ctx context.Context, app *v1alpha1.Application) error {
	logger := r.logger.With(slog.String("name", app.Name))

	logger.Debug("deleting Application")

	r.pm.RemoveApplication(ctx, app)

	// remove finalizer
	if controllerutil.ContainsFinalizer(app, v1alpha1.ApplicationProcessedFinalizer) {
		logger.Debug("remove finalizer")

		controllerutil.RemoveFinalizer(app, v1alpha1.ApplicationProcessedFinalizer)

		err := r.client.Update(ctx, app)
		if err != nil {
			return fmt.Errorf("remove finalizer for %s: %w", app.Name, err)
		}
	}

	logger.Debug("delete Application complete")

	return nil
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
