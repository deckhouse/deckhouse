package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

type YandexMachineTemplateSpec struct {
	Template YandexMachineTemplateResource `json:"template"`
}

type YandexMachineTemplateResource struct {
	ObjectMeta clusterv1.ObjectMeta `json:"metadata,omitempty"`
	Spec       YandexMachineSpec    `json:"spec"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type YandexMachineTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec YandexMachineTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true
type YandexMachineTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []YandexMachineTemplate `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &YandexMachineTemplate{}, &YandexMachineTemplateList{})
}
