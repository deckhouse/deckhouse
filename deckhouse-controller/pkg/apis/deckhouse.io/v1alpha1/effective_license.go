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

const (
	EffectiveLicenseKind = "EffectiveLicense"
	// EffectiveLicenseName is the name of the only EffectiveLicense instance.
	EffectiveLicenseName = "dkp"
)

var _ runtime.Object = (*EffectiveLicense)(nil)

// LicenseComplianceState is the aggregated state of the license policy.
type LicenseComplianceState string

const (
	LicenseComplianceValid         LicenseComplianceState = "Valid"
	LicenseComplianceWarning       LicenseComplianceState = "Warning"
	LicenseComplianceGrace         LicenseComplianceState = "Grace"
	LicenseComplianceViolation     LicenseComplianceState = "Violation"
	LicenseComplianceNoUpdateRight LicenseComplianceState = "NoUpdateRight"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=el
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.compliance.state`
// +kubebuilder:printcolumn:name="Within limits",type=boolean,JSONPath=`.status.withinLimits`
// +kubebuilder:printcolumn:name="Next reduction",type=date,JSONPath=`.status.nextReduction.at`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// EffectiveLicense is the aggregated license policy of the cluster.
// The object is computed by the controller from all ClusterLicense resources,
// it has no spec and the only instance is named dkp.
type EffectiveLicense struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Status EffectiveLicenseStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// EffectiveLicenseList is a list of EffectiveLicense resources.
type EffectiveLicenseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EffectiveLicense `json:"items"`
}

type EffectiveLicenseStatus struct {
	// +optional
	Compliance LicenseCompliance `json:"compliance,omitempty"`

	// +optional
	Counts LicenseCounts `json:"counts,omitempty"`

	// +optional
	Effective LicenseEffective `json:"effective,omitempty"`

	// Metrics is the observed consumption, keyed by resource name.
	// +optional
	Metrics map[string]LicenseMetricValue `json:"metrics,omitempty"`

	// WithinLimits is false when any observed resource exceeds its effective limit.
	// It is always published: a missing field and an explicit false would read the
	// same to a client, and they are different statements.
	// +optional
	WithinLimits bool `json:"withinLimits"`

	// NextReduction is the nearest point in time when the effective limits shrink.
	// It is null when no reduction is known.
	// +optional
	NextReduction *LicenseReduction `json:"nextReduction,omitempty"`

	// Timeline is the effective limits over time, from the current segment onwards.
	// +optional
	Timeline []LicenseTimelineSegment `json:"timeline,omitempty"`

	// RegistrationRequest is a bare compact JWT to paste into the license server.
	// It is regenerated hourly.
	// +optional
	RegistrationRequest string `json:"registrationRequest,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type LicenseCompliance struct {
	// +optional
	State LicenseComplianceState `json:"state,omitempty"`

	// Reason explains the state. At Valid it is either empty or the informational
	// LimitsApproaching.
	// One of Unregistered, Expired, Revoked, LimitsExceeded, ExpiringSoon,
	// LimitsApproaching, ProjectedOverLimit.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Since is when the state was entered. It is null while the controller has not
	// observed a transition yet.
	// +optional
	Since *metav1.Time `json:"since,omitempty"`
}

type LicenseCounts struct {
	// +optional
	Packages int `json:"packages,omitempty"`

	// +optional
	Records int `json:"records,omitempty"`

	// +optional
	Accepted int `json:"accepted,omitempty"`

	// +optional
	Rejected int `json:"rejected,omitempty"`
}

type LicenseEffective struct {
	// Limits is the summed quota, keyed by resource name.
	// A null value means the resource is unlimited; an absent key means no limit
	// for this resource has been granted.
	// +optional
	Limits map[string]*int64 `json:"limits,omitempty"`

	// Unlimited is true when at least one resource in Limits ended up unlimited.
	// +optional
	Unlimited bool `json:"unlimited,omitempty"`

	// GrantedBy lists every record that contributes to a resource, keyed by resource name.
	// +optional
	GrantedBy map[string][]string `json:"grantedBy,omitempty"`
}

type LicenseMetricValue struct {
	// Instant is the value of the last observation.
	// +optional
	Instant float64 `json:"instant,omitempty"`

	// Avg7d is the mean over the observation window.
	// +optional
	Avg7d float64 `json:"avg_7d,omitempty"`

	// Extrapolated is the value projected to the nearest future expiration.
	// It is null while there are not enough observations to project from, that is,
	// while the journal is shorter than the sustained window.
	// +optional
	Extrapolated *float64 `json:"extrapolated"`
}

type LicenseReduction struct {
	At metav1.Time `json:"at"`

	// In is the human readable time left until the reduction, for example 61d.
	// +optional
	In string `json:"in,omitempty"`

	// Limits is the change of every affected resource, keyed by resource name.
	// +optional
	Limits map[string]LicenseLimitChange `json:"limits,omitempty"`
}

// LicenseLimitChange is one resource moving from one limit to another.
// A null bound means unlimited, and both are always published: dropping a zero
// would turn "the quota becomes zero" into "the quota does not change".
type LicenseLimitChange struct {
	// +optional
	From *int64 `json:"from"`

	// +optional
	To *int64 `json:"to"`
}

type LicenseTimelineSegment struct {
	From metav1.Time `json:"from"`

	// To is null for the last, open ended segment.
	// +optional
	To *metav1.Time `json:"to,omitempty"`

	// +optional
	State LicenseComplianceState `json:"state,omitempty"`

	// Limits is the effective quota during the segment, keyed by resource name.
	// A null value means the resource is unlimited; an absent key means no limit
	// for this resource has been granted.
	// +optional
	Limits map[string]*int64 `json:"limits,omitempty"`
}
