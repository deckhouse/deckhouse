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

type ProjectTypeSpec struct {
	// 	List of ServiceAccounts, Groups and Users to provide access to the created project (isolated environment).
	Subjects []AuthorizationRule `json:"subjects,omitempty"`

	// +optional
	// Labels and annotations that apply to the generated Project namespaces.
	NamespaceMetadata NamespaceMetadata `json:"namespaceMetadata,omitempty"`

	// OpenAPI specification for template values (`values`) in resourcesTemplate.
	OpenAPI map[string]*apiext.JSON `json:"openAPI,omitempty"`

	// Resource templates in `helm` format to be created when starting a new `Project` (environment).
	// Fully compatible with all `helm` functions.
	//
	// it is possible to use several types of `values`:
	// - `{{ .projectName }}` stores the name `Project` (see [Creating a Isolated Environment](usage.html#create-an-isolated-environment)).
	// - `{{ .projectTypeName }}` stores the name of the `ProjectType`.
	// - `{{ .params }}` stores a dictionary of custom values, described in `.spec.openAPI` and defined in the `Project` `.spec.template` field.
	//
	// > **Note!** Specifying `.metadata.namespace` fields for objects is optional,
	// > as they are automatically setted with the name of the created `Project` (see [Creating an isolated environment](usage.html#create-an-isolated-environment))).
	ResourcesTemplate string `json:"resourcesTemplate,omitempty"`
}

type NamespaceMetadata struct {
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// +kubebuilder:validation:OneOf=../ee/modules/160-multitenancy-manager/hooks/apis/deckhouse.io/v1alpha1/projecttype_types.go=OneOfForSubjects
type AuthorizationRule struct {
	// +kubebuilder:validation:Enum=ServiceAccount;User;Group
	// Kind of the target resource to apply access to project (`ServiceAccount`, `Group` or `User`).
	Kind string `json:"kind,omitempty"`

	// +kubebuilder:validation:MinLength=1
	// The name of the target resource to apply access to the project.
	Name string `json:"name,omitempty"`

	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern="[a-z0-9]([-a-z0-9]*[a-z0-9])?"
	// The namespace of the target resource to apply Project access to.
	// Required only when using `ServiceAccount` from another NS.
	Namespace string `json:"namespace,omitempty"`

	// +kubebuilder:validation:Enum=User;PrivilegedUser;Editor;Admin
	// Role name from [user-authz module](../../modules/140-user-authz/cr.html#clusterauthorizationrule-v1-spec-accesslevel)
	Role string `json:"role,omitempty"`
}

type ProjectTypeStatus struct {
	// Summary about ProjectType status.
	StatusSummary StatusSummary `json:"statusSummary,omitempty"`
}

type StatusSummary struct {
	// Status message.
	Message string `json:"message,omitempty"`

	// Summary of the status (ready or not ready).
	Status bool `json:"status,omitempty"`
}

// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels=module=deckhouse;heritage=deckhouse
// +kubebuilder:resource:shortName=pt,scope=Cluster
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.statusSummary.status`
// +kubebuilder:printcolumn:name="Message",type=string,JSONPath=`.status.statusSummary.message`
type ProjectType struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ProjectTypeSpec `json:"spec,omitempty"`

	Status ProjectTypeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type ProjectTypeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectType `json:"items"`
}

// This const is used for controller-gen fork
// TODO (alex123012): remove after closing https://github.com/deckhouse/deckhouse/issues/4251
const OneOfForSubjects = `
- required: [kind, name, namespace, role]
  properties:
    kind:
      enum: [ServiceAccount]
    name: {}
    namespace: {}
    role: {}
- required: [kind, name, role]
  properties:
    kind:
      enum: [User,Group]
    name: {}
    role: {}
`
