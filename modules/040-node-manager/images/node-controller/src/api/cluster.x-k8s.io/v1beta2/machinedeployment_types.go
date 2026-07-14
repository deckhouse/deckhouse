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

package v1beta2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MachineTemplateSpec describes the data needed to create a Machine from a template.
type MachineTemplateSpec struct {
	// Standard object's metadata.
	// +optional
	ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired behavior of the machine.
	// +optional
	Spec MachineSpec `json:"spec,omitempty"`
}

// MachineDeploymentSpec defines the desired state of MachineDeployment.
type MachineDeploymentSpec struct {
	// ClusterName is the name of the Cluster this object belongs to.
	ClusterName string `json:"clusterName,omitempty"`

	// Replicas is the number of desired machines.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Selector is the label selector for machines.
	Selector metav1.LabelSelector `json:"selector,omitempty"`

	// Template describes the machines that will be created.
	Template MachineTemplateSpec `json:"template,omitempty"`
}

// MachineDeploymentStatus defines the observed state of MachineDeployment.
type MachineDeploymentStatus struct {
	// Replicas is the total number of non-terminated machines targeted by this deployment.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// ReadyReplicas is the total number of ready machines targeted by this deployment.
	// +optional
	ReadyReplicas *int32 `json:"readyReplicas,omitempty"`

	// AvailableReplicas is the total number of available machines targeted by this deployment.
	// +optional
	AvailableReplicas *int32 `json:"availableReplicas,omitempty"`

	// Phase represents the current phase of a MachineDeployment.
	// +optional
	Phase string `json:"phase,omitempty"`
}

// +kubebuilder:object:root=true

// MachineDeployment is the Schema for the machinedeployments API.
type MachineDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MachineDeploymentSpec   `json:"spec,omitempty"`
	Status MachineDeploymentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MachineDeploymentList contains a list of MachineDeployment.
type MachineDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MachineDeployment `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &MachineDeployment{}, &MachineDeploymentList{})
}
