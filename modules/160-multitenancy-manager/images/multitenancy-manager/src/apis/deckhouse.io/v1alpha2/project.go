/*
Copyright 2024 Flant JSC

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
	"slices"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	ProjectStateError    = "Error"
	ProjectStateDeployed = "Deployed"

	ProjectConditionProjectTemplateFound     = "ProjectTemplateFound"
	ProjectConditionProjectValidated         = "Validated"
	ProjectConditionProjectResourcesUpgraded = "ResourcesUpgraded"

	ProjectAnnotationRequireSync = "projects.deckhouse.io/require-sync"

	ProjectFinalizer = "projects.deckhouse.io/project-exists"

	ProjectLabelVirtualProject = "projects.deckhouse.io/virtual-project"

	ResourceLabelProject  = "projects.deckhouse.io/project"
	ResourceLabelTemplate = "projects.deckhouse.io/project-template"

	ResourceLabelHeritage        = "heritage"
	ResourceHeritageMultitenancy = "multitenancy-manager"
	ResourceHeritageDeckhouse    = "deckhouse"

	NamespaceAnnotationAdopt = "projects.deckhouse.io/adopt"

	ReleaseLabelHashsum = "hashsum"
)

const (
	ProjectKind     = "Project"
	ProjectResource = "projects"
)

var _ runtime.Object = &Project{}

type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Project `json:"items"`
}

func (p *ProjectList) DeepCopyObject() runtime.Object {
	return p.DeepCopy()
}
func (p *ProjectList) DeepCopy() *ProjectList {
	if p == nil {
		return nil
	}
	newObj := new(ProjectList)
	p.DeepCopyInto(newObj)
	return newObj
}
func (p *ProjectList) DeepCopyInto(newObj *ProjectList) {
	*newObj = *p
	newObj.TypeMeta = p.TypeMeta
	p.ListMeta.DeepCopyInto(&newObj.ListMeta)
	if p.Items != nil {
		in, out := &p.Items, &newObj.Items
		*out = make([]Project, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

type Project struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProjectSpec   `json:"spec,omitempty"`
	Status ProjectStatus `json:"status,omitempty"`
}

func (p *Project) DeepCopyObject() runtime.Object {
	return p.DeepCopy()
}
func (p *Project) DeepCopy() *Project {
	if p == nil {
		return nil
	}
	newObj := Project{}
	p.DeepCopyInto(&newObj)
	return &newObj
}
func (p *Project) DeepCopyInto(newObj *Project) {
	*newObj = *p
	newObj.TypeMeta = p.TypeMeta
	p.ObjectMeta.DeepCopyInto(&newObj.ObjectMeta)
	p.Spec.DeepCopyInto(&newObj.Spec)
	p.Status.DeepCopyInto(&newObj.Status)
}

type ProjectSpec struct {
	// Description of the Project
	Description string `json:"description,omitempty"`

	// Name of ProjectTemplate to use to create Project
	ProjectTemplateName string `json:"projectTemplateName,omitempty"`

	// Values for resource templates from ProjectTemplate
	// in helm values format that map to the open-api specification
	// from the ValuesSchema ProjectTemplate field
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

func (p *ProjectSpec) DeepCopy() *ProjectSpec {
	if p == nil {
		return nil
	}
	newObj := new(ProjectSpec)
	p.DeepCopyInto(newObj)
	return newObj
}
func (p *ProjectSpec) DeepCopyInto(newObj *ProjectSpec) {
	*newObj = *p
	newObj.Parameters = make(map[string]interface{})
	for key, value := range p.Parameters {
		newObj.Parameters[key] = value
	}
}

type ProjectStatus struct {
	// Used namespaces
	Namespaces []string `json:"namespaces,omitempty"`

	// Rendered resources
	Resources map[string]map[string]ResourceKind `json:"resources,omitempty"`

	// Observed generation
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Template generation
	TemplateGeneration int64 `json:"templateGeneration,omitempty"`

	// Project conditions
	Conditions []Condition `json:"conditions,omitempty"`

	// Current state.
	State string `json:"state,omitempty"`
}

func (p *Project) SetState(state string) {
	p.Status.State = state
}

func (p *Project) SetObservedGeneration(generation int64) {
	p.Status.ObservedGeneration = generation
}

func (p *Project) SetTemplateGeneration(generation int64) {
	p.Status.TemplateGeneration = generation
}

func (p *Project) AddResource(obj *unstructured.Unstructured, installed bool) {
	if p.Status.Resources == nil {
		p.Status.Resources = make(map[string]map[string]ResourceKind)
	}

	if _, exists := p.Status.Resources[obj.GetAPIVersion()]; !exists {
		p.Status.Resources[obj.GetAPIVersion()] = make(map[string]ResourceKind)
	}

	if existing, ok := p.Status.Resources[obj.GetAPIVersion()][obj.GetKind()]; ok {
		if !slices.Contains(existing.Names, obj.GetName()) {
			existing.Names = append(existing.Names, obj.GetName())
		}
		existing.Installed = installed
		p.Status.Resources[obj.GetAPIVersion()][obj.GetKind()] = existing
		return
	}

	p.Status.Resources[obj.GetAPIVersion()][obj.GetKind()] = ResourceKind{
		Installed: installed,
		Names:     []string{obj.GetName()},
	}
}

func (p *ProjectStatus) DeepCopy() *ProjectStatus {
	if p == nil {
		return nil
	}
	newObj := new(ProjectStatus)
	p.DeepCopyInto(newObj)
	return newObj
}
func (p *ProjectStatus) DeepCopyInto(newObj *ProjectStatus) {
	*newObj = *p
	if p.Conditions != nil {
		in, out := &p.Conditions, &newObj.Conditions
		*out = make([]Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if p.Namespaces != nil {
		in, out := &p.Namespaces, &newObj.Namespaces
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if p.Resources != nil {
		newObj.Resources = make(map[string]map[string]ResourceKind, len(p.Resources))
		for outerKey, innerMap := range p.Resources {
			if innerMap == nil {
				newObj.Resources[outerKey] = nil
				continue
			}

			newInnerMap := make(map[string]ResourceKind, len(innerMap))
			for innerKey, resourceKind := range innerMap {
				var newResourceKind ResourceKind
				resourceKind.DeepCopyInto(&newResourceKind)
				newInnerMap[innerKey] = newResourceKind
			}
			newObj.Resources[outerKey] = newInnerMap
		}
	}
}

type ResourceKind struct {
	Installed bool     `json:"installed"`
	Names     []string `json:"names,omitempty"`
}

func (o *ResourceKind) DeepCopyInto(newObj *ResourceKind) {
	*newObj = *o
	if o.Names != nil {
		newObj.Names = make([]string, len(o.Names))
		copy(newObj.Names, o.Names)
	}
}

type Condition struct {
	// Type is the type of the condition.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	Type string `json:"type,omitempty"`
	// Human-readable message indicating details about last transition.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	Message string `json:"message,omitempty"`
	// Status is the status of the condition.
	// Can be True, False, Unknown.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	Status corev1.ConditionStatus `json:"status,omitempty"`
	// Timestamp of when the condition was last probed.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	LastProbeTime metav1.Time `json:"lastProbeTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

func (p *Project) ClearConditions() {
	p.Status.Conditions = []Condition{}
}

func (p *Project) SetConditionTrue(condName string) {
	for idx, cond := range p.Status.Conditions {
		if cond.Type == condName {
			p.Status.Conditions[idx].LastProbeTime = metav1.Now()
			if cond.Status == corev1.ConditionFalse {
				p.Status.Conditions[idx].LastTransitionTime = metav1.Now()
				p.Status.Conditions[idx].Status = corev1.ConditionTrue
			}
			p.Status.Conditions[idx].Message = ""
			return
		}
	}

	p.Status.Conditions = append(p.Status.Conditions, Condition{
		Type:               condName,
		Status:             corev1.ConditionTrue,
		LastProbeTime:      metav1.Now(),
		LastTransitionTime: metav1.Now(),
	})
}

func (p *Project) SetConditionFalse(condName, message string) {
	for idx, cond := range p.Status.Conditions {
		if cond.Type == condName {
			p.Status.Conditions[idx].LastProbeTime = metav1.Now()
			if cond.Status == corev1.ConditionTrue {
				p.Status.Conditions[idx].LastTransitionTime = metav1.Now()
				p.Status.Conditions[idx].Status = corev1.ConditionFalse
			}
			if cond.Message != message {
				p.Status.Conditions[idx].Message = message
			}
			return
		}
	}

	p.Status.Conditions = append(p.Status.Conditions, Condition{
		Type:               condName,
		Status:             corev1.ConditionFalse,
		Message:            message,
		LastProbeTime:      metav1.Now(),
		LastTransitionTime: metav1.Now(),
	})
}

func (c *Condition) DeepCopy() *Condition {
	if c == nil {
		return nil
	}
	newObj := new(Condition)
	c.DeepCopyInto(newObj)
	return newObj
}
func (c *Condition) DeepCopyInto(newObj *Condition) {
	*newObj = *c
	c.LastTransitionTime.DeepCopyInto(&newObj.LastTransitionTime)
	c.LastProbeTime.DeepCopyInto(&newObj.LastProbeTime)
	newObj.Type = c.Type
	newObj.Status = c.Status
	newObj.Message = c.Message
}
