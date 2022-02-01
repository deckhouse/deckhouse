/*
Copyright 2021 Flant JSC

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

package v1

import (
	v1 "k8s.io/api/core/v1"

	"github.com/deckhouse/deckhouse/go_lib/taints"
	"github.com/deckhouse/deckhouse/modules/040-node-manager"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type VsphereProviderClusterConfiguration struct {
	SshPublicKey         string                 `json:"sshPublicKey" yaml:"sshPublicKey"`
	RegionTagCategory    string                 `json:"regionTagCategory" yaml:"regionTagCategory"`
	ZoneTagCategory      string                 `json:"zoneTagCategory" yaml:"zoneTagCategory"`
	DisableTimesync      bool                   `json:"disableTimesync" yaml:"disableTimesync"`
	ExternalNetworkNames []string               `json:"externalNetworkNames" yaml:"externalNetworkNames"`
	InternalNetworkNames []string               `json:"internalNetworkNames" yaml:"internalNetworkNames"`
	InternalNetworkCIDR  []string               `json:"internalNetworkCIDR" yaml:"internalNetworkCIDR"`
	VmFolderPath         string                 `json:"vmFolderPath" yaml:"vmFolderPath"`
	Region               string                 `json:"region" yaml:"region"`
	Zones                []string               `json:"zones" yaml:"zones"`
	BaseResourcePool     string                 `json:"baseResourcePool" yaml:"baseResourcePool"`
	Layout               string                 `json:"layout" yaml:"layout"`
	Provider             VsphereProvider        `json:"provider" yaml:"provider"`
	Nsxt                 VsphereNsxt            `json:"nsxt" yaml:"nsxt"`
	MasterNodeGroup      VsphereMasterNodeGroup `json:"masterNodeGroup" yaml:"masterNodeGroup"`
	NodeGroups           []VsphereNodeGroup     `json:"nodeGroups" yaml:"nodeGroups"`
}

type VsphereProvider struct {
	Server   string `json:"server" yaml:"server"`
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	Insecure bool   `json:"insecure" yaml:"insecure"`
}

type VsphereMasterNodeGroup struct {
	Replicas      int32                           `json:"replicas" yaml:"replicas"`
	Zones         []string                      `json:"zones" yaml:"zones"`
	InstanceClass VsphereNodeGroupInstanceClass `json:"instanceClass" yaml:"instanceClass"`
}

type VsphereNodeGroupInstanceClass struct {
	VsphereInstanceClass
	MainNetworkIPAddresses []VsphereMainNetworkIPAddresses `json:"mainNetworkIPAddresses" yaml:"mainNetworkIPAddresses"`
}

type VsphereMainNetworkIPAddresses struct {
	Address     string                 `json:"address" yaml:"address"`
	Gateway     string                 `json:"gateway" yaml:"gateway"`
	Nameservers IPAddressesNameservers `json:"nameservers" yaml:"nameservers"`
}

type IPAddressesNameservers struct {
	Addresses []string `json:"addresses" yaml:"addresses"`
	Search    []string `json:"search" yaml:"search"`
}

type VsphereNodeGroup struct {
	Name          string                        `json:"name" yaml:"name"`
	Replicas      int32                           `json:"replicas" yaml:"replicas"`
	Zones         []string                      `json:"zones" yaml:"zones"`
	Taints		  []v1.Taint					`json:"taints" yaml:"taints"`
	NodeTemplate  nodegroupv1
	InstanceClass VsphereNodeGroupInstanceClass `json:"instanceClass" yaml:"instanceClass"``
}
