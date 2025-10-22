/*
Copyright 2021 Flant JSC

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

// ClusterLoggingConfig specify target for logs collecting
type ClusterLoggingConfig struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of a cluster log source.
	Spec ClusterLoggingConfigSpec `json:"spec"`

	// Most recently observed status of a cluster log source.
	// Populated by the system.

	Status ClusterLoggingConfigStatus `json:"status,omitempty"`
}

type ClusterLoggingConfigSpec struct {
	// Type of cluster log source: KubernetesPods, File
	Type string `json:"type,omitempty"`

	// KubernetesPods describes spec for kubernetes pod source
	KubernetesPods KubernetesPodsSpec `json:"kubernetesPods,omitempty"`

	// File describes spec for file source
	File FileSpec `json:"file,omitempty"`

	// Filters
	LogFilters   []Filter `json:"logFilter,omitempty"`
	LabelFilters []Filter `json:"labelFilter,omitempty"`

	// Multiline parsers
	MultiLineParser MultiLineParser `json:"multilineParser,omitempty"`

	// DestinationRefs slice of ClusterLogDestination names
	DestinationRefs []string `json:"destinationRefs,omitempty"`
}

type ClusterLoggingConfigStatus struct {
}

type KubernetesPodsSpec struct {
	NamespaceSelector NamespaceSelector `json:"namespaceSelector,omitempty"`

	LabelSelector metav1.LabelSelector `json:"labelSelector,omitempty"`

	KeepDeletedFilesOpenedFor metav1.Duration `json:"keepDeletedFilesOpenedFor,omitempty"`
}

type NamespaceSelector struct {
	MatchNames   []string `json:"matchNames"`
	ExcludeNames []string `json:"excludeNames"`

	LabelSelector metav1.LabelSelector `json:"labelSelector,omitempty"`
}

type FileSpec struct {
	Include       []string `json:"include,omitempty"`
	Exclude       []string `json:"exclude,omitempty"`
	LineDelimiter string   `json:"lineDelimiter,omitempty"`
}
