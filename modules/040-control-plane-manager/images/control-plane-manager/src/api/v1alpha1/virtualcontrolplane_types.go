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

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type VirtualControlPlaneDatastoreRef struct {
	// Name is the datastore configuration name used by the tenant control plane.
	Name string `json:"name"`
}

type VirtualControlPlaneExpose struct {
	// Type is the Service type used to expose the tenant Kubernetes API.
	// +kubebuilder:validation:Enum=ClusterIP;LoadBalancer;NodePort
	// +kubebuilder:default=ClusterIP
	// +optional
	Type string `json:"type,omitempty"`
}

type VirtualControlPlaneKubeconfigSecretRef struct {
	// Namespace is the namespace that contains the kubeconfig Secret.
	Namespace string `json:"namespace,omitempty"`

	// Name is the kubeconfig Secret name.
	Name string `json:"name,omitempty"`
}

type VirtualControlPlaneSpec struct {
	// KubernetesVersion is the desired Kubernetes version for the tenant control plane.
	KubernetesVersion string `json:"kubernetesVersion"`

	// Replicas is the desired number of control plane replicas.
	// +kubebuilder:default=1
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// DatastoreRef points to the datastore configuration used by the tenant control plane.
	// +optional
	DatastoreRef *VirtualControlPlaneDatastoreRef `json:"datastoreRef,omitempty"`

	// Expose describes how the tenant Kubernetes API should be published.
	// +optional
	Expose *VirtualControlPlaneExpose `json:"expose,omitempty"`
}

type VirtualControlPlaneStatus struct {
	// Endpoint is the published Kubernetes API endpoint for this virtual control plane.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// KubeconfigSecretRef points to a Secret with a kubeconfig for this virtual control plane.
	// +optional
	KubeconfigSecretRef *VirtualControlPlaneKubeconfigSecretRef `json:"kubeconfigSecretRef,omitempty"`

	// ObservedKubernetesVersion is the version currently observed by the controller.
	// +optional
	ObservedKubernetesVersion string `json:"observedKubernetesVersion,omitempty"`

	// Conditions describe the current state of the virtual control plane.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=vcp
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.kubernetesVersion",description="Desired Kubernetes version"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.replicas",description="Desired number of control plane replicas"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status",description="Virtual control plane readiness"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type VirtualControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualControlPlaneSpec   `json:"spec,omitempty"`
	Status VirtualControlPlaneStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type VirtualControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualControlPlane `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VirtualControlPlane{}, &VirtualControlPlaneList{})
}
