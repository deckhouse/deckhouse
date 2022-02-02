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
	var applyAfter *time.Time
	if releaseChecker.IsCanaryRelease() {
		clusterUUID := input.Values.Get("global.discovery.clusterUUID").String()
		applyAfter = releaseChecker.CalculateReleaseDelay(clusterUUID)
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
			if release.Phase == v1alpha1.PhasePending {
				input.PatchCollector.Delete("deckhouse.io/v1alpha1", "DeckhouseRelease", "", release.Name, object_patch.InBackground())
			}

		case release.Version.Equal(newSemver):
			input.LogEntry.Debugf("Release with version %s already exists", release.Version)
			if releaseChecker.releaseMetadata.Suspend {
				p := map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]string{
							"release.deckhouse.io/suspended": "true",
						},
					},
				}
				input.PatchCollector.MergePatch(p, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", release.Name)
			}
			return nil

		default:
			break releaseLoop
		}
	}

	release := &v1alpha1.DeckhouseRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DeckhouseRelease",
			APIVersion: "deckhouse.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: releaseName,
		},
		Spec: v1alpha1.DeckhouseReleaseSpec{
			Version:      releaseChecker.releaseMetadata.Version,
			ApplyAfter:   applyAfter,
			Requirements: releaseChecker.releaseMetadata.Requirements,
		},
		Approved: false,
	}

	if releaseChecker.releaseMetadata.Suspend {
		release.ObjectMeta.Annotations = map[string]string{"release.deckhouse.io/suspended": "true"}
	}

	input.PatchCollector.Create(release, object_patch.IgnoreIfExists())

	return nil
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

	var tarReader io.Reader
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

		tarReader, err = untarLayer(rc)
		if err != nil {
			rc.Close()
			dcr.logger.Warnf("layer is invalid: %s", err)
			continue
		}
		rc.Close()
	}

	err = json.NewDecoder(tarReader).Decode(&meta)

	return meta, err
}

func untarLayer(rc io.Reader) (io.Reader, error) {
	result := bytes.NewBuffer(nil)
	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of archive
			return result, nil
		}
		if err != nil {
			return nil, err
		}
		if hdr.Name != "version.json" {
			continue
		}
		if _, err := io.Copy(result, tr); err != nil {
			return nil, err
		}

		return result, nil
	}
}

type releaseMetadata struct {
	Version      string                    `json:"version"`
	Canary       map[string]canarySettings `json:"canary"`
	Requirements map[string]string         `json:"requirements"`
	Suspend      bool                      `json:"suspend"`
}

type canarySettings struct {
	Enabled  bool     `json:"enabled"`
	Waves    uint     `json:"waves"`
	Interval Duration `json:"interval"` // in minutes
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
		return "", fmt.Errorf("version not found. Probably image is broken or layer is not exist")
	}

	dcr.releaseMetadata = releaseMeta

	return imageDigest.String(), nil
}

func (dcr *DeckhouseReleaseChecker) CalculateReleaseDelay(clusterUUID string) *time.Time {
	hash := murmur3.Sum64([]byte(clusterUUID + dcr.releaseMetadata.Version))
	wave := hash % uint64(dcr.releaseCanarySettings().Waves)

	if wave != 0 {
		delay := time.Duration(wave) * dcr.releaseCanarySettings().Interval.Duration
		applyAfter := time.Now().UTC().Add(delay)
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

// custom type for appropriate json marshalling / unmarshalling (like "15m")
type Duration struct {
	time.Duration
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		d.Duration = time.Duration(value)
		return nil
	case string:
		var err error
		d.Duration, err = time.ParseDuration(value)
		if err != nil {
			return err
		}
		return nil
	default:
		return errors.New("invalid duration")
	}
}
