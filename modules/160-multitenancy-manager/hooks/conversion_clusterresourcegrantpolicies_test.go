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
	"encoding/json"
	"testing"

	"github.com/itchyny/gojq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	grantPolicyConversionHook = "../webhooks/conversion/clusterresourcegrantpolicies"
	// grantPolicyProjectSelectorAnnotation keeps a v1alpha2 projectSelector, which v1alpha1 has no field
	// for, while the object is served as v1alpha1.
	grantPolicyProjectSelectorAnnotation = "multitenancy.deckhouse.io/v1alpha2-project-selector"
)

func grantPolicy(apiVersion string, spec map[string]any) map[string]any {
	return map[string]any{
		"apiVersion": apiVersion,
		"kind":       "ClusterResourceGrantPolicy",
		"metadata":   map[string]any{"name": "p"},
		"spec":       spec,
	}
}

func selector(key, value string) map[string]any {
	return map[string]any{"matchLabels": map[string]any{key: value}}
}

// normalize turns a literal into the shape a decoded JSON document has, so values compare equal
// whatever Go types the literal used.
func normalize(t *testing.T, value any) any {
	t.Helper()

	raw, err := json.Marshal(value)
	require.NoError(t, err)

	var out any
	require.NoError(t, json.Unmarshal(raw, &out))

	return out
}

// TestGrantPolicyUpConversionKeepsTheReleasedMeaning: in v1alpha1 projectSelector was matched against
// namespace labels only, which is exactly what v1alpha2 calls namespaceSelector.
func TestGrantPolicyUpConversionKeepsTheReleasedMeaning(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		v1       map[string]any
		expected map[string]any
	}{
		{
			name:     "a selector moves to namespaceSelector",
			v1:       map[string]any{"projectSelector": selector("projects.deckhouse.io/project", "a"), "resources": []any{}},
			expected: map[string]any{"namespaceSelector": selector("projects.deckhouse.io/project", "a"), "resources": []any{}},
		},
		{
			name:     "an empty selector stays empty and still matches everything",
			v1:       map[string]any{"projectSelector": map[string]any{}, "resources": []any{}},
			expected: map[string]any{"namespaceSelector": map[string]any{}, "resources": []any{}},
		},
		{
			name:     "a library policy has no selector in either version",
			v1:       map[string]any{"resources": []any{}},
			expected: map[string]any{"resources": []any{}},
		},
		{
			name: "match expressions are kept as they are",
			v1: map[string]any{"projectSelector": map[string]any{"matchExpressions": []any{
				map[string]any{"key": "projects.deckhouse.io/project-namespace", "operator": "DoesNotExist"},
			}}, "resources": []any{}},
			expected: map[string]any{"namespaceSelector": map[string]any{"matchExpressions": []any{
				map[string]any{"key": "projects.deckhouse.io/project-namespace", "operator": "DoesNotExist"},
			}}, "resources": []any{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := convertWith(t, grantPolicyConversionHook, "v1alpha1_to_v1alpha2", grantPolicy("multitenancy.deckhouse.io/v1alpha1", tt.v1))

			assert.Equal(t, "multitenancy.deckhouse.io/v1alpha2", got["apiVersion"])
			assert.Equal(t, normalize(t, tt.expected), normalize(t, got["spec"]))
			assert.Nil(t, got["metadata"].(map[string]any)["annotations"], "no annotation is added on the way up")
		})
	}
}

// TestGrantPolicyRoundTrip: a v1alpha2 object served as v1alpha1 and written back is unchanged.
func TestGrantPolicyRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec map[string]any
	}{
		{name: "both selectors", spec: map[string]any{
			"projectSelector":   selector("team", "a"),
			"namespaceSelector": selector("projects.deckhouse.io/project-namespace", "cache"),
			"resources":         []any{},
		}},
		{name: "project selector only", spec: map[string]any{"projectSelector": selector("team", "a"), "resources": []any{}}},
		{name: "namespace selector only", spec: map[string]any{"namespaceSelector": selector("env", "prod"), "resources": []any{}}},
		{name: "empty project selector", spec: map[string]any{"projectSelector": map[string]any{}, "resources": []any{}}},
		{name: "a library policy", spec: map[string]any{"resources": []any{}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			original := grantPolicy("multitenancy.deckhouse.io/v1alpha2", tt.spec)
			down := convertWith(t, grantPolicyConversionHook, "v1alpha2_to_v1alpha1", original)
			assert.Equal(t, "multitenancy.deckhouse.io/v1alpha1", down["apiVersion"])
			assert.NotContains(t, down["spec"], "namespaceSelector", "v1alpha1 has no namespaceSelector")

			up := convertWith(t, grantPolicyConversionHook, "v1alpha1_to_v1alpha2", down)
			assert.Equal(t, normalize(t, original), normalize(t, up))
		})
	}
}

// TestGrantPolicyDownConversionShowsTheNamespaceSelector: a v1alpha1 client reads what v1alpha2
// calls namespaceSelector in the field it knows, and the project selector it cannot express in an
// annotation.
func TestGrantPolicyDownConversionShowsTheNamespaceSelector(t *testing.T) {
	t.Parallel()

	down := convertWith(t, grantPolicyConversionHook, "v1alpha2_to_v1alpha1", grantPolicy("multitenancy.deckhouse.io/v1alpha2", map[string]any{
		"projectSelector":   selector("team", "a"),
		"namespaceSelector": selector("env", "prod"),
		"resources":         []any{},
	}))

	assert.Equal(t, normalize(t, selector("env", "prod")), normalize(t, specField(down, "projectSelector")))

	annotations, _ := down["metadata"].(map[string]any)["annotations"].(map[string]any)
	require.Contains(t, annotations, grantPolicyProjectSelectorAnnotation)

	var kept any
	require.NoError(t, json.Unmarshal([]byte(annotations[grantPolicyProjectSelectorAnnotation].(string)), &kept))
	assert.Equal(t, normalize(t, selector("team", "a")), kept)
}

// TestGrantPolicyConversionKeepsOtherAnnotations: the conversion touches only its own annotation.
func TestGrantPolicyConversionKeepsOtherAnnotations(t *testing.T) {
	t.Parallel()

	object := grantPolicy("multitenancy.deckhouse.io/v1alpha2", map[string]any{"projectSelector": selector("team", "a"), "resources": []any{}})
	object["metadata"] = map[string]any{"name": "p", "annotations": map[string]any{"owner": "platform"}}

	up := convertWith(t, grantPolicyConversionHook, "v1alpha1_to_v1alpha2", convertWith(t, grantPolicyConversionHook, "v1alpha2_to_v1alpha1", object))

	assert.Equal(t, normalize(t, map[string]any{"owner": "platform"}), normalize(t, up["metadata"].(map[string]any)["annotations"]))
}

// TestGrantPolicyConversionSurvivesAMissingSpec: a spec-less object converts without failing the
// whole review.
func TestGrantPolicyConversionSurvivesAMissingSpec(t *testing.T) {
	t.Parallel()

	for _, function := range []string{"v1alpha1_to_v1alpha2", "v1alpha2_to_v1alpha1"} {
		object := map[string]any{"apiVersion": "multitenancy.deckhouse.io/v1alpha1", "kind": "ClusterResourceGrantPolicy", "metadata": map[string]any{"name": "p"}}
		if function == "v1alpha2_to_v1alpha1" {
			object["apiVersion"] = "multitenancy.deckhouse.io/v1alpha2"
		}

		got := convertWith(t, grantPolicyConversionHook, function, object)
		assert.NotContains(t, got, "spec", function)
	}
}

// conversionError runs one conversion function of the hook and returns the error its jq program
// raises, or nil when it converts.
func conversionError(t *testing.T, function string, object map[string]any) error {
	t.Helper()

	query, err := gojq.Parse(jqProgram(t, grantPolicyConversionHook, function))
	require.NoError(t, err)

	review := map[string]any{"review": map[string]any{"request": map[string]any{"objects": []any{object}}}}
	result, ok := query.Run(review).Next()
	require.True(t, ok, "%s produced nothing", function)
	if err, isErr := result.(error); isErr {
		return err
	}

	return nil
}

func withProjectSelectorAnnotation(object map[string]any, value string) map[string]any {
	object["metadata"] = map[string]any{"name": "p", "annotations": map[string]any{grantPolicyProjectSelectorAnnotation: value}}

	return object
}

// TestGrantPolicyUpConversionRefusesAnAnnotationThatIsNotASelector: the API server validates a
// v1alpha1 write against the v1alpha1 schema, where the annotation is a string, and stores whatever
// the conversion returns. A projectSelector of the wrong shape would make every typed list of the
// policies fail to decode, so the conversion refuses the write instead.
func TestGrantPolicyUpConversionRefusesAnAnnotationThatIsNotASelector(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		`{"matchLabels":{"tier":1}}`,
		`42`,
		`"prod"`,
		`[]`,
		`{"foo":"bar"}`,
		`{"matchLabels":{"tier":"gold"},"extra":true}`,
		`{"matchExpressions":[{"key":"a","operator":"In","values":"x"}]}`,
		`{"matchExpressions":[{"key":"a","operator":"Gt","values":["1"]}]}`,
		`{"matchExpressions":[{"key":1,"operator":"Exists"}]}`,
		`{"matchExpressions":{"key":"a","operator":"Exists"}}`,
		`{"matchLabels":false}`,
		`{"matchExpressions":false}`,
		`{"matchExpressions":[{"key":"a","operator":"Exists","values":false}]}`,
		`{"matchLabels":{"tier":false}}`,
		``,
		`not json`,
	} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			object := withProjectSelectorAnnotation(grantPolicy("multitenancy.deckhouse.io/v1alpha1", map[string]any{"resources": []any{}}), value)
			assert.Error(t, conversionError(t, "v1alpha1_to_v1alpha2", object))
		})
	}
}

// TestGrantPolicyUpConversionAcceptsEverySelectorShape: matchExpressions and an empty selector pass
// through the annotation like matchLabels do.
func TestGrantPolicyUpConversionAcceptsEverySelectorShape(t *testing.T) {
	t.Parallel()

	for _, kept := range []map[string]any{
		{},
		{"matchLabels": map[string]any{}},
		{"matchExpressions": []any{
			map[string]any{"key": "team", "operator": "In", "values": []any{"a", "b"}},
			map[string]any{"key": "tier", "operator": "Exists"},
		}},
		{"matchLabels": map[string]any{"team": "a"}, "matchExpressions": []any{map[string]any{"key": "tier", "operator": "DoesNotExist", "values": []any{}}}},
		{"matchLabels": nil, "matchExpressions": []any{map[string]any{"key": "tier", "operator": "Exists", "values": nil}}},
	} {
		raw, err := json.Marshal(kept)
		require.NoError(t, err)

		t.Run(string(raw), func(t *testing.T) {
			t.Parallel()

			object := withProjectSelectorAnnotation(grantPolicy("multitenancy.deckhouse.io/v1alpha1", map[string]any{"resources": []any{}}), string(raw))
			up := convertWith(t, grantPolicyConversionHook, "v1alpha1_to_v1alpha2", object)
			assert.Equal(t, normalize(t, kept), normalize(t, specField(up, "projectSelector")))
		})
	}
}

// TestGrantPolicyDownConversionDropsAStaleAnnotation: the annotation a v1alpha1 client sees is always
// derived from spec.projectSelector. A copy written through v1alpha2 (a v1alpha1 view turned into a
// v1alpha2 manifest) does not survive, so a later v1alpha1 write cannot bring it back as a selector.
func TestGrantPolicyDownConversionDropsAStaleAnnotation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec map[string]any
	}{
		{name: "a library policy", spec: map[string]any{"resources": []any{}}},
		{name: "namespace selector only", spec: map[string]any{"namespaceSelector": selector("env", "prod"), "resources": []any{}}},
		{name: "a different project selector", spec: map[string]any{"projectSelector": selector("team", "a"), "resources": []any{}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stale := withProjectSelectorAnnotation(grantPolicy("multitenancy.deckhouse.io/v1alpha2", tt.spec), `{}`)
			up := convertWith(t, grantPolicyConversionHook, "v1alpha1_to_v1alpha2", convertWith(t, grantPolicyConversionHook, "v1alpha2_to_v1alpha1", stale))

			assert.Equal(t, normalize(t, tt.spec), normalize(t, up["spec"]))
			assert.Nil(t, up["metadata"].(map[string]any)["annotations"])
		})
	}
}
