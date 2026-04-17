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

func (r *reconciler) processNextPackage(ctx context.Context, op *v1alpha1.PackageRepositoryOperation, svc *OperationService) (ctrl.Result, error) {
	// Get first package from queue
	currentPackage := op.Status.Packages.Discovered[0]
	r.logger.Info("processing package",
		slog.String("package", currentPackage.Name))

	processResult, err := svc.ProcessPackageVersions(ctx, currentPackage.Name, op)
	if err != nil {
		r.logger.Error("failed to process package versions",
			slog.String("package", currentPackage.Name),
			log.Err(err))
	}

	// Processing failed entirely - record error and move to next package
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
