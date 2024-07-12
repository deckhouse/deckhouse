/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	ClusterMisconfiguredReason  = "ClusterMisconfigured"
	ClusterIDNotProvidedMessage = ".spec.id does not contain a valid zVirt cluster identifier"
)

// ZvirtClusterSpec defines the desired state of ZvirtCluster
type ZvirtClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ID holds zVirt cluster identifier of this ZvirtCluster.
	ID string `json:"id,omitempty"`

	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint,omitempty"`
}

// ZvirtClusterStatus defines the observed state of ZvirtCluster
type ZvirtClusterStatus struct {
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

// ZvirtCluster is the Schema for the zvirtclusters API
type ZvirtCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ZvirtClusterSpec   `json:"spec,omitempty"`
	Status ZvirtClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ZvirtClusterList contains a list of ZvirtCluster
type ZvirtClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ZvirtCluster `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &ZvirtCluster{}, &ZvirtClusterList{})
}
