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

package machineclass

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// Full-object render tests: pin the entire rendered MachineClass so any field
// drift is caught, not only the handful spot-checked in render_*_test.go.
func assertRenderedMachineClass(t *testing.T, templatePath string, ctx map[string]interface{}, expectedJSON string) {
	t.Helper()

	tmpl, err := os.ReadFile(templatePath)
	require.NoError(t, err, "provider machine-class.yaml must exist")

	out, err := RenderMachineClass(tmpl, ctx)
	require.NoError(t, err)

	var mc map[string]interface{}
	require.NoError(t, yaml.Unmarshal(out, &mc), "rendered MachineClass must be valid YAML")

	gotJSON, err := json.Marshal(mc)
	require.NoError(t, err)

	var got, want interface{}
	require.NoError(t, json.Unmarshal(gotJSON, &got))
	require.NoError(t, json.Unmarshal([]byte(expectedJSON), &want))

	assert.Equal(t, want, got)
}

func TestRenderMachineClass_AWSFullObject(t *testing.T) {
	assertRenderedMachineClass(t, awsMachineClassTemplatePath, awsRenderContext(), `{
		"apiVersion": "machine.sapcloud.io/v1alpha1",
		"kind": "AWSMachineClass",
		"metadata": {
			"labels": { "heritage": "deckhouse", "module": "node-manager" },
			"name": "worker-5d62b9b2",
			"namespace": "d8-cloud-instance-manager"
		},
		"spec": {
			"ami": "ami-default",
			"blockDevices": [ { "ebs": { "volumeSize": 20, "volumeType": "gp2" } } ],
			"iam": { "name": "node-profile" },
			"keyName": "kube-key",
			"machineType": "m5.large",
			"networkInterfaces": [ {
				"associatePublicIPAddress": true,
				"deleteOnTermination": true,
				"securityGroupIDs": [ "sg-base", "sg-extra" ],
				"subnetID": "subnet-aaa"
			} ],
			"region": "eu-central-1",
			"secretRef": { "name": "worker-5d62b9b2", "namespace": "d8-cloud-instance-manager" },
			"sourceDestCheck": false,
			"tags": {
				"env": "prod",
				"kubernetes.io/cluster/aaaa-bbbb": "1",
				"kubernetes.io/role/aaaa-bbbb": "1"
			},
			"useMachineNameAsNodeName": true
		}
	}`)
}

func TestRenderMachineClass_AzureFullObject(t *testing.T) {
	assertRenderedMachineClass(t, azureMachineClassTemplatePath, azureRenderContext(), `{
		"apiVersion": "machine.sapcloud.io/v1alpha1",
		"kind": "AzureMachineClass",
		"metadata": {
			"labels": { "heritage": "deckhouse", "module": "node-manager" },
			"name": "worker-83a30ed3",
			"namespace": "d8-cloud-instance-manager"
		},
		"spec": {
			"location": "westeurope",
			"properties": {
				"hardwareProfile": { "vmSize": "Standard_D4s_v3" },
				"networkProfile": { "acceleratedNetworking": true },
				"osProfile": {
					"adminUsername": "azureuser",
					"linuxConfiguration": {
						"disablePasswordAuthentication": true,
						"ssh": {
							"publicKeys": {
								"keyData": "ssh-rsa AAAA",
								"path": "/home/azureuser/.ssh/authorized_keys"
							}
						}
					}
				},
				"storageProfile": {
					"imageReference": { "urn": "Canonical:0001:22_04-lts:latest" },
					"osDisk": {
						"caching": "ReadWrite",
						"createOption": "FromImage",
						"diskSizeGB": 50,
						"managedDisk": { "storageAccountType": "Premium_LRS" }
					}
				},
				"zone": 1
			},
			"resourceGroup": "kube-rg",
			"secretRef": { "name": "worker-83a30ed3", "namespace": "d8-cloud-instance-manager" },
			"subnetInfo": { "subnetName": "kube-subnet", "vnetName": "kube-vnet" },
			"tags": {
				"env": "prod",
				"kubernetes.io-cluster-aaaa-bbbb": "1",
				"kubernetes.io-role-worker-1": "1"
			}
		}
	}`)
}

func TestRenderMachineClass_GCPFullObject(t *testing.T) {
	assertRenderedMachineClass(t, gcpMachineClassTemplatePath, gcpRenderContext(), `{
		"apiVersion": "machine.sapcloud.io/v1alpha1",
		"kind": "GCPMachineClass",
		"metadata": {
			"labels": { "heritage": "deckhouse", "module": "node-manager" },
			"name": "worker-560dfd0c",
			"namespace": "d8-cloud-instance-manager"
		},
		"spec": {
			"canIpForward": true,
			"disks": [ {
				"autoDelete": true,
				"boot": true,
				"image": "img-default",
				"sizeGb": 30,
				"type": "pd-ssd"
			} ],
			"labels": { "team": "platform" },
			"machineType": "n1-standard-4",
			"metadata": [ { "key": "ssh-keys", "value": "user:ssh-rsa AAAA" } ],
			"networkInterfaces": [ {
				"disableExternalIP": true,
				"network": "kube-net",
				"subnetwork": "kube-subnet"
			} ],
			"region": "europe-west1",
			"scheduling": {
				"automaticRestart": true,
				"onHostMaintenance": "MIGRATE",
				"preemptible": false
			},
			"secretRef": { "name": "worker-560dfd0c", "namespace": "d8-cloud-instance-manager" },
			"serviceAccounts": [ {
				"email": "sa@project.iam.gserviceaccount.com",
				"scopes": [ "https://www.googleapis.com/auth/cloud-platform" ]
			} ],
			"tags": [
				"kubernetes-io-cluster-deckhouse-10413e1053ddd87840224411ac5e71",
				"kubernetes-io-role-deckhouse-worker-europe-west1-b",
				"tag-a"
			],
			"zone": "europe-west1-b"
		}
	}`)
}

func TestRenderMachineClass_OpenStackFullObject(t *testing.T) {
	assertRenderedMachineClass(t, openstackMachineClassTemplatePath, openstackRenderContext(), `{
		"apiVersion": "machine.sapcloud.io/v1alpha1",
		"kind": "OpenStackMachineClass",
		"metadata": {
			"labels": { "heritage": "deckhouse", "module": "node-manager" },
			"name": "worker-7be97df9",
			"namespace": "d8-cloud-instance-manager"
		},
		"spec": {
			"availabilityZone": "nova",
			"flavorName": "m1.large",
			"imageName": "ubuntu-22",
			"keyName": "kube-key",
			"networks": [ { "name": "internal-net", "podNetwork": true } ],
			"podNetworkCidr": "10.111.0.0/16",
			"region": "RegionOne",
			"secretRef": { "name": "worker-7be97df9", "namespace": "d8-cloud-instance-manager" },
			"securityGroups": [ "sg-base", "sg-extra" ],
			"tags": {
				"env": "prod",
				"kubernetes.io-cluster-deckhouse-aaaa-bbbb": "1",
				"kubernetes.io-role-deckhouse-worker-nova": "1"
			},
			"useConfigDrive": true
		}
	}`)
}

func TestRenderMachineClass_VsphereFullObject(t *testing.T) {
	assertRenderedMachineClass(t, vsphereMachineClassTemplatePath, vsphereRenderContext(), `{
		"apiVersion": "machine.sapcloud.io/v1alpha1",
		"kind": "VsphereMachineClass",
		"metadata": {
			"labels": { "heritage": "deckhouse", "module": "node-manager" },
			"name": "worker-83d445f9",
			"namespace": "d8-cloud-instance-manager"
		},
		"spec": {
			"clusterNameTag": "aaaa-bbbb",
			"datastore": "ds1",
			"disableTimesync": false,
			"mainNetwork": "vlan-100",
			"memory": 8192,
			"nodeRoleTag": "worker-zone-a",
			"numCPUs": 4,
			"region": "datacenter-1",
			"rootDiskSize": 20,
			"runtimeOptions": {
				"nestedHardwareVirtualization": true,
				"resourceAllocationInfo": { "memoryReservation": 6480 }
			},
			"secretRef": { "name": "worker-83d445f9", "namespace": "d8-cloud-instance-manager" },
			"sshKeys": [ "ssh-rsa AAAA" ],
			"template": "base-tmpl",
			"virtualMachineFolder": "kube/vms",
			"zone": "zone-a"
		}
	}`)
}
