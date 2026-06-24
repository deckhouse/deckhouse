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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/go_lib/configtools"
	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/moduledependency"
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

// newStorageModule builds a real *moduletypes.Module with the given stage and
// exclusive group, used to exercise the experimental / exclusive-group branches.
func newStorageModule(t *testing.T, name, stage, exclusiveGroup string) *moduletypes.Module {
	t.Helper()

	def := &moduletypes.Definition{
		Name:           name,
		Stage:          stage,
		ExclusiveGroup: exclusiveGroup,
	}

	module, err := moduletypes.NewModule(def, nil, nil, nil, log.NewNop())
	require.NoError(t, err)

	return module
}

// newModuleCR builds a v1alpha1.Module custom resource so that the cli.Get
// lookup in the handler returns an object instead of a NotFound error.
func newModuleCR(name string, availableSources []string, stage string) *v1alpha1.Module {
	return &v1alpha1.Module{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Properties: v1alpha1.ModuleProperties{
			Stage:            stage,
			AvailableSources: availableSources,
		},
	}
}

func newModuleConfigFull(name string, enabled *bool, source, updatePolicy string) *v1alpha1.ModuleConfig {
	cfg := newModuleConfig(name, enabled, nil)
	cfg.Spec.Source = source
	cfg.Spec.UpdatePolicy = updatePolicy

	return cfg
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

	return newTestHandlerWithObjects(t, storage, manager, dependencyExtender, false)
}

// newTestHandlerWithObjects builds the validation handler with a fake client
// seeded with the given objects (e.g. Module / ModuleUpdatePolicy CRs). Seeding
// a Module CR is what lets tests exercise the branches after the cli.Get lookup
// in moduleConfigValidationHandler instead of always hitting the IsNotFound
// short-circuit. allowExperimental toggles the AllowExperimentalModules setting.
func newTestHandlerWithObjects(t *testing.T, storage *fakeModuleStorage, manager *fakeModuleManager, dependencyExtender moduleDependencyExtender, allowExperimental bool, objs ...client.Object) http.Handler {
	t.Helper()

	return newTestHandlerWithValidator(t, storage, manager, dependencyExtender, allowExperimental, nil, configtools.NewValidator(nil, nil), objs...)
}

// newTestHandlerWithValidator is the most flexible builder: it also lets a test
// supply the config validator, which is needed to exercise the
// configValidator.Validate branch in the handler.
func newTestHandlerWithValidator(t *testing.T, storage *fakeModuleStorage, manager *fakeModuleManager, dependencyExtender moduleDependencyExtender, allowExperimental bool, allowedExperimental []string, validator *configtools.Validator, objs ...client.Object) http.Handler {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))
	require.NoError(t, v1alpha2.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()

	metricStorage := metricstorage.NewMetricStorage(metricstorage.WithNewRegistry(), metricstorage.WithLogger(log.NewNop()))

	deckhouseSettings := helpers.DefaultDeckhouseSettings()
	deckhouseSettings.AllowExperimentalModules = allowExperimental
	deckhouseSettings.AllowedExperimentalModules = allowedExperimental
	settings := helpers.NewDeckhouseSettingsContainer(deckhouseSettings, metricStorage)

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
		// expectCheckEnabling asserts whether the dependency extender must be
		// consulted for this case (i.e. the request performs an enabling transition)
		expectCheckEnabling bool

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
			name:                "update: enabling a module is allowed",
			confirmation:        true,
			currentlyEnabled:    false,
			operation:           "UPDATE",
			newConfig:           newModuleConfig(moduleName, boolPtr(true), nil),
			oldConfig:           newModuleConfig(moduleName, boolPtr(false), nil),
			expectCheckEnabling: true,
			wantAllowed:         true,
			description:         "enabling is never restricted by disable confirmation, but the dependency extender is consulted",
		},
		{
			name:                "update: enabling a module rejected by dependency constraint",
			confirmation:        true,
			currentlyEnabled:    false,
			operation:           "UPDATE",
			newConfig:           newModuleConfig(moduleName, boolPtr(true), nil),
			oldConfig:           newModuleConfig(moduleName, boolPtr(false), nil),
			dependencyErr:       fmt.Errorf("module %q depends on a disabled module", moduleName),
			expectCheckEnabling: true,
			wantAllowed:         false,
			wantMessage:         "depends on a disabled module",
			description:         "enabling must respect module dependency constraints",
		},
		{
			name:                "update: keeping an already enabled module enabled does not re-check dependency",
			confirmation:        true,
			currentlyEnabled:    true,
			operation:           "UPDATE",
			newConfig:           newModuleConfig(moduleName, boolPtr(true), nil),
			oldConfig:           newModuleConfig(moduleName, boolPtr(true), nil),
			expectCheckEnabling: false,
			wantAllowed:         true,
			description:         "no disabled->enabled transition, the dependency extender must not be consulted",
		},
		{
			name:                "update: enabling a default-disabled module (no explicit old enabled) is checked by dependency",
			confirmation:        true,
			currentlyEnabled:    false,
			operation:           "UPDATE",
			newConfig:           newModuleConfig(moduleName, boolPtr(true), nil),
			oldConfig:           newModuleConfig(moduleName, nil, nil),
			dependencyErr:       fmt.Errorf("module %q depends on a disabled module", moduleName),
			expectCheckEnabling: true,
			wantAllowed:         false,
			wantMessage:         "depends on a disabled module",
			description:         "an absent old enabled flag is treated as disabled, so enabling triggers the dependency check",
		},
		{
			name:                "create: enabling a module rejected by dependency constraint",
			confirmation:        true,
			currentlyEnabled:    false,
			operation:           "CREATE",
			newConfig:           newModuleConfig(moduleName, boolPtr(true), nil),
			dependencyErr:       fmt.Errorf("module %q depends on a disabled module", moduleName),
			expectCheckEnabling: true,
			wantAllowed:         false,
			wantMessage:         "depends on a disabled module",
			description:         "enabling on create must respect module dependency constraints",
		},
		{
			name:                "create: enabling a module that passes the dependency check is rejected only by the missing Module CR",
			confirmation:        true,
			currentlyEnabled:    false,
			operation:           "CREATE",
			newConfig:           newModuleConfig(moduleName, boolPtr(true), nil),
			expectCheckEnabling: true,
			wantAllowed:         false,
			wantMessage:         "not found",
			description:         "the dependency extender allows enabling; the later Module CR lookup is what rejects it",
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
			// CheckEnabling must be consulted only on an enabling transition.
			// Leaving the mock without expectations fails the test on any
			// unexpected call.
			dependencyExtender := moduledependency.NewIExtenderMock(t)
			if tt.expectCheckEnabling {
				dependencyExtender.CheckEnablingMock.Expect(moduleName).Return(tt.dependencyErr)
			}

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

	// disabling transition, so the dependency extender must not be consulted;
	// the mock is left without expectations to fail on any unexpected call
	dependencyExtender := moduledependency.NewIExtenderMock(t)

	handler := newTestHandler(t, storage, manager, dependencyExtender)
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

// TestModuleConfigValidationHandler_ModuleResolution exercises the branches that
// run only after the Module CR is successfully fetched from the client (i.e. the
// path that the IsNotFound short-circuit otherwise skips): source availability
// validation and the multiple-sources warning.
func TestModuleConfigValidationHandler_ModuleResolution(t *testing.T) {
	const moduleName = "resolvable-module"

	tests := []struct {
		name                string
		operation           string
		newConfig           *v1alpha1.ModuleConfig
		oldConfig           *v1alpha1.ModuleConfig
		moduleCR            *v1alpha1.Module
		expectCheckEnabling bool
		wantAllowed         bool
		wantMessage         string
		wantWarning         string
	}{
		{
			name:        "module CR found, disabled config without source is allowed",
			operation:   "UPDATE",
			newConfig:   newModuleConfigFull(moduleName, boolPtr(false), "", ""),
			oldConfig:   newModuleConfigFull(moduleName, boolPtr(false), "", ""),
			moduleCR:    newModuleCR(moduleName, []string{"alpha"}, ""),
			wantAllowed: true,
		},
		{
			name:        "config referencing an unavailable source is rejected",
			operation:   "UPDATE",
			newConfig:   newModuleConfigFull(moduleName, boolPtr(false), "beta", ""),
			oldConfig:   newModuleConfigFull(moduleName, boolPtr(false), "", ""),
			moduleCR:    newModuleCR(moduleName, []string{"alpha"}, ""),
			wantAllowed: false,
			wantMessage: "unavailable source",
		},
		{
			name:        "config referencing an available source is allowed",
			operation:   "UPDATE",
			newConfig:   newModuleConfigFull(moduleName, boolPtr(false), "alpha", ""),
			oldConfig:   newModuleConfigFull(moduleName, boolPtr(false), "", ""),
			moduleCR:    newModuleCR(moduleName, []string{"alpha", "beta"}, ""),
			wantAllowed: true,
		},
		{
			name:                "enabled module with multiple sources and no source specified warns",
			operation:           "CREATE",
			newConfig:           newModuleConfigFull(moduleName, boolPtr(true), "", ""),
			moduleCR:            newModuleCR(moduleName, []string{"alpha", "beta"}, ""),
			expectCheckEnabling: true,
			wantAllowed:         true,
			wantWarning:         "multiple sources",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeModuleStorage{
				modules: map[string]*moduletypes.Module{
					moduleName: newStorageModule(t, moduleName, "", ""),
				},
			}
			manager := &fakeModuleManager{enabled: map[string]bool{}}

			dependencyExtender := moduledependency.NewIExtenderMock(t)
			if tt.expectCheckEnabling {
				dependencyExtender.CheckEnablingMock.Expect(moduleName).Return(nil)
			}

			handler := newTestHandlerWithObjects(t, storage, manager, dependencyExtender, false, tt.moduleCR)

			review := newModuleConfigAdmissionReview(tt.operation, tt.newConfig, tt.oldConfig)

			resp := callHandler(t, handler, review)

			if tt.wantAllowed {
				assert.True(t, resp.Allowed)
				if tt.wantWarning != "" {
					require.NotEmpty(t, resp.Warnings)
					assert.Contains(t, strings.Join(resp.Warnings, " "), tt.wantWarning)
				}
				return
			}

			require.False(t, resp.Allowed)
			require.NotNil(t, resp.Result)
			assert.Contains(t, resp.Result.Message, tt.wantMessage)
		})
	}
}

// deckhouseConfigSchemaYAML is a minimal config-values schema with an immutable
// rule for the bundle field. It deliberately mirrors the real rule from
// modules/002-deckhouse/openapi/config-values.yaml so that the test reproduces
// production webhook behaviour end-to-end.
const deckhouseConfigSchemaYAML = `
type: object
properties:
  bundle:
    type: string
    enum:
      - Default
      - Minimal
      - Managed
    default: Default
x-deckhouse-validations:
  - expression: "(has(oldSelf.bundle) ? oldSelf.bundle : 'Default') == (has(self.bundle) ? self.bundle : 'Default')"
    message: "bundle is immutable"
`

// newModuleWithCELSchema creates a *moduletypes.Module whose GetBasicModule().GetSchemaStorage()
// contains the supplied config-values schema with x-deckhouse-validations rules.
// The schema is converted from YAML to JSON because addon-operator NewBasicModule expects JSON.
func newModuleWithCELSchema(t *testing.T, name, schemaYAML string) *moduletypes.Module {
	t.Helper()

	configJSON, err := yaml.YAMLToJSON([]byte(schemaYAML))
	require.NoError(t, err, "yaml->json for config schema of %q", name)

	def := &moduletypes.Definition{Name: name}
	mod, err := moduletypes.NewModule(def, nil, configJSON, nil, log.NewNop())
	require.NoError(t, err, "create module %q with config schema", name)

	return mod
}

// newModuleConfigWithSettings creates a ModuleConfig with spec.version and
// spec.settings filled in. When settings is nil, spec.settings is left unset
// so that spec.version == 0 is valid (no settings → no version required).
// An empty (non-nil) map is explicitly set so that extractOldSettings treats it
// as "settings present but no keys" — which is necessary for the CEL regression
// tests that check the "bundle absent → effective Default" behaviour.
func newModuleConfigWithSettings(name string, enabled *bool, version int, settings map[string]any) *v1alpha1.ModuleConfig {
	cfg := newModuleConfig(name, enabled, nil)
	cfg.Spec.Version = version
	if settings != nil {
		cfg.Spec.Settings = v1alpha1.MakeMappedFields(settings)
	}

	return cfg
}

// TestModuleConfigValidationHandler_CELTransition verifies that the admission
// webhook correctly evaluates x-deckhouse-validations CEL transition rules
// (those referencing oldSelf) during UPDATE operations.
//
// This is the end-to-end test for the
// extractOldSettings → validateCELTransition → configSchema → cel.ValidateTransition
func TestModuleConfigValidationHandler_CELTransition(t *testing.T) {
	const moduleName = "deckhouse"

	tests := []struct {
		name        string
		operation   string
		newSettings map[string]any
		newVersion  int
		oldSettings map[string]any
		oldVersion  int
		// expectCheckEnabling is set when the operation causes an enabling
		// transition so the dependency mock must expect a CheckEnabling call.
		expectCheckEnabling bool
		wantAllowed         bool
		wantMessage         string
		description         string
	}{
		{
			name:        "UPDATE: changing immutable bundle field is rejected",
			operation:   "UPDATE",
			newSettings: map[string]any{"bundle": "Minimal"},
			newVersion:  1,
			oldSettings: map[string]any{"bundle": "Default"},
			oldVersion:  1,
			wantAllowed: false,
			wantMessage: "bundle is immutable",
			description: "the webhook must reject a bundle change end-to-end via CEL transition rule",
		},
		{
			name:        "UPDATE: keeping bundle unchanged is allowed",
			operation:   "UPDATE",
			newSettings: map[string]any{"bundle": "Default"},
			newVersion:  1,
			oldSettings: map[string]any{"bundle": "Default"},
			oldVersion:  1,
			wantAllowed: true,
			description: "no change to bundle — transition rule passes",
		},
		{
			name:        "UPDATE: changing bundle from Default to Managed is rejected",
			operation:   "UPDATE",
			newSettings: map[string]any{"bundle": "Managed"},
			newVersion:  1,
			oldSettings: map[string]any{"bundle": "Default"},
			oldVersion:  1,
			wantAllowed: false,
			wantMessage: "bundle is immutable",
			description: "any change to bundle value triggers the immutability rule",
		},
		{
			name:                "CREATE: setting bundle skips CEL; request fails only due to missing Module CR",
			operation:           "CREATE",
			newSettings:         map[string]any{"bundle": "Minimal"},
			newVersion:          1,
			expectCheckEnabling: true,
			wantAllowed:         false,
			wantMessage:         "not found",
			description:         "CEL transition rule is skipped on CREATE; rejection comes from the absent Module CR",
		},
		{
			name:        "UPDATE: old config has no version — transition rule is skipped",
			operation:   "UPDATE",
			newSettings: map[string]any{"bundle": "Minimal"},
			newVersion:  1,
			oldSettings: nil,
			oldVersion:  0, // version == 0 → ExtractLatestSettings returns nil → CEL skipped
			wantAllowed: true,
			description: "old object without spec.version → extractOldSettings returns nil → CEL rules skipped",
		},
		{
			// Regression test: when old settings are present but without an explicit
			// bundle key, the CEL expression "(has(oldSelf.bundle) ? oldSelf.bundle : 'Default')"
			// treats the effective old value as "Default". Setting bundle: Minimal must
			// therefore be rejected because "Default" != "Minimal".
			//
			// Note: a non-empty map without "bundle" is used so that MakeMappedFields produces
			// non-nil Settings — otherwise extractOldSettings returns nil and CEL is skipped entirely.
			name:        "UPDATE: setting bundle when old config had no explicit bundle is rejected",
			operation:   "UPDATE",
			newSettings: map[string]any{"bundle": "Minimal"},
			newVersion:  1,
			oldSettings: map[string]any{"logLevel": "Info"}, // non-empty, no bundle → Settings != nil → CEL runs → effective old bundle is "Default"
			oldVersion:  1,
			wantAllowed: false,
			wantMessage: "bundle is immutable",
			description: "old config without explicit bundle defaults to 'Default' via CEL expression; adding bundle: Minimal must be rejected",
		},
		{
			name:        "UPDATE: old and new config both omit bundle — rule passes",
			operation:   "UPDATE",
			newSettings: map[string]any{"logLevel": "Info"}, // non-empty, no bundle → CEL runs → effective "Default"
			newVersion:  1,
			oldSettings: map[string]any{"logLevel": "Info"}, // non-empty, no bundle → CEL runs → effective "Default"
			oldVersion:  1,
			wantAllowed: true,
			description: "both sides default to 'Default' via CEL expression — no effective change, rule passes",
		},
		{
			name:        "UPDATE: removing explicit bundle when it was Default is allowed",
			operation:   "UPDATE",
			newSettings: map[string]any{"logLevel": "Info"}, // non-empty, no bundle → CEL runs → effective "Default"
			newVersion:  1,
			oldSettings: map[string]any{"bundle": "Default"}, // explicit Default
			oldVersion:  1,
			wantAllowed: true,
			description: "removing an explicit bundle Default is a no-op in effective value — allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeModuleStorage{
				modules: map[string]*moduletypes.Module{
					moduleName: newModuleWithCELSchema(t, moduleName, deckhouseConfigSchemaYAML),
				},
			}
			manager := &fakeModuleManager{enabled: map[string]bool{moduleName: true}}

			dependencyExtender := moduledependency.NewIExtenderMock(t)
			if tt.expectCheckEnabling {
				dependencyExtender.CheckEnablingMock.Expect(moduleName).Return(nil)
			}

			// conversion store is required so that validateCR / ExtractLatestSettings
			// work correctly when spec.version > 0; nil valuesValidator skips the
			// OpenAPI settings check — we only care about CEL here.
			validator := configtools.NewValidator(nil, conversion.NewConversionsStore())

			// For UPDATE operations, a Module CR must be present in the fake client so
			// that resolveModuleSource does not short-circuit before validateCELTransition
			// is reached. For CREATE the CR is intentionally absent so that
			// checkExperimentalFromModuleCR rejects the request with "not found".
			var objs []client.Object
			if tt.operation == "UPDATE" {
				objs = append(objs, newModuleCR(moduleName, []string{}, ""))
			}
			handler := newTestHandlerWithValidator(t, storage, manager, dependencyExtender, false, nil, validator, objs...)

			newCfg := newModuleConfigWithSettings(moduleName, boolPtr(true), tt.newVersion, tt.newSettings)

			var oldCfg *v1alpha1.ModuleConfig
			if tt.operation == "UPDATE" {
				oldCfg = newModuleConfigWithSettings(moduleName, boolPtr(true), tt.oldVersion, tt.oldSettings)
			}

			review := newModuleConfigAdmissionReview(tt.operation, newCfg, oldCfg)
			resp := callHandler(t, handler, review)

			if tt.wantAllowed {
				assert.True(t, resp.Allowed, "expected allowed: %s", tt.description)
				return
			}

			require.False(t, resp.Allowed, "expected rejection: %s", tt.description)
			require.NotNil(t, resp.Result)
			assert.Contains(t, resp.Result.Message, tt.wantMessage,
				"rejection message should contain expected text: %s", tt.description)
		})
	}
}

// TestExtractOldSettings_SkipsWhenNoVersion verifies that extractOldSettings
// (called inside validate() for every UPDATE) gracefully handles old objects
// without spec.version by returning nil — which causes CEL transition rules to
// be skipped rather than returning an error.
func TestExtractOldSettings_SkipsWhenNoVersion(t *testing.T) {
	const moduleName = "deckhouse"

	storage := &fakeModuleStorage{
		modules: map[string]*moduletypes.Module{
			moduleName: newModuleWithCELSchema(t, moduleName, deckhouseConfigSchemaYAML),
		},
	}
	manager := &fakeModuleManager{enabled: map[string]bool{moduleName: true}}
	dependencyExtender := moduledependency.NewIExtenderMock(t)

	validator := configtools.NewValidator(nil, conversion.NewConversionsStore())
	handler := newTestHandlerWithValidator(t, storage, manager, dependencyExtender, false, nil, validator)

	// old object has no spec.version and no spec.settings; ExtractLatestSettings
	// must return nil, nil → CEL check is skipped
	oldCfg := newModuleConfig(moduleName, boolPtr(true), nil)

	// new object carries a bundle setting that would normally fire the immutability
	// rule — but since old settings are nil the check must be skipped
	newCfg := newModuleConfigWithSettings(moduleName, boolPtr(true), 1, map[string]any{"bundle": "Minimal"})

	review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)
	resp := callHandler(t, handler, review)

	assert.True(t, resp.Allowed,
		"UPDATE with old config lacking spec.version must be allowed (CEL skipped)")
}

// TestConfigSchema_ReturnsNilForUnknownModule verifies that when the module is
// absent from moduleStorage (configSchema returns nil), validateCELTransition
// gracefully skips CEL rules instead of returning an error.
func TestConfigSchema_ReturnsNilForUnknownModule(t *testing.T) {
	const moduleName = "unknown-cel-module"

	// storage is empty — configSchema will return nil
	storage := &fakeModuleStorage{modules: map[string]*moduletypes.Module{}}
	manager := &fakeModuleManager{enabled: map[string]bool{moduleName: true}}
	dependencyExtender := moduledependency.NewIExtenderMock(t)

	validator := configtools.NewValidator(nil, conversion.NewConversionsStore())
	handler := newTestHandlerWithValidator(t, storage, manager, dependencyExtender, false, nil, validator)

	newCfg := newModuleConfigWithSettings(moduleName, boolPtr(true), 1, map[string]any{"bundle": "Minimal"})
	oldCfg := newModuleConfigWithSettings(moduleName, boolPtr(true), 1, map[string]any{"bundle": "Default"})

	review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)
	resp := callHandler(t, handler, review)

	// allowed: no schema found in storage, so CEL transition validation is skipped
	assert.True(t, resp.Allowed,
		"when configSchema returns nil for unknown module, CEL must be skipped and request allowed")
}

// TestModuleConfigValidationHandler_DeletePullOverride covers the DELETE guard
// that forbids deleting a module config while a ModulePullOverride for the
// module still exists.
func TestModuleConfigValidationHandler_DeletePullOverride(t *testing.T) {
	const moduleName = "overridden-module"

	tests := []struct {
		name         string
		pullOverride bool
		wantAllowed  bool
		wantMessage  string
	}{
		{
			name:         "delete is rejected while a ModulePullOverride exists",
			pullOverride: true,
			wantAllowed:  false,
			wantMessage:  "delete the ModulePullOverride before deleting the module config",
		},
		{
			name:         "delete is allowed when no ModulePullOverride exists",
			pullOverride: false,
			wantAllowed:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeModuleStorage{
				modules: map[string]*moduletypes.Module{
					moduleName: newStorageModule(t, moduleName, "", ""),
				},
			}
			manager := &fakeModuleManager{enabled: map[string]bool{}}

			// DELETE never enables a module, so the dependency extender must not be consulted.
			dependencyExtender := moduledependency.NewIExtenderMock(t)

			var objs []client.Object
			if tt.pullOverride {
				objs = append(objs, &v1alpha2.ModulePullOverride{
					ObjectMeta: metav1.ObjectMeta{Name: moduleName},
				})
			}

			handler := newTestHandlerWithObjects(t, storage, manager, dependencyExtender, false, objs...)

			// the disabled config keeps the confirmation guard out of the way so that
			// the ModulePullOverride check is the deciding factor
			review := newModuleConfigAdmissionReview("DELETE", nil, newModuleConfig(moduleName, boolPtr(false), nil))

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

// TestModuleConfigValidationHandler_EmbeddedSource verifies that 'Embedded' is
// rejected as an explicit module config source.
func TestModuleConfigValidationHandler_EmbeddedSource(t *testing.T) {
	const moduleName = "embedded-source-module"

	storage := &fakeModuleStorage{
		modules: map[string]*moduletypes.Module{
			moduleName: newStorageModule(t, moduleName, "", ""),
		},
	}
	manager := &fakeModuleManager{enabled: map[string]bool{}}

	// disabled config, no enabling transition - the dependency extender is not consulted
	dependencyExtender := moduledependency.NewIExtenderMock(t)

	handler := newTestHandler(t, storage, manager, dependencyExtender)

	review := newModuleConfigAdmissionReview(
		"UPDATE",
		newModuleConfigFull(moduleName, boolPtr(false), v1alpha1.ModuleSourceEmbedded, ""),
		newModuleConfigFull(moduleName, boolPtr(false), "", ""),
	)

	resp := callHandler(t, handler, review)

	require.False(t, resp.Allowed)
	require.NotNil(t, resp.Result)
	assert.Contains(t, resp.Result.Message, "'Embedded' is a forbidden source")
}

// TestModuleConfigValidationHandler_UpdatePolicy covers the update-policy
// existence check, which is only reachable after the Module CR is found.
func TestModuleConfigValidationHandler_UpdatePolicy(t *testing.T) {
	const (
		moduleName = "policy-module"
		policyName = "my-policy"
	)

	tests := []struct {
		name           string
		registerPolicy bool
		wantAllowed    bool
		wantMessage    string
	}{
		{
			name:           "referencing a missing update policy is rejected",
			registerPolicy: false,
			wantAllowed:    false,
			wantMessage:    "module policy does not exist",
		},
		{
			name:           "referencing an existing update policy is allowed",
			registerPolicy: true,
			wantAllowed:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeModuleStorage{
				modules: map[string]*moduletypes.Module{
					moduleName: newStorageModule(t, moduleName, "", ""),
				},
			}
			manager := &fakeModuleManager{enabled: map[string]bool{moduleName: true}}

			// old=true,new=true: no enabling transition, no dependency check
			dependencyExtender := moduledependency.NewIExtenderMock(t)

			objs := []client.Object{newModuleCR(moduleName, []string{"alpha"}, "")}
			if tt.registerPolicy {
				objs = append(objs, newUpdatePolicy(policyName))
			}

			handler := newTestHandlerWithObjects(t, storage, manager, dependencyExtender, false, objs...)

			review := newModuleConfigAdmissionReview(
				"UPDATE",
				newModuleConfigFull(moduleName, boolPtr(true), "alpha", policyName),
				newModuleConfigFull(moduleName, boolPtr(true), "alpha", policyName),
			)

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

// TestModuleConfigValidationHandler_ExperimentalOnUpdate covers the experimental
// gate on the UPDATE enabling transition, both via the storage definition and
// via the fetched Module CR, plus the "Module CR not found is skipped" branch.
func TestModuleConfigValidationHandler_ExperimentalOnUpdate(t *testing.T) {
	const moduleName = "experimental-update-module"

	tests := []struct {
		name                string
		allowExperimental   bool
		storageStage        string
		registerModuleCR    bool
		moduleCRStage       string
		expectCheckEnabling bool
		wantAllowed         bool
		wantMessage         string
	}{
		{
			name:         "experimental per storage definition is rejected before the dependency check",
			storageStage: moduletypes.ExperimentalModuleStage,
			wantAllowed:  false,
			wantMessage:  "experimental",
		},
		{
			name:                "experimental per Module CR is rejected after the dependency check",
			registerModuleCR:    true,
			moduleCRStage:       v1alpha1.ExperimentalModuleStage,
			expectCheckEnabling: true,
			wantAllowed:         false,
			wantMessage:         "experimental",
		},
		{
			name:                "missing Module CR is skipped and the update is allowed",
			registerModuleCR:    false,
			expectCheckEnabling: true,
			wantAllowed:         true,
		},
		{
			name:                "non-experimental Module CR enabling is allowed",
			registerModuleCR:    true,
			moduleCRStage:       "",
			expectCheckEnabling: true,
			wantAllowed:         true,
		},
		{
			name:                "experimental Module CR enabling is allowed when allowExperimentalModules is true",
			allowExperimental:   true,
			registerModuleCR:    true,
			moduleCRStage:       v1alpha1.ExperimentalModuleStage,
			expectCheckEnabling: true,
			wantAllowed:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeModuleStorage{
				modules: map[string]*moduletypes.Module{
					moduleName: newStorageModule(t, moduleName, tt.storageStage, ""),
				},
			}
			manager := &fakeModuleManager{enabled: map[string]bool{}}

			dependencyExtender := moduledependency.NewIExtenderMock(t)
			if tt.expectCheckEnabling {
				dependencyExtender.CheckEnablingMock.Expect(moduleName).Return(nil)
			}

			var objs []client.Object
			if tt.registerModuleCR {
				objs = append(objs, newModuleCR(moduleName, []string{"alpha"}, tt.moduleCRStage))
			}

			handler := newTestHandlerWithObjects(t, storage, manager, dependencyExtender, tt.allowExperimental, objs...)

			// disabled -> enabled transition triggers the experimental gate
			review := newModuleConfigAdmissionReview(
				"UPDATE",
				newModuleConfigFull(moduleName, boolPtr(true), "alpha", ""),
				newModuleConfigFull(moduleName, boolPtr(false), "", ""),
			)

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

// TestModuleConfigValidationHandler_ConfigValidation covers the config validator
// branch: a hard validation error is rejected, while a validation warning is
// surfaced as an admission warning without blocking the request.
func TestModuleConfigValidationHandler_ConfigValidation(t *testing.T) {
	const moduleName = "validated-module"

	t.Run("validation error is rejected", func(t *testing.T) {
		storage := &fakeModuleStorage{
			modules: map[string]*moduletypes.Module{
				moduleName: newStorageModule(t, moduleName, "", ""),
			},
		}
		manager := &fakeModuleManager{enabled: map[string]bool{moduleName: true}}

		// no enabling transition, the dependency extender must not be consulted
		dependencyExtender := moduledependency.NewIExtenderMock(t)

		// nil validator returns an error for settings supplied without spec.version
		handler := newTestHandlerWithObjects(t, storage, manager, dependencyExtender, false, newModuleCR(moduleName, []string{"alpha"}, ""))

		cfg := newModuleConfigFull(moduleName, boolPtr(true), "alpha", "")
		cfg.Spec.Version = 0
		cfg.Spec.Settings = v1alpha1.MakeMappedFields(map[string]any{"foo": "bar"})

		review := newModuleConfigAdmissionReview("UPDATE", cfg, newModuleConfigFull(moduleName, boolPtr(true), "alpha", ""))

		resp := callHandler(t, handler, review)

		require.False(t, resp.Allowed)
		require.NotNil(t, resp.Result)
		assert.Contains(t, resp.Result.Message, "spec.version is required when spec.settings are specified")
	})

	t.Run("validation warning is surfaced and request is allowed", func(t *testing.T) {
		storage := &fakeModuleStorage{
			modules: map[string]*moduletypes.Module{
				moduleName: newStorageModule(t, moduleName, "", ""),
			},
		}
		manager := &fakeModuleManager{enabled: map[string]bool{moduleName: true}}

		dependencyExtender := moduledependency.NewIExtenderMock(t)

		// a validator with a (empty) conversions store but no values validator emits
		// a warning for a spec.version without spec.settings
		validator := configtools.NewValidator(nil, conversion.NewConversionsStore())
		handler := newTestHandlerWithValidator(t, storage, manager, dependencyExtender, false, nil, validator, newModuleCR(moduleName, []string{"alpha"}, ""))

		cfg := newModuleConfigFull(moduleName, boolPtr(true), "alpha", "")
		cfg.Spec.Version = 1

		review := newModuleConfigAdmissionReview("UPDATE", cfg, newModuleConfigFull(moduleName, boolPtr(true), "alpha", ""))

		resp := callHandler(t, handler, review)

		assert.True(t, resp.Allowed)
		require.NotEmpty(t, resp.Warnings)
		assert.Contains(t, strings.Join(resp.Warnings, " "), "spec.version has no effect without spec.settings")
	})
}

// TestModuleConfigValidationHandler_StorageModuleNotFound covers the branch where
// the Module CR is found (so the handler proceeds past the cli.Get lookup) but
// the module is absent from the storage: accumulated warnings are returned and
// the request is allowed.
func TestModuleConfigValidationHandler_StorageModuleNotFound(t *testing.T) {
	const moduleName = "storage-missing-module"

	// storage is intentionally empty: GetModuleByName will fail
	storage := &fakeModuleStorage{modules: map[string]*moduletypes.Module{}}
	manager := &fakeModuleManager{enabled: map[string]bool{moduleName: true}}

	// old=true,new=true: no enabling transition, no dependency check
	dependencyExtender := moduledependency.NewIExtenderMock(t)

	// the Module CR is found and advertises multiple sources, so a "multiple sources"
	// warning is produced before the storage lookup fails
	moduleCR := newModuleCR(moduleName, []string{"alpha", "beta"}, "")
	handler := newTestHandlerWithObjects(t, storage, manager, dependencyExtender, false, moduleCR)

	review := newModuleConfigAdmissionReview(
		"UPDATE",
		newModuleConfigFull(moduleName, boolPtr(true), "", ""),
		newModuleConfigFull(moduleName, boolPtr(true), "", ""),
	)

	resp := callHandler(t, handler, review)

	assert.True(t, resp.Allowed)
	require.NotEmpty(t, resp.Warnings)
	assert.Contains(t, strings.Join(resp.Warnings, " "), "multiple sources")
}

// TestModuleConfigValidationHandler_ExclusiveGroup verifies the exclusive-group
// conflict check, which is only reachable when the Module CR is found (the
// IsNotFound branch returns before it).
func TestModuleConfigValidationHandler_ExclusiveGroup(t *testing.T) {
	const (
		moduleName = "group-module"
		other      = "other-group-module"
		group      = "networking"
	)

	tests := []struct {
		name        string
		enabledMods map[string]bool
		wantAllowed bool
		wantMessage string
	}{
		{
			name:        "another enabled module in the same exclusive group is rejected",
			enabledMods: map[string]bool{moduleName: true, other: true},
			wantAllowed: false,
			wantMessage: "exclusiveGroup",
		},
		{
			name:        "no other module in the exclusive group is enabled is allowed",
			enabledMods: map[string]bool{moduleName: true},
			wantAllowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeModuleStorage{
				modules: map[string]*moduletypes.Module{
					moduleName: newStorageModule(t, moduleName, "", group),
				},
				exclusive: map[string][]string{group: {moduleName, other}},
			}
			manager := &fakeModuleManager{enabled: tt.enabledMods}

			// old=true,new=true: no enabling transition, the dependency extender
			// must not be consulted.
			dependencyExtender := moduledependency.NewIExtenderMock(t)

			moduleCR := newModuleCR(moduleName, []string{"alpha"}, "")
			handler := newTestHandlerWithObjects(t, storage, manager, dependencyExtender, false, moduleCR)

			review := newModuleConfigAdmissionReview(
				"UPDATE",
				newModuleConfigFull(moduleName, boolPtr(true), "alpha", ""),
				newModuleConfigFull(moduleName, boolPtr(true), "alpha", ""),
			)

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

// TestModuleConfigValidationHandler_Experimental covers the experimental-module
// gate, both via the storage module definition and via the fetched Module CR,
// together with the AllowExperimentalModules setting bypass.
func TestModuleConfigValidationHandler_Experimental(t *testing.T) {
	const moduleName = "experimental-module"

	tests := []struct {
		name                string
		allowExperimental   bool
		allowedExperimental []string
		storageStage        string
		moduleCRStage       string
		expectCheckEnabling bool
		wantAllowed         bool
		wantMessage         string
	}{
		{
			name:         "experimental module (per storage definition) is rejected by default",
			storageStage: moduletypes.ExperimentalModuleStage,
			wantAllowed:  false,
			wantMessage:  "experimental",
		},
		{
			name:                "experimental module (per Module CR) is rejected by default",
			moduleCRStage:       v1alpha1.ExperimentalModuleStage,
			expectCheckEnabling: true,
			wantAllowed:         false,
			wantMessage:         "experimental",
		},
		{
			name:                "experimental module is allowed when allowExperimentalModules is true",
			allowExperimental:   true,
			storageStage:        moduletypes.ExperimentalModuleStage,
			expectCheckEnabling: true,
			wantAllowed:         true,
		},
		{
			name:                "experimental module is allowed when listed in allowedExperimentalModules",
			allowedExperimental: []string{moduleName},
			storageStage:        moduletypes.ExperimentalModuleStage,
			expectCheckEnabling: true,
			wantAllowed:         true,
		},
		{
			name:                "experimental module is rejected when a different module is allowlisted",
			allowedExperimental: []string{"other-experimental-module"},
			storageStage:        moduletypes.ExperimentalModuleStage,
			wantAllowed:         false,
			wantMessage:         "experimental",
		},
		{
			name:                "experimental module is allowed when allowExperimentalModules is true and it is also listed",
			allowExperimental:   true,
			allowedExperimental: []string{moduleName},
			storageStage:        moduletypes.ExperimentalModuleStage,
			expectCheckEnabling: true,
			wantAllowed:         true,
		},
		{
			name:                "experimental module is allowed when allowExperimentalModules is true and a different module is listed",
			allowExperimental:   true,
			allowedExperimental: []string{"other-experimental-module"},
			storageStage:        moduletypes.ExperimentalModuleStage,
			expectCheckEnabling: true,
			wantAllowed:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeModuleStorage{
				modules: map[string]*moduletypes.Module{
					moduleName: newStorageModule(t, moduleName, tt.storageStage, ""),
				},
			}
			manager := &fakeModuleManager{enabled: map[string]bool{}}

			dependencyExtender := moduledependency.NewIExtenderMock(t)
			if tt.expectCheckEnabling {
				dependencyExtender.CheckEnablingMock.Expect(moduleName).Return(nil)
			}

			moduleCR := newModuleCR(moduleName, []string{"alpha"}, tt.moduleCRStage)
			handler := newTestHandlerWithValidator(t, storage, manager, dependencyExtender, tt.allowExperimental, tt.allowedExperimental, configtools.NewValidator(nil, nil), moduleCR)

			// CREATE that enables the module triggers the experimental gate
			review := newModuleConfigAdmissionReview("CREATE", newModuleConfigFull(moduleName, boolPtr(true), "alpha", ""), nil)

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
