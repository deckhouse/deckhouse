package update

import (
	"fmt"
	"sort"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/modules/020-deckhouse/hooks/internal/apis/v1alpha1"
)

type deckhouseUpdater struct {
	now          time.Time
	inManualMode bool

	patchCollector patchCollector

	// don't modify releases order, logic is based on this sorted slice
	releases                   []DeckhouseRelease
	totalPendingManualReleases int

	predictedReleaseIndex       int
	skippedPatchesIndexes       []int
	currentDeployedReleaseIndex int
	forcedReleaseIndex          int

	deckhousePodIsReady      bool
	deckhouseIsBootstrapping bool
}

func NewDeckhouseUpdater(mode string, podIsReady, isBootstrapping bool) *deckhouseUpdater {
	return &deckhouseUpdater{
		now:                         time.Now().UTC(),
		inManualMode:                mode == "Manual",
		predictedReleaseIndex:       -1,
		currentDeployedReleaseIndex: -1,
		forcedReleaseIndex:          -1,
		skippedPatchesIndexes:       make([]int, 0),
		deckhousePodIsReady:         podIsReady,
		deckhouseIsBootstrapping:    isBootstrapping,
	}
}

// ApplyPredictedRelease applies predicted release, checks everything:
//   - Deckhouse is ready (except patch)
//   - Canary settings
//   - Manual approving
//   - Release requirements
func (du *deckhouseUpdater) ApplyPredictedRelease(input *go_hook.HookInput) {
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
		du.runReleaseDeploy(input, predictedRelease, currentRelease)
		return
	}

	// check: only for minor versions (Ignore patches)
	if !du.PredictedReleaseIsPatch() {
		// check: release cooldown
		if predictedRelease.CooldownUntil != nil {
			if du.now.Before(*predictedRelease.CooldownUntil) {
				input.LogEntry.Infof("Release %s in cooldown", predictedRelease.Name)
				du.updateStatus(input, predictedRelease, fmt.Sprintf("Release is in cooldown until: %s", predictedRelease.CooldownUntil.Format(time.RFC822)), v1alpha1.PhasePending)
				return
			}
		}

		// check: Deckhouse pod is ready
		if !du.deckhousePodIsReady {
			input.LogEntry.Info("Deckhouse is not ready. Skipping upgrade")
			du.updateStatus(input, predictedRelease, "Waiting for Deckhouse pod to be ready", v1alpha1.PhasePending)
			return
		}
	}

	// check: canary settings
	if predictedRelease.ApplyAfter != nil {
		if du.now.Before(*predictedRelease.ApplyAfter) {
			input.LogEntry.Infof("Release %s is postponed by canary process. Waiting", predictedRelease.Name)
			du.updateStatus(input, predictedRelease, fmt.Sprintf("Waiting for canary apply time: %s", predictedRelease.ApplyAfter.Format(time.RFC822)), v1alpha1.PhasePending)
			return
		}
	}

	// check: release is approved or it's a patch
	if !predictedRelease.Status.Approved && !du.PredictedReleaseIsPatch() {
		input.LogEntry.Infof("Release %s is waiting for manual approval", predictedRelease.Name)
		input.MetricsCollector.Set("d8_release_waiting_manual", float64(du.totalPendingManualReleases), map[string]string{"name": predictedRelease.Name}, metrics.WithGroup(metricReleasesGroup))
		du.updateStatus(input, predictedRelease, "Waiting for manual approval", v1alpha1.PhasePending)
		return
	}

	// check: release requirements
	passed := du.checkReleaseRequirements(input, predictedRelease)
	if !passed {
		input.MetricsCollector.Set("d8_release_blocked", 1, map[string]string{"name": predictedRelease.Name, "reason": "requirement"}, metrics.WithGroup(metricReleasesGroup))
		input.LogEntry.Warningf("Release %s requirements are not met", predictedRelease.Name)
		return
	}

	// check: release disruptions
	passed = du.checkReleaseDisruptions(input, predictedRelease)
	if !passed {
		input.MetricsCollector.Set("d8_release_blocked", 1, map[string]string{"name": predictedRelease.Name, "reason": "disruption"}, metrics.WithGroup(metricReleasesGroup))
		input.LogEntry.Warningf("Release %s disruption approval required", predictedRelease.Name)
		return
	}

	// all checks are passed, deploy release
	du.runReleaseDeploy(input, predictedRelease, currentRelease)
}

func (du *deckhouseUpdater) PredictedRelease() *DeckhouseRelease {
	if du.predictedReleaseIndex == -1 {
		return nil // has no predicted release
	}

	predictedRelease := &(du.releases[du.predictedReleaseIndex])

	return predictedRelease
}

func (du *deckhouseUpdater) checkReleaseDisruptions(input *go_hook.HookInput, rl *DeckhouseRelease) bool {
	dMode, ok := input.Values.GetOk("deckhouse.update.disruptionApprovalMode")
	if !ok || dMode.String() == "Auto" {
		return true
	}

	for _, key := range rl.Disruptions {
		hasDisruptionUpdate, reason := requirements.HasDisruption(key)
		if hasDisruptionUpdate {
			if !rl.HasDisruptionApprovedAnnotation {
				msg := fmt.Sprintf("Release requires disruption approval (`kubectl annotate DeckhouseRelease %s release.deckhouse.io/disruption-approved=true`): %s", rl.Name, reason)
				du.updateStatus(input, rl, msg, v1alpha1.PhasePending)
				return false
			}
		}
	}

	return true
}

func (du *deckhouseUpdater) runReleaseDeploy(input *go_hook.HookInput, predictedRelease, currentRelease *DeckhouseRelease) {
	input.LogEntry.Infof("Applying release %s", predictedRelease.Name)

	repo := input.Values.Get("global.modulesImages.registry").String()

	createUpdatingCM(input, predictedRelease.Version.String())

	// patch deckhouse deployment is faster then set internal values and then upgrade by helm
	// we can set "deckhouse.internal.currentReleaseImageName" value but lets left it this way
	input.PatchCollector.Filter(func(u *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		var depl appsv1.Deployment
		err := sdk.FromUnstructured(u, &depl)
		if err != nil {
			return nil, err
		}

		depl.Spec.Template.Spec.Containers[0].Image = repo + ":" + predictedRelease.Version.Original()

		return sdk.ToUnstructured(&depl)
	}, "apps/v1", "Deployment", "d8-system", "deckhouse")

	du.updateStatus(input, predictedRelease, "", v1alpha1.PhaseDeployed, true)

	if currentRelease != nil {
		du.updateStatus(input, currentRelease, "Last Deployed release outdated", v1alpha1.PhaseOutdated)
	}

	if len(du.skippedPatchesIndexes) > 0 {
		for _, index := range du.skippedPatchesIndexes {
			release := du.releases[index]
			du.updateStatus(input, &release, "Skipped because of new patches", v1alpha1.PhaseOutdated, true)
		}
	}
}

// PredictNextRelease runs prediction of the next release to deploy.
// it skips patch releases and save only the latest one
func (du *deckhouseUpdater) PredictNextRelease() {
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
func (du *deckhouseUpdater) LastReleaseDeployed() bool {
	return du.currentDeployedReleaseIndex == len(du.releases)-1
}

// HasForceRelease check the existence of the forced release
func (du *deckhouseUpdater) HasForceRelease() bool {
	return du.forcedReleaseIndex != -1
}

// ApplyForcedRelease deploys forced release without any checks (windows, requirements, approvals and so on)
func (du *deckhouseUpdater) ApplyForcedRelease(input *go_hook.HookInput) {
	if du.forcedReleaseIndex == -1 {
		return
	}
	forcedRelease := &(du.releases[du.forcedReleaseIndex])
	var currentRelease *DeckhouseRelease
	if du.currentDeployedReleaseIndex != -1 {
		currentRelease = &(du.releases[du.currentDeployedReleaseIndex])
	}

	input.LogEntry.Warnf("Forcing release %s", forcedRelease.Name)

	du.runReleaseDeploy(input, forcedRelease, currentRelease)

	annotationsPatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"release.deckhouse.io/force": nil,
			},
		},
	}
	// remove annotation
	input.PatchCollector.MergePatch(annotationsPatch, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", forcedRelease.Name)

	// Outdate all previous releases

	for i, release := range du.releases {
		if i < du.forcedReleaseIndex {
			du.updateStatus(input, &release, "", v1alpha1.PhaseOutdated, true)
		}
	}
}

// PredictedReleaseIsPatch shows if the predicted release is a patch with respect to the Deployed one
func (du *deckhouseUpdater) PredictedReleaseIsPatch() bool {
	if du.currentDeployedReleaseIndex == -1 {
		return false
	}

	if du.predictedReleaseIndex == -1 {
		return false
	}

	current := du.releases[du.currentDeployedReleaseIndex]
	predicted := du.releases[du.predictedReleaseIndex]

	if current.Version.Major() != predicted.Version.Major() {
		return false
	}

	if current.Version.Minor() != predicted.Version.Minor() {
		return false
	}

	return true
}

func (du *deckhouseUpdater) processPendingRelease(index int, release DeckhouseRelease) {
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

func (du *deckhouseUpdater) patchInitialStatus(input *go_hook.HookInput, release DeckhouseRelease) DeckhouseRelease {
	if release.Status.Phase != "" {
		return release
	}

	du.updateStatus(input, &release, "", v1alpha1.PhasePending)

	return release
}

func (du *deckhouseUpdater) patchSuspendedStatus(input *go_hook.HookInput, release DeckhouseRelease) DeckhouseRelease {
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

	input.PatchCollector.MergePatch(annotationsPatch, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", release.Name)
	du.updateStatus(input, &release, "Release is suspended", v1alpha1.PhaseSuspended, false)

	return release
}

func (du *deckhouseUpdater) patchManualRelease(input *go_hook.HookInput, release DeckhouseRelease) DeckhouseRelease {
	if release.Status.Phase != v1alpha1.PhasePending {
		return release
	}

	var statusChanged bool

	statusPatch := statusPatch{
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
		input.PatchCollector.MergePatch(statusPatch, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", release.Name, object_patch.WithSubresource("/status"))
		release.Status.Approved = statusPatch.Approved
	}

	return release
}

// FetchAndPrepareReleases fetches releases from snapshot and then:
//   - patch releases with empty status (just created)
//   - handle suspended releases (patch status and remove annotation)
//   - patch manual releases (change status)
func (du *deckhouseUpdater) FetchAndPrepareReleases(input *go_hook.HookInput) {
	snap := input.Snapshots["releases"]
	if len(snap) == 0 {
		return
	}

	releases := make([]DeckhouseRelease, 0, len(snap))

	for _, rl := range snap {
		release := rl.(DeckhouseRelease)

		release = du.patchInitialStatus(input, release)

		release = du.patchSuspendedStatus(input, release)

		release = du.patchManualRelease(input, release)

		releases = append(releases, release)
	}

	sort.Sort(byVersion(releases))

	du.releases = releases
}

func (du *deckhouseUpdater) checkReleaseRequirements(input *go_hook.HookInput, rl *DeckhouseRelease) bool {
	for key, value := range rl.Requirements {
		passed, err := requirements.CheckRequirement(key, value, input.Values)
		if !passed {
			msg := fmt.Sprintf("%q requirement for DeckhouseRelease %q not met: %s", key, rl.Version, err)
			if errors.Is(err, requirements.ErrNotRegistered) {
				input.LogEntry.Error(err)
				msg = fmt.Sprintf("%q requirement not registered", key)
			}
			du.updateStatus(input, rl, msg, v1alpha1.PhasePending, false)
			return false
		}
	}

	return true
}

func (du *deckhouseUpdater) updateStatus(input *go_hook.HookInput, release *DeckhouseRelease, msg, phase string, approvedFlag ...bool) {
	approved := release.Status.Approved
	if len(approvedFlag) > 0 {
		approved = approvedFlag[0]
	}

	if phase == release.Status.Phase && msg == release.Status.Message && approved == release.Status.Approved {
		return
	}

	st := statusPatch{
		Phase:          phase,
		Message:        msg,
		Approved:       approved,
		TransitionTime: time.Now().UTC(),
	}
	du.patchCollector.MergePatch(st, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", release.Name, object_patch.WithSubresource("/status"))

	release.Status.Phase = phase
	release.Status.Message = msg
	release.Status.Approved = approved
}

type patchCollector interface {
	MergePatch(mergePatch interface{}, apiVersion, kind, namespace, name string, options ...object_patch.PatchOption)
}
