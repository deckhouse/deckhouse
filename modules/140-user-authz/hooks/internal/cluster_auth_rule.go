/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package internal

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClusterAuthorizationRule struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterAuthorizationRuleSpec `json:"spec"`

	Status ClusterAuthorizationRuleStatus `json:"status,omitempty"`
}

type ClusterAuthorizationRuleSpec struct {
	AccessLevel string `json:"accessLevel,omitempty"`

	PortForwarding bool `json:"portForwarding,omitempty"`

	AllowScale bool `json:"allowScale,omitempty"`

	AllowAccessToSystemNamespaces bool `json:"allowAccessToSystemNamespaces,omitempty"`

	LimitNamespaces []string `json:"limitNamespaces,omitempty"`

	Subjects []rbacv1.Subject `json:"subjects,omitempty"`

	AdditionalRoles interface{} `json:"additionalRoles,omitempty"`
}

type ClusterAuthorizationRuleStatus struct {
}

func (car ClusterAuthorizationRule) IsMultitenancy() bool {
	if len(car.Spec.LimitNamespaces) > 0 {
		return true
	}

	if car.Spec.AllowAccessToSystemNamespaces {
		return true
	}

	return false
}

// ValuesClusterAuthorizationRule is a cutted version of ClusterAuthorizationRule, special for values openapi schema
type ValuesClusterAuthorizationRule struct {
	Name string                       `json:"name"`
	Spec ClusterAuthorizationRuleSpec `json:"spec"`
}

func (car ClusterAuthorizationRule) ToValues() ValuesClusterAuthorizationRule {
	return ValuesClusterAuthorizationRule{
		Name: car.Name,
		Spec: car.Spec,
	}
}
