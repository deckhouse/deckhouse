/*
Copyright 2023 Flant JSC

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// StaticInstanceSpec defines the desired state of StaticInstance
type StaticInstanceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Address        string                  `json:"address"`
	CredentialsRef *corev1.ObjectReference `json:"credentialsRef"`
}

// StaticInstanceStatus defines the observed state of StaticInstance
type StaticInstanceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +optional
	MachineRef *corev1.ObjectReference `json:"machineRef,omitempty"`

	// +optional
	NodeRef *corev1.ObjectReference `json:"nodeRef,omitempty"`

	// +optional
	CurrentStatus *StaticInstanceStatusCurrentStatus `json:"currentStatus,omitempty"`

	// Conditions defines current service state of the StaticInstance.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

type StaticInstanceStatusCurrentStatus struct {
	// +optional
	LastUpdateTime metav1.Time `json:"lastUpdateTime"`

	// +optional
	// +kubebuilder:validation:Enum=Error;Pending;Bootstrapping;Running;Cleaning
	Phase StaticInstanceStatusCurrentStatusPhase `json:"phase"`
}

type StaticInstanceStatusCurrentStatusPhase string

const (
	StaticInstanceStatusCurrentStatusPhaseError         StaticInstanceStatusCurrentStatusPhase = "Error"
	StaticInstanceStatusCurrentStatusPhasePending       StaticInstanceStatusCurrentStatusPhase = "Pending"
	StaticInstanceStatusCurrentStatusPhaseBootstrapping StaticInstanceStatusCurrentStatusPhase = "Bootstrapping"
	StaticInstanceStatusCurrentStatusPhaseRunning       StaticInstanceStatusCurrentStatusPhase = "Running"
	StaticInstanceStatusCurrentStatusPhaseCleaning      StaticInstanceStatusCurrentStatusPhase = "Cleaning"
)

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.currentStatus.phase",description="Static instance state"
//+kubebuilder:printcolumn:name="Node",type="string",JSONPath=".status.nodeRef.name",description="Node associated with this static instance"
//+kubebuilder:printcolumn:name="Machine",type="string",JSONPath=".status.machineRef.name",description="Static machine associated with this static instance"

// StaticInstance is the Schema for the staticinstances API
type StaticInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StaticInstanceSpec   `json:"spec,omitempty"`
	Status StaticInstanceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster

// StaticInstanceList contains a list of StaticInstance
type StaticInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StaticInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StaticInstance{}, &StaticInstanceList{})
}

// GetConditions gets the StaticInstance status conditions
func (r *StaticInstance) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the StaticInstance status conditions
func (r *StaticInstance) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}
