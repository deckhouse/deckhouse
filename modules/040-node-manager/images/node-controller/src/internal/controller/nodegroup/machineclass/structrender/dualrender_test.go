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

package structrender

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/deckhouse/node-controller/internal/controller/nodegroup/machineclass"
)

// applyCommonMetadata is what production node-controller would add around a v2 render:
// under v2 the provider ships only kind/apiVersion/spec, and the metadata contract
// (name, namespace, checksum annotation, keep policy, labels) is owned by node-controller.
func applyCommonMetadata(obj map[string]interface{}, rctx Context) {
	obj["metadata"] = map[string]interface{}{
		"name":      rctx.TemplateName,
		"namespace": "d8-cloud-instance-manager",
		"annotations": map[string]interface{}{
			"checksum/instance-class": rctx.Checksum,
			"helm.sh/resource-policy": "keep",
		},
		"labels": map[string]interface{}{
			"heritage":   "deckhouse",
			"module":     "node-manager",
			"node-group": rctx.NodeGroupName,
		},
	}
}

// renderV1 renders the real provider template through the production v1 path and parses it.
func renderV1(t *testing.T, templatePath string, rctx Context) map[string]interface{} {
	t.Helper()
	tmpl, err := os.ReadFile(templatePath)
	require.NoError(t, err, "v1 template must exist")

	v1ctx := map[string]interface{}{
		"Values": map[string]interface{}{
			"global": map[string]interface{}{
				"discovery": map[string]interface{}{
					"clusterUUID": "uuid-1",
					"podSubnet":   "10.111.0.0/16",
				},
			},
			"nodeManager": map[string]interface{}{
				"internal": map[string]interface{}{
					"cloudProvider": rctx.CloudProvider,
				},
			},
		},
		"nodeGroup": map[string]interface{}{
			"name":          rctx.NodeGroupName,
			"nodeType":      "CloudEphemeral",
			"instanceClass": rctx.InstanceClass,
		},
		"zoneName":              rctx.Zone,
		"templateName":          rctx.TemplateName,
		"instanceClassChecksum": rctx.Checksum,
	}

	rendered, err := machineclass.RenderMachineClass(tmpl, v1ctx)
	require.NoError(t, err, "v1 render must succeed")
	obj := map[string]interface{}{}
	require.NoError(t, sigsyaml.Unmarshal(rendered, &obj), "v1 output must be valid YAML")
	return obj
}

func renderV2(t *testing.T, specPath string, rctx Context) map[string]interface{} {
	t.Helper()
	raw, err := os.ReadFile(specPath)
	require.NoError(t, err)
	spec, err := ParseSpec(raw)
	require.NoError(t, err)
	obj, err := Render(spec, rctx)
	require.NoError(t, err, "v2 render must succeed")
	applyCommonMetadata(obj, rctx)
	return obj
}

// canonical marshals through JSON so that int64(4) from the v1 YAML parse and float64(4)
// carried by the v2 context compare equal — which is exactly the equality SSA cares about.
func canonical(t *testing.T, obj map[string]interface{}) string {
	t.Helper()
	b, err := json.Marshal(obj)
	require.NoError(t, err)
	return string(b)
}

func yandexContext() Context {
	return Context{
		InstanceClass: map[string]interface{}{
			"cores":             float64(4),
			"memory":            float64(8192),
			"diskSizeGB":        float64(64),
			"additionalSubnets": []interface{}{"extra-1", "extra-2"},
		},
		CloudProvider: map[string]interface{}{
			"yandex": map[string]interface{}{
				"instanceClassDefaults": map[string]interface{}{"imageID": "fd-default-image"},
				"zoneToSubnetIdMap": map[string]interface{}{
					"ru-central1-a": "subnet-a",
					"ru-central1-b": "subnet-b",
				},
				"shouldAssignPublicIPAddress": false,
				"sshKey":                      "ssh-ed25519 AAAA test",
				"nodeNetworkCIDR":             "10.222.0.0/16",
			},
		},
		Zone:          "ru-central1-a",
		TemplateName:  "worker-abc12345",
		Checksum:      "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		NodeGroupName: "worker",
	}
}

func TestDualRender_Yandex_DefaultsPath(t *testing.T) {
	rctx := yandexContext()
	v1 := renderV1(t, "../../../../../../../../../030-cloud-provider-yandex/capi/machine-template.yaml", rctx)
	v2 := renderV2(t, "testdata/yandex-machine-template.v2.yaml", rctx)
	assert.JSONEq(t, canonical(t, v1), canonical(t, v2),
		"v2 must reproduce v1 for the defaults-heavy case (platformID/coreFraction/diskType/gpus/imageID/mainSubnet all defaulted)")
}

func TestDualRender_Yandex_ExplicitPath(t *testing.T) {
	rctx := yandexContext()
	rctx.InstanceClass["platformID"] = "standard-v2"
	rctx.InstanceClass["coreFraction"] = float64(50)
	rctx.InstanceClass["gpus"] = float64(2)
	rctx.InstanceClass["diskType"] = "network-ssd"
	rctx.InstanceClass["imageID"] = "fd-explicit"
	rctx.InstanceClass["mainSubnet"] = "subnet-main"
	rctx.InstanceClass["assignPublicIPAddress"] = true
	v1 := renderV1(t, "../../../../../../../../../030-cloud-provider-yandex/capi/machine-template.yaml", rctx)
	v2 := renderV2(t, "testdata/yandex-machine-template.v2.yaml", rctx)
	assert.JSONEq(t, canonical(t, v1), canonical(t, v2),
		"v2 must reproduce v1 when every field is explicit")
}

func dvpContext() Context {
	return Context{
		InstanceClass: map[string]interface{}{
			"virtualMachine": map[string]interface{}{
				"virtualMachineClassName": "vmclass-1",
				"bootloader":              "EFI",
				"cpu": map[string]interface{}{
					"cores":        float64(4),
					"coreFraction": "50%",
				},
				"memory": map[string]interface{}{"size": "8Gi"},
			},
			"rootDisk": map[string]interface{}{
				"size":         "50Gi",
				"storageClass": "ceph-pool",
				"image":        map[string]interface{}{"kind": "ClusterVirtualImage", "name": "ubuntu-22"},
			},
			"additionalDisks": []interface{}{
				map[string]interface{}{"size": "100Gi", "storageClass": "ceph-pool"},
			},
		},
		CloudProvider: map[string]interface{}{"dvp": map[string]interface{}{}},
		Zone:          "default",
		TemplateName:  "worker-def45678",
		Checksum:      "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210",
		NodeGroupName: "worker",
	}
}

func TestDualRender_DVP(t *testing.T) {
	rctx := dvpContext()
	v1 := renderV1(t, "../../../../../../../../../030-cloud-provider-dvp/capi/machine-template.yaml", rctx)
	v2 := renderV2(t, "testdata/dvp-machine-template.v2.yaml", rctx)
	assert.JSONEq(t, canonical(t, v1), canonical(t, v2), "v2 must reproduce v1 for dvp")
}

// A hostile value cannot change the object shape in v2: it stays a string field.
func TestV2_HostileValueStaysAString(t *testing.T) {
	rctx := yandexContext()
	rctx.InstanceClass["platformID"] = "standard: v3\nspec: {hacked: true}"
	v2 := renderV2(t, "testdata/yandex-machine-template.v2.yaml", rctx)
	spec := v2["spec"].(map[string]interface{})["template"].(map[string]interface{})["spec"].(map[string]interface{})
	assert.Equal(t, "standard: v3\nspec: {hacked: true}", spec["platformID"])
	_, hacked := v2["hacked"]
	assert.False(t, hacked)
}

// Required inputs fail loudly instead of rendering an empty field.
func TestV2_RequiredFailsLoudly(t *testing.T) {
	rctx := yandexContext()
	yandexTree := rctx.CloudProvider["yandex"].(map[string]interface{})
	delete(yandexTree, "instanceClassDefaults") // no imageID anywhere
	raw, err := os.ReadFile("testdata/yandex-machine-template.v2.yaml")
	require.NoError(t, err)
	spec, err := ParseSpec(raw)
	require.NoError(t, err)
	_, err = Render(spec, rctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bootDisk.imageID")
}
