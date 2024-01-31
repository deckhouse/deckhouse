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
	"errors"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/flant/addon-operator/pkg/utils/logger"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
)

const (
	waitingManualApprovalMsg = "Waiting for manual approval"
)

const (
	PhasePending    = "Pending"
	PhaseDeployed   = "Deployed"
	PhaseSuperseded = "Superseded"
	PhaseSuspended  = "Suspended"
	PhaseSkipped    = "Skipped"
)

type Updater[R Release] struct {
	now          time.Time
	inManualMode bool

	logger logger.Logger

	// don't modify releases order, logic is based on this sorted slice
	releases                   []R
	totalPendingManualReleases int

	predictedReleaseIndex       int
	skippedPatchesIndexes       []int
	currentDeployedReleaseIndex int
	forcedReleaseIndex          int
	appliedNowReleaseIndex      int
	predictedReleaseIsPatch     *bool

	deckhousePodIsReady      bool
	deckhouseIsBootstrapping bool

	releaseData        DeckhouseReleaseData
	notificationConfig *NotificationConfig

	kubeAPI        KubeAPI[R]
	metricsUpdater MetricsUpdater
	settings       Settings
}

func NewUpdater[R Release](logger logger.Logger, notificationConfig *NotificationConfig, mode string, data DeckhouseReleaseData, podIsReady, isBootstrapping bool, kubeAPI KubeAPI[R], metricsUpdater MetricsUpdater, settings Settings) *Updater[R] {
	now := time.Now().UTC()
	if os.Getenv("D8_IS_TESTS_ENVIRONMENT") != "" {
		now = time.Date(2021, 01, 01, 13, 30, 00, 00, time.UTC)
	}
	return &Updater[R]{
		now:                         now,
		inManualMode:                mode == "Manual",
		logger:                      logger,
		predictedReleaseIndex:       -1,
		currentDeployedReleaseIndex: -1,
		forcedReleaseIndex:          -1,
		appliedNowReleaseIndex:      -1,
		skippedPatchesIndexes:       make([]int, 0),
		deckhousePodIsReady:         podIsReady,
		deckhouseIsBootstrapping:    isBootstrapping,
		releaseData:                 data,
		notificationConfig:          notificationConfig,

		kubeAPI:        kubeAPI,
		metricsUpdater: metricsUpdater,
		settings:       settings,
	}
}

// for patch we check less conditions, then for minor release
// - Canary settings
func (du *Updater[R]) checkPatchReleaseConditions(predictedRelease *R) bool {
	// check: canary settings
	if (*predictedRelease).GetApplyAfter() != nil {
		if du.now.Before(*(*predictedRelease).GetApplyAfter()) {
			du.logger.Infof("Release %s is postponed by canary process. Waiting", (*predictedRelease).GetName())
			err := du.updateStatus(predictedRelease, fmt.Sprintf("Release is postponed until: %s", (*predictedRelease).GetApplyAfter().Format(time.RFC822)), PhasePending)
			if err != nil {
				du.logger.Error(err)
			}
			return false
		}
	}

	return true
}

func (du *Updater[R]) checkReleaseNotification(predictedRelease *R, updateWindows update.Windows) bool {
	if du.releaseData.Notified {
		return true
	}

	var applyTimeChanged bool
	predictedReleaseApplyTime := du.predictedReleaseApplyTime(predictedRelease)
	if du.notificationConfig.MinimalNotificationTime.Duration > 0 {
		minApplyTime := du.now.Add(du.notificationConfig.MinimalNotificationTime.Duration)
		if minApplyTime.Before(predictedReleaseApplyTime) {
			minApplyTime = predictedReleaseApplyTime
		} else {
			predictedReleaseApplyTime = minApplyTime
			applyTimeChanged = true
		}
	}
	releaseApplyTime := updateWindows.NextAllowedTime(predictedReleaseApplyTime)

	predictedReleaseVersion := (*predictedRelease).GetVersion()
	version := fmt.Sprintf("%d.%d", predictedReleaseVersion.Major(), predictedReleaseVersion.Minor())
	msg := fmt.Sprintf("New Deckhouse Release %s is available. Release will be applied at: %s", version, releaseApplyTime.Format(time.RFC850))
	if du.notificationConfig.WebhookURL != "" {
		data := webhookData{
			Version:       fmt.Sprintf("%d.%d", predictedReleaseVersion.Major(), predictedReleaseVersion.Minor()),
			Requirements:  (*predictedRelease).GetRequirements(),
			ChangelogLink: (*predictedRelease).GetChangelogLink(),
			ApplyTime:     releaseApplyTime.Format(time.RFC3339),
			Message:       msg,
		}

		err := sendWebhookNotification(du.notificationConfig, data)
		if err != nil {
			du.logger.Errorf("Send release notification failed: %s", err)
			return false
		}
	}

	err := du.changeNotifiedFlag(true)
	if err != nil {
		du.logger.Error("change notified flag: %s", err.Error())
		return false
	}

	if applyTimeChanged {
		err = du.kubeAPI.PatchReleaseApplyAfter((*predictedRelease).GetName(), releaseApplyTime)
		if err != nil {
			du.logger.Errorf("patch apply after: %s", err.Error())
		}
		return false
	}

	return true
}

// for minor release (version change) we check more conditions
// - Release requirements
// - Disruptions
// - Notification
// - Cooldown
// - Canary settings
// - Update windows or manual approval
// - Deckhouse pod is ready
func (du *Updater[R]) checkMinorReleaseConditions(predictedRelease *R, updateWindows update.Windows) bool {
	// check: release requirements (hard lock)
	passed := du.checkReleaseRequirements(predictedRelease)
	if !passed {
		du.metricsUpdater.ReleaseBlocked((*predictedRelease).GetName(), "requirement")
		du.logger.Warnf("Release %s requirements are not met", (*predictedRelease).GetName())
		return false
	}

	// check: release disruptions (hard lock)
	passed = du.checkReleaseDisruptions(predictedRelease)
	if !passed {
		du.metricsUpdater.ReleaseBlocked((*predictedRelease).GetName(), "disruption")
		du.logger.Warnf("Release %s disruption approval required", (*predictedRelease).GetName())
		return false
	}

	// check: Notification
	if du.notificationConfig != nil {
		passed = du.checkReleaseNotification(predictedRelease, updateWindows)
		if !passed {
			return false
		}
	}

	// check: release cooldown
	if (*predictedRelease).GetCooldownUntil() != nil {
		if du.now.Before(*(*predictedRelease).GetCooldownUntil()) {
			du.logger.Infof("Release %s in cooldown", (*predictedRelease).GetName())
			err := du.updateStatus(predictedRelease, fmt.Sprintf("Release is in cooldown until: %s", (*predictedRelease).GetCooldownUntil().Format(time.RFC822)), PhasePending)
			if err != nil {
				du.logger.Error(err)
			}
			return false
		}
	}

	// check: canary settings
	if (*predictedRelease).GetApplyAfter() != nil && !du.inManualMode {
		if du.now.Before(*(*predictedRelease).GetApplyAfter()) {
			du.logger.Infof("Release %s is postponed by canary process. Waiting", (*predictedRelease).GetName())
			err := du.updateStatus(predictedRelease, fmt.Sprintf("Release is postponed until: %s", (*predictedRelease).GetApplyAfter().Format(time.RFC822)), PhasePending)
			if err != nil {
				du.logger.Error(err)
			}
			return false
		}
	}

	if du.inManualMode {
		// check: release is approved in Manual mode
		if !(*predictedRelease).GetApprovedStatus() {
			du.logger.Infof("Release %s is waiting for manual approval", (*predictedRelease).GetName())
			du.metricsUpdater.WaitingManual((*predictedRelease).GetName(), float64(du.totalPendingManualReleases))
			err := du.updateStatus(predictedRelease, waitingManualApprovalMsg, PhasePending)
			if err != nil {
				du.logger.Error(err)
			}
			return false
		}
	} else {
		// check: update windows in Auto mode
		if len(updateWindows) > 0 {
			updatePermitted := updateWindows.IsAllowed(du.now)
			if !updatePermitted {
				applyTime := updateWindows.NextAllowedTime(du.now)
				du.logger.Info("Deckhouse update does not get into update windows. Skipping")
				err := du.updateStatus(predictedRelease, fmt.Sprintf("Release is waiting for the update window: %s", applyTime.Format(time.RFC822)), PhasePending)
				if err != nil {
					du.logger.Error(err)
				}
				return false
			}
		}
	}

	// check: Deckhouse pod is ready
	if !du.deckhousePodIsReady {
		du.logger.Info("Deckhouse is not ready. Skipping upgrade")
		err := du.updateStatus(predictedRelease, "Waiting for Deckhouse pod to be ready", PhasePending)
		if err != nil {
			du.logger.Error(err)
		}
		return false
	}

	return true
}

// ApplyPredictedRelease applies predicted release, checks everything:
//   - Deckhouse is ready (except patch)
//   - Canary settings
//   - Manual approving
//   - Release requirements
func (du *Updater[R]) ApplyPredictedRelease(updateWindows update.Windows) bool {
	if du.predictedReleaseIndex == -1 {
		return false // has no predicted release
	}

	var currentRelease *R

	predictedRelease := &(du.releases[du.predictedReleaseIndex])

	if du.currentDeployedReleaseIndex != -1 {
		currentRelease = &(du.releases[du.currentDeployedReleaseIndex])
	}

	// if deckhouse pod has bootstrap image -> apply first release
	// doesn't matter which is update mode
	if du.deckhouseIsBootstrapping && len(du.releases) == 1 {
		return du.runReleaseDeploy(predictedRelease, currentRelease)
	}

	var readyForDeploy bool

	if du.PredictedReleaseIsPatch() {
		readyForDeploy = du.checkPatchReleaseConditions(predictedRelease)
	} else {
		readyForDeploy = du.checkMinorReleaseConditions(predictedRelease, updateWindows)
	}

	if !readyForDeploy {
		return false
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

func (du *Updater[R]) predictedReleaseApplyTime(predictedRelease *R) time.Time {
	if (*predictedRelease).GetApplyAfter() != nil {
		return *(*predictedRelease).GetApplyAfter()
	}

	return du.now
}

func (du *Updater[R]) checkReleaseDisruptions(rl *R) bool {
	dMode, ok := du.settings.GetDisruptionApprovalMode()
	if !ok || dMode == "Auto" {
		return true
	}

	for _, key := range (*rl).GetDisruptions() {
		hasDisruptionUpdate, reason := requirements.HasDisruption(key)
		if hasDisruptionUpdate {
			if !(*rl).GetDisruptionApproved() {
				msg := fmt.Sprintf("Release requires disruption approval (`kubectl annotate DeckhouseRelease %s release.deckhouse.io/disruption-approved=true`): %s", (*rl).GetName(), reason)
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

func (du *Updater[R]) ReleasesCount() int {
	return len(du.releases)
}

func (du *Updater[R]) InManualMode() bool {
	return du.inManualMode
}

func (du *Updater[R]) runReleaseDeploy(predictedRelease, currentRelease *R) bool {
	du.logger.Infof("Applying release %s", (*predictedRelease).GetName())

	err := du.ChangeUpdatingFlag(true)
	if err != nil {
		du.logger.Error("change updating flag: %s", err.Error())
		return false
	}
	err = du.changeNotifiedFlag(false)
	if err != nil {
		du.logger.Error("change notified flag: %s", err.Error())
		return false
	}

	err = du.kubeAPI.DeployRelease(*predictedRelease)
	if err != nil {
		du.logger.Error(err)
		return false
	}

	err = du.updateStatus(predictedRelease, "", PhaseDeployed)
	if err != nil {
		du.logger.Error(err)
		return false
	}

	if currentRelease != nil {
		// skip last deployed release
		err = du.updateStatus(currentRelease, "", PhaseSuperseded)
		if err != nil {
			du.logger.Error(err)
			return false
		}
	}

	if len(du.skippedPatchesIndexes) > 0 {
		for _, index := range du.skippedPatchesIndexes {
			release := du.releases[index]
			// skip not-deployed patches
			err = du.updateStatus(&release, "", PhaseSkipped)
			if err != nil {
				du.logger.Error(err)
				return false
			}
		}
	}

	return true
}

// PredictNextRelease runs prediction of the next release to deploy.
// it skips patch releases and save only the latest one
func (du *Updater[R]) PredictNextRelease() {
	for i, release := range du.releases {
		switch release.GetPhase() {
		case PhaseSuperseded, PhaseSuspended:
			// pass

		case PhasePending:
			du.processPendingRelease(i, release)

		case PhaseDeployed:
			du.currentDeployedReleaseIndex = i
		}

		if release.GetForce() {
			du.forcedReleaseIndex = i
		}

		if release.GetApplyNow() {
			du.appliedNowReleaseIndex = i
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
func (du *Updater[R]) ApplyForcedRelease() {
	if du.forcedReleaseIndex == -1 {
		return
	}
	forcedRelease := &(du.releases[du.forcedReleaseIndex])
	var currentRelease *R
	if du.currentDeployedReleaseIndex != -1 {
		currentRelease = &(du.releases[du.currentDeployedReleaseIndex])
	}

	du.logger.Warnf("Forcing release %s", (*forcedRelease).GetName())

	du.runReleaseDeploy(forcedRelease, currentRelease)

	// remove annotation
	err := du.kubeAPI.PatchReleaseAnnotations((*forcedRelease).GetName(), map[string]any{
		"release.deckhouse.io/force": nil,
	})
	if err != nil {
		du.logger.Errorf("patch force annotation: %s", err.Error())
	}

	// Outdate all previous releases
	for i, release := range du.releases {
		if i < du.forcedReleaseIndex {
			err := du.updateStatus(&release, "", PhaseSuperseded)
			if err != nil {
				du.logger.Error(err)
			}
		}
	}
}

func (du *Updater[R]) HasAppliedNowRelease() bool {
	return du.appliedNowReleaseIndex != -1
}

func (du *Updater[R]) ApplyAppliedNowRelease() {
	appliedNowRelease := &(du.releases[du.appliedNowReleaseIndex])
	var currentRelease *R

	if !du.checkAppliedNowConditions(appliedNowRelease) {
		return
	}

	if du.currentDeployedReleaseIndex != -1 {
		currentRelease = &(du.releases[du.currentDeployedReleaseIndex])
	}

	du.logger.Warnf("Applying release %s", (*appliedNowRelease).GetName())

	// all checks are passed, deploy release
	du.runReleaseDeploy(appliedNowRelease, currentRelease)

	// remove annotation
	err := du.kubeAPI.PatchReleaseAnnotations((*appliedNowRelease).GetName(), map[string]interface{}{
		"release.deckhouse.io/apply-now": nil,
	})
	if err != nil {
		du.logger.Errorf("patch apply now annotation: %s", err.Error())
	}

	// Outdate all previous releases
	for i, release := range du.releases {
		if i < du.appliedNowReleaseIndex {
			err := du.updateStatus(&release, "", PhaseSuperseded)
			if err != nil {
				du.logger.Error(err)
			}
		}
	}
}

// PredictedReleaseIsPatch shows if the predicted release is a patch with respect to the Deployed one
func (du *Updater[R]) PredictedReleaseIsPatch() bool {
	if du.predictedReleaseIsPatch != nil {
		return *du.predictedReleaseIsPatch
	}

	if du.currentDeployedReleaseIndex == -1 {
		du.predictedReleaseIsPatch = pointer.Bool(false)
		return false
	}

	if du.predictedReleaseIndex == -1 {
		du.predictedReleaseIsPatch = pointer.Bool(false)
		return false
	}

	current := du.releases[du.currentDeployedReleaseIndex]
	predicted := du.releases[du.predictedReleaseIndex]

	if current.GetVersion().Major() != predicted.GetVersion().Major() {
		du.predictedReleaseIsPatch = pointer.Bool(false)
		return false
	}

	if current.GetVersion().Minor() != predicted.GetVersion().Minor() {
		du.predictedReleaseIsPatch = pointer.Bool(false)
		return false
	}

	du.predictedReleaseIsPatch = pointer.Bool(true)
	return true
}

func (du *Updater[R]) processPendingRelease(index int, release R) {
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
		du.skippedPatchesIndexes = append(du.skippedPatchesIndexes, du.predictedReleaseIndex)
	}

	// release is predicted to be Deployed
	du.predictedReleaseIndex = index
}

func (du *Updater[R]) patchInitialStatus(release R) R {
	if release.GetPhase() != "" {
		return release
	}
	release.SetApprovedStatus(true)

	err := du.updateStatus(&release, "", PhasePending)
	if err != nil {
		du.logger.Error(err)
	}

	return release
}

func (du *Updater[R]) patchSuspendedStatus(release R) R {
	if !release.GetSuspend() {
		return release
	}

	err := du.kubeAPI.PatchReleaseAnnotations(release.GetName(), map[string]any{
		"release.deckhouse.io/suspended": nil,
	})
	if err != nil {
		du.logger.Error("patch suspended annotation:", err)
	}

	err = du.updateStatus(&release, "", PhaseSuspended)
	if err != nil {
		du.logger.Error(err)
	}

	return release
}

// patch manual Pending release if update mode was changed
func (du *Updater[R]) patchManualRelease(release R) R {
	if release.GetPhase() != PhasePending {
		return release
	}

	if !du.inManualMode {
		return release
	}

	if !release.GetManuallyApproved() {
		release.SetApprovedStatus(false)
		du.totalPendingManualReleases++
	} else {
		release.SetApprovedStatus(true)
	}

	return release
}

// PrepareReleases fetches releases from snapshot and then:
//   - patch releases with empty status (just created)
//   - handle suspended releases (patch status and remove annotation)
//   - patch manual releases (change status)
func (du *Updater[R]) PrepareReleases(releases []R) {
	if len(releases) == 0 {
		return
	}

	for i, release := range releases {
		release = du.patchInitialStatus(release)

		release = du.patchSuspendedStatus(release)

		release = du.patchManualRelease(release)

		releases[i] = release
	}

	sort.Sort(ByVersion[R](releases))

	du.releases = releases
}

func (du *Updater[R]) checkReleaseRequirements(rl *R) bool {
	for key, value := range (*rl).GetRequirements() {
		passed, err := requirements.CheckRequirement(key, value)
		if !passed {
			msg := fmt.Sprintf("%q requirement for DeckhouseRelease %q not met: %s", key, (*rl).GetVersion(), err)
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

	return true
}

func (du *Updater[R]) updateStatus(release *R, msg, phase string) error {
	if phase == (*release).GetPhase() && msg == (*release).GetMessage() {
		return nil
	}

	return du.kubeAPI.UpdateReleaseStatus(*release, msg, phase)
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
	var release *R

	if du.predictedReleaseIndex != -1 {
		release = &du.releases[du.predictedReleaseIndex]
	}

	return du.kubeAPI.SaveReleaseData(release, du.releaseData)
}

// for applied now we check less conditions, then for minor release
// - Release requirements
// - Disruptions
// - Deckhouse pod is ready
func (du *Updater[R]) checkAppliedNowConditions(appliedNowRelease *R) bool {
	// check: release requirements (hard lock)
	passed := du.checkReleaseRequirements(appliedNowRelease)
	if !passed {
		du.metricsUpdater.ReleaseBlocked((*appliedNowRelease).GetName(), "requirement")
		du.logger.Warnf("Release %s requirements are not met", (*appliedNowRelease).GetName())
		return false
	}

	// check: release disruptions (hard lock)
	passed = du.checkReleaseDisruptions(appliedNowRelease)
	if !passed {
		du.metricsUpdater.ReleaseBlocked((*appliedNowRelease).GetName(), "disruption")
		du.logger.Warnf("Release %s disruption approval required", (*appliedNowRelease).GetName())
		return false
	}

	// check: Deckhouse pod is ready
	if !du.deckhousePodIsReady {
		du.logger.Info("Deckhouse is not ready. Skipping upgrade")
		err := du.updateStatus(appliedNowRelease, "Waiting for Deckhouse pod to be ready", PhasePending)
		if err != nil {
			du.logger.Error(err)
		}
		return false
	}

	return true
}

func (du *Updater[R]) GetPredictedReleaseIndex() int {
	return du.predictedReleaseIndex
}

func (du *Updater[R]) SetMode(mode string) {
	du.inManualMode = mode == "Manual"
}
