/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1alpha1

import (
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
	Template map[string]interface{} `json:"template,omitempty"`
}

type ProjectStatus struct {
	// Status message.
	Message string `json:"message,omitempty"`

	// Summary of the status.
	State string `json:"state,omitempty"`

	// Project definition sync with cluster.
	Sync bool `json:"sync,omitempty"`
}

type Project struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProjectSpec   `json:"spec,omitempty"`
	Status ProjectStatus `json:"status,omitempty"`
}

type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Project `json:"items"`
}
