/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

// +kubebuilder:object:generate=true
// +kubebuilder:validation:Required
// +groupName=deckhouse.io
// +versionName=v1alpha1

package v1alpha1

import (
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ProjectSpec struct {
	// Description of the Project
	Description string `json:"description,omitempty"`

	// Name of ProjectType to use to create Project
	ProjectTypeName string `json:"projectTypeName,omitempty"`

	// Values for resource templates from ProjectType
	// in helm values format that map to the open-api specification
	// from the openAPI ProjectType field
	Template map[string]*apiext.JSON `json:"template,omitempty"`
}

type ProjectStatus struct {
	// A list of Project conditions
	Conditions []Condition `json:"conditions,omitempty"`
	// Summary for the Project status
	StatusSummary StatusSummary `json:"statusSummary,omitempty"`
}

type Condition struct {
	Name    string `json:"name,omitempty"`
	Status  bool   `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
}

// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels=module=deckhouse;heritage=deckhouse
// +kubebuilder:resource:shortName=project,scope=Cluster
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.statusSummary.status`
// +kubebuilder:printcolumn:name="Description",type=string,JSONPath=`.spec.description`
// +kubebuilder:printcolumn:name="Message",type=string,JSONPath=`.status.statusSummary.message`
type Project struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProjectSpec   `json:"spec,omitempty"`
	Status ProjectStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Project `json:"items"`
}
