package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
type SSHCredentials struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec SSHCredentialsSpec `json:"spec"`
}

type SSHCredentialsSpec struct {
	User          string `json:"user"`
	PrivateSSHKey string `json:"privateSSHKey"`
}

// +kubebuilder:object:root=true
type SSHCredentialsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SSHCredentials `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SSHCredentials{}, &SSHCredentialsList{})
}
