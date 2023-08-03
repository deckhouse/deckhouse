/*
Copyright 2023.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// StaticInstanceCredentialsSpec defines the desired state of StaticInstanceCredentials
type StaticInstanceCredentialsSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	User          string `json:"user"`
	PrivateSSHKey string `json:"privateSSHKey"`
	SudoPassword  string `json:"sudoPassword,omitempty"`

	//+kubebuilder:validation:Minimum=1
	//+kubebuilder:validation:Maximum=65535
	SSHPort int `json:"sshPort,omitempty"`

	SSHExtraArgs string `json:"sshExtraArgs,omitempty"`
}

// StaticInstanceCredentialsStatus defines the observed state of StaticInstanceCredentials
type StaticInstanceCredentialsStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// StaticInstanceCredentials is the Schema for the staticinstancecredentials API
type StaticInstanceCredentials struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StaticInstanceCredentialsSpec   `json:"spec,omitempty"`
	Status StaticInstanceCredentialsStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// StaticInstanceCredentialsList contains a list of StaticInstanceCredentials
type StaticInstanceCredentialsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StaticInstanceCredentials `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StaticInstanceCredentials{}, &StaticInstanceCredentialsList{})
}
