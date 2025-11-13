/*
Copyright 2025 Flant JSC

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
)

// DeckhouseClusterSpec defines the desired state of DeckhouseCluster.
type DeckhouseClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint,omitempty"`
}

// DeckhouseClusterStatus defines the observed state of DeckhouseCluster.
type DeckhouseClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// initialization provides observations of the Cluster initialization process.
	// NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Cluster provisioning.
	// +optional
	Initialization ClusterInitializationStatus `json:"initialization,omitempty,omitzero"`

	// +optional
	Ready bool `json:"ready,omitempty"`

	// +optional
	FailureReason string `json:"failureReason,omitempty"`

	// +optional
	FailureMessage string `json:"failureMessage,omitempty"`
}

// ClusterInitializationStatus provides observations of the Cluster initialization process.
// NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Cluster provisioning.
// +kubebuilder:validation:MinProperties=1
type ClusterInitializationStatus struct {
	// infrastructureProvisioned is true when the infrastructure provider reports that Cluster's infrastructure is fully provisioned.
	// NOTE: this field is part of the Cluster API contract, and it is used to orchestrate provisioning.
	// The value of this field is never updated after provisioning is completed.
	// +optional
	InfrastructureProvisioned *bool `json:"infrastructureProvisioned,omitempty"`

	// controlPlaneInitialized denotes when the control plane is functional enough to accept requests.
	// This information is usually used as a signal for starting all the provisioning operations that depends on
	// a functional API server, but do not require a full HA control plane to exists, like e.g. join worker Machines,
	// install core addons like CNI, CPI, CSI etc.
	// NOTE: this field is part of the Cluster API contract, and it is used to orchestrate provisioning.
	// The value of this field is never updated after initialization is completed.
	// +optional
	ControlPlaneInitialized *bool `json:"controlPlaneInitialized,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// DeckhouseCluster is the Schema for the deckhouseclusters API.
type DeckhouseCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeckhouseClusterSpec   `json:"spec,omitempty"`
	Status DeckhouseClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DeckhouseClusterList contains a list of DeckhouseCluster.
type DeckhouseClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeckhouseCluster `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &DeckhouseCluster{}, &DeckhouseClusterList{})
}
