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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	r.logger.Debug("reconciling Application", slog.String("name", req.Name), slog.String("namespace", req.Namespace))

	application := new(v1alpha1.Application)
	if err := r.client.Get(ctx, req.NamespacedName, application); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Warn("application not found", slog.String("name", req.Name), slog.String("namespace", req.Namespace))
			return ctrl.Result{}, nil
		}
		r.logger.Error("failed to get application", slog.String("name", req.Name), slog.String("namespace", req.Namespace), log.Err(err))
		return ctrl.Result{Requeue: true}, nil
	}

	// handle delete event
	if !application.DeletionTimestamp.IsZero() {
		r.logger.Debug("deleting application", slog.String("name", req.Name), slog.String("namespace", req.Namespace))
		return r.delete(ctx, application)
	}

	// handle create/update events
	return r.handle(ctx, application)
}

func (r *reconciler) handle(ctx context.Context, application *v1alpha1.Application) (ctrl.Result, error) {
	res := ctrl.Result{}

	r.logger.Info("handling Application", slog.String("name", application.Name), slog.String("namespace", application.Namespace))

	// from spec.packageName and spec.version find ApplicationPackageVersion
	apvName := application.Spec.ApplicationPackageName + "-" + application.Spec.Version
	var apv *v1alpha1.ApplicationPackageVersion
	err := r.client.Get(ctx, types.NamespacedName{Name: apvName}, apv)
	if err != nil || apv == nil {
		return res, fmt.Errorf("get ApplicationPackageVersion for %s: %w", application.Name, err)
	}

	// from ApplicationPackageVersion get requirements and check it
	// if requirements exists
	if apv.Status.Metadata != nil && apv.Status.Metadata.Requirements != nil {
		// TODO: check validation
		r.exts.KubernetesVersion.ValidateBaseVersion(apv.Status.Metadata.Requirements.Kubernetes)
		r.exts.DeckhouseVersion.ValidateBaseVersion(apv.Status.Metadata.Requirements.Deckhouse)
	}

	// if requirements ok - get PackageRegistry from spec.repository to get registry client

	// call PackageOperator method (maybe PackageAdder interface)

	return res, nil
}

func (r *reconciler) delete(_ context.Context, application *v1alpha1.Application) (ctrl.Result, error) {
	// TODO: implement application deletion logic
	r.logger.Info("deleting Application", slog.String("name", application.Name), slog.String("namespace", application.Namespace))
	return ctrl.Result{}, nil
}
