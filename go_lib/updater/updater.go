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
	"strconv"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

	// probably we have to change to interfaces but later
	input *go_hook.HookInput

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

	releaseUpdater ReleaseUpdater
	metricsUpdater MetricsUpdater
}

func NewUpdater[R Release](input *go_hook.HookInput, mode string, data DeckhouseReleaseData, podIsReady, isBootstrapping bool, releaseUpdater ReleaseUpdater, metricsUpdater MetricsUpdater) (*Updater[R], error) {
	nConfig, err := ParseNotificationConfigFromValues(input)
	if err != nil {
		return nil, fmt.Errorf("parsing notification config: %v", err)
	}
	now := time.Now().UTC()
	if os.Getenv("D8_IS_TESTS_ENVIRONMENT") != "" {
		now = time.Date(2021, 01, 01, 13, 30, 00, 00, time.UTC)
	}
	return &Updater[R]{
		now:                         now,
		inManualMode:                mode == "Manual",
		input:                       input,
		predictedReleaseIndex:       -1,
		currentDeployedReleaseIndex: -1,
		forcedReleaseIndex:          -1,
		appliedNowReleaseIndex:      -1,
		skippedPatchesIndexes:       make([]int, 0),
		deckhousePodIsReady:         podIsReady,
		deckhouseIsBootstrapping:    isBootstrapping,
		releaseData:                 data,
		notificationConfig:          nConfig,

		releaseUpdater: releaseUpdater,
		metricsUpdater: metricsUpdater,
	}, nil
}

// for patch we check less conditions, then for minor release
// - Canary settings
func (du *Updater[R]) checkPatchReleaseConditions(predictedRelease *R) bool {
	// check: canary settings
	if (*predictedRelease).GetApplyAfter() != nil {
		if du.now.Before(*(*predictedRelease).GetApplyAfter()) {
			du.input.LogEntry.Infof("Release %s is postponed by canary process. Waiting", (*predictedRelease).GetName())
			du.updateStatus(predictedRelease, fmt.Sprintf("Release is postponed until: %s", (*predictedRelease).GetApplyAfter().Format(time.RFC822)), PhasePending)
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
			du.input.LogEntry.Errorf("Send deckhouse release notification failed: %s", err)
			return false
		}
	}

	du.changeNotifiedFlag(true)
	if applyTimeChanged {
		patch := map[string]interface{}{
			"spec": map[string]interface{}{
				"applyAfter": releaseApplyTime,
			},
			"metadata": map[string]interface{}{
				"annotations": map[string]string{
					"release.deckhouse.io/notification-time-shift": "true",
				},
			},
		}
		du.input.PatchCollector.MergePatch(patch, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", (*predictedRelease).GetName())
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
		du.input.LogEntry.Warnf("Release %s requirements are not met", (*predictedRelease).GetName())
		return false
	}

	// check: release disruptions (hard lock)
	passed = du.checkReleaseDisruptions(predictedRelease)
	if !passed {
		du.metricsUpdater.ReleaseBlocked((*predictedRelease).GetName(), "disruption")
		du.input.LogEntry.Warnf("Release %s disruption approval required", (*predictedRelease).GetName())
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
			du.input.LogEntry.Infof("Release %s in cooldown", (*predictedRelease).GetName())
			du.updateStatus(predictedRelease, fmt.Sprintf("Release is in cooldown until: %s", (*predictedRelease).GetCooldownUntil().Format(time.RFC822)), PhasePending)
			return false
		}
	}

	// check: canary settings
	if (*predictedRelease).GetApplyAfter() != nil && !du.inManualMode {
		if du.now.Before(*(*predictedRelease).GetApplyAfter()) {
			du.input.LogEntry.Infof("Release %s is postponed by canary process. Waiting", (*predictedRelease).GetName())
			du.updateStatus(predictedRelease, fmt.Sprintf("Release is postponed until: %s", (*predictedRelease).GetApplyAfter().Format(time.RFC822)), PhasePending)
			return false
		}
	}

	if du.inManualMode {
		// check: release is approved in Manual mode
		if !(*predictedRelease).GetApprovedStatus() {
			du.input.LogEntry.Infof("Release %s is waiting for manual approval", (*predictedRelease).GetName())
			du.metricsUpdater.WaitingManual((*predictedRelease).GetName(), float64(du.totalPendingManualReleases))
			du.updateStatus(predictedRelease, waitingManualApprovalMsg, PhasePending)
			return false
		}
	} else {
		// check: update windows in Auto mode
		if len(updateWindows) > 0 {
			updatePermitted := updateWindows.IsAllowed(du.now)
			if !updatePermitted {
				applyTime := updateWindows.NextAllowedTime(du.now)
				du.input.LogEntry.Info("Deckhouse update does not get into update windows. Skipping")
				du.updateStatus(predictedRelease, fmt.Sprintf("Release is waiting for the update window: %s", applyTime.Format(time.RFC822)), PhasePending)
				return false
			}
		}
	}

	// check: Deckhouse pod is ready
	if !du.deckhousePodIsReady {
		du.input.LogEntry.Info("Deckhouse is not ready. Skipping upgrade")
		du.updateStatus(predictedRelease, "Waiting for Deckhouse pod to be ready", PhasePending)
		return false
	}

	return true
}

// ApplyPredictedRelease applies predicted release, checks everything:
//   - Deckhouse is ready (except patch)
//   - Canary settings
//   - Manual approving
//   - Release requirements
func (du *Updater[R]) ApplyPredictedRelease(updateWindows update.Windows) {
	if du.predictedReleaseIndex == -1 {
		return // has no predicted release
	}

	var currentRelease *R

	predictedRelease := &(du.releases[du.predictedReleaseIndex])

	if du.currentDeployedReleaseIndex != -1 {
		currentRelease = &(du.releases[du.currentDeployedReleaseIndex])
	}

	// if deckhouse pod has bootstrap image -> apply first release
	// doesn't matter which is update mode
	if du.deckhouseIsBootstrapping && len(du.releases) == 1 {
		du.runReleaseDeploy(predictedRelease, currentRelease)
		return
	}

	var readyForDeploy bool

	if du.PredictedReleaseIsPatch() {
		readyForDeploy = du.checkPatchReleaseConditions(predictedRelease)
	} else {
		readyForDeploy = du.checkMinorReleaseConditions(predictedRelease, updateWindows)
	}

	if !readyForDeploy {
		return
	}

	// all checks are passed, deploy release
	du.runReleaseDeploy(predictedRelease, currentRelease)
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
	dMode, ok := du.input.Values.GetOk("deckhouse.update.disruptionApprovalMode")
	if !ok || dMode.String() == "Auto" {
		return true
	}

	for _, key := range (*rl).GetDisruptions() {
		hasDisruptionUpdate, reason := requirements.HasDisruption(key)
		if hasDisruptionUpdate {
			if !(*rl).GetDisruptionApproved() {
				msg := fmt.Sprintf("Release requires disruption approval (`kubectl annotate DeckhouseRelease %s release.deckhouse.io/disruption-approved=true`): %s", (*rl).GetName(), reason)
				du.updateStatus(rl, msg, PhasePending)
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

func (du *Updater[R]) runReleaseDeploy(predictedRelease, currentRelease *R) {
	du.input.LogEntry.Infof("Applying release %s", (*predictedRelease).GetName())

	repo := du.input.Values.Get("global.modulesImages.registry.base").String()

	du.ChangeUpdatingFlag(true)
	du.changeNotifiedFlag(false)

	// patch deckhouse deployment is faster than set internal values and then upgrade by helm
	// we can set "deckhouse.internal.currentReleaseImageName" value but lets left it this way
	du.input.PatchCollector.Filter(func(u *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		var depl appsv1.Deployment
		err := sdk.FromUnstructured(u, &depl)
		if err != nil {
			return nil, err
		}

		depl.Spec.Template.Spec.Containers[0].Image = repo + ":" + (*predictedRelease).GetVersion().Original()

		return sdk.ToUnstructured(&depl)
	}, "apps/v1", "Deployment", "d8-system", "deckhouse")

	du.updateStatus(predictedRelease, "", PhaseDeployed)

	if currentRelease != nil {
		// skip last deployed release
		du.updateStatus(currentRelease, "", PhaseSuperseded)
	}

	if len(du.skippedPatchesIndexes) > 0 {
		for _, index := range du.skippedPatchesIndexes {
			release := du.releases[index]
			// skip not-deployed patches
			du.updateStatus(&release, "", PhaseSkipped)
		}
	}
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

	du.input.LogEntry.Warnf("Forcing release %s", (*forcedRelease).GetName())

	du.runReleaseDeploy(forcedRelease, currentRelease)

	annotationsPatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"release.deckhouse.io/force": nil,
			},
		},
	}
	// remove annotation
	du.input.PatchCollector.MergePatch(annotationsPatch, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", (*forcedRelease).GetName())

	// Outdate all previous releases

	for i, release := range du.releases {
		if i < du.forcedReleaseIndex {
			du.updateStatus(&release, "", PhaseSuperseded)
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

	du.input.LogEntry.Warnf("Applying release %s", (*appliedNowRelease).GetName())

	// all checks are passed, deploy release
	du.runReleaseDeploy(appliedNowRelease, currentRelease)

	annotationsPatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"release.deckhouse.io/apply-now": nil,
			},
		},
	}
	// remove annotation
	du.input.PatchCollector.MergePatch(annotationsPatch, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", (*appliedNowRelease).GetName())

	// Outdate all previous releases
	for i, release := range du.releases {
		if i < du.appliedNowReleaseIndex {
			du.updateStatus(&release, "", PhaseSuperseded)
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

	du.updateStatus(&release, "", PhasePending)

	return release
}

func (du *Updater[R]) patchSuspendedStatus(release R) R {
	if !release.GetSuspend() {
		return release
	}

	annotationsPatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"release.deckhouse.io/suspended": nil,
			},
		},
	}

	du.input.PatchCollector.MergePatch(annotationsPatch, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", release.GetName())
	du.updateStatus(&release, "", PhaseSuspended)

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

// FetchAndPrepareReleases fetches releases from snapshot and then:
//   - patch releases with empty status (just created)
//   - handle suspended releases (patch status and remove annotation)
//   - patch manual releases (change status)
func (du *Updater[R]) FetchAndPrepareReleases(snap []go_hook.FilterResult) {
	if len(snap) == 0 {
		return
	}

	releases := make([]R, 0, len(snap))

	for _, rl := range snap {
		release := rl.(R)

		release = du.patchInitialStatus(release)

		release = du.patchSuspendedStatus(release)

		release = du.patchManualRelease(release)

		releases = append(releases, release)
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
				du.input.LogEntry.Error(err)
				msg = fmt.Sprintf("%q requirement is not registered", key)
			}
			du.updateStatus(rl, msg, PhasePending)
			return false
		}
	}

	return true
}

func (du *Updater[R]) updateStatus(release *R, msg, phase string) {
	if phase == (*release).GetPhase() && msg == (*release).GetMessage() {
		return
	}

	du.releaseUpdater.UpdateStatus(*release, msg, phase)
}

func (du *Updater[R]) ChangeUpdatingFlag(fl bool) {
	if du.releaseData.IsUpdating == fl {
		return
	}

	du.releaseData.IsUpdating = fl
	du.createReleaseDataCM()
}

func (du *Updater[R]) changeNotifiedFlag(fl bool) {
	if du.releaseData.Notified == fl {
		return
	}

	du.releaseData.Notified = fl
	du.createReleaseDataCM()
}

func (du *Updater[R]) createReleaseDataCM() {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "d8-release-data",
			Namespace: "d8-system",
			Labels: map[string]string{
				"heritage": "deckhouse",
			},
		},
		Data: map[string]string{
			// current release is updating
			"isUpdating": strconv.FormatBool(du.releaseData.IsUpdating),
			// notification about next release is sent, flag will be reset when new release is deployed
			"notified": strconv.FormatBool(du.releaseData.Notified),
		},
	}

	du.input.PatchCollector.Create(cm, object_patch.UpdateIfExists())
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
		du.input.LogEntry.Warnf("Release %s requirements are not met", (*appliedNowRelease).GetName())
		return false
	}

	// check: release disruptions (hard lock)
	passed = du.checkReleaseDisruptions(appliedNowRelease)
	if !passed {
		du.metricsUpdater.ReleaseBlocked((*appliedNowRelease).GetName(), "disruption")
		du.input.LogEntry.Warnf("Release %s disruption approval required", (*appliedNowRelease).GetName())
		return false
	}

	// check: Deckhouse pod is ready
	if !du.deckhousePodIsReady {
		du.input.LogEntry.Info("Deckhouse is not ready. Skipping upgrade")
		du.updateStatus(appliedNowRelease, "Waiting for Deckhouse pod to be ready", PhasePending)
		return false
	}

	return true
}
