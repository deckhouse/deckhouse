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

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/iancoleman/strcase"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
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
	repo := input.Values.Get("global.modulesImages.registry").String() // host/ns/repo
	var previousImageHash string
	previousHashRaw, exists := input.Values.GetOk("deckhouse.internal.releaseVersionImageHash")
	if exists {
		previousImageHash = previousHashRaw.String()
	}

	// registry.deckhouse.io/deckhouse/ce/release-channel:$release-channel
	regCli, err := dc.GetRegistryClient(path.Join(repo, "release-channel"), GetCA(input), IsHTTP(input))
	if err != nil {
		return err
	}

	image, err := regCli.Image(releaseChannelName)
	if err != nil {
		return err
	}

	var digestExists bool
	digest, err := image.Digest()
	if err == nil {
		digestExists = true
		if previousImageHash == digest.String() {
			// image has not been changed
			return nil
		}
	}

	meta, err := fetchReleaseMetadata(input, image)
	if err != nil {
		return err
	}

	if meta.Version == "" {
		return fmt.Errorf("version not found. Probably image is broken or layer is not exist")
	}

	newSemver, err := semver.NewVersion(meta.Version)
	if err != nil {
		// TODO: maybe set something like v1.0.0-{meta.Version} for developing purpose
		return err
	}

	snap := input.Snapshots["releases"]
	releases := make([]deckhouseReleaseUpdate, 0, len(snap))
	for _, rl := range snap {
		releases = append(releases, rl.(deckhouseReleaseUpdate))
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
			return nil

		default:
			break releaseLoop
		}
	}

	releaseName := strings.ReplaceAll(meta.Version, ".", "-")

	release := &v1alpha1.DeckhouseRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DeckhouseRelease",
			APIVersion: "deckhouse.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: releaseName,
		},
		Spec: v1alpha1.DeckhouseReleaseSpec{
			Version: meta.Version,
		},
		Approved: false,
	}

	input.PatchCollector.Create(release, object_patch.IgnoreIfExists())
	if digestExists {
		input.Values.Set("deckhouse.internal.releaseVersionImageHash", digest.String())
	}

	return nil
}

func fetchReleaseMetadata(input *go_hook.HookInput, image v1.Image) (releaseMetadata, error) {
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
			input.LogEntry.Warnf("couldn't calculate layer size")
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
			input.LogEntry.Warnf("layer is invalid: %s", err)
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
	Version     string `json:"version"`
	ReleaseDate string `json:"release_date"`
}

func GetCA(input *go_hook.HookInput) string {
	return input.Values.Get("global.modulesImages.registryCA").String()
}

func IsHTTP(input *go_hook.HookInput) bool {
	registryScheme := input.Values.Get("global.modulesImages.registryScheme").String()
	return registryScheme == "http"
}
