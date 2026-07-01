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
	"errors"
	"testing"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	checkErr       error
	checkCalled    bool
	gotConstraints schedule.Constraints
}

func (f *fakePackageManager) ValidatePackageSettings(_ context.Context, _ string, _ addonutils.Values) (settingscheck.Result, error) {
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
				Type:       "object",
				Properties: props,
				Required:   required,
			},
		}
	}

	t.Run("nil package schemas is a no-op", func(t *testing.T) {
		apv := &v1alpha1.ApplicationPackageVersion{}
		app := newApplication("repo", "pkg", "1.0.0")
		require.NoError(t, validateAppSettings(apv, app))
	})

	t.Run("nil settings schema is a no-op", func(t *testing.T) {
		apv := &v1alpha1.ApplicationPackageVersion{}
		apv.Status.PackageSchemas = &v1alpha1.ApplicationPackageVersionStatusSchemas{}
		app := newApplication("repo", "pkg", "1.0.0")
		require.NoError(t, validateAppSettings(apv, app))
	})

	t.Run("settings satisfying the schema pass", func(t *testing.T) {
		apv := &v1alpha1.ApplicationPackageVersion{}
		apv.Status.PackageSchemas = &v1alpha1.ApplicationPackageVersionStatusSchemas{
			SettingsSchema: objectSchema(map[string]openapi.OpenAPIV3Schema{
				"foo": {Type: "string"},
			}, []string{"foo"}),
		}
		app := newApplication("repo", "pkg", "1.0.0")
		app.Spec.Settings = v1alpha1.MakeMappedFields(map[string]any{"foo": "bar"})

		require.NoError(t, validateAppSettings(apv, app))
	})

	t.Run("settings violating the schema are rejected", func(t *testing.T) {
		apv := &v1alpha1.ApplicationPackageVersion{}
		apv.Status.PackageSchemas = &v1alpha1.ApplicationPackageVersionStatusSchemas{
			SettingsSchema: objectSchema(map[string]openapi.OpenAPIV3Schema{
				"foo": {Type: "string"},
			}, []string{"foo"}),
		}
		app := newApplication("repo", "pkg", "1.0.0")
		// "foo" is required but missing
		app.Spec.Settings = v1alpha1.MakeMappedFields(map[string]any{"other": "value"})

		require.Error(t, validateAppSettings(apv, app))
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

			err := validateAppAgainstApv(context.Background(), cli, manager, app)

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
