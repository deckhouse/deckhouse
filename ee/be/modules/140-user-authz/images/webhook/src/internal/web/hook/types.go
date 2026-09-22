/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hook

// WebhookRequest is a replica of the SubjectAccessReview Kubernetes kind with only important fields
type WebhookRequest struct {
	APIVersion string               `json:"apiVersion"`
	Kind       string               `json:"kind"`
	Spec       WebhookResourceSpec  `json:"spec"`
	Status     WebhookRequestStatus `json:"status"`
}

type WebhookResourceSpec struct {
	ResourceAttributes WebhookResourceAttributes `json:"resourceAttributes"`

	Group []string `json:"groups"`
	User  string   `json:"user"`
}

type WebhookResourceAttributes struct {
	Group       string `json:"group,omitempty"`
	Version     string `json:"version,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	Resource    string `json:"resource"`
	Subresource string `json:"subresource,omitempty"`
	Name        string `json:"name,omitempty"`
	Verb        string `json:"verb"`
}

type WebhookRequestStatus struct {
	Allowed bool   `json:"allowed"`
	Denied  bool   `json:"denied,omitempty"`
	Reason  string `json:"reason,omitempty"`
}
