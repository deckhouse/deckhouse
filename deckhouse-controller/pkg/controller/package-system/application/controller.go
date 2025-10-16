// Copyright 2024 Flant JSC
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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	controllerName = "d8-application-controller"

	maxConcurrentReconciles = 3
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
	// TODO: implement application reconciliation logic
	r.logger.Debug("handling Application", slog.String("name", application.Name), slog.String("namespace", application.Namespace))
	return ctrl.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, application *v1alpha1.Application) (ctrl.Result, error) {
	// TODO: implement application deletion logic
	r.logger.Debug("deleting Application", slog.String("name", application.Name), slog.String("namespace", application.Namespace))
	return ctrl.Result{}, nil
}
