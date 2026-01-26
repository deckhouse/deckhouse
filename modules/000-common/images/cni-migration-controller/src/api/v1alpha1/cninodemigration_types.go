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

// CNINodeMigrationSpec defines the desired state of CNINodeMigration.
type CNINodeMigrationSpec struct {
	// The spec can be empty, as all configuration is taken from the parent CNIMigration resource.
}

const (
	NodeConditionPodsAnnotated = "PodsAnnotated"
	NodeConditionCleanupDone   = "CleanupDone"
	NodeConditionPodsRestarted = "PodsRestarted"
)

const (
	NodePhasePreparing  = "PodsAnnotating"
	NodePhaseCleaning   = "NodeCleaning"
	NodePhaseRestarting = "RestartingPods"
	NodePhaseCompleted  = "Completed"
)

// CNINodeMigrationStatus defines the observed state of CNINodeMigration.
type CNINodeMigrationStatus struct {
	// Phase is the phase of this particular node.
	// +optional
	Phase string `json:"phase,omitempty"`

	// Conditions are the detailed conditions reflecting the steps performed on the node.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=cninodemigrations,scope=Cluster

// CNINodeMigration is the Schema for the cninodemigrations API
type CNINodeMigration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CNINodeMigrationSpec   `json:"spec,omitempty"`
	Status CNINodeMigrationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CNINodeMigrationList contains a list of CNINodeMigration
type CNINodeMigrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CNINodeMigration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CNINodeMigration{}, &CNINodeMigrationList{})
}
