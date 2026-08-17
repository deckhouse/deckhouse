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

package domain

// Maintenance annotations. Any of them disables fencing for the Node: the same
// three keys already exclude a Node in the node-manager fencing hook, and agent
// and controller must never disagree on what maintenance means.
const (
	// FencingDisableAnnotation is set by an operator for manual maintenance.
	FencingDisableAnnotation = "node-manager.deckhouse.io/fencing-disable"
	// DisruptionApprovedAnnotation marks an approved disruptive node update.
	DisruptionApprovedAnnotation = "update.node.deckhouse.io/disruption-approved"
	// ApprovedAnnotation marks an approved node update of any kind.
	ApprovedAnnotation = "update.node.deckhouse.io/approved"
)

// ClusterAutoscalerDeleteTaint marks a Node the cluster-autoscaler is about to remove.
const ClusterAutoscalerDeleteTaint = "ToBeDeletedByClusterAutoscaler"

// Reasons for a planned removal, reported in logs and Kubernetes Events.
const (
	RemovalReasonDeleting   = "NodeDeleting"
	RemovalReasonAutoscaler = "ClusterAutoscalerTaint"
	RemovalReasonDeleted    = "NodeDeleted"
)

// MaintenanceAnnotations returns the annotations that disable fencing, in a
// stable order so logs and Events stay comparable.
func MaintenanceAnnotations() []string {
	return []string{
		FencingDisableAnnotation,
		DisruptionApprovedAnnotation,
		ApprovedAnnotation,
	}
}

// NodeSignals is the watchdog-relevant view of the agent's own Node. It carries
// no Kubernetes types so the policy layer never has to know about them.
type NodeSignals struct {
	// UID detects a Node recreated under the same name.
	UID string
	// Maintenance is true while any maintenance annotation is present.
	Maintenance bool
	// MaintenanceReasons holds the annotations actually present.
	MaintenanceReasons []string
	// PlannedRemoval is true once the Node is known to be on its way out.
	PlannedRemoval bool
	RemovalReason  string
}
