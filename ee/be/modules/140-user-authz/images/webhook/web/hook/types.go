/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hook

import (
	"regexp"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DirectoryEntry describes an entry with limited namespaces options for a single user
type DirectoryEntry struct {
	AllowAccessToSystemNamespaces bool
	LimitNamespaces               []*regexp.Regexp
	NamespaceSelectors            []*NamespaceSelector
	// If there is no LimitNamespaces  nor NamespaceSelectors options, the user has access to all namespaces except system namespaces.
	// If LimitNamespaces is present, we do not need to mind about allowed access to system namespaces.
	// Thus presence of LimitNamespaces matters when we summarise rules from all CRs to get the allowed namespaces.
	NamespaceFiltersAbsent bool
}

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

// WebhookRequest is a replica of the SubjectAccessReview Kubernetes kind with only important fields
type WebhookRequest struct {
	APIVersion string               `json:"apiVersion"`
	Kind       string               `json:"kind"`
	Spec       WebhookResourceSpec  `json:"spec"`
	Status     WebhookRequestStatus `json:"status"`
}

type WebhookResourceSpec struct {
	ResourceAttributes WebhookResourceAttributes `json:"resourceAttributes"`

	Group []string `json:"group"`
	User  string   `json:"user"`
}

type WebhookResourceAttributes struct {
	Group     string `json:"group,omitempty"`
	Version   string `json:"version,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Resource  string `json:"resource"`
	Verb      string `json:"verb"`
}

type WebhookRequestStatus struct {
	Allowed bool   `json:"allowed"`
	Denied  bool   `json:"denied,omitempty"`
	Reason  string `json:"reason,omitempty"`
}
