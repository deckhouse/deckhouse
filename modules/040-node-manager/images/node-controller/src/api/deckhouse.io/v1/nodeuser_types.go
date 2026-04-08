package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
type NodeUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeUserSpec   `json:"spec"`
	Status NodeUserStatus `json:"status,omitempty"`
}

type NodeUserSpec struct {
	UID           int      `json:"uid"`
	SSHPublicKey  string   `json:"sshPublicKey,omitempty"`
	SSHPublicKeys []string `json:"sshPublicKeys,omitempty"`
	PasswordHash  string   `json:"passwordHash,omitempty"`
	IsSudoer      bool     `json:"isSudoer"`
	NodeGroups    []string `json:"nodeGroups"`
	ExtraGroups   []string `json:"extraGroups,omitempty"`
}

type NodeUserStatus struct {
	Errors map[string]string `json:"errors,omitempty"`
}

// +kubebuilder:object:root=true
type NodeUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeUser `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeUser{}, &NodeUserList{})
}
