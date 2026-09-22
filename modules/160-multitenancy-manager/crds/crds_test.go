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

// Package crds_test validates the multitenancy-manager CRD manifests the way the
// API server does on create: the schemas must be structural, the CEL rules must
// compile, and their estimated cost must fit the per-rule and per-CRD budgets.
//
// The cost check matters because the API server estimates the worst case: a CEL
// rule on an array nested in another array without maxItems/maxLength blows the
// budget, the CRD is refused, and deckhouse keeps retrying ModuleEnsureCRDs
// forever. Nothing short of the API server's own validation catches that.
package crds_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsinternal "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	crdvalidation "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	structuralcel "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/yaml"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
)

// expectedCRDs maps a manifest file to the CRD it must define. Listing them
// explicitly means a renamed or forgotten file fails the test instead of being
// silently skipped.
var expectedCRDs = map[string]struct {
	name string
	kind string
}{
	"clusterprojectrolebindings.yaml":                                    {name: "clusterprojectrolebindings.deckhouse.io", kind: "ClusterProjectRoleBinding"},
	"projectnamespaces.yaml":                                             {name: "projectnamespaces.deckhouse.io", kind: "ProjectNamespace"},
	"projectrolebindings.yaml":                                           {name: "projectrolebindings.deckhouse.io", kind: "ProjectRoleBinding"},
	"projects.yaml":                                                      {name: "projects.deckhouse.io", kind: "Project"},
	"projecttemplate.yaml":                                               {name: "projecttemplates.deckhouse.io", kind: "ProjectTemplate"},
	"multitenancy.deckhouse.io_availableclusterresources.yaml":           {name: "availableclusterresources.multitenancy.deckhouse.io", kind: "AvailableClusterResource"},
	"multitenancy.deckhouse.io_clusterresourcegrantpolicies.yaml":        {name: "clusterresourcegrantpolicies.multitenancy.deckhouse.io", kind: "ClusterResourceGrantPolicy"},
	"multitenancy.deckhouse.io_grantableclusterresourcedefinitions.yaml": {name: "grantableclusterresourcedefinitions.multitenancy.deckhouse.io", kind: "GrantableClusterResourceDefinition"},
	"multitenancy.deckhouse.io_grantableclusterresourcereferences.yaml":  {name: "grantableclusterresourcereferences.multitenancy.deckhouse.io", kind: "GrantableClusterResourceReference"},
}

// stripDocMarkers removes the `x-doc-*` keys the Deckhouse documentation tooling
// reads out of the manifest files. They are not fields of JSONSchemaProps, so
// stripping them keeps strict decoding in place for everything else.
func stripDocMarkers(node any) any {
	switch v := node.(type) {
	case map[string]any:
		for key, value := range v {
			if strings.HasPrefix(key, "x-doc-") {
				delete(v, key)
				continue
			}
			v[key] = stripDocMarkers(value)
		}
		return v
	case []any:
		for i := range v {
			v[i] = stripDocMarkers(v[i])
		}
		return v
	default:
		return node
	}
}

func loadCRD(t *testing.T, file string) *apiextensionsv1.CustomResourceDefinition {
	t.Helper()

	data, err := os.ReadFile(file)
	require.NoError(t, err)

	var tree map[string]any
	require.NoError(t, yaml.Unmarshal(data, &tree), "%s is not valid YAML", file)

	stripped, err := json.Marshal(stripDocMarkers(tree))
	require.NoError(t, err)

	crd := &apiextensionsv1.CustomResourceDefinition{}
	require.NoError(t, yaml.UnmarshalStrict(stripped, crd), "%s is not a valid CustomResourceDefinition", file)
	return crd
}

// TestCRDsPassAPIServerValidation runs every manifest through the same
// validation the API server applies on CRD create. That covers structural
// schema checks, CEL compilation and the estimated CEL cost limits
// (crdvalidation.StaticEstimatedCostLimit per rule and
// crdvalidation.StaticEstimatedCRDCostLimit for the whole CRD).
func TestCRDsPassAPIServerValidation(t *testing.T) {
	files, err := filepath.Glob("*.yaml")
	require.NoError(t, err)

	seen := make(map[string]bool, len(expectedCRDs))

	for _, file := range files {
		base := filepath.Base(file)
		if strings.HasPrefix(base, "doc-ru-") {
			// Russian documentation overlays are read by the docs tooling, not by
			// the API server, and are deliberately partial.
			continue
		}

		want, ok := expectedCRDs[base]
		require.Truef(t, ok, "%s is not listed in expectedCRDs; add it or remove the file", base)
		seen[base] = true

		t.Run(base, func(t *testing.T) {
			crd := loadCRD(t, file)

			assert.Equal(t, want.name, crd.Name)
			assert.Equal(t, want.kind, crd.Spec.Names.Kind)
			assert.Equal(t, "deckhouse", crd.Labels["heritage"])
			assert.Equal(t, "multitenancy-manager", crd.Labels["module"])

			// The API server defaults the v1 object before converting and validating it.
			apiextensionsv1.SetObjectDefaults_CustomResourceDefinition(crd)
			internal := &apiextensionsinternal.CustomResourceDefinition{}
			require.NoError(t, apiextensionsv1.Convert_v1_CustomResourceDefinition_To_apiextensions_CustomResourceDefinition(crd, internal, nil))

			errs := crdvalidation.ValidateCustomResourceDefinition(context.Background(), internal)
			assert.Empty(t, errs.ToAggregate(), "the API server would refuse this CRD")
		})
	}

	for base := range expectedCRDs {
		assert.Truef(t, seen[base], "%s is missing from the crds directory", base)
	}
}

func structuralOf(t *testing.T, crd *apiextensionsv1.CustomResourceDefinition, version string) *structuralschema.Structural {
	t.Helper()

	var v1Schema *apiextensionsv1.JSONSchemaProps
	for i := range crd.Spec.Versions {
		if crd.Spec.Versions[i].Name == version {
			require.NotNil(t, crd.Spec.Versions[i].Schema, "version %s has no schema", version)
			v1Schema = crd.Spec.Versions[i].Schema.OpenAPIV3Schema
		}
	}
	require.NotNil(t, v1Schema, "version %s not found", version)

	internal := &apiextensionsinternal.JSONSchemaProps{}
	require.NoError(t, apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(v1Schema, internal, nil))

	s, err := structuralschema.NewStructural(internal)
	require.NoError(t, err, "schema is not structural")
	return s
}

// validateSpec runs the CEL rules of the CRD against a spec, as the API server
// does on write.
func validateSpec(t *testing.T, s *structuralschema.Structural, spec map[string]any) field.ErrorList {
	t.Helper()

	validator := structuralcel.NewValidator(s, true, celconfig.PerCallLimit)
	require.NotNil(t, validator, "no CEL rules found in the schema")

	errs, _ := validator.Validate(
		context.Background(), field.NewPath(""), s, map[string]any{"spec": spec}, nil, celconfig.RuntimeCELCostBudget,
	)
	return errs
}

// TestGrantableClusterResourceReferenceRules checks that the webhook-facing
// rule rejects a wildcard group, while fieldPaths[].resources accepts one: it
// weighs nothing in path selection and behaves like an omitted field.
func TestGrantableClusterResourceReferenceRules(t *testing.T) {
	s := structuralOf(t, loadCRD(t, "multitenancy.deckhouse.io_grantableclusterresourcereferences.yaml"), "v1alpha1")

	spec := func(apiGroups []any, resources []any) map[string]any {
		fp := map[string]any{"path": "$.spec.x"}
		if resources != nil {
			fp["resources"] = resources
		}
		return map[string]any{
			"grantableClusterResourceName": "priorityclasses",
			"rule": map[string]any{
				"apiGroups":   apiGroups,
				"apiVersions": []any{"*"},
				"resources":   []any{"pods"},
			},
			"fieldPaths": []any{fp},
		}
	}

	tests := []struct {
		name    string
		spec    map[string]any
		wantErr string
	}{
		{name: "explicit group is accepted", spec: spec([]any{"", "batch"}, nil)},
		{name: "wildcard group is rejected", spec: spec([]any{"*"}, nil), wantErr: "apiGroups must list the groups explicitly"},
		{name: "wildcard fieldPaths resources is accepted", spec: spec([]any{""}, []any{"*"})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateSpec(t, s, tt.spec)
			if tt.wantErr == "" {
				assert.Empty(t, errs.ToAggregate(), "spec should be accepted")
				return
			}
			require.NotEmpty(t, errs, "spec should be rejected")
			assert.Contains(t, errs.ToAggregate().Error(), tt.wantErr)
		})
	}
}
