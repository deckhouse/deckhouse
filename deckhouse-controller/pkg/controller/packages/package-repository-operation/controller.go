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

package packagerepositoryoperation

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/ctrlutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/pkg/log"
	corev1 "k8s.io/api/core/v1"
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
	logger *log.Logger
	dc     dependency.Container
}

func RegisterController(
	runtimeManager manager.Manager,
	dc dependency.Container,
	logger *log.Logger,
) error {
	r := &reconciler{
		client: runtimeManager.GetClient(),
		logger: logger,
		dc:     dc,
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

	// handle create/update events - state machine
	res, err := r.handle(ctx, operation)
	if err != nil {
		logger.Warn("failed to handle package repository operation", log.Err(err))

		return res, err
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
	case v1alpha1.PackageRepositoryOperationPhaseProcessing:
		res, err = r.handleProcessingState(ctx, operation)
	case v1alpha1.PackageRepositoryOperationPhaseCompleted:
		res, err = r.handleCompletedState(ctx, operation)
	case v1alpha1.PackageRepositoryOperationPhaseFailed:
		res, err = r.handleFailedState(ctx, operation)
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

	operation.Status.Phase = v1alpha1.PackageRepositoryOperationPhaseProcessing

	if err := r.client.Status().Patch(ctx, operation, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, fmt.Errorf("update operation status: %w", err)
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

			now := metav1.Now()
			operation.Status.CompletionTime = &now
			message := fmt.Sprintf("PackageRepository not found: %v", err)

			r.SetConditionFalse(
				operation,
				v1alpha1.PackageRepositoryOperationConditionFailed,
				v1alpha1.PackageRepositoryOperationReasonPackageRepositoryNotFound,
				message,
			)

			if err := r.client.Status().Patch(ctx, operation, client.MergeFrom(operation.DeepCopy())); err != nil {
				return ctrl.Result{}, err
			}

			r.logger.Warn("operation failed", slog.String("name", operation.Name), slog.String("message", message))

			return ctrl.Result{}, nil
		}

		return res, fmt.Errorf("get package repository: %w", err)
	}

	// If packagesToProcess is empty, we need to discover packages
	if len(operation.Status.PackagesToProcess) == 0 {
		return r.discoverPackages(ctx, operation, repo)
	}

	// Process the first package in the queue
	return r.processNextPackage(ctx, operation, repo)
}

func (r *reconciler) discoverPackages(ctx context.Context, operation *v1alpha1.PackageRepositoryOperation, repo *v1alpha1.PackageRepository) (ctrl.Result, error) {
	r.logger.Info("discovering packages", slog.String("repository", repo.Name))

	// Generate registry options
	registryConfig := &utils.RegistryConfig{
		DockerConfig: repo.Spec.Registry.DockerCFG,
		Scheme:       repo.Spec.Registry.Scheme,
		CA:           repo.Spec.Registry.CA,
		UserAgent:    "deckhouse-package-controller",
	}
	opts := utils.GenerateRegistryOptions(registryConfig, r.logger)

	// Create registry client for the packages path
	registryClient, err := r.dc.GetRegistryClient(repo.Spec.Registry.Repo, opts...)
	if err != nil {
		r.logger.Error("failed to create registry client", log.Err(err))

		now := metav1.Now()
		operation.Status.CompletionTime = &now
		message := fmt.Sprintf("Failed to create registry client: %v", err)

		r.SetConditionFalse(
			operation,
			v1alpha1.PackageRepositoryOperationConditionFailed,
			v1alpha1.PackageRepositoryOperationReasonRegistryClientCreationFailed,
			message,
		)

		if err := r.client.Status().Patch(ctx, operation, client.MergeFrom(operation.DeepCopy())); err != nil {
			return ctrl.Result{}, err
		}

		r.logger.Warn("operation failed", slog.String("name", operation.Name), slog.String("message", message))

		return ctrl.Result{}, nil
	}

	// List packages (packages at the packages level)
	packages, err := registryClient.ListTags(ctx)
	if err != nil {
		r.logger.Error("failed to list packages", log.Err(err))

		now := metav1.Now()
		operation.Status.CompletionTime = &now
		message := fmt.Sprintf("Failed to list packages: %v", err)

		r.SetConditionFalse(
			operation,
			v1alpha1.PackageRepositoryOperationConditionFailed,
			v1alpha1.PackageRepositoryOperationReasonPackageListingFailed,
			message,
		)

		if err := r.client.Status().Patch(ctx, operation, client.MergeFrom(operation.DeepCopy())); err != nil {
			return ctrl.Result{}, err
		}

		r.logger.Warn("operation failed", slog.String("name", operation.Name), slog.String("message", message))

		return ctrl.Result{}, nil
	}

	r.logger.Info("discovered packages", slog.Int("count", len(packages)))

	originalRepo := repo.DeepCopy()
	originalOperation := operation.DeepCopy()

	// Build list of packages with their types
	operationStatusPackages := make([]v1alpha1.PackageRepositoryOperationStatusPackageQueue, 0, len(packages))
	repoStatusPackages := make([]v1alpha1.PackageRepositoryStatusPackage, 0, len(packages))
	discoveredPackagesMap := make(map[string]struct{}, len(packages))

	for _, pkg := range packages {
		// Get package type from Docker image label by inspecting manifest
		packageType, err := r.determinePackageType(ctx, repo.Spec.Registry.Repo, pkg, opts)
		if err != nil {
			r.logger.Warn("failed to determine package type, skipping", slog.String("package", pkg), log.Err(err))

			continue
		}

		queueItem := v1alpha1.PackageRepositoryOperationStatusPackageQueue{
			Name: pkg,
			Type: packageType,
		}

		operationStatusPackages = append(operationStatusPackages, queueItem)
		repoStatusPackages = append(repoStatusPackages, v1alpha1.PackageRepositoryStatusPackage(queueItem))

		discoveredPackagesMap[pkg] = struct{}{}
	}

	// Compare with existing packages in PackageRepository status
	existingPackages := make(map[string]bool)
	for _, pkg := range repo.Status.Packages {
		existingPackages[pkg.Name] = true
	}

	for _, pkg := range operationStatusPackages {
		if !existingPackages[pkg.Name] {
			r.logger.Info("new package discovered", slog.String("package", pkg.Name), slog.String("type", pkg.Type))
		}
	}

	for _, pkg := range repo.Status.Packages {
		_, ok := discoveredPackagesMap[pkg.Name]
		if !ok {
			r.logger.Warn("package removed from registry", slog.String("package", pkg.Name))
		}
	}

	repo.Status.Packages = repoStatusPackages
	repo.Status.PackagesCount = len(repoStatusPackages)
	repo.Status.Phase = v1alpha1.PackageRepositoryPhaseActive
	repo.Status.SyncTime = metav1.Now()

	if err := r.client.Status().Patch(ctx, repo, client.MergeFrom(originalRepo)); err != nil {
		return ctrl.Result{}, fmt.Errorf("update package repository status: %w", err)
	}

	// Update operation status with packages to process
	operation.Status.PackagesToProcess = operationStatusPackages
	if operation.Status.Packages == nil {
		operation.Status.Packages = &v1alpha1.PackageRepositoryOperationStatusPackages{}
	}
	operation.Status.Packages.Discovered = len(operationStatusPackages)
	operation.Status.Packages.Total = len(operationStatusPackages)
	operation.Status.Packages.Processed = 0

	if err := r.client.Status().Patch(ctx, operation, client.MergeFrom(originalOperation)); err != nil {
		return ctrl.Result{}, fmt.Errorf("update operation status: %w", err)
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *reconciler) processNextPackage(ctx context.Context, operation *v1alpha1.PackageRepositoryOperation, repo *v1alpha1.PackageRepository) (ctrl.Result, error) {
	if len(operation.Status.PackagesToProcess) == 0 {
		// All packages processed, mark as completed
		return r.markAsCompleted(ctx, operation)
	}

	// Get first package from queue
	currentPackage := operation.Status.PackagesToProcess[0]
	r.logger.Info("processing package",
		slog.String("package", currentPackage.Name),
		slog.String("type", currentPackage.Type))

	// Generate registry options
	registryConfig := &utils.RegistryConfig{
		DockerConfig: repo.Spec.Registry.DockerCFG,
		Scheme:       repo.Spec.Registry.Scheme,
		CA:           repo.Spec.Registry.CA,
		UserAgent:    "deckhouse-package-controller",
	}
	opts := utils.GenerateRegistryOptions(registryConfig, r.logger)

	// Create or update ApplicationPackage or ClusterApplicationPackage
	err := r.ensurePackageResource(ctx, currentPackage.Name, currentPackage.Type, repo.Name)
	if err != nil {
		r.logger.Error("failed to ensure package resource",
			slog.String("package", currentPackage.Name),
			log.Err(err))
		// Continue with next package even if this one fails
	}

	// List versions for this package
	err = r.processPackageVersions(ctx, currentPackage, repo, operation, opts)
	if err != nil {
		r.logger.Error("failed to process package versions",
			slog.String("package", currentPackage.Name),
			log.Err(err))
		// Continue with next package even if this one fails
	}

	// Remove processed package from queue
	var queueEmpty bool
	err = ctrlutils.UpdateStatusWithRetry(ctx, r.client, operation, func() error {
		if len(operation.Status.PackagesToProcess) > 0 {
			operation.Status.PackagesToProcess = operation.Status.PackagesToProcess[1:]
		}
		if operation.Status.Packages != nil {
			operation.Status.Packages.Processed++
		}
		queueEmpty = len(operation.Status.PackagesToProcess) == 0
		return nil
	})
	if err != nil {
		r.logger.Error("failed to update operation status", log.Err(err))
		return ctrl.Result{}, err
	}

	if queueEmpty {
		r.logger.Info("all packages processed, marking as completed",
			slog.Int("total", operation.Status.Packages.Total))
		return r.markAsCompleted(ctx, operation)
	}

	// Requeue to process next package
	return ctrl.Result{Requeue: true}, nil
}

func (r *reconciler) processPackageVersions(ctx context.Context, pkg v1alpha1.PackageRepositoryOperationStatusPackageQueue, repo *v1alpha1.PackageRepository, operation *v1alpha1.PackageRepositoryOperation, opts []cr.Option) error {
	// Create registry client for package versions
	packagePath := path.Join(repo.Spec.Registry.Repo, pkg.Name)
	registryClient, err := r.dc.GetRegistryClient(packagePath, opts...)
	if err != nil {
		return fmt.Errorf("create registry client for package: %w", err)
	}

	var allTags []string
	var scanErr error

	// Handle fullScan vs incremental scan
	if operation.Spec.Update != nil && operation.Spec.Update.FullScan {
		allTags, scanErr = r.performFullScan(ctx, registryClient, pkg.Name)
		if scanErr != nil {
			return scanErr
		}
		r.logger.Info("found package versions",
			slog.String("package", pkg.Name),
			slog.Int("versions", len(allTags)))

		// Create PackageVersion resources for each version
		return r.createPackageVersions(ctx, pkg, repo, allTags)
	}

	allTags, scanErr = r.performIncrementalScan(ctx, registryClient, pkg.Name, pkg.Type, repo.Name)
	if scanErr != nil {
		return scanErr
	}

	r.logger.Info("found package versions",
		slog.String("package", pkg.Name),
		slog.Int("versions", len(allTags)))

	// Create PackageVersion resources for each version
	return r.createPackageVersions(ctx, pkg, repo, allTags)
}

func (r *reconciler) createPackageVersions(ctx context.Context, pkg v1alpha1.PackageRepositoryOperationStatusPackageQueue, repo *v1alpha1.PackageRepository, allTags []string) error {
	for _, versionTag := range allTags {
		// Skip non-version tags (like "release-channel", "version", etc.)
		if !r.isVersionTag(versionTag) {
			continue
		}

		err := r.ensurePackageVersion(ctx, pkg.Name, pkg.Type, versionTag, repo.Name)
		if err != nil {
			r.logger.Warn("failed to create package version",
				slog.String("package", pkg.Name),
				slog.String("version", versionTag),
				log.Err(err))
			// Continue with other versions
			continue
		}
	}

	return nil
}

func (r *reconciler) ensurePackageResource(ctx context.Context, packageName, packageType, repositoryName string) error {
	switch packageType {
	case packageTypeApplication:
		return r.ensureApplicationPackage(ctx, packageName, repositoryName)
	default:
		return fmt.Errorf("unsupported package type: %s", packageType)
	}
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
	return ctrlutils.UpdateStatusWithRetry(ctx, r.client, pkg, func() error {
		if !slices.Contains(pkg.Status.AvailableRepositories, repositoryName) {
			pkg.Status.AvailableRepositories = append(pkg.Status.AvailableRepositories, repositoryName)
		}
		return nil
	})
}

func (r *reconciler) ensurePackageVersion(ctx context.Context, packageName, packageType, version, repositoryName string) error {
	// Generate resource name: <repo>-<package>-<version>
	resourceName := fmt.Sprintf("%s-%s-%s", repositoryName, packageName, strings.TrimPrefix(version, "v"))

	switch packageType {
	case packageTypeApplication:
		return r.ensureApplicationPackageVersion(ctx, resourceName, packageName, version, repositoryName)
	default:
		return fmt.Errorf("unsupported package type: %s", packageType)
	}
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
				PackageName: packageName,
				Version:     version,
				Repository:  repositoryName,
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

func (r *reconciler) isVersionTag(tag string) bool {
	if len(tag) == 0 {
		return false
	}

	// Skip special tags
	if tag == "latest" || tag == "release-channel" || tag == "version" {
		return false
	}

	// Use semver validation for proper version tag detection
	_, err := semver.NewVersion(strings.TrimPrefix(tag, "v"))
	return err == nil
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

func (r *reconciler) performIncrementalScan(ctx context.Context, registryClient cr.Client, packageName, packageType, repositoryName string) ([]string, error) {
	// Incremental scan: start from the last processed version
	r.logger.Debug("performing incremental scan", slog.String("package", packageName))

	lastVersion := r.getLastProcessedVersion(ctx, packageName, packageType, repositoryName)
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

func (r *reconciler) getLastProcessedVersion(ctx context.Context, packageName, packageType, repositoryName string) string {
	// Find the latest PackageVersion for this package from this repository
	var versionList client.ObjectList

	switch packageType {
	case packageTypeApplication:
		versionList = &v1alpha1.ApplicationPackageVersionList{}
	default:
		return ""
	}

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
	case *v1alpha1.ClusterApplicationPackageVersionList:
		for _, item := range list.Items {
			if item.Status.Version != "" {
				versionTags = append(versionTags, item.Status.Version)
			}
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

func (r *reconciler) markAsCompleted(ctx context.Context, operation *v1alpha1.PackageRepositoryOperation) (ctrl.Result, error) {
	original := operation.DeepCopy()

	operation.Status.Phase = v1alpha1.PackageRepositoryOperationPhaseCompleted
	now := metav1.Now()
	operation.Status.CompletionTime = &now

	if err := r.client.Status().Patch(ctx, operation, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, fmt.Errorf("update operation status: %w", err)
	}

	r.logger.Info("operation completed", slog.String("name", operation.Name))

	return ctrl.Result{}, nil
}

func (r *reconciler) handleCompletedState(_ context.Context, operation *v1alpha1.PackageRepositoryOperation) (ctrl.Result, error) {
	r.logger.Debug("operation already completed", slog.String("name", operation.Name))
	return ctrl.Result{}, nil
}

func (r *reconciler) handleFailedState(_ context.Context, operation *v1alpha1.PackageRepositoryOperation) (ctrl.Result, error) {
	r.logger.Debug("operation already failed", slog.String("name", operation.Name))
	return ctrl.Result{}, nil
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
