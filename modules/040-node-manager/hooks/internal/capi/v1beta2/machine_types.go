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

// ContractVersionedObjectReference is a reference to a resource for which the version
// is inferred from contract labels (v1beta2 pattern replacing corev1.ObjectReference).
type ContractVersionedObjectReference struct {
	// Kind of the resource being referenced.
	Kind string `json:"kind,omitempty"`

	// Name of the resource being referenced.
	Name string `json:"name,omitempty"`

	// APIGroup is the group of the resource being referenced.
	APIGroup string `json:"apiGroup,omitempty"`
}

// MachineNodeReference is a reference to the node running on the machine.
type MachineNodeReference struct {
	// Name of the node.
	Name string `json:"name,omitempty"`
}

// IsDefined returns true if the MachineNodeReference is set.
func (r *MachineNodeReference) IsDefined() bool {
	if r == nil {
		return false
	}
	return r.Name != ""
}

// MachineDeletionSpec contains configuration options for Machine deletion.
type MachineDeletionSpec struct {
	// NodeDrainTimeoutSeconds is the total amount of time in seconds that the controller
	// will spend on draining a node.
	// +optional
	NodeDrainTimeoutSeconds *int32 `json:"nodeDrainTimeoutSeconds,omitempty"`

	// NodeVolumeDetachTimeoutSeconds is the total amount of time in seconds that the controller
	// will spend on waiting for all volumes to be detached.
	// +optional
	NodeVolumeDetachTimeoutSeconds *int32 `json:"nodeVolumeDetachTimeoutSeconds,omitempty"`

	// NodeDeletionTimeoutSeconds defines how long the controller will attempt to delete
	// the Node that the Machine hosts after the Machine is marked for deletion.
	// +optional
	NodeDeletionTimeoutSeconds *int32 `json:"nodeDeletionTimeoutSeconds,omitempty"`
}

// MachineSpec defines the desired state of Machine.
type MachineSpec struct {
	// ClusterName is the name of the Cluster this object belongs to.
	ClusterName string `json:"clusterName,omitempty"`

	// Bootstrap is a reference to a local struct which encapsulates
	// fields to configure the Machine's bootstrapping mechanism.
	Bootstrap Bootstrap `json:"bootstrap,omitempty"`

	// InfrastructureRef is a required reference to a custom resource
	// offered by an infrastructure provider.
	InfrastructureRef ContractVersionedObjectReference `json:"infrastructureRef,omitempty"`

	// Version defines the desired Kubernetes version.
	// +optional
	Version string `json:"version,omitempty"`

	// ProviderID is the identification ID of the machine provided by the provider.
	// +optional
	ProviderID string `json:"providerID,omitempty"`

	// FailureDomain is the failure domain the machine will be created in.
	// +optional
	FailureDomain string `json:"failureDomain,omitempty"`

	// Deletion contains configuration options for Machine deletion.
	// +optional
	Deletion MachineDeletionSpec `json:"deletion,omitempty"`
}

// Bootstrap encapsulates fields to configure the Machine's bootstrapping mechanism.
type Bootstrap struct {
	// ConfigRef is a reference to a bootstrap provider-specific resource
	// that holds configuration details.
	// +optional
	ConfigRef ContractVersionedObjectReference `json:"configRef,omitempty"`

	// DataSecretName is the name of the secret that stores the bootstrap data script.
	// +optional
	DataSecretName *string `json:"dataSecretName,omitempty"`
}

// MachineInitializationStatus provides observations of the Machine initialization process.
type MachineInitializationStatus struct {
	// InfrastructureProvisioned is true when the infrastructure provider reports
	// that Machine's infrastructure is fully provisioned.
	// +optional
	InfrastructureProvisioned *bool `json:"infrastructureProvisioned,omitempty"`

	// BootstrapDataSecretCreated is true when the bootstrap provider reports
	// that the Machine's bootstrap secret is created.
	// +optional
	BootstrapDataSecretCreated *bool `json:"bootstrapDataSecretCreated,omitempty"`
}

// MachineDeprecatedStatus groups all the status fields that are deprecated
// and will be removed when support for v1beta1 is dropped.
type MachineDeprecatedStatus struct {
	// V1Beta1 groups deprecated v1beta1 fields.
	// +optional
	V1Beta1 *MachineV1Beta1DeprecatedStatus `json:"v1beta1,omitempty"`
}

// MachineV1Beta1DeprecatedStatus contains deprecated v1beta1 status fields.
type MachineV1Beta1DeprecatedStatus struct {
	// FailureReason will be set in the event that there is a terminal problem.
	// +optional
	FailureReason *string `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`
}

// MachineStatus defines the observed state of Machine.
type MachineStatus struct {
	// Conditions represents the observations of a Machine's current state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Initialization provides observations of the Machine initialization process.
	// +optional
	Initialization MachineInitializationStatus `json:"initialization,omitempty"`

	// NodeRef will point to the corresponding Node if it exists.
	// +optional
	NodeRef MachineNodeReference `json:"nodeRef,omitempty"`

	// LastUpdated identifies when the phase of the Machine last transitioned.
	// +optional
	LastUpdated metav1.Time `json:"lastUpdated,omitempty"`

	// Addresses is a list of addresses assigned to the machine.
	// +optional
	Addresses MachineAddresses `json:"addresses,omitempty"`

	// Phase represents the current phase of machine actuation.
	// +optional
	Phase string `json:"phase,omitempty"`

	// ObservedGeneration is the latest generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Deprecated groups all the status fields that are deprecated.
	// +optional
	Deprecated *MachineDeprecatedStatus `json:"deprecated,omitempty"`
}

// SetTypedPhase sets the Phase field to the string representation of MachinePhase.
func (m *MachineStatus) SetTypedPhase(p MachinePhase) {
	m.Phase = string(p)
}

// GetTypedPhase attempts to parse the Phase field and return
// the typed MachinePhase representation.
func (m *MachineStatus) GetTypedPhase() MachinePhase {
	switch phase := MachinePhase(m.Phase); phase {
	case
		MachinePhasePending,
		MachinePhaseProvisioning,
		MachinePhaseProvisioned,
		MachinePhaseRunning,
		MachinePhaseDeleting,
		MachinePhaseDeleted,
		MachinePhaseFailed:
		return phase
	default:
		return MachinePhaseUnknown
	}
}

// MachineAddressType describes a valid MachineAddress type.
type MachineAddressType string

const (
	MachineHostName    MachineAddressType = "Hostname"
	MachineExternalIP  MachineAddressType = "ExternalIP"
	MachineInternalIP  MachineAddressType = "InternalIP"
	MachineExternalDNS MachineAddressType = "ExternalDNS"
	MachineInternalDNS MachineAddressType = "InternalDNS"
)

// MachineAddress contains information for the node's address.
type MachineAddress struct {
	// Type is the machine address type.
	Type MachineAddressType `json:"type"`

	// Address is the machine address.
	Address string `json:"address"`
}

// MachineAddresses is a slice of MachineAddress items.
type MachineAddresses []MachineAddress

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=machines,shortName=ma,scope=Namespaced,categories=cluster-api
// +kubebuilder:subresource:status
// +kubebuilder:storageversion

// Machine is the Schema for the machines API.
type Machine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MachineSpec   `json:"spec,omitempty"`
	Status MachineStatus `json:"status,omitempty"`
}

// GetConditions returns the set of conditions for this object.
func (m *Machine) GetConditions() []metav1.Condition {
	return m.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (m *Machine) SetConditions(conditions []metav1.Condition) {
	m.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// MachineList contains a list of Machine.
type MachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Machine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Machine{}, &MachineList{})
}
