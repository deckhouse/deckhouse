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
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	"github.com/deckhouse/deckhouse/modules/020-deckhouse/hooks/internal/v1alpha1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/deckhouse/update_deckhouse_image",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "update_deckhouse_image",
			Crontab: "*/15 * * * * *",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "deckhouse_pod",
			ApiVersion: "v1",
			Kind:       "Pod",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "deckhouse",
				},
			},
			ExecuteHookOnEvents:          pointer.BoolPtr(false),
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			FilterFunc:                   filterDeckhousePod,
		},
		{
			Name:                         "releases",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "DeckhouseRelease",
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			FilterFunc:                   filterDeckhouseRelease,
		},
	},
}, dependency.WithExternalDependencies(updateDeckhouse))

type deckhousePodInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Image     string `json:"image"`
	ImageID   string `json:"imageID"`
	Ready     bool   `json:"ready"`
}

// isNextReleasePatch check SORTED array of DeckhouseReleases.
// If the next release after CURRENT Deployed release is a patch release - returns true
// else returns false
func isNextReleasePatch(releases []deckhouseReleaseUpdate) bool {
	var currentReleaseIndex = -1
	var currentRelease *semver.Version

	for i, r := range releases {
		if r.Phase == v1alpha1.PhaseDeployed {
			currentReleaseIndex = i
			currentRelease = r.Version
			continue
		}

		if currentRelease != nil && i == currentReleaseIndex+1 {
			// check next release
			if r.Version.Major() == currentRelease.Major() && r.Version.Minor() == currentRelease.Minor() {
				return true
			}

			return false
		}
	}

	return false
}

func updateDeckhouse(input *go_hook.HookInput, dc dependency.Container) error {
	if !input.Values.Exists("deckhouse.releaseChannel") {
		// dev upgrade - by tag
		return tagUpdate(input, dc)
	}

	// production upgrade
	approvalMode := input.Values.Get("deckhouse.update.mode").String()
	isManualMode := approvalMode == "Manual"

	releases := fetchAndPrepareReleases(input, isManualMode)

	isPatch := isNextReleasePatch(releases)

	// update windows works only for Auto deployment mode
	if !isManualMode {
		windows, exists := input.Values.GetOk("deckhouse.update.windows")
		if exists {
			if isPatch {
				// patch release does not respect update windows
				return releaseChannelUpdate(input, releases, isPatch)
			}

			updatePermitted, err := isUpdatePermitted([]byte(windows.Raw))
			if err != nil {
				return fmt.Errorf("update windows configuration is not valid: %s", err)
			}
			if !updatePermitted {
				input.LogEntry.Debug("Deckhouse update does not get into update windows. Skipping")
				return nil
			}
		}
	}

	return releaseChannelUpdate(input, releases, isPatch)
}

// used also in check_deckhouse_release.go
func filterDeckhouseRelease(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var release v1alpha1.DeckhouseRelease

	err := sdk.FromUnstructured(unstructured, &release)
	if err != nil {
		return nil, err
	}

	return deckhouseReleaseUpdate{
		Name:           release.Name,
		Version:        semver.MustParse(release.Spec.Version),
		Phase:          release.Status.Phase,
		ManualApproved: release.Approved,
		StatusApproved: release.Status.Approved,
	}, nil
}

type deckhouseReleaseUpdate struct {
	Name           string
	Version        *semver.Version
	Phase          string
	ManualApproved bool
	StatusApproved bool
}

type byVersion []deckhouseReleaseUpdate

func (a byVersion) Len() int {
	return len(a)
}
func (a byVersion) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
func (a byVersion) Less(i, j int) bool {
	return a[i].Version.LessThan(a[j].Version)
}

func filterDeckhousePod(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod corev1.Pod
	err := sdk.FromUnstructured(unstructured, &pod)
	if err != nil {
		return nil, err
	}

	var imageName, imageID string

	if len(pod.Spec.Containers) > 0 {
		imageName = pod.Spec.Containers[0].Image
	}

	var ready bool

	if len(pod.Status.ContainerStatuses) > 0 {
		imageID = pod.Status.ContainerStatuses[0].ImageID
		ready = pod.Status.ContainerStatuses[0].Ready
	}

	return deckhousePodInfo{
		Image:     imageName,
		ImageID:   imageID,
		Name:      pod.Name,
		Namespace: pod.Namespace,
		Ready:     ready,
	}, nil
}

// fetch releases from snapshots and sort them into ascending semver order
// also patch status for a new (Pending) releases
func fetchAndPrepareReleases(input *go_hook.HookInput, isManualMode bool) []deckhouseReleaseUpdate {
	snap := input.Snapshots["releases"]
	if len(snap) == 0 {
		return nil
	}
	now := time.Now()

	releases := make([]deckhouseReleaseUpdate, 0, len(snap))
	for _, rl := range snap {
		releases = append(releases, rl.(deckhouseReleaseUpdate))
	}

	sort.Sort(byVersion(releases))

	for i, rl := range releases {
		var statusChanged bool

		statusPatch := statusPatch{
			Phase:          rl.Phase,
			Approved:       rl.StatusApproved,
			TransitionTime: now,
		}

		if rl.Phase == "" {
			statusPatch.Phase = v1alpha1.PhasePending
			statusPatch.TransitionTime = now
			statusChanged = true
		}

		if statusPatch.Phase == v1alpha1.PhasePending {
			// check and set .status.approved for pending releases
			if isManualMode && !rl.ManualApproved {
				statusPatch.Approved = false
				if rl.StatusApproved {
					statusChanged = true
				}
			} else {
				statusPatch.Approved = true
				if !rl.StatusApproved {
					statusChanged = true
				}
			}
		}

		if statusChanged {
			input.PatchCollector.MergePatch(statusPatch, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", rl.Name, object_patch.WithSubresource("/status"))

			rl.StatusApproved = statusPatch.Approved
			rl.Phase = statusPatch.Phase
			releases[i] = rl
		}
	}

	return releases
}

// releaseChannelUpdate update with previously set release channel when CR DeckhouseRelease exists
func releaseChannelUpdate(input *go_hook.HookInput, releases []deckhouseReleaseUpdate, isPatch bool) error {
	input.MetricsCollector.Expire("d8_releases")

	snap := input.Snapshots["deckhouse_pod"]
	if len(snap) == 0 {
		return nil
	}

	// upgrade only when current release is ready.
	// skip it for patches.
	deckhousePod := snap[0].(deckhousePodInfo)
	if !isPatch && !deckhousePod.Ready {
		input.LogEntry.Info("Deckhouse is not ready. Skipping upgrade")
		return nil
	}

	now := time.Now()

	currentDeployedReleaseIndex := -1

	for i, rl := range releases {
		switch rl.Phase {
		// "Deployed" shows only Actual (current) release. All previous releases are marked as Outdated
		// It's much more comfortable to observe DeckhouseReleases like this because by default they are sorted by Name
		// and sometimes it's a bit weird for semver names. This statuses shows you the real view of releases
		case v1alpha1.PhaseOutdated:
			// pass

		case v1alpha1.PhasePending:
			if i < currentDeployedReleaseIndex {
				// some old release, for example - when downgrade the release channel
				// mark it as Outdated
				sp := statusPatch{
					Phase:          v1alpha1.PhaseOutdated,
					TransitionTime: now,
				}
				input.PatchCollector.MergePatch(sp, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", rl.Name, object_patch.WithSubresource("/status"))
				continue
			}

			if i != currentDeployedReleaseIndex+1 {
				continue
			}

			// always deploy Patch releases
			if !rl.StatusApproved && !isPatch {
				input.LogEntry.Infof("Release %s is waiting for manual approval", rl.Name)
				input.MetricsCollector.Set("d8_release_waiting_manual", 1, map[string]string{"name": rl.Name}, metrics.WithGroup("d8_releases"))
				return nil
			}

			// mark previous release as outdated
			currentDeployedRelease := releases[currentDeployedReleaseIndex]
			sp := statusPatch{
				Phase:          v1alpha1.PhaseOutdated,
				Approved:       currentDeployedRelease.StatusApproved,
				TransitionTime: now,
			}
			input.PatchCollector.MergePatch(sp, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", currentDeployedRelease.Name, object_patch.WithSubresource("/status"))

			// apply patch - update deployment
			applyRelease(input, rl, now)

			return nil

		case v1alpha1.PhaseDeployed:
			if i == len(releases)-1 {
				// last release, don't update
				return nil
			}
			currentDeployedReleaseIndex = i
		}
	}

	// self-healing, if deployed release was deleted
	if currentDeployedReleaseIndex == -1 {
		// no deployed releases found, deploy first pending release
		for _, rl := range releases {
			if rl.Phase == v1alpha1.PhasePending {
				applyRelease(input, rl, now)
				return nil
			}
		}
	}

	return nil
}

func applyRelease(input *go_hook.HookInput, rl deckhouseReleaseUpdate, ts time.Time) {
	input.LogEntry.Infof("Applying release %s", rl.Name)

	repo := input.Values.Get("global.modulesImages.registry").String()

	st := statusPatch{
		Phase:          v1alpha1.PhaseDeployed,
		Approved:       true,
		TransitionTime: ts,
	}
	input.PatchCollector.MergePatch(st, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", rl.Name, object_patch.WithSubresource("/status"))
	input.PatchCollector.Filter(func(u *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		var depl appsv1.Deployment
		err := sdk.FromUnstructured(u, &depl)
		if err != nil {
			return nil, err
		}

		depl.Spec.Template.Spec.Containers[0].Image = repo + ":" + rl.Version.Original()

		return sdk.ToUnstructured(&depl)
	}, "apps/v1", "Deployment", "d8-system", "deckhouse")
}

func isUpdatePermitted(windowsData []byte) (bool, error) {
	if len(windowsData) == 0 {
		return true, nil
	}

	now := time.Now()

	if os.Getenv("D8_IS_TESTS_ENVIRONMENT") != "" {
		now = time.Date(2021, 01, 01, 13, 30, 00, 00, time.UTC)
	}

	windows, err := update.FromJSON(windowsData)
	if err != nil {
		return false, err
	}

	return windows.IsAllowed(now), nil
}

// tagUpdate update by tag, in dev mode or specified image
func tagUpdate(input *go_hook.HookInput, dc dependency.Container) error {
	snap := input.Snapshots["deckhouse_pod"]
	if len(snap) == 0 {
		return nil
	}

	deckhousePod := snap[0].(deckhousePodInfo)
	if deckhousePod.Image == "" && deckhousePod.ImageID == "" {
		// pod is restarting or something like that, try more in a 15 seconds
		return nil
	}

	if deckhousePod.Image == "" || deckhousePod.ImageID == "" {
		input.LogEntry.Debug("Deckhouse pod is not ready. Try to update later")
		return nil
	}

	idSplitIndex := strings.LastIndex(deckhousePod.ImageID, "@")
	if idSplitIndex == -1 {
		return fmt.Errorf("image hash not found: %s", deckhousePod.ImageID)
	}
	imageHash := deckhousePod.ImageID[idSplitIndex+1:]

	imageSplitIndex := strings.LastIndex(deckhousePod.Image, ":")
	if imageSplitIndex == -1 {
		return fmt.Errorf("image tag not found: %s", deckhousePod.Image)
	}
	repo := deckhousePod.Image[:imageSplitIndex]
	tag := deckhousePod.Image[imageSplitIndex+1:]

	regClient, err := dc.GetRegistryClient(repo, GetCA(input), IsHTTP(input))
	if err != nil {
		input.LogEntry.Errorf("Registry (%s) client init failed: %s", repo, err)
		return nil
	}

	input.MetricsCollector.Inc("deckhouse_registry_check_total", map[string]string{})
	input.MetricsCollector.Inc("deckhouse_kube_image_digest_check_total", map[string]string{})

	repoDigest, err := regClient.Digest(tag)
	if err != nil {
		input.MetricsCollector.Inc("deckhouse_registry_check_errors_total", map[string]string{})
		input.LogEntry.Errorf("Registry (%s) get digest failed: %s", repo, err)
		return nil
	}

	input.MetricsCollector.Set("deckhouse_kube_image_digest_check_success", 1.0, map[string]string{})

	if strings.TrimSpace(repoDigest) == strings.TrimSpace(imageHash) {
		return nil
	}

	input.LogEntry.Info("New deckhouse image found. Restarting.")

	input.PatchCollector.Delete("v1", "Pod", deckhousePod.Namespace, deckhousePod.Name)

	return nil
}

type statusPatch v1alpha1.DeckhouseReleaseStatus

func (sp statusPatch) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"status": v1alpha1.DeckhouseReleaseStatus(sp),
	}

	return json.Marshal(m)
}
