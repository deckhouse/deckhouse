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

package status

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
)

// External condition types — what the user sees on the Application resource.
//
// Every condition but Installed is gated on it: none is reported until
// Installed is True, and each is removed from the resource while Installed is
// not. The first install is therefore reported by Installed alone, and so is an
// application that lost a hard requirement, until it is installed again.
//
// While an application is being removed the projection bypasses the rules entirely and
// reports every condition here as False/Deleting.
const (
	// ConditionInstalled reflects whether the application's release is installed.
	// False while the first install is in progress (Pending) or blocked or failed
	// at one of: unmet requirements (RequirementsUnmet), download, load from filesystem,
	// settings validation, hooks, Helm manifest apply. The first True also
	// waits for the workloads to be ready. Sticky: once True it is never
	// retracted on a failure — failures surface on the other conditions —
	// only after a disable has taken the release down (see isDisabled).
	ConditionInstalled = "Installed"

	// ConditionUpdateInstalled reflects installing a new version over a
	// running application: False while the update is blocked (WaitingWindow,
	// RequirementsUnmet, SettingsInvalid), its preparation failed
	// (AttachFailed, LoadFromFilesystemFailed), it is being applied
	// (ApplyingManifests) or the new version's first run failed
	// (HookInitializationFailed, HookFailed, ManifestsApplyFailed,
	// ScalingForbidden); True once
	// the new version's own run succeeded.
	ConditionUpdateInstalled = "UpdateInstalled"

	// ConditionReady reflects user-facing readiness of the application.
	// Possible reasons: HookInitializationFailed, HookFailed,
	// ManifestsApplyFailed, ScalingForbidden, ReleaseDeleteFailed, ApplyingManifests,
	// SettingsChanged, Ready (when True).
	ConditionReady = "Ready"

	// ConditionScaled reflects the runtime scaling state of the application, owned by
	// the workload health monitor; lifecycle failures of an update override it.
	// Possible reasons: Reconciling (False), Degraded (False), Scaled (True, also
	// for a release without workloads).
	ConditionScaled = "Scaled"

	// ConditionManaged reflects whether the controller is actively managing the
	// application. Settings-only failures do not break it.
	// Possible reasons: HookInitializationFailed, HookFailed,
	// ManifestsApplyFailed, ScalingForbidden, ReleaseDeleteFailed, ApplyingManifests,
	// SettingsChanged, NoResourceReconciliation (maintenance applied), Managed.
	ConditionManaged = "Managed"

	// ConditionConfigurationApplied reflects whether the desired configuration —
	// settings, hooks, manifests — was applied.
	// Possible reasons: SettingsInvalid, HookInitializationFailed, HookFailed,
	// ManifestsApplyFailed, ScalingForbidden, ReleaseDeleteFailed, ApplyingManifests,
	// SettingsChanged, ConfigurationApplied (when True).
	ConditionConfigurationApplied = "ConfigurationApplied"
)

// reasonRequirementsUnmet is the external reason of every unmet requirement of an application.
const reasonRequirementsUnmet = "RequirementsUnmet"

// reasonDeleting is the reason of every condition of a application being removed.
const reasonDeleting = "Deleting"

// mapped is the result of one projection: the conditions to set and the types to remove.
type mapped struct {
	set    []metav1.Condition
	remove []string
}

// gatedMappers are the conditions behind the Installed gate, in evaluation order.
var gatedMappers = []struct {
	condType string
	fn       func(f *facts) metav1.Condition
}{
	{ConditionUpdateInstalled, mapUpdateInstalled},
	{ConditionReady, mapReady},
	{ConditionScaled, mapScaled},
	{ConditionManaged, mapManaged},
	{ConditionConfigurationApplied, mapConfigurationApplied},
}

// mapConditions maps the facts to the external conditions. A application being removed reports
// every condition False/Deleting. Otherwise Installed is mapped first; the others are reported only
// while it is True (freshly mapped, else the one on the resource) and removed while it is not. A
// mapper that returns no condition keeps the one on the resource (sticky).
func mapConditions(f *facts) mapped {
	if f.deleting {
		var m mapped
		for _, condType := range append([]string{ConditionInstalled}, gatedTypes()...) {
			m.set = append(m.set, metav1.Condition{Type: condType, Status: metav1.ConditionFalse, Reason: reasonDeleting})
		}

		return m
	}

	var m mapped

	installed := mapInstalled(f)
	if installed.Type != "" {
		m.set = append(m.set, installed)
	}

	open := f.extTrue(ConditionInstalled)
	if installed.Type != "" {
		open = installed.Status == metav1.ConditionTrue
	}

	for _, g := range gatedMappers {
		if !open {
			m.remove = append(m.remove, g.condType)
			continue
		}

		if c := g.fn(f); c.Type != "" {
			m.set = append(m.set, c)
		}
	}

	return m
}

// gatedTypes are the condition types behind the Installed gate.
func gatedTypes() []string {
	types := make([]string, 0, len(gatedMappers))
	for _, g := range gatedMappers {
		types = append(types, g.condType)
	}

	return types
}

// emit builds an external condition from a signal: a True one carries the external type as its
// reason, a False or Unknown one the signal's canonical reason and message.
func emit(ext string, status metav1.ConditionStatus, s *signal) metav1.Condition {
	if status == metav1.ConditionTrue {
		return metav1.Condition{Type: ext, Status: status, Reason: ext}
	}

	return metav1.Condition{Type: ext, Status: status, Reason: s.reason, Message: s.message}
}

// applyingCondition is ext False/ApplyingManifests: the release apply is in flight.
func applyingCondition(ext string) metav1.Condition {
	return metav1.Condition{Type: ext, Status: metav1.ConditionFalse, Reason: model.ReasonApplyingManifests}
}

// settingsChangedCondition is ext False/SettingsChanged: accepted settings no run applied yet.
func settingsChangedCondition(ext string) metav1.Condition {
	return metav1.Condition{Type: ext, Status: metav1.ConditionFalse, Reason: reasonSettingsChanged}
}

// phase classifies a mapping run by the externally observed install state.
type phase int

const (
	phaseInstall   phase = iota // not yet installed
	phaseUpdate                 // installed and a new version is on its way
	phaseReconcile              // installed and not updating
)

// phaseOf classifies the current run.
func phaseOf(f *facts) phase {
	if !f.extTrue(ConditionInstalled) {
		return phaseInstall
	}
	if f.updating {
		return phaseUpdate
	}

	return phaseReconcile
}

// installPipeline lists every gate of a first install in priority order.
func (f *facts) installPipeline() []*signal {
	return []*signal{&f.requirements, &f.readyOnFilesystem, &f.loaded, &f.configured, &f.hooks, &f.manifests}
}

// updatePipeline returns the gates of the update in progress: of the candidate while there is one
// (the running version's conditions are not among them, its settings verdict included: they
// describe it), else of the activated new version.
func (f *facts) updatePipeline() []*signal {
	if f.candidate {
		return []*signal{&f.updateGate, &f.readyOnFilesystem, &f.loaded, &f.candidateConfigured}
	}

	return []*signal{&f.configured, &f.hooks, &f.manifests}
}

// lateStage: hooks + manifests, what breaks a running application.
func (f *facts) lateStage() []*signal {
	return []*signal{&f.hooks, &f.manifests}
}

// configPipeline: settings + hooks + manifests.
func (f *facts) configPipeline() []*signal {
	return []*signal{&f.configured, &f.hooks, &f.manifests}
}

// firstFalse returns the first False signal of chain; nil when none is. Applying and changed
// settings are progress, never False.
func firstFalse(chain []*signal) *signal {
	for _, s := range chain {
		if s.isFalse() {
			return s
		}
	}

	return nil
}

// settingsChanged emits False/SettingsChanged for ext while new settings are being applied to an
// installed application.
func settingsChanged(f *facts, ph phase, ext string) (metav1.Condition, bool) {
	if ph == phaseInstall || !f.settingsChanged {
		return metav1.Condition{}, false
	}

	return settingsChangedCondition(ext), true
}

// applyingProgress emits False/ApplyingManifests for ext during an update's apply when ext
// carries no True: a True one stays (the previous version still works), a stale failure follows
// the apply.
func applyingProgress(f *facts, ext string) (metav1.Condition, bool) {
	if !f.applying || f.extTrue(ext) {
		return metav1.Condition{}, false
	}

	return applyingCondition(ext), true
}

// isInstallComplete reports whether the first install landed: the release is installed and its
// workloads are ready (or it has none).
func isInstallComplete(f *facts) bool {
	return f.installed.isTrue() && f.scaled.isTrue()
}

// Convention for all mappers below: failure checks come BEFORE success checks.

// isDisabled reports whether the release of an installed application was taken down by a disable:
// Installed goes False and the closed gate removes the other conditions until the application is
// installed again. Only after the disable succeeded: until then the release is still there.
func isDisabled(f *facts) bool {
	return f.extTrue(ConditionInstalled) && f.installed.isFalse()
}

// isSuspended reports whether an installed application lost a hard requirement; the disable may
// still be running.
func isSuspended(f *facts) bool {
	return f.extTrue(ConditionInstalled) && f.requirements.isFalse()
}

// mapInstalled is sticky: once True it is retracted only by a disable.
func mapInstalled(f *facts) metav1.Condition {
	if isDisabled(f) {
		if f.requirements.isFalse() {
			return emit(ConditionInstalled, metav1.ConditionFalse, &f.requirements)
		}

		return metav1.Condition{Type: ConditionInstalled, Status: metav1.ConditionFalse, Reason: reasonRequirementsUnmet}
	}
	if f.extTrue(ConditionInstalled) {
		return metav1.Condition{}
	}
	if s := firstFalse(f.installPipeline()); s != nil {
		return emit(ConditionInstalled, metav1.ConditionFalse, s)
	}
	if isInstallComplete(f) {
		return emit(ConditionInstalled, metav1.ConditionTrue, &f.installed)
	}
	if f.pending {
		return metav1.Condition{Type: ConditionInstalled, Status: metav1.ConditionFalse, Reason: reasonPending}
	}

	return metav1.Condition{}
}

// mapUpdateInstalled reports installing a new version over an installed application: True only once
// the new version's own run succeeded.
func mapUpdateInstalled(f *facts) metav1.Condition {
	if !f.extTrue(ConditionInstalled) {
		return metav1.Condition{}
	}

	if !f.updating && !f.hasExt(ConditionUpdateInstalled) {
		return metav1.Condition{}
	}

	if f.updating {
		if s := firstFalse(f.updatePipeline()); s != nil {
			return emit(ConditionUpdateInstalled, metav1.ConditionFalse, s)
		}
		// The running version's conditions say nothing about a candidate.
		if f.candidate {
			return metav1.Condition{}
		}
		// An activated version is not installed before its own run succeeded, whatever an
		// earlier update left.
		if f.applying {
			return applyingCondition(ConditionUpdateInstalled)
		}
	}
	if f.manifests.isTrue() {
		return emit(ConditionUpdateInstalled, metav1.ConditionTrue, &f.manifests)
	}

	return metav1.Condition{}
}

// mapReady tracks user-facing readiness: only hook and manifest failures break it (settings alone
// leave the running version as it is).
func mapReady(f *facts) metav1.Condition {
	ph := phaseOf(f)

	if ph != phaseInstall {
		if s := firstFalse(f.lateStage()); s != nil {
			return emit(ConditionReady, metav1.ConditionFalse, s)
		}
	}
	if c, ok := settingsChanged(f, ph, ConditionReady); ok {
		return c
	}
	if f.scaled.isTrue() {
		return emit(ConditionReady, metav1.ConditionTrue, &f.scaled)
	}
	if ph == phaseUpdate {
		if c, ok := applyingProgress(f, ConditionReady); ok {
			return c
		}
	}

	return metav1.Condition{}
}

// mapScaled mirrors the workload health monitor; during an update a hook failure makes the
// workload state unobservable (Unknown) and a manifests failure is the workload's own (False).
func mapScaled(f *facts) metav1.Condition {
	if phaseOf(f) == phaseUpdate {
		if s := firstFalse(f.lateStage()); s != nil {
			if s == &f.manifests {
				return emit(ConditionScaled, metav1.ConditionFalse, s)
			}
			return emit(ConditionScaled, metav1.ConditionUnknown, s)
		}
	}

	if f.scaled.status == "" {
		return metav1.Condition{}
	}

	return emit(ConditionScaled, f.scaled.status, &f.scaled)
}

// mapManaged reports whether the controller can actively manage the application.
func mapManaged(f *facts) metav1.Condition {
	ph := phaseOf(f)

	if s := firstFalse(f.lateStage()); s != nil {
		return emit(ConditionManaged, metav1.ConditionFalse, s)
	}
	// Maintenance is deliberate, so it outranks progress but never masks a failure.
	if f.maintenance.isTrue() {
		return emit(ConditionManaged, metav1.ConditionFalse, &f.maintenance)
	}
	if c, ok := settingsChanged(f, ph, ConditionManaged); ok {
		return c
	}
	if f.loaded.isTrue() && f.scaled.isTrue() && f.hooks.isTrue() && f.manifests.isTrue() {
		return emit(ConditionManaged, metav1.ConditionTrue, &f.loaded)
	}
	if ph == phaseUpdate {
		if c, ok := applyingProgress(f, ConditionManaged); ok {
			return c
		}
	}

	return metav1.Condition{}
}

// mapConfigurationApplied tracks whether settings, hooks and manifests of the desired
// configuration were applied. During an update settings failures of the candidate change nothing
// applied.
func mapConfigurationApplied(f *facts) metav1.Condition {
	ph := phaseOf(f)

	chain := f.configPipeline()
	if ph == phaseUpdate {
		chain = f.lateStage()
	}

	if s := firstFalse(chain); s != nil {
		return emit(ConditionConfigurationApplied, metav1.ConditionFalse, s)
	}
	if c, ok := settingsChanged(f, ph, ConditionConfigurationApplied); ok {
		return c
	}
	if f.configured.isTrue() && f.hooks.isTrue() && f.manifests.isTrue() {
		return emit(ConditionConfigurationApplied, metav1.ConditionTrue, &f.configured)
	}
	if ph == phaseUpdate {
		if c, ok := applyingProgress(f, ConditionConfigurationApplied); ok {
			return c
		}
	}

	return metav1.Condition{}
}
