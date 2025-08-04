/*
Copyright 2025 Flant JSC

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

package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SSHCredentialsSpec defines the desired state of SSHCredentials
type SSHCredentialsSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	User          string `json:"user"`
	PrivateSSHKey string `json:"privateSSHKey,omitempty"`
	// base64 encoded password for user
	SudoPasswordEncoded string `json:"sudoPasswordEncoded,omitempty"`

	//+kubebuilder:default:=22
	//+kubebuilder:validation:Minimum=1
	//+kubebuilder:validation:Maximum=65535
	SSHPort int `json:"sshPort,omitempty"`

	SSHExtraArgs string `json:"sshExtraArgs,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster

// SSHCredentials is the Schema for the sshcredentials API
type SSHCredentials struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec SSHCredentialsSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster

// SSHCredentialsList contains a list of SSHCredentials
type SSHCredentialsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SSHCredentials `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SSHCredentials{}, &SSHCredentialsList{})
}
