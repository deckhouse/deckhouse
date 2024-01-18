/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ProjectTemplateSpec struct {
	// 	List of ServiceAccounts, Groups and Users to provide access to the created project (isolated environment).
	Subjects []AuthorizationRule `json:"subjects,omitempty" yaml:"subjects,omitempty"`

	// Labels and annotations that apply to the generated Project namespaces.
	NamespaceMetadata NamespaceMetadata `json:"namespaceMetadata,omitempty" yaml:"namespaceMetadata,omitempty"`

	// ValuesSchema specification for template values (`values`) in TemplateValues.
	ValuesSchema map[string]interface{} `json:"valuesSchema,omitempty" yaml:"valuesSchema,omitempty"`

	// Resource templates in `helm` format to be created when starting a new `Project` (environment).
	// Fully compatible with all `helm` functions.
	Template string `json:"template,omitempty" yaml:"template,omitempty"`
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
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ProjectTemplateSpec `json:"spec,omitempty"`

	Status ProjectTemplateStatus `json:"status,omitempty"`
}

type ProjectTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectTemplate `json:"items"`
}
