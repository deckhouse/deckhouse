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

package hooks

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/itchyny/gojq"
	"github.com/stretchr/testify/assert"
)

// conversionHook is the shell hook whose jq programs are under test. The programs are read out of the
// shipped file rather than copied here: a copy would drift, and the thing worth pinning is what runs
// in the cluster.
const (
	conversionHook         = "../webhooks/conversion/projects"
	templateConversionHook = "../webhooks/conversion/projecttemplates"
)

// A project version round-trip has to return the object it started from. The two conversions are
// written independently, in jq, against a quota whose shape is only partly nesting -- "requests.cpu"
// nests, "pods" does not, "requests" alone is a key that merely looks like nesting -- and both bugs
// this test was written for were in that seam.
func TestProjectQuotaRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		quota map[string]any
		// want is the quota after v1alpha3 -> v1alpha2 -> v1alpha3; empty means "the same as quota".
		want map[string]any
	}{
		{
			name:  "requests and limits",
			quota: map[string]any{"requests.cpu": "1", "requests.memory": "1Gi", "limits.memory": "2Gi"},
		},
		{
			name:  "a key with no nesting at all",
			quota: map[string]any{"pods": "10", "count/deployments.apps": "5"},
		},
		{
			name:  "a resource name that itself contains a dot",
			quota: map[string]any{"requests.nvidia.com/gpu": "2"},
		},
		{
			name:  "nesting and flat keys together",
			quota: map[string]any{"requests.cpu": "1", "pods": "10", "services.loadbalancers": "2"},
		},
		{
			// Nothing in the schema forbids a key spelled exactly "requests". Expanding it as nesting
			// fed a string to to_entries, which failed the conversion of the whole object.
			name:  "a bare requests key",
			quota: map[string]any{"requests": "5", "limits": "7"},
		},
		{
			// The two cannot coexist in v1alpha2, which has one key for both. The nested form wins,
			// whatever the order of the keys.
			name:  "a bare requests key next to a nested one",
			quota: map[string]any{"requests": "5", "requests.cpu": "1"},
			want:  map[string]any{"requests.cpu": "1"},
		},
		{
			name:  "an empty quota",
			quota: map[string]any{},
			want:  map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			project := map[string]any{
				"apiVersion": "deckhouse.io/v1alpha3",
				"kind":       "Project",
				"metadata":   map[string]any{"name": "test"},
				"spec": map[string]any{
					"projectTemplateName": "default",
					"administrators":      []any{map[string]any{"kind": "User", "name": "alice"}},
					"quota":               tt.quota,
				},
			}

			down := convert(t, "v1alpha3_to_v1alpha2", project)
			if version := down["apiVersion"]; version != "deckhouse.io/v1alpha2" {
				t.Fatalf("down-conversion produced %v", version)
			}

			up := convert(t, "v1alpha2_to_v1alpha3", down)

			want := tt.want
			if want == nil {
				want = tt.quota
			}
			if got := specField(up, "quota"); !reflect.DeepEqual(got, asAny(want)) {
				t.Errorf("quota after the round trip:\n got: %v\nwant: %v", got, asAny(want))
			}

			// Administrators travel the same road, and the round trip has to leave them alone too.
			if got := specField(up, "administrators"); !reflect.DeepEqual(got, specField(project, "administrators")) {
				t.Errorf("administrators after the round trip: %v", got)
			}
		})
	}
}

// The v1alpha2 layout has nowhere to put a nested object that is not requests/limits, so the
// up-conversion drops it. That is a deliberate loss, and this is where it is written down.
func TestProjectQuotaDropsNestedObjects(t *testing.T) {
	t.Parallel()

	project := map[string]any{
		"apiVersion": "deckhouse.io/v1alpha2",
		"kind":       "Project",
		"metadata":   map[string]any{"name": "test"},
		"spec": map[string]any{
			"parameters": map[string]any{
				"resourceQuota": map[string]any{
					"requests": map[string]any{"cpu": "1"},
					"nested":   map[string]any{"any": "thing"},
					"listed":   []any{"a"},
				},
			},
		},
	}

	quota, ok := specField(convert(t, "v1alpha2_to_v1alpha3", project), "quota").(map[string]any)
	if !ok {
		t.Fatal("the up-conversion produced no quota")
	}

	if _, dropped := quota["nested"]; dropped {
		t.Error("a nested object reached spec.quota, which is a map of strings")
	}
	if _, dropped := quota["listed"]; dropped {
		t.Error("a list reached spec.quota, which is a map of strings")
	}
	if quota["requests.cpu"] != "1" {
		t.Errorf("the nesting next to it was lost: %v", quota)
	}
}

// v1alpha2 has no resourcesTemplate. A v1alpha1 template that carried a Helm string comes up without
// it and marked with the legacy-helm-template annotation, so the controller refuses to render the
// empty structured shape over the release that string built; a template whose string was empty (or
// whitespace) is not marked -- there was nothing to lose.
func TestProjectTemplateUpConversionDropsTheHelmString(t *testing.T) {
	t.Parallel()

	const mark = "projects.deckhouse.io/legacy-helm-template"

	tests := []struct {
		name     string
		resource any
		marked   bool
	}{
		{name: "a helm string", resource: "---\napiVersion: v1\nkind: Namespace\n", marked: true},
		{name: "an empty string", resource: "", marked: false},
		{name: "whitespace only", resource: "  \n\t", marked: false},
		{name: "no field at all", resource: nil, marked: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			spec := map[string]any{
				"description":      "a template",
				"parametersSchema": map[string]any{"openAPIV3Schema": map[string]any{"type": "object"}},
			}
			if tt.resource != nil {
				spec["resourcesTemplate"] = tt.resource
			}
			template := map[string]any{
				"apiVersion": "deckhouse.io/v1alpha1",
				"kind":       "ProjectTemplate",
				"metadata":   map[string]any{"name": "test", "annotations": map[string]any{"keep": "me"}},
				"spec":       spec,
			}

			up := convertWith(t, templateConversionHook, "v1alpha1_to_v1alpha2", template)
			assert.Equal(t, "deckhouse.io/v1alpha2", up["apiVersion"])
			assert.Nil(t, specField(up, "resourcesTemplate"), "v1alpha2 must not carry the Helm string")
			assert.Equal(t, "a template", specField(up, "description"))
			assert.Equal(t, spec["parametersSchema"], specField(up, "parametersSchema"))

			annotations, _ := up["metadata"].(map[string]any)["annotations"].(map[string]any)
			assert.Equal(t, "me", annotations["keep"], "the other annotations survive")
			if tt.marked {
				assert.Equal(t, "true", annotations[mark])
			} else {
				assert.NotContains(t, annotations, mark)
			}
		})
	}
}

// The apiserver keeps asking for the conversion of whatever v1alpha1 objects it holds, and a
// template with no spec at all is a shape it can hold. The conversion must answer, not fail.
func TestProjectTemplateConversionSurvivesAMissingSpec(t *testing.T) {
	t.Parallel()

	bare := map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ProjectTemplate",
		"metadata":   map[string]any{"name": "test"},
	}

	up := convertWith(t, templateConversionHook, "v1alpha1_to_v1alpha2", bare)
	assert.Equal(t, "deckhouse.io/v1alpha2", up["apiVersion"])
	_, hasSpec := up["spec"]
	assert.False(t, hasSpec, "a missing spec is not conjured")

	down := convertWith(t, templateConversionHook, "v1alpha2_to_v1alpha1", map[string]any{
		"apiVersion": "deckhouse.io/v1alpha2",
		"kind":       "ProjectTemplate",
		"metadata":   map[string]any{"name": "test"},
	})
	assert.Equal(t, "deckhouse.io/v1alpha1", down["apiVersion"])
	assert.Equal(t, "", specField(down, "resourcesTemplate"), "v1alpha1 requires the field, so it is backfilled")
}

// A round trip through v1alpha1 keeps what both versions can describe. The Helm string is not among
// that: it goes down as the empty string v1alpha1 requires and does not come back.
func TestProjectTemplateVersionBump(t *testing.T) {
	t.Parallel()

	template := map[string]any{
		"apiVersion": "deckhouse.io/v1alpha2",
		"kind":       "ProjectTemplate",
		"metadata":   map[string]any{"name": "test"},
		"spec": map[string]any{
			"description":      "a template",
			"parametersSchema": map[string]any{"openAPIV3Schema": map[string]any{"type": "object"}},
		},
	}

	down := convertWith(t, templateConversionHook, "v1alpha2_to_v1alpha1", template)
	assert.Equal(t, "deckhouse.io/v1alpha1", down["apiVersion"])
	assert.Equal(t, specField(template, "description"), specField(down, "description"))
	assert.Equal(t, "", specField(down, "resourcesTemplate"))

	up := convertWith(t, templateConversionHook, "v1alpha1_to_v1alpha2", down)
	assert.Equal(t, "deckhouse.io/v1alpha2", up["apiVersion"])
	assert.Equal(t, template["spec"], up["spec"])
	annotations, _ := up["metadata"].(map[string]any)["annotations"].(map[string]any)
	assert.NotContains(t, annotations, "projects.deckhouse.io/legacy-helm-template", "an empty string is not a Helm template")
}

// status.namespaces is a list of names in v1alpha2 and a list of {name, kind} objects in v1alpha3.
// A view at either version has to validate against its own schema, so the status is converted with
// the spec: down to names, up to objects with the kind derived from the project name.
func TestProjectStatusNamespacesRoundTrip(t *testing.T) {
	t.Parallel()

	project := map[string]any{
		"apiVersion": "deckhouse.io/v1alpha3",
		"kind":       "Project",
		"metadata":   map[string]any{"name": "backend"},
		"spec":       map[string]any{"projectTemplateName": "default"},
		"status": map[string]any{
			"state": "Deployed",
			"namespaces": []any{
				map[string]any{"name": "backend", "kind": "Main"},
				map[string]any{"name": "backend-cache", "kind": "Additional"},
			},
		},
	}

	down := convert(t, "v1alpha3_to_v1alpha2", project)
	assert.Equal(t, []any{"backend", "backend-cache"}, statusField(down, "namespaces"))
	assert.Equal(t, "Deployed", statusField(down, "state"), "the rest of the status is untouched")

	up := convert(t, "v1alpha2_to_v1alpha3", down)
	assert.Equal(t, project["status"], up["status"])
}

// The conversion of the status must not depend on there being one, or on there being a spec: the
// apiserver converts whatever it stored, including a project that never reconciled.
func TestProjectConversionSurvivesMissingSpecAndStatus(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		function string
		object   map[string]any
		want     string
	}{
		{"v1alpha1 without spec", "v1alpha1_to_v1alpha2", map[string]any{"apiVersion": "deckhouse.io/v1alpha1"}, "deckhouse.io/v1alpha2"},
		{"v1alpha2 without spec or status", "v1alpha2_to_v1alpha3", map[string]any{"apiVersion": "deckhouse.io/v1alpha2"}, "deckhouse.io/v1alpha3"},
		{"v1alpha3 without spec or status", "v1alpha3_to_v1alpha2", map[string]any{"apiVersion": "deckhouse.io/v1alpha3"}, "deckhouse.io/v1alpha2"},
		{"v1alpha2 with a status but no namespaces", "v1alpha2_to_v1alpha3", map[string]any{"apiVersion": "deckhouse.io/v1alpha2", "status": map[string]any{"state": "Error"}}, "deckhouse.io/v1alpha3"},
		{"v1alpha3 with an empty namespace list", "v1alpha3_to_v1alpha2", map[string]any{"apiVersion": "deckhouse.io/v1alpha3", "status": map[string]any{"namespaces": []any{}}}, "deckhouse.io/v1alpha2"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.object["kind"] = "Project"
			tt.object["metadata"] = map[string]any{"name": "test"}
			out := convert(t, tt.function, tt.object)
			assert.Equal(t, tt.want, out["apiVersion"])
			_, hasSpec := out["spec"]
			_, hadSpec := tt.object["spec"]
			assert.Equal(t, hadSpec, hasSpec, "a missing spec is not conjured")
		})
	}
}

// A structured template has neither of the two fields the v1alpha1 schema requires, and the apiserver
// validates what a conversion returns, so they are backfilled rather than left missing.
func TestProjectTemplateBackfillsWhatV1alpha1Requires(t *testing.T) {
	t.Parallel()

	structured := map[string]any{
		"apiVersion": "deckhouse.io/v1alpha2",
		"kind":       "ProjectTemplate",
		"metadata":   map[string]any{"name": "test"},
		"spec":       map[string]any{"description": "structured only"},
	}

	down := convertWith(t, templateConversionHook, "v1alpha2_to_v1alpha1", structured)
	assert.Equal(t, "", specField(down, "resourcesTemplate"))
	assert.Equal(t, map[string]any{"openAPIV3Schema": map[string]any{}}, specField(down, "parametersSchema"))
}

// convert runs one conversion function of the projects hook over a single object and returns it.
func convert(t *testing.T, function string, object map[string]any) map[string]any {
	t.Helper()

	return convertWith(t, conversionHook, function, object)
}

func convertWith(t *testing.T, hook, function string, object map[string]any) map[string]any {
	t.Helper()

	query, err := gojq.Parse(jqProgram(t, hook, function))
	if err != nil {
		t.Fatalf("the jq program of %s does not parse: %v", function, err)
	}

	review := map[string]any{"review": map[string]any{"request": map[string]any{"objects": []any{object}}}}

	iter := query.Run(review)
	result, ok := iter.Next()
	if !ok {
		t.Fatalf("%s produced nothing", function)
	}
	if err, isErr := result.(error); isErr {
		t.Fatalf("%s failed: %v", function, err)
	}

	converted, ok := result.([]any)
	if !ok || len(converted) != 1 {
		t.Fatalf("%s produced %v", function, result)
	}

	out, ok := converted[0].(map[string]any)
	if !ok {
		t.Fatalf("%s produced a non-object: %v", function, converted[0])
	}

	return out
}

// jqProgram extracts the jq source of one conversion function from the shell hook. The programs are
// single-quoted bash strings, which cannot themselves contain a single quote, so the quotes delimit
// them unambiguously.
func jqProgram(t *testing.T, hookPath, function string) string {
	t.Helper()

	hook, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("reading %s: %v", hookPath, err)
	}

	body := string(hook)
	start := strings.Index(body, "function __on_conversion::"+function+"()")
	if start < 0 {
		t.Fatalf("the hook has no %s function", function)
	}

	body = body[start:]
	open := strings.Index(body, "'")
	if open < 0 {
		t.Fatalf("%s runs no jq program", function)
	}
	body = body[open+1:]

	end := strings.Index(body, "'")
	if end < 0 {
		t.Fatalf("the jq program of %s is not closed", function)
	}

	return body[:end]
}

func specField(object map[string]any, name string) any {
	spec, _ := object["spec"].(map[string]any)

	return spec[name]
}

func statusField(object map[string]any, name string) any {
	status, _ := object["status"].(map[string]any)

	return status[name]
}

// asAny retypes a literal map the way a decoded JSON document looks, so comparisons are of values
// rather than of Go types.
func asAny(value map[string]any) map[string]any {
	out := make(map[string]any, len(value))
	for key, nested := range value {
		out[key] = nested
	}

	return out
}
