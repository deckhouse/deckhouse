/*
Copyright 2024 The Kubernetes Authors.

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

package v1beta1

// Conditions types that are used across different objects.
const (
	// AvailableV1Beta2Condition reports if an object is available.
	AvailableV1Beta2Condition = "Available"
	// ReadyV1Beta2Condition reports if an object is ready.
	ReadyV1Beta2Condition = "Ready"
	// BootstrapConfigReadyV1Beta2Condition reports if an object's bootstrap config is ready.
	BootstrapConfigReadyV1Beta2Condition = "BootstrapConfigReady"
	// InfrastructureReadyV1Beta2Condition reports if an object's infrastructure is ready.
	InfrastructureReadyV1Beta2Condition = "InfrastructureReady"
	// MachinesReadyV1Beta2Condition surfaces detail of issues on the controlled machines, if any.
	MachinesReadyV1Beta2Condition = "MachinesReady"
	// MachinesUpToDateV1Beta2Condition surfaces details of controlled machines not up to date, if any.
	MachinesUpToDateV1Beta2Condition = "MachinesUpToDate"
	// RollingOutV1Beta2Condition reports if an object is rolling out changes to machines.
	RollingOutV1Beta2Condition = "RollingOut"
	// ScalingUpV1Beta2Condition reports if an object is scaling up.
	ScalingUpV1Beta2Condition = "ScalingUp"
	// ScalingDownV1Beta2Condition reports if an object is scaling down.
	ScalingDownV1Beta2Condition = "ScalingDown"
	// RemediatingV1Beta2Condition surfaces details about ongoing remediation of the controlled machines.
	RemediatingV1Beta2Condition = "Remediating"
	// DeletingV1Beta2Condition surfaces details about progress of the object deletion workflow.
	DeletingV1Beta2Condition = "Deleting"
	// PausedV1Beta2Condition reports if reconciliation for an object or the cluster is paused.
	PausedV1Beta2Condition = "Paused"
)

// Reasons that are used across different objects.
const (
	AvailableV1Beta2Reason    = "Available"
	NotAvailableV1Beta2Reason = "NotAvailable"
	AvailableUnknownV1Beta2Reason = "AvailableUnknown"

	ReadyV1Beta2Reason       = "Ready"
	NotReadyV1Beta2Reason    = "NotReady"
	ReadyUnknownV1Beta2Reason = "ReadyUnknown"

	UpToDateV1Beta2Reason      = "UpToDate"
	NotUpToDateV1Beta2Reason   = "NotUpToDate"
	UpToDateUnknownV1Beta2Reason = "UpToDateUnknown"

	RollingOutV1Beta2Reason    = "RollingOut"
	NotRollingOutV1Beta2Reason = "NotRollingOut"

	ScalingUpV1Beta2Reason    = "ScalingUp"
	NotScalingUpV1Beta2Reason = "NotScalingUp"

	ScalingDownV1Beta2Reason    = "ScalingDown"
	NotScalingDownV1Beta2Reason = "NotScalingDown"

	RemediatingV1Beta2Reason    = "Remediating"
	NotRemediatingV1Beta2Reason = "NotRemediating"

	NoReplicasV1Beta2Reason           = "NoReplicas"
	WaitingForReplicasSetV1Beta2Reason = "WaitingForReplicasSet"

	InvalidConditionReportedV1Beta2Reason = "InvalidConditionReported"
	InternalErrorV1Beta2Reason            = "InternalError"
	ObjectDoesNotExistV1Beta2Reason       = "ObjectDoesNotExist"
	ObjectDeletedV1Beta2Reason            = "ObjectDeleted"

	NotPausedV1Beta2Reason = "NotPaused"
	PausedV1Beta2Reason    = "Paused"

	ConnectionDownV1Beta2Reason = "ConnectionDown"

	NotDeletingV1Beta2Reason      = "NotDeleting"
	DeletingV1Beta2Reason         = "Deleting"
	DeletionCompletedV1Beta2Reason = "DeletionCompleted"

	InspectionFailedV1Beta2Reason = "InspectionFailed"

	WaitingForClusterInfrastructureReadyV1Beta2Reason = "WaitingForClusterInfrastructureReady"
	WaitingForControlPlaneInitializedV1Beta2Reason    = "WaitingForControlPlaneInitialized"
	WaitingForBootstrapDataV1Beta2Reason              = "WaitingForBootstrapData"

	ProvisionedV1Beta2Reason    = "Provisioned"
	NotProvisionedV1Beta2Reason = "NotProvisioned"
)
