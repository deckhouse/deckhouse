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
	"fmt"
	"io"
	"log/slog"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/iancoleman/strcase"
	"github.com/pkg/errors"
	"github.com/spaolacci/murmur3"
	"gopkg.in/yaml.v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/go_lib/libapi"
	"github.com/deckhouse/deckhouse/go_lib/updater"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	metricUpdatingFailedGroup = "d8_updating_failed"
)

func (r *deckhouseReleaseReconciler) checkDeckhouseReleaseLoop(ctx context.Context) {
	wait.UntilWithContext(ctx, func(ctx context.Context) {
		if r.updateSettings.Get().ReleaseChannel == "" {
			return
		}
		err := r.checkDeckhouseRelease(ctx)
		if err != nil {
			r.logger.Errorf("check Deckhouse release: %s", err)
		}
	}, 3*time.Minute)
}

func (r *deckhouseReleaseReconciler) checkDeckhouseRelease(ctx context.Context) error {
	if r.updateSettings.Get().ReleaseChannel == "" {
		r.logger.Debug("Release channel does not set.")
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
		drs, _ := utils.ParseDeckhouseRegistrySecret(registrySecret.Data)
		rconf := &utils.RegistryConfig{
			DockerConfig: drs.DockerConfig,
			Scheme:       drs.Scheme,
			CA:           drs.CA,
			UserAgent:    r.clusterUUID,
		}

		opts = utils.GenerateRegistryOptions(rconf, r.logger)

		imagesRegistry = drs.ImageRegistry
	}

	releaseChecker, err := NewDeckhouseReleaseChecker(opts, r.logger, r.dc, r.moduleManager, imagesRegistry, releaseChannelName)
	if err != nil {
		return errors.Wrap(err, "create DeckhouseReleaseChecker failed")
	}

	newImageHash, err := releaseChecker.FetchReleaseMetadata(r.releaseVersionImageHash)
	if err != nil {
		return err
	}

	// no new image found
	if newImageHash == "" {
		return nil
	}

	// run only if it's a canary release
	var (
		cooldownUntil, notificationShiftTime *metav1.Time
	)
	if releaseChecker.releaseMetadata.Cooldown != nil {
		cooldownUntil = releaseChecker.releaseMetadata.Cooldown
	}

	newSemver, err := semver.NewVersion(releaseChecker.releaseMetadata.Version)
	if err != nil {
		// TODO: maybe set something like v1.0.0-{meta.Version} for developing purpose
		return fmt.Errorf("parse semver: %w", err)
	}
	r.releaseVersionImageHash = newImageHash

	var releases v1alpha1.DeckhouseReleaseList
	err = r.client.List(ctx, &releases)
	if err != nil {
		return fmt.Errorf("get deckhouse releases: %w", err)
	}

	pointerReleases := make([]*v1alpha1.DeckhouseRelease, 0, len(releases.Items))
	for _, r := range releases.Items {
		pointerReleases = append(pointerReleases, &r)
	}
	sort.Sort(sort.Reverse(updater.ByVersion[*v1alpha1.DeckhouseRelease](pointerReleases)))
	r.metricStorage.Grouped().ExpireGroupMetrics(metricUpdatingFailedGroup)

	for _, release := range pointerReleases {
		switch {
		// GT
		case release.GetVersion().GreaterThan(newSemver):
			// cleanup versions which are older than current version in a specified channel and are in a Pending state
			if release.Status.Phase == v1alpha1.ModuleReleasePhasePending {
				err = r.client.Delete(ctx, release, client.PropagationPolicy(metav1.DeletePropagationBackground))
				if err != nil {
					return fmt.Errorf("delete old release: %w", err)
				}
			}

			// EQ
		case release.GetVersion().Equal(newSemver):
			r.logger.Debugf("Release with version %s already exists", release.GetVersion())
			switch release.Status.Phase {
			case v1alpha1.ModuleReleasePhasePending, "":
				if releaseChecker.releaseMetadata.Suspend {
					patch := client.RawPatch(types.MergePatchType, buildSuspendAnnotation(releaseChecker.releaseMetadata.Suspend))
					err := r.client.Patch(ctx, release, patch)
					if err != nil {
						return fmt.Errorf("patch release %v: %w", release.Name, err)
					}

					err = r.client.Status().Patch(ctx, release, patch)
					if err != nil {
						return fmt.Errorf("patch release %v status: %w", release.Name, err)
					}
				}

			case v1alpha1.ModuleReleasePhaseSuspended:
				if !releaseChecker.releaseMetadata.Suspend {
					patch := client.RawPatch(types.MergePatchType, buildSuspendAnnotation(releaseChecker.releaseMetadata.Suspend))
					err := r.client.Patch(ctx, release, patch)
					if err != nil {
						return fmt.Errorf("patch release %v: %w", release.Name, err)
					}

					err = r.client.Status().Patch(ctx, release, patch)
					if err != nil {
						return fmt.Errorf("patch release %v status: %w", release.Name, err)
					}
				}
			}

			return nil

		// LT
		default:
			// inherit cooldown from previous patch release
			// we need this to automatically set cooldown for next patch releases
			if cooldownUntil == nil &&
				release.GetCooldownUntil() != nil &&
				release.GetVersion().Major() == newSemver.Major() &&
				release.GetVersion().Minor() == newSemver.Minor() {
				cooldownUntil = &metav1.Time{Time: *release.GetCooldownUntil()}
			}

			if release.GetNotificationShift() &&
				release.GetApplyAfter() != nil &&
				release.GetVersion().Major() == newSemver.Major() &&
				release.GetVersion().Minor() == newSemver.Minor() {
				notificationShiftTime = &metav1.Time{Time: *release.GetApplyAfter()}
			}

			actual := release.GetVersion()
			for !actual.Equal(newSemver) {
				if actual, err = releaseChecker.StepByStepUpdate(ctx, actual, newSemver); err != nil {
					releaseChecker.logger.Error("step by step update failed", log.Err(err))
					labels := map[string]string{
						"version": release.GetVersion().Original(),
					}

					r.metricStorage.Grouped().GaugeSet(metricUpdatingFailedGroup, "d8_updating_is_failed", 1, labels)
					return err
				}

				err = r.createRelease(ctx, releaseChecker, cooldownUntil, notificationShiftTime)
				if err != nil {
					return fmt.Errorf("create release %s: %w", releaseChecker.releaseMetadata.Version, err)
				}

				if releaseChecker.releaseMetadata.Cooldown != nil {
					cooldownUntil = releaseChecker.releaseMetadata.Cooldown
				}
			}
		}
	}

	// if there are no releases in the cluster, we apply the latest release
	if len(pointerReleases) == 0 {
		err = r.createRelease(ctx, releaseChecker, cooldownUntil, notificationShiftTime)
		if err != nil {
			return fmt.Errorf("create release %s: %w", releaseChecker.releaseMetadata.Version, err)
		}
	}
	return nil
}

func (r *deckhouseReleaseReconciler) createRelease(ctx context.Context, releaseChecker *DeckhouseReleaseChecker,
	cooldownUntil, notificationShiftTime *metav1.Time,
) error {
	var applyAfter *metav1.Time

	ts := metav1.Time{Time: r.dc.GetClock().Now()}
	if releaseChecker.IsCanaryRelease() {
		// if cooldown is set, calculate canary delay from cooldown time, not current
		if cooldownUntil != nil && cooldownUntil.After(ts.Time) {
			ts = *cooldownUntil
		}
		applyAfter = releaseChecker.CalculateReleaseDelay(ts, r.clusterUUID)
	}

	// inherit applyAfter from notified release
	if notificationShiftTime != nil && notificationShiftTime.After(ts.Time) {
		applyAfter = notificationShiftTime
	}

	var disruptions []string
	if len(releaseChecker.releaseMetadata.Disruptions) > 0 {
		version, err := semver.NewVersion(releaseChecker.releaseMetadata.Version)
		if err != nil {
			return err
		}
		disruptionsVersion := fmt.Sprintf("%d.%d", version.Major(), version.Minor())
		disruptions = releaseChecker.releaseMetadata.Disruptions[disruptionsVersion]
	}

	enabledModulesChangelog := releaseChecker.generateChangelogForEnabledModules()

	release := &v1alpha1.DeckhouseRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DeckhouseRelease",
			APIVersion: "deckhouse.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: releaseChecker.releaseMetadata.Version,
			Annotations: map[string]string{
				"release.deckhouse.io/isUpdating": "false",
				"release.deckhouse.io/notified":   "false",
			},
		},
		Spec: v1alpha1.DeckhouseReleaseSpec{
			Version:       releaseChecker.releaseMetadata.Version,
			ApplyAfter:    applyAfter,
			Requirements:  releaseChecker.releaseMetadata.Requirements,
			Disruptions:   disruptions,
			Changelog:     enabledModulesChangelog,
			ChangelogLink: fmt.Sprintf("https://github.com/deckhouse/deckhouse/releases/tag/%s", releaseChecker.releaseMetadata.Version),
		},
		Approved: false,
	}

	if releaseChecker.releaseMetadata.Suspend {
		release.ObjectMeta.Annotations["release.deckhouse.io/suspended"] = "true"
	}
	if cooldownUntil != nil {
		release.ObjectMeta.Annotations["release.deckhouse.io/cooldown"] = cooldownUntil.UTC().Format(time.RFC3339)
	}
	if notificationShiftTime != nil {
		release.ObjectMeta.Annotations["release.deckhouse.io/notification-time-shift"] = "true"
	}

	return client.IgnoreAlreadyExists(r.client.Create(ctx, release))
}

func NewDeckhouseReleaseChecker(opts []cr.Option, logger *log.Logger, dc dependency.Container, moduleManager moduleManager, imagesRegistry, releaseChannel string) (*DeckhouseReleaseChecker, error) {
	// registry.deckhouse.io/deckhouse/ce/release-channel:$release-channel
	regCli, err := dc.GetRegistryClient(path.Join(imagesRegistry, "release-channel"), opts...)
	if err != nil {
		return nil, err
	}

	dcr := &DeckhouseReleaseChecker{
		registryClient: regCli,
		logger:         logger,
		moduleManager:  moduleManager,
		releaseChannel: releaseChannel,
	}

	return dcr, nil
}

type DeckhouseReleaseChecker struct {
	registryClient cr.Client
	logger         *log.Logger
	moduleManager  moduleManager

	releaseChannel  string
	releaseMetadata ReleaseMetadata
	tags            []*semver.Version
}

func (dcr *DeckhouseReleaseChecker) IsCanaryRelease() bool {
	settings := dcr.releaseCanarySettings()
	return settings.Enabled
}

func (dcr *DeckhouseReleaseChecker) releaseCanarySettings() canarySettings {
	return dcr.releaseMetadata.Canary[dcr.releaseChannel]
}

func (dcr *DeckhouseReleaseChecker) FetchReleaseMetadata(previousImageHash string) (string, error) {
	image, err := dcr.registryClient.Image(context.TODO(), dcr.releaseChannel)
	if err != nil {
		return "", err
	}

	imageDigest, err := image.Digest()
	if err != nil {
		return "", err
	}
	if previousImageHash == imageDigest.String() {
		// image has not been changed
		return "", nil
	}

	releaseMeta, err := dcr.fetchReleaseMetadata(image)
	if err != nil {
		return "", err
	}

	if releaseMeta.Version == "" {
		return "", fmt.Errorf("version not found. Probably image is broken or layer does not exist")
	}

	dcr.releaseMetadata = *releaseMeta

	return imageDigest.String(), nil
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

func (dcr *DeckhouseReleaseChecker) fetchReleaseMetadata(img v1.Image) (*ReleaseMetadata, error) {
	meta := new(ReleaseMetadata)

	rc, err := cr.Extract(img)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	rr := &releaseReader{
		versionReader:   bytes.NewBuffer(nil),
		changelogReader: bytes.NewBuffer(nil),
	}

	err = rr.untarMetadata(rc)
	if err != nil {
		return nil, err
	}

	if rr.versionReader.Len() > 0 {
		err = json.NewDecoder(rr.versionReader).Decode(&meta)
		if err != nil {
			return nil, err
		}
	}

	if rr.changelogReader.Len() > 0 {
		var changelog map[string]any

		err = yaml.NewDecoder(rr.changelogReader).Decode(&changelog)
		if err != nil {
			// if changelog build failed - warn about it but don't fail the release
			dcr.logger.Warnf("Unmarshal CHANGELOG yaml failed: %s", err)
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

func (dcr *DeckhouseReleaseChecker) fetchCooldown(image v1.Image) *metav1.Time {
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

func (dcr *DeckhouseReleaseChecker) CalculateReleaseDelay(ts metav1.Time, clusterUUID string) *metav1.Time {
	hash := murmur3.Sum64([]byte(clusterUUID + dcr.releaseMetadata.Version))
	wave := hash % uint64(dcr.releaseCanarySettings().Waves)

	if wave != 0 {
		delay := time.Duration(wave) * dcr.releaseCanarySettings().Interval.Duration
		applyAfter := metav1.NewTime(ts.Add(delay))
		return &applyAfter
	}

	return nil
}

func (dcr *DeckhouseReleaseChecker) StepByStepUpdate(ctx context.Context, actual, target *semver.Version) (*semver.Version, error) {
	nextVersion, err := dcr.nextVersion(ctx, actual, target)
	if err != nil {
		return nil, fmt.Errorf("get next version: %w", err)
	}
	image, err := dcr.registryClient.Image(ctx, nextVersion.Original())
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

	dcr.releaseMetadata = *releaseMeta

	return nextVersion, nil
}

// nextVersion returns the next version of the target version
// uf we have some versions:
// 1.67.0
// 1.67.1
// 1.67.2
// 1.68.0
// and actual = 1.67.0, target = 1.68.0
// result will be 1.67.2
// if actual = 1.67.2, target = 1.68.0
// result will be 1.68.0
// for LTS channel we return only target version
func (dcr *DeckhouseReleaseChecker) nextVersion(ctx context.Context, actual, target *semver.Version) (*semver.Version, error) {
	if strings.ToUpper(dcr.releaseChannel) == "LTS" {
		return target, nil
	}

	tags, err := dcr.listTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	var vs []*semver.Version
	for _, tag := range tags {
		if tag.Compare(actual) > 0 && tag.Compare(target) <= 0 {
			vs = append(vs, tag)
		}
	}
	sort.Sort(semver.Collection(vs))

	var patchVersion, nextVersion *semver.Version
	for _, tag := range vs {
		if tag.Major() == actual.Major() &&
			tag.Minor() == actual.Minor() &&
			tag.Patch() > actual.Patch() {
			patchVersion = tag
		}
		if tag.Major() > actual.Major() || tag.Minor() > actual.Minor() {
			if nextVersion == nil ||
				(tag.Major() == nextVersion.Major() &&
					tag.Minor() == nextVersion.Minor() &&
					tag.Patch() > nextVersion.Patch()) {
				nextVersion = tag
			}
		}
	}

	if patchVersion != nil {
		return patchVersion, nil
	}

	if nextVersion != nil {
		return nextVersion, nil
	}

	return nil, fmt.Errorf("no suitable versions found")
}

var globalModules = []string{"candi", "deckhouse-controller", "global"}

func (dcr *DeckhouseReleaseChecker) generateChangelogForEnabledModules() map[string]interface{} {
	enabledModules := dcr.moduleManager.GetEnabledModuleNames()
	enabledModulesChangelog := make(map[string]interface{})

	for _, enabledModule := range enabledModules {
		if v, ok := dcr.releaseMetadata.Changelog[enabledModule]; ok {
			enabledModulesChangelog[enabledModule] = v
		}
	}

	// enable global modules
	for _, globalModule := range globalModules {
		if v, ok := dcr.releaseMetadata.Changelog[globalModule]; ok {
			enabledModulesChangelog[globalModule] = v
		}
	}

	return enabledModulesChangelog
}

func (dcr *DeckhouseReleaseChecker) listTags(ctx context.Context) ([]*semver.Version, error) {
	if dcr.tags == nil {
		tags, err := dcr.registryClient.ListTags(ctx)
		if err != nil {
			return nil, fmt.Errorf("registry client list tags: %w", err)
		}
		for _, tag := range tags {
			version, err := semver.NewVersion(tag)
			if err != nil {
				dcr.logger.Warn("bad semver", slog.String("version", tag))
			}
			dcr.tags = append(dcr.tags, version)
		}
	}

	return dcr.tags, nil
}

type ReleaseMetadata struct {
	// TODO: semVer as module?
	Version      string                    `json:"version"`
	Canary       map[string]canarySettings `json:"canary"`
	Requirements map[string]string         `json:"requirements"`
	Disruptions  map[string][]string       `json:"disruptions"`
	Suspend      bool                      `json:"suspend"`

	Changelog map[string]interface{}

	Cooldown *metav1.Time `json:"-"`
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
				"release.deckhouse.io/suspended": annotationValue,
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
