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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StepName is the name of a single step performed within an operation.
//
// Possible values:
//   - Backup           — backs up the current component configuration (static pod manifest, extra-files) into /etc/kubernetes/deckhouse/backup before any change.
//   - SyncCA           — synchronizes CA certificates (ca.crt, ca.key) with the current d8-pki secret.
//   - RenewPKICerts    — re-issues component certificates (server, peer, client).
//   - RenewKubeconfigs — re-issues kubeconfig files used by the component to authenticate against the API.
//   - SyncManifests    — updates the static pod manifest and accompanying files (/etc/kubernetes/manifests, extra-files) and records changes in /etc/kubernetes/deckhouse/diffs.
//   - JoinEtcdCluster  — joins a new member to the etcd cluster.
//   - WaitPodReady     — waits for the component static pod to become Ready after restart.
//   - CertObserve      — collects current certificate expiration dates for the component and publishes them to status.observedState.
//
// +kubebuilder:validation:Enum=Backup;SyncCA;RenewPKICerts;RenewKubeconfigs;SyncManifests;JoinEtcdCluster;WaitPodReady;CertObserve
type StepName string

const (
	StepBackup           StepName = "Backup"
	StepSyncCA           StepName = "SyncCA"
	StepRenewPKICerts    StepName = "RenewPKICerts"
	StepRenewKubeconfigs StepName = "RenewKubeconfigs"
	StepSyncManifests    StepName = "SyncManifests"
	StepJoinEtcdCluster  StepName = "JoinEtcdCluster"
	StepWaitPodReady     StepName = "WaitPodReady"
	StepCertObserve      StepName = "CertObserve"
)

// OperationComponent identifies the control plane component the operation targets.
//
// Possible values:
//   - Etcd                  — etcd.
//   - KubeAPIServer         — kube-apiserver.
//   - KubeControllerManager — kube-controller-manager.
//   - KubeScheduler         — kube-scheduler.
//
// +kubebuilder:validation:Enum=Etcd;KubeAPIServer;KubeControllerManager;KubeScheduler
type OperationComponent string

const (
	OperationComponentEtcd                  OperationComponent = "Etcd"
	OperationComponentKubeAPIServer         OperationComponent = "KubeAPIServer"
	OperationComponentKubeControllerManager OperationComponent = "KubeControllerManager"
	OperationComponentKubeScheduler         OperationComponent = "KubeScheduler"
)

var componentRegistry = map[OperationComponent]string{
	OperationComponentEtcd:                  "etcd",
	OperationComponentKubeAPIServer:         "kube-apiserver",
	OperationComponentKubeControllerManager: "kube-controller-manager",
	OperationComponentKubeScheduler:         "kube-scheduler",
}

// podNameToComponent is the reverse of componentRegistry, built in init.
var podNameToComponent map[string]OperationComponent

func init() {
	podNameToComponent = make(map[string]OperationComponent, len(componentRegistry))
	for comp, name := range componentRegistry {
		podNameToComponent[name] = comp
	}
}

// PodComponentName returns the static pod component name used as pod label "component" in kube-system ns.
// Returns "" for unknown components.
func (c OperationComponent) PodComponentName() string {
	return componentRegistry[c]
}

// ComponentRegistry returns the static pod component registry (OperationComponent -> pod name).
// DONT MODIFY the returned map, it must be treated as read-only by callers.
func ComponentRegistry() map[OperationComponent]string {
	return componentRegistry
}

// SecretKey returns the main template key in d8-control-plane-manager-config secret.
// Returns "" for unknown components.
func (c OperationComponent) SecretKey() string {
	name := c.PodComponentName()
	if name == "" {
		return ""
	}
	return name + ".yaml.tpl"
}

// IsStaticPodComponent returns true if this component is managed as a static pod.
func (c OperationComponent) IsStaticPodComponent() bool {
	return c.PodComponentName() != ""
}

func (c OperationComponent) LabelValue() string {
	return c.PodComponentName()
}

// OperationComponentFromPodName returns the OperationComponent for a given pod component label value.
// Returns "", false if the name is not a known static pod component.
func OperationComponentFromPodName(name string) (OperationComponent, bool) {
	c, ok := podNameToComponent[name]
	return c, ok
}

// ControlPlaneOperationSpec describes the desired state of an operation.
type ControlPlaneOperationSpec struct {
	// NodeName is the name of the control plane node on which the operation must be executed.
	// +kubebuilder:validation:Required
	NodeName string `json:"nodeName"`

	// Component is the control plane component the operation targets.
	// +kubebuilder:validation:Required
	Component OperationComponent `json:"component"`

	// Steps is the ordered list of steps to perform within the operation.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Steps []StepName `json:"steps"`

	// DesiredConfigChecksum is the expected component configuration fingerprint
	// (static pod manifest and extra-files) once the operation completes.
	//
	// Used to confirm that the operation actually applied the intended configuration:
	// after the pod restarts the fingerprint in ControlPlaneNode.status must match this value.
	// +optional
	DesiredConfigChecksum string `json:"desiredConfigChecksum,omitempty"`

	// DesiredPKIChecksum is the expected component PKI fingerprint
	// (certSANs, encryption-algorithm) once the operation completes.
	//
	// Populated only for steps that change PKI (RenewPKICerts and related).
	// +optional
	DesiredPKIChecksum string `json:"desiredPkiChecksum,omitempty"`

	// DesiredCAChecksum is the expected CA certificates fingerprint once the operation completes.
	//
	// Populated only for steps that update the root CA (SyncCA and related).
	// +optional
	DesiredCAChecksum string `json:"desiredCaChecksum,omitempty"`

	// Approved indicates whether the operation is allowed to run.
	//
	// Only one approved operation may run on a node at a time.
	// The approver controller sets this automatically based on the current control plane state.
	// +kubebuilder:default=false
	Approved bool `json:"approved"`
}

// ObservedComponentState holds the certificate state of a component collected by CertObserve.
type ObservedComponentState struct {
	// CertificatesExpirationDate maps each component certificate file name to its NotAfter timestamp.
	// Used by the module to renew certificates in time and to drive related alerts.
	// +optional
	CertificatesExpirationDate map[string]metav1.Time `json:"certificatesExpirationDate,omitempty"`
}

// ControlPlaneOperationStatus describes the observed state of an operation.
type ControlPlaneOperationStatus struct {
	// Conditions reflects the operation progress.
	//
	// The primary condition is "Completed". Its "reason" field is shown in the Phase column of `kubectl get cpo`:
	//   - InProgress — the operation is running. The current step name is shown in the CurrentStep column.
	//   - Succeeded  — the operation finished successfully.
	//   - Failed     — the operation finished with an error; details are in "message".
	//
	// In addition to "Completed", a separate condition is created for each executed step,
	// where "type" equals the step name (for example RenewPKICerts, SyncManifests).
	// +optional
	// +listMapKey=type
	// +listType=map
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedState contains the component state collected by the CertObserve step.
	// Populated only for static pod components (etcd, kube-apiserver, kube-controller-manager, kube-scheduler).
	// +optional
	ObservedState *ObservedComponentState `json:"observedState,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=cpo
// +kubebuilder:printcolumn:name="Component",type="string",JSONPath=".spec.component",description="Target component",priority=1
// +kubebuilder:printcolumn:name="Node",type="string",JSONPath=".spec.nodeName",description="Target node"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=`.status.conditions[?(@.type=="Completed")].reason`,description="Operation phase"
// +kubebuilder:printcolumn:name="CurrentStep",type="string",JSONPath=`.status.conditions[?(@.reason=="InProgress")].type`,description="Currently executing step",priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ControlPlaneOperation describes a single update operation for a control plane component on a specific node —
// for example, certificate renewal or applying a new manifest.
//
// The resource is created and updated by the control-plane-manager module automatically; users do not need to create or edit it.
//
// Useful for diagnostics: shows which operation is running on the node, which step is currently active and
// whether the operation completed successfully (see Phase and CurrentStep columns of `kubectl get cpo`).
type ControlPlaneOperation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ControlPlaneOperationSpec   `json:"spec,omitempty"`
	Status ControlPlaneOperationStatus `json:"status,omitempty"`
}

// IsObserveOnlyOperation reports whether this operation is a read-only observe for a single static-pod component.
func (op *ControlPlaneOperation) IsObserveOnlyOperation() bool {
	return op.Spec.Component.IsStaticPodComponent() &&
		len(op.Spec.Steps) == 1 &&
		op.Spec.Steps[0] == StepCertObserve
}

// HasStep reports whether operation step pipeline includes step.
func (op *ControlPlaneOperation) HasStep(step StepName) bool {
	for i := range op.Spec.Steps {
		if op.Spec.Steps[i] == step {
			return true
		}
	}
	return false
}

// +kubebuilder:object:root=true

// ControlPlaneOperationList contains a list of ControlPlaneOperation.
type ControlPlaneOperationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ControlPlaneOperation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ControlPlaneOperation{}, &ControlPlaneOperationList{})
}
