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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/observed"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

const (
	appNamespace = "ns"
	appName      = "app"
	eventName    = appNamespace + "." + appName
	v1           = "v1.0.0"
	v2           = "v2.0.0"
)

// fakeSource is the engine as the status service reads it.
type fakeSource struct {
	report model.Report
	known  bool
	obs    observed.Observation
}

func (f fakeSource) Report(string) (model.Report, bool)   { return f.report, f.known }
func (f fakeSource) Observed(string) observed.Observation { return f.obs }

// cond is an expected external condition: status and reason.
type cond struct {
	status metav1.ConditionStatus
	reason string
}

func wantTrue(reason string) cond            { return cond{metav1.ConditionTrue, reason} }
func wantFalse(reason string) cond           { return cond{metav1.ConditionFalse, reason} }
func wantUnknown(reason string) cond         { return cond{metav1.ConditionUnknown, reason} }
func trueCondition() model.Condition         { return model.TrueCondition() }
func failed(reason string) model.Condition   { return model.FalseCondition(reason, "boom") }
func enabled() *model.Decision               { return &model.Decision{Enabled: true} }
func disabled(reason string) *model.Decision { return &model.Decision{Reason: reason, Message: "off"} }

// running is the Report of an application whose version runs and whose last run succeeded.
func running(version string) model.Report {
	return model.Report{
		CurrentVersion: version,
		Current: model.CurrentReport{
			Version:          version,
			Configured:       trueCondition(),
			HooksProcessed:   trueCondition(),
			ManifestsApplied: trueCondition(),
			Installed:        trueCondition(),
			Decision:         enabled(),
			AcceptedChecksum: "a",
			AppliedChecksum:  "a",
			AppliedValues:    map[string]any{"replicas": 1.0},
		},
	}
}

// ready is the observation of healthy workloads.
var ready = observed.Observation{Health: observed.Health{State: observed.HealthReady}}

// installedConditions are the external conditions of an installed, healthy application.
func installedConditions() []metav1.Condition {
	return []metav1.Condition{
		{Type: ConditionInstalled, Status: metav1.ConditionTrue, Reason: ConditionInstalled},
		{Type: ConditionReady, Status: metav1.ConditionTrue, Reason: ConditionReady},
		{Type: ConditionScaled, Status: metav1.ConditionTrue, Reason: ConditionScaled},
		{Type: ConditionManaged, Status: metav1.ConditionTrue, Reason: ConditionManaged},
		{Type: ConditionConfigurationApplied, Status: metav1.ConditionTrue, Reason: ConditionConfigurationApplied},
	}
}

// with returns conditions with c set.
func with(conditions []metav1.Condition, c metav1.Condition) []metav1.Condition {
	out := append([]metav1.Condition(nil), conditions...)
	meta.SetStatusCondition(&out, c)

	return out
}

// projection is one status computation: the engine's status and the resource's before it.
type projection struct {
	source         fakeSource
	conditions     []metav1.Condition
	currentVersion string
	deleting       bool
}

// project runs the status service over p and returns the resulting Application.
func project(t *testing.T, p projection) *v1alpha1.Application {
	t.Helper()

	app := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{Namespace: appNamespace, Name: appName},
		Status: v1alpha1.ApplicationStatus{
			Conditions:     p.conditions,
			CurrentVersion: &v1alpha1.ApplicationStatusVersion{Version: p.currentVersion},
		},
	}

	if p.deleting {
		now := metav1.Now()
		app.DeletionTimestamp = &now
	}

	svc := &Service{source: p.source}
	svc.computeAndApplyConditions(eventName, app)

	return app
}

// assertConditions checks that the application carries exactly want.
func assertConditions(t *testing.T, app *v1alpha1.Application, want map[string]cond) {
	t.Helper()

	got := make(map[string]cond, len(app.Status.Conditions))
	for _, c := range app.Status.Conditions {
		got[c.Type] = cond{c.Status, c.Reason}
	}

	assert.Equal(t, want, got)
}

// healthy are the gated conditions of an installed, healthy application.
func healthy(extra map[string]cond) map[string]cond {
	want := map[string]cond{
		ConditionInstalled:            wantTrue(ConditionInstalled),
		ConditionReady:                wantTrue(ConditionReady),
		ConditionScaled:               wantTrue(ConditionScaled),
		ConditionManaged:              wantTrue(ConditionManaged),
		ConditionConfigurationApplied: wantTrue(ConditionConfigurationApplied),
	}
	for k, v := range extra {
		want[k] = v
	}

	return want
}

func TestFirstInstall(t *testing.T) {
	preparing := model.Report{Candidate: model.CandidateReport{Version: v1}}

	tests := []struct {
		name    string
		report  model.Report
		obs     observed.Observation
		want    map[string]cond
		summary string
	}{
		{
			name:    "in progress, nothing failed: Pending",
			report:  preparing,
			want:    map[string]cond{ConditionInstalled: wantFalse("Pending")},
			summary: statePending,
		},
		{
			name:    "requirements: the scheduler's reason",
			report:  model.Report{Candidate: model.CandidateReport{Version: v1, Blocked: model.ReasonRequirementsNotMet, Reason: "DependencyNotEnabled"}},
			want:    map[string]cond{ConditionInstalled: wantFalse(reasonRequirementsUnmet)},
			summary: statePending,
		},
		{
			name:    "requirements without a reason",
			report:  model.Report{Candidate: model.CandidateReport{Version: v1, Blocked: model.ReasonRequirementsNotMet}},
			want:    map[string]cond{ConditionInstalled: wantFalse(reasonRequirementsUnmet)},
			summary: statePending,
		},
		{
			name:    "attach failed",
			report:  model.Report{Candidate: model.CandidateReport{Version: v1, Step: "attach", Reason: model.ReasonAttachFailed}},
			want:    map[string]cond{ConditionInstalled: wantFalse(model.ReasonAttachFailed)},
			summary: stateFailed,
		},
		{
			name:    "load failed",
			report:  model.Report{Candidate: model.CandidateReport{Version: v1, Step: "load", Reason: model.ReasonLoadFromFilesystemFailed}},
			want:    map[string]cond{ConditionInstalled: wantFalse(model.ReasonLoadFromFilesystemFailed)},
			summary: stateFailed,
		},
		{
			name:    "settings rejected by the revision",
			report:  model.Report{Candidate: model.CandidateReport{Version: v1, Blocked: model.ReasonSettingsInvalid}},
			want:    map[string]cond{ConditionInstalled: wantFalse(model.ReasonSettingsInvalid)},
			summary: stateFailed,
		},
		{
			name: "activated, first run's hooks failed",
			report: model.Report{CurrentVersion: v1, Current: model.CurrentReport{
				Version: v1, Configured: trueCondition(), HooksProcessed: failed(model.ReasonHookFailed), Decision: enabled(),
			}},
			want:    map[string]cond{ConditionInstalled: wantFalse(model.ReasonHookFailed)},
			summary: stateFailed,
		},
		{
			name: "activated, first run's hook initialization failed",
			report: model.Report{CurrentVersion: v1, Current: model.CurrentReport{
				Version: v1, HooksProcessed: failed(model.ReasonHookInitializationFailed), Decision: enabled(),
			}},
			want:    map[string]cond{ConditionInstalled: wantFalse(model.ReasonHookInitializationFailed)},
			summary: stateFailed,
		},
		{
			name: "activated, first run's manifests failed",
			report: model.Report{CurrentVersion: v1, Current: model.CurrentReport{
				Version: v1, HooksProcessed: trueCondition(), ManifestsApplied: failed("RenderFailed"), Decision: enabled(),
			}},
			want:    map[string]cond{ConditionInstalled: wantFalse(model.ReasonManifestsApplyFailed)},
			summary: stateFailed,
		},
		{
			name: "activated with a requirement lost: RequirementsUnmet",
			report: model.Report{CurrentVersion: v1, Current: model.CurrentReport{
				Decision: disabled("DisabledByBundle"),
			}},
			want:    map[string]cond{ConditionInstalled: wantFalse(reasonRequirementsUnmet)},
			summary: statePending,
		},
		{
			name:    "release installed, workloads rolling out: still Pending (G4)",
			report:  running(v1),
			obs:     observed.Observation{Health: observed.Health{State: observed.HealthProgressing}},
			want:    map[string]cond{ConditionInstalled: wantFalse("Pending")},
			summary: statePending,
		},
		{
			name:    "release installed, workloads ready: installed",
			report:  running(v1),
			obs:     ready,
			want:    healthy(nil),
			summary: stateReady,
		},
		{
			name:    "release installed, no workloads: installed",
			report:  running(v1),
			obs:     observed.Observation{NoWorkloads: true},
			want:    healthy(nil),
			summary: stateReady,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := project(t, projection{source: fakeSource{report: tt.report, known: true, obs: tt.obs}})

			assertConditions(t, app, tt.want)
			assert.Equal(t, tt.summary, app.Status.Summary.State)
		})
	}
}

func TestUnknownKeepsTheExternalConditions(t *testing.T) {
	tests := []struct {
		name   string
		source fakeSource
	}{
		{name: "restart: the engine reported nothing yet", source: fakeSource{known: true, report: model.Report{CurrentVersion: v1}}},
		{name: "the engine does not know the application", source: fakeSource{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := project(t, projection{source: tt.source, conditions: installedConditions(), currentVersion: v1})

			assertConditions(t, app, healthy(nil))
			assert.Equal(t, v1, app.Status.CurrentVersion.Version)
		})
	}
}

func TestRestartDoesNotFlapInstalled(t *testing.T) {
	// A restored Current reports nothing until its first run; then a run failure surfaces on the
	// other conditions, never on Installed.
	steps := []model.Report{
		{CurrentVersion: v1},
		{CurrentVersion: v1, Current: model.CurrentReport{Version: v1, HooksProcessed: failed(model.ReasonHookFailed), Decision: enabled()}},
	}

	conditions := installedConditions()
	for _, report := range steps {
		app := project(t, projection{source: fakeSource{report: report, known: true, obs: ready}, conditions: conditions, currentVersion: v1})
		conditions = app.Status.Conditions

		installed := meta.FindStatusCondition(conditions, ConditionInstalled)
		require.NotNil(t, installed)
		assert.Equal(t, metav1.ConditionTrue, installed.Status)
	}

	assert.Equal(t, metav1.ConditionFalse, meta.FindStatusCondition(conditions, ConditionReady).Status)
}

func TestReconcile(t *testing.T) {
	tests := []struct {
		name    string
		report  func() model.Report
		obs     observed.Observation
		want    map[string]cond
		summary string
	}{
		{
			name:    "healthy",
			report:  func() model.Report { return running(v1) },
			obs:     ready,
			want:    healthy(nil),
			summary: stateReady,
		},
		{
			name: "operation hook failed",
			report: func() model.Report {
				r := running(v1)
				r.Current.HooksProcessed = failed(model.ReasonHookFailed)
				return r
			},
			obs: ready,
			want: healthy(map[string]cond{
				ConditionReady:                wantFalse(model.ReasonHookFailed),
				ConditionManaged:              wantFalse(model.ReasonHookFailed),
				ConditionConfigurationApplied: wantFalse(model.ReasonHookFailed),
			}),
			summary: stateDegraded,
		},
		{
			name:   "runtime hook synchronization failed (observed)",
			report: func() model.Report { return running(v1) },
			obs: observed.Observation{Health: ready.Health, HookFailure: &observed.HookFailure{
				Hook: "h", Reason: observed.HookReasonSync, Message: "boom",
			}},
			want: healthy(map[string]cond{
				ConditionReady:                wantFalse(reasonHookInitializationFailed),
				ConditionManaged:              wantFalse(reasonHookInitializationFailed),
				ConditionConfigurationApplied: wantFalse(reasonHookInitializationFailed),
			}),
			summary: stateDegraded,
		},
		{
			name:   "runtime hook failed on an event (observed)",
			report: func() model.Report { return running(v1) },
			obs: observed.Observation{Health: ready.Health, HookFailure: &observed.HookFailure{
				Hook: "h", Reason: observed.HookReasonEvent, Message: "boom",
			}},
			want: healthy(map[string]cond{
				ConditionReady:                wantFalse(model.ReasonHookFailed),
				ConditionManaged:              wantFalse(model.ReasonHookFailed),
				ConditionConfigurationApplied: wantFalse(model.ReasonHookFailed),
			}),
			summary: stateDegraded,
		},
		{
			name: "manifests failed",
			report: func() model.Report {
				r := running(v1)
				r.Current.ManifestsApplied = failed("RenderFailed")
				return r
			},
			obs: ready,
			want: healthy(map[string]cond{
				ConditionReady:                wantFalse(model.ReasonManifestsApplyFailed),
				ConditionManaged:              wantFalse(model.ReasonManifestsApplyFailed),
				ConditionConfigurationApplied: wantFalse(model.ReasonManifestsApplyFailed),
			}),
			summary: stateDegraded,
		},
		{
			name: "applying a changed release: no flap",
			report: func() model.Report {
				r := running(v1)
				r.Current.ManifestsApplied = model.FalseCondition(model.ReasonApplyingManifests, "")
				return r
			},
			obs:     ready,
			want:    healthy(nil),
			summary: stateReady,
		},
		{
			name: "settings rejected: only ConfigurationApplied",
			report: func() model.Report {
				r := running(v1)
				r.Current.Configured = failed(model.ReasonSettingsInvalid)
				return r
			},
			obs: ready,
			want: healthy(map[string]cond{
				ConditionConfigurationApplied: wantFalse(model.ReasonSettingsInvalid),
			}),
			summary: stateDegraded,
		},
		{
			name: "settings accepted, not applied yet: SettingsChanged",
			report: func() model.Report {
				r := running(v1)
				r.Current.AcceptedChecksum = "b"
				return r
			},
			obs: ready,
			want: healthy(map[string]cond{
				ConditionReady:                wantFalse(reasonSettingsChanged),
				ConditionManaged:              wantFalse(reasonSettingsChanged),
				ConditionConfigurationApplied: wantFalse(reasonSettingsChanged),
			}),
			summary: stateConfiguring,
		},
		{
			name: "maintenance applied by a run",
			report: func() model.Report {
				r := running(v1)
				r.Current.Maintenance = "NoResourceReconciliation"
				return r
			},
			obs: ready,
			want: healthy(map[string]cond{
				ConditionManaged: wantFalse(reasonNoResourceReconciliation),
			}),
			summary: stateReady,
		},
		{
			name: "maintenance accepted, not applied yet: SettingsChanged",
			report: func() model.Report {
				r := running(v1)
				r.Current.Maintenance = "NoResourceReconciliation"
				r.Current.AcceptedChecksum = "b"
				return r
			},
			obs: ready,
			want: healthy(map[string]cond{
				ConditionReady:                wantFalse(reasonSettingsChanged),
				ConditionManaged:              wantFalse(reasonSettingsChanged),
				ConditionConfigurationApplied: wantFalse(reasonSettingsChanged),
			}),
			summary: stateConfiguring,
		},
		{
			name:   "workloads degraded",
			report: func() model.Report { return running(v1) },
			obs:    observed.Observation{Health: observed.Health{State: observed.HealthDegraded, Message: "crash"}},
			want: healthy(map[string]cond{
				ConditionReady:   wantTrue(ConditionReady),
				ConditionScaled:  wantFalse(reasonDegraded),
				ConditionManaged: wantTrue(ConditionManaged),
			}),
			summary: stateDegraded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := project(t, projection{
				source:         fakeSource{report: tt.report(), known: true, obs: tt.obs},
				conditions:     installedConditions(),
				currentVersion: v1,
			})

			assertConditions(t, app, tt.want)
			assert.Equal(t, tt.summary, app.Status.Summary.State)
		})
	}
}

func TestUpdate(t *testing.T) {
	updateInstalled := func(status metav1.ConditionStatus, reason string) metav1.Condition {
		return metav1.Condition{Type: ConditionUpdateInstalled, Status: status, Reason: reason}
	}

	candidate := func(u model.CandidateReport) func() model.Report {
		return func() model.Report {
			r := running(v1)
			u.Version = v2
			r.Candidate = u
			return r
		}
	}

	// activated is v2 activated while the Current section still holds v1's run.
	activated := func(revision string, change func(*model.CurrentReport)) func() model.Report {
		return func() model.Report {
			r := running(v1)
			r.CurrentVersion = v2
			r.Current.Version = revision
			if change != nil {
				change(&r.Current)
			}
			return r
		}
	}

	tests := []struct {
		name           string
		report         func() model.Report
		conditions     []metav1.Condition
		currentVersion string
		want           map[string]cond
		summary        string
	}{
		{
			name:    "waiting for the window",
			report:  candidate(model.CandidateReport{Blocked: model.ReasonWaitingWindow}),
			want:    healthy(map[string]cond{ConditionUpdateInstalled: wantFalse(model.ReasonWaitingWindow)}),
			summary: stateUpdating,
		},
		{
			name:    "requirements of the new version: RequirementsUnmet",
			report:  candidate(model.CandidateReport{Blocked: model.ReasonRequirementsNotMet, Reason: "DependencyNotEnabled"}),
			want:    healthy(map[string]cond{ConditionUpdateInstalled: wantFalse(reasonRequirementsUnmet)}),
			summary: stateUpdating,
		},
		{
			name:    "requirements of the new version without a reason",
			report:  candidate(model.CandidateReport{Blocked: model.ReasonRequirementsNotMet}),
			want:    healthy(map[string]cond{ConditionUpdateInstalled: wantFalse(reasonRequirementsUnmet)}),
			summary: stateUpdating,
		},
		{
			// The running version keeps its last accepted settings (CUR-04); its rejection describes
			// it, not the update.
			name: "settings rejected by the running version while the new one prepares",
			report: func() model.Report {
				r := candidate(model.CandidateReport{})()
				r.Current.Configured = failed(model.ReasonSettingsInvalid)
				return r
			},
			want:    healthy(nil),
			summary: stateUpdating,
		},
		{
			name:    "settings rejected by the new version",
			report:  candidate(model.CandidateReport{Blocked: model.ReasonSettingsInvalid}),
			want:    healthy(map[string]cond{ConditionUpdateInstalled: wantFalse(model.ReasonSettingsInvalid)}),
			summary: stateUpdating,
		},
		{
			name:    "download failed",
			report:  candidate(model.CandidateReport{Step: "attach", Reason: model.ReasonAttachFailed}),
			want:    healthy(map[string]cond{ConditionUpdateInstalled: wantFalse(model.ReasonAttachFailed)}),
			summary: stateUpdating,
		},
		{
			name:       "preparing: an earlier UpdateInstalled stays",
			report:     candidate(model.CandidateReport{}),
			conditions: with(installedConditions(), updateInstalled(metav1.ConditionTrue, ConditionUpdateInstalled)),
			want:       healthy(map[string]cond{ConditionUpdateInstalled: wantTrue(ConditionUpdateInstalled)}),
			summary:    stateUpdating,
		},
		{
			name:       "activated, the new version has not reported: in progress, an earlier True is not kept",
			report:     activated(v1, nil),
			conditions: with(installedConditions(), updateInstalled(metav1.ConditionTrue, ConditionUpdateInstalled)),
			want:       healthy(map[string]cond{ConditionUpdateInstalled: wantFalse(model.ReasonApplyingManifests)}),
			summary:    stateUpdating,
		},
		{
			name:           "the new version's run succeeded",
			report:         activated(v2, nil),
			conditions:     with(installedConditions(), updateInstalled(metav1.ConditionFalse, model.ReasonApplyingManifests)),
			currentVersion: v2,
			want:           healthy(map[string]cond{ConditionUpdateInstalled: wantTrue(ConditionUpdateInstalled)}),
			summary:        stateReady,
		},
		{
			name: "the new version's run failed: Update failed",
			report: activated(v2, func(c *model.CurrentReport) {
				c.HooksProcessed = failed(model.ReasonHookFailed)
			}),
			conditions:     with(installedConditions(), updateInstalled(metav1.ConditionFalse, model.ReasonApplyingManifests)),
			currentVersion: v2,
			want: healthy(map[string]cond{
				ConditionUpdateInstalled:      wantFalse(model.ReasonHookFailed),
				ConditionReady:                wantFalse(model.ReasonHookFailed),
				ConditionScaled:               wantUnknown(model.ReasonHookFailed),
				ConditionManaged:              wantFalse(model.ReasonHookFailed),
				ConditionConfigurationApplied: wantFalse(model.ReasonHookFailed),
			}),
			summary: stateFailed,
		},
		{
			name:           "a failed update succeeds on a later run",
			report:         activated(v2, nil),
			conditions:     with(installedConditions(), updateInstalled(metav1.ConditionFalse, model.ReasonHookFailed)),
			currentVersion: v2,
			want:           healthy(map[string]cond{ConditionUpdateInstalled: wantTrue(ConditionUpdateInstalled)}),
			summary:        stateReady,
		},
		{
			name:           "activation and success seen at once: the version change keeps the update phase",
			report:         activated(v2, nil),
			currentVersion: v1,
			want:           healthy(map[string]cond{ConditionUpdateInstalled: wantTrue(ConditionUpdateInstalled)}),
			summary:        stateReady,
		},
		{
			name:           "a blocked update reverted: UpdateInstalled settles",
			report:         func() model.Report { return running(v1) },
			conditions:     with(installedConditions(), updateInstalled(metav1.ConditionFalse, model.ReasonWaitingWindow)),
			currentVersion: v1,
			want:           healthy(map[string]cond{ConditionUpdateInstalled: wantTrue(ConditionUpdateInstalled)}),
			summary:        stateReady,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conditions := tt.conditions
			if conditions == nil {
				conditions = installedConditions()
			}

			currentVersion := tt.currentVersion
			if currentVersion == "" {
				currentVersion = v1
			}

			app := project(t, projection{
				source:         fakeSource{report: tt.report(), known: true, obs: ready},
				conditions:     conditions,
				currentVersion: currentVersion,
			})

			assertConditions(t, app, tt.want)
			assert.Equal(t, tt.summary, app.Status.Summary.State)
		})
	}
}

func TestRequirementLost(t *testing.T) {
	tests := []struct {
		name    string
		report  func() model.Report
		want    map[string]cond
		message string
	}{
		{
			name: "decided, the release is still installed: Suspended",
			report: func() model.Report {
				r := running(v1)
				r.Current.Decision = disabled("DependencyNotEnabled")
				return r
			},
			want:    healthy(nil),
			message: summarySuspended.message,
		},
		{
			name: "the disable keeps failing: named in the summary",
			report: func() model.Report {
				r := running(v1)
				r.Current.Decision = disabled("DependencyNotEnabled")
				r.Current.ManifestsApplied = failed(model.ReasonReleaseDeleteFailed)
				return r
			},
			want: healthy(map[string]cond{
				ConditionReady:                wantFalse(model.ReasonReleaseDeleteFailed),
				ConditionManaged:              wantFalse(model.ReasonReleaseDeleteFailed),
				ConditionConfigurationApplied: wantFalse(model.ReasonReleaseDeleteFailed),
			}),
			message: summarySuspended.message + "; the disable is failing: ReleaseDeleteFailed: boom",
		},
		{
			name: "disabled: Installed False, RequirementsUnmet",
			report: func() model.Report {
				r := running(v1)
				r.Current.Decision = disabled("DependencyNotEnabled")
				r.Current.Installed = model.FalseCondition(model.ReasonDisabled, "")
				return r
			},
			want:    map[string]cond{ConditionInstalled: wantFalse(reasonRequirementsUnmet)},
			message: summarySuspended.message,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := project(t, projection{
				source:         fakeSource{report: tt.report(), known: true, obs: ready},
				conditions:     installedConditions(),
				currentVersion: v1,
			})

			assertConditions(t, app, tt.want)
			assert.Equal(t, stateSuspended, app.Status.Summary.State)
			assert.Equal(t, tt.message, app.Status.Summary.Message)
		})
	}
}

func TestDeleting(t *testing.T) {
	report := running(v2)
	report.RemoveError = "delete release: forbidden"

	app := project(t, projection{
		source:         fakeSource{report: report, known: true, obs: ready},
		conditions:     installedConditions(),
		currentVersion: v1,
		deleting:       true,
	})

	want := map[string]cond{}
	for _, c := range []string{ConditionInstalled, ConditionUpdateInstalled, ConditionReady, ConditionScaled, ConditionManaged, ConditionConfigurationApplied} {
		want[c] = wantFalse("Deleting")
	}

	assertConditions(t, app, want)
	assert.Equal(t, stateDeleting, app.Status.Summary.State)
	assert.Contains(t, app.Status.Summary.Message, "delete release: forbidden")
	assert.Equal(t, v1, app.Status.CurrentVersion.Version, "nothing is committed on the way out")
}

func TestDeletingSummaryNamesTheUninstallFailure(t *testing.T) {
	report := running(v1)
	report.Current.HooksProcessed = model.FalseCondition(model.ReasonHookFailed, "beforeDeleteHelm: boom")

	app := project(t, projection{
		source:         fakeSource{report: report, known: true, obs: ready},
		conditions:     installedConditions(),
		currentVersion: v1,
		deleting:       true,
	})

	assert.Equal(t, stateDeleting, app.Status.Summary.State)
	assert.Equal(t, summaryDeleting.message+": HookFailed: beforeDeleteHelm: boom", app.Status.Summary.Message)
}

func TestUnknownKeepsEachExternalCondition(t *testing.T) {
	// Each condition carries a value no current input would produce; a restart reports nothing,
	// so every one of them stays.
	for _, condType := range []string{
		ConditionInstalled, ConditionUpdateInstalled, ConditionReady, ConditionScaled, ConditionManaged, ConditionConfigurationApplied,
	} {
		t.Run(condType, func(t *testing.T) {
			status := metav1.ConditionFalse
			if condType == ConditionInstalled {
				status = metav1.ConditionTrue // the gate: False would remove the others
			}

			previous := metav1.Condition{Type: condType, Status: status, Reason: "Previous", Message: "kept"}
			conditions := with(installedConditions(), previous)

			app := project(t, projection{
				source:         fakeSource{known: true, report: model.Report{CurrentVersion: v1}},
				conditions:     conditions,
				currentVersion: v1,
			})

			got := meta.FindStatusCondition(app.Status.Conditions, condType)
			require.NotNil(t, got)
			assert.Equal(t, previous.Status, got.Status)
			assert.Equal(t, previous.Reason, got.Reason)
			assert.Equal(t, previous.Message, got.Message)
		})
	}
}
