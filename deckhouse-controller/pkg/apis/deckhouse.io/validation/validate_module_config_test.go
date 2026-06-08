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
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/go_lib/configtools"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

const confirmationMessage = "Disabling this module will stop the cluster."

// fakeModuleStorage implements the moduleStorage interface for tests.
type fakeModuleStorage struct {
	modules   map[string]*moduletypes.Module
	exclusive map[string][]string
}

func (f *fakeModuleStorage) GetModuleByName(name string) (*moduletypes.Module, error) {
	if m, ok := f.modules[name]; ok {
		return m, nil
	}
	return nil, fmt.Errorf("module %q not found", name)
}

func (f *fakeModuleStorage) GetModulesByExclusiveGroup(group string) []string {
	return f.exclusive[group]
}

// fakeModuleManager implements the moduleManager interface for tests.
type fakeModuleManager struct {
	enabled map[string]bool
}

func (f *fakeModuleManager) IsModuleEnabled(name string) bool {
	return f.enabled[name]
}

func (f *fakeModuleManager) GetEnabledModuleNames() []string {
	names := make([]string, 0, len(f.enabled))
	for name, on := range f.enabled {
		if on {
			names = append(names, name)
		}
	}
	return names
}

// fakeDependencyExtender implements the moduleDependencyExtender interface for tests.
type fakeDependencyExtender struct {
	err error
}

func (f *fakeDependencyExtender) CheckEnabling(string) error {
	return f.err
}

func boolPtr(v bool) *bool {
	return &v
}

// newModuleWithDisableOptions builds a real *moduletypes.Module carrying the
// given disable options so that GetConfirmationDisableReason returns them.
func newModuleWithDisableOptions(t *testing.T, name string, confirmation bool, message string) *moduletypes.Module {
	t.Helper()

	def := &moduletypes.Definition{
		Name: name,
		DisableOptions: &v1alpha1.ModuleDisableOptions{
			Confirmation: confirmation,
			Message:      message,
		},
	}

	module, err := moduletypes.NewModule(def, nil, nil, nil, log.NewNop())
	require.NoError(t, err)

	return module
}

func newModuleConfig(name string, enabled *bool, annotations map[string]string) *v1alpha1.ModuleConfig {
	return &v1alpha1.ModuleConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
		},
		Spec: v1alpha1.ModuleConfigSpec{
			Enabled: enabled,
		},
	}
}

func newModuleConfigAdmissionReview(operation string, obj, oldObj interface{}) *admissionv1.AdmissionReview {
	review := &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Request: &admissionv1.AdmissionRequest{
			UID:       "test-uid",
			Operation: admissionv1.Operation(operation),
		},
	}

	if obj != nil {
		objBytes, _ := json.Marshal(obj)
		review.Request.Object = runtime.RawExtension{Raw: objBytes}
	}

	if oldObj != nil {
		oldObjBytes, _ := json.Marshal(oldObj)
		review.Request.OldObject = runtime.RawExtension{Raw: oldObjBytes}
	}

	return review
}

func newTestHandler(t *testing.T, storage *fakeModuleStorage, manager *fakeModuleManager, dependencyExtender moduleDependencyExtender) http.Handler {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))
	require.NoError(t, v1alpha2.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	metricStorage := metricstorage.NewMetricStorage(metricstorage.WithNewRegistry(), metricstorage.WithLogger(log.NewNop()))
	settings := helpers.NewDeckhouseSettingsContainer(helpers.DefaultDeckhouseSettings(), metricStorage)
	validator := configtools.NewValidator(nil, nil)

	return moduleConfigValidationHandler(fakeClient, storage, metricStorage, manager, validator, settings, dependencyExtender)
}

func callHandler(t *testing.T, handler http.Handler, review *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	t.Helper()

	body, err := json.Marshal(review)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response admissionv1.AdmissionReview
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.NotNil(t, response.Response)

	return response.Response
}

// TestModuleConfigValidationHandler_DisableConfirmation covers the confirmation
// restriction that forbids disabling a module that declares disable.confirmation: true.
func TestModuleConfigValidationHandler_DisableConfirmation(t *testing.T) {
	const moduleName = "critical-module"

	tests := []struct {
		name string
		// module state in the storage
		confirmation bool
		// whether the module is currently enabled (by config or by default)
		currentlyEnabled bool
		// admission request
		operation string
		newConfig *v1alpha1.ModuleConfig
		oldConfig *v1alpha1.ModuleConfig

		// dependencyErr is returned by the module dependency extender on CheckEnabling
		dependencyErr error

		wantAllowed bool
		wantMessage string
		description string
	}{
		{
			name:             "update: disable explicitly enabled module without annotation is rejected",
			confirmation:     true,
			currentlyEnabled: true,
			operation:        "UPDATE",
			newConfig:        newModuleConfig(moduleName, boolPtr(false), nil),
			oldConfig:        newModuleConfig(moduleName, boolPtr(true), nil),
			wantAllowed:      false,
			wantMessage:      confirmationMessage,
			description:      "classic case: enabled:true -> enabled:false requires confirmation",
		},
		{
			name:             "update: disable default-enabled module (no explicit enabled in old config) is rejected",
			confirmation:     true,
			currentlyEnabled: true,
			operation:        "UPDATE",
			newConfig:        newModuleConfig(moduleName, boolPtr(false), nil),
			oldConfig:        newModuleConfig(moduleName, nil, nil),
			wantAllowed:      false,
			wantMessage:      confirmationMessage,
			description:      "fix: module enabled by default still requires confirmation to disable",
		},
		{
			name:             "update: disable with allow-disabling annotation on new config is allowed",
			confirmation:     true,
			currentlyEnabled: true,
			operation:        "UPDATE",
			newConfig:        newModuleConfig(moduleName, boolPtr(false), map[string]string{v1alpha1.ModuleConfigAnnotationAllowDisable: "true"}),
			oldConfig:        newModuleConfig(moduleName, boolPtr(true), nil),
			wantAllowed:      true,
			description:      "annotation on the new config bypasses the confirmation restriction",
		},
		{
			name:             "update: disable with allow-disabling annotation on old config is allowed",
			confirmation:     true,
			currentlyEnabled: true,
			operation:        "UPDATE",
			newConfig:        newModuleConfig(moduleName, boolPtr(false), nil),
			oldConfig:        newModuleConfig(moduleName, boolPtr(true), map[string]string{v1alpha1.ModuleConfigAnnotationAllowDisable: "true"}),
			wantAllowed:      true,
			description:      "annotation on the old config bypasses the confirmation restriction",
		},
		{
			name:             "update: disable module that does not require confirmation is allowed",
			confirmation:     false,
			currentlyEnabled: true,
			operation:        "UPDATE",
			newConfig:        newModuleConfig(moduleName, boolPtr(false), nil),
			oldConfig:        newModuleConfig(moduleName, boolPtr(true), nil),
			wantAllowed:      true,
			description:      "module without disable.confirmation can be disabled freely",
		},
		{
			name:             "update: disabling an already disabled module is allowed",
			confirmation:     true,
			currentlyEnabled: false,
			operation:        "UPDATE",
			newConfig:        newModuleConfig(moduleName, boolPtr(false), nil),
			oldConfig:        newModuleConfig(moduleName, boolPtr(false), nil),
			wantAllowed:      true,
			description:      "no enabled->disabled transition, nothing to confirm",
		},
		{
			name:             "update: enabling a module is allowed",
			confirmation:     true,
			currentlyEnabled: false,
			operation:        "UPDATE",
			newConfig:        newModuleConfig(moduleName, boolPtr(true), nil),
			oldConfig:        newModuleConfig(moduleName, boolPtr(false), nil),
			wantAllowed:      true,
			description:      "enabling is never restricted by disable confirmation",
		},
		{
			name:             "update: enabling a module rejected by dependency constraint",
			confirmation:     true,
			currentlyEnabled: false,
			operation:        "UPDATE",
			newConfig:        newModuleConfig(moduleName, boolPtr(true), nil),
			oldConfig:        newModuleConfig(moduleName, boolPtr(false), nil),
			dependencyErr:    fmt.Errorf("module %q depends on a disabled module", moduleName),
			wantAllowed:      false,
			wantMessage:      "depends on a disabled module",
			description:      "enabling must respect module dependency constraints",
		},
		{
			name:             "create: disabling a currently enabled module without annotation is rejected",
			confirmation:     true,
			currentlyEnabled: true,
			operation:        "CREATE",
			newConfig:        newModuleConfig(moduleName, boolPtr(false), nil),
			wantAllowed:      false,
			wantMessage:      confirmationMessage,
			description:      "fix: creating a config with enabled:false for a running module requires confirmation",
		},
		{
			name:             "create: disabling a currently enabled module with annotation is allowed",
			confirmation:     true,
			currentlyEnabled: true,
			operation:        "CREATE",
			newConfig:        newModuleConfig(moduleName, boolPtr(false), map[string]string{v1alpha1.ModuleConfigAnnotationAllowDisable: "true"}),
			wantAllowed:      true,
			description:      "annotation bypasses confirmation on create",
		},
		{
			name:             "create: disabling a module that is not enabled is allowed",
			confirmation:     true,
			currentlyEnabled: false,
			operation:        "CREATE",
			newConfig:        newModuleConfig(moduleName, boolPtr(false), nil),
			wantAllowed:      true,
			description:      "no running module to protect, disabling is allowed",
		},
		{
			name:             "delete: deleting an enabled confirmation-required config without annotation is rejected",
			confirmation:     true,
			currentlyEnabled: true,
			operation:        "DELETE",
			newConfig:        newModuleConfig(moduleName, boolPtr(true), nil),
			wantAllowed:      false,
			wantMessage:      confirmationMessage,
			description:      "deleting an enabled config effectively disables the module",
		},
		{
			name:             "delete: deleting with allow-disabling annotation is allowed",
			confirmation:     true,
			currentlyEnabled: true,
			operation:        "DELETE",
			newConfig:        newModuleConfig(moduleName, boolPtr(true), map[string]string{v1alpha1.ModuleConfigAnnotationAllowDisable: "true"}),
			wantAllowed:      true,
			description:      "annotation bypasses confirmation on delete",
		},
		{
			name:             "delete: deleting an already disabled config is allowed",
			confirmation:     true,
			currentlyEnabled: false,
			operation:        "DELETE",
			newConfig:        newModuleConfig(moduleName, boolPtr(false), nil),
			wantAllowed:      true,
			description:      "deleting a disabled config does not disable anything",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeModuleStorage{
				modules: map[string]*moduletypes.Module{
					moduleName: newModuleWithDisableOptions(t, moduleName, tt.confirmation, confirmationMessage),
				},
			}
			manager := &fakeModuleManager{
				enabled: map[string]bool{moduleName: tt.currentlyEnabled},
			}
			dependencyExtender := &fakeDependencyExtender{err: tt.dependencyErr}

			handler := newTestHandler(t, storage, manager, dependencyExtender)

			// On DELETE the admission framework carries the object in OldObject.
			review := newModuleConfigAdmissionReview(tt.operation, tt.newConfig, tt.oldConfig)
			if tt.operation == "DELETE" {
				review = newModuleConfigAdmissionReview(tt.operation, nil, tt.newConfig)
			}

			resp := callHandler(t, handler, review)

			if tt.wantAllowed {
				assert.True(t, resp.Allowed, "expected allowed: %s", tt.description)
				return
			}

			require.False(t, resp.Allowed, "expected rejection: %s", tt.description)
			require.NotNil(t, resp.Result)
			assert.Contains(t, resp.Result.Message, tt.wantMessage, "rejection should contain the expected message")

			// disable-confirmation rejections must also explain how to bypass the restriction
			if tt.wantMessage == confirmationMessage {
				assert.Contains(t, resp.Result.Message, "allow-disabling=true", "rejection should explain how to bypass it")
			}
		})
	}
}

// TestModuleConfigValidationHandler_UnknownModule verifies that disabling a module
// that is not present in the storage is allowed regardless of confirmation, because
// there is no definition to enforce.
func TestModuleConfigValidationHandler_UnknownModule(t *testing.T) {
	const moduleName = "unknown-module"

	storage := &fakeModuleStorage{modules: map[string]*moduletypes.Module{}}
	manager := &fakeModuleManager{enabled: map[string]bool{moduleName: true}}

	handler := newTestHandler(t, storage, manager, &fakeDependencyExtender{})
	review := newModuleConfigAdmissionReview(
		"UPDATE",
		newModuleConfig(moduleName, boolPtr(false), nil),
		newModuleConfig(moduleName, boolPtr(true), nil),
	)

	resp := callHandler(t, handler, review)
	assert.True(t, resp.Allowed, "unknown module has no definition to enforce confirmation")
}

func TestDisableConfirmationReason(t *testing.T) {
	tests := []struct {
		name         string
		reason       string
		needConfirm  bool
		wantOK       bool
		wantContains []string
	}{
		{
			name:        "no confirmation needed",
			reason:      "whatever",
			needConfirm: false,
			wantOK:      false,
		},
		{
			name:         "message without trailing dot gets one appended",
			reason:       "Disabling stops the cluster",
			needConfirm:  true,
			wantOK:       true,
			wantContains: []string{"Disabling stops the cluster.", disableReasonSuffix},
		},
		{
			name:         "message with trailing dot is kept as is",
			reason:       "Disabling stops the cluster.",
			needConfirm:  true,
			wantOK:       true,
			wantContains: []string{"Disabling stops the cluster.", disableReasonSuffix},
		},
		{
			name:         "empty message still returns the suffix",
			reason:       "",
			needConfirm:  true,
			wantOK:       true,
			wantContains: []string{disableReasonSuffix},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := disableConfirmationReason(tt.reason, tt.needConfirm)
			require.Equal(t, tt.wantOK, ok)

			if !tt.wantOK {
				assert.Empty(t, got)
				return
			}

			for _, want := range tt.wantContains {
				assert.Contains(t, got, want)
			}
			// the suffix must never be duplicated nor glued to the message without a space
			assert.NotContains(t, got, ".Please")
		})
	}
}
