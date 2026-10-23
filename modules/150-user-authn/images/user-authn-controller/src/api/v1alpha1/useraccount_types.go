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

	// SessionKindBrowser is a browser signed in to Dex: a session of Dex and the applications it
	// signed in to.
	SessionKindBrowser = "Browser"
	// SessionKindGrant is a device holding a refresh token issued without a session of Dex in a
	// browser: kubeconfig, device flow, an application of its own, basic authentication.
	SessionKindGrant = "Grant"

	// MaxSessions bounds UserAccount.status.sessions; the most recently active are listed.
	MaxSessions = 50
)

// UserAccountSpec is empty. UserAccount is a controller-projected view of Dex
// Password, OfflineSessions, and UserIdentity; there is no user-editable specification.
type UserAccountSpec struct{}

// UserAccountTwoFactor is the second factor (TOTP) Dex keeps for the account, without secrets.
type UserAccountTwoFactor struct {
	// Enrolled tells whether Dex holds a TOTP secret for the account.
	Enrolled bool `json:"enrolled"`
	// Confirmed tells whether the user entered a code for the secret; until then a sign-in shows
	// the QR code again.
	Confirmed bool `json:"confirmed"`
}

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
	// LastLogin is the last sign-in through Dex with every factor done, from UserIdentity.
	LastLogin *metav1.Time `json:"lastLogin,omitempty"`
	// TwoFactor is set once Dex keeps a UserIdentity for the account.
	TwoFactor *UserAccountTwoFactor `json:"twoFactor,omitempty"`
	// Sessions are the devices signed in to the account, the most recently active first.
	Sessions []UserAccountSession `json:"sessions,omitempty"`
	// SessionsTruncated tells that the account has more than MaxSessions devices and the least
	// recently active are not listed.
	SessionsTruncated bool `json:"sessionsTruncated,omitempty"`
}

// UserAccountSession is a device signed in to the account: a browser session of Dex or a grant.
type UserAccountSession struct {
	// ID is the ID of the session of Dex (the sid claim) or of the grant (the d8_gid claim).
	ID string `json:"id"`
	// Kind is SessionKindBrowser or SessionKindGrant.
	Kind string `json:"kind"`
	// Clients are the applications signed in on the device.
	Clients []string `json:"clients,omitempty"`
	// IPAddress is the address the ingress saw at the last sign-in of the device.
	IPAddress string `json:"ipAddress,omitempty"`
	// UserAgent is the User-Agent of the device as it sent it.
	UserAgent string `json:"userAgent,omitempty"`
	// SignedInAt is when the device signed in.
	SignedInAt *metav1.Time `json:"signedInAt,omitempty"`
	// KnownSince is when Dex first saw the browser of the device complete a login of the account,
	// set when more than a minute earlier than SignedInAt: the browser signed in before, signed out, and
	// signed in again.
	// The device is as old as that for the actions on the other devices and the password.
	KnownSince *metav1.Time `json:"knownSince,omitempty"`
	// LastActivity is the last use of the device, rounded down to the hour and never before
	// SignedInAt.
	LastActivity *metav1.Time `json:"lastActivity,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// UserAccount is a cluster-scoped projection of a Dex Password, OfflineSessions, or UserIdentity object.
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
