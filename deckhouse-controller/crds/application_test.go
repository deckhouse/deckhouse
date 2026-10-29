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

package crds

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsinternal "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	crdvalidation "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	structuralcel "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	"k8s.io/apimachinery/pkg/util/validation/field"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
)

func applicationCRD(t *testing.T) *apiextensionsv1.CustomResourceDefinition {
	t.Helper()

	list, err := List()
	require.NoError(t, err)

	for i := range list {
		if list[i].Name == "applications.deckhouse.io" {
			return &list[i]
		}
	}

	t.Fatal("the applications.deckhouse.io CRD is not in the directory")

	return nil
}

// TestApplicationCRDPassesAPIServerValidation runs the manifest through the
// validation the API server applies on CRD create: the schema must be
// structural, the CEL rules must compile, and their estimated cost must fit the
// per-rule and per-CRD budgets. The cost is estimated for the worst case, so a
// rule inside lists without maxItems can blow it; nothing short of the API
// server's own validation catches that.
func TestApplicationCRDPassesAPIServerValidation(t *testing.T) {
	crd := applicationCRD(t)

	// The API server defaults the v1 object before converting and validating it.
	apiextensionsv1.SetObjectDefaults_CustomResourceDefinition(crd)
	internal := &apiextensionsinternal.CustomResourceDefinition{}
	require.NoError(t, apiextensionsv1.Convert_v1_CustomResourceDefinition_To_apiextensions_CustomResourceDefinition(crd, internal, nil))

	errs := crdvalidation.ValidateCustomResourceDefinition(context.Background(), internal)
	assert.Empty(t, errs.ToAggregate(), "the API server would refuse this CRD")
}

func applicationSchema(t *testing.T) *structuralschema.Structural {
	t.Helper()

	crd := applicationCRD(t)
	require.Len(t, crd.Spec.Versions, 1)
	require.NotNil(t, crd.Spec.Versions[0].Schema)

	internal := &apiextensionsinternal.JSONSchemaProps{}
	require.NoError(t, apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(crd.Spec.Versions[0].Schema.OpenAPIV3Schema, internal, nil))

	s, err := structuralschema.NewStructural(internal)
	require.NoError(t, err, "schema is not structural")

	return s
}

// TestApplicationResourceRequestsRules runs the CEL rules of spec.resourceRequests
// as the API server does on write: each kind takes only the fields it has.
func TestApplicationResourceRequestsRules(t *testing.T) {
	s := applicationSchema(t)

	validator := structuralcel.NewValidator(s, true, celconfig.PerCallLimit)
	require.NotNil(t, validator, "no CEL rules found in the schema")

	validate := func(request map[string]any) field.ErrorList {
		object := map[string]any{
			"spec": map[string]any{
				"packageName":      "postgres",
				"resourceRequests": []any{request},
			},
		}

		errs, _ := validator.Validate(context.Background(), field.NewPath(""), s, object, nil, celconfig.RuntimeCELCostBudget)

		return errs
	}

	templates := []any{
		map[string]any{"name": "data", "storage": "100Gi"},
		map[string]any{"name": "wal", "storage": "20Gi"},
	}

	tests := []struct {
		name    string
		request map[string]any
		wantErr string
	}{
		{
			name:    "volume claim templates of a StatefulSet",
			request: map[string]any{"kind": "StatefulSet", "name": "postgres", "replicas": int64(3), "volumeClaimTemplates": templates},
		},
		{
			name: "storage in bytes",
			request: map[string]any{
				"kind":                 "StatefulSet",
				"name":                 "postgres",
				"volumeClaimTemplates": []any{map[string]any{"name": "data", "storage": int64(1073741824)}},
			},
		},
		{
			name:    "volume claim templates of a Deployment",
			request: map[string]any{"kind": "Deployment", "name": "api", "volumeClaimTemplates": templates},
			wantErr: "volumeClaimTemplates can only be set for StatefulSet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validate(tt.request)
			if tt.wantErr == "" {
				assert.Empty(t, errs.ToAggregate(), "the request should be accepted")
				return
			}

			require.NotEmpty(t, errs, "the request should be rejected")
			assert.Contains(t, errs.ToAggregate().Error(), tt.wantErr)
		})
	}
}
