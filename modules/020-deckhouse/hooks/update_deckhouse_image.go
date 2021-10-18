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
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/tidwall/gjson"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
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
			Name:       "releases",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "DeckhouseRelease",
			FilterFunc: filterDeckhouseRelease,
		},
	},
}, dependency.WithExternalDependencies(updateDeckhouse))

type deckhousePodInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Image     string `json:"image"`
	ImageID   string `json:"imageID"`
}

// isNextReleasePatch check SORTED array of DeckhouseReleases.
// If the next release after CURRENT Deployed release is a patch release - returns true
// else returns false
func isNextReleasePatch(releases []deckhouseReleaseUpdate) bool {
	var currentReleaseIndex = -1
	var currentRelease *semver.Version

	for i, r := range releases {
		if r.Phase == "Deployed" {
			currentReleaseIndex = i
			var err error
			currentRelease, err = semver.NewVersion(r.Version)
			if err != nil {
				return false
			}
			continue
		}

		if currentRelease != nil && i == currentReleaseIndex+1 {
			// check next release
			nextRelease, err := semver.NewVersion(r.Version)
			if err != nil {
				return false
			}
			if nextRelease.Major() == currentRelease.Major() && nextRelease.Minor() == currentRelease.Minor() {
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
	releases := fetchAndPrepareReleases(input)

	windows, exists := input.Values.GetOk("deckhouse.update.windows")
	if exists {
		updatePermitted, err := isUpdatePermitted(windows.Array())
		if err != nil {
			return fmt.Errorf("update windows configuration is not valid: %s", err)
		}
		if !updatePermitted {
			if isNextReleasePatch(releases) {
				// patch upgrade does not respect update windows
				return releaseChannelUpdate(input, releases)
			}

			input.LogEntry.Debug("Deckhouse update does not get into update windows. Skipping")
			return nil
		}
	}

	return releaseChannelUpdate(input, releases)
}

// used also in check_deckhouse_release.go
func filterDeckhouseRelease(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var release v1alpha1.DeckhouseRelease

	err := sdk.FromUnstructured(unstructured, &release)
	if err != nil {
		return nil, err
	}

	return deckhouseReleaseUpdate{
		Name:    release.Name,
		Version: release.Spec.Version,
		Phase:   release.Status.Phase,
	}, nil
}

type deckhouseReleaseUpdate struct {
	Name    string
	Version string
	Phase   string
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

	if len(pod.Status.ContainerStatuses) > 0 {
		imageID = pod.Status.ContainerStatuses[0].ImageID
	}

	return deckhousePodInfo{
		Image:     imageName,
		ImageID:   imageID,
		Name:      pod.Name,
		Namespace: pod.Namespace,
	}, nil
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

// fetch releases from snapshots and sort them into ascending semver order
// also patch status for a new (Pending) releases
func fetchAndPrepareReleases(input *go_hook.HookInput) []deckhouseReleaseUpdate {
	snap := input.Snapshots["releases"]
	if len(snap) == 0 {
		return nil
	}
	now := time.Now()

	releases := make([]deckhouseReleaseUpdate, 0, len(snap))
	for _, rl := range snap {
		releases = append(releases, rl.(deckhouseReleaseUpdate))
	}

	sort.Slice(releases, func(i, j int) bool {
		v1r, err := semver.NewVersion(releases[i].Version)
		if err != nil {
			return false // could be in dev tags
		}
		v2r, err := semver.NewVersion(releases[j].Version)
		if err != nil {
			return false // could be in dev tags
		}

		return v1r.LessThan(v2r)
	})

	for i, rl := range releases {
		if rl.Phase == "" {
			patch := json.RawMessage(fmt.Sprintf(`{"status": {"phase": "Pending", "transitionTime": "%s"}}`, now.Format(time.RFC3339)))
			input.PatchCollector.MergePatch(patch, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", rl.Name, object_patch.WithSubresource("/status"))
			rl.Phase = "Pending"
			releases[i] = rl
		}
	}

	return releases
}

// releaseChannelUpdate update with previously set release channel when CR DeckhouseRelease exists
func releaseChannelUpdate(input *go_hook.HookInput, releases []deckhouseReleaseUpdate) error {
	repo := input.Values.Get("global.modulesImages.registry").String()
	now := time.Now()

	currentRelease := -1
	for i, rl := range releases {
		switch rl.Phase {
		// "Deployed" shows only Actual (current) release. All previous releases are marked as Outdated
		// It's much more comfortable to observe DeckhouseReleases like this because by default they are sorted by Name
		// and sometimes it's a bit weird for semver names. This statuses shows you the real view of releases
		case "Outdated":
			// pass

		case "Pending":
			if i == currentRelease+1 {
				patch := json.RawMessage(fmt.Sprintf(`{"status": {"phase": "Deployed", "transitionTime": "%s"}}`, now.Format(time.RFC3339)))
				input.PatchCollector.MergePatch(patch, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", rl.Name, object_patch.WithSubresource("/status"))
				input.PatchCollector.Filter(func(u *unstructured.Unstructured) (*unstructured.Unstructured, error) {
					var depl appsv1.Deployment
					err := sdk.FromUnstructured(u, &depl)
					if err != nil {
						return nil, err
					}

					depl.Spec.Template.Spec.Containers[0].Image = repo + ":" + rl.Version

					return sdk.ToUnstructured(&depl)
				}, "apps/v1", "Deployment", "d8-system", "deckhouse")
				return nil
			}

		case "Deployed":
			if i == len(releases)-1 {
				// last release, don't update
				return nil
			}
			currentRelease = i
			patch := json.RawMessage(fmt.Sprintf(`{"status": {"phase": "Outdated", "transitionTime": "%s"}}`, now.Format(time.RFC3339)))
			input.PatchCollector.MergePatch(patch, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", rl.Name, object_patch.WithSubresource("/status"))
		}
	}

	return nil
}

func isUpdatePermitted(windows []gjson.Result) (bool, error) {
	if len(windows) == 0 {
		return true, nil
	}

	var now time.Time

	if os.Getenv("D8_IS_TESTS_ENVIRONMENT") != "" {
		now = time.Date(2021, 01, 01, 13, 30, 00, 00, time.Local)
	} else {
		now = time.Now()
	}

	for _, window := range windows {
		var w updateWindow
		err := json.Unmarshal([]byte(window.Raw), &w)
		if err != nil {
			return false, err
		}
		if w.IsAllowed(now) {
			return true, nil
		}
	}

	return false, nil
}

type updateWindow struct {
	From string   `json:"from"`
	To   string   `json:"to"`
	Days []string `json:"days"`
}

// IsAllowed check if specified window is allowed at the moment or not
func (uw updateWindow) IsAllowed(now time.Time) bool {
	fromInput, _ := time.Parse("15:04", uw.From)
	toInput, _ := time.Parse("15:04", uw.To)

	fromTime := time.Date(now.Year(), now.Month(), now.Day(), fromInput.Hour(), fromInput.Minute(), 0, 0, now.Location())
	toTime := time.Date(now.Year(), now.Month(), now.Day(), toInput.Hour(), toInput.Minute(), 0, 0, now.Location())

	updateToday := uw.isTodayAllowed(now, uw.Days)

	if !updateToday {
		return false
	}

	if now.After(fromTime) && now.Before(toTime) {
		return true
	}

	return false
}

func (uw updateWindow) isDay(today time.Time, day string) bool {
	switch strings.ToLower(day) {
	case "mon":
		day = "Monday"

	case "tue":
		day = "Tuesday"

	case "wed":
		day = "Wednesday"

	case "thu":
		day = "Thursday"

	case "fri":
		day = "Friday"

	case "sat":
		day = "Saturday"

	case "sun":
		day = "Sunday"
	}

	return today.Weekday().String() == day
}

func (uw updateWindow) isTodayAllowed(now time.Time, days []string) bool {
	if len(days) == 0 {
		return true
	}

	for _, day := range days {
		if uw.isDay(now, day) {
			return true
		}
	}

	return false
}
