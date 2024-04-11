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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ runtime.Object = (*Module)(nil)
var ModuleGVK = schema.GroupVersionKind{Group: SchemeGroupVersion.Group, Version: SchemeGroupVersion.Version, Kind: "Module"}

const SourceEmbedded = "Embedded"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Module kubernetes object
type Module struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Properties ModuleProperties `json:"properties,omitempty"`

	Status ModuleStatus `json:"status,omitempty"`
}

type ModuleProperties struct {
	Weight      uint32 `json:"weight"`
	State       string `json:"state,omitempty"`
	Source      string `json:"source,omitempty"`
	Stage       string `json:"stage,omitempty"`
	Description string `json:"description,omitempty"`
}

type ModuleStatus struct {
	Status     string `json:"status"`
	Message    string `json:"message"`
	HooksState string `json:"hooksState"`
}

type moduleKind struct{}

func (in *ModuleStatus) GetObjectKind() schema.ObjectKind {
	return &moduleKind{}
}

func (mk *moduleKind) SetGroupVersionKind(_ schema.GroupVersionKind) {}
func (mk *moduleKind) GroupVersionKind() schema.GroupVersionKind {
	return ModuleGVK
}

func (m *Module) GetObjectKind() schema.ObjectKind {
	return &moduleKind{}
}

// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ModuleList is a list of Module resources
type ModuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Module `json:"items"`
}
