/*
Copyright 2024 Flant JSC

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Descheduler is a struct containing descheduler policies
type Descheduler struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec DeschedulerSpec `json:"spec" yaml:"spec"`
}

type DeschedulerSpec struct {
	NodeSelector           string                  `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
	NodeLabelSelector      *metav1.LabelSelector   `json:"nodeLabelSelector,omitempty" yaml:"nodeLabelSelector,omitempty"`
	PodLabelSelector       *metav1.LabelSelector   `json:"podLabelSelector,omitempty" yaml:"podLabelSelector,omitempty"`
	NamespaceLabelSelector *metav1.LabelSelector   `json:"namespaceLabelSelector,omitempty" yaml:"namespaceLabelSelector,omitempty"`
	PriorityClassThreshold *PriorityClassThreshold `json:"priorityClassThreshold,omitempty" yaml:"priorityClassThreshold,omitempty"`
	EvictLocalStoragePods  *EvictLocalStoragePods  `json:"evictLocalStoragePods,omitempty" yaml:"evictLocalStoragePods,omitempty"`
	Strategies             Strategies              `json:"strategies" yaml:"strategies"`
}

type PriorityClassThreshold struct {
	Name  string `json:"name,omitempty" yaml:"name,omitempty"`
	Value int    `json:"value,omitempty" yaml:"value,omitempty"`
}

type EvictLocalStoragePods bool

type Strategies struct {
	LowNodeUtilization                      *LowNodeUtilization                      `json:"lowNodeUtilization,omitempty" yaml:"lowNodeUtilization,omitempty"`
	HighNodeUtilization                     *HighNodeUtilization                     `json:"highNodeUtilization,omitempty" yaml:"highNodeUtilization,omitempty"`
	RemoveDuplicates                        *RemoveDuplicates                        `json:"removeDuplicates,omitempty" yaml:"removeDuplicates,omitempty"`
	RemovePodsViolatingNodeAffinity         *RemovePodsViolatingNodeAffinity         `json:"removePodsViolatingNodeAffinity,omitempty" yaml:"removePodsViolatingNodeAffinity,omitempty"`
	RemovePodsViolatingInterPodAntiAffinity *RemovePodsViolatingInterPodAntiAffinity `json:"removePodsViolatingInterPodAntiAffinity,omitempty" yaml:"removePodsViolatingInterPodAntiAffinity,omitempty"`
}

type LowNodeUtilization struct {
	Enabled          bool                   `json:"enabled" yaml:"enabled"`
	Thresholds       map[string]interface{} `json:"thresholds,omitempty" yaml:"thresholds,omitempty"`
	TargetThresholds map[string]interface{} `json:"targetThresholds,omitempty" yaml:"targetThresholds,omitempty"`
}

type HighNodeUtilization struct {
	Enabled    bool                   `json:"enabled" yaml:"enabled"`
	Thresholds map[string]interface{} `json:"thresholds,omitempty" yaml:"thresholds,omitempty"`
}

type RemoveDuplicates struct {
	Enabled bool `json:"enabled" yaml:"enabled"`
}

type RemovePodsViolatingNodeAffinity struct {
	Enabled          bool     `json:"enabled" yaml:"enabled"`
	NodeAffinityType []string `json:"nodeAffinityType,omitempty" yaml:"nodeAffinityType,omitempty"`
}

type RemovePodsViolatingInterPodAntiAffinity struct {
	Enabled bool `json:"enabled" yaml:"enabled"`
}
