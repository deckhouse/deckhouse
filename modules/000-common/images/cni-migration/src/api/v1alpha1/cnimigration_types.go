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

// CNIMigrationSpec defines the desired state of CNIMigration.
type CNIMigrationSpec struct {
	// TargetCNI is the CNI to switch to.
	// Set by the d8 cli utility when starting Phase 1.
	TargetCNI string `json:"targetCNI"`

	// Phase is the phase controlled by the d8 cli to command the agents.
	// Possible values: Prepare, Migrate, Cleanup, Abort.
	Phase string `json:"phase"`
}

// CNIMigrationStatus defines the observed state of CNIMigration.
type CNIMigrationStatus struct {
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

	// Conditions reflect the state of the migration as a whole.
	// The d8 cli aggregates statuses from all CNINodeMigrations here.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
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
