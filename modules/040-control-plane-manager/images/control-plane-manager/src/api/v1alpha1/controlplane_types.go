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

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)



// ComponentChecksum defines checksum for a specific control plane component
type ComponentChecksum struct {
	// Checksum is SHA256 hash of component manifest and its referenced files
	// +kubebuilder:validation:Required
	Checksum string `json:"checksum"`
}

// ControlPlaneComponents contains checksums for all control plane components
type ControlPlaneComponents struct {
	// Etcd component checksum (SHA256 of etcd.yaml + referenced files)
	// +optional
	Etcd *ComponentChecksum `json:"etcd,omitempty"`

	// KubeAPIServer component checksum (SHA256 of kube-apiserver.yaml + oidc-ca.crt + webhook-config.yaml + ...)
	// +optional
	KubeAPIServer *ComponentChecksum `json:"kube-apiserver,omitempty"`

	// KubeControllerManager component checksum (SHA256 of kube-controller-manager.yaml + referenced files)
	// +optional
	KubeControllerManager *ComponentChecksum `json:"kube-controller-manager,omitempty"`

	// KubeScheduler component checksum (SHA256 of kube-scheduler.yaml + scheduler-config.yaml)
	// +optional
	KubeScheduler *ComponentChecksum `json:"kube-scheduler,omitempty"`
}

// ControlPlaneConfigurationSpec defines the desired state of ControlPlaneConfiguration
type ControlPlaneConfigurationSpec struct {
	// PKIChecksum is SHA256 hash of PKI certificates from Secret d8-pki
	// +optional
	PKIChecksum string `json:"pkiChecksum,omitempty"`

	// Components contains per-component checksums
	// +optional
	Components *ControlPlaneComponents `json:"components,omitempty"`
}

// ControlPlaneConfigurationStatus defines the observed state of ControlPlaneConfiguration
type ControlPlaneConfigurationStatus struct {
	// ObservedGeneration reflects the generation of the most recently observed ControlPlaneConfiguration
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase represents the current phase of control plane configuration
	// +optional
	// +kubebuilder:validation:Enum=Pending;InProgress;Ready;Failed
	Phase string `json:"phase,omitempty"`

	// Message provides additional details about the current state
	// +optional
	Message string `json:"message,omitempty"`

	// LastUpdateTime is the last time the configuration was updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`
}
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=cpc
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ControlPlaneConfiguration is the Schema for the control plane configuration API
type ControlPlaneConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ControlPlaneConfigurationSpec   `json:"spec,omitempty"`
	Status ControlPlaneConfigurationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ControlPlaneConfigurationList contains a list of ControlPlaneConfiguration
type ControlPlaneConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ControlPlaneConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ControlPlaneConfiguration{}, &ControlPlaneConfigurationList{})
}
