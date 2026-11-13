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

// NodeGroupLabel assigns a Node to a NodeGroup.
const NodeGroupLabel = "node.deckhouse.io/group"

// Maintenance annotations. Any of them disarms the watchdog of the Node.
const (
	// FencingDisableAnnotation is set by an operator for manual maintenance.
	FencingDisableAnnotation = "node-manager.deckhouse.io/fencing-disable"
	// DisruptionApprovedAnnotation marks an approved disruptive node update.
	DisruptionApprovedAnnotation = "update.node.deckhouse.io/disruption-approved"
	// ApprovedAnnotation marks an approved node update of any kind.
	ApprovedAnnotation = "update.node.deckhouse.io/approved"
)

// MaintenanceAnnotations returns the maintenance annotations in a stable order.
func MaintenanceAnnotations() []string {
	return []string{
		FencingDisableAnnotation,
		DisruptionApprovedAnnotation,
		ApprovedAnnotation,
	}
}

// NodeSignals is the state of the own Node that the agent uses.
type NodeSignals struct {
	UID       string
	NodeGroup string
	// InternalIPs and ExternalIPs keep the order of the Node status.
	InternalIPs []string
	ExternalIPs []string
	// Maintenance is true while one or more maintenance annotations are present.
	Maintenance bool
	// MaintenanceReasons holds the maintenance annotations that are present.
	MaintenanceReasons []string
}
