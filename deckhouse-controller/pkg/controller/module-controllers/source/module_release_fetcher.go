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

package source

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"regexp"
	"sort"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/shell-operator/pkg/metric"
	"github.com/jonboulle/clockwork"
	"go.opentelemetry.io/otel"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/downloader"
	releaseUpdater "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/releaseupdater"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type ModuleReleaseFetcherConfig struct {
	K8sClient                 client.Client
	RegistryClientTagFetcher  cr.Client
	RegistryClientMetaFetcher cr.Client
	Clock                     clockwork.Clock
	ModuleDownloader          *downloader.ModuleDownloader
	// moduleManager             moduleManager

	ModuleName        string
	TargetReleaseMeta *downloader.ModuleDownloadResult

	Source           *v1alpha1.ModuleSource
	UpdatePolicyName string

	MetricStorage     metric.Storage
	MetricModuleGroup string

	Logger *log.Logger
}

func NewDeckhouseReleaseFetcher(cfg *ModuleReleaseFetcherConfig) *ModuleReleaseFetcher {
	return &ModuleReleaseFetcher{
		k8sClient:                cfg.K8sClient,
		registryClientTagFetcher: cfg.RegistryClientTagFetcher,
		clock:                    cfg.Clock,
		moduleDownloader:         cfg.ModuleDownloader,
		// moduleManager:             cfg.moduleManager,
		moduleName:        cfg.ModuleName,
		targetReleaseMeta: cfg.TargetReleaseMeta,
		source:            cfg.Source,
		updatePolicyName:  cfg.UpdatePolicyName,
		metricStorage:     cfg.MetricStorage,
		metricGroupName:   cfg.MetricModuleGroup,
		logger:            cfg.Logger,
	}
}

type ModuleReleaseFetcher struct {
	k8sClient                client.Client
	registryClientTagFetcher cr.Client
	clock                    clockwork.Clock
	moduleDownloader         *downloader.ModuleDownloader
	// moduleManager  moduleManager

	moduleName        string
	targetReleaseMeta *downloader.ModuleDownloadResult

	source           *v1alpha1.ModuleSource
	updatePolicyName string

	metricStorage   metric.Storage
	metricGroupName string

	logger *log.Logger
}

// checkDeckhouseRelease create fetcher and start
func (r *reconciler) fetchModuleReleases(
	ctx context.Context,
	moduleDownloader *downloader.ModuleDownloader,
	moduleName string,
	targetReleaseMeta *downloader.ModuleDownloadResult,
	source *v1alpha1.ModuleSource,
	updatePolicyName string,
	metricModuleGroup string,
	opts []cr.Option,
) error {
	ctx, span := otel.Tracer(serviceName).Start(ctx, "checkDeckhouseRelease")
	defer span.End()

	// client watch only one channel
	// registry.deckhouse.io/deckhouse/ce/release-channel:$release-channel
	registryClient, err := r.dc.GetRegistryClient(path.Join(source.Spec.Registry.Repo, moduleName), opts...)
	if err != nil {
		return fmt.Errorf("get registry client: %w", err)
	}

	cfg := &ModuleReleaseFetcherConfig{
		K8sClient:                r.client,
		RegistryClientTagFetcher: registryClient,
		Clock:                    r.dc.GetClock(),
		ModuleDownloader:         moduleDownloader,
		ModuleName:               moduleName,
		TargetReleaseMeta:        targetReleaseMeta,
		Source:                   source,
		UpdatePolicyName:         updatePolicyName,
		MetricStorage:            r.metricStorage,
		MetricModuleGroup:        metricModuleGroup,
		Logger:                   r.logger.Named("release-fetcher"),
	}

	releaseFetcher := NewDeckhouseReleaseFetcher(cfg)

	return releaseFetcher.fetchModuleReleases(ctx)
}

// fetchModuleReleases is a complete flow for loop
func (f *ModuleReleaseFetcher) fetchModuleReleases(ctx context.Context) error {
	ctx, span := otel.Tracer(serviceName).Start(ctx, "fetchDeckhouseRelease")
	defer span.End()

	releases, err := f.listModuleReleases(ctx, f.moduleName)
	if err != nil {
		return fmt.Errorf("list module releases: %w", err)
	}

	var releaseForUpdate *v1alpha1.ModuleRelease
	releasesInCluster := make([]*v1alpha1.ModuleRelease, 0, len(releases))

	idx, deployedRelease := getLatestDeployedRelease(releases)
	if idx != -1 {
		releasesInCluster = releases[:idx+1]
		releaseForUpdate = deployedRelease
	}

	// check sequence from the start if no deckhouse release deployed
	// last element because it's reversed
	if len(releasesInCluster) == 0 && len(releases) > 0 {
		releaseForUpdate = releases[len(releases)-1]
		releasesInCluster = releases
	}

	newSemver, err := semver.NewVersion(f.targetReleaseMeta.ModuleVersion)
	if err != nil {
		// TODO: maybe set something like v1.0.0-{meta.Version} for developing purpose
		return fmt.Errorf("parse semver: %w", err)
	}

	// sort releases before
	sort.Sort(releaseUpdater.ByVersion[*v1alpha1.ModuleRelease](releasesInCluster))

	err = f.ensureReleases(ctx, releaseForUpdate, releasesInCluster, newSemver)
	if err != nil {
		return fmt.Errorf("ensure releases: %w", err)
	}

	return nil
}

// ensureReleases create releases and return metadata of last created release.
// flow:
//  1. if no releases in cluster - create from channel
//  2. if deployed release patch version is lower than channel (with same minor and major) - create from channel
//  3. if deployed release minor version is lower than channel (with same major) - create from channel
//  4. if deployed release minor version is lower by 2 or more than channel (with same major) - look at releases in cluster
//     4.1 if update sequence between deployed release and last release in cluster is broken - get releases from registry between deployed and version from channel, and create releases
//     4.2 if update sequence between deployed release and last release in cluster not broken - check update sequence between last release in cluster and version in channel
//     4.2.1 if update sequence between last release in cluster and version in channel is broken - get releases from registry between last release in cluster and version from channel, and create releases
//     4.2.2 if update sequence between last release in cluster and version in channel not broken - create from channel
//     4.3 if update sequences not broken - create from channel
func (f *ModuleReleaseFetcher) ensureReleases(
	ctx context.Context,
	releaseForUpdate *v1alpha1.ModuleRelease,
	releasesInCluster []*v1alpha1.ModuleRelease,
	newSemver *semver.Version) error {
	ctx, span := otel.Tracer(serviceName).Start(ctx, "ensureReleases")
	defer span.End()

	metricLabels := map[string]string{
		"module":   f.moduleName,
		"version":  f.targetReleaseMeta.ModuleVersion,
		"registry": f.source.Spec.Registry.Repo,
	}

	if len(releasesInCluster) == 0 {
		err := f.ensureModuleRelease(ctx, f.targetReleaseMeta, "no releases in cluster")
		if err != nil {
			return fmt.Errorf("create release %s: %w", f.targetReleaseMeta.ModuleVersion, err)
		}

		return nil
	}

	// create release if deployed release and new release are in updating sequence
	actual := releaseForUpdate
	metricLabels["actual_version"] = "v" + actual.GetVersion().String()
	if isUpdatingSequence(actual.GetVersion(), newSemver) {
		err := f.ensureModuleRelease(ctx, f.targetReleaseMeta, "from deployed")
		if err != nil {
			return fmt.Errorf("create release %s: %w", f.targetReleaseMeta.ModuleVersion, err)
		}

		return nil
	}

	isSequence := false
	for i := 1; i < len(releasesInCluster); i++ {
		isSequence = isUpdatingSequence(releasesInCluster[i-1].GetVersion(), releasesInCluster[i].GetVersion())
		if !isSequence {
			break
		}
	}

	if isSequence {
		// check
		actual = releasesInCluster[len(releasesInCluster)-1]

		// create release if last release and new release are in updating sequence
		if isUpdatingSequence(actual.GetVersion(), newSemver) {
			err := f.ensureModuleRelease(ctx, f.targetReleaseMeta, "from last release in cluster")
			if err != nil {
				return fmt.Errorf("create release %s: %w", f.targetReleaseMeta.ModuleVersion, err)
			}

			return nil
		}
	}

	vers, err := f.getNewVersions(ctx, actual.GetVersion(), newSemver)
	if err != nil {
		return fmt.Errorf("get next version: %w", err)
	}

	var ErrModuleIsCorrupted = errors.New("module is corrupted")

	current := actual.GetVersion()
	for _, ver := range vers {
		ensureErr := func() error {
			f.logger.Debug("ensure module release for module for the module source",
				slog.String("name", f.moduleName),
				slog.String("source_name", f.source.Name))

			m, err := f.moduleDownloader.DownloadMetadataByVersion(f.moduleName, "v"+ver.String())
			if err != nil {
				f.logger.Error("download metadata by version", slog.String("module_name", f.moduleName), slog.String("module_version", "v"+ver.String()), log.Err(err))

				return fmt.Errorf("download metadata by version: %w, %w", err, ErrModuleIsCorrupted)
			}

			err = f.ensureModuleRelease(ctx, &m, "step-by-step")
			if err != nil {
				f.logger.Error("ensure module release", slog.String("module_name", f.moduleName), slog.String("module_version", "v"+ver.String()), log.Err(err))

				return fmt.Errorf("ensure module release: %w", err)
			}

			// if next version is not in sequence with actual
			if !isUpdatingSequence(current, ver) {
				f.logger.Warn("version sequence is broken", slog.String("previous", "v"+current.String()), slog.String("next", "v"+ver.String()))

				return fmt.Errorf("not sequential version: prev 'v%s' next 'v%s'", current.String(), ver.String())
			}

			return nil
		}()
		if ensureErr != nil {
			err = errors.Join(err, ensureErr)

			metricLabels["version"] = "v" + ver.String()

			if errors.Is(ensureErr, ErrModuleIsCorrupted) {
				f.metricStorage.Grouped().GaugeSet(f.metricGroupName, metricUpdatingModuleIsNotValid, 1, metricLabels)
			} else {
				f.metricStorage.Grouped().GaugeSet(f.metricGroupName, metricUpdatingFailedBrokenSequence, 1, metricLabels)
			}
		}

		current = ver
	}

	if err != nil {
		f.logger.Error("step by step update failed", log.Err(err))

		return fmt.Errorf("step by step update failed: %w", err)
	}

	return nil
}

// isUpdatingSequence checks that version 'a' and 'b' allowed to updating from 'a' to 'b'.
// this helper function is to calculate necessary of registry listing.
// 'a' version must be lower than 'b' version,
// if 'a' major version +1 is lower than 'b' major version - it's no updating sequence,
// if 'a' minor version +1 is lower than 'b' minor version - it's no updating sequence
func isUpdatingSequence(a, b *semver.Version) bool {
	if a.Major()+1 < b.Major() {
		return false
	}

	if a.Minor()+1 < b.Minor() {
		return false
	}

	return true
}

func (f *ModuleReleaseFetcher) listModuleReleases(ctx context.Context, moduleName string) ([]*v1alpha1.ModuleRelease, error) {
	releases := new(v1alpha1.ModuleReleaseList)

	if err := f.k8sClient.List(ctx, releases, client.MatchingLabels{"module": moduleName}); err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}

	result := make([]*v1alpha1.ModuleRelease, 0, len(releases.Items))

	for _, release := range releases.Items {
		result = append(result, &release)
	}

	return result, nil
}

func getLatestDeployedRelease(releases []*v1alpha1.ModuleRelease) (int, *v1alpha1.ModuleRelease) {
	sort.Sort(sort.Reverse(releaseUpdater.ByVersion[*v1alpha1.ModuleRelease](releases)))

	for idx, release := range releases {
		if release.GetPhase() == v1alpha1.ModuleReleasePhaseDeployed {
			return idx, release
		}
	}

	return -1, nil
}

func (f *ModuleReleaseFetcher) ensureModuleRelease(ctx context.Context, meta *downloader.ModuleDownloadResult, createProcess string) error {
	ctx, span := otel.Tracer(serviceName).Start(ctx, "ensureModuleRelease")
	defer span.End()

	changeCause := "check release"
	if createProcess != "" {
		changeCause += " (" + createProcess + ")"
	}

	release := new(v1alpha1.ModuleRelease)
	if err := f.k8sClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s-%s", f.moduleName, meta.ModuleVersion)}, release); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get the module release: %w", err)
		}

		release = &v1alpha1.ModuleRelease{
			TypeMeta: metav1.TypeMeta{
				Kind:       v1alpha1.ModuleReleaseGVK.Kind,
				APIVersion: "deckhouse.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s-%s", f.moduleName, meta.ModuleVersion),
				Annotations: map[string]string{
					v1alpha1.ModuleReleaseAnnotationChangeCause: changeCause,
				},
				Labels: map[string]string{
					v1alpha1.ModuleReleaseLabelModule: f.moduleName,
					v1alpha1.ModuleReleaseLabelSource: f.source.GetName(),
					// image digest has 64 symbols, while label can have maximum 63 symbols, so make md5 sum here
					v1alpha1.ModuleReleaseLabelReleaseChecksum: fmt.Sprintf("%x", md5.Sum([]byte(meta.Checksum))),
					v1alpha1.ModuleReleaseLabelUpdatePolicy:    f.updatePolicyName,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: v1alpha1.ModuleSourceGVK.GroupVersion().String(),
						Kind:       v1alpha1.ModuleSourceGVK.Kind,
						Name:       f.source.GetName(),
						UID:        f.source.GetUID(),
						Controller: ptr.To(true),
					},
				},
			},
			Spec: v1alpha1.ModuleReleaseSpec{
				ModuleName: f.moduleName,
				Version:    semver.MustParse(meta.ModuleVersion).String(),
				Weight:     meta.ModuleWeight,
				Changelog:  meta.Changelog,
			},
		}

		if meta.ModuleDefinition != nil && meta.ModuleDefinition.Requirements != nil {
			release.Spec.Requirements = &v1alpha1.ModuleReleaseRequirements{
				ModuleReleasePlatformRequirements: v1alpha1.ModuleReleasePlatformRequirements{
					Deckhouse:  meta.ModuleDefinition.Requirements.Deckhouse,
					Kubernetes: meta.ModuleDefinition.Requirements.Kubernetes,
				},
				ParentModules: meta.ModuleDefinition.Requirements.ParentModules,
			}
		}

		// if it's a first release for a Module, we have to install it immediately
		// without any update Windows and update.mode manual approval
		// the easiest way is to check the count or ModuleReleases for this module
		{
			labelSelector := client.MatchingLabels{v1alpha1.ModuleReleaseLabelModule: f.moduleName}

			releases := new(v1alpha1.ModuleReleaseList)
			if err = f.k8sClient.List(ctx, releases, labelSelector, client.Limit(1)); err != nil {
				return fmt.Errorf("list the '%s' module releases: %w", f.moduleName, err)
			}
			if len(releases.Items) == 0 {
				// no other releases
				if len(release.Annotations) == 0 {
					release.Annotations = make(map[string]string, 1)
				}
				release.Annotations[v1alpha1.ModuleReleaseAnnotationApplyNow] = "true"
			}
		}

		if err = f.k8sClient.Create(ctx, release); err != nil {
			return fmt.Errorf("create module release: %w", err)
		}
		return nil
	}

	// seems weird to update already deployed/suspended release
	if release.Status.Phase != v1alpha1.ModuleReleasePhasePending {
		return nil
	}

	release.Annotations[v1alpha1.ModuleReleaseAnnotationChangeCause] = changeCause

	release.Spec = v1alpha1.ModuleReleaseSpec{
		ModuleName: f.moduleName,
		Version:    semver.MustParse(meta.ModuleVersion).String(),
		Weight:     meta.ModuleWeight,
		Changelog:  meta.Changelog,
	}

	if err := f.k8sClient.Update(ctx, release); err != nil {
		return fmt.Errorf("update module release: %w", err)
	}

	return nil
}

// getNewVersions - getting all last patches from registry
// it's ignore last patch of actual minor version, if it has new minor version
//
// f.e.
// in registry:
// 1.66.3 (deployed)
// 1.66.5
// result will be 1.66.5
//
// but if we have a new minor version like:
// 1.66.3 (deployed)
// 1.66.5
// 1.67.11
// result will be 1.67.11
//
// several patches:
// 1.66.3 (deployed)
// 1.66.5
// 1.67.5
// 1.67.11
// 1.68.1
// 1.68.3
// 1.68.5
// result will be [1.67.11, 1.68.5]
func (f *ModuleReleaseFetcher) getNewVersions(ctx context.Context, actual, target *semver.Version) ([]*semver.Version, error) {
	tags, err := f.registryClientTagFetcher.ListTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	collection := f.parseAndFilterVersions(tags)
	if len(collection) == 0 {
		return nil, fmt.Errorf("no matched tags in registry")
	}

	sort.Sort(semver.Collection(collection))

	const minVersionsCapacity = 10

	// Get only highest patch version for each minor version between actual and target
	result := make([]*semver.Version, 0, minVersionsCapacity)
	var lastVer *semver.Version

	for _, ver := range collection {
		// Skip versions outside the actual-target range
		if !isVersionInRange(ver, actual, target) {
			continue
		}

		// Add version if it's first or has different minor/major from previous
		if lastVer != nil &&
			(lastVer.Minor() < ver.Minor() || lastVer.Major() < ver.Major()) {
			result = append(result, lastVer)
		}

		lastVer = ver
	}

	// Add the final version
	if lastVer != nil {
		if isVersionGreaterThanTarget(lastVer, target) {
			f.logger.Warn("last release is not equals to target, using target instead",
				slog.String("last", lastVer.Original()),
				slog.String("target", target.Original()))

			result = append(result, target)
		} else {
			result = append(result, lastVer)
		}
	}

	// Remove highest patch from actual minor version if we have more versions
	if len(result) > 1 && result[0].Minor() == actual.Minor() {
		result = result[1:]
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no acceptable for step by step update tags in registry")
	}

	return result, nil
}

func (f *ModuleReleaseFetcher) parseAndFilterVersions(tags []string) []*semver.Version {
	versionMatcher := regexp.MustCompile(`^v(([0-9]+).([0-9]+).([0-9]+))$`)
	versions := make([]*semver.Version, 0)

	for _, tag := range tags {
		if !versionMatcher.MatchString(tag) {
			f.logger.Debug("not suitable. This version will be skipped.", slog.String("version", tag))
			continue
		}

		ver, err := semver.NewVersion(tag)
		if err != nil {
			f.logger.Warn("unable to parse semver from the registry. This version will be skipped.", slog.String("version", tag))
			continue
		}

		versions = append(versions, ver)
	}

	return versions
}

func isVersionInRange(ver, actual, target *semver.Version) bool {
	return (ver.Major() > actual.Major() ||
		(ver.Major() == actual.Major() && ver.Minor() >= actual.Minor())) &&
		(ver.Major() < target.Major() || (ver.Major() == target.Major() && ver.Minor() <= target.Minor()))
}

func isVersionGreaterThanTarget(ver, target *semver.Version) bool {
	return ver.Major() > target.Major() ||
		(ver.Major() == target.Major() && ver.Minor() > target.Minor()) ||
		(ver.Major() == target.Major() && ver.Minor() == target.Minor() && ver.Patch() > target.Patch())
}
