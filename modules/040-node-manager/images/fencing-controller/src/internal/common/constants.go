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

package common

const (
	ComponentName    = "fencing-controller"
	LeaderElectionID = "fencing-controller.deckhouse.io"
	ControllerName   = "fencingfailednodestate"
)

const (
	EnvPodNamespace = "POD_NAMESPACE"
	EnvLogLevel     = "LOG_LEVEL"
)

// Conditions report the problems of an incident. A problem does not change the
// phase. When the problem is gone, the incident continues from its phase.

// ConditionTypeConfigurationError reports that the incident cannot be processed
// because its SLA profile is unusable.
const ConditionTypeConfigurationError = "ConfigurationError"

const (
	ReasonProfileUnavailable = "ProfileUnavailable"
	ReasonProfileResolved    = "ProfileResolved"
)

// ConditionTypeProgressing has the status False while the eviction of an
// incident in the phase Evicting does not complete in time. The reason tells
// what stops it now. The controller removes the condition when the eviction
// completes.
const ConditionTypeProgressing = "Progressing"

// The reasons of the condition Progressing.
const (
	// ReasonVolumesAttached: the Node still has VolumeAttachments, so the
	// volumes of its pods are not free for other nodes.
	ReasonVolumesAttached = "VolumesAttached"
	// ReasonPodsRemaining: the controller could not delete pods of the Node,
	// or another error of the API stops the eviction.
	ReasonPodsRemaining = "PodsRemaining"
)

// The reasons of the events that tell the progress of an incident.
const (
	// ReasonPhaseChanged: the phase of the incident changed.
	ReasonPhaseChanged = "PhaseChanged"
	// ReasonOutOfServiceTaintSet: the controller set the out-of-service taint
	// on the Node.
	ReasonOutOfServiceTaintSet = "OutOfServiceTaintSet"
	// ReasonOutOfServiceTaintRemoved: the controller removed the out-of-service
	// taint from the Node.
	ReasonOutOfServiceTaintRemoved = "OutOfServiceTaintRemoved"
	// ReasonEvictionNotProgressing: the eviction of the Node does not
	// complete in time, or the cause changed.
	ReasonEvictionNotProgressing = "EvictionNotProgressing"
)

// ReasonTooManyUnhealthyMachines: the controller did not delete the Node of a
// cloud Cluster API machine, because the MachineHealthCheck does not replace
// it: more than half of the machines are unhealthy, or the cluster has no
// MachineHealthCheck that reports its machines.
const ReasonTooManyUnhealthyMachines = "TooManyUnhealthyMachines"
