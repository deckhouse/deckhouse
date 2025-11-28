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
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	registryService "github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry/service"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
	regClient "github.com/deckhouse/deckhouse/pkg/registry/client"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type OperationService struct {
	client client.Client
	repo   *v1alpha1.PackageRepository
	svc    *registryService.PackagesService

	logger *log.Logger
}

func NewOperationService(ctx context.Context, client client.Client, repoName string, psm *registryService.PackageServiceManager, logger *log.Logger) (*OperationService, error) {
	repo := &v1alpha1.PackageRepository{}
	err := client.Get(ctx, types.NamespacedName{Name: repoName}, repo)
	if err != nil {
		return nil, fmt.Errorf("get package repository: %w", err)
	}

	// Create registry service for the packages path
	svc, err := psm.PackagesService(
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

type discoverResult struct {
	Packages        []packageInfo
	RepositoryPhase string
	SyncTime        time.Time
}

type packageInfo struct {
	Name string
	Type string
}

func (s *OperationService) DiscoverPackage(ctx context.Context) (*discoverResult, error) {
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

	res := &discoverResult{
		Packages:        discoveredPackages,
		RepositoryPhase: v1alpha1.PackageRepositoryPhaseActive,
		SyncTime:        time.Now(),
	}

	return res, nil
}

func (s *OperationService) foundTagsToProcess(ctx context.Context, packageName string, operation *v1alpha1.PackageRepositoryOperation) ([]string, error) {
	var foundTags []string
	var err error

	// Handle fullScan vs incremental scan
	if operation.Spec.Update != nil && operation.Spec.Update.FullScan {
		foundTags, err = s.svc.Package(packageName).ListTags(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list package tags: %w", err)
		}

		return foundTags, nil
	}

	foundTags, err = s.performIncrementalScan(ctx, packageName)
	if err != nil {
		return nil, err
	}

	return foundTags, nil
}

func (s *OperationService) performIncrementalScan(ctx context.Context, packageName string) ([]string, error) {
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

func (s *OperationService) listTagsFromVersion(ctx context.Context, packageName string, lastVersion string) ([]string, error) {
	// List all tags from the registry and filter those that are greater than lastVersion
	// WARNING! it works only if your registry supports tag listing with filtering by last version
	allTags, err := s.svc.Package(packageName).ListTags(ctx, regClient.WithTagsLast(lastVersion))
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	// Filter tags to only include versions after lastVersion
	var newTags []string
	lastVer, err := semver.NewVersion(lastVersion)
	if err != nil {
		// If we can't parse last version, return all tags
		return allTags, nil
	}

	for _, tag := range allTags {
		tagVer, err := semver.NewVersion(tag)
		if err != nil {
			continue
		}

		// Only include tags that are newer than lastVersion
		if tagVer.GreaterThan(lastVer) {
			newTags = append(newTags, tag)
		}
	}

	// double check for registries that do not support filtering
	// to warn user about it
	if len(newTags) != len(allTags) {
		s.logger.Info("looks like your registry does not support tag listing with filtering by last version",
			slog.String("package", packageName),
			slog.String("lastVersion", lastVersion),
			slog.Int("allTagsCount", len(allTags)),
			slog.Int("newTagsCount", len(newTags)))
	}

	return newTags, nil
}

func (s *OperationService) getLastProcessedVersion(ctx context.Context, packageName string) string {
	// Find the latest PackageVersion for this package from this repository
	var versionList client.ObjectList = &v1alpha1.ApplicationPackageVersionList{}

	err := s.client.List(ctx, versionList, client.MatchingLabels{
		"repository": s.repo.Name,
		"package":    packageName,
	})
	if err != nil {
		s.logger.Warn("failed to list package versions",
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
			s.logger.Warn("unsupported package version list type",
				slog.String("package", packageName))

			return ""
		}
	}

	// Parse all versions
	for _, vTag := range versionTags {
		v, err := semver.NewVersion(vTag)
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

func (s *OperationService) ProcessPackageVersions(ctx context.Context, packageName string, operation *v1alpha1.PackageRepositoryOperation) error {
	var foundTags []string

	foundTags, err := s.foundTagsToProcess(ctx, packageName, operation)
	if err != nil {
		return fmt.Errorf("get found tags to process: %w", err)
	}

	s.logger.Info("found package versions",
		slog.String("package", packageName),
		slog.Int("versions", len(foundTags)))

	err = s.ensurePackageVersionForTags(ctx, packageName, foundTags)
	if err != nil {
		return fmt.Errorf("ensure package versions for tags: %w", err)
	}

	return nil
}

type processResult struct {
	Failed []failedPackage
}

type failedPackage struct {
	Name  string
	Error error
}

// TODO replace tags with semver.Version
func (s *OperationService) ensurePackageVersionForTags(ctx context.Context, packageName string, tags []string) error {
	for _, versionTag := range tags {
		// Skip non-version tags (like "release-channel", "version", etc.)
		_, err := semver.NewVersion(strings.TrimPrefix(versionTag, "v"))
		if err != nil {
			s.logger.Debug("skipping non-version tag",
				slog.String("package", packageName),
				slog.String("tag", versionTag),
				log.Err(err))

			continue
		}

		err = s.ensureApplicationPackageVersion(ctx, packageName, versionTag)
		if err != nil {
			s.logger.Warn("failed to create package version",
				slog.String("package", packageName),
				slog.String("version", versionTag),
				log.Err(err))

			continue
		}
	}

	return nil
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
			PackageName:       packageName,
			Version:           version,
			PackageRepository: s.repo.Name,
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

	// resource already exists
	if err == nil {
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
