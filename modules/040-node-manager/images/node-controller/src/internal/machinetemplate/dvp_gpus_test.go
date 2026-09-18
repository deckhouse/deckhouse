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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDVPRendersGPUs covers the one DVP field the parity harness cannot reach: virtualMachine.gpus
// appeared after the v1 checksum was frozen, so it may not enter the parity fixture and its
// mutations are listed in rolloutExceptions — nothing else renders it.
//
// The list is atomic and duplicates are meaningful ("two cards of the same GPUClass"), so a
// template that deduplicated, sorted or reordered it would silently hand the VM the wrong number of
// devices. The absent case is just as load-bearing: emitting an empty `gpus: []` for a GPU-less
// InstanceClass would change the manifest of every existing machine and roll the whole fleet.
func TestDVPRendersGPUs(t *testing.T) {
	fixture := fixtureByName(t, "dvp")
	contract := loadContract(t, fixture.contractPath)

	tests := []struct {
		name     string
		gpus     []any
		expected []any
	}{
		{
			name: "two devices of the same GPUClass",
			gpus: []any{
				map[string]any{"gpuClassName": "nvidia-h100"},
				map[string]any{"gpuClassName": "nvidia-h100"},
			},
			expected: []any{
				map[string]any{"gpuClassName": "nvidia-h100"},
				map[string]any{"gpuClassName": "nvidia-h100"},
			},
		},
		{
			name: "two devices of different GPUClasses keep their order",
			gpus: []any{
				map[string]any{"gpuClassName": "nvidia-h100"},
				map[string]any{"gpuClassName": "nvidia-t4"},
			},
			expected: []any{
				map[string]any{"gpuClassName": "nvidia-h100"},
				map[string]any{"gpuClassName": "nvidia-t4"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			instanceClass := deepCopySpec(t, fixture.instanceClass)
			instanceClass["virtualMachine"].(map[string]any)["gpus"] = tc.gpus

			obj, err := renderV2Spec(fixture, contract, instanceClass)
			require.NoError(t, err)

			assert.Equal(t, tc.expected, machineSpec(t, obj)["gpus"])
		})
	}

	t.Run("no gpus in the InstanceClass leaves the key out", func(t *testing.T) {
		instanceClass := deepCopySpec(t, fixture.instanceClass)
		require.NotContains(t, instanceClass["virtualMachine"], "gpus",
			"the parity fixture must stay GPU-free: TestProviderRenderParity compares it against the frozen v1 template")

		obj, err := renderV2Spec(fixture, contract, instanceClass)
		require.NoError(t, err)

		assert.NotContains(t, machineSpec(t, obj), "gpus",
			"a GPU-less InstanceClass must render no gpus key at all: an empty list is a different manifest and would roll every existing machine")
	})
}

// machineSpec returns spec.template.spec of a rendered machine template.
func machineSpec(t *testing.T, obj map[string]any) map[string]any {
	t.Helper()
	template, ok := obj["spec"].(map[string]any)["template"].(map[string]any)
	require.True(t, ok, "rendered machine template has no spec.template")
	spec, ok := template["spec"].(map[string]any)
	require.True(t, ok, "rendered machine template has no spec.template.spec")
	return spec
}
