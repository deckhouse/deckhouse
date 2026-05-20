// Copyright 2026 Flant JSC
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

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// processNextPackage processes exactly one package per reconcile — the head of
// status.Packages.Discovered. The caller (handleProcessingState) guarantees the
// queue is non-empty before calling.
//
// Outcomes:
//   - hard failure (processResult == nil): dequeue and record under Failed.
//   - success (possibly with per-version failures in result.Failed): ensure the
//     package resource matching the detected type, then dequeue and record under
//     Processed (with any per-version errors preserved in Failed).
//
// EnsureModulePackage / EnsureApplicationPackage errors are logged but not
// surfaced — the package itself processed successfully, only the umbrella
// resource creation failed, and that is recoverable on the next discovery cycle.
//
// Always returns Requeue=true so the next Discovered entry is picked up on the
// following reconcile, draining the queue one-at-a-time with etcd checkpoints
// between packages.
func (r *reconciler) processNextPackage(ctx context.Context, op *v1alpha1.PackageRepositoryOperation, svc *OperationService) (ctrl.Result, error) {
	currentPackage := op.Status.Packages.Discovered[0]
	r.logger.Info("processing package",
		slog.String("package", currentPackage.Name))

	processResult, err := svc.ProcessPackageVersions(ctx, currentPackage.Name, op)
	if err != nil {
		r.logger.Error("failed to process package versions",
			slog.String("package", currentPackage.Name),
			log.Err(err))
	}

	// ProcessPackageVersions contract: (nil, err) on hard failure, (result, nil) on success.
	// A nil result therefore implies err != nil — safe to use err.Error() downstream.
	if processResult == nil {
		return r.dequeuePackageWithError(ctx, op, currentPackage.Name, err)
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

	return r.dequeuePackageWithResult(ctx, op, currentPackage.Name, processResult)
}

// dequeuePackageWithError removes the head of the Discovered queue and records the
// package under Failed with a single aggregate error message (no per-version detail,
// since the package failed before its versions could be enumerated).
//
// Counts the package as processed (ProcessedOverall++) so that total accounting in
// status reflects queue drain progress regardless of success/failure.
//
// Precondition: Packages non-nil and Discovered non-empty (guaranteed by caller).
// The defensive guards below are redundant under current call sites but kept to
// tolerate any future caller that may invoke with an already-drained queue.
func (r *reconciler) dequeuePackageWithError(ctx context.Context, op *v1alpha1.PackageRepositoryOperation, packageName string, processErr error) (ctrl.Result, error) {
	original := op.DeepCopy()

	if len(op.Status.Packages.Discovered) > 0 {
		op.Status.Packages.Discovered = op.Status.Packages.Discovered[1:]
	}
	if op.Status.Packages != nil {
		op.Status.Packages.ProcessedOverall++
	}

	op.Status.Packages.Failed = append(op.Status.Packages.Failed, v1alpha1.PackageRepositoryOperationStatusFailedPackage{
		Name: packageName,
		Errors: []v1alpha1.PackageRepositoryOperationStatusFailedPackageError{
			{Message: processErr.Error()},
		},
	})

	if err := r.client.Status().Patch(ctx, op, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, fmt.Errorf("update operation status: %w", err)
	}
	return ctrl.Result{Requeue: true}, nil
}

// dequeuePackageWithResult removes the head of the Discovered queue and records the
// package under Processed with its detected type and version count. If the process
// result contains per-version failures (e.g. invalid image metadata on specific tags),
// they are additionally recorded under Failed so operators can see partial success.
//
// A package that succeeded at the package level but had every version fail still lands
// in Processed — the distinction is deliberate: Failed at the package level means we
// couldn't even determine what the package IS, while per-version failures mean we
// knew the package but couldn't ingest some of its versions.
//
// Precondition: Packages non-nil and Discovered non-empty (guaranteed by caller).
func (r *reconciler) dequeuePackageWithResult(ctx context.Context, op *v1alpha1.PackageRepositoryOperation, packageName string, result *PackageProcessResult) (ctrl.Result, error) {
	original := op.DeepCopy()

	if len(op.Status.Packages.Discovered) > 0 {
		op.Status.Packages.Discovered = op.Status.Packages.Discovered[1:]
	}
	if op.Status.Packages != nil {
		op.Status.Packages.ProcessedOverall++
	}

	op.Status.Packages.Processed = append(op.Status.Packages.Processed, v1alpha1.PackageRepositoryOperationStatusPackage{
		Name:          packageName,
		Type:          string(result.PackageType),
		FoundVersions: result.FoundVersions,
	})

	failedList := make([]v1alpha1.PackageRepositoryOperationStatusFailedPackageError, 0, len(result.Failed))
	for _, fv := range result.Failed {
		failedList = append(failedList, v1alpha1.PackageRepositoryOperationStatusFailedPackageError{
			Version: fv.Name,
			Message: fv.Error,
		})
	}
	if len(failedList) > 0 {
		op.Status.Packages.Failed = append(op.Status.Packages.Failed, v1alpha1.PackageRepositoryOperationStatusFailedPackage{
			Name:   packageName,
			Errors: failedList,
		})
	}

	if err := r.client.Status().Patch(ctx, op, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, fmt.Errorf("update operation status: %w", err)
	}
	return ctrl.Result{Requeue: true}, nil
}
