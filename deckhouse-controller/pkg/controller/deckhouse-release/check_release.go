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
	"path"
	"regexp"
	"sort"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/utils/logger"
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
	d8utils "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/go_lib/libapi"
	"github.com/deckhouse/deckhouse/go_lib/updater"
)

func (r *deckhouseReleaseReconciler) checkDeckhouseReleaseLoop(ctx context.Context) {
	// Check if Module Manager has been initialized
	_ = wait.PollUntilContextCancel(ctx, d8utils.SyncedPollPeriod, false,
		func(context.Context) (bool, error) {
			return r.moduleManager.AreModulesInited(), nil
		})

	for {
		err := r.checkDeckhouseRelease(ctx)
		if err != nil {
			r.logger.Errorf("check Deckhouse release: %s", err)
		}

		time.Sleep(24 * time.Hour)
	}
}

func (r *deckhouseReleaseReconciler) checkDeckhouseRelease(ctx context.Context) error {
	discoveryData, err := r.getDeckhouseDiscoveryData(ctx)
	if err != nil {
		return fmt.Errorf("get release channel: %w", err)
	}

	releaseChannelName := discoveryData.ReleaseChannel
	if releaseChannelName == "" {
		r.logger.Debug("Release channel does not set.")
		return nil
	}

	// move release channel to kebab-case because CI makes tags in kebab-case
	// Alpha -> alpha
	// EarlyAccess -> early-access
	// etc...
	releaseChannelName = strcase.ToKebab(releaseChannelName)

	registrySecret, err := r.getRegistrySecret(ctx)
	if apierrors.IsNotFound(err) {
		err = nil
	}
	if err != nil {
		return fmt.Errorf("get registry secret: %w", err)
	}

	var opts []cr.Option
	if registrySecret != nil {
		opts = []cr.Option{
			cr.WithCA(string(registrySecret.Data["ca"])),
			cr.WithInsecureSchema(string(registrySecret.Data["scheme"]) == "http"),
			cr.WithUserAgent(discoveryData.ClusterUUID),
			cr.WithAuth(string(registrySecret.Data[".dockerconfigjson"])),
		}
	}

	releaseChecker, err := NewDeckhouseReleaseChecker(opts, r.logger, r.dc, r.moduleManager, discoveryData.ImagesRegistry, releaseChannelName)
	if err != nil {
		return errors.Wrap(err, "create DeckhouseReleaseChecker failed")
	}

	previousImageHash := r.releaseVersionImageHash.Get()

	newImageHash, err := releaseChecker.FetchReleaseMetadata(previousImageHash)
	if err != nil {
		return err
	}

	// no new image found
	if newImageHash == "" {
		return nil
	}

	// run only if it's a canary release
	var (
		applyAfter, cooldownUntil, notificationShiftTime *metav1.Time
	)
	if releaseChecker.releaseMetadata.Cooldown != nil {
		cooldownUntil = releaseChecker.releaseMetadata.Cooldown
	}

	newSemver, err := semver.NewVersion(releaseChecker.releaseMetadata.Version)
	if err != nil {
		// TODO: maybe set something like v1.0.0-{meta.Version} for developing purpose
		return fmt.Errorf("parse semver: %w", err)
	}
	r.releaseVersionImageHash.Set(newImageHash)

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

releaseLoop:
	for _, release := range pointerReleases {
		switch {
		// GT
		case release.GetVersion().GreaterThan(newSemver):
			// cleanup versions which are older than current version in a specified channel and are in a Pending state
			if release.Status.Phase == v1alpha1.PhasePending {
				r.client.Delete(ctx, release, client.PropagationPolicy(metav1.DeletePropagationBackground))
			}

			// EQ
		case release.GetVersion().Equal(newSemver):
			r.logger.Debugf("Release with version %s already exists", release.GetVersion())
			switch release.Status.Phase {
			case v1alpha1.PhasePending, "":
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

			case v1alpha1.PhaseSuspended:
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
			if cooldownUntil == nil && release.GetCooldownUntil() != nil {
				if release.GetVersion().Major() == newSemver.Major() && release.GetVersion().Minor() == newSemver.Minor() {
					cooldownUntil = &metav1.Time{Time: *release.GetCooldownUntil()}
				}
			}
			if release.GetNotificationShift() {
				if release.GetVersion().Major() == newSemver.Major() && release.GetVersion().Minor() == newSemver.Minor() {
					notificationShiftTime = &metav1.Time{Time: *release.GetApplyAfter()}
				}
			}
			if err := releaseChecker.StepByStepUpdate(release.GetVersion(), newSemver); err != nil {
				releaseChecker.logger.Errorf("step by step update failed. err: %v", err)
				return err
			}

			break releaseLoop
		}
	}

	ts := metav1.Time{Time: r.dc.GetClock().Now()}
	if releaseChecker.IsCanaryRelease() {
		// if cooldown is set, calculate canary delay from cooldown time, not current
		if cooldownUntil != nil && cooldownUntil.After(ts.Time) {
			ts = *cooldownUntil
		}
		applyAfter = releaseChecker.CalculateReleaseDelay(ts, discoveryData.ClusterUUID)
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

	err = client.IgnoreAlreadyExists(r.client.Create(ctx, release))
	if err != nil {
		return fmt.Errorf("crate release: %w", err)
	}

	return nil
}

func NewDeckhouseReleaseChecker(opts []cr.Option, logger logger.Logger, dc dependency.Container, moduleManager moduleManager, imagesRegistry, releaseChannel string) (*DeckhouseReleaseChecker, error) {
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
	logger         logger.Logger
	moduleManager  moduleManager

	releaseChannel  string
	releaseMetadata releaseMetadata
}

func (dcr *DeckhouseReleaseChecker) IsCanaryRelease() bool {
	settings := dcr.releaseCanarySettings()
	return settings.Enabled
}

func (dcr *DeckhouseReleaseChecker) releaseCanarySettings() canarySettings {
	return dcr.releaseMetadata.Canary[dcr.releaseChannel]
}

func (dcr *DeckhouseReleaseChecker) FetchReleaseMetadata(previousImageHash string) (digestHash string, err error) {
	image, err := dcr.registryClient.Image(dcr.releaseChannel)
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

	dcr.releaseMetadata = releaseMeta

	return imageDigest.String(), nil
}

type releaseReader struct {
	versionReader   *bytes.Buffer
	changelogReader *bytes.Buffer
}

func (rr *releaseReader) untarLayer(rc io.Reader) error {
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

func (dcr *DeckhouseReleaseChecker) fetchReleaseMetadata(image v1.Image) (releaseMetadata, error) {
	var meta releaseMetadata

	layers, err := image.Layers()
	if err != nil {
		return meta, err
	}

	if len(layers) == 0 {
		return meta, fmt.Errorf("no layers found")
	}

	rr := &releaseReader{
		versionReader:   bytes.NewBuffer(nil),
		changelogReader: bytes.NewBuffer(nil),
	}
	for _, layer := range layers {
		size, err := layer.Size()
		if err != nil {
			dcr.logger.Warnf("couldn't calculate layer size")
		}
		if size == 0 {
			// skip some empty werf layers
			continue
		}
		rc, err := layer.Uncompressed()
		if err != nil {
			return meta, err
		}

		err = rr.untarLayer(rc)
		if err != nil {
			rc.Close()
			dcr.logger.Warnf("layer is invalid: %s", err)
			continue
		}
		rc.Close()
	}

	if rr.versionReader.Len() > 0 {
		err = json.NewDecoder(rr.versionReader).Decode(&meta)
		if err != nil {
			return meta, err
		}
	}

	if rr.changelogReader.Len() > 0 {
		var changelog map[string]interface{}
		err = yaml.NewDecoder(rr.changelogReader).Decode(&changelog)
		if err != nil {
			// if changelog build failed - warn about it but don't fail the release
			dcr.logger.Warnf("Unmarshal CHANGELOG yaml failed: %s", err)
			meta.Changelog = make(map[string]interface{})
			return meta, nil
		}
		meta.Changelog = changelog
	}

	cooldown := dcr.fetchCooldown(image)
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

func (dcr *DeckhouseReleaseChecker) StepByStepUpdate(actual, target *semver.Version) error {
	nextVersion, err := dcr.nextVersion(actual, target)
	if err != nil {
		return err
	}
	if nextVersion == target {
		return nil
	}

	image, err := dcr.registryClient.Image(nextVersion.Original())
	if err != nil {
		return err
	}

	releaseMeta, err := dcr.fetchReleaseMetadata(image)
	if err != nil {
		return err
	}
	if releaseMeta.Version == "" {
		return fmt.Errorf("version not found. Probably image is broken or layer does not exist")
	}

	dcr.releaseMetadata = releaseMeta

	return nil
}

func (dcr *DeckhouseReleaseChecker) nextVersion(actual, target *semver.Version) (*semver.Version, error) {
	if actual.Major() != target.Major() {
		return nil, fmt.Errorf("major version updated") // TODO step by step update for major version
	}

	if actual.Minor() == target.Minor() || actual.IncMinor().Minor() == target.Minor() {
		return target, nil
	}

	listTags, err := dcr.registryClient.ListTags()
	if err != nil {
		return nil, err
	}

	// Here we get the following minor with the maximum patch version.
	// <major.minor+1.max>
	expr := fmt.Sprintf("^v1.%d.([0-9]+)$", actual.IncMinor().Minor())
	r, err := regexp.Compile(expr)
	if err != nil {
		return nil, err
	}

	collection := make([]*semver.Version, 0)
	for _, ver := range listTags {
		if r.MatchString(ver) {
			newSemver, err := semver.NewVersion(ver)
			if err != nil {
				dcr.logger.Errorf("unable to parse semver from the registry Version: %v. This version will be skipped.", ver)
				continue
			}
			collection = append(collection, newSemver)
		}
	}

	if len(collection) == 0 {
		return nil, fmt.Errorf("next minor version is missed")
	}

	sort.Sort(sort.Reverse(semver.Collection(collection)))

	return collection[0], nil
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

type releaseMetadata struct {
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
	AreModulesInited() bool
}
