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
	"github.com/flant/shell-operator/pkg/metric_storage"
	"github.com/gofrs/uuid/v5"
	gcr "github.com/google/go-containerregistry/pkg/name"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	d8updater "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/deckhouse-release/updater"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	"github.com/deckhouse/deckhouse/go_lib/updater"
)

const (
	metricReleasesGroup = "d8_releases"
	metricUpdatingGroup = "d8_updating"
)

const defaultCheckInterval = 15 * time.Second

type deckhouseReleaseReconciler struct {
	client        client.Client
	dc            dependency.Container
	logger        logger.Logger
	moduleManager moduleManager

	updateSettings          *helpers.DeckhouseSettingsContainer
	metricStorage           *metric_storage.MetricStorage
	clusterUUID             string
	releaseVersionImageHash string
}

func NewDeckhouseReleaseController(ctx context.Context, mgr manager.Manager, dc dependency.Container,
	moduleManager moduleManager, updateSettings *helpers.DeckhouseSettingsContainer, metricStorage *metric_storage.MetricStorage,
) error {
	lg := log.WithField("component", "DeckhouseRelease")

	r := &deckhouseReleaseReconciler{
		client:         mgr.GetClient(),
		dc:             dc,
		logger:         lg,
		moduleManager:  moduleManager,
		updateSettings: updateSettings,
		metricStorage:  metricStorage,
	}

	// wait for cache sync
	go func() {
		if ok := mgr.GetCache().WaitForCacheSync(ctx); !ok {
			r.logger.Fatalf("Sync cache failed")
		}
		go r.updateByImageHashLoop(ctx)
		go r.checkDeckhouseReleaseLoop(ctx)
		go r.cleanupDeckhouseReleaseLoop(ctx)
	}()

	ctr, err := controller.New("deckhouse-release", mgr, controller.Options{
		MaxConcurrentReconciles: 1,
		CacheSyncTimeout:        3 * time.Minute,
		NeedLeaderElection:      pointer.Bool(false),
		Reconciler:              r,
	})
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.DeckhouseRelease{}).
		WithEventFilter(predicate.And(
			predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{}),
			releasePhasePredicate{},
		)).
		Complete(ctr)
}

type releasePhasePredicate struct{}

func (rp releasePhasePredicate) Create(ev event.CreateEvent) bool {
	switch ev.Object.(*v1alpha1.DeckhouseRelease).Status.Phase {
	case v1alpha1.PhaseSkipped, v1alpha1.PhaseSuperseded, v1alpha1.PhaseSuspended, v1alpha1.PhaseDeployed:
		return false
	}
	return true
}

// Delete returns true if the Delete event should be processed
func (rp releasePhasePredicate) Delete(_ event.DeleteEvent) bool {
	return false
}

// Update returns true if the Update event should be processed
func (rp releasePhasePredicate) Update(ev event.UpdateEvent) bool {
	switch ev.ObjectNew.(*v1alpha1.DeckhouseRelease).Status.Phase {
	case v1alpha1.PhaseSkipped, v1alpha1.PhaseSuperseded, v1alpha1.PhaseSuspended, v1alpha1.PhaseDeployed:
		return false
	}
	return true
}

// Generic returns true if the Generic event should be processed
func (rp releasePhasePredicate) Generic(_ event.GenericEvent) bool {
	return true
}

func (r *deckhouseReleaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	r.logger.Debugf("%s release processing started", req.Name)
	defer func() { r.logger.Debugf("%s release processing complete: %+v", req.Name, result) }()

	if r.updateSettings.Get().ReleaseChannel == "" {
		return ctrl.Result{}, nil
	}

	r.metricStorage.GroupedVault.ExpireGroupMetrics(metricReleasesGroup)

	release := new(v1alpha1.DeckhouseRelease)
	err = r.client.Get(ctx, req.NamespacedName, release)
	if err != nil {
		// The DeckhouseRelease resource may no longer exist, in which case we stop
		// processing.
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	if !release.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	return r.createOrUpdateReconcile(ctx, release)
}

func (r *deckhouseReleaseReconciler) createOrUpdateReconcile(ctx context.Context, dr *v1alpha1.DeckhouseRelease) (ctrl.Result, error) {
	// prepare releases
	switch dr.Status.Phase {
	// thees phases should be ignored by predicate, but let's check it
	case "":
		// initial state
		dr.Status.Phase = v1alpha1.PhasePending
		dr.Status.TransitionTime = metav1.NewTime(r.dc.GetClock().Now().UTC())
		if e := r.client.Status().Update(ctx, dr); e != nil {
			return ctrl.Result{Requeue: true}, e
		}

		return ctrl.Result{Requeue: true}, nil // process to the next phase

	case v1alpha1.PhaseSkipped, v1alpha1.PhaseSuperseded, v1alpha1.PhaseSuspended:
		return ctrl.Result{}, nil

	case v1alpha1.PhaseDeployed:
		// don't think we have to do anything with Deployed release
		// probably, we have to move the Deployment's image update logic here
		return ctrl.Result{}, nil
	}

	// update pending release with suspend annotation
	err := r.patchSuspendAnnotation(dr)
	if err != nil {
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, err
	}

	err = r.patchManualRelease(dr)
	if err != nil {
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, err
	}

	return r.pendingReleaseReconcile(ctx, dr)
}

func (r *deckhouseReleaseReconciler) patchManualRelease(dr *v1alpha1.DeckhouseRelease) error {
	if r.updateSettings.Get().Update.Mode != "Manual" {
		return nil
	}

	if !dr.GetManuallyApproved() {
		dr.SetApprovedStatus(false)
		// TODO: don't know yet how to count manual releases
		// du.totalPendingManualReleases++
	} else {
		dr.SetApprovedStatus(true)
	}
	dr.Status.TransitionTime = metav1.NewTime(r.dc.GetClock().Now().UTC())

	return r.client.Status().Update(context.Background(), dr)
}

func (r *deckhouseReleaseReconciler) patchSuspendAnnotation(dr *v1alpha1.DeckhouseRelease) error {
	if !dr.GetSuspend() {
		return nil
	}

	ctx := context.Background()
	patch, _ := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]any{
				"release.deckhouse.io/suspended": nil,
			},
		},
	})

	p := client.RawPatch(types.MergePatchType, patch)
	return r.client.Patch(ctx, dr, p)
}

func (r *deckhouseReleaseReconciler) pendingReleaseReconcile(ctx context.Context, dr *v1alpha1.DeckhouseRelease) (ctrl.Result, error) {
	if r.clusterUUID == "" {
		r.clusterUUID = r.getClusterUUID(ctx)
	}

	releaseData, err := r.getReleaseData(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("get release data: %w", err)
	}

	clusterBootstrapping := true
	var imagesRegistry string
	registrySecret, err := r.getRegistrySecret(ctx)
	if apierrors.IsNotFound(err) {
		err = nil
	}
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("get registry secret: %w", err)
	}

	if registrySecret != nil {
		clusterBootstrapped, ok := registrySecret.Data["clusterIsBootstrapped"]
		if ok {
			clusterBootstrapping = string(clusterBootstrapped) != `"true"`
		}

		imagesRegistry = string(registrySecret.Data["imagesRegistry"])
	}

	podReady := r.isDeckhousePodReady()
	us := r.updateSettings.Get()
	dus := &updater.DeckhouseUpdateSettings{
		NotificationConfig:     &us.Update.NotificationConfig,
		DisruptionApprovalMode: us.Update.DisruptionApprovalMode,
		Mode:                   us.Update.Mode,
		ClusterUUID:            r.clusterUUID,
	}
	deckhouseUpdater, err := d8updater.NewDeckhouseUpdater(
		r.logger, r.client, r.dc, dus, releaseData, r.metricStorage, podReady,
		clusterBootstrapping, imagesRegistry, r.moduleManager.GetEnabledModuleNames(),
	)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("initializing deckhouse updater: %w", err)
	}

	if podReady {
		r.metricStorage.GroupedVault.ExpireGroupMetrics(metricUpdatingGroup)

		if releaseData.IsUpdating {
			_ = deckhouseUpdater.ChangeUpdatingFlag(false)
		}
	} else if releaseData.IsUpdating {
		labels := map[string]string{
			"releaseChannel": r.updateSettings.Get().ReleaseChannel,
		}
		r.metricStorage.GroupedVault.GaugeSet(metricUpdatingGroup, "d8_is_updating", 1, labels)
	}

	var releases v1alpha1.DeckhouseReleaseList
	err = r.client.List(ctx, &releases)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("get deckhouse releases: %w", err)
	}

	pointerReleases := make([]*v1alpha1.DeckhouseRelease, 0, len(releases.Items))
	for _, rl := range releases.Items {
		pointerReleases = append(pointerReleases, &rl)
	}
	deckhouseUpdater.SetReleases(pointerReleases)

	if deckhouseUpdater.ReleasesCount() == 0 {
		return ctrl.Result{}, nil
	}

	// predict next patch for Deploy
	deckhouseUpdater.PredictNextRelease()

	// has already Deployed the latest release
	if deckhouseUpdater.LastReleaseDeployed() {
		return ctrl.Result{}, nil
	}

	skipped := deckhouseUpdater.GetSkippedPatchReleases()
	if len(skipped) > 0 {
		for _, sk := range skipped {
			sk.Status.Phase = v1alpha1.PhaseSkipped
			sk.Status.Message = ""
			sk.Status.TransitionTime = metav1.NewTime(r.dc.GetClock().Now().UTC())
			if e := r.client.Status().Update(ctx, sk); e != nil {
				return ctrl.Result{Requeue: true}, e
			}
		}
	}

	if rel := deckhouseUpdater.GetPredictedRelease(); rel != nil {
		if rel.GetName() != dr.GetName() {
			// don't requeue releases other than predicted one
			return ctrl.Result{}, nil
		}
	}

	// some release is forced, burn everything, apply this patch!
	if deckhouseUpdater.HasForceRelease() {
		if deckhouseUpdater.ApplyForcedRelease() {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	var windows update.Windows
	if deckhouseUpdater.PredictedReleaseIsPatch() {
		// patch release and ManualMode does not respect update windows
		windows = nil
	} else if !deckhouseUpdater.InManualMode() {
		windows = r.updateSettings.Get().Update.Windows
	}

	if deckhouseUpdater.ApplyPredictedRelease(windows) {
		return ctrl.Result{}, nil
	}

	return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
}

func (r *deckhouseReleaseReconciler) getDeckhouseLatestPod(ctx context.Context) (*corev1.Pod, error) {
	var pods corev1.PodList
	err := r.client.List(
		ctx,
		&pods,
		client.InNamespace("d8-system"),
		client.MatchingLabels{"app": "deckhouse", "leader": "true"},
	)
	if err != nil {
		return nil, fmt.Errorf("list deckhouse pods: %w", err)
	}

	var latestPod *corev1.Pod

	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}

		if latestPod == nil {
			latestPod = &pod
			continue
		}

		if pod.Status.StartTime != nil && latestPod.Status.StartTime != nil && pod.Status.StartTime.After(latestPod.Status.StartTime.Time) {
			latestPod = &pod
		}
	}

	return latestPod, nil
}

func (r *deckhouseReleaseReconciler) getClusterUUID(ctx context.Context) string {
	var secret corev1.Secret
	key := types.NamespacedName{Namespace: "d8-system", Name: "deckhouse-discovery"}
	err := r.client.Get(ctx, key, &secret)
	if err != nil {
		r.logger.Warnf("Read clusterUUID from secret %s failed: %v. Generating random uuid", key, err)
		return uuid.Must(uuid.NewV4()).String()
	}

	if clusterUUID, ok := secret.Data["clusterUUID"]; ok {
		return string(clusterUUID)
	}

	return uuid.Must(uuid.NewV4()).String()
}

func (r *deckhouseReleaseReconciler) tagUpdate(ctx context.Context, leaderPod *corev1.Pod) error {
	if len(leaderPod.Spec.Containers) == 0 || len(leaderPod.Status.ContainerStatuses) == 0 {
		r.logger.Debug("Deckhouse pod has no containers")
		return nil
	}

	deckhouseContainerIndex := getDeckhouseContainerIndex(leaderPod.Spec.Containers)
	deckhouseContainerStatusIndex := getDeckhouseContainerStatusIndex(leaderPod.Status.ContainerStatuses)

	if deckhouseContainerIndex == -1 {
		r.logger.Warnf("Pod %s does not contain a deckhouse container", leaderPod.Name)
		return nil
	}

	image := leaderPod.Spec.Containers[deckhouseContainerIndex].Image
	imageID := leaderPod.Status.ContainerStatuses[deckhouseContainerStatusIndex].ImageID

	if image == "" || imageID == "" {
		// pod is restarting or something like that, try more in 15 seconds
		return nil
	}

	if deckhouseContainerStatusIndex == -1 {
		r.logger.Warnf("Pod %s does not contain a deckhouse container status", leaderPod.Name)
		return nil
	}

	idSplitIndex := strings.LastIndex(imageID, "@")
	if idSplitIndex == -1 {
		return fmt.Errorf("image hash not found: %s", imageID)
	}
	imageHash := imageID[idSplitIndex+1:]

	imageRepoTag, err := gcr.NewTag(image)
	if err != nil {
		return fmt.Errorf("incorrect image: %s", image)
	}

	repo := imageRepoTag.Context().Name()
	tag := imageRepoTag.TagStr()

	registrySecret, err := r.getRegistrySecret(ctx)
	if apierrors.IsNotFound(err) {
		err = nil
	}
	if err != nil {
		return err
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
		return fmt.Errorf("registry (%s) client init failed: %s", repo, err)
	}

	r.metricStorage.CounterAdd("deckhouse_registry_check_total", 1, map[string]string{})
	r.metricStorage.CounterAdd("deckhouse_kube_image_digest_check_total", 1, map[string]string{})

	repoDigest, err := regClient.Digest(tag)
	if err != nil {
		r.metricStorage.CounterAdd("deckhouse_registry_check_errors_total", 1, map[string]string{})
		return fmt.Errorf("registry (%s) get digest failed: %s", repo, err)
	}

	r.metricStorage.CounterAdd("deckhouse_kube_image_digest_check_success", 1.0, map[string]string{})

	if strings.TrimSpace(repoDigest) == strings.TrimSpace(imageHash) {
		return nil
	}

	r.logger.Info("New deckhouse image found. Restarting")
	now := r.dc.GetClock().Now().Format(time.RFC3339)

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
				Namespace: leaderPod.Namespace,
				Name:      "deckhouse",
			},
		},
		client.RawPatch(types.MergePatchType, jsonPatch),
	)
	if err != nil {
		return fmt.Errorf("patch deckhouse deployment failed: %s", err)
	}

	return nil
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

	url := fmt.Sprintf("http://%s:4222/readyz", deckhousePodIP)
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

// development mode, without release channel
func (r *deckhouseReleaseReconciler) updateByImageHashLoop(ctx context.Context) {
	wait.UntilWithContext(ctx, func(ctx context.Context) {
		if r.updateSettings.Get().ReleaseChannel != "" {
			return
		}

		deckhouseLeaderPod, err := r.getDeckhouseLatestPod(ctx)
		if err != nil {
			r.logger.Warnf("Error getting deckhouse pods: %s", err)
			return
		}

		if deckhouseLeaderPod == nil {
			r.logger.Debug("Deckhouse pods not found. Skipping update")
			return
		}

		err = r.tagUpdate(ctx, deckhouseLeaderPod)
		if err != nil {
			r.logger.Errorf("deckhouse image tag update failed: %s", err)
		}
	}, 15*time.Second)
}

func getDeckhouseContainerIndex(containers []corev1.Container) int {
	for i := range containers {
		if containers[i].Name == "deckhouse" {
			return i
		}
	}

	return -1
}

func getDeckhouseContainerStatusIndex(statuses []corev1.ContainerStatus) int {
	for i := range statuses {
		if statuses[i].Name == "deckhouse" {
			return i
		}
	}

	return -1
}
