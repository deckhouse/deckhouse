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

// Package testprovider provides minimal provider resource stubs implementing
// go_lib/cloud-provider/api contracts. It exists only to instantiate generic
// validation types in tests and must not import go_lib/cloud-provider/validation/api.
package testprovider

import (
	"reflect"
	"strings"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

var (
	_ cpapi.InstanceClassObject         = (*InstanceClass)(nil)
	_ cpapi.ModuleSettingsObject        = (*Settings)(nil)
	_ cpapi.ProviderClusterConfigObject = (*ProviderClusterConfig)(nil)
)

// InstanceClass is a stub provider InstanceClass resource.
type InstanceClass struct {
	cpapi.TypeMeta   `json:",inline"`
	cpapi.ObjectMeta `json:"metadata,omitempty"`

	Spec   InstanceClassSpec   `json:"spec,omitempty"`
	Status InstanceClassStatus `json:"status,omitempty"`
}

// InstanceClassSpec holds stub instance class parameters.
type InstanceClassSpec struct {
	EtcdDisk *EtcdDisk `json:"etcdDisk,omitempty"`
}

// EtcdDisk is a stub etcd disk definition.
type EtcdDisk struct {
	Size string `json:"size,omitempty"`
}

// InstanceClassStatus holds stub instance class status fields.
type InstanceClassStatus struct {
	NodeGroupConsumers []string `json:"nodeGroupConsumers,omitempty"`
}

// GetName returns the resource name.
func (c *InstanceClass) GetName() string {
	if c == nil {
		return ""
	}

	return c.Name
}

// GroupVersionKind returns the GroupVersionKind for the resource.
func (c *InstanceClass) GroupVersionKind() cpapi.GroupVersionKind {
	if c == nil {
		return cpapi.GroupVersionKind{}
	}

	parts := strings.SplitN(c.APIVersion, "/", 2)
	if len(parts) != 2 {
		return cpapi.GroupVersionKind{Kind: c.Kind}
	}
	return cpapi.GroupVersionKind{Group: parts[0], Version: parts[1], Kind: c.Kind}
}

// GetEtcdDisk returns the etcd disk value for error reporting, or nil when the
// stub class defines no dedicated etcd disk.
func (c *InstanceClass) GetEtcdDisk() any {
	if c == nil || c.Spec.EtcdDisk == nil {
		return nil
	}
	return c.Spec.EtcdDisk
}

// GetNodeGroupConsumers returns names of NodeGroups that use the stub class.
func (c *InstanceClass) GetNodeGroupConsumers() []string {
	if c == nil {
		return nil
	}

	return c.Status.NodeGroupConsumers
}

// Settings is a stub ModuleConfig settings root.
type Settings struct {
	Provider Section `json:"provider,omitempty"`
	Nodes    Section `json:"nodes,omitempty"`
	Storage  Section `json:"storage,omitempty"`
	CCM      Section `json:"ccm,omitempty"`
}

// Section is a stub settings section.
type Section struct {
	Disabled   bool              `json:"disabled,omitempty"`
	Parameters map[string]string `json:"parameters,omitempty"`
}

// HasProviderSection reports whether the provider section is set.
func (s *Settings) HasProviderSection() bool {
	return s != nil && !reflect.DeepEqual(s.Provider, Section{})
}

// HasNodesSection reports whether the nodes section is set.
func (s *Settings) HasNodesSection() bool {
	return s != nil && !reflect.DeepEqual(s.Nodes, Section{})
}

// HasStorageSection reports whether the storage section is set.
func (s *Settings) HasStorageSection() bool {
	return s != nil && !reflect.DeepEqual(s.Storage, Section{})
}

// HasCCMSection reports whether the ccm section is set.
func (s *Settings) HasCCMSection() bool {
	return s != nil && !reflect.DeepEqual(s.CCM, Section{})
}

// ProviderClusterConfig is a stub legacy providerClusterConfiguration.
type ProviderClusterConfig struct {
	APIVersion      string           `json:"apiVersion,omitempty"`
	Kind            string           `json:"kind,omitempty"`
	MasterNodeGroup *MasterNodeGroup `json:"masterNodeGroup,omitempty"`
	NodeGroups      []NodeGroup      `json:"nodeGroups,omitempty"`
}

// MasterNodeGroup is a stub masterNodeGroup section.
type MasterNodeGroup struct {
	Replicas int `json:"replicas,omitempty"`
}

// NodeGroup is a stub nodeGroups item.
type NodeGroup struct {
	Name     string `json:"name,omitempty"`
	Replicas int    `json:"replicas,omitempty"`
}

// HasMasterNodeGroup reports whether the masterNodeGroup section is set.
func (c *ProviderClusterConfig) HasMasterNodeGroup() bool {
	return c != nil && c.MasterNodeGroup != nil
}

// NodeGroupNames returns names of the additional node groups.
func (c *ProviderClusterConfig) NodeGroupNames() []string {
	if c == nil || len(c.NodeGroups) == 0 {
		return nil
	}

	names := make([]string, 0, len(c.NodeGroups))
	for _, nodeGroup := range c.NodeGroups {
		names = append(names, nodeGroup.Name)
	}

	return names
}
