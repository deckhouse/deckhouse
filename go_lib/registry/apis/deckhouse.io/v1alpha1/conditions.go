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

// Condition types on RegistryConfig.status.
const (
	// ConditionValid reports whether the resolved configuration is coherent.
	ConditionValid = "Valid"

	// ConditionUpstreamValid reports the result of the preflight probe of the
	// primary upstream: reachable, credentials accepted, sentinel content in
	// place. False means the controller is holding the last known good upstream
	// and has NOT switched over.
	ConditionUpstreamValid = "UpstreamValid"

	// ConditionStorageConverged reports whether the storage has reached the
	// state the configuration asks for.
	ConditionStorageConverged = "StorageConverged"
)

// Condition types on RegistryUpstream.status.
const (
	// ConditionAccepted reports whether the controller compiled this upstream
	// into the per-node layout.
	ConditionAccepted = "Accepted"
)

// Condition types on RegistryNode.status.
const (
	// ConditionReconciled reports whether the agent has applied the spec it
	// currently sees.
	ConditionReconciled = "Reconciled"
)

// Condition reasons.
const (
	ReasonResolved         = "Resolved"
	ReasonAccepted         = "Accepted"
	ReasonProbed           = "Probed"
	ReasonProbeFailed      = "ProbeFailed"
	ReasonUnreachable      = "Unreachable"
	ReasonAuthFailed       = "AuthFailed"
	ReasonSentinelMissing  = "SentinelMissing"
	ReasonHoldingLastGood  = "HoldingLastKnownGood"
	ReasonFillInProgress   = "FillInProgress"
	ReasonFillFailed       = "FillFailed"
	ReasonConverged        = "Converged"
	ReasonMatchConflict    = "MatchConflict"
	ReasonShadowsPrimary   = "ShadowsPrimary"
	ReasonInvalidSpec      = "InvalidSpec"
	ReasonCacheDisabled    = "CacheDisabled"
	ReasonAirGap           = "AirGap"
	ReasonAPIUnavailable   = "APIUnavailable"
	ReasonAppliedFromCache = "AppliedFromCache"
)

// StoragePhase is a coarse, human-facing summary of RegistryStorage.status.
// The machine-readable truth lives in the individual status fields; this exists
// so `kubectl get registrystorage` is readable at a glance.
//
// +kubebuilder:validation:Enum=Idle;Filling;Verifying;Ready;Failed
type StoragePhase string

const (
	// StoragePhaseIdle means no fill or verification task is running.
	StoragePhaseIdle StoragePhase = "Idle"

	// StoragePhaseFilling means the leader is being populated.
	StoragePhaseFilling StoragePhase = "Filling"

	// StoragePhaseVerifying means the contents are being checked against
	// spec.source.
	StoragePhaseVerifying StoragePhase = "Verifying"

	// StoragePhaseReady means the leader holds the expected set.
	StoragePhaseReady StoragePhase = "Ready"

	// StoragePhaseFailed means a fill or verification task failed. The upstream
	// is NOT dropped in this state.
	StoragePhaseFailed StoragePhase = "Failed"
)

// ReplicaRole is a storage replica's role in replication.
//
// +kubebuilder:validation:Enum=leader;follower
type ReplicaRole string

const (
	// ReplicaRoleLeader is the replica that is filled from the upstream (or by
	// `d8 mirror push` in air-gap) and that hands out replication tasks.
	ReplicaRoleLeader ReplicaRole = "Leader"

	// ReplicaRoleFollower is a replica filled from the leader, ahead of time.
	ReplicaRoleFollower ReplicaRole = "Follower"
)

// Mode is whether the module manages the image pull path at all.
//
// +kubebuilder:validation:Enum=Managed;Unmanaged
type Mode string

const (
	// ModeManaged means the module owns the pull path.
	ModeManaged Mode = "Managed"

	// ModeUnmanaged means it does not: no CRs, no components. The design must
	// survive the absence of these objects entirely.
	ModeUnmanaged Mode = "Unmanaged"
)

// BackendName identifies a RegistryNode backend.
//
// +kubebuilder:validation:Enum=storage;upstream
type BackendName string

const (
	// BackendStorage is the in-cluster cache.
	BackendStorage BackendName = "Storage"

	// BackendUpstream is the primary upstream, used either as the only backend
	// (cache disabled) or as a fallback while the cache is still filling. It
	// disappears from the layout when the cluster goes air-gap.
	BackendUpstream BackendName = "Upstream"
)
