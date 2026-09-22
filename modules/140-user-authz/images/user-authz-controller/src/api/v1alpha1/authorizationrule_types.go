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
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	v1 "user-authz-controller/api/v1"
)

// AuthorizationRuleSpec lists the fields of an AuthorizationRule the controller needs to render
// bindings. The controller never writes the spec.
type AuthorizationRuleSpec struct {
	AccessLevel    string           `json:"accessLevel,omitempty"`
	PortForwarding bool             `json:"portForwarding,omitempty"`
	AllowScale     bool             `json:"allowScale,omitempty"`
	Subjects       []rbacv1.Subject `json:"subjects"`
}

// The DeepCopy methods of this package are written by hand (no controller-gen in this module):
// every field of the spec and status is a string, a bool or a slice of such structs, so a copy of
// the slice is a deep copy. A field with a pointer, map or nested slice needs a real deep copy.
//
// AuthorizationRule grants an access level to users and groups within its namespace.
type AuthorizationRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AuthorizationRuleSpec `json:"spec,omitempty"`
	Status v1.RuleStatus         `json:"status,omitempty"`
}

// AuthorizationRuleList contains a list of AuthorizationRule.
type AuthorizationRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AuthorizationRule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AuthorizationRule{}, &AuthorizationRuleList{})
}

// DeepCopyInto copies the receiver into out.
func (in *AuthorizationRuleSpec) DeepCopyInto(out *AuthorizationRuleSpec) {
	*out = *in
	if in.Subjects != nil {
		out.Subjects = make([]rbacv1.Subject, len(in.Subjects))
		copy(out.Subjects, in.Subjects)
	}
}

// DeepCopyInto copies the receiver into out.
func (in *AuthorizationRule) DeepCopyInto(out *AuthorizationRule) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy returns a deep copy of the receiver.
func (in *AuthorizationRule) DeepCopy() *AuthorizationRule {
	if in == nil {
		return nil
	}
	out := new(AuthorizationRule)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject implements runtime.Object.
func (in *AuthorizationRule) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto copies the receiver into out.
func (in *AuthorizationRuleList) DeepCopyInto(out *AuthorizationRuleList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]AuthorizationRule, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

// DeepCopy returns a deep copy of the receiver.
func (in *AuthorizationRuleList) DeepCopy() *AuthorizationRuleList {
	if in == nil {
		return nil
	}
	out := new(AuthorizationRuleList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject implements runtime.Object.
func (in *AuthorizationRuleList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}
