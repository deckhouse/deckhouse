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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/itchyny/gojq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

// conversionHook is the shell hook whose jq programs are under test. The programs are read out of the
// shipped file rather than copied here: a copy would drift, and the thing worth pinning is what runs
// in the cluster.
const (
	conversionHook         = "../webhooks/conversion/projects"
	templateConversionHook = "../webhooks/conversion/projecttemplates"
)

// conversionHookSHA256 is the SHA-256 of the projects hook. The hook and this file also ship ahead in
// the previous release branch, so that its webhook-handler pods answer Project conversions while an
// upgrade rolls out, and both branches pin the same digest. Neither copy of the hook changes without
// this constant, and a change to it here is a change the other branch has to take as well.
const conversionHookSHA256 = "70b50e01b65c06b2bb4d881bdb7abdd7c51b9719b940e054933a690372928e0c"

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

// The projects hook is the one the previous release branch carries, byte for byte.
func TestProjectConversionHookDigest(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(conversionHook)
	require.NoError(t, err)

	sum := sha256.Sum256(raw)
	assert.Equal(t, conversionHookSHA256, hex.EncodeToString(sum[:]),
		"the projects hook changed: update conversionHookSHA256 and carry the hook to the release branch that ships it ahead")
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
			// The number 0 goes down as the string "0", the only 0 the up-conversion lifts. Given back as a
			// number, it would leave the whole quota in the parameters and spec.quota empty.
			name:  "the number 0",
			quota: map[string]any{"requests.cpu": 0, "requests.memory": "4Gi", "services.loadbalancers": 0},
			want:  map[string]any{"requests.cpu": "0", "requests.memory": "4Gi", "services.loadbalancers": "0"},
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

// The up-conversion lifts a parameter into a standard field only if v1alpha3 can hold it and the
// down-conversion gives it back as it was, apart from a fraction, which comes back as a string. Every
// shape here, whether it is lifted or left in the parameters, has to come back from v1alpha3 as the
// client wrote it, with every jq engine, and what reaches the standard fields has to read the way the
// controller and the schema read it.
func TestProjectLiftGivesBack(t *testing.T) {
	t.Parallel()

	admin := map[string]any{"subject": "User", "name": "alice@example.com"}
	group := map[string]any{"subject": "Group", "name": "backend-admins"}
	both := []string{"administrators", "quota"}

	tests := []struct {
		name       string
		parameters map[string]any
		// lifted lists the standard fields the up-conversion fills.
		lifted []string
		// back is the parameters after the round trip; nil means the same as parameters.
		back map[string]any
	}{
		{
			name: "the shape of the default template",
			parameters: map[string]any{
				"administrators": []any{admin, group},
				"resourceQuota": map[string]any{
					"requests": map[string]any{"cpu": 5, "memory": "5Gi", "storage": "1Gi"},
					"limits":   map[string]any{"cpu": "500m", "memory": "5Gi"},
				},
				"networkPolicy": "Isolated",
			},
			lifted: both,
		},
		{
			name: "flat keys and a dotted resource name next to the nesting",
			parameters: map[string]any{"resourceQuota": map[string]any{
				"requests":               map[string]any{"nvidia.com/gpu": 2},
				"pods":                   "10",
				"count/deployments.apps": "5",
				"services.loadbalancers": 1,
			}},
			lifted: []string{"quota"},
		},
		{
			// spec.quota holds an integer or a string, so a fraction can only be lifted as a string.
			name: "a fraction",
			parameters: map[string]any{"resourceQuota": map[string]any{
				"requests": map[string]any{"cpu": 0.5, "memory": "512Mi"},
				"limits":   map[string]any{"cpu": 1},
			}},
			lifted: []string{"quota"},
			back: map[string]any{"resourceQuota": map[string]any{
				"requests": map[string]any{"cpu": "0.5", "memory": "512Mi"},
				"limits":   map[string]any{"cpu": 1},
			}},
		},
		{
			name:       "a bare requests key",
			parameters: map[string]any{"resourceQuota": map[string]any{"requests": "5", "limits": "7"}},
			lifted:     []string{"quota"},
		},
		{
			// The controller drops an empty standard field when it writes the project back, and the
			// v1alpha2 view would lose the parameter with it.
			name:       "an empty list and an empty quota",
			parameters: map[string]any{"administrators": []any{}, "resourceQuota": map[string]any{}},
		},
		{
			name:       "an empty requests next to a flat key",
			parameters: map[string]any{"resourceQuota": map[string]any{"requests": map[string]any{}, "pods": 3}},
		},
		{
			name: "an empty limits next to requests",
			parameters: map[string]any{"resourceQuota": map[string]any{
				"requests": map[string]any{"cpu": "1"},
				"limits":   map[string]any{},
			}},
		},
		{
			name:       "names instead of subjects",
			parameters: map[string]any{"administrators": []any{"alice@example.com"}},
		},
		{
			name:       "one subject instead of a list",
			parameters: map[string]any{"administrators": admin},
		},
		{
			name: "an item with an access level",
			parameters: map[string]any{"administrators": []any{
				admin,
				map[string]any{"subject": "Group", "name": "developers", "accessLevel": "Editor"},
			}},
		},
		{
			name:       "a ServiceAccount subject",
			parameters: map[string]any{"administrators": []any{map[string]any{"subject": "ServiceAccount", "name": "ci"}}},
		},
		{
			// The User and the Group RBAC matches to ServiceAccounts are administrators like any other,
			// whatever namespace they name, and the controller binds them as such.
			name: "the user and the group of ServiceAccounts",
			parameters: map[string]any{"administrators": []any{
				map[string]any{"subject": "User", "name": "system:serviceaccount:ci:runner"},
				map[string]any{"subject": "Group", "name": "system:serviceaccounts:ci"},
			}},
			lifted: []string{"administrators"},
		},
		{
			name:       "a name with a line break",
			parameters: map[string]any{"administrators": []any{map[string]any{"subject": "User", "name": "alice\nkind: Group"}}},
		},
		{
			name:       "an empty name",
			parameters: map[string]any{"administrators": []any{map[string]any{"subject": "User", "name": ""}}},
		},
		{
			name:       "a quota given as a string",
			parameters: map[string]any{"resourceQuota": "10Gi"},
		},
		{
			name: "a nested quota object",
			parameters: map[string]any{"resourceQuota": map[string]any{
				"requests": map[string]any{"cpu": "1"},
				"services": map[string]any{"loadbalancers": 2},
			}},
		},
		{
			name: "scopes",
			parameters: map[string]any{"resourceQuota": map[string]any{
				"requests": map[string]any{"cpu": "1"},
				"scopes":   []any{"BestEffort"},
			}},
		},
		{
			name: "a flat key next to the nesting it collides with",
			parameters: map[string]any{"resourceQuota": map[string]any{
				"requests":     map[string]any{"cpu": "1"},
				"requests.cpu": "2",
			}},
		},
		{
			name:       "a flat key that would come back nested",
			parameters: map[string]any{"resourceQuota": map[string]any{"limits.memory": "1Gi"}},
		},
		{
			name:       "a value that is no quantity",
			parameters: map[string]any{"resourceQuota": map[string]any{"requests": map[string]any{"cpu": "lots"}}},
		},
		{
			name:       "a quantity with a trailing newline",
			parameters: map[string]any{"resourceQuota": map[string]any{"requests": map[string]any{"cpu": "1\n"}}},
		},
		{
			name:       "an exponent longer than three digits",
			parameters: map[string]any{"resourceQuota": map[string]any{"limits": map[string]any{"cpu": "1e-1000"}}},
		},
		{
			// The templates of the previous release took the number 0 for no limit, and spec.quota
			// would make it a hard one.
			name: "the number 0",
			parameters: map[string]any{"resourceQuota": map[string]any{
				"requests": map[string]any{"cpu": 0, "memory": "5Gi"},
				"pods":     10,
			}},
		},
		{
			name:       "the number 0 as a flat key",
			parameters: map[string]any{"resourceQuota": map[string]any{"services.loadbalancers": 0}},
		},
		{
			name: "the string 0",
			parameters: map[string]any{"resourceQuota": map[string]any{
				"requests": map[string]any{"cpu": "0"},
				"limits":   map[string]any{"memory": "0"},
			}},
			lifted: []string{"quota"},
		},
		{
			name:       "a quantity of 64 characters",
			parameters: map[string]any{"resourceQuota": map[string]any{"pods": "1" + strings.Repeat("0", 63)}},
			lifted:     []string{"quota"},
		},
		{
			// The canonical form of a long mantissa takes time quadratic in its length, and the
			// controller computes it each time it writes the project.
			name:       "a quantity longer than 64 characters",
			parameters: map[string]any{"resourceQuota": map[string]any{"pods": "1" + strings.Repeat("0", 64)}},
		},
		{
			name: "administrators lifted next to a quota left behind",
			parameters: map[string]any{
				"administrators": []any{admin},
				"resourceQuota":  map[string]any{"requests": map[string]any{"cpu": "1"}, "scopes": []any{"BestEffort"}},
			},
			lifted: []string{"administrators"},
		},
	}

	for _, engine := range jqEngines(t) {
		for _, tt := range tests {
			t.Run(engine.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				project := map[string]any{
					"apiVersion": "deckhouse.io/v1alpha2",
					"kind":       "Project",
					"metadata":   map[string]any{"name": "test"},
					"spec":       map[string]any{"projectTemplateName": "custom", "parameters": tt.parameters},
				}

				up := runConversion(t, engine, conversionHook, "v1alpha2_to_v1alpha3", []any{project})
				spec, ok := up[0].(map[string]any)["spec"].(map[string]any)
				require.True(t, ok, "the up-conversion produced no spec: %v", up[0])

				var lifted []string
				for _, field := range []string{"administrators", "quota"} {
					if _, found := spec[field]; found {
						lifted = append(lifted, field)
					}
				}
				assert.Equal(t, tt.lifted, lifted, "the standard fields the up-conversion filled")
				assertStandardFieldsRead(t, spec)

				down := runConversion(t, engine, conversionHook, "v1alpha3_to_v1alpha2", up)
				if tt.back != nil {
					project["spec"] = map[string]any{"projectTemplateName": "custom", "parameters": tt.back}
				}
				want, err := json.Marshal(project)
				require.NoError(t, err)
				got, err := json.Marshal(down[0])
				require.NoError(t, err)
				assert.JSONEq(t, string(want), string(got), "the round trip through v1alpha3")
			})
		}
	}
}

// administratorName is the pattern of spec.administrators[].name in the v1alpha3 schema of
// crds/projects.yaml. It is spelled out rather than read from there because the conversion and its
// fixtures also ship ahead in a release whose schema has no v1alpha3.
var administratorName = regexp.MustCompile(`^[^\x00-\x1f\x7f]+$`)

// assertStandardFieldsRead checks that the standard fields of a converted project read the way the
// controller reads them: spec.quota is decoded as a ResourceList, so one value that is no quantity
// fails the read of every project, and an administrator is a User or Group with a name the schema
// accepts.
func assertStandardFieldsRead(t *testing.T, spec map[string]any) {
	t.Helper()

	if quota, found := spec["quota"]; found {
		raw, err := json.Marshal(quota)
		require.NoError(t, err)
		var hard corev1.ResourceList
		assert.NoError(t, json.Unmarshal(raw, &hard), "spec.quota %s does not read as quantities", raw)
	}

	if administrators, found := spec["administrators"]; found {
		raw, err := json.Marshal(administrators)
		require.NoError(t, err)

		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		var subjects []struct {
			Kind string `json:"kind"`
			Name string `json:"name"`
		}
		require.NoError(t, decoder.Decode(&subjects), "spec.administrators %s", raw)

		for _, subject := range subjects {
			assert.Contains(t, []string{"User", "Group"}, subject.Kind, "the kind of %q", subject.Name)
			assert.Regexp(t, administratorName, subject.Name, "the name of a %s", subject.Kind)
		}
	}
}

// jqPattern matches the string literal a jq regex builtin takes as its pattern.
var jqPattern = regexp.MustCompile(`\b(?:test|match|capture|scan|splits|sub|gsub)\(\s*("(?:[^"\\]|\\.)*")`)

// The unit test image has no jq binary, so in CI the programs run on gojq alone, whose patterns are
// RE2 and whose $ is the end of the text. The $ of jq is Oniguruma's and also matches before a
// trailing newline, which would let "1\n" through as a quantity. Every pattern of the projects hook
// is therefore anchored with \A and \z, and that is checked here without an engine to show it.
func TestProjectConversionPatternsAreAnchored(t *testing.T) {
	t.Parallel()

	functions := []string{"v1alpha1_to_v1alpha2", "v1alpha2_to_v1alpha1", "v1alpha2_to_v1alpha3", "v1alpha3_to_v1alpha2"}

	var patterns []string
	for _, function := range functions {
		for _, match := range jqPattern.FindAllStringSubmatch(jqProgram(t, conversionHook, function), -1) {
			// A jq string literal without interpolation is a JSON string.
			var pattern string
			require.NoError(t, json.Unmarshal([]byte(match[1]), &pattern), "the pattern %s of %s", match[1], function)
			patterns = append(patterns, pattern)
		}
	}
	require.NotEmpty(t, patterns, "the projects hook hands no pattern to test()")

	for _, pattern := range patterns {
		anchored := strings.HasPrefix(pattern, `\A`) && strings.HasSuffix(pattern, `\z`)
		assert.True(t, anchored, "%q is not anchored with \\A and \\z", pattern)
		assert.NotContains(t, pattern, "$", "the $ of jq matches before a trailing newline")
		assert.NotContains(t, pattern, "^", "an anchor of %q is not \\A", pattern)
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
// only inputs the two agree on. The unit test image has no jq, so the anchors on which the two would
// disagree are pinned by TestProjectConversionPatternsAreAnchored, which needs no engine.
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
