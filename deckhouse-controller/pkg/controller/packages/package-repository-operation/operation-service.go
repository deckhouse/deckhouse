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
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	transport "github.com/google/go-containerregistry/pkg/v1/remote/transport"
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

// errNoPackageMetadata is returned by detectPackageType when a package
// - has no type labels
// - has no package.yaml in the version image
//
// This typically means the package is a legacy module which lives under packages/ path.
var errNoPackageMetadata = errors.New("package has no type labels and no package.yaml, processed as the legacy module (v1alpha1)")

// errPackageTypeInvalid is returned by detectPackageType when a package has manifest files
// (labels or package.yaml) but the type value is empty or not recognized.
var errPackageTypeInvalid = errors.New("package type could not be determined")

// isRepoNotFoundError checks if the error chain contains a registry NAME_UNKNOWN error,
// which means the repository path does not exist in the registry.
// This is consistent with the pattern used in deckhouse-controller/pkg/registry/module.go.
func isRepoNotFoundError(err error) bool {
	return strings.Contains(err.Error(), string(transport.NameUnknownErrorCode))
}

// packageType represents the type of a package as detected from Docker labels or package.yaml.
type packageType string

const (
	packageTypeApplication packageType = "Application"
	packageTypeModule      packageType = "Module"
)

// parsePackageType converts a raw string to packageType.
//
// returning an error if the value is not recognized. f.e: type: "Garbage", type: ""
func parsePackageType(raw string) (packageType, error) {
	switch packageType(raw) {
	case packageTypeApplication:
		return packageTypeApplication, nil
	case packageTypeModule:
		return packageTypeModule, nil
	default:
		return "", fmt.Errorf("%w: unknown value %q", errPackageTypeInvalid, raw)
	}
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
			// Don't show the legacy module as a package in PackageRepository.status.packages
			continue
		}
		s.repo.Status.Packages = append(s.repo.Status.Packages, v1alpha1.PackageRepositoryStatusPackage{
			Name: pkg.Name,
			Type: pkgType,
		})
	}

	s.repo.Status.PackagesCount = len(s.repo.Status.Packages)
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
		// NAME_UNKNOWN means <package>/version path doesn't exist in the registry.
		// This could be a legacy v1alpha1 module (old registry format) or a broken new package.
		// Try to detect the type by checking the release image at <package>:<tag> directly.
		if isRepoNotFoundError(err) {
			return s.handleMissingVersionPath(ctx, packageName)
		}
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
	latestTag := "v" + foundTags[len(foundTags)-1].String()

	pkgType, err := s.detectPackageType(ctx, packageName, latestTag)
	if err != nil {
		// Probably legacy module (v1alpha1)
		if errors.Is(err, errNoPackageMetadata) {
			return &PackageProcessResult{
				Failed: []failedVersion{{
					Error: err.Error(),
				}},
			}, nil
		}
		if errors.Is(err, errPackageTypeInvalid) {
			return &PackageProcessResult{
				Failed: []failedVersion{{Name: latestTag, Error: err.Error()}},
			}, nil
		}
		// Environment / Network problem
		return nil, err
	}

	var failedVersions = make([]failedVersion, 0)
	for _, versionTag := range foundTags {
		version := "v" + versionTag.String()

		var ensureErr error
		switch pkgType {
		case packageTypeModule:
			ensureErr = s.ensureModulePackageVersion(ctx, packageName, version)
		default:
			ensureErr = s.ensureApplicationPackageVersion(ctx, packageName, version)
		}

		if ensureErr != nil {
			s.logger.Warn("failed to create package version",
				slog.String("package", packageName),
				slog.String("version", version),
				slog.String("type", string(pkgType)),
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
		PackageType: pkgType,
		Done:        foundTags,
		Failed:      failedVersions,
	}, nil
}

// handleMissingVersionPath handles the case when <package>/version path doesn't exist.
// It tries to detect the package type via the release image to distinguish between:
// - A broken new package (has type label on release image) → record as failed with informative error
// - A legacy v1alpha1 module (no type metadata at all) → record as legacy module
func (s *OperationService) handleMissingVersionPath(ctx context.Context, packageName string) (*PackageProcessResult, error) {
	pkg := s.svc.Package(packageName)

	// Get tags from the package path directly (without /version segment)
	tags, err := pkg.ListTags(ctx)
	if err != nil {
		// Can't list tags on <package> either — real connectivity/auth problem
		return nil, fmt.Errorf("list package tags for legacy detection: %w", err)
	}

	if len(tags) == 0 {
		// Package path exists but has no tags — treat as legacy module with no content
		s.logger.Info(
			"package has no version path and no tags, treating as legacy module (v1alpha1)",
			slog.String("package", packageName),
		)
		return &PackageProcessResult{
			Failed: []failedVersion{{
				Error: fmt.Sprintf("%s: %s", errNoPackageMetadata.Error(), packageName),
			}},
		}, nil
	}

	// Try to detect type from release image label using the first available tag
	sampleTag := tags[0]
	releaseConfig, err := pkg.GetImageConfig(ctx, sampleTag)
	if err != nil {
		s.logger.Warn(
			"failed to get release image config for legacy detection",
			slog.String("package", packageName),
			slog.String("tag", sampleTag),
			log.Err(err),
		)
		releaseConfig = nil
	}

	if releaseConfig != nil && releaseConfig.Config.Labels != nil {
		if rawType := releaseConfig.Config.Labels[packageTypeLabel]; rawType != "" {
			// Package HAS a type label but is missing /version path — broken new package (CI issue)
			s.logger.Warn(
				"package has type label but missing /version path",
				slog.String("package", packageName),
				slog.String("type", rawType),
			)
			return &PackageProcessResult{
				Failed: []failedVersion{{
					Error: fmt.Sprintf("package %q has type label %q but /version path does not exist in registry", packageName, rawType),
				}},
			}, nil
		}
	}

	// No type label on release image → legacy v1alpha1 module
	s.logger.Info(
		"package has no version path and no type label, treating as legacy module (v1alpha1)",
		slog.String("package", packageName),
	)
	return &PackageProcessResult{
		Failed: []failedVersion{{
			Error: fmt.Sprintf("%s: %s", errNoPackageMetadata.Error(), packageName),
		}},
	}, nil
}

// detectPackageType determines the package type using the following strategy:
//
//  1. Read label from version image ConfigFile (<package>/version:<tag>)
//  2. Read label from artifact bundle ConfigFile (<package>:<tag>)
//  3. Fallback: extract package.yaml from version image (~500 B tar)
//     - Found with known type → use type from package.yaml
//     - Found without type → anomalous, record as failed (it's a package but without a proper type)
//     - package.yaml Not found → skip (not a new-style package)
//
// At each step, found values are validated via parsePackageType
// unknown values (e.g. unrecognized type in label) result in errPackageTypeInvalid
//
// Returns:
//   - (packageTypeApplication or packageTypeModule, nil) — valid type detected
//   - ("", errNoPackageMetadata) — no labels and no package.yaml, skip as legacy module
//   - ("", errPackageTypeInvalid) — type could not be determined or is unknown
//   - ("", err) — hard error (network, tar corruption, etc.)
func (s *OperationService) detectPackageType(ctx context.Context, packageName, latestTag string) (packageType, error) {
	pkg := s.svc.Package(packageName)

	// Step 1: Read label from version image ConfigFile (<package>/version:<tag>)
	versionConfig, err := pkg.Versions().GetImageConfig(ctx, latestTag)
	if err != nil {
		s.logger.Warn("failed to get version image config",
			slog.String("package", packageName),
			log.Err(err))
		versionConfig = nil
	}
	if versionConfig != nil && versionConfig.Config.Labels != nil {
		if rawPackageType := versionConfig.Config.Labels[packageTypeLabel]; rawPackageType != "" {
			return parsePackageType(rawPackageType)
		}
	}

	// Step 2: Read label from artifact bundle ConfigFile (<package>:<tag>)
	releaseConfig, err := pkg.GetImageConfig(ctx, latestTag)
	if err != nil {
		s.logger.Warn("failed to get release image config",
			slog.String("package", packageName),
			log.Err(err))
		releaseConfig = nil
	}
	if releaseConfig != nil && releaseConfig.Config.Labels != nil {
		if raw := releaseConfig.Config.Labels[packageTypeLabel]; raw != "" {
			return parsePackageType(raw)
		}
	}

	// Step 3: No labels — fall back to package.yaml from version image
	pkgDef, err := pkg.Versions().ReadPackageDefinition(ctx, latestTag)
	if err != nil {
		return "", fmt.Errorf("read package definition: %w", err)
	}

	// The image we process doesn't have any sign that it's a package
	if pkgDef == nil {
		// TODO(Glitchy-Sheep): implement legacy module handling via different registry path
		s.logger.Info("no package type label and no package.yaml, skipping",
			slog.String("package", packageName))
		return "", fmt.Errorf("%w: %s", errNoPackageMetadata, packageName)
	}

	if pkgDef.Type != "" {
		s.logger.Warn("package type label not found on any image, using type from package.yaml",
			slog.String("package", packageName),
			slog.String("type", pkgDef.Type))
		return parsePackageType(pkgDef.Type)
	}

	// package.yaml exists but type field is empty
	s.logger.Warn("package type not determined from labels or package.yaml",
		slog.String("package", packageName))
	return "", fmt.Errorf("%w: %s", errPackageTypeInvalid, packageName)
}

type PackageProcessResult struct {
	PackageType packageType
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
