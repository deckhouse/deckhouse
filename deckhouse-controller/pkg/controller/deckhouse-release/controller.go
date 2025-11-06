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

	"github.com/Masterminds/semver/v3"
	aoapp "github.com/flant/addon-operator/pkg/app"
	"github.com/gofrs/uuid/v5"
	gcr "github.com/google/go-containerregistry/pkg/name"
	"go.opentelemetry.io/otel"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/ctrlutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	releaseUpdater "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/releaseupdater"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

const (
	deckhouseNamespace          = "d8-system"
	deckhouseDeployment         = "deckhouse"
	deckhouseRegistrySecretName = "deckhouse-registry"

	controllerName = "d8-deckhouse-release-controller"
)

const defaultCheckInterval = 15 * time.Second

type ReleaseUpdateInfo struct {
	TaskCalculation struct {
		TaskType string `json:"taskType,omitempty"`
		IsPatch  bool   `json:"isPatch"`
		IsMajor  bool   `json:"isMajor"`
		IsFromTo bool   `json:"isFromTo"`
		IsSingle bool   `json:"isSingle"`
		IsLatest bool   `json:"isLatest"`
	} `json:"taskCalculation"`

	UpdatePolicy struct {
		Mode string `json:"mode,omitempty"`
	} `json:"updatePolicy"`

	ForceRelease struct {
		IsForced bool `json:"isForced"`
	} `json:"forceRelease"`

	PodReadiness struct {
		IsReady bool `json:"isReady"`
	} `json:"podReadiness"`

	RequirementsCheck struct {
		RequirementsMet bool `json:"requirementsMet"`
	} `json:"requirementsCheck"`
}

type MetricsUpdater interface {
	UpdateReleaseMetric(string, releaseUpdater.MetricLabels)
	PurgeReleaseMetric(string)
}

type deckhouseReleaseReconciler struct {
	client client.Client
	dc     dependency.Container
	exts   *extenders.ExtendersStack

	logger        *log.Logger
	moduleManager moduleManager

	updateSettings *helpers.DeckhouseSettingsContainer
	metricStorage  metricsstorage.Storage

	preflightCountDown      *sync.WaitGroup
	clusterUUID             string
	releaseVersionImageHash string

	registrySecret *utils.DeckhouseRegistrySecret
	metricsUpdater MetricsUpdater
	eventRecorder  record.EventRecorder

	deckhouseVersion string
}

func NewDeckhouseReleaseController(ctx context.Context, mgr manager.Manager, dc dependency.Container, exts *extenders.ExtendersStack,
	moduleManager moduleManager, updateSettings *helpers.DeckhouseSettingsContainer, metricStorage metricsstorage.Storage,
	preflightCountDown *sync.WaitGroup, deckhouseVersion string, logger *log.Logger,
) error {
	parsedVersion, err := semver.NewVersion(deckhouseVersion)
	if err != nil {
		return fmt.Errorf("parse deckhouse version: %w", err)
	}

	r := &deckhouseReleaseReconciler{
		client:             mgr.GetClient(),
		dc:                 dc,
		exts:               exts,
		logger:             logger,
		moduleManager:      moduleManager,
		updateSettings:     updateSettings,
		metricStorage:      metricStorage,
		preflightCountDown: preflightCountDown,
		deckhouseVersion:   fmt.Sprintf("v%d.%d.%d", parsedVersion.Major(), parsedVersion.Minor(), parsedVersion.Patch()),

		metricsUpdater: releaseUpdater.NewMetricsUpdater(metricStorage, releaseUpdater.D8ReleaseBlockedMetricName),
		eventRecorder:  mgr.GetEventRecorderFor("deckhouse-release-controller"),
	}

	// Add Preflight Check
	if err = mgr.Add(manager.RunnableFunc(r.PreflightCheck)); err != nil {
		return fmt.Errorf("add a runnable function: %w", err)
	}
	r.preflightCountDown.Add(1)

	// wait for cache sync
	go func() {
		if ok := mgr.GetCache().WaitForCacheSync(ctx); !ok {
			r.logger.Fatal("Sync cache failed")
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

	r.logger.Debug("release processing started", slog.String("resource_name", req.Name))
	defer func() {
		r.logger.Debug("release processing complete", slog.String("resource_name", req.Name), slog.Any("reconcile_result", res))
	}()

	if r.updateSettings.Get().ReleaseChannel == "" {
		r.logger.Warn("release channel not set")
		return res, nil
	}

	release := new(v1alpha1.DeckhouseRelease)
	err := r.client.Get(ctx, req.NamespacedName, release)
	if err != nil {
		// The DeckhouseRelease resource may no longer exist, in which case we stop
		// processing.
		if apierrors.IsNotFound(err) {
			return res, nil
		}

		r.logger.Debug("get release", log.Err(err))

		return res, err
	}

	if !release.DeletionTimestamp.IsZero() {
		r.logger.Debug("release deletion", slog.String("deletion_timestamp", release.DeletionTimestamp.String()))
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
		r.logger.Warn("read clusterUUID from secret", slog.Any("namespaced_name", key), log.Err(err))
		r.logger.Warn("generating random uuid")

		return uuid.Must(uuid.NewV4()).String()
	}

	if clusterUUID, ok := secret.Data["clusterUUID"]; ok {
		return string(clusterUUID)
	}

	return uuid.Must(uuid.NewV4()).String()
}

func (r *deckhouseReleaseReconciler) createOrUpdateReconcile(ctx context.Context, dr *v1alpha1.DeckhouseRelease) (ctrl.Result, error) {
	ctx, span := otel.Tracer(controllerName).Start(ctx, "createOrUpdateReconcile")
	defer span.End()

	var res ctrl.Result

	// prepare releases
	switch dr.Status.Phase {
	// these phases should be ignored by predicate, but let's check it
	case "":
		// set current restored release as deployed
		if dr.GetCurrentRestored() {
			return res, r.proceedRestoredRelease(ctx, dr)
		}

		// initial state
		dr.Status.Phase = v1alpha1.DeckhouseReleasePhasePending
		dr.Status.TransitionTime = metav1.NewTime(r.dc.GetClock().Now().UTC())
		if err := r.client.Status().Update(ctx, dr); err != nil {
			return res, err
		}

		return ctrl.Result{Requeue: true}, nil // process to the next phase

	case v1alpha1.DeckhouseReleasePhaseSkipped, v1alpha1.DeckhouseReleasePhaseSuperseded, v1alpha1.DeckhouseReleasePhaseSuspended:
		r.logger.Debug("release phase", slog.String("phase", dr.Status.Phase))
		return res, nil

	case v1alpha1.DeckhouseReleasePhaseDeployed:
		res, err := r.reconcileDeployedRelease(ctx, dr)
		if err != nil {
			r.logger.Debug("result of reconcile deployed release",
				slog.String("release_name", dr.GetName()),
				slog.String("release_version", dr.Spec.Version),
				log.Err(err))
		}
		return res, err
	}

	// update pending release with suspend annotation
	err := r.patchSuspendAnnotation(ctx, dr)
	if err != nil {
		return res, err
	}

	err = r.patchManualRelease(ctx, dr)
	if err != nil {
		return res, err
	}

	res, err = r.pendingReleaseReconcile(ctx, dr)
	if err != nil {
		r.logger.Debug("result of reconcile pending release",
			slog.String("release_name", dr.GetName()),
			slog.String("release_version", dr.Spec.Version),
			log.Err(err))
	}
	return res, err
}

// patchManualRelease modify deckhouse release with approved status
func (r *deckhouseReleaseReconciler) patchManualRelease(ctx context.Context, dr *v1alpha1.DeckhouseRelease) error {
	if r.updateSettings.Get().Update.Mode != v1alpha2.UpdateModeManual.String() {
		return nil
	}

	patch := client.MergeFrom(dr.DeepCopy())

	dr.SetApprovedStatus(dr.GetManuallyApproved())

	err := r.client.Status().Patch(ctx, dr, patch)
	if err != nil {
		return fmt.Errorf("patch approved status: %w", err)
	}

	return nil
}

func (r *deckhouseReleaseReconciler) proceedRestoredRelease(ctx context.Context, dr *v1alpha1.DeckhouseRelease) error {
	dr.Status.Approved = true
	dr.Status.Phase = v1alpha1.DeckhouseReleasePhaseDeployed
	dr.Status.TransitionTime = metav1.NewTime(r.dc.GetClock().Now().UTC())
	dr.Status.Message = "Release object was restored"

	if err := r.client.Status().Update(ctx, dr); err != nil {
		return err
	}

	return nil
}

// patchSuspendAnnotation modify deckhouse release with suspend phase and message
// and remove suspend annotation
func (r *deckhouseReleaseReconciler) patchSuspendAnnotation(ctx context.Context, dr *v1alpha1.DeckhouseRelease) error {
	if !dr.GetSuspend() {
		return nil
	}

	patch := client.MergeFrom(dr.DeepCopy())

	dr.Status.Phase = v1alpha1.DeckhouseReleasePhaseSuspended
	dr.Status.Message = "Release is suspended"

	err := r.client.Status().Patch(ctx, dr, patch)
	if err != nil {
		return fmt.Errorf("patch suspend phase: %w", err)
	}

	delete(dr.Annotations, v1alpha1.DeckhouseReleaseAnnotationSuspended)

	err = r.client.Patch(ctx, dr, patch)
	if err != nil {
		return fmt.Errorf("patch suspend annotation: %w", err)
	}

	return nil
}

// pendingReleaseReconcile
//
// 1) Calculate task for current release
// 1.1) if skip - update phase to Skipped and stop reconcile
// 1.2) if await - update phase to Pending and requeue
// 1.3) if process - go forward
// 1.4) if forced - apply release
// 2) Apply if force with force logic
// 3) Check requirements
// 3.1) if not met any requirements - update phase to Pending with all requirements errors and requeue
// 4) Check deploy time and notify
// 5) Apply ussually release
func (r *deckhouseReleaseReconciler) pendingReleaseReconcile(ctx context.Context, dr *v1alpha1.DeckhouseRelease) (ctrl.Result, error) {
	ctx, span := otel.Tracer(controllerName).Start(ctx, "pendingReleaseReconcile")
	defer span.End()

	var res ctrl.Result

	if r.registrySecret == nil {
		// TODO: make registry service to check secrets in it (make issue)
		registrySecret, err := r.getRegistrySecret(ctx)
		if err != nil {
			return res, fmt.Errorf("get registry secret: %w", err)
		}

		r.registrySecret = registrySecret
	}

	taskCalculator := releaseUpdater.NewDeckhouseReleaseTaskCalculator(r.client, r.logger, r.updateSettings.Get().ReleaseChannel)

	task, err := taskCalculator.CalculatePendingReleaseTask(ctx, dr)
	if err != nil {
		return res, err
	}

	// Initialize release update info structure for collecting processing information
	updateInfo := &ReleaseUpdateInfo{}

	// Collect task calculation information
	updateInfo.TaskCalculation.TaskType = task.TaskType.String()
	updateInfo.TaskCalculation.IsPatch = task.IsPatch
	updateInfo.TaskCalculation.IsSingle = task.IsSingle
	updateInfo.TaskCalculation.IsLatest = task.IsLatest
	updateInfo.TaskCalculation.IsFromTo = task.IsFromTo
	updateInfo.TaskCalculation.IsMajor = task.IsMajor

	// Collect update policy information
	updateInfo.UpdatePolicy.Mode = r.updateSettings.Get().Update.Mode

	if dr.GetForce() {
		// Collect force release information
		updateInfo.ForceRelease.IsForced = true

		r.logger.Warn("forced release found")

		// deploy forced release without any checks (windows, requirements, approvals and so on)
		err := r.ApplyRelease(ctx, dr, task, updateInfo)
		if err != nil {
			return res, fmt.Errorf("apply forced release: %w", err)
		}

		// stop requeue because we restart deckhouse (deployment)
		return ctrl.Result{}, nil
	}

	switch task.TaskType {
	case releaseUpdater.Process:
		// pass
	case releaseUpdater.Skip:
		err := r.updateReleaseStatus(ctx, dr, &v1alpha1.DeckhouseReleaseStatus{
			Phase:   v1alpha1.DeckhouseReleasePhaseSkipped,
			Message: task.Message,
		})
		if err != nil {
			r.logger.Warn("skip order status update ", slog.String("name", dr.GetName()), log.Err(err))
			return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
		}

		return res, nil
	case releaseUpdater.Await:
		err := r.updateReleaseStatus(ctx, dr, &v1alpha1.DeckhouseReleaseStatus{
			Phase:   v1alpha1.DeckhouseReleasePhasePending,
			Message: task.Message,
		})
		if err != nil {
			r.logger.Warn("await order status update ", slog.String("name", dr.GetName()), log.Err(err))
		}

		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	if !r.isDeckhousePodReady(ctx) && !task.IsPatch {
		r.logger.Info("Deckhouse is not ready, waiting")

		drs := &v1alpha1.DeckhouseReleaseStatus{
			Phase: v1alpha1.DeckhouseReleasePhasePending,
		}

		if task.DeployedReleaseInfo == nil {
			r.logger.Warn("could not find deployed version, awaiting", slog.String("name", dr.GetName()))
			return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
		}

		drs.Message = fmt.Sprintf("awaiting for Deckhouse v%s pod to be ready", task.DeployedReleaseInfo.Version.String())

		updateErr := r.updateReleaseStatus(ctx, dr, drs)
		if updateErr != nil {
			r.logger.Warn("await deckhouse pod status update ", slog.String("name", dr.GetName()), log.Err(err))
		}

		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	// Collect pod readiness information - if we reached here, pod is ready
	updateInfo.PodReadiness.IsReady = true

	checker, err := releaseUpdater.NewDeckhouseReleaseRequirementsChecker(r.client, r.moduleManager.GetEnabledModuleNames(), r.exts, r.metricStorage, releaseUpdater.WithLogger(r.logger))
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

	metricLabels := releaseUpdater.NewReleaseMetricLabels(dr)
	defer func() {
		metricLabels[releaseUpdater.MajorReleaseDepth] = strconv.Itoa(task.QueueDepth.GetMajorReleaseDepth())
		if metricLabels[releaseUpdater.ManualApprovalRequired] == "true" {
			metricLabels[releaseUpdater.ReleaseQueueDepth] = strconv.Itoa(task.QueueDepth.GetReleaseQueueDepth())
		}
		r.metricsUpdater.UpdateReleaseMetric(dr.GetName(), metricLabels)
	}()

	reasons := checker.MetRequirements(ctx, dr)
	if len(reasons) > 0 {
		metricLabels.SetTrue(releaseUpdater.RequirementsNotMet)
		msgs := make([]string, 0, len(reasons))
		for _, reason := range reasons {
			msgs = append(msgs, reason.Message)
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

	// Collect requirements check information - requirements are met
	updateInfo.RequirementsCheck.RequirementsMet = true

	// handling error inside function
	// we do not pass update info, because if we have an error - we can't apply release and set update info
	err = r.PreApplyReleaseCheck(ctx, dr, task, metricLabels)
	if err != nil {
		// ignore this err, just requeue because of check failed
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	err = r.ApplyRelease(ctx, dr, task, updateInfo)
	if err != nil {
		return res, fmt.Errorf("apply predicted release: %w", err)
	}

	return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
}

var ErrPreApplyCheckIsFailed = errors.New("pre apply check is failed")

// PreApplyReleaseCheck checks final conditions before apply
//
// - Calculating deploy time (if zero - deploy)
func (r *deckhouseReleaseReconciler) PreApplyReleaseCheck(ctx context.Context, dr *v1alpha1.DeckhouseRelease, task *releaseUpdater.Task, metricLabels releaseUpdater.MetricLabels) error {
	ctx, span := otel.Tracer(controllerName).Start(ctx, "preApplyReleaseCheck")
	defer span.End()

	timeResult := r.DeployTimeCalculate(ctx, dr, task, metricLabels)

	if timeResult == nil {
		// No delay, ready to deploy immediately
		return nil
	}

	err := r.updateReleaseStatus(ctx, dr, &v1alpha1.DeckhouseReleaseStatus{
		Phase:   v1alpha1.DeckhouseReleasePhasePending,
		Message: timeResult.Message,
	})
	if err != nil {
		r.logger.Warn("met release conditions status update ", slog.String("name", dr.GetName()), log.Err(err))
	}

	err = ctrlutils.UpdateWithRetry(ctx, r.client, dr, func() error {
		if len(dr.Annotations) == 0 {
			dr.Annotations = make(map[string]string, 2)
		}

		dr.Annotations[v1alpha1.DeckhouseReleaseAnnotationIsUpdating] = "false"
		dr.Annotations[v1alpha1.DeckhouseReleaseAnnotationNotified] = strconv.FormatBool(timeResult.Notified)

		if !timeResult.ReleaseApplyAfterTime.IsZero() {
			dr.Spec.ApplyAfter = &metav1.Time{Time: timeResult.ReleaseApplyAfterTime.UTC()}

			dr.Annotations[v1alpha1.DeckhouseReleaseAnnotationNotificationTimeShift] = "true"
		}

		return nil
	})
	if err != nil {
		r.logger.Warn("met release conditions resource update ", slog.String("name", dr.GetName()), log.Err(err))
	}

	return ErrPreApplyCheckIsFailed
}

const (
	msgReleaseIsBlockedByNotification = "Release is blocked, failed to send release notification"
)

type TimeResult struct {
	*releaseUpdater.ProcessedDeployTimeResult
	Notified bool
}

// DeployTimeCalculate calculate time for release deploy
//
// If patch, calculate by checking this conditions:
// - Canary
// - Notify
// - Window
// - ManualApproved
//
// If minor, calculate by checking this conditions:
// - Cooldown
// - Canary
// - Notify
// - Window
// - Manual Approved
func (r *deckhouseReleaseReconciler) DeployTimeCalculate(ctx context.Context, dr v1alpha1.Release, task *releaseUpdater.Task, metricLabels releaseUpdater.MetricLabels) *TimeResult {
	us := r.updateSettings.Get()

	dus := &releaseUpdater.Settings{
		NotificationConfig:     us.Update.NotificationConfig,
		DisruptionApprovalMode: us.Update.DisruptionApprovalMode,
		// if we have wrong mode - autopatch
		Mode:    v1alpha2.ParseUpdateMode(us.Update.Mode),
		Windows: us.Update.Windows,
		Subject: releaseUpdater.SubjectDeckhouse,
	}

	releaseNotifier := releaseUpdater.NewReleaseNotifier(dus)
	timeChecker := releaseUpdater.NewDeployTimeService(r.dc, dus, r.logger)

	var deployTimeResult *releaseUpdater.DeployTimeResult

	if task.IsPatch {
		deployTimeResult = timeChecker.CalculatePatchDeployTime(dr, metricLabels)

		notifyErr := releaseNotifier.SendPatchReleaseNotification(ctx, dr, deployTimeResult.ReleaseApplyAfterTime, metricLabels)
		if notifyErr != nil {
			r.logger.Warn("send [patch] release notification", log.Err(notifyErr))

			message := fmt.Sprintf("%s: %s", msgReleaseIsBlockedByNotification, notifyErr.Error())

			return &TimeResult{
				ProcessedDeployTimeResult: &releaseUpdater.ProcessedDeployTimeResult{
					Message:               message,
					ReleaseApplyAfterTime: deployTimeResult.ReleaseApplyAfterTime,
				},
			}
		}

		processedDTR := timeChecker.ProcessPatchReleaseDeployTime(dr, deployTimeResult)
		if processedDTR == nil {
			return nil
		}

		return &TimeResult{
			ProcessedDeployTimeResult: processedDTR,
			Notified:                  true,
		}
	}

	// for minor release we must check additional conditions
	checker := releaseUpdater.NewPreApplyChecker(dus, r.logger)
	reasons := checker.MetRequirements(ctx, &dr)
	if len(reasons) > 0 {
		metricLabels.SetTrue(releaseUpdater.DisruptionApprovalRequired)

		msgs := make([]string, 0, len(reasons))
		for _, reason := range reasons {
			msgs = append(msgs, reason.Message)
		}

		return &TimeResult{
			ProcessedDeployTimeResult: &releaseUpdater.ProcessedDeployTimeResult{
				Message: fmt.Sprintf("release blocked, disruption approval required: %s", strings.Join(msgs, ", ")),
			},
		}
	}

	deployTimeResult = timeChecker.CalculateMinorDeployTime(dr, metricLabels)

	notifyErr := releaseNotifier.SendMinorReleaseNotification(ctx, dr, deployTimeResult.ReleaseApplyTime, metricLabels)
	if notifyErr != nil {
		r.logger.Warn("send minor release notification", log.Err(notifyErr))

		message := fmt.Sprintf("%s: %s", msgReleaseIsBlockedByNotification, notifyErr.Error())

		return &TimeResult{
			ProcessedDeployTimeResult: &releaseUpdater.ProcessedDeployTimeResult{
				Message:               message,
				ReleaseApplyAfterTime: deployTimeResult.ReleaseApplyAfterTime,
			},
		}
	}

	processedDTR := timeChecker.ProcessMinorReleaseDeployTime(dr, deployTimeResult)
	if processedDTR == nil {
		return nil
	}

	return &TimeResult{
		ProcessedDeployTimeResult: processedDTR,
		Notified:                  true,
	}
}

// ApplyRelease applies predicted release
func (r *deckhouseReleaseReconciler) ApplyRelease(ctx context.Context, dr *v1alpha1.DeckhouseRelease, task *releaseUpdater.Task, updateInfo *ReleaseUpdateInfo) error {
	ctx, span := otel.Tracer(controllerName).Start(ctx, "applyRelease")
	defer span.End()

	var dri *releaseUpdater.ReleaseInfo

	if task != nil {
		dri = task.DeployedReleaseInfo
	}

	err := r.runReleaseDeploy(ctx, dr, dri, updateInfo)
	if err != nil {
		return fmt.Errorf("run release deploy: %w", err)
	}

	return nil
}

// runReleaseDeploy
//
// 1) bump deckhouse deployment (retry if error) if the dryrun annotations isn't set (stop deploying in the opposite case)
// 2) bump previous deployment status superseded (retry if error)
// 3) bump release annotations (retry if error)
// 3) bump release status to deployed (retry if error)
func (r *deckhouseReleaseReconciler) runReleaseDeploy(ctx context.Context, dr *v1alpha1.DeckhouseRelease, deployedReleaseInfo *releaseUpdater.ReleaseInfo, updateInfo *ReleaseUpdateInfo) error {
	r.logger.Info("applying release", slog.String("name", dr.GetName()))

	// Record event about release update process
	r.recordReleaseUpdateEvent(dr, updateInfo)

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
			v1alpha1.DeckhouseReleaseAnnotationIsUpdating: "true",
			v1alpha1.DeckhouseReleaseAnnotationNotified:   "false",
		}

		// Serialize update info to JSON and add to annotations
		if updateInfo != nil {
			updateInfoJSON, jsonErr := json.Marshal(updateInfo)
			if jsonErr != nil {
				r.logger.Warn("failed to marshal update info to JSON", log.Err(jsonErr))
			} else {
				annotations[v1alpha1.DeckhouseReleaseAnnotationUpdateInfo] = string(updateInfoJSON)
			}
		}

		if len(dr.Annotations) == 0 {
			dr.Annotations = make(map[string]string, len(annotations))
		}

		for k, v := range annotations {
			dr.Annotations[k] = v
		}

		if dr.GetApplyNow() {
			delete(dr.Annotations, v1alpha1.DeckhouseReleaseAnnotationApplyNow)
		}

		if dr.GetForce() {
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

	return nil
}

var ErrDeploymentContainerIsNotFound = errors.New("deployment container is not found")

func (r *deckhouseReleaseReconciler) bumpDeckhouseDeployment(ctx context.Context, dr *v1alpha1.DeckhouseRelease) error {
	key := client.ObjectKey{Namespace: deckhouseNamespace, Name: deckhouseDeployment}

	depl := new(appsv1.Deployment)

	err := r.client.Get(ctx, key, depl)
	if err != nil {
		return fmt.Errorf("get deployment %s: %w", key, err)
	}

	// dryrun for testing purpose
	if dr.GetDryRun() {
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

				if release.Status.Phase != v1alpha1.DeckhouseReleasePhasePending {
					continue
				}

				// update releases to trigger their requeue
				err := ctrlutils.UpdateWithRetry(ctxwt, r.client, release, func() error {
					if len(release.Annotations) == 0 {
						release.Annotations = make(map[string]string, 1)
					}

					release.Annotations[v1alpha1.DeckhouseReleaseAnnotationTriggeredByDryrun] = dr.GetName()

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

	patch := client.MergeFrom(depl.DeepCopy())

	if len(depl.Spec.Template.Spec.Containers) == 0 {
		return ErrDeploymentContainerIsNotFound
	}
	depl.Spec.Template.Spec.Containers[0].Image = r.registrySecret.ImageRegistry + ":" + dr.Spec.Version

	err = r.client.Patch(ctx, depl, patch)
	if err != nil {
		return fmt.Errorf("patch deployment %s: %w", depl.Name, err)
	}

	return nil
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
		r.logger.Warn("pod does not contain a deckhouse container", slog.String("pod", leaderPod.GetName()))
		return nil
	}

	image := leaderPod.Spec.Containers[deckhouseContainerIndex].Image
	imageID := leaderPod.Status.ContainerStatuses[deckhouseContainerStatusIndex].ImageID

	if image == "" || imageID == "" {
		// pod is restarting or something like that, try more in 15 seconds
		return nil
	}

	if deckhouseContainerStatusIndex == -1 {
		r.logger.Warn("pod does not contain a deckhouse container status", slog.String("pod", leaderPod.GetName()))
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

	if r.registrySecret == nil {
		// TODO: make registry service to check secrets in it (make issue)
		registrySecret, err := r.getRegistrySecret(ctx)
		if client.IgnoreNotFound(err) != nil {
			return err
		}

		r.registrySecret = registrySecret
	}

	var opts []cr.Option
	if r.registrySecret != nil {
		rconf := &utils.RegistryConfig{
			DockerConfig: r.registrySecret.DockerConfig,
			Scheme:       r.registrySecret.Scheme,
			CA:           r.registrySecret.CA,
			UserAgent:    r.clusterUUID,
		}
		opts = utils.GenerateRegistryOptions(rconf, r.logger)
	}

	regClient, err := r.dc.GetRegistryClient(repo, opts...)
	if err != nil {
		return fmt.Errorf("registry (%s) client init failed: %s", repo, err)
	}

	r.metricStorage.CounterAdd(metrics.DeckhouseRegistryCheckTotal, 1, map[string]string{})
	r.metricStorage.CounterAdd(metrics.DeckhouseKubeImageDigestCheckTotal, 1, map[string]string{})

	repoDigest, err := regClient.Digest(ctx, tag)
	if err != nil {
		r.metricStorage.CounterAdd(metrics.DeckhouseRegistryCheckErrorsTotal, 1, map[string]string{})
		return fmt.Errorf("registry (%s) get digest failed: %s", repo, err)
	}

	r.metricStorage.CounterAdd(metrics.DeckhouseKubeImageDigestCheckSuccess, 1.0, map[string]string{})

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
	ctx, span := otel.Tracer(controllerName).Start(ctx, "getRegistrySecret")
	defer span.End()

	key := types.NamespacedName{Namespace: deckhouseNamespace, Name: deckhouseRegistrySecretName}

	secret := new(corev1.Secret)

	err := r.client.Get(ctx, key, secret)
	if err != nil {
		return nil, fmt.Errorf("get secret %s: %w", key, err)
	}

	regSecret, err := utils.ParseDeckhouseRegistrySecret(secret.Data)
	if errors.Is(err, utils.ErrImageRegistryFieldIsNotFound) {
		regSecret.ImageRegistry = regSecret.Address + regSecret.Path
	}

	return regSecret, nil
}

func (r *deckhouseReleaseReconciler) isDeckhousePodReady(ctx context.Context) bool {
	deckhousePodIP := aoapp.ListenAddress

	url := fmt.Sprintf("http://%s:4222/readyz", deckhousePodIP)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		r.logger.Error("error getting deckhouse pod readyz", log.Err(err))
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
			r.logger.Warn("getting deckhouse pods", log.Err(err))
			return
		}

		if deckhouseLeaderPod == nil {
			r.logger.Debug("deckhouse pods not found. Skipping update")
			return
		}

		err = r.tagUpdate(ctx, deckhouseLeaderPod)
		if err != nil {
			r.logger.Error("deckhouse image tag update", log.Err(err))
		}
	}, 15*time.Second)
}

func (r *deckhouseReleaseReconciler) reconcileDeployedRelease(ctx context.Context, dr *v1alpha1.DeckhouseRelease) (ctrl.Result, error) {
	ctx, span := otel.Tracer(controllerName).Start(ctx, "deployedReleaseReconcile")
	defer span.End()

	var res ctrl.Result

	if r.isDeckhousePodReady(ctx) {
		err := ctrlutils.UpdateWithRetry(ctx, r.client, dr, func() error {
			if len(dr.Annotations) == 0 {
				dr.Annotations = make(map[string]string, 2)
			}

			dr.Annotations[v1alpha1.DeckhouseReleaseAnnotationIsUpdating] = "false"
			dr.Annotations[v1alpha1.DeckhouseReleaseAnnotationNotified] = "true"
			r.metricStorage.Grouped().ExpireGroupMetrics(metrics.D8Updating)

			return nil
		})
		if err != nil {
			return res, err
		}

		return res, nil
	}

	if dr.Status.Message != "" {
		err := ctrlutils.UpdateStatusWithRetry(ctx, r.client, dr, func() error {
			dr.Status.Message = ""
			return nil
		})
		if err != nil {
			return res, err
		}
	}

	if dr.GetIsUpdating() {
		r.metricStorage.Grouped().GaugeSet(metrics.D8Updating, metrics.D8IsUpdating, 1, map[string]string{"deployingRelease": dr.GetName()})

		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	return res, nil
}

func (r *deckhouseReleaseReconciler) updateReleaseStatus(ctx context.Context, dr *v1alpha1.DeckhouseRelease, status *v1alpha1.DeckhouseReleaseStatus) error {
	r.logger.Debug("refresh release status", slog.String("name", dr.GetName()))

	switch status.Phase {
	case v1alpha1.DeckhouseReleasePhaseSuperseded, v1alpha1.DeckhouseReleasePhaseSuspended, v1alpha1.DeckhouseReleasePhaseSkipped:
		r.metricsUpdater.PurgeReleaseMetric(dr.GetName())
	}

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

func newDeckhouseReleaseWithName(name string) *v1alpha1.DeckhouseRelease {
	return &v1alpha1.DeckhouseRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// recordReleaseUpdateEvent records a Kubernetes event for release update process
func (r *deckhouseReleaseReconciler) recordReleaseUpdateEvent(release *v1alpha1.DeckhouseRelease, updateInfo *ReleaseUpdateInfo) {
	if updateInfo == nil || r.eventRecorder == nil {
		return
	}

	r.eventRecorder.Eventf(release, corev1.EventTypeNormal, "ReleaseUpdateInitiated",
		"Release update initiated: task=%s, updateMode=%s, force=%t, podReady=%t, requirementsMet=%t",
		updateInfo.TaskCalculation.TaskType,
		updateInfo.UpdatePolicy.Mode,
		updateInfo.ForceRelease.IsForced,
		updateInfo.PodReadiness.IsReady,
		updateInfo.RequirementsCheck.RequirementsMet)
}
