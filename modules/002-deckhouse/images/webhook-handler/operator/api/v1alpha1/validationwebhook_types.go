/*
Copyright 2025.

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
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ValidationWebhookSpec defines the desired state of ValidationWebhook
type ValidationWebhookSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	Foo *string `json:"foo,omitempty"`

	// ValidatingWebhook describes an webhook and the resources and operations it applies to.
	// +optional
	Webhook *admissionregistrationv1.ValidatingWebhook `json:"webhook,omitempty"`

	Context []Context `json:"context,omitempty"`

	Handler Handler `json:"handler,omitempty"`
}

type Context struct {
	Context []struct {
		Name string `yaml:"name"`
	} `yaml:"context"`
	Kubernetes        interface{} `yaml:"kubernetes"`
	APIVersion        string      `yaml:"apiVersion"`
	Kind              string      `yaml:"kind"`
	NameSelector      interface{} `yaml:"nameSelector"`
	MatchNames        []string    `yaml:"matchNames"`
	LabelSelector     interface{} `yaml:"labelSelector"`
	MatchLabels       interface{} `yaml:"matchLabels"`
	Foo               string      `yaml:"foo"`
	NamespaceSelector interface{} `yaml:"namespaceSelector"`
	JqFilter          struct {
		NodeName string `yaml:"nodeName"`
	} `yaml:"jqFilter"`
}

type Handler struct {
	// this is a python script handler for object
	Python string `json:"python,omitempty"`
	// this is a cel rules handler for object
	CEL string `json:"cel,omitempty"`
}

// ValidationWebhookStatus defines the observed state of ValidationWebhook.
type ValidationWebhookStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=vwhc

// ValidationWebhook is the Schema for the validationwebhooks API
type ValidationWebhook struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of ValidationWebhook
	// +required
	Spec ValidationWebhookSpec `json:"spec"`

	// status defines the observed state of ValidationWebhook
	// +optional
	Status ValidationWebhookStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// ValidationWebhookList contains a list of ValidationWebhook
type ValidationWebhookList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ValidationWebhook `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ValidationWebhook{}, &ValidationWebhookList{})
}
