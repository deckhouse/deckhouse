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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/observed"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

func TestCurrentVersionIsTheActivatedVersion(t *testing.T) {
	// Activated, nothing ran yet: status.currentVersion moves at once, Boot restores it.
	app := project(t, projection{
		source:         fakeSource{report: model.Report{CurrentVersion: v2}, known: true},
		conditions:     installedConditions(),
		currentVersion: v1,
	})

	assert.Equal(t, v2, app.Status.CurrentVersion.Version)
}

func TestLastAppliedConfigurationIsTheCheckpoint(t *testing.T) {
	report := running(v1)
	report.Current.AcceptedChecksum = "b" // accepted later, not applied: not the checkpoint

	app := project(t, projection{source: fakeSource{report: report, known: true, obs: ready}})

	assert.JSONEq(t, `{"replicas":1}`, string(app.Status.LastAppliedConfiguration.Raw))

	// No checkpoint yet: the field is left as it is.
	app = project(t, projection{source: fakeSource{report: model.Report{CurrentVersion: v1}, known: true}})
	assert.Empty(t, app.Status.LastAppliedConfiguration.Raw)
}

func TestNotifiedFollowsTheReport(t *testing.T) {
	sent := time.Date(2026, 11, 12, 10, 0, 0, 500, time.UTC)

	report := running(v1)
	report.Notified = model.Notified{Version: v2, At: sent}

	app := project(t, projection{source: fakeSource{report: report, known: true, obs: ready}})
	require.NotNil(t, app.Status.Update)
	assert.Equal(t, &v1alpha1.ApplicationStatusUpdateNotified{
		Version: v2,
		Time:    metav1.NewTime(sent.Truncate(time.Second)),
	}, app.Status.Update.Notified, "kept in seconds, as the resource stores it")

	// Cleared once the new version is installed; the plan, the Updater's, stays.
	app.Status.Update.Plan = []string{v2}
	svc := &Service{source: fakeSource{report: running(v2), known: true, obs: ready}}
	svc.computeAndApplyConditions(eventName, app)
	assert.Equal(t, &v1alpha1.ApplicationStatusUpdate{Plan: []string{v2}}, app.Status.Update)

	// A status.update holding only the mark goes with it.
	app.Status.Update = &v1alpha1.ApplicationStatusUpdate{Notified: &v1alpha1.ApplicationStatusUpdateNotified{Version: v2}}
	svc.computeAndApplyConditions(eventName, app)
	assert.Nil(t, app.Status.Update)
}

func TestURLsFollowTheAppliedRelease(t *testing.T) {
	obs := observed.Observation{Health: ready.Health, URLs: []observed.URL{{URL: "https://app.example.com", Description: "UI"}}}

	app := project(t, projection{source: fakeSource{report: running(v1), known: true, obs: obs}})
	assert.Equal(t, []v1alpha1.ApplicationStatusURL{{URL: "https://app.example.com", Description: "UI"}}, app.Status.URLs)

	// A failed apply keeps the endpoints of the release that is there.
	report := running(v1)
	report.Current.ManifestsApplied = failed("RenderFailed")

	svc := &Service{source: fakeSource{report: report, known: true}}
	svc.computeAndApplyConditions(eventName, app)
	assert.Len(t, app.Status.URLs, 1)
}

func TestTrackingFollowsTheObservation(t *testing.T) {
	tracking := observed.Tracking{Completed: 1, Remaining: 1, Report: json.RawMessage(`{"operations":[]}`)}
	want, err := json.Marshal(tracking)
	require.NoError(t, err)

	app := project(t, projection{source: fakeSource{known: true, obs: observed.Observation{Tracking: tracking}}, deleting: true})
	assert.JSONEq(t, string(want), string(app.Status.Tracking.Raw), "the uninstall progress is committed on the way out")

	// An empty progress keeps the previous one.
	svc := &Service{source: fakeSource{known: true}}
	svc.computeAndApplyConditions(eventName, app)
	assert.JSONEq(t, string(want), string(app.Status.Tracking.Raw))
}

// TestSummaryRows routes a scenario of every Summary through the whole projection: every
// summaryTable row (phase × reason), the fallback of every phase and every fixed Summary. The
// expectations name the rows rather than copy their texts, so a text is changed in summary.go only.
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

	// The candidate reports of an install and of an update alike.
	requirements := model.CandidateReport{Blocked: model.ReasonRequirementsNotMet, Reason: "DependencyNotEnabled"}
	attach := model.CandidateReport{Step: "attach", Reason: model.ReasonAttachFailed}
	load := model.CandidateReport{Step: "load", Reason: model.ReasonLoadFromFilesystemFailed}
	settingsInvalid := model.CandidateReport{Blocked: model.ReasonSettingsInvalid}

	// row is the summaryTable row of the phase and the reason.
	row := func(ph phase, reason string) advice {
		a, ok := summaryTable[ph][reason]
		require.True(t, ok, "no summaryTable row of phase %d and reason %s", ph, reason)
		return a
	}

	// The Summaries the code composes: the fallback of a reason outside the table in every phase,
	// and the fixed ones that name a failure.
	installFallback := advice{state: stateFailed, message: "Installation failed: ReleaseDeleteFailed"}
	updateFallback := advice{state: stateFailed, message: "Update failed: ReleaseDeleteFailed; previous version is no longer serving"}
	reconcileFallback := advice{state: stateDegraded, message: "Reconcile failed: ReleaseDeleteFailed"}

	failingDisable := summarySuspended
	failingDisable.message += "; the disable is failing: ReleaseDeleteFailed: boom"

	uninstallFailure := summaryDeleting
	uninstallFailure.message += ": HookFailed: boom"

	deleting := reconcile(hooks(model.ReasonHookFailed), ready)
	deleting.deleting = true

	tests := []struct {
		name string
		p    projection
		want advice
	}{
		{"install/RequirementsUnmet", install(requirements), row(phaseInstall, reasonRequirementsUnmet)},
		{"install/in progress", install(model.CandidateReport{}), summaryInstalling},
		{"install/AttachFailed", install(attach), row(phaseInstall, model.ReasonAttachFailed)},
		{"install/LoadFromFilesystemFailed", install(load), row(phaseInstall, model.ReasonLoadFromFilesystemFailed)},
		{"install/SettingsInvalid", install(settingsInvalid), row(phaseInstall, model.ReasonSettingsInvalid)},
		{"install/HookInitializationFailed", installRun(hooks(model.ReasonHookInitializationFailed)), row(phaseInstall, model.ReasonHookInitializationFailed)},
		{"install/HookFailed", installRun(hooks(model.ReasonHookFailed)), row(phaseInstall, model.ReasonHookFailed)},
		{"install/ManifestsApplyFailed", installRun(manifests("RenderFailed")), row(phaseInstall, model.ReasonManifestsApplyFailed)},
		{"install/ScalingForbidden", installRun(manifests(model.ReasonScalingForbidden)), row(phaseInstall, model.ReasonScalingForbidden)},
		{"install/fallback", installRun(manifests(model.ReasonReleaseDeleteFailed)), installFallback},
		{"install/complete", projection{source: fakeSource{known: true, report: running(v1), obs: ready}}, summaryReady},
		{"update/WaitingWindow", candidate(model.CandidateReport{Blocked: model.ReasonWaitingWindow}), row(phaseUpdate, model.ReasonWaitingWindow)},
		{"update/WaitingNotification", candidate(model.CandidateReport{Blocked: model.ReasonWaitingNotification}), row(phaseUpdate, model.ReasonWaitingNotification)},
		{"update/RequirementsUnmet", candidate(requirements), row(phaseUpdate, reasonRequirementsUnmet)},
		{"update/AttachFailed", candidate(attach), row(phaseUpdate, model.ReasonAttachFailed)},
		{"update/LoadFromFilesystemFailed", candidate(load), row(phaseUpdate, model.ReasonLoadFromFilesystemFailed)},
		{"update/SettingsInvalid", candidate(settingsInvalid), row(phaseUpdate, model.ReasonSettingsInvalid)},
		{"update/preparing", candidate(model.CandidateReport{}), summaryUpdating},
		{"update/HookInitializationFailed", activatedV2(hooks(model.ReasonHookInitializationFailed), ready), row(phaseUpdate, model.ReasonHookInitializationFailed)},
		{"update/HookFailed", activatedV2(hooks(model.ReasonHookFailed), ready), row(phaseUpdate, model.ReasonHookFailed)},
		{"update/ManifestsApplyFailed", activatedV2(manifests("RenderFailed"), ready), row(phaseUpdate, model.ReasonManifestsApplyFailed)},
		{"update/ScalingForbidden", activatedV2(manifests(model.ReasonScalingForbidden), ready), row(phaseUpdate, model.ReasonScalingForbidden)},
		{"update/fallback", activatedV2(manifests(model.ReasonReleaseDeleteFailed), ready), updateFallback},
		{"update/Reconciling", activatedV2(nil, progressing), row(phaseUpdate, reasonReconciling)},
		{"update/Degraded", activatedV2(nil, degraded), row(phaseUpdate, reasonDegraded)},
		{"update/HealthUnknown", activatedV2(nil, observed.Observation{}), row(phaseUpdate, reasonHealthUnknown)},
		{"update/complete", activatedV2(nil, ready), summaryReady},
		{"reconcile/healthy", reconcile(nil, ready), summaryReady},
		{"reconcile/Configuring", reconcile(func(c *model.CurrentReport) { c.AcceptedChecksum = "b" }, ready), summaryConfiguring},
		{"reconcile/SettingsInvalid", reconcile(func(c *model.CurrentReport) { c.Configured = failed(model.ReasonSettingsInvalid) }, ready), row(phaseReconcile, model.ReasonSettingsInvalid)},
		{"reconcile/HookInitializationFailed", reconcile(hooks(model.ReasonHookInitializationFailed), ready), row(phaseReconcile, model.ReasonHookInitializationFailed)},
		{"reconcile/HookFailed", reconcile(hooks(model.ReasonHookFailed), ready), row(phaseReconcile, model.ReasonHookFailed)},
		{"reconcile/ManifestsApplyFailed", reconcile(manifests("RenderFailed"), ready), row(phaseReconcile, model.ReasonManifestsApplyFailed)},
		{"reconcile/ScalingForbidden", reconcile(manifests(model.ReasonScalingForbidden), ready), row(phaseReconcile, model.ReasonScalingForbidden)},
		{"reconcile/fallback", reconcile(manifests(model.ReasonReleaseDeleteFailed), ready), reconcileFallback},
		{"reconcile/Reconciling", reconcile(nil, progressing), row(phaseReconcile, reasonReconciling)},
		{"reconcile/Degraded", reconcile(nil, degraded), row(phaseReconcile, reasonDegraded)},
		{"suspended/requirements", reconcile(func(c *model.CurrentReport) { c.Decision = disabled("DependencyNotEnabled") }, ready), summarySuspended},
		{"suspended/failing disable", reconcile(func(c *model.CurrentReport) {
			c.Decision = disabled("DependencyNotEnabled")
			c.ManifestsApplied = failed(model.ReasonReleaseDeleteFailed)
		}, ready), failingDisable},
		{"deleting/uninstall failure", deleting, uninstallFailure},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := project(t, tt.p).Status.Summary

			assert.Equal(t, tt.want, advice{state: summary.State, message: summary.Message, tip: summary.Tip})
		})
	}
}
