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

const testRPPBootstrapServerPort = 4282

func TestClusterUUIDIsPreservedInTemplateContexts(t *testing.T) {
	const clusterUUID = "ce64db27-f724-4b50-bb86-e4ac57a1d49d"

	var input inputData
	if err := yaml.Unmarshal([]byte("clusterUUID: "+clusterUUID+"\n"), &input); err != nil {
		t.Fatalf("unmarshal inputData: %v", err)
	}
	if input.ClusterUUID != clusterUUID {
		t.Fatalf("inputData.ClusterUUID = %q, want %q", input.ClusterUUID, clusterUUID)
	}

	common := tplContextCommon{ClusterUUID: input.ClusterUUID}
	bundle := bundleNGContext{tplContextCommon: &common}
	bundleData, err := yaml.Marshal(bundle)
	if err != nil {
		t.Fatalf("marshal bundleNGContext: %v", err)
	}
	bundleMap := make(map[string]interface{})
	if err := yaml.Unmarshal(bundleData, &bundleMap); err != nil {
		t.Fatalf("unmarshal bundleNGContext: %v", err)
	}
	if got := bundleMap["clusterUUID"]; got != clusterUUID {
		t.Fatalf("bundleNGContext clusterUUID = %v, want %q", got, clusterUUID)
	}

	bc := bashibleContext{ClusterUUID: input.ClusterUUID}
	bcData, err := yaml.Marshal(bc)
	if err != nil {
		t.Fatalf("marshal bashibleContext: %v", err)
	}
	bcMap := make(map[string]interface{})
	if err := yaml.Unmarshal(bcData, &bcMap); err != nil {
		t.Fatalf("unmarshal bashibleContext: %v", err)
	}
	if got := bcMap["clusterUUID"]; got != clusterUUID {
		t.Fatalf("bashibleContext clusterUUID = %v, want %q", got, clusterUUID)
	}
}

func TestClusterMasterEndpointAddresses(t *testing.T) {
	kubeAPIEndpoints, rppAddresses, rppBootstrapAddresses := clusterMasterEndpointAddresses([]clusterMasterEndpoint{
		{
			Address:                "10.0.0.1",
			KubeAPIPort:            6443,
			RPPServerPort:          4219,
			RPPBootstrapServerPort: testRPPBootstrapServerPort,
		},
		{
			Address:                "10.0.0.2",
			RPPServerPort:          4219,
			RPPBootstrapServerPort: testRPPBootstrapServerPort,
		},
	})

	if got, want := fmt.Sprint(kubeAPIEndpoints), "[10.0.0.1:6443]"; got != want {
		t.Fatalf("kubeAPIEndpoints = %s, want %s", got, want)
	}
	if got, want := fmt.Sprint(rppAddresses), "[10.0.0.1:4219 10.0.0.2:4219]"; got != want {
		t.Fatalf("rppAddresses = %s, want %s", got, want)
	}
	if got, want := fmt.Sprint(rppBootstrapAddresses), fmt.Sprintf("[10.0.0.1:%d 10.0.0.2:%d]", testRPPBootstrapServerPort, testRPPBootstrapServerPort); got != want {
		t.Fatalf("rppBootstrapAddresses = %s, want %s", got, want)
	}
}

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
kubernetesVersion: "1.31"
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
		Normal: map[string]interface{}{
			"apiserverEndpoints": clusterMasterAddresses,
			"clusterMasterEndpoints": []map[string]interface{}{
				{
					"address":                "10.0.0.1",
					"kubeApiPort":            6443,
					"rppServerPort":          4219,
					"rppBootstrapServerPort": testRPPBootstrapServerPort,
				},
			},
		},
		NodeGroup: ng,
		RunType:   "Normal",

		Images: map[string]map[string]string{
			"common": {
				"pause": "c5120536ab49040dbbff34be987469227fd9c241a6fd73da694c13c1-1654517843943",
			},
		},

		Registry: map[string]interface{}{
			"registryModuleEnable": true,
			"mode":                 "unmanaged",
			"version":              "unknown",
			"imagesBase":           "registry.d8-system.svc/deckhouse/system",
			"proxyEndpoints":       []interface{}{"192.168.1.1"},
			"hosts": map[string]interface{}{
				"registry.d8-system.svc": map[string]interface{}{
					"mirrors": []interface{}{
						map[string]interface{}{
							"host":   "r.example.com",
							"scheme": "https",
							"ca":     "==exampleCA==",
							"auth": map[string]interface{}{
								"username": "user",
								"password": "password",
								"auth":     "auth",
							},
							"rewrites": []interface{}{
								map[string]interface{}{
									"from": "^deckhouse/system",
									"to":   "deckhouse/ce",
								},
							},
						},
					},
				},
			},
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
