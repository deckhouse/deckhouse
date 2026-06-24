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
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="result",type="string",JSONPath=".status.result",description="Current render status."
// +kubebuilder:metadata:labels="heritage=deckhouse"
// +kubebuilder:metadata:labels="app.kubernetes.io/name=deckhouse"
// +kubebuilder:metadata:labels="app.kubernetes.io/part-of=deckhouse"
// +crd-enricher:deckhouse:crd:preserveUnknownFields=false
// +crd-enricher:deckhouse:crd:minimal=true

// Defines the rendering configuration of the Deckhouse module documentation.
//
// **Deckhouse creates ModuleDocumentation resources by itself.**
type ModuleDocumentation struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ModuleDocumentationSpec `json:"spec"`

	Status ModuleDocumentationStatus `json:"status,omitempty"`
}

func (md *ModuleDocumentation) GetConditionByAddress(addr string) (ModuleDocumentationCondition, int) {
	for idx, cond := range md.Status.Conditions {
		if cond.Address == addr {
			return cond, idx
		}
	}

	// TODO: pointer?
	return ModuleDocumentationCondition{}, -1
}

type ModuleDocumentationSpec struct {
	// Module version.
	// +crd-enricher:deckhouse:documentation:examples=v1.0.0
	Version string `json:"version"`
	// Path to the module version.
	Path string `json:"path,omitempty"`
	// Module version checksum.
	Checksum string `json:"checksum,omitempty"`
}

type ModuleDocumentationStatus struct {
	// +optional
	// +crd-enricher:raw:x-kubernetes-patch-strategy=retainKeys
	Conditions   []ModuleDocumentationCondition           `json:"conditions,omitempty" patchStrategy:"retainKeys" patchKey:"address"`
	RenderResult ModuleDocumentationConditionRenderResult `json:"result,omitempty"`
}

type ModuleDocumentationCondition struct {
	Type               ModuleDocumentationConditionType `json:"type,omitempty"`
	Version            string                           `json:"version,omitempty"`
	Checksum           string                           `json:"checksum,omitempty"`
	Address            string                           `json:"address,omitempty"`
	LastTransitionTime metav1.Time                      `json:"lastTransitionTime,omitempty"`
	Message            string                           `json:"message,omitempty"`
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

// +kubebuilder:object:root=true

// ModuleDocumentationList is a list of ModuleDocumentation resources
type ModuleDocumentationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ModuleDocumentation `json:"items"`
}
