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
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CommandName defines a single unit of work in the operation pipeline.
// +kubebuilder:validation:Enum=Backup;SyncCA;RenewPKICerts;RenewKubeconfigs;SyncManifests;JoinEtcdCluster;WaitPodReady;SyncHotReload;CertObserve
type CommandName string

const (
	CommandBackup           CommandName = "Backup"
	CommandSyncCA           CommandName = "SyncCA"
	CommandRenewPKICerts    CommandName = "RenewPKICerts"
	CommandRenewKubeconfigs CommandName = "RenewKubeconfigs"
	CommandSyncManifests    CommandName = "SyncManifests"
	CommandJoinEtcdCluster  CommandName = "JoinEtcdCluster"
	CommandWaitPodReady     CommandName = "WaitPodReady"
	CommandSyncHotReload    CommandName = "SyncHotReload"
	CommandCertObserve      CommandName = "CertObserve"
)

// OperationComponent identifies a control plane component targeted by the operation.
// +kubebuilder:validation:Enum=Etcd;KubeAPIServer;KubeControllerManager;KubeScheduler;HotReload;CertObserver
type OperationComponent string

const (
	OperationComponentEtcd                  OperationComponent = "Etcd"
	OperationComponentKubeAPIServer         OperationComponent = "KubeAPIServer"
	OperationComponentKubeControllerManager OperationComponent = "KubeControllerManager"
	OperationComponentKubeScheduler         OperationComponent = "KubeScheduler"
	OperationComponentHotReload             OperationComponent = "HotReload"
	OperationComponentCertObserver          OperationComponent = "CertObserver"
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
// Returns "" for non-static-pod components - HotReload, CertObserver
func (c OperationComponent) PodComponentName() string {
	return componentRegistry[c]
}

// ComponentRegistry returns a copy of the static pod component registry (OperationComponent -> pod name).
func ComponentRegistry() map[OperationComponent]string {
	result := make(map[OperationComponent]string, len(componentRegistry))
	for k, v := range componentRegistry {
		result[k] = v
	}
	return result
}

// SecretKey returns the main template key in d8-control-plane-manager-config secret.
// Returns "" for non-static-pod components.
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

// OperationComponentFromPodName returns the OperationComponent for a given pod component label value.
// Returns "", false if the name is not a known static pod component.
func OperationComponentFromPodName(name string) (OperationComponent, bool) {
	c, ok := podNameToComponent[name]
	return c, ok
}

// ControlPlaneOperationSpec defines the desired state of ControlPlaneOperation.
type ControlPlaneOperationSpec struct {
	// NodeName is the name of the control-plane node on which the operation should be executed.
	// +kubebuilder:validation:Required
	NodeName string `json:"nodeName"`

	// Component is the control plane component this operation targets.
	// +kubebuilder:validation:Required
	Component OperationComponent `json:"component"`

	// Commands defines the ordered list of actions to perform on the component.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Commands []CommandName `json:"commands"`

	// DesiredConfigChecksum is the expected configChecksum after the operation completed.
	// Present for Update and UpdateWithPKI commands.
	// +optional
	DesiredConfigChecksum string `json:"desiredConfigChecksum,omitempty"`

	// DesiredPKIChecksum is the expected pkiChecksum after the operation completed.
	// Present for UpdatePKI and UpdateWithPKI commands.
	// +optional
	DesiredPKIChecksum string `json:"desiredPkiChecksum,omitempty"`

	// DesiredCAChecksum is the expected caChecksum after the operation completed.
	// Present for UpdatePKI and UpdateWithPKI commands.
	// +optional
	DesiredCAChecksum string `json:"desiredCaChecksum,omitempty"`

	// Approved indicates whether this operation is allowed to proceed.
	// Only one operation per node may be approved at a time.
	// +kubebuilder:default=false
	Approved bool `json:"approved"`
}

// ObservedComponentState holds the observed certificate state of a single control plane component.
type ObservedComponentState struct {
	// CertificatesExpirationDate maps cert file names to their NotAfter timestamps.
	// +optional
	CertificatesExpirationDate map[string]metav1.Time `json:"certificatesExpirationDate,omitempty"`
}

// ControlPlaneOperationStatus defines the observed state of ControlPlaneOperation.
type ControlPlaneOperationStatus struct {
	// +optional
	// +listMapKey=type
	// +listType=map
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedState holds per-component observed state from completed CertObserve command (CertObserver component).
	// For static pod components only: Etcd, KubeAPIServer, KubeControllerManager, KubeScheduler.
	// +optional
	ObservedState map[OperationComponent]ObservedComponentState `json:"observedState,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=cpo
// +kubebuilder:printcolumn:name="Node",type="string",JSONPath=".spec.nodeName",description="Target node"
// +kubebuilder:printcolumn:name="Component",type="string",JSONPath=".spec.component",description="Target component"
// +kubebuilder:printcolumn:name="Commands",type="string",JSONPath=".spec.commands",description="Operation commands"
// +kubebuilder:printcolumn:name="Approved",type="boolean",JSONPath=".spec.approved",description="Approved for execution"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ControlPlaneOperation represents a single pending or completed action
// that must be applied to a specific component on a control-plane node.
type ControlPlaneOperation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ControlPlaneOperationSpec   `json:"spec,omitempty"`
	Status ControlPlaneOperationStatus `json:"status,omitempty"`
}

// IsRenewalOperation reports whether this CPO is a cert renewal operation (detected by name).
func (op *ControlPlaneOperation) IsRenewalOperation() bool {
	return strings.Contains(op.Name, "-certrenewal-")
}

// CertRenewalID returns op.Name if this is renewal operation, otherwise "".
// For cert-renewal-id annotation on the static pod manifest to force kubelet restart.
func (op *ControlPlaneOperation) CertRenewalID() string {
	if op.IsRenewalOperation() {
		return op.Name
	}
	return ""
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
