package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type MachineDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MachineDeploymentSpec   `json:"spec,omitempty"`
	Status MachineDeploymentStatus `json:"status,omitempty"`
}

type MachineDeploymentSpec struct {
	Replicas             int32                     `json:"replicas,omitempty"`
	Selector             *metav1.LabelSelector     `json:"selector,omitempty"`
	Template             MachineTemplateSpec       `json:"template"`
	Strategy             MachineDeploymentStrategy `json:"strategy,omitempty"`
	MinReadySeconds      int32                     `json:"minReadySeconds,omitempty"`
	RevisionHistoryLimit *int32                    `json:"revisionHistoryLimit,omitempty"`
	Paused               bool                      `json:"paused,omitempty"`
}

type MachineDeploymentStrategy struct {
	Type          MachineDeploymentStrategyType   `json:"type,omitempty"`
	RollingUpdate *RollingUpdateMachineDeployment `json:"rollingUpdate,omitempty"`
}

type MachineDeploymentStrategyType string

const (
	RecreateMachineDeploymentStrategyType      MachineDeploymentStrategyType = "Recreate"
	RollingUpdateMachineDeploymentStrategyType MachineDeploymentStrategyType = "RollingUpdate"
)

type RollingUpdateMachineDeployment struct {
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
	MaxSurge       *intstr.IntOrString `json:"maxSurge,omitempty"`
}

type MachineDeploymentStatus struct {
	ObservedGeneration  int64                        `json:"observedGeneration,omitempty"`
	Replicas            int32                        `json:"replicas,omitempty"`
	UpdatedReplicas     int32                        `json:"updatedReplicas,omitempty"`
	ReadyReplicas       int32                        `json:"readyReplicas,omitempty"`
	AvailableReplicas   int32                        `json:"availableReplicas,omitempty"`
	UnavailableReplicas int32                        `json:"unavailableReplicas,omitempty"`
	Conditions          []MachineDeploymentCondition `json:"conditions,omitempty"`
	FailedMachines      []*MachineSummary            `json:"failedMachines,omitempty"`
}

type MachineDeploymentConditionType string

const (
	MachineDeploymentAvailable      MachineDeploymentConditionType = "Available"
	MachineDeploymentProgressing    MachineDeploymentConditionType = "Progressing"
	MachineDeploymentReplicaFailure MachineDeploymentConditionType = "ReplicaFailure"
	MachineDeploymentFrozen         MachineDeploymentConditionType = "Frozen"
)

type MachineDeploymentCondition struct {
	Type               MachineDeploymentConditionType `json:"type"`
	Status             ConditionStatus                `json:"status"`
	LastUpdateTime     metav1.Time                    `json:"lastUpdateTime,omitempty"`
	LastTransitionTime metav1.Time                    `json:"lastTransitionTime,omitempty"`
	Reason             string                         `json:"reason,omitempty"`
	Message            string                         `json:"message,omitempty"`
}

type ConditionStatus string

const (
	ConditionTrue    ConditionStatus = "True"
	ConditionFalse   ConditionStatus = "False"
	ConditionUnknown ConditionStatus = "Unknown"
)

type MachineTemplateSpec struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              MachineSpec `json:"spec,omitempty"`
}

type MachineSummary struct {
	Name          string               `json:"name,omitempty"`
	ProviderID    string               `json:"providerID,omitempty"`
	LastOperation MachineLastOperation `json:"lastOperation,omitempty"`
	OwnerRef      string               `json:"ownerRef,omitempty"`
}

// +kubebuilder:object:root=true
type MachineDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MachineDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MachineDeployment{}, &MachineDeploymentList{})
}
