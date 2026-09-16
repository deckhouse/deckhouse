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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	ApplicationResource = "applications"
	ApplicationKind     = "Application"

	ApplicationFinalizerStatisticRegistered = "application.deckhouse.io/statistic-registered"
)

var (
	ApplicationGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: ApplicationResource,
	}
	ApplicationGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    ApplicationKind,
	}
)

var _ runtime.Object = (*Application)(nil)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name=Package,type=string,JSONPath=.spec.packageName
// +kubebuilder:printcolumn:name=Version,type=string,JSONPath=.spec.packageVersion
// +kubebuilder:printcolumn:name=Repository,type=string,JSONPath=.spec.packageRepositoryName,priority=1
// +kubebuilder:printcolumn:name=State,type=string,JSONPath=.status.summary.state
// +kubebuilder:printcolumn:name=Installed,type=string,JSONPath=.status.conditions[?(@.type=='Installed')].status,priority=1
// +kubebuilder:printcolumn:name=Ready,type=string,JSONPath=.status.conditions[?(@.type=='Ready')].status,priority=1
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.summary.message"
// +kubebuilder:printcolumn:name=Age,type=date,JSONPath=.metadata.creationTimestamp
// +crd-enricher:raw:properties.apiVersion.description="APIVersion defines the versioned schema of this representation of an object.\nServers should convert recognized schemas to the latest internal value, and\nmay reject unrecognized values.\n\nMore info [in the Kubernetes documentation](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources)."
// +crd-enricher:raw:properties.kind.description="Kind is a string value representing the REST resource this object represents.\nServers may infer this from the endpoint the client submits requests to.\nCannot be updated.\nIn CamelCase.\n\nMore info [in the Kubernetes documentation](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds)."

// +crd-enricher:deckhouse:documentation:examples={apiVersion: deckhouse.io/v1alpha1, kind: Application, metadata: {name: example}, spec: {maintenance: NoResourceReconciliation, packageName: console, packageRepositoryName: deckhouse, packageVersion: v1.0.0, releaseChannel: stable}}
// Application represents a namespace-scoped application instance.
type Application struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Defines the application configuration.
	Spec ApplicationSpec `json:"spec"`

	// Application status.
	Status ApplicationStatus `json:"status,omitempty"`
}

type ApplicationSpec struct {
	// Name of the application package to install.
	// +crd-enricher:deckhouse:documentation:examples=console
	PackageName string `json:"packageName"`

	// Name of the repository where the package is located.
	// If not specified, the default repository is used.
	// +optional
	// +crd-enricher:deckhouse:documentation:examples=deckhouse
	PackageRepositoryName string `json:"packageRepositoryName,omitempty"`

	// Version of the application package to install.
	// +crd-enricher:deckhouse:documentation:examples=v1.0.0
	PackageVersion string `json:"packageVersion"`

	// Release channel for the application package.
	// +optional
	// +crd-enricher:deckhouse:documentation:examples=stable
	ReleaseChannel string `json:"releaseChannel,omitempty"`

	// Configuration settings for the application.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Settings *MappedFields `json:"settings,omitempty"`

	// Defines the application maintenance mode.
	//
	// - `NoResourceReconciliation`: A mode for developing or tweaking the application.
	//
	//   In this mode:
	//
	//   - Configuration or hook changes are not reconciled, which prevents resources from being updated automatically.
	//   - Resource monitoring is disabled, which prevents deleted resources from being restored.
	//   - All the application's resources are labeled with `maintenance.deckhouse.io/no-resource-reconciliation`.
	//   - The [`ApplicationIsInMaintenanceMode`](/products/kubernetes-platform/documentation/v1/reference/alerts.html#deckhouse-applicationisinmaintenancemode) alert is triggered.
	// +kubebuilder:validation:Enum=NoResourceReconciliation
	// +optional
	// +crd-enricher:deckhouse:documentation:examples=NoResourceReconciliation
	Maintenance string `json:"maintenance,omitempty"`

	// Resource footprint of the workloads the application deploys: how many
	// replicas each workload runs and what its containers may consume. Entries
	// override what the application package ships; a workload or container that
	// is not listed keeps the package defaults.
	//
	// A workload is addressed by its kind and name, so at most one entry exists
	// per pair.
	// +listType=map
	// +listMapKey=kind
	// +listMapKey=name
	// +optional
	ResourceRequests []ApplicationResourceRequest `json:"resourceRequests,omitempty"`
}

// ApplicationResourceRequest pins the resource footprint of a single workload
// that the application deploys.
// +kubebuilder:validation:XValidation:rule="has(self.replicas) ? self.kind in ['Deployment','StatefulSet','ReplicaSet'] : true",message="replicas can only be set for Deployment, StatefulSet and ReplicaSet"
type ApplicationResourceRequest struct {
	// Kind of the workload to size.
	// +kubebuilder:validation:Enum=Deployment;StatefulSet;DaemonSet;ReplicaSet;Pod;Job;CronJob
	// +crd-enricher:deckhouse:documentation:examples=Deployment
	Kind string `json:"kind"`

	// Name of the workload to size, as the application package names it.
	// +kubebuilder:validation:MinLength=1
	// +crd-enricher:deckhouse:documentation:examples=controller
	Name string `json:"name"`

	// Number of replicas the workload runs. Only meaningful for the kinds that
	// have a replica count: `Deployment`, `StatefulSet` and `ReplicaSet`.
	// +kubebuilder:validation:Minimum=0
	// +optional
	// +crd-enricher:deckhouse:documentation:examples=3
	Replicas *int32 `json:"replicas,omitempty"`

	// Compute resources of the workload's containers. A container the workload
	// does not have is ignored.
	// +listType=map
	// +listMapKey=name
	// +optional
	Containers []ApplicationContainerResourceRequest `json:"containers,omitempty"`
}

// ApplicationContainerResourceRequest pins the compute resources of a single
// container inside a workload.
type ApplicationContainerResourceRequest struct {
	// Name of the container inside the workload.
	// +kubebuilder:validation:MinLength=1
	// +crd-enricher:deckhouse:documentation:examples=controller
	Name string `json:"name"`

	// Compute resources the container requests and is limited to, in the same
	// form as a Pod container's `resources`: `cpu`, `memory` and
	// `ephemeral-storage` for disk.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

type ApplicationStatus struct {
	// Summary aggregates the high-level user-facing state, message and
	// resolution hint for the application. The controller always populates it
	// on reconcile — every application maps to exactly one lifecycle state — so
	// it is the single source of truth for the UI; clients should not re-derive
	// these values from the conditions. The pointer leaves it absent only
	// before the first status computation.
	// +optional
	Summary *ApplicationStatusSummary `json:"summary,omitempty"`

	// Information about the currently installed version.
	// +optional
	CurrentVersion *ApplicationStatusVersion `json:"currentVersion,omitempty"`

	// URLs of application endpoints, collected from Ingress resources of the
	// application chart annotated with `packages.deckhouse.io/application-endpoint-description`.
	// +optional
	URLs []ApplicationStatusURL `json:"urls,omitempty"`

	// Nelm tracking.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Tracking runtime.RawExtension `json:"tracking"`

	// LastAppliedConfiguration is the effective settings (user configuration merged
	// with config-schema defaults) that drove the most recent successful apply.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	LastAppliedConfiguration runtime.RawExtension `json:"lastAppliedConfiguration"`

	// Conditions reflecting the latest observations of the application state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// ApplicationStatusSummary aggregates the high-level lifecycle state, message
// and resolution hint for the application. It is consumed by the UI as a single
// source of truth so that the frontend does not have to re-implement the state
// machine on top of conditions.
type ApplicationStatusSummary struct {
	// State is the high-level lifecycle state observed for the application.
	// Always one of: Pending, Failed, Updating, Ready, Degraded, Suspended, Deleting.
	// +optional
	// +crd-enricher:deckhouse:documentation:examples=[Pending, Failed, Updating, Ready, Degraded, Suspended, Deleting]
	State string `json:"state,omitempty"`

	// Message is a human-readable description of the current state.
	// +optional
	Message string `json:"message,omitempty"`

	// Tip is a human-readable instruction on how to resolve the current
	// state. Empty when no action is required.
	// +optional
	Tip string `json:"tip,omitempty"`
}

// ApplicationStatusURL is a single application endpoint built from an Ingress
// of the application chart.
type ApplicationStatusURL struct {
	// URL of the application endpoint.
	URL string `json:"url"`

	// Description of the endpoint, taken from the value of the
	// `packages.deckhouse.io/application-endpoint-description` annotation.
	//
	// Empty when the annotation value is "true".
	// +optional
	Description string `json:"description,omitempty"`
}

type ApplicationStatusVersion struct {
	// Semantic version of the installed application.
	// +optional
	Version string `json:"version,omitempty"`

	// Release channel from which the version was installed.
	// +optional
	Channel string `json:"channel,omitempty"`
}

// +kubebuilder:object:root=true

// ApplicationList is a list of Application resources
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Application `json:"items"`
}
