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
	"github.com/flant/shell-operator/pkg/metric_storage"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/updater"
)

const (
	IsUpdatingAnnotation = "release.deckhouse.io/isUpdating"
	NotifiedAnnotation   = "release.deckhouse.io/notified"
)

func NewDeckhouseUpdater(logger logger.Logger, client client.Client, dc dependency.Container,
	updateSettings *updater.DeckhouseUpdateSettings, releaseData updater.DeckhouseReleaseData, metricStorage *metric_storage.MetricStorage,
	podIsReady, clusterBootstrapping bool, imagesRegistry string, enabledModules []string) (*updater.Updater[*v1alpha1.DeckhouseRelease], error) {
	return updater.NewUpdater[*v1alpha1.DeckhouseRelease](logger, updateSettings.NotificationConfig, updateSettings.Mode, releaseData,
		podIsReady, clusterBootstrapping, NewKubeAPI(client, dc, imagesRegistry),
		newMetricUpdater(metricStorage), newValueSettings(updateSettings.DisruptionApprovalMode), newWebhookDataSource(logger), enabledModules), nil
}

func newWebhookDataSource(logger logger.Logger) *webhookDataSource {
	return &webhookDataSource{logger: logger}
}

type webhookDataSource struct {
	logger logger.Logger
}

func (s *webhookDataSource) Fill(output *updater.WebhookData, _ *v1alpha1.DeckhouseRelease, applyTime time.Time) {
	if output == nil {
		s.logger.Error("webhook data must be defined")
		return
	}

	output.Subject = updater.SubjectDeckhouse
	output.Message = fmt.Sprintf("New Deckhouse Release %s is available. Release will be applied at: %s", output.Version, applyTime.Format(time.RFC850))
}

func NewKubeAPI(client client.Client, dc dependency.Container, imagesRegistry string) *KubeAPI {
	return &KubeAPI{client: client, dc: dc, imagesRegistry: imagesRegistry}
}

type KubeAPI struct {
	client         client.Client
	dc             dependency.Container
	imagesRegistry string
}

func (api *KubeAPI) UpdateReleaseStatus(release *v1alpha1.DeckhouseRelease, msg, phase string) error {
	ctx := context.Background()
	release.Status.Phase = phase
	release.Status.Message = msg
	release.Status.TransitionTime = metav1.NewTime(api.dc.GetClock().Now().UTC())

	return api.client.Status().Update(ctx, release)
}

func (api *KubeAPI) PatchReleaseAnnotations(ctx context.Context, release *v1alpha1.DeckhouseRelease, annotations map[string]any) error {
	patch, _ := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": annotations,
		},
	})

	p := client.RawPatch(types.MergePatchType, patch)
	return api.client.Patch(ctx, release, p)
}

func (api *KubeAPI) PatchReleaseApplyAfter(release *v1alpha1.DeckhouseRelease, applyTime time.Time) error {
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

func (api *KubeAPI) DeployRelease(ctx context.Context, release *v1alpha1.DeckhouseRelease) error {
	key := client.ObjectKey{Namespace: "d8-system", Name: "deckhouse"}
	var depl appsv1.Deployment
	err := api.client.Get(ctx, key, &depl)
	if err != nil {
		return fmt.Errorf("get deployment %s: %w", key, err)
	}

	// patch deckhouse deployment is faster than set internal values and then upgrade by helm
	// we can set "deckhouse.internal.currentReleaseImageName" value but lets left it this way
	depl.Spec.Template.Spec.Containers[0].Image = api.imagesRegistry + ":" + release.Spec.Version

	// dryrun
	if val, ok := release.GetAnnotations()["dryrun"]; ok && val == "true" {
		// TODO: write log about dry run
		go func() {
			time.Sleep(3 * time.Second)
			var releases v1alpha1.DeckhouseReleaseList
			err = api.client.List(ctx, &releases)
			if err != nil {
				return
			}

			for _, r := range releases.Items {
				if r.GetName() == release.GetName() {
					continue
				}
				if r.Status.Phase != v1alpha1.PhasePending {
					continue
				}
				// patch releases to trigger their requeue
				_ = api.PatchReleaseAnnotations(ctx, &r, map[string]any{"triggered_by_dryrun": release.GetName()})
			}
		}()
		return nil
	}

	return api.client.Update(ctx, &depl)
}

func (api *KubeAPI) SaveReleaseData(ctx context.Context, release *v1alpha1.DeckhouseRelease, data updater.DeckhouseReleaseData) error {
	return api.PatchReleaseAnnotations(ctx, release, map[string]interface{}{
		IsUpdatingAnnotation: strconv.FormatBool(data.IsUpdating),
		NotifiedAnnotation:   strconv.FormatBool(data.Notified),
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
