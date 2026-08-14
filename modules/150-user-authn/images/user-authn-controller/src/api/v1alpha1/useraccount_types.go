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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	LabelKind        = "userauthn.deckhouse.io/kind"
	LabelConnectorID = "userauthn.deckhouse.io/connector-id"
	LabelLocked      = "userauthn.deckhouse.io/locked"

	KindLocal    = "Local"
	KindExternal = "External"
)

// UserAccountSpec is empty. UserAccount is a controller-projected view of Dex
// Password and OfflineSessions; there is no user-editable specification.
type UserAccountSpec struct{}

// UserAccountStatus is the observed account state projected from Dex.
// Credentials and connector secrets are omitted.
type UserAccountStatus struct {
	Email                  string       `json:"email,omitempty"`
	Username               string       `json:"username,omitempty"`
	UserID                 string       `json:"userID,omitempty"`
	Kind                   string       `json:"kind,omitempty"`
	ConnectorID            string       `json:"connectorID,omitempty"`
	ProviderType           string       `json:"providerType,omitempty"`
	IncorrectLoginAttempts int64        `json:"incorrectLoginAttempts,omitempty"`
	Locked                 bool         `json:"locked,omitempty"`
	LockedUntil            *metav1.Time `json:"lockedUntil,omitempty"`
	LockedByAdministrator  bool         `json:"lockedByAdministrator,omitempty"`
	UserRef                string       `json:"userRef,omitempty"`
	ExpireAt               *metav1.Time `json:"expireAt,omitempty"`
	Groups                 []string     `json:"groups,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// UserAccount is a cluster-scoped projection of a Dex Password or OfflineSessions object.
type UserAccount struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserAccountSpec   `json:"spec,omitempty"`
	Status UserAccountStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UserAccountList contains a list of UserAccount.
type UserAccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UserAccount `json:"items"`
}

func init() {
	SchemeBuilder.Register(&UserAccount{}, &UserAccountList{})
}
