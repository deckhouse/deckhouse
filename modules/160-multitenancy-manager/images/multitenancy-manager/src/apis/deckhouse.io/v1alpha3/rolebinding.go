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
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	ProjectRoleBindingKind            = "ProjectRoleBinding"
	ProjectRoleBindingResource        = "projectrolebindings"
	ClusterProjectRoleBindingKind     = "ClusterProjectRoleBinding"
	ClusterProjectRoleBindingResource = "clusterprojectrolebindings"

	// ProjectRoleBindingFinalizer ensures service RoleBindings are cleaned up before the PRB is removed.
	ProjectRoleBindingFinalizer = "projects.deckhouse.io/project-role-binding"
	// ClusterProjectRoleBindingFinalizer ensures service RoleBindings are cleaned up before the CPRB is removed.
	ClusterProjectRoleBindingFinalizer = "projects.deckhouse.io/cluster-project-role-binding"

	// ResourceLabelOwnedByPRB/ResourceLabelOwnedByCPRB mark the service RoleBindings fanned out by the
	// PRB/CPRB controllers, so they can be found and garbage-collected.
	ResourceLabelOwnedByPRB  = "projects.deckhouse.io/owned-by-prb"
	ResourceLabelOwnedByCPRB = "projects.deckhouse.io/owned-by-cprb"

	// ResourceAnnotationRelatedWith links a service RoleBinding back to its source binding.
	ResourceAnnotationRelatedWith = "projects.deckhouse.io/related-with"

	ProjectRoleBindingConditionReady        = "Ready"
	ClusterProjectRoleBindingConditionReady = "Ready"
)

// RoleRef references the ClusterRole granted by a (Cluster)ProjectRoleBinding.
type RoleRef struct {
	// Kind of the role. Only ClusterRole is allowed.
	Kind string `json:"kind"`
	// Name of the ClusterRole.
	Name string `json:"name"`
}

// ProjectRoleBinding grants a role to subjects across all namespaces of a single project.
type ProjectRoleBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProjectRoleBindingSpec   `json:"spec,omitempty"`
	Status ProjectRoleBindingStatus `json:"status,omitempty"`
}

type ProjectRoleBindingSpec struct {
	Subjects []rbacv1.Subject `json:"subjects"`
	RoleRef  RoleRef          `json:"roleRef"`
}

type ProjectRoleBindingStatus struct {
	Conditions         []Condition `json:"conditions,omitempty"`
	ObservedGeneration int64       `json:"observedGeneration,omitempty"`
}

var _ runtime.Object = &ProjectRoleBinding{}

func (p *ProjectRoleBinding) DeepCopyObject() runtime.Object { return p.DeepCopy() }
func (p *ProjectRoleBinding) DeepCopy() *ProjectRoleBinding {
	if p == nil {
		return nil
	}
	newObj := new(ProjectRoleBinding)
	p.DeepCopyInto(newObj)
	return newObj
}
func (p *ProjectRoleBinding) DeepCopyInto(newObj *ProjectRoleBinding) {
	*newObj = *p
	p.ObjectMeta.DeepCopyInto(&newObj.ObjectMeta)
	p.Spec.DeepCopyInto(&newObj.Spec)
	p.Status.DeepCopyInto(&newObj.Status)
}

func (s *ProjectRoleBindingSpec) DeepCopyInto(newObj *ProjectRoleBindingSpec) {
	*newObj = *s
	newObj.RoleRef = s.RoleRef
	if s.Subjects != nil {
		newObj.Subjects = make([]rbacv1.Subject, len(s.Subjects))
		copy(newObj.Subjects, s.Subjects)
	}
}

func (s *ProjectRoleBindingStatus) DeepCopyInto(newObj *ProjectRoleBindingStatus) {
	*newObj = *s
	newObj.ObservedGeneration = s.ObservedGeneration
	if s.Conditions != nil {
		newObj.Conditions = make([]Condition, len(s.Conditions))
		for i := range s.Conditions {
			s.Conditions[i].DeepCopyInto(&newObj.Conditions[i])
		}
	}
}

type ProjectRoleBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectRoleBinding `json:"items"`
}

var _ runtime.Object = &ProjectRoleBindingList{}

func (p *ProjectRoleBindingList) DeepCopyObject() runtime.Object { return p.DeepCopy() }
func (p *ProjectRoleBindingList) DeepCopy() *ProjectRoleBindingList {
	if p == nil {
		return nil
	}
	newObj := new(ProjectRoleBindingList)
	p.DeepCopyInto(newObj)
	return newObj
}
func (p *ProjectRoleBindingList) DeepCopyInto(newObj *ProjectRoleBindingList) {
	*newObj = *p
	newObj.TypeMeta = p.TypeMeta
	p.ListMeta.DeepCopyInto(&newObj.ListMeta)
	if p.Items != nil {
		newObj.Items = make([]ProjectRoleBinding, len(p.Items))
		for i := range p.Items {
			p.Items[i].DeepCopyInto(&newObj.Items[i])
		}
	}
}

// ClusterProjectRoleBinding grants a role to subjects across all namespaces of all projects.
type ClusterProjectRoleBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterProjectRoleBindingSpec   `json:"spec,omitempty"`
	Status ClusterProjectRoleBindingStatus `json:"status,omitempty"`
}

type ClusterProjectRoleBindingSpec struct {
	Subjects []rbacv1.Subject `json:"subjects"`
	RoleRef  RoleRef          `json:"roleRef"`
}

type ClusterProjectRoleBindingStatus struct {
	Conditions         []Condition `json:"conditions,omitempty"`
	ObservedGeneration int64       `json:"observedGeneration,omitempty"`
	BoundProjects      int32       `json:"boundProjects,omitempty"`
}

var _ runtime.Object = &ClusterProjectRoleBinding{}

func (p *ClusterProjectRoleBinding) DeepCopyObject() runtime.Object { return p.DeepCopy() }
func (p *ClusterProjectRoleBinding) DeepCopy() *ClusterProjectRoleBinding {
	if p == nil {
		return nil
	}
	newObj := new(ClusterProjectRoleBinding)
	p.DeepCopyInto(newObj)
	return newObj
}
func (p *ClusterProjectRoleBinding) DeepCopyInto(newObj *ClusterProjectRoleBinding) {
	*newObj = *p
	p.ObjectMeta.DeepCopyInto(&newObj.ObjectMeta)
	p.Spec.DeepCopyInto(&newObj.Spec)
	p.Status.DeepCopyInto(&newObj.Status)
}

func (s *ClusterProjectRoleBindingSpec) DeepCopyInto(newObj *ClusterProjectRoleBindingSpec) {
	*newObj = *s
	newObj.RoleRef = s.RoleRef
	if s.Subjects != nil {
		newObj.Subjects = make([]rbacv1.Subject, len(s.Subjects))
		copy(newObj.Subjects, s.Subjects)
	}
}

func (s *ClusterProjectRoleBindingStatus) DeepCopyInto(newObj *ClusterProjectRoleBindingStatus) {
	*newObj = *s
	newObj.ObservedGeneration = s.ObservedGeneration
	newObj.BoundProjects = s.BoundProjects
	if s.Conditions != nil {
		newObj.Conditions = make([]Condition, len(s.Conditions))
		for i := range s.Conditions {
			s.Conditions[i].DeepCopyInto(&newObj.Conditions[i])
		}
	}
}

type ClusterProjectRoleBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterProjectRoleBinding `json:"items"`
}

var _ runtime.Object = &ClusterProjectRoleBindingList{}

func (p *ClusterProjectRoleBindingList) DeepCopyObject() runtime.Object { return p.DeepCopy() }
func (p *ClusterProjectRoleBindingList) DeepCopy() *ClusterProjectRoleBindingList {
	if p == nil {
		return nil
	}
	newObj := new(ClusterProjectRoleBindingList)
	p.DeepCopyInto(newObj)
	return newObj
}
func (p *ClusterProjectRoleBindingList) DeepCopyInto(newObj *ClusterProjectRoleBindingList) {
	*newObj = *p
	newObj.TypeMeta = p.TypeMeta
	p.ListMeta.DeepCopyInto(&newObj.ListMeta)
	if p.Items != nil {
		newObj.Items = make([]ClusterProjectRoleBinding, len(p.Items))
		for i := range p.Items {
			p.Items[i].DeepCopyInto(&newObj.Items[i])
		}
	}
}
