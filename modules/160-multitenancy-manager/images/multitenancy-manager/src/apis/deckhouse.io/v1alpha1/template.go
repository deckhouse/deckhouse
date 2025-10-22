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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	ProjectTemplateKind     = "ProjectTemplate"
	ProjectTemplateResource = "projecttemplates"
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

	Spec ProjectTemplateSpec `json:"spec,omitempty" yaml:"spec,omitempty"`

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

type ProjectTemplateSpec struct {
	// Description of the Template
	Description string `json:"description,omitempty"`

	// ParametersSchema specification for template values (`values`) in TemplateValues.
	ParametersSchema ParametersSchema `json:"parametersSchema,omitempty" yaml:"parametersSchema,omitempty"`

	// Resource templates in `helm` format to be created when starting a new `Project` (environment).
	// Fully compatible with all `helm` functions.
	ResourcesTemplate string `json:"resourcesTemplate,omitempty" yaml:"resourcesTemplate,omitempty"`
}

func (p *ProjectTemplateSpec) DeepCopyInto(newObj *ProjectTemplateSpec) {
	*newObj = *p
	newObj.Description = p.Description
	newObj.ResourcesTemplate = p.ResourcesTemplate
	p.ParametersSchema.DeepCopyInto(&newObj.ParametersSchema)
}

type ParametersSchema struct {
	OpenAPIV3Schema map[string]interface{} `json:"openAPIV3Schema,omitempty" yaml:"openAPIV3Schema,omitempty"`
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
	for key, value := range p.OpenAPIV3Schema {
		newObj.OpenAPIV3Schema[key] = value
	}
}

type ProjectTemplateStatus struct {
	// Status message.
	Message string `json:"message,omitempty"`

	// Current state.
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
	newObj.Ready = p.Ready
	newObj.Message = p.Message
}
