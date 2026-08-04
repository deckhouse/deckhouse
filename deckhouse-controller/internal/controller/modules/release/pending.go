// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package release

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"go.opentelemetry.io/otel"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/ctrlutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	releaseUpdater "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/releaseupdater"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// handlePendingRelease decides whether a pending release may deploy now: it resolves the
// update policy, asks the task calculator for a verdict, then gates on module readiness,
// requirements and the deploy-time checks before applying. A forced release skips every gate.
func (r *reconciler) handlePendingRelease(ctx context.Context, mr *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	ctx, span := otel.Tracer(controllerName).Start(ctx, "handlePendingRelease")
	defer span.End()

	var res ctrl.Result

	logger := r.releaseLogger(mr)
	logger.Debug("handle pending release")

	policy, policyRes, err := r.resolvePolicy(ctx, mr, logger)
	if err != nil {
		return res, err
	}

	if policyRes != nil {
		return *policyRes, nil
	}

	// parse notification config from the deckhouse-discovery secret
	config, err := utils.GetNotificationConfig(ctx, r.client)
	if err != nil {
		logger.Error("failed to parse the notification config", log.Err(err))

		return res, fmt.Errorf("get notification config: %w", err)
	}

	us := &releaseUpdater.Settings{
		NotificationConfig: config,
		Mode:               v1alpha2.ParseUpdateMode(policy.Spec.Update.Mode),
		Windows:            policy.Spec.Update.Windows,
		Subject:            releaseUpdater.SubjectModule,
	}

	if err = r.patchManualRelease(ctx, mr, us); err != nil {
		return res, err
	}

	taskCalculator := releaseUpdater.NewModuleReleaseTaskCalculator(r.client, policy.Spec.ReleaseChannel, logger)

	task, err := taskCalculator.CalculatePendingReleaseTask(ctx, mr)
	if err != nil {
		return res, fmt.Errorf("calculate pending release task: %w", err)
	}

	if mr.GetForce() {
		logger.Warn("forced release found")

		// deploy forced release without any checks (windows, requirements, approvals and so on)
		if err = r.applyRelease(ctx, mr, task); err != nil {
			logger.Error("apply forced release", log.Err(err))

			return res, fmt.Errorf("apply forced release: %w", err)
		}

		r.logger.Info("a new module release deployed", slog.String("module", mr.GetModuleName()))

		return ctrl.Result{}, nil
	}

	switch task.TaskType {
	case releaseUpdater.Process:
		// pass
	case releaseUpdater.Skip:
		logger.Debug("skip pending release")

		err = r.updateReleaseStatus(ctx, mr, &v1alpha1.ModuleReleaseStatus{
			Phase:   v1alpha1.ModuleReleasePhaseSkipped,
			Message: task.Message,
		})
		if err != nil {
			logger.Warn("skip order status update ", slog.String("release", mr.GetName()), log.Err(err))
			return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
		}

		return res, nil
	case releaseUpdater.Await:
		logger.Debug("await pending release")

		err = r.updateReleaseStatus(ctx, mr, &v1alpha1.ModuleReleaseStatus{
			Phase:   v1alpha1.ModuleReleasePhasePending,
			Message: task.Message,
		})
		if err != nil {
			logger.Warn("await order status update ", log.Err(err))
		}

		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	// a single or patch release is safe to lay down onto a module that is not settled yet,
	// anything larger waits for the currently deployed version to become ready
	if !task.IsSingle && !task.IsPatch && !r.isModuleReady(ctx, mr.GetModuleName()) {
		logger.Debug("module is not ready, waiting")

		message := "awaiting for module to be ready"
		if task.DeployedReleaseInfo != nil {
			message = fmt.Sprintf("awaiting for module v%s to be ready", task.DeployedReleaseInfo.Version.String())
		}

		if updateErr := r.updateReleaseStatus(ctx, mr, &v1alpha1.ModuleReleaseStatus{
			Phase:   v1alpha1.ModuleReleasePhasePending,
			Message: message,
		}); updateErr != nil {
			logger.Warn("module release status update failed", log.Err(updateErr))
		}

		err = r.updateModuleLastReleaseDeployedStatus(ctx, mr, "ModuleRelease could not be applied, awaiting for deployed release be ready", "ReleaseDeployedIsNotReady", false)
		if err != nil {
			return res, fmt.Errorf("update module last release deployed status: %w", err)
		}

		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	logger.Debug("process pending release")

	metricLabels := releaseUpdater.NewReleaseMetricLabels(mr)
	defer func() {
		metricLabels[metrics.LabelMajorReleaseDepth] = strconv.Itoa(task.QueueDepth.GetMajorReleaseDepth())
		if task.IsMajor {
			metricLabels[metrics.LabelMajorReleaseName] = mr.GetName()
		}

		if task.IsFromTo {
			metricLabels[metrics.LabelFromToName] = mr.GetName()
		}

		if metricLabels[metrics.LabelManualApprovalRequired] == "true" {
			metricLabels[metrics.LabelReleaseQueueDepth] = strconv.Itoa(task.QueueDepth.GetReleaseQueueDepth())
		}

		r.metricsUpdater.UpdateReleaseMetric(mr.GetName(), metricLabels)
	}()

	// handling error inside function
	if err = r.checkRequirements(ctx, mr, metricLabels, logger); err != nil {
		// ignore this err, just requeue because of check failed
		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	logger.Debug("requirements checks passed")

	// handling error inside function
	if err = r.preApplyReleaseCheck(ctx, mr, task, us, metricLabels); err != nil {
		// ignore this err, just requeue because of check failed
		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	logger.Debug("pre apply checks passed")

	if err = r.applyRelease(ctx, mr, task); err != nil {
		return res, fmt.Errorf("apply predicted release: %w", err)
	}

	// no deckhouse restart if dryrun
	if mr.GetDryRun() {
		return ctrl.Result{}, nil
	}

	r.logger.Info("a new module release deployed", slog.String("module", mr.GetModuleName()))

	logger.Debug("module release deployed")

	return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
}

// resolvePolicy returns the update policy governing the release. A non-nil result means the
// caller should return it instead of continuing: the policy is missing, or it ignores updates.
func (r *reconciler) resolvePolicy(ctx context.Context, mr *v1alpha1.ModuleRelease, logger *log.Logger) (*v1alpha2.ModuleUpdatePolicy, *ctrl.Result, error) {
	// if the release has associated update policy and it's not empty - just get it
	// otherwise, try to get it from the module
	policyName, found := mr.GetObjectMeta().GetLabels()[v1alpha1.ModuleReleaseLabelUpdatePolicy]
	if !found || policyName == "" {
		return r.updatePolicy(ctx, mr)
	}

	policy, err := r.getUpdatePolicy(ctx, policyName)
	if err != nil {
		r.metricStorage.CounterAdd(metrics.ModuleUpdatePolicyNotFound, 1.0, map[string]string{
			metrics.LabelVersion:       mr.GetReleaseVersion(),
			metrics.LabelModuleRelease: mr.GetName(),
			metrics.LabelModule:        mr.GetModuleName(),
		})

		if uerr := r.updateReleaseStatusMessage(ctx, mr, fmt.Sprintf("Update policy %s not found", policyName)); uerr != nil {
			logger.Error("failed to update release status", log.Err(uerr))

			return nil, nil, uerr
		}

		logger.Error("failed to get update policy", slog.String("policy", policyName), log.Err(err))

		return nil, &ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	// TODO(ipaqsa): remove it
	if policy.Spec.Update.Mode == v1alpha2.ModuleUpdatePolicyModeIgnore {
		if uerr := r.updateReleaseStatusMessage(ctx, mr, disabledByIgnorePolicy); uerr != nil {
			logger.Error("failed to update release status", slog.String("release", mr.GetName()), log.Err(uerr))

			return nil, nil, uerr
		}

		return nil, &ctrl.Result{RequeueAfter: defaultRequeueAfter * 4}, nil
	}

	return policy, nil, nil
}

// getUpdatePolicy reads the named policy, falling back to the embedded one when the name is
// empty.
func (r *reconciler) getUpdatePolicy(ctx context.Context, name string) (*v1alpha2.ModuleUpdatePolicy, error) {
	if name == "" {
		return &v1alpha2.ModuleUpdatePolicy{
			TypeMeta: metav1.TypeMeta{
				Kind:       v1alpha2.ModuleUpdatePolicyGVK.Kind,
				APIVersion: v1alpha2.ModuleUpdatePolicyGVK.GroupVersion().String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "",
			},
			Spec: *r.embeddedPolicy.Get(),
		}, nil
	}

	policy := new(v1alpha2.ModuleUpdatePolicy)
	if err := r.client.Get(ctx, client.ObjectKey{Name: name}, policy); err != nil {
		return nil, fmt.Errorf("get update policy: %w", err)
	}

	return policy, nil
}

// updatePolicy discovers the policy that matches the module and stamps its name onto the
// release, so later reconciles resolve it by label instead of rediscovering it.
func (r *reconciler) updatePolicy(ctx context.Context, mr *v1alpha1.ModuleRelease) (*v1alpha2.ModuleUpdatePolicy, *ctrl.Result, error) {
	policy, err := utils.GetUpdatePolicyByModule(ctx, r.client, r.embeddedPolicy, mr.GetModuleName())
	if err != nil {
		r.logger.Error("failed to get update policy", slog.String("release", mr.GetName()), log.Err(err))

		if uerr := r.updateReleaseStatusMessage(ctx, mr, "Update policy not set. Create a suitable ModuleUpdatePolicy object"); uerr != nil {
			r.logger.Error("failed to update release status", slog.String("release", mr.GetName()), log.Err(uerr))

			return nil, nil, uerr
		}

		return nil, &ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	marshalledPatch, _ := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"labels": map[string]any{
				v1alpha1.ModuleReleaseLabelUpdatePolicy: policy.GetName(),
			},
		},
		"status": map[string]string{
			"message": "",
		},
	})

	patch := client.RawPatch(types.MergePatchType, marshalledPatch)
	if err = r.client.Patch(ctx, mr, patch); err != nil {
		r.logger.Error("failed to patch module release", slog.String("release", mr.GetName()), log.Err(err))

		return nil, &ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	// also patch status field
	if err = r.client.Status().Patch(ctx, mr, patch); err != nil {
		r.logger.Error("failed to patch module release status", slog.String("release", mr.GetName()), log.Err(err))

		return nil, &ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	return policy, nil, nil
}

// patchManualRelease mirrors the release's manual approval into its status, so the task
// calculator sees the approval an operator granted through the annotation.
func (r *reconciler) patchManualRelease(ctx context.Context, mr *v1alpha1.ModuleRelease, us *releaseUpdater.Settings) error {
	if us.Mode.String() != v1alpha2.UpdateModeManual.String() {
		return nil
	}

	patch := client.MergeFrom(mr.DeepCopy())

	mr.SetApprovedStatus(mr.GetManuallyApproved())

	if err := r.client.Status().Patch(ctx, mr, patch); err != nil {
		return fmt.Errorf("patch approved status: %w", err)
	}

	return nil
}

// errRequirementsNotMet reports that a release may not deploy yet. It is a control signal, not a
// failure: the caller requeues on it rather than surfacing it.
var errRequirementsNotMet = errors.New("release requirements are not met")

// checkRequirements asks the scheduler whether the release's declared requirements hold against
// the live cluster, and records the verdict on both the release and its module. It returns
// errRequirementsNotMet when the release has to wait.
func (r *reconciler) checkRequirements(ctx context.Context, mr *v1alpha1.ModuleRelease, metricLabels releaseUpdater.MetricLabels, logger *log.Logger) error {
	constraints, err := releaseConstraints(mr)
	if err != nil {
		// requirements that cannot be parsed will not start parsing on a retry, but the release
		// still has to report why it is stuck
		logger.Error("failed to parse release requirements", log.Err(err))

		return r.reportRequirementsNotMet(ctx, mr, metricLabels, err, logger)
	}

	if err = r.manager.CheckConstraints(mr.GetModuleName(), constraints); err != nil {
		logger.Debug("release requirements are not met", log.Err(err))

		return r.reportRequirementsNotMet(ctx, mr, metricLabels, err, logger)
	}

	return nil
}

// reportRequirementsNotMet parks the release in Pending with the reason and marks the module's
// last-release-deployed condition false.
func (r *reconciler) reportRequirementsNotMet(ctx context.Context, mr *v1alpha1.ModuleRelease, metricLabels releaseUpdater.MetricLabels, reason error, logger *log.Logger) error {
	metricLabels.SetTrue(metrics.LabelRequirementsNotMet)

	if err := r.updateReleaseStatus(ctx, mr, &v1alpha1.ModuleReleaseStatus{
		Phase:   v1alpha1.ModuleReleasePhasePending,
		Message: reason.Error(),
	}); err != nil {
		logger.Warn("met requirements status update ", log.Err(err))
	}

	if err := r.updateModuleLastReleaseDeployedStatus(ctx, mr, "ModuleRelease could not be applied, not met requirements", "ReleaseRequirementsCheck", false); err != nil {
		return fmt.Errorf("update module last release deployed status: %w", err)
	}

	return errRequirementsNotMet
}

// releaseConstraints maps the release's declared requirements onto the scheduler's constraint
// shape. Licensing, subscriptions and the enablement floor stay unset: admission evaluates only
// the gate rules, and the floor is intent rather than a requirement.
func releaseConstraints(mr *v1alpha1.ModuleRelease) (schedule.Constraints, error) {
	// Order decides whether the bootstrap gate applies, so it has to match what
	// modules.Definition builds for the same module or admission checks a different set of rules
	// than the scheduler will.
	order := schedule.Order(mr.GetWeight())
	if order == 0 {
		order = schedule.FunctionalOrder
	}

	constraints := schedule.Constraints{Order: order}

	requirements := mr.GetModuleReleaseRequirements()
	if requirements == nil {
		return constraints, nil
	}

	kubernetes, err := parseConstraint(requirements.Kubernetes)
	if err != nil {
		return constraints, fmt.Errorf("parse kubernetes requirement: %w", err)
	}

	deckhouse, err := parseConstraint(requirements.Deckhouse)
	if err != nil {
		return constraints, fmt.Errorf("parse deckhouse requirement: %w", err)
	}

	constraints.Kubernetes = kubernetes
	constraints.Deckhouse = deckhouse

	if len(requirements.ParentModules) == 0 {
		return constraints, nil
	}

	// parent modules are mandatory: the release declares them so they are enabled, so none of
	// them is Optional
	dependencies := make(map[string]schedule.Dependency, len(requirements.ParentModules))
	for name, raw := range requirements.ParentModules {
		constraint, cerr := parseConstraint(raw)
		if cerr != nil {
			return constraints, fmt.Errorf("parse the '%s' module requirement: %w", name, cerr)
		}

		dependencies[name] = schedule.Dependency{Constraint: constraint}
	}

	constraints.Dependencies = dependencies

	return constraints, nil
}

// parseConstraint parses a semver constraint, reading an empty string as "no constraint".
func parseConstraint(raw string) (*semver.Constraints, error) {
	if raw == "" {
		return nil, nil
	}

	return semver.NewConstraint(raw)
}

// msgReleaseIsBlockedByNotification is the status message of a release held back because its
// notification could not be delivered.
const msgReleaseIsBlockedByNotification = "Release is blocked, failed to send release notification"

// errPreApplyCheckFailed reports that a release may not deploy yet. It is a control signal, not
// a failure: the caller requeues on it rather than surfacing it.
var errPreApplyCheckFailed = errors.New("pre apply check is failed")

// timeResult is a deploy verdict plus whether the operator has already been notified about it.
type timeResult struct {
	*releaseUpdater.ProcessedDeployTimeResult
	Notified bool
}

// preApplyReleaseCheck reports whether the release may deploy now. It returns nil when it may,
// and errPreApplyCheckFailed after recording the reason and rescheduling the release.
func (r *reconciler) preApplyReleaseCheck(ctx context.Context, mr *v1alpha1.ModuleRelease, task *releaseUpdater.Task, us *releaseUpdater.Settings, metricLabels releaseUpdater.MetricLabels) error {
	ctx, span := otel.Tracer(controllerName).Start(ctx, "preApplyReleaseCheck")
	defer span.End()

	result := r.deployTimeCalculate(ctx, mr, task, us, metricLabels)
	if result == nil {
		return nil
	}

	err := r.updateReleaseStatus(ctx, mr, &v1alpha1.ModuleReleaseStatus{
		Phase:   v1alpha1.ModuleReleasePhasePending,
		Message: result.Message,
	})
	if err != nil {
		r.logger.Warn("met release conditions status update ", slog.String("release", mr.GetName()), log.Err(err))
	}

	err = r.updateModuleLastReleaseDeployedStatus(ctx, mr, "ModuleRelease could not be applied, release postponed", "ReleaseDeployTimeCheck", false)
	if err != nil {
		return fmt.Errorf("update module last release deployed status: %w", err)
	}

	err = ctrlutils.UpdateWithRetry(ctx, r.client, mr, func() error {
		if len(mr.Annotations) == 0 {
			mr.Annotations = make(map[string]string, 2)
		}

		mr.Annotations[v1alpha1.ModuleReleaseAnnotationIsUpdating] = "false"
		mr.Annotations[v1alpha1.ModuleReleaseAnnotationNotified] = strconv.FormatBool(result.Notified)

		// a shifted apply time is the notification's promise to the operator, so record that the
		// release now waits on a time it did not originally carry
		if !result.ReleaseApplyAfterTime.IsZero() {
			mr.Spec.ApplyAfter = &metav1.Time{Time: result.ReleaseApplyAfterTime.UTC()}

			mr.Annotations[v1alpha1.ModuleReleaseAnnotationNotificationTimeShift] = "true"
		}

		return nil
	}, ctrlutils.WithRetryOnConflictBackoff(retryBackoff()))
	if err != nil {
		r.logger.Warn("met release conditions resource update ", slog.String("release", mr.GetName()), log.Err(err))
	}

	return errPreApplyCheckFailed
}

// deployTimeCalculate reports why a release may not deploy yet, or nil when it may. A patch
// release only weighs canary, notification and windows; anything larger additionally needs
// disruption approval. A notification that cannot be delivered blocks the deploy.
func (r *reconciler) deployTimeCalculate(ctx context.Context, mr v1alpha1.Release, task *releaseUpdater.Task, us *releaseUpdater.Settings, metricLabels releaseUpdater.MetricLabels) *timeResult {
	releaseNotifier := releaseUpdater.NewReleaseNotifier(us)
	timeChecker := releaseUpdater.NewDeployTimeServiceAt(time.Now().UTC(), us, r.logger)

	if task.IsPatch {
		deployTime := timeChecker.CalculatePatchDeployTime(mr, metricLabels)

		if err := releaseNotifier.SendPatchReleaseNotification(ctx, mr, deployTime.ReleaseApplyTime, metricLabels); err != nil {
			r.logger.Warn("send [patch] release notification", log.Err(err))

			return blockedByNotification(deployTime)
		}

		processed := timeChecker.ProcessPatchReleaseDeployTime(mr, deployTime)
		if processed == nil {
			return nil
		}

		return &timeResult{ProcessedDeployTimeResult: processed, Notified: true}
	}

	// for minor release we must check additional conditions
	checker := releaseUpdater.NewPreApplyChecker(us, r.logger)
	if reasons := checker.MetRequirements(ctx, &mr); len(reasons) > 0 {
		metricLabels.SetTrue(metrics.LabelDisruptionApprovalRequired)

		msgs := make([]string, 0, len(reasons))
		for _, reason := range reasons {
			msgs = append(msgs, reason.Message)
		}

		return &timeResult{
			ProcessedDeployTimeResult: &releaseUpdater.ProcessedDeployTimeResult{
				Message: fmt.Sprintf("release blocked, disruption approval required: %s", strings.Join(msgs, ", ")),
			},
		}
	}

	deployTime := timeChecker.CalculateMinorDeployTime(mr, metricLabels)

	if err := releaseNotifier.SendMinorReleaseNotification(ctx, mr, deployTime.ReleaseApplyTime, metricLabels); err != nil {
		r.logger.Warn("send minor release notification", log.Err(err))

		return blockedByNotification(deployTime)
	}

	processed := timeChecker.ProcessMinorReleaseDeployTime(mr, deployTime)
	if processed == nil {
		return nil
	}

	return &timeResult{ProcessedDeployTimeResult: processed, Notified: true}
}

// blockedByNotification builds the verdict for a release whose notification failed, carrying
// the apply time forward so the release is retried rather than deployed unannounced.
func blockedByNotification(deployTime *releaseUpdater.DeployTimeResult) *timeResult {
	return &timeResult{
		ProcessedDeployTimeResult: &releaseUpdater.ProcessedDeployTimeResult{
			Message:               msgReleaseIsBlockedByNotification,
			ReleaseApplyAfterTime: deployTime.ReleaseApplyAfterTime,
		},
	}
}
