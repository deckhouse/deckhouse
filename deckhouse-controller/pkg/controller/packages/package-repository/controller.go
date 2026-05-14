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
	"crypto/md5"
	"encoding/json"
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

	registryService "github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry/service"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
	regClient "github.com/deckhouse/deckhouse/pkg/registry/client"
)

const (
	controllerName = "d8-package-repository-controller"

	maxConcurrentReconciles = 1

	defaultScanInterval = 6 * time.Hour
	minScanInterval     = 3 * time.Minute
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
	logger := r.logger.With(slog.String("name", req.Name))

	logger.Debug("reconcile resource")

	repo := new(v1alpha1.PackageRepository)
	if err := r.client.Get(ctx, req.NamespacedName, repo); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Debug("resource not found")

			return ctrl.Result{}, nil
		}

		logger.Warn("failed to get resource", log.Err(err))

		return ctrl.Result{}, err
	}

	// handle delete event
	if !repo.DeletionTimestamp.IsZero() {
		logger.Debug("delete resource")

		if err := r.delete(ctx, repo); err != nil {
			logger.Warn("failed to delete resource", log.Err(err))

			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	// handle create/update events
	res, err := r.handleCreateOrUpdate(ctx, repo)
	if err != nil {
		logger.Warn("failed to handle package repository", log.Err(err))

		return ctrl.Result{}, err
	}

	return res, nil
}

func (r *reconciler) handleCreateOrUpdate(ctx context.Context, repo *v1alpha1.PackageRepository) (ctrl.Result, error) {
	logger := r.logger.With(slog.String("name", repo.Name))

	logger.Debug("handle resource")

	if err := r.syncRegistrySettings(ctx, repo); err != nil {
		return ctrl.Result{}, fmt.Errorf("sync registry settings: %w", err)
	}

	if r.psm != nil {
		if err := r.checkPaginationSupport(ctx, repo); err != nil {
			logger.Warn("failed to check pagination support", log.Err(err))
		}
	}

	scanInterval := defaultScanInterval
	if interval := repo.Spec.ScanInterval; interval != nil {
		scanInterval = max(interval.Duration, minScanInterval)
	}

	// Check if there are any existing PackageRepositoryOperations for this repository
	operations := new(v1alpha1.PackageRepositoryOperationList)
	err := r.client.List(ctx, operations, client.MatchingLabels{
		v1alpha1.PackagesRepositoryOperationLabelRepository: repo.Name,
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("list operations: %w", err)
	}

	// Check if there is an active operation
	for _, op := range operations.Items {
		if !op.IsCompleted() {
			logger.Debug("active operation exists, skipping creation", slog.String("operation", op.Name))
			// Requeue to check again later
			return ctrl.Result{RequeueAfter: scanInterval}, nil
		}
	}

	// Determine if we should do a full scan or incremental scan
	// fullScan = true if this is the first operation ever (no operations at all)
	fullScan := len(operations.Items) == 0

	// Create a new PackageRepositoryOperation
	operationName := fmt.Sprintf("%s-scan-%d", repo.Name, r.dc.GetClock().Now().Unix())

	logger.With(slog.String("operation", operationName), slog.Bool("full_scan", fullScan))

	operation := &v1alpha1.PackageRepositoryOperation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.PackageRepositoryOperationGVK.GroupVersion().String(),
			Kind:       v1alpha1.PackageRepositoryOperationKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: operationName,
			Labels: map[string]string{
				v1alpha1.PackagesRepositoryOperationLabelRepository:       repo.Name,
				v1alpha1.PackagesRepositoryOperationLabelOperationTrigger: v1alpha1.PackagesRepositoryTriggerAuto,
				v1alpha1.PackagesRepositoryOperationLabelOperationType:    v1alpha1.PackageRepositoryOperationTypeUpdate,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: v1alpha1.PackageRepositoryGVK.GroupVersion().String(),
					Kind:       v1alpha1.PackageRepositoryGVK.Kind,
					Name:       repo.Name,
					UID:        repo.UID,
					Controller: &[]bool{true}[0],
				},
			},
		},
		Spec: v1alpha1.PackageRepositoryOperationSpec{
			PackageRepositoryName: repo.Name,
			Type:                  v1alpha1.PackageRepositoryOperationTypeUpdate,
			Update: &v1alpha1.PackageRepositoryOperationUpdate{
				FullScan: fullScan,
				Timeout:  "5m",
			},
		},
	}

	if err = r.client.Create(ctx, operation); err != nil {
		// If operation already exists (race condition), that's fine - just requeue
		if apierrors.IsAlreadyExists(err) {
			logger.Debug("operation already exists, skipping creation")

			return ctrl.Result{RequeueAfter: scanInterval}, nil
		}

		return ctrl.Result{}, fmt.Errorf("create operation %s: %w", operationName, err)
	}

	logger.Info("created package repository operation")
	logger.Debug("package repository reconciled", slog.String("interval", scanInterval.String()))

	return ctrl.Result{RequeueAfter: scanInterval}, nil
}

// checkPaginationSupport checks if the container registry supports pagination for tag listing.
// It compares a full tag listing with a limited one; if the results differ, pagination is supported.
func (r *reconciler) checkPaginationSupport(ctx context.Context, repo *v1alpha1.PackageRepository) error {
	svc, err := r.psm.Service(repo.Spec.Registry.Repo, utils.RegistryConfig{
		DockerConfig: repo.Spec.Registry.DockerCFG,
		Login:        repo.Spec.Registry.Login,
		Password:     repo.Spec.Registry.Password,
		CA:           repo.Spec.Registry.CA,
		Scheme:       repo.Spec.Registry.Scheme,
		UserAgent:    "deckhouse-package-controller",
	})
	if err != nil {
		return fmt.Errorf("create package service: %w", err)
	}

	// Request 1: full tag list (all applications)
	allTags, err := svc.ListTags(ctx)
	if err != nil {
		return fmt.Errorf("list all tags: %w", err)
	}

	// Request 2: limited tag list (with pagination)
	pagedTags, err := svc.ListTags(ctx, regClient.WithTagsLimit(1))
	if err != nil {
		return fmt.Errorf("list tags with limit: %w", err)
	}

	// If the lists differ in length, the registry supports pagination
	partialScanAvailable := len(allTags) != len(pagedTags)

	if repo.Status.PartialScanAvailable != partialScanAvailable {
		original := repo.DeepCopy()
		repo.Status.PartialScanAvailable = partialScanAvailable
		if err := r.client.Status().Patch(ctx, repo, client.MergeFrom(original)); err != nil {
			return fmt.Errorf("update PartialScanAvailable status: %w", err)
		}
	}

	return nil
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

// syncRegistrySettings checks if package repository registry settings were updated
// (comparing PackageRepositoryAnnotationRegistryChecksum annotation and the current registry spec)
// and triggers reconciliation of related Applications if it is the case
func (r *reconciler) syncRegistrySettings(ctx context.Context, repo *v1alpha1.PackageRepository) error {
	marshaled, err := json.Marshal(repo.Spec.Registry)
	if err != nil {
		return fmt.Errorf("marshal registry spec: %w", err)
	}

	currentChecksum := fmt.Sprintf("%x", md5.Sum(marshaled))

	if len(repo.ObjectMeta.Annotations) == 0 {
		original := repo.DeepCopy()
		repo.ObjectMeta.Annotations = map[string]string{
			v1alpha1.PackageRepositoryAnnotationRegistryChecksum: currentChecksum,
		}
		if err := r.client.Patch(ctx, repo, client.MergeFrom(original)); err != nil {
			return fmt.Errorf("set initial checksum annotation: %w", err)
		}
		return nil
	}

	if repo.ObjectMeta.Annotations[v1alpha1.PackageRepositoryAnnotationRegistryChecksum] == currentChecksum {
		return nil
	}

	apps := new(v1alpha1.ApplicationList)
	if err := r.client.List(ctx, apps); err != nil {
		return fmt.Errorf("list applications: %w", err)
	}

	now := r.dc.GetClock().Now().UTC().Format(time.RFC3339)

	var (
		updateErrors []error
		updatedCount = 0
	)

	for _, app := range apps.Items {
		if app.Spec.PackageRepositoryName != repo.Name {
			continue
		}

		original := app.DeepCopy()
		if len(app.ObjectMeta.Annotations) == 0 {
			app.ObjectMeta.Annotations = make(map[string]string)
		}

		app.ObjectMeta.Annotations[v1alpha1.ApplicationAnnotationRegistrySpecChanged] = now
		if err := r.client.Patch(ctx, &app, client.MergeFrom(original)); err != nil {
			updateErrors = append(updateErrors, fmt.Errorf("application %s/%s: %w", app.Namespace, app.Name, err))
			r.logger.Warn("failed to set registry-spec-changed annotation on application",
				slog.String("application", app.Name),
				slog.String("namespace", app.Namespace),
				log.Err(err))
			continue
		}

		updatedCount++
		r.logger.Info("triggered application reconciliation due to registry settings change",
			slog.String("application", app.Name),
			slog.String("namespace", app.Namespace))
	}

	original := repo.DeepCopy()
	repo.ObjectMeta.Annotations[v1alpha1.PackageRepositoryAnnotationRegistryChecksum] = currentChecksum
	if err := r.client.Patch(ctx, repo, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("update checksum annotation: %w", err)
	}

	if len(updateErrors) > 0 {
		r.logger.Warn("failed to update some applications",
			slog.Int("failed", len(updateErrors)),
			slog.Int("succeeded", updatedCount))
		if updatedCount == 0 {
			return fmt.Errorf("failed to update all %d application(s): %w", len(updateErrors), updateErrors[0])
		}
	}

	return nil
}
