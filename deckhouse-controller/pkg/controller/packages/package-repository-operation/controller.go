// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package packagerepositoryoperation

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metautils "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	registryService "github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry/service"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	controllerName = "d8-package-repository-operation-controller"

	maxConcurrentReconciles = 1

	// packageTypeLabel is a label on Docker images that indicates the package type
	packageTypeLabel = "io.deckhouse.package.type"

	// cleanupOldOperationsCount is the number of operations to keep for the same repository, older operations will be deleted
	cleanupOldOperationsCount = 10
)

type reconciler struct {
	client client.Client
	dc     dependency.Container
	psm    registryService.ServiceManagerInterface[registryService.PackagesService]
	logger *log.Logger
}

func RegisterController(runtimeManager manager.Manager, dc dependency.Container, logger *log.Logger) error {
	r := &reconciler{
		client: runtimeManager.GetClient(),
		dc:     dc,
		psm:    registryService.NewPackageServiceManager(logger.Named("packages_manager")),
		logger: logger,
	}

	packageRepositoryOperationController, err := controller.New(controllerName, runtimeManager, controller.Options{
		MaxConcurrentReconciles: maxConcurrentReconciles,
		Reconciler:              r,
	})
	if err != nil {
		return fmt.Errorf("create controller: %w", err)
	}

	return ctrl.NewControllerManagedBy(runtimeManager).
		For(&v1alpha1.PackageRepositoryOperation{}).
		Complete(packageRepositoryOperationController)
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.logger.With(slog.String("name", req.Name))

	logger.Debug("reconcile resource")

	op := new(v1alpha1.PackageRepositoryOperation)
	if err := r.client.Get(ctx, req.NamespacedName, op); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Debug("resource not found")

			return ctrl.Result{}, nil
		}

		logger.Warn("failed to get resource", log.Err(err))

		return ctrl.Result{}, err
	}

	// handle delete event
	if !op.DeletionTimestamp.IsZero() {
		logger.Debug("resource deleted")
		return ctrl.Result{}, nil
	}

	// ensure operation labels
	res, err := r.ensureOperationLabels(ctx, op)
	if err != nil {
		logger.Warn("failed to ensure operation labels", log.Err(err))

		return ctrl.Result{}, err
	}

	if res.Requeue {
		return res, nil
	}

	// handle create/update events
	res, err = r.handleCreateOrUpdate(ctx, op)
	if err != nil {
		logger.Warn("failed to handle application package version", log.Err(err))

		return ctrl.Result{}, err
	}

	return res, nil
}

// ensureOperationLabels ensures that operation has all required labels set correctly.
// It sets default trigger (manual), syncs operation type with spec, and adds repository label for filtering.
// Returns Requeue=true if labels were updated to allow fresh object retrieval in next reconciliation.
func (r *reconciler) ensureOperationLabels(ctx context.Context, op *v1alpha1.PackageRepositoryOperation) (ctrl.Result, error) {
	if len(op.Labels) == 0 {
		op.Labels = make(map[string]string)
	}

	var update bool
	original := op.DeepCopy()

	// Set default trigger to manual if not already set
	if _, ok := op.Labels[v1alpha1.PackagesRepositoryOperationLabelOperationTrigger]; !ok {
		update = true
		op.Labels[v1alpha1.PackagesRepositoryOperationLabelOperationTrigger] = v1alpha1.PackagesRepositoryTriggerManual
	}

	// Ensure operation type label matches spec (sync on every reconcile)
	if label, ok := op.Labels[v1alpha1.PackagesRepositoryOperationLabelOperationType]; !ok || label != op.Spec.Type {
		update = true
		op.Labels[v1alpha1.PackagesRepositoryOperationLabelOperationType] = op.Spec.Type
	}

	// Set repository label for efficient filtering/querying
	if _, ok := op.Labels[v1alpha1.PackagesRepositoryOperationLabelRepository]; !ok {
		update = true
		op.Labels[v1alpha1.PackagesRepositoryOperationLabelRepository] = op.Spec.PackageRepositoryName
	}

	// Ensure ownerReference to PackageRepository is set (for cascade deletion via GC).
	// Auto-created operations get this at creation time, manually created ones need enrichment.
	if !hasPackageRepositoryOwnerRef(op) {
		repo := new(v1alpha1.PackageRepository)
		if err := r.client.Get(ctx, client.ObjectKey{Name: op.Spec.PackageRepositoryName}, repo); err != nil {
			if !apierrors.IsNotFound(err) {
				return ctrl.Result{}, fmt.Errorf("get package repository for owner ref: %w", err)
			}
			// Repository not found - skip ownerRef, operation will be processed without it
		} else {
			update = true
			op.OwnerReferences = append(op.OwnerReferences, metav1.OwnerReference{
				APIVersion: v1alpha1.PackageRepositoryGVK.GroupVersion().String(),
				Kind:       v1alpha1.PackageRepositoryGVK.Kind,
				Name:       repo.Name,
				UID:        repo.UID,
				Controller: &[]bool{true}[0],
			})
		}
	}

	if update {
		if err := r.client.Patch(ctx, op, client.MergeFrom(original)); err != nil {
			return ctrl.Result{}, fmt.Errorf("patch operation labels: %w", err)
		}

		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

// hasPackageRepositoryOwnerRef checks if the operation already has an ownerReference to a PackageRepository.
func hasPackageRepositoryOwnerRef(op *v1alpha1.PackageRepositoryOperation) bool {
	for _, ref := range op.OwnerReferences {
		if ref.Kind == v1alpha1.PackageRepositoryGVK.Kind {
			return true
		}
	}
	return false
}

func (r *reconciler) handleCreateOrUpdate(ctx context.Context, op *v1alpha1.PackageRepositoryOperation) (ctrl.Result, error) {
	switch op.GetStateByCondition() {
	case "": // Initial state
		return r.handlePendingState(ctx, op)
	case v1alpha1.PackageRepositoryOperationReasonDiscover:
		return r.handleDiscoverState(ctx, op)
	case v1alpha1.PackageRepositoryOperationReasonProcessing:
		return r.handleProcessingState(ctx, op)
	case v1alpha1.PackageRepositoryOperationReasonCompleted:
		return r.handleCleanupState(ctx, op)
	default:
		r.logger.Warn("operation in unknown phase", slog.String("name", op.Name))

		return ctrl.Result{}, nil
	}
}

func (r *reconciler) handlePendingState(ctx context.Context, op *v1alpha1.PackageRepositoryOperation) (ctrl.Result, error) {
	r.logger.Debug("handle pending state", slog.String("name", op.Name))

	// Move to Pending phase
	original := op.DeepCopy()

	r.setCompletedConditionFalse(op, v1alpha1.PackageRepositoryOperationReasonDiscover, "")
	op.Status.StartTime = ptr.To(metav1.Now())

	if err := r.client.Status().Patch(ctx, op, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, fmt.Errorf("update operation status: %w", err)
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *reconciler) handleDiscoverState(ctx context.Context, op *v1alpha1.PackageRepositoryOperation) (ctrl.Result, error) {
	logger := r.logger.With(slog.String("name", op.Name))

	logger.Debug("handle discover state")

	original := op.DeepCopy()

	svc, err := NewOperationService(ctx, r.client, op.Spec.PackageRepositoryName, r.psm, r.logger)
	if err != nil {
		now := metav1.Now()
		op.Status.CompletionTime = &now

		r.setCompletedConditionTrue(op, v1alpha1.PackageRepositoryOperationReasonFailed, err.Error())

		if err := r.client.Status().Patch(ctx, op, client.MergeFrom(original)); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.updatePackageRepositoryCondition(ctx, op); err != nil {
			logger.Warn("failed to update package repository condition", log.Err(err))
		}

		logger.Warn("operation failed", log.Err(err))
		return ctrl.Result{}, nil
	}

	discovered, err := svc.DiscoverPackage(ctx)
	if err != nil {
		now := metav1.Now()
		op.Status.CompletionTime = &now

		r.setCompletedConditionTrue(op, v1alpha1.PackageRepositoryOperationReasonFailed, err.Error())

		if err := r.client.Status().Patch(ctx, op, client.MergeFrom(original)); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.updatePackageRepositoryCondition(ctx, op); err != nil {
			logger.Warn("failed to update package repository condition", log.Err(err))
		}

		logger.Warn("operation failed", log.Err(err))
		return ctrl.Result{}, nil
	}

	if op.Status.Packages == nil {
		op.Status.Packages = new(v1alpha1.PackageRepositoryOperationStatusPackages)
	}

	packages := make([]v1alpha1.PackageRepositoryOperationStatusDiscoveredPackage, 0, len(discovered.Packages))
	for _, pkg := range discovered.Packages {
		packages = append(packages, v1alpha1.PackageRepositoryOperationStatusDiscoveredPackage{
			Name: pkg.Name,
		})
	}

	op.Status.Packages.Discovered = packages
	op.Status.Packages.Total = len(discovered.Packages)
	op.Status.Packages.ProcessedOverall = 0

	r.setCompletedConditionFalse(op, v1alpha1.PackageRepositoryOperationReasonProcessing, "")

	if err = r.client.Status().Patch(ctx, op, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, fmt.Errorf("update operation status: %w", err)
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *reconciler) handleProcessingState(ctx context.Context, op *v1alpha1.PackageRepositoryOperation) (ctrl.Result, error) {
	logger := r.logger.With(slog.String("name", op.Name))

	logger.Debug("handle processing state")

	// no packages found
	if op.Status.Packages == nil {
		return ctrl.Result{}, nil
	}

	original := op.DeepCopy()

	svc, err := NewOperationService(ctx, r.client, op.Spec.PackageRepositoryName, r.psm, r.logger)
	if err != nil {
		now := metav1.Now()
		op.Status.CompletionTime = &now

		r.setCompletedConditionTrue(op, v1alpha1.PackageRepositoryOperationReasonFailed, err.Error())

		if err := r.client.Status().Patch(ctx, op, client.MergeFrom(original)); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.updatePackageRepositoryCondition(ctx, op); err != nil {
			logger.Warn("failed to update package repository condition", log.Err(err))
		}

		logger.Warn("operation failed", log.Err(err))
		return ctrl.Result{}, nil
	}

	// Check if all packages have been processed
	if len(op.Status.Packages.Discovered) == 0 {
		r.logger.Info("all packages processed", slog.Int("total", op.Status.Packages.Total))

		if err := svc.UpdateRepositoryStatus(ctx, op.Status.Packages.Processed); err != nil {
			logger.Warn("failed to update repository status", log.Err(err))
			// Continue with operation completion even if repository update fails
		}

		// All packages processed, mark as completed
		now := metav1.Now()
		op.Status.CompletionTime = &now

		r.setCompletedConditionTrue(op, v1alpha1.PackageRepositoryOperationConditionCompleted, "")

		if err := r.client.Status().Patch(ctx, op, client.MergeFrom(original)); err != nil {
			return ctrl.Result{}, fmt.Errorf("update operation status: %w", err)
		}

		if err := r.updatePackageRepositoryCondition(ctx, op); err != nil {
			logger.Warn("failed to update package repository condition", log.Err(err))
		}

		r.logger.Info("operation completed", slog.String("name", op.Name))

		return ctrl.Result{}, nil
	}

	return r.processNextPackage(ctx, op, svc)
}

// handleCleanupState is used to process operations in completed phase (cleanup old operations for the same repository)
func (r *reconciler) handleCleanupState(ctx context.Context, op *v1alpha1.PackageRepositoryOperation) (ctrl.Result, error) {
	logger := r.logger.With(slog.String("name", op.Name))

	logger.Debug("handle completed state")

	// List all operations for the same repository
	operations := new(v1alpha1.PackageRepositoryOperationList)
	err := r.client.List(ctx, operations, client.MatchingLabels{
		v1alpha1.PackagesRepositoryOperationLabelRepository: op.Spec.PackageRepositoryName,
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("list operations: %w", err)
	}

	logger.Debug("found operations for the same repository", slog.Int("count", len(operations.Items)))

	if len(operations.Items) <= cleanupOldOperationsCount {
		logger.Debug("not enough operations to delete")
		return ctrl.Result{}, nil
	}

	// sort operations by creation timestamp descending
	sort.Slice(operations.Items, func(i, j int) bool {
		return !operations.Items[i].CreationTimestamp.Before(&operations.Items[j].CreationTimestamp)
	})

	// delete all operations except the most recent
	for _, toDelete := range operations.Items[cleanupOldOperationsCount:] {
		logger.Debug("delete old operation", slog.String("name", op.Name))
		if err = r.client.Delete(ctx, &toDelete); err != nil {
			return ctrl.Result{}, fmt.Errorf("delete old operation: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

// setCompletedConditionTrue sets the condition Completed to True, clearing reason and message.
func (r *reconciler) setCompletedConditionTrue(op *v1alpha1.PackageRepositoryOperation, reason, message string) {
	metautils.SetStatusCondition(&op.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.PackageRepositoryOperationConditionCompleted,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: op.Generation,
		LastTransitionTime: metav1.NewTime(r.dc.GetClock().Now()),
	})
}

// setCompletedConditionFalse sets the condition Completed to False with a reason and message.
func (r *reconciler) setCompletedConditionFalse(op *v1alpha1.PackageRepositoryOperation, reason, message string) {
	metautils.SetStatusCondition(&op.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.PackageRepositoryOperationConditionCompleted,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: op.Generation,
		LastTransitionTime: metav1.NewTime(r.dc.GetClock().Now()),
	})
}

func (r *reconciler) updatePackageRepositoryCondition(ctx context.Context, op *v1alpha1.PackageRepositoryOperation) error {
	repo := new(v1alpha1.PackageRepository)
	if err := r.client.Get(ctx, client.ObjectKey{Name: op.Spec.PackageRepositoryName}, repo); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("get package repository: %w", err)
	}

	cond := metautils.FindStatusCondition(op.Status.Conditions, v1alpha1.PackageRepositoryOperationConditionCompleted)
	if cond == nil {
		return nil
	}

	status := metav1.ConditionTrue
	if cond.Reason == v1alpha1.PackageRepositoryOperationReasonFailed {
		status = metav1.ConditionFalse
	}

	original := repo.DeepCopy()

	metautils.SetStatusCondition(&repo.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.PackageRepositoryConditionLastScanSucceeded,
		Status:             status,
		Reason:             cond.Reason,
		Message:            cond.Message,
		ObservedGeneration: repo.Generation,
		LastTransitionTime: metav1.NewTime(r.dc.GetClock().Now()),
	})

	if err := r.client.Status().Patch(ctx, repo, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("update package repository status: %w", err)
	}

	return nil
}
