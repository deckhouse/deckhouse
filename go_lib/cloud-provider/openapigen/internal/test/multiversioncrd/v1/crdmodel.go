// Package v1 is the v1 version of a multi-version test CRD.
//
// +groupName=test.openapigen.io
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MultiVersionResource is a test CRD resource with multiple versions.
//
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:storageversion
type MultiVersionResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec MultiVersionResourceSpec `json:"spec"`
}

// MultiVersionResourceSpec defines the desired state of MultiVersionResource v1.
type MultiVersionResourceSpec struct {
	// Host is the hostname.
	//
	// +kubebuilder:validation:MaxLength=253
	// +deckhouse:XDocSearch=true
	Host string `json:"host"`

	// Replicas is the number of replicas.
	//
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=1
	// +deckhouse:XDocExample:value="3"
	Replicas int32 `json:"replicas,omitempty"`

	// Mode controls the operation mode.
	//
	// +kubebuilder:validation:Enum=active;passive;standby
	// +deckhouse:XRules=mode-check
	Mode string `json:"mode,omitempty"`
}
