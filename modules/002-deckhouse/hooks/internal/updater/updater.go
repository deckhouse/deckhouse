/*
Copyright 2022 Flant JSC

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

package d8updater

import (
	"fmt"
	"strconv"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/updater"
)

func NewDeckhouseUpdater(input *go_hook.HookInput, mode string, data updater.DeckhouseReleaseData,
	podIsReady, isBootstrapping bool) (*updater.Updater[*DeckhouseRelease], error) {
	nConfig, err := updater.ParseNotificationConfigFromValues(input)
	if err != nil {
		return nil, fmt.Errorf("parsing notification config: %v", err)
	}

	return updater.NewUpdater[*DeckhouseRelease](input.LogEntry, nConfig, mode, data, podIsReady, isBootstrapping,
		newKubeAPI(input), newMetricsUpdater(input), newValueSettings(input), newWebhookDataGetter()), nil
}

func newWebhookDataGetter() *webhookDataGetter {
	return &webhookDataGetter{}
}

type webhookDataGetter struct {
}

func (w *webhookDataGetter) GetMessage(release *DeckhouseRelease, releaseApplyTime time.Time) string {
	version := fmt.Sprintf("%d.%d", release.GetVersion().Major(), release.GetVersion().Minor())
	return fmt.Sprintf("New Deckhouse Release %s is available. Release will be applied at: %s", version, releaseApplyTime.Format(time.RFC850))
}

func newKubeAPI(input *go_hook.HookInput) *kubeAPI {
	return &kubeAPI{input.PatchCollector, input.Values}
}

type kubeAPI struct {
	patchCollector *object_patch.PatchCollector
	values         *go_hook.PatchableValues
}

func (ru *kubeAPI) UpdateReleaseStatus(release *DeckhouseRelease, msg, phase string) error {
	st := StatusPatch{
		Phase:          phase,
		Message:        msg,
		Approved:       release.Status.Approved,
		TransitionTime: metav1.Now(), // TODO: UTC?
	}
	ru.patchCollector.MergePatch(st, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", release.Name, object_patch.WithSubresource("/status"))

	release.Status.Phase = phase
	release.Status.Message = msg
	return nil
}

func (ru *kubeAPI) PatchReleaseAnnotations(release *DeckhouseRelease, annotations map[string]any) error {
	annotationsPatch := map[string]any{
		"metadata": map[string]any{
			"annotations": annotations,
		},
	}

	ru.patchCollector.MergePatch(annotationsPatch, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", release.Name)
	return nil
}

func (ru *kubeAPI) PatchReleaseApplyAfter(release *DeckhouseRelease, applyTime time.Time) error {
	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"applyAfter": applyTime,
		},
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				"release.deckhouse.io/notification-time-shift": "true",
			},
		},
	}
	ru.patchCollector.MergePatch(patch, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", release.Name)
	return nil
}

func (ru *kubeAPI) DeployRelease(release *DeckhouseRelease) error {
	repo := ru.values.Get("global.modulesImages.registry.base").String()

	// patch deckhouse deployment is faster than set internal values and then upgrade by helm
	// we can set "deckhouse.internal.currentReleaseImageName" value but lets left it this way
	ru.patchCollector.Filter(func(u *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		var depl appsv1.Deployment
		err := sdk.FromUnstructured(u, &depl)
		if err != nil {
			return nil, err
		}

		depl.Spec.Template.Spec.Containers[0].Image = repo + ":" + release.Version.Original()

		return sdk.ToUnstructured(&depl)
	}, "apps/v1", "Deployment", "d8-system", "deckhouse")

	return nil
}

func (ru *kubeAPI) SaveReleaseData(_ *DeckhouseRelease, data updater.DeckhouseReleaseData) error {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "d8-release-data",
			Namespace: "d8-system",
			Labels: map[string]string{
				"heritage": "deckhouse",
			},
		},
		Data: map[string]string{
			// current release is updating
			"isUpdating": strconv.FormatBool(data.IsUpdating),
			// notification about next release is sent, flag will be reset when new release is deployed
			"notified": strconv.FormatBool(data.Notified),
		},
	}

	ru.patchCollector.Create(cm, object_patch.UpdateIfExists())
	return nil
}

func newValueSettings(input *go_hook.HookInput) *ValueSettings {
	return &ValueSettings{input.Values}
}

type ValueSettings struct {
	values *go_hook.PatchableValues
}

func (v *ValueSettings) GetDisruptionApprovalMode() (string, bool) {
	result, ok := v.values.GetOk("deckhouse.update.disruptionApprovalMode")
	if ok {
		return result.String(), ok
	}

	return "", false
}
