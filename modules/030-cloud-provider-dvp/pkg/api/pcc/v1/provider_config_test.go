// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1

import (
	"encoding/json"
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

func TestDVPProviderClusterConfigurationImplementsContract(t *testing.T) {
	t.Parallel()
	var _ cpapi.ProviderClusterConfigObject = (*DVPProviderClusterConfiguration)(nil)
	pcc := &DVPProviderClusterConfiguration{
		MasterNodeGroup: DVPMasterNodeGroup{Replicas: 3},
		NodeGroups: []DVPStaticNodeGroup{
			{Name: "worker", Replicas: 1},
		},
	}
	if !pcc.HasMasterNodeGroup() {
		t.Fatal("HasMasterNodeGroup() = false, want true")
	}
	names := pcc.NodeGroupNames()
	if len(names) != 1 || names[0] != "worker" {
		t.Fatalf("NodeGroupNames() = %v, want [worker]", names)
	}
}

func TestDVPProviderClusterConfigurationContractAbsent(t *testing.T) {
	t.Parallel()
	empty := &DVPProviderClusterConfiguration{}
	if empty.HasMasterNodeGroup() {
		t.Fatal("HasMasterNodeGroup() on zero value = true, want false")
	}
	var nilPCC *DVPProviderClusterConfiguration
	if nilPCC.HasMasterNodeGroup() {
		t.Fatal("HasMasterNodeGroup() on nil must be false")
	}
	if nilPCC.NodeGroupNames() != nil {
		t.Fatal("NodeGroupNames() on nil must be nil")
	}
}

func TestDVPProviderClusterConfigurationDecodesClusterConfiguration(t *testing.T) {
	t.Parallel()
	raw := `{
		"apiVersion": "deckhouse.io/v1",
		"kind": "DVPClusterConfiguration",
		"layout": "Standard",
		"sshPublicKey": "ssh-rsa AAA",
		"region": "r1",
		"zones": ["zone-a", "zone-b"],
		"masterNodeGroup": {
			"replicas": 3,
			"zones": ["zone-a"],
			"instanceClass": {
				"virtualMachine": {
					"virtualMachineClassName": "generic",
					"ipAddresses": ["10.66.30.100", "10.66.30.101", "10.66.30.102"]
				},
				"rootDisk": {
					"size": "10Gi",
					"storageClass": "linstor-thin-r1",
					"image": {"kind": "ClusterVirtualImage", "name": "ubuntu-2204"}
				},
				"etcdDisk": {"size": "10Gi", "storageClass": "linstor-thin-r1"}
			}
		},
		"nodeGroups": [
			{"name": "worker", "replicas": 1, "instanceClass": {"virtualMachine": {"virtualMachineClassName": "generic"}}}
		],
		"provider": {
			"kubeconfigDataBase64": "ZXhhbXBsZQo=",
			"namespace": "dvp-cluster",
			"networkPolicy": "Isolated"
		}
	}`
	var pcc DVPProviderClusterConfiguration
	if err := json.Unmarshal([]byte(raw), &pcc); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if pcc.Provider.KubeconfigDataBase64 != "ZXhhbXBsZQo=" {
		t.Fatalf("Provider.KubeconfigDataBase64 = %q, want ZXhhbXBsZQo=", pcc.Provider.KubeconfigDataBase64)
	}
	if pcc.MasterNodeGroup.Replicas != 3 {
		t.Fatalf("MasterNodeGroup.Replicas = %d, want 3", pcc.MasterNodeGroup.Replicas)
	}
	if len(pcc.MasterNodeGroup.InstanceClass.VirtualMachine.IPAddresses) != 3 {
		t.Fatalf("MasterNodeGroup IP addresses = %v, want three", pcc.MasterNodeGroup.InstanceClass.VirtualMachine.IPAddresses)
	}
	if pcc.MasterNodeGroup.InstanceClass.EtcdDisk.Size != "10Gi" {
		t.Fatalf("MasterNodeGroup etcdDisk size = %q, want 10Gi", pcc.MasterNodeGroup.InstanceClass.EtcdDisk.Size)
	}
	if names := pcc.NodeGroupNames(); len(names) != 1 || names[0] != "worker" {
		t.Fatalf("NodeGroupNames() = %v, want [worker]", names)
	}
}
