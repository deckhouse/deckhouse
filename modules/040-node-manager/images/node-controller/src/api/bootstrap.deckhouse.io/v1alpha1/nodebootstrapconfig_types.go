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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeBootstrapConfigSpec is intentionally empty in v1alpha1: the controller
// renders the bootstrap userdata from live cluster state at Machine creation;
// the field is reserved for future per-machine overrides.
type NodeBootstrapConfigSpec struct{}

// NodeBootstrapConfigStatus is the receipt the CAPI Machine controller waits
// on: under the v1beta2 bootstrap contract it reads dataSecretName and
// initialization.dataSecretCreated to hand the userdata to the provider.
type NodeBootstrapConfigStatus struct {
	// Conditions record why the bootstrap data is or is not available yet.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// DataSecretName names the Secret holding the rendered bootstrap userdata.
	// +optional
	DataSecretName *string `json:"dataSecretName,omitempty"`
	// Initialization carries the CAPI v1beta2 bootstrap-provider contract flag.
	// +optional
	Initialization *NodeBootstrapInitializationStatus `json:"initialization,omitempty"`
}

// NodeBootstrapInitializationStatus carries the v1beta2 bootstrap contract: the
// Machine controller waits for dataSecretCreated before reading the secret.
type NodeBootstrapInitializationStatus struct {
	// DataSecretCreated is true once the bootstrap Secret has been rendered.
	// +optional
	DataSecretCreated bool `json:"dataSecretCreated,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// NodeBootstrapConfig is the per-Machine bootstrap request the CAPI MachineSet
// clones from a NodeBootstrapConfigTemplate. The controller renders the node's
// NodeConfig userdata into a Secret and points to it through the status.
type NodeBootstrapConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeBootstrapConfigSpec   `json:"spec,omitempty"`
	Status NodeBootstrapConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type NodeBootstrapConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeBootstrapConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeBootstrapConfig{}, &NodeBootstrapConfigList{})
}
