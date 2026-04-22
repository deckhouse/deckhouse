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

// Checksums holds config/pki/ca hashes for a single control plane component.
type Checksums struct {
	// Config is the hash of the component static pod template and extra-files.
	// +optional
	Config string `json:"config,omitempty"`

	// PKI is the hash of PKI-related config keys (certSANs, encryption-algorithm).
	// +optional
	PKI string `json:"pki,omitempty"`

	// CA is the hash of CA certificates applied to this component.
	// In spec absent (CA is global). In status set when the component's pod restarts with the new CA.
	// +optional
	CA string `json:"ca,omitempty"`
}

// ComponentSpec holds the desired state of a single control plane component (used in CPN spec).
type ComponentSpec struct {
	// +optional
	Checksums Checksums `json:"checksums,omitempty"`
}

// ComponentStatus holds the observed state of a single control plane component (used in CPN status).
type ComponentStatus struct {
	// +optional
	Checksums Checksums `json:"checksums,omitempty"`

	// CertificatesExpirationDate maps cert file names to their NotAfter timestamps.
	// Populated via CertObserve command.
	// +optional
	CertificatesExpirationDate map[string]metav1.Time `json:"certificatesExpirationDate,omitempty"`

	// LastObservedAt is the timestamp of the last completed CertObserve for this component.
	// +optional
	LastObservedAt metav1.Time `json:"lastObservedAt,omitempty"`
}

// ComponentsSpec holds spec checksums for all control plane components.
// Zero values are allowed for etcd-arbiter nodes (etcd only).
type ComponentsSpec struct {
	// +optional
	Etcd ComponentSpec `json:"etcd,omitempty"`

	// +optional
	KubeAPIServer ComponentSpec `json:"kube-apiserver,omitempty"`

	// +optional
	KubeControllerManager ComponentSpec `json:"kube-controller-manager,omitempty"`

	// +optional
	KubeScheduler ComponentSpec `json:"kube-scheduler,omitempty"`
}

// ComponentsStatus holds status checksums and observed state for all control plane components.
type ComponentsStatus struct {
	// +optional
	Etcd ComponentStatus `json:"etcd,omitempty"`

	// +optional
	KubeAPIServer ComponentStatus `json:"kube-apiserver,omitempty"`

	// +optional
	KubeControllerManager ComponentStatus `json:"kube-controller-manager,omitempty"`

	// +optional
	KubeScheduler ComponentStatus `json:"kube-scheduler,omitempty"`
}

// Component returns a pointer to the ComponentSpec for the given component.
// Returns nil for non-static-pod components (for example CertObserver).
func (c *ComponentsSpec) Component(comp OperationComponent) *ComponentSpec {
	switch comp {
	case OperationComponentEtcd:
		return &c.Etcd
	case OperationComponentKubeAPIServer:
		return &c.KubeAPIServer
	case OperationComponentKubeControllerManager:
		return &c.KubeControllerManager
	case OperationComponentKubeScheduler:
		return &c.KubeScheduler
	}
	return nil
}

// Component returns a pointer to the ComponentStatus for the given component.
// Returns nil for non-static-pod components (for example CertObserver).
func (c *ComponentsStatus) Component(comp OperationComponent) *ComponentStatus {
	switch comp {
	case OperationComponentEtcd:
		return &c.Etcd
	case OperationComponentKubeAPIServer:
		return &c.KubeAPIServer
	case OperationComponentKubeControllerManager:
		return &c.KubeControllerManager
	case OperationComponentKubeScheduler:
		return &c.KubeScheduler
	}
	return nil
}

type ControlPlaneNodeSpec struct {
	// CAChecksum is the hash of d8-pki secret (CA certificates).
	// +optional
	CAChecksum string `json:"caChecksum,omitempty"`

	// Checksums per component
	// +optional
	Components ComponentsSpec `json:"components,omitempty"`
}

type ControlPlaneNodeStatus struct {
	// +optional
	CAChecksum string `json:"caChecksum,omitempty"`

	// +optional
	Components ComponentsStatus `json:"components,omitempty"`

	// LastObservedAt is the timestamp of the last completed Observe operation.
	// +optional
	LastObservedAt *metav1.Time `json:"lastObservedAt,omitempty"`

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
