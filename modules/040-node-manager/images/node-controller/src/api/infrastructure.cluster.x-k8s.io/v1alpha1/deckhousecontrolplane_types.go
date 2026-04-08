package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type DeckhouseControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeckhouseControlPlaneSpec   `json:"spec,omitempty"`
	Status DeckhouseControlPlaneStatus `json:"status,omitempty"`
}

type DeckhouseControlPlaneSpec struct {
	Replicas *int32 `json:"replicas,omitempty"`
}

type DeckhouseControlPlaneStatus struct {
	Ready                       bool   `json:"ready,omitempty"`
	Initialized                 bool   `json:"initialized,omitempty"`
	Replicas                    int32  `json:"replicas,omitempty"`
	ReadyReplicas               int32  `json:"readyReplicas,omitempty"`
	UnavailableReplicas         int32  `json:"unavailableReplicas,omitempty"`
	Version                     string `json:"version,omitempty"`
	ExternalManagedControlPlane bool   `json:"externalManagedControlPlane,omitempty"`
}

// +kubebuilder:object:root=true
type DeckhouseControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeckhouseControlPlane `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DeckhouseControlPlane{}, &DeckhouseControlPlaneList{})
}
