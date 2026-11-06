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
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	celgo "github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apiserver/pkg/admission"
	admissioncel "k8s.io/apiserver/pkg/admission/plugin/cel"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/cel/environment"
	"sigs.k8s.io/yaml"
)

// unguardedOnlyMarkedObjects is the matchCondition shipped in ffe8610561. has() tests only its
// last field, so has(object.spec.template.metadata.labels) reads object.spec on the way there.
const unguardedOnlyMarkedObjects = `(object != null && has(object.metadata.labels) && (
('deckhouse.io/system-resource' in object.metadata.labels && object.metadata.labels['deckhouse.io/system-resource'] == 'true')
|| ('heritage' in object.metadata.labels && object.metadata.labels['heritage'] == 'multitenancy-manager')
)) || (object != null && has(object.spec.template.metadata.labels) && (
('deckhouse.io/system-resource' in object.spec.template.metadata.labels && object.spec.template.metadata.labels['deckhouse.io/system-resource'] == 'true')
|| ('heritage' in object.spec.template.metadata.labels && object.spec.template.metadata.labels['heritage'] == 'multitenancy-manager')
)) || (object != null && has(object.spec.jobTemplate.spec.template.metadata.labels) && (
('deckhouse.io/system-resource' in object.spec.jobTemplate.spec.template.metadata.labels && object.spec.jobTemplate.spec.template.metadata.labels['deckhouse.io/system-resource'] == 'true')
|| ('heritage' in object.spec.jobTemplate.spec.template.metadata.labels && object.spec.jobTemplate.spec.template.metadata.labels['heritage'] == 'multitenancy-manager')
)) || (oldObject != null && has(oldObject.metadata.labels) && (
('deckhouse.io/system-resource' in oldObject.metadata.labels && oldObject.metadata.labels['deckhouse.io/system-resource'] == 'true')
|| ('heritage' in oldObject.metadata.labels && oldObject.metadata.labels['heritage'] == 'multitenancy-manager')
)) || (oldObject != null && has(oldObject.spec.template.metadata.labels) && (
('deckhouse.io/system-resource' in oldObject.spec.template.metadata.labels && oldObject.spec.template.metadata.labels['deckhouse.io/system-resource'] == 'true')
|| ('heritage' in oldObject.spec.template.metadata.labels && oldObject.spec.template.metadata.labels['heritage'] == 'multitenancy-manager')
)) || (oldObject != null && has(oldObject.spec.jobTemplate.spec.template.metadata.labels) && (
('deckhouse.io/system-resource' in oldObject.spec.jobTemplate.spec.template.metadata.labels && oldObject.spec.jobTemplate.spec.template.metadata.labels['deckhouse.io/system-resource'] == 'true')
|| ('heritage' in oldObject.spec.jobTemplate.spec.template.metadata.labels && oldObject.spec.jobTemplate.spec.template.metadata.labels['heritage'] == 'multitenancy-manager')
))`

// TestOnlyMarkedObjectsMatchCondition evaluates the shipped only-marked-objects expression with the
// same CEL compiler kube-apiserver uses for admission match conditions. go test -run
// TestOnlyMarkedObjectsMatchCondition runs it without the helm suite.
func TestOnlyMarkedObjectsMatchCondition(t *testing.T) {
	shipped := onlyMarkedObjectsExpression(t)
	if !strings.Contains(shipped, "type(object.spec) == map") {
		t.Fatal("shipped expression has no per-level guard")
	}

	// The unguarded form, on a ConfigMap that has metadata but no spec and no labels.
	// false || <error> must surface as an error; a clean false would mean the outage claim is wrong.
	_, unguardedErr := evalMatchCondition(t, unguardedOnlyMarkedObjects, admission.Create, configMap(nil), nil)
	if unguardedErr == nil {
		t.Fatal("unguarded expression evaluated a ConfigMap without error; expected \"no such key\"")
	}
	if !strings.Contains(unguardedErr.Error(), "no such key") {
		t.Fatalf("unguarded ConfigMap error = %v, want no such key", unguardedErr)
	}
	t.Logf("unguarded ConfigMap: ERROR %s", unguardedErr)

	marked := map[string]any{"deckhouse.io/system-resource": "true"}
	cases := []struct {
		name      string
		operation admission.Operation
		object    map[string]any
		old       map[string]any
		want      bool
	}{
		{name: "ConfigMap without labels", operation: admission.Create, object: configMap(nil), want: false},
		{name: "ConfigMap with the marking", operation: admission.Create, object: configMap(marked), want: true},
		{name: "Pod with the marking", operation: admission.Create, object: pod(marked), want: true},
		{name: "Deployment with marking in the pod template", operation: admission.Create, object: deployment(marked), want: true},
		{name: "Deployment without marking in the pod template", operation: admission.Create, object: deployment(map[string]any{"app": "web"}), want: false},
		{name: "CronJob with marking in the job template", operation: admission.Create, object: cronJob(marked), want: true},
		{name: "UPDATE removing the marking from a Deployment template", operation: admission.Update, object: deployment(map[string]any{"app": "web"}), old: deployment(marked), want: true},
		{name: "DELETE of a marked Deployment", operation: admission.Delete, old: deployment(marked), want: true},
		{name: "DELETE of an unmarked ConfigMap", operation: admission.Delete, old: configMap(nil), want: false},
		{name: "CRD whose spec has no template", operation: admission.Create, object: crdWithoutTemplate(), want: false},
		{name: "CRD whose spec is a string", operation: admission.Create, object: crdStringSpec(), want: false},
		{name: "CRD whose pod-template labels are a string", operation: admission.Create, object: crdTemplateLabels("not-a-map"), want: false},
		{name: "CRD whose pod-template labels are a list", operation: admission.Create, object: crdTemplateLabels([]any{"app"}), want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := evalMatchCondition(t, shipped, tc.operation, tc.object, tc.old)
			if err != nil {
				t.Fatalf("evaluation error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("match = %v, want %v", got, tc.want)
			}
		})
	}
}

func onlyMarkedObjectsExpression(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(thisFile), "..", "templates", "system-resources-edit-validation-webhook.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const marker = "name: only-marked-objects"
	idx := strings.Index(string(raw), marker)
	if idx < 0 {
		t.Fatal("only-marked-objects not found")
	}
	rest := string(raw)[idx:]
	const key = "\n    expression: >-\n"
	start := strings.Index(rest, key)
	if start < 0 {
		t.Fatal("expression block not found")
	}
	body := rest[start+1:] // keep "    expression: >-"
	end := strings.Index(body, "\n  rules:")
	if end < 0 {
		t.Fatal("expression block is not closed by rules")
	}
	var decoded struct {
		Expression string `yaml:"expression"`
	}
	if err := yaml.Unmarshal([]byte(body[:end]), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Expression == "" {
		t.Fatal("decoded expression is empty")
	}
	return decoded.Expression
}

type matchBoolExpression struct{ expression string }

func (e matchBoolExpression) GetExpression() string { return e.expression }
func (e matchBoolExpression) ReturnTypes() []*celgo.Type {
	return []*celgo.Type{celgo.BoolType}
}

// evalMatchCondition compiles expr with the admission CEL environment pinned to the oldest
// Kubernetes this branch supports (candi/version_map.yml) and evaluates it the way a
// ValidatingWebhook matchCondition is evaluated: object and oldObject are dyn maps, null when
// the admission object is absent. A nil object or old map means that variable is null.
func evalMatchCondition(t *testing.T, expr string, operation admission.Operation, object, old map[string]any) (bool, error) {
	t.Helper()
	// 1.33 is the lowest minor in candi/version_map.yml. NewExpressions is what kube-apiserver
	// validates a newly written match condition against.
	compiler, err := admissioncel.NewCompositedCompiler(environment.MustBaseEnvSet(version.MustParseGeneric("1.33.0")))
	if err != nil {
		t.Fatal(err)
	}
	vars := admissioncel.OptionalVariableDeclarations{}
	accessor := matchBoolExpression{expression: expr}
	if result := compiler.CompileCELExpression(accessor, vars, environment.NewExpressions); result.Error != nil {
		t.Fatalf("compile: %v", result.Error)
	}
	evaluator := compiler.CompileCondition([]admissioncel.ExpressionAccessor{accessor}, vars, environment.NewExpressions)

	source := object
	if source == nil {
		source = old
	}
	gvk := gvkOf(source)
	gvr := schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: "objects"}
	var obj, oldObj k8sruntime.Object
	if object != nil {
		u := &unstructured.Unstructured{Object: object}
		u.SetGroupVersionKind(gvk)
		obj = u
	}
	if old != nil {
		u := &unstructured.Unstructured{Object: old}
		u.SetGroupVersionKind(gvk)
		oldObj = u
	}
	attributes := admission.NewAttributesRecord(
		obj, oldObj, gvk, "team-a", "app", gvr, "", operation, nil, false,
		&user.DefaultInfo{Name: "alice@example.com"},
	)
	versioned, err := admission.NewVersionedAttributes(attributes, gvk, nil)
	if err != nil {
		t.Fatal(err)
	}
	request := admissioncel.CreateAdmissionRequest(
		attributes, metav1.GroupVersionResource(gvr), metav1.GroupVersionKind(gvk),
	)
	evaluations, _, err := evaluator.ForInput(
		context.Background(), versioned, request, admissioncel.OptionalVariableBindings{}, nil,
		celconfig.RuntimeCELCostBudgetMatchConditions,
	)
	if err != nil {
		return false, err
	}
	if len(evaluations) != 1 {
		t.Fatalf("evaluations = %d, want 1", len(evaluations))
	}
	evaluation := evaluations[0]
	if evaluation.Error != nil {
		return false, evaluation.Error
	}
	if evaluation.EvalResult == nil || types.IsError(evaluation.EvalResult) {
		return false, evaluation.EvalResult.(error)
	}
	value, ok := evaluation.EvalResult.Value().(bool)
	if !ok {
		t.Fatalf("result type %T (%v), want bool", evaluation.EvalResult.Value(), evaluation.EvalResult.Type())
	}
	return value, nil
}

func gvkOf(object map[string]any) schema.GroupVersionKind {
	gvk := schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
	if object == nil {
		return gvk
	}
	if kind, ok := object["kind"].(string); ok && kind != "" {
		gvk.Kind = kind
	}
	apiVersion, _ := object["apiVersion"].(string)
	if group, version, ok := strings.Cut(apiVersion, "/"); ok {
		gvk.Group = group
		gvk.Version = version
	} else if apiVersion != "" {
		gvk.Version = apiVersion
	}
	return gvk
}

func configMap(labels map[string]any) map[string]any {
	return object("ConfigMap", labels, nil)
}

func pod(labels map[string]any) map[string]any {
	return object("Pod", labels, map[string]any{"containers": []any{map[string]any{"name": "app"}}})
}

func deployment(templateLabels map[string]any) map[string]any {
	return object("Deployment", nil, map[string]any{
		"template": map[string]any{"metadata": map[string]any{"labels": templateLabels}},
	})
}

func cronJob(templateLabels map[string]any) map[string]any {
	return object("CronJob", nil, map[string]any{
		"jobTemplate": map[string]any{
			"spec": map[string]any{
				"template": map[string]any{"metadata": map[string]any{"labels": templateLabels}},
			},
		},
	})
}

func crdWithoutTemplate() map[string]any {
	return object("Widget", nil, map[string]any{"replicas": int64(1)})
}

func crdStringSpec() map[string]any {
	obj := object("Widget", nil, nil)
	obj["spec"] = "scalar"
	return obj
}

func crdTemplateLabels(labels any) map[string]any {
	return object("Widget", nil, map[string]any{
		"template": map[string]any{"metadata": map[string]any{"labels": labels}},
	})
}

func object(kind string, labels, spec map[string]any) map[string]any {
	meta := map[string]any{"name": "app", "namespace": "team-a"}
	if labels != nil {
		meta["labels"] = labels
	}
	obj := map[string]any{
		"apiVersion": "v1",
		"kind":       kind,
		"metadata":   meta,
	}
	if spec != nil {
		obj["spec"] = spec
	}
	return obj
}
