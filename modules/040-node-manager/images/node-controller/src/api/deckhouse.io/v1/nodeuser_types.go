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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeUserSpec defines a system user provisioned on nodes.
type NodeUserSpec struct {
	UID           int      `json:"uid"`
	SSHPublicKey  string   `json:"sshPublicKey,omitempty"`
	SSHPublicKeys []string `json:"sshPublicKeys,omitempty"`
	PasswordHash  string   `json:"passwordHash,omitempty"`
	IsSudoer      bool     `json:"isSudoer"`
	NodeGroups    []string `json:"nodeGroups"`
	ExtraGroups   []string `json:"extraGroups,omitempty"`
}

// NodeUserStatus holds per-node provisioning errors keyed by node name.
type NodeUserStatus struct {
	Errors map[string]string `json:"errors"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// NodeUser is a system user on nodes.
type NodeUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeUserSpec   `json:"spec"`
	Status NodeUserStatus `json:"status"`
}

// +kubebuilder:object:root=true

// NodeUserList contains a list of NodeUser.
type NodeUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeUser `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeUser{}, &NodeUserList{})
}
