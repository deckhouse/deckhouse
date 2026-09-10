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
	NodeGroupLabel      = "node.deckhouse.io/group"
	NodeTypeLabel       = "node.deckhouse.io/type"
	FencingEnabledLabel = "node-manager.deckhouse.io/fencing-enabled"
	FencingModeLabel    = "node-manager.deckhouse.io/fencing-mode"
)

const (
	FencingDisableAnnotation     = "node-manager.deckhouse.io/fencing-disable"
	DisruptionApprovedAnnotation = "update.node.deckhouse.io/disruption-approved"
	UpdateApprovedAnnotation     = "update.node.deckhouse.io/approved"
)

const (
	FencingModeNotify   = "Notify"
	FencingModeWatchdog = "Watchdog"
)

const (
	NodeTypeStatic      = "Static"
	NodeTypeCloudStatic = "CloudStatic"
)

const (
	EnvPodNamespace = "POD_NAMESPACE"
	EnvPodName      = "POD_NAME"
	EnvLogLevel     = "LOG_LEVEL"
)

// ConditionTypeConfigurationError reports that the incident cannot be processed
// because its SLA profile is unusable. Conditions sit on top of a phase instead
// of replacing it, so the state machine keeps the phase it reached.
const ConditionTypeConfigurationError = "ConfigurationError"

const (
	ReasonProfileUnavailable = "ProfileUnavailable"
	ReasonProfileResolved    = "ProfileResolved"
)

// ConditionTypeInvalidNodeReference reports that the incident does not identify
// a live Node, so the pods of that Node are never deleted on its behalf.
const ConditionTypeInvalidNodeReference = "InvalidNodeReference"

// The reasons of ConditionTypeInvalidNodeReference are the machine-readable
// causes the ADR names, one per rule of the pre-reconcile validation.
const (
	// ReasonMissingOwnerReference: metadata.ownerReferences does not hold
	// exactly one reference to a v1 Node.
	ReasonMissingOwnerReference = "MissingOwnerReference"
	// ReasonNameMismatch: the owner reference names a Node other than the one
	// metadata.name names.
	ReasonNameMismatch = "NameMismatch"
	// ReasonUIDMismatch: the Node was recreated, so the object refers to an
	// identity that no longer exists.
	ReasonUIDMismatch = "UIDMismatch"
	// ReasonNodeNotFound: the Node the object names is gone.
	ReasonNodeNotFound = "NodeNotFound"
	// ReasonNodeReferenceValid: the object identifies the live Node it names.
	ReasonNodeReferenceValid = "NodeReferenceValid"
)
