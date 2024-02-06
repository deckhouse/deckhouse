/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ProjectTemplateSpec struct {
	// TODO. Legacy. Delete together with projectType crd.
	// List of ServiceAccounts, Groups and Users to provide access to the created project (isolated environment).
	Subjects []AuthorizationRule `json:"subjects,omitempty" yaml:"subjects,omitempty"`

	// TODO. Legacy. Delete together with projectType crd.
	// Labels and annotations that apply to the generated Project namespaces.
	NamespaceMetadata NamespaceMetadata `json:"namespaceMetadata,omitempty" yaml:"namespaceMetadata,omitempty"`

	// ParametersSchema specification for template values (`values`) in TemplateValues.
	ParametersSchema ParametersSchema `json:"parametersSchema,omitempty" yaml:"parametersSchema,omitempty"`

	// Resource templates in `helm` format to be created when starting a new `Project` (environment).
	// Fully compatible with all `helm` functions.
	ResourcesTemplate string `json:"resourcesTemplate,omitempty" yaml:"resourcesTemplate,omitempty"`
}

type ParametersSchema struct {
	OpenAPIV3Schema map[string]interface{} `json:"openAPIV3Schema,omitempty" yaml:"openAPIV3Schema,omitempty"`
}

type NamespaceMetadata struct {
	Labels map[string]string `json:"labels,omitempty" yaml:"labels"`

	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations"`
}

type ProjectTemplateStatus struct {
	// Status message.
	Message string `json:"message,omitempty"`

	// Summary of the status.
	Ready bool `json:"ready,omitempty"`
}

type ProjectTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec ProjectTemplateSpec `json:"spec,omitempty" yaml:"spec,omitempty"`

	Status ProjectTemplateStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

type ProjectTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectTemplate `json:"items"`
}
