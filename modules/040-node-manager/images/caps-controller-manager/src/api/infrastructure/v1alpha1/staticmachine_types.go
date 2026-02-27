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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	"caps-controller-manager/internal/providerid"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	// MachineFinalizer allows ReconcileStaticMachine to clean up Static resources associated with StaticMachine before
	// removing it from the apiserver.
	MachineFinalizer = "staticmachine.infrastructure.cluster.x-k8s.io"
)

// StaticMachineSpec defines the desired state of StaticMachine.
type StaticMachineSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +optional
	ProviderID providerid.ProviderID `json:"providerID,omitempty"`

	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

// StaticMachineStatus defines the observed state of StaticMachine.
type StaticMachineStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +optional
	Ready bool `json:"ready,omitempty"`

	// +optional
	Addresses clusterv1.MachineAddresses `json:"addresses,omitempty"`

	// +optional
	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the StaticMachine and will contain a succinct value suitable
	// for machine interpretation.
	FailureReason *string `json:"failureReason,omitempty"`

	// +optional
	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the StaticMachine and will contain a more verbose string suitable
	// for logging and human consumption.
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Conditions defines current service state of the StaticMachine.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Initialization provides observations of the StaticMachine initialization process.
	// NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Machine provisioning.
	// +optional
	Initialization StaticMachineInitializationStatus `json:"initialization,omitempty,omitzero"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:metadata:labels="heritage=deckhouse"
//+kubebuilder:metadata:labels="module=node-manager"
//+kubebuilder:metadata:labels="cluster.x-k8s.io/provider=infrastructure-static"
//+kubebuilder:metadata:labels="cluster.x-k8s.io/v1beta2=v1alpha1"
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Machine ready status"
//+kubebuilder:printcolumn:name="ProviderID",type="string",JSONPath=".spec.providerID",description="Static instance ID"
//+kubebuilder:printcolumn:name="Machine",type="string",JSONPath=".metadata.ownerReferences[?(@.kind==\"Machine\")].name",description="Machine object which owns with this StaticMachine"

// StaticMachine is the Schema for the Cluster API Provider Static.
type StaticMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StaticMachineSpec   `json:"spec"`
	Status StaticMachineStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// StaticMachineList contains a list of StaticMachine.
type StaticMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StaticMachine `json:"items"`
}

// StaticMachineInitializationStatus provides observations of the FooMachine initialization process.
// +kubebuilder:validation:MinProperties=1
type StaticMachineInitializationStatus struct {
	// Provisioned is true when the infrastructure provider reports that the Machine's infrastructure is fully provisioned.
	// NOTE: this field is part of the Cluster API contract, and it is used to orchestrate initial Machine provisioning.
	// +optional
	Provisioned *bool `json:"provisioned,omitempty"`
}

func init() {
	SchemeBuilder.Register(&StaticMachine{}, &StaticMachineList{})
}

// GetConditions gets the StaticInstance status conditions
func (r *StaticMachine) GetConditions() []metav1.Condition {
	return r.Status.Conditions
}

// SetConditions sets the StaticInstance status conditions
func (r *StaticMachine) SetConditions(conditions []metav1.Condition) {
	r.Status.Conditions = conditions
}
