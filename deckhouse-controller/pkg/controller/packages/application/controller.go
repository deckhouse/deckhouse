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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	controllerName = "d8-application-controller"

	maxConcurrentReconciles = 1
)

type reconciler struct {
	client client.Client
	dc     dependency.Container
	exts   *extenders.ExtendersStack
	logger *log.Logger
}

func RegisterController(
	runtimeManager manager.Manager,
	dc dependency.Container,
	exts *extenders.ExtendersStack,
	logger *log.Logger,
) error {
	r := &reconciler{
		client: runtimeManager.GetClient(),
		dc:     dc,
		exts:   exts,
		logger: logger,
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

	application := new(v1alpha1.Application)
	if err := r.client.Get(ctx, req.NamespacedName, application); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Warn("application not found", slog.String("name", req.Name), slog.String("namespace", req.Namespace))
			return res, nil
		}
		r.logger.Error("failed to get application", slog.String("name", req.Name), slog.String("namespace", req.Namespace), log.Err(err))
		return res, err
	}

	// handle delete event
	if !application.DeletionTimestamp.IsZero() {
		return r.delete(ctx, application)
	}

	// handle create/update events
	return r.handle(ctx, application)
}

func (r *reconciler) handle(ctx context.Context, app *v1alpha1.Application) (ctrl.Result, error) {
	res := ctrl.Result{}

	original := app.DeepCopy()

	logger := r.logger.With(slog.String("name", app.Name))
	logger.Debug("handling Application")
	defer logger.Debug("handle Application complete")

	// find ApplicationPackageVersion by spec.ApplicationPackageName and spec.version
	apvName := app.Spec.ApplicationPackageName + "-" + app.Spec.Version
	apv := new(v1alpha1.ApplicationPackageVersion)
	err := r.client.Get(ctx, types.NamespacedName{Name: apvName}, apv)
	if err != nil {
		return res, fmt.Errorf("get ApplicationPackageVersion for %s: %w", app.Name, err)
	}

	// get requirements from ApplicationPackageVersion and check it
	if apv.Status.Metadata != nil && apv.Status.Metadata.Requirements != nil {
		// TODO: check correct work of validation
		// checker, err := releaseUpdater.NewDeckhouseReleaseRequirementsChecker(r.client, []string{}, r.exts, nil, releaseUpdater.WithLogger(r.logger.Named("requirements_checker")))
		// if err != nil {
		// 	return res, fmt.Errorf("failed to create requirements checker: %w", err)
		// }

		_, err = r.exts.KubernetesVersion.ValidateBaseVersion(apv.Status.Metadata.Requirements.Kubernetes)
		if err != nil {
			app = r.SetConditionFalse(app, v1alpha1.ApplicationConditionRequirementsMet, "KubernetesVersionInvalid", err.Error())

			err = r.client.Status().Patch(ctx, app, client.MergeFrom(original))
			if err != nil {
				return res, fmt.Errorf("failed to patch application status: %w", err)
			}

			return res, fmt.Errorf("failed to validate Kubernetes version %s: %w", app.Name, err)
		}
		_, err = r.exts.DeckhouseVersion.ValidateBaseVersion(apv.Status.Metadata.Requirements.Deckhouse)
		if err != nil {
			app = r.SetConditionFalse(app, v1alpha1.ApplicationConditionRequirementsMet, "DeckhouseVersionInvalid", err.Error())

			err = r.client.Status().Patch(ctx, app, client.MergeFrom(original))
			if err != nil {
				return res, fmt.Errorf("failed to patch application status: %w", err)
			}

			return res, fmt.Errorf("failed to validate Deckhouse version %s: %w", app.Name, err)
		}
	}

	// if requirements ok - get PackageRegistry from spec.repository to get registry client

	// call PackageOperator method (maybe PackageAdder interface)

	return res, nil
}

func (r *reconciler) delete(_ context.Context, application *v1alpha1.Application) (ctrl.Result, error) {
	res := ctrl.Result{}

	logger := r.logger.With(slog.String("name", application.Name))
	logger.Debug("deleting Application")
	defer logger.Debug("delete Application complete")

	// TODO: implement application deletion logic
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
