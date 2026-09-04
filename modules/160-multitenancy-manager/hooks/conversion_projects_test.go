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

// A ProjectTemplate keeps its fields across the version bump. v1alpha1 is not served -- the
// structured fields it cannot describe are pruned on the way down and cannot come back, which is why
// -- but the CRD still declares the conversion and the apiserver still asks for it, so it has to
// answer. It once did not: removing this hook left a live cluster failing about one conversion a
// second.
func TestProjectTemplateVersionBump(t *testing.T) {
	t.Parallel()

	template := map[string]any{
		"apiVersion": "deckhouse.io/v1alpha2",
		"kind":       "ProjectTemplate",
		"metadata":   map[string]any{"name": "test"},
		"spec": map[string]any{
			"description":       "a template",
			"resourcesTemplate": "---\napiVersion: v1\nkind: Namespace\n",
			"parametersSchema":  map[string]any{"openAPIV3Schema": map[string]any{"type": "object"}},
		},
	}

	down := convertWith(t, templateConversionHook, "v1alpha2_to_v1alpha1", template)
	assert.Equal(t, "deckhouse.io/v1alpha1", down["apiVersion"])
	assert.Equal(t, specField(template, "description"), specField(down, "description"))
	assert.Equal(t, specField(template, "resourcesTemplate"), specField(down, "resourcesTemplate"))

	up := convertWith(t, templateConversionHook, "v1alpha1_to_v1alpha2", down)
	assert.Equal(t, "deckhouse.io/v1alpha2", up["apiVersion"])
	assert.Equal(t, template["spec"], up["spec"])
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

// asAny retypes a literal map the way a decoded JSON document looks, so comparisons are of values
// rather than of Go types.
func asAny(value map[string]any) map[string]any {
	out := make(map[string]any, len(value))
	for key, nested := range value {
		out[key] = nested
	}

	return out
}
