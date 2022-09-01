/*
Copyright 2022 Flant JSC

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
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	"github.com/deckhouse/deckhouse/modules/020-deckhouse/hooks/internal/apis/v1alpha1"
)

const (
	metricReleasesGroup = "d8_releases"
)

type DeckhouseUpdater struct {
	now          time.Time
	inManualMode bool

	// probably we have to change to interfaces but later
	input *go_hook.HookInput

	// don't modify releases order, logic is based on this sorted slice
	releases                   []DeckhouseRelease
	totalPendingManualReleases int

	predictedReleaseIndex       int
	skippedPatchesIndexes       []int
	currentDeployedReleaseIndex int
	forcedReleaseIndex          int
	predictedReleaseIsPatch     *bool

	deckhousePodIsReady      bool
	deckhouseIsBootstrapping bool

	releaseData        DeckhouseReleaseData
	notificationConfig *NotificationConfig
}

func NewDeckhouseUpdater(input *go_hook.HookInput, mode string, data DeckhouseReleaseData, podIsReady, isBootstrapping bool) *DeckhouseUpdater {
	nConfig := ParseNotificationConfigFromValues(input)
	now := time.Now().UTC()
	if os.Getenv("D8_IS_TESTS_ENVIRONMENT") != "" {
		now = time.Date(2021, 01, 01, 13, 30, 00, 00, time.UTC)
	}
	return &DeckhouseUpdater{
		now:                         now,
		inManualMode:                mode == "Manual",
		input:                       input,
		predictedReleaseIndex:       -1,
		currentDeployedReleaseIndex: -1,
		forcedReleaseIndex:          -1,
		skippedPatchesIndexes:       make([]int, 0),
		deckhousePodIsReady:         podIsReady,
		deckhouseIsBootstrapping:    isBootstrapping,
		releaseData:                 data,
		notificationConfig:          nConfig,
	}
}

// for patch we check less conditions, then for minor release
// - Canary settings
// - Requirements
// - Disruptions
func (du *DeckhouseUpdater) checkPatchReleaseConditions(predictedRelease *DeckhouseRelease) bool {
	// check: canary settings
	if predictedRelease.ApplyAfter != nil {
		if du.now.Before(*predictedRelease.ApplyAfter) {
			du.input.LogEntry.Infof("Release %s is postponed by canary process. Waiting", predictedRelease.Name)
			du.updateStatus(predictedRelease, fmt.Sprintf("Release is postponed until: %s", predictedRelease.ApplyAfter.Format(time.RFC822)), v1alpha1.PhasePending)
			return false
		}
	}

	// TODO: maybe we should remove this check for a patch release
	// check: release requirements
	passed := du.checkReleaseRequirements(predictedRelease)
	if !passed {
		du.input.MetricsCollector.Set("d8_release_blocked", 1, map[string]string{"name": predictedRelease.Name, "reason": "requirement"}, metrics.WithGroup(metricReleasesGroup))
		du.input.LogEntry.Warnf("Release %s requirements are not met", predictedRelease.Name)
		return false
	}

	// check: release disruptions
	passed = du.checkReleaseDisruptions(predictedRelease)
	if !passed {
		du.input.MetricsCollector.Set("d8_release_blocked", 1, map[string]string{"name": predictedRelease.Name, "reason": "disruption"}, metrics.WithGroup(metricReleasesGroup))
		du.input.LogEntry.Warnf("Release %s disruption approval required", predictedRelease.Name)
		return false
	}

	return true
}

func (du *DeckhouseUpdater) checkReleaseNotification(predictedRelease *DeckhouseRelease, updateWindows update.Windows) bool {
	if du.inManualMode || du.releaseData.Notified {
		return true
	}

	var applyTimeChanged bool
	predictedReleaseApplyTime := du.predictedReleaseApplyTime(predictedRelease)
	if du.notificationConfig.MinimalNotificationTime.Duration > 0 {
		minApplyTime := du.now.Add(du.notificationConfig.MinimalNotificationTime.Duration)
		if minApplyTime.Before(predictedReleaseApplyTime) {
			minApplyTime = predictedReleaseApplyTime
		}

		predictedReleaseApplyTime = minApplyTime
		applyTimeChanged = true
	}
	releaseApplyTime := updateWindows.NextAllowedTime(predictedReleaseApplyTime)

	version := fmt.Sprintf("%d.%d", predictedRelease.Version.Major(), predictedRelease.Version.Minor())
	msg := fmt.Sprintf("New Deckhouse Release %s is available. Release will be applied at: %s", version, releaseApplyTime.Format(time.RFC850))
	data := webhookData{
		Version:       fmt.Sprintf("%d.%d", predictedRelease.Version.Major(), predictedRelease.Version.Minor()),
		Requirements:  predictedRelease.Requirements,
		ChangelogLink: predictedRelease.ChangelogLink,
		ApplyTime:     releaseApplyTime.Format(time.RFC3339),
		Message:       msg,
	}
	err := sendWebhookNotification(du.notificationConfig.WebhookURL, data)
	if err != nil {
		du.input.LogEntry.Errorf("Send deckhouse release notification failed: %s", err)
		return false
	}

	du.changeNotifiedFlag(true)
	if applyTimeChanged {
		patch := map[string]interface{}{
			"spec": map[string]interface{}{
				"applyAfter": releaseApplyTime,
			},
		}
		du.input.PatchCollector.MergePatch(patch, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", predictedRelease.Name)
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
func (du *DeckhouseUpdater) checkMinorReleaseConditions(predictedRelease *DeckhouseRelease, updateWindows update.Windows) bool {
	// check: release requirements (hard lock)
	passed := du.checkReleaseRequirements(predictedRelease)
	if !passed {
		du.input.MetricsCollector.Set("d8_release_blocked", 1, map[string]string{"name": predictedRelease.Name, "reason": "requirement"}, metrics.WithGroup(metricReleasesGroup))
		du.input.LogEntry.Warnf("Release %s requirements are not met", predictedRelease.Name)
		return false
	}

	// check: release disruptions (hard lock)
	passed = du.checkReleaseDisruptions(predictedRelease)
	if !passed {
		du.input.MetricsCollector.Set("d8_release_blocked", 1, map[string]string{"name": predictedRelease.Name, "reason": "disruption"}, metrics.WithGroup(metricReleasesGroup))
		du.input.LogEntry.Warnf("Release %s disruption approval required", predictedRelease.Name)
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
	if predictedRelease.CooldownUntil != nil {
		if du.now.Before(*predictedRelease.CooldownUntil) {
			du.input.LogEntry.Infof("Release %s in cooldown", predictedRelease.Name)
			du.updateStatus(predictedRelease, fmt.Sprintf("Release is in cooldown until: %s", predictedRelease.CooldownUntil.Format(time.RFC822)), v1alpha1.PhasePending)
			return false
		}
	}

	// check: canary settings
	if predictedRelease.ApplyAfter != nil {
		if du.now.Before(*predictedRelease.ApplyAfter) {
			du.input.LogEntry.Infof("Release %s is postponed by canary process. Waiting", predictedRelease.Name)
			du.updateStatus(predictedRelease, fmt.Sprintf("Release is postponed until: %s", predictedRelease.ApplyAfter.Format(time.RFC822)), v1alpha1.PhasePending)
			return false
		}
	}

	if du.inManualMode {
		// check: release is approved
		if !predictedRelease.Status.Approved {
			du.input.LogEntry.Infof("Release %s is waiting for manual approval", predictedRelease.Name)
			du.input.MetricsCollector.Set("d8_release_waiting_manual", float64(du.totalPendingManualReleases), map[string]string{"name": predictedRelease.Name}, metrics.WithGroup(metricReleasesGroup))
			du.updateStatus(predictedRelease, "Waiting for manual approval", v1alpha1.PhasePending)
			return false
		}
	} else {
		// check: update windows for Auto mode
		if len(updateWindows) > 0 {
			applyTime := du.predictedReleaseApplyTime(predictedRelease)
			updatePermitted := updateWindows.IsAllowed(applyTime)
			if !updatePermitted {
				du.input.LogEntry.Info("Deckhouse update does not get into update windows. Skipping")
				du.updateStatus(predictedRelease, "Release is waiting for update window", v1alpha1.PhasePending)
				return false
			}
		}
	}

	// check: Deckhouse pod is ready
	if !du.deckhousePodIsReady {
		du.input.LogEntry.Info("Deckhouse is not ready. Skipping upgrade")
		du.updateStatus(predictedRelease, "Waiting for Deckhouse pod to be ready", v1alpha1.PhasePending)
		return false
	}

	return true
}

// ApplyPredictedRelease applies predicted release, checks everything:
//   - Deckhouse is ready (except patch)
//   - Canary settings
//   - Manual approving
//   - Release requirements
func (du *DeckhouseUpdater) ApplyPredictedRelease(updateWindows update.Windows) {
	if du.predictedReleaseIndex == -1 {
		return // has no predicted release
	}

	var currentRelease *DeckhouseRelease

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

func (du *DeckhouseUpdater) predictedRelease() *DeckhouseRelease {
	if du.predictedReleaseIndex == -1 {
		return nil // has no predicted release
	}

	predictedRelease := &(du.releases[du.predictedReleaseIndex])

	return predictedRelease
}

func (du *DeckhouseUpdater) predictedReleaseApplyTime(predictedRelease *DeckhouseRelease) time.Time {
	if predictedRelease.ApplyAfter != nil {
		return *predictedRelease.ApplyAfter
	}

	return du.now
}

func (du *DeckhouseUpdater) checkReleaseDisruptions(rl *DeckhouseRelease) bool {
	dMode, ok := du.input.Values.GetOk("deckhouse.update.disruptionApprovalMode")
	if !ok || dMode.String() == "Auto" {
		return true
	}

	for _, key := range rl.Disruptions {
		hasDisruptionUpdate, reason := requirements.HasDisruption(key)
		if hasDisruptionUpdate {
			if !rl.HasDisruptionApprovedAnnotation {
				msg := fmt.Sprintf("Release requires disruption approval (`kubectl annotate DeckhouseRelease %s release.deckhouse.io/disruption-approved=true`): %s", rl.Name, reason)
				du.updateStatus(rl, msg, v1alpha1.PhasePending)
				return false
			}
		}
	}

	return true
}

func (du *DeckhouseUpdater) ReleasesCount() int {
	return len(du.releases)
}

func (du *DeckhouseUpdater) InManualMode() bool {
	return du.inManualMode
}

func (du *DeckhouseUpdater) runReleaseDeploy(predictedRelease, currentRelease *DeckhouseRelease) {
	du.input.LogEntry.Infof("Applying release %s", predictedRelease.Name)

	repo := du.input.Values.Get("global.modulesImages.registry").String()

	du.ChangeUpdatingFlag(true)
	du.changeNotifiedFlag(false)

	// patch deckhouse deployment is faster then set internal values and then upgrade by helm
	// we can set "deckhouse.internal.currentReleaseImageName" value but lets left it this way
	du.input.PatchCollector.Filter(func(u *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		var depl appsv1.Deployment
		err := sdk.FromUnstructured(u, &depl)
		if err != nil {
			return nil, err
		}

		depl.Spec.Template.Spec.Containers[0].Image = repo + ":" + predictedRelease.Version.Original()

		return sdk.ToUnstructured(&depl)
	}, "apps/v1", "Deployment", "d8-system", "deckhouse")

	du.updateStatus(predictedRelease, "", v1alpha1.PhaseDeployed, true)

	if currentRelease != nil {
		du.updateStatus(currentRelease, "Last Deployed release outdated", v1alpha1.PhaseOutdated)
	}

	if len(du.skippedPatchesIndexes) > 0 {
		for _, index := range du.skippedPatchesIndexes {
			release := du.releases[index]
			du.updateStatus(&release, "Skipped because of new patches", v1alpha1.PhaseOutdated, true)
		}
	}
}

// PredictNextRelease runs prediction of the next release to deploy.
// it skips patch releases and save only the latest one
func (du *DeckhouseUpdater) PredictNextRelease() {
	for i, release := range du.releases {
		switch release.Status.Phase {
		case v1alpha1.PhaseOutdated, v1alpha1.PhaseSuspended:
			// pass

		case v1alpha1.PhasePending:
			du.processPendingRelease(i, release)

		case v1alpha1.PhaseDeployed:
			du.currentDeployedReleaseIndex = i
		}

		if release.HasForceAnnotation {
			du.forcedReleaseIndex = i
		}
	}
}

// LastReleaseDeployed returns the equality of the latest existed release with the latest deployed
func (du *DeckhouseUpdater) LastReleaseDeployed() bool {
	return du.currentDeployedReleaseIndex == len(du.releases)-1
}

// HasForceRelease check the existence of the forced release
func (du *DeckhouseUpdater) HasForceRelease() bool {
	return du.forcedReleaseIndex != -1
}

// ApplyForcedRelease deploys forced release without any checks (windows, requirements, approvals and so on)
func (du *DeckhouseUpdater) ApplyForcedRelease() {
	if du.forcedReleaseIndex == -1 {
		return
	}
	forcedRelease := &(du.releases[du.forcedReleaseIndex])
	var currentRelease *DeckhouseRelease
	if du.currentDeployedReleaseIndex != -1 {
		currentRelease = &(du.releases[du.currentDeployedReleaseIndex])
	}

	du.input.LogEntry.Warnf("Forcing release %s", forcedRelease.Name)

	du.runReleaseDeploy(forcedRelease, currentRelease)

	annotationsPatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"release.deckhouse.io/force": nil,
			},
		},
	}
	// remove annotation
	du.input.PatchCollector.MergePatch(annotationsPatch, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", forcedRelease.Name)

	// Outdate all previous releases

	for i, release := range du.releases {
		if i < du.forcedReleaseIndex {
			du.updateStatus(&release, "", v1alpha1.PhaseOutdated, true)
		}
	}
}

// PredictedReleaseIsPatch shows if the predicted release is a patch with respect to the Deployed one
func (du *DeckhouseUpdater) PredictedReleaseIsPatch() bool {
	if du.predictedReleaseIsPatch != nil {
		return *du.predictedReleaseIsPatch
	}

	if du.currentDeployedReleaseIndex == -1 {
		du.predictedReleaseIsPatch = pointer.BoolPtr(false)
		return false
	}

	if du.predictedReleaseIndex == -1 {
		du.predictedReleaseIsPatch = pointer.BoolPtr(false)
		return false
	}

	current := du.releases[du.currentDeployedReleaseIndex]
	predicted := du.releases[du.predictedReleaseIndex]

	if current.Version.Major() != predicted.Version.Major() {
		du.predictedReleaseIsPatch = pointer.BoolPtr(false)
		return false
	}

	if current.Version.Minor() != predicted.Version.Minor() {
		du.predictedReleaseIsPatch = pointer.BoolPtr(false)
		return false
	}

	du.predictedReleaseIsPatch = pointer.BoolPtr(true)
	return true
}

func (du *DeckhouseUpdater) processPendingRelease(index int, release DeckhouseRelease) {
	// check: already has predicted release and current is a patch
	if du.predictedReleaseIndex >= 0 {
		previousPredictedRelease := du.releases[du.predictedReleaseIndex]
		if previousPredictedRelease.Version.Major() != release.Version.Major() {
			return
		}

		if previousPredictedRelease.Version.Minor() != release.Version.Minor() {
			return
		}
		// it's a patch for predicted release, continue
		du.skippedPatchesIndexes = append(du.skippedPatchesIndexes, du.predictedReleaseIndex)
	}

	// release is predicted to be Deployed
	du.predictedReleaseIndex = index
}

func (du *DeckhouseUpdater) patchInitialStatus(release DeckhouseRelease) DeckhouseRelease {
	if release.Status.Phase != "" {
		return release
	}

	du.updateStatus(&release, "", v1alpha1.PhasePending)

	return release
}

func (du *DeckhouseUpdater) patchSuspendedStatus(release DeckhouseRelease) DeckhouseRelease {
	if !release.HasSuspendAnnotation {
		return release
	}

	annotationsPatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"release.deckhouse.io/suspended": nil,
			},
		},
	}

	du.input.PatchCollector.MergePatch(annotationsPatch, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", release.Name)
	du.updateStatus(&release, "Release is suspended", v1alpha1.PhaseSuspended, false)

	return release
}

func (du *DeckhouseUpdater) patchManualRelease(release DeckhouseRelease) DeckhouseRelease {
	if release.Status.Phase != v1alpha1.PhasePending {
		return release
	}

	var statusChanged bool

	statusPatch := StatusPatch{
		Phase:          release.Status.Phase,
		Approved:       release.Status.Approved,
		TransitionTime: du.now,
	}

	// check and set .status.approved for pending releases
	if du.inManualMode && !release.ManuallyApproved {
		statusPatch.Approved = false
		statusPatch.Message = "Release is waiting for manual approval"
		du.totalPendingManualReleases++
		if release.Status.Approved {
			statusChanged = true
		}
	} else {
		statusPatch.Approved = true
		if !release.Status.Approved {
			statusChanged = true
		}
	}

	if statusChanged {
		du.input.PatchCollector.MergePatch(statusPatch, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", release.Name, object_patch.WithSubresource("/status"))
		release.Status.Approved = statusPatch.Approved
	}

	return release
}

// FetchAndPrepareReleases fetches releases from snapshot and then:
//   - patch releases with empty status (just created)
//   - handle suspended releases (patch status and remove annotation)
//   - patch manual releases (change status)
func (du *DeckhouseUpdater) FetchAndPrepareReleases(snap []go_hook.FilterResult) {
	if len(snap) == 0 {
		return
	}

	releases := make([]DeckhouseRelease, 0, len(snap))

	for _, rl := range snap {
		release := rl.(DeckhouseRelease)

		release = du.patchInitialStatus(release)

		release = du.patchSuspendedStatus(release)

		release = du.patchManualRelease(release)

		releases = append(releases, release)
	}

	sort.Sort(ByVersion(releases))

	du.releases = releases
}

func (du *DeckhouseUpdater) checkReleaseRequirements(rl *DeckhouseRelease) bool {
	for key, value := range rl.Requirements {
		passed, err := requirements.CheckRequirement(key, value, du.input.Values)
		if !passed {
			msg := fmt.Sprintf("%q requirement for DeckhouseRelease %q not met: %s", key, rl.Version, err)
			if errors.Is(err, requirements.ErrNotRegistered) {
				du.input.LogEntry.Error(err)
				msg = fmt.Sprintf("%q requirement not registered", key)
			}
			du.updateStatus(rl, msg, v1alpha1.PhasePending, false)
			return false
		}
	}

	return true
}

func (du *DeckhouseUpdater) updateStatus(release *DeckhouseRelease, msg, phase string, approvedFlag ...bool) {
	approved := release.Status.Approved
	if len(approvedFlag) > 0 {
		approved = approvedFlag[0]
	}

	if phase == release.Status.Phase && msg == release.Status.Message && approved == release.Status.Approved {
		return
	}

	st := StatusPatch{
		Phase:          phase,
		Message:        msg,
		Approved:       approved,
		TransitionTime: time.Now().UTC(),
	}
	du.input.PatchCollector.MergePatch(st, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", release.Name, object_patch.WithSubresource("/status"))

	release.Status.Phase = phase
	release.Status.Message = msg
	release.Status.Approved = approved
}

func (du *DeckhouseUpdater) ChangeUpdatingFlag(fl bool) {
	if du.releaseData.IsUpdating == fl {
		return
	}

	du.releaseData.IsUpdating = fl
	du.createReleaseDataCM()
}

func (du *DeckhouseUpdater) changeNotifiedFlag(fl bool) {
	if du.releaseData.Notified == fl {
		return
	}

	du.releaseData.Notified = fl
	du.createReleaseDataCM()
}

func (du *DeckhouseUpdater) createReleaseDataCM() {
	predicted := du.predictedRelease()
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
			"version":    predicted.Version.String(),
			"isUpdating": strconv.FormatBool(du.releaseData.IsUpdating),
			"notified":   strconv.FormatBool(du.releaseData.Notified),
		},
	}

	du.input.PatchCollector.Create(cm, object_patch.UpdateIfExists())
}
