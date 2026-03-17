/*
Copyright 2025 Flant JSC

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
	CNINameCilium       = "cilium"
	CNINameFlannel      = "flannel"
	CNINameSimpleBridge = "simple-bridge"

	PhasePreparing           = "Preparing"
	PhaseWaitingForAgents    = "WaitingForAgents"
	PhaseEnablingTargetCNI   = "EnablingTargetCNI"
	PhaseDisablingCurrentCNI = "DisablingCurrentCNI"
	PhaseCleaningNodes       = "CleaningNodes"
	PhaseWaitingTargetCNI    = "WaitingTargetCNI"
	PhaseRestartingPods      = "RestartingPods"
	PhaseCompleted           = "Completed"

	ConditionEnvironmentPrepared       = "EnvironmentPrepared"
	ConditionCurrentCNIDetectionFailed = "CurrentCNIDetectionFailed"
	ConditionAgentsReady               = "AgentsReady"
	ConditionTargetCNIEnabled          = "TargetCNIEnabled"
	ConditionCurrentCNIDisabled        = "CurrentCNIDisabled"
	ConditionNodesCleaned              = "NodesCleaned"
	ConditionTargetCNIReady            = "TargetCNIReady"
	ConditionPodsRestarted             = "PodsRestarted"
	ConditionSucceeded                 = "Succeeded"
)

// CNIMigrationSpec defines the desired state of CNIMigration.
type CNIMigrationSpec struct {
	// TargetCNI is the CNI to switch to (e.g., cilium, flannel).
	TargetCNI string `json:"targetCNI"`
}

// CNIMigrationStatus defines the observed state of CNIMigration.
type CNIMigrationStatus struct {
	// Phase indicates the current step of the migration process.
	// +optional
	Phase string `json:"phase,omitempty"`

	// CurrentCNI is the detected CNI from which the switch is being made.
	// +optional
	CurrentCNI string `json:"currentCNI,omitempty"`

	// NodesTotal is the total number of nodes involved in the migration.
	// +optional
	NodesTotal int `json:"nodesTotal,omitempty"`

	// NodesSucceeded is the number of nodes that have successfully completed the current phase.
	// +optional
	NodesSucceeded int `json:"nodesSucceeded,omitempty"`

	// NodesFailed is the number of nodes where an error occurred.
	// +optional
	NodesFailed int `json:"nodesFailed,omitempty"`

	// FailedSummary contains details about nodes that failed the migration.
	// +optional
	FailedSummary []FailedNodeSummary `json:"failedSummary,omitempty"`

	// Conditions reflect the state of the migration as a whole.
	// The controller aggregates statuses from all CNINodeMigrations here.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// FailedNodeSummary captures the error state of a specific node.
type FailedNodeSummary struct {
	Node   string `json:"node"`
	Reason string `json:"reason"`
}

// CNIMigration is the Schema for the cnimigrations API
type CNIMigration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CNIMigrationSpec   `json:"spec,omitempty"`
	Status CNIMigrationStatus `json:"status,omitempty"`
}

// CNIMigrationList contains a list of CNIMigration
type CNIMigrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CNIMigration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CNIMigration{}, &CNIMigrationList{})
}
