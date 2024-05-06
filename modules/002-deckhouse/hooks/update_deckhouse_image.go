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
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	gcr "github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	d8http "github.com/deckhouse/deckhouse/go_lib/dependency/http"
	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	"github.com/deckhouse/deckhouse/go_lib/updater"
	d8updater "github.com/deckhouse/deckhouse/modules/002-deckhouse/hooks/internal/updater"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/deckhouse/update_deckhouse_image",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "update_deckhouse_image",
			Crontab: "*/15 * * * * *",
		},
	},
	Settings: &go_hook.HookConfigSettings{
		EnableSchedulesOnStartup: true,
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
			FieldSelector: &types.FieldSelector{
				MatchExpressions: []types.FieldSelectorRequirement{
					{
						Field:    "status.phase",
						Operator: "Equals",
						Value:    "Running",
					},
				},
			},
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   filterDeckhousePod,
		},
		{
			Name:                         "releases",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "DeckhouseRelease",
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   filterDeckhouseRelease,
		},
		{
			Name:       "release_data",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-release-data"},
			},
			ExecuteHookOnSynchronization: pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
			FilterFunc:                   filterReleaseDataCM,
		},
	},
}, dependency.WithExternalDependencies(updateDeckhouse))

type deckhousePodInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Image     string `json:"image"`
	ImageID   string `json:"imageID"`
}

const (
	metricReleasesGroup = "d8_releases"
	metricUpdatingGroup = "d8_updating"
)

func updateDeckhouse(input *go_hook.HookInput, dc dependency.Container) error {
	deckhousePods, err := getDeckhousePods(input.Snapshots["deckhouse_pod"])
	if err != nil {
		input.LogEntry.Warnf("Error getting deckhouse pods: %s", err)
		return nil
	}

	if len(deckhousePods) == 0 {
		input.LogEntry.Warn("Deckhouse pods not found. Skipping update")
		return nil
	}

	if !input.Values.Exists("deckhouse.releaseChannel") {
		// dev upgrade - by tag
		return tagUpdate(input, dc, deckhousePods)
	}

	// production upgrade
	input.MetricsCollector.Expire(metricReleasesGroup)

	var releaseData updater.DeckhouseReleaseData
	snap := input.Snapshots["release_data"]
	if len(snap) > 0 {
		releaseData = snap[0].(updater.DeckhouseReleaseData)
	}

	// initialize deckhouseUpdater
	approvalMode := input.Values.Get("deckhouse.update.mode").String()
	// if values key does not exist, then cluster is just bootstrapping
	clusterBootstrapping := true
	clusterBootstrappedV, ok := input.Values.GetOk("global.clusterIsBootstrapped")
	if ok {
		clusterBootstrapping = !clusterBootstrappedV.Bool()
	}

	podReady := isDeckhousePodReady(dc.GetHTTPClient())
	deckhouseUpdater, err := d8updater.NewDeckhouseUpdater(input, approvalMode, releaseData, podReady, clusterBootstrapping)

	if err != nil {
		return fmt.Errorf("initializing deckhouse updater: %v", err)
	}

	if podReady {
		input.MetricsCollector.Expire(metricUpdatingGroup)
		if releaseData.IsUpdating {
			_ = deckhouseUpdater.ChangeUpdatingFlag(false)
		}
	} else if releaseData.IsUpdating {
		labels := map[string]string{
			"releaseChannel": input.Values.Get("deckhouse.releaseChannel").String(),
		}
		input.MetricsCollector.Set("d8_is_updating", 1, labels, metrics.WithGroup(metricUpdatingGroup))
	}

	// fetch releases from snapshot and patch initial statuses
	releases := make([]*d8updater.DeckhouseRelease, 0, len(snap))
	for _, rl := range input.Snapshots["releases"] {
		releases = append(releases, rl.(*d8updater.DeckhouseRelease))
	}

	// fetch releases from snapshot and patch initial statuses
	deckhouseUpdater.PrepareReleases(releases)
	if deckhouseUpdater.ReleasesCount() == 0 {
		return nil
	}

	// predict next patch for Deploy
	deckhouseUpdater.PredictNextRelease()

	// has already Deployed the latest release
	if deckhouseUpdater.LastReleaseDeployed() {
		return nil
	}

	// some release is forced, burn everything, apply this patch!
	if deckhouseUpdater.HasForceRelease() {
		deckhouseUpdater.ApplyForcedRelease()
		return nil
	}

	if deckhouseUpdater.PredictedReleaseIsPatch() {
		// patch release does not respect update windows or ManualMode
		deckhouseUpdater.ApplyPredictedRelease(nil)
		return nil
	}

	var windows update.Windows
	if !deckhouseUpdater.InManualMode() {
		var err error
		windows, err = getUpdateWindows(input)
		if err != nil {
			return fmt.Errorf("update windows configuration is not valid: %s", err)
		}
	}

	deckhouseUpdater.ApplyPredictedRelease(windows)
	return nil
}

// getUpdateWindows return set update windows
func getUpdateWindows(input *go_hook.HookInput) (update.Windows, error) {
	windowsData, exists := input.Values.GetOk("deckhouse.update.windows")
	if !exists {
		return nil, nil
	}

	return update.FromJSON([]byte(windowsData.Raw))
}

// used also in check_deckhouse_release.go
func filterDeckhouseRelease(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var release v1alpha1.DeckhouseRelease

	err := sdk.FromUnstructured(unstructured, &release)
	if err != nil {
		return nil, err
	}

	var annotationFlags d8updater.DeckhouseReleaseAnnotationsFlags

	if v, ok := release.Annotations["release.deckhouse.io/suspended"]; ok {
		if v == "true" {
			annotationFlags.Suspend = true
		}
	}

	if v, ok := release.Annotations["release.deckhouse.io/force"]; ok {
		if v == "true" {
			annotationFlags.Force = true
		}
	}

	if v, ok := release.Annotations["release.deckhouse.io/apply-now"]; ok {
		if v == "true" {
			annotationFlags.ApplyNow = true
		}
	}

	if v, ok := release.Annotations["release.deckhouse.io/disruption-approved"]; ok {
		if v == "true" {
			annotationFlags.DisruptionApproved = true
		}
	}

	if v, ok := release.Annotations["release.deckhouse.io/notification-time-shift"]; ok {
		if v == "true" {
			annotationFlags.NotificationShift = true
		}
	}

	var releaseApproved bool
	if v, ok := release.Annotations["release.deckhouse.io/approved"]; ok {
		if v == "true" {
			releaseApproved = true
		}
	} else {
		releaseApproved = release.Approved
	}

	var cooldown *v1.Time
	if v, ok := release.Annotations["release.deckhouse.io/cooldown"]; ok {
		cd, err := time.Parse(time.RFC3339, v)
		if err == nil {
			cdv := v1.NewTime(cd)
			cooldown = &cdv
		}
	}

	return &d8updater.DeckhouseRelease{
		Name:          release.Name,
		Version:       semver.MustParse(release.Spec.Version),
		ApplyAfter:    release.Spec.ApplyAfter,
		CooldownUntil: cooldown,
		Requirements:  release.Spec.Requirements,
		ChangelogLink: release.Spec.ChangelogLink,
		Disruptions:   release.Spec.Disruptions,
		Status: v1alpha1.DeckhouseReleaseStatus{
			Phase:    release.Status.Phase,
			Approved: release.Status.Approved,
			Message:  release.Status.Message,
		},
		ManuallyApproved: releaseApproved,
		AnnotationFlags:  annotationFlags,
	}, nil
}

func filterReleaseDataCM(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm corev1.ConfigMap

	err := sdk.FromUnstructured(unstructured, &cm)
	if err != nil {
		return nil, err
	}

	var isUpdating, notified bool

	if v, ok := cm.Data["isUpdating"]; ok {
		if v == "true" {
			isUpdating = true
		}
	}

	if v, ok := cm.Data["notified"]; ok {
		if v == "true" {
			notified = true
		}
	}

	return updater.DeckhouseReleaseData{
		IsUpdating: isUpdating,
		Notified:   notified,
	}, nil
}

func filterDeckhousePod(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod corev1.Pod
	err := sdk.FromUnstructured(unstructured, &pod)
	if err != nil {
		return nil, err
	}

	// ignore evicted and shutdown pods
	if pod.Status.Phase == corev1.PodFailed {
		return nil, nil
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
func tagUpdate(input *go_hook.HookInput, dc dependency.Container, deckhousePods []deckhousePodInfo) error {
	for _, deckhousePod := range deckhousePods {
		if deckhousePod.Image == "" && deckhousePod.ImageID == "" {
			// pod is restarting or something like that, try more in 15 seconds
			return nil
		}

		if deckhousePod.Image == "" || deckhousePod.ImageID == "" {
			input.LogEntry.Debug("Deckhouse pod is not ready. Try to update later")
			return nil
		}
	}

	idSplitIndex := strings.LastIndex(deckhousePods[0].ImageID, "@")
	if idSplitIndex == -1 {
		return fmt.Errorf("image hash not found: %s", deckhousePods[0].ImageID)
	}
	imageHash := deckhousePods[0].ImageID[idSplitIndex+1:]

	imageRepoTag, err := gcr.NewTag(deckhousePods[0].Image)
	if err != nil {
		return fmt.Errorf("incorrect image: %s", deckhousePods[0].Image)
	}
	repo := imageRepoTag.Context().Name()
	tag := imageRepoTag.TagStr()

	dockerCfg := input.Values.Get("global.modulesImages.registry.dockercfg").String()

	opts := []cr.Option{
		cr.WithCA(getCA(input)),
		cr.WithInsecureSchema(isHTTP(input)),
		cr.WithAuth(dockerCfg),
	}

	regClient, err := dc.GetRegistryClient(repo, opts...)
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

	input.LogEntry.Info("New deckhouse image found. Restarting")

	now := time.Now().Format(time.RFC3339)
	if os.Getenv("D8_IS_TESTS_ENVIRONMENT") != "" {
		now = time.Date(2021, 01, 01, 13, 30, 00, 00, time.UTC).Format(time.RFC3339)
	}

	annotationsPatch := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]string{
						"kubectl.kubernetes.io/restartedAt": now,
					},
				},
			},
		},
	}

	input.PatchCollector.MergePatch(annotationsPatch, "apps/v1", "Deployment", deckhousePods[0].Namespace, "deckhouse")

	return nil
}

func getDeckhousePods(snap []go_hook.FilterResult) ([]deckhousePodInfo, error) {
	if len(snap) == 0 {
		return nil, nil
	}

	var image, imageID string
	deckhousePods := make([]deckhousePodInfo, 0, len(snap))

	for _, sn := range snap {
		if sn == nil {
			continue
		}
		deckhousePod := sn.(deckhousePodInfo)
		deckhousePods = append(deckhousePods, deckhousePod)
		// init image and imageID for comparison images/imageIDs across all pods if there are more than one pod in the snapshot
		if len(snap) > 1 {
			if len(image)+len(imageID) == 0 && len(deckhousePod.Image) != 0 && len(deckhousePod.ImageID) != 0 {
				image, imageID = deckhousePod.Image, deckhousePod.ImageID
				continue
			}

			if image != deckhousePod.Image || imageID != deckhousePod.ImageID {
				return nil, fmt.Errorf("deckhouse pods run different images")
			}
		}
	}

	return deckhousePods, nil
}

func isDeckhousePodReady(httpClient d8http.Client) bool {
	deckhousePodIP := os.Getenv("ADDON_OPERATOR_LISTEN_ADDRESS")

	url := fmt.Sprintf("http://%s:9650/readyz", deckhousePodIP)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		klog.Errorf("error getting deckhouse pod readyz status: %s", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return false
	}

	return true
}
