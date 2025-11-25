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
	"fmt"
	"log/slog"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	registryService "github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry/service"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	controllerName = "d8-package-repository-operation-controller"

	maxConcurrentReconciles = 1

	// packageTypeLabel is a label on Docker images that indicates the package type
	packageTypeLabel = "io.deckhouse.package.type"

	// TODO: unify constant
	packageTypeApplication = "Application"
)

type reconciler struct {
	client client.Client
	dc     dependency.Container
	psm    *registryService.PackageServiceManager
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

	// ensure operation trigger label
	res, err := r.EnsureLabelOperationTrigger(ctx, operation)
	if err != nil {
		logger.Warn("failed to ensure operation trigger label", log.Err(err))

		return res, err
	}

	if res.Requeue {
		return res, nil
	}

	// ensure operation type label
	res, err = r.EnsureLabelOperationType(ctx, operation)
	if err != nil {
		logger.Warn("failed to ensure operation type label", log.Err(err))

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

func (r *reconciler) EnsureLabelOperationTrigger(ctx context.Context, operation *v1alpha1.PackageRepositoryOperation) (ctrl.Result, error) {
	res := ctrl.Result{}

	if operation.Labels == nil {
		operation.Labels = make(map[string]string)
	}

	if _, ok := operation.Labels[v1alpha1.PackagesRepositoryOperationLabelOperationTrigger]; !ok {
		original := operation.DeepCopy()
		operation.Labels[v1alpha1.PackagesRepositoryOperationLabelOperationTrigger] = v1alpha1.PackagesRepositoryTriggerManual

		if err := r.client.Patch(ctx, operation, client.MergeFrom(original)); err != nil {
			return res, fmt.Errorf("patch operation trigger label: %w", err)
		}

		return ctrl.Result{Requeue: true}, nil
	}

	return res, nil
}

func (r *reconciler) EnsureLabelOperationType(ctx context.Context, operation *v1alpha1.PackageRepositoryOperation) (ctrl.Result, error) {
	res := ctrl.Result{}

	if operation.Labels == nil {
		operation.Labels = make(map[string]string)
	}

	var opType string
	if operation.Spec.Type != "" {
		opType = operation.Spec.Type
	} else {
		opType = ""
	}

	if existing, ok := operation.Labels[v1alpha1.PackagesRepositoryOperationLabelOperationType]; !ok || existing != opType {
		original := operation.DeepCopy()
		operation.Labels[v1alpha1.PackagesRepositoryOperationLabelOperationType] = opType

		if err := r.client.Patch(ctx, operation, client.MergeFrom(original)); err != nil {
			return res, fmt.Errorf("patch operation type label: %w", err)
		}

		return ctrl.Result{Requeue: true}, nil
	}

	return res, nil
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
		r.logger.Debug("operation already completed", slog.String("name", operation.Name))
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

	// Get PackageRepository
	repo := &v1alpha1.PackageRepository{}
	err := r.client.Get(ctx, types.NamespacedName{Name: operation.Spec.PackageRepository}, repo)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Warn("package repository operation not found")

			original := operation.DeepCopy()

			now := metav1.Now()
			operation.Status.CompletionTime = &now
			message := fmt.Sprintf("PackageRepository not found: %v", err)

			r.SetConditionFalse(
				operation,
				v1alpha1.PackageRepositoryOperationConditionProcessed,
				v1alpha1.PackageRepositoryOperationReasonPackageRepositoryNotFound,
				message,
			)

			if err := r.client.Status().Patch(ctx, operation, client.MergeFrom(original)); err != nil {
				return ctrl.Result{}, err
			}

			r.logger.Warn("operation failed", slog.String("name", operation.Name), slog.String("message", message))

			return ctrl.Result{}, nil
		}

		return res, fmt.Errorf("get package repository: %w", err)
	}

	// Create registry service for the packages path
	svc, err := r.psm.PackagesService(
		repo.Spec.Registry.Repo,
		repo.Spec.Registry.DockerCFG,
		repo.Spec.Registry.CA,
		"deckhouse-package-controller",
		repo.Spec.Registry.Scheme,
	)
	if err != nil {
		r.logger.Error("failed to get registry service", log.Err(err))

		original := operation.DeepCopy()

		now := metav1.Now()
		operation.Status.CompletionTime = &now
		message := fmt.Sprintf("Failed to get registry service: %v", err)

		r.SetConditionFalse(
			operation,
			v1alpha1.PackageRepositoryOperationConditionProcessed,
			v1alpha1.PackageRepositoryOperationReasonRegistryClientCreationFailed,
			message,
		)

		if err := r.client.Status().Patch(ctx, operation, client.MergeFrom(original)); err != nil {
			return ctrl.Result{}, err
		}

		r.logger.Warn("operation failed", slog.String("name", operation.Name), slog.String("message", message))

		return ctrl.Result{}, nil
	}

	if operation.Status.Packages == nil {
		operation.Status.Packages = &v1alpha1.PackageRepositoryOperationStatusPackages{}
	}

	r.logger.Info("discovering packages", slog.String("repository", repo.Name))

	discovered, err := r.discoverPackages(ctx, operation, svc)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("discover packages: %w", err)
	}

	// Handle discovered packages
	err = r.handleOperationDiscoverResult(ctx, operation, discovered)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("handle operation discover result: %w", err)
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *reconciler) handleProcessingState(ctx context.Context, operation *v1alpha1.PackageRepositoryOperation) (ctrl.Result, error) {
	res := ctrl.Result{}

	logger := r.logger.With(slog.String("name", operation.Name))

	logger.Debug("handling processing state")

	// Get PackageRepository
	repo := &v1alpha1.PackageRepository{}
	err := r.client.Get(ctx, types.NamespacedName{Name: operation.Spec.PackageRepository}, repo)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Warn("package repository operation not found")

			original := operation.DeepCopy()

			now := metav1.Now()
			operation.Status.CompletionTime = &now
			message := fmt.Sprintf("PackageRepository not found: %v", err)

			r.SetConditionFalse(
				operation,
				v1alpha1.PackageRepositoryOperationConditionProcessed,
				v1alpha1.PackageRepositoryOperationReasonPackageRepositoryNotFound,
				message,
			)

			if err := r.client.Status().Patch(ctx, operation, client.MergeFrom(original)); err != nil {
				return ctrl.Result{}, err
			}

			r.logger.Warn("operation failed", slog.String("name", operation.Name), slog.String("message", message))

			return ctrl.Result{}, nil
		}

		return res, fmt.Errorf("get package repository: %w", err)
	}

	// TODO: change finalizing
	if operation.Status.Packages != nil && len(operation.Status.Packages.Discovered) == operation.Status.Packages.ProcessedOverall {
		r.logger.Info("all packages processed, marking as completed",
			slog.Int("total", operation.Status.Packages.Total))

		original := operation.DeepCopy()

		// All packages processed, mark as completed
		operation.Status.Phase = v1alpha1.PackageRepositoryOperationPhaseCompleted
		now := metav1.Now()
		operation.Status.CompletionTime = &now

		r.SetConditionTrue(
			operation,
			v1alpha1.PackageRepositoryOperationConditionProcessed,
		)

		if err := r.client.Status().Patch(ctx, operation, client.MergeFrom(original)); err != nil {
			return ctrl.Result{}, fmt.Errorf("update operation status: %w", err)
		}

		r.logger.Info("operation completed", slog.String("name", operation.Name))

		return ctrl.Result{}, nil
	}

	// // Create registry service for the packages path
	// svc, err := r.psm.PackagesService(
	// 	repo.Spec.Registry.Repo,
	// 	repo.Spec.Registry.DockerCFG,
	// 	repo.Spec.Registry.CA,
	// 	"deckhouse-package-controller",
	// 	repo.Spec.Registry.Scheme,
	// )
	// if err != nil {
	// 	r.logger.Error("failed to get registry service", log.Err(err))

	// 	original := operation.DeepCopy()

	// 	now := metav1.Now()
	// 	operation.Status.CompletionTime = &now
	// 	message := fmt.Sprintf("Failed to get registry service: %v", err)

	// 	r.SetConditionFalse(
	// 		operation,
	// 		v1alpha1.PackageRepositoryOperationConditionProcessed,
	// 		v1alpha1.PackageRepositoryOperationReasonRegistryClientCreationFailed,
	// 		message,
	// 	)

	// 	if err := r.client.Status().Patch(ctx, operation, client.MergeFrom(original)); err != nil {
	// 		return ctrl.Result{}, err
	// 	}

	// 	r.logger.Warn("operation failed", slog.String("name", operation.Name), slog.String("message", message))

	// 	return ctrl.Result{}, nil
	// }

	// Process the first package in the queue
	return r.processNextPackage(ctx, operation, repo)
}

type discoverResult struct {
	Packages        []packageInfo
	RepositoryPhase string
	SyncTime        time.Time
}

type packageInfo struct {
	Name string
	Type string
}

func (r *reconciler) discoverPackages(ctx context.Context, operation *v1alpha1.PackageRepositoryOperation, svc *registryService.PackagesService) (*discoverResult, error) {
	// List packages (packages at the packages level)
	packages, err := svc.ListTags(ctx)
	if err != nil {
		r.logger.Error("failed to list packages", log.Err(err))

		original := operation.DeepCopy()

		now := metav1.Now()
		operation.Status.CompletionTime = &now
		message := fmt.Sprintf("Failed to list packages: %v", err)

		r.SetConditionFalse(
			operation,
			v1alpha1.PackageRepositoryOperationConditionProcessed,
			v1alpha1.PackageRepositoryOperationReasonPackageListingFailed,
			message,
		)

		if err := r.client.Status().Patch(ctx, operation, client.MergeFrom(original)); err != nil {
			return nil, err
		}

		r.logger.Warn("operation failed", slog.String("name", operation.Name), slog.String("message", message))

		return nil, err
	}

	r.logger.Info("discovered packages", slog.Int("count", len(packages)))

	discoveredPackages := make([]packageInfo, 0, len(packages))

	for _, pkg := range packages {
		discoveredPackages = append(discoveredPackages, packageInfo{
			Name: pkg,
		})
	}

	res := &discoverResult{
		Packages:        discoveredPackages,
		RepositoryPhase: v1alpha1.PackageRepositoryPhaseActive,
		SyncTime:        time.Now(),
	}

	return res, nil
}

func (r *reconciler) handleOperationDiscoverResult(ctx context.Context, operation *v1alpha1.PackageRepositoryOperation, discovered *discoverResult) error {
	// Update operation status with discovered packages
	original := operation.DeepCopy()

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

// // TODO
// func (r *reconciler) handleRepositoryProcessingResult(ctx context.Context, repo *v1alpha1.PackageRepository, discovered *discoverResult) error {
// 	original := repo.DeepCopy()

// 	repo.Status.Packages = make([]v1alpha1.PackageRepositoryStatusPackage, 0, len(discovered.Packages))

// 	for _, pkg := range discovered.Packages {
// 		repo.Status.Packages = append(repo.Status.Packages, v1alpha1.PackageRepositoryStatusPackage{
// 			Name: pkg.Name,
// 			Type: pkg.Type,
// 		})
// 	}

// 	repo.Status.PackagesCount = len(discovered.Packages)
// 	repo.Status.Phase = v1alpha1.PackageRepositoryPhaseActive
// 	repo.Status.SyncTime = metav1.NewTime(discovered.SyncTime)

// 	if err := r.client.Status().Patch(ctx, repo, client.MergeFrom(original)); err != nil {
// 		return fmt.Errorf("update repository status: %w", err)
// 	}

// 	return nil
// }

func (r *reconciler) processNextPackage(ctx context.Context, operation *v1alpha1.PackageRepositoryOperation, repo *v1alpha1.PackageRepository) (ctrl.Result, error) {
	// Get first package from queue
	currentPackage := operation.Status.Packages.Discovered[0]
	r.logger.Info("processing package",
		slog.String("package", currentPackage.Name))

	// Generate registry options
	registryConfig := &utils.RegistryConfig{
		DockerConfig: repo.Spec.Registry.DockerCFG,
		Scheme:       repo.Spec.Registry.Scheme,
		CA:           repo.Spec.Registry.CA,
		UserAgent:    "deckhouse-package-controller",
	}
	opts := utils.GenerateRegistryOptions(registryConfig, r.logger)

	// Create or update ApplicationPackage or ClusterApplicationPackage
	err := r.ensurePackageResource(ctx, currentPackage.Name, repo.Name)
	if err != nil {
		r.logger.Error("failed to ensure package resource",
			slog.String("package", currentPackage.Name),
			log.Err(err))
		// Continue with next package even if this one fails
	}

	// List versions for this package
	err = r.processPackageVersions(ctx, currentPackage.Name, repo, operation, opts)
	if err != nil {
		r.logger.Error("failed to process package versions",
			slog.String("package", currentPackage.Name),
			log.Err(err))
		// Continue with next package even if this one fails
	}

	// Remove processed package from queue
	original := operation.DeepCopy()
	if len(operation.Status.Packages.Discovered) > 0 {
		operation.Status.Packages.Discovered = operation.Status.Packages.Discovered[1:]
	}
	if operation.Status.Packages != nil {
		operation.Status.Packages.ProcessedOverall++
	}

	operation.Status.Packages.Processed = append(operation.Status.Packages.Processed, v1alpha1.PackageRepositoryOperationStatusPackage{
		Name: currentPackage.Name,
	})

	if err := r.client.Status().Patch(ctx, operation, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, fmt.Errorf("update operation status: %w", err)
	}

	// Requeue to process next package
	return ctrl.Result{Requeue: true}, nil
}

func (r *reconciler) processPackageVersions(ctx context.Context, packageName string, repo *v1alpha1.PackageRepository, operation *v1alpha1.PackageRepositoryOperation, opts []cr.Option) error {
	// Create registry client for package versions
	packagePath := path.Join(repo.Spec.Registry.Repo, packageName)
	registryClient, err := r.dc.GetRegistryClient(packagePath, opts...)
	if err != nil {
		return fmt.Errorf("create registry client for package: %w", err)
	}

	var allTags []string
	var scanErr error

	// Handle fullScan vs incremental scan
	if operation.Spec.Update != nil && operation.Spec.Update.FullScan {
		allTags, scanErr = r.performFullScan(ctx, registryClient, packageName)
		if scanErr != nil {
			return scanErr
		}
		r.logger.Info("found package versions",
			slog.String("package", packageName),
			slog.Int("versions", len(allTags)))

		// Create PackageVersion resources for each version
		return r.createPackageVersions(ctx, packageName, repo, allTags)
	}

	allTags, scanErr = r.performIncrementalScan(ctx, registryClient, packageName, repo.Name)
	if scanErr != nil {
		return scanErr
	}

	r.logger.Info("found package versions",
		slog.String("package", packageName),
		slog.Int("versions", len(allTags)))

	// Create PackageVersion resources for each version
	return r.createPackageVersions(ctx, packageName, repo, allTags)
}

func (r *reconciler) createPackageVersions(ctx context.Context, packageName string, repo *v1alpha1.PackageRepository, allTags []string) error {
	for _, versionTag := range allTags {
		// Skip non-version tags (like "release-channel", "version", etc.)
		if err := r.checkVersionTag(versionTag); err != nil {
			r.logger.Debug("skipping non-version tag",
				slog.String("package", packageName),
				slog.String("tag", versionTag),
				log.Err(err))

			continue
		}

		err := r.ensurePackageVersion(ctx, packageName, versionTag, repo.Name)
		if err != nil {
			r.logger.Warn("failed to create package version",
				slog.String("package", packageName),
				slog.String("version", versionTag),
				log.Err(err))
			// Continue with other versions
			continue
		}
	}

	return nil
}

func (r *reconciler) ensurePackageResource(ctx context.Context, packageName, repositoryName string) error {
	return r.ensureApplicationPackage(ctx, packageName, repositoryName)
}

func (r *reconciler) ensureApplicationPackage(ctx context.Context, packageName, repositoryName string) error {
	pkg := &v1alpha1.ApplicationPackage{}
	err := r.client.Get(ctx, types.NamespacedName{Name: packageName}, pkg)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		// Create new ApplicationPackage
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

		// Add owner reference to PackageRepository
		if err := r.setOwnerReference(ctx, pkg, repositoryName); err != nil {
			r.logger.Warn("failed to set owner reference",
				slog.String("repository", repositoryName),
				log.Err(err))
		}

		return r.client.Create(ctx, pkg)
	}

	// Update existing package to add repository to available repositories
	original := pkg.DeepCopy()

	if !slices.Contains(pkg.Status.AvailableRepositories, repositoryName) {
		pkg.Status.AvailableRepositories = append(pkg.Status.AvailableRepositories, repositoryName)
	}

	err = r.client.Status().Patch(ctx, pkg, client.MergeFrom(original))
	if err != nil {
		return fmt.Errorf("update application package status: %w", err)
	}

	return nil
}

func (r *reconciler) ensurePackageVersion(ctx context.Context, packageName, version, repositoryName string) error {
	apvName := v1alpha1.MakeApplicationPackageVersionName(repositoryName, packageName, version)

	return r.ensureApplicationPackageVersion(ctx, apvName, packageName, version, repositoryName)
}

func (r *reconciler) ensureApplicationPackageVersion(ctx context.Context, resourceName, packageName, version, repositoryName string) error {
	pkgVersion := &v1alpha1.ApplicationPackageVersion{}
	err := r.client.Get(ctx, types.NamespacedName{Name: resourceName}, pkgVersion)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		// Create new ApplicationPackageVersion with draft label
		pkgVersion = &v1alpha1.ApplicationPackageVersion{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.ApplicationPackageVersionGVK.GroupVersion().String(),
				Kind:       v1alpha1.ApplicationPackageVersionKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: resourceName,
				Labels: map[string]string{
					"heritage": "deckhouse",
					v1alpha1.ApplicationPackageVersionLabelRepository: repositoryName,
					v1alpha1.ApplicationPackageVersionLabelPackage:    packageName,
					v1alpha1.ApplicationPackageVersionLabelDraft:      "true",
				},
			},
			Spec: v1alpha1.ApplicationPackageVersionSpec{
				PackageName:       packageName,
				Version:           version,
				PackageRepository: repositoryName,
			},
		}

		// Add owner reference to PackageRepository
		if err := r.setOwnerReference(ctx, pkgVersion, repositoryName); err != nil {
			r.logger.Warn("failed to set owner reference",
				slog.String("repository", repositoryName),
				log.Err(err))
		}

		return r.client.Create(ctx, pkgVersion)
	}

	// Version already exists
	return nil
}

func (r *reconciler) determinePackageType(ctx context.Context, registryRepo, packageName string, opts []cr.Option) (string, error) {
	logger := r.logger.With(slog.String("package", packageName))

	// Create registry client for the package marker image
	registryClient, err := r.dc.GetRegistryClient(registryRepo, opts...)
	if err != nil {
		return "", fmt.Errorf("create registry client: %w", err)
	}

	// Get image to read labels from config
	image, err := registryClient.Image(ctx, packageName)
	if err != nil {
		// If we can't read the image, default to Application
		logger.Warn("failed to get image, defaulting to Application", log.Err(err))

		return packageTypeApplication, nil
	}

	// Get image config to extract labels
	configFile, err := image.ConfigFile()
	if err != nil {
		r.logger.Warn("failed to get config file, defaulting to Application", log.Err(err))

		return packageTypeApplication, nil
	}

	// Extract package type from label
	if configFile != nil && configFile.Config.Labels != nil {
		if packageType, ok := configFile.Config.Labels[packageTypeLabel]; ok {
			return packageType, nil
		}
	}

	// Default to Application if label not found
	r.logger.Debug("package type label not found, defaulting to Application")

	return packageTypeApplication, nil
}

func (r *reconciler) checkVersionTag(tag string) error {
	if len(tag) == 0 {
		return nil
	}

	// Use semver validation for proper version tag detection
	_, err := semver.NewVersion(strings.TrimPrefix(tag, "v"))
	if err != nil {
		return fmt.Errorf("invalid version tag: %w", err)
	}

	return nil
}

func (r *reconciler) listAllTagsWithPagination(ctx context.Context, registryClient cr.Client) ([]string, error) {
	// Note: The current registry client (cr.Client) returns all tags at once.
	// Pagination is handled by the underlying registry API if the registry supports it.
	// The limit parameter is typically handled by the registry itself (e.g., Docker Registry API v2)
	// and will automatically paginate internally if needed.

	// For registries that support the Link header for pagination, the client library
	// should handle it automatically. If not, we get all available tags in one call.
	tags, err := registryClient.ListTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	r.logger.Debug("listed all tags",
		slog.Int("count", len(tags)),
		slog.String("registry", "current"))

	return tags, nil
}

func (r *reconciler) listTagsFromVersion(ctx context.Context, registryClient cr.Client, lastVersion string) ([]string, error) {
	// If no last version, do a full scan
	if lastVersion == "" {
		return r.listAllTagsWithPagination(ctx, registryClient)
	}

	// List tags starting from the last version
	// Note: This requires registry client support for the "last" parameter
	// For now, we'll do a full list and filter (not true incremental scan)
	// TODO: Implement true incremental scan
	allTags, err := registryClient.ListTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	// Filter tags to only include versions after lastVersion
	var newTags []string
	lastVer, err := semver.NewVersion(strings.TrimPrefix(lastVersion, "v"))
	if err != nil {
		// If we can't parse last version, return all tags
		return allTags, nil
	}

	for _, tag := range allTags {
		tagVer, err := semver.NewVersion(strings.TrimPrefix(tag, "v"))
		if err != nil {
			continue
		}

		// Only include tags that are newer than lastVersion
		if tagVer.GreaterThan(lastVer) {
			newTags = append(newTags, tag)
		}
	}

	return newTags, nil
}

func (r *reconciler) performFullScan(ctx context.Context, registryClient cr.Client, packageName string) ([]string, error) {
	// Full scan: list all tags with pagination
	r.logger.Debug("performing full scan", slog.String("package", packageName))

	tags, err := r.listAllTagsWithPagination(ctx, registryClient)
	if err != nil {
		return nil, fmt.Errorf("list all tags with pagination: %w", err)
	}

	return tags, nil
}

func (r *reconciler) performIncrementalScan(ctx context.Context, registryClient cr.Client, packageName, repositoryName string) ([]string, error) {
	// Incremental scan: start from the last processed version
	r.logger.Debug("performing incremental scan", slog.String("package", packageName))

	lastVersion := r.getLastProcessedVersion(ctx, packageName, repositoryName)
	if lastVersion != "" {
		r.logger.Debug("found last processed version",
			slog.String("package", packageName),
			slog.String("lastVersion", lastVersion))
	}

	tags, err := r.listTagsFromVersion(ctx, registryClient, lastVersion)
	if err != nil {
		return nil, fmt.Errorf("list tags from version: %w", err)
	}

	return tags, nil
}

func (r *reconciler) setOwnerReference(ctx context.Context, obj client.Object, repositoryName string) error {
	repo := &v1alpha1.PackageRepository{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: repositoryName}, repo); err != nil {
		return err
	}

	ownerRef := metav1.OwnerReference{
		APIVersion: v1alpha1.PackageRepositoryGVK.GroupVersion().String(),
		Kind:       v1alpha1.PackageRepositoryKind,
		Name:       repo.Name,
		UID:        repo.UID,
		Controller: &[]bool{true}[0],
	}

	obj.SetOwnerReferences([]metav1.OwnerReference{ownerRef})

	return nil
}

func (r *reconciler) getLastProcessedVersion(ctx context.Context, packageName, repositoryName string) string {
	// Find the latest PackageVersion for this package from this repository
	var versionList client.ObjectList = &v1alpha1.ApplicationPackageVersionList{}

	err := r.client.List(ctx, versionList, client.MatchingLabels{
		"repository": repositoryName,
		"package":    packageName,
	})
	if err != nil {
		r.logger.Warn("failed to list package versions",
			slog.String("package", packageName),
			log.Err(err))
		return ""
	}

	// Extract versions and find the latest one
	var versions []*semver.Version
	var versionTags []string

	switch list := versionList.(type) {
	case *v1alpha1.ApplicationPackageVersionList:
		for _, item := range list.Items {
			if item.Status.Version != "" {
				versionTags = append(versionTags, item.Status.Version)
			}
		}
	default:
		{
			r.logger.Warn("unsupported package version list type",
				slog.String("package", packageName))
			return ""
		}
	}

	// Parse all versions
	for _, vTag := range versionTags {
		v, err := semver.NewVersion(strings.TrimPrefix(vTag, "v"))
		if err == nil {
			versions = append(versions, v)
		}
	}

	// Find the latest version
	if len(versions) == 0 {
		return ""
	}

	latest := versions[0]
	for _, v := range versions[1:] {
		if v.GreaterThan(latest) {
			latest = v
		}
	}

	return "v" + latest.String()
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
