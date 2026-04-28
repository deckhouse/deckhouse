package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

type YandexClusterSpec struct {
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint,omitempty"`
	Region string `json:"region,omitempty"`
	FolderID string `json:"folderID,omitempty"`
	ZoneToSubnetIDMap map[string]string `json:"zoneToSubnetIdMap,omitempty"`
	ShouldAssignPublicIPAddress bool `json:"shouldAssignPublicIPAddress,omitempty"`
	NodeNetworkCIDR string `json:"nodeNetworkCIDR,omitempty"`
}

type ClusterInitializationStatus struct {
	Provisioned *bool `json:"provisioned,omitempty"`
}

type YandexClusterStatus struct {
	Initialization ClusterInitializationStatus `json:"initialization,omitempty,omitzero"`
	Ready bool `json:"ready,omitempty"`
	FailureReason string `json:"failureReason,omitempty"`
	FailureMessage string `json:"failureMessage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type YandexCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   YandexClusterSpec   `json:"spec,omitempty"`
	Status YandexClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type YandexClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []YandexCluster `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &YandexCluster{}, &YandexClusterList{})
}
