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

package v1beta2

// MachinePhase is a string representation of a Machine Phase.
type MachinePhase string

const (
	// MachinePhasePending is the first state a Machine is assigned by
	// Cluster API Machine controller after being created.
	MachinePhasePending = MachinePhase("Pending")

	// MachinePhaseProvisioning is the state when the
	// Machine infrastructure is being created.
	MachinePhaseProvisioning = MachinePhase("Provisioning")

	// MachinePhaseProvisioned is the state when its
	// infrastructure has been created and configured.
	MachinePhaseProvisioned = MachinePhase("Provisioned")

	// MachinePhaseRunning is the Machine state when it has
	// become a Kubernetes Node in a Ready state.
	MachinePhaseRunning = MachinePhase("Running")

	// MachinePhaseDeleting is the Machine state when a delete
	// request has been sent to the API Server,
	// but its infrastructure has not yet been fully deleted.
	MachinePhaseDeleting = MachinePhase("Deleting")

	// MachinePhaseDeleted is the Machine state when the object
	// and the related infrastructure is deleted and
	// ready to be garbage collected by the API Server.
	MachinePhaseDeleted = MachinePhase("Deleted")

	// MachinePhaseFailed is the Machine state when the system
	// might require user intervention.
	MachinePhaseFailed = MachinePhase("Failed")

	// MachinePhaseUnknown is returned if the Machine state cannot be determined.
	MachinePhaseUnknown = MachinePhase("Unknown")
)
