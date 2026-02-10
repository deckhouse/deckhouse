/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package modulepackage

import (
	"context"
	"log/slog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type reconciler struct {
	client client.Client
	dc     dependency.Container
	logger *log.Logger
}

func RegisterController(
	runtimeManager manager.Manager,
	dc dependency.Container,
	logger *log.Logger,
) error {
	r := &reconciler{
		client: runtimeManager.GetClient(),
		dc:     dc,
		logger: logger,
	}

	return ctrl.NewControllerManagedBy(runtimeManager).
		For(&v1alpha1.ModulePackage{}).
		Complete(r)
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.logger.With(slog.String("name", req.Name))

	logger.Info("reconcile module package")

	mp := new(v1alpha1.ModulePackage)
	if err := r.client.Get(ctx, req.NamespacedName, mp); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Debug("module package not found")

			return ctrl.Result{}, nil
		}

		logger.Warn("failed to get module package", log.Err(err))

		return ctrl.Result{}, err
	}

	// handle delete event
	if !mp.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// handle create/update events
	if err := r.handleCreateOrUpdate(ctx, logger, mp); err != nil {
		logger.Warn("failed to handle module package", log.Err(err))

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *reconciler) handleCreateOrUpdate(_ context.Context, logger *log.Logger, _ *v1alpha1.ModulePackage) error { //nolint:unparam
	logger.Debug("handle create or update module package")

	return nil
}
