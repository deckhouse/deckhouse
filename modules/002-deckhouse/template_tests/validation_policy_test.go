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
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"gopkg.in/yaml.v3"
)

// This test covers the label-objects.deckhouse.io ValidatingAdmissionPolicy end
// to end: every matchCondition and both validation outcomes. It compiles the
// expressions straight out of the chart template and evaluates them the way the
// API server does - matchConditions first (any false one skips the policy
// entirely, an error rejects the request because failurePolicy is Fail), then
// the audit annotations, then the validations.
//
// The bindings are out of scope here: the test assumes a request the binding
// already matched, that is an object labeled `heritage: deckhouse` without the
// maintenance.deckhouse.io/no-resource-reconciliation label.

const restartedAtAnnotation = "kubectl.kubernetes.io/restartedAt"

type object = map[string]any

func TestLabelObjectsPolicy(t *testing.T) {
	policy := loadPolicy(t, "templates/validation.yaml", "exclude-restartedAt")

	tests := []struct {
		name    string
		request object
		// oldObject is nil for CREATE, object is nil for DELETE.
		oldObject any
		object    any
		want      policyDecision
		// detail is the matchCondition that skipped the policy, or a substring of
		// the denial message.
		detail string
	}{
		// -- exclude-groups ---------------------------------------------------
		{
			name:      "a node may change protected objects",
			request:   updateRequest("Deployment", "system:node:worker-0", "system:nodes"),
			oldObject: deployment(nil),
			object:    updated(func(o object) { container(o)["image"] = "example.com/injected:latest" }),
			want:      skipped,
			detail:    "exclude-groups",
		},
		{
			name:      "any kube-system service account may change protected objects",
			request:   updateRequest("Deployment", "system:serviceaccount:kube-system:some-controller", "system:serviceaccounts:kube-system"),
			oldObject: deployment(nil),
			object:    updated(func(o object) { container(o)["image"] = "example.com/injected:latest" }),
			want:      skipped,
			detail:    "exclude-groups",
		},
		{
			name:      "any d8-system service account may change protected objects",
			request:   updateRequest("Deployment", "system:serviceaccount:d8-system:deckhouse", "system:serviceaccounts:d8-system"),
			oldObject: deployment(nil),
			object:    updated(func(o object) { container(o)["image"] = "example.com/injected:latest" }),
			want:      skipped,
			detail:    "exclude-groups",
		},

		// -- exclude-users ----------------------------------------------------
		{
			name:      "dhctl may change protected objects",
			request:   updateRequest("Deployment", "dhctl"),
			oldObject: deployment(nil),
			object:    updated(func(o object) { container(o)["image"] = "example.com/injected:latest" }),
			want:      skipped,
			detail:    "exclude-users",
		},
		{
			name:      "observability may change protected objects",
			request:   updateRequest("Deployment", "observability"),
			oldObject: deployment(nil),
			object:    updated(func(o object) { container(o)["image"] = "example.com/injected:latest" }),
			want:      skipped,
			detail:    "exclude-users",
		},
		{
			name:      "kube-controller-manager may change protected objects",
			request:   updateRequest("Deployment", "system:kube-controller-manager"),
			oldObject: deployment(nil),
			object:    updated(func(o object) { container(o)["image"] = "example.com/injected:latest" }),
			want:      skipped,
			detail:    "exclude-users",
		},

		// -- exclude-kinds ----------------------------------------------------
		{
			name:      "StorageClass is out of scope",
			request:   updateRequest("StorageClass", "victor"),
			oldObject: object{"apiVersion": "storage.k8s.io/v1", "kind": "StorageClass", "metadata": object{"name": "fast"}},
			object:    object{"apiVersion": "storage.k8s.io/v1", "kind": "StorageClass", "metadata": object{"name": "fast"}, "reclaimPolicy": "Delete"},
			want:      skipped,
			detail:    "exclude-kinds",
		},
		{
			name:      "DeckhouseRelease is out of scope",
			request:   updateRequest("DeckhouseRelease", "victor"),
			oldObject: object{"apiVersion": "deckhouse.io/v1alpha1", "kind": "DeckhouseRelease", "metadata": object{"name": "v1-76-9"}},
			object:    object{"apiVersion": "deckhouse.io/v1alpha1", "kind": "DeckhouseRelease", "metadata": object{"name": "v1-76-9"}, "approved": true},
			want:      skipped,
			detail:    "exclude-kinds",
		},
		{
			name:      "ModuleRelease is out of scope",
			request:   updateRequest("ModuleRelease", "victor"),
			oldObject: object{"apiVersion": "deckhouse.io/v1alpha1", "kind": "ModuleRelease", "metadata": object{"name": "sds-1-0-0"}},
			object:    object{"apiVersion": "deckhouse.io/v1alpha1", "kind": "ModuleRelease", "metadata": object{"name": "sds-1-0-0"}, "approved": true},
			want:      skipped,
			detail:    "exclude-kinds",
		},

		// -- exclude-restartedAt ----------------------------------------------
		{
			name:      "a pure rollout restart is allowed",
			request:   updateRequest("Deployment", "victor"),
			oldObject: deployment(nil),
			object:    updated(restartedAt),
			want:      skipped,
			detail:    "exclude-restartedAt",
		},
		{
			name:      "restarting an already restarted object is allowed",
			request:   updateRequest("Deployment", "victor"),
			oldObject: deployment(func(o object) { templateAnnotations(o)[restartedAtAnnotation] = "2026-08-01T00:00:00Z" }),
			object:    updated(restartedAt),
			want:      skipped,
			detail:    "exclude-restartedAt",
		},
		{
			name:      "an update that changes nothing is allowed",
			request:   updateRequest("Deployment", "victor"),
			oldObject: deployment(restartedAt),
			object:    updated(restartedAt),
			want:      skipped,
			detail:    "exclude-restartedAt",
		},
		{
			name:    "a restart may not smuggle a new image and command",
			request: updateRequest("Deployment", "victor"),
			// The reported bypass: the annotation switched the policy off and the
			// rest of the pod spec rode along with it.
			oldObject: deployment(nil),
			object: updated(func(o object) {
				restartedAt(o)
				container(o)["image"] = "docker.io/library/busybox:1.36"
				container(o)["command"] = []any{"sh", "-c", "id; sleep infinity"}
			}),
			want:   denied,
			detail: "heritage: deckhouse",
		},
		{
			name:      "a restart may not smuggle an environment variable",
			request:   updateRequest("Deployment", "victor"),
			oldObject: deployment(nil),
			object: updated(func(o object) {
				restartedAt(o)
				container(o)["env"] = []any{object{"name": "INJECTED", "value": "1"}}
			}),
			want: denied,
		},
		{
			name:      "a restart may not smuggle the no-resource-reconciliation label",
			request:   updateRequest("Deployment", "victor"),
			oldObject: deployment(nil),
			object: updated(func(o object) {
				restartedAt(o)
				labels(o)["maintenance.deckhouse.io/no-resource-reconciliation"] = "true"
			}),
			want: denied,
		},
		{
			name:      "a restart may not smuggle the heritage-bypass label",
			request:   updateRequest("Deployment", "victor"),
			oldObject: deployment(nil),
			object: updated(func(o object) {
				restartedAt(o)
				labels(o)["security.deckhouse.io/heritage-bypass"] = "delete"
			}),
			want: denied,
		},
		{
			name:      "a restart may not smuggle a replicas change",
			request:   updateRequest("Deployment", "victor"),
			oldObject: deployment(nil),
			object: updated(func(o object) {
				restartedAt(o)
				o["spec"].(object)["replicas"] = int64(0)
			}),
			want: denied,
		},
		{
			name:      "a restart may not smuggle a pod template label",
			request:   updateRequest("Deployment", "victor"),
			oldObject: deployment(nil),
			object: updated(func(o object) {
				restartedAt(o)
				o["spec"].(object)["template"].(object)["metadata"].(object)["labels"].(object)["app"] = "injected"
			}),
			want: denied,
		},
		{
			name:      "a restart may not drop another pod template annotation",
			request:   updateRequest("Deployment", "victor"),
			oldObject: deployment(nil),
			object: updated(func(o object) {
				restartedAt(o)
				delete(templateAnnotations(o), "checksum/config")
			}),
			want: denied,
		},
		{
			name:      "a restart may not smuggle an object annotation",
			request:   updateRequest("Deployment", "victor"),
			oldObject: deployment(nil),
			object: updated(func(o object) {
				restartedAt(o)
				o["metadata"].(object)["annotations"].(object)["injected"] = "1"
			}),
			want: denied,
		},
		{
			name:      "a restart may not smuggle an owner reference",
			request:   updateRequest("Deployment", "victor"),
			oldObject: deployment(nil),
			object: updated(func(o object) {
				restartedAt(o)
				o["metadata"].(object)["ownerReferences"] = []any{object{"kind": "ConfigMap", "name": "attacker"}}
			}),
			want: denied,
		},
		{
			name:      "a plain image change is denied",
			request:   updateRequest("Deployment", "victor"),
			oldObject: deployment(nil),
			object:    updated(func(o object) { container(o)["image"] = "docker.io/library/busybox:1.36" }),
			want:      denied,
		},

		// -- exclude-heritage-bypass-delete -----------------------------------
		{
			name:      "an object labeled for heritage bypass may be deleted",
			request:   deleteRequest("Deployment", "victor"),
			oldObject: deployment(func(o object) { labels(o)["security.deckhouse.io/heritage-bypass"] = "delete" }),
			object:    nil,
			want:      skipped,
			detail:    "exclude-heritage-bypass-delete",
		},
		{
			name:      "a protected object may not be deleted",
			request:   deleteRequest("Deployment", "victor"),
			oldObject: deployment(nil),
			object:    nil,
			want:      denied,
		},
		{
			name:      "another value of the heritage-bypass label does not allow deletion",
			request:   deleteRequest("Deployment", "victor"),
			oldObject: deployment(func(o object) { labels(o)["security.deckhouse.io/heritage-bypass"] = "update" }),
			object:    nil,
			want:      denied,
		},

		// -- exclude-module-source-scan-interval ------------------------------
		{
			name:      "a scan interval change is allowed",
			request:   moduleSourceRequest("victor"),
			oldObject: moduleSource(nil),
			object:    moduleSourceUpdate(func(o object) { o["spec"].(object)["scanInterval"] = "1m" }),
			want:      skipped,
			detail:    "exclude-module-source-scan-interval",
		},
		{
			name:      "dropping the scan interval is allowed",
			request:   moduleSourceRequest("victor"),
			oldObject: moduleSource(nil),
			object:    moduleSourceUpdate(func(o object) { delete(o["spec"].(object), "scanInterval") }),
			want:      skipped,
			detail:    "exclude-module-source-scan-interval",
		},
		{
			name:      "a scan interval change may not smuggle a release channel",
			request:   moduleSourceRequest("victor"),
			oldObject: moduleSource(nil),
			object: moduleSourceUpdate(func(o object) {
				o["spec"].(object)["scanInterval"] = "1m"
				o["spec"].(object)["releaseChannel"] = "alpha"
			}),
			want:   denied,
			detail: "heritage: deckhouse",
		},
		{
			name:      "a scan interval change may not smuggle a registry",
			request:   moduleSourceRequest("victor"),
			oldObject: moduleSource(nil),
			object: moduleSourceUpdate(func(o object) {
				o["spec"].(object)["scanInterval"] = "1m"
				o["spec"].(object)["registry"].(object)["repo"] = "evil.example.com/modules"
			}),
			want: denied,
		},
		{
			name:      "a scan interval change may not smuggle a label",
			request:   moduleSourceRequest("victor"),
			oldObject: moduleSource(nil),
			object: moduleSourceUpdate(func(o object) {
				o["spec"].(object)["scanInterval"] = "1m"
				labels(o)["maintenance.deckhouse.io/no-resource-reconciliation"] = "true"
			}),
			want: denied,
		},
		{
			name:      "a release channel change alone is denied",
			request:   moduleSourceRequest("victor"),
			oldObject: moduleSource(nil),
			object:    moduleSourceUpdate(func(o object) { o["spec"].(object)["releaseChannel"] = "alpha" }),
			want:      denied,
		},

		// -- validations ------------------------------------------------------
		{
			name:      "the deckhouse service account may change protected objects",
			request:   updateRequest("Deployment", "system:serviceaccount:d8-system:deckhouse"),
			oldObject: deployment(nil),
			object:    updated(func(o object) { container(o)["image"] = "registry.deckhouse.io/metrics-scraper:v2" }),
			want:      allowed,
		},
		{
			name:      "a service account of any d8- namespace may change protected objects",
			request:   updateRequest("Deployment", "system:serviceaccount:d8-monitoring:prometheus"),
			oldObject: deployment(nil),
			object:    updated(func(o object) { container(o)["image"] = "registry.deckhouse.io/metrics-scraper:v2" }),
			want:      allowed,
		},
		{
			name:      "a service account of any kube- namespace may change protected objects",
			request:   updateRequest("Deployment", "system:serviceaccount:kube-system:kubelet-config"),
			oldObject: deployment(nil),
			object:    updated(func(o object) { container(o)["image"] = "registry.deckhouse.io/metrics-scraper:v2" }),
			want:      allowed,
		},
		{
			name:      "a service account of a user namespace may not",
			request:   updateRequest("Deployment", "system:serviceaccount:default:attacker"),
			oldObject: deployment(nil),
			object:    updated(func(o object) { container(o)["image"] = "docker.io/library/busybox:1.36" }),
			want:      denied,
			detail:    "heritage: deckhouse",
		},
		{
			name:    "a regular user may not create protected objects",
			request: createRequest("Deployment", "victor"),
			object:  deployment(restartedAt),
			want:    denied,
		},
	}

	skippedBy := make(map[string]bool)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, detail := policy.evaluate(t, test.request, test.oldObject, test.object)
			if got != test.want {
				t.Fatalf("decision = %s (%s), want %s", got, detail, test.want)
			}
			if test.detail != "" && !strings.Contains(detail, test.detail) {
				t.Errorf("detail = %q, want it to contain %q", detail, test.detail)
			}
			if got == skipped {
				skippedBy[detail] = true
			}
		})
	}

	// Every exemption must be exercised: a new matchCondition without a test is
	// a new way to switch the whole policy off unnoticed.
	for _, condition := range policy.matchConditions {
		if !skippedBy[condition.name] {
			t.Errorf("no test case is exempted by the %s match condition", condition.name)
		}
	}
}

// -- fixtures ----------------------------------------------------------------

func deployment(mutate func(o object)) object {
	o := object{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": object{
			"name":            "metrics-scraper",
			"namespace":       "d8-dashboard",
			"generation":      int64(3),
			"resourceVersion": "1001",
			"managedFields":   []any{object{"manager": "deckhouse"}},
			"labels":          object{"heritage": "deckhouse", "app": "metrics-scraper"},
			"annotations":     object{"deployment.kubernetes.io/revision": "2"},
		},
		"spec": object{
			"replicas": int64(1),
			"template": object{
				"metadata": object{
					"labels":      object{"app": "metrics-scraper"},
					"annotations": object{"checksum/config": "abc"},
				},
				"spec": object{
					"serviceAccountName": "metrics-scraper",
					"containers": []any{object{
						"name":  "metrics-scraper",
						"image": "registry.deckhouse.io/metrics-scraper:v1",
					}},
				},
			},
		},
		"status": object{"replicas": int64(1)},
	}
	if mutate != nil {
		mutate(o)
	}
	return o
}

func moduleSource(mutate func(o object)) object {
	o := object{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModuleSource",
		"metadata": object{
			"name":            "deckhouse",
			"generation":      int64(2),
			"resourceVersion": "2001",
			"managedFields":   []any{object{"manager": "deckhouse"}},
			"labels":          object{"heritage": "deckhouse"},
		},
		"spec": object{
			"releaseChannel": "stable",
			"scanInterval":   "3m",
			"registry": object{
				"repo":   "registry.deckhouse.io/deckhouse/ee/modules",
				"scheme": "HTTPS",
			},
		},
		"status": object{"phase": "Available"},
	}
	if mutate != nil {
		mutate(o)
	}
	return o
}

// updated and moduleSourceUpdate mimic what the API server hands to admission:
// the metadata fields it bumps itself differ, everything else is up to the request.
func updated(mutate func(o object)) object { return serverSideBump(deployment(nil), mutate) }

func moduleSourceUpdate(mutate func(o object)) object { return serverSideBump(moduleSource(nil), mutate) }

func serverSideBump(o object, mutate func(o object)) object {
	metadata := o["metadata"].(object)
	metadata["generation"] = metadata["generation"].(int64) + 1
	metadata["resourceVersion"] = metadata["resourceVersion"].(string) + "0"
	metadata["managedFields"] = []any{object{"manager": "kubectl-patch"}}
	if mutate != nil {
		mutate(o)
	}
	return o
}

func restartedAt(o object)               { templateAnnotations(o)[restartedAtAnnotation] = "2026-08-17T17:55:00Z" }
func labels(o object) object             { return o["metadata"].(object)["labels"].(object) }
func templateAnnotations(o object) object {
	return o["spec"].(object)["template"].(object)["metadata"].(object)["annotations"].(object)
}
func container(o object) object {
	return o["spec"].(object)["template"].(object)["spec"].(object)["containers"].([]any)[0].(object)
}

func updateRequest(kind, username string, groups ...string) object {
	return admissionRequest("UPDATE", kind, username, groups)
}
func createRequest(kind, username string, groups ...string) object {
	return admissionRequest("CREATE", kind, username, groups)
}
func deleteRequest(kind, username string, groups ...string) object {
	return admissionRequest("DELETE", kind, username, groups)
}
func moduleSourceRequest(username string, groups ...string) object {
	return admissionRequest("UPDATE", "ModuleSource", username, groups)
}

func admissionRequest(operation, kind, username string, groups []string) object {
	return object{
		"operation": operation,
		"kind":      object{"group": "", "version": "v1", "kind": kind},
		"userInfo": object{
			"username": username,
			"groups":   append([]string{"system:authenticated"}, groups...),
		},
	}
}

// -- policy harness ----------------------------------------------------------

type policyDecision string

const (
	// skipped means a matchCondition evaluated to false, so the policy did not run
	// and the request is on its own - for this policy that means allowed.
	skipped policyDecision = "skipped"
	allowed policyDecision = "allowed"
	// denied covers both a failed validation and an expression error, since the
	// policy sets failurePolicy: Fail.
	denied policyDecision = "denied"
)

type admissionPolicy struct {
	matchConditions  []namedProgram
	auditAnnotations []namedProgram
	validations      []validationProgram
}

type namedProgram struct {
	name    string
	program cel.Program
}

type validationProgram struct {
	expression cel.Program
	message    cel.Program
}

// evaluate returns the decision and, depending on it, the name of the
// matchCondition that skipped the policy or the denial message.
func (p admissionPolicy) evaluate(t *testing.T, request object, oldObject, object any) (policyDecision, string) {
	t.Helper()

	activation := map[string]any{
		"request":   request,
		"object":    celValue(object),
		"oldObject": celValue(oldObject),
	}

	for _, condition := range p.matchConditions {
		matches, err := evalBool(condition.program, activation)
		if err != nil {
			return denied, fmt.Sprintf("the %s match condition errored: %v", condition.name, err)
		}
		if !matches {
			return skipped, condition.name
		}
	}

	// Audit annotations are evaluated for every request the policy matched,
	// whatever the validations decide.
	for _, annotation := range p.auditAnnotations {
		if _, _, err := annotation.program.Eval(activation); err != nil {
			t.Errorf("evaluating the %s audit annotation: %v", annotation.name, err)
		}
	}

	for _, validation := range p.validations {
		valid, err := evalBool(validation.expression, activation)
		if err != nil {
			return denied, fmt.Sprintf("a validation errored: %v", err)
		}
		if valid {
			continue
		}
		message := "no message expression"
		if validation.message != nil {
			out, _, err := validation.message.Eval(activation)
			if err != nil {
				t.Errorf("evaluating the message expression: %v", err)
			} else {
				message = fmt.Sprint(out.Value())
			}
		}
		return denied, message
	}

	return allowed, ""
}

func evalBool(program cel.Program, activation map[string]any) (bool, error) {
	out, _, err := program.Eval(activation)
	if err != nil {
		return false, err
	}
	value, ok := out.Value().(bool)
	if !ok {
		return false, fmt.Errorf("expression returned %T, want bool", out.Value())
	}
	return value, nil
}

func celValue(o any) ref.Val {
	if o == nil {
		return types.NullValue
	}
	return types.DefaultTypeAdapter.NativeToValue(o)
}

// loadPolicy compiles the ValidatingAdmissionPolicy that owns the given match
// condition out of a chart template. The policy is looked up by condition name
// because its metadata.name comes from a Helm variable.
func loadPolicy(t *testing.T, path, matchConditionName string) admissionPolicy {
	t.Helper()

	content, err := os.ReadFile("../" + path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	type expression struct {
		Name              string `yaml:"name"`
		Key               string `yaml:"key"`
		Expression        string `yaml:"expression"`
		MessageExpression string `yaml:"messageExpression"`
		ValueExpression   string `yaml:"valueExpression"`
	}
	type document struct {
		Kind string `yaml:"kind"`
		Spec struct {
			MatchConditions  []expression `yaml:"matchConditions"`
			Validations      []expression `yaml:"validations"`
			AuditAnnotations []expression `yaml:"auditAnnotations"`
		} `yaml:"spec"`
	}

	decoder := yaml.NewDecoder(strings.NewReader(withoutHelmActions(string(content))))
	for {
		var doc document
		if err := decoder.Decode(&doc); err != nil {
			t.Fatalf("no ValidatingAdmissionPolicy with the %s match condition in %s (%v)", matchConditionName, path, err)
		}
		if doc.Kind != "ValidatingAdmissionPolicy" {
			continue
		}
		found := false
		for _, condition := range doc.Spec.MatchConditions {
			found = found || condition.Name == matchConditionName
		}
		if !found {
			continue
		}

		env := celEnvironment(t)
		policy := admissionPolicy{}
		for _, condition := range doc.Spec.MatchConditions {
			policy.matchConditions = append(policy.matchConditions, namedProgram{
				name:    condition.Name,
				program: compile(t, env, condition.Name, condition.Expression, cel.BoolType),
			})
		}
		for _, annotation := range doc.Spec.AuditAnnotations {
			policy.auditAnnotations = append(policy.auditAnnotations, namedProgram{
				name:    annotation.Key,
				program: compile(t, env, annotation.Key, annotation.ValueExpression, cel.StringType),
			})
		}
		for i, validation := range doc.Spec.Validations {
			compiled := validationProgram{
				expression: compile(t, env, fmt.Sprintf("validation %d", i), validation.Expression, cel.BoolType),
			}
			if validation.MessageExpression != "" {
				compiled.message = compile(t, env, fmt.Sprintf("message %d", i), validation.MessageExpression, cel.StringType)
			}
			policy.validations = append(policy.validations, compiled)
		}
		if len(policy.matchConditions) == 0 || len(policy.validations) == 0 {
			t.Fatalf("the policy has %d match conditions and %d validations", len(policy.matchConditions), len(policy.validations))
		}
		return policy
	}
}

// celEnvironment declares the variables a ValidatingAdmissionPolicy exposes to
// its expressions. object and oldObject are dynamic because the policy matches
// every resource.
func celEnvironment(t *testing.T) *cel.Env {
	t.Helper()

	env, err := cel.NewEnv(
		cel.Variable("object", cel.DynType),
		cel.Variable("oldObject", cel.DynType),
		cel.Variable("request", cel.DynType),
	)
	if err != nil {
		t.Fatalf("building the CEL environment: %v", err)
	}
	return env
}

func compile(t *testing.T, env *cel.Env, name, expression string, want *cel.Type) cel.Program {
	t.Helper()

	ast, issues := env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		t.Fatalf("compiling %s: %v", name, issues.Err())
	}
	if !ast.OutputType().IsExactType(want) {
		t.Fatalf("%s evaluates to %s, want %s", name, ast.OutputType(), want)
	}
	program, err := env.Program(ast)
	if err != nil {
		t.Fatalf("instantiating %s: %v", name, err)
	}
	return program
}

var helmAction = regexp.MustCompile(`{{.*?}}`)

// withoutHelmActions turns a chart template into parseable YAML: lines that hold
// nothing but a Helm action are dropped, inline actions become a placeholder.
func withoutHelmActions(template string) string {
	var kept []string
	for _, line := range strings.Split(template, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "{{") && strings.HasSuffix(trimmed, "}}") {
			continue
		}
		kept = append(kept, helmAction.ReplaceAllString(line, "helm-value"))
	}
	return strings.Join(kept, "\n")
}
