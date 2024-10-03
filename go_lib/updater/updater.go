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

	"github.com/flant/addon-operator/pkg/utils/logger"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	waitingManualApprovalMsg = "Waiting for the 'release.deckhouse.io/approved: \"true\"' annotation"
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

type Updater[R Release] struct {
	now  time.Time
	mode UpdateMode

	logger logger.Logger

	// don't modify releases order, logic is based on this sorted slice
	releases                   []R
	totalPendingManualReleases int

	predictedReleaseIndex       int
	skippedPatchesIndexes       []int
	currentDeployedReleaseIndex int
	forcedReleaseIndex          int
	predictedReleaseIsPatch     *bool

	deckhousePodIsReady      bool
	deckhouseIsBootstrapping bool

	releaseData        DeckhouseReleaseData
	notificationConfig NotificationConfig

	kubeAPI           KubeAPI[R]
	metricsUpdater    MetricsUpdater
	settings          Settings
	webhookDataSource WebhookDataSource[R]

	enabledModules set.Set
}

func NewUpdater[R Release](dc dependency.Container, logger logger.Logger, notificationConfig NotificationConfig, mode string,
	data DeckhouseReleaseData, podIsReady, isBootstrapping bool, kubeAPI KubeAPI[R], metricsUpdater MetricsUpdater,
	settings Settings, webhookDataSource WebhookDataSource[R], enabledModules []string,
) *Updater[R] {
	now := dc.GetClock().Now().UTC()

	return &Updater[R]{
		now:                         now,
		mode:                        ParseUpdateMode(mode),
		logger:                      logger,
		predictedReleaseIndex:       -1,
		currentDeployedReleaseIndex: -1,
		forcedReleaseIndex:          -1,
		skippedPatchesIndexes:       make([]int, 0),
		deckhousePodIsReady:         podIsReady,
		deckhouseIsBootstrapping:    isBootstrapping,
		releaseData:                 data,
		notificationConfig:          notificationConfig,

		kubeAPI:           kubeAPI,
		metricsUpdater:    metricsUpdater,
		settings:          settings,
		webhookDataSource: webhookDataSource,

		enabledModules: set.New(enabledModules...),
	}
}

// for patch, we check fewer conditions, then for minor release
// - Canary settings
func (du *Updater[R]) checkPatchReleaseConditions(release R) error {
	applyTime, reason, err := du.calculatePatchResultDeployTime(release)
	if err != nil {
		return fmt.Errorf("calculate patch result deploy time: %w", err)
	}

	// check: Notification
	if du.notificationConfig != (NotificationConfig{}) && du.notificationConfig.ReleaseType == ReleaseTypeAll {
		err = du.sendReleaseNotification(release, applyTime)
		if err != nil {
			return fmt.Errorf("send release notification: %w", err)
		}
	}

	if release.GetApplyNow() {
		return nil
	}

	return du.postponeDeploy(reason, applyTime)
}

func (du *Updater[R]) sendReleaseNotification(release R, releaseApplyTime time.Time) error {
	if du.releaseData.Notified {
		return nil
	}

	predictedReleaseVersion := release.GetVersion()

	if du.notificationConfig.WebhookURL != "" {
		data := WebhookData{
			Version:       predictedReleaseVersion.String(),
			Requirements:  release.GetRequirements(),
			ChangelogLink: release.GetChangelogLink(),
			ApplyTime:     releaseApplyTime.Format(time.RFC3339),
		}
		du.webhookDataSource.Fill(&data, release, releaseApplyTime)

		err := sendWebhookNotification(du.notificationConfig, data)
		if err != nil {
			return fmt.Errorf("send release notification failed: %w", err)
		}
	}

	err := du.changeNotifiedFlag(true)
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
func (du *Updater[R]) checkMinorReleaseConditions(release R, updateWindows update.Windows) error {
	// check: release requirements (hard lock)
	passed := du.checkReleaseRequirements(release)
	if !passed {
		du.metricsUpdater.ReleaseBlocked(release.GetName(), "requirement")
		return fmt.Errorf("release %s requirements are not met: %w", release.GetName(), ErrDeployConditionsNotMet)
	}

	// check: release disruptions (hard lock)
	passed = du.checkReleaseDisruptions(release)
	if !passed {
		du.metricsUpdater.ReleaseBlocked(release.GetName(), "disruption")
		return fmt.Errorf("release %s disruption approval required: %w", release.GetName(), ErrDeployConditionsNotMet)
	}

	resultDeployTime, delayReason, err := du.calculateMinorResultDeployTime(release, updateWindows)
	if err != nil {
		return fmt.Errorf("calculate minor result deploy time: %w", err)
	}

	// check: Notification
	if du.notificationConfig != (NotificationConfig{}) {
		err = du.sendReleaseNotification(release, resultDeployTime)
		if err != nil {
			return fmt.Errorf("send release notification: %w", err)
		}
	}

	if release.GetApplyNow() {
		return nil
	}

	err = du.postponeDeploy(delayReason, resultDeployTime)
	if err != nil {
		return err
	}

	// check: Deckhouse pod is ready
	if !du.deckhousePodIsReady {
		du.logger.Info("Deckhouse is not ready. Skipping upgrade")
		err := du.updateStatus(release, "Waiting for Deckhouse pod to be ready", PhasePending)
		if err != nil {
			return fmt.Errorf("update status: %w", err)
		}
		return ErrDeployConditionsNotMet
	}

	return nil
}

type deployDelayReason byte

const (
	noDelay deployDelayReason = iota
	cooldownDelayReason
	canaryDelayReason
	manualApprovalRequiredReason
	outOfWindowReason
	notificationDelayReason
)

func (du *Updater[R]) calculateMinorResultDeployTime(release R, updateWindows update.Windows) (releaseApplyTime time.Time, reason deployDelayReason, err error) {
	var (
		applyTimeChanged bool
		statusMessage    string
	)
	releaseApplyTime = du.now

	if release.GetApplyNow() {
		return releaseApplyTime, reason, nil
	}

	// check: release cooldown
	if release.GetCooldownUntil() != nil {
		cooldownUntild := *release.GetCooldownUntil()
		if du.now.Before(cooldownUntild) {
			du.logger.Warnf("Release %s in cooldown", release.GetName())
			releaseApplyTime, reason = *release.GetCooldownUntil(), cooldownDelayReason
			statusMessage = fmt.Sprintf("Release is in cooldown until: %s", release.GetCooldownUntil().Format(time.RFC822))
		}
	}

	// check: canary settings
	if release.GetApplyAfter() != nil && !du.InManualMode() {
		applyAfter := *release.GetApplyAfter()
		if du.now.Before(applyAfter) {
			du.logger.Warnf("Release %s is postponed by canary process. Waiting", release.GetName())
			releaseApplyTime, reason = applyAfter, canaryDelayReason
			statusMessage = fmt.Sprintf("Release is postponed until: %s", applyAfter.Format(time.RFC822))
		}
	}

	if !du.releaseData.Notified &&
		du.notificationConfig.MinimalNotificationTime.Duration > 0 {
		minApplyTime := du.now.Add(du.notificationConfig.MinimalNotificationTime.Duration)
		if minApplyTime.Before(releaseApplyTime) {
			minApplyTime = releaseApplyTime
		} else {
			releaseApplyTime = minApplyTime
			applyTimeChanged = true
		}
	}

	if !updateWindows.IsAllowed(releaseApplyTime) {
		releaseApplyTime, applyTimeChanged = updateWindows.NextAllowedTime(releaseApplyTime), true
		statusMessage = fmt.Sprintf("Release is waiting for the update window: %s", releaseApplyTime.Format(time.RFC822))
	}

	// check: release is approved in Manual mode
	if du.mode != ModeAuto && !release.GetManuallyApproved() {
		applyTimeChanged = false // skip patch release apply after
		du.logger.Infof("Release %s is waiting for manual approval", release.GetName())
		du.metricsUpdater.WaitingManual(release.GetName(), float64(du.totalPendingManualReleases))
		statusMessage, reason = waitingManualApprovalMsg, manualApprovalRequiredReason
	}

	if applyTimeChanged {
		releaseApplyTime = updateWindows.NextAllowedTime(releaseApplyTime)
		err := du.kubeAPI.PatchReleaseApplyAfter(release, releaseApplyTime)
		if err != nil {
			return time.Time{}, 0, fmt.Errorf("patch release %s apply after: %w", release.GetName(), err)
		}
	}

	if statusMessage != "" {
		err := du.updateStatus(release, statusMessage, PhasePending)
		if err != nil {
			return time.Time{}, 0, fmt.Errorf("update release %s status: %w", release.GetName(), err)
		}
	}

	return releaseApplyTime, reason, nil
}

func (du *Updater[R]) calculatePatchResultDeployTime(release R) (releaseApplyTime time.Time, reason deployDelayReason, err error) {
	var (
		applyTimeChanged bool
		statusMessage    string
	)
	releaseApplyTime = du.now

	if release.GetApplyNow() {
		return releaseApplyTime, reason, nil
	}

	// check: canary settings
	if release.GetApplyAfter() != nil {
		applyAfter := *release.GetApplyAfter()
		if du.now.Before(applyAfter) {
			du.logger.Warnf("Release %s is postponed by canary process. Waiting", release.GetName())
			releaseApplyTime, reason = applyAfter, canaryDelayReason
			statusMessage = fmt.Sprintf("Release is postponed until: %s", applyAfter.Format(time.RFC822))
		}
	}

	if !du.releaseData.Notified &&
		du.notificationConfig.MinimalNotificationTime.Duration > 0 {
		minApplyTime := du.now.Add(du.notificationConfig.MinimalNotificationTime.Duration)
		if minApplyTime.Before(releaseApplyTime) {
			minApplyTime = releaseApplyTime
		} else {
			releaseApplyTime = minApplyTime
			applyTimeChanged = true
		}
	}

	if du.mode == ModeManual && !release.GetManuallyApproved() {
		applyTimeChanged = false // skip patch release apply after
		du.logger.Infof("Release %s is waiting for manual approval", release.GetName())
		du.metricsUpdater.WaitingManual(release.GetName(), float64(du.totalPendingManualReleases))
		statusMessage, reason = waitingManualApprovalMsg, manualApprovalRequiredReason
	}

	if applyTimeChanged {
		err := du.kubeAPI.PatchReleaseApplyAfter(release, releaseApplyTime)
		if err != nil {
			return time.Time{}, 0, fmt.Errorf("patch release %s apply after: %w", release.GetName(), err)
		}

		return releaseApplyTime, notificationDelayReason, nil
	}

	if statusMessage != "" {
		err := du.updateStatus(release, statusMessage, PhasePending)
		if err != nil {
			return time.Time{}, 0, fmt.Errorf("update release %s status: %w", release.GetName(), err)
		}
	}

	return releaseApplyTime, reason, nil
}

// ApplyPredictedRelease applies predicted release, checks everything:
//   - Deckhouse is ready (except patch)
//   - Canary settings
//   - Manual approving
//   - Release requirements
//
// In addition to the regular error, ErrDeployConditionsNotMet or NotReadyForDeployError is returned as appropriate.
func (du *Updater[R]) ApplyPredictedRelease(updateWindows update.Windows) (err error) {
	if du.predictedReleaseIndex == -1 {
		return ErrDeployConditionsNotMet // has no predicted release
	}

	var (
		currentRelease   *R
		predictedRelease = du.releases[du.predictedReleaseIndex]
	)

	if du.currentDeployedReleaseIndex != -1 {
		currentRelease = &(du.releases[du.currentDeployedReleaseIndex])
	}

	// if deckhouse pod has bootstrap image -> apply first release
	// doesn't matter which is update mode
	if du.deckhouseIsBootstrapping && len(du.releases) == 1 {
		return du.runReleaseDeploy(predictedRelease, currentRelease)
	}

	if du.PredictedReleaseIsPatch() {
		err = du.checkPatchReleaseConditions(predictedRelease)
	} else {
		err = du.checkMinorReleaseConditions(predictedRelease, updateWindows)
	}
	if err != nil {
		return fmt.Errorf("check release %s conditions: %w", predictedRelease.GetName(), err)
	}

	// all checks are passed, deploy release

	return du.runReleaseDeploy(predictedRelease, currentRelease)
}

func (du *Updater[R]) predictedRelease() *R {
	if du.predictedReleaseIndex == -1 {
		return nil // has no predicted release
	}

	predictedRelease := &(du.releases[du.predictedReleaseIndex])

	return predictedRelease
}

func (du *Updater[R]) deployedRelease() *R {
	if du.currentDeployedReleaseIndex == -1 {
		return nil // has no deployed
	}

	deployedRelease := &(du.releases[du.currentDeployedReleaseIndex])

	return deployedRelease
}

func (du *Updater[R]) checkReleaseDisruptions(rl R) bool {
	dMode, ok := du.settings.GetDisruptionApprovalMode()
	if !ok || dMode == "Auto" {
		return true
	}

	for _, key := range rl.GetDisruptions() {
		hasDisruptionUpdate, reason := requirements.HasDisruption(key)
		if hasDisruptionUpdate {
			if !rl.GetDisruptionApproved() {
				msg := fmt.Sprintf("Release requires disruption approval (`kubectl annotate DeckhouseRelease %s release.deckhouse.io/disruption-approved=true`): %s", rl.GetName(), reason)
				err := du.updateStatus(rl, msg, PhasePending)
				if err != nil {
					du.logger.Error(err)
				}
				return false
			}
		}
	}

	return true
}

// SetReleases set and sort releases for updater
func (du *Updater[R]) SetReleases(releases []R) {
	if len(releases) == 0 {
		return
	}

	sort.Sort(ByVersion[R](releases))

	du.releases = releases
}

func (du *Updater[R]) ReleasesCount() int {
	return len(du.releases)
}

func (du *Updater[R]) InManualMode() bool {
	return du.mode == ModeManual
}

func (du *Updater[R]) runReleaseDeploy(predictedRelease R, currentRelease *R) error {
	ctx := context.TODO()
	du.logger.Infof("Applying release %s", predictedRelease.GetName())

	err := du.ChangeUpdatingFlag(true)
	if err != nil {
		return fmt.Errorf("change updating flag: %w", err)
	}
	err = du.changeNotifiedFlag(false)
	if err != nil {
		return fmt.Errorf("change notified flag: %w", err)
	}

	err = du.kubeAPI.DeployRelease(ctx, predictedRelease)
	if err != nil {
		return fmt.Errorf("deploy release: %w", err)
	}

	err = du.updateStatus(predictedRelease, "", PhaseDeployed)
	if err != nil {
		return fmt.Errorf("update status to deployed: %w", err)
	}

	// remove annotation if exists
	if predictedRelease.GetApplyNow() {
		err = du.kubeAPI.PatchReleaseAnnotations(
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
		err = du.updateStatus(*currentRelease, "", PhaseSuperseded)
		if err != nil {
			return fmt.Errorf("update status to superseded: %w", err)
		}
	}

	if len(du.skippedPatchesIndexes) > 0 {
		for _, index := range du.skippedPatchesIndexes {
			release := du.releases[index]
			// skip not-deployed patches
			err = du.updateStatus(release, "", PhaseSkipped)
			if err != nil {
				return fmt.Errorf("update status to skipped: %w", err)
			}
		}
	}

	return nil
}

// PredictNextRelease runs prediction of the next release to deploy.
// it skips patch releases and save only the latest one
func (du *Updater[R]) PredictNextRelease() {
	for index, rl := range du.releases {
		if rl.GetPhase() == PhaseDeployed {
			du.currentDeployedReleaseIndex = index
			break
		}
	}

	for i, release := range du.releases {
		switch release.GetPhase() {
		case PhaseSuperseded, PhaseSuspended, PhaseSkipped:
			// pass

		case PhasePending:
			du.processPendingRelease(i, release)
		}

		if release.GetForce() {
			du.forcedReleaseIndex = i
		}
	}
}

// LastReleaseDeployed returns the equality of the latest existed release with the latest deployed
func (du *Updater[R]) LastReleaseDeployed() bool {
	return du.currentDeployedReleaseIndex == len(du.releases)-1
}

func (du *Updater[R]) GetCurrentDeployedReleaseIndex() int {
	return du.currentDeployedReleaseIndex
}

// HasForceRelease check the existence of the forced release
func (du *Updater[R]) HasForceRelease() bool {
	return du.forcedReleaseIndex != -1
}

// ApplyForcedRelease deploys forced release without any checks (windows, requirements, approvals and so on)
func (du *Updater[R]) ApplyForcedRelease(ctx context.Context) error {
	if du.forcedReleaseIndex == -1 {
		return nil
	}
	forcedRelease := du.releases[du.forcedReleaseIndex]

	var currentRelease *R
	if du.currentDeployedReleaseIndex != -1 {
		currentRelease = &(du.releases[du.currentDeployedReleaseIndex])
	}

	du.logger.Warnf("Forcing release %s", forcedRelease.GetName())

	result := du.runReleaseDeploy(forcedRelease, currentRelease)

	// remove annotation
	err := du.kubeAPI.PatchReleaseAnnotations(ctx, forcedRelease, map[string]any{
		"release.deckhouse.io/force": nil,
	})
	if err != nil {
		return fmt.Errorf("patch force annotation: %w", err)
	}

	// Outdate all previous releases
	for i, release := range du.releases {
		if i < du.forcedReleaseIndex {
			err := du.updateStatus(release, "", PhaseSuperseded)
			if err != nil {
				du.logger.Error(err)
			}
		}
	}

	return result
}

// PredictedReleaseIsPatch shows if the predicted release is a patch with respect to the Deployed one
func (du *Updater[R]) PredictedReleaseIsPatch() bool {
	if du.predictedReleaseIsPatch != nil {
		return *du.predictedReleaseIsPatch
	}

	if du.currentDeployedReleaseIndex == -1 {
		du.predictedReleaseIsPatch = ptr.To(false)
		return false
	}

	if du.predictedReleaseIndex == -1 {
		du.predictedReleaseIsPatch = ptr.To(false)
		return false
	}

	current := du.releases[du.currentDeployedReleaseIndex]
	predicted := du.releases[du.predictedReleaseIndex]

	if current.GetVersion().Major() != predicted.GetVersion().Major() {
		du.predictedReleaseIsPatch = ptr.To(false)
		return false
	}

	if current.GetVersion().Minor() != predicted.GetVersion().Minor() {
		du.predictedReleaseIsPatch = ptr.To(false)
		return false
	}

	du.predictedReleaseIsPatch = ptr.To(true)
	return true
}

func (du *Updater[R]) processPendingRelease(index int, release R) {
	releaseRequirementsMet := du.checkReleaseRequirements(release)
	// check: already has predicted release and current is a patch
	if du.predictedReleaseIndex >= 0 {
		previousPredictedRelease := du.releases[du.predictedReleaseIndex]
		if previousPredictedRelease.GetVersion().Major() != release.GetVersion().Major() {
			return
		}

		if previousPredictedRelease.GetVersion().Minor() != release.GetVersion().Minor() {
			return
		}
		// it's a patch for predicted release, continue
		if releaseRequirementsMet {
			du.skippedPatchesIndexes = append(du.skippedPatchesIndexes, du.predictedReleaseIndex)
		}
	}

	// if we have a deployed a release
	if du.currentDeployedReleaseIndex >= 0 {
		// if deployed version is greater than the pending one, this pending release should be superseded
		if du.releases[du.currentDeployedReleaseIndex].GetVersion().GreaterThan(release.GetVersion()) {
			du.skippedPatchesIndexes = append(du.skippedPatchesIndexes, index)
			return
		}
	}

	// release is predicted to be Deployed
	if releaseRequirementsMet {
		du.predictedReleaseIndex = index
	}
}

func (du *Updater[R]) checkReleaseRequirements(rl R) bool {
	switch any(rl).(type) {
	case *v1alpha1.ModuleRelease:
		du.logger.Debugf("checking requirements of '%s' for module '%s' by extenders", rl.GetName(), rl.GetModuleName())
		if err := extenders.CheckModuleReleaseRequirements(rl.GetName(), rl.GetRequirements()); err != nil {
			err = du.updateStatus(rl, err.Error(), PhasePending)
			if err != nil {
				du.logger.Error(err)
			}
			return false
		}

	case *v1alpha1.DeckhouseRelease:
		for key, value := range rl.GetRequirements() {
			// these fields are checked by extenders in module release controller
			if extenders.IsExtendersField(key) {
				continue
			}
			passed, err := requirements.CheckRequirement(key, value, du.enabledModules)
			if !passed {
				msg := fmt.Sprintf("%q requirement for DeckhouseRelease %q not met: %s", key, rl.GetVersion(), err)
				if errors.Is(err, requirements.ErrNotRegistered) {
					du.logger.Error(err)
					msg = fmt.Sprintf("%q requirement is not registered", key)
				}
				err := du.updateStatus(rl, msg, PhasePending)
				if err != nil {
					du.logger.Error(err)
				}
				return false
			}
		}
	default:
		du.logger.Error("Unknown release %s type: %T", rl.GetName(), rl)
		return false
	}

	return true
}

func (du *Updater[R]) updateStatus(release R, msg, phase string) error {
	if phase == release.GetPhase() && msg == release.GetMessage() {
		return nil
	}

	return du.kubeAPI.UpdateReleaseStatus(release, msg, phase)
}

func (du *Updater[R]) ChangeUpdatingFlag(fl bool) error {
	if du.releaseData.IsUpdating == fl {
		return nil
	}

	du.releaseData.IsUpdating = fl
	return du.saveReleaseData()
}

func (du *Updater[R]) changeNotifiedFlag(fl bool) error {
	if du.releaseData.Notified == fl {
		return nil
	}

	du.releaseData.Notified = fl
	return du.saveReleaseData()
}

func (du *Updater[R]) saveReleaseData() error {
	if du.predictedReleaseIndex != -1 {
		ctx := context.TODO()
		release := du.releases[du.predictedReleaseIndex]
		return du.kubeAPI.SaveReleaseData(ctx, release, du.releaseData)
	}

	du.logger.Warn("save release data: release not found")
	return nil
}

func (du *Updater[R]) GetPredictedReleaseIndex() int {
	return du.predictedReleaseIndex
}

func (du *Updater[R]) GetPredictedRelease() R {
	var release R
	if du.predictedReleaseIndex == -1 {
		return release
	}
	return du.releases[du.predictedReleaseIndex]
}

func (du *Updater[R]) GetSkippedPatchesIndexes() []int {
	return du.skippedPatchesIndexes
}

func (du *Updater[R]) GetSkippedPatchReleases() []R {
	if len(du.skippedPatchesIndexes) == 0 {
		return nil
	}

	skippedPatches := make([]R, 0, len(du.skippedPatchesIndexes))
	for _, index := range du.skippedPatchesIndexes {
		skippedPatches = append(skippedPatches, du.releases[index])
	}
	return skippedPatches
}

// postponeDeploy returns new NotReadyForDeployError if reason not equal to noDelay and nil otherwise.
// This function returns an interface so that the caller does not have to check the pointer to the structure for nil
// in order to return a nil error itself
func (du *Updater[R]) postponeDeploy(reason deployDelayReason, applyTime time.Time) error {
	if reason == noDelay {
		return nil
	}

	var (
		message    = "not ready for deploy: "
		retryDelay time.Duration
	)

	if !applyTime.IsZero() {
		retryDelay = applyTime.Sub(du.now)
	}

	switch reason {
	case cooldownDelayReason:
		message += fmt.Sprintf("release is in cooldown until %s", applyTime.Format(time.RFC822))
	case canaryDelayReason:
		message += fmt.Sprintf("release is postponed by canary process untill %s", applyTime.Format(time.RFC822))
	case manualApprovalRequiredReason:
		message += fmt.Sprintf("release is waiting for manual approval")
	case outOfWindowReason:
		message += fmt.Sprintf("release is waiting for the update window: %s", applyTime.Format(time.RFC822))
	case notificationDelayReason:
		message += fmt.Sprintf("release is postponed by notification ")

	default:
		panic(fmt.Errorf("invalid reason: %d", reason))
	}

	return &NotReadyForDeployError{message: message, retryDelay: retryDelay}
}
