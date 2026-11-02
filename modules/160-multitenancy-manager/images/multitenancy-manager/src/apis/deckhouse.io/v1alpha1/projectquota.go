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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	ProjectQuotaKind     = "ProjectQuota"
	ProjectQuotaResource = "projectquotas"

	// ProjectQuotaFinalizer holds a ProjectQuota until the controller takes the quota label off the
	// namespaces of the project: the ResourceQuota objects it owns are deleted after the quota is gone,
	// and the guard policy refuses every create in a labelled namespace that has no quota.
	ProjectQuotaFinalizer = "projects.deckhouse.io/project-quota"
)

// Condition types of a ProjectQuota.
const (
	// ProjectQuotaConditionEnforced is True when every namespace of the project has a calculated
	// d8-project-quota ResourceQuota and the quota is applied there; its reason says why it is not.
	ProjectQuotaConditionEnforced = "Enforced"
	// ProjectQuotaConditionExceeded is True when the usage of the project is above the quota on some
	// key; its reason says how it got there.
	ProjectQuotaConditionExceeded = "Exceeded"
)

// Reasons of the Enforced condition.
const (
	ProjectQuotaReasonEnforced               = "Enforced"
	ProjectQuotaReasonControlPlaneNotManaged = "ControlPlaneNotManaged"
	ProjectQuotaReasonKeyRollout             = "KeyRollout"
	ProjectQuotaReasonKeyNotServed           = "KeyNotServed"
	ProjectQuotaReasonProjectNotFound        = "ProjectNotFound"
	ProjectQuotaReasonSuspended              = "Suspended"
	ProjectQuotaReasonWebhookMissing         = "WebhookMissing"
	ProjectQuotaReasonWebhookMisconfigured   = "WebhookMisconfigured"
	ProjectQuotaReasonWebhookNotCalled       = "WebhookNotCalled"
)

// Reasons of the Exceeded condition.
const (
	ProjectQuotaReasonWithinLimits = "WithinLimits"
	ProjectQuotaReasonHardLowered  = "HardLowered"
	ProjectQuotaReasonOvershoot    = "Overshoot"
)

// ProjectQuotaTopNamespaces is the most namespaces the status lists; the usage of every namespace is in
// its d8-project-quota ResourceQuota.
const ProjectQuotaTopNamespaces = 50

var _ runtime.Object = &ProjectQuota{}

// ProjectQuota is one limit on the sum of resource usage over all namespaces of a project. Its name is
// the name of the project.
type ProjectQuota struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProjectQuotaSpec   `json:"spec,omitempty"`
	Status ProjectQuotaStatus `json:"status,omitempty"`
}

// ProjectQuotaSpec is the quota of a project.
type ProjectQuotaSpec struct {
	// Hard is the limit of the project, in the format of ResourceQuota.spec.hard.
	Hard corev1.ResourceList `json:"hard,omitempty"`
}

// ProjectQuotaStatus is the usage of a project against its quota.
type ProjectQuotaStatus struct {
	// Hard is the limit the controller applies: spec.hard with the aliases expanded.
	Hard corev1.ResourceList `json:"hard,omitempty"`
	// Used is the usage summed over all namespaces of the project.
	Used corev1.ResourceList `json:"used,omitempty"`
	// NamespaceCount is the number of namespaces of the project.
	NamespaceCount int32 `json:"namespaceCount,omitempty"`
	// TopNamespaces are the namespaces closest to the limit, at most ProjectQuotaTopNamespaces of them.
	TopNamespaces []ProjectQuotaNamespaceUsage `json:"topNamespaces,omitempty"`
	// ObservedGeneration is the generation of the spec the status describes.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions are Enforced and Exceeded.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ProjectQuotaNamespaceUsage is the usage of one namespace of the project.
type ProjectQuotaNamespaceUsage struct {
	Name string              `json:"name"`
	Used corev1.ResourceList `json:"used,omitempty"`
}

func (q *ProjectQuota) DeepCopyObject() runtime.Object {
	return q.DeepCopy()
}

func (q *ProjectQuota) DeepCopy() *ProjectQuota {
	if q == nil {
		return nil
	}
	out := new(ProjectQuota)
	q.DeepCopyInto(out)
	return out
}

func (q *ProjectQuota) DeepCopyInto(out *ProjectQuota) {
	*out = *q
	out.TypeMeta = q.TypeMeta
	q.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	q.Spec.DeepCopyInto(&out.Spec)
	q.Status.DeepCopyInto(&out.Status)
}

func (s *ProjectQuotaSpec) DeepCopyInto(out *ProjectQuotaSpec) {
	*out = *s
	if s.Hard != nil {
		out.Hard = s.Hard.DeepCopy()
	}
}

func (s *ProjectQuotaStatus) DeepCopyInto(out *ProjectQuotaStatus) {
	*out = *s
	if s.Hard != nil {
		out.Hard = s.Hard.DeepCopy()
	}
	if s.Used != nil {
		out.Used = s.Used.DeepCopy()
	}
	if s.TopNamespaces != nil {
		out.TopNamespaces = make([]ProjectQuotaNamespaceUsage, len(s.TopNamespaces))
		for i := range s.TopNamespaces {
			out.TopNamespaces[i] = ProjectQuotaNamespaceUsage{Name: s.TopNamespaces[i].Name, Used: s.TopNamespaces[i].Used.DeepCopy()}
		}
	}
	if s.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(s.Conditions))
		for i := range s.Conditions {
			s.Conditions[i].DeepCopyInto(&out.Conditions[i])
		}
	}
}

// ProjectQuotaList is a list of ProjectQuota objects.
type ProjectQuotaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectQuota `json:"items"`
}

func (l *ProjectQuotaList) DeepCopyObject() runtime.Object {
	return l.DeepCopy()
}

func (l *ProjectQuotaList) DeepCopy() *ProjectQuotaList {
	if l == nil {
		return nil
	}
	out := new(ProjectQuotaList)
	*out = *l
	l.ListMeta.DeepCopyInto(&out.ListMeta)
	if l.Items != nil {
		out.Items = make([]ProjectQuota, len(l.Items))
		for i := range l.Items {
			l.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
	return out
}
