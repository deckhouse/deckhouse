// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const ClusterLicenseKind = "ClusterLicense"

var _ runtime.Object = (*ClusterLicense)(nil)

// LicenseRecordReason explains why a record of a license package does not contribute
// to the effective policy. An empty reason means the record is accepted.
type LicenseRecordReason string

const (
	// LicenseRecordRenewed - the record is extinguished by a successor listed in its renews.
	LicenseRecordRenewed LicenseRecordReason = "Renewed"
	// LicenseRecordSuperseded - the record is extinguished by a successor listed in its supersedes.
	LicenseRecordSuperseded LicenseRecordReason = "Superseded"
	// LicenseRecordExpired - expireAt is in the past.
	LicenseRecordExpired LicenseRecordReason = "Expired"
	// LicenseRecordDuplicate - a record with the same id is already installed.
	LicenseRecordDuplicate LicenseRecordReason = "Duplicate"
	// LicenseRecordNotYetValid - startAt is in the future.
	LicenseRecordNotYetValid LicenseRecordReason = "NotYetValid"
	// LicenseRecordRevoked - the record is on the vendor revocation list.
	LicenseRecordRevoked LicenseRecordReason = "Revoked"
	// LicenseRecordClusterMismatch - the record is issued for another cluster or another cluster key.
	LicenseRecordClusterMismatch LicenseRecordReason = "ClusterMismatch"
	// LicenseRecordSchemaViolation - a field of the record has an unexpected shape.
	LicenseRecordSchemaViolation LicenseRecordReason = "SchemaViolation"
	// LicenseRecordUnsupportedType - the record type is unknown to this version of Deckhouse.
	// Such a record is ignored, it never rejects the package and may start working after an update.
	// A record that does not decode at all is a SchemaViolation too: from the
	// outside there is no difference between a field of the wrong shape and a
	// record of the wrong shape.
	LicenseRecordUnsupportedType LicenseRecordReason = "UnsupportedType"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=cl
// +kubebuilder:printcolumn:name="Accepted",type=boolean,JSONPath=`.status.accepted`
// +kubebuilder:printcolumn:name="Superseded",type=boolean,JSONPath=`.status.superseded`
// +kubebuilder:printcolumn:name="Message",type=string,JSONPath=`.status.message`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ClusterLicense holds a single Deckhouse license key installed in the cluster.
type ClusterLicense struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterLicenseSpec `json:"spec"`

	// +optional
	Status ClusterLicenseStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterLicenseList is a list of ClusterLicense resources.
type ClusterLicenseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterLicense `json:"items"`
}

type ClusterLicenseSpec struct {
	// LicenseKey is a bare compact JWT issued by the license server.
	LicenseKey string `json:"licenseKey"`
}

type ClusterLicenseStatus struct {
	// Accepted is true when the package signature and envelope are valid,
	// regardless of how many of its records currently contribute to the policy.
	// It is always published: a missing field and an explicit false would read
	// the same to a client, and they are different statements.
	// +optional
	Accepted bool `json:"accepted"`

	// Superseded is true when every record of this key was extinguished by a
	// successor of an accepted key whose start has passed. Such a key is deleted
	// by the controller. It is always published, see Accepted.
	// +optional
	Superseded bool `json:"superseded"`

	// PackageJti is the jti claim of the package, it identifies the key itself.
	// +optional
	PackageJti string `json:"packageJti,omitempty"`

	// CustomerName is the customer name carried by the package.
	// +optional
	CustomerName string `json:"customerName,omitempty"`

	// Message is a human readable summary of the key state.
	// +optional
	Message string `json:"message,omitempty"`

	// Records is the per-record verdict, one entry per record of the package.
	// +optional
	Records []LicenseRecordStatus `json:"records,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type LicenseRecordStatus struct {
	// ID is the record id from the package.
	ID string `json:"id"`

	// Type of the record, for example Platform. Unknown types are ignored.
	// +optional
	Type string `json:"type,omitempty"`

	// Origin of the record: purchase, demo or self-service.
	// +optional
	Origin string `json:"origin,omitempty"`

	// Accepted is true when the record contributes to the effective policy now or in the future.
	// It is always published, see ClusterLicenseStatus.Accepted.
	// +optional
	Accepted bool `json:"accepted"`

	// Reason is set when the record is not accepted.
	// +optional
	Reason LicenseRecordReason `json:"reason,omitempty"`

	// Message explains the reason in a human readable form.
	// +optional
	Message string `json:"message,omitempty"`

	// SupersededBy is the id of the record that extinguished this one, for both
	// Superseded and Renewed.
	// +optional
	SupersededBy string `json:"supersededBy,omitempty"`

	// StartAt is null when the record was rejected before its dates could be read.
	// +optional
	StartAt *metav1.Time `json:"startAt,omitempty"`

	// ExpireAt is null for a perpetual record.
	// +optional
	ExpireAt *metav1.Time `json:"expireAt,omitempty"`

	// GraceDays is the grace period of the record, in days.
	// It is null when the record does not declare one, and the controller default applies.
	// +optional
	GraceDays *int `json:"graceDays,omitempty"`

	// Grants is the resource_limits delta this record adds to the effective limits.
	// A null value means the record grants the resource without a limit; an absent key
	// means the record grants no limit for this resource at all.
	// +optional
	Grants map[string]*int64 `json:"grants,omitempty"`
}
