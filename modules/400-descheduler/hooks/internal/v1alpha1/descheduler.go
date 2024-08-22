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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	FitNodesLabelSelector     *metav1.LabelSelector   `json:"fitNodesLabelSelector,omitempty" yaml:"fitNodesLabelSelector,omitempty"`
	PodLabelSelector          *metav1.LabelSelector   `json:"podLabelSelector,omitempty" yaml:"podLabelSelector,omitempty"`
	PodNamespaceLabelSelector *metav1.LabelSelector   `json:"podNamespaceLabelSelector,omitempty" yaml:"podNamespaceLabelSelector,omitempty"`
	PriorityClassThreshold    *PriorityClassThreshold `json:"priorityClassThreshold,omitempty" yaml:"priorityClassThreshold,omitempty"`
	Strategies                Strategies              `json:"strategies" yaml:"strategies"`
}

type PriorityClassThreshold struct {
	Name  string `json:"name,omitempty" yaml:"name,omitempty"`
	Value int    `json:"value,omitempty" yaml:"value,omitempty"`
}

type Strategies struct {
	LowNodeUtilization  *LowNodeUtilization  `json:"lowNodeUtilization,omitempty" yaml:"lowNodeUtilization,omitempty"`
	HighNodeUtilization *HighNodeUtilization `json:"highNodeUtilization,omitempty" yaml:"highNodeUtilization,omitempty"`
}

type LowNodeUtilization struct {
	Thresholds       map[string]interface{} `json:"thresholds" yaml:"thresholds"`
	TargetThresholds map[string]interface{} `json:"targetThresholds" yaml:"targetThresholds"`
}

type HighNodeUtilization struct {
	Thresholds map[string]interface{} `json:"thresholds" yaml:"thresholds"`
}

type deschedulerKind struct{}

func (f *deschedulerKind) SetGroupVersionKind(_ schema.GroupVersionKind) {}
func (f *deschedulerKind) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "Descheduler"}
}
