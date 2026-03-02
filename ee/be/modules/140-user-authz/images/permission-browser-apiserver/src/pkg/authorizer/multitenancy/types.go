/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package multitenancy

import (
	"regexp"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DirectoryEntry describes an entry with limited namespaces options for a single user
type DirectoryEntry struct {
	AllowAccessToSystemNamespaces bool
	LimitNamespaces               []*regexp.Regexp
	NamespaceSelectors            []*NamespaceSelector
	// If there is no LimitNamespaces nor NamespaceSelectors options, the user has access to all namespaces except system namespaces.
	// If LimitNamespaces is present, we do not need to mind about allowed access to system namespaces.
	// Thus presence of LimitNamespaces matters when we summarise rules from all CRs to get the allowed namespaces.
	NamespaceFiltersAbsent bool
}

// NamespaceSelector defines a selector for namespaces
type NamespaceSelector struct {
	LabelSelector *metav1.LabelSelector `json:"labelSelector"`
	MatchAny      bool                  `json:"matchAny"`
}

// UserAuthzConfig is a config composed from ClusterAuthorizationRules collected from Kubernetes cluster
type UserAuthzConfig struct {
	CRDs []struct {
		Name string `json:"name"`
		Spec struct {
			AccessLevel                   string             `json:"accessLevel"`
			PortForwarding                bool               `json:"portForwarding"`
			AllowScale                    bool               `json:"allowScale"`
			AllowAccessToSystemNamespaces bool               `json:"allowAccessToSystemNamespaces"`
			LimitNamespaces               []string           `json:"limitNamespaces"`
			NamespaceSelector             *NamespaceSelector `json:"namespaceSelector"`
			AdditionalRoles               []struct {
				APIGroup string `json:"apiGroup"`
				Kind     string `json:"kind"`
				Name     string `json:"name"`
			} `json:"additionalRoles"`
			Subjects []struct {
				Kind      string `json:"kind"`
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"subjects"`
		} `json:"spec,omitempty"`
	} `json:"crds"`
}
