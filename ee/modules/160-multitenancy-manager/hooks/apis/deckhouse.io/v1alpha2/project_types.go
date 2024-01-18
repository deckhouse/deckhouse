/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1alpha2

import (
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ProjectSpec struct {
	// Description of the Project
	Description string `json:"description,omitempty"`

	// Name of ProjectTemplate to use to create Project
	TemplateName string `json:"templateName,omitempty"`

	// Values for resource templates from ProjectTemplate
	// in helm values format that map to the open-api specification
	// from the ValuesSchema ProjectTemplate field
	TemplateValues map[string]interface{} `json:"templateValues,omitempty"`

	// 	List of ServiceAccounts, Groups and Users to provide access to the created project (isolated environment).
	AuthorizationRules []AuthorizationRule `json:"authorizationRules,omitempty" yaml:"authorizationRules,omitempty"`

	// DedicatedNodes
	DedicatedNodes DedicatedNode `json:"dedicatedNodes,omitempty"`
}

type AuthorizationRule struct {
	// Kind of the target resource to apply access to project (`ServiceAccount`, `Group` or `User`).
	Kind string `json:"kind,omitempty" yaml:"kind,omitempty"`

	// The name of the target resource to apply access to the project.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// The namespace of the target resource to apply Project access to.
	// Required only when using `ServiceAccount` from another NS.
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`

	// Role name from [user-authz module](../../modules/140-user-authz/cr.html#clusterauthorizationrule-v1-spec-accesslevel)
	Role string `json:"role,omitempty" yaml:"role,omitempty"`
}

type DedicatedNode struct {
	LabelSelector *metav1.LabelSelector `json:"labelSelector"`
	Tolerations   []apiv1.Toleration    `json:"tolerations,omitempty"`
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
