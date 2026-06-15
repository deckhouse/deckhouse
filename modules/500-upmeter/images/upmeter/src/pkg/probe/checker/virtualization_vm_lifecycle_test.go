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
		"alpine-3-23-bios-base",
		"https://example.com/alpine.qcow2",
	)

	var obj map[string]interface{}
	err := yaml.Unmarshal([]byte(manifest), &obj)
	assert.NoError(t, err)

	metadata := obj["metadata"].(map[string]interface{})
	assert.Equal(t, "alpine-3-23-bios-base", metadata["name"])
	assert.Equal(t, "test-ns", metadata["namespace"])

	spec := obj["spec"].(map[string]interface{})
	dataSource := spec["dataSource"].(map[string]interface{})
	http := dataSource["http"].(map[string]interface{})
	assert.Equal(t, "https://example.com/alpine.qcow2", http["url"])
}

func Test_virtualDiskManifest(t *testing.T) {
	manifest := virtualDiskManifest("agent-01", "test-ns", "probe-disk", "upmeter-probe")

	var obj map[string]interface{}
	err := yaml.Unmarshal([]byte(manifest), &obj)
	assert.NoError(t, err)

	metadata := obj["metadata"].(map[string]interface{})
	assert.Equal(t, "probe-disk", metadata["name"])
	assert.Equal(t, "test-ns", metadata["namespace"])

	spec := obj["spec"].(map[string]interface{})
	dataSource := spec["dataSource"].(map[string]interface{})
	objectRef := dataSource["objectRef"].(map[string]interface{})
	assert.Equal(t, "VirtualImage", objectRef["kind"])
	assert.Equal(t, "upmeter-probe", objectRef["name"])
}

func Test_virtualMachineManifest(t *testing.T) {
	manifest := virtualMachineManifest("agent-01", "test-ns", "probe-vm", "probe-disk", "generic")

	var obj map[string]interface{}
	err := yaml.Unmarshal([]byte(manifest), &obj)
	assert.NoError(t, err)

	metadata := obj["metadata"].(map[string]interface{})
	assert.Equal(t, "probe-vm", metadata["name"])

	spec := obj["spec"].(map[string]interface{})
	assert.Equal(t, "generic", spec["virtualMachineClassName"])
	assert.Equal(t, "AlwaysOn", spec["runPolicy"])

	cpu := spec["cpu"].(map[string]interface{})
	assert.EqualValues(t, 1, cpu["cores"])

	blockDeviceRefs := spec["blockDeviceRefs"].([]interface{})
	assert.Len(t, blockDeviceRefs, 1)
	ref := blockDeviceRefs[0].(map[string]interface{})
	assert.Equal(t, "VirtualDisk", ref["kind"])
	assert.Equal(t, "probe-disk", ref["name"])
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
