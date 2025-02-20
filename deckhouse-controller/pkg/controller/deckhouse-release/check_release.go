/*
Copyright 2024 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package deckhouse_release

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"
	"regexp"
	"sort"
	"time"

	"github.com/Masterminds/semver/v3"
	metricstorage "github.com/flant/shell-operator/pkg/metric_storage"
	registryv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/iancoleman/strcase"
	"github.com/jonboulle/clockwork"
	"github.com/spaolacci/murmur3"
	"gopkg.in/yaml.v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	releaseUpdater "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/releaseupdater"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/go_lib/libapi"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	metricUpdatingFailedGroup = "d8_updating_failed"
)

func (r *deckhouseReleaseReconciler) checkDeckhouseReleaseLoop(ctx context.Context) {
	wait.UntilWithContext(ctx, func(ctx context.Context) {
		err := r.checkDeckhouseRelease(ctx)
		if err != nil {
			r.logger.Error("check Deckhouse release", log.Err(err))
		}
	}, 3*time.Minute)
}

type DeckhouseReleaseFetcherConfig struct {
	k8sClient      client.Client
	registryClient cr.Client
	clock          clockwork.Clock
	moduleManager  moduleManager

	clusterUUID             string
	deckhouseVersion        string
	releaseChannel          string
	releaseVersionImageHash string

	metricStorage *metricstorage.MetricStorage

	logger *log.Logger
}

func NewDeckhouseReleaseFetcher(cfg *DeckhouseReleaseFetcherConfig) *DeckhouseReleaseFetcher {
	return &DeckhouseReleaseFetcher{
		k8sClient:               cfg.k8sClient,
		registryClient:          cfg.registryClient,
		clock:                   cfg.clock,
		moduleManager:           cfg.moduleManager,
		releaseChannel:          cfg.releaseChannel,
		releaseVersionImageHash: cfg.releaseVersionImageHash,
		clusterUUID:             cfg.clusterUUID,
		deckhouseVersion:        cfg.deckhouseVersion,
		metricStorage:           cfg.metricStorage,
		logger:                  cfg.logger,
	}
}

type DeckhouseReleaseFetcher struct {
	k8sClient      client.Client
	registryClient cr.Client
	clock          clockwork.Clock
	moduleManager  moduleManager

	clusterUUID             string
	deckhouseVersion        string
	releaseChannel          string
	releaseVersionImageHash string

	metricStorage *metricstorage.MetricStorage

	logger *log.Logger
}

func (dcr *DeckhouseReleaseFetcher) GetReleaseChannel() string {
	return dcr.releaseChannel
}

func (r *deckhouseReleaseReconciler) checkDeckhouseRelease(ctx context.Context) error {
	if r.updateSettings.Get().ReleaseChannel == "" {
		r.logger.Debug("Release channel isn't set.")
		return nil
	}

	// move release channel to kebab-case because CI makes tags in kebab-case
	// Alpha -> alpha
	// EarlyAccess -> early-access
	// etc...
	releaseChannelName := strcase.ToKebab(r.updateSettings.Get().ReleaseChannel)

	registrySecret, err := r.getRegistrySecret(ctx)
	if apierrors.IsNotFound(err) {
		err = nil
	}
	if err != nil {
		return fmt.Errorf("get registry secret: %w", err)
	}

	var (
		opts           []cr.Option
		imagesRegistry string
	)

	if registrySecret != nil {
		rconf := &utils.RegistryConfig{
			DockerConfig: registrySecret.DockerConfig,
			Scheme:       registrySecret.Scheme,
			CA:           registrySecret.CA,
			UserAgent:    r.clusterUUID,
		}

		opts = utils.GenerateRegistryOptions(rconf, r.logger)

		imagesRegistry = registrySecret.ImageRegistry
	}

	// client watch only one channel
	// registry.deckhouse.io/deckhouse/ce/release-channel:$release-channel
	registryClient, err := r.dc.GetRegistryClient(path.Join(imagesRegistry, "release-channel"), opts...)
	if err != nil {
		return fmt.Errorf("get registry client: %w", err)
	}

	cfg := &DeckhouseReleaseFetcherConfig{
		k8sClient:               r.client,
		registryClient:          registryClient,
		clock:                   r.dc.GetClock(),
		moduleManager:           r.moduleManager,
		releaseChannel:          releaseChannelName,
		releaseVersionImageHash: r.releaseVersionImageHash,
		clusterUUID:             r.clusterUUID,
		deckhouseVersion:        r.deckhouseVersion,
		metricStorage:           r.metricStorage,
		logger:                  r.logger.Named("release-fetcher"),
	}

	releaseChecker := NewDeckhouseReleaseFetcher(cfg)

	return releaseChecker.checkDeckhouseRelease(ctx)
}

func (r *DeckhouseReleaseFetcher) checkDeckhouseRelease(ctx context.Context) error {
	// get image info from release channel
	imageInfo, imageErr := r.GetNewImageInfo(ctx, r.releaseVersionImageHash)
	if imageErr != nil && !errors.Is(imageErr, ErrImageNotChanged) {
		return fmt.Errorf("get new image: %w", imageErr)
	}

	var releaseMetadata *ReleaseMetadata

	// only if image changed
	if imageErr == nil {
		releaseMetadata = imageInfo.Metadata
	}

	var (
		deployedRelease *v1alpha1.DeckhouseRelease
	)

	releases, err := r.listDeckhouseReleases(ctx)
	if err != nil {
		return fmt.Errorf("list deckhouse releases: %w", err)
	}

	sort.Sort(releaseUpdater.ByVersion[*v1alpha1.DeckhouseRelease](releases))

	releasesFromDeployed := make([]*v1alpha1.DeckhouseRelease, 0, len(releases))
	for _, release := range releases {
		if release.GetPhase() == v1alpha1.DeckhouseReleasePhaseDeployed {
			// no deployed release was found or there is more than one deployed release (get the latest)
			if deployedRelease == nil || release.GetVersion().GreaterThan(deployedRelease.GetVersion()) {
				deployedRelease = release
			}
		}

		if deployedRelease != nil {
			releasesFromDeployed = append(releasesFromDeployed, release)
		}
	}

	// restore current deployed release if no deployed releases found
	if deployedRelease == nil {
		r.logger.Warn("deployed deckhouse-release is not found, restoring...")

		restored, err := r.restoreCurrentDeployedRelease(ctx, r.deckhouseVersion)
		if err != nil {
			return fmt.Errorf("restore current deployed release: %w", err)
		}

		r.logger.Warn("deployed deckhouse-release restored")

		releases = append(releases, restored)

		sort.Sort(releaseUpdater.ByVersion[*v1alpha1.DeckhouseRelease](releases))
	}

	// no new image found
	if errors.Is(imageErr, ErrImageNotChanged) {
		return nil
	}

	newSemver, err := semver.NewVersion(releaseMetadata.Version)
	if err != nil {
		// TODO: maybe set something like v1.0.0-{meta.Version} for developing purpose
		return fmt.Errorf("parse semver: %w", err)
	}

	r.metricStorage.Grouped().ExpireGroupMetrics(metricUpdatingFailedGroup)

	var releaseForUpdate *v1alpha1.DeckhouseRelease

	// check sequence from the start if no deckhouse release deployed
	if len(releases) > 0 {
		releaseForUpdate = releases[0]
	}
	releasesInCluster := releases

	// shortened slice for only releases after deployed
	if deployedRelease != nil {
		releaseForUpdate = deployedRelease
		releasesInCluster = releasesFromDeployed
	}

	err = r.createReleases(ctx, releaseMetadata, releaseForUpdate, releasesInCluster, newSemver)
	if err != nil {
		return fmt.Errorf("create releases: %w", err)
	}

	// update image hash only if create all releases
	r.releaseVersionImageHash = imageInfo.Digest.String()

	sort.Sort(sort.Reverse(releaseUpdater.ByVersion[*v1alpha1.DeckhouseRelease](releasesInCluster)))

	// filter by skipped and suspended
	for _, release := range releasesInCluster {
		switch release.GetVersion().Compare(newSemver) {
		// Lesser than
		case -1:
			// pass
		// Greater than
		case 1:
			// cleanup versions which are older than current version in a specified channel and are in a Pending state
			if release.Status.Phase == v1alpha1.DeckhouseReleasePhasePending {
				err = r.k8sClient.Delete(ctx, release, client.PropagationPolicy(metav1.DeletePropagationBackground))
				if err != nil {
					return fmt.Errorf("delete old release: %w", err)
				}
			}
		// Equal
		case 0:
			r.logger.Debug("Release already exists", slog.String("version", release.GetVersion().Original()))

			switch release.Status.Phase {
			case v1alpha1.DeckhouseReleasePhasePending, "":
				if releaseMetadata.Suspend {
					err := r.patchSetSuspendAnnotation(ctx, release, true)
					if err != nil {
						return fmt.Errorf("patch suspend annotation: %w", err)
					}
				}

			case v1alpha1.DeckhouseReleasePhaseSuspended:
				if !releaseMetadata.Suspend {
					err := r.patchSetSuspendAnnotation(ctx, release, false)
					if err != nil {
						return fmt.Errorf("patch suspend annotation: %w", err)
					}
				}
			}

			return nil
		default:
			r.logger.Error("bad compare output, possibly bug")
		}
	}

	return nil
}

// isUpdatingSequence checks that version 'a' and 'b' allowed to updating from 'a' to 'b'
// this helper function is to calculate necessary of registry listing
// 'a' version must be lower than 'b' version
// if 'a' major version +1 is lower than 'b' major version - it's no updating sequence
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

func (r *DeckhouseReleaseFetcher) listDeckhouseReleases(ctx context.Context) ([]*v1alpha1.DeckhouseRelease, error) {
	releases := new(v1alpha1.DeckhouseReleaseList)

	if err := r.k8sClient.List(ctx, releases); err != nil {
		return nil, fmt.Errorf("get deckhouse releases: %w", err)
	}

	result := make([]*v1alpha1.DeckhouseRelease, 0, len(releases.Items))

	for _, release := range releases.Items {
		result = append(result, &release)
	}

	return result, nil
}

func (r *DeckhouseReleaseFetcher) restoreCurrentDeployedRelease(ctx context.Context, tag string) (*v1alpha1.DeckhouseRelease, error) {
	var releaseMetadata *ReleaseMetadata

	image, err := r.registryClient.Image(ctx, tag)
	if err != nil {
		r.logger.Warn("couldn't get current deployed release's image from registry", slog.String("image", tag), log.Err(err))
	}

	releaseMetadata, err = r.fetchReleaseMetadata(image)
	if err != nil {
		r.logger.Warn("couldn't fetch current deployed release's image metadata", slog.String("image", tag), log.Err(err))
	}

	release := &v1alpha1.DeckhouseRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DeckhouseRelease",
			APIVersion: "deckhouse.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: tag,
			Annotations: map[string]string{
				v1alpha1.DeckhouseReleaseAnnotationIsUpdating:      "false",
				v1alpha1.DeckhouseReleaseAnnotationNotified:        "false",
				v1alpha1.DeckhouseReleaseAnnotationCurrentRestored: "true",
				v1alpha1.DeckhouseReleaseAnnotationChangeCause:     "check release (restore release)",
			},
			Labels: map[string]string{
				"heritage": "deckhouse",
			},
		},
		Spec: v1alpha1.DeckhouseReleaseSpec{
			Version: tag,
		},
		Approved: true,
	}

	if releaseMetadata != nil {
		release.Spec.Requirements = releaseMetadata.Requirements
		release.Spec.ChangelogLink = fmt.Sprintf("https://github.com/deckhouse/deckhouse/releases/tag/%s", releaseMetadata.Version)
	}

	err = client.IgnoreAlreadyExists(r.k8sClient.Create(ctx, release))
	if err != nil {
		return nil, fmt.Errorf("create release: %w", err)
	}

	return release, nil
}

// createReleases flow:
// 1) if deployed release patch version is lower than channel (with same minor and major) - create from channel
// 2) if deployed release minor version is lower than channel (with same major) - create from channel
// 3) if deployed release minor version is lower by 2 or more than channel (with same major) - look at releases in cluster
// 3.1) if update sequence between deployed release and last release in cluster is broken - get releases from registry between deployed and version from channel, and create releases
// 3.2) if update sequence between deployed release and last release in cluster not broken - check update sequence between last release in cluster and version in channel
// 3.2.1) if update sequence between last release in cluster and version in channel is broken - get releases from registry between last release in cluster and version from channel, and create releases
// 3.2.2) if update sequence between last release in cluster and version in channel not broken - create from channel
// 3.3) if update sequences not broken - create from channel
func (r *DeckhouseReleaseFetcher) createReleases(
	ctx context.Context,
	releaseMetadata *ReleaseMetadata,
	releaseForUpdate *v1alpha1.DeckhouseRelease,
	releasesInCluster []*v1alpha1.DeckhouseRelease,
	newSemver *semver.Version) error {
	var (
		cooldownUntil, notificationShiftTime *metav1.Time
	)

	if releaseMetadata.Cooldown != nil {
		cooldownUntil = releaseMetadata.Cooldown
	}

	if len(releasesInCluster) == 0 {
		err := r.createRelease(ctx, releaseMetadata, cooldownUntil, notificationShiftTime, "no releases in cluster")
		if err != nil {
			return fmt.Errorf("create release %s: %w", releaseMetadata.Version, err)
		}

		return nil
	}

	// create release if deployed release and new release are in updating sequence
	actual := releaseForUpdate
	if isUpdatingSequence(actual.GetVersion(), newSemver) {
		err := r.createRelease(ctx, releaseMetadata, cooldownUntil, notificationShiftTime, "from deployed")
		if err != nil {
			return fmt.Errorf("create release %s: %w", releaseMetadata.Version, err)
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
			// TODO: remove cooldown?
			err := r.createRelease(ctx, releaseMetadata, cooldownUntil, notificationShiftTime, "from last release in cluster")
			if err != nil {
				return fmt.Errorf("create release %s: %w", releaseMetadata.Version, err)
			}

			return nil
		}
	}

	// inherit cooldown from previous patch release
	// we need this to automatically set cooldown for next patch releases
	if cooldownUntil == nil &&
		actual.GetCooldownUntil() != nil &&
		actual.GetVersion().Major() == newSemver.Major() &&
		actual.GetVersion().Minor() == newSemver.Minor() {
		cooldownUntil = &metav1.Time{Time: *actual.GetCooldownUntil()}
	}

	if actual.GetNotificationShift() &&
		actual.GetApplyAfter() != nil &&
		actual.GetVersion().Major() == newSemver.Major() &&
		actual.GetVersion().Minor() == newSemver.Minor() {
		notificationShiftTime = &metav1.Time{Time: *actual.GetApplyAfter()}
	}

	metas, err := r.GetNewReleasesMetadata(ctx, actual.GetVersion(), newSemver)
	if err != nil {
		r.logger.Error("step by step update failed", log.Err(err))

		labels := map[string]string{
			"version": releaseForUpdate.GetVersion().Original(),
		}

		r.metricStorage.Grouped().GaugeSet(metricUpdatingFailedGroup, "d8_updating_is_failed", 1, labels)

		return err
	}

	for _, meta := range metas {
		*releaseMetadata = meta

		err = r.createRelease(ctx, releaseMetadata, cooldownUntil, notificationShiftTime, "step-by-step")
		if err != nil {
			return fmt.Errorf("create release %s: %w", releaseMetadata.Version, err)
		}

		if releaseMetadata.Cooldown != nil {
			cooldownUntil = releaseMetadata.Cooldown
		}
	}

	return nil
}

func (r *DeckhouseReleaseFetcher) createRelease(
	ctx context.Context,
	releaseMetadata *ReleaseMetadata,
	cooldownUntil,
	notificationShiftTime *metav1.Time,
	createProcess string,
) error {
	var applyAfter *metav1.Time

	ts := metav1.Time{Time: r.clock.Now()}
	if releaseMetadata.IsCanaryRelease(r.GetReleaseChannel()) {
		// if cooldown is set, calculate canary delay from cooldown time, not current
		if cooldownUntil != nil && cooldownUntil.After(ts.Time) {
			ts = *cooldownUntil
		}
		applyAfter = releaseMetadata.CalculateReleaseDelay(r.GetReleaseChannel(), ts, r.clusterUUID)
	}

	// inherit applyAfter from notified release
	if notificationShiftTime != nil && notificationShiftTime.After(ts.Time) {
		applyAfter = notificationShiftTime
	}

	var disruptions []string
	if len(releaseMetadata.Disruptions) > 0 {
		version, err := semver.NewVersion(releaseMetadata.Version)
		if err != nil {
			return err
		}
		disruptionsVersion := fmt.Sprintf("%d.%d", version.Major(), version.Minor())
		disruptions = releaseMetadata.Disruptions[disruptionsVersion]
	}

	enabledModulesChangelog := r.generateChangelogForEnabledModules(releaseMetadata)
	changeCause := "check release"
	if createProcess != "" {
		changeCause += " (" + createProcess + ")"
	}

	release := &v1alpha1.DeckhouseRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DeckhouseRelease",
			APIVersion: "deckhouse.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: releaseMetadata.Version,
			Annotations: map[string]string{
				v1alpha1.DeckhouseReleaseAnnotationIsUpdating:  "false",
				v1alpha1.DeckhouseReleaseAnnotationNotified:    "false",
				v1alpha1.DeckhouseReleaseAnnotationChangeCause: changeCause,
			},
			Labels: map[string]string{
				"heritage": "deckhouse",
			},
		},
		Spec: v1alpha1.DeckhouseReleaseSpec{
			Version:       releaseMetadata.Version,
			ApplyAfter:    applyAfter,
			Requirements:  releaseMetadata.Requirements,
			Disruptions:   disruptions,
			Changelog:     enabledModulesChangelog,
			ChangelogLink: fmt.Sprintf("https://github.com/deckhouse/deckhouse/releases/tag/%s", releaseMetadata.Version),
		},
		Approved: false,
	}

	if releaseMetadata.Suspend {
		release.ObjectMeta.Annotations[v1alpha1.DeckhouseReleaseAnnotationSuspended] = "true"
	}
	if cooldownUntil != nil {
		release.ObjectMeta.Annotations[v1alpha1.DeckhouseReleaseAnnotationCooldown] = cooldownUntil.UTC().Format(time.RFC3339)
	}
	if notificationShiftTime != nil {
		release.ObjectMeta.Annotations[v1alpha1.DeckhouseReleaseAnnotationNotificationTimeShift] = "true"
	}

	return client.IgnoreAlreadyExists(r.k8sClient.Create(ctx, release))
}

func (r *DeckhouseReleaseFetcher) patchSetSuspendAnnotation(ctx context.Context, release *v1alpha1.DeckhouseRelease, suspend bool) error {
	patch := client.RawPatch(types.MergePatchType, buildSuspendAnnotation(suspend))

	err := r.k8sClient.Patch(ctx, release, patch)
	if err != nil {
		return fmt.Errorf("patch release %v: %w", release.Name, err)
	}

	err = r.k8sClient.Status().Patch(ctx, release, patch)
	if err != nil {
		return fmt.Errorf("patch release %v status: %w", release.Name, err)
	}

	return nil
}

var ErrImageNotChanged = errors.New("image not changed")

type ImageInfo struct {
	Metadata *ReleaseMetadata
	Image    registryv1.Image
	Digest   registryv1.Hash
}

func (dcr *DeckhouseReleaseFetcher) GetNewImageInfo(ctx context.Context, previousImageHash string) (*ImageInfo, error) {
	image, err := dcr.registryClient.Image(ctx, dcr.GetReleaseChannel())
	if err != nil {
		return nil, fmt.Errorf("get image from channel '%s': %w", dcr.GetReleaseChannel(), err)
	}

	imageDigest, err := image.Digest()
	if err != nil {
		return nil, fmt.Errorf("get image digest: %w", err)
	}

	if previousImageHash == imageDigest.String() {
		return &ImageInfo{
			Image:  image,
			Digest: imageDigest,
		}, ErrImageNotChanged
	}

	releaseMeta, err := dcr.fetchReleaseMetadata(image)
	if err != nil {
		return nil, fmt.Errorf("fetch image metadata: %w", err)
	}

	if releaseMeta.Version == "" {
		return nil, fmt.Errorf("version not found, probably image is broken or layer does not exist")
	}

	return &ImageInfo{
		Image:    image,
		Digest:   imageDigest,
		Metadata: releaseMeta,
	}, nil
}

type releaseReader struct {
	versionReader   *bytes.Buffer
	changelogReader *bytes.Buffer
}

func (rr *releaseReader) untarMetadata(rc io.Reader) error {
	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of archive
			return nil
		}
		if err != nil {
			return err
		}

		switch hdr.Name {
		case "version.json":
			_, err = io.Copy(rr.versionReader, tr)
			if err != nil {
				return err
			}
		case "changelog.yaml", "changelog.yml":
			_, err = io.Copy(rr.changelogReader, tr)
			if err != nil {
				return err
			}

		default:
			continue
		}
	}
}

var ErrImageIsNil = errors.New("image is nil")

// TODO: make registry service with this method
func (dcr *DeckhouseReleaseFetcher) fetchReleaseMetadata(img registryv1.Image) (*ReleaseMetadata, error) {
	if img == nil {
		return nil, ErrImageIsNil
	}

	meta := new(ReleaseMetadata)

	rc, err := cr.Extract(img)
	if err != nil {
		return nil, fmt.Errorf("extract image: %w", err)
	}
	defer rc.Close()

	rr := &releaseReader{
		versionReader:   bytes.NewBuffer(nil),
		changelogReader: bytes.NewBuffer(nil),
	}

	err = rr.untarMetadata(rc)
	if err != nil {
		return nil, fmt.Errorf("untar metadata: %w", err)
	}

	if rr.versionReader.Len() > 0 {
		err = json.NewDecoder(rr.versionReader).Decode(&meta)
		if err != nil {
			return nil, fmt.Errorf("metadata decode: %w", err)
		}
	}

	if rr.changelogReader.Len() > 0 {
		var changelog map[string]any

		err = yaml.NewDecoder(rr.changelogReader).Decode(&changelog)
		if err != nil {
			// if changelog build failed - warn about it but don't fail the release
			dcr.logger.Warn("Unmarshal CHANGELOG yaml failed", log.Err(err))

			meta.Changelog = make(map[string]any)

			return meta, nil
		}

		meta.Changelog = changelog
	}

	cooldown := dcr.fetchCooldown(img)
	if cooldown != nil {
		meta.Cooldown = cooldown
	}

	return meta, nil
}

func (dcr *DeckhouseReleaseFetcher) fetchCooldown(image registryv1.Image) *metav1.Time {
	cfg, err := image.ConfigFile()
	if err != nil {
		dcr.logger.Warnf("image config error: %s", err)
		return nil
	}

	if cfg == nil {
		return nil
	}

	if len(cfg.Config.Labels) == 0 {
		return nil
	}

	if v, ok := cfg.Config.Labels["cooldown"]; ok {
		t, err := parseTime(v)
		if err != nil {
			dcr.logger.Errorf("parse cooldown(%s) error: %s", v, err)
			return nil
		}
		mt := metav1.NewTime(t)

		return &mt
	}

	return nil
}

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02 15:04", s)
	if err == nil {
		return t, nil
	}

	t, err = time.Parse("2006-01-02 15:04:05", s)
	if err == nil {
		return t, nil
	}

	return time.Parse(time.RFC3339, s)
}

// FetchReleasesMetadata realize step by step update
func (dcr *DeckhouseReleaseFetcher) GetNewReleasesMetadata(ctx context.Context, actual, target *semver.Version) ([]ReleaseMetadata, error) {
	vers, err := dcr.getNewVersions(ctx, actual, target)
	if err != nil {
		return nil, fmt.Errorf("get next version: %w", err)
	}

	result := make([]ReleaseMetadata, 0, len(vers))

	current := actual
	for idx, ver := range vers {
		// if next version is not in sequence with actual
		if !isUpdatingSequence(current, ver) {
			if idx == 0 {
				return nil, fmt.Errorf("versions is not in sequence: '%s' and '%s'", actual.Original(), ver.Original())
			}

			dcr.logger.Warn("not sequential version", slog.String("previous", actual.Original()), slog.String("next", ver.Original()))

			break
		}

		image, err := dcr.registryClient.Image(ctx, ver.Original())
		if err != nil {
			return nil, fmt.Errorf("get image: %w", err)
		}

		releaseMeta, err := dcr.fetchReleaseMetadata(image)
		if err != nil {
			return nil, fmt.Errorf("fetch release metadata: %w", err)
		}

		if releaseMeta.Version == "" {
			return nil, fmt.Errorf("version not found. Probably image is broken or layer does not exist")
		}

		result = append(result, *releaseMeta)

		current = ver
	}

	return result, nil
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
func (dcr *DeckhouseReleaseFetcher) getNewVersions(ctx context.Context, actual, target *semver.Version) ([]*semver.Version, error) {
	tags, err := dcr.registryClient.ListTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	versionMatcher := regexp.MustCompile(`^v(([0-9]+).([0-9]+).([0-9]+))$`)

	collection := make([]*semver.Version, 0)

	// to be sure, they are sorted correctly we parse them before working
	for _, ver := range tags {
		if !versionMatcher.MatchString(ver) {
			dcr.logger.Debug("not suitable. This version will be skipped.", slog.String("version", ver))

			continue
		}

		newSemver, err := semver.NewVersion(ver)
		if err != nil {
			dcr.logger.Warn("unable to parse semver from the registry. This version will be skipped.", slog.String("version", ver))

			continue
		}

		collection = append(collection, newSemver)
	}

	if len(collection) == 0 {
		return nil, fmt.Errorf("no matched tags in registry")
	}

	sort.Sort(semver.Collection(collection))

	result := make([]*semver.Version, 0)
	prevVersion := new(semver.Version)

	for _, ver := range collection {
		// skip all versions out of actual and target range
		if actual.Major() > ver.Major() ||
			(actual.Major() == ver.Major() && actual.Minor() > ver.Minor()) ||
			target.Major() < ver.Major() ||
			(target.Major() == ver.Major() && target.Minor() < ver.Minor()) {
			continue
		}

		// add only last minor or last major releases
		if !prevVersion.Equal(new(semver.Version)) &&
			(prevVersion.Major() < ver.Major() || prevVersion.Minor() < ver.Minor()) {
			result = append(result, prevVersion)
		}

		prevVersion = ver
	}

	// if last patch is more than target release - skip target
	if prevVersion.Major() > target.Major() ||
		(prevVersion.Major() == target.Major() && prevVersion.Minor() > target.Minor()) ||
		(prevVersion.Major() == target.Major() && prevVersion.Minor() == target.Minor() && prevVersion.Patch() > target.Patch()) {
		dcr.logger.Warn("last release is not equals to target, skipped", slog.String("last", prevVersion.Original()), slog.String("target", target.Original()))
	} else {
		result = append(result, prevVersion)
	}

	// trim max patch from current minor version
	if len(result) > 1 {
		if result[0].Minor() == actual.Minor() {
			result = result[1:]
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no acceptable for step by step update tags in registry")
	}

	return result, nil
}

var globalModules = []string{"candi", "deckhouse-controller", "global"}

func (r *DeckhouseReleaseFetcher) generateChangelogForEnabledModules(releaseMetadata *ReleaseMetadata) map[string]interface{} {
	enabledModules := r.moduleManager.GetEnabledModuleNames()
	enabledModulesChangelog := make(map[string]interface{})

	for _, enabledModule := range enabledModules {
		if v, ok := releaseMetadata.Changelog[enabledModule]; ok {
			enabledModulesChangelog[enabledModule] = v
		}
	}

	// enable global modules
	for _, globalModule := range globalModules {
		if v, ok := releaseMetadata.Changelog[globalModule]; ok {
			enabledModulesChangelog[globalModule] = v
		}
	}

	return enabledModulesChangelog
}

type ReleaseMetadata struct {
	Version      string                    `json:"version"`
	Canary       map[string]canarySettings `json:"canary"`
	Requirements map[string]string         `json:"requirements"`
	Disruptions  map[string][]string       `json:"disruptions"`
	Suspend      bool                      `json:"suspend"`

	Changelog map[string]interface{}

	Cooldown *metav1.Time `json:"-"`
}

func (m *ReleaseMetadata) IsCanaryRelease(channel string) bool {
	settings := m.releaseCanarySettings(channel)
	return settings.Enabled
}

func (m *ReleaseMetadata) releaseCanarySettings(channel string) canarySettings {
	return m.Canary[channel]
}

// https://github.com/deckhouse/deckhouse/issues/332
func (m *ReleaseMetadata) CalculateReleaseDelay(channel string, ts metav1.Time, clusterUUID string) *metav1.Time {
	hash := murmur3.Sum64([]byte(clusterUUID + m.Version))
	wave := hash % uint64(m.releaseCanarySettings(channel).Waves)

	if wave != 0 {
		delay := time.Duration(wave) * m.releaseCanarySettings(channel).Interval.Duration
		applyAfter := metav1.NewTime(ts.Add(delay))
		return &applyAfter
	}

	return nil
}

type canarySettings struct {
	Enabled  bool            `json:"enabled"`
	Waves    uint            `json:"waves"`
	Interval libapi.Duration `json:"interval"` // in minutes
}

func buildSuspendAnnotation(suspend bool) []byte {
	var annotationValue interface{}

	if suspend {
		annotationValue = "true"
	}

	p := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				v1alpha1.DeckhouseReleaseAnnotationSuspended: annotationValue,
			},
		},
	}

	if !suspend {
		p["status"] = map[string]interface{}{
			"phase":   "Pending",
			"message": "",
		}
	}

	patch, _ := json.Marshal(p)
	return patch
}

type moduleManager interface {
	GetEnabledModuleNames() []string
	IsModuleEnabled(name string) bool
}
