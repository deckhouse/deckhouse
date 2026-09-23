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

package v1alpha1_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	apiextensionsinternal "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	schemacel "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	apiservervalidation "k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"sigs.k8s.io/yaml"
)

// loadCRD reads one of the crdCopies. A missing file is a skip, not a failure: the hand-maintained
// copy lives under ee/ and is absent from a CE checkout.
func loadCRD(t *testing.T, path string) *apiextensionsv1.CustomResourceDefinition {
	t.Helper()

	raw, err := os.ReadFile(filepath.Clean(path))
	if os.IsNotExist(err) {
		t.Skipf("CRD %s is absent in this checkout", path)
	}
	if err != nil {
		t.Fatalf("read CRD: %v", err)
	}

	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := yaml.UnmarshalStrict(raw, crd); err != nil {
		t.Fatalf("unmarshal CRD: %v", err)
	}
	return crd
}

// specSchema returns the v1alpha1 spec subschema of a CRD copy.
func specSchema(t *testing.T, crd *apiextensionsv1.CustomResourceDefinition) apiextensionsv1.JSONSchemaProps {
	t.Helper()

	for i := range crd.Spec.Versions {
		if crd.Spec.Versions[i].Name != "v1alpha1" || crd.Spec.Versions[i].Schema == nil {
			continue
		}
		return crd.Spec.Versions[i].Schema.OpenAPIV3Schema.Properties["spec"]
	}
	t.Fatal("no v1alpha1 version with a schema found in CRD")
	return apiextensionsv1.JSONSchemaProps{}
}

// The generated CRD is ported into crds/ by hand, and the hand-maintained copy is the one that
// reaches a cluster, so the two must agree on every constraint. Wording may differ; validation
// may not.
func TestPlacementSchemaMatchesAcrossCRDCopies(t *testing.T) {
	for _, fieldName := range []string{"nodeSelector", "tolerations"} {
		t.Run(fieldName, func(t *testing.T) {
			schemas := map[string]apiextensionsv1.JSONSchemaProps{}

			for copyName, path := range crdCopies {
				schema, ok := specSchema(t, loadCRD(t, path)).Properties[fieldName]
				if !ok {
					t.Fatalf("spec.%s not found in the %s CRD copy", fieldName, copyName)
				}
				stripDescriptions(&schema)
				schemas[copyName] = schema
			}

			assert.Equal(t, schemas["generated"], schemas["hand-maintained"],
				"spec.%s carries different validation in the two CRD copies; regenerate with `make manifests` and port the change into crds/", fieldName)
		})
	}
}

// stripDescriptions clears every description in a schema subtree so that two copies can be
// compared on validation alone.
func stripDescriptions(schema *apiextensionsv1.JSONSchemaProps) {
	if schema == nil {
		return
	}
	schema.Description = ""
	for name, prop := range schema.Properties {
		stripDescriptions(&prop)
		schema.Properties[name] = prop
	}
	if schema.Items != nil {
		stripDescriptions(schema.Items.Schema)
		for i := range schema.Items.JSONSchemas {
			stripDescriptions(&schema.Items.JSONSchemas[i])
		}
	}
	if schema.AdditionalProperties != nil {
		stripDescriptions(schema.AdditionalProperties.Schema)
	}
}

func toleration(fields map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"tolerations": []interface{}{fields}}
}

func tolerations(n int) map[string]interface{} {
	list := make([]interface{}, 0, n)
	for i := range n {
		list = append(list, map[string]interface{}{"key": fmt.Sprintf("k%d", i), "operator": "Exists"})
	}
	return map[string]interface{}{"tolerations": list}
}

func nodeSelector(n int) map[string]interface{} {
	labels := map[string]interface{}{}
	for i := range n {
		labels[fmt.Sprintf("k%d", i)] = "v"
	}
	return map[string]interface{}{"nodeSelector": labels}
}

// Runs sample placement through both validators an apiserver uses: OpenAPI for enum, pattern and
// maxLength, CEL for the cross-field rules a schema alone cannot express.
func TestPlacementValidation(t *testing.T) {
	cases := []struct {
		name    string
		obj     map[string]interface{}
		wantErr bool
	}{
		{
			name: "a dedicated node pool, keyed and tainted",
			obj: map[string]interface{}{
				"nodeSelector": map[string]interface{}{"node-role.deckhouse.io/vcp": ""},
				"tolerations": []interface{}{map[string]interface{}{
					"key": "dedicated.deckhouse.io", "operator": "Equal", "value": "vcp", "effect": "NoSchedule",
				}},
			},
		},
		{
			name: "tolerate everything",
			obj:  toleration(map[string]interface{}{"operator": "Exists"}),
		},
		{
			name:    "misspelled effect",
			obj:     toleration(map[string]interface{}{"key": "a", "operator": "Exists", "effect": "NoSchedul"}),
			wantErr: true,
		},
		{
			name:    "misspelled operator",
			obj:     toleration(map[string]interface{}{"key": "a", "operator": "Exist"}),
			wantErr: true,
		},
		{
			name:    "lowercased effect",
			obj:     toleration(map[string]interface{}{"key": "a", "operator": "Exists", "effect": "noschedule"}),
			wantErr: true,
		},
		{
			name:    "Exists together with a value",
			obj:     toleration(map[string]interface{}{"key": "a", "operator": "Exists", "value": "b"}),
			wantErr: true,
		},
		{
			name:    "empty key without Exists matches nothing",
			obj:     toleration(map[string]interface{}{"operator": "Equal", "value": "b"}),
			wantErr: true,
		},
		{
			name:    "explicitly empty key",
			obj:     toleration(map[string]interface{}{"key": "", "operator": "Equal", "value": "b"}),
			wantErr: true,
		},
		{
			name:    "tolerationSeconds without NoExecute",
			obj:     toleration(map[string]interface{}{"key": "a", "operator": "Exists", "effect": "NoSchedule", "tolerationSeconds": int64(30)}),
			wantErr: true,
		},
		{
			name: "tolerationSeconds with NoExecute",
			obj:  toleration(map[string]interface{}{"key": "a", "operator": "Exists", "effect": "NoExecute", "tolerationSeconds": int64(30)}),
		},
		{
			name:    "key is not a label key",
			obj:     toleration(map[string]interface{}{"key": "bad key!", "operator": "Exists"}),
			wantErr: true,
		},
		{
			name:    "value is not a label value",
			obj:     toleration(map[string]interface{}{"key": "a", "operator": "Equal", "value": "bad value!"}),
			wantErr: true,
		},
		{
			name:    "nodeSelector key is not a label key",
			obj:     map[string]interface{}{"nodeSelector": map[string]interface{}{"not a key": "v"}},
			wantErr: true,
		},
		{
			name:    "nodeSelector value is not a label value",
			obj:     map[string]interface{}{"nodeSelector": map[string]interface{}{"kubernetes.io/hostname": "not a value"}},
			wantErr: true,
		},

		// The bounds below are what the apiserver sizes the CEL rules against: it estimates a
		// rule's worst-case cost from maxLength, maxItems and maxProperties, and without them
		// assumes a whole 3 MB request, which puts the rules over budget and makes the CRD itself
		// unappliable.
		{
			name: "tolerations at the limit",
			obj:  tolerations(64),
		},
		{
			name:    "one toleration over the limit",
			obj:     tolerations(65),
			wantErr: true,
		},
		{
			name:    "key longer than a label key",
			obj:     toleration(map[string]interface{}{"key": strings.Repeat("a", 317), "operator": "Exists"}),
			wantErr: true,
		},
		{
			name:    "value longer than a label value",
			obj:     toleration(map[string]interface{}{"key": "a", "operator": "Equal", "value": strings.Repeat("b", 64)}),
			wantErr: true,
		},
		{
			name: "nodeSelector at the limit",
			obj:  nodeSelector(64),
		},
		{
			name:    "one nodeSelector entry over the limit",
			obj:     nodeSelector(65),
			wantErr: true,
		},
		{
			name:    "nodeSelector value longer than a label value",
			obj:     map[string]interface{}{"nodeSelector": map[string]interface{}{"a": strings.Repeat("b", 64)}},
			wantErr: true,
		},
	}

	for copyName, path := range crdCopies {
		t.Run(copyName, func(t *testing.T) {
			spec := specSchema(t, loadCRD(t, path))
			// The samples carry placement only; without this every result is masked by
			// "Required value" for the rest of spec.
			spec.Required = nil

			internalSpec := &apiextensionsinternal.JSONSchemaProps{}
			if err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(&spec, internalSpec, nil); err != nil {
				t.Fatalf("convert spec schema: %v", err)
			}

			openAPIValidator, _, err := apiservervalidation.NewSchemaValidator(internalSpec)
			if err != nil {
				t.Fatalf("build OpenAPI validator: %v", err)
			}

			structural, err := structuralschema.NewStructural(internalSpec)
			if err != nil {
				t.Fatalf("build structural schema: %v", err)
			}
			celValidator := schemacel.NewValidator(structural, true, celconfig.PerCallLimit)
			if celValidator == nil {
				t.Fatal("no CEL validator built from spec")
			}

			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					errs := apiservervalidation.ValidateCustomResource(field.NewPath("spec"), tc.obj, openAPIValidator)
					celErrs, _ := celValidator.Validate(
						context.Background(),
						field.NewPath("spec"),
						structural,
						tc.obj,
						nil,
						celconfig.RuntimeCELCostBudget,
					)
					errs = append(errs, celErrs...)

					if tc.wantErr && len(errs) == 0 {
						t.Fatal("expected rejection, got none")
					}
					if !tc.wantErr && len(errs) > 0 {
						t.Fatalf("expected acceptance, got %v", errs)
					}
				})
			}
		})
	}
}
