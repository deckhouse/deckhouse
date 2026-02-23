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
	"slices"
	"time"

	"github.com/Masterminds/semver/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryService "github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry/service"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
	regClient "github.com/deckhouse/deckhouse/pkg/registry/client"
)

type OperationService struct {
	client client.Client
	repo   *v1alpha1.PackageRepository
	svc    *registryService.PackagesService

	logger *log.Logger
}

func NewOperationService(ctx context.Context, client client.Client, repoName string, psm registryService.ServiceManagerInterface[registryService.PackagesService], logger *log.Logger) (*OperationService, error) {
	repo := &v1alpha1.PackageRepository{}
	err := client.Get(ctx, types.NamespacedName{Name: repoName}, repo)
	if err != nil {
		return nil, fmt.Errorf("get package repository: %w", err)
	}

	// Create registry service for the packages path
	svc, err := psm.Service(
		repo.Spec.Registry.Repo,
		repo.Spec.Registry.DockerCFG,
		repo.Spec.Registry.CA,
		"deckhouse-package-controller",
		repo.Spec.Registry.Scheme,
	)
	if err != nil {
		return nil, fmt.Errorf("create package service: %w", err)
	}

	return &OperationService{
		client: client,
		repo:   repo,
		svc:    svc,
		logger: logger,
	}, nil
}

type DiscoverResult struct {
	Packages        []packageInfo
	RepositoryPhase string
	SyncTime        time.Time
}

type packageInfo struct {
	Name string
	Type string
}

func (s *OperationService) DiscoverPackage(ctx context.Context) (*DiscoverResult, error) {
	// List packages (packages at the packages level)
	packages, err := s.svc.ListTags(ctx)
	if err != nil {
		s.logger.Error("failed to list packages", log.Err(err))

		return nil, fmt.Errorf("failed to list packages: %w", err)
	}

	s.logger.Info("discovered packages", slog.Int("count", len(packages)))

	discoveredPackages := make([]packageInfo, 0, len(packages))

	for _, pkg := range packages {
		discoveredPackages = append(discoveredPackages, packageInfo{
			Name: pkg,
		})
	}

	res := &DiscoverResult{
		Packages:        discoveredPackages,
		RepositoryPhase: v1alpha1.PackageRepositoryPhaseActive,
		SyncTime:        time.Now(),
	}

	return res, nil
}

// UpdateRepositoryStatus updates the PackageRepository status with the processed packages
func (s *OperationService) UpdateRepositoryStatus(ctx context.Context, packages []v1alpha1.PackageRepositoryOperationStatusPackage) error {
	original := s.repo.DeepCopy()

	s.repo.Status.Packages = make([]v1alpha1.PackageRepositoryStatusPackage, 0, len(packages))

	for _, pkg := range packages {
		pkgType := pkg.Type
		if pkgType == "" {
			pkgType = packageTypeApplication
		}
		s.repo.Status.Packages = append(s.repo.Status.Packages, v1alpha1.PackageRepositoryStatusPackage{
			Name: pkg.Name,
			Type: pkgType,
		})
	}

	s.repo.Status.PackagesCount = len(packages)
	s.repo.Status.Phase = v1alpha1.PackageRepositoryPhaseActive
	s.repo.Status.SyncTime = metav1.NewTime(time.Now())

	if err := s.client.Status().Patch(ctx, s.repo, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("update repository status: %w", err)
	}

	return nil
}

func (s *OperationService) foundTagsToProcess(ctx context.Context, packageName string, operation *v1alpha1.PackageRepositoryOperation) ([]*semver.Version, error) {
	// Handle fullScan vs incremental scan
	if operation.Spec.Update != nil && operation.Spec.Update.FullScan {
		rawTags, err := s.svc.Package(packageName).Versions().ListTags(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list package tags: %w", err)
		}

		foundTags := extractOnlySemverTags(rawTags)

		return foundTags, nil
	}

	foundTags, err := s.performIncrementalScan(ctx, packageName)
	if err != nil {
		return nil, err
	}

	return foundTags, nil
}

func (s *OperationService) performIncrementalScan(ctx context.Context, packageName string) ([]*semver.Version, error) {
	// Incremental scan: start from the last processed version
	s.logger.Debug("performing incremental scan", slog.String("package", packageName))

	lastVersion := s.getLastProcessedVersion(ctx, packageName)
	if lastVersion != "" {
		s.logger.Debug("found last processed version",
			slog.String("package", packageName),
			slog.String("lastVersion", lastVersion))
	}

	tags, err := s.listTagsFromVersion(ctx, packageName, lastVersion)
	if err != nil {
		return nil, fmt.Errorf("list tags from version: %w", err)
	}

	return tags, nil
}

func extractOnlySemverTags(rawTags []string) []*semver.Version {
	allTags := make([]*semver.Version, 0, len(rawTags))
	for _, tag := range rawTags {
		// filter all non semver tags here
		tagVer, err := semver.NewVersion(tag)
		if err != nil {
			continue
		}

		allTags = append(allTags, tagVer)
	}

	return allTags
}

func (s *OperationService) listTagsFromVersion(ctx context.Context, packageName string, lastVersion string) ([]*semver.Version, error) {
	// List all tags from the registry and filter those that are greater than lastVersion
	// WARNING! it works only if your registry supports tag listing with filtering by last version
	rawTags, err := s.svc.Package(packageName).Versions().ListTags(ctx, regClient.WithTagsLast(lastVersion))
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	allTags := extractOnlySemverTags(rawTags)

	// Filter tags to only include versions after lastVersion
	lastVer, err := semver.NewVersion(lastVersion)
	if err != nil {
		// If we can't parse last version, return all tags
		return allTags, nil
	}

	var newTags []*semver.Version
	for _, tag := range allTags {
		// Only include tags that are newer than lastVersion
		if tag.GreaterThan(lastVer) {
			newTags = append(newTags, tag)
		}
	}

	// double check for registries that do not support filtering
	// to warn user about it
	if len(newTags) != len(rawTags) {
		s.logger.Info("looks like your registry does not support tag listing with filtering by last version",
			slog.String("package", packageName),
			slog.String("lastVersion", lastVersion),
			slog.Int("allTagsCount", len(rawTags)),
			slog.Int("newTagsCount", len(newTags)))
	}

	return newTags, nil
}

func (s *OperationService) getLastProcessedVersion(ctx context.Context, packageName string) string {
	// Query both ApplicationPackageVersion and ModulePackageVersion lists since
	// the package type is not known yet at this point.
	var versions []*semver.Version

	matchLabels := client.MatchingLabels{
		v1alpha1.ApplicationPackageVersionLabelRepository: s.repo.Name,
		v1alpha1.ApplicationPackageVersionLabelPackage:    packageName,
	}

	appList := &v1alpha1.ApplicationPackageVersionList{}
	if err := s.client.List(ctx, appList, matchLabels); err != nil {
		s.logger.Warn("failed to list application package versions", slog.String("package", packageName), log.Err(err))
	}
	for _, item := range appList.Items {
		if v := parseProcessedVersion(item.Spec.PackageVersion, item.Status.PackageMetadata != nil); v != nil {
			versions = append(versions, v)
		}
	}

	modList := &v1alpha1.ModulePackageVersionList{}
	if err := s.client.List(ctx, modList, matchLabels); err != nil {
		s.logger.Warn("failed to list module package versions", slog.String("package", packageName), log.Err(err))
	}
	for _, item := range modList.Items {
		if v := parseProcessedVersion(item.Spec.PackageVersion, item.Status.PackageMetadata != nil); v != nil {
			versions = append(versions, v)
		}
	}

	return latestVersionString(versions)
}

func parseProcessedVersion(tag string, hasMetadata bool) *semver.Version {
	if !hasMetadata {
		return nil
	}
	v, _ := semver.NewVersion(tag)
	return v
}

func latestVersionString(versions []*semver.Version) string {
	if len(versions) == 0 {
		return ""
	}
	slices.SortFunc(versions, func(a, b *semver.Version) int { return a.Compare(b) })
	return "v" + versions[len(versions)-1].String()
}

func (s *OperationService) ProcessPackageVersions(ctx context.Context, packageName string, operation *v1alpha1.PackageRepositoryOperation) (*PackageProcessResult, error) {
	foundTags, err := s.foundTagsToProcess(ctx, packageName, operation)
	if err != nil {
		return nil, fmt.Errorf("get found tags to process: %w", err)
	}

	s.logger.Info("found package versions",
		slog.String("package", packageName),
		slog.Int("versions", len(foundTags)))

	// If no tags found, return empty result
	if len(foundTags) == 0 {
		return &PackageProcessResult{
			Done:   nil,
			Failed: nil,
		}, nil
	}

	// Sort tags to pick the latest version for label check (older versions may have outdated/missing labels)
	slices.SortFunc(foundTags, func(a, b *semver.Version) int { return a.Compare(b) })
	img, err := s.svc.Package(packageName).Versions().GetImage(ctx, "v"+foundTags[len(foundTags)-1].String())
	if err != nil {
		return nil, fmt.Errorf("get package image: %w", err)
	}

	configFile, err := img.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("get package image config file: %w", err)
	}

	var packageType string
	if configFile != nil && configFile.Config.Labels != nil {
		packageType = configFile.Config.Labels[packageTypeLabel]
	}

	var failedVersions = make([]failedVersion, 0)
	for _, versionTag := range foundTags {
		version := "v" + versionTag.String()

		var ensureErr error
		switch packageType {
		case packageTypeModule:
			ensureErr = s.ensureModulePackageVersion(ctx, packageName, version)
		default:
			ensureErr = s.ensureApplicationPackageVersion(ctx, packageName, version)
		}

		if ensureErr != nil {
			s.logger.Warn("failed to create package version",
				slog.String("package", packageName),
				slog.String("version", version),
				slog.String("type", packageType),
				log.Err(ensureErr),
			)

			failedVersions = append(failedVersions, failedVersion{
				Name:  version,
				Error: "ensure package version: " + ensureErr.Error(),
			})

			continue
		}
	}

	return &PackageProcessResult{
		PackageType: packageType,
		Done:        foundTags,
		Failed:      failedVersions,
	}, nil
}

type PackageProcessResult struct {
	PackageType string
	Done        []*semver.Version
	Failed      []failedVersion
}

type failedVersion struct {
	Name  string
	Error string
}

func (s *OperationService) ensureApplicationPackageVersion(ctx context.Context, packageName, version string) error {
	apvName := v1alpha1.MakeApplicationPackageVersionName(s.repo.Name, packageName, version)

	pkgVersion := &v1alpha1.ApplicationPackageVersion{}
	err := s.client.Get(ctx, types.NamespacedName{Name: apvName}, pkgVersion)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("get application package version: %w", err)
	}

	// Version already exists
	if err == nil {
		return nil
	}

	// Create new ApplicationPackageVersion with draft label
	pkgVersion = &v1alpha1.ApplicationPackageVersion{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.ApplicationPackageVersionGVK.GroupVersion().String(),
			Kind:       v1alpha1.ApplicationPackageVersionKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: apvName,
			Labels: map[string]string{
				"heritage": "deckhouse",
				v1alpha1.ApplicationPackageVersionLabelRepository: s.repo.Name,
				v1alpha1.ApplicationPackageVersionLabelPackage:    packageName,
				v1alpha1.ApplicationPackageVersionLabelDraft:      "true",
			},
		},
		Spec: v1alpha1.ApplicationPackageVersionSpec{
			PackageName:           packageName,
			PackageVersion:        version,
			PackageRepositoryName: s.repo.Name,
		},
	}

	// Add owner reference to PackageRepository
	s.setOwnerReference(pkgVersion)

	err = s.client.Create(ctx, pkgVersion)
	if err != nil {
		return fmt.Errorf("create application package version: %w", err)
	}

	return nil
}

func (s *OperationService) EnsureApplicationPackage(ctx context.Context, packageName string) error {
	pkg := &v1alpha1.ApplicationPackage{}
	err := s.client.Get(ctx, types.NamespacedName{Name: packageName}, pkg)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("get application package: %w", err)
	}

	// err - apierrors.IsNotFound
	if err != nil {
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
		s.setOwnerReference(pkg)

		err = s.client.Create(ctx, pkg)
		if err != nil {
			return fmt.Errorf("create application package: %w", err)
		}
	}

	// Check if repository is already listed
	if slices.Contains(pkg.Status.AvailableRepositories, s.repo.Name) {
		return nil
	}

	// Update existing package to add repository to available repositories
	original := pkg.DeepCopy()

	pkg.Status.AvailableRepositories = append(pkg.Status.AvailableRepositories, s.repo.Name)

	err = s.client.Status().Patch(ctx, pkg, client.MergeFrom(original))
	if err != nil {
		return fmt.Errorf("update application package status: %w", err)
	}

	return nil
}

func (s *OperationService) ensureModulePackageVersion(ctx context.Context, packageName, version string) error {
	mpvName := v1alpha1.MakeModulePackageVersionName(s.repo.Name, packageName, version)

	pkgVersion := &v1alpha1.ModulePackageVersion{}
	err := s.client.Get(ctx, types.NamespacedName{Name: mpvName}, pkgVersion)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("get module package version: %w", err)
	}

	// Version already exists
	if err == nil {
		return nil
	}

	// Create new ModulePackageVersion with draft label
	pkgVersion = &v1alpha1.ModulePackageVersion{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.ModulePackageVersionGVK.GroupVersion().String(),
			Kind:       v1alpha1.ModulePackageVersionKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: mpvName,
			Labels: map[string]string{
				"heritage": "deckhouse",
				v1alpha1.ModulePackageVersionLabelRepository: s.repo.Name,
				v1alpha1.ModulePackageVersionLabelPackage:    packageName,
				v1alpha1.ModulePackageVersionLabelDraft:      "true",
			},
		},
		Spec: v1alpha1.ModulePackageVersionSpec{
			PackageName:           packageName,
			PackageVersion:        version,
			PackageRepositoryName: s.repo.Name,
		},
	}

	// Add owner reference to PackageRepository
	s.setOwnerReference(pkgVersion)

	err = s.client.Create(ctx, pkgVersion)
	if err != nil {
		return fmt.Errorf("create module package version: %w", err)
	}

	return nil
}

func (s *OperationService) EnsureModulePackage(ctx context.Context, packageName string) error {
	pkg := &v1alpha1.ModulePackage{}
	err := s.client.Get(ctx, types.NamespacedName{Name: packageName}, pkg)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("get module package: %w", err)
	}

	// err - apierrors.IsNotFound
	if err != nil {
		// Create new ModulePackage
		pkg = &v1alpha1.ModulePackage{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.ModulePackageGVK.GroupVersion().String(),
				Kind:       v1alpha1.ModulePackageKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: packageName,
				Labels: map[string]string{
					"heritage": "deckhouse",
				},
			},
		}

		// Add owner reference to PackageRepository
		s.setOwnerReference(pkg)

		err = s.client.Create(ctx, pkg)
		if err != nil {
			return fmt.Errorf("create module package: %w", err)
		}
	}

	// Check if repository is already listed
	if slices.Contains(pkg.Status.AvailableRepositories, s.repo.Name) {
		return nil
	}

	// Update existing package to add repository to available repositories
	original := pkg.DeepCopy()

	pkg.Status.AvailableRepositories = append(pkg.Status.AvailableRepositories, s.repo.Name)

	err = s.client.Status().Patch(ctx, pkg, client.MergeFrom(original))
	if err != nil {
		return fmt.Errorf("update module package status: %w", err)
	}

	return nil
}

func (s *OperationService) setOwnerReference(obj client.Object) {
	ownerRef := metav1.OwnerReference{
		APIVersion: v1alpha1.PackageRepositoryGVK.GroupVersion().String(),
		Kind:       v1alpha1.PackageRepositoryKind,
		Name:       s.repo.Name,
		UID:        s.repo.UID,
		Controller: &[]bool{true}[0],
	}

	obj.SetOwnerReferences([]metav1.OwnerReference{ownerRef})
}
