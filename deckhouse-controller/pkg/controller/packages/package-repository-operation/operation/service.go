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

package operation

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
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/pkg/log"
	regClient "github.com/deckhouse/deckhouse/pkg/registry/client"
)

const (
	// packageTypeLabel identifies the package type in an OCI image label.
	packageTypeLabel = "io.deckhouse.package.type"
)

// errPackageTypeInvalid indicates that package metadata contains an empty or unsupported type.
var errPackageTypeInvalid = errors.New("package type could not be determined")

// errTooOldImage indicates that a version image has neither a type label nor a package definition.
var errTooOldImage = errors.New("version image has no type labels and no package.yaml")

// isRepoNotFoundError reports whether err indicates a missing registry repository path.
func isRepoNotFoundError(err error) bool {
	return strings.Contains(err.Error(), string(transport.NameUnknownErrorCode))
}

// PackageType identifies the package kind declared by an OCI image label or package definition.
type PackageType string

const (
	// PackageTypeApplication identifies an application package.
	PackageTypeApplication PackageType = "Application"
	// PackageTypeModule identifies a module package.
	PackageTypeModule PackageType = "Module"
)

// parsePackageType parses a supported package type.
func parsePackageType(raw string) (PackageType, error) {
	switch PackageType(raw) {
	case PackageTypeApplication:
		return PackageTypeApplication, nil
	case PackageTypeModule:
		return PackageTypeModule, nil
	default:
		return "", fmt.Errorf("%w: unknown value %q", errPackageTypeInvalid, raw)
	}
}

// Service discovers packages in a repository and persists their Kubernetes resources.
type Service struct {
	client client.Client
	repo   *v1alpha1.PackageRepository
	svc    *registryService.PackagesService

	logger *log.Logger
}

// NewService creates a package repository operation service for repoName.
func NewService(ctx context.Context, client client.Client, repoName string, psm registryService.ServiceManagerInterface[registryService.PackagesService], logger *log.Logger) (*Service, error) {
	repo := &v1alpha1.PackageRepository{}
	err := client.Get(ctx, types.NamespacedName{Name: repoName}, repo)
	if err != nil {
		return nil, fmt.Errorf("get package repository: %w", err)
	}

	svc, err := psm.Service(repo.Spec.Registry.Repo, utils.RegistryConfig{
		DockerConfig: repo.Spec.Registry.DockerCFG,
		Login:        repo.Spec.Registry.Login,
		Password:     repo.Spec.Registry.Password,
		CA:           repo.Spec.Registry.CA,
		Scheme:       repo.Spec.Registry.Scheme,
		UserAgent:    "deckhouse-package-controller",
	})
	if err != nil {
		return nil, fmt.Errorf("create package service: %w", err)
	}

	return &Service{
		client: client,
		repo:   repo,
		svc:    svc,
		logger: logger,
	}, nil
}

// DiscoverResult describes the packages and repository state found during discovery.
type DiscoverResult struct {
	Packages        []packageInfo
	RepositoryPhase string
	SyncTime        time.Time
}

// packageInfo identifies a package discovered in the repository.
type packageInfo struct {
	Name string
	Type string
}

// DiscoverPackage lists packages available in the configured repository.
func (s *Service) DiscoverPackage(ctx context.Context) (*DiscoverResult, error) {
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

// UpdateRepositoryStatus updates the PackageRepository status with the processed packages.
func (s *Service) UpdateRepositoryStatus(ctx context.Context, packages []v1alpha1.PackageRepositoryOperationStatusPackage) error {
	original := s.repo.DeepCopy()

	// Type is stable per package; recover it from the previous status when an incremental scan returned empty.
	cachedTypes := make(map[string]string, len(s.repo.Status.Packages))
	for _, p := range s.repo.Status.Packages {
		cachedTypes[p.Name] = p.Type
	}

	s.repo.Status.Packages = make([]v1alpha1.PackageRepositoryStatusPackage, 0, len(packages))

	var newVersionsTotal int
	for _, pkg := range packages {
		newVersionsTotal += pkg.NewVersions

		pkgType := pkg.Type
		if pkgType == "" {
			pkgType = cachedTypes[pkg.Name]
		}
		if pkgType == "" {
			continue
		}
		s.repo.Status.Packages = append(s.repo.Status.Packages, v1alpha1.PackageRepositoryStatusPackage{
			Name: pkg.Name,
			Type: pkgType,
		})
	}

	now := metav1.NewTime(time.Now())

	s.repo.Status.PackagesCount = len(s.repo.Status.Packages)
	s.repo.Status.Phase = v1alpha1.PackageRepositoryPhaseActive

	s.repo.Status.LastScanTime = &now
	s.repo.Status.LastNewVersions = newVersionsTotal

	// LastChangeTime is preserved across scans that find nothing new, so only
	// advance it when the current scan actually found versions.
	if newVersionsTotal > 0 {
		s.repo.Status.LastChangeTime = &now
	}

	if err := s.client.Status().Patch(ctx, s.repo, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("update repository status: %w", err)
	}

	return nil
}

// foundTagsToProcess returns package versions selected by the requested scan mode.
func (s *Service) foundTagsToProcess(ctx context.Context, packageName string, operation *v1alpha1.PackageRepositoryOperation) ([]*semver.Version, error) {
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

// performIncrementalScan returns package versions newer than the last processed version.
func (s *Service) performIncrementalScan(ctx context.Context, packageName string) ([]*semver.Version, error) {
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

// extractOnlySemverTags parses semantic-version tags and discards all other tags.
func extractOnlySemverTags(rawTags []string) []*semver.Version {
	allTags := make([]*semver.Version, 0, len(rawTags))
	for _, tag := range rawTags {
		tagVer, err := semver.NewVersion(tag)
		if err != nil {
			continue
		}

		allTags = append(allTags, tagVer)
	}

	return allTags
}

// listTagsFromVersion lists package tags newer than lastVersion.
func (s *Service) listTagsFromVersion(ctx context.Context, packageName string, lastVersion string) ([]*semver.Version, error) {
	rawTags, err := s.svc.Package(packageName).Versions().ListTags(ctx, regClient.WithTagsLast(lastVersion))
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	allTags := extractOnlySemverTags(rawTags)

	lastVer, err := semver.NewVersion(lastVersion)
	if err != nil {
		// An empty or invalid checkpoint requires processing every semantic version.
		return allTags, nil
	}

	var newTags []*semver.Version
	for _, tag := range allTags {
		if tag.GreaterThan(lastVer) {
			newTags = append(newTags, tag)
		}
	}

	// The client-side result exposes registries that ignored the server-side last-tag filter.
	if len(newTags) != len(rawTags) {
		s.logger.Info("looks like your registry does not support tag listing with filtering by last version",
			slog.String("package", packageName),
			slog.String("lastVersion", lastVersion),
			slog.Int("allTagsCount", len(rawTags)),
			slog.Int("newTagsCount", len(newTags)))
	}

	return newTags, nil
}

// getLastProcessedVersion returns the newest successfully processed package version.
func (s *Service) getLastProcessedVersion(ctx context.Context, packageName string) string {
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

// parseProcessedVersion parses tag when the corresponding resource has package metadata.
func parseProcessedVersion(tag string, hasMetadata bool) *semver.Version {
	if !hasMetadata {
		return nil
	}
	v, _ := semver.NewVersion(tag)
	return v
}

// latestVersionString returns the newest version with a leading v or an empty string.
func latestVersionString(versions []*semver.Version) string {
	if len(versions) == 0 {
		return ""
	}
	slices.SortFunc(versions, func(a, b *semver.Version) int { return a.Compare(b) })
	return "v" + versions[len(versions)-1].String()
}

// ProcessPackageVersions discovers package versions and ensures their version resources exist.
// It falls back to legacy module releases when a full scan finds no semantic versions.
func (s *Service) ProcessPackageVersions(ctx context.Context, packageName string, operation *v1alpha1.PackageRepositoryOperation) (*PackageProcessResult, error) {
	foundTags, err := s.foundTagsToProcess(ctx, packageName, operation)
	if err != nil {
		// Legacy modules publish releases without a version repository path.
		if isRepoNotFoundError(err) {
			return s.handleMissingVersionPath(ctx, packageName)
		}
		return nil, fmt.Errorf("get found tags to process: %w", err)
	}

	s.logger.Info(
		"found package versions",
		slog.String("package", packageName),
		slog.Int("versions", len(foundTags)),
	)

	// A full scan with no semantic versions may indicate a legacy release layout.
	// An incremental scan with no versions only means nothing changed since the checkpoint.
	if len(foundTags) == 0 {
		if operation.Spec.Update != nil && operation.Spec.Update.FullScan {
			s.logger.Warn(
				"no semver tags found in /version path for package, falling back to /release",
				slog.String("package", packageName),
			)
			return s.handleMissingVersionPath(ctx, packageName)
		}

		return &PackageProcessResult{}, nil
	}

	// The newest image is authoritative because older images may lack current type metadata.
	slices.SortFunc(foundTags, func(a, b *semver.Version) int { return a.Compare(b) })
	latestTag := "v" + foundTags[len(foundTags)-1].String()

	pkgType, detectErr := s.detectPackageType(ctx, packageName, latestTag)
	if detectErr != nil {
		if errors.Is(detectErr, errTooOldImage) {
			return &PackageProcessResult{
				Failed: []failedVersion{{
					Error: detectErr.Error(),
				}},
			}, nil
		}
		if errors.Is(detectErr, errPackageTypeInvalid) {
			return &PackageProcessResult{
				Failed: []failedVersion{{Name: latestTag, Error: detectErr.Error()}},
			}, nil
		}
		return nil, detectErr
	}

	failedVersions := make([]failedVersion, 0)
	var newVersions int
	for _, versionTag := range foundTags {
		version := "v" + versionTag.String()

		var ensureErr error
		var isNew bool
		switch pkgType {
		case PackageTypeModule:
			isNew, ensureErr = s.ensureModulePackageVersion(ctx, packageName, version, nil)
		default:
			isNew, ensureErr = s.ensureApplicationPackageVersion(ctx, packageName, version)
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

		if isNew {
			newVersions++
		}
	}

	return &PackageProcessResult{
		PackageType:   pkgType,
		Done:          foundTags,
		Failed:        failedVersions,
		FoundVersions: len(foundTags),
		NewVersions:   newVersions,
	}, nil
}

// handleMissingVersionPath discovers legacy module releases when a package has no usable version path.
func (s *Service) handleMissingVersionPath(ctx context.Context, packageName string) (*PackageProcessResult, error) {
	s.logger.Info(
		"no semver tags in /version path, checking /release for legacy module (v1alpha1)",
		slog.String("package", packageName),
	)

	rawTags, err := s.svc.Package(packageName).Release().ListTags(ctx)
	if err != nil {
		if isRepoNotFoundError(err) {
			s.logger.Warn(
				"package has neither /version nor /release path",
				slog.String("package", packageName),
			)
			return &PackageProcessResult{
				Failed: []failedVersion{{
					Error: fmt.Sprintf("package %q has neither /version nor /release path", packageName),
				}},
			}, nil
		}
		return nil, fmt.Errorf("list release tags: %w", err)
	}

	foundTags := extractOnlySemverTags(rawTags)

	if len(foundTags) == 0 {
		s.logger.Warn(
			"no semver tags found in /release path for package",
			slog.String("package", packageName),
		)
		return &PackageProcessResult{
			Failed: []failedVersion{{
				Error: fmt.Sprintf("no semver release tags found for legacy module %q", packageName),
			}},
		}, nil
	}

	s.logger.Info(
		"found legacy module versions in /release path",
		slog.String("package", packageName),
		slog.Int("versions", len(foundTags)),
	)

	legacyLabels := map[string]string{
		v1alpha1.ModulePackageVersionLabelLegacy: "true",
	}

	var failedVersions []failedVersion
	var newVersions int
	for _, versionTag := range foundTags {
		version := "v" + versionTag.String()

		isNew, ensureErr := s.ensureModulePackageVersion(ctx, packageName, version, legacyLabels)
		if ensureErr != nil {
			s.logger.Warn(
				"failed to create legacy module package version",
				slog.String("package", packageName),
				slog.String("version", version),
				log.Err(ensureErr),
			)
			failedVersions = append(failedVersions, failedVersion{
				Name:  version,
				Error: "ensure legacy module package version: " + ensureErr.Error(),
			})
			continue
		}

		if isNew {
			newVersions++
		}
	}

	return &PackageProcessResult{
		PackageType:   PackageTypeModule,
		Done:          foundTags,
		Failed:        failedVersions,
		FoundVersions: len(foundTags),
		NewVersions:   newVersions,
	}, nil
}

// detectPackageType determines a package type from the version image label or package definition.
//
// An empty or unsupported declared type returns errPackageTypeInvalid. Images without either
// source return errTooOldImage.
func (s *Service) detectPackageType(ctx context.Context, packageName, latestTag string) (PackageType, error) {
	pkg := s.svc.Package(packageName)

	versionConfig, err := pkg.Versions().GetImageConfig(ctx, latestTag)
	if err != nil {
		s.logger.Warn(
			"failed to get version image config",
			slog.String("package", packageName),
			log.Err(err),
		)
		versionConfig = nil
	}
	if versionConfig != nil && versionConfig.Config.Labels != nil {
		if rawPackageType, hasLabel := versionConfig.Config.Labels[packageTypeLabel]; hasLabel {
			return parsePackageType(rawPackageType)
		}
	}

	pkgDef, err := pkg.Versions().ReadPackageDefinition(ctx, latestTag)
	if err != nil {
		return "", fmt.Errorf("read package definition: %w", err)
	}

	if pkgDef != nil {
		if pkgDef.Type != "" {
			s.logger.Warn(
				"package type label not found, using type from package.yaml",
				slog.String("package", packageName),
				slog.String("type", pkgDef.Type),
			)
			return parsePackageType(pkgDef.Type)
		}
		s.logger.Warn(
			"package type not determined from labels or package.yaml",
			slog.String("package", packageName),
		)
		return "", fmt.Errorf("%w: %s", errPackageTypeInvalid, packageName)
	}

	s.logger.Warn(
		"version image has no type labels and no package.yaml",
		slog.String("package", packageName),
	)
	return "", fmt.Errorf("%w: %s", errTooOldImage, packageName)
}

// PackageProcessResult summarizes package versions handled during an operation.
type PackageProcessResult struct {
	PackageType   PackageType
	Done          []*semver.Version
	Failed        []failedVersion
	FoundVersions int
	// NewVersions counts package versions that were either created for the
	// first time during this operation, or (for ApplicationPackageVersion)
	// transitioned from the "not in registry" state because their image was
	// rediscovered in the registry.
	NewVersions int
}

// failedVersion describes a package version that could not be processed.
type failedVersion struct {
	Name  string
	Error string
}

// ensureApplicationPackageVersion creates or rediscovers an application package version.
// It reports whether the version became available during this call.
func (s *Service) ensureApplicationPackageVersion(ctx context.Context, packageName, version string) (bool, error) {
	apvName := v1alpha1.MakeApplicationPackageVersionName(s.repo.Name, packageName, version)

	logger := s.logger.With(slog.String("package version", apvName))

	pkgVersion := &v1alpha1.ApplicationPackageVersion{}
	err := s.client.Get(ctx, types.NamespacedName{Name: apvName}, pkgVersion)
	if err != nil && !apierrors.IsNotFound(err) {
		return false, fmt.Errorf("get application package version: %w", err)
	}
	if err == nil {
		isBundleExistInRegistry, ok := pkgVersion.Labels[v1alpha1.ApplicationPackageVersionLabelExistInRegistry]
		if !ok || isBundleExistInRegistry != "false" {
			return false, nil
		}
		logger.Debug("version marked as not exist in registry, checking if bundle image exists")

		if err := s.svc.Package(packageName).CheckImageExists(ctx, version); err != nil {
			if errors.Is(err, regClient.ErrImageNotFound) {
				logger.Debug("bundle image not found")
				return false, nil
			}
			return false, fmt.Errorf("check bundle image exists: %w", err)
		}

		logger.Debug("bundle image exists, marking package version as draft")

		original := pkgVersion.DeepCopy()

		pkgVersion.Labels[v1alpha1.ApplicationPackageVersionLabelExistInRegistry] = "true"
		pkgVersion.Labels[v1alpha1.ApplicationPackageVersionLabelDraft] = "true"

		err = s.client.Patch(ctx, pkgVersion, client.MergeFrom(original))
		if err != nil {
			return false, fmt.Errorf("update application package version: %w", err)
		}

		return true, nil
	}

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

	s.setOwnerReference(pkgVersion)

	err = s.client.Create(ctx, pkgVersion)
	if err != nil {
		return false, fmt.Errorf("create application package version: %w", err)
	}

	return true, nil
}

// EnsureApplicationPackage creates an application package and records the current repository.
func (s *Service) EnsureApplicationPackage(ctx context.Context, packageName string) error {
	pkg := &v1alpha1.ApplicationPackage{}
	err := s.client.Get(ctx, types.NamespacedName{Name: packageName}, pkg)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("get application package: %w", err)
	}

	if err != nil {
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
		s.ensureSharedOwnerReference(pkg)

		err = s.client.Create(ctx, pkg)
		if err != nil {
			return fmt.Errorf("create application package: %w", err)
		}
	} else {
		// Existing — make sure we are listed as an owner so a single repo deletion
		// does not cascade-delete a package that other repositories still contribute.
		original := pkg.DeepCopy()
		if s.ensureSharedOwnerReference(pkg) {
			if err := s.client.Patch(ctx, pkg, client.MergeFrom(original)); err != nil {
				return fmt.Errorf("sync application package owner refs: %w", err)
			}
		}
	}

	if slices.Contains(pkg.Status.AvailableRepositories, s.repo.Name) {
		return nil
	}

	original := pkg.DeepCopy()

	pkg.Status.AvailableRepositories = append(pkg.Status.AvailableRepositories, s.repo.Name)

	err = s.client.Status().Patch(ctx, pkg, client.MergeFrom(original))
	if err != nil {
		return fmt.Errorf("update application package status: %w", err)
	}

	return nil
}

// ensureModulePackageVersion creates a module package version when it does not already exist.
// It reports whether the version was created during this call.
func (s *Service) ensureModulePackageVersion(ctx context.Context, packageName, version string, extraLabels map[string]string) (bool, error) {
	mpvName := v1alpha1.MakeModulePackageVersionName(s.repo.Name, packageName, version)

	pkgVersion := &v1alpha1.ModulePackageVersion{}
	err := s.client.Get(ctx, types.NamespacedName{Name: mpvName}, pkgVersion)
	if err != nil && !apierrors.IsNotFound(err) {
		return false, fmt.Errorf("get module package version: %w", err)
	}

	if err == nil {
		return false, nil
	}

	labels := map[string]string{
		"heritage": "deckhouse",
		v1alpha1.ModulePackageVersionLabelRepository: s.repo.Name,
		v1alpha1.ModulePackageVersionLabelPackage:    packageName,
		v1alpha1.ModulePackageVersionLabelDraft:      "true",
	}
	for k, v := range extraLabels {
		labels[k] = v
	}

	pkgVersion = &v1alpha1.ModulePackageVersion{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.ModulePackageVersionGVK.GroupVersion().String(),
			Kind:       v1alpha1.ModulePackageVersionKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   mpvName,
			Labels: labels,
		},
		Spec: v1alpha1.ModulePackageVersionSpec{
			PackageName:           packageName,
			PackageVersion:        version,
			PackageRepositoryName: s.repo.Name,
		},
	}

	s.setOwnerReference(pkgVersion)

	err = s.client.Create(ctx, pkgVersion)
	if err != nil {
		return false, fmt.Errorf("create module package version: %w", err)
	}

	return true, nil
}

// EnsureModulePackage creates a module package and records the current repository.
func (s *Service) EnsureModulePackage(ctx context.Context, packageName string) error {
	pkg := &v1alpha1.ModulePackage{}
	err := s.client.Get(ctx, types.NamespacedName{Name: packageName}, pkg)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("get module package: %w", err)
	}

	if err != nil {
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
		s.ensureSharedOwnerReference(pkg)

		err = s.client.Create(ctx, pkg)
		if err != nil {
			return fmt.Errorf("create module package: %w", err)
		}
	} else {
		// Existing — make sure we are listed as an owner so a single repo deletion
		// does not cascade-delete a package that other repositories still contribute.
		original := pkg.DeepCopy()
		if s.ensureSharedOwnerReference(pkg) {
			if err := s.client.Patch(ctx, pkg, client.MergeFrom(original)); err != nil {
				return fmt.Errorf("sync module package owner refs: %w", err)
			}
		}
	}

	if slices.Contains(pkg.Status.AvailableRepositories, s.repo.Name) {
		return nil
	}

	original := pkg.DeepCopy()

	pkg.Status.AvailableRepositories = append(pkg.Status.AvailableRepositories, s.repo.Name)

	err = s.client.Status().Patch(ctx, pkg, client.MergeFrom(original))
	if err != nil {
		return fmt.Errorf("update module package status: %w", err)
	}

	return nil
}

// ensureSharedOwnerReference adds the repository as a non-controller owner when absent.
// It reports whether the object's owner references changed.
func (s *Service) ensureSharedOwnerReference(obj client.Object) bool {
	refs := obj.GetOwnerReferences()
	for _, ref := range refs {
		if ref.Kind != v1alpha1.PackageRepositoryKind {
			continue
		}
		// Match by UID when both sides have one (real cluster); otherwise fall back
		// to Name (PackageRepository names are cluster-unique, and UIDs may be empty
		// in test fixtures).
		if ref.UID != "" && ref.UID == s.repo.UID {
			return false
		}
		if ref.Name == s.repo.Name {
			return false
		}
	}
	refs = append(refs, metav1.OwnerReference{
		APIVersion: v1alpha1.PackageRepositoryGVK.GroupVersion().String(),
		Kind:       v1alpha1.PackageRepositoryKind,
		Name:       s.repo.Name,
		UID:        s.repo.UID,
		Controller: &[]bool{false}[0],
	})
	obj.SetOwnerReferences(refs)
	return true
}

// setOwnerReference assigns the repository as the object's controller owner.
func (s *Service) setOwnerReference(obj client.Object) {
	ownerRef := metav1.OwnerReference{
		APIVersion: v1alpha1.PackageRepositoryGVK.GroupVersion().String(),
		Kind:       v1alpha1.PackageRepositoryKind,
		Name:       s.repo.Name,
		UID:        s.repo.UID,
		Controller: &[]bool{true}[0],
	}

	obj.SetOwnerReferences([]metav1.OwnerReference{ownerRef})
}
