/*
Copyright 2023 Flant JSC

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

package deckhouse_release

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/utils/logger"
	gcr "github.com/google/go-containerregistry/pkg/name"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	d8updater "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/deckhouse-release/updater"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	"github.com/deckhouse/deckhouse/go_lib/updater"
)

const defaultCheckInterval = 15 * time.Second

type deckhouseReleaseReconciler struct {
	client       client.Client
	dc           dependency.Container
	logger       logger.Logger
	updatePolicy *v1alpha1.ModuleUpdatePolicySpecContainer
}

func NewDeckhouseReleaseController(
	mgr manager.Manager,
	dc dependency.Container,
	updatePolicy *v1alpha1.ModuleUpdatePolicySpecContainer,
) error {
	lg := log.WithField("component", "DeckhouseRelease")

	r := &deckhouseReleaseReconciler{
		mgr.GetClient(),
		dc,
		lg,
		updatePolicy,
	}

	ctr, err := controller.New("module-documentation", mgr, controller.Options{
		MaxConcurrentReconciles: 1,
		CacheSyncTimeout:        15 * time.Minute,
		NeedLeaderElection:      pointer.Bool(false),
		Reconciler:              r,
	})
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.DeckhouseRelease{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(ctr)
}

func (r *deckhouseReleaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var release v1alpha1.DeckhouseRelease
	err := r.client.Get(ctx, req.NamespacedName, &release)
	if err != nil {
		// The ModuleSource resource may no longer exist, in which case we stop
		// processing.
		if apierrors.IsNotFound(err) {
			// if source is not exists anymore - drop the checksum cache
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	if !release.DeletionTimestamp.IsZero() {
		// TODO: probably we have to delete documentation but we don't have such http handler atm
		return ctrl.Result{}, nil
	}

	return r.createOrUpdateReconcile(ctx)
}

func (r *deckhouseReleaseReconciler) createOrUpdateReconcile(ctx context.Context) (ctrl.Result, error) {
	deckhousePods, err := r.getDeckhousePods(ctx)
	if err != nil {
		r.logger.Warnf("Error getting deckhouse pods: %s", err)
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	if deckhousePods == nil || len(deckhousePods) == 0 {
		r.logger.Warn("Deckhouse pods not found. Skipping update")
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	discoveryData, err := r.getDeckhouseDiscoveryData(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("get release channel: %w", err)
	}

	if len(discoveryData.ReleaseChannel) == 0 {
		return r.tagUpdate(ctx, deckhousePods)
	}

	releaseData, err := r.getReleaseData(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("get release data: %w", err)
	}

	podReady := r.isDeckhousePodReady()
	deckhouseUpdater, err := d8updater.NewDeckhouseUpdater(r.logger, r.client, discoveryData, r.updatePolicy.Get().Update.Mode, releaseData, podReady)

	if err != nil {
		return ctrl.Result{}, fmt.Errorf("initializing deckhouse updater: %w", err)
	}

	if podReady {
		if releaseData.IsUpdating {
			_ = deckhouseUpdater.ChangeUpdatingFlag(false)
		}
	}

	var releases v1alpha1.DeckhouseReleaseList
	err = r.client.List(ctx, &releases)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("get deckhouse releases: %w", err)
	}

	pointerReleases := make([]*v1alpha1.DeckhouseRelease, 0, len(releases.Items))
	for _, r := range releases.Items {
		pointerReleases = append(pointerReleases, &r)
	}
	deckhouseUpdater.PrepareReleases(pointerReleases)
	if deckhouseUpdater.ReleasesCount() == 0 {
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	// predict next patch for Deploy
	deckhouseUpdater.PredictNextRelease()

	// has already Deployed the latest release
	if deckhouseUpdater.LastReleaseDeployed() {
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	// some release is forced, burn everything, apply this patch!
	if deckhouseUpdater.HasForceRelease() {
		deckhouseUpdater.ApplyForcedRelease()
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	if deckhouseUpdater.PredictedReleaseIsPatch() {
		// patch release does not respect update windows or ManualMode
		deckhouseUpdater.ApplyPredictedRelease(nil)
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	var windows update.Windows
	if !deckhouseUpdater.InManualMode() {
		windows = r.updatePolicy.Get().Update.Windows
	}

	deckhouseUpdater.ApplyPredictedRelease(windows)
	return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
}

func (r *deckhouseReleaseReconciler) getDeckhousePods(ctx context.Context) ([]corev1.Pod, error) {
	var pods corev1.PodList
	err := r.client.List(
		ctx,
		&pods,
		client.InNamespace("d8-system"),
		client.MatchingLabels{"app": "deckhouse"},
	)

	if err != nil {
		return nil, fmt.Errorf("list deckhouse pods: %w", err)
	}

	filtered := make([]corev1.Pod, 0)
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodFailed {
			continue
		}

		filtered = append(filtered, pod)
	}

	var image, imageID string

	for _, pod := range filtered {
		// init image and imageID for comparison images/imageIDs across all pods if there are more than one pod in the snapshot
		if len(image)+len(imageID) == 0 &&
			len(pod.Spec.Containers[0].Image) != 0 &&
			len(pod.Status.ContainerStatuses[0].ImageID) != 0 {
			image, imageID = pod.Spec.Containers[0].Image, pod.Status.ContainerStatuses[0].ImageID
			continue
		}

		if image != pod.Spec.Containers[0].Image || imageID != pod.Status.ContainerStatuses[0].ImageID {
			return nil, fmt.Errorf("deckhouse pods run different images")
		}
	}

	return filtered, nil
}

func (r *deckhouseReleaseReconciler) getDeckhouseDiscoveryData(ctx context.Context) (updater.DeckhouseDiscoveryData, error) {
	var secret corev1.Secret
	key := types.NamespacedName{Namespace: "d8-system", Name: "deckhouse-discovery"}
	err := r.client.Get(ctx, key, &secret)
	if err != nil {
		return updater.DeckhouseDiscoveryData{}, fmt.Errorf("get secret %s: %w", key, err)
	}

	var data = updater.DeckhouseDiscoveryData{
		ReleaseChannel:         string(secret.Data["releaseChannel"]),
		ClusterBootstrapping:   true,
		DisruptionApprovalMode: "Auto",
		NotificationConfig:     new(updater.NotificationConfig),
	}

	//default case in template
	if data.ReleaseChannel == "Unknown" {
		data.ReleaseChannel = ""
	}

	clusterBootstrapped, ok := secret.Data["clusterIsBootstrapped"]
	if ok {
		data.ClusterBootstrapping = string(clusterBootstrapped) != "true"
	}

	imagesRegistry, ok := secret.Data["imagesRegistry"]
	if ok {
		data.ImagesRegistry = string(imagesRegistry)
	}

	if jsonSettings, ok := secret.Data["updateSettings.json"]; ok {
		var settings struct {
			NotificationConfig     *updater.NotificationConfig `json:"notification"`
			DisruptionApprovalMode *string                     `json:"disruptionApprovalMode"`
		}

		err = json.Unmarshal(jsonSettings, &settings)
		if err != nil {
			return data, fmt.Errorf("unmarshal json: %w", err)
		}

		if settings.NotificationConfig != nil {
			data.NotificationConfig = settings.NotificationConfig
		}

		if settings.DisruptionApprovalMode != nil {
			data.DisruptionApprovalMode = *settings.DisruptionApprovalMode
		}
	}

	return data, nil
}

func (r *deckhouseReleaseReconciler) tagUpdate(ctx context.Context, pods []corev1.Pod) (ctrl.Result, error) {
	for _, pod := range pods {
		if pod.Spec.Containers[0].Image == "" && pod.Status.ContainerStatuses[0].ImageID == "" {
			// pod is restarting or something like that, try more in 15 seconds
			return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
		}

		if pod.Spec.Containers[0].Image == "" || pod.Status.ContainerStatuses[0].ImageID == "" {
			r.logger.Debug("Deckhouse pod is not ready. Try to update later")
			return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
		}
	}

	imageID := pods[0].Status.ContainerStatuses[0].ImageID
	idSplitIndex := strings.LastIndex(imageID, "@")
	if idSplitIndex == -1 {
		return ctrl.Result{}, fmt.Errorf("image hash not found: %s", imageID)
	}
	imageHash := imageID[idSplitIndex+1:]

	image := pods[0].Spec.Containers[0].Image
	imageRepoTag, err := gcr.NewTag(image)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("incorrect image: %s", image)
	}

	repo := imageRepoTag.Context().Name()
	tag := imageRepoTag.TagStr()

	registrySecret, err := r.getRegistrySecret(ctx)
	if apierrors.IsNotFound(err) {
		err = nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	var opts []cr.Option
	if registrySecret != nil {
		opts = []cr.Option{
			cr.WithCA(string(registrySecret.Data["ca"])),
			cr.WithInsecureSchema(string(registrySecret.Data["scheme"]) == "http"),
			cr.WithAuth(string(registrySecret.Data[".dockerconfigjson"])),
		}
	}

	regClient, err := r.dc.GetRegistryClient(repo, opts...)
	if err != nil {
		r.logger.Errorf("Registry (%s) client init failed: %s", repo, err)
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	repoDigest, err := regClient.Digest(tag)
	if err != nil {
		r.logger.Errorf("Registry (%s) get digest failed: %s", repo, err)
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	if strings.TrimSpace(repoDigest) == strings.TrimSpace(imageHash) {
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	r.logger.Info("New deckhouse image found. Restarting")
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

	jsonPatch, _ := json.Marshal(annotationsPatch)
	err = r.client.Patch(
		ctx,
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: pods[0].Namespace,
				Name:      "deckhouse",
			},
		},
		client.RawPatch(types.MergePatchType, jsonPatch),
	)

	if err != nil {
		r.logger.Errorf("Patch deckhouse deploymaent failed: %s", err)
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil

}

func (r *deckhouseReleaseReconciler) getRegistrySecret(ctx context.Context) (*corev1.Secret, error) {
	var secret corev1.Secret
	key := types.NamespacedName{Namespace: "d8-system", Name: "deckhouse-registry"}
	err := r.client.Get(ctx, key, &secret)
	if err != nil {
		return nil, fmt.Errorf("get secret %s: %w", key, err)
	}

	return &secret, nil
}

// TODO: get from release object
func (r *deckhouseReleaseReconciler) getReleaseData(ctx context.Context) (updater.DeckhouseReleaseData, error) {
	var cm corev1.ConfigMap

	key := types.NamespacedName{Namespace: "d8-system", Name: "d8-release-data"}
	err := r.client.Get(ctx, key, &cm)
	if apierrors.IsNotFound(err) {
		return updater.DeckhouseReleaseData{}, nil
	}
	if err != nil {
		return updater.DeckhouseReleaseData{}, fmt.Errorf("get config map %s: %w", key, err)
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

func (r *deckhouseReleaseReconciler) isDeckhousePodReady() bool {
	deckhousePodIP := os.Getenv("ADDON_OPERATOR_LISTEN_ADDRESS")

	url := fmt.Sprintf("http://%s:9650/readyz", deckhousePodIP)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		r.logger.Errorf("error getting deckhouse pod readyz status: %s", err)
	}

	resp, err := r.dc.GetHTTPClient().Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return false
	}

	return true
}
