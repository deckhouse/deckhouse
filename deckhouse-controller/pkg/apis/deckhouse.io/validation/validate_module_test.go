/*
Copyright 2026 Flant JSC

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

package validation

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/module-sdk/pkg/settingscheck"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1beta1"
)

func newModuleAdmissionReview(operation, username string, module *v1alpha1.Module) *admissionv1.AdmissionReview {
	review := &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Request: &admissionv1.AdmissionRequest{
			UID:       "test-uid",
			Operation: admissionv1.Operation(operation),
			UserInfo:  authenticationv1.UserInfo{Username: username},
		},
	}

	if module != nil {
		raw, _ := json.Marshal(module)
		review.Request.Object = runtime.RawExtension{Raw: raw}
	}

	return review
}

// TestModuleValidationHandler verifies that only the deckhouse service account is
// allowed to mutate Module objects; every other identity is rejected.
func TestModuleValidationHandler(t *testing.T) {
	module := &v1alpha1.Module{ObjectMeta: metav1.ObjectMeta{Name: "test-module"}}

	tests := []struct {
		name        string
		operation   string
		username    string
		wantAllowed bool
		wantMessage string
	}{
		{
			name:        "deckhouse service account is allowed to create",
			operation:   "CREATE",
			username:    deckhouseServiceAccount,
			wantAllowed: true,
		},
		{
			name:        "deckhouse service account is allowed to update",
			operation:   "UPDATE",
			username:    deckhouseServiceAccount,
			wantAllowed: true,
		},
		{
			name:        "regular user is forbidden from changing modules",
			operation:   "UPDATE",
			username:    "system:serviceaccount:default:some-user",
			wantAllowed: false,
			wantMessage: "manual Module change is forbidden",
		},
		{
			name:        "empty username is forbidden",
			operation:   "CREATE",
			username:    "",
			wantAllowed: false,
			wantMessage: "manual Module change is forbidden",
		},
		{
			name:        "cluster admin is forbidden from changing modules",
			operation:   "DELETE",
			username:    "kubernetes-admin",
			wantAllowed: false,
			wantMessage: "manual Module change is forbidden",
		},
	}

	handler := moduleValidationHandler()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := module
			if tt.operation == "DELETE" {
				obj = nil
			}

			review := newModuleAdmissionReview(tt.operation, tt.username, obj)
			if tt.operation == "DELETE" {
				review.Request.OldObject = func() runtime.RawExtension {
					raw, _ := json.Marshal(module)
					return runtime.RawExtension{Raw: raw}
				}()
			}

			resp := callHandler(t, handler, review)

			if tt.wantAllowed {
				assert.True(t, resp.Allowed)
				return
			}

			require.False(t, resp.Allowed)
			require.NotNil(t, resp.Result)
			assert.Contains(t, resp.Result.Message, tt.wantMessage)
		})
	}
}

// newModuleV2AdmissionReview builds a review of a v1beta1 Module sent by username.
func newModuleV2AdmissionReview(operation, username string, obj, oldObj *v1beta1.Module) *admissionv1.AdmissionReview {
	var newObj, prev any
	if obj != nil {
		newObj = obj
	}
	if oldObj != nil {
		prev = oldObj
	}

	review := newModuleConfigAdmissionReview(operation, newObj, prev)
	review.Request.UserInfo = authenticationv1.UserInfo{Username: username}

	return review
}

// withEnabled returns the Module with spec.enabled set.
func withEnabled(module *v1beta1.Module, enabled bool) *v1beta1.Module {
	module.Spec.Enabled = boolPtr(enabled)
	return module
}

// TestModuleValidationHandlerV2 covers the Module v2 webhook: who is trusted and what a user edit must pass.
func TestModuleValidationHandlerV2(t *testing.T) {
	const user = "kubernetes-admin"

	allowDisable := map[string]string{v1alpha1.PackageAnnotationAllowDisabling: "true"}
	confirm := &v1alpha1.ModulePackageVersionStatusMetadata{
		DisableOptions: &v1alpha1.PackageDisableOptions{
			Confirmation: true,
			Messages:     &v1alpha1.PackageDisableMessages{En: confirmationMessage},
		},
		Requirements: &v1alpha1.PackageRequirements{
			Modules: &v1alpha1.PackageModulesRequirements{
				Mandatory: []v1alpha1.PackageModuleDependency{{Name: "parent"}},
			},
		},
	}

	draft := newMPV(nil, nil)
	draft.Labels = map[string]string{v1alpha1.PackageLabelDraft: "true"}

	tests := []struct {
		name             string
		operation        string
		username         string
		module           *v1beta1.Module
		oldModule        *v1beta1.Module
		objs             []client.Object
		currentlyEnabled bool
		checkErr         error
		wantAllowed      bool
		wantMessage      string
		wantCheck        bool
	}{
		{
			name:        "a user naming a missing package version is rejected",
			operation:   "CREATE",
			username:    user,
			module:      newModuleV2(false),
			wantMessage: "not found",
		},
		{
			name:        "a draft package version is rejected",
			operation:   "CREATE",
			username:    user,
			module:      newModuleV2(false),
			objs:        []client.Object{draft},
			wantMessage: "is draft",
		},
		{
			name:        "settings violating the package version schema are rejected",
			operation:   "UPDATE",
			username:    user,
			module:      withSettings(newModuleV2(false), map[string]any{"storageClass": "fast"}),
			oldModule:   newModuleV2(false),
			objs:        []client.Object{newMPV(nil, v2SettingsSchema())},
			wantMessage: "spec.settings are not valid",
		},
		{
			name:        "enabling runs the package version requirements",
			operation:   "UPDATE",
			username:    user,
			module:      withEnabled(newModuleV2(false), true),
			oldModule:   newModuleV2(false),
			objs:        []client.Object{newMPV(confirm, nil)},
			wantAllowed: true,
			wantCheck:   true,
		},
		{
			name:        "unmet requirements are rejected",
			operation:   "UPDATE",
			username:    user,
			module:      withEnabled(newModuleV2(false), true),
			oldModule:   newModuleV2(false),
			objs:        []client.Object{newMPV(confirm, nil)},
			checkErr:    assert.AnError,
			wantMessage: "requirements are not met",
			wantCheck:   true,
		},
		{
			name:             "disabling a running module needs confirmation",
			operation:        "UPDATE",
			username:         user,
			module:           withEnabled(newModuleV2(true), false),
			oldModule:        withEnabled(newModuleV2(true), true),
			objs:             []client.Object{newMPV(confirm, nil)},
			currentlyEnabled: true,
			wantMessage:      confirmationMessage,
		},
		{
			name:      "the allow-disabling annotation lets the disable through",
			operation: "UPDATE",
			username:  user,
			module: func() *v1beta1.Module {
				module := withEnabled(newModuleV2(true), false)
				module.Annotations = allowDisable
				return module
			}(),
			oldModule:        withEnabled(newModuleV2(true), true),
			objs:             []client.Object{newMPV(confirm, nil)},
			currentlyEnabled: true,
			wantAllowed:      true,
		},
		{
			name:             "deleting a running module needs confirmation",
			operation:        "DELETE",
			username:         user,
			oldModule:        newModuleV2(true),
			objs:             []client.Object{newMPV(confirm, nil)},
			currentlyEnabled: true,
			wantMessage:      confirmationMessage,
		},
		{
			name:      "the allow-disabling annotation lets the delete through",
			operation: "DELETE",
			username:  user,
			oldModule: func() *v1beta1.Module {
				module := newModuleV2(true)
				module.Annotations = allowDisable
				return module
			}(),
			objs:             []client.Object{newMPV(confirm, nil)},
			currentlyEnabled: true,
			wantAllowed:      true,
		},
		{
			name:        "deleting a disabled module is allowed",
			operation:   "DELETE",
			username:    user,
			oldModule:   newModuleV2(false),
			objs:        []client.Object{newMPV(confirm, nil)},
			wantAllowed: true,
		},
		{
			name:        "connect is not applicable",
			operation:   "CONNECT",
			username:    user,
			module:      newModuleV2(false),
			wantMessage: "is not applicable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &fakeModulePackageManager{
				enabled:        map[string]bool{v2ModuleName: tt.currentlyEnabled},
				validateResult: settingscheck.Result{Valid: true},
				checkErr:       tt.checkErr,
			}

			cli, metricStorage, settings := newTestDepsV2(t, false, tt.objs...)
			handler := moduleValidationHandlerV2(cli, manager, metricStorage, settings)

			resp := callHandler(t, handler, newModuleV2AdmissionReview(tt.operation, tt.username, tt.module, tt.oldModule))

			assertResponse(t, resp, tt.wantAllowed, tt.wantMessage)
			assert.Equal(t, tt.wantCheck, manager.checkCalled)
		})
	}
}

// TestModuleValidationHandlerV2_VersionChange confirms an UPDATE is checked against the version it moves to.
func TestModuleValidationHandlerV2_VersionChange(t *testing.T) {
	cli, metricStorage, settings := newTestDepsV2(t, false, newMPV(nil, nil))
	handler := moduleValidationHandlerV2(cli, &fakeModulePackageManager{}, metricStorage, settings)

	upgraded := newModuleV2(false)
	upgraded.Spec.PackageVersion = "v1.3.0"

	resp := callHandler(t, handler, newModuleV2AdmissionReview("UPDATE", "kubernetes-admin", upgraded, newModuleV2(false)))

	assertResponse(t, resp, false, v1alpha1.MakeModulePackageVersionName(v2Repository, v2ModuleName, "v1.3.0"))
}

// withSettings returns the Module with spec.settings set, written for settings version 1.
func withSettings(module *v1beta1.Module, settings map[string]any) *v1beta1.Module {
	module.Spec.SettingsVersion = 1
	module.Spec.Settings = v1alpha1.MakeMappedFields(settings)
	return module
}

// moduleUser is a user edit; the deckhouse service account never reaches the webhook.
const moduleUser = "kubernetes-admin"

// newModuleRequestV2 returns the module with the spec a user writes: enabled, settings at version 1 and annotations.
func newModuleRequestV2(module *v1beta1.Module, enabled *bool, settings map[string]any, annotations map[string]string) *v1beta1.Module {
	module.Spec.Enabled = enabled
	module.Spec.SettingsVersion = 1
	if settings != nil {
		module.Spec.Settings = v1alpha1.MakeMappedFields(settings)
	}
	module.Annotations = annotations

	return module
}

// newModuleHandlerV2 builds the Module v2 webhook over objs.
func newModuleHandlerV2(t *testing.T, manager *fakeModulePackageManager, allowExperimental bool, objs ...client.Object) http.Handler {
	t.Helper()

	cli, metricStorage, settings := newTestDepsV2(t, allowExperimental, objs...)

	return moduleValidationHandlerV2(cli, manager, metricStorage, settings)
}

// TestModuleValidationHandlerV2_Settings covers which side validates the settings: the runtime or the package version schema.
func TestModuleValidationHandlerV2_Settings(t *testing.T) {
	tests := []struct {
		name           string
		installed      bool
		dev            bool
		convert        bool
		version        int
		settings       map[string]any
		validateResult settingscheck.Result
		wantAllowed    bool
		wantMessage    string
		wantRuntime    bool
		wantWarnings   []string
	}{
		{
			name:         "not installed: older settings are converted before the schema check",
			convert:      true,
			settings:     map[string]any{"count": "2"},
			wantAllowed:  true,
			wantWarnings: []string{"spec.settingsVersion=1 is obsolete. Please migrate spec.settings to the latest version 2"},
		},
		{
			name:        "the latest settings version does not warn",
			convert:     true,
			version:     2,
			settings:    map[string]any{"replicas": "2"},
			wantAllowed: true,
		},
		{
			name:        "not installed: latest settings are not converted",
			convert:     true,
			version:     2,
			settings:    map[string]any{"count": "2"},
			wantMessage: "spec.settings are not valid",
		},
		{
			name:        "a settings version beyond the chain is rejected",
			convert:     true,
			version:     3,
			settings:    map[string]any{"replicas": "2"},
			wantMessage: "settings version 3 is not supported, the latest is 2",
		},
		{
			name:        "a settings version beyond a package without conversions is rejected",
			version:     2,
			settings:    map[string]any{"replicas": "2"},
			wantMessage: "the latest is 1",
		},
		{
			name:           "installed: the runtime validates older settings at their version",
			installed:      true,
			convert:        true,
			settings:       map[string]any{"count": "2"},
			validateResult: settingscheck.Result{Valid: true},
			wantAllowed:    true,
			wantWarnings:   []string{"spec.settingsVersion=1 is obsolete. Please migrate spec.settings to the latest version 2"},
			wantRuntime:    true,
		},
		{
			name:        "not installed: settings matching the package version schema are allowed",
			settings:    map[string]any{"replicas": "2"},
			wantAllowed: true,
		},
		{
			name:        "not installed: settings violating the package version schema are rejected",
			settings:    map[string]any{"storageClass": "fast"},
			wantMessage: "spec.settings are not valid",
		},
		{
			name:           "installed: the runtime validates and its warnings pass through",
			installed:      true,
			settings:       map[string]any{"storageClass": "fast"},
			validateResult: settingscheck.Result{Valid: true, Warnings: []string{"deprecated field"}},
			wantAllowed:    true,
			wantRuntime:    true,
			wantWarnings:   []string{"deprecated field"},
		},
		{
			name:           "installed: a runtime rejection is returned",
			installed:      true,
			settings:       map[string]any{"replicas": "2"},
			validateResult: settingscheck.Result{Valid: false, Message: "replicas out of range"},
			wantMessage:    "replicas out of range",
			wantRuntime:    true,
		},
		{
			name:           "dev: only the dev build validates, the package version schema and chain are not used",
			dev:            true,
			convert:        true,
			version:        3,
			settings:       map[string]any{"storageClass": "fast"},
			validateResult: settingscheck.Result{Valid: true},
			wantAllowed:    true,
			wantRuntime:    true,
		},
		{
			name:           "dev: a dev build rejection is returned",
			dev:            true,
			settings:       map[string]any{"replicas": "2"},
			validateResult: settingscheck.Result{Valid: false, Message: "replicas out of range"},
			wantMessage:    "replicas out of range",
			wantRuntime:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &fakeModulePackageManager{validateResult: tt.validateResult}
			mpv := newMPV(nil, v2SettingsSchema())
			if tt.convert {
				mpv = withConversions(mpv, renameCountConversion)
			}

			module := newModuleV2(tt.installed)
			if tt.dev {
				module.Status.CurrentVersion = &v1beta1.ModuleStatusVersion{
					Version: "dev-0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
				}
			}

			req := newModuleRequestV2(module, nil, tt.settings, nil)
			if tt.version != 0 {
				req.Spec.SettingsVersion = tt.version
			}

			resp := callHandler(t, newModuleHandlerV2(t, manager, false, mpv), newModuleV2AdmissionReview("CREATE", moduleUser, req, nil))

			assertResponse(t, resp, tt.wantAllowed, tt.wantMessage)
			assert.Equal(t, tt.wantRuntime, manager.validateCalled)
			if tt.wantRuntime {
				assert.Equal(t, req.Spec.SettingsVersion, manager.validateVersion, "the runtime converts from the request's version")
			}
			if tt.wantWarnings != nil || tt.wantAllowed {
				assert.Equal(t, tt.wantWarnings, resp.Warnings)
			}
		})
	}
}

// TestModuleValidationHandlerV2_Transition covers the UPDATE-only rules: CEL transitions and immutable fields.
func TestModuleValidationHandlerV2_Transition(t *testing.T) {
	tests := []struct {
		name        string
		ran         bool
		convert     bool
		oldSettings map[string]any
		newSettings map[string]any
		oldVersion  int
		newVersion  int
		wantAllowed bool
		wantMessage string
	}{
		{
			name:        "ran (checkpoint present): changing an immutable field is rejected",
			ran:         true,
			oldSettings: map[string]any{"replicas": "1", "storageClass": "slow"},
			newSettings: map[string]any{"replicas": "1", "storageClass": "fast"},
			wantMessage: "storageClass",
		},
		{
			name:        "never activated: an immutable field is still open",
			oldSettings: map[string]any{"replicas": "1", "storageClass": "slow"},
			newSettings: map[string]any{"replicas": "1", "storageClass": "fast"},
			wantAllowed: true,
		},
		{
			name:        "a CEL transition rule rejects the change",
			oldSettings: map[string]any{"replicas": "1", "tier": "gold"},
			newSettings: map[string]any{"replicas": "1", "tier": "silver"},
			wantMessage: "gold tier is sticky",
		},
		{
			name:        "a settings version change compares both sides at the latest version",
			ran:         true,
			convert:     true,
			oldSettings: map[string]any{"count": "1", "storageClass": "slow"},
			newSettings: map[string]any{"replicas": "1", "storageClass": "fast"},
			newVersion:  2,
			wantMessage: "storageClass",
		},
		{
			name:        "a settings version change keeping the values is allowed",
			ran:         true,
			convert:     true,
			oldSettings: map[string]any{"count": "1", "storageClass": "slow", "tier": "gold"},
			newSettings: map[string]any{"replicas": "1", "storageClass": "slow", "tier": "gold"},
			newVersion:  2,
			wantAllowed: true,
		},
		{
			name:        "a settings version change runs the CEL transition rules",
			convert:     true,
			oldSettings: map[string]any{"count": "1", "tier": "gold"},
			newSettings: map[string]any{"replicas": "1", "tier": "silver"},
			newVersion:  2,
			wantMessage: "gold tier is sticky",
		},
		{
			name:        "old settings beyond the chain skip the transition rules",
			ran:         true,
			oldSettings: map[string]any{"replicas": "1", "storageClass": "slow", "tier": "gold"},
			newSettings: map[string]any{"replicas": "1", "storageClass": "fast", "tier": "silver"},
			oldVersion:  2,
			wantAllowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &fakeModulePackageManager{validateResult: settingscheck.Result{Valid: true}}
			base := func() *v1beta1.Module {
				if tt.ran {
					return ranModuleV2(`{"replicas":"1","storageClass":"slow"}`)
				}

				return newModuleV2(false)
			}

			mpv := newMPV(nil, v2SettingsSchema())
			if tt.convert {
				mpv = withConversions(mpv, renameCountConversion)
			}

			oldReq := newModuleRequestV2(base(), nil, tt.oldSettings, nil)
			if tt.oldVersion != 0 {
				oldReq.Spec.SettingsVersion = tt.oldVersion
			}

			newReq := newModuleRequestV2(base(), nil, tt.newSettings, nil)
			if tt.newVersion != 0 {
				newReq.Spec.SettingsVersion = tt.newVersion
			}

			resp := callHandler(t, newModuleHandlerV2(t, manager, false, mpv), newModuleV2AdmissionReview("UPDATE", moduleUser, newReq, oldReq))

			assertResponse(t, resp, tt.wantAllowed, tt.wantMessage)
		})
	}
}

// TestModuleValidationHandlerV2_ImmutableBeforeFirstRun (F50): the engine writes
// status.currentVersion at activation, before the first run; a module whose first run failed has no
// lastAppliedConfiguration, has frozen nothing, and its immutable fields stay open (decision 2026-10-09).
func TestModuleValidationHandlerV2_ImmutableBeforeFirstRun(t *testing.T) {
	base := func() *v1beta1.Module {
		module := newModuleV2(true)
		module.Status.Conditions = []metav1.Condition{{
			Type: "Installed", Status: metav1.ConditionFalse, Reason: "ManifestsApplyFailed",
		}}

		return module
	}

	manager := &fakeModulePackageManager{validateResult: settingscheck.Result{Valid: true}}

	oldReq := newModuleRequestV2(base(), nil, map[string]any{"replicas": "1", "storageClass": "missing"}, nil)
	newReq := newModuleRequestV2(base(), nil, map[string]any{"replicas": "1", "storageClass": "fast"}, nil)

	resp := callHandler(t, newModuleHandlerV2(t, manager, false, newMPV(nil, v2SettingsSchema())), newModuleV2AdmissionReview("UPDATE", moduleUser, newReq, oldReq))

	assertResponse(t, resp, true, "")
}

// TestModuleValidationHandlerV2_Enabling covers the checks a disabled module passes to be enabled.
func TestModuleValidationHandlerV2_Enabling(t *testing.T) {
	meta := func(stage, group string, weight int32) *v1alpha1.ModulePackageVersionStatusMetadata {
		return &v1alpha1.ModulePackageVersionStatusMetadata{
			Stage:          stage,
			ExclusiveGroup: group,
			Weight:         weight,
			Requirements: &v1alpha1.PackageRequirements{
				Kubernetes: &v1alpha1.VersionConstraint{Constraint: ">= 1.28"},
				Modules: &v1alpha1.PackageModulesRequirements{
					Mandatory: []v1alpha1.PackageModuleDependency{{Name: "parent"}},
				},
			},
		}
	}
	critical := func(m *v1alpha1.ModulePackageVersionStatusMetadata) *v1alpha1.ModulePackageVersionStatusMetadata {
		m.Critical = true
		return m
	}

	tests := []struct {
		name              string
		meta              *v1alpha1.ModulePackageVersionStatusMetadata
		currentlyEnabled  bool
		allowExperimental bool
		checkErr          error
		exclusiveErr      error
		wantAllowed       bool
		wantMessage       string
		wantCheck         bool
		wantOrder         schedule.Order
		wantExclusive     string
	}{
		{
			name:        "requirements are checked with a critical module's order",
			meta:        critical(meta("", "", 20)),
			wantAllowed: true,
			wantCheck:   true,
			wantOrder:   20,
		},
		{
			name:        "a weighted non-critical module is functional",
			meta:        meta("", "", 20),
			wantAllowed: true,
			wantCheck:   true,
			wantOrder:   schedule.FunctionalOrder,
		},
		{
			name:        "a module without weight takes the functional order",
			meta:        meta("", "", 0),
			wantAllowed: true,
			wantCheck:   true,
			wantOrder:   schedule.FunctionalOrder,
		},
		{
			name:        "unmet requirements are rejected",
			meta:        meta("", "", 0),
			checkErr:    errors.New("parent is disabled"),
			wantMessage: "parent is disabled",
			wantCheck:   true,
			wantOrder:   schedule.FunctionalOrder,
		},
		{
			name:             "an already enabled module is not re-checked",
			meta:             meta("", "", 0),
			currentlyEnabled: true,
			wantAllowed:      true,
		},
		{
			name:        "an experimental module is rejected unless allowed",
			meta:        meta("Experimental", "", 0),
			wantMessage: "is experimental",
		},
		{
			name:              "an allowed experimental module passes",
			meta:              meta("Experimental", "", 0),
			allowExperimental: true,
			wantAllowed:       true,
			wantCheck:         true,
			wantOrder:         schedule.FunctionalOrder,
		},
		{
			name:          "a conflict in the exclusive group is rejected",
			meta:          meta("", "cni", 0),
			exclusiveErr:  errors.New("another module enabled"),
			wantMessage:   "exclusiveGroup cni",
			wantCheck:     true,
			wantOrder:     schedule.FunctionalOrder,
			wantExclusive: "cni",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &fakeModulePackageManager{
				enabled:      map[string]bool{v2ModuleName: tt.currentlyEnabled},
				checkErr:     tt.checkErr,
				exclusiveErr: tt.exclusiveErr,
			}
			handler := newModuleHandlerV2(t, manager, tt.allowExperimental, newMPV(tt.meta, nil))

			req := newModuleRequestV2(newModuleV2(false), boolPtr(true), nil, nil)
			resp := callHandler(t, handler, newModuleV2AdmissionReview("CREATE", moduleUser, req, nil))

			assertResponse(t, resp, tt.wantAllowed, tt.wantMessage)
			require.Equal(t, tt.wantCheck, manager.checkCalled)
			if tt.wantCheck {
				assert.Equal(t, tt.wantOrder, manager.gotConstraints.Order)
				assert.NotNil(t, manager.gotConstraints.Kubernetes)
				assert.Contains(t, manager.gotConstraints.Dependencies, "parent")
			}
			assert.Equal(t, tt.wantExclusive, manager.gotExclusive)
		})
	}
}

// TestModuleValidationHandlerV2_DisableConfirmation covers the confirmation the package version may ask for.
func TestModuleValidationHandlerV2_DisableConfirmation(t *testing.T) {
	allowDisable := map[string]string{v1alpha1.PackageAnnotationAllowDisabling: "true"}
	confirm := &v1alpha1.ModulePackageVersionStatusMetadata{
		DisableOptions: &v1alpha1.PackageDisableOptions{
			Confirmation: true,
			Messages:     &v1alpha1.PackageDisableMessages{En: confirmationMessage},
		},
	}

	tests := []struct {
		name        string
		operation   string
		newModule   *v1beta1.Module
		oldModule   *v1beta1.Module
		wantAllowed bool
		wantMessage string
	}{
		{
			name:        "update: disabling an enabled module is rejected",
			operation:   "UPDATE",
			newModule:   newModuleRequestV2(newModuleV2(true), boolPtr(false), nil, nil),
			oldModule:   newModuleRequestV2(newModuleV2(true), boolPtr(true), nil, nil),
			wantMessage: confirmationMessage,
		},
		{
			name:        "update: the allow-disabling annotation lets it through",
			operation:   "UPDATE",
			newModule:   newModuleRequestV2(newModuleV2(true), boolPtr(false), nil, allowDisable),
			oldModule:   newModuleRequestV2(newModuleV2(true), boolPtr(true), nil, nil),
			wantAllowed: true,
		},
		{
			name:        "update: the rejection names the package annotation",
			operation:   "UPDATE",
			newModule:   newModuleRequestV2(newModuleV2(true), boolPtr(false), nil, nil),
			oldModule:   newModuleRequestV2(newModuleV2(true), boolPtr(true), nil, nil),
			wantMessage: "Please annotate Module with `" + v1alpha1.PackageAnnotationAllowDisabling + "=true`",
		},
		{
			name:        "update: the legacy modules annotation no longer confirms",
			operation:   "UPDATE",
			newModule:   newModuleRequestV2(newModuleV2(true), boolPtr(false), nil, map[string]string{v1alpha1.ModuleConfigAnnotationAllowDisable: "true"}),
			oldModule:   newModuleRequestV2(newModuleV2(true), boolPtr(true), nil, nil),
			wantMessage: confirmationMessage,
		},
		{
			name:        "create: an explicit disable of a running module is rejected",
			operation:   "CREATE",
			newModule:   newModuleRequestV2(newModuleV2(true), boolPtr(false), nil, nil),
			wantMessage: confirmationMessage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &fakeModulePackageManager{
				enabled:        map[string]bool{v2ModuleName: true},
				validateResult: settingscheck.Result{Valid: true},
			}

			resp := callHandler(t, newModuleHandlerV2(t, manager, false, newMPV(confirm, nil)), newModuleV2AdmissionReview(tt.operation, moduleUser, tt.newModule, tt.oldModule))

			assertResponse(t, resp, tt.wantAllowed, tt.wantMessage)
		})
	}
}
