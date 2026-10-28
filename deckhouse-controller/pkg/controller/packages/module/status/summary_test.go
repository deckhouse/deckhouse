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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/observed"
	"github.com/deckhouse/deckhouse/testing/controller/reconcilertest"
)

func TestCurrentVersionIsTheActivatedVersion(t *testing.T) {
	// Activated, nothing ran yet: status.currentVersion moves at once, Boot restores it.
	module := project(t, projection{
		source:         fakeSource{report: model.Report{CurrentVersion: v2}, known: true},
		conditions:     installedConditions(),
		currentVersion: v1,
	})

	assert.Equal(t, v2, module.Status.CurrentVersion.Version)
}

func TestLastAppliedConfigurationIsTheCheckpoint(t *testing.T) {
	report := running(v1)
	report.Current.AcceptedChecksum = "b" // accepted later, not applied: not the checkpoint

	module := project(t, projection{source: fakeSource{report: report, known: true, obs: ready}})

	assert.JSONEq(t, `{"replicas":1}`, string(module.Status.LastAppliedConfiguration.Raw))

	// No checkpoint yet: the field is left as it is.
	module = project(t, projection{source: fakeSource{report: model.Report{CurrentVersion: v1}, known: true}})
	assert.Empty(t, module.Status.LastAppliedConfiguration.Raw)
}

func TestEnabledIsTheSchedulerDecision(t *testing.T) {
	tests := []struct {
		name     string
		decision *model.Decision
		current  *bool
		want     *bool
	}{
		{name: "enabled", decision: enabled(), want: ptr.To(true)},
		{name: "disabled", decision: disabled("DependencyNotEnabled"), want: ptr.To(false)},
		{name: "absent before the first decision", want: nil},
		{name: "the last decision stands while none arrived", current: ptr.To(false), want: ptr.To(false)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := model.Report{CurrentVersion: v1, Current: model.CurrentReport{Decision: tt.decision}}

			svc := &Service{source: fakeSource{report: report, known: true}}
			module := project(t, projection{source: fakeSource{report: report, known: true}})
			if tt.current != nil {
				module.Status.Enabled = tt.current
				svc.computeAndApplyConditions(moduleName, module)
			}

			assert.Equal(t, tt.want, module.Status.Enabled)
		})
	}
}

func TestTrackingFollowsTheObservation(t *testing.T) {
	tracking := observed.Tracking{Completed: 1, Remaining: 1, Report: json.RawMessage(`{"operations":[]}`)}
	want, err := json.Marshal(tracking)
	require.NoError(t, err)

	module := project(t, projection{source: fakeSource{known: true, obs: observed.Observation{Tracking: tracking}}, deleting: true})
	assert.JSONEq(t, string(want), string(module.Status.Tracking.Raw), "the uninstall progress is committed on the way out")

	// An empty progress keeps the previous one.
	svc := &Service{source: fakeSource{known: true}}
	svc.computeAndApplyConditions(moduleName, module)
	assert.JSONEq(t, string(want), string(module.Status.Tracking.Raw))
}

// summaryRow is one golden row: the scenario and the Summary it produces.
type summaryRow struct {
	Case    string `json:"case"`
	State   string `json:"state"`
	Message string `json:"message"`
	Tip     string `json:"tip,omitempty"`
}

// TestSummaryRows pins every Summary row (phase × reason → state, message, tip) and every fixed
// Summary, through the whole projection. Regenerate with -golden.
func TestSummaryRows(t *testing.T) {
	install := func(update model.CandidateReport) projection {
		update.Version = v1
		return projection{source: fakeSource{known: true, report: model.Report{Candidate: update}}}
	}
	installRun := func(change func(*model.CurrentReport)) projection {
		r := model.Report{CurrentVersion: v1, Current: model.CurrentReport{Version: v1, Decision: enabled()}}
		change(&r.Current)
		return projection{source: fakeSource{known: true, report: r}}
	}
	installed := func(r model.Report, obs observed.Observation, conditions []metav1.Condition) projection {
		if conditions == nil {
			conditions = installedConditions()
		}
		return projection{source: fakeSource{known: true, report: r, obs: obs}, conditions: conditions, currentVersion: v1}
	}
	candidate := func(u model.CandidateReport) projection {
		r := running(v1)
		u.Version = v2
		r.Candidate = u
		return installed(r, ready, nil)
	}
	activatedV2 := func(change func(*model.CurrentReport), obs observed.Observation) projection {
		r := running(v2)
		if change != nil {
			change(&r.Current)
		}
		p := installed(r, obs, with(installedConditions(), metav1.Condition{
			Type: ConditionUpdateInstalled, Status: metav1.ConditionFalse, Reason: model.ReasonApplyingManifests,
		}))
		p.currentVersion = v2
		return p
	}
	reconcile := func(change func(*model.CurrentReport), obs observed.Observation) projection {
		r := running(v1)
		if change != nil {
			change(&r.Current)
		}
		return installed(r, obs, nil)
	}
	hooks := func(reason string) func(*model.CurrentReport) {
		return func(c *model.CurrentReport) { c.HooksProcessed = failed(reason) }
	}
	manifests := func(reason string) func(*model.CurrentReport) {
		return func(c *model.CurrentReport) { c.ManifestsApplied = failed(reason) }
	}
	progressing := observed.Observation{Health: observed.Health{State: observed.HealthProgressing}}
	degraded := observed.Observation{Health: observed.Health{State: observed.HealthDegraded}}

	type scenario struct {
		name string
		p    projection
	}

	var scenarios []scenario

	for _, reason := range []string{
		reasonDisabled, reasonDisabledByBundle, reasonDisabledByScript, "EnabledScriptError", "DependencyNotEnabled",
		"DependencyVersionMismatch", "AnyOfDependenciesUnmet", "NoneOfDependenciesViolated", "VersionLookupFailed",
		"CustomRuleReason",
	} {
		scenarios = append(scenarios, scenario{
			"install/scheduler/" + reason,
			install(model.CandidateReport{Blocked: model.ReasonRequirementsNotMet, Reason: reason}),
		})
	}

	scenarios = append(scenarios,
		scenario{"install/RequirementsNotMet", install(model.CandidateReport{Blocked: model.ReasonRequirementsNotMet})},
		scenario{"install/in progress", install(model.CandidateReport{})},
		scenario{"install/AttachFailed", install(model.CandidateReport{Step: "attach", Reason: model.ReasonAttachFailed})},
		scenario{"install/LoadFromFilesystemFailed", install(model.CandidateReport{Step: "load", Reason: model.ReasonLoadFromFilesystemFailed})},
		scenario{"install/SettingsInvalid", install(model.CandidateReport{Blocked: model.ReasonSettingsInvalid})},
		scenario{"install/HookInitializationFailed", installRun(hooks(model.ReasonHookInitializationFailed))},
		scenario{"install/HookFailed", installRun(hooks(model.ReasonHookFailed))},
		scenario{"install/ManifestsApplyFailed", installRun(manifests("RenderFailed"))},
		scenario{"install/ScalingForbidden", installRun(manifests(model.ReasonScalingForbidden))},
		scenario{"install/fallback", installRun(manifests(model.ReasonReleaseDeleteFailed))},
		scenario{"install/complete", projection{source: fakeSource{known: true, report: running(v1), obs: ready}}},
		scenario{"update/WaitingWindow", candidate(model.CandidateReport{Blocked: model.ReasonWaitingWindow})},
		scenario{"update/RequirementsNotMet", candidate(model.CandidateReport{Blocked: model.ReasonRequirementsNotMet})},
		scenario{"update/scheduler fallback", candidate(model.CandidateReport{Blocked: model.ReasonRequirementsNotMet, Reason: "DependencyNotEnabled"})},
		scenario{"update/AttachFailed", candidate(model.CandidateReport{Step: "attach", Reason: model.ReasonAttachFailed})},
		scenario{"update/LoadFromFilesystemFailed", candidate(model.CandidateReport{Step: "load", Reason: model.ReasonLoadFromFilesystemFailed})},
		scenario{"update/SettingsInvalid", candidate(model.CandidateReport{Blocked: model.ReasonSettingsInvalid})},
		scenario{"update/preparing", candidate(model.CandidateReport{})},
		scenario{"update/HookInitializationFailed", activatedV2(hooks(model.ReasonHookInitializationFailed), ready)},
		scenario{"update/HookFailed", activatedV2(hooks(model.ReasonHookFailed), ready)},
		scenario{"update/ManifestsApplyFailed", activatedV2(manifests("RenderFailed"), ready)},
		scenario{"update/ScalingForbidden", activatedV2(manifests(model.ReasonScalingForbidden), ready)},
		scenario{"update/fallback", activatedV2(manifests(model.ReasonReleaseDeleteFailed), ready)},
		scenario{"update/Reconciling", activatedV2(nil, progressing)},
		scenario{"update/Degraded", activatedV2(nil, degraded)},
		scenario{"update/HealthUnknown", activatedV2(nil, observed.Observation{})},
		scenario{"update/complete", activatedV2(nil, ready)},
		scenario{"reconcile/healthy", reconcile(nil, ready)},
		scenario{"reconcile/Configuring", reconcile(func(c *model.CurrentReport) { c.AcceptedChecksum = "b" }, ready)},
		scenario{"reconcile/SettingsInvalid", reconcile(func(c *model.CurrentReport) { c.Configured = failed(model.ReasonSettingsInvalid) }, ready)},
		scenario{"reconcile/HookInitializationFailed", reconcile(hooks(model.ReasonHookInitializationFailed), ready)},
		scenario{"reconcile/HookFailed", reconcile(hooks(model.ReasonHookFailed), ready)},
		scenario{"reconcile/ManifestsApplyFailed", reconcile(manifests("RenderFailed"), ready)},
		scenario{"reconcile/ScalingForbidden", reconcile(manifests(model.ReasonScalingForbidden), ready)},
		scenario{"reconcile/fallback", reconcile(manifests(model.ReasonReleaseDeleteFailed), ready)},
		scenario{"reconcile/Reconciling", reconcile(nil, progressing)},
		scenario{"reconcile/Degraded", reconcile(nil, degraded)},
		scenario{"suspended/disabled", reconcile(func(c *model.CurrentReport) { c.Decision = disabled(reasonDisabled) }, ready)},
		scenario{"suspended/requirements", reconcile(func(c *model.CurrentReport) { c.Decision = disabled("DependencyNotEnabled") }, ready)},
		scenario{"suspended/failing disable", reconcile(func(c *model.CurrentReport) {
			c.Decision = disabled(reasonDisabled)
			c.ManifestsApplied = failed(model.ReasonReleaseDeleteFailed)
		}, ready)},
	)

	deleting := reconcile(func(c *model.CurrentReport) { c.HooksProcessed = failed(model.ReasonHookFailed) }, ready)
	deleting.deleting = true
	scenarios = append(scenarios, scenario{"deleting/uninstall failure", deleting})

	rows := make([]summaryRow, 0, len(scenarios))
	for _, s := range scenarios {
		summary := project(t, s.p).Status.Summary
		rows = append(rows, summaryRow{Case: s.name, State: summary.State, Message: summary.Message, Tip: summary.Tip})
	}

	got, err := yaml.Marshal(rows)
	require.NoError(t, err)

	reconcilertest.CompareOrUpdate(t, "testdata/golden/summary.yaml", got, reconcilertest.WholeDocument)
}

// Every scheduler verdict that blocks a module has its own row, as the install table says: the
// generic fallback carries no tip.
func TestEveryBlockingVerdictHasARow(t *testing.T) {
	for _, reason := range []string{schedule.ReasonDependentRequirementsUnmet, schedule.ReasonDependencyCycle} {
		for _, ph := range []phase{phaseInstall, phaseUpdate} {
			_, ok := summaryTable[ph][reason]
			assert.True(t, ok, "phase %v, reason %s", ph, reason)
		}
	}
}
