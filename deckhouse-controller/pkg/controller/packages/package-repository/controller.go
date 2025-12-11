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

package packagerepository

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	controllerName = "d8-package-repository-controller"

	maxConcurrentReconciles = 1

	// requeueInterval is the interval at which the controller will requeue the PackageRepository
	// after successful reconciliation to trigger periodic scanning
	requeueInterval = 6 * time.Hour
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

	packageRepositoryController, err := controller.New(controllerName, runtimeManager, controller.Options{
		MaxConcurrentReconciles: maxConcurrentReconciles,
		Reconciler:              r,
	})
	if err != nil {
		return fmt.Errorf("create controller: %w", err)
	}

	return ctrl.NewControllerManagedBy(runtimeManager).
		For(&v1alpha1.PackageRepository{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(packageRepositoryController)
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	res := ctrl.Result{}

	logger := r.logger.With(slog.String("name", req.Name))

	logger.Debug("reconciling PackageRepository")

	packageRepository := new(v1alpha1.PackageRepository)
	if err := r.client.Get(ctx, req.NamespacedName, packageRepository); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Debug("package repository not found")

			return res, nil
		}

		logger.Warn("failed to get package repository", log.Err(err))

		return res, err
	}

	// handle delete event
	if !packageRepository.DeletionTimestamp.IsZero() {
		logger.Debug("deleting package repository")

		err := r.delete(ctx, packageRepository)
		if err != nil {
			logger.Warn("failed to delete package repository", log.Err(err))

			return res, err
		}

		return res, nil
	}

	// handle create/update events
	res, err := r.handle(ctx, packageRepository)
	if err != nil {
		logger.Warn("failed to handle package repository", log.Err(err))

		return res, err
	}

	return res, nil
}

func (r *reconciler) handle(ctx context.Context, packageRepository *v1alpha1.PackageRepository) (ctrl.Result, error) {
	logger := r.logger.With(slog.String("name", packageRepository.Name))

	logger.Debug("handling PackageRepository")

	// Check if there are any existing PackageRepositoryOperations for this repository
	operationList := &v1alpha1.PackageRepositoryOperationList{}
	err := r.client.List(ctx, operationList, client.MatchingLabels{
		v1alpha1.PackagesRepositoryOperationLabelRepository: packageRepository.Name,
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("list operations: %w", err)
	}

	// Check if there is an active operation (Pending or Processing)
	hasActiveOperation := false
	for _, op := range operationList.Items {
		if op.Status.Phase == "" ||
			op.Status.Phase == v1alpha1.PackageRepositoryOperationPhasePending ||
			op.Status.Phase == v1alpha1.PackageRepositoryOperationPhaseProcessing {
			hasActiveOperation = true

			logger.Debug("active operation exists, skipping creation",
				slog.String("operation", op.Name),
				slog.String("phase", op.Status.Phase))

			break
		}
	}

	// Only create a new operation if there is no active operation
	if hasActiveOperation {
		logger.Debug("skipping operation creation, active operation in progress")

		// Requeue to check again later
		return ctrl.Result{RequeueAfter: requeueInterval}, nil
	}

	// Determine if we should do a full scan or incremental scan
	// fullScan = true if this is the first operation ever (no operations at all)
	fullScan := len(operationList.Items) == 0

	// Create a new PackageRepositoryOperation
	operationName := fmt.Sprintf("%s-scan-%d", packageRepository.Name, r.dc.GetClock().Now().Unix())

	logger.With(slog.String("operation", operationName), slog.Bool("full_scan", fullScan))

	operation := &v1alpha1.PackageRepositoryOperation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.PackageRepositoryOperationGVK.GroupVersion().String(),
			Kind:       v1alpha1.PackageRepositoryOperationKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: operationName,
			Labels: map[string]string{
				v1alpha1.PackagesRepositoryOperationLabelRepository:       packageRepository.Name,
				v1alpha1.PackagesRepositoryOperationLabelOperationTrigger: v1alpha1.PackagesRepositoryTriggerAuto,
				v1alpha1.PackagesRepositoryOperationLabelOperationType:    v1alpha1.PackageRepositoryOperationTypeUpdate,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: v1alpha1.PackageRepositoryGVK.GroupVersion().String(),
					Kind:       v1alpha1.PackageRepositoryGVK.Kind,
					Name:       packageRepository.Name,
					UID:        packageRepository.UID,
					Controller: &[]bool{true}[0],
				},
			},
		},
		Spec: v1alpha1.PackageRepositoryOperationSpec{
			PackageRepository: packageRepository.Name,
			Type:              v1alpha1.PackageRepositoryOperationTypeUpdate,
			Update: &v1alpha1.PackageRepositoryOperationUpdate{
				FullScan: fullScan,
				Timeout:  "5m",
			},
		},
	}

	err = r.client.Create(ctx, operation)
	if err != nil {
		// If operation already exists (race condition), that's fine - just requeue
		if apierrors.IsAlreadyExists(err) {
			logger.Debug("operation already exists, skipping creation")

			return ctrl.Result{RequeueAfter: requeueInterval}, nil
		}

		return ctrl.Result{}, fmt.Errorf("create operation %s: %w", operationName, err)
	}

	logger.Info("created package repository operation")

	// Requeue after requeueInterval to trigger the next scan
	return ctrl.Result{RequeueAfter: requeueInterval}, nil
}

func (r *reconciler) delete(ctx context.Context, packageRepository *v1alpha1.PackageRepository) error {
	logger := r.logger.With("name", packageRepository.Name)

	logger.Info("deleting PackageRepository")

	// Delete all PackageRepositoryOperations associated with this repository
	operationList := &v1alpha1.PackageRepositoryOperationList{}
	err := r.client.List(ctx, operationList, client.MatchingLabels{
		v1alpha1.PackagesRepositoryOperationLabelRepository: packageRepository.Name,
	})
	if err != nil {
		return fmt.Errorf("list operations: %w", err)
	}

	for _, op := range operationList.Items {
		if err := r.client.Delete(ctx, &op); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete operation %s: %w", op.Name, err)
		}
	}

	logger.Info("cleanup completed")

	return nil
}
