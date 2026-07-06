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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blobFromJSON decodes a JSON literal into a blob element the same way the
// runtime blob is produced (json.Unmarshal into interface{} → float64 numbers),
// so the test exercises the exact numeric kinds mcmDrainTimeout/intOrDefault see.
func blobFromJSON(t *testing.T, s string) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(s), &m))
	return m
}

func mdSpec(t *testing.T, md map[string]interface{}) map[string]interface{} {
	t.Helper()
	spec, ok := md["spec"].(map[string]interface{})
	require.True(t, ok, "spec must be a map")
	return spec
}

func mdTemplateSpec(t *testing.T, md map[string]interface{}) map[string]interface{} {
	t.Helper()
	tmpl := mdSpec(t, md)["template"].(map[string]interface{})
	return tmpl["spec"].(map[string]interface{})
}

// TestBuildMCMMachineDeployment_Defaults covers the plain path: no nodeCapacity,
// no quickShutdown, no nodeDrainTimeoutSecond, no nodeTemplate → 600s/30 drain,
// maxSurge 1 / maxUnavailable 0 defaults, only the mandatory nodeTemplate labels,
// no scale-from-zero annotations, no creationTimeout.
func TestBuildMCMMachineDeployment_Defaults(t *testing.T) {
	blob := blobFromJSON(t, `{"name":"worker","nodeType":"CloudEphemeral"}`)
	md := buildMCMMachineDeployment(mcmMachineDeploymentInput{
		blob:             blob,
		ngName:           "worker",
		zone:             "eu-west-1a",
		mdName:           "worker-abcdef01",
		machineClassName: "worker-abcdef01",
		machineClassKind: "AWSMachineClass",
		checksum:         "deadbeef",
		replicas:         3,
	})

	assert.Equal(t, "machine.sapcloud.io/v1alpha1", md.GetAPIVersion())
	assert.Equal(t, "MachineDeployment", md.GetKind())

	meta := md.Object["metadata"].(map[string]interface{})
	assert.Equal(t, "worker-abcdef01", meta["name"])
	assert.Equal(t, capiNamespace, meta["namespace"])
	assert.Equal(t, map[string]interface{}{
		"heritage":   "deckhouse",
		"module":     "node-manager",
		"node-group": "worker",
	}, meta["labels"])
	// Only the zone annotation, no scale-from-zero.
	assert.Equal(t, map[string]interface{}{"zone": "eu-west-1a"}, meta["annotations"])

	spec := mdSpec(t, md.Object)
	assert.Equal(t, int64(3), spec["replicas"])
	assert.Equal(t, int64(300), spec["minReadySeconds"])

	strat := spec["strategy"].(map[string]interface{})
	assert.Equal(t, "RollingUpdate", strat["type"])
	ru := strat["rollingUpdate"].(map[string]interface{})
	assert.Equal(t, int64(1), ru["maxSurge"])
	assert.Equal(t, int64(0), ru["maxUnavailable"])

	sel := spec["selector"].(map[string]interface{})["matchLabels"].(map[string]interface{})
	assert.Equal(t, "worker-eu-west-1a", sel["instance-group"])

	tmpl := spec["template"].(map[string]interface{})
	tmplMeta := tmpl["metadata"].(map[string]interface{})
	assert.Equal(t, map[string]interface{}{"instance-group": "worker-eu-west-1a"}, tmplMeta["labels"])
	assert.Equal(t, map[string]interface{}{"checksum/machine-class": "deadbeef"}, tmplMeta["annotations"])

	ts := mdTemplateSpec(t, md.Object)
	assert.Equal(t, map[string]interface{}{
		"kind": "AWSMachineClass",
		"name": "worker-abcdef01",
	}, ts["class"])
	assert.Equal(t, "600s", ts["drainTimeout"])
	assert.Equal(t, int64(30), ts["maxEvictRetries"])
	_, hasCreationTimeout := ts["creationTimeout"]
	assert.False(t, hasCreationTimeout, "no creationTimeout without aws spot")

	// Mandatory nodeTemplate labels only, no annotations/taints.
	ntMeta := ts["nodeTemplate"].(map[string]interface{})["metadata"].(map[string]interface{})
	assert.Equal(t, map[string]interface{}{
		"node-role.kubernetes.io/worker": "",
		"node.deckhouse.io/group":        "worker",
		"node.deckhouse.io/type":         "CloudEphemeral",
	}, ntMeta["labels"])
	_, hasAnn := ntMeta["annotations"]
	assert.False(t, hasAnn)
	_, hasSpec := ts["nodeTemplate"].(map[string]interface{})["spec"]
	assert.False(t, hasSpec)
}

// TestBuildMCMMachineDeployment_ScaleFromZero covers the nodeCapacity branch:
// the five cluster-autoscaler annotations with region wired in.
func TestBuildMCMMachineDeployment_ScaleFromZero(t *testing.T) {
	blob := blobFromJSON(t, `{"name":"worker","nodeCapacity":{"cpu":"4","memory":"8Gi"}}`)
	md := buildMCMMachineDeployment(mcmMachineDeploymentInput{
		blob:     blob,
		ngName:   "worker",
		zone:     "eu-west-1a",
		mdName:   "worker-abcdef01",
		region:   "eu-west-1",
		checksum: "deadbeef",
	})
	ann := md.Object["metadata"].(map[string]interface{})["annotations"].(map[string]interface{})
	assert.Equal(t, "eu-west-1a", ann["zone"])
	assert.Equal(t, "true", ann["cluster-autoscaler.kubernetes.io/scale-from-zero"])
	assert.Equal(t, "eu-west-1", ann["cluster-autoscaler.kubernetes.io/node-region"])
	assert.Equal(t, "4", ann["cluster-autoscaler.kubernetes.io/node-cpu"])
	assert.Equal(t, "8Gi", ann["cluster-autoscaler.kubernetes.io/node-memory"])
	assert.Equal(t, "eu-west-1a", ann["cluster-autoscaler.kubernetes.io/node-zone"])
}

// TestBuildMCMMachineDeployment_QuickShutdown covers the 5m/9 drain tier.
func TestBuildMCMMachineDeployment_QuickShutdown(t *testing.T) {
	blob := blobFromJSON(t, `{"name":"worker","cloudInstances":{"quickShutdown":true}}`)
	md := buildMCMMachineDeployment(mcmMachineDeploymentInput{blob: blob, ngName: "worker", zone: "z"})
	ts := mdTemplateSpec(t, md.Object)
	assert.Equal(t, "5m", ts["drainTimeout"])
	assert.Equal(t, int64(9), ts["maxEvictRetries"])
}

// TestBuildMCMMachineDeployment_NodeDrainTimeout covers the {n}s/(n/20) tier
// with the integer division floor (200/20=10, 210/20=10).
func TestBuildMCMMachineDeployment_NodeDrainTimeout(t *testing.T) {
	blob := blobFromJSON(t, `{"name":"worker","nodeDrainTimeoutSecond":210}`)
	md := buildMCMMachineDeployment(mcmMachineDeploymentInput{blob: blob, ngName: "worker", zone: "z"})
	ts := mdTemplateSpec(t, md.Object)
	assert.Equal(t, "210s", ts["drainTimeout"])
	assert.Equal(t, int64(10), ts["maxEvictRetries"])
}

// TestBuildMCMMachineDeployment_MaxSurgeUnavailable covers the per-zone overrides.
func TestBuildMCMMachineDeployment_MaxSurgeUnavailable(t *testing.T) {
	blob := blobFromJSON(t, `{"name":"worker","cloudInstances":{"maxSurgePerZone":3,"maxUnavailablePerZone":2}}`)
	md := buildMCMMachineDeployment(mcmMachineDeploymentInput{blob: blob, ngName: "worker", zone: "z"})
	ru := mdSpec(t, md.Object)["strategy"].(map[string]interface{})["rollingUpdate"].(map[string]interface{})
	assert.Equal(t, int64(3), ru["maxSurge"])
	assert.Equal(t, int64(2), ru["maxUnavailable"])
}

// TestBuildMCMMachineDeployment_NodeTemplate covers labels merge, annotations
// passthrough, and the taints slice (with and without value).
func TestBuildMCMMachineDeployment_NodeTemplate(t *testing.T) {
	blob := blobFromJSON(t, `{
		"name":"worker",
		"nodeTemplate":{
			"labels":{"custom/label":"v","node.deckhouse.io/type":"override-ignored-by-order"},
			"annotations":{"custom/ann":"a"},
			"taints":[
				{"key":"k1","effect":"NoSchedule","value":"v1"},
				{"key":"k2","effect":"NoExecute"}
			]
		}
	}`)
	md := buildMCMMachineDeployment(mcmMachineDeploymentInput{blob: blob, ngName: "worker", zone: "z"})
	nt := mdTemplateSpec(t, md.Object)["nodeTemplate"].(map[string]interface{})
	ntMeta := nt["metadata"].(map[string]interface{})

	labels := ntMeta["labels"].(map[string]interface{})
	assert.Equal(t, "", labels["node-role.kubernetes.io/worker"])
	assert.Equal(t, "worker", labels["node.deckhouse.io/group"])
	assert.Equal(t, "v", labels["custom/label"])
	// User label overrides the mandatory one (helm merge order: user last wins).
	assert.Equal(t, "override-ignored-by-order", labels["node.deckhouse.io/type"])

	assert.Equal(t, map[string]interface{}{"custom/ann": "a"}, ntMeta["annotations"])

	taints := nt["spec"].(map[string]interface{})["taints"].([]interface{})
	require.Len(t, taints, 2)
	assert.Equal(t, map[string]interface{}{"key": "k1", "effect": "NoSchedule", "value": "v1"}, taints[0])
	assert.Equal(t, map[string]interface{}{"key": "k2", "effect": "NoExecute"}, taints[1])
}

// TestBuildMCMMachineDeployment_AWSSpot covers the creationTimeout 5m addition.
func TestBuildMCMMachineDeployment_AWSSpot(t *testing.T) {
	blob := blobFromJSON(t, `{"name":"worker"}`)
	md := buildMCMMachineDeployment(mcmMachineDeploymentInput{blob: blob, ngName: "worker", zone: "z", awsSpot: true})
	ts := mdTemplateSpec(t, md.Object)
	assert.Equal(t, "5m", ts["creationTimeout"])
}
