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

// Package v1 contains the typed legacy DVPClusterConfiguration model used by validation.
package v1

import (
	"reflect"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

var (
	_ cpapi.ProviderClusterConfigObject = (*DVPProviderClusterConfiguration)(nil)
)

// DVPProviderClusterConfiguration describes the configuration of a cloud cluster in DVP.
type DVPProviderClusterConfiguration struct {
	APIVersion      string               `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind            string               `json:"kind,omitempty" yaml:"kind,omitempty"`
	Layout          string               `json:"layout,omitempty" yaml:"layout,omitempty"`
	SSHPublicKey    string               `json:"sshPublicKey,omitempty" yaml:"sshPublicKey,omitempty"`
	Region          string               `json:"region,omitempty" yaml:"region,omitempty"`
	Zones           []string             `json:"zones,omitempty" yaml:"zones,omitempty"`
	MasterNodeGroup DVPMasterNodeGroup   `json:"masterNodeGroup,omitempty" yaml:"masterNodeGroup,omitempty"`
	NodeGroups      []DVPStaticNodeGroup `json:"nodeGroups,omitempty" yaml:"nodeGroups,omitempty"`
	Provider        DVPProvider          `json:"provider,omitempty" yaml:"provider,omitempty"`
}

// DVPProvider contains settings to connect to the DVP cluster API.
type DVPProvider struct {
	KubeconfigDataBase64 string `json:"kubeconfigDataBase64,omitempty" yaml:"kubeconfigDataBase64,omitempty"`
	Namespace            string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	NetworkPolicy        string `json:"networkPolicy,omitempty" yaml:"networkPolicy,omitempty"`
}

// DVPMasterNodeGroup defines the master's NodeGroup.
type DVPMasterNodeGroup struct {
	Replicas      int              `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	Zones         []string         `json:"zones,omitempty" yaml:"zones,omitempty"`
	InstanceClass DVPInstanceClass `json:"instanceClass,omitempty" yaml:"instanceClass,omitempty"`
}

// DVPStaticNodeGroup defines a NodeGroup for creating static nodes.
type DVPStaticNodeGroup struct {
	Name          string           `json:"name,omitempty" yaml:"name,omitempty"`
	Replicas      int              `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	Zones         []string         `json:"zones,omitempty" yaml:"zones,omitempty"`
	InstanceClass DVPInstanceClass `json:"instanceClass,omitempty" yaml:"instanceClass,omitempty"`
}

// DVPInstanceClass describes virtual machine and disk parameters of a node group.
type DVPInstanceClass struct {
	VirtualMachine  DVPVirtualMachine `json:"virtualMachine,omitempty" yaml:"virtualMachine,omitempty"`
	RootDisk        DVPDisk           `json:"rootDisk,omitempty" yaml:"rootDisk,omitempty"`
	EtcdDisk        DVPDisk           `json:"etcdDisk,omitempty" yaml:"etcdDisk,omitempty"`
	AdditionalDisks []DVPDisk         `json:"additionalDisks,omitempty" yaml:"additionalDisks,omitempty"`
}

// DVPVirtualMachine describes virtual machine parameters relevant to validation.
type DVPVirtualMachine struct {
	VirtualMachineClassName string   `json:"virtualMachineClassName,omitempty" yaml:"virtualMachineClassName,omitempty"`
	IPAddresses             []string `json:"ipAddresses,omitempty" yaml:"ipAddresses,omitempty"`
}

// DVPDisk describes a virtual machine disk.
type DVPDisk struct {
	Size         string    `json:"size,omitempty" yaml:"size,omitempty"`
	StorageClass string    `json:"storageClass,omitempty" yaml:"storageClass,omitempty"`
	Image        *DVPImage `json:"image,omitempty" yaml:"image,omitempty"`
}

// DVPImage describes the image used to create a disk.
type DVPImage struct {
	Kind string `json:"kind,omitempty" yaml:"kind,omitempty"`
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
}

// HasMasterNodeGroup reports whether the masterNodeGroup section is set.
func (c *DVPProviderClusterConfiguration) HasMasterNodeGroup() bool {
	return c != nil && !reflect.DeepEqual(c.MasterNodeGroup, DVPMasterNodeGroup{})
}

// NodeGroupNames returns names of the additional node groups.
func (c *DVPProviderClusterConfiguration) NodeGroupNames() []string {
	if c == nil || len(c.NodeGroups) == 0 {
		return nil
	}
	names := make([]string, 0, len(c.NodeGroups))
	for _, nodeGroup := range c.NodeGroups {
		names = append(names, nodeGroup.Name)
	}
	return names
}
