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

	// Weight is an optional numeric weight with no Russian description marker.
	Weight int32 `json:"weight,omitempty"`
}
