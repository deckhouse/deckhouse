/*
Copyright 2021 Flant JSC

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

package hooks

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/iancoleman/strcase"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spaolacci/murmur3"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/modules/020-deckhouse/hooks/internal/v1alpha1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/deckhouse/check_deckhouse_release",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "check_deckhouse_release",
			Crontab: "* * * * *", // every minute
		},
	},
	Settings: &go_hook.HookConfigSettings{
		EnableSchedulesOnStartup: true,
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "releases",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "DeckhouseRelease",
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			ExecuteHookOnEvents:          pointer.BoolPtr(false),
			FilterFunc:                   filterDeckhouseRelease,
		},
	},
}, dependency.WithExternalDependencies(checkReleases))

func checkReleases(input *go_hook.HookInput, dc dependency.Container) error {
	releaseChannelNameRaw, exists := input.Values.GetOk("deckhouse.releaseChannel")
	if !exists {
		input.LogEntry.Debug("Release channel does not set.")
		return nil
	}

	// move release channel to kebab-case because CI makes tags in kebab-case
	// Alpha -> alpha
	// EarlyAccess -> early-access
	// etc...
	releaseChannelName := strcase.ToKebab(releaseChannelNameRaw.String())

	releaseChecker, err := NewDeckhouseReleaseChecker(input, dc, releaseChannelName)
	if err != nil {
		return errors.Wrap(err, "create DeckhouseReleaseChecker failed")
	}

	var previousImageHash string
	previousHashRaw, exists := input.Values.GetOk("deckhouse.internal.releaseVersionImageHash")
	if exists {
		previousImageHash = previousHashRaw.String()
	}

	newImageHash, err := releaseChecker.FetchReleaseMetadata(previousImageHash)
	if err != nil {
		return err
	}

	// no new image found
	if newImageHash == "" {
		return nil
	}

	releaseName := strings.ReplaceAll(releaseChecker.releaseMetadata.Version, ".", "-")

	// run only if it's a canary release
	var applyAfter, cooldownUntil *time.Time
	if releaseChecker.releaseCooldown().Duration > 0 {
		cd := time.Now().UTC().Add(releaseChecker.releaseCooldown().Duration)
		cooldownUntil = &cd
	}

	if releaseChecker.IsCanaryRelease() {
		ts := time.Now()
		// if cooldown is set, calculate canary delay from cooldown time, not current
		if cooldownUntil != nil {
			ts = *cooldownUntil
		}
		clusterUUID := input.Values.Get("global.discovery.clusterUUID").String()
		applyAfter = releaseChecker.CalculateReleaseDelay(ts.UTC(), clusterUUID)
	}

	newSemver, err := semver.NewVersion(releaseChecker.releaseMetadata.Version)
	if err != nil {
		// TODO: maybe set something like v1.0.0-{meta.Version} for developing purpose
		return err
	}
	input.Values.Set("deckhouse.internal.releaseVersionImageHash", newImageHash)

	snap := input.Snapshots["releases"]
	releases := make([]deckhouseRelease, 0, len(snap))
	for _, rl := range snap {
		releases = append(releases, rl.(deckhouseRelease))
	}

	sort.Sort(sort.Reverse(byVersion(releases)))

releaseLoop:
	for _, release := range releases {
		switch {
		case release.Version.GreaterThan(newSemver):
			// cleanup versions which are older then current version in a specified channel and are in a Pending state
			if release.Status.Phase == v1alpha1.PhasePending {
				input.PatchCollector.Delete("deckhouse.io/v1alpha1", "DeckhouseRelease", "", release.Name, object_patch.InBackground())
			}

		case release.Version.Equal(newSemver):
			input.LogEntry.Debugf("Release with version %s already exists", release.Version)
			switch release.Status.Phase {
			case v1alpha1.PhasePending, "":
				if releaseChecker.releaseMetadata.Suspend {
					patch := buildSuspendAnnotation(releaseChecker.releaseMetadata.Suspend)
					input.PatchCollector.MergePatch(patch, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", release.Name)
				}

			case v1alpha1.PhaseSuspended:
				if !releaseChecker.releaseMetadata.Suspend {
					patch := buildSuspendAnnotation(releaseChecker.releaseMetadata.Suspend)
					input.PatchCollector.MergePatch(patch, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", release.Name)
				}
			}

			return nil

		default:
			break releaseLoop
		}
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

	enabledModulesChangelog := releaseChecker.generateChangelogForEnabledModules(input)

	release := &v1alpha1.DeckhouseRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DeckhouseRelease",
			APIVersion: "deckhouse.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        releaseName,
			Annotations: make(map[string]string),
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

	input.PatchCollector.Create(release, object_patch.IgnoreIfExists())

	return nil
}

var globalModules = []string{"candi", "deckhouse-controller", "global"}

func (dcr *DeckhouseReleaseChecker) generateChangelogForEnabledModules(input *go_hook.HookInput) map[string]interface{} {
	enabledModules := input.Values.Get("global.enabledModules").Array()
	enabledModulesChangelog := make(map[string]interface{})

	for _, enabledModule := range enabledModules {
		if v, ok := dcr.releaseMetadata.Changelog[enabledModule.String()]; ok {
			enabledModulesChangelog[enabledModule.String()] = v
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

	return meta, nil
}

type releaseMetadata struct {
	Version      string                       `json:"version"`
	Canary       map[string]canarySettings    `json:"canary"`
	Requirements map[string]string            `json:"requirements"`
	Disruptions  map[string][]string          `json:"disruptions"`
	Cooldown     map[string]v1alpha1.Duration `json:"cooldown"`
	Suspend      bool                         `json:"suspend"`

	Changelog map[string]interface{}
}

type canarySettings struct {
	Enabled  bool              `json:"enabled"`
	Waves    uint              `json:"waves"`
	Interval v1alpha1.Duration `json:"interval"` // in minutes
}

func getCA(input *go_hook.HookInput) string {
	return input.Values.Get("global.modulesImages.registryCA").String()
}

func isHTTP(input *go_hook.HookInput) bool {
	registryScheme := input.Values.Get("global.modulesImages.registryScheme").String()
	return registryScheme == "http"
}

type DeckhouseReleaseChecker struct {
	registryClient cr.Client
	logger         *logrus.Entry

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

func (dcr *DeckhouseReleaseChecker) releaseCooldown() v1alpha1.Duration {
	return dcr.releaseMetadata.Cooldown[dcr.releaseChannel]
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

func (dcr *DeckhouseReleaseChecker) CalculateReleaseDelay(ts time.Time, clusterUUID string) *time.Time {
	hash := murmur3.Sum64([]byte(clusterUUID + dcr.releaseMetadata.Version))
	wave := hash % uint64(dcr.releaseCanarySettings().Waves)

	if wave != 0 {
		delay := time.Duration(wave) * dcr.releaseCanarySettings().Interval.Duration
		applyAfter := ts.Add(delay)
		return &applyAfter
	}

	return nil
}

func NewDeckhouseReleaseChecker(input *go_hook.HookInput, dc dependency.Container, releaseChannel string) (*DeckhouseReleaseChecker, error) {
	repo := input.Values.Get("global.modulesImages.registry").String() // host/ns/repo

	// registry.deckhouse.io/deckhouse/ce/release-channel:$release-channel
	regCli, err := dc.GetRegistryClient(path.Join(repo, "release-channel"), cr.WithCA(getCA(input)), cr.WithInsecureSchema(isHTTP(input)))
	if err != nil {
		return nil, err
	}

	dcr := &DeckhouseReleaseChecker{
		registryClient: regCli,
		logger:         input.LogEntry,
		releaseChannel: releaseChannel,
	}

	return dcr, nil
}

func buildSuspendAnnotation(suspend bool) map[string]interface{} {
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

	return p
}
