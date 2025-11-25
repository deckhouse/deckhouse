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
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	registryv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/iancoleman/strcase"
	"github.com/jonboulle/clockwork"
	"github.com/spaolacci/murmur3"
	"go.opentelemetry.io/otel"
	"gopkg.in/yaml.v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	releaseUpdater "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/releaseupdater"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/go_lib/libapi"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

const (
	serviceName                 = "check-release"
	ltsChannelName              = "lts"
	checkDeckhouseReleasePeriod = 3 * time.Minute
)

func (r *deckhouseReleaseReconciler) checkDeckhouseReleaseLoop(ctx context.Context) {
	wait.UntilWithContext(ctx, func(ctx context.Context) {
		err := r.checkDeckhouseRelease(ctx)
		if err != nil {
			r.logger.Warn("check Deckhouse release", log.Err(err))
		}
	}, checkDeckhouseReleasePeriod)
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

	metricStorage metricsstorage.Storage

	logger *log.Logger
}

// checkDeckhouseRelease create fetcher and start
func (r *deckhouseReleaseReconciler) checkDeckhouseRelease(ctx context.Context) error {
	ctx, span := otel.Tracer(serviceName).Start(ctx, "checkDeckhouseRelease")
	defer span.End()

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

	releaseFetcher := NewDeckhouseReleaseFetcher(cfg)

	return releaseFetcher.fetchDeckhouseRelease(ctx)
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

	metricStorage metricsstorage.Storage

	logger *log.Logger
}

func (f *DeckhouseReleaseFetcher) GetReleaseChannel() string {
	return f.releaseChannel
}

// fetchDeckhouseRelease is a complete flow for loop
func (f *DeckhouseReleaseFetcher) fetchDeckhouseRelease(ctx context.Context) error {
	ctx, span := otel.Tracer(serviceName).Start(ctx, "fetchDeckhouseRelease")
	defer span.End()

	releases, err := f.listDeckhouseReleases(ctx)
	if err != nil {
		return fmt.Errorf("list deckhouse releases: %w", err)
	}

	var releaseForUpdate *v1alpha1.DeckhouseRelease
	releasesInCluster := make([]*v1alpha1.DeckhouseRelease, 0, len(releases))

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

	// restore current deployed release if no deployed releases found
	if deployedRelease == nil {
		f.logger.Warn("deployed deckhouse-release is not found, restoring...")

		restored, err := f.restoreCurrentDeployedRelease(ctx, f.deckhouseVersion)
		if err != nil {
			return fmt.Errorf("restore current deployed release: %w", err)
		}

		f.logger.Warn("deployed deckhouse-release restored")

		restored.Status.Phase = v1alpha1.DeckhouseReleasePhaseDeployed

		releasesInCluster = append(releasesInCluster, restored)
		releaseForUpdate = restored

		idx, _ = getLatestDeployedRelease(releasesInCluster)
		if idx != -1 {
			releasesInCluster = releasesInCluster[:idx+1]
		}
	}

	// get image info from release channel
	imageInfo, err := f.GetReleaseImageInfo(ctx, f.releaseVersionImageHash)
	if err != nil && !errors.Is(err, ErrImageNotChanged) {
		return fmt.Errorf("get new image: %w", err)
	}

	// no new image found
	if err != nil {
		return nil
	}

	newSemver, err := semver.NewVersion(imageInfo.Metadata.Version)
	if err != nil {
		// TODO: maybe set something like v1.0.0-{meta.Version} for developing purpose
		return fmt.Errorf("parse semver: %w", err)
	}

	// forbid pre-release versions
	if newSemver.Prerelease() != "" {
		return fmt.Errorf("pre-release versions are not supported: %s", newSemver.Original())
	}

	f.metricStorage.Grouped().ExpireGroupMetrics(metrics.D8UpdatingIsFailed)

	// sort releases before
	sort.Sort(releaseUpdater.ByVersion[*v1alpha1.DeckhouseRelease](releasesInCluster))

	lastCreatedMeta, err := f.ensureReleases(ctx, imageInfo.Metadata, releaseForUpdate, releasesInCluster, newSemver)
	if err != nil {
		return fmt.Errorf("create releases: %w", err)
	}

	// update image hash only if create all releases
	f.releaseVersionImageHash = imageInfo.Digest.String()

	sort.Sort(sort.Reverse(releaseUpdater.ByVersion[*v1alpha1.DeckhouseRelease](releasesInCluster)))

	const (
		lesserThan  = -1
		greaterThan = 1
		equal       = 0
	)

	// filter by skipped and suspended
	for _, release := range releasesInCluster {
		if release.Status.Phase != v1alpha1.DeckhouseReleasePhasePending &&
			release.Status.Phase != v1alpha1.DeckhouseReleasePhaseSuspended {
			continue
		}

		switch release.GetVersion().Compare(newSemver) {
		case lesserThan:
			// pass
		case greaterThan:
			// cleanup versions which are older than current version in a specified channel and are in a Pending state
			if release.Status.Phase == v1alpha1.DeckhouseReleasePhasePending {
				err = f.k8sClient.Delete(ctx, release, client.PropagationPolicy(metav1.DeletePropagationBackground))
				if err != nil {
					return fmt.Errorf("delete old release: %w", err)
				}
			}
		case equal:
			f.logger.Debug("Release already exists", slog.String("version", release.GetVersion().Original()))

			switch release.Status.Phase {
			case v1alpha1.DeckhouseReleasePhasePending, "":
				if lastCreatedMeta.Suspend {
					err := f.patchSetSuspendAnnotation(ctx, release, true)
					if err != nil {
						return fmt.Errorf("patch suspend annotation: %w", err)
					}
				}

			case v1alpha1.DeckhouseReleasePhaseSuspended:
				if !lastCreatedMeta.Suspend {
					err := f.patchSetSuspendAnnotation(ctx, release, false)
					if err != nil {
						return fmt.Errorf("patch suspend annotation: %w", err)
					}
				}
			}

			return nil
		default:
			f.logger.Error("bad compare output, possibly bug")
		}
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

func (f *DeckhouseReleaseFetcher) listDeckhouseReleases(ctx context.Context) ([]*v1alpha1.DeckhouseRelease, error) {
	releases := new(v1alpha1.DeckhouseReleaseList)

	if err := f.k8sClient.List(ctx, releases); err != nil {
		return nil, fmt.Errorf("get deckhouse releases: %w", err)
	}

	result := make([]*v1alpha1.DeckhouseRelease, 0, len(releases.Items))

	for _, release := range releases.Items {
		result = append(result, &release)
	}

	return result, nil
}

// restoreCurrentDeployedRelease restores release in cluster by given tag,
// if not found any data about release - creating it without them
func (f *DeckhouseReleaseFetcher) restoreCurrentDeployedRelease(ctx context.Context, tag string) (*v1alpha1.DeckhouseRelease, error) {
	ctx, span := otel.Tracer(serviceName).Start(ctx, "restoreCurrentDeployedRelease")
	defer span.End()

	var releaseMetadata *ReleaseMetadata

	image, err := f.registryClient.Image(ctx, tag)
	if err != nil {
		f.logger.Warn("couldn't get current deployed release's image from registry", slog.String("image", tag), log.Err(err))
	}

	releaseMetadata, err = f.fetchReleaseMetadata(ctx, image)
	if err != nil {
		f.logger.Warn("couldn't fetch current deployed release's image metadata", slog.String("image", tag), log.Err(err))
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

	err = client.IgnoreAlreadyExists(f.k8sClient.Create(ctx, release))
	if err != nil {
		return nil, fmt.Errorf("create release: %w", err)
	}

	patch := client.MergeFrom(release.DeepCopy())

	release.Status.Phase = v1alpha1.DeckhouseReleasePhaseDeployed

	err = f.k8sClient.Status().Patch(ctx, release, patch)
	if err != nil {
		return nil, fmt.Errorf("patch release status: %w", err)
	}

	return release, nil
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
func (f *DeckhouseReleaseFetcher) ensureReleases(
	ctx context.Context,
	releaseMetadata *ReleaseMetadata,
	releaseForUpdate *v1alpha1.DeckhouseRelease,
	releasesInCluster []*v1alpha1.DeckhouseRelease,
	newSemver *semver.Version) (*ReleaseMetadata, error) {
	ctx, span := otel.Tracer(serviceName).Start(ctx, "ensureReleases")
	defer span.End()

	var (
		notificationShiftTime *metav1.Time
	)

	// if no releases in cluster - create from channel
	if len(releasesInCluster) == 0 {
		err := f.createRelease(ctx, releaseMetadata, notificationShiftTime, "no releases in cluster")
		if err != nil {
			return nil, fmt.Errorf("create release %s: %w", releaseMetadata.Version, err)
		}

		return releaseMetadata, nil
	}

	// if release channel is LTS - create release from channel
	if strings.EqualFold(f.releaseChannel, ltsChannelName) {
		err := f.createRelease(ctx, releaseMetadata, notificationShiftTime, "lts channel")
		if err != nil {
			return nil, fmt.Errorf("create release %s: %w", releaseMetadata.Version, err)
		}

		return releaseMetadata, nil
	}

	// create release if deployed release and new release are in updating sequence
	actual := releaseForUpdate
	if isUpdatingSequence(actual.GetVersion(), newSemver) {
		err := f.createRelease(ctx, releaseMetadata, notificationShiftTime, "from deployed")
		if err != nil {
			return nil, fmt.Errorf("create release %s: %w", releaseMetadata.Version, err)
		}

		return releaseMetadata, nil
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
			err := f.createRelease(ctx, releaseMetadata, notificationShiftTime, "from last release in cluster")
			if err != nil {
				return nil, fmt.Errorf("create release %s: %w", releaseMetadata.Version, err)
			}

			return releaseMetadata, nil
		}
	}

	if actual.GetNotificationShift() &&
		actual.GetApplyAfter() != nil &&
		actual.GetVersion().Major() == newSemver.Major() &&
		actual.GetVersion().Minor() == newSemver.Minor() {
		notificationShiftTime = &metav1.Time{Time: *actual.GetApplyAfter()}
	}

	metricLabels := map[string]string{
		"version": releaseForUpdate.GetVersion().Original(),
	}

	metas, err := f.GetNewReleasesMetadata(ctx, actual.GetVersion(), newSemver)
	if err != nil {
		f.logger.Error("step by step update failed", log.Err(err))

		f.metricStorage.Grouped().GaugeSet(metrics.D8UpdatingIsFailed, metrics.D8UpdatingIsFailed, 1, metricLabels)

		return nil, fmt.Errorf("get new releases metadata: %w", err)
	}

	f.metricStorage.Grouped().GaugeSet(metrics.D8UpdatingIsFailed, metrics.D8UpdatingIsFailed, 0, metricLabels)

	for _, meta := range metas {
		releaseMetadata = &meta

		err = f.createRelease(ctx, releaseMetadata, notificationShiftTime, "step-by-step")
		if err != nil {
			return nil, fmt.Errorf("create release %s: %w", releaseMetadata.Version, err)
		}
	}

	return releaseMetadata, nil
}

// createRelease create new release by metadata,
// if canary - add time applyAfter time,
// if has disruptions - add disruptions,
// also add suspend annotation if release is suspended
func (f *DeckhouseReleaseFetcher) createRelease(
	ctx context.Context,
	releaseMetadata *ReleaseMetadata,
	notificationShiftTime *metav1.Time,
	createProcess string,
) error {
	ctx, span := otel.Tracer(serviceName).Start(ctx, "createRelease")
	defer span.End()

	var applyAfter *metav1.Time

	ts := metav1.Time{Time: f.clock.Now()}
	if releaseMetadata.IsCanaryRelease(f.GetReleaseChannel()) {
		// if cooldown is set, calculate canary delay from cooldown time, not current
		applyAfter = releaseMetadata.CalculateReleaseDelay(f.GetReleaseChannel(), ts, f.clusterUUID)
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

	enabledModulesChangelog := f.generateChangelogForEnabledModules(releaseMetadata)
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
	if notificationShiftTime != nil {
		release.ObjectMeta.Annotations[v1alpha1.DeckhouseReleaseAnnotationNotificationTimeShift] = "true"
	}

	return client.IgnoreAlreadyExists(f.k8sClient.Create(ctx, release))
}

func (f *DeckhouseReleaseFetcher) patchSetSuspendAnnotation(ctx context.Context, release *v1alpha1.DeckhouseRelease, suspend bool) error {
	patch := client.RawPatch(types.MergePatchType, buildSuspendAnnotation(suspend))

	err := f.k8sClient.Patch(ctx, release, patch)
	if err != nil {
		return fmt.Errorf("patch release %v: %w", release.Name, err)
	}

	err = f.k8sClient.Status().Patch(ctx, release, patch)
	if err != nil {
		return fmt.Errorf("patch release %v status: %w", release.Name, err)
	}

	return nil
}

var ErrImageNotChanged = errors.New("image not changed")

type ReleaseImageInfo struct {
	Metadata *ReleaseMetadata
	Image    registryv1.Image
	Digest   registryv1.Hash
}

// GetReleaseImageInfo get Image, Digest and release metadata using imageTag with existing registry client
// return error if version.json not found in metadata
// return ErrImageNotChanged with ReleaseImageInfo if image hash matches with previousImageHash
func (f *DeckhouseReleaseFetcher) GetReleaseImageInfo(ctx context.Context, previousImageHash string) (*ReleaseImageInfo, error) {
	ctx, span := otel.Tracer(serviceName).Start(ctx, "getNewImageInfo")
	defer span.End()

	image, err := f.registryClient.Image(ctx, f.GetReleaseChannel())
	if err != nil {
		return nil, fmt.Errorf("get image from channel '%s': %w", f.GetReleaseChannel(), err)
	}

	imageDigest, err := image.Digest()
	if err != nil {
		return nil, fmt.Errorf("get image digest: %w", err)
	}

	if previousImageHash == imageDigest.String() {
		return &ReleaseImageInfo{
			Image:  image,
			Digest: imageDigest,
		}, ErrImageNotChanged
	}

	releaseMeta, err := f.fetchReleaseMetadata(ctx, image)
	if err != nil {
		return nil, fmt.Errorf("fetch image metadata: %w", err)
	}

	if releaseMeta.Version == "" {
		return nil, fmt.Errorf("version not found, probably image is broken or layer does not exist")
	}

	return &ReleaseImageInfo{
		Image:    image,
		Digest:   imageDigest,
		Metadata: releaseMeta,
	}, nil
}

type releaseReader struct {
	versionReader   *bytes.Buffer
	changelogReader *bytes.Buffer
	moduleReader    *bytes.Buffer
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
		case "module.yaml":
			_, err := io.Copy(rr.moduleReader, tr)
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
func (f *DeckhouseReleaseFetcher) fetchReleaseMetadata(ctx context.Context, img registryv1.Image) (*ReleaseMetadata, error) {
	_, span := otel.Tracer(serviceName).Start(ctx, "fetchReleaseMetadata")
	defer span.End()

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
		moduleReader:    bytes.NewBuffer(nil),
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

	if rr.moduleReader.Len() > 0 {
		var moduleDefinition moduletypes.Definition
		err = yaml.NewDecoder(rr.moduleReader).Decode(&moduleDefinition)
		if err != nil {
			return nil, fmt.Errorf("unmarshal module yaml failed: %w", err)
		}

		meta.ModuleDefinition = &moduleDefinition
		if moduleDefinition.Requirements != nil {
			if meta.Requirements == nil {
				meta.Requirements = make(map[string]string, 1)
			}
			meta.Requirements["kubernetes"] = moduleDefinition.Requirements.Kubernetes
		}
	}

	if rr.changelogReader.Len() > 0 {
		var changelog map[string]any

		err = yaml.NewDecoder(rr.changelogReader).Decode(&changelog)
		if err != nil {
			// if changelog build failed - warn about it but don't fail the release
			f.logger.Warn("Unmarshal CHANGELOG yaml failed", log.Err(err))

			changelog = make(map[string]any)
		}

		meta.Changelog = changelog
	}

	return meta, nil
}

func (f *DeckhouseReleaseFetcher) fetchCooldown(image registryv1.Image) *metav1.Time {
	cfg, err := image.ConfigFile()
	if err != nil {
		f.logger.Warn("image config error", log.Err(err))
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
			f.logger.Error("parse cooldown", slog.String("cooldown", v), log.Err(err))
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
func (f *DeckhouseReleaseFetcher) GetNewReleasesMetadata(ctx context.Context, actual, target *semver.Version) ([]ReleaseMetadata, error) {
	ctx, span := otel.Tracer(serviceName).Start(ctx, "getNewReleasesMetadata")
	defer span.End()

	vers, err := f.getNewVersions(ctx, actual, target)
	if err != nil {
		return nil, fmt.Errorf("get next versions: %w", err)
	}

	result := make([]ReleaseMetadata, 0, len(vers))

	current := actual
	for idx, ver := range vers {
		// if next version is not in sequence with actual
		if !isUpdatingSequence(current, ver) {
			if idx == 0 {
				return nil, fmt.Errorf("versions is not in sequence: '%s' and '%s'", actual.Original(), ver.Original())
			}

			f.logger.Warn("not sequential version", slog.String("previous", actual.Original()), slog.String("next", ver.Original()))

			break
		}

		image, err := f.registryClient.Image(ctx, ver.Original())
		if err != nil {
			return nil, fmt.Errorf("get image: %w", err)
		}

		releaseMeta, err := f.fetchReleaseMetadata(ctx, image)
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
func (f *DeckhouseReleaseFetcher) getNewVersions(ctx context.Context, actual, target *semver.Version) ([]*semver.Version, error) {
	tags, err := f.registryClient.ListTags(ctx)
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

func (f *DeckhouseReleaseFetcher) parseAndFilterVersions(tags []string) []*semver.Version {
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

var globalModules = []string{"candi", "deckhouse-controller", "global"}

func (f *DeckhouseReleaseFetcher) generateChangelogForEnabledModules(releaseMetadata *ReleaseMetadata) map[string]interface{} {
	enabledModules := f.moduleManager.GetEnabledModuleNames()
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

func getLatestDeployedRelease(releases []*v1alpha1.DeckhouseRelease) (int, *v1alpha1.DeckhouseRelease) {
	sort.Sort(sort.Reverse(releaseUpdater.ByVersion[*v1alpha1.DeckhouseRelease](releases)))

	for idx, release := range releases {
		if release.GetPhase() == v1alpha1.DeckhouseReleasePhaseDeployed {
			return idx, release
		}
	}

	return -1, nil
}

type ReleaseMetadata struct {
	Version          string                  `json:"version"`
	Changelog        map[string]interface{}  `json:"-"`
	ModuleDefinition *moduletypes.Definition `json:"module,omitempty"`

	Canary       map[string]canarySettings `json:"canary"`
	Requirements map[string]string         `json:"requirements"`
	Disruptions  map[string][]string       `json:"disruptions"`
	Suspend      bool                      `json:"suspend"`
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
