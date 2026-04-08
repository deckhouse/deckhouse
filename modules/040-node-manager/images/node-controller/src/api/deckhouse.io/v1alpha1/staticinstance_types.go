package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
type StaticInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StaticInstanceSpec   `json:"spec,omitempty"`
	Status StaticInstanceStatus `json:"status,omitempty"`
}

type StaticInstanceSpec struct {
	Address        string                       `json:"address"`
	CredentialsRef StaticInstanceCredentialsRef `json:"credentialsRef"`
}

type StaticInstanceCredentialsRef struct {
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	APIVersion string `json:"apiVersion,omitempty"`
}

type StaticInstanceStatus struct {
	CurrentStatus StaticInstanceCurrentStatus `json:"currentStatus,omitempty"`
	NodeRef       StaticInstanceNodeRef       `json:"nodeRef,omitempty"`
	MachineRef    StaticInstanceMachineRef    `json:"machineRef,omitempty"`
}

type StaticInstanceCurrentStatus struct {
	Phase          string      `json:"phase,omitempty"`
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`
}

type StaticInstanceNodeRef struct {
	Name string `json:"name,omitempty"`
}

type StaticInstanceMachineRef struct {
	Kind      string `json:"kind,omitempty"`
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

// +kubebuilder:object:root=true
type StaticInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StaticInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StaticInstance{}, &StaticInstanceList{})
}
