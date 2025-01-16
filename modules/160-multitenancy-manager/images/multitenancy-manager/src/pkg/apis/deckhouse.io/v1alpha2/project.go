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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	ProjectStateError     = "Error"
	ProjectStateDeploying = "Deploying"
	ProjectStateDeployed  = "Deployed"

	ConditionTypeProjectTemplateFound     = "ProjectTemplateFound"
	ConditionTypeProjectValidated         = "Validated"
	ConditionTypeProjectResourcesUpgraded = "ResourcesUpgraded"

	ConditionTypeTrue    = "True"
	ConditionTypeFalse   = "False"
	ConditionTypeUnknown = "Unknown"

	ProjectAnnotationRequireSync = "projects.deckhouse.io/require-sync"

	ProjectFinalizer = "projects.deckhouse.io/project-exists"

	ProjectLabelVirtualProject = "projects.deckhouse.io/virtual-project"

	ResourceLabelProject  = "projects.deckhouse.io/project"
	ResourceLabelTemplate = "projects.deckhouse.io/project-template"

	ResourceLabelHeritage        = "heritage"
	ResourceHeritageMultitenancy = "multitenancy-manager"
	ResourceHeritageDeckhouse    = "deckhouse"

	ReleaseLabelHashsum = "hashsun"
)

const (
	ProjectKind     = "Project"
	ProjectResource = "projects"
)

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
	newObj.Description = p.Description
	newObj.ProjectTemplateName = p.ProjectTemplateName
	newObj.Parameters = make(map[string]interface{})
	for key, value := range p.Parameters {
		newObj.Parameters[key] = value
	}
}

type ProjectStatus struct {
	// Used namespaces
	Namespaces []string `json:"namespaces,omitempty"`

	// Observed generation
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Template generation
	TemplateGeneration int64 `json:"templateGeneration,omitempty"`

	// Project conditions
	Conditions []Condition `json:"conditions,omitempty"`

	// Current state.
	State string `json:"state,omitempty"`
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
	newObj.ObservedGeneration = p.ObservedGeneration
	newObj.TemplateGeneration = p.TemplateGeneration
	newObj.State = p.State
}

type Condition struct {
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	Type               string      `json:"type,omitempty"`
	Status             string      `json:"status,omitempty"`
	Message            string      `json:"message,omitempty"`
}

func NewCondition(condType, condStatus, condMessage string) *Condition {
	return &Condition{
		Type:               condType,
		Status:             condStatus,
		Message:            condMessage,
		LastTransitionTime: metav1.Now(),
	}
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
	newObj.Type = c.Type
	newObj.Status = c.Status
	newObj.Message = c.Message
}
