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

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	controllerName = "d8-package-repository-controller"

	maxConcurrentReconciles = 1

	// requeueInterval is the interval at which the controller will requeue the PackageRepository
	// after successful reconciliation to trigger periodic scanning
	// TODO: switch to 6h before merging
	requeueInterval = 2 * time.Minute

	// operationLabelRepository is the label used to identify PackageRepositoryOperations
	// that belong to a specific PackageRepository
	operationLabelRepository = "packages.deckhouse.io/repository"
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
	r.logger.Debug("reconciling PackageRepository", slog.String("name", req.Name))

	packageRepository := new(v1alpha1.PackageRepository)
	if err := r.client.Get(ctx, req.NamespacedName, packageRepository); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Warn("package repository not found", slog.String("name", req.Name))
			return ctrl.Result{}, nil
		}
		r.logger.Error("failed to get package repository", slog.String("name", req.Name), log.Err(err))
		return ctrl.Result{Requeue: true}, nil
	}

	// handle delete event
	if !packageRepository.DeletionTimestamp.IsZero() {
		r.logger.Debug("deleting package repository", slog.String("name", req.Name))
		return r.delete(ctx, packageRepository)
	}

	// handle create/update events
	return r.handle(ctx, packageRepository)
}

func (r *reconciler) handle(ctx context.Context, packageRepository *v1alpha1.PackageRepository) (ctrl.Result, error) {
	r.logger.Debug("handling PackageRepository", slog.String("name", packageRepository.Name))

	// Check if there are any existing PackageRepositoryOperations for this repository
	operationList := &v1alpha1.PackageRepositoryOperationList{}
	err := r.client.List(ctx, operationList, client.MatchingLabels{
		operationLabelRepository: packageRepository.Name,
	})
	if err != nil {
		r.logger.Error("failed to list package repository operations",
			slog.String("name", packageRepository.Name),
			log.Err(err))
		return ctrl.Result{}, err
	}

	// Check if there is an active operation (Pending or Processing)
	hasActiveOperation := false
	for _, op := range operationList.Items {
		if op.Status.Phase == "" ||
			op.Status.Phase == v1alpha1.PackageRepositoryOperationPhasePending ||
			op.Status.Phase == v1alpha1.PackageRepositoryOperationPhaseProcessing {
			hasActiveOperation = true
			r.logger.Debug("active operation exists, skipping creation",
				slog.String("operation", op.Name),
				slog.String("phase", op.Status.Phase))
			break
		}
	}

	// Only create a new operation if there is no active operation
	if hasActiveOperation {
		r.logger.Debug("skipping operation creation, active operation in progress",
			slog.String("repository", packageRepository.Name))
		// Requeue to check again later
		return ctrl.Result{RequeueAfter: requeueInterval}, nil
	}

	// Determine if we should do a full scan or incremental scan
	// fullScan = true if this is the first operation ever (no operations at all)
	fullScan := len(operationList.Items) == 0

	// Create a new PackageRepositoryOperation
	operationName := fmt.Sprintf("%s-scan-%d", packageRepository.Name, time.Now().Unix())
	operation := &v1alpha1.PackageRepositoryOperation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.PackageRepositoryOperationGVK.GroupVersion().String(),
			Kind:       v1alpha1.PackageRepositoryOperationKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: operationName,
			Labels: map[string]string{
				operationLabelRepository: packageRepository.Name,
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
			Type:              v1alpha1.PackageRepositoryOperationTypeScan,
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
			r.logger.Debug("operation already exists, skipping creation",
				slog.String("operation", operationName))
			return ctrl.Result{RequeueAfter: requeueInterval}, nil
		}
		r.logger.Error("failed to create package repository operation",
			slog.String("name", packageRepository.Name),
			slog.String("operation", operationName),
			log.Err(err))
		return ctrl.Result{}, err
	}

	r.logger.Info("created package repository operation",
		slog.String("repository", packageRepository.Name),
		slog.String("operation", operationName),
		slog.Bool("fullScan", fullScan))

	// Requeue after requeueInterval to trigger the next scan
	return ctrl.Result{RequeueAfter: requeueInterval}, nil
}

func (r *reconciler) delete(ctx context.Context, packageRepository *v1alpha1.PackageRepository) (ctrl.Result, error) {
	r.logger.Info("deleting PackageRepository", slog.String("name", packageRepository.Name))

	// Delete all PackageRepositoryOperations associated with this repository
	operationList := &v1alpha1.PackageRepositoryOperationList{}
	err := r.client.List(ctx, operationList, client.MatchingLabels{
		operationLabelRepository: packageRepository.Name,
	})
	if err != nil {
		r.logger.Error("failed to list operations for deletion", log.Err(err))
		return ctrl.Result{}, err
	}

	for _, op := range operationList.Items {
		if err := r.client.Delete(ctx, &op); err != nil && !apierrors.IsNotFound(err) {
			r.logger.Error("failed to delete operation",
				slog.String("operation", op.Name),
				log.Err(err))
			return ctrl.Result{}, err
		}
	}

	r.logger.Info("cleanup completed", slog.String("repository", packageRepository.Name))
	return ctrl.Result{}, nil
}
