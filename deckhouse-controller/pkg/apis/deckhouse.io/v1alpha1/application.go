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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	ApplicationResource = "applications"
	ApplicationKind     = "Application"

	ApplicationFinalizerStatisticRegistered = "application.deckhouse.io/statistic-registered"

	ApplicationAnnotationRegistrySpecChanged = "packages.deckhouse.io/registry-spec-changed"

	// ApplicationAnnotationIsEndpoint marks an Ingress in the application chart
	// as an application endpoint; its hosts and paths are reflected in status.urls.
	ApplicationAnnotationIsEndpoint = "packages.deckhouse.io/is-application-endpoint"
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
// +kubebuilder:resource:scope=Namespaced,shortName=app
// +kubebuilder:printcolumn:name=Package,type=string,JSONPath=.spec.packageName
// +kubebuilder:printcolumn:name=Version,type=string,JSONPath=.spec.packageVersion
// +kubebuilder:printcolumn:name=Repository,type=string,JSONPath=.spec.packageRepositoryName,priority=1
// +kubebuilder:printcolumn:name=State,type=string,JSONPath=.status.summary.state
// +kubebuilder:printcolumn:name=Installed,type=string,JSONPath=.status.conditions[?(@.type=='Installed')].status,priority=1
// +kubebuilder:printcolumn:name=Ready,type=string,JSONPath=.status.conditions[?(@.type=='Ready')].status,priority=1
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.summary.message"
// +kubebuilder:printcolumn:name=Age,type=date,JSONPath=.metadata.creationTimestamp

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
	PackageName string `json:"packageName"`

	// Name of the repository where the package is located.
	// If not specified, the default repository is used.
	// +optional
	PackageRepositoryName string `json:"packageRepositoryName,omitempty"`

	// Version of the application package to install.
	PackageVersion string `json:"packageVersion"`

	// Release channel for the application package.
	// +optional
	ReleaseChannel string `json:"releaseChannel,omitempty"`

	// Configuration settings for the application.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Settings *MappedFields `json:"settings,omitempty"`
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
	// application chart annotated with packages.deckhouse.io/is-application-endpoint.
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
	// Always one of: Pending, Failed, Updating, Ready, Degraded, Suspended.
	// +optional
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
	// packages.deckhouse.io/is-application-endpoint annotation.
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
