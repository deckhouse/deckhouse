// Package v1alpha1 is the v1alpha1 version of a multi-version test CRD.
//
// +groupName=test.openapigen.io
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MultiVersionResource is a test CRD resource with multiple versions.
//
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
type MultiVersionResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec MultiVersionResourceSpec `json:"spec"`
}

// MultiVersionResourceSpec defines the desired state of MultiVersionResource v1alpha1.
type MultiVersionResourceSpec struct {
	// Host is the hostname.
	//
	// +kubebuilder:validation:MaxLength=253
	// +deckhouse:XDocSearch=true
	Host string `json:"host"`

	// Replicas is the number of replicas.
	//
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas,omitempty"`
}
