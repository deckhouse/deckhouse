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

package template_tests

import (
	"context"
	"os"
	"testing"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	kjson "k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/validation/field"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"sigs.k8s.io/yaml"
)

// An enabled Prometheus profile without the cpu key in both maps would render an empty MetricResource
// threshold, and upstream rejects the whole policy at startup, so the CRD refuses such objects.
// A disabled strategy is not rendered, so the rule does not apply to it.
func TestLowNodeUtilizationPrometheusRequiresCPUThresholds(t *testing.T) {
	raw, err := os.ReadFile("../crds/deschedulers.yaml")
	if err != nil {
		t.Fatal(err)
	}
	crd := apiextv1.CustomResourceDefinition{}
	if err := yaml.Unmarshal(raw, &crd); err != nil {
		t.Fatal(err)
	}
	var schema *apiextv1.JSONSchemaProps
	for _, v := range crd.Spec.Versions {
		if v.Name == "v1alpha2" {
			schema = v.Schema.OpenAPIV3Schema
		}
	}
	internal := &apiextensions.JSONSchemaProps{}
	if err := apiextv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(schema, internal, nil); err != nil {
		t.Fatal(err)
	}
	structural, err := structuralschema.NewStructural(internal)
	if err != nil {
		t.Fatal(err)
	}
	validator := cel.NewValidator(structural, true, celconfig.PerCallLimit)

	for lnu, valid := range map[string]bool{
		`{"enabled":true,"metricsSource":"Prometheus","thresholds":{"cpu":5},"targetThresholds":{"cpu":5}}`:        true,
		`{"enabled":true,"metricsSource":"Prometheus","thresholds":{"cpu":5},"targetThresholds":{"memory":5}}`:     false,
		`{"enabled":true,"metricsSource":"Prometheus","targetThresholds":{"cpu":5}}`:                               false,
		`{"enabled":true,"metricsSource":"Default","thresholds":{"memory":5},"targetThresholds":{"memory":5}}`:     true,
		`{"enabled":true,"thresholds":{"memory":5},"targetThresholds":{"memory":5}}`:                               true,
		`{"enabled":false,"metricsSource":"Prometheus","thresholds":{"memory":5},"targetThresholds":{"memory":5}}`: true,
		`{"metricsSource":"Prometheus","thresholds":{"memory":5},"targetThresholds":{"memory":5}}`:                 true,
	} {
		obj := map[string]interface{}{}
		doc := `{"apiVersion":"deckhouse.io/v1alpha2","kind":"Descheduler","metadata":{"name":"x"},"spec":{"strategies":{"lowNodeUtilization":` + lnu + `}}}`
		if err := kjson.Unmarshal([]byte(doc), &obj); err != nil {
			t.Fatal(err)
		}
		errs, _ := validator.Validate(context.TODO(), field.NewPath("root"), structural, obj, nil, celconfig.RuntimeCELCostBudget)
		if (len(errs) == 0) != valid {
			t.Errorf("lowNodeUtilization %s: want valid=%v, got %v", lnu, valid, errs)
		}
	}
}
