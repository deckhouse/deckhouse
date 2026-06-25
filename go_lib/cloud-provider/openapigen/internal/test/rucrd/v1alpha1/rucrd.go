// Package v1alpha1 contains a test CRD root type for openapigen CRD RU description tests.
//
// +groupName=ru.openapigen.io
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RUCRDResource is a synthetic test CRD resource with ru:description markers.
//
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:storageversion
type RUCRDResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec RUCRDResourceSpec `json:"spec"`
}

// RUCRDResourceSpec defines the desired state of RUCRDResource.
type RUCRDResourceSpec struct {
	// Host is the target host.
	//
	// +deckhouse:ru:description:value="Целевой хост."
	Host string `json:"host"`

	// Port is the target port.
	//
	// +deckhouse:ru:description:value="Целевой порт."
	Port int32 `json:"port"`

	// Protocol is the communication protocol.
	//
	// +deckhouse:ru:description:value="Протокол соединения."
	Protocol string `json:"protocol,omitempty"`
}
