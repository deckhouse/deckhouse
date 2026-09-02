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
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sigsyaml "sigs.k8s.io/yaml"
)

// The core fraction of a DVP node travels as an opaque string: the DVPInstanceClass carries it,
// this template copies it into the DeckhouseMachineTemplate, and the CAPI provider hands it to the
// VirtualMachine untouched. So "Auto" reaching the machine template is not a given — the template
// could quote or coerce it, and the CRD of the machine template has its own pattern that must
// accept the value. Either failure means a node group with `coreFraction: Auto` never gets a
// machine, and it would only show up on a live cluster.
func TestDVPCoreFractionValuesReachMachineTemplate(t *testing.T) {
	fixture := dvpFixture(t)
	contract := loadContract(t, fixture.contractPath)
	pattern := crdFieldPattern(t,
		"../../../../../../030-cloud-provider-dvp/crds/external/deckhousemachinetemplates.yaml",
		"spec", "template", "spec", "cpu", "cpuFraction")

	for _, coreFraction := range []string{"Auto", "20%", "100%"} {
		t.Run(coreFraction, func(t *testing.T) {
			instanceClass := deepCopySpec(t, fixture.instanceClass)
			setPath(instanceClass, "virtualMachine.cpu.coreFraction", coreFraction)

			object, err := renderV2Spec(fixture, contract, instanceClass)
			require.NoError(t, err)

			rendered, found, err := nestedValue(object, "spec", "template", "spec", "cpu", "cpuFraction")
			require.NoError(t, err)
			require.True(t, found, "the template must render cpuFraction whenever the instance class sets it")
			assert.Equal(t, coreFraction, rendered, "the core fraction must reach the machine template verbatim")

			assert.Regexp(t, pattern, coreFraction,
				"the DeckhouseMachineTemplate CRD must accept the value the template renders")
		})
	}
}

// dvpFixture returns the shipped DVP fixture, failing loudly if the provider is ever renamed or
// dropped from the parity harness — silently skipping would leave the path above untested.
func dvpFixture(t *testing.T) providerFixture {
	t.Helper()

	for _, fixture := range providerFixtures() {
		if fixture.name == "dvp" {
			return fixture
		}
	}
	t.Fatal("the dvp fixture is gone from providerFixtures()")
	return providerFixture{}
}

// crdFieldPattern digs the pattern of one field out of a CRD, so the test asserts against the
// schema the cluster really enforces instead of a copy of it.
func crdFieldPattern(t *testing.T, crdPath string, path ...string) *regexp.Regexp {
	t.Helper()

	raw, err := os.ReadFile(crdPath)
	require.NoError(t, err)

	var crd map[string]any
	require.NoError(t, sigsyaml.Unmarshal(raw, &crd))

	versions, found, err := nestedSlice(crd, "spec", "versions")
	require.NoError(t, err)
	require.True(t, found, "the CRD must declare versions")
	require.NotEmpty(t, versions)

	version, ok := versions[0].(map[string]any)
	require.True(t, ok)

	keys := append([]string{"schema", "openAPIV3Schema"}, propertyPath(path)...)
	field, found, err := nestedMap(version, keys...)
	require.NoError(t, err)
	require.True(t, found, "the CRD must describe %v", path)

	pattern, ok := field["pattern"].(string)
	require.True(t, ok, "the field %v must be constrained by a pattern", path)

	return regexp.MustCompile(pattern)
}

// propertyPath turns a field path into the "properties" chain an OpenAPI schema is nested by.
func propertyPath(path []string) []string {
	keys := make([]string, 0, len(path)*2)
	for _, segment := range path {
		keys = append(keys, "properties", segment)
	}
	return keys
}
