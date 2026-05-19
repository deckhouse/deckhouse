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

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// MachineDeploymentRolloutStrategyType defines the type of MachineDeployment rollout strategies.
type MachineDeploymentRolloutStrategyType string

const (
	// RollingUpdateMachineDeploymentStrategyType replaces the old MachineSet by new one using rolling update.
	RollingUpdateMachineDeploymentStrategyType MachineDeploymentRolloutStrategyType = "RollingUpdate"

	// OnDeleteMachineDeploymentStrategyType replaces old MachineSets when the deletion of the associated machines are completed.
	OnDeleteMachineDeploymentStrategyType MachineDeploymentRolloutStrategyType = "OnDelete"
)

// MachineSetDeletionOrder defines the order in which Machines are deleted when downscaling.
type MachineSetDeletionOrder string

// MachineDeploymentDeletionSpec contains configuration options for MachineDeployment deletion.
type MachineDeploymentDeletionSpec struct {
	// Order defines the order in which Machines are deleted when downscaling.
	// Defaults to "Random". Valid values are "Random", "Newest", "Oldest".
	// +optional
	Order MachineSetDeletionOrder `json:"order,omitempty"`
}

// MachineDeploymentRolloutStrategyRollingUpdate is used to control the desired behavior of rolling update.
type MachineDeploymentRolloutStrategyRollingUpdate struct {
	// MaxUnavailable is the maximum number of machines that can be unavailable during the update.
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`

	// MaxSurge is the maximum number of machines that can be scheduled above the desired number of machines.
	// +optional
	MaxSurge *intstr.IntOrString `json:"maxSurge,omitempty"`
}

// MachineDeploymentRolloutStrategy describes how to roll out machines.
type MachineDeploymentRolloutStrategy struct {
	// Type of rollout. Allowed values are RollingUpdate and OnDelete.
	// Default is RollingUpdate.
	Type MachineDeploymentRolloutStrategyType `json:"type,omitempty"`

	// RollingUpdate is the rolling update config params.
	// Present only if type = RollingUpdate.
	// +optional
	RollingUpdate MachineDeploymentRolloutStrategyRollingUpdate `json:"rollingUpdate,omitempty"`
}

// MachineDeploymentRolloutSpec defines the rollout behavior.
type MachineDeploymentRolloutSpec struct {
	// After is a field to indicate a rollout should be performed
	// after the specified time even if no changes have been made.
	// +optional
	After metav1.Time `json:"after,omitempty"`

	// Strategy specifies how to roll out machines.
	// +optional
	Strategy MachineDeploymentRolloutStrategy `json:"strategy,omitempty"`
}

// ObjectMeta is metadata that all persisted resources must have.
// This is a subset of metav1.ObjectMeta used in Machine templates.
type ObjectMeta struct {
	// Labels is a map of string keys and values.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations is an unstructured key value map.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// MachineTemplateSpec describes the data needed to create a Machine from a template.
type MachineTemplateSpec struct {
	// Standard object's metadata.
	// +optional
	ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired behavior of the machine.
	// +optional
	Spec MachineSpec `json:"spec,omitempty"`
}

// MachineDeploymentSpec defines the desired state of MachineDeployment.
type MachineDeploymentSpec struct {
	// ClusterName is the name of the Cluster this object belongs to.
	ClusterName string `json:"clusterName,omitempty"`

	// Replicas is the number of desired machines.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Rollout allows you to configure the behaviour of rolling updates.
	// +optional
	Rollout MachineDeploymentRolloutSpec `json:"rollout,omitempty"`

	// Selector is the label selector for machines.
	Selector metav1.LabelSelector `json:"selector,omitempty"`

	// Template describes the machines that will be created.
	Template MachineTemplateSpec `json:"template,omitempty"`

	// Remediation controls how unhealthy Machines are remediated.
	// +optional
	Remediation MachineDeploymentRemediationSpec `json:"remediation,omitempty"`

	// Deletion contains configuration options for MachineDeployment deletion.
	// +optional
	Deletion MachineDeploymentDeletionSpec `json:"deletion,omitempty"`

	// Paused indicates that the deployment is paused.
	// +optional
	Paused *bool `json:"paused,omitempty"`
}

// MachineDeploymentRemediationSpec controls how unhealthy Machines are remediated.
type MachineDeploymentRemediationSpec struct {
	// MaxInFlight determines how many in flight remediations should happen at the same time.
	// +optional
	MaxInFlight *intstr.IntOrString `json:"maxInFlight,omitempty"`
}

// MachineDeploymentStatus defines the observed state of MachineDeployment.
type MachineDeploymentStatus struct {
	// Conditions represents the observations of a MachineDeployment's current state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation observed by the deployment controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Selector is the same as the label selector but in string format.
	// +optional
	Selector string `json:"selector,omitempty"`

	// Replicas is the total number of non-terminated machines targeted by this deployment.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// ReadyReplicas is the total number of ready machines targeted by this deployment.
	// +optional
	ReadyReplicas *int32 `json:"readyReplicas,omitempty"`

	// AvailableReplicas is the total number of available machines targeted by this deployment.
	// +optional
	AvailableReplicas *int32 `json:"availableReplicas,omitempty"`

	// UpToDateReplicas is the total number of non-terminated machines targeted by this deployment
	// that have the desired template spec. Replaces UpdatedReplicas from v1beta1.
	// +optional
	UpToDateReplicas *int32 `json:"upToDateReplicas,omitempty"`

	// Phase represents the current phase of a MachineDeployment.
	// +optional
	Phase string `json:"phase,omitempty"`
}

// MachineDeploymentPhase indicates the progress of the machine deployment.
type MachineDeploymentPhase string

const (
	// MachineDeploymentPhaseScalingUp indicates the MachineDeployment is scaling up.
	MachineDeploymentPhaseScalingUp = MachineDeploymentPhase("ScalingUp")

	// MachineDeploymentPhaseScalingDown indicates the MachineDeployment is scaling down.
	MachineDeploymentPhaseScalingDown = MachineDeploymentPhase("ScalingDown")

	// MachineDeploymentPhaseRunning indicates scaling has completed and all Machines are running.
	MachineDeploymentPhaseRunning = MachineDeploymentPhase("Running")

	// MachineDeploymentPhaseFailed indicates there was a problem scaling.
	MachineDeploymentPhaseFailed = MachineDeploymentPhase("Failed")

	// MachineDeploymentPhaseUnknown indicates the state of the MachineDeployment cannot be determined.
	MachineDeploymentPhaseUnknown = MachineDeploymentPhase("Unknown")
)

// SetTypedPhase sets the Phase field to the string representation of MachineDeploymentPhase.
func (md *MachineDeploymentStatus) SetTypedPhase(p MachineDeploymentPhase) {
	md.Phase = string(p)
}

// GetTypedPhase attempts to parse the Phase field and return
// the typed MachineDeploymentPhase representation.
func (md *MachineDeploymentStatus) GetTypedPhase() MachineDeploymentPhase {
	switch phase := MachineDeploymentPhase(md.Phase); phase {
	case
		MachineDeploymentPhaseScalingDown,
		MachineDeploymentPhaseScalingUp,
		MachineDeploymentPhaseRunning,
		MachineDeploymentPhaseFailed:
		return phase
	default:
		return MachineDeploymentPhaseUnknown
	}
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=machinedeployments,shortName=md,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:subresource:status

// MachineDeployment is the Schema for the machinedeployments API.
type MachineDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MachineDeploymentSpec   `json:"spec,omitempty"`
	Status MachineDeploymentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MachineDeploymentList contains a list of MachineDeployment.
type MachineDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MachineDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MachineDeployment{}, &MachineDeploymentList{})
}

// GetConditions returns the set of conditions for the machinedeployment.
func (m *MachineDeployment) GetConditions() []metav1.Condition {
	return m.Status.Conditions
}

// SetConditions updates the set of conditions on the machinedeployment.
func (m *MachineDeployment) SetConditions(conditions []metav1.Condition) {
	m.Status.Conditions = conditions
}
