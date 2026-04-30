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

// Checksums holds the component fingerprints used to compare desired vs. applied state.
type Checksums struct {
	// Config is the fingerprint of the component static pod manifest and extra-files.
	// +optional
	Config string `json:"config,omitempty"`

	// PKI is the fingerprint of PKI-related settings of the component (certSANs, encryption-algorithm).
	// +optional
	PKI string `json:"pki,omitempty"`

	// CA is the fingerprint of CA certificates applied to the component.
	// Absent in spec (CA is global). Set in status when the component pod restarts with the new CA.
	// +optional
	CA string `json:"ca,omitempty"`
}

// ComponentSpec is the desired state of a single control plane component (used under spec.components).
type ComponentSpec struct {
	// Checksums is the desired set of fingerprints for the component.
	// +optional
	Checksums Checksums `json:"checksums,omitempty"`
}

// ComponentStatus is the observed state of a single control plane component (used under status.components).
type ComponentStatus struct {
	// Checksums is the set of fingerprints applied to the component.
	// +optional
	Checksums Checksums `json:"checksums,omitempty"`

	// CertificatesExpirationDate maps each component certificate file name to its NotAfter timestamp.
	// Populated by the CertObserve step.
	// +optional
	CertificatesExpirationDate map[string]metav1.Time `json:"certificatesExpirationDate,omitempty"`

	// LastObservedAt is the time of the last successful CertObserve step for the component.
	// +optional
	LastObservedAt metav1.Time `json:"lastObservedAt,omitempty"`
}

// ComponentsSpec describes the desired fingerprints of every control plane component on the node.
//
// Zero values are allowed for etcd-arbiter nodes (etcd only).
// If a value here differs from the matching value under status.components, the module starts a
// ControlPlaneOperation to bring the component to the desired state.
type ComponentsSpec struct {
	// Etcd is the desired state of the etcd component.
	// +optional
	Etcd ComponentSpec `json:"etcd,omitempty"`

	// KubeAPIServer is the desired state of the kube-apiserver component.
	// +optional
	KubeAPIServer ComponentSpec `json:"kube-apiserver,omitempty"`

	// KubeControllerManager is the desired state of the kube-controller-manager component.
	// +optional
	KubeControllerManager ComponentSpec `json:"kube-controller-manager,omitempty"`

	// KubeScheduler is the desired state of the kube-scheduler component.
	// +optional
	KubeScheduler ComponentSpec `json:"kube-scheduler,omitempty"`
}

// ComponentsStatus describes the observed state of every control plane component on the node:
// applied fingerprints and certificate expirations.
type ComponentsStatus struct {
	// Etcd is the observed state of the etcd component.
	// +optional
	Etcd ComponentStatus `json:"etcd,omitempty"`

	// KubeAPIServer is the observed state of the kube-apiserver component.
	// +optional
	KubeAPIServer ComponentStatus `json:"kube-apiserver,omitempty"`

	// KubeControllerManager is the observed state of the kube-controller-manager component.
	// +optional
	KubeControllerManager ComponentStatus `json:"kube-controller-manager,omitempty"`

	// KubeScheduler is the observed state of the kube-scheduler component.
	// +optional
	KubeScheduler ComponentStatus `json:"kube-scheduler,omitempty"`
}

// Component returns a pointer to the ComponentSpec for the given component.
// Returns nil for unknown components.
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
// Returns nil for unknown components.
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

// ControlPlaneNodeSpec describes the desired state of control plane components on the node.
type ControlPlaneNodeSpec struct {
	// CAChecksum is the fingerprint of the d8-pki secret (CA certificates) that must be applied to all components on the node.
	// +optional
	CAChecksum string `json:"caChecksum,omitempty"`

	// Components holds the desired configuration and PKI fingerprints for each control plane component.
	//
	// If a value here differs from the matching value under status.components, the module starts a
	// ControlPlaneOperation to bring the component to the desired state.
	// +optional
	Components ComponentsSpec `json:"components,omitempty"`
}

// ControlPlaneNodeStatus describes the observed state of control plane components on the node.
type ControlPlaneNodeStatus struct {
	// CAChecksum is the actually applied fingerprint of CA certificates.
	// +optional
	CAChecksum string `json:"caChecksum,omitempty"`

	// Components holds the observed state of each component: applied fingerprints and certificate expirations.
	// +optional
	Components ComponentsStatus `json:"components,omitempty"`

	// Conditions reflects the readiness of control plane components on the node.
	//
	// Possible condition types:
	//   - EtcdReady              — etcd is running and accepting requests (shown in the ETCD column).
	//   - APIServerReady         — kube-apiserver is running and accepting requests (APISERVER column).
	//   - ControllerManagerReady — kube-controller-manager is running (CONTROLLERMANAGER column).
	//   - SchedulerReady         — kube-scheduler is running (SCHEDULER column).
	//   - CertificatesHealthy    — all component certificates are valid and have enough lifetime left (CERTIFICATES column).
	//
	// When status is False, the cause is described in "reason" and "message".
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
// +kubebuilder:printcolumn:name="ETCD",type="string",JSONPath=".status.conditions[?(@.type=='EtcdReady')].status",description="Etcd ready"
// +kubebuilder:printcolumn:name="APISERVER",type="string",JSONPath=".status.conditions[?(@.type=='APIServerReady')].status",description="API server ready"
// +kubebuilder:printcolumn:name="CONTROLLERMANAGER",type="string",JSONPath=".status.conditions[?(@.type=='ControllerManagerReady')].status",description="Controller manager ready"
// +kubebuilder:printcolumn:name="SCHEDULER",type="string",JSONPath=".status.conditions[?(@.type=='SchedulerReady')].status",description="Scheduler ready"
// +kubebuilder:printcolumn:name="CERTIFICATES",type="string",JSONPath=".status.conditions[?(@.type=='CertificatesHealthy')].status",description="Certificates healthy"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ControlPlaneNode describes the desired and observed state of control plane components on a single node:
// etcd, kube-apiserver, kube-controller-manager, kube-scheduler.
//
// The resource is created and updated by the control-plane-manager module automatically; users do not need to create or edit it.
//
// Useful for diagnosing the health of the control plane on the node:
//   - the ETCD, APISERVER, CONTROLLERMANAGER and SCHEDULER columns of `kubectl get cpn` show component readiness;
//   - the CERTIFICATES column shows overall certificate health;
//   - spec.components contains desired fingerprints, status.components contains the actually applied ones.
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
