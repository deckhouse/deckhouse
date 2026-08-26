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
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/package-repository-operation/operations"
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
func (r *reconciler) processNextPackage(ctx context.Context, op *v1alpha1.PackageRepositoryOperation, svc *operations.OperationService) (ctrl.Result, error) {
	currentPackage := op.Status.Packages.Discovered[0]
	r.logger.Info("processing package",
		slog.String("package", currentPackage.Name))

	result, err := svc.ProcessPackageVersions(ctx, currentPackage.Name, op)
	if err != nil {
		r.logger.Error("failed to process package versions",
			slog.String("package", currentPackage.Name),
			log.Err(err))
	}

	// ProcessPackageVersions contract: (nil, err) on hard failure, (result, nil) on success.
	// A nil result therefore implies err != nil — safe to use err.Error() downstream.
	if result == nil {
		return r.dequeuePackageWithError(ctx, op, currentPackage.Name, err)
	}

	repo := svc.GetRepository()

	// Ensure the appropriate package resource based on detected type.
	// Skip resource creation for unrecognized packages (e.g. legacy modules without metadata).
	switch result.PackageType {
	case operations.PackageTypeModule:
		if ensureErr := r.ensureModulePackage(ctx, currentPackage.Name, repo.Name, repo.UID); ensureErr != nil {
			r.logger.Error("failed to ensure module package resource",
				slog.String("package", currentPackage.Name),
				log.Err(ensureErr))
		}
	case operations.PackageTypeApplication:
		if ensureErr := r.ensureApplicationPackage(ctx, currentPackage.Name, repo.Name, repo.UID); ensureErr != nil {
			r.logger.Error("failed to ensure application package resource",
				slog.String("package", currentPackage.Name),
				log.Err(ensureErr))
		}
	}

	return r.dequeuePackageWithResult(ctx, op, currentPackage.Name, result)
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
func (r *reconciler) dequeuePackageWithResult(ctx context.Context, op *v1alpha1.PackageRepositoryOperation, packageName string, result *operations.ProcessResult) (ctrl.Result, error) {
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
		NewVersions:   result.NewVersions,
	})
	op.Status.Packages.NewVersionsOverall += result.NewVersions

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

// ensureModulePackage creates the ModulePackage for a scanned module and lists the repository
// among the ones offering it.
func (r *reconciler) ensureModulePackage(ctx context.Context, packageName, repoName string, repoUID apitypes.UID) error {
	pkg := new(v1alpha1.ModulePackage)
	err := r.client.Get(ctx, client.ObjectKey{Name: packageName}, pkg)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("get module package: %w", err)
	}

	// err - apierrors.IsNotFound
	if err != nil {
		// Create new ModulePackage with this repository as a non-controller owner.
		pkg = &v1alpha1.ModulePackage{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.ModulePackageGVK.GroupVersion().String(),
				Kind:       v1alpha1.ModulePackageKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:   packageName,
				Labels: map[string]string{"heritage": "deckhouse"},
			},
		}
		ensureSharedOwnerReference(pkg, repoName, repoUID)

		if err = r.client.Create(ctx, pkg); err != nil {
			return fmt.Errorf("create module package: %w", err)
		}
	} else {
		// Existing — make sure we are listed as an owner so a single repo deletion
		// does not cascade-delete a package that other repositories still contribute.
		original := pkg.DeepCopy()

		if update := ensureSharedOwnerReference(pkg, repoName, repoUID); update {
			if err := r.client.Patch(ctx, pkg, client.MergeFrom(original)); err != nil {
				return fmt.Errorf("sync module package: %w", err)
			}
		}
	}

	// Check if repository is already listed
	if slices.Contains(pkg.Status.AvailableRepositories, repoName) {
		return nil
	}

	// Update existing package to add repository to available repositories
	original := pkg.DeepCopy()

	pkg.Status.AvailableRepositories = append(pkg.Status.AvailableRepositories, repoName)

	if err := r.client.Status().Patch(ctx, pkg, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("update module package status: %w", err)
	}

	return nil
}

// ensureApplicationPackage creates the ApplicationPackage for a scanned package and lists the
// repository among the ones offering it.
func (r *reconciler) ensureApplicationPackage(ctx context.Context, packageName, repoName string, repoUID apitypes.UID) error {
	pkg := new(v1alpha1.ApplicationPackage)
	err := r.client.Get(ctx, client.ObjectKey{Name: packageName}, pkg)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("get application package: %w", err)
	}

	// err - apierrors.IsNotFound
	if err != nil {
		// Create new ApplicationPackage with this repository as a non-controller owner.
		pkg = &v1alpha1.ApplicationPackage{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.ApplicationPackageGVK.GroupVersion().String(),
				Kind:       v1alpha1.ApplicationPackageKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: packageName,
				Labels: map[string]string{
					"heritage": "deckhouse",
				},
			},
		}
		ensureSharedOwnerReference(pkg, repoName, repoUID)

		if err := r.client.Create(ctx, pkg); err != nil {
			return fmt.Errorf("create application package: %w", err)
		}
	} else {
		// Existing — make sure we are listed as an owner so a single repo deletion
		// does not cascade-delete a package that other repositories still contribute.
		original := pkg.DeepCopy()
		if ensureSharedOwnerReference(pkg, repoName, repoUID) {
			if err := r.client.Patch(ctx, pkg, client.MergeFrom(original)); err != nil {
				return fmt.Errorf("sync application package owner refs: %w", err)
			}
		}
	}

	// Check if repository is already listed
	if slices.Contains(pkg.Status.AvailableRepositories, repoName) {
		return nil
	}

	// Update existing package to add repository to available repositories
	original := pkg.DeepCopy()

	pkg.Status.AvailableRepositories = append(pkg.Status.AvailableRepositories, repoName)

	if err := r.client.Status().Patch(ctx, pkg, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("update application package status: %w", err)
	}

	return nil
}

// ensureSharedOwnerReference appends s.repo as a non-controller owner of obj if it is not
// already present. ApplicationPackage / ModulePackage CRs can be contributed by several
// repositories, so no single repository should be the sole controller-owner — otherwise
// Kubernetes GC would cascade-delete the package when that one repo is removed, even if
// other repos still contribute it. Returns true if obj.OwnerReferences was modified.
func ensureSharedOwnerReference(obj client.Object, repoName string, repoUID apitypes.UID) bool {
	refs := obj.GetOwnerReferences()
	for _, ref := range refs {
		if ref.Kind != v1alpha1.PackageRepositoryKind {
			continue
		}
		// Match by UID when both sides have one (real cluster); otherwise fall back
		// to Name (PackageRepository names are cluster-unique, and UIDs may be empty
		// in test fixtures).
		if ref.UID != "" && ref.UID == repoUID {
			return false
		}
		if ref.Name == repoName {
			return false
		}
	}
	refs = append(refs, metav1.OwnerReference{
		APIVersion: v1alpha1.PackageRepositoryGVK.GroupVersion().String(),
		Kind:       v1alpha1.PackageRepositoryKind,
		Name:       repoName,
		UID:        repoUID,
		Controller: &[]bool{false}[0],
	})
	obj.SetOwnerReferences(refs)
	return true
}
