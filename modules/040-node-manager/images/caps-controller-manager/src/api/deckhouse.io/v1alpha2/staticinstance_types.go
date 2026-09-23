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

package v1alpha2

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/util/conditions"
)

// Connectivity condition types set on a StaticInstance while its StaticMachine bootstraps.
//
// They are declared here, next to the ToPending that has to clear them, and re-exported by
// api/infrastructure/v1alpha1 for the controllers that set them. The dependency runs this way
// because api/infrastructure/v1alpha1 is reachable from this package through the v1alpha1
// conversion, so importing it back would close a cycle in the test build.
const (
	StaticInstanceCheckTCPConnectionCondition = "CheckTcpConnection"
	StaticInstanceCheckSSHConnectionCondition = "CheckSshCondition"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// StaticInstanceSpec defines the desired state of StaticInstance.
type StaticInstanceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The IP address of the host.
	//+kubebuilder:validation:Pattern=`^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$`
	Address string `json:"address"`

	// The reference to the `SSHCredentials` object.
	CredentialsRef *corev1.ObjectReference `json:"credentialsRef"`

	// The name the node registers under in the cluster.
	//
	// Leave it unset and the node keeps the name it has always had: its hostname.
	// Set it and the hostname of the host is left alone - only the Node object is
	// named this way. The name is fixed when the instance is bootstrapped, so
	// changing it afterwards has no effect on a node that is already running.
	//+optional
	//+kubebuilder:validation:MaxLength=253
	//+kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	NodeName string `json:"nodeName,omitempty"`
}

// StaticInstanceStatus defines the observed state of StaticInstance.
type StaticInstanceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +optional
	// The reference to the `StaticMachine` object.
	MachineRef *corev1.ObjectReference `json:"machineRef,omitempty"`

	// +optional
	// The reference to the `Node` object.
	NodeRef *corev1.ObjectReference `json:"nodeRef,omitempty"`

	// +optional
	CurrentStatus *StaticInstanceStatusCurrentStatus `json:"currentStatus,omitempty"`

	// Conditions defines current service state of the StaticInstance.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type StaticInstanceStatusCurrentStatus struct {
	// +optional
	LastUpdateTime metav1.Time `json:"lastUpdateTime"`

	// +optional
	// +kubebuilder:validation:Enum=Error;Pending;Bootstrapping;Running;Cleaning
	Phase StaticInstanceStatusCurrentStatusPhase `json:"phase"`
}

type StaticInstanceStatusCurrentStatusPhase string

const (
	StaticInstanceStatusCurrentStatusPhaseError         StaticInstanceStatusCurrentStatusPhase = "Error"
	StaticInstanceStatusCurrentStatusPhasePending       StaticInstanceStatusCurrentStatusPhase = "Pending"
	StaticInstanceStatusCurrentStatusPhaseBootstrapping StaticInstanceStatusCurrentStatusPhase = "Bootstrapping"
	StaticInstanceStatusCurrentStatusPhaseRunning       StaticInstanceStatusCurrentStatusPhase = "Running"
	StaticInstanceStatusCurrentStatusPhaseCleaning      StaticInstanceStatusCurrentStatusPhase = "Cleaning"
)

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:subresource:status
//+kubebuilder:storageversion
//+kubebuilder:metadata:labels="heritage=deckhouse"
//+kubebuilder:metadata:labels="module=node-manager"
//+kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.currentStatus.phase",description="Static instance state"
//+kubebuilder:printcolumn:name="Node",type="string",JSONPath=".status.nodeRef.name",description="Node associated with this static instance"
//+kubebuilder:printcolumn:name="Machine",type="string",JSONPath=".status.machineRef.name",description="Static machine associated with this static instance"

// StaticInstance describes a machine for the Cluster API Provider Static.
type StaticInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StaticInstanceSpec   `json:"spec,omitempty"`
	Status StaticInstanceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster

// StaticInstanceList contains a list of StaticInstance.
type StaticInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StaticInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StaticInstance{}, &StaticInstanceList{})
}

// GetConditions gets the StaticInstance status conditions
func (r *StaticInstance) GetConditions() []metav1.Condition {
	return r.Status.Conditions
}

// SetConditions sets the StaticInstance status conditions
func (r *StaticInstance) SetConditions(conditions []metav1.Condition) {
	r.Status.Conditions = conditions
}

// SetPhase gets the current phase of the static instance.
func (r *StaticInstance) GetPhase() StaticInstanceStatusCurrentStatusPhase {
	if r.Status.CurrentStatus == nil {
		return ""
	}

	return r.Status.CurrentStatus.Phase
}

// SetPhase sets the current phase of the static instance.
//
// LastUpdateTime is refreshed only on a real phase transition: rewriting it on every
// call makes the patch helper see a diff on every reconcile, which turns into a write
// to etcd, a watch event and an immediate re-reconcile. It also keeps the phase
// timeouts (bootstrap, cleanup) from ever being reached.
func (r *StaticInstance) SetPhase(phase StaticInstanceStatusCurrentStatusPhase) {
	if r.Status.CurrentStatus == nil {
		r.Status.CurrentStatus = &StaticInstanceStatusCurrentStatus{}
	} else if r.Status.CurrentStatus.Phase == phase {
		return
	}

	r.Status.CurrentStatus.Phase = phase
	r.Status.CurrentStatus.LastUpdateTime = metav1.NewTime(time.Now().UTC())
}

func (r *StaticInstance) ToPending() {
	r.Status.MachineRef = nil
	r.Status.NodeRef = nil

	// The connectivity checks belong to the StaticMachine being detached, and nothing else
	// clears them: metadata.generation cannot be used to tell a stale one from a fresh one,
	// because the status subresource keeps it pinned for the whole life of the object. A
	// leftover CheckTcpConnection=True would make the next StaticMachine skip the TCP check
	// and go straight to ssh against a host that may well be gone.
	conditions.Delete(r, StaticInstanceCheckTCPConnectionCondition)
	conditions.Delete(r, StaticInstanceCheckSSHConnectionCondition)

	conditions.Set(r, metav1.Condition{
		Type:               "BootstrapSucceeded",
		Status:             metav1.ConditionFalse,
		Reason:             "WaitingForNodeRefToBeAssigned",
		Message:            "StaticInstance is pending",
		LastTransitionTime: metav1.Now(),
	})

	r.SetPhase(StaticInstanceStatusCurrentStatusPhasePending)
}
