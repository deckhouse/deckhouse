/*
Copyright 2023 Flant JSC

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

package template

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestBashibleChecksum(t *testing.T) {
	hash := func(t *testing.T, bc *bashibleContext) string {
		h := sha256.New()

		bcDataExpected, err := yaml.Marshal(bc)
		if err != nil {
			t.Fatal("cannot marshal bashible context")
		}

		err = bc.AddToChecksum(h)
		if err != nil {
			t.Fatalf("Add to checksum error: %v", err)
		}

		bcDataAfter, err := yaml.Marshal(bc)
		if err != nil {
			t.Fatal("cannot marshal bashible context")
		}

		if string(bcDataExpected) != string(bcDataAfter) {
			t.Fatal("AddToChecksum should not change object")
		}

		return fmt.Sprintf("%x", h.Sum(nil))
	}

	clusterMasterAddresses := []string{
		"10.0.0.1",
	}

	const ngYaml = `
cloudInstances:
  classReference:
    kind: OpenStackInstanceClass
    name: pico
  maxPerZone: 0
  maxSurgePerZone: 0
  maxUnavailablePerZone: 0
  minPerZone: 0
  zones:
  - nova
cri:
  type: Docker
disruptions:
  approvalMode: Manual
instanceClass:
  flavorName: nm1.small
  imageName: ubuntu-18-04-cloud-amd64
  mainNetwork: sandbox
kubelet:
  containerLogMaxFiles: 4
  containerLogMaxSize: 50Mi
  maxPods: 13
kubernetesVersion: "1.29"
manualRolloutID: ""
name: stage
nodeTemplate:
  labels:
    node-role.aaaaa.io/staging: ""
nodeType: CloudEphemeral
updateEpoch: "1680009541"
`
	ng := make(map[string]interface{})

	err := yaml.Unmarshal([]byte(ngYaml), &ng)
	if err != nil {
		t.Errorf("unmarshal error: %v", err)
		return
	}

	bc := bashibleContext{
		KubernetesVersion: "1.26",
		Bundle:            "bundle",
		Normal:            map[string][]string{"apiserverEndpoints": clusterMasterAddresses},
		NodeGroup:         ng,
		RunType:           "Normal",

		Images: map[string]map[string]string{
			"common": {
				"pause": "c5120536ab49040dbbff34be987469227fd9c241a6fd73da694c13c1-1654517843943",
			},
		},

		Registry: &registry{
			Address: "registry.deckhouse.io",
			Path:    "/deckhouse/ce",
		},

		Proxy: map[string]interface{}{
			"httpProxy": "proxy.example.com:444",
		},
	}

	expectedHash := hash(t, &bc)

	t.Run("changing counter in cloudInstances object does not affect checksum", func(t *testing.T) {
		bc.NodeGroup["cloudInstances"].(map[string]interface{})["maxPerZone"] = 2
		bc.NodeGroup["cloudInstances"].(map[string]interface{})["maxSurgePerZone"] = 1
		bc.NodeGroup["cloudInstances"].(map[string]interface{})["maxUnavailablePerZone"] = 1
		bc.NodeGroup["cloudInstances"].(map[string]interface{})["minPerZone"] = 1
		bc.NodeGroup["cloudInstances"].(map[string]interface{})["zones"] = []string{"aaaaaa"}

		newHash := hash(t, &bc)

		if expectedHash != newHash {
			t.Errorf("%s != %s", expectedHash, newHash)
			return
		}
	})
}
