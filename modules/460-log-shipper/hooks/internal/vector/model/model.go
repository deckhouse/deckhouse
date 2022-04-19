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

package model

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/impl"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/v1alpha1"
)

const (
	DestElasticsearch = "Elasticsearch"
	DestLogstash      = "Logstash"
	DestLoki          = "Loki"
)

const (
	SourceKubernetesPods = "KubernetesPods"
	SourceFile           = "File"
)

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
	Status v1alpha1.ClusterLoggingConfigStatus `json:"status,omitempty"`
}

type ClusterLoggingConfigSpec struct {
	// Type of cluster log source: KubernetesPods, File
	Type string `json:"type,omitempty"`

	// KubernetesPods describes spec for kubernetes pod source
	KubernetesPods v1alpha1.KubernetesPodsSpec `json:"kubernetesPods,omitempty"`

	// File describes spec for file source
	File v1alpha1.FileSpec `json:"file,omitempty"`

	// Transforms set ordered array of transforms. Possible values you can find into crd
	Transforms []impl.LogTransform `json:"transforms"`

	// DestinationRefs slice of ClusterLogDestination names
	DestinationRefs []string `json:"destinationRefs,omitempty"`
}

// PodLoggingConfig specify target for kubernetes pods logs collecting in specified namespace
type PodLoggingConfig struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of a namespaced log source.
	Spec PodLoggingConfigSpec `json:"spec"`

	// Most recently observed status of a namespaced log source.
	Status v1alpha1.PodLoggingConfigStatus `json:"status,omitempty"`
}

type PodLoggingConfigSpec struct {
	// LabelSelector filter pods by label
	LabelSelector metav1.LabelSelector `json:"labelSelector,omitempty"`

	// Transforms set ordered array of transforms. Possible values you can find into crd
	Transforms []impl.LogTransform `json:"transforms"`

	// ClusterDestinationRefs slice of ClusterLogDestination names
	ClusterDestinationRefs []string `json:"clusterDestinationRefs,omitempty"`
}
