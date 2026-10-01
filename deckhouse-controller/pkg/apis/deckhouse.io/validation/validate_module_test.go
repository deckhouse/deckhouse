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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/module-sdk/pkg/settingscheck"

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

	allowDisable := map[string]string{v1alpha1.ModuleConfigAnnotationAllowDisable: "true"}
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
			name:        "the deckhouse service account is trusted without a package version",
			operation:   "UPDATE",
			username:    deckhouseServiceAccount,
			module:      newModuleV2(false),
			oldModule:   newModuleV2(false),
			wantAllowed: true,
		},
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

// withSettings returns the Module with spec.settings set.
func withSettings(module *v1beta1.Module, settings map[string]any) *v1beta1.Module {
	module.Spec.Settings = v1alpha1.MakeMappedFields(settings)
	return module
}
