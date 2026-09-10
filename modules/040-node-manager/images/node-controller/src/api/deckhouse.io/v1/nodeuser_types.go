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

// NodeUserSpec defines the desired state of NodeUser. The authoritative schema is the CRD at
// modules/040-node-manager/crds/nodeuser.yaml; node-controller only ever reads and writes
// status.errors, the spec is here so the object round-trips unharmed through a typed client.
type NodeUserSpec struct {
	// UID is the node user ID
	// +kubebuilder:validation:Required
	UID int64 `json:"uid"`

	// SSHPublicKey is the node user SSH public key
	//
	// Deprecated: use SSHPublicKeys instead.
	// +optional
	SSHPublicKey string `json:"sshPublicKey,omitempty"`

	// SSHPublicKeys are the node user SSH public keys
	// +optional
	SSHPublicKeys []string `json:"sshPublicKeys,omitempty"`

	// PasswordHash is the hashed user password, in the /etc/shadow format
	// +optional
	PasswordHash string `json:"passwordHash,omitempty"`

	// IsSudoer specifies whether the user belongs to the sudo group
	// +optional
	IsSudoer bool `json:"isSudoer,omitempty"`

	// NodeGroups lists the NodeGroups the user is applied to
	// +optional
	NodeGroups []string `json:"nodeGroups,omitempty"`

	// ExtraGroups lists additional system groups of the user
	// +optional
	ExtraGroups []string `json:"extraGroups,omitempty"`
}

// NodeUserStatus defines the observed state of NodeUser
type NodeUserStatus struct {
	// Errors maps a node name to the user creation error reported on that node
	// +optional
	Errors map[string]string `json:"errors,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// NodeUser is the Schema for the nodeusers API
type NodeUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeUserSpec   `json:"spec,omitempty"`
	Status NodeUserStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NodeUserList contains a list of NodeUser
type NodeUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeUser `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeUser{}, &NodeUserList{})
}
