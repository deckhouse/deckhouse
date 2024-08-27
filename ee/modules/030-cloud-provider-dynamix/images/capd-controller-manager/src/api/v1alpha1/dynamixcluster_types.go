/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DynamixClusterSpec defines the desired state of DynamixCluster
type DynamixClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ResourceGroup holds Dynamix resource group name.
	ResourceGroup string `json:"resourceGroup"`

	// ExternalNetwork holds Dynamix external network name.
	// +optional
	ExternalNetwork string `json:"externalNetwork,omitempty"`

	// InternalNetwork holds Dynamix internal network name.
	// +optional
	InternalNetwork string `json:"internalNetwork,omitempty"`

	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint,omitempty"`
}

// DynamixClusterStatus defines the observed state of DynamixCluster
type DynamixClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +optional
	Ready bool `json:"ready,omitempty"`

	// +optional
	FailureReason string `json:"failureReason,omitempty"`

	// +optional
	FailureMessage string `json:"failureMessage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// DynamixCluster is the Schema for the dynamixclusters API
type DynamixCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DynamixClusterSpec   `json:"spec,omitempty"`
	Status DynamixClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DynamixClusterList contains a list of DynamixCluster
type DynamixClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DynamixCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DynamixCluster{}, &DynamixClusterList{})
}
