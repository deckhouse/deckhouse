/*
Copyright 2024 Flant JSC

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

package updater

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/deckhouseversion"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/kubernetesversion"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	PhasePending    = "Pending"
	PhaseDeployed   = "Deployed"
	PhaseSuperseded = "Superseded"
	PhaseSuspended  = "Suspended"
	PhaseSkipped    = "Skipped"
)

type UpdateMode string

const (
	// ModeAutoPatch is default mode for updater,
	// deckhouse automatically applies patch releases, but asks for approval of minor releases
	ModeAutoPatch UpdateMode = "AutoPatch"
	// ModeAuto is updater mode when deckhouse automatically applies all releases
	ModeAuto UpdateMode = "Auto"
	// ModeManual is updater mode when deckhouse downloads releases info, but does not apply them
	ModeManual UpdateMode = "Manual"
)

type Updater[R v1alpha1.Release] struct {
	ctx            context.Context
	now            time.Time
	settings       *Settings
	enabledModules set.Set

	logger            *log.Logger
	kubeAPI           KubeAPI[R]
	metricsUpdater    MetricsUpdater[R]
	webhookDataSource WebhookDataSource[R]

	// don't modify releases order, logic is based on this sorted slice
	releases                    []R
	predictedReleaseIndex       int
	skippedPatchesIndexes       []int
	currentDeployedReleaseIndex int
	forcedReleaseIndex          int
	predictedReleaseIsPatch     *bool

	deckhousePodIsReady      bool
	deckhouseIsBootstrapping bool
	releaseData              DeckhouseReleaseData
}

func NewUpdater[R v1alpha1.Release](
	ctx context.Context,
	dc dependency.Container,
	logger *log.Logger,
	settings *Settings,
	data DeckhouseReleaseData,
	podIsReady, isBootstrapping bool,
	kubeAPI KubeAPI[R],
	metricsUpdater MetricsUpdater[R],
	webhookDataSource WebhookDataSource[R],
	enabledModules []string,
) *Updater[R] {
	return &Updater[R]{
		ctx:            ctx,
		now:            dc.GetClock().Now().UTC(),
		settings:       settings,
		enabledModules: set.New(enabledModules...),

		logger:            logger,
		kubeAPI:           kubeAPI,
		metricsUpdater:    metricsUpdater,
		webhookDataSource: webhookDataSource,

		predictedReleaseIndex:       -1,
		currentDeployedReleaseIndex: -1,
		forcedReleaseIndex:          -1,
		skippedPatchesIndexes:       make([]int, 0),
		deckhousePodIsReady:         podIsReady,
		deckhouseIsBootstrapping:    isBootstrapping,
		releaseData:                 data,
	}
}

// for patch, we check fewer conditions, then for minor release
// - Canary settings
func (u *Updater[R]) checkPatchReleaseConditions(release R, metricLabels MetricLabels) error {
	applyTime, reason, err := u.calculatePatchResultDeployTime(release, metricLabels)
	if err != nil {
		return fmt.Errorf("calculate patch result deploy time: %w", err)
	}

	// check: Notification
	if u.settings.NotificationConfig != (NotificationConfig{}) && u.settings.NotificationConfig.ReleaseType == ReleaseTypeAll {
		metricLabels[NotificationNotSent] = "false"
		err = u.sendReleaseNotification(release, applyTime)
		if err != nil {
			metricLabels[NotificationNotSent] = "true"
			if err := u.updateStatus(release, "Release blocked: failed to send release notification", PhasePending); err != nil {
				return fmt.Errorf("update status: %w", err)
			}
			return fmt.Errorf("send release notification: %w", err)
		}
	}

	if release.GetApplyNow() {
		return nil
	}

	return u.postponeDeploy(release, reason, applyTime)
}

func (u *Updater[R]) sendReleaseNotification(release R, releaseApplyTime time.Time) error {
	if u.releaseData.Notified {
		return nil
	}

	predictedReleaseVersion := release.GetVersion()

	if u.settings.NotificationConfig.WebhookURL != "" {
		data := WebhookData{
			Version:       predictedReleaseVersion.String(),
			Requirements:  release.GetRequirements(),
			ChangelogLink: release.GetChangelogLink(),
			ApplyTime:     releaseApplyTime.Format(time.RFC3339),
		}
		u.webhookDataSource.Fill(&data, release, releaseApplyTime)

		err := sendWebhookNotification(u.settings.NotificationConfig, data)
		if err != nil {
			return fmt.Errorf("send release notification failed: %w", err)
		}
	}

	err := u.changeNotifiedFlag(true)
	if err != nil {
		return fmt.Errorf("change notified flag: %w", err)
	}

	return nil
}

// for minor release (version change) we check more conditions
// - Release requirements
// - Disruptions
// - Notification
// - Cooldown
// - Canary settings
// - Update windows or manual approval
// - Deckhouse pod is ready
func (u *Updater[R]) checkMinorReleaseConditions(release R, metricLabels MetricLabels) error {
	// check: release disruptions (hard lock)
	passed := u.checkReleaseDisruptions(release)
	if !passed {
		metricLabels[DisruptionApprovalRequired] = "true"
		return fmt.Errorf("release %s disruption approval required: %w", release.GetName(), ErrDeployConditionsNotMet)
	}

	resultDeployTime, delayReason, err := u.calculateMinorResultDeployTime(release, metricLabels)
	if err != nil {
		return fmt.Errorf("calculate minor result deploy time: %w", err)
	}

	// check: Notification
	if u.settings.NotificationConfig != (NotificationConfig{}) {
		metricLabels[NotificationNotSent] = "false"
		err = u.sendReleaseNotification(release, resultDeployTime)
		if err != nil {
			metricLabels[NotificationNotSent] = "true"
			if err := u.updateStatus(release, "Release blocked: failed to send release notification", PhasePending); err != nil {
				return fmt.Errorf("update status: %w", err)
			}
			return fmt.Errorf("send release notification: %w", err)
		}
	}

	// check: Deckhouse pod is ready
	if !u.deckhousePodIsReady {
		u.logger.Info("Deckhouse is not ready. Skipping upgrade")
		if err := u.updateStatus(release, "Awaiting for Deckhouse pod to be ready", PhasePending); err != nil {
			return fmt.Errorf("update status: %w", err)
		}
		return ErrDeployConditionsNotMet
	}

	if release.GetApplyNow() {
		return nil
	}

	return u.postponeDeploy(release, delayReason, resultDeployTime)
}

func (u *Updater[R]) calculateMinorResultDeployTime(release R, metricLabels MetricLabels) (time.Time, deployDelayReason, error) {
	var (
		newApplyAfter    time.Time
		releaseApplyTime = u.now
		reason           deployDelayReason
	)

	if release.GetApplyNow() {
		return releaseApplyTime, reason, nil
	}

	// check: release cooldown
	if release.GetCooldownUntil() != nil {
		cooldownUntil := *release.GetCooldownUntil()
		if u.now.Before(cooldownUntil) {
			u.logger.Warnf("Release %s in cooldown", release.GetName())
			releaseApplyTime, reason = *release.GetCooldownUntil(), reason.add(cooldownDelayReason)
		}
	}

	// check: canary settings
	if release.GetApplyAfter() != nil && !u.InManualMode() {
		applyAfter := *release.GetApplyAfter()
		if u.now.Before(applyAfter) {
			u.logger.Warnf("Release %s is postponed by canary process. Waiting", release.GetName())
			releaseApplyTime, reason = applyAfter, reason.add(canaryDelayReason)
		}
	}

	if !u.releaseData.Notified &&
		u.settings.NotificationConfig.MinimalNotificationTime.Duration > 0 {
		minApplyTime := u.now.Add(u.settings.NotificationConfig.MinimalNotificationTime.Duration)
		if minApplyTime.Before(releaseApplyTime) {
			minApplyTime = releaseApplyTime
		} else {
			releaseApplyTime, newApplyAfter, reason = minApplyTime, minApplyTime, reason.add(notificationDelayReason)
		}
	}

	if u.settings.Mode == ModeAuto && !u.settings.Windows.IsAllowed(releaseApplyTime) {
		releaseApplyTime, reason = u.settings.Windows.NextAllowedTime(releaseApplyTime), reason.add(outOfWindowReason)
	}

	// check: release is approved in Manual mode
	if u.settings.Mode != ModeAuto && !release.GetManuallyApproved() {
		u.logger.Infof("Release %s is waiting for manual approval ", release.GetName())
		metricLabels[ManualApprovalRequired] = "true"
		releaseApplyTime, reason = u.now, manualApprovalRequiredReason
	}

	if !newApplyAfter.IsZero() {
		err := u.kubeAPI.PatchReleaseApplyAfter(release, newApplyAfter)
		if err != nil {
			return time.Time{}, 0, fmt.Errorf("patch release %s apply after: %w", release.GetName(), err)
		}

		return releaseApplyTime, notificationDelayReason, nil
	}

	return releaseApplyTime, reason, nil
}

func (u *Updater[R]) calculatePatchResultDeployTime(release R, metricLabels MetricLabels) (time.Time, deployDelayReason, error) {
	var (
		newApplyAfter    time.Time
		releaseApplyTime = u.now
		reason           deployDelayReason
	)

	if release.GetApplyNow() {
		return releaseApplyTime, reason, nil
	}

	// check: canary settings
	if release.GetApplyAfter() != nil {
		applyAfter := *release.GetApplyAfter()
		if u.now.Before(applyAfter) {
			u.logger.Warnf("Release %s is postponed by canary process. Waiting", release.GetName())
			releaseApplyTime, reason = applyAfter, reason.add(canaryDelayReason)
		}
	}

	if !u.releaseData.Notified &&
		u.settings.NotificationConfig.MinimalNotificationTime.Duration > 0 {
		minApplyTime := u.now.Add(u.settings.NotificationConfig.MinimalNotificationTime.Duration)
		if minApplyTime.Before(releaseApplyTime) {
			minApplyTime = releaseApplyTime
		} else {
			releaseApplyTime, newApplyAfter, reason = minApplyTime, minApplyTime, reason.add(notificationDelayReason)
		}
	}

	if u.settings.Mode == ModeAutoPatch && !u.settings.Windows.IsAllowed(releaseApplyTime) {
		releaseApplyTime, reason = u.settings.Windows.NextAllowedTime(releaseApplyTime), reason.add(outOfWindowReason)
	}

	if u.settings.Mode == ModeManual && !release.GetManuallyApproved() {
		u.logger.Infof("Release %s is waiting for manual approval", release.GetName())
		metricLabels[ManualApprovalRequired] = "true"
		releaseApplyTime, reason = u.now, manualApprovalRequiredReason
	}

	if !newApplyAfter.IsZero() {
		err := u.kubeAPI.PatchReleaseApplyAfter(release, newApplyAfter)
		if err != nil {
			return time.Time{}, 0, fmt.Errorf("patch release %s apply after: %w", release.GetName(), err)
		}

		return releaseApplyTime, notificationDelayReason, nil
	}

	return releaseApplyTime, reason, nil
}

func (u *Updater[R]) setReleaseQueueDepthLabel(metricLabels map[string]string) {
	if metricLabels[ManualApprovalRequired] == "true" {
		diff := len(u.releases) - 1 - u.predictedReleaseIndex
		switch {
		case diff == 1:
			metricLabels[ReleaseQueueDepth] = "one"
		case diff == 2:
			metricLabels[ReleaseQueueDepth] = "two"
		case diff > 2:
			metricLabels[ReleaseQueueDepth] = "three"
		}
	}
}

// ApplyPredictedRelease applies predicted release, checks everything:
//   - Deckhouse is ready (except patch)
//   - Canary settings
//   - Manual approving
//   - Release requirements
//
// In addition to the regular error, ErrDeployConditionsNotMet or NotReadyForDeployError is returned as appropriate.
func (u *Updater[R]) ApplyPredictedRelease() error {
	var err error
	if u.predictedReleaseIndex == -1 {
		return ErrDeployConditionsNotMet // has no predicted release
	}

	var (
		currentRelease   *R
		predictedRelease = u.releases[u.predictedReleaseIndex]
	)

	if u.currentDeployedReleaseIndex != -1 {
		currentRelease = &(u.releases[u.currentDeployedReleaseIndex])
	}

	// if deckhouse pod has bootstrap image -> apply first release
	// doesn't matter which is update mode
	if u.deckhouseIsBootstrapping && len(u.releases) == 1 {
		return u.runReleaseDeploy(predictedRelease, currentRelease)
	}

	metricLabels := NewReleaseMetricLabels(predictedRelease)

	if u.PredictedReleaseIsPatch() {
		err = u.checkPatchReleaseConditions(predictedRelease, metricLabels)
	} else {
		err = u.checkMinorReleaseConditions(predictedRelease, metricLabels)
	}
	u.setReleaseQueueDepthLabel(metricLabels)

	// if the predicted release has an index less than the number of awaiting releases
	// calculate and set releaseDepthQueue label
	u.metricsUpdater.UpdateReleaseMetric(predictedRelease.GetName(), metricLabels)
	if err != nil {
		return fmt.Errorf("check release %s conditions: %w", predictedRelease.GetName(), err)
	}

	// all checks are passed, deploy release

	return u.runReleaseDeploy(predictedRelease, currentRelease)
}

func (u *Updater[R]) predictedRelease() *R {
	if u.predictedReleaseIndex == -1 {
		return nil // has no predicted release
	}

	predictedRelease := &(u.releases[u.predictedReleaseIndex])

	return predictedRelease
}

func (u *Updater[R]) DeployedRelease() *R {
	if u.currentDeployedReleaseIndex == -1 {
		return nil // has no deployed
	}

	deployedRelease := &(u.releases[u.currentDeployedReleaseIndex])
	u.logger.Debugf("Deployed release found by updater: %v", deployedRelease)

	return deployedRelease
}

func (u *Updater[R]) checkReleaseDisruptions(rl R) bool {
	mode := u.settings.DisruptionApprovalMode
	if mode == "" || mode == "Auto" {
		return true
	}

	for _, key := range rl.GetDisruptions() {
		hasDisruptionUpdate, reason := requirements.HasDisruption(key)
		if hasDisruptionUpdate {
			if !rl.GetDisruptionApproved() {
				msg := fmt.Sprintf("Release requires disruption approval (`kubectl annotate DeckhouseRelease %s release.deckhouse.io/disruption-approved=true`): %s", rl.GetName(), reason)
				err := u.updateStatus(rl, msg, PhasePending)
				if err != nil {
					u.logger.Error("update status", log.Err(err))
				}
				return false
			}
		}
	}

	return true
}

// SetReleases set and sort releases for updater
func (u *Updater[R]) SetReleases(releases []R) {
	if len(releases) == 0 {
		return
	}

	sort.Sort(ByVersion[R](releases))

	u.releases = releases
}

func (u *Updater[R]) ReleasesCount() int {
	return len(u.releases)
}

func (u *Updater[R]) InManualMode() bool {
	return u.settings.Mode == ModeManual
}

func (u *Updater[R]) runReleaseDeploy(predictedRelease R, currentRelease *R) error {
	ctx := context.TODO()
	u.logger.Infof("Applying release %s", predictedRelease.GetName())

	err := u.ChangeUpdatingFlag(true)
	if err != nil {
		return fmt.Errorf("change updating flag: %w", err)
	}
	err = u.changeNotifiedFlag(false)
	if err != nil {
		return fmt.Errorf("change notified flag: %w", err)
	}

	err = u.kubeAPI.DeployRelease(ctx, predictedRelease)
	if err != nil {
		return fmt.Errorf("deploy release: %w", err)
	}

	err = u.updateStatus(predictedRelease, "", PhaseDeployed)
	if err != nil {
		return fmt.Errorf("update status to deployed: %w", err)
	}

	// remove annotation if exists
	if predictedRelease.GetApplyNow() {
		err = u.kubeAPI.PatchReleaseAnnotations(
			ctx,
			predictedRelease,
			map[string]interface{}{
				"release.deckhouse.io/apply-now": nil,
			})
		if err != nil {
			return fmt.Errorf("remove apply-now annotation: %w", err)
		}
	}

	if currentRelease != nil {
		// skip last deployed release
		err = u.updateStatus(*currentRelease, "", PhaseSuperseded)
		if err != nil {
			return fmt.Errorf("update status to superseded: %w", err)
		}
	}

	return u.CommitSkippedReleases()
}

// PredictNextRelease runs prediction of the next release to deploy.
// It skips patch releases and saves only the latest one.
// Also, the metrics of the referenced release are updated.
func (u *Updater[R]) PredictNextRelease(release R) {
	for index, rl := range u.releases {
		if rl.GetPhase() == PhaseDeployed {
			u.currentDeployedReleaseIndex = index
			break
		}
	}

	for i, rl := range u.releases {
		switch rl.GetPhase() {
		case PhaseSuperseded, PhaseSuspended, PhaseSkipped:
			// pass

		case PhasePending:
			releaseRequirementsMet := u.checkReleaseRequirements(rl)
			u.processPendingRelease(i, rl, releaseRequirementsMet)
			// update metric only for the release that initiated prediction so as not to provoke metrics churn on every prediction
			if rl.GetName() == release.GetName() {
				metricLabels := NewReleaseMetricLabels(rl)
				if !releaseRequirementsMet {
					metricLabels[RequirementsNotMet] = "true"
				}
				u.metricsUpdater.UpdateReleaseMetric(rl.GetName(), metricLabels)
			}
		}

		if rl.GetForce() {
			u.forcedReleaseIndex = i
		}
	}
}

// LastReleaseDeployed returns the equality of the latest existed release with the latest deployed
func (u *Updater[R]) LastReleaseDeployed() bool {
	return u.currentDeployedReleaseIndex == len(u.releases)-1
}

func (u *Updater[R]) GetCurrentDeployedReleaseIndex() int {
	return u.currentDeployedReleaseIndex
}

// HasForceRelease check the existence of the forced release
func (u *Updater[R]) HasForceRelease() bool {
	return u.forcedReleaseIndex != -1
}

// ApplyForcedRelease deploys forced release without any checks (windows, requirements, approvals and so on)
func (u *Updater[R]) ApplyForcedRelease(ctx context.Context) error {
	if u.forcedReleaseIndex == -1 {
		return nil
	}
	forcedRelease := u.releases[u.forcedReleaseIndex]

	var currentRelease *R
	if u.currentDeployedReleaseIndex != -1 {
		currentRelease = &(u.releases[u.currentDeployedReleaseIndex])
	}

	u.logger.Warnf("Forcing release %s", forcedRelease.GetName())

	result := u.runReleaseDeploy(forcedRelease, currentRelease)

	// remove annotation
	err := u.kubeAPI.PatchReleaseAnnotations(ctx, forcedRelease, map[string]any{
		"release.deckhouse.io/force": nil,
	})
	if err != nil {
		return fmt.Errorf("patch force annotation: %w", err)
	}

	// Outdate all previous releases
	for i, release := range u.releases {
		if i < u.forcedReleaseIndex {
			err := u.updateStatus(release, "", PhaseSuperseded)
			if err != nil {
				u.logger.Error("update status", log.Err(err))
			}
		}
	}

	return result
}

// PredictedReleaseIsPatch shows if the predicted release is a patch with respect to the Deployed one
func (u *Updater[R]) PredictedReleaseIsPatch() bool {
	if u.predictedReleaseIsPatch != nil {
		return *u.predictedReleaseIsPatch
	}

	if u.currentDeployedReleaseIndex == -1 {
		u.predictedReleaseIsPatch = ptr.To(false)
		return false
	}

	if u.predictedReleaseIndex == -1 {
		u.predictedReleaseIsPatch = ptr.To(false)
		return false
	}

	current := u.releases[u.currentDeployedReleaseIndex]
	predicted := u.releases[u.predictedReleaseIndex]

	if current.GetVersion().Major() != predicted.GetVersion().Major() {
		u.predictedReleaseIsPatch = ptr.To(false)
		return false
	}

	if current.GetVersion().Minor() != predicted.GetVersion().Minor() {
		u.predictedReleaseIsPatch = ptr.To(false)
		return false
	}

	u.predictedReleaseIsPatch = ptr.To(true)
	return true
}

func (u *Updater[R]) processPendingRelease(index int, release R, releaseRequirementsMet bool) {
	// if we have a deployed a release
	if u.currentDeployedReleaseIndex >= 0 {
		// if deployed version is greater than the pending one, this pending release should be superseded
		if u.releases[u.currentDeployedReleaseIndex].GetVersion().GreaterThan(release.GetVersion()) {
			u.skippedPatchesIndexes = append(u.skippedPatchesIndexes, index)
			return
		}
	}

	if releaseRequirementsMet {
		// check: already has predicted release and current is a patch
		if u.predictedReleaseIndex >= 0 {
			previousPredictedRelease := u.releases[u.predictedReleaseIndex]
			if previousPredictedRelease.GetVersion().Major() < release.GetVersion().Major() || previousPredictedRelease.GetVersion().Minor() < release.GetVersion().Minor() {
				if err := u.updateStatus(release, fmt.Sprintf("Awaiting for %s release to be deployed", previousPredictedRelease.GetName()), PhasePending); err != nil {
					u.logger.Error("update status", log.Err(err))
				}
				return
			}
			// it's a patch for predicted release, continue
			u.skippedPatchesIndexes = append(u.skippedPatchesIndexes, u.predictedReleaseIndex)
		}
		u.predictedReleaseIndex = index
	}
}

func (u *Updater[R]) checkReleaseRequirements(rl R) bool {
	switch any(rl).(type) {
	case *v1alpha1.ModuleRelease:
		u.logger.Debugf("checking requirements of '%s' for module '%s' by extenders", rl.GetName(), rl.GetModuleName())
		moduleRelease := any(rl).(*v1alpha1.ModuleRelease)
		if err := extenders.CheckModuleReleaseRequirements(moduleRelease.GetModuleName(), moduleRelease.GetName(), moduleRelease.GetVersion(), moduleRelease.GetModuleReleaseRequirements()); err != nil {
			err = u.updateStatus(rl, err.Error(), PhasePending)
			if err != nil {
				u.logger.Error("update status", log.Err(err))
			}
			return false
		}

	case *v1alpha1.DeckhouseRelease:
		if moduleName, err := deckhouseversion.Instance().ValidateBaseVersion(rl.GetVersion().String()); err != nil {
			// invalid deckhouse version in deckhouse release or an enabled module has requirements that prevent deckhouse release from becoming predicted
			if moduleName == "" || u.enabledModules.Has(moduleName) {
				if err = u.updateStatus(rl, err.Error(), PhasePending); err != nil {
					u.logger.Error("update status", log.Err(err))
				}
				return false
			}
		}

		k8sVersionAutomatic, err := u.kubeAPI.IsKubernetesVersionAutomatic(u.ctx)
		// if discovery failed, we musn't suspend the release
		if err != nil {
			u.logger.Error("check k8s automatic version", log.Err(err))
			return false
		}
		if k8sVersionAutomatic && len(rl.GetRequirements()["autoK8sVersion"]) > 0 {
			if moduleName, err := kubernetesversion.Instance().ValidateBaseVersion(rl.GetRequirements()["autoK8sVersion"]); err != nil {
				// invalid auto kubernetes version in deckhouse release or an enabled module has requirements that prevent deckhouse release from becoming predicted
				if moduleName == "" || u.enabledModules.Has(moduleName) {
					if err = u.updateStatus(rl, err.Error(), PhasePending); err != nil {
						u.logger.Error("update status", log.Err(err))
					}
					return false
				}
			}
		}

		for key, value := range rl.GetRequirements() {
			// these fields are checked by extenders in module release controller
			if extenders.IsExtendersField(key) {
				continue
			}
			passed, err := requirements.CheckRequirement(key, value, u.enabledModules)
			if !passed {
				msg := fmt.Sprintf("%q requirement for DeckhouseRelease %q not met: %s", key, rl.GetVersion(), err)
				if errors.Is(err, requirements.ErrNotRegistered) {
					u.logger.Error("check requirements", log.Err(err))
					msg = fmt.Sprintf("%q requirement is not registered", key)
				}
				if err := u.updateStatus(rl, msg, PhasePending); err != nil {
					u.logger.Error("update status", log.Err(err))
				}
				return false
			}
		}
	default:
		u.logger.Error("Unknown release %s type: %T", rl.GetName(), rl)
		return false
	}

	return true
}

func (u *Updater[R]) updateStatus(release R, msg, phase string) error {
	if phase == release.GetPhase() && msg == release.GetMessage() {
		return nil
	}

	switch phase {
	case PhaseSuperseded, PhaseSuspended, PhaseSkipped, PhaseDeployed:
		u.metricsUpdater.PurgeReleaseMetric(release.GetName())
	}

	return u.kubeAPI.UpdateReleaseStatus(u.ctx, release, msg, phase)
}

func (u *Updater[R]) ChangeUpdatingFlag(fl bool) error {
	if u.releaseData.IsUpdating == fl {
		return nil
	}

	u.releaseData.IsUpdating = fl
	return u.saveReleaseData()
}

func (u *Updater[R]) changeNotifiedFlag(fl bool) error {
	if u.releaseData.Notified == fl {
		return nil
	}

	u.releaseData.Notified = fl
	return u.saveReleaseData()
}

func (u *Updater[R]) saveReleaseData() error {
	if u.predictedReleaseIndex != -1 {
		ctx := context.TODO()
		release := u.releases[u.predictedReleaseIndex]
		return u.kubeAPI.SaveReleaseData(ctx, release, u.releaseData)
	}

	u.logger.Warn("save release data: release not found")
	return nil
}

func (u *Updater[R]) GetPredictedReleaseIndex() int {
	return u.predictedReleaseIndex
}

func (u *Updater[R]) GetPredictedRelease() R {
	var release R
	if u.predictedReleaseIndex == -1 {
		return release
	}
	return u.releases[u.predictedReleaseIndex]
}

func (u *Updater[R]) CommitSkippedReleases() error {
	if len(u.skippedPatchesIndexes) > 0 {
		for _, index := range u.skippedPatchesIndexes {
			release := u.releases[index]
			// skip not-deployed patches
			if err := u.updateStatus(release, "", PhaseSkipped); err != nil {
				return fmt.Errorf("update status to skipped: %w", err)
			}
		}
	}

	return nil
}

func (u *Updater[R]) GetSkippedPatchesIndexes() []int {
	return u.skippedPatchesIndexes
}

func (u *Updater[R]) GetSkippedPatchReleases() []R {
	if len(u.skippedPatchesIndexes) == 0 {
		return nil
	}

	skippedPatches := make([]R, 0, len(u.skippedPatchesIndexes))
	for _, index := range u.skippedPatchesIndexes {
		skippedPatches = append(skippedPatches, u.releases[index])
	}
	return skippedPatches
}

// postponeDeploy update release status and returns new NotReadyForDeployError if reason not equal to noDelay and nil otherwise.
func (u *Updater[R]) postponeDeploy(release R, reason deployDelayReason, applyTime time.Time) error {
	if reason == noDelay {
		return nil
	}

	var (
		zeroTime      time.Time
		retryDelay    time.Duration
		statusMessage string
	)

	if !applyTime.IsZero() {
		retryDelay = applyTime.Sub(u.now)
	}

	if applyTime == u.now {
		applyTime = zeroTime
	}
	statusMessage = reason.Message(release, applyTime)

	err := u.updateStatus(release, statusMessage, PhasePending)
	if err != nil {
		return fmt.Errorf("update release %s status: %w", release.GetName(), err)
	}

	return NewNotReadyForDeployError(statusMessage, retryDelay)
}
