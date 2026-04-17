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

// handleCreateOrUpdate dispatches the operation to the handler for its current phase.
//
// The state machine has two axes, both encoded in the "Completed" status condition.
// Note the deliberate naming split: "Completed" is the condition *type* (the slot),
// while "Succeeded" / "Failed" are terminal *reasons* that fill it — the type name
// indicates presence of a terminal verdict, not success.
//
//	Pre-terminal (Status=False) — routed explicitly by Reason:
//	    (no condition)           → handlePendingState    (fresh operation)
//	    Reason=Discover          → handleDiscoverState
//	    Reason=Processing        → handleProcessingState
//
//	Terminal (Status=True) — routed uniformly via op.IsCompleted():
//	    Reason=Succeeded (OK) or Reason=Failed (KO) → handleCleanupState
//
// Each active pre-terminal handler advances the condition and requeues, so a full run
// performs one state transition per reconcile — the condition IS the durable checkpoint.
//
// Routing terminal states through IsCompleted() (rather than listing reasons) means
// any future terminal reason automatically participates in retention; conversely, any
// future pre-terminal reason must be added as an explicit case or it silently no-ops.
func (r *reconciler) handleCreateOrUpdate(ctx context.Context, op *v1alpha1.PackageRepositoryOperation) (ctrl.Result, error) {
	switch op.GetStateByCondition() {
	case "": // no Completed condition yet — fresh operation
		return r.handlePendingState(ctx, op)
	case v1alpha1.PackageRepositoryOperationReasonDiscover:
		return r.handleDiscoverState(ctx, op)
	case v1alpha1.PackageRepositoryOperationReasonProcessing:
		return r.handleProcessingState(ctx, op)
	default:
		if op.IsCompleted() {
			return r.handleCleanupState(ctx, op)
		}

		return ctrl.Result{}, nil
	}
}

// handlePendingState runs once when the operation has no Completed condition yet.
// It stamps StartTime and advances the phase to Discover, then requeues so the next
// reconcile runs the actual discovery under a fresh object snapshot.
func (r *reconciler) handlePendingState(ctx context.Context, op *v1alpha1.PackageRepositoryOperation) (ctrl.Result, error) {
	r.logger.Debug("handle pending state", slog.String("name", op.Name))

	original := op.DeepCopy()

	r.setCompletedConditionFalse(op, v1alpha1.PackageRepositoryOperationReasonDiscover, "")
	op.Status.StartTime = ptr.To(metav1.Now())

	if err := r.client.Status().Patch(ctx, op, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, fmt.Errorf("update operation status: %w", err)
	}

	return ctrl.Result{Requeue: true}, nil
}

// handleDiscoverState connects to the registry, lists packages, and records them in
// status.Packages.Discovered. On success it advances the phase to Processing and requeues.
// On failure it delegates to failOperation, which marks the operation terminally Failed
// and propagates the failure to the parent PackageRepository's LastScanSucceeded condition.
func (r *reconciler) handleDiscoverState(ctx context.Context, op *v1alpha1.PackageRepositoryOperation) (ctrl.Result, error) {
	logger := r.logger.With(slog.String("name", op.Name))

	logger.Debug("handle discover state")

	svc, err := NewOperationService(ctx, r.client, op.Spec.PackageRepositoryName, r.psm, r.logger)
	if err != nil {
		return r.failOperation(ctx, op, err)
	}

	discovered, err := svc.DiscoverPackage(ctx)
	if err != nil {
		return r.failOperation(ctx, op, err)
	}

	original := op.DeepCopy()

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

// handleProcessingState drains the Discovered queue one package per reconcile.
// Each reconcile either:
//   - processes one package via processNextPackage (dequeues from Discovered, appends
//     to Processed or Failed), then requeues; or
//   - detects an empty queue, pushes the final aggregate to the PackageRepository via
//     UpdateRepositoryStatus, marks the operation terminally Completed=True, and returns.
//
// Dequeueing one-at-a-time per reconcile persists progress to etcd between packages,
// so a crash mid-processing doesn't lose work — the next leader resumes on the next
// Discovered entry.
func (r *reconciler) handleProcessingState(ctx context.Context, op *v1alpha1.PackageRepositoryOperation) (ctrl.Result, error) {
	logger := r.logger.With(slog.String("name", op.Name))

	logger.Debug("handle processing state")

	svc, err := NewOperationService(ctx, r.client, op.Spec.PackageRepositoryName, r.psm, r.logger)
	if err != nil {
		return r.failOperation(ctx, op, err)
	}

	// Check if all packages have been processed
	if len(op.Status.Packages.Discovered) == 0 {
		r.logger.Info("all packages processed", slog.Int("total", op.Status.Packages.Total))

		if err := svc.UpdateRepositoryStatus(ctx, op.Status.Packages.Processed); err != nil {
			logger.Warn("failed to update repository status", log.Err(err))
			// Continue with operation completion even if repository update fails
		}

		original := op.DeepCopy()

		// All packages processed, mark as completed
		now := metav1.Now()
		op.Status.CompletionTime = &now

		r.setCompletedConditionTrue(op, v1alpha1.PackageRepositoryOperationReasonScanSucceeded, "")

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

// handleCleanupState is the terminal handler for both Completed and Failed operations.
// It keeps the N most recent operations for this repository (cleanupOldOperationsCount)
// and deletes the rest, regardless of whether each one succeeded or failed.
//
// The sibling list is filtered by the repository label, sorted newest-first, and
// everything past index N is deleted. If a delete fails, the reconciler returns an
// error and controller-runtime retries — the loop is idempotent because subsequent
// runs will still see the same "newest N" prefix preserved.
//
// Note: this runs on every reconcile while the operation stays terminal, so the
// no-op fast path (`len <= N`) is important for controller throughput — especially
// for repositories stuck in a failure loop that would otherwise re-scan+re-cleanup
// on every reconcile.
func (r *reconciler) handleCleanupState(ctx context.Context, op *v1alpha1.PackageRepositoryOperation) (ctrl.Result, error) {
	logger := r.logger.With(slog.String("name", op.Name))

	logger.Debug("handle completed state")

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

	// Sort newest-first so the retention window is the prefix [0:cleanupOldOperationsCount).
	sort.Slice(operations.Items, func(i, j int) bool {
		return !operations.Items[i].CreationTimestamp.Before(&operations.Items[j].CreationTimestamp)
	})

	// Delete everything older than the retention window.
	for _, toDelete := range operations.Items[cleanupOldOperationsCount:] {
		logger.Debug("delete old operation", slog.String("name", toDelete.Name))
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

// failOperation marks op terminally Failed, patches status, and mirrors the failure
// to the parent PackageRepository's LastScanSucceeded condition. All pre-terminal
// handlers funnel errors here so the terminate-as-failed sequence has one
// authoritative implementation.
//
// The patch baseline is captured inside the helper (DeepCopy of op on entry), which
// implies callers MUST call this before mutating op — any prior mutations are in the
// baseline and will therefore be dropped from the patch. That matches the "discard
// in-flight work, just record the failure" semantics we want; if you ever need to
// preserve pending mutations on failure, capture the baseline before mutating and
// pass it in instead.
//
// Always returns (Result{}, nil): the failure is persisted into status rather than
// bubbled back to controller-runtime, so there is no automatic retry — a new
// operation must be created to retry.
func (r *reconciler) failOperation(ctx context.Context, op *v1alpha1.PackageRepositoryOperation, cause error) (ctrl.Result, error) {
	logger := r.logger.With(slog.String("name", op.Name))

	original := op.DeepCopy()

	now := metav1.Now()
	op.Status.CompletionTime = &now
	r.setCompletedConditionTrue(op, v1alpha1.PackageRepositoryOperationReasonScanFailed, cause.Error())

	if err := r.client.Status().Patch(ctx, op, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.updatePackageRepositoryCondition(ctx, op); err != nil {
		logger.Warn("failed to update package repository condition", log.Err(err))
	}

	logger.Warn("operation failed", log.Err(cause))
	return ctrl.Result{}, nil
}

// updatePackageRepositoryCondition mirrors the operation's terminal Completed condition
// onto the parent PackageRepository's LastScanSucceeded condition, so consumers watching
// only the repository can tell whether the most recent scan succeeded.
//
// Mapping: operation Reason=Failed → LastScanSucceeded=False; any other reason → True.
// Missing repository (NotFound) is treated as a silent no-op — the operation has
// outlived its parent and cascade deletion will clean it up.
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
	if cond.Reason == v1alpha1.PackageRepositoryOperationReasonScanFailed {
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
