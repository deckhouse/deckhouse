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

package v1

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// AdditionalRole is a ClusterRole granted to the subjects of a rule on top of its access level.
type AdditionalRole struct {
	APIGroup string `json:"apiGroup"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
}

// ClusterAuthorizationRuleSpec lists the fields of a ClusterAuthorizationRule the controller
// needs to render bindings. Fields consumed only by the authorization webhook (limitNamespaces,
// namespaceSelector, allowAccessToSystemNamespaces) are intentionally not modeled: the controller
// never writes the spec.
type ClusterAuthorizationRuleSpec struct {
	AccessLevel     string           `json:"accessLevel,omitempty"`
	PortForwarding  bool             `json:"portForwarding,omitempty"`
	AllowScale      bool             `json:"allowScale,omitempty"`
	Subjects        []rbacv1.Subject `json:"subjects"`
	AdditionalRoles []AdditionalRole `json:"additionalRoles,omitempty"`
}

// RuleStatus is shared by ClusterAuthorizationRule and AuthorizationRule.
type RuleStatus struct {
	// Conditions of the rule. The Ready condition reports whether the bindings of the rule are applied.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Bindings is the number of (Cluster)RoleBindings rendered for the rule.
	Bindings int32 `json:"bindings,omitempty"`
	// ObservedGeneration is the generation of the rule the status refers to.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// The DeepCopy methods of this package are written by hand (no controller-gen in this module):
// every field of the spec and status is a string, a bool or a slice of such structs, so a copy of
// the slice is a deep copy. A field with a pointer, map or nested slice needs a real deep copy.
//
// ClusterAuthorizationRule grants a cluster-wide access level to users and groups.
type ClusterAuthorizationRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterAuthorizationRuleSpec `json:"spec,omitempty"`
	Status RuleStatus                   `json:"status,omitempty"`
}

// ClusterAuthorizationRuleList contains a list of ClusterAuthorizationRule.
type ClusterAuthorizationRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterAuthorizationRule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterAuthorizationRule{}, &ClusterAuthorizationRuleList{})
}

// DeepCopyInto copies the receiver into out.
func (in *ClusterAuthorizationRuleSpec) DeepCopyInto(out *ClusterAuthorizationRuleSpec) {
	*out = *in
	if in.Subjects != nil {
		out.Subjects = make([]rbacv1.Subject, len(in.Subjects))
		copy(out.Subjects, in.Subjects)
	}
	if in.AdditionalRoles != nil {
		out.AdditionalRoles = make([]AdditionalRole, len(in.AdditionalRoles))
		copy(out.AdditionalRoles, in.AdditionalRoles)
	}
}

// DeepCopyInto copies the receiver into out.
func (in *RuleStatus) DeepCopyInto(out *RuleStatus) {
	*out = *in
	if in.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(in.Conditions))
		for i := range in.Conditions {
			in.Conditions[i].DeepCopyInto(&out.Conditions[i])
		}
	}
}

// DeepCopyInto copies the receiver into out.
func (in *ClusterAuthorizationRule) DeepCopyInto(out *ClusterAuthorizationRule) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy returns a deep copy of the receiver.
func (in *ClusterAuthorizationRule) DeepCopy() *ClusterAuthorizationRule {
	if in == nil {
		return nil
	}
	out := new(ClusterAuthorizationRule)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject implements runtime.Object.
func (in *ClusterAuthorizationRule) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto copies the receiver into out.
func (in *ClusterAuthorizationRuleList) DeepCopyInto(out *ClusterAuthorizationRuleList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]ClusterAuthorizationRule, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

// DeepCopy returns a deep copy of the receiver.
func (in *ClusterAuthorizationRuleList) DeepCopy() *ClusterAuthorizationRuleList {
	if in == nil {
		return nil
	}
	out := new(ClusterAuthorizationRuleList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject implements runtime.Object.
func (in *ClusterAuthorizationRuleList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}
