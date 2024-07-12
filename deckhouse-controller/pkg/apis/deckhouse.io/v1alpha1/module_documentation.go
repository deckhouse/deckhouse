/*
Copyright 2023 Flant JSC

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
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	ModuleDocumentationGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: "moduledocumentations",
	}
	ModuleDocumentationGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    "ModuleDocumentation",
	}
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ModuleDocumentation is a Module documentation rendering object.
type ModuleDocumentation struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ModuleDocumentationSpec `json:"spec"`

	Status ModuleDocumentationStatus `json:"status,omitempty"`
}

func (md *ModuleDocumentation) GetConditionByAddress(addr string) (ModuleDocumentationCondition, bool) {
	for _, cond := range md.Status.Conditions {
		if cond.Address == addr {
			return cond, true
		}
	}

	return ModuleDocumentationCondition{}, false
}

type ModuleDocumentationSpec struct {
	Version  string `json:"version,omitempty"`
	Path     string `json:"path,omitempty"`
	Checksum string `json:"checksum,omitempty"`
}

type ModuleDocumentationStatus struct {
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions   []ModuleDocumentationCondition           `json:"conditions,omitempty" patchStrategy:"retainKeys" patchKey:"address"`
	RenderResult ModuleDocumentationConditionRenderResult `json:"result,omitempty"`
}

type ModuleDocumentationCondition struct {
	// Type is the type of the condition.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	Type     ModuleDocumentationConditionType `json:"type"`
	Version  string                           `json:"version"`
	Checksum string                           `json:"checksum"`
	// Status is the status of the condition.
	// Can be True, False, Unknown.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	Address string `json:"address"`
	// Last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// Human-readable message indicating details about last transition.
	// +optional
	Message string `json:"message"`
}
type ModuleDocumentationConditionRenderResult string

const (
	ResultRendered  ModuleDocumentationConditionRenderResult = "Rendered"
	ResultPartially ModuleDocumentationConditionRenderResult = "Partially"
	ResultError     ModuleDocumentationConditionRenderResult = "Error"
)

type ModuleDocumentationConditionType string

const (
	TypeRendered   ModuleDocumentationConditionType = "Rendered"
	TypeError      ModuleDocumentationConditionType = "Error"
	TypeSuperseded ModuleDocumentationConditionType = "Superseded"
)

type ModuleDocumentationKind struct{}

func (in *ModuleDocumentationStatus) GetObjectKind() schema.ObjectKind {
	return &ModuleDocumentationKind{}
}

func (f *ModuleDocumentationKind) SetGroupVersionKind(_ schema.GroupVersionKind) {}
func (f *ModuleDocumentationKind) GroupVersionKind() schema.GroupVersionKind {
	return ModuleDocumentationGVK
}

// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ModuleDocumentationList is a list of ModuleDocumentation resources
type ModuleDocumentationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ModuleDocumentation `json:"items"`
}
