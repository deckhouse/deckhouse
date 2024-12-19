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
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	metricstorage "github.com/flant/shell-operator/pkg/metric_storage"
	"github.com/gofrs/uuid/v5"
	gcr "github.com/google/go-containerregistry/pkg/name"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	d8updater "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/deckhouse-release/updater"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/go_lib/updater"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	metricReleasesGroup = "d8_releases"
	metricUpdatingGroup = "d8_updating"
)

const defaultCheckInterval = 15 * time.Second

type deckhouseReleaseReconciler struct {
	client        client.Client
	dc            dependency.Container
	logger        *log.Logger
	moduleManager moduleManager

	updateSettings *helpers.DeckhouseSettingsContainer
	metricStorage  *metricstorage.MetricStorage

	preflightCountDown      *sync.WaitGroup
	clusterUUID             string
	releaseVersionImageHash string
}

func NewDeckhouseReleaseController(ctx context.Context, mgr manager.Manager, dc dependency.Container,
	moduleManager moduleManager, updateSettings *helpers.DeckhouseSettingsContainer, metricStorage *metricstorage.MetricStorage,
	preflightCountDown *sync.WaitGroup, logger *log.Logger,
) error {
	r := &deckhouseReleaseReconciler{
		client:             mgr.GetClient(),
		dc:                 dc,
		logger:             logger.Named("deckhouse-release-controller"),
		moduleManager:      moduleManager,
		updateSettings:     updateSettings,
		metricStorage:      metricStorage,
		preflightCountDown: preflightCountDown,
	}

	// Add Preflight Check
	err := mgr.Add(manager.RunnableFunc(r.PreflightCheck))
	if err != nil {
		return err
	}
	r.preflightCountDown.Add(1)

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
		NeedLeaderElection:      ptr.To(false),
		Reconciler:              r,
	})
	if err != nil {
		return err
	}

	r.logger.Info("Controller started")

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.DeckhouseRelease{}).
		WithEventFilter(logWrapper{r.logger, newEventFilter()}).
		Complete(ctr)
}

func (r *deckhouseReleaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var result ctrl.Result
	r.logger.Debugf("%s release processing started", req.Name)
	defer func() { r.logger.Debugf("%s release processing complete: %+v", req.Name, result) }()

	if r.updateSettings.Get().ReleaseChannel == "" {
		r.logger.Debug("release channel not set")
		return result, nil
	}

	r.metricStorage.Grouped().ExpireGroupMetrics(metricReleasesGroup)

	release := new(v1alpha1.DeckhouseRelease)
	err := r.client.Get(ctx, req.NamespacedName, release)
	if err != nil {
		r.logger.Debugf("get release: %s", err.Error())
		// The DeckhouseRelease resource may no longer exist, in which case we stop
		// processing.
		if apierrors.IsNotFound(err) {
			return result, nil
		}

		return result, err
	}

	if !release.DeletionTimestamp.IsZero() {
		r.logger.Debugf("release deletion timestamp: %s", release.DeletionTimestamp.String())
		return result, nil
	}

	return r.createOrUpdateReconcile(ctx, release)
}

func (r *deckhouseReleaseReconciler) PreflightCheck(ctx context.Context) error {
	r.clusterUUID = r.getClusterUUID(ctx)
	r.preflightCountDown.Done()

	return nil
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

func (r *deckhouseReleaseReconciler) createOrUpdateReconcile(ctx context.Context, dr *v1alpha1.DeckhouseRelease) (ctrl.Result, error) {
	var result ctrl.Result

	// prepare releases
	switch dr.Status.Phase {
	// these phases should be ignored by predicate, but let's check it
	case "":
		// initial state
		dr.Status.Phase = v1alpha1.ModuleReleasePhasePending
		dr.Status.TransitionTime = metav1.NewTime(r.dc.GetClock().Now().UTC())
		if e := r.client.Status().Update(ctx, dr); e != nil {
			return ctrl.Result{Requeue: true}, e
		}

		return ctrl.Result{Requeue: true}, nil // process to the next phase

	case v1alpha1.ModuleReleasePhaseSkipped, v1alpha1.ModuleReleasePhaseSuperseded, v1alpha1.ModuleReleasePhaseSuspended:
		r.logger.Debugf("release phase: %s", dr.Status.Phase)
		return result, nil

	case v1alpha1.ModuleReleasePhaseDeployed:
		// don't think we have to do anything with Deployed release
		// probably, we have to move the Deployment's image update logic here
		return r.reconcileDeployedRelease(ctx, dr)
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
	if r.updateSettings.Get().Update.Mode != updater.ModeManual.String() {
		return nil
	}

	if !dr.GetManuallyApproved() {
		dr.SetApprovedStatus(false)
		// TODO: don't know yet how to count manual releases
		// du.totalPendingManualReleases++
	} else {
		dr.SetApprovedStatus(true)
	}

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
	var result ctrl.Result

	clusterBootstrapping := true
	var imagesRegistry string
	registrySecret, err := r.getRegistrySecret(ctx)
	if apierrors.IsNotFound(err) {
		err = nil
	}
	if err != nil {
		return result, fmt.Errorf("get registry secret: %w", err)
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
	dus := &updater.Settings{
		NotificationConfig:     us.Update.NotificationConfig,
		DisruptionApprovalMode: us.Update.DisruptionApprovalMode,
		Mode:                   updater.ParseUpdateMode(us.Update.Mode),
		Windows:                us.Update.Windows,
	}
	releaseData := getReleaseData(dr)
	deckhouseUpdater := d8updater.NewDeckhouseUpdater(
		ctx, r.logger, r.client, r.dc, dus, releaseData, r.metricStorage, podReady,
		clusterBootstrapping, imagesRegistry, r.moduleManager.GetEnabledModuleNames(),
	)

	if podReady {
		r.metricStorage.Grouped().ExpireGroupMetrics(metricUpdatingGroup)

		if releaseData.IsUpdating {
			_ = deckhouseUpdater.ChangeUpdatingFlag(false)
		}
	} else if releaseData.IsUpdating {
		r.metricStorage.Grouped().GaugeSet(metricUpdatingGroup, "d8_is_updating", 1, map[string]string{"releaseChannel": r.updateSettings.Get().ReleaseChannel})
	}
	{
		var releases v1alpha1.DeckhouseReleaseList
		err = r.client.List(ctx, &releases)
		if err != nil {
			return result, fmt.Errorf("get deckhouse releases: %w", err)
		}

		pointerReleases := make([]*v1alpha1.DeckhouseRelease, 0, len(releases.Items))
		for _, rl := range releases.Items {
			pointerReleases = append(pointerReleases, &rl)
		}
		deckhouseUpdater.SetReleases(pointerReleases)
	}

	if deckhouseUpdater.ReleasesCount() == 0 {
		r.logger.Debug("releases count is zero")
		return result, nil
	}

	// predict next patch for Deploy
	deckhouseUpdater.PredictNextRelease(dr)

	// has already Deployed the latest release
	if deckhouseUpdater.LastReleaseDeployed() {
		r.logger.Debug("latest release is deployed")
		return result, nil
	}

	// set skipped releases to PhaseSkipped
	if err = deckhouseUpdater.CommitSkippedReleases(); err != nil {
		return result, err
	}

	if rel := deckhouseUpdater.GetPredictedRelease(); rel != nil {
		if rel.GetName() != dr.GetName() {
			// requeue all releases to keep syncing releases' metrics
			r.logger.Debugf("processing wrong release (current: %s, predicted: %s)", dr.Name, rel.Name)
			return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
		}
	}

	// some release is forced, burn everything, apply this patch!
	if deckhouseUpdater.HasForceRelease() {
		err = deckhouseUpdater.ApplyForcedRelease(ctx)
		if err == nil {
			return result, nil
		}

		return result, fmt.Errorf("apply forced release: %w", err)
	}

	err = deckhouseUpdater.ApplyPredictedRelease()
	if err != nil {
		return r.wrapApplyReleaseError(err)
	}

	return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
}

func (r *deckhouseReleaseReconciler) wrapApplyReleaseError(err error) (ctrl.Result, error) {
	var result ctrl.Result
	var notReadyErr *updater.NotReadyForDeployError
	if errors.As(err, &notReadyErr) {
		r.logger.Info(err.Error())
		// TODO: requeue all releases if deckhouse update settings is changed
		// requeueAfter := notReadyErr.RetryDelay()
		// if requeueAfter == 0 {
		// requeueAfter = defaultCheckInterval
		// }
		// r.logger.Infof("%s: retry after %s", err.Error(), requeueAfter)
		// return ctrl.Result{RequeueAfter: requeueAfter}, nil
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	return result, fmt.Errorf("apply predicted release: %w", err)
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
		drs, _ := utils.ParseDeckhouseRegistrySecret(registrySecret.Data)
		rconf := &utils.RegistryConfig{
			DockerConfig: drs.DockerConfig,
			Scheme:       drs.Scheme,
			CA:           drs.CA,
			UserAgent:    r.clusterUUID,
		}
		opts = utils.GenerateRegistryOptions(rconf, r.logger)
	}

	regClient, err := r.dc.GetRegistryClient(repo, opts...)
	if err != nil {
		return fmt.Errorf("registry (%s) client init failed: %s", repo, err)
	}

	r.metricStorage.CounterAdd("deckhouse_registry_check_total", 1, map[string]string{})
	r.metricStorage.CounterAdd("deckhouse_kube_image_digest_check_total", 1, map[string]string{})

	repoDigest, err := regClient.Digest(ctx, tag)
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

func (r *deckhouseReleaseReconciler) reconcileDeployedRelease(ctx context.Context, dr *v1alpha1.DeckhouseRelease) (ctrl.Result, error) {
	var result ctrl.Result

	if r.isDeckhousePodReady() {
		data := getReleaseData(dr)
		data.IsUpdating = false
		err := r.newUpdaterKubeAPI().SaveReleaseData(ctx, dr, data)
		if err != nil {
			return result, fmt.Errorf("change updating flag: %w", err)
		}
		return result, nil
	}

	return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
}

func (r *deckhouseReleaseReconciler) newUpdaterKubeAPI() *d8updater.KubeAPI {
	return d8updater.NewKubeAPI(r.client, r.dc, "")
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

func getReleaseData(dr *v1alpha1.DeckhouseRelease) updater.DeckhouseReleaseData {
	return updater.DeckhouseReleaseData{
		IsUpdating: dr.Annotations[d8updater.IsUpdatingAnnotation] == "true",
		Notified:   dr.Annotations[d8updater.NotifiedAnnotation] == "true",
	}
}
