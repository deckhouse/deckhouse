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

package v1alpha3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	ProjectNamespaceKind     = "ProjectNamespace"
	ProjectNamespaceResource = "projectnamespaces"

	// ProjectNamespaceFinalizer ensures the controller removes the created namespace before the
	// ProjectNamespace object disappears.
	ProjectNamespaceFinalizer = "projects.deckhouse.io/project-namespace"

	// ResourceLabelProjectNamespace marks the additional Namespace created by the controller with the
	// name of the ProjectNamespace that claimed it, so the namespace can be found and garbage-collected.
	ResourceLabelProjectNamespace = "projects.deckhouse.io/project-namespace"

	ProjectNamespaceConditionReady = "Ready"
)

// ProjectNamespace orders an additional namespace inside a project. It is a namespaced resource that
// is valid only in the project's main namespace (the namespace whose name equals the project name).
// The controller creates a Namespace named "<project>-<spec.name>" labelled with the project
// ownership labels and adds it to Project.status.namespaces with kind Additional.
type ProjectNamespace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProjectNamespaceSpec   `json:"spec,omitempty"`
	Status ProjectNamespaceStatus `json:"status,omitempty"`
}

type ProjectNamespaceSpec struct {
	// Name is the suffix of the resulting namespace; the namespace is "<project>-<name>".
	Name string `json:"name"`

	// Features is an optional subset of the project features to enable for the namespace. The
	// "subset of project features" validation is a no-op placeholder: the Project resource does not
	// model features in this codebase, so the field is carried through as-is until project features
	// exist.
	Features []string `json:"features,omitempty"`
}

type ProjectNamespaceStatus struct {
	// Namespace is the name of the namespace the controller created ("<project>-<spec.name>").
	Namespace string `json:"namespace,omitempty"`

	Conditions         []Condition `json:"conditions,omitempty"`
	ObservedGeneration int64       `json:"observedGeneration,omitempty"`
}

var _ runtime.Object = &ProjectNamespace{}

func (p *ProjectNamespace) DeepCopyObject() runtime.Object { return p.DeepCopy() }
func (p *ProjectNamespace) DeepCopy() *ProjectNamespace {
	if p == nil {
		return nil
	}
	newObj := new(ProjectNamespace)
	p.DeepCopyInto(newObj)
	return newObj
}
func (p *ProjectNamespace) DeepCopyInto(newObj *ProjectNamespace) {
	*newObj = *p
	p.ObjectMeta.DeepCopyInto(&newObj.ObjectMeta)
	p.Spec.DeepCopyInto(&newObj.Spec)
	p.Status.DeepCopyInto(&newObj.Status)
}

func (s *ProjectNamespaceSpec) DeepCopyInto(newObj *ProjectNamespaceSpec) {
	*newObj = *s
	if s.Features != nil {
		newObj.Features = make([]string, len(s.Features))
		copy(newObj.Features, s.Features)
	}
}

func (s *ProjectNamespaceStatus) DeepCopyInto(newObj *ProjectNamespaceStatus) {
	*newObj = *s
	newObj.ObservedGeneration = s.ObservedGeneration
	if s.Conditions != nil {
		newObj.Conditions = make([]Condition, len(s.Conditions))
		for i := range s.Conditions {
			s.Conditions[i].DeepCopyInto(&newObj.Conditions[i])
		}
	}
}

type ProjectNamespaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectNamespace `json:"items"`
}

var _ runtime.Object = &ProjectNamespaceList{}

func (p *ProjectNamespaceList) DeepCopyObject() runtime.Object { return p.DeepCopy() }
func (p *ProjectNamespaceList) DeepCopy() *ProjectNamespaceList {
	if p == nil {
		return nil
	}
	newObj := new(ProjectNamespaceList)
	p.DeepCopyInto(newObj)
	return newObj
}
func (p *ProjectNamespaceList) DeepCopyInto(newObj *ProjectNamespaceList) {
	*newObj = *p
	newObj.TypeMeta = p.TypeMeta
	p.ListMeta.DeepCopyInto(&newObj.ListMeta)
	if p.Items != nil {
		newObj.Items = make([]ProjectNamespace, len(p.Items))
		for i := range p.Items {
			p.Items[i].DeepCopyInto(&newObj.Items[i])
		}
	}
}
