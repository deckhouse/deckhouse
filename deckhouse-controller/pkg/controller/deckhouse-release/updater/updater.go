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

	metricstorage "github.com/flant/shell-operator/pkg/metric_storage"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	d8config "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/updater"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	IsUpdatingAnnotation = "release.deckhouse.io/isUpdating"
	NotifiedAnnotation   = "release.deckhouse.io/notified"
)

func NewDeckhouseUpdater(
	ctx context.Context,
	logger *log.Logger,
	client client.Client,
	dc dependency.Container,
	updateSettings *updater.Settings,
	releaseData updater.DeckhouseReleaseData,
	metricStorage *metricstorage.MetricStorage,
	podIsReady,
	clusterBootstrapping bool,
	imagesRegistry string,
	enabledModules []string,
) *updater.Updater[*v1alpha1.DeckhouseRelease] {
	return updater.NewUpdater[*v1alpha1.DeckhouseRelease](
		ctx,
		dc,
		logger,
		updateSettings,
		releaseData,
		podIsReady,
		clusterBootstrapping,
		NewKubeAPI(client, dc, imagesRegistry),
		newMetricsUpdater(metricStorage),
		newWebhookDataSource(logger),
		enabledModules,
	)
}

func newWebhookDataSource(logger *log.Logger) *webhookDataSource {
	return &webhookDataSource{logger: logger}
}

type webhookDataSource struct {
	logger *log.Logger
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

func (api *KubeAPI) UpdateReleaseStatus(ctx context.Context, release *v1alpha1.DeckhouseRelease, msg, phase string) error {
	// reset transition time only if phase changes
	if release.Status.Phase != phase {
		release.Status.TransitionTime = metav1.NewTime(api.dc.GetClock().Now().UTC())
	}
	release.Status.Phase = phase
	release.Status.Message = msg

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
				if r.Status.Phase != v1alpha1.ModuleReleasePhasePending {
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

func (api *KubeAPI) IsKubernetesVersionAutomatic(ctx context.Context) (bool, error) {
	key := client.ObjectKey{Namespace: d8config.APINamespaceName, Name: d8config.DeckhouseClusterConfigurationConfigMapName}
	secret := new(corev1.Secret)
	if err := api.client.Get(ctx, key, secret); err != nil {
		return false, fmt.Errorf("check kubernetes version: failed to get secret: %w", err)
	}

	var clusterConf struct {
		KubernetesVersion string `json:"kubernetesVersion"`
	}
	clusterConfigurationRaw, ok := secret.Data["cluster-configuration.yaml"]
	if !ok {
		return false, fmt.Errorf("check kubernetes version: expected field 'cluster-configuration.yaml' not found in secret %s", secret.Name)
	}
	if err := yaml.Unmarshal(clusterConfigurationRaw, &clusterConf); err != nil {
		return false, fmt.Errorf("check kubernetes version: failed to unmarshal cluster configuration: %w", err)
	}
	return clusterConf.KubernetesVersion == d8config.K8sAutomaticVersion, nil
}
