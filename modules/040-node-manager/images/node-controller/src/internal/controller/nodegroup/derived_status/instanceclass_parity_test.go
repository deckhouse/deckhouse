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

package derived_status

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// applyCloudSpecificDefaults returns the raw InstanceClass .spec unchanged for
// every provider except vsphere, which fills mainNetwork from
// cloudProvider.vsphere.instances.mainNetwork when absent. JSON-marshal both
// sides so number types (int vs float64) compare as they hit the checksum.
func assertInstanceClassGolden(t *testing.T, cloudProvider map[string]interface{}, spec interface{}, goldenJSON string) {
	t.Helper()

	got, err := applyCloudSpecificDefaults(cloudProvider, spec)
	require.NoError(t, err)

	gotJSON, err := json.Marshal(got)
	require.NoError(t, err)

	var gotVal, wantVal interface{}
	require.NoError(t, json.Unmarshal(gotJSON, &gotVal))
	require.NoError(t, json.Unmarshal([]byte(goldenJSON), &wantVal))

	assert.Equal(t, wantVal, gotVal)
}

// jsonSpec mimics readInstanceClassSpec: json-decoded map (numbers -> float64).
func jsonSpec(t *testing.T, raw string) interface{} {
	t.Helper()
	var out interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &out))
	return out
}

func TestApplyCloudSpecificDefaults_YandexPassthrough(t *testing.T) {
	cloudProvider := map[string]interface{}{
		"type": "yandex",
		"yandex": map[string]interface{}{
			"region": "ru-central1",
		},
	}
	spec := jsonSpec(t, `{
		"platformID": "standard-v3",
		"cores": 4,
		"memory": 8192,
		"diskSizeGB": 50,
		"coreFraction": 100
	}`)

	// Verbatim passthrough: nothing added, nothing dropped, numbers preserved.
	assertInstanceClassGolden(t, cloudProvider, spec, `{
		"platformID": "standard-v3",
		"cores": 4,
		"memory": 8192,
		"diskSizeGB": 50,
		"coreFraction": 100
	}`)
}

func TestApplyCloudSpecificDefaults_AWSPassthrough(t *testing.T) {
	cloudProvider := map[string]interface{}{
		"type": "aws",
		"aws": map[string]interface{}{
			"region": "eu-central-1",
		},
	}
	spec := jsonSpec(t, `{
		"instanceType": "m5.large",
		"spot": true,
		"diskType": "gp3",
		"diskSizeGb": 80
	}`)

	assertInstanceClassGolden(t, cloudProvider, spec, `{
		"instanceType": "m5.large",
		"spot": true,
		"diskType": "gp3",
		"diskSizeGb": 80
	}`)
}

func TestApplyCloudSpecificDefaults_UnknownProviderPassthrough(t *testing.T) {
	// provider key absent from cloudProvider map -> spec returned unchanged.
	cloudProvider := map[string]interface{}{
		"type": "openstack",
	}
	spec := jsonSpec(t, `{"flavorName": "m1.large", "rootDiskSize": 30}`)

	assertInstanceClassGolden(t, cloudProvider, spec, `{"flavorName": "m1.large", "rootDiskSize": 30}`)
}

func TestApplyCloudSpecificDefaults_VsphereFillsMainNetwork(t *testing.T) {
	cloudProvider := map[string]interface{}{
		"type": "vsphere",
		"vsphere": map[string]interface{}{
			"instances": map[string]interface{}{
				"mainNetwork": "k8s-msk-178",
			},
		},
	}
	spec := jsonSpec(t, `{
		"numCPUs": 4,
		"memory": 8192,
		"rootDiskSize": 40,
		"template": "dev/golden-image"
	}`)

	// mainNetwork absent in spec -> filled from cloudProvider.vsphere.instances.
	assertInstanceClassGolden(t, cloudProvider, spec, `{
		"numCPUs": 4,
		"memory": 8192,
		"rootDiskSize": 40,
		"template": "dev/golden-image",
		"mainNetwork": "k8s-msk-178"
	}`)
}

func TestApplyCloudSpecificDefaults_VsphereKeepsExplicitMainNetwork(t *testing.T) {
	cloudProvider := map[string]interface{}{
		"type": "vsphere",
		"vsphere": map[string]interface{}{
			"instances": map[string]interface{}{
				"mainNetwork": "k8s-msk-178",
			},
		},
	}
	spec := jsonSpec(t, `{
		"numCPUs": 4,
		"mainNetwork": "explicit-net"
	}`)

	// mainNetwork set in spec -> the cloudProvider default must NOT override it.
	assertInstanceClassGolden(t, cloudProvider, spec, `{
		"numCPUs": 4,
		"mainNetwork": "explicit-net"
	}`)
}

func TestApplyCloudSpecificDefaults_VsphereNoDefaultAvailable(t *testing.T) {
	// vsphere provider present but cloudProvider.vsphere.instances.mainNetwork
	// absent -> spec stays untouched (no mainNetwork key injected).
	cloudProvider := map[string]interface{}{
		"type":    "vsphere",
		"vsphere": map[string]interface{}{},
	}
	spec := jsonSpec(t, `{"numCPUs": 4}`)

	assertInstanceClassGolden(t, cloudProvider, spec, `{"numCPUs": 4}`)
}

func TestApplyCloudSpecificDefaults_NonMapSpecUntouched(t *testing.T) {
	// Defensive: a non-map spec (e.g. null) is returned as-is.
	got, err := applyCloudSpecificDefaults(map[string]interface{}{"type": "yandex"}, nil)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestApplyCloudSpecificDefaults_DoesNotMutateInputForNonVsphere(t *testing.T) {
	spec := map[string]interface{}{"cores": float64(4)}
	_, err := applyCloudSpecificDefaults(map[string]interface{}{"type": "yandex"}, spec)
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"cores": float64(4)}, spec,
		"non-vsphere spec must be returned untouched")
}
