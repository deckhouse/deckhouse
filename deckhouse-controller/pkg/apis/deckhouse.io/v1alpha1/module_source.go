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
	ModuleSourceGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: "modulesources",
	}
	ModuleSourceGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    "ModuleSource",
	}
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ModuleSource source
type ModuleSource struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of an ModuleSource.
	Spec ModuleSourceSpec `json:"spec"`

	// Status of an ModuleSource.
	Status ModuleSourceStatus `json:"status,omitempty"`
}

type ModuleSourceSpec struct {
	Registry       ModuleSourceSpecRegistry `json:"registry"`
	ReleaseChannel string                   `json:"releaseChannel"`
}

type ModuleSourceSpecRegistry struct {
	Scheme    string `json:"scheme,omitempty"`
	Repo      string `json:"repo"`
	DockerCFG string `json:"dockerCfg"`
	CA        string `json:"ca"`
}

type ModuleSourceStatus struct {
	SyncTime         metav1.Time       `json:"syncTime"`
	ModulesCount     int               `json:"modulesCount"`
	AvailableModules []AvailableModule `json:"modules"`
	Msg              string            `json:"message"`
	ModuleErrors     []ModuleError     `json:"moduleErrors" patchStrategy:"retainKeys" patchKey:"name"`
}

type AvailableModule struct {
	Name       string `json:"name"`
	Policy     string `json:"policy,omitempty"`
	Overridden bool   `json:"overridden,omitempty"`
}

type ModuleError struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}

// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ModuleSourceList is a list of ModuleSource resources
type ModuleSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ModuleSource `json:"items"`
}

type moduleSourceKind struct{}

func (in *ModuleSourceStatus) GetObjectKind() schema.ObjectKind {
	return &moduleSourceKind{}
}

func (f *moduleSourceKind) SetGroupVersionKind(_ schema.GroupVersionKind) {}
func (f *moduleSourceKind) GroupVersionKind() schema.GroupVersionKind {
	return ModuleSourceGVK
}
