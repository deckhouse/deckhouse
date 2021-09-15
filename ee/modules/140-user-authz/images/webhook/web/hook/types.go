/*
Copyright 2021 Flant CJSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hook

import "regexp"

// DirectoryEntry describes an entry with limited namespaces options for a single user
type DirectoryEntry struct {
	AllowAccessToSystemNamespaces bool
	LimitNamespaces               []*regexp.Regexp
}

// UserAuthzConfig is a config composed from ClusterAuthorizationRules collected from Kubernetes cluster
type UserAuthzConfig struct {
	CRDs []struct {
		Name string `json:"name"`
		Spec struct {
			AccessLevel                   string   `json:"accessLevel"`
			PortForwarding                bool     `json:"portForwarding"`
			AllowScale                    bool     `json:"allowScale"`
			AllowAccessToSystemNamespaces bool     `json:"allowAccessToSystemNamespaces"`
			LimitNamespaces               []string `json:"limitNamespaces"`
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
