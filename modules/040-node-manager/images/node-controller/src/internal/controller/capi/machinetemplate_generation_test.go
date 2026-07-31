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

package capi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/node-controller/internal/machinetemplate"
)

// parseGeneration decides the number of the next generation, so a wrong answer here either
// re-uses a live generation's name (Create fails, or worse, the object is reused with the wrong
// spec) or skips numbers for no reason.
func TestParseGeneration(t *testing.T) {
	tests := []struct {
		name   string
		object string
		expGen int
	}{
		{name: "generation name", object: "worker-a1b2c3d4-gen4", expGen: 4},
		{name: "first generation", object: "worker-a1b2c3d4-gen1", expGen: 1},
		{name: "v1 checksum name has no generation", object: "worker-8ad9c341", expGen: 0},
		{name: "node group named like a generation", object: "gen5-a1b2c3d4-gen2", expGen: 2},
		{name: "trailing garbage is not a generation", object: "worker-a1b2c3d4-genX", expGen: 0},
		{name: "empty suffix", object: "worker-a1b2c3d4-gen", expGen: 0},
		{name: "negative number", object: "worker-a1b2c3d4-gen-2", expGen: 0},
		{name: "empty name", object: "", expGen: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expGen, parseGeneration(tc.object))
		})
	}
}

func TestMachineTemplateSnapshotRoundTrip(t *testing.T) {
	spec := map[string]any{
		"vmClassName": "generic",
		"rootDisk":    map[string]any{"size": "50Gi"},
	}

	annotations, err := machineTemplateSnapshot(spec, "2026-07-31")
	require.NoError(t, err)

	obj := &unstructured.Unstructured{}
	obj.SetAnnotations(annotations)

	stored, rolloutID, ok := readMachineTemplateSnapshot(obj)
	require.True(t, ok)
	assert.Equal(t, "2026-07-31", rolloutID)

	changes, err := machinetemplate.Changes(stored, spec, []string{"vmClassName", "rootDisk.size"})
	require.NoError(t, err)
	assert.Empty(t, changes, "a snapshot read back must compare equal to what was stored")
}

// A snapshot that cannot be parsed must not be read as "everything changed": that would recreate
// the template and roll the machines because of a corrupted annotation.
func TestUnreadableSnapshotIsTreatedAsAbsent(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAnnotations(map[string]string{appliedInstanceClassAnnotation: "{not json"})

	_, _, ok := readMachineTemplateSnapshot(obj)
	assert.False(t, ok, "an unparsable snapshot must fall back to re-adoption, not to a rollout")
}

func TestApplyMachineDeploymentAdditionalFields(t *testing.T) {
	contract, err := machinetemplate.ParseContract([]byte(`version: v2
rolloutFields: [flavorName]
machineDeployment:
  additionalFields:
    failureDomain: zone
template: |
  apiVersion: v1
  kind: X
  spec: {}
`))
	require.NoError(t, err)

	spec := map[string]interface{}{
		"template": map[string]interface{}{
			"spec": map[string]interface{}{"clusterName": "openstack"},
		},
	}
	require.NoError(t, applyMachineDeploymentAdditionalFields(spec, contract, "ru-1a"))

	got, found, err := unstructured.NestedString(spec, "template", "spec", "failureDomain")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "ru-1a", got)
	assert.Equal(t, "openstack", spec["template"].(map[string]interface{})["spec"].(map[string]interface{})["clusterName"],
		"existing fields must survive")
}

func TestApplyMachineDeploymentAdditionalFieldsNoop(t *testing.T) {
	contract, err := machinetemplate.ParseContract([]byte("version: v2\nrolloutFields: [a]\ntemplate: |\n  kind: X\n"))
	require.NoError(t, err)

	spec := map[string]interface{}{"replicas": int64(1)}
	require.NoError(t, applyMachineDeploymentAdditionalFields(spec, contract, "ru-1a"))
	assert.Equal(t, map[string]interface{}{"replicas": int64(1)}, spec)
}
