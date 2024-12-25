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
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	aoapp "github.com/flant/addon-operator/pkg/app"
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
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/ctrlutils"
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

	deckhouseNamespace          = "d8-system"
	deckhouseDeployment         = "deckhouse"
	deckhouseRegistrySecretName = "deckhouse-registry"

	deckhouseReleaseAnnotationDryrun            = "dryrun"
	deckhouseReleaseAnnotationTriggeredByDryrun = "triggered_by_dryrun"
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

	imageRegistry            string
	deckhouseIsBootstrapping bool
	metricsUpdater           updater.MetricsUpdater
}

func NewDeckhouseReleaseController(ctx context.Context, mgr manager.Manager, dc dependency.Container,
	moduleManager moduleManager, updateSettings *helpers.DeckhouseSettingsContainer, metricStorage *metricstorage.MetricStorage,
	preflightCountDown *sync.WaitGroup, logger *log.Logger,
) error {
	r := &deckhouseReleaseReconciler{
		client:             mgr.GetClient(),
		dc:                 dc,
		logger:             logger,
		moduleManager:      moduleManager,
		updateSettings:     updateSettings,
		metricStorage:      metricStorage,
		preflightCountDown: preflightCountDown,

		metricsUpdater: d8updater.NewMetricsUpdater(metricStorage),
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
	var res ctrl.Result

	r.logger.Debugf("%s release processing started", req.Name)
	defer func() { r.logger.Debugf("%s release processing complete: %+v", req.Name, res) }()

	if r.updateSettings.Get().ReleaseChannel == "" {
		r.logger.Debug("release channel not set")
		return res, nil
	}

	r.metricStorage.Grouped().ExpireGroupMetrics(metricReleasesGroup)

	release := new(v1alpha1.DeckhouseRelease)
	err := r.client.Get(ctx, req.NamespacedName, release)
	if err != nil {
		// The DeckhouseRelease resource may no longer exist, in which case we stop
		// processing.
		if apierrors.IsNotFound(err) {
			return res, nil
		}

		r.logger.Debugf("get release: %s", err.Error())

		return res, err
	}

	if !release.DeletionTimestamp.IsZero() {
		r.logger.Debugf("release deletion timestamp: %s", release.DeletionTimestamp.String())
		return res, nil
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

// 1) all new releases state is pending
// 2) sort releases by version
// 3) do predict
// 4) all releases between pending (include) and predicted became skipped
// 5) enroll predicted
// 6) requeue
func (r *deckhouseReleaseReconciler) createOrUpdateReconcile(ctx context.Context, dr *v1alpha1.DeckhouseRelease) (ctrl.Result, error) {
	var res ctrl.Result

	// prepare releases
	switch dr.Status.Phase {
	// these phases should be ignored by predicate, but let's check it
	case "":
		// initial state
		dr.Status.Phase = v1alpha1.ModuleReleasePhasePending
		dr.Status.TransitionTime = metav1.NewTime(r.dc.GetClock().Now().UTC())
		if err := r.client.Status().Update(ctx, dr); err != nil {
			return res, err
		}

		return ctrl.Result{Requeue: true}, nil // process to the next phase

	case v1alpha1.ModuleReleasePhaseSkipped, v1alpha1.ModuleReleasePhaseSuperseded, v1alpha1.ModuleReleasePhaseSuspended:
		r.logger.Debugf("release phase: %s", dr.Status.Phase)
		return res, nil

	case v1alpha1.ModuleReleasePhaseDeployed:
		// don't think we have to do anything with Deployed release
		// probably, we have to move the Deployment's image update logic here
		return r.reconcileDeployedRelease(ctx, dr)
	}

	// TODO: getting check out?
	//
	// update pending release with suspend annotation
	err := r.patchSuspendAnnotation(dr)
	if err != nil {
		return res, err
	}

	// TODO: getting check out?
	//
	err = r.patchManualRelease(dr)
	if err != nil {
		return res, err
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
				v1alpha1.DeckhouseReleaseAnnotationSuspended: nil,
			},
		},
	})

	p := client.RawPatch(types.MergePatchType, patch)
	return r.client.Patch(ctx, dr, p)
}

func (r *deckhouseReleaseReconciler) pendingReleaseReconcile(ctx context.Context, dr *v1alpha1.DeckhouseRelease) (ctrl.Result, error) {
	var res ctrl.Result

	clusterBootstrapping := true
	var imagesRegistry string
	// TODO: make registry service to check secrets in it
	// TODO: incapsulate to kubernetes service
	registrySecret, err := r.getRegistrySecret(ctx)
	if apierrors.IsNotFound(err) {
		err = nil
	}
	if err != nil {
		return res, fmt.Errorf("get registry secret: %w", err)
	}

	if registrySecret != nil {
		if registrySecret.ClusterIsBootstrapped != "" {
			// is it working???
			clusterBootstrapping = registrySecret.ClusterIsBootstrapped != `"true"`
		}

		imagesRegistry = registrySecret.ImageRegistry

		r.imageRegistry = registrySecret.ImageRegistry
		r.deckhouseIsBootstrapping = registrySecret.ClusterIsBootstrapped != `"true"`
		defer func() {
			r.imageRegistry = ""
			r.deckhouseIsBootstrapping = false
		}()
	}

	// TODO: ready check service?
	// note: in module release we have pod ready by default
	podReady := r.isDeckhousePodReady(ctx)

	us := r.updateSettings.Get()

	dus := &updater.Settings{
		NotificationConfig:     us.Update.NotificationConfig,
		DisruptionApprovalMode: us.Update.DisruptionApprovalMode,
		// if we have whrong mode - autopatch
		Mode:    updater.ParseUpdateMode(us.Update.Mode),
		Windows: us.Update.Windows,
	}

	// TODO: rename get release update settings? because there's no data
	releaseData := getReleaseData(dr)

	// note: in module we have no
	// 1) release data
	// 2) pod is ready true by default
	// 3) bootstrapping is false by default
	//
	// TODO: replace updater (struct with logic) with permanent service with info about current release info?
	deckhouseUpdater := d8updater.NewDeckhouseUpdater(
		ctx, r.logger, r.client, r.dc, dus, releaseData, r.metricStorage, podReady,
		clusterBootstrapping, imagesRegistry, r.moduleManager.GetEnabledModuleNames(),
	)

	if releaseData.IsUpdating && !podReady {
		r.metricStorage.Grouped().GaugeSet(metricUpdatingGroup, "d8_is_updating", 1, map[string]string{"releaseChannel": r.updateSettings.Get().ReleaseChannel})
	}

	if podReady {
		r.metricStorage.Grouped().ExpireGroupMetrics(metricUpdatingGroup)

		if releaseData.IsUpdating {
			// note: if pod is ready and release is updating - patch annotations if changed
			// here, we patch updating annotation
			// but we have no patch here, because we have no predicted release and just change update settings (what???)
			_ = deckhouseUpdater.ChangeUpdatingFlag(false)

			// releaseData.IsUpdating = false
		}
	}

	////////////////////////////////////////////////////
	// New Logic start
	//
	// 1) Calculate task for current release
	// 1.1) if skip - update phase to Skipped and stop reconcile
	// 1.2) if await - update phase to Pending and requeue
	// 1.3) if process - go forward
	// 2) Check requirements
	// 2.1) if not met any requirements - update phase to Pending with all requirements errors and requeue
	// 3) Apply if force with force logic ???
	// 4) Apply ussually release
	oCalc := d8updater.NewTaskCalculator(r.client, r.logger)
	task, err := oCalc.CalculatePendingReleaseOrder(ctx, dr)
	if err != nil {
		return res, err
	}

	switch task.TaskType {
	case d8updater.Process:
		// pass
	case d8updater.Skip:
		err := r.updateReleaseStatus(ctx, dr, &v1alpha1.DeckhouseReleaseStatus{
			Phase:   v1alpha1.DeckhouseReleasePhaseSkipped,
			Message: task.Message,
		})
		if err != nil {
			r.logger.Warn("skip order status update ", slog.String("name", dr.GetName()), log.Err(err))
			return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
		}

		return res, nil
	case d8updater.Await:
		err := r.updateReleaseStatus(ctx, dr, &v1alpha1.DeckhouseReleaseStatus{
			Phase:   v1alpha1.DeckhouseReleasePhasePending,
			Message: task.Message,
		})
		if err != nil {
			r.logger.Warn("await order status update ", slog.String("name", dr.GetName()), log.Err(err))
		}

		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	// add to reconciler???
	checker, err := d8updater.NewRequirementsChecker(r.client, r.moduleManager.GetEnabledModuleNames(), r.logger)
	if err != nil {
		updateErr := r.updateReleaseStatus(ctx, dr, &v1alpha1.DeckhouseReleaseStatus{
			Phase:   v1alpha1.DeckhouseReleasePhasePending,
			Message: err.Error(),
		})
		if updateErr != nil {
			r.logger.Warn("create release checker status update ", slog.String("name", dr.GetName()), log.Err(err))
		}

		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	reasons := checker.MetRequirements(dr)
	if len(reasons) > 0 {
		msgs := make([]string, 0, len(reasons))
		for _, reason := range reasons {
			msgs = append(msgs, fmt.Sprintf("reason: %s, requirement: %s", reason.Reason, reason.Message))
		}

		err := r.updateReleaseStatus(ctx, dr, &v1alpha1.DeckhouseReleaseStatus{
			Phase:   v1alpha1.DeckhouseReleasePhasePending,
			Message: strings.Join(msgs, ";"),
		})
		if err != nil {
			r.logger.Warn("met requirements status update ", slog.String("name", dr.GetName()), log.Err(err))
		}

		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	if dr.GetForce() {
		err := r.ApplyForcedRelease(ctx, dr, task)
		if err != nil {
			return res, fmt.Errorf("apply forced release: %w", err)
		}

		// TODO: in original code we return empty result (reconcile was ended)
		// is it correct???
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	// if deckhouse pod has bootstrap image -> apply first release
	// doesn't matter which is update mode
	if r.deckhouseIsBootstrapping && task.IsSingle {
		err := r.runReleaseDeploy(ctx, dr, task.DeployedReleaseInfo)
		if err != nil {
			return res, fmt.Errorf("run single bootstrapping release deploy: %w", err)
		}

		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	metricLabels := updater.NewReleaseMetricLabels(dr)

	var dtr *d8updater.DeployTimeReason
	timeChecker := d8updater.NewDeployTimeChecker(r.dc, r.metricsUpdater, dus, r.isDeckhousePodReady, r.logger)
	if task.IsPatch {
		dtr = timeChecker.CheckPatchReleaseConditions(ctx, dr, metricLabels)
	} else {
		dtr = timeChecker.CheckMinorReleaseConditions(ctx, dr, metricLabels)
	}

	if dtr != nil {
		err := r.updateReleaseStatus(ctx, dr, &v1alpha1.DeckhouseReleaseStatus{
			Phase:   v1alpha1.DeckhouseReleasePhasePending,
			Message: dtr.Message,
		})
		if err != nil {
			r.logger.Warn("met release conditions status update ", slog.String("name", dr.GetName()), log.Err(err))
		}

		err = ctrlutils.UpdateWithRetry(ctx, r.client, dr, func() error {
			if !dtr.ReleaseApplyAfterTime.IsZero() {
				dr.Spec.ApplyAfter = &metav1.Time{Time: dtr.ReleaseApplyAfterTime.UTC()}
			}

			if dr.Annotations == nil {
				dr.Annotations = make(map[string]string, 1)
			}

			dr.Annotations[v1alpha1.DeckhouseReleaseAnnotationNotificationTimeShift] = "true"

			return nil
		})
		if err != nil {
			r.logger.Warn("met release conditions resource update ", slog.String("name", dr.GetName()), log.Err(err))
		}

		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	err = r.ApplyPredictedRelease(ctx, dr, task, metricLabels)
	if err != nil {
		return res, fmt.Errorf("apply predicted release: %w", err)
	}

	return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil

	////////////////////////////////////////////////////
	// New Logic End
	//

	{
		var releases v1alpha1.DeckhouseReleaseList
		err = r.client.List(ctx, &releases)
		if err != nil {
			return res, fmt.Errorf("get deckhouse releases: %w", err)
		}

		if len(releases.Items) == 0 {
			r.logger.Debug("releases count is zero")
			return res, nil
		}

		// note: slice pointer? purpose? only because of use generics???
		pointerReleases := make([]*v1alpha1.DeckhouseRelease, 0, len(releases.Items))
		for _, rl := range releases.Items {
			pointerReleases = append(pointerReleases, &rl)
		}

		// sort by version and save it to updater
		deckhouseUpdater.SetReleases(pointerReleases)
	}

	// predict next patch for Deploy
	deckhouseUpdater.PredictNextRelease(dr)

	// has already Deployed the latest release
	if deckhouseUpdater.LastReleaseDeployed() {
		r.logger.Debug("latest release is deployed")
		return res, nil
	}

	// set skipped releases to PhaseSkipped
	if err = deckhouseUpdater.CommitSkippedReleases(); err != nil {
		return res, err
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
		if err != nil {
			return res, fmt.Errorf("apply forced release: %w", err)
		}

		return res, nil
	}

	err = deckhouseUpdater.ApplyPredictedRelease()
	if err != nil {
		return r.wrapApplyReleaseError(err)
	}

	return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
}

// ApplyForcedRelease deploys forced release without any checks (windows, requirements, approvals and so on)
func (r *deckhouseReleaseReconciler) ApplyForcedRelease(ctx context.Context, dr *v1alpha1.DeckhouseRelease, order *d8updater.Task) error {
	r.logger.Warn("forcing release", slog.String("release", dr.GetName()))

	err := r.runReleaseDeploy(ctx, dr, order.DeployedReleaseInfo)
	if err != nil {
		return fmt.Errorf("run release deploy: %w", err)
	}

	// TODO: if force, make ALL previous releases superseded??? or just deployed?
	// // Outdate all previous releases
	// for i, release := range u.releases {
	// 	if i < u.forcedReleaseIndex {
	// 		err := u.updateStatus(release, "", PhaseSuperseded)
	// 		if err != nil {
	// 			u.logger.Error("update status", log.Err(err))
	// 		}
	// 	}
	// }

	return nil
}

// ApplyPredictedRelease applies predicted release, checks everything:
//   - Deckhouse is ready (except patch)
//   - Canary settings
//   - Manual approving
//   - Release requirements
//
// In addition to the regular error, ErrDeployConditionsNotMet or NotReadyForDeployError is returned as appropriate.
func (r *deckhouseReleaseReconciler) ApplyPredictedRelease(ctx context.Context, dr *v1alpha1.DeckhouseRelease, order *d8updater.Task, metricLabels updater.MetricLabels) error {
	var err error
	if metricLabels[updater.ManualApprovalRequired] == "true" {
		metricLabels[updater.ReleaseQueueDepth] = strconv.Itoa(order.QueueDepth)
	}

	// if the predicted release has an index less than the number of awaiting releases
	// calculate and set releaseDepthQueue label
	r.metricsUpdater.UpdateReleaseMetric(dr.GetName(), metricLabels)
	if err != nil {
		return fmt.Errorf("check release %s conditions: %w", dr.GetName(), err)
	}

	err = r.runReleaseDeploy(ctx, dr, order.DeployedReleaseInfo)
	if err != nil {
		return fmt.Errorf("run release deploy: %w", err)
	}

	return nil
}

// 1) bump deckhouse deployment (retry if error)
// 2) bump previous deployment status superseded (retry if error)
// 3) bump release annotations (retry if error)
// 3) bump release status to deployed (retry if error)
func (r *deckhouseReleaseReconciler) runReleaseDeploy(ctx context.Context, dr *v1alpha1.DeckhouseRelease, deployedReleaseInfo *d8updater.ReleaseInfo) error {
	r.logger.Infof("Applying release %s", dr.GetName())

	err := r.bumpDeckhouseDeployment(ctx, dr)
	if err != nil {
		return fmt.Errorf("deploy release: %w", err)
	}

	if deployedReleaseInfo != nil {
		err := r.updateReleaseStatus(ctx, newDeckhouseReleaseWithName(deployedReleaseInfo.Name), &v1alpha1.DeckhouseReleaseStatus{
			Phase:   v1alpha1.DeckhouseReleasePhaseSuperseded,
			Message: "",
		})
		if err != nil {
			r.logger.Error("update status", slog.String("release", deployedReleaseInfo.Name), log.Err(err))
		}
	}

	err = ctrlutils.UpdateWithRetry(ctx, r.client, dr, func() error {
		annotations := map[string]string{
			v1alpha1.DeckhouseReleaseAnnotationIsUpdating: strconv.FormatBool(true),
			v1alpha1.DeckhouseReleaseAnnotationNotified:   strconv.FormatBool(false),
		}

		if dr.Annotations == nil {
			dr.Annotations = make(map[string]string, 2)
		}

		for k, v := range annotations {
			dr.Annotations[k] = v
		}

		if dr.GetApplyNow() {
			delete(dr.Annotations, v1alpha1.DeckhouseReleaseAnnotationApplyNow)
			delete(dr.Annotations, v1alpha1.DeckhouseReleaseAnnotationForce)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("update with retry: %w", err)
	}

	err = r.updateReleaseStatus(ctx, dr, &v1alpha1.DeckhouseReleaseStatus{
		Phase: v1alpha1.DeckhouseReleasePhaseDeployed,
	})
	if err != nil {
		return fmt.Errorf("update status with retry: %w", err)
	}

	// TODO: make it after all deployed instructions???
	// if currentRelease != nil {
	// 	// skip last deployed release
	// 	err = r.updateStatus(*currentRelease, "", PhaseSuperseded)
	// 	if err != nil {
	// 		return fmt.Errorf("update status to superseded: %w", err)
	// 	}
	// }

	// TODO: purpose???
	// return r.CommitSkippedReleases()

	return nil
}

func (r *deckhouseReleaseReconciler) bumpDeckhouseDeployment(ctx context.Context, dr *v1alpha1.DeckhouseRelease) error {
	key := client.ObjectKey{Namespace: deckhouseNamespace, Name: deckhouseDeployment}

	depl := new(appsv1.Deployment)

	err := r.client.Get(ctx, key, depl)
	if err != nil {
		return fmt.Errorf("get deployment %s: %w", key, err)
	}

	// dryrun for testing purpose
	val, ok := dr.GetAnnotations()[deckhouseReleaseAnnotationDryrun]
	if ok && val == "true" {
		go func() {
			r.logger.Debug("dryrun start soon...")

			time.Sleep(3 * time.Second)

			r.logger.Debug("dryrun started")

			// because we do not know how long is parent context and how long will be update
			// 1 minute - magic constant
			ctxwt, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
			defer cancel()

			releases := new(v1alpha1.DeckhouseReleaseList)
			err = r.client.List(ctxwt, releases)
			if err != nil {
				r.logger.Error("dryrun list deckhouse releases", log.Err(err))

				return
			}

			for _, release := range releases.Items {
				release := &release

				if release.GetName() == dr.GetName() {
					continue
				}

				if release.Status.Phase != v1alpha1.ModuleReleasePhasePending {
					continue
				}

				// update releases to trigger their requeue
				err := ctrlutils.UpdateWithRetry(ctxwt, r.client, release, func() error {
					if release.Annotations == nil {
						release.Annotations = make(map[string]string, 1)
					}

					release.Annotations[deckhouseReleaseAnnotationTriggeredByDryrun] = dr.GetName()

					return nil
				})
				if err != nil {
					r.logger.Error("dryrun update release to requeue", log.Err(err))
				}

				r.logger.Debug("dryrun release successfully updated", slog.String("release", release.Name))
			}
		}()

		return nil
	}

	return ctrlutils.UpdateWithRetry(ctx, r.client, depl, func() error {
		depl.Spec.Template.Spec.Containers[0].Image = r.imageRegistry + ":" + dr.Spec.Version

		return nil
	})
}

func (r *deckhouseReleaseReconciler) wrapApplyReleaseError(err error) (ctrl.Result, error) {
	var res ctrl.Result

	var notReadyErr *updater.NotReadyForDeployError
	if errors.As(err, &notReadyErr) {
		r.logger.Warn(err.Error())
		// TODO: requeue all releases if deckhouse update settings is changed
		// requeueAfter := notReadyErr.RetryDelay()
		// if requeueAfter == 0 {
		// requeueAfter = defaultCheckInterval
		// }
		// r.logger.Infof("%s: retry after %s", err.Error(), requeueAfter)
		// return ctrl.Result{RequeueAfter: requeueAfter}, nil
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	return res, fmt.Errorf("apply predicted release: %w", err)
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
		rconf := &utils.RegistryConfig{
			DockerConfig: registrySecret.DockerConfig,
			Scheme:       registrySecret.Scheme,
			CA:           registrySecret.CA,
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

func (r *deckhouseReleaseReconciler) getRegistrySecret(ctx context.Context) (*utils.DeckhouseRegistrySecret, error) {
	key := types.NamespacedName{Namespace: deckhouseNamespace, Name: deckhouseRegistrySecretName}

	secret := new(corev1.Secret)

	err := r.client.Get(ctx, key, secret)
	if err != nil {
		return nil, fmt.Errorf("get secret %s: %w", key, err)
	}

	regSecret, _ := utils.ParseDeckhouseRegistrySecret(secret.Data)

	return regSecret, nil
}

func (r *deckhouseReleaseReconciler) isDeckhousePodReady(ctx context.Context) bool {
	deckhousePodIP := aoapp.ListenAddress

	url := fmt.Sprintf("http://%s:4222/readyz", deckhousePodIP)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
	var res ctrl.Result

	if r.isDeckhousePodReady(ctx) {
		data := getReleaseData(dr)
		data.IsUpdating = false
		err := r.newUpdaterKubeAPI().SaveReleaseData(ctx, dr, data)
		if err != nil {
			return res, fmt.Errorf("change updating flag: %w", err)
		}
		return res, nil
	}

	return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
}

func (r *deckhouseReleaseReconciler) newUpdaterKubeAPI() *d8updater.KubeAPI {
	return d8updater.NewKubeAPI(r.client, r.dc, "")
}

func (r *deckhouseReleaseReconciler) updateReleaseStatus(ctx context.Context, dr *v1alpha1.DeckhouseRelease, status *v1alpha1.DeckhouseReleaseStatus) error {
	r.logger.Debugf("refresh the %q release status", dr.GetName())

	return ctrlutils.UpdateStatusWithRetry(ctx, r.client, dr, func() error {
		if dr.Status.Phase != status.Phase {
			dr.Status.TransitionTime = metav1.NewTime(r.dc.GetClock().Now().UTC())
		}

		dr.Status.Phase = status.Phase
		dr.Status.Message = status.Message

		return nil
	})
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
		IsUpdating: dr.Annotations[v1alpha1.DeckhouseReleaseAnnotationIsUpdating] == "true",
		Notified:   dr.Annotations[v1alpha1.DeckhouseReleaseAnnotationNotified] == "true",
	}
}

func newDeckhouseReleaseWithName(name string) *v1alpha1.DeckhouseRelease {
	return &v1alpha1.DeckhouseRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}
