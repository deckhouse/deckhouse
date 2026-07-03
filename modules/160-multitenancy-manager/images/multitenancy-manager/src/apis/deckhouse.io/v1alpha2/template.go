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

package v1alpha2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	grantsv1alpha1 "controller/api/v1alpha1"
)

const (
	ProjectTemplateKind     = "ProjectTemplate"
	ProjectTemplateResource = "projecttemplates"
)

// Pod Security Standard profiles, mirroring the legacy parameters.podSecurityProfile values.
const (
	PodSecurityStandardPrivileged = "Privileged"
	PodSecurityStandardBaseline   = "Baseline"
	PodSecurityStandardRestricted = "Restricted"
)

// NetworkPolicy modes, mirroring the legacy parameters.networkPolicy values.
const (
	NetworkPolicyModeIsolated      = "Isolated"
	NetworkPolicyModeNotRestricted = "NotRestricted"
)

var _ runtime.Object = &ProjectTemplate{}

type ProjectTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectTemplate `json:"items"`
}

func (p *ProjectTemplateList) DeepCopyObject() runtime.Object {
	return p.DeepCopy()
}

func (p *ProjectTemplateList) DeepCopy() *ProjectTemplateList {
	if p == nil {
		return nil
	}
	newObj := new(ProjectTemplateList)
	p.DeepCopyInto(newObj)
	return newObj
}

func (p *ProjectTemplateList) DeepCopyInto(newObj *ProjectTemplateList) {
	*newObj = *p
	newObj.TypeMeta = p.TypeMeta
	p.ListMeta.DeepCopyInto(&newObj.ListMeta)
	if p.Items != nil {
		in, out := &p.Items, &newObj.Items
		*out = make([]ProjectTemplate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

type ProjectTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec   ProjectTemplateSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status ProjectTemplateStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

func (p *ProjectTemplate) DeepCopyObject() runtime.Object {
	return p.DeepCopy()
}

func (p *ProjectTemplate) DeepCopy() *ProjectTemplate {
	if p == nil {
		return nil
	}
	newObj := new(ProjectTemplate)
	p.DeepCopyInto(newObj)
	return newObj
}

func (p *ProjectTemplate) DeepCopyInto(newObj *ProjectTemplate) {
	*newObj = *p
	newObj.TypeMeta = p.TypeMeta
	p.ObjectMeta.DeepCopyInto(&newObj.ObjectMeta)
	p.Spec.DeepCopyInto(&newObj.Spec)
	p.Status.DeepCopyInto(&newObj.Status)
}

// ProjectTemplateSpec describes a project template as a set of structured, declarative fields
// instead of a Helm string. The cluster-resource availability fields (Resources, GrantPolicies)
// are materialized into ClusterResourceGrantPolicy objects; the remaining fields are rendered into
// per-namespace objects when a Project references the template.
type ProjectTemplateSpec struct {
	// Title is a short human-readable name of the template.
	Title string `json:"title,omitempty"`

	// Description of the template's purpose.
	Description string `json:"description,omitempty"`

	// Resources are inline cluster-resource grants materialized into a managed
	// ClusterResourceGrantPolicy bound to projects of this template.
	Resources []grantsv1alpha1.GrantResource `json:"resources,omitempty"`

	// GrantPolicies references library ClusterResourceGrantPolicy objects (created without a
	// projectSelector). The controller materializes one managed policy per reference, copying its
	// resources and binding them to projects of this template.
	GrantPolicies []string `json:"grantPolicies,omitempty"`

	// PodSecurityStandard selects the Pod Security Standard profile applied to the project
	// namespaces: Privileged, Baseline or Restricted. Empty leaves the namespace label unset.
	PodSecurityStandard Param[string] `json:"podSecurityStandard,omitempty"`

	// NetworkPolicy configures the default network isolation of the project namespaces.
	NetworkPolicy *NetworkPolicySpec `json:"networkPolicy,omitempty"`

	// NodeSelector restricts the project pods to nodes matching these labels (rendered as the
	// namespace's scheduler node-selector annotation).
	NodeSelector Param[map[string]string] `json:"nodeSelector,omitempty"`

	// Tolerations are applied as the default tolerations of the project pods (rendered as the
	// namespace's scheduler default-tolerations annotation).
	Tolerations Param[[]corev1.Toleration] `json:"tolerations,omitempty"`

	// NamespaceMetadata adds extra labels and annotations to the project namespaces.
	NamespaceMetadata *NamespaceMetadata `json:"namespaceMetadata,omitempty"`

	// Features toggles optional project capabilities (monitoring, vulnerability scanning).
	Features *FeaturesSpec `json:"features,omitempty"`

	// LogShipping configures forwarding of the project logs to a cluster log destination.
	LogShipping *LogShippingSpec `json:"logShipping,omitempty"`

	// AllowedUIDs is the range of user IDs allowed in the project containers.
	AllowedUIDs Param[IDRange] `json:"allowedUIDs,omitempty"`

	// AllowedGIDs is the range of group IDs allowed in the project containers.
	AllowedGIDs Param[IDRange] `json:"allowedGIDs,omitempty"`

	// RuntimeAudit enables runtime audit rules; effective only when AllowedUIDs/AllowedGIDs are set.
	RuntimeAudit *RuntimeAuditSpec `json:"runtimeAudit,omitempty"`

	// ParametersSchema is the OpenAPI v3 schema validating Project.spec.parameters.
	ParametersSchema ParametersSchema `json:"parametersSchema,omitempty"`

	// ResourcesTemplate is the legacy Helm template string.
	//
	// Deprecated: kept for backward compatibility with v1alpha1. When the structured fields above
	// are set, the controller uses them and ignores ResourcesTemplate.
	//
	// The yaml tag is required: the helm renderer maps the spec to values via structs.Map (yaml
	// tag name), and helmlib reads .Values.projectTemplate.resourcesTemplate.
	ResourcesTemplate string `json:"resourcesTemplate,omitempty" yaml:"resourcesTemplate,omitempty"`
}

// ParamRef pairs a structured field path (for diagnostics) with the parameter it references.
type ParamRef struct {
	// Field is the dotted path of the templated field, e.g. "networkPolicy.mode".
	Field string
	// Param is the referenced parameter name (the fromParam value), possibly dotted.
	Param string
}

// FromParamRefs returns every fromParam reference set on the structured fields. It is used by the
// admission webhook to verify each referenced parameter is declared in spec.parametersSchema, and is
// safe to call on a v1alpha1-shaped template (it simply returns nothing).
func (p *ProjectTemplateSpec) FromParamRefs() []ParamRef {
	var refs []ParamRef
	add := func(field, param string) {
		if param != "" {
			refs = append(refs, ParamRef{Field: field, Param: param})
		}
	}

	add("podSecurityStandard", p.PodSecurityStandard.Ref())
	add("nodeSelector", p.NodeSelector.Ref())
	add("tolerations", p.Tolerations.Ref())
	add("allowedUIDs", p.AllowedUIDs.Ref())
	add("allowedGIDs", p.AllowedGIDs.Ref())
	if p.NetworkPolicy != nil {
		add("networkPolicy.mode", p.NetworkPolicy.Mode.Ref())
	}
	if p.NamespaceMetadata != nil {
		add("namespaceMetadata.labels", p.NamespaceMetadata.Labels.Ref())
		add("namespaceMetadata.annotations", p.NamespaceMetadata.Annotations.Ref())
	}
	if p.Features != nil {
		add("features.monitoring", p.Features.Monitoring.Ref())
		add("features.vulnerabilityScanning", p.Features.VulnerabilityScanning.Ref())
	}
	if p.LogShipping != nil {
		add("logShipping.clusterDestinationRef", p.LogShipping.ClusterDestinationRef.Ref())
	}
	if p.RuntimeAudit != nil {
		add("runtimeAudit.enabled", p.RuntimeAudit.Enabled.Ref())
	}
	return refs
}

func (p *ProjectTemplateSpec) DeepCopyInto(newObj *ProjectTemplateSpec) {
	*newObj = *p
	if p.Resources != nil {
		newObj.Resources = make([]grantsv1alpha1.GrantResource, len(p.Resources))
		for i := range p.Resources {
			p.Resources[i].DeepCopyInto(&newObj.Resources[i])
		}
	}
	if p.GrantPolicies != nil {
		newObj.GrantPolicies = make([]string, len(p.GrantPolicies))
		copy(newObj.GrantPolicies, p.GrantPolicies)
	}
	newObj.PodSecurityStandard = p.PodSecurityStandard.DeepCopyParam()
	if p.NetworkPolicy != nil {
		newObj.NetworkPolicy = p.NetworkPolicy.DeepCopy()
	}
	newObj.NodeSelector = p.NodeSelector.DeepCopyParam()
	newObj.Tolerations = p.Tolerations.DeepCopyParam()
	if p.NamespaceMetadata != nil {
		newObj.NamespaceMetadata = p.NamespaceMetadata.DeepCopy()
	}
	if p.Features != nil {
		newObj.Features = p.Features.DeepCopy()
	}
	if p.LogShipping != nil {
		newObj.LogShipping = p.LogShipping.DeepCopy()
	}
	newObj.AllowedUIDs = p.AllowedUIDs.DeepCopyParam()
	newObj.AllowedGIDs = p.AllowedGIDs.DeepCopyParam()
	if p.RuntimeAudit != nil {
		newObj.RuntimeAudit = p.RuntimeAudit.DeepCopy()
	}
	p.ParametersSchema.DeepCopyInto(&newObj.ParametersSchema)
}

// NetworkPolicySpec configures the default project network isolation.
type NetworkPolicySpec struct {
	// Mode is Isolated (deny all but in-project, dns, metrics and ingress) or NotRestricted.
	Mode Param[string] `json:"mode,omitempty"`
}

func (n *NetworkPolicySpec) DeepCopy() *NetworkPolicySpec {
	if n == nil {
		return nil
	}
	newObj := new(NetworkPolicySpec)
	newObj.Mode = n.Mode.DeepCopyParam()
	return newObj
}

// NamespaceMetadata holds extra labels and annotations for the project namespaces. Each map may be a
// literal or a fromParam reference.
type NamespaceMetadata struct {
	Labels      Param[map[string]string] `json:"labels,omitempty"`
	Annotations Param[map[string]string] `json:"annotations,omitempty"`
}

func (n *NamespaceMetadata) DeepCopy() *NamespaceMetadata {
	if n == nil {
		return nil
	}
	newObj := new(NamespaceMetadata)
	newObj.Labels = n.Labels.DeepCopyParam()
	newObj.Annotations = n.Annotations.DeepCopyParam()
	return newObj
}

// FeaturesSpec toggles optional project capabilities. A zero (unset) Param leaves the feature off;
// the built-in templates wire each toggle to a fromParam so the per-project default applies.
type FeaturesSpec struct {
	// Monitoring enables extended monitoring for the project.
	Monitoring Param[bool] `json:"monitoring,omitempty"`

	// VulnerabilityScanning enables periodic vulnerability scans for the project.
	VulnerabilityScanning Param[bool] `json:"vulnerabilityScanning,omitempty"`
}

func (f *FeaturesSpec) DeepCopy() *FeaturesSpec {
	if f == nil {
		return nil
	}
	newObj := new(FeaturesSpec)
	newObj.Monitoring = f.Monitoring.DeepCopyParam()
	newObj.VulnerabilityScanning = f.VulnerabilityScanning.DeepCopyParam()
	return newObj
}

// LogShippingSpec configures forwarding of project logs to a cluster log destination.
type LogShippingSpec struct {
	// ClusterDestinationRef is the name of the ClusterLogDestination to ship logs to.
	ClusterDestinationRef Param[string] `json:"clusterDestinationRef,omitempty"`
}

func (l *LogShippingSpec) DeepCopy() *LogShippingSpec {
	if l == nil {
		return nil
	}
	newObj := new(LogShippingSpec)
	newObj.ClusterDestinationRef = l.ClusterDestinationRef.DeepCopyParam()
	return newObj
}

// IDRange is an inclusive range of user or group IDs.
type IDRange struct {
	Min int64 `json:"min"`
	Max int64 `json:"max"`
}

func (r *IDRange) DeepCopy() *IDRange {
	if r == nil {
		return nil
	}
	newObj := new(IDRange)
	*newObj = *r
	return newObj
}

// RuntimeAuditSpec enables runtime audit rules for the project.
type RuntimeAuditSpec struct {
	Enabled Param[bool] `json:"enabled,omitempty"`
}

func (r *RuntimeAuditSpec) DeepCopy() *RuntimeAuditSpec {
	if r == nil {
		return nil
	}
	newObj := new(RuntimeAuditSpec)
	newObj.Enabled = r.Enabled.DeepCopyParam()
	return newObj
}

type ParametersSchema struct {
	OpenAPIV3Schema map[string]any `json:"openAPIV3Schema,omitempty" yaml:"openAPIV3Schema,omitempty"`
}

func (p *ParametersSchema) DeepCopy() *ParametersSchema {
	if p == nil {
		return nil
	}
	newObj := new(ParametersSchema)
	p.DeepCopyInto(newObj)
	return newObj
}

func (p *ParametersSchema) DeepCopyInto(newObj *ParametersSchema) {
	*newObj = *p
	if p.OpenAPIV3Schema != nil {
		newObj.OpenAPIV3Schema = runtime.DeepCopyJSON(p.OpenAPIV3Schema)
	}
}

type ProjectTemplateStatus struct {
	// Message indicates the cause of the current status.
	Message string `json:"message,omitempty"`

	// Ready reports whether the template has been successfully validated.
	Ready bool `json:"ready,omitempty"`
}

func (p *ProjectTemplateStatus) DeepCopy() *ProjectTemplateStatus {
	if p == nil {
		return nil
	}
	newObj := new(ProjectTemplateStatus)
	p.DeepCopyInto(newObj)
	return newObj
}

func (p *ProjectTemplateStatus) DeepCopyInto(newObj *ProjectTemplateStatus) {
	*newObj = *p
}
