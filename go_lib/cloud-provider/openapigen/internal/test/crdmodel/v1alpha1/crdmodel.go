// Package v1alpha1 contains a test CRD root type for openapigen CRD generation tests.
//
// +groupName=test.openapigen.io
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestResource is a test CRD resource.
//
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:storageversion
type TestResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec TestResourceSpec `json:"spec"`
}

// TestResourceSpec defines the desired state of TestResource.
type TestResourceSpec struct {
	// Host is the hostname.
	//
	// +kubebuilder:validation:Pattern=`^[a-z0-9-]+$`
	// +kubebuilder:validation:MaxLength=253
	// +deckhouse:XDocSearch=true
	Host string `json:"host"`

	// Replicas is the number of replicas.
	//
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +deckhouse:XDocExample:value="3"
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas,omitempty"`

	// Mode controls the operation mode.
	//
	// +kubebuilder:validation:Enum=active;passive;standby
	// +deckhouse:XRules=mode-check
	Mode string `json:"mode,omitempty"`
}
