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

// OperationCommand defines the action to perform on a control plane component.
// +kubebuilder:validation:Enum=Update
type OperationCommand string

const (
	// OperationCommandUpdate updates the component configuration to the desired state.
	OperationCommandUpdate OperationCommand = "Update"
)

// OperationComponent identifies a control plane component targeted by the operation.
// +kubebuilder:validation:Enum=Etcd;KubeAPIServer;KubeControllerManager;KubeScheduler;HotReload;PKI
type OperationComponent string

const (
	OperationComponentEtcd                  OperationComponent = "Etcd"
	OperationComponentKubeAPIServer         OperationComponent = "KubeAPIServer"
	OperationComponentKubeControllerManager OperationComponent = "KubeControllerManager"
	OperationComponentKubeScheduler         OperationComponent = "KubeScheduler"
	OperationComponentHotReload             OperationComponent = "HotReload"
	OperationComponentPKI                   OperationComponent = "PKI"
)

// ControlPlaneOperationSpec defines the desired state of ControlPlaneOperation.
type ControlPlaneOperationSpec struct {
	// ConfigVersion is "[resourceVersion of cpm secret].[resourceVersion of pki secret]"
	// that this operation targets.
	// +kubebuilder:validation:Required
	ConfigVersion string `json:"configVersion"`

	// NodeName is the name of the control-plane node on which the operation should be executed.
	// +kubebuilder:validation:Required
	NodeName string `json:"nodeName"`

	// Component is the control plane component this operation targets.
	// +kubebuilder:validation:Required
	Component OperationComponent `json:"component"`

	// Command defines what action to perform on the component.
	// +kubebuilder:validation:Required
	Command OperationCommand `json:"command"`

	// DesiredChecksum is the expected checksum of the component configuration after the operation.
	// The component is identified by the Component field (e.g. for HotReload it's hot-reload config checksum, for PKI it's PKI secret checksum).
	// +kubebuilder:validation:Required
	DesiredChecksum string `json:"desiredChecksum"`

	// Approved indicates whether this operation is allowed to proceed.
	// Only one operation per node may be approved at a time.
	// +kubebuilder:default=false
	Approved bool `json:"approved"`
}

// ControlPlaneOperationStatus defines the observed state of ControlPlaneOperation.
type ControlPlaneOperationStatus struct {
	// +optional
	// +listMapKey=type
	// +listType=map
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=cpo
// +kubebuilder:printcolumn:name="Node",type="string",JSONPath=".spec.nodeName",description="Target node"
// +kubebuilder:printcolumn:name="Component",type="string",JSONPath=".spec.component",description="Target component"
// +kubebuilder:printcolumn:name="Command",type="string",JSONPath=".spec.command",description="Operation command"
// +kubebuilder:printcolumn:name="Approved",type="boolean",JSONPath=".spec.approved",description="Approved for execution"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ControlPlaneOperation represents a single pending or completed action
// that must be applied to a specific component on a control-plane node.
type ControlPlaneOperation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ControlPlaneOperationSpec   `json:"spec,omitempty"`
	Status ControlPlaneOperationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ControlPlaneOperationList contains a list of ControlPlaneOperation.
type ControlPlaneOperationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ControlPlaneOperation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ControlPlaneOperation{}, &ControlPlaneOperationList{})
}
