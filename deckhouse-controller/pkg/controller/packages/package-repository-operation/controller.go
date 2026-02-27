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

// nolint: unused
package packagerepositoryoperation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

func RegisterController(
	runtimeManager manager.Manager,
	dc dependency.Container,
	logger *log.Logger,
) error {
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
	res := ctrl.Result{}

	logger := r.logger.With(slog.String("name", req.Name))

	logger.Debug("reconciling PackageRepositoryOperation")

	operation := new(v1alpha1.PackageRepositoryOperation)
	if err := r.client.Get(ctx, req.NamespacedName, operation); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Debug("package repository operation not found")

			return res, nil
		}

		logger.Warn("failed to get package repository operation", log.Err(err))

		return res, err
	}

	// handle delete event
	if !operation.DeletionTimestamp.IsZero() {
		logger.Debug("deleting package repository operation")

		err := r.delete(ctx, operation)
		if err != nil {
			logger.Warn("failed to delete package repository operation", log.Err(err))

			return res, err
		}

		return res, nil
	}

	// ensure operation labels
	res, err := r.ensureOperationLabels(ctx, operation)
	if err != nil {
		logger.Warn("failed to ensure operation trigger label", log.Err(err))

		return res, err
	}

	if res.Requeue {
		return res, nil
	}

	// handle create/update events - state machine
	res, err = r.handle(ctx, operation)
	if err != nil {
		logger.Warn("failed to handle package repository operation", log.Err(err))

		return res, err
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

	if update {
		if err := r.client.Patch(ctx, op, client.MergeFrom(original)); err != nil {
			return ctrl.Result{}, fmt.Errorf("patch operation labels: %w", err)
		}

		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

func (r *reconciler) handle(ctx context.Context, operation *v1alpha1.PackageRepositoryOperation) (ctrl.Result, error) {
	var res ctrl.Result
	var err error

	// State machine based on phase
	switch operation.Status.Phase {
	case "": // Initial state
		res, err = r.handleInitialState(ctx, operation)
	case v1alpha1.PackageRepositoryOperationPhasePending:
		res, err = r.handlePendingState(ctx, operation)
	case v1alpha1.PackageRepositoryOperationPhaseDiscover:
		res, err = r.handleDiscoverState(ctx, operation)
	case v1alpha1.PackageRepositoryOperationPhaseProcessing:
		res, err = r.handleProcessingState(ctx, operation)
	case v1alpha1.PackageRepositoryOperationPhaseCompleted:
		err = r.handleCompletedState(ctx, operation)
	default:
		r.logger.Warn("unknown phase", slog.String("phase", operation.Status.Phase))

		return ctrl.Result{}, nil
	}

	if err != nil {
		return res, fmt.Errorf("handle %s state: %w", operation.Status.Phase, err)
	}

	return res, nil
}

func (r *reconciler) handleInitialState(ctx context.Context, operation *v1alpha1.PackageRepositoryOperation) (ctrl.Result, error) {
	r.logger.Debug("handling initial state", slog.String("name", operation.Name))

	// Move to Pending phase
	original := operation.DeepCopy()

	operation.Status.Phase = v1alpha1.PackageRepositoryOperationPhasePending
	now := metav1.Now()
	operation.Status.StartTime = &now

	if err := r.client.Status().Patch(ctx, operation, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, fmt.Errorf("update operation status: %w", err)
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *reconciler) handlePendingState(ctx context.Context, operation *v1alpha1.PackageRepositoryOperation) (ctrl.Result, error) {
	r.logger.Debug("handling pending state", slog.String("name", operation.Name))

	// Move to Processing phase
	original := operation.DeepCopy()

	operation.Status.Phase = v1alpha1.PackageRepositoryOperationPhaseDiscover

	if err := r.client.Status().Patch(ctx, operation, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, fmt.Errorf("update operation status: %w", err)
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *reconciler) handleDiscoverState(ctx context.Context, operation *v1alpha1.PackageRepositoryOperation) (ctrl.Result, error) {
	res := ctrl.Result{}

	logger := r.logger.With(slog.String("name", operation.Name))

	logger.Debug("handling discover state")

	opService, err := NewOperationService(ctx, r.client, operation.Spec.PackageRepositoryName, r.psm, r.logger)
	if err != nil {
		// Handle specific error cases with status updates
		original := operation.DeepCopy()
		now := metav1.Now()
		operation.Status.CompletionTime = &now
		operation.Status.Phase = v1alpha1.PackageRepositoryOperationPhaseProcessing

		var reason, message string
		// Check if the underlying error is NotFound (works with wrapped errors)
		switch {
		case apierrors.IsNotFound(err):
			reason = v1alpha1.PackageRepositoryOperationReasonPackageRepositoryNotFound
			// Extract the root cause error for cleaner message
			var statusErr *apierrors.StatusError
			if errors.As(err, &statusErr) {
				message = fmt.Sprintf("PackageRepository not found: %v", statusErr)
			} else {
				message = fmt.Sprintf("PackageRepository not found: %v", err)
			}
		case strings.Contains(err.Error(), "create package service"):
			reason = v1alpha1.PackageRepositoryOperationReasonRegistryClientCreationFailed
			message = fmt.Sprintf("Failed to create registry client: %v", err)
		default:
			reason = v1alpha1.PackageRepositoryOperationReasonPackageRepositoryNotFound
			message = fmt.Sprintf("Failed to create operation service: %v", err)
		}

		r.SetConditionFalse(
			operation,
			v1alpha1.PackageRepositoryOperationConditionCompleted,
			reason,
			message,
		)

		if patchErr := r.client.Status().Patch(ctx, operation, client.MergeFrom(original)); patchErr != nil {
			return ctrl.Result{}, patchErr
		}

		if updateErr := r.updatePackageRepositoryCondition(ctx, operation.Spec.PackageRepositoryName, false, reason, message); updateErr != nil {
			logger.Warn("failed to update package repository condition", log.Err(updateErr))
		}

		logger.Warn("operation failed", slog.String("message", message))
		return ctrl.Result{}, nil
	}

	discovered, err := opService.DiscoverPackage(ctx)
	if err != nil {
		// Handle package listing failure
		original := operation.DeepCopy()
		now := metav1.Now()
		operation.Status.CompletionTime = &now
		operation.Status.Phase = v1alpha1.PackageRepositoryOperationPhaseProcessing
		message := fmt.Sprintf("Failed to list packages: %v", err)

		r.SetConditionFalse(
			operation,
			v1alpha1.PackageRepositoryOperationConditionCompleted,
			v1alpha1.PackageRepositoryOperationReasonPackageListingFailed,
			message,
		)

		if patchErr := r.client.Status().Patch(ctx, operation, client.MergeFrom(original)); patchErr != nil {
			return ctrl.Result{}, patchErr
		}

		if updateErr := r.updatePackageRepositoryCondition(ctx, operation.Spec.PackageRepositoryName, false, v1alpha1.PackageRepositoryOperationReasonPackageListingFailed, message); updateErr != nil {
			logger.Warn("failed to update package repository condition", log.Err(updateErr))
		}

		logger.Warn("operation failed", slog.String("message", message))
		return ctrl.Result{}, nil
	}

	// Handle discovered packages
	err = r.handleOperationDiscoverResult(ctx, operation, discovered)
	if err != nil {
		return res, fmt.Errorf("handle operation discover result: %w", err)
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *reconciler) handleProcessingState(ctx context.Context, operation *v1alpha1.PackageRepositoryOperation) (ctrl.Result, error) {
	res := ctrl.Result{}

	logger := r.logger.With(slog.String("name", operation.Name))

	logger.Debug("handling processing state")

	// Check if operation already has a failed condition - skip processing if so
	for _, cond := range operation.Status.Conditions {
		if cond.Type == v1alpha1.PackageRepositoryOperationConditionCompleted && cond.Status == corev1.ConditionFalse {
			logger.Debug("operation already has failed condition, skipping processing")
			return res, nil
		}
	}

	opService, err := NewOperationService(ctx, r.client, operation.Spec.PackageRepositoryName, r.psm, r.logger)
	if err != nil {
		// Handle specific error cases with status updates
		original := operation.DeepCopy()
		now := metav1.Now()
		operation.Status.CompletionTime = &now

		var reason, message string
		// Check if the underlying error is NotFound (works with wrapped errors)
		switch {
		case apierrors.IsNotFound(err):
			reason = v1alpha1.PackageRepositoryOperationReasonPackageRepositoryNotFound
			// Extract the root cause error for cleaner message
			var statusErr *apierrors.StatusError
			if errors.As(err, &statusErr) {
				message = fmt.Sprintf("PackageRepository not found: %v", statusErr)
			} else {
				message = fmt.Sprintf("PackageRepository not found: %v", err)
			}
		case strings.Contains(err.Error(), "create package service"):
			reason = v1alpha1.PackageRepositoryOperationReasonRegistryClientCreationFailed
			message = fmt.Sprintf("Failed to create registry client: %v", err)
		default:
			reason = v1alpha1.PackageRepositoryOperationReasonPackageRepositoryNotFound
			message = fmt.Sprintf("Failed to create operation service: %v", err)
		}

		r.SetConditionFalse(
			operation,
			v1alpha1.PackageRepositoryOperationConditionCompleted,
			reason,
			message,
		)

		if patchErr := r.client.Status().Patch(ctx, operation, client.MergeFrom(original)); patchErr != nil {
			return ctrl.Result{}, patchErr
		}

		if updateErr := r.updatePackageRepositoryCondition(ctx, operation.Spec.PackageRepositoryName, false, reason, message); updateErr != nil {
			logger.Warn("failed to update package repository condition", log.Err(updateErr))
		}

		logger.Warn("operation failed", slog.String("message", message))
		return ctrl.Result{}, nil
	}

	// Check if all packages have been processed
	if operation.Status.Packages != nil && len(operation.Status.Packages.Discovered) == 0 {
		r.logger.Info("all packages processed, marking as completed",
			slog.Int("total", operation.Status.Packages.Total))

		if err := opService.UpdateRepositoryStatus(ctx, operation.Status.Packages.Processed); err != nil {
			logger.Warn("failed to update repository status", log.Err(err))
			// Continue with operation completion even if repository update fails
		}

		original := operation.DeepCopy()

		// All packages processed, mark as completed
		operation.Status.Phase = v1alpha1.PackageRepositoryOperationPhaseCompleted
		now := metav1.Now()
		operation.Status.CompletionTime = &now

		r.SetConditionTrue(
			operation,
			v1alpha1.PackageRepositoryOperationConditionCompleted,
		)

		if err := r.client.Status().Patch(ctx, operation, client.MergeFrom(original)); err != nil {
			return ctrl.Result{}, fmt.Errorf("update operation status: %w", err)
		}

		successMessage := fmt.Sprintf("Successfully scanned repository, found %d package(s)", operation.Status.Packages.Total)
		if updateErr := r.updatePackageRepositoryCondition(ctx, operation.Spec.PackageRepositoryName, true, "", successMessage); updateErr != nil {
			logger.Warn("failed to update package repository condition", log.Err(updateErr))
		}

		r.logger.Info("operation completed", slog.String("name", operation.Name))

		return ctrl.Result{}, nil
	}

	return r.processNextPackage(ctx, operation, opService)
}

func (r *reconciler) handleOperationDiscoverResult(ctx context.Context, operation *v1alpha1.PackageRepositoryOperation, discovered *DiscoverResult) error {
	// Update operation status with discovered packages
	original := operation.DeepCopy()

	// Initialize Packages if nil
	if operation.Status.Packages == nil {
		operation.Status.Packages = &v1alpha1.PackageRepositoryOperationStatusPackages{}
	}

	operationStatusPackages := make([]v1alpha1.PackageRepositoryOperationStatusDiscoveredPackage, 0, len(discovered.Packages))

	for _, pkg := range discovered.Packages {
		queueItem := v1alpha1.PackageRepositoryOperationStatusDiscoveredPackage{
			Name: pkg.Name,
		}

		operationStatusPackages = append(operationStatusPackages, queueItem)
	}

	operation.Status.Packages.Discovered = operationStatusPackages
	operation.Status.Packages.Total = len(discovered.Packages)
	operation.Status.Packages.ProcessedOverall = 0
	operation.Status.Phase = v1alpha1.PackageRepositoryOperationPhaseProcessing

	if err := r.client.Status().Patch(ctx, operation, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("update operation status: %w", err)
	}

	return nil
}

func (r *reconciler) processNextPackage(ctx context.Context, operation *v1alpha1.PackageRepositoryOperation, svc *OperationService) (ctrl.Result, error) {
	// Get first package from queue
	currentPackage := operation.Status.Packages.Discovered[0]
	r.logger.Info("processing package",
		slog.String("package", currentPackage.Name))

	processResult, err := svc.ProcessPackageVersions(ctx, currentPackage.Name, operation)
	if err != nil {
		r.logger.Error("failed to process package versions",
			slog.String("package", currentPackage.Name),
			log.Err(err))
	}

	// Processing failed entirely — record error and move to next package
	if processResult == nil {
		return r.dequeuePackageWithError(ctx, operation, currentPackage.Name, err)
	}

	// Ensure the appropriate package resource based on detected type.
	// Skip resource creation for unrecognized packages (e.g. legacy modules without metadata).
	switch processResult.PackageType {
	case packageTypeModule:
		if ensureErr := svc.EnsureModulePackage(ctx, currentPackage.Name); ensureErr != nil {
			r.logger.Error("failed to ensure module package resource",
				slog.String("package", currentPackage.Name),
				log.Err(ensureErr))
		}
	case packageTypeApplication:
		if ensureErr := svc.EnsureApplicationPackage(ctx, currentPackage.Name); ensureErr != nil {
			r.logger.Error("failed to ensure application package resource",
				slog.String("package", currentPackage.Name),
				log.Err(ensureErr))
		}
	}

	return r.dequeuePackageWithResult(ctx, operation, currentPackage.Name, processResult)
}

func (r *reconciler) dequeuePackageWithError(ctx context.Context, operation *v1alpha1.PackageRepositoryOperation, packageName string, processErr error) (ctrl.Result, error) {
	original := operation.DeepCopy()

	if len(operation.Status.Packages.Discovered) > 0 {
		operation.Status.Packages.Discovered = operation.Status.Packages.Discovered[1:]
	}
	if operation.Status.Packages != nil {
		operation.Status.Packages.ProcessedOverall++
	}

	operation.Status.Packages.Failed = append(operation.Status.Packages.Failed, v1alpha1.PackageRepositoryOperationStatusFailedPackage{
		Name: packageName,
		Errors: []v1alpha1.PackageRepositoryOperationStatusFailedPackageError{
			{Error: processErr.Error()},
		},
	})

	if err := r.client.Status().Patch(ctx, operation, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, fmt.Errorf("update operation status: %w", err)
	}
	return ctrl.Result{Requeue: true}, nil
}

func (r *reconciler) dequeuePackageWithResult(ctx context.Context, operation *v1alpha1.PackageRepositoryOperation, packageName string, result *PackageProcessResult) (ctrl.Result, error) {
	original := operation.DeepCopy()

	if len(operation.Status.Packages.Discovered) > 0 {
		operation.Status.Packages.Discovered = operation.Status.Packages.Discovered[1:]
	}
	if operation.Status.Packages != nil {
		operation.Status.Packages.ProcessedOverall++
	}

	operation.Status.Packages.Processed = append(operation.Status.Packages.Processed, v1alpha1.PackageRepositoryOperationStatusPackage{
		Name: packageName,
		Type: string(result.PackageType),
	})

	failedList := make([]v1alpha1.PackageRepositoryOperationStatusFailedPackageError, 0, len(result.Failed))
	for _, fv := range result.Failed {
		failedList = append(failedList, v1alpha1.PackageRepositoryOperationStatusFailedPackageError{
			Version: fv.Name,
			Error:   fv.Error,
		})
	}
	if len(failedList) > 0 {
		operation.Status.Packages.Failed = append(operation.Status.Packages.Failed, v1alpha1.PackageRepositoryOperationStatusFailedPackage{
			Name:   packageName,
			Errors: failedList,
		})
	}

	if err := r.client.Status().Patch(ctx, operation, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, fmt.Errorf("update operation status: %w", err)
	}
	return ctrl.Result{Requeue: true}, nil
}

// handleCompletedState is used to process operations in completed phase (cleanup old operations for the same repository)
func (r *reconciler) handleCompletedState(ctx context.Context, operation *v1alpha1.PackageRepositoryOperation) error {
	logger := r.logger.With(slog.String("name", operation.Name))
	logger.Debug("handling completed state")
	defer logger.Debug("handling completed state complete")

	// List all operations for the same repository
	operations := new(v1alpha1.PackageRepositoryOperationList)
	err := r.client.List(ctx, operations, client.MatchingLabels{
		v1alpha1.PackagesRepositoryOperationLabelRepository: operation.Spec.PackageRepositoryName,
	})
	if err != nil {
		return fmt.Errorf("list operations: %w", err)
	}

	logger.Debug("found operations for the same repository", slog.Int("count", len(operations.Items)))

	if len(operations.Items) <= cleanupOldOperationsCount {
		logger.Debug("not enough operations to delete")
		return nil
	}

	// sort operations by creation timestamp descending
	sort.Slice(operations.Items, func(i, j int) bool {
		return !operations.Items[i].CreationTimestamp.Before(&operations.Items[j].CreationTimestamp)
	})

	// delete all operations except the most recent
	for _, op := range operations.Items[cleanupOldOperationsCount:] {
		logger.Debug("deleting old operation", slog.String("name", op.Name))
		if err := r.client.Delete(ctx, &op); err != nil {
			return fmt.Errorf("delete old operation: %w", err)
		}
	}

	return nil
}

func (r *reconciler) delete(ctx context.Context, operation *v1alpha1.PackageRepositoryOperation) error {
	r.logger.Info("deleting PackageRepositoryOperation", slog.String("name", operation.Name))

	// Remove finalizer if present
	if controllerutil.ContainsFinalizer(operation, "packages.deckhouse.io/finalizer") {
		original := operation.DeepCopy()

		controllerutil.RemoveFinalizer(operation, "packages.deckhouse.io/finalizer")

		if err := r.client.Patch(ctx, operation, client.MergeFrom(original)); err != nil {
			return err
		}
	}

	return nil
}

func (r *reconciler) SetConditionTrue(operation *v1alpha1.PackageRepositoryOperation, condType string) *v1alpha1.PackageRepositoryOperation {
	time := metav1.NewTime(r.dc.GetClock().Now())

	for idx, cond := range operation.Status.Conditions {
		if cond.Type == condType {
			operation.Status.Conditions[idx].LastProbeTime = time
			if cond.Status != corev1.ConditionTrue {
				operation.Status.Conditions[idx].LastTransitionTime = time
				operation.Status.Conditions[idx].Status = corev1.ConditionTrue
			}

			operation.Status.Conditions[idx].Reason = ""
			operation.Status.Conditions[idx].Message = ""

			return operation
		}
	}

	operation.Status.Conditions = append(operation.Status.Conditions, v1alpha1.PackageRepositoryOperationStatusCondition{
		Type:               condType,
		Status:             corev1.ConditionTrue,
		LastProbeTime:      time,
		LastTransitionTime: time,
	})

	return operation
}

func (r *reconciler) SetConditionFalse(operation *v1alpha1.PackageRepositoryOperation, condType string, reason string, message string) *v1alpha1.PackageRepositoryOperation {
	time := metav1.NewTime(r.dc.GetClock().Now())

	for idx, cond := range operation.Status.Conditions {
		if cond.Type == condType {
			operation.Status.Conditions[idx].LastProbeTime = time
			if cond.Status != corev1.ConditionFalse {
				operation.Status.Conditions[idx].LastTransitionTime = time
				operation.Status.Conditions[idx].Status = corev1.ConditionFalse
			}

			operation.Status.Conditions[idx].Reason = reason
			operation.Status.Conditions[idx].Message = message

			return operation
		}
	}

	operation.Status.Conditions = append(operation.Status.Conditions, v1alpha1.PackageRepositoryOperationStatusCondition{
		Type:               condType,
		Status:             corev1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		LastProbeTime:      time,
		LastTransitionTime: time,
	})

	return operation
}

func (r *reconciler) updatePackageRepositoryCondition(ctx context.Context, repoName string, success bool, reason, message string) error {
	repo := new(v1alpha1.PackageRepository)
	if err := r.client.Get(ctx, client.ObjectKey{Name: repoName}, repo); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get package repository: %w", err)
	}

	original := repo.DeepCopy()
	now := metav1.NewTime(r.dc.GetClock().Now())

	status := metav1.ConditionTrue
	if !success {
		status = metav1.ConditionFalse
	}

	conditionExists := false
	for idx, cond := range repo.Status.Conditions {
		if cond.Type == v1alpha1.PackageRepositoryConditionLastOperationScanFinished {
			conditionExists = true

			if cond.Status != status {
				repo.Status.Conditions[idx].LastTransitionTime = now
				repo.Status.Conditions[idx].Status = status
			}

			repo.Status.Conditions[idx].Reason = reason
			repo.Status.Conditions[idx].Message = message
			break
		}
	}

	if !conditionExists {
		repo.Status.Conditions = append(repo.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.PackageRepositoryConditionLastOperationScanFinished,
			Status:             status,
			Reason:             reason,
			Message:            message,
			LastTransitionTime: now,
		})
	}

	if err := r.client.Status().Patch(ctx, repo, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("update package repository status: %w", err)
	}

	return nil
}
