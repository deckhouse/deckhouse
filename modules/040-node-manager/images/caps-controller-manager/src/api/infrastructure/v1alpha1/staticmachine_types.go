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
	"caps-controller-manager/internal/providerid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/errors"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	// MachineFinalizer allows ReconcileStaticMachine to clean up Static resources associated with StaticMachine before
	// removing it from the apiserver.
	MachineFinalizer = "staticmachine.infrastructure.cluster.x-k8s.io"
)

// StaticMachineSpec defines the desired state of StaticMachine
type StaticMachineSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +optional
	ProviderID providerid.ProviderID `json:"providerID,omitempty"`

	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

// StaticMachineStatus defines the observed state of StaticMachine
type StaticMachineStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +optional
	Ready bool `json:"ready,omitempty"`

	// +optional
	Addresses clusterv1.MachineAddresses `json:"addresses,omitempty"`

	// +optional
	FailureReason *errors.MachineStatusError `json:"failureReason,omitempty"`

	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Conditions defines current service state of the StaticMachine.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Machine ready status"
//+kubebuilder:printcolumn:name="ProviderID",type="string",JSONPath=".spec.providerID",description="Static instance ID"
//+kubebuilder:printcolumn:name="Machine",type="string",JSONPath=".metadata.ownerReferences[?(@.kind==\"Machine\")].name",description="Machine object which owns with this StaticMachine"

// StaticMachine is the Schema for the staticmachines API
type StaticMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StaticMachineSpec   `json:"spec"`
	Status StaticMachineStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// StaticMachineList contains a list of StaticMachine
type StaticMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StaticMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StaticMachine{}, &StaticMachineList{})
}

// GetConditions gets the StaticInstance status conditions
func (r *StaticMachine) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the StaticInstance status conditions
func (r *StaticMachine) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}
