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

package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
)

func Test_virtualImageManifest(t *testing.T) {
	manifest := virtualImageManifest(
		"agent-01",
		"test-ns",
		VirtualizationCreationProbeName,
		"alpine-3-23-bios-base",
		"registry.example.com/upmeter-vm@sha256:abc",
	)

	var obj map[string]interface{}
	err := yaml.Unmarshal([]byte(manifest), &obj)
	assert.NoError(t, err)

	metadata := obj["metadata"].(map[string]interface{})
	assert.Equal(t, "alpine-3-23-bios-base", metadata["name"])
	assert.Equal(t, "test-ns", metadata["namespace"])
	labels := metadata["labels"].(map[string]interface{})
	assert.Equal(t, VirtualizationGroupName, labels["upmeter-group"])
	assert.Equal(t, VirtualizationCreationProbeName, labels["upmeter-probe"])

	spec := obj["spec"].(map[string]interface{})
	dataSource := spec["dataSource"].(map[string]interface{})
	containerImage := dataSource["containerImage"].(map[string]interface{})
	assert.Equal(t, "ContainerImage", dataSource["type"])
	assert.Equal(t, "registry.example.com/upmeter-vm@sha256:abc", containerImage["image"])
}

func Test_virtualDiskManifest(t *testing.T) {
	manifest := virtualDiskManifest("agent-01", "test-ns", VirtualizationCreationProbeName, "probe-disk", "upmeter-probe")

	var obj map[string]interface{}
	err := yaml.Unmarshal([]byte(manifest), &obj)
	assert.NoError(t, err)

	metadata := obj["metadata"].(map[string]interface{})
	assert.Equal(t, "probe-disk", metadata["name"])
	assert.Equal(t, "test-ns", metadata["namespace"])
	labels := metadata["labels"].(map[string]interface{})
	assert.Equal(t, VirtualizationGroupName, labels["upmeter-group"])
	assert.Equal(t, VirtualizationCreationProbeName, labels["upmeter-probe"])

	spec := obj["spec"].(map[string]interface{})
	dataSource := spec["dataSource"].(map[string]interface{})
	objectRef := dataSource["objectRef"].(map[string]interface{})
	assert.Equal(t, "VirtualImage", objectRef["kind"])
	assert.Equal(t, "upmeter-probe", objectRef["name"])
}

func Test_virtualMachineManifest(t *testing.T) {
	manifest := virtualMachineManifest("agent-01", "test-ns", VirtualizationCreationProbeName, "probe-vm", "probe-disk")

	var obj map[string]interface{}
	err := yaml.Unmarshal([]byte(manifest), &obj)
	assert.NoError(t, err)

	metadata := obj["metadata"].(map[string]interface{})
	assert.Equal(t, "probe-vm", metadata["name"])
	labels := metadata["labels"].(map[string]interface{})
	assert.Equal(t, VirtualizationGroupName, labels["upmeter-group"])
	assert.Equal(t, VirtualizationCreationProbeName, labels["upmeter-probe"])

	spec := obj["spec"].(map[string]interface{})
	assert.NotContains(t, spec, "virtualMachineClassName")
	assert.Equal(t, "AlwaysOn", spec["runPolicy"])

	cpu := spec["cpu"].(map[string]interface{})
	assert.EqualValues(t, 1, cpu["cores"])

	memory := spec["memory"].(map[string]interface{})
	assert.Equal(t, "256Mi", memory["size"])

	blockDeviceRefs := spec["blockDeviceRefs"].([]interface{})
	assert.Len(t, blockDeviceRefs, 1)
	ref := blockDeviceRefs[0].(map[string]interface{})
	assert.Equal(t, "VirtualDisk", ref["kind"])
	assert.Equal(t, "probe-disk", ref["name"])
}

func Test_virtualMachineOperationManifest(t *testing.T) {
	manifest := virtualMachineOperationManifest("agent-01", "test-ns", VirtualizationMigrationProbeName, "probe-vm-evict", "probe-vm")

	var obj map[string]interface{}
	err := yaml.Unmarshal([]byte(manifest), &obj)
	assert.NoError(t, err)

	metadata := obj["metadata"].(map[string]interface{})
	assert.Equal(t, "probe-vm-evict", metadata["name"])
	assert.Equal(t, "test-ns", metadata["namespace"])
	labels := metadata["labels"].(map[string]interface{})
	assert.Equal(t, VirtualizationGroupName, labels["upmeter-group"])
	assert.Equal(t, VirtualizationMigrationProbeName, labels["upmeter-probe"])

	spec := obj["spec"].(map[string]interface{})
	assert.Equal(t, "probe-vm", spec["virtualMachineName"])
	assert.Equal(t, "Evict", spec["type"])
}

func Test_unstructuredNestedString(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"phase": "Running",
		},
	}

	assert.Equal(t, "Running", unstructuredNestedString(obj, "status", "phase"))
	assert.Equal(t, "", unstructuredNestedString(obj, "status", "missing"))
}

func Test_unstructuredNestedStringSlice(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"availableNodes": []interface{}{"node-a", "node-b"},
		},
	}

	assert.Equal(t, []string{"node-a", "node-b"}, unstructuredNestedStringSlice(obj, "status", "availableNodes"))
	assert.Nil(t, unstructuredNestedStringSlice(obj, "status", "missing"))
}

func Test_unstructuredConditionStatus(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{
					"type":   "VirtualMachineClassReady",
					"status": "True",
				},
				map[string]interface{}{
					"type":   "AgentReady",
					"status": "False",
				},
			},
		},
	}

	assert.Equal(t, "False", unstructuredConditionStatus(obj, "AgentReady"))
	assert.Equal(t, "True", unstructuredConditionStatus(obj, "VirtualMachineClassReady"))
	assert.Equal(t, "", unstructuredConditionStatus(obj, "Missing"))
}
