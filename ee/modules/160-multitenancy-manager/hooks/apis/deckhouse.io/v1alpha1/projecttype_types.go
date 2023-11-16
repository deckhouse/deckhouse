/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ProjectTypeSpec struct {
	// 	List of ServiceAccounts, Groups and Users to provide access to the created project (isolated environment).
	Subjects []AuthorizationRule `json:"subjects,omitempty" yaml:"subjects,omitempty"`

	// Labels and annotations that apply to the generated Project namespaces.
	NamespaceMetadata NamespaceMetadata `json:"namespaceMetadata,omitempty" yaml:"namespaceMetadata,omitempty"`

	// OpenAPI specification for template values (`values`) in resourcesTemplate.
	OpenAPI map[string]interface{} `json:"openAPI,omitempty" yaml:"openAPI,omitempty"`

	// Resource templates in `helm` format to be created when starting a new `Project` (environment).
	// Fully compatible with all `helm` functions.
	ResourcesTemplate string `json:"resourcesTemplate,omitempty" yaml:"resourcesTemplate,omitempty"`
}

type ProjectTypeStatus struct {
	// Status message.
	Message string `json:"message,omitempty"`

	// Summary of the status.
	Ready bool `json:"ready,omitempty"`
}

type ProjectType struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ProjectTypeSpec `json:"spec,omitempty"`

	Status ProjectTypeStatus `json:"status,omitempty"`
}

type ProjectTypeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectType `json:"items"`
}
