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

package applicationpackageversion

import (
	"context"
	"fmt"
	"log/slog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	controllerName = "d8-application-package-version-controller"

	maxConcurrentReconciles = 1
)

type reconciler struct {
	client client.Client
	logger *log.Logger
}

func RegisterController(
	runtimeManager manager.Manager,
	logger *log.Logger,
) error {
	r := &reconciler{
		client: runtimeManager.GetClient(),
		logger: logger,
	}

	applicationPackageVersionController, err := controller.New(controllerName, runtimeManager, controller.Options{
		MaxConcurrentReconciles: maxConcurrentReconciles,
		Reconciler:              r,
	})
	if err != nil {
		return fmt.Errorf("create controller: %w", err)
	}

	return ctrl.NewControllerManagedBy(runtimeManager).
		For(&v1alpha1.ApplicationPackageVersion{}).
		Complete(applicationPackageVersionController)
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.logger.Debug("reconciling ApplicationPackageVersion", slog.String("name", req.Name))

	packageVersion := new(v1alpha1.ApplicationPackageVersion)
	if err := r.client.Get(ctx, req.NamespacedName, packageVersion); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Warn("application package version not found", slog.String("name", req.Name))
			return ctrl.Result{}, nil
		}
		r.logger.Error("failed to get application package version", slog.String("name", req.Name), log.Err(err))
		return ctrl.Result{Requeue: true}, nil
	}

	// handle delete event
	if !packageVersion.DeletionTimestamp.IsZero() {
		r.logger.Debug("deleting application package version", slog.String("name", req.Name))
		return r.delete(ctx, packageVersion)
	}

	// handle create/update events
	return r.handle(ctx, packageVersion)
}

func (r *reconciler) handle(_ context.Context, packageVersion *v1alpha1.ApplicationPackageVersion) (ctrl.Result, error) {
	// TODO: implement application package version reconciliation logic
	r.logger.Info("handling ApplicationPackageVersion", slog.String("name", packageVersion.Name))
	return ctrl.Result{}, nil
}

func (r *reconciler) delete(_ context.Context, packageVersion *v1alpha1.ApplicationPackageVersion) (ctrl.Result, error) {
	// TODO: implement application package version deletion logic
	r.logger.Info("deleting ApplicationPackageVersion", slog.String("name", packageVersion.Name))
	return ctrl.Result{}, nil
}
