/*
Copyright 2025 Flant JSC

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

package validation_test

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/validation"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

// moduleManager implements deckhouseReleaseModuleManager interface for testing
type moduleManager struct {
	enabledModules []string
}

func (m *moduleManager) GetEnabledModuleNames() []string {
	return m.enabledModules
}

// Helper functions for creating test objects
// nolint:unparam
func createDeckhouseRelease(name string, approved bool, requirements map[string]string) *v1alpha1.DeckhouseRelease {
	dr := &v1alpha1.DeckhouseRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Approved: approved,
		Spec: v1alpha1.DeckhouseReleaseSpec{
			Version: "v1.60.0",
		},
	}

	if requirements != nil {
		dr.Spec.Requirements = requirements
	}

	return dr
}

// nolint:unparam
func createClusterConfigSecret(kubernetesVersion string) *corev1.Secret {
	clusterConfig := fmt.Sprintf(`kubernetesVersion: "%s"`, kubernetesVersion)

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "d8-cluster-configuration",
			Namespace: "kube-system",
		},
		Data: map[string][]byte{
			"cluster-configuration.yaml": []byte(clusterConfig),
		},
	}
}

func createModuleSource(name string, modules []string) *v1alpha1.ModuleSource {
	availableModules := make([]v1alpha1.AvailableModule, len(modules))
	for i, module := range modules {
		availableModules[i] = v1alpha1.AvailableModule{Name: module}
	}

	return &v1alpha1.ModuleSource{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: v1alpha1.ModuleSourceStatus{
			AvailableModules: availableModules,
		},
	}
}

func createModuleConfig(name string) *v1alpha1.ModuleConfig {
	return &v1alpha1.ModuleConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.ModuleConfigSpec{
			Enabled: &[]bool{true}[0],
		},
	}
}

func createModule(name string) *v1alpha1.Module {
	return &v1alpha1.Module{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: v1alpha1.ModuleStatus{
			Conditions: []v1alpha1.ModuleCondition{
				{
					Type:   v1alpha1.ModuleConditionEnabledByModuleManager,
					Status: corev1.ConditionTrue,
				},
				{
					Type:   v1alpha1.ModuleConditionEnabledByModuleConfig,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}

func createDisabledModule(name string) *v1alpha1.Module {
	return &v1alpha1.Module{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: v1alpha1.ModuleStatus{
			Conditions: []v1alpha1.ModuleCondition{
				{
					Type:   v1alpha1.ModuleConditionEnabledByModuleManager,
					Status: corev1.ConditionFalse,
				},
				{
					Type:   v1alpha1.ModuleConditionEnabledByModuleConfig,
					Status: corev1.ConditionFalse,
				},
			},
		},
	}
}

func createDisabledModuleConfig(name string) *v1alpha1.ModuleConfig {
	return &v1alpha1.ModuleConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.ModuleConfigSpec{
			Enabled: &[]bool{false}[0],
		},
	}
}

func createAdmissionReview(operation string, obj, oldObj interface{}) *admissionv1.AdmissionReview {
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

// TestDeckhouseReleaseValidationHandler tests the main validation logic with maximum coverage
func TestDeckhouseReleaseValidationHandler(t *testing.T) {
	tests := []struct {
		name           string
		enabledModules []string
		kubernetesObjs []client.Object
		operation      string
		release        *v1alpha1.DeckhouseRelease
		oldRelease     *v1alpha1.DeckhouseRelease
		wantAllowed    bool
		wantMessage    string
		description    string
	}{
		{
			name:           "allow unapproved release creation",
			enabledModules: []string{"module1", "module2"},
			kubernetesObjs: []client.Object{
				createClusterConfigSecret("1.28.0"),
			},
			operation:   "CREATE",
			release:     createDeckhouseRelease("test-release", false, nil),
			wantAllowed: true,
			description: "Unapproved DeckhouseReleases should always be allowed",
		},
		{
			name:           "allow approved release without requirements",
			enabledModules: []string{"module1", "module2"},
			kubernetesObjs: []client.Object{
				createClusterConfigSecret("1.28.0"),
			},
			operation:   "CREATE",
			release:     createDeckhouseRelease("test-release", true, nil),
			wantAllowed: true,
			description: "Approved DeckhouseReleases without requirements should be allowed",
		},
		{
			name:           "allow update when old release was approved",
			enabledModules: []string{"module1"},
			kubernetesObjs: []client.Object{
				createClusterConfigSecret("1.28.0"),
			},
			operation:   "UPDATE",
			release:     createDeckhouseRelease("test-release", true, nil),
			oldRelease:  createDeckhouseRelease("test-release", true, nil),
			wantAllowed: true,
			description: "Updates to already approved DeckhouseReleases should be allowed",
		},
		{
			name:           "reject approved release with migrated modules not found",
			enabledModules: []string{"module1", "module2", "non-existent-module"},
			kubernetesObjs: []client.Object{
				createClusterConfigSecret("1.28.0"),
				createModuleConfig("non-existent-module"),
				createModule("non-existent-module"),
			},
			operation: "CREATE",
			release: createDeckhouseRelease("test-release", true, map[string]string{
				"migratedModules": "non-existent-module",
			}),
			wantAllowed: false,
			wantMessage: "requirements not met",
			description: "Approved releases with migrated modules not found should be rejected",
		},
		{
			name:           "allow approved release with migrated modules found",
			enabledModules: []string{"module1", "module2", "migrated-module1", "migrated-module2"},
			kubernetesObjs: []client.Object{
				createClusterConfigSecret("1.28.0"),
				createModuleConfig("migrated-module1"),
				createModuleConfig("migrated-module2"),
				createModuleSource("test-source", []string{"migrated-module1", "migrated-module2"}),
			},
			operation: "CREATE",
			release: createDeckhouseRelease("test-release", true, map[string]string{
				"migratedModules": "migrated-module1, migrated-module2",
			}),
			wantAllowed: true,
			description: "Approved releases with migrated modules found should be allowed",
		},
		{
			name:           "allow empty migrated modules",
			enabledModules: []string{"module1"},
			kubernetesObjs: []client.Object{
				createClusterConfigSecret("1.28.0"),
			},
			operation: "CREATE",
			release: createDeckhouseRelease("test-release", true, map[string]string{
				"migratedModules": "",
			}),
			wantAllowed: true,
			description: "Approved releases with empty migrated modules should be allowed",
		},
		{
			name:           "allow whitespace-only migrated modules",
			enabledModules: []string{"module1"},
			kubernetesObjs: []client.Object{
				createClusterConfigSecret("1.28.0"),
			},
			operation: "CREATE",
			release: createDeckhouseRelease("test-release", true, map[string]string{
				"migratedModules": "   ,  ,   ",
			}),
			wantAllowed: true,
			description: "Approved releases with whitespace-only migrated modules should be allowed",
		},
		{
			name:           "allow when migrated module is disabled in ModuleConfig",
			enabledModules: []string{"module1"},
			kubernetesObjs: []client.Object{
				createClusterConfigSecret("1.28.0"),
				createDisabledModuleConfig("disabled-module"),
			},
			operation: "CREATE",
			release: createDeckhouseRelease("test-release", true, map[string]string{
				"migratedModules": "disabled-module",
			}),
			wantAllowed: true,
			wantMessage: "",
			description: "Releases with migrated modules that are disabled in ModuleConfig should be allowed (ModuleConfig presence is sufficient)",
		},
		{
			name:           "allow when one migrated module is enabled and another is disabled (both present in ModuleConfig)",
			enabledModules: []string{"enabled-module"},
			kubernetesObjs: []client.Object{
				createClusterConfigSecret("1.28.0"),
				createModuleConfig("enabled-module"),
				createDisabledModuleConfig("disabled-module"),
				createModule("enabled-module"),
				createDisabledModule("disabled-module"),
			},
			operation: "CREATE",
			release: createDeckhouseRelease("test-release", true, map[string]string{
				"migratedModules": "enabled-module, disabled-module",
			}),
			wantAllowed: false,
			wantMessage: "requirements not met",
			description: "Releases with mixed enabled/disabled migrated modules should be rejected (enabled module not found in ModuleSource)",
		},
		{
			name:           "reject when migrated module is not found in ModuleSource",
			enabledModules: []string{"cert-manager", "prometheus", "dashboard"},
			kubernetesObjs: []client.Object{
				createClusterConfigSecret("1.28.0"),
				createModuleConfig("cert-manager"),
				createModule("cert-manager"),
				createModuleConfig("prometheus"),
				createModule("prometheus"),
				createModuleSource("test-source", []string{"cert-manager"}),
			},
			operation: "CREATE",
			release: createDeckhouseRelease("test-release", true, map[string]string{
				"migratedModules": "cert-manager, prometheus, non-enabled-module",
			}),
			wantAllowed: false,
			wantMessage: "requirements not met",
			description: "Releases with migrated modules that are not found in ModuleSource and not disabled in MC should be rejected",
		},
		{
			name:           "allow when ModuleConfig exists disabled and not in source",
			enabledModules: []string{"module-x"},
			kubernetesObjs: []client.Object{
				createClusterConfigSecret("1.28.0"),
				createDisabledModuleConfig("module-x"),
				createModuleSource("src", []string{}),
			},
			operation:   "CREATE",
			release:     createDeckhouseRelease("test-release", true, map[string]string{"migratedModules": "module-x"}),
			wantAllowed: true,
			description: "ModuleConfig disabled bypasses ModuleSource",
		},
		{
			name:           "reject when ModuleConfig exists enabled and not in source",
			enabledModules: []string{"module-y"},
			kubernetesObjs: []client.Object{
				createClusterConfigSecret("1.28.0"),
				createModuleConfig("module-y"),
				createModule("module-y"),
				createModuleSource("src", []string{}),
			},
			operation:   "CREATE",
			release:     createDeckhouseRelease("test-release", true, map[string]string{"migratedModules": "module-y"}),
			wantAllowed: false,
			wantMessage: "requirements not met",
			description: "Enabled ModuleConfig requires ModuleSource",
		},
		{
			name:           "reject when ModuleConfig exists enabled and exists in ModuleSource",
			enabledModules: []string{"module-z"},
			kubernetesObjs: []client.Object{
				createClusterConfigSecret("1.28.0"),
				createModuleConfig("module-z"),
				createModuleSource("src", []string{"module-z"}),
			},
			operation:   "CREATE",
			release:     createDeckhouseRelease("test-release", true, map[string]string{"migratedModules": "module-z"}),
			wantAllowed: true,
			description: "Enabled ModuleConfig with presence in ModuleSource",
		},
		{
			name:           "allow when no in ModuleConfig and exists in ModuleSource",
			enabledModules: []string{"module-a1"},
			kubernetesObjs: []client.Object{
				createClusterConfigSecret("1.28.0"),
				createModuleSource("src", []string{"module-a1"}),
			},
			operation:   "CREATE",
			release:     createDeckhouseRelease("test-release", true, map[string]string{"migratedModules": "module-a1"}),
			wantAllowed: true,
			description: "Absent ModuleConfig falls back to ModuleSource and passes",
		},
		{
			name:           "allow when no in ModuleConfig and not in ModuleSource",
			enabledModules: []string{"module-a2"},
			kubernetesObjs: []client.Object{
				createClusterConfigSecret("1.28.0"),
				createModuleSource("src", []string{}),
			},
			operation:   "CREATE",
			release:     createDeckhouseRelease("test-release", true, map[string]string{"migratedModules": "module-a2"}),
			wantAllowed: true,
			wantMessage: "",
			description: "Absent ModuleConfig and absence in ModuleSource allows",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create scheme and fake client
			scheme := runtime.NewScheme()
			require.NoError(t, v1alpha1.AddToScheme(scheme))
			require.NoError(t, corev1.AddToScheme(scheme))

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.kubernetesObjs...).
				Build()

			// Create dependencies
			modManager := &moduleManager{enabledModules: tt.enabledModules}
			metricStorage := metricstorage.NewMetricStorage(metricstorage.WithNewRegistry(), metricstorage.WithLogger(log.NewNop()))

			// Create extenders stack
			logger := log.NewNop()
			edition := &edition.Edition{}
			isHA := func() (bool, error) { return false, nil }
			exts := extenders.NewExtendersStack(edition, isHA, logger)

			// Create the validation handler
			handler := validation.DeckhouseReleaseValidationHandler(
				fakeClient,
				metricStorage,
				modManager,
				exts,
			)

			// Create admission review
			admissionReview := createAdmissionReview(tt.operation, tt.release, tt.oldRelease)

			// Marshal to JSON
			body, err := json.Marshal(admissionReview)
			require.NoError(t, err)

			// Create HTTP request
			req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			recorder := httptest.NewRecorder()

			// Call the handler
			handler.ServeHTTP(recorder, req)

			// Check response
			assert.Equal(t, http.StatusOK, recorder.Code)

			var response admissionv1.AdmissionReview
			err = json.Unmarshal(recorder.Body.Bytes(), &response)
			require.NoError(t, err)
			require.NotNil(t, response.Response)

			// Verify admission result
			if tt.wantAllowed {
				assert.True(t, response.Response.Allowed, "Expected validation to pass: %s", tt.description)
			} else {
				assert.False(t, response.Response.Allowed, "Expected validation to fail: %s", tt.description)
				if tt.wantMessage != "" {
					assert.Contains(t, response.Response.Result.Message, tt.wantMessage)
				}
			}

			t.Logf("✓ %s", tt.description)
		})
	}
}

// TestDeckhouseReleaseValidation_RequirementsCoverage tests comprehensive requirements coverage
func TestDeckhouseReleaseValidation_RequirementsCoverage(t *testing.T) {
	tests := []struct {
		name           string
		release        *v1alpha1.DeckhouseRelease
		enabledModules []string
		kubernetesObjs []client.Object
		wantAllowed    bool
		description    string
	}{
		{
			name: "test deckhouse version requirement",
			release: createDeckhouseRelease("test-release", true, map[string]string{
				"deckhouse": ">=1.60.0",
			}),
			enabledModules: []string{"module1"},
			kubernetesObjs: []client.Object{
				createClusterConfigSecret("1.28.0"),
			},
			wantAllowed: true,
			description: "DeckhouseRelease with valid deckhouse version should be allowed",
		},
		{
			name: "test complex migrated modules",
			release: createDeckhouseRelease("test-release", true, map[string]string{
				"migratedModules": "module-a, module-b , module-c",
			}),
			enabledModules: []string{"module-a", "module-b", "module-c"},
			kubernetesObjs: []client.Object{
				createClusterConfigSecret("1.28.0"),
				createModuleConfig("module-a"),
				createModuleConfig("module-b"),
				createModuleConfig("module-c"),
				createModuleSource("test-source", []string{"module-a", "module-b", "module-c"}),
			},
			wantAllowed: true,
			description: "DeckhouseRelease with complex migratedModules should be allowed when all modules exist",
		},
		{
			name: "test partial migrated modules availability",
			release: createDeckhouseRelease("test-release", true, map[string]string{
				"migratedModules": "available-module, missing-module",
			}),
			enabledModules: []string{"module1"},
			kubernetesObjs: []client.Object{
				createClusterConfigSecret("1.28.0"),
				createModuleConfig("available-module"),
				createModule("available-module"),
			},
			wantAllowed: false,
			description: "DeckhouseRelease with partially available migratedModules should be rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create scheme and fake client
			scheme := runtime.NewScheme()
			require.NoError(t, v1alpha1.AddToScheme(scheme))
			require.NoError(t, corev1.AddToScheme(scheme))

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.kubernetesObjs...).
				Build()

			// Create dependencies
			modManager := &moduleManager{enabledModules: tt.enabledModules}
			metricStorage := metricstorage.NewMetricStorage(metricstorage.WithNewRegistry(), metricstorage.WithLogger(log.NewNop()))

			// Create extenders stack
			logger := log.NewNop()
			edition := &edition.Edition{}
			isHA := func() (bool, error) { return false, nil }
			exts := extenders.NewExtendersStack(edition, isHA, logger)

			// Create the validation handler
			handler := validation.DeckhouseReleaseValidationHandler(
				fakeClient,
				metricStorage,
				modManager,
				exts,
			)

			// Create admission review
			admissionReview := createAdmissionReview("CREATE", tt.release, nil)

			// Marshal to JSON
			body, err := json.Marshal(admissionReview)
			require.NoError(t, err)

			// Create HTTP request
			req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			recorder := httptest.NewRecorder()

			// Call the handler
			handler.ServeHTTP(recorder, req)

			// Check response
			assert.Equal(t, http.StatusOK, recorder.Code)

			var response admissionv1.AdmissionReview
			err = json.Unmarshal(recorder.Body.Bytes(), &response)
			require.NoError(t, err)
			require.NotNil(t, response.Response)

			// Verify admission result
			if tt.wantAllowed {
				assert.True(t, response.Response.Allowed, "Expected validation to pass: %s", tt.description)
			} else {
				assert.False(t, response.Response.Allowed, "Expected validation to fail: %s", tt.description)
				assert.NotEmpty(t, response.Response.Result.Message, "Failed validation should have a message")
			}

			t.Logf("✓ %s", tt.description)
		})
	}
}
