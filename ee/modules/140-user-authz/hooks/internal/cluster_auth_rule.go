/*
Copyright 2021 Flant JSC
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
	AccessLevel string `json:"accessLevel"`

	PortForwarding bool `json:"portForwarding"`

	AllowScale bool `json:"allowScale"`

	AllowAccessToSystemNamespaces bool `json:"allowAccessToSystemNamespaces"`

	LimitNamespaces []string `json:"limitNamespaces,omitempty"`

	Subjects []rbacv1.Subject `json:"subjects,omitempty"`

	// TODO: additionalRoles
	AdditionalRoles interface{} `json:"additionalRoles,omitempty"`
}

type ClusterAuthorizationRuleStatus struct {
}
