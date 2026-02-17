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
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// StaticClusterSpec defines the desired state of StaticCluster.
type StaticClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +optional
	// APIEndpoint represents a reachable Kubernetes API endpoint.
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint,omitempty"`
}

// StaticClusterStatus defines the observed state of StaticCluster.
type StaticClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +optional
	// Ready denotes that the static cluster (infrastructure) is ready.
	Ready bool `json:"ready,omitempty"`

	// +optional
	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the StaticCluster and will contain a succinct value suitable
	// for machine interpretation.
	FailureReason string `json:"failureReason,omitempty"`

	// +optional
	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the StaticCluster and will contain a more verbose string suitable
	// for logging and human consumption.
	FailureMessage string `json:"failureMessage,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:metadata:labels="heritage=deckhouse"
//+kubebuilder:metadata:labels="module=node-manager"
//+kubebuilder:metadata:labels="cluster.x-k8s.io/provider=infrastructure-static"
//+kubebuilder:metadata:labels="cluster.x-k8s.io/v1beta1=v1alpha1"

// StaticCluster is the Schema for the Cluster API Provider Static.
type StaticCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StaticClusterSpec   `json:"spec,omitempty"`
	Status StaticClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// StaticClusterList contains a list of StaticCluster.
type StaticClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StaticCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StaticCluster{}, &StaticClusterList{})
}
