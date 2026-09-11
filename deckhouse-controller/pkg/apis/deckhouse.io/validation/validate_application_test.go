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
	"context"
	"encoding/json"
	"errors"
	"testing"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	kwhmodel "github.com/slok/kubewebhook/v2/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/openapi"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/module-sdk/pkg/settingscheck"
)

// fakePackageManager implements the packageManager interface for tests.
type fakePackageManager struct {
	validateResult settingscheck.Result
	validateErr    error
	validateCalled bool

	checkErr       error
	checkCalled    bool
	gotConstraints schedule.Constraints
}

func (f *fakePackageManager) ValidatePackageSettings(_ context.Context, _ string, _ int, _ addonutils.Values) (settingscheck.Result, error) {
	f.validateCalled = true

	return f.validateResult, f.validateErr
}

func (f *fakePackageManager) CheckConstraints(_ string, constraints schedule.Constraints) error {
	f.checkCalled = true
	f.gotConstraints = constraints
	return f.checkErr
}

func newApplication(repo, pkg, version string) *v1alpha1.Application {
	return &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{Name: "my-app", Namespace: "default"},
		Spec: v1alpha1.ApplicationSpec{
			PackageRepositoryName: repo,
			PackageName:           pkg,
			PackageVersion:        version,
		},
	}
}

func newAPV(name string, draft bool, reqs *v1alpha1.PackageRequirements) *v1alpha1.ApplicationPackageVersion {
	apv := &v1alpha1.ApplicationPackageVersion{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}

	if draft {
		apv.Labels = map[string]string{v1alpha1.ApplicationPackageVersionLabelDraft: "true"}
	}

	if reqs != nil {
		apv.Status.PackageMetadata = &v1alpha1.ApplicationPackageVersionStatusMetadata{
			Requirements: reqs,
		}
	}

	return apv
}

// applicationValidationHandlerSuite exercises manager and APV validation selection.
type applicationValidationHandlerSuite struct {
	suite.Suite
}

// TestApplicationValidationHandler runs Application admission validation scenarios.
func TestApplicationValidationHandler(t *testing.T) {
	suite.Run(t, new(applicationValidationHandlerSuite))
}

// TestSettingsAreValidatedByManagerForCurrentVersion verifies loaded-schema validation.
func (s *applicationValidationHandlerSuite) TestSettingsAreValidatedByManagerForCurrentVersion() {
	const version = "v1.0.0"

	app := newApplication("repo", "pkg", version)
	app.Status.CurrentVersion = &v1alpha1.ApplicationStatusVersion{Version: version}
	manager := &fakePackageManager{
		validateResult: settingscheck.Result{Valid: false, Message: "settings rejected by the loaded schema"},
	}

	response := s.validate(app, manager)

	s.False(response.Allowed)
	s.True(manager.validateCalled)
	s.False(manager.checkCalled)
}

// TestSettingsAreValidatedOnlyByAPVWhileVersionChanges verifies stale manager schemas are skipped.
func (s *applicationValidationHandlerSuite) TestSettingsAreValidatedOnlyByAPVWhileVersionChanges() {
	app := newApplication("repo", "pkg", "v2.0.0")
	app.Status.CurrentVersion = &v1alpha1.ApplicationStatusVersion{Version: "v1.0.0"}
	manager := &fakePackageManager{
		validateResult: settingscheck.Result{Valid: false, Message: "settings rejected by the loaded schema"},
	}

	response := s.validate(app, manager)

	s.True(response.Allowed)
	s.False(manager.validateCalled)
	s.True(manager.checkCalled)
}

// validate submits an Application update to the admission handler.
func (s *applicationValidationHandlerSuite) validate(app *v1alpha1.Application, manager *fakePackageManager) *admissionv1.AdmissionResponse {
	s.T().Helper()

	apvName := v1alpha1.MakeApplicationPackageVersionName(app.Spec.PackageRepositoryName, app.Spec.PackageName, app.Spec.PackageVersion)

	// A real UPDATE always carries the object it replaces, and the handler rejects one
	// that does not: an update to itself is the neutral old object for tests that are
	// not about immutability.
	return s.submit("UPDATE", app, app, newAPV(apvName, false, nil), manager)
}

// submit submits one admission request, letting a test choose the operation, the
// previously stored object and the APV whose settings schema is published.
func (s *applicationValidationHandlerSuite) submit(
	operation string,
	app, oldApp *v1alpha1.Application,
	apv *v1alpha1.ApplicationPackageVersion,
	manager *fakePackageManager,
) *admissionv1.AdmissionResponse {
	s.T().Helper()

	ap := &v1alpha1.ApplicationPackage{ObjectMeta: metav1.ObjectMeta{Name: app.Spec.PackageName}}
	handler := applicationValidationHandler(newFakeClient(s.T(), ap, apv), manager)

	var old interface{}
	if oldApp != nil {
		old = oldApp
	}

	return callHandler(s.T(), handler, newModuleConfigAdmissionReview(operation, app, old))
}

func TestParsePackageDependencyConstraint(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantNil   bool
		wantError bool
	}{
		{name: "empty string yields nil without error", raw: "", wantNil: true},
		{name: "valid constraint parses", raw: ">=1.0.0", wantNil: false},
		{name: "valid bare version parses", raw: "1.2.3", wantNil: false},
		{name: "garbage constraint errors", raw: "abc", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePackageDependencyConstraint(tt.raw)
			if tt.wantError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
			}
		})
	}
}

func TestParsePackageConstraint(t *testing.T) {
	t.Run("nil wrapper yields nil", func(t *testing.T) {
		got, err := parsePackageConstraint(nil)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("empty constraint yields nil", func(t *testing.T) {
		got, err := parsePackageConstraint(&v1alpha1.VersionConstraint{Constraint: ""})
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("valid constraint parses", func(t *testing.T) {
		got, err := parsePackageConstraint(&v1alpha1.VersionConstraint{Constraint: ">=1.26"})
		require.NoError(t, err)
		assert.NotNil(t, got)
	})

	t.Run("invalid constraint errors", func(t *testing.T) {
		_, err := parsePackageConstraint(&v1alpha1.VersionConstraint{Constraint: "abc"})
		require.Error(t, err)
	})
}

func TestValidateApplicationSettings(t *testing.T) {
	objectSchema := func(props map[string]openapi.OpenAPIV3Schema, required []string) *v1alpha1.PackageSchema {
		return &v1alpha1.PackageSchema{
			OpenAPIV3Schema: &openapi.OpenAPIV3Schema{
				Type:       openapi.StringOrArray{"object"},
				Properties: props,
				Required:   required,
			},
		}
	}

	t.Run("nil package schemas is a no-op", func(t *testing.T) {
		apv := &v1alpha1.ApplicationPackageVersion{}
		app := newApplication("repo", "pkg", "1.0.0")
		require.NoError(t, validateAppSettings(apv, app, nil))
	})

	t.Run("nil settings schema is a no-op", func(t *testing.T) {
		apv := &v1alpha1.ApplicationPackageVersion{}
		apv.Status.PackageSchemas = &v1alpha1.PackageVersionStatusSchemas{}
		app := newApplication("repo", "pkg", "1.0.0")
		require.NoError(t, validateAppSettings(apv, app, nil))
	})

	t.Run("settings satisfying the schema pass", func(t *testing.T) {
		apv := &v1alpha1.ApplicationPackageVersion{}
		apv.Status.PackageSchemas = &v1alpha1.PackageVersionStatusSchemas{
			SettingsSchema: objectSchema(map[string]openapi.OpenAPIV3Schema{
				"foo": {Type: openapi.StringOrArray{"string"}},
			}, []string{"foo"}),
		}
		app := newApplication("repo", "pkg", "1.0.0")
		app.Spec.Settings = v1alpha1.MakeMappedFields(map[string]any{"foo": "bar"})

		require.NoError(t, validateAppSettings(apv, app, nil))
	})

	t.Run("settings violating the schema are rejected", func(t *testing.T) {
		apv := &v1alpha1.ApplicationPackageVersion{}
		apv.Status.PackageSchemas = &v1alpha1.PackageVersionStatusSchemas{
			SettingsSchema: objectSchema(map[string]openapi.OpenAPIV3Schema{
				"foo": {Type: openapi.StringOrArray{"string"}},
			}, []string{"foo"}),
		}
		app := newApplication("repo", "pkg", "1.0.0")
		// "foo" is required but missing
		app.Spec.Settings = v1alpha1.MakeMappedFields(map[string]any{"other": "value"})

		require.Error(t, validateAppSettings(apv, app, nil))
	})
}

// TestValidateAppAgainstApv exercises the requirement-parsing branches of
// validateAppAgainstApv: APV lookup, draft guard, and the many ways the module
// dependency requirements can be malformed.
func TestValidateAppAgainstApv(t *testing.T) {
	const (
		repo    = "repo"
		pkg     = "pkg"
		version = "1.0.0"
	)
	apvName := v1alpha1.MakeApplicationPackageVersionName(repo, pkg, version)

	modulesReqs := func(m *v1alpha1.PackageModulesRequirements) *v1alpha1.PackageRequirements {
		return &v1alpha1.PackageRequirements{Modules: m}
	}

	tests := []struct {
		name        string
		apv         *v1alpha1.ApplicationPackageVersion
		checkErr    error
		wantErr     bool
		wantErrText string
		wantCheck   bool
	}{
		{
			name:        "apv not found",
			apv:         nil,
			wantErr:     true,
			wantErrText: "get application package version",
		},
		{
			name:        "draft apv is rejected",
			apv:         newAPV(apvName, true, nil),
			wantErr:     true,
			wantErrText: "is draft",
		},
		{
			name:      "no requirements delegates to the manager",
			apv:       newAPV(apvName, false, nil),
			wantCheck: true,
		},
		{
			name:        "manager rejection is propagated",
			apv:         newAPV(apvName, false, nil),
			checkErr:    errors.New("dependency cycle"),
			wantErr:     true,
			wantErrText: "dependency cycle",
			wantCheck:   true,
		},
		{
			name:        "invalid kubernetes constraint",
			apv:         newAPV(apvName, false, &v1alpha1.PackageRequirements{Kubernetes: &v1alpha1.VersionConstraint{Constraint: "abc"}}),
			wantErr:     true,
			wantErrText: "parse kubernetes requirement",
		},
		{
			name:        "invalid deckhouse constraint",
			apv:         newAPV(apvName, false, &v1alpha1.PackageRequirements{Deckhouse: &v1alpha1.VersionConstraint{Constraint: "abc"}}),
			wantErr:     true,
			wantErrText: "parse deckhouse requirement",
		},
		{
			name: "invalid mandatory module constraint",
			apv: newAPV(apvName, false, modulesReqs(&v1alpha1.PackageModulesRequirements{
				Mandatory: []v1alpha1.PackageModuleDependency{{Name: "mod-a", Constraint: "abc"}},
			})),
			wantErr:     true,
			wantErrText: "parse mandatory module requirement 'mod-a'",
		},
		{
			name: "mandatory module without constraint is allowed",
			apv: newAPV(apvName, false, modulesReqs(&v1alpha1.PackageModulesRequirements{
				Mandatory: []v1alpha1.PackageModuleDependency{{Name: "mod-a"}},
			})),
			wantCheck: true,
		},
		{
			name: "conditional module without constraint is rejected",
			apv: newAPV(apvName, false, modulesReqs(&v1alpha1.PackageModulesRequirements{
				Conditional: []v1alpha1.PackageModuleDependency{{Name: "mod-a"}},
			})),
			wantErr:     true,
			wantErrText: "constraint is required",
		},
		{
			name: "conditional module also listed as mandatory is rejected",
			apv: newAPV(apvName, false, modulesReqs(&v1alpha1.PackageModulesRequirements{
				Mandatory:   []v1alpha1.PackageModuleDependency{{Name: "mod-a", Constraint: ">=1.0.0"}},
				Conditional: []v1alpha1.PackageModuleDependency{{Name: "mod-a", Constraint: ">=1.0.0"}},
			})),
			wantErr:     true,
			wantErrText: "also listed as mandatory",
		},
		{
			name: "anyOf group without name is rejected",
			apv: newAPV(apvName, false, modulesReqs(&v1alpha1.PackageModulesRequirements{
				AnyOf: []v1alpha1.PackageModuleGroup{{Modules: []v1alpha1.PackageModuleDependency{{Name: "mod-a"}}}},
			})),
			wantErr:     true,
			wantErrText: "name is required",
		},
		{
			name: "anyOf duplicate group name is rejected",
			apv: newAPV(apvName, false, modulesReqs(&v1alpha1.PackageModulesRequirements{
				AnyOf: []v1alpha1.PackageModuleGroup{
					{Name: "grp", Modules: []v1alpha1.PackageModuleDependency{{Name: "mod-a"}}},
					{Name: "grp", Modules: []v1alpha1.PackageModuleDependency{{Name: "mod-b"}}},
				},
			})),
			wantErr:     true,
			wantErrText: "duplicate group name",
		},
		{
			name: "anyOf group without members is rejected",
			apv: newAPV(apvName, false, modulesReqs(&v1alpha1.PackageModulesRequirements{
				AnyOf: []v1alpha1.PackageModuleGroup{{Name: "grp"}},
			})),
			wantErr:     true,
			wantErrText: "at least one member is required",
		},
		{
			name: "anyOf member also mandatory is rejected",
			apv: newAPV(apvName, false, modulesReqs(&v1alpha1.PackageModulesRequirements{
				Mandatory: []v1alpha1.PackageModuleDependency{{Name: "mod-a"}},
				AnyOf: []v1alpha1.PackageModuleGroup{
					{Name: "grp", Modules: []v1alpha1.PackageModuleDependency{{Name: "mod-a"}}},
				},
			})),
			wantErr:     true,
			wantErrText: "also listed as mandatory",
		},
		{
			name: "noneOf member also listed in anyOf is rejected",
			apv: newAPV(apvName, false, modulesReqs(&v1alpha1.PackageModulesRequirements{
				AnyOf: []v1alpha1.PackageModuleGroup{
					{Name: "grp-any", Modules: []v1alpha1.PackageModuleDependency{{Name: "mod-a"}}},
				},
				NoneOf: []v1alpha1.PackageModuleGroup{
					{Name: "grp-none", Modules: []v1alpha1.PackageModuleDependency{{Name: "mod-a"}}},
				},
			})),
			wantErr:     true,
			wantErrText: "also listed in anyOf group 'grp-any'",
		},
		{
			name: "fully valid requirements delegate to the manager",
			apv: newAPV(apvName, false, &v1alpha1.PackageRequirements{
				Kubernetes: &v1alpha1.VersionConstraint{Constraint: ">=1.28"},
				Deckhouse:  &v1alpha1.VersionConstraint{Constraint: ">=1.60"},
				Modules: &v1alpha1.PackageModulesRequirements{
					Mandatory:   []v1alpha1.PackageModuleDependency{{Name: "mod-a", Constraint: ">=1.0.0"}},
					Conditional: []v1alpha1.PackageModuleDependency{{Name: "mod-b", Constraint: ">=2.0.0"}},
					AnyOf: []v1alpha1.PackageModuleGroup{
						{Name: "grp-any", Modules: []v1alpha1.PackageModuleDependency{{Name: "mod-c", Constraint: ">=1.0.0"}}},
					},
					NoneOf: []v1alpha1.PackageModuleGroup{
						{Name: "grp-none", Modules: []v1alpha1.PackageModuleDependency{{Name: "mod-d"}}},
					},
				},
			}),
			wantCheck: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objs []client.Object
			if tt.apv != nil {
				objs = append(objs, tt.apv)
			}

			cli := newFakeClient(t, objs...)
			manager := &fakePackageManager{checkErr: tt.checkErr}
			app := newApplication(repo, pkg, version)

			err := validateAppAgainstApv(context.Background(), cli, manager, app, nil)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrText != "" {
					assert.Contains(t, err.Error(), tt.wantErrText)
				}
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.wantCheck, manager.checkCalled)
		})
	}
}

// immutableSettingsAPV publishes a settings schema whose storageClass field carries
// x-deckhouse-immutable, the shape the console and the webhook both read.
func immutableSettingsAPV(app *v1alpha1.Application) *v1alpha1.ApplicationPackageVersion {
	apvName := v1alpha1.MakeApplicationPackageVersionName(app.Spec.PackageRepositoryName, app.Spec.PackageName, app.Spec.PackageVersion)
	apv := newAPV(apvName, false, nil)
	apv.Status.PackageSchemas = &v1alpha1.PackageVersionStatusSchemas{
		SettingsSchema: &v1alpha1.PackageSchema{
			OpenAPIV3Schema: &openapi.OpenAPIV3Schema{
				Type: openapi.StringOrArray{"object"},
				Properties: map[string]openapi.OpenAPIV3Schema{
					"storageClass": {
						Type:       openapi.StringOrArray{"string"},
						XImmutable: true,
					},
					// Both marks on one field: the value is frozen, and the controller
					// resolves it from the project when the manifest leaves it empty.
					"grantedClass": {
						Type:       openapi.StringOrArray{"string"},
						XGrant:     "storageclasses",
						XImmutable: true,
					},
					"replicas": {Type: openapi.StringOrArray{"integer"}},
				},
			},
		},
	}

	return apv
}

// TestImmutableSettingsFieldCannotChangeOnUpdate is the end-to-end check for
// extractOldApplication → validateAppSettings → checkImmutableSettings.
func (s *applicationValidationHandlerSuite) TestImmutableSettingsFieldCannotChangeOnUpdate() {
	tests := []struct {
		name        string
		operation   string
		newSettings map[string]any
		oldSettings map[string]any
		oldApplied  map[string]any
		wantAllowed bool
		wantMessage string
	}{
		{
			name:        "UPDATE changing the immutable field is rejected",
			operation:   "UPDATE",
			newSettings: map[string]any{"storageClass": "slow"},
			oldSettings: map[string]any{"storageClass": "fast"},
			wantMessage: "storageClass",
		},
		{
			name:        "UPDATE keeping the immutable field is allowed",
			operation:   "UPDATE",
			newSettings: map[string]any{"storageClass": "fast"},
			oldSettings: map[string]any{"storageClass": "fast"},
			wantAllowed: true,
		},
		{
			name:        "UPDATE changing another field is allowed",
			operation:   "UPDATE",
			newSettings: map[string]any{"storageClass": "fast", "replicas": float64(3)},
			oldSettings: map[string]any{"storageClass": "fast", "replicas": float64(1)},
			wantAllowed: true,
		},
		{
			name:        "CREATE may set the immutable field",
			operation:   "CREATE",
			newSettings: map[string]any{"storageClass": "fast"},
			wantAllowed: true,
		},
		{
			// The manifest never named the field, so spec.settings alone would read it
			// as "never set" and take any value the update offers.
			name:        "UPDATE overriding a value only the release knows is rejected",
			operation:   "UPDATE",
			newSettings: map[string]any{"storageClass": "slow"},
			oldSettings: map[string]any{},
			oldApplied:  map[string]any{"storageClass": "fast"},
			wantMessage: "storageClass",
		},
		{
			name:        "UPDATE leaving a granted field to the project is allowed",
			operation:   "UPDATE",
			newSettings: map[string]any{"replicas": float64(3)},
			oldSettings: map[string]any{"replicas": float64(1)},
			oldApplied:  map[string]any{"grantedClass": "fast-ssd", "replicas": float64(1)},
			wantAllowed: true,
		},
		{
			name:        "UPDATE overriding the granted field is rejected",
			operation:   "UPDATE",
			newSettings: map[string]any{"grantedClass": "cheap-hdd"},
			oldSettings: map[string]any{},
			oldApplied:  map[string]any{"grantedClass": "fast-ssd"},
			wantMessage: "grantedClass",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			app := newApplication("repo", "pkg", "1.0.0")
			app.Spec.Settings = v1alpha1.MakeMappedFields(tt.newSettings)

			var oldApp *v1alpha1.Application
			if tt.oldSettings != nil {
				oldApp = newApplication("repo", "pkg", "1.0.0")
				oldApp.Spec.Settings = v1alpha1.MakeMappedFields(tt.oldSettings)
			}

			if tt.oldApplied != nil {
				raw, err := json.Marshal(tt.oldApplied)
				s.Require().NoError(err)
				oldApp.Status.LastAppliedConfiguration = runtime.RawExtension{Raw: raw}
			}

			response := s.submit(tt.operation, app, oldApp, immutableSettingsAPV(app), &fakePackageManager{})

			s.Require().Equal(tt.wantAllowed, response.Allowed)
			if tt.wantAllowed {
				return
			}

			s.Require().NotNil(response.Result)
			s.Contains(response.Result.Message, tt.wantMessage)
			// The denial is the only feedback kubectl prints, so it must name the object.
			s.Contains(response.Result.Message, "my-app")
		})
	}
}

// TestExtractOldApplicationRejectsUpdateWithoutOldObject pins the fail-closed direction:
// an UPDATE always carries the object it replaces, and admitting one that does not would
// leave every immutable field unchecked.
func TestExtractOldApplicationRejectsUpdateWithoutOldObject(t *testing.T) {
	_, err := extractOldApplication(&kwhmodel.AdmissionReview{Operation: kwhmodel.OperationUpdate})
	require.Error(t, err)

	oldApp, err := extractOldApplication(&kwhmodel.AdmissionReview{Operation: kwhmodel.OperationCreate})
	require.NoError(t, err)
	assert.Nil(t, oldApp)
}
