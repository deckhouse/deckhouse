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
	// EffectiveLicenseName is the name of the EffectiveLicense of the platform
	// itself. The object is named after the product: licences for individual
	// products get their own object, computed from the same ClusterLicense keys.
	EffectiveLicenseName = "deckhouse-platform"
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

// LicenseNodeBilling is the group a node was allocated to.
type LicenseNodeBilling string

const (
	// LicenseNodeFree - a platform node without user workload, see the reason field.
	LicenseNodeFree LicenseNodeBilling = "Free"
	// LicenseNodeServer - covered by a whole-node server licence.
	LicenseNodeServer LicenseNodeBilling = "Server"
	// LicenseNodeVCPU - paid for out of the vCPU metric.
	LicenseNodeVCPU LicenseNodeBilling = "VCPU"
	// LicenseNodeCores - paid for out of the cores metric.
	LicenseNodeCores LicenseNodeBilling = "Cores"
	// LicenseNodeUnlicensed - not covered by the license at all.
	LicenseNodeUnlicensed LicenseNodeBilling = "Unlicensed"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=el
// +kubebuilder:printcolumn:name="Licensed",type=boolean,JSONPath=`.status.licensed`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.compliance.state`
// +kubebuilder:printcolumn:name="Valid until",type=date,JSONPath=`.status.key.validUntil`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// EffectiveLicense is the aggregated license policy of one product.
// The object is computed by the controller from all ClusterLicense resources,
// it has no spec, and the instance of the platform itself is named
// deckhouse-platform.
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
	// Licensed is the binary answer: true while the compliance state is Valid or
	// Warning. It is always published: a missing field and an explicit false
	// would read the same to a client, and they are different statements.
	// +optional
	Licensed bool `json:"licensed"`

	// +optional
	Compliance LicenseCompliance `json:"compliance,omitempty"`

	// Key describes the license key in force. It is absent while the cluster
	// holds no key.
	// +optional
	Key *LicenseKeyInfo `json:"key,omitempty"`

	// +optional
	Limits LicenseLimits `json:"limits,omitempty"`

	// Consumption is what went into the cluster data file: the metrics of the
	// licensable nodes.
	// +optional
	Consumption LicenseConsumption `json:"consumption,omitempty"`

	// +optional
	Allocation LicenseAllocation `json:"allocation,omitempty"`

	// OverLimitSince is when unlicensed nodes appeared. It is null while every
	// node is covered, and seven days after it the cluster is in violation.
	// +optional
	OverLimitSince *metav1.Time `json:"overLimitSince,omitempty"`

	// Nodes is the full allocation, free nodes included, sorted by the rule of
	// the specification inside each group.
	// +optional
	Nodes []LicenseNodeStatus `json:"nodes,omitempty"`

	// RegistrationRequest is a bare compact JWT with the cluster data, to be
	// handed to the license server as deckhouse-cluster-<first 8 of
	// cluster_id>-<YYYYMMDD>.jwt. It is rebuilt whenever the
	// accepted records, the installed keys or the consumption change.
	// +optional
	RegistrationRequest string `json:"registrationRequest,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type LicenseCompliance struct {
	// +optional
	State LicenseComplianceState `json:"state,omitempty"`

	// Reason explains a state that is not Valid.
	// One of Unregistered, Expired, Revoked, UnlicensedNodes, ExpiringSoon.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Since is when the state was entered. It is null while the controller has
	// not observed a transition yet.
	// +optional
	Since *metav1.Time `json:"since,omitempty"`
}

// LicenseKeyInfo is the key in force, the way it is shown to the customer. The
// stepped terms of the individual records are deliberately not part of it.
type LicenseKeyInfo struct {
	// Name is the ClusterLicense object that carries the key.
	// +optional
	Name string `json:"name,omitempty"`

	// Jti identifies the key itself.
	// +optional
	Jti string `json:"jti,omitempty"`

	// +optional
	CustomerName string `json:"customerName,omitempty"`

	// Origin of the key: purchase, demo or self-service.
	// +optional
	Origin string `json:"origin,omitempty"`

	// ValidUntil is the greatest expiration of the records of the key.
	// It is null for a key that never expires.
	// +optional
	ValidUntil *metav1.Time `json:"validUntil,omitempty"`

	// GraceDays comes from the record that expires last.
	// +optional
	GraceDays *int `json:"graceDays,omitempty"`
}

type LicenseLimits struct {
	// Values is the summed quota of the active records, keyed by metric name.
	// A null value means the metric is unlimited; an absent key means no active
	// record named it.
	// +optional
	Values map[string]*int64 `json:"values,omitempty"`

	// Unlimited lists the metrics that ended up without a limit.
	// +optional
	Unlimited []string `json:"unlimited,omitempty"`

	// GrantedBy lists every record that contributes to a metric, keyed by metric
	// name. It is diagnostics and is not shown in the Console.
	// +optional
	GrantedBy map[string][]string `json:"grantedBy,omitempty"`
}

// LicenseConsumption is the instant reading of the three metrics over the
// licensable nodes, plus the number of free nodes left out of them.
type LicenseConsumption struct {
	// +optional
	Servers int64 `json:"servers"`

	// +optional
	VCPU int64 `json:"vCPU"`

	// +optional
	Cores int64 `json:"cores"`

	// +optional
	FreeNodes int64 `json:"freeNodes"`
}

// LicenseAllocation is the node allocation by metric: every licensable node is
// attributed to exactly one of servers, vCPU and cores, or to unlicensed.
type LicenseAllocation struct {
	// +optional
	Servers LicenseMetricAllocation `json:"servers,omitempty"`

	// +optional
	VCPU LicenseMetricAllocation `json:"vCPU,omitempty"`

	// +optional
	Cores LicenseMetricAllocation `json:"cores,omitempty"`

	// +optional
	Unlicensed LicenseUnlicensedNodes `json:"unlicensed,omitempty"`
}

// LicenseMetricAllocation is the quota of one metric and what the nodes
// attributed to it consume of it.
type LicenseMetricAllocation struct {
	// Limit is the granted quota. It is absent when the metric is unlimited.
	// +optional
	Limit *int64 `json:"limit,omitempty"`

	// Unlimited is true when the key grants the metric without a limit; Limit is
	// then absent.
	// +optional
	Unlimited bool `json:"unlimited,omitempty"`

	// Used is what the nodes attributed to this metric consume of the quota: one
	// per node for servers, vCPU for vCPU, ceil(vCPU/2) for cores.
	// +optional
	Used int64 `json:"used"`
}

type LicenseUnlicensedNodes struct {
	// +optional
	Nodes int64 `json:"nodes"`

	// +optional
	VCPU int64 `json:"vCPU"`
}

type LicenseNodeStatus struct {
	Name string `json:"name"`

	// +optional
	VCPU int64 `json:"vCPU"`

	// Billing is the group the node was allocated to.
	// One of Free, Server, VCPU, Cores, Unlicensed.
	// +optional
	Billing LicenseNodeBilling `json:"billing,omitempty"`

	// Reason is filled for free nodes only: it says why the node is not billed.
	// +optional
	Reason string `json:"reason,omitempty"`
}
