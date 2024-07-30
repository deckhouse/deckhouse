/*
Copyright 2023 Flant JSC

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
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Instance is resource for instance in the cloud.
type Instance struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Most recently observed status of the instance.
	Status InstanceStatus `json:"status,omitempty"`
}

// InstanceStatus is a status of instance.
type InstanceStatus struct {
	// Reference to kubernetes node object
	NodeRef NodeRef `json:"nodeRef,omitempty"`

	// Reference to specific machine in the cloud
	MachineRef MachineRef `json:"machineRef,omitempty"`

	// Current status of the instance object
	CurrentStatus CurrentStatus `json:"currentStatus,omitempty"`

	// Last operation refers to the status of the last operation performed
	LastOperation LastOperation `json:"lastOperation,omitempty"`

	// Information about instance bootstrapping process
	BootstrapStatus BootstrapStatus `json:"bootstrapStatus,omitempty"`

	// Reference to a ClassInstance resource.
	ClassReference ClassReference `json:"classReference,omitempty"`
}

type ClassReference struct {
	// Kind of a ClassReference resource: OpenStackInstanceClass, GCPInstanceClass, ...
	Kind string `json:"kind,omitempty"`

	// Name of a ClassReference resource.
	Name string `json:"name,omitempty"`
}

// BootstrapStatus is information about bootstrapping process
type BootstrapStatus struct {
	// Endpoint for getting bootstrap logs
	LogsEndpoint string `json:"logsEndpoint,omitempty"`
	Description  string `json:"description,omitempty"`
}

// State is a current state of the machine.
type State string

// These are the valid statuses of machines.
const (
	// StatePending means there are operations pending on this machine state
	StateProcessing State = "Processing"

	// StateFailed means operation failed leading to machine status failure
	StateFailed State = "Failed"

	// StateSuccessful indicates that the node is not ready at the moment
	StateSuccessful State = "Successful"
)

// OperationType is a label for the operation performed on a machine object.
type OperationType string

// These are the valid statuses of machines.
const (
	// OperationCreate indicates that the operation was a create
	OperationCreate OperationType = "Create"

	// OperationUpdate indicates that the operation was an update
	OperationUpdate OperationType = "Update"

	// OperationHealthCheck indicates that the operation was a create
	OperationHealthCheck OperationType = "HealthCheck"

	// OperationDelete indicates that the operation was a create
	OperationDelete OperationType = "Delete"
)

// LastOperation suggests the last operation performed on the object
type LastOperation struct {
	// Description of the current operation
	Description string `json:"description,omitempty"`

	// Last update time of current operation
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`

	// State of operation
	State State `json:"state,omitempty"`

	// Type of operation
	Type OperationType `json:"type,omitempty"`
}

// InstancePhase is a label for the condition of a machines at the current time.
type InstancePhase string

// These are the valid statuses of machines.
const (
	// InstancePending means that the machine is being created
	InstancePending InstancePhase = "Pending"

	// InstanceAvailable means that machine is present on provider but hasn't joined cluster yet
	InstanceAvailable InstancePhase = "Available"

	// InstanceRunning means node is ready and running successfully
	InstanceRunning InstancePhase = "Running"

	// InstanceTerminating means node is terminating
	InstanceTerminating InstancePhase = "Terminating"

	// InstanceUnknown indicates that the node is not ready at the movement
	InstanceUnknown InstancePhase = "Unknown"

	// InstanceFailed means operation failed leading to machine status failure
	InstanceFailed InstancePhase = "Failed"

	// InstanceCrashLoopBackOff means creation or deletion of the machine is failing.
	InstanceCrashLoopBackOff InstancePhase = "CrashLoopBackOff"
)

// CurrentStatus contains information about the current status of Machine.
type CurrentStatus struct {
	Phase InstancePhase `json:"phase,omitempty"`

	// Last update time of current status
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`
}

// MachineRef is reference to specific machine object
type MachineRef struct {
	Kind string `json:"kind,omitempty"`

	APIVersion string `json:"apiVersion,omitempty"`

	Name string `json:"name,omitempty"`

	Namespace string `json:"namespace,omitempty"`
}

// NodeRef is reference to node object.
type NodeRef struct {
	// Node object name
	Name string `json:"name,omitempty"`
}

type instanceKind struct{}

func (in *InstanceStatus) GetObjectKind() schema.ObjectKind {
	return &instanceKind{}
}

func (f *instanceKind) SetGroupVersionKind(_ schema.GroupVersionKind) {}
func (f *instanceKind) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "Instance"}
}
