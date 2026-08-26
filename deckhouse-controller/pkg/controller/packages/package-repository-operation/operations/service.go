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

package operations

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	transport "github.com/google/go-containerregistry/pkg/v1/remote/transport"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryService "github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry/service"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/pkg/log"
	regClient "github.com/deckhouse/deckhouse/pkg/registry/client"
)

var (
	// ErrPackageTypeInvalid is returned by detectPackageType when a package has manifest files
	// (labels or package.yaml) but the type value is empty or not recognized.
	ErrPackageTypeInvalid = errors.New("package type could not be determined")

	// ErrTooOldImage is returned when a version image has no type labels and no package.yaml -
	// it cannot be processed.
	ErrTooOldImage = errors.New("version image has no type labels and no package.yaml")
)

// isRepoNotFoundError checks if the error chain contains a registry NAME_UNKNOWN error,
// which means the repository path does not exist in the registry.
// This is consistent with the pattern used in deckhouse-controller/pkg/registry/module.go.
func isRepoNotFoundError(err error) bool {
	return strings.Contains(err.Error(), string(transport.NameUnknownErrorCode))
}

// PackageType represents the type of a package as detected from Docker labels or package.yaml.
type PackageType string

const (
	// PackageTypeLabel is a label on Docker images that indicates the package type
	PackagesRepositoryOperationLabelPackageType = "io.deckhouse.package.type"

	PackageTypeApplication PackageType = "Application"
	PackageTypeModule      PackageType = "Module"
)

// ParsePackageType converts a raw string to v1alpha1.PackageType.
//
// returning an error if the value is not recognized. f.e: type: "Garbage", type: ""
func ParsePackageType(raw string) (PackageType, error) {
	switch PackageType(raw) {
	case PackageTypeApplication:
		return PackageTypeApplication, nil
	case PackageTypeModule:
		return PackageTypeModule, nil
	default:
		return "", fmt.Errorf("%w: unknown value %q", ErrPackageTypeInvalid, raw)
	}
}

type Result struct {
	PackageType PackageType
	Failed      []failedVersion
	// FoundVersions counts the versions the registry offered, including the ones left
	// unprocessed after a failure stopped the walk.
	FoundVersions int
	// NewVersions counts package versions that were either created for the
	// first time during this operation, or transitioned from the "not in
	// registry" state because their image was rediscovered in the registry.
	NewVersions int
}

type failedVersion struct {
	Name  string
	Error string
}

// OperationService scans a single PackageRepository and materialises the package CRs it offers.
type OperationService struct {
	client client.Client
	repo   *v1alpha1.PackageRepository
	svc    *registryService.PackagesService

	logger *log.Logger
}

// NewService reads the repository and binds a registry service to its packages path.
func NewService(ctx context.Context, cli client.Client, repoName string, psm registryService.ServiceManagerInterface[registryService.PackagesService], logger *log.Logger) (*OperationService, error) {
	repo := new(v1alpha1.PackageRepository)
	if err := cli.Get(ctx, client.ObjectKey{Name: repoName}, repo); err != nil {
		return nil, fmt.Errorf("get package repository: %w", err)
	}

	// Create registry service for the packages path
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

	return &OperationService{
		client: cli,
		repo:   repo,
		svc:    svc,
		logger: logger,
	}, nil
}

// GetRepository returns the PackageRepository associated with this service.
func (s *OperationService) GetRepository() *v1alpha1.PackageRepository {
	return s.repo
}

// Discover lists the package names published under the repository's packages path.
func (s *OperationService) Discover(ctx context.Context) ([]string, error) {
	// List packages (packages at the packages level)
	packages, err := s.svc.ListTags(ctx)
	if err != nil {
		s.logger.Error("failed to list packages", log.Err(err))

		return nil, fmt.Errorf("failed to list packages: %w", err)
	}

	s.logger.Info("discovered packages", slog.Int("count", len(packages)))

	return packages, nil
}

// ScanPackageVersions lists <package>/version, detects the type from the newest tag and creates
// an APV or MPV per version, oldest first, stopping at the first one it fails to create. A package
// with no /version path, or none carrying semver tags on a full scan, is a legacy module, and
// walkModuleReleases reads its versions from /release instead.
func (s *OperationService) ScanPackageVersions(ctx context.Context, packageName string, operation *v1alpha1.PackageRepositoryOperation) (*Result, error) {
	foundTags, err := s.foundTagsToProcess(ctx, packageName, operation)
	if err != nil {
		// NAME_UNKNOWN means <package>/version path doesn't exist in the registry.
		// No /version path → legacy module (v1alpha1), its versions live under /release.
		if isRepoNotFoundError(err) {
			return s.walkModuleReleases(ctx, packageName)
		}
		return nil, fmt.Errorf("get found tags to process: %w", err)
	}

	s.logger.Info(
		"found package versions",
		slog.String("package", packageName),
		slog.Int("versions", len(foundTags)),
	)

	// An empty result on a full scan means /version carries no semver tags at all, so the package
	// can still be a legacy module holding its versions under /release. On an incremental scan it
	// only means "nothing new since the watermark", and walking /release would re-read the history
	// on every scan.
	if len(foundTags) == 0 {
		if operation.Spec.Update != nil && operation.Spec.Update.FullScan {
			return s.walkModuleReleases(ctx, packageName)
		}

		return &Result{}, nil
	}

	// Ascending: the newest tag carries the type labels older ones may lack, and the walk below
	// must not create a version above one it failed to create.
	slices.SortFunc(foundTags, func(a, b *semver.Version) int { return a.Compare(b) })
	latestTag := "v" + foundTags[len(foundTags)-1].String()

	var pkgType PackageType
	if pkgType, err = s.detectPackageType(ctx, packageName, latestTag); err != nil {
		// No type labels and no package.yaml on /version path - skip
		if errors.Is(err, ErrTooOldImage) {
			return &Result{
				Failed: []failedVersion{{Error: err.Error()}},
			}, nil
		}

		if errors.Is(err, ErrPackageTypeInvalid) {
			return &Result{
				Failed: []failedVersion{{Name: latestTag, Error: err.Error()}},
			}, nil
		}

		return nil, err
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

			// Creating the newer versions above this one would raise the watermark past it, and no
			// later incremental scan would list it again. The next scan resumes from here instead.
			break
		}

		if isNew {
			newVersions++
		}
	}

	return &Result{
		PackageType:   pkgType,
		Failed:        failedVersions,
		FoundVersions: len(foundTags),
		NewVersions:   newVersions,
	}, nil
}

// foundTagsToProcess returns the semver tags to process: every tag on a full scan, only the tags
// newer than the last processed version otherwise.
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

// performIncrementalScan lists the version tags newer than the newest processed version, plus the
// known versions whose image is marked gone from the registry.
func (s *OperationService) performIncrementalScan(ctx context.Context, packageName string) ([]*semver.Version, error) {
	// Incremental scan: start from the last processed version
	s.logger.Debug("perform incremental scan", slog.String("package", packageName))

	processed := s.collectProcessedVersions(ctx, packageName)

	var lastVersion string
	if processed.last != nil {
		lastVersion = "v" + processed.last.String()

		s.logger.Debug("found last processed version",
			slog.String("package", packageName),
			slog.String("last_version", lastVersion))
	}

	tags, err := s.listTagsFromVersion(ctx, packageName, lastVersion)
	if err != nil {
		return nil, fmt.Errorf("list tags from version: %w", err)
	}

	// A version whose image went missing sits at or below the watermark, so the listing never
	// returns it. Left out, its return would go unnoticed until the next full scan.
	return appendMissingVersions(tags, processed.missing), nil
}

// processedVersions is what the cluster already holds for a package: the watermark an incremental
// scan resumes from, and the versions whose image is marked gone and must be re-checked below it.
type processedVersions struct {
	last    *semver.Version
	missing []*semver.Version
}

// collectProcessedVersions returns the newest processed version together with the ones whose image
// is marked gone. Both CR kinds are queried because the package type is not known yet at this point.
func (s *OperationService) collectProcessedVersions(ctx context.Context, packageName string) processedVersions {
	matchLabels := client.MatchingLabels{
		v1alpha1.ApplicationPackageVersionLabelRepository: s.repo.Name,
		v1alpha1.ApplicationPackageVersionLabelPackage:    packageName,
	}

	var (
		state    processedVersions
		versions []*semver.Version
	)

	apvs := new(v1alpha1.ApplicationPackageVersionList)
	if err := s.client.List(ctx, apvs, matchLabels); err != nil {
		s.logger.Warn("failed to list application package versions", slog.String("package", packageName), log.Err(err))
	}
	for _, item := range apvs.Items {
		if v := parseProcessedVersion(item.Spec.PackageVersion, item.Status.PackageMetadata != nil); v != nil {
			versions = append(versions, v)
		}
		if v := parseMissingVersion(item.Spec.PackageVersion, item.Labels); v != nil {
			state.missing = append(state.missing, v)
		}
	}

	mpvs := new(v1alpha1.ModulePackageVersionList)
	if err := s.client.List(ctx, mpvs, matchLabels); err != nil {
		s.logger.Warn("failed to list module package versions", slog.String("package", packageName), log.Err(err))
	}
	for _, item := range mpvs.Items {
		if v := parseProcessedVersion(item.Spec.PackageVersion, item.Status.PackageMetadata != nil); v != nil {
			versions = append(versions, v)
		}
		if v := parseMissingVersion(item.Spec.PackageVersion, item.Labels); v != nil {
			state.missing = append(state.missing, v)
		}
	}

	if len(versions) > 0 {
		slices.SortFunc(versions, func(a, b *semver.Version) int { return a.Compare(b) })
		state.last = versions[len(versions)-1]
	}

	return state
}

// parseProcessedVersion parses tag as semver, but only for a version that was actually processed.
func parseProcessedVersion(tag string, hasMetadata bool) *semver.Version {
	if !hasMetadata {
		return nil
	}
	v, _ := semver.NewVersion(tag)
	return v
}

// parseMissingVersion parses tag as semver, but only for a version whose image is marked gone.
// The label carries the same key for both CR kinds.
func parseMissingVersion(tag string, labels map[string]string) *semver.Version {
	if labels[v1alpha1.ApplicationPackageVersionLabelExistInRegistry] != "false" {
		return nil
	}

	v, _ := semver.NewVersion(tag)

	return v
}

// appendMissingVersions adds the versions the listing could not return, skipping any already there.
func appendMissingVersions(tags, missing []*semver.Version) []*semver.Version {
	for _, version := range missing {
		if slices.ContainsFunc(tags, version.Equal) {
			continue
		}

		tags = append(tags, version)
	}

	return tags
}

// listTagsFromVersion asks the registry for the tags after lastVersion and re-filters them locally,
// since the registry is free to ignore the hint.
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

	return newTags, nil
}

// extractOnlySemverTags keeps the tags that parse as semver and drops the rest.
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

// walkModuleReleases walks <package>/release from the newest version down, creating a
// ModulePackageVersion for every release image that carries a module definition.
//
// The walk stops at the first image without a definition: that version cannot be offered, so the
// module's history is cut off there. A version the cluster already holds
// is not re-read from /release, only re-checked for a "not in registry" mark that has since lifted.
func (s *OperationService) walkModuleReleases(ctx context.Context, packageName string) (*Result, error) {
	release := s.svc.Package(packageName).Release()

	rawTags, err := release.ListTags(ctx)
	if err != nil {
		// Neither /version nor /release: the package offers nothing to install.
		if isRepoNotFoundError(err) {
			return &Result{}, nil
		}

		return nil, fmt.Errorf("list release tags: %w", err)
	}

	foundTags := extractOnlySemverTags(rawTags)
	if len(foundTags) == 0 {
		return &Result{}, nil
	}

	slices.SortFunc(foundTags, func(a, b *semver.Version) int { return b.Compare(a) })

	legacyLabels := map[string]string{
		v1alpha1.ModulePackageVersionLabelLegacy: "true",
	}

	result := &Result{PackageType: PackageTypeModule}

	for _, versionTag := range foundTags {
		version := "v" + versionTag.String()

		// A known version keeps its release image unread: only its "not in registry" mark is
		// re-checked, and the walk goes on. Stopping here would strand a version that failed to be
		// created: the newer ones above it are known from then on, so no later scan would reach it.
		existing, err := s.getModulePackageVersion(ctx, packageName, version)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			rediscovered, err := s.rediscoverModulePackageVersion(ctx, existing, packageName, version)
			if err != nil {
				s.logger.Warn(
					"failed to rediscover legacy module package version",
					slog.String("package", packageName),
					slog.String("version", version),
					log.Err(err),
				)

				result.Failed = append(result.Failed, failedVersion{
					Name:  version,
					Error: "rediscover legacy module package version: " + err.Error(),
				})

				continue
			}

			if rediscovered {
				result.FoundVersions++
				result.NewVersions++
			}

			continue
		}

		hasDefinition, err := release.HasModuleDefinition(ctx, version)
		if err != nil {
			return nil, fmt.Errorf("check module definition: %w", err)
		}
		if !hasDefinition {
			s.logger.Info(
				"release image carries no module definition, older versions are left out",
				slog.String("package", packageName),
				slog.String("version", version),
			)

			break
		}

		if _, err := s.ensureModulePackageVersion(ctx, packageName, version, legacyLabels); err != nil {
			s.logger.Warn(
				"failed to create legacy module package version",
				slog.String("package", packageName),
				slog.String("version", version),
				log.Err(err),
			)

			result.Failed = append(result.Failed, failedVersion{
				Name:  version,
				Error: "ensure legacy module package version: " + err.Error(),
			})

			continue
		}

		result.FoundVersions++
		result.NewVersions++
	}

	return result, nil
}

// getModulePackageVersion returns the version the cluster already holds, or nil when it holds none.
// The release walk reuses the returned object, so a known version costs one read, not two.
func (s *OperationService) getModulePackageVersion(ctx context.Context, packageName, version string) (*v1alpha1.ModulePackageVersion, error) {
	name := v1alpha1.MakeModulePackageVersionName(s.repo.Name, packageName, version)

	pkgVersion := new(v1alpha1.ModulePackageVersion)
	if err := s.client.Get(ctx, client.ObjectKey{Name: name}, pkgVersion); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("get module package version: %w", err)
	}

	return pkgVersion, nil
}

// detectPackageType reads the type from the version image label, falling back to the type field of
// package.yaml. An unrecognised value yields ErrPackageTypeInvalid, neither source ErrTooOldImage.
func (s *OperationService) detectPackageType(ctx context.Context, packageName, latestTag string) (PackageType, error) {
	pkg := s.svc.Package(packageName)

	// Step 1: Read label from version image ConfigFile (<package>/version:<tag>)
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
		if rawPackageType, hasLabel := versionConfig.Config.Labels[PackagesRepositoryOperationLabelPackageType]; hasLabel {
			return ParsePackageType(rawPackageType)
		}
	}

	// Step 2: No label - fall back to package.yaml from version image
	pkgDef, err := pkg.Versions().ReadPackageDefinition(ctx, latestTag)
	if err != nil {
		return "", fmt.Errorf("read package definition: %w", err)
	}

	if pkgDef != nil {
		// Trying to get .type from the package.yaml definition itself
		if pkgDef.Type != "" {
			s.logger.Warn(
				"package type label not found, using type from package.yaml",
				slog.String("package", packageName),
				slog.String("type", pkgDef.Type),
			)
			return ParsePackageType(pkgDef.Type)
		}
		// package.yaml exists but type field is empty
		s.logger.Warn(
			"package type not determined from labels or package.yaml",
			slog.String("package", packageName),
		)
		return "", fmt.Errorf("%w: %s", ErrPackageTypeInvalid, packageName)
	}

	// No labels and no package.yaml
	s.logger.Warn(
		"version image has no type labels and no package.yaml",
		slog.String("package", packageName),
	)
	return "", fmt.Errorf("%w: %s", ErrTooOldImage, packageName)
}

// ensureApplicationPackageVersion creates the ApplicationPackageVersion, or clears the "not in
// registry" mark of an existing one once its image reappears. Reports whether either happened.
func (s *OperationService) ensureApplicationPackageVersion(ctx context.Context, packageName, version string) (bool, error) {
	apvName := v1alpha1.MakeApplicationPackageVersionName(s.repo.Name, packageName, version)

	pkgVersion := new(v1alpha1.ApplicationPackageVersion)
	err := s.client.Get(ctx, client.ObjectKey{Name: apvName}, pkgVersion)
	if err != nil && !apierrors.IsNotFound(err) {
		return false, fmt.Errorf("get application package version: %w", err)
	}

	// Version already exists
	if err == nil {
		return s.rediscoverApplicationPackageVersion(ctx, pkgVersion, packageName, version)
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

	if err := s.client.Create(ctx, pkgVersion); err != nil {
		return false, fmt.Errorf("create application package version: %w", err)
	}

	return true, nil
}

// rediscoverApplicationPackageVersion clears the "not in registry" mark of an existing version once
// its bundle image is back in the registry. Reports whether the mark was cleared.
func (s *OperationService) rediscoverApplicationPackageVersion(ctx context.Context, pkgVersion *v1alpha1.ApplicationPackageVersion, packageName, version string) (bool, error) {
	isBundleExistInRegistry, ok := pkgVersion.Labels[v1alpha1.ApplicationPackageVersionLabelExistInRegistry]
	if !ok || isBundleExistInRegistry != "false" {
		return false, nil
	}

	logger := s.logger.With(slog.String("package_version", pkgVersion.Name))

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

	if err := s.client.Patch(ctx, pkgVersion, client.MergeFrom(original)); err != nil {
		return false, fmt.Errorf("update application package version: %w", err)
	}

	return true, nil
}

// ensureModulePackageVersion creates the ModulePackageVersion with extraLabels on top of the
// defaults, or clears the "not in registry" mark of an existing one once its image reappears.
// Reports whether either happened.
func (s *OperationService) ensureModulePackageVersion(ctx context.Context, packageName, version string, extraLabels map[string]string) (bool, error) {
	mpvName := v1alpha1.MakeModulePackageVersionName(s.repo.Name, packageName, version)

	pkgVersion := new(v1alpha1.ModulePackageVersion)
	err := s.client.Get(ctx, client.ObjectKey{Name: mpvName}, pkgVersion)
	if err != nil && !apierrors.IsNotFound(err) {
		return false, fmt.Errorf("get module package version: %w", err)
	}

	// Version already exists
	if err == nil {
		return s.rediscoverModulePackageVersion(ctx, pkgVersion, packageName, version)
	}

	labels := map[string]string{
		"heritage": "deckhouse",
		v1alpha1.ModulePackageVersionLabelRepository: s.repo.Name,
		v1alpha1.ModulePackageVersionLabelPackage:    packageName,
		v1alpha1.ModulePackageVersionLabelDraft:      "true",
	}
	maps.Copy(labels, extraLabels)

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

	// Add owner reference to PackageRepository
	s.setOwnerReference(pkgVersion)

	if err := s.client.Create(ctx, pkgVersion); err != nil {
		return false, fmt.Errorf("create module package version: %w", err)
	}

	return true, nil
}

// rediscoverModulePackageVersion clears the "not in registry" mark of an existing version once its
// image is back in the registry. Reports whether the mark was cleared.
func (s *OperationService) rediscoverModulePackageVersion(ctx context.Context, pkgVersion *v1alpha1.ModulePackageVersion, packageName, version string) (bool, error) {
	isBundleExistInRegistry, ok := pkgVersion.Labels[v1alpha1.ModulePackageVersionLabelExistInRegistry]
	if !ok || isBundleExistInRegistry != "false" {
		return false, nil
	}

	logger := s.logger.With(slog.String("package_version", pkgVersion.Name))

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

	pkgVersion.Labels[v1alpha1.ModulePackageVersionLabelExistInRegistry] = "true"
	pkgVersion.Labels[v1alpha1.ModulePackageVersionLabelDraft] = "true"

	if err := s.client.Patch(ctx, pkgVersion, client.MergeFrom(original)); err != nil {
		return false, fmt.Errorf("update module package version: %w", err)
	}

	return true, nil
}

// setOwnerReference makes s.repo the sole controller-owner of obj, so removing the repository
// garbage-collects it.
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
