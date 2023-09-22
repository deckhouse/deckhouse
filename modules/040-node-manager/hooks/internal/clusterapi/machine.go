package clusterapi

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Machine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MachineSpec   `json:"spec,omitempty"`
	Status MachineStatus `json:"status,omitempty"`
}

type MachineSpec struct{}

type MachineStatus struct {
	NodeRef *corev1.ObjectReference `json:"nodeRef,omitempty"`

	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
	Phase       MachinePhase `json:"phase,omitempty"`
}
