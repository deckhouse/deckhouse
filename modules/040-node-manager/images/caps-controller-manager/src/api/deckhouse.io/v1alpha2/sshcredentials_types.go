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

// SSHCredentialsSpec defines the desired state of SSHCredentials.
type SSHCredentialsSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// A username to connect to the host via SSH.
	User string `json:"user"`
	// Private SSH key in PEM format encoded as base64 string.
	PrivateSSHKey string `json:"privateSSHKey,omitempty"`
	// Base64 encoded sudo password for the user.
	SudoPasswordEncoded string `json:"sudoPasswordEncoded,omitempty"`

	//+kubebuilder:default:=22
	//+kubebuilder:validation:Minimum=1
	//+kubebuilder:validation:Maximum=65535
	// A port to connect to the host via SSH.
	SSHPort int `json:"sshPort,omitempty"`

	// A list of additional arguments to pass to the openssh command.
	SSHExtraArgs string `json:"sshExtraArgs,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:storageversion
//+kubebuilder:metadata:labels="heritage=deckhouse"
//+kubebuilder:metadata:labels="module=node-manager"

// Contains credentials required by Cluster API Provider Static (CAPS) to connect over SSH. CAPS connects to the server (virtual machine) defined in the [StaticInstance](cr.html#staticinstance) custom resource to manage its state.
//
// A reference to this resource is specified in the [credentialsRef](cr.html#staticinstance-v1alpha1-spec-credentialsref) parameter of the `StaticInstance` resource.
type SSHCredentials struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec SSHCredentialsSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster

// SSHCredentialsList contains a list of SSHCredentials.
type SSHCredentialsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SSHCredentials `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SSHCredentials{}, &SSHCredentialsList{})
}
