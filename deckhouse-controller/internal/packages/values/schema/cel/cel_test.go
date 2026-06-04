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

package cel

import (
	"encoding/json"
	"testing"

	"github.com/go-openapi/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// schemaFromYAML is a small test helper that mirrors how schemas are loaded
// in production (YAML -> JSON -> spec.Schema) so that x-deckhouse-validations
// extensions are decoded the same way as real module schemas.
func schemaFromYAML(t *testing.T, src string) *spec.Schema {
	t.Helper()

	jsonDoc, err := yaml.YAMLToJSON([]byte(src))
	require.NoError(t, err, "yaml to json")

	s := new(spec.Schema)
	require.NoError(t, json.Unmarshal(jsonDoc, s), "unmarshal schema")

	return s
}

// valuesFromYAML decodes a YAML object into a generic map suitable for
// passing as values / oldValues to ValidateTransition.
func valuesFromYAML(t *testing.T, src string) map[string]any {
	t.Helper()

	jsonDoc, err := yaml.YAMLToJSON([]byte(src))
	require.NoError(t, err, "yaml to json")

	var v map[string]any
	require.NoError(t, json.Unmarshal(jsonDoc, &v), "unmarshal values")

	return v
}

// joinMessages turns a slice of validation errors into a single string for
// concise assertions.
func joinMessages(errs []error) []string {
	out := make([]string, 0, len(errs))
	for _, e := range errs {
		out = append(out, e.Error())
	}
	return out
}

func TestValidate_NoRules(t *testing.T) {
	s := schemaFromYAML(t, `
type: object
properties:
  foo:
    type: string
`)
	values := valuesFromYAML(t, `foo: bar`)

	errs, err := Validate(s, values)
	require.NoError(t, err)
	assert.Empty(t, errs)
}

func TestValidate_PlainSelfRulePass(t *testing.T) {
	s := schemaFromYAML(t, `
type: object
properties:
  replicas:
    type: integer
x-deckhouse-validations:
  - expression: "self.replicas >= 1"
    message: "replicas must be >= 1"
`)
	values := valuesFromYAML(t, `replicas: 3`)

	errs, err := Validate(s, values)
	require.NoError(t, err)
	assert.Empty(t, errs)
}

func TestValidate_PlainSelfRuleFail(t *testing.T) {
	s := schemaFromYAML(t, `
type: object
properties:
  replicas:
    type: integer
x-deckhouse-validations:
  - expression: "self.replicas >= 1"
    message: "replicas must be >= 1"
`)
	values := valuesFromYAML(t, `replicas: 0`)

	errs, err := Validate(s, values)
	require.NoError(t, err)
	require.Len(t, errs, 1)
	assert.Equal(t, "replicas must be >= 1", errs[0].Error())
}

// ValidateTransition tests below cover the new oldSelf-aware behaviour.

func TestValidateTransition_OldSelfSkippedWhenNoOldValues(t *testing.T) {
	// A rule referencing oldSelf is a transition rule and must be skipped
	// when no previous values are provided (e.g. on initial create).
	s := schemaFromYAML(t, `
type: object
properties:
  clusterDomain:
    type: string
x-deckhouse-validations:
  - expression: "self.clusterDomain == oldSelf.clusterDomain"
    message: "clusterDomain is immutable"
`)
	values := valuesFromYAML(t, `clusterDomain: cluster.local`)

	errs, err := ValidateTransition(s, values, nil)
	require.NoError(t, err)
	assert.Empty(t, errs, "transition rule should be skipped without oldValues")
}

func TestValidateTransition_ImmutableTopLevelFieldUnchanged(t *testing.T) {
	s := schemaFromYAML(t, `
type: object
properties:
  clusterDomain:
    type: string
x-deckhouse-validations:
  - expression: "self.clusterDomain == oldSelf.clusterDomain"
    message: "clusterDomain is immutable"
`)
	values := valuesFromYAML(t, `clusterDomain: cluster.local`)
	oldValues := valuesFromYAML(t, `clusterDomain: cluster.local`)

	errs, err := ValidateTransition(s, values, oldValues)
	require.NoError(t, err)
	assert.Empty(t, errs)
}

func TestValidateTransition_ImmutableTopLevelFieldChanged(t *testing.T) {
	s := schemaFromYAML(t, `
type: object
properties:
  clusterDomain:
    type: string
x-deckhouse-validations:
  - expression: "self.clusterDomain == oldSelf.clusterDomain"
    message: "clusterDomain is immutable"
`)
	values := valuesFromYAML(t, `clusterDomain: cluster.local`)
	oldValues := valuesFromYAML(t, `clusterDomain: example.com`)

	errs, err := ValidateTransition(s, values, oldValues)
	require.NoError(t, err)
	require.Len(t, errs, 1)
	assert.Equal(t, "clusterDomain is immutable", errs[0].Error())
}

func TestValidateTransition_NestedImmutableField(t *testing.T) {
	// Transition rules must work at any depth - oldSelf is rebound to the
	// matching subtree of the previous values during recursion.
	s := schemaFromYAML(t, `
type: object
properties:
  registry:
    type: object
    properties:
      mode:
        type: string
    x-deckhouse-validations:
      - expression: "self.mode == oldSelf.mode"
        message: "registry.mode is immutable"
`)
	values := valuesFromYAML(t, `
registry:
  mode: Direct
`)
	oldValues := valuesFromYAML(t, `
registry:
  mode: Proxy
`)

	errs, err := ValidateTransition(s, values, oldValues)
	require.NoError(t, err)
	require.Len(t, errs, 1)
	assert.Equal(t, "registry.mode is immutable", errs[0].Error())
}

func TestValidateTransition_NestedTransitionSkippedForNewlyAddedSubtree(t *testing.T) {
	// When a nested object did not exist in the previous values, transition
	// rules at that level must be skipped (it's effectively a create for
	// that subtree), not fired against an undefined oldSelf.
	s := schemaFromYAML(t, `
type: object
properties:
  registry:
    type: object
    properties:
      mode:
        type: string
    x-deckhouse-validations:
      - expression: "self.mode == oldSelf.mode"
        message: "registry.mode is immutable"
`)
	values := valuesFromYAML(t, `
registry:
  mode: Direct
`)
	oldValues := valuesFromYAML(t, `{}`)

	errs, err := ValidateTransition(s, values, oldValues)
	require.NoError(t, err)
	assert.Empty(t, errs, "transition rule should be skipped when subtree is new")
}

func TestValidateTransition_NonTransitionRuleStillEnforcedOnUpdate(t *testing.T) {
	// Mixing plain rules and transition rules: plain ones still fire, even
	// when oldValues are provided.
	s := schemaFromYAML(t, `
type: object
properties:
  replicas:
    type: integer
  mode:
    type: string
x-deckhouse-validations:
  - expression: "self.replicas >= 1"
    message: "replicas must be >= 1"
  - expression: "self.mode == oldSelf.mode"
    message: "mode is immutable"
`)
	values := valuesFromYAML(t, `
replicas: 0
mode: Proxy
`)
	oldValues := valuesFromYAML(t, `
replicas: 5
mode: Direct
`)

	errs, err := ValidateTransition(s, values, oldValues)
	require.NoError(t, err)
	msgs := joinMessages(errs)
	assert.ElementsMatch(t, []string{
		"replicas must be >= 1",
		"mode is immutable",
	}, msgs)
}

func TestValidateTransition_OldSelfInComprehension(t *testing.T) {
	// Make sure oldSelf is detected as used even when it appears inside a
	// CEL macro/comprehension, not as a top-level identifier.
	s := schemaFromYAML(t, `
type: object
properties:
  items:
    type: array
    items:
      type: string
x-deckhouse-validations:
  - expression: "self.items.all(x, x in oldSelf.items)"
    message: "items can only be appended to"
`)
	values := valuesFromYAML(t, `
items: [a, b, c]
`)
	oldValues := valuesFromYAML(t, `
items: [a, b]
`)

	errs, err := ValidateTransition(s, values, oldValues)
	require.NoError(t, err)
	require.Len(t, errs, 1)
	assert.Equal(t, "items can only be appended to", errs[0].Error())
}

func TestValidateTransition_OldSelfMacroSkippedWithoutOld(t *testing.T) {
	// Same expression, but no old value: it is still recognised as a
	// transition rule (oldSelf appears in the AST) and skipped.
	s := schemaFromYAML(t, `
type: object
properties:
  items:
    type: array
    items:
      type: string
x-deckhouse-validations:
  - expression: "self.items.all(x, x in oldSelf.items)"
    message: "items can only be appended to"
`)
	values := valuesFromYAML(t, `
items: [a, b, c]
`)

	errs, err := ValidateTransition(s, values, nil)
	require.NoError(t, err)
	assert.Empty(t, errs)
}

func TestValidate_BackwardCompatibilityWithoutOldSelf(t *testing.T) {
	// The plain Validate wrapper must keep working for schemas that don't
	// know about oldSelf at all.
	s := schemaFromYAML(t, `
type: object
properties:
  minReplicas:
    type: integer
  replicas:
    type: integer
  maxReplicas:
    type: integer
x-deckhouse-validations:
  - expression: "self.minReplicas <= self.replicas && self.replicas <= self.maxReplicas"
    message: "replicas must be between minReplicas and maxReplicas"
`)
	values := valuesFromYAML(t, `
minReplicas: 1
replicas: 10
maxReplicas: 5
`)

	errs, err := Validate(s, values)
	require.NoError(t, err)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "replicas must be between")
}

func TestValidate_InvalidExtensionShape(t *testing.T) {
	// x-deckhouse-validations must be a list of {expression, message}
	// objects; anything else is a configuration error.
	s := schemaFromYAML(t, `
type: object
x-deckhouse-validations: "not a list"
`)
	values := valuesFromYAML(t, `{}`)

	_, err := Validate(s, values)
	assert.Error(t, err)
}

func TestValidate_MissingExpression(t *testing.T) {
	s := schemaFromYAML(t, `
type: object
x-deckhouse-validations:
  - message: "no expression"
`)
	values := valuesFromYAML(t, `{}`)

	_, err := Validate(s, values)
	assert.Error(t, err)
}
