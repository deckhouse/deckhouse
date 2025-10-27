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
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	ClusterApplicationResource = "clusterapplications"
	ClusterApplicationKind     = "ClusterApplication"

	ClusterApplicationStatusHealthy  = "Healthy"
	ClusterApplicationStatusDegraded = "Degraded"
	ClusterApplicationStatusError    = "Error"

	ClusterApplicationConditionRequirementsMet        = "RequirementsMet"
	ClusterApplicationConditionStartupHooksSuccessful = "StartupHooksSuccessful"
	ClusterApplicationConditionManifestsDeployed      = "ManifestsDeployed"
	ClusterApplicationConditionReplicasAvailable      = "ReplicasAvailable"
)

var (
	ClusterApplicationGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: ClusterApplicationResource,
	}
	ClusterApplicationGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    ClusterApplicationKind,
	}
)

var _ runtime.Object = (*ClusterApplication)(nil)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// ClusterApplication represents a cluster-scoped application instance.
type ClusterApplication struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of a ClusterApplication.
	Spec ClusterApplicationSpec `json:"spec"`

	// Status of a ClusterApplication.
	Status ClusterApplicationStatus `json:"status,omitempty"`
}

type ClusterApplicationSpec struct {
	Repository     string `json:"repository,omitempty"`
	Version        string `json:"version"`
	ReleaseChannel string `json:"releaseChannel,omitempty"`
	// +kubebuilder:pruning:PreserveUnknownFields
	Settings *apiextensionsv1.JSON `json:"settings,omitempty"`
}

type ClusterApplicationStatus struct {
	Version    *ClusterApplicationStatusVersion    `json:"version,omitempty"`
	Repository string                              `json:"repository,omitempty"`
	Status     string                              `json:"status,omitempty"`
	Conditions []ClusterApplicationStatusCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

type ClusterApplicationStatusVersion struct {
	Current string `json:"current,omitempty"`
	Channel string `json:"channel,omitempty"`
}

type ClusterApplicationStatusCondition struct {
	Type               string                 `json:"type"`
	Status             corev1.ConditionStatus `json:"status"`
	Reason             string                 `json:"reason,omitempty"`
	Message            string                 `json:"message,omitempty"`
	LastProbeTime      metav1.Time            `json:"lastProbeTime,omitempty"`
	LastTransitionTime metav1.Time            `json:"lastTransitionTime,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterApplicationList is a list of ClusterApplication resources
type ClusterApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterApplication `json:"items"`
}
