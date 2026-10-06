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
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/itchyny/gojq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// conversionHook is the shell hook whose jq programs are under test. The programs are read out of the
// shipped file rather than copied here: a copy would drift, and the thing worth pinning is what runs
// in the cluster.
const (
	conversionHook         = "../webhooks/conversion/projects"
	templateConversionHook = "../webhooks/conversion/projecttemplates"
)

// conversionFixtures holds the golden fixtures of the conversion hooks, a directory per hook and a file
// per conversion path. The fixtures describe the hooks, not this test, so any copy of the hooks can be
// checked against them as they are.
const conversionFixtures = "testdata/conversion"

// The golden fixtures pin each conversion of the projects hook object by object rather than field by
// field, so whatever a change to the hook adds, drops or leaves behind shows up. The last path is the
// chain shell-operator runs for a project still stored as v1alpha1 and read at the storage version.
func TestProjectConversionGolden(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"v1alpha1_to_v1alpha2",
		"v1alpha2_to_v1alpha1",
		"v1alpha2_to_v1alpha3",
		"v1alpha3_to_v1alpha2",
		"v1alpha1_to_v1alpha2_to_v1alpha3",
	} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			testConversionGolden(t, conversionHook, filepath.Join(conversionFixtures, "projects", path+".yaml"))
		})
	}
}

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

// convert runs one conversion function of the projects hook over a single object and returns it.
func convert(t *testing.T, function string, object map[string]any) map[string]any {
	t.Helper()

	return convertWith(t, conversionHook, function, object)
}

func convertWith(t *testing.T, hook, function string, object map[string]any) map[string]any {
	t.Helper()

	converted := runConversion(t, gojqEngine, hook, function, []any{object})

	out, ok := converted[0].(map[string]any)
	if !ok {
		t.Fatalf("%s produced a non-object: %v", function, converted[0])
	}

	return out
}

// conversionCase is one ConversionReview of a golden fixture: the objects the apiserver sends and the
// objects the hook has to answer with, in the same order.
type conversionCase struct {
	Name      string `json:"name"`
	Objects   []any  `json:"objects"`
	Converted []any  `json:"converted"`
}

// testConversionGolden runs every case of a fixture file through the conversion path the file is named
// after, with every jq engine. A name such as v1alpha1_to_v1alpha2 is one function of the hook; a longer
// one is the chain shell-operator builds when no single function covers the pair: the whole review goes
// through each step in turn, and each step converts the objects at its source version and has to leave
// the others alone.
func testConversionGolden(t *testing.T, hook, fixture string) {
	t.Helper()

	raw, err := os.ReadFile(fixture)
	require.NoError(t, err)

	var cases []conversionCase
	require.NoError(t, yaml.UnmarshalStrict(raw, &cases), "decoding %s", fixture)
	require.NotEmpty(t, cases, "%s holds no cases", fixture)

	chain := conversionChain(t, strings.TrimSuffix(filepath.Base(fixture), filepath.Ext(fixture)))

	for _, engine := range jqEngines(t) {
		for _, tc := range cases {
			t.Run(engine.name+"/"+tc.Name, func(t *testing.T) {
				t.Parallel()

				objects := tc.Objects
				for _, function := range chain {
					objects = runConversion(t, engine, hook, function, objects)
				}

				want, err := json.Marshal(tc.Converted)
				require.NoError(t, err)
				got, err := json.Marshal(objects)
				require.NoError(t, err)
				assert.JSONEq(t, string(want), string(got))
			})
		}
	}
}

// conversionChain splits a conversion path such as v1alpha1_to_v1alpha2_to_v1alpha3 into the functions
// of the hook it runs, in order.
func conversionChain(t *testing.T, path string) []string {
	t.Helper()

	versions := strings.Split(path, "_to_")
	if len(versions) < 2 {
		t.Fatalf("%q names no conversion", path)
	}

	chain := make([]string, 0, len(versions)-1)
	for i := 1; i < len(versions); i++ {
		chain = append(chain, versions[i-1]+"_to_"+versions[i])
	}

	return chain
}

// runConversion runs one conversion function of a hook over the objects of one review and returns the
// converted objects. shell-operator refuses an answer that does not hold one object per object sent,
// so that is checked here too.
func runConversion(t *testing.T, engine jqEngine, hook, function string, objects []any) []any {
	t.Helper()

	review := map[string]any{"review": map[string]any{"request": map[string]any{"objects": objects}}}
	result := engine.run(t, jqProgram(t, hook, function), review)

	converted, ok := result.([]any)
	if !ok || len(converted) != len(objects) {
		t.Fatalf("%s with %s produced %v for %d objects", function, engine.name, result, len(objects))
	}

	return converted
}

// jqEngine runs a jq program over one input and returns its one output.
type jqEngine struct {
	name string
	run  func(t *testing.T, program string, input any) any
}

// gojqEngine is linked into the test binary, so it is always there.
var gojqEngine = jqEngine{name: "gojq", run: runGojq}

// jqEngines lists the engines the fixtures are checked with. The hook runs jq in the cluster, and the
// jq binary is used whenever it is on PATH, because gojq is not the same language in every corner: its
// \s is the ASCII class of RE2, where the \s of jq is the Unicode class of Oniguruma. The fixtures hold
// only inputs the two agree on.
func jqEngines(t *testing.T) []jqEngine {
	t.Helper()

	engines := []jqEngine{gojqEngine}

	path, err := exec.LookPath("jq")
	if err != nil {
		t.Log("jq is not on PATH, the conversions are checked with gojq only")

		return engines
	}

	return append(engines, jqEngine{name: "jq", run: jqBinary(path)})
}

func runGojq(t *testing.T, program string, input any) any {
	t.Helper()

	query, err := gojq.Parse(program)
	if err != nil {
		t.Fatalf("the jq program does not parse: %v", err)
	}

	// gojq writes into the maps of its input while it deletes paths, and the input of one test can be
	// read by another one running in parallel, so the program runs over a copy.
	raw, err := json.Marshal(input)
	require.NoError(t, err)
	var copied any
	require.NoError(t, json.Unmarshal(raw, &copied))

	result, ok := query.Run(copied).Next()
	if !ok {
		t.Fatal("the jq program produced nothing")
	}
	if err, isErr := result.(error); isErr {
		t.Fatalf("the jq program failed: %v", err)
	}

	return result
}

// jqBinary runs a program the way the hook does, with jq reading the review from its input.
func jqBinary(path string) func(t *testing.T, program string, input any) any {
	return func(t *testing.T, program string, input any) any {
		t.Helper()

		raw, err := json.Marshal(input)
		require.NoError(t, err)

		var stderr bytes.Buffer
		cmd := exec.CommandContext(t.Context(), path, "-c", program)
		cmd.Stdin = bytes.NewReader(raw)
		cmd.Stderr = &stderr

		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("jq failed: %v: %s", err, stderr.String())
		}

		var result any
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatalf("jq produced %q: %v", out, err)
		}

		return result
	}
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
