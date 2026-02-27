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

// ComponentChecksum holds checksum for a single control plane component
type ComponentChecksum struct {
	// +kubebuilder:validation:Required
	Checksum string `json:"checksum"`
}

// ComponentChecksums holds checksums for control plane components
type ComponentChecksums struct {
	// +kubebuilder:validation:Required
	Etcd *ComponentChecksum `json:"etcd"`

	// +kubebuilder:validation:Required
	KubeAPIServer *ComponentChecksum `json:"kube-apiserver"`

	// +kubebuilder:validation:Required
	KubeControllerManager *ComponentChecksum `json:"kube-controller-manager"`

	// +kubebuilder:validation:Required
	KubeScheduler *ComponentChecksum `json:"kube-scheduler"`
}

type ControlPlaneNodeSpec struct {
	// ConfigVersion is "[resourceVersion of cpm secret].[resourceVersion of pki secret]"
	// +kubebuilder:validation:Required
	ConfigVersion string `json:"configVersion"`

	// Checksum of PKI secret
	// +kubebuilder:validation:Required
	PKIChecksum string `json:"pkiChecksum"`

	// Checksums per component
	// +kubebuilder:validation:Required
	Components ComponentChecksums `json:"components"`

	// For reload mechanisms (e.g. in-place reload)
	// +kubebuilder:validation:Required
	HotReloadChecksum string `json:"hotReloadChecksum"`
}

type ControlPlaneNodeStatus struct {
	// ConfigVersion that is actually applied / running on the node: "[cpm secret resourceVersion].[pki secret resourceVersion]"
	// +optional
	ConfigVersion string `json:"configVersion"`

	// +optional
	PKIChecksum string `json:"pkiChecksum"`

	// +optional
	Components ComponentChecksums `json:"components,omitempty"`

	// +optional
	HotReloadChecksum string `json:"hotReloadChecksum"`

	// +optional
	// +listMapKey=type
	// +listType=map
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=cpn
// +kubebuilder:printcolumn:name="APIReady",type="string",JSONPath=".status.conditions[?(@.type=='APIServerReady')].status",description="API server ready"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

type ControlPlaneNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ControlPlaneNodeSpec   `json:"spec,omitempty"`
	Status ControlPlaneNodeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ControlPlaneNodeList contains a list of ControlPlaneNode
type ControlPlaneNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ControlPlaneNode `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ControlPlaneNode{}, &ControlPlaneNodeList{})
}
