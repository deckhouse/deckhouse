/*
Copyright 2021 Flant CJSC

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
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/tidwall/gjson"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
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
			Name:       "deckhouse",
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
	},
}, dependency.WithExternalDependencies(updateDeckhouse))

type deckhousePodInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Image     string `json:"image"`
	ImageID   string `json:"imageID"`
}

func updateDeckhouse(input *go_hook.HookInput, dc dependency.Container) error {
	windows, exists := input.Values.GetOk("deckhouse.update.windows")
	if exists {
		updatePermitted, err := isUpdatePermitted(windows.Array())
		if err != nil {
			return fmt.Errorf("update windows configuration is not valid: %s", err)
		}
		if !updatePermitted {
			input.LogEntry.Debug("Deckhouse update does not get into update windows. Skipping")
			return nil
		}
	}

	snap := input.Snapshots["deckhouse"]

	deckhousePod := snap[0].(deckhousePodInfo)

	idSplit := strings.Split(deckhousePod.ImageID, "@")
	if len(idSplit) < 2 {
		return fmt.Errorf("image hash not found: %s", deckhousePod.ImageID)
	}
	imageHash := idSplit[1]

	imageSplit := strings.Split(deckhousePod.Image, ":")
	repo := imageSplit[0]
	tag := imageSplit[1]

	regClient, err := dc.GetRegistryClient(repo)
	if err != nil {
		input.LogEntry.Errorf("Registry client init failed: %s", err)
		return nil
	}

	input.MetricsCollector.Inc("deckhouse_registry_check_total", map[string]string{})
	input.MetricsCollector.Inc("deckhouse_kube_image_digest_check_total", map[string]string{})

	repoDigest, err := regClient.Digest(tag)
	if err != nil {
		input.MetricsCollector.Inc("deckhouse_registry_check_errors_total", map[string]string{})
		input.LogEntry.Errorf("Registry get digest failed: %s", err)
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

func filterDeckhousePod(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod corev1.Pod
	err := sdk.FromUnstructured(unstructured, &pod)
	if err != nil {
		return nil, err
	}

	return deckhousePodInfo{
		Image:     pod.Spec.Containers[0].Image,
		ImageID:   pod.Status.ContainerStatuses[0].ImageID,
		Name:      pod.Name,
		Namespace: pod.Namespace,
	}, nil
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
