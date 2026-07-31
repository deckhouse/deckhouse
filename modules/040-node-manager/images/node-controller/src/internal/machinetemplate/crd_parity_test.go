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

package machinetemplate

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sigsyaml "sigs.k8s.io/yaml"
)

// The hand-written fixtures in provider_parity_test.go only cover the fields somebody thought to
// put in them. This harness takes the field list from the provider's own InstanceClass CRD
// instead, so a field nobody remembered — and therefore nobody put in rolloutFields — still gets
// asked the only question that matters:
//
//	does changing it roll machines under v2 exactly when it rolled them under v1?
//
// A field the provider forgot in rolloutFields fails here as "v1 rolls, v2 does not" (a change the
// user asks for silently not reaching the machines), and a field listed by mistake fails as "v2
// rolls, v1 does not" (a rollout the user did not ask for). Both are the failure modes this whole
// migration exists to avoid.

type crdField struct {
	path string
	kind string
}

func TestProviderRolloutParityAgainstCRD(t *testing.T) {
	for _, fixture := range providerFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			contract := loadContract(t, fixture.contractPath)
			checksumTemplate, err := os.ReadFile(fixture.legacyPath("instance-class.checksum"))
			require.NoError(t, err)

			fields := crdSpecFields(t, fixture.crdPath)
			require.NotEmpty(t, fields, "the provider CRD must describe its InstanceClass spec")

			baseChecksum := renderLegacyChecksum(t, fixture, checksumTemplate, fixture.instanceClass, "")

			for _, field := range fields {
				t.Run(field.path, func(t *testing.T) {
					mutated := deepCopySpec(t, fixture.instanceClass)
					setPath(mutated, field.path, crdSampleValue(field.kind))

					v1Rolls := renderLegacyChecksum(t, fixture, checksumTemplate, mutated, "") != baseChecksum

					changes, err := Changes(fixture.instanceClass, mutated, contract.RolloutFields)
					require.NoError(t, err)
					v2Rolls := len(changes) > 0

					assert.Equal(t, v1Rolls, v2Rolls,
						"changing %s (declared in the provider CRD): v1 checksum rolls=%v, rolloutFields roll=%v — "+
							"either the field is missing from rolloutFields, or it is listed there while v1 ignored it",
						field.path, v1Rolls, v2Rolls)
				})
			}
		})
	}
}

// TestProviderRenderParityOnEdgeSpecs runs both engines on the InstanceClass shapes a fixture
// never has: only the CRD-required fields, and every optional field set to its zero value.
//
// This is where a mechanical migration is most likely to have drifted. The v1 templates gate
// optional fields on truthiness (`{{- if .nodeGroup.instanceClass.rootDiskSize }}`), while the v2
// ones must use `get`/`hasKey` to survive missingkey=error — and `hasKey` says yes for a field that
// is present and zero, where truthiness says no. A user with `rootDiskSize: 0` or an empty list in
// their InstanceClass would then get a different object out of the two engines.
func TestProviderRenderParityOnEdgeSpecs(t *testing.T) {
	for _, fixture := range providerFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			contract := loadContract(t, fixture.contractPath)
			fields := crdSpecFields(t, fixture.crdPath)
			required := crdRequiredPaths(t, fixture.crdPath)

			specs := map[string]map[string]any{
				"only required fields":     minimalSpec(t, fixture.instanceClass, required),
				"optional fields zeroed":   zeroedSpec(t, fixture.instanceClass, fields, required),
				"optional fields absent":   minimalSpec(t, fixture.instanceClass, required),
				"fixture with empty lists": emptyListSpec(t, fixture.instanceClass),
			}

			for name, spec := range specs {
				t.Run(name, func(t *testing.T) {
					v1Object, v1Err := renderLegacySpec(t, fixture, spec)
					v2Object, v2Err := renderV2Spec(fixture, contract, spec)

					if v1Err != nil {
						require.Error(t, v2Err,
							"v1 refuses this InstanceClass but v2 renders it — the migration changed what is accepted")
						return
					}
					require.NoError(t, v2Err,
						"v1 renders this InstanceClass but v2 refuses it — the migration changed what is accepted")
					assert.Equal(t, v1Object, v2Object,
						"the two engines disagree on an edge-case InstanceClass")
				})
			}
		})
	}
}

// TestProviderTemplateMatchesRegistrationGVK guards the one place the contract states the same
// fact twice: the apiVersion/kind literal at the top of the provider's template, and
// capiMachineTemplateKind/capiMachineTemplateAPIVersion in the provider's registration secret.
//
// node-controller compares them at generation time and refuses to create an object when they
// disagree — which means a mismatch shipped in a release stops machines from being created at all,
// and nothing before this test would have caught it.
func TestProviderTemplateMatchesRegistrationGVK(t *testing.T) {
	for _, fixture := range providerFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			contract := loadContract(t, fixture.contractPath)
			rendered, err := renderV2Spec(fixture, contract, fixture.instanceClass)
			require.NoError(t, err)

			registration, err := os.ReadFile(fixture.registrationPath)
			require.NoError(t, err)

			assert.Equal(t, registrationValue(t, registration, "capiMachineTemplateKind"), rendered["kind"],
				"the template renders a different kind than the registration secret declares")
			assert.Equal(t, registrationValue(t, registration, "capiMachineTemplateAPIVersion"), rendered["apiVersion"],
				"the template renders a different apiVersion than the registration secret declares")
		})
	}
}

// registrationValue pulls a literal out of `key: {{ b64enc "value" | quote }}`, the shape every
// provider's registration template uses for these two fields.
func registrationValue(t *testing.T, registration []byte, key string) string {
	t.Helper()
	pattern := regexp.MustCompile(key + `:\s*{{\s*b64enc\s+"([^"]+)"`)
	match := pattern.FindSubmatch(registration)
	require.NotNil(t, match, "%s must be declared as a literal in the registration template", key)
	return string(match[1])
}

// crdRequiredPaths collects the dot-paths the CRD marks required, at every level.
func crdRequiredPaths(t *testing.T, path string) map[string]struct{} {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	crd := map[string]any{}
	require.NoError(t, sigsyaml.Unmarshal(data, &crd))

	versions, _, err := nestedSlice(crd, "spec", "versions")
	require.NoError(t, err)

	out := map[string]struct{}{}
	for _, version := range versions {
		versionMap, ok := version.(map[string]any)
		if !ok {
			continue
		}
		spec, found, err := nestedMap(versionMap, "schema", "openAPIV3Schema", "properties", "spec")
		require.NoError(t, err)
		if !found {
			continue
		}
		collectRequired(spec, "", out)
	}
	return out
}

func collectRequired(schema map[string]any, prefix string, out map[string]struct{}) {
	if list, ok := schema["required"].([]any); ok {
		for _, raw := range list {
			name, ok := raw.(string)
			if !ok {
				continue
			}
			path := name
			if prefix != "" {
				path = prefix + "." + name
			}
			out[path] = struct{}{}
		}
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return
	}
	for name, raw := range properties {
		property, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		collectRequired(property, path, out)
	}
}

// minimalSpec keeps only the values the CRD requires (a nested map survives when anything under it
// does), which is the smallest InstanceClass a user can actually create.
func minimalSpec(t *testing.T, spec map[string]any, required map[string]struct{}) map[string]any {
	t.Helper()
	out := map[string]any{}
	for key, value := range deepCopySpec(t, spec) {
		keep(out, key, key, value, required)
	}
	return out
}

func keep(out map[string]any, path, key string, value any, required map[string]struct{}) {
	nested, isMap := value.(map[string]any)
	if !isMap {
		if _, ok := required[path]; ok {
			out[key] = value
		}
		return
	}

	kept := map[string]any{}
	for nestedKey, nestedValue := range nested {
		keep(kept, path+"."+nestedKey, nestedKey, nestedValue, required)
	}
	if len(kept) > 0 {
		out[key] = kept
	}
}

// zeroedSpec sets every optional CRD field to the zero value of its declared type — the shape a
// user creates by writing `rootDiskSize: 0` or `additionalTags: {}`.
func zeroedSpec(t *testing.T, spec map[string]any, fields []crdField, required map[string]struct{}) map[string]any {
	t.Helper()
	out := deepCopySpec(t, spec)
	for _, field := range fields {
		if _, isRequired := required[field.path]; isRequired {
			continue
		}
		if _, present := fieldPresent(out, field.path); !present {
			continue
		}
		setPath(out, field.path, crdZeroValue(field.kind))
	}
	return out
}

// emptyListSpec replaces every list in the fixture with an empty one: v1 skipped an empty list by
// truthiness, and a v2 template that switched to hasKey would render an empty block instead.
func emptyListSpec(t *testing.T, spec map[string]any) map[string]any {
	t.Helper()
	out := deepCopySpec(t, spec)
	emptyLists(out)
	return out
}

func emptyLists(spec map[string]any) {
	for key, value := range spec {
		switch typed := value.(type) {
		case []any:
			spec[key] = []any{}
		case map[string]any:
			emptyLists(typed)
		}
	}
}

func fieldPresent(spec map[string]any, path string) (any, bool) {
	current := any(spec)
	for _, segment := range strings.Split(path, ".") {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[segment]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func crdZeroValue(kind string) any {
	switch kind {
	case "integer", "number":
		return float64(0)
	case "boolean":
		return false
	case "array":
		return []any{}
	case "object":
		return map[string]any{}
	default:
		return ""
	}
}

// crdSpecFields reads every leaf of .spec out of the provider's InstanceClass CRD. Leaves are
// scalars, arrays and free-form objects; nested objects are walked into, because that is how the
// rolloutFields dot-paths address them (DVP's "virtualMachine.cpu.cores").
func crdSpecFields(t *testing.T, path string) []crdField {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err, "provider InstanceClass CRD must exist")

	crd := map[string]any{}
	require.NoError(t, sigsyaml.Unmarshal(data, &crd))

	versions, _, err := nestedSlice(crd, "spec", "versions")
	require.NoError(t, err)
	require.NotEmpty(t, versions)

	fields := map[string]string{}
	for _, version := range versions {
		versionMap, ok := version.(map[string]any)
		if !ok {
			continue
		}
		spec, found, err := nestedMap(versionMap, "schema", "openAPIV3Schema", "properties", "spec")
		require.NoError(t, err)
		if !found {
			continue
		}
		collectCRDFields(spec, "", fields)
	}

	out := make([]crdField, 0, len(fields))
	for path, kind := range fields {
		out = append(out, crdField{path: path, kind: kind})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out
}

func collectCRDFields(schema map[string]any, prefix string, out map[string]string) {
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return
	}
	for name, raw := range properties {
		property, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		kind, _ := property["type"].(string)
		if kind == "object" {
			if _, hasProperties := property["properties"]; hasProperties {
				collectCRDFields(property, path, out)
				continue
			}
		}
		out[path] = kind
	}
}

// crdSampleValue returns a value of the declared type that differs from anything the fixtures use,
// so setting it is always a real change.
func crdSampleValue(kind string) any {
	switch kind {
	case "integer", "number":
		return float64(4242)
	case "boolean":
		return true
	case "array":
		return []any{"crd-parity"}
	case "object":
		return map[string]any{"crd-parity": "value"}
	default:
		return "crd-parity"
	}
}

func setPath(spec map[string]any, path string, value any) {
	segments := strings.Split(path, ".")
	current := spec
	for _, segment := range segments[:len(segments)-1] {
		next, ok := current[segment].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[segment] = next
		}
		current = next
	}
	current[segments[len(segments)-1]] = value
}

func nestedMap(obj map[string]any, keys ...string) (map[string]any, bool, error) {
	value, found, err := nestedValue(obj, keys...)
	if err != nil || !found {
		return nil, found, err
	}
	out, ok := value.(map[string]any)
	if !ok {
		return nil, false, fmt.Errorf("%s is %T, not a map", strings.Join(keys, "."), value)
	}
	return out, true, nil
}

func nestedSlice(obj map[string]any, keys ...string) ([]any, bool, error) {
	value, found, err := nestedValue(obj, keys...)
	if err != nil || !found {
		return nil, found, err
	}
	out, ok := value.([]any)
	if !ok {
		return nil, false, fmt.Errorf("%s is %T, not a list", strings.Join(keys, "."), value)
	}
	return out, true, nil
}

func nestedValue(obj map[string]any, keys ...string) (any, bool, error) {
	current := any(obj)
	for _, key := range keys {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("%s is not a map", key)
		}
		current, ok = m[key]
		if !ok {
			return nil, false, nil
		}
	}
	return current, true, nil
}
