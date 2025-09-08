// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Important: Run "make" to regenerate code after modifying this file
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=cwhc

// ConversionWebhook is the Schema for the conversionwebhooks API
type ConversionWebhook struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// +optional
	Context []Context `json:"context,omitempty"`

	// spec defines the desired state of ConversionWebhook
	// +required
	Conversions []ConversionRule `json:"conversions,omitempty"`

	// status defines the observed state of ConversionWebhook
	// +optional
	Status ConversionWebhookStatus `json:"status,omitempty,omitzero"`
}

type ConversionRule struct {
	FromVersion string                   `json:"fromVersion"`
	ToVersion   string                   `json:"toVersion"`
	Handler     ConversionWebhookHandler `json:"handler"`
}

type ConversionWebhookHandler struct {
	// this is a python script handler for object
	Python string `json:"python,omitempty"`
}

// ConversionWebhookSpec defines the desired state of ConversionWebhook
type ConversionWebhookSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// foo is an example field of ConversionWebhook. Edit conversionwebhook_types.go to remove/update
	// +optional
	Foo *string `json:"foo,omitempty"`
}

// ConversionWebhookStatus defines the observed state of ConversionWebhook.
type ConversionWebhookStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// ConversionWebhookList contains a list of ConversionWebhook
type ConversionWebhookList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ConversionWebhook `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ConversionWebhook{}, &ConversionWebhookList{})
}
