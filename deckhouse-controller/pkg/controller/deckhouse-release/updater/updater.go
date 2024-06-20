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
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/flant/addon-operator/pkg/utils/logger"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/updater"
)

func NewDeckhouseUpdater(logger logger.Logger, client client.Client, dc dependency.Container, discoveryData updater.DeckhouseDiscoveryData, mode string, releaseData updater.DeckhouseReleaseData, podIsReady bool) (*updater.Updater[*v1alpha1.DeckhouseRelease], error) {
	return updater.NewUpdater[*v1alpha1.DeckhouseRelease](logger, discoveryData.NotificationConfig, mode, releaseData,
		podIsReady, discoveryData.ClusterBootstrapping, newKubeAPI(client, dc, discoveryData.ImagesRegistry),
		newMetricsUpdater(), newValueSettings(discoveryData.DisruptionApprovalMode), newWebhookDataGetter()), nil
}

func newWebhookDataGetter() *webhookDataGetter {
	return &webhookDataGetter{}
}

type webhookDataGetter struct {
}

func (w *webhookDataGetter) GetMessage(release *v1alpha1.DeckhouseRelease, releaseApplyTime time.Time) string {
	version := fmt.Sprintf("%d.%d", release.GetVersion().Major(), release.GetVersion().Minor())
	return fmt.Sprintf("New Deckhouse Release %s is available. Release will be applied at: %s", version, releaseApplyTime.Format(time.RFC850))
}

func newKubeAPI(client client.Client, dc dependency.Container, imagesRegistry string) *kubeAPI {
	return &kubeAPI{client: client, dc: dc, imagesRegistry: imagesRegistry}
}

type kubeAPI struct {
	client         client.Client
	dc             dependency.Container
	imagesRegistry string
}

func (api *kubeAPI) UpdateReleaseStatus(release *v1alpha1.DeckhouseRelease, msg, phase string) error {
	ctx := context.Background()
	release.Status.Phase = phase
	release.Status.Message = msg
	release.Status.TransitionTime = metav1.NewTime(api.dc.GetClock().Now().UTC())

	return api.client.Status().Update(ctx, release)
}

func (api *kubeAPI) PatchReleaseAnnotations(release *v1alpha1.DeckhouseRelease, annotations map[string]any) error {
	ctx := context.Background()
	patch, _ := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": annotations,
		},
	})

	p := client.RawPatch(types.MergePatchType, patch)
	return api.client.Patch(ctx, release, p)
}

func (api *kubeAPI) PatchReleaseApplyAfter(release *v1alpha1.DeckhouseRelease, applyTime time.Time) error {
	ctx := context.Background()
	patch, _ := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{
			"applyAfter": applyTime,
		},
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				"release.deckhouse.io/notification-time-shift": "true",
			},
		},
	})

	p := client.RawPatch(types.MergePatchType, patch)
	return api.client.Patch(ctx, release, p)
}

func (api *kubeAPI) DeployRelease(release *v1alpha1.DeckhouseRelease) error {
	ctx := context.Background()
	key := client.ObjectKey{Namespace: "d8-system", Name: "deckhouse"}
	var depl appsv1.Deployment
	err := api.client.Get(ctx, key, &depl)
	if err != nil {
		return fmt.Errorf("get deployment %s: %w", key, err)
	}

	// patch deckhouse deployment is faster than set internal values and then upgrade by helm
	// we can set "deckhouse.internal.currentReleaseImageName" value but lets left it this way
	depl.Spec.Template.Spec.Containers[0].Image = api.imagesRegistry + ":" + release.Spec.Version
	return api.client.Update(ctx, &depl)
}

func (api *kubeAPI) SaveReleaseData(release *v1alpha1.DeckhouseRelease, data updater.DeckhouseReleaseData) error {
	ctx := context.Background()
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

	err := api.client.Create(ctx, cm)
	if errors.IsAlreadyExists(err) {
		err = api.client.Update(ctx, cm)
	}
	if err != nil {
		return fmt.Errorf("update release data: %w", err)
	}

	return api.PatchReleaseAnnotations(release, map[string]interface{}{
		"release.deckhouse.io/isUpdating": strconv.FormatBool(data.IsUpdating),
		"release.deckhouse.io/notified":   strconv.FormatBool(data.Notified),
	})
}

func newValueSettings(disruptionApprovalMode string) *ValueSettings {
	return &ValueSettings{disruptionApprovalMode: disruptionApprovalMode}
}

type ValueSettings struct {
	disruptionApprovalMode string
}

func (v *ValueSettings) GetDisruptionApprovalMode() (string, bool) {
	return v.disruptionApprovalMode, true
}
